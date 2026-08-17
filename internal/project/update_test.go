package project

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dravengarden/lasso/internal/config"
)

func TestUpdateTreatsSubdirAsNotApplicable(t *testing.T) {
	root := t.TempDir()
	defs := filepath.Join(root, config.ProjectDefsDir)
	if err := os.MkdirAll(filepath.Join(defs, "local"), 0o750); err != nil {
		t.Fatalf("mkdir registry: %v", err)
	}
	if err := os.WriteFile(filepath.Join(defs, "registry.toml"), []byte("version = 1\n"), 0o644); err != nil {
		t.Fatalf("write registry marker: %v", err)
	}
	if err := os.WriteFile(config.ProjectFilePath(root, "local"), []byte("kind = \"subdir\"\n"), 0o644); err != nil {
		t.Fatalf("write project: %v", err)
	}

	result, err := Update(context.Background(), root, "local", UpdateOptions{Fetch: true})
	if err != nil {
		t.Fatalf("Update subdir: %v", err)
	}
	if !result.NotApplicable || result.RegistryUpdated || result.Checkout != nil {
		t.Fatalf("subdir update should be a no-op: %+v", result)
	}
}

func TestStableCheckoutPathPrefersFlatRepositoryOverMainContent(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "alpha")
	if err := os.MkdirAll(filepath.Join(projectDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir git metadata: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, "main"), 0o755); err != nil {
		t.Fatalf("mkdir main content: %v", err)
	}
	if got := stableCheckoutPath(projectDir); got != projectDir {
		t.Fatalf("stableCheckoutPath = %q, want flat repository %q", got, projectDir)
	}
}

func TestStableCheckoutPathRejectsNestedLegacyRepository(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "alpha")
	legacy := filepath.Join(projectDir, "main")
	if err := os.MkdirAll(filepath.Join(legacy, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir legacy checkout: %v", err)
	}
	if got := stableCheckoutPath(projectDir); got != "" {
		t.Fatalf("stableCheckoutPath = %q, want no flat repository", got)
	}
}

func TestPatchRegistryMetadataPersistsReconciledDefaultBranch(t *testing.T) {
	root := t.TempDir()
	defs := filepath.Join(root, config.ProjectDefsDir)
	if err := os.MkdirAll(filepath.Join(defs, "alpha"), 0o750); err != nil {
		t.Fatalf("mkdir registry: %v", err)
	}
	if err := os.WriteFile(filepath.Join(defs, "registry.toml"), []byte("version = 1\n"), 0o644); err != nil {
		t.Fatalf("write registry marker: %v", err)
	}
	old := projectEntry{Repo: "git@github.com:org/alpha.git", DefaultBranch: "master"}
	if err := os.WriteFile(config.ProjectFilePath(root, "alpha"), []byte(renderProjectTOML(Spec{
		Name: "alpha", Repo: old.Repo, DefaultBranch: old.DefaultBranch,
	})), 0o644); err != nil {
		t.Fatalf("write project: %v", err)
	}
	res := &UpdateResult{
		Name: "alpha", OldDefaultBranch: "master", NewDefaultBranch: "main", DefaultBranchChanged: true,
	}
	if err := patchRegistryMetadata(root, old, UpdateOptions{}, res); err != nil {
		t.Fatalf("patchRegistryMetadata: %v", err)
	}
	if !res.RegistryUpdated {
		t.Fatal("RegistryUpdated = false")
	}
	registry, err := config.LoadFromRoot(root)
	if err != nil {
		t.Fatalf("LoadFromRoot: %v", err)
	}
	project, err := registry.MustGet("alpha")
	if err != nil {
		t.Fatalf("MustGet: %v", err)
	}
	if project.DefaultBranch != "main" || project.Repo != old.Repo {
		t.Fatalf("project = %#v", project)
	}
}

func TestPatchRegistryMetadataCheckOnlyDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	defs := filepath.Join(root, config.ProjectDefsDir)
	if err := os.MkdirAll(filepath.Join(defs, "alpha"), 0o750); err != nil {
		t.Fatalf("mkdir registry: %v", err)
	}
	path := config.ProjectFilePath(root, "alpha")
	original := renderProjectTOML(Spec{Name: "alpha", Repo: "git@example/alpha", DefaultBranch: "master"})
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("write project: %v", err)
	}
	res := &UpdateResult{Name: "alpha", NewDefaultBranch: "main", DefaultBranchChanged: true}
	if err := patchRegistryMetadata(root, projectEntry{Repo: "git@example/alpha"}, UpdateOptions{CheckOnly: true}, res); err != nil {
		t.Fatalf("patchRegistryMetadata: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read project: %v", err)
	}
	if string(got) != original || res.RegistryUpdated {
		t.Fatalf("check-only mutated registry: updated=%v body=%q", res.RegistryUpdated, got)
	}
}

func TestPatchRegistryMetadataDoesNotRelocateFlatCheckout(t *testing.T) {
	root := t.TempDir()
	defs := filepath.Join(root, config.ProjectDefsDir)
	if err := os.MkdirAll(filepath.Join(defs, "alpha"), 0o750); err != nil {
		t.Fatalf("mkdir registry: %v", err)
	}
	old := projectEntry{Repo: "git@example/alpha", DefaultBranch: "master"}
	path := config.ProjectFilePath(root, "alpha")
	original := renderProjectTOML(Spec{Name: "alpha", Repo: old.Repo, DefaultBranch: old.DefaultBranch})
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("write project: %v", err)
	}
	checkout := filepath.Join(root, config.ProjectsRoot, "alpha")
	if err := os.MkdirAll(filepath.Join(checkout, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir flat checkout: %v", err)
	}
	res := &UpdateResult{
		Name: "alpha", OldDefaultBranch: "master", NewDefaultBranch: "main", DefaultBranchChanged: true,
	}

	if err := patchRegistryMetadata(root, old, UpdateOptions{}, res); err != nil {
		t.Fatalf("patchRegistryMetadata: %v", err)
	}
	got, readErr := os.ReadFile(path)
	want := renderProjectTOML(Spec{Name: "alpha", Repo: old.Repo, DefaultBranch: "main"})
	if readErr != nil || string(got) != want || !res.RegistryUpdated {
		t.Fatalf("registry update = updated=%v body=%q err=%v", res.RegistryUpdated, got, readErr)
	}
	if gotPath := stableCheckoutPath(checkout); gotPath != checkout {
		t.Fatalf("stable checkout moved after branch update: got %q want %q", gotPath, checkout)
	}
}

func TestUpdateUsesDetectedRemoteDefaultBranchForCheckout(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	writer := filepath.Join(root, "writer")
	stable := filepath.Join(root, config.ProjectsRoot, "alpha")

	runGit(t, root, "init", "--bare", "--initial-branch=master", remote)
	runGit(t, root, "clone", remote, writer)
	if err := os.WriteFile(filepath.Join(writer, "README.md"), []byte("master\n"), 0o644); err != nil {
		t.Fatalf("write master fixture: %v", err)
	}
	runGit(t, writer, "add", "README.md")
	runGit(t, writer, "-c", "user.name=Lasso Test", "-c", "user.email=test@example.invalid", "commit", "-m", "master")
	runGit(t, writer, "push", "-u", "origin", "master")
	runGit(t, writer, "switch", "-c", "main")
	if err := os.WriteFile(filepath.Join(writer, "main.txt"), []byte("main\n"), 0o644); err != nil {
		t.Fatalf("write main fixture: %v", err)
	}
	runGit(t, writer, "add", "main.txt")
	runGit(t, writer, "-c", "user.name=Lasso Test", "-c", "user.email=test@example.invalid", "commit", "-m", "main")
	runGit(t, writer, "push", "-u", "origin", "main")
	runGit(t, remote, "symbolic-ref", "HEAD", "refs/heads/main")

	if err := os.MkdirAll(filepath.Dir(stable), 0o755); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}
	runGit(t, root, "clone", remote, stable)
	defs := filepath.Join(root, config.ProjectDefsDir)
	if err := os.MkdirAll(filepath.Join(defs, "alpha"), 0o755); err != nil {
		t.Fatalf("mkdir project definition: %v", err)
	}
	if err := os.WriteFile(filepath.Join(defs, "registry.toml"), []byte("version = 1\n"), 0o644); err != nil {
		t.Fatalf("write registry marker: %v", err)
	}
	if err := os.WriteFile(config.ProjectFilePath(root, "alpha"), []byte(renderProjectTOML(Spec{
		Name: "alpha", Repo: remote, DefaultBranch: "master",
	})), 0o644); err != nil {
		t.Fatalf("write project definition: %v", err)
	}

	result, err := Update(context.Background(), root, "alpha", UpdateOptions{Fetch: true, CheckOnly: true})
	if err != nil {
		t.Fatalf("Update default branch drift: %v", err)
	}
	if !result.DefaultBranchChanged || result.NewDefaultBranch != "main" {
		t.Fatalf("default branch drift not detected: %+v", result)
	}
	if result.Checkout == nil || result.Checkout.Branch != "main" || result.Checkout.ExpectedBranch != "main" || result.Checkout.State != "up-to-date" {
		t.Fatalf("checkout was not evaluated against detected main branch: %+v", result.Checkout)
	}
}

func TestCheckoutStatusFastForwardsCleanExpectedBranch(t *testing.T) {
	stable, writer := newUpdateGitFixture(t)
	want := advanceUpdateRemote(t, writer, "remote.txt")

	status := checkoutStatus(context.Background(), stable, "main", UpdateOptions{
		Fetch:      true,
		PullFFOnly: true,
	})

	if status.State != "up-to-date" || status.Dirty || !status.FetchAttempted || !status.FetchOK {
		t.Fatalf("unexpected clean fast-forward status: %+v", status)
	}
	if status.PulledFastForward == nil || !*status.PulledFastForward || status.PullBlockedReason != "" {
		t.Fatalf("fast-forward was not recorded as successful: %+v", status)
	}
	if got := gitOutput(t, stable, "rev-parse", "HEAD"); got != want {
		t.Fatalf("stable HEAD = %s, want %s", got, want)
	}
}

func TestCheckoutStatusRefusesDirtyBehindCheckout(t *testing.T) {
	stable, writer := newUpdateGitFixture(t)
	before := gitOutput(t, stable, "rev-parse", "HEAD")
	advanceUpdateRemote(t, writer, "remote.txt")
	if err := os.WriteFile(filepath.Join(stable, "local.txt"), []byte("local\n"), 0o644); err != nil {
		t.Fatalf("write dirty fixture: %v", err)
	}

	status := checkoutStatus(context.Background(), stable, "main", UpdateOptions{
		Fetch:      true,
		PullFFOnly: true,
	})

	if status.State != "behind 1" || !status.Dirty || status.PullBlockedReason != "checkout is dirty" {
		t.Fatalf("dirty checkout was not blocked: %+v", status)
	}
	if status.PulledFastForward != nil {
		t.Fatalf("dirty checkout attempted a fast-forward: %+v", status)
	}
	if got := gitOutput(t, stable, "rev-parse", "HEAD"); got != before {
		t.Fatalf("dirty stable HEAD moved from %s to %s", before, got)
	}
}

func TestCheckoutStatusRefusesWrongBranch(t *testing.T) {
	stable, writer := newUpdateGitFixture(t)
	runGit(t, stable, "switch", "-c", "local-work")
	before := gitOutput(t, stable, "rev-parse", "HEAD")
	advanceUpdateRemote(t, writer, "remote.txt")

	status := checkoutStatus(context.Background(), stable, "main", UpdateOptions{
		Fetch:      true,
		PullFFOnly: true,
	})

	if status.State != "behind 1" || status.Branch != "local-work" {
		t.Fatalf("wrong-branch status = %+v", status)
	}
	if !strings.Contains(status.PullBlockedReason, "is not expected branch main") {
		t.Fatalf("wrong branch was not blocked: %+v", status)
	}
	if got := gitOutput(t, stable, "rev-parse", "HEAD"); got != before {
		t.Fatalf("wrong-branch HEAD moved from %s to %s", before, got)
	}
}

func TestCheckoutStatusMarksUnfetchedRefsAsCached(t *testing.T) {
	stable, writer := newUpdateGitFixture(t)
	advanceUpdateRemote(t, writer, "remote.txt")

	status := checkoutStatus(context.Background(), stable, "main", UpdateOptions{})

	if status.FetchAttempted || status.FetchOK || status.State != "up-to-date" {
		t.Fatalf("unfetched status must be explicitly cached: %+v", status)
	}
}

func newUpdateGitFixture(t *testing.T) (stable, writer string) {
	t.Helper()
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	writer = filepath.Join(root, "writer")
	stable = filepath.Join(root, "stable")

	runGit(t, root, "init", "--bare", "--initial-branch=main", remote)
	runGit(t, root, "clone", remote, writer)
	if err := os.WriteFile(filepath.Join(writer, "README.md"), []byte("initial\n"), 0o644); err != nil {
		t.Fatalf("write initial fixture: %v", err)
	}
	runGit(t, writer, "add", "README.md")
	runGit(t, writer, "-c", "user.name=Lasso Test", "-c", "user.email=test@example.invalid", "commit", "-m", "initial")
	runGit(t, writer, "push", "-u", "origin", "main")
	runGit(t, root, "clone", remote, stable)

	return stable, writer
}

func advanceUpdateRemote(t *testing.T, writer, name string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(writer, name), []byte(name+"\n"), 0o644); err != nil {
		t.Fatalf("write remote fixture: %v", err)
	}
	runGit(t, writer, "add", name)
	runGit(t, writer, "-c", "user.name=Lasso Test", "-c", "user.email=test@example.invalid", "commit", "-m", "advance")
	runGit(t, writer, "push", "origin", "main")

	return gitOutput(t, writer, "rev-parse", "HEAD")
}

func runGit(t *testing.T, cwd string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func gitOutput(t *testing.T, cwd string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}

	return strings.TrimSpace(string(output))
}
