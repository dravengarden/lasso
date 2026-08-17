package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dravengarden/lasso/internal/config"
)

func TestInferCurrentProjectFromRegisteredCheckout(t *testing.T) {
	root := contextRegistry(t)
	checkout := filepath.Join(root, "projects", "books")
	initGitRepo(t, checkout, "git@github.com:dravengarden/books.git")
	t.Chdir(checkout)
	registry, err := config.LoadFromRoot(root)
	if err != nil {
		t.Fatalf("LoadFromRoot: %v", err)
	}
	got, err := inferCurrentProject(context.Background(), registry, root)
	if err != nil || got != "books" {
		t.Fatalf("inferCurrentProject = (%q, %v), want books", got, err)
	}
}

func TestInferCurrentProjectFromIndependentCheckoutRemote(t *testing.T) {
	root := contextRegistry(t)
	checkout := filepath.Join(t.TempDir(), "books-clone")
	initGitRepo(t, checkout, "git@github.com:dravengarden/books.git")
	t.Chdir(checkout)
	registry, err := config.LoadFromRoot(root)
	if err != nil {
		t.Fatalf("LoadFromRoot: %v", err)
	}
	got, err := inferCurrentProject(context.Background(), registry, root)
	if err != nil || got != "books" {
		t.Fatalf("inferCurrentProject = (%q, %v), want books", got, err)
	}
}

func contextRegistry(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "project-defs", "registry.toml"), "version = 1\n")
	writeTestFile(t, filepath.Join(root, "project-defs", "books", "project.toml"),
		"kind = \"external\"\nrepo = \"git@github.com:dravengarden/books.git\"\ndefault_branch = \"main\"\n")
	return root
}

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func initGitRepo(t *testing.T, path, remote string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	for _, args := range [][]string{{"init"}, {"remote", "add", "origin", remote}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = path
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
}
