package cli

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dravengarden/lasso/internal/config"
)

// A task or session worktree of the workspace carries the registry but not the
// external clones, which live only under the main worktree's projects/.
func TestProjectCheckoutPathFallsBackToMainWorktree(t *testing.T) {
	main := contextRegistry(t)
	writeTestFile(t, filepath.Join(main, "project-defs", "notes", "project.toml"), "kind = \"subdir\"\n")
	writeTestFile(t, filepath.Join(main, ".gitignore"), "/projects/books/\n")
	writeTestFile(t, filepath.Join(main, "projects", "notes", "README.md"), "notes\n")
	runGit(t, main, "init", "-q", "-b", "main")
	runGit(t, main, "add", "-A")
	runGit(t, main, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "init")
	books := filepath.Join(main, "projects", "books")
	initGitRepo(t, books, "git@github.com:dravengarden/books.git")

	linked := filepath.Join(t.TempDir(), "task")
	runGit(t, main, "worktree", "add", "-q", "-b", "task", linked)
	registry, err := config.LoadFromRoot(linked)
	if err != nil {
		t.Fatalf("LoadFromRoot: %v", err)
	}

	got, err := projectCheckoutPath(context.Background(), registry, linked, "books")
	if err != nil || canonicalPath(got) != canonicalPath(books) {
		t.Fatalf("external = (%q, %v), want main worktree clone %q", got, err, books)
	}
	got, err = projectCheckoutPath(context.Background(), registry, linked, "notes")
	want := filepath.Join(linked, "projects", "notes")
	if err != nil || canonicalPath(got) != canonicalPath(want) {
		t.Fatalf("subdir = (%q, %v), want the linked worktree's own copy %q", got, err, want)
	}
	if got, err := projectCheckoutPath(context.Background(), registry, main, "books"); err != nil ||
		canonicalPath(got) != canonicalPath(books) {
		t.Fatalf("main worktree = (%q, %v), want %q", got, err, books)
	}
}

func TestProjectCheckoutPathReportsMissingClone(t *testing.T) {
	root := contextRegistry(t)
	runGit(t, root, "init", "-q", "-b", "main")
	registry, err := config.LoadFromRoot(root)
	if err != nil {
		t.Fatalf("LoadFromRoot: %v", err)
	}
	_, err = projectCheckoutPath(context.Background(), registry, root, "books")
	if err == nil || !strings.Contains(err.Error(), "project checkout not found") {
		t.Fatalf("err = %v, want project checkout not found", err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}
