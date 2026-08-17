package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dravengarden/lasso/internal/config"
	"github.com/dravengarden/lasso/internal/gitx"
)

func TestEnsureCheckoutCreatesOrdinaryClone(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(t.TempDir(), "source")
	if err := gitx.Run(t.Context(), "", true, "git", "init", "--initial-branch=main", source); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	for _, args := range [][]string{
		{"config", "user.name", "Test"},
		{"config", "user.email", "test@example.com"},
		{"add", "README.md"},
		{"commit", "-m", "initial"},
	} {
		if err := gitx.Run(t.Context(), source, true, "git", args...); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}

	registry := &config.Root{Projects: map[string]config.Project{
		"alpha": {Kind: config.KindExternal, Repo: source, DefaultBranch: "main"},
	}}
	result, err := EnsureCheckout(t.Context(), registry, root, "alpha")
	if err != nil {
		t.Fatalf("EnsureCheckout: %v", err)
	}
	if !result.Cloned || result.AlreadyExisted {
		t.Fatalf("result = %+v", result)
	}
	if info, statErr := os.Stat(filepath.Join(result.Path, ".git")); statErr != nil || !info.IsDir() {
		t.Fatalf("ordinary .git directory missing: %v", statErr)
	}

	again, err := EnsureCheckout(t.Context(), registry, root, "alpha")
	if err != nil || !again.AlreadyExisted || again.Cloned {
		t.Fatalf("idempotent EnsureCheckout = (%+v, %v)", again, err)
	}
}

func TestNormalizeRepoIdentityDistinguishesRemotePorts(t *testing.T) {
	one, err := normalizeRepoIdentity(t.TempDir(), "ssh://git@example.com:22/org/repo.git")
	if err != nil {
		t.Fatalf("normalize port 22: %v", err)
	}
	two, err := normalizeRepoIdentity(t.TempDir(), "ssh://git@example.com:2222/org/repo.git")
	if err != nil {
		t.Fatalf("normalize port 2222: %v", err)
	}
	if one == two {
		t.Fatalf("remote identities unexpectedly match: %q", one)
	}
}

func TestEnsureCheckoutIsIdempotentForRelativeLocalRepository(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := gitx.Run(t.Context(), "", true, "git", "init", "--initial-branch=main", source); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := gitx.Run(t.Context(), source, true, "git", "config", "user.name", "Test"); err != nil {
		t.Fatalf("configure name: %v", err)
	}
	if err := gitx.Run(t.Context(), source, true, "git", "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("configure email: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := gitx.Run(t.Context(), source, true, "git", "add", "README.md"); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := gitx.Run(t.Context(), source, true, "git", "commit", "-m", "initial"); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	registry := &config.Root{Projects: map[string]config.Project{
		"alpha": {Kind: config.KindExternal, Repo: "../source", DefaultBranch: "main"},
	}}
	if _, err := EnsureCheckout(t.Context(), registry, root, "alpha"); err != nil {
		t.Fatalf("first EnsureCheckout: %v", err)
	}
	result, err := EnsureCheckout(t.Context(), registry, root, "alpha")
	if err != nil || !result.AlreadyExisted {
		t.Fatalf("second EnsureCheckout = (%+v, %v)", result, err)
	}
}

func TestEnsureCheckoutRejectsNonCloneAtStablePath(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, config.ProjectsRoot, "alpha")
	if err := os.MkdirAll(filepath.Join(projectDir, "main"), 0o755); err != nil {
		t.Fatalf("mkdir checkout: %v", err)
	}
	registry := &config.Root{Projects: map[string]config.Project{
		"alpha": {Kind: config.KindExternal, Repo: "unused", DefaultBranch: "main"},
	}}

	if _, err := EnsureCheckout(t.Context(), registry, root, "alpha"); err == nil {
		t.Fatal("EnsureCheckout unexpectedly accepted a non-clone path")
	}
}

func TestEnsureCheckoutRejectsNestedLegacyClone(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(t.TempDir(), "source")
	if err := gitx.Run(t.Context(), "", true, "git", "init", "--bare", source); err != nil {
		t.Fatalf("git init source: %v", err)
	}
	legacy := filepath.Join(root, config.ProjectsRoot, "alpha", "main")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := gitx.Run(t.Context(), "", true, "git", "clone", source, legacy); err != nil {
		t.Fatalf("git clone: %v", err)
	}
	if err := gitx.Run(t.Context(), legacy, true, "git", "switch", "-c", "main"); err != nil {
		t.Fatalf("git switch: %v", err)
	}
	registry := &config.Root{Projects: map[string]config.Project{
		"alpha": {Kind: config.KindExternal, Repo: source, DefaultBranch: "main"},
	}}

	result, err := EnsureCheckout(t.Context(), registry, root, "alpha")
	if err == nil || result.AlreadyExisted || result.Path != filepath.Dir(legacy) {
		t.Fatalf("EnsureCheckout = (%+v, %v), want nested legacy rejection", result, err)
	}
}

func TestEnsureCheckoutRejectsWrongExistingClone(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, config.ProjectsRoot, "alpha")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := gitx.Run(t.Context(), "", true, "git", "init", "--initial-branch=main", target); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := gitx.Run(t.Context(), target, true, "git", "remote", "add", "origin", "git@github.com:other/repo.git"); err != nil {
		t.Fatalf("add origin: %v", err)
	}
	registry := &config.Root{Projects: map[string]config.Project{
		"alpha": {Kind: config.KindExternal, Repo: "https://github.com/example/repo.git", DefaultBranch: "main"},
	}}

	if _, err := EnsureCheckout(t.Context(), registry, root, "alpha"); err == nil || !strings.Contains(err.Error(), "origin mismatch") {
		t.Fatalf("EnsureCheckout error = %v, want origin mismatch", err)
	}
}

func TestEnsureCheckoutRejectsWrongExistingBranch(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(t.TempDir(), "source")
	if err := gitx.Run(t.Context(), "", true, "git", "init", "--initial-branch=other", source); err != nil {
		t.Fatalf("git init source: %v", err)
	}
	target := filepath.Join(root, config.ProjectsRoot, "alpha")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := gitx.Run(t.Context(), "", true, "git", "clone", source, target); err != nil {
		t.Fatalf("git clone: %v", err)
	}
	registry := &config.Root{Projects: map[string]config.Project{
		"alpha": {Kind: config.KindExternal, Repo: source, DefaultBranch: "main"},
	}}

	if _, err := EnsureCheckout(t.Context(), registry, root, "alpha"); err == nil || !strings.Contains(err.Error(), "branch mismatch") {
		t.Fatalf("EnsureCheckout error = %v, want branch mismatch", err)
	}
}

func TestEnsureCheckoutDoesNotLeavePartialTargetAfterCloneFailure(t *testing.T) {
	root := t.TempDir()
	registry := &config.Root{Projects: map[string]config.Project{
		"alpha": {Kind: config.KindExternal, Repo: filepath.Join(t.TempDir(), "missing"), DefaultBranch: "main"},
	}}

	if _, err := EnsureCheckout(t.Context(), registry, root, "alpha"); err == nil {
		t.Fatal("EnsureCheckout unexpectedly cloned a missing repository")
	}
	target := filepath.Join(root, config.ProjectsRoot, "alpha")
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("failed clone left stable target behind: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(root, config.ProjectsRoot, ".checkout-next-alpha-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("failed clone left staging paths: %v, %v", matches, err)
	}
}
