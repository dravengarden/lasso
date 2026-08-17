package worktree

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateName(t *testing.T) {
	valid := []string{"perf-test", "a", "deepseek.telemetry.v2", "machine_verify"}
	for _, name := range valid {
		if err := ValidateName(name); err != nil {
			t.Fatalf("ValidateName(%q): %v", name, err)
		}
	}
	invalid := []string{"", "UPPER", "with space", "a/b", ".hidden", "..", strings.Repeat("x", 65)}
	for _, name := range invalid {
		if err := ValidateName(name); err == nil {
			t.Fatalf("ValidateName(%q) accepted invalid name", name)
		}
	}
}

func TestCanonicalPath(t *testing.T) {
	got := CanonicalPath("/wt", "cowboy", "perf-test")
	want := filepath.Join("/wt", "cowboy", "perf-test")
	if got != want {
		t.Fatalf("CanonicalPath = %q, want %q", got, want)
	}
}

func TestClassifyRequiresProjectAndNameSegments(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "worktrees")
	project, name, kind := classify([]string{root}, filepath.Join(root, "legacy-task"), "", "alpha")
	if project != "alpha" || name != "legacy-task" || kind != KindLegacy {
		t.Fatalf("classify top-level worktree = %q, %q, %q; want alpha, legacy-task, legacy", project, name, kind)
	}
}

func TestClassifyKeepsDefaultHomeRootWhenOverrideSet(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	primary := filepath.Join(string(filepath.Separator), "srv", "storage", "fast0", "agent", "worktrees")
	legacy := filepath.Join(home, "worktrees", "cowboy", "old-task")
	project, name, kind := classify(TaskRoots(primary), legacy, "", "cowboy")
	if project != "cowboy" || name != "old-task" || kind != KindTask {
		t.Fatalf("classify override+home = %q, %q, %q; want cowboy, old-task, task", project, name, kind)
	}
	fresh := filepath.Join(primary, "cowboy", "new-task")
	project, name, kind = classify(TaskRoots(primary), fresh, "", "cowboy")
	if project != "cowboy" || name != "new-task" || kind != KindTask {
		t.Fatalf("classify override create root = %q, %q, %q; want cowboy, new-task, task", project, name, kind)
	}
}

func TestResolveTaskPathPrefersPrimaryThenHome(t *testing.T) {
	rootDir := t.TempDir()
	primary := filepath.Join(rootDir, "fast0")
	if err := os.MkdirAll(CanonicalPath(primary, "cowboy", "new-task"), 0o750); err != nil {
		t.Fatalf("mkdir primary: %v", err)
	}
	got, err := resolveTaskPath([]string{primary}, "cowboy", "new-task")
	if err != nil {
		t.Fatalf("resolveTaskPath: %v", err)
	}
	if got != CanonicalPath(primary, "cowboy", "new-task") {
		t.Fatalf("resolveTaskPath = %q", got)
	}
	if _, err := resolveTaskPath([]string{primary}, "cowboy", "missing"); err == nil {
		t.Fatal("resolveTaskPath accepted a missing worktree")
	}
}

func TestCreateListRemoveRoundTrip(t *testing.T) {
	ctx := context.Background()
	fixture := newWorktreeFixture(t)

	path, branch, err := Create(ctx, fixture.repo, fixture.wtRoot, "alpha", "perf-test", CreateOptions{
		From:  "origin/main",
		Fetch: true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if branch != "codex/perf-test" {
		t.Fatalf("branch = %q, want codex/perf-test", branch)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("created worktree missing: %v", err)
	}
	if !refExists(t, fixture.repo, "refs/heads/codex/perf-test") {
		t.Fatal("branch ref missing")
	}

	entries, err := List(ctx, fixture.wtRoot, fixture.repos(), "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	task := findEntry(entries, "alpha", "perf-test")
	if task == nil {
		t.Fatalf("task worktree not listed: %+v", entries)
	}
	if task.Kind != KindTask || task.Branch != branch || !task.Merged || task.Dirty {
		t.Fatalf("unexpected task entry: %+v", task)
	}

	if err := os.WriteFile(filepath.Join(path, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("write dirty fixture: %v", err)
	}
	entries, err = List(ctx, fixture.wtRoot, fixture.repos(), "")
	if err != nil {
		t.Fatalf("List after dirty: %v", err)
	}
	if task = findEntry(entries, "alpha", "perf-test"); task == nil || !task.Dirty {
		t.Fatalf("dirty flag not reported: %+v", task)
	}
	if err := os.Remove(filepath.Join(path, "dirty.txt")); err != nil {
		t.Fatalf("clean dirty fixture: %v", err)
	}

	if err := Remove(ctx, fixture.repo, "main", fixture.wtRoot, "alpha", "perf-test", RemoveOptions{
		DeleteBranch: true,
	}); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("worktree still present: %v", err)
	}
	if refExists(t, fixture.repo, "refs/heads/codex/perf-test") {
		t.Fatal("local branch was not deleted")
	}
}

func TestRemoveRefusesDirtyAndUnmerged(t *testing.T) {
	ctx := context.Background()
	fixture := newWorktreeFixture(t)

	path, _, err := Create(ctx, fixture.repo, fixture.wtRoot, "alpha", "dirty-one", CreateOptions{From: "origin/main"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "keep.txt"), []byte("keep\n"), 0o644); err != nil {
		t.Fatalf("write dirty fixture: %v", err)
	}
	if err := Remove(ctx, fixture.repo, "main", fixture.wtRoot, "alpha", "dirty-one", RemoveOptions{}); err == nil {
		t.Fatal("Remove accepted a dirty worktree")
	}

	unmergedPath, _, err := Create(ctx, fixture.repo, fixture.wtRoot, "alpha", "unmerged-one", CreateOptions{From: "origin/main"})
	if err != nil {
		t.Fatalf("Create unmerged: %v", err)
	}
	runGit(t, unmergedPath, "-c", "user.name=Lasso Test", "-c", "user.email=test@example.invalid",
		"commit", "--allow-empty", "-m", "unmerged")
	if err := Remove(ctx, fixture.repo, "main", fixture.wtRoot, "alpha", "unmerged-one", RemoveOptions{}); err == nil {
		t.Fatal("Remove accepted an unmerged worktree")
	}
}

func TestPlanAndRunGC(t *testing.T) {
	ctx := context.Background()
	fixture := newWorktreeFixture(t)

	if _, _, err := Create(ctx, fixture.repo, fixture.wtRoot, "alpha", "merged-one", CreateOptions{From: "origin/main"}); err != nil {
		t.Fatalf("Create merged: %v", err)
	}
	unmergedPath, _, err := Create(ctx, fixture.repo, fixture.wtRoot, "alpha", "unmerged-two", CreateOptions{From: "origin/main"})
	if err != nil {
		t.Fatalf("Create unmerged: %v", err)
	}
	runGit(t, unmergedPath, "-c", "user.name=Lasso Test", "-c", "user.email=test@example.invalid",
		"commit", "--allow-empty", "-m", "unmerged")

	actions, err := PlanGC(ctx, fixture.wtRoot, fixture.repos(), "", GCOptions{MinAge: 0})
	if err != nil {
		t.Fatalf("PlanGC: %v", err)
	}
	got := actionsByEntry(actions)
	if got["alpha/merged-one"] != "remove" {
		t.Fatalf("merged-one action = %q, want remove (%+v)", got["alpha/merged-one"], actions)
	}
	if got["alpha/unmerged-two"] != "skip: unmerged" {
		t.Fatalf("unmerged-two action = %q, want skip: unmerged (%+v)", got["alpha/unmerged-two"], actions)
	}

	done, err := RunGC(ctx, fixture.wtRoot, fixture.repos(), "", GCOptions{MinAge: 0})
	if err != nil {
		t.Fatalf("RunGC: %v", err)
	}
	if gotByEntry(done, "alpha/merged-one") != "remove" {
		t.Fatalf("RunGC did not remove merged-one: %+v", done)
	}
	if _, err := os.Stat(CanonicalPath(fixture.wtRoot, "alpha", "merged-one")); !os.IsNotExist(err) {
		t.Fatal("merged-one still present after GC")
	}
	if _, err := os.Stat(CanonicalPath(fixture.wtRoot, "alpha", "unmerged-two")); err != nil {
		t.Fatalf("unmerged-two was removed: %v", err)
	}
}

func TestPlanGCSkipsYoungWorktree(t *testing.T) {
	ctx := context.Background()
	fixture := newWorktreeFixture(t)
	if _, _, err := Create(ctx, fixture.repo, fixture.wtRoot, "alpha", "young-one", CreateOptions{From: "origin/main"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	actions, err := PlanGC(ctx, fixture.wtRoot, fixture.repos(), "", GCOptions{MinAge: 48 * time.Hour})
	if err != nil {
		t.Fatalf("PlanGC: %v", err)
	}
	if gotByEntry(actions, "alpha/young-one") != "skip: too young" {
		t.Fatalf("young worktree action = %+v", actions)
	}
}

type worktreeFixture struct {
	repo    string
	wtRoot  string
	writer  string
	rootDir string
}

func newWorktreeFixture(t *testing.T) *worktreeFixture {
	t.Helper()
	rootDir := t.TempDir()
	remote := filepath.Join(rootDir, "remote.git")
	repo := filepath.Join(rootDir, "repo")
	writer := filepath.Join(rootDir, "writer")
	wtRoot := filepath.Join(rootDir, "worktrees")

	runGit(t, rootDir, "init", "--bare", "--initial-branch=main", remote)
	runGit(t, rootDir, "clone", remote, writer)
	runGit(t, writer, "config", "user.name", "Lasso Test")
	runGit(t, writer, "config", "user.email", "test@example.invalid")
	if err := os.WriteFile(filepath.Join(writer, "README.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	runGit(t, writer, "add", "README.md")
	runGit(t, writer, "commit", "-m", "fixture")
	runGit(t, writer, "push", "-u", "origin", "main")
	runGit(t, rootDir, "clone", remote, repo)
	runGit(t, repo, "config", "user.name", "Lasso Test")
	runGit(t, repo, "config", "user.email", "test@example.invalid")

	return &worktreeFixture{repo: repo, wtRoot: wtRoot, writer: writer, rootDir: rootDir}
}

func (f *worktreeFixture) repos() []Repo {
	return []Repo{{
		Project:       "alpha",
		Repo:          f.repo,
		Stable:        f.repo,
		DefaultBranch: "main",
	}}
}

func findEntry(entries []Entry, project, name string) *Entry {
	for i := range entries {
		if entries[i].Project == project && entries[i].Name == name {
			return &entries[i]
		}
	}
	return nil
}

func actionsByEntry(actions []Action) map[string]string {
	got := make(map[string]string, len(actions))
	for _, a := range actions {
		got[a.Entry.Project+"/"+a.Entry.Name] = a.Action
	}
	return got
}

func gotByEntry(actions []Action, key string) string {
	return actionsByEntry(actions)[key]
}

func refExists(t *testing.T, repo, ref string) bool {
	t.Helper()
	cmd := exec.Command("git", "-C", repo, "rev-parse", "--verify", "--quiet", ref)
	return cmd.Run() == nil
}

func runGit(t *testing.T, cwd string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
}
