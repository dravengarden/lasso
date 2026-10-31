package worktree

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func commitFile(t *testing.T, dir, name, content, message string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", name)
	runGit(t, dir, "-c", "user.name=Lasso Test", "-c", "user.email=test@example.invalid",
		"commit", "-m", message)
}

// ageTree moves every timestamp GC treats as activity into the past.
func ageTree(t *testing.T, path string, age time.Duration) {
	t.Helper()
	old := time.Now().Add(-age)
	gitDir, err := exec.Command("git", "-C", path, "rev-parse", "--absolute-git-dir").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{path, string(gitDir[:len(gitDir)-1])} {
		if err := filepath.Walk(root, func(p string, _ os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			return os.Chtimes(p, old, old)
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSquashMergedBranchIsMerged(t *testing.T) {
	ctx := context.Background()
	f := newWorktreeFixture(t)
	path, _, err := Create(ctx, f.repo, f.wtRoot, "alpha", "squashed", CreateOptions{From: "origin/main"})
	if err != nil {
		t.Fatal(err)
	}
	commitFile(t, path, "a.txt", "one\n", "first")
	commitFile(t, path, "b.txt", "two\n", "second")
	if isMerged(ctx, f.repo, "codex/squashed", "main") {
		t.Fatal("unmerged branch reported merged")
	}
	// Upstream lands the same change as one squashed commit.
	commitFile(t, f.writer, "a.txt", "one\n", "squash part")
	commitFile(t, f.writer, "b.txt", "two\n", "squash")
	runGit(t, f.writer, "push", "origin", "main")
	runGit(t, f.repo, "fetch", "origin")
	if !isMerged(ctx, f.repo, "codex/squashed", "main") {
		t.Fatal("squash-merged branch reported unmerged")
	}
	// A branch with a change upstream lacks stays unmerged.
	commitFile(t, path, "c.txt", "three\n", "third")
	if isMerged(ctx, f.repo, "codex/squashed", "main") {
		t.Fatal("branch with an unlanded change reported merged")
	}
}

func TestPruneRegenerableAcceptsOnlyCacheDirectories(t *testing.T) {
	ctx := context.Background()
	f := newWorktreeFixture(t)
	if err := os.WriteFile(filepath.Join(f.repo, ".git", "info", "exclude"),
		[]byte("node_modules/\ntarget/\nprivate-data\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	caches, _, err := Create(ctx, f.repo, f.wtRoot, "alpha", "caches", CreateOptions{From: "origin/main"})
	if err != nil {
		t.Fatal(err)
	}
	mixed, _, err := Create(ctx, f.repo, f.wtRoot, "alpha", "mixed", CreateOptions{From: "origin/main"})
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{caches, mixed} {
		for _, sub := range []string{"web/node_modules/pkg", "target/debug"} {
			if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, sub, "f"), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(mixed, "private-data"), []byte("irreplaceable"), 0o600); err != nil {
		t.Fatal(err)
	}
	plain, err := PlanGC(ctx, f.wtRoot, f.repos(), "", GCOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := gotByEntry(plain, "alpha/caches"); got != "skip: ignored content" {
		t.Fatalf("default caches = %q", got)
	}
	done, err := RunGC(ctx, f.wtRoot, f.repos(), "", GCOptions{PruneRegenerable: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := gotByEntry(done, "alpha/caches"); got != "remove" {
		t.Fatalf("regenerable caches = %q", got)
	}
	if got := gotByEntry(done, "alpha/mixed"); got != "skip: ignored content" {
		t.Fatalf("mixed ignored content = %q", got)
	}
	if _, err := os.Stat(caches); !os.IsNotExist(err) {
		t.Fatal("cache-only worktree still present")
	}
	if _, err := os.Stat(filepath.Join(mixed, "private-data")); err != nil {
		t.Fatal("worktree with private ignored data was removed")
	}
}

func TestIdleAgeCountsFileAndGitActivity(t *testing.T) {
	ctx := context.Background()
	f := newWorktreeFixture(t)
	path, _, err := Create(ctx, f.repo, f.wtRoot, "alpha", "idle", CreateOptions{From: "origin/main"})
	if err != nil {
		t.Fatal(err)
	}
	ageTree(t, path, 10*24*time.Hour)
	if days := ageDays(path); days < 9 {
		t.Fatalf("aged worktree idle days = %d", days)
	}
	// Editing a tracked file deep in the tree is activity even though the
	// worktree root's own mtime does not move.
	now := time.Now()
	if err := os.Chtimes(filepath.Join(path, "README.md"), now, now); err != nil {
		t.Fatal(err)
	}
	if days := ageDays(path); days != 0 {
		t.Fatalf("recent edit idle days = %d", days)
	}
	actions, err := PlanGC(ctx, f.wtRoot, f.repos(), "", GCOptions{MinAge: 7 * 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if got := gotByEntry(actions, "alpha/idle"); got != "skip: too young" {
		t.Fatalf("recently edited worktree = %q", got)
	}
}

func TestGCSkipsWorktreeInUse(t *testing.T) {
	if _, err := os.Stat("/proc/self/cwd"); err != nil {
		t.Skip("no /proc")
	}
	ctx := context.Background()
	f := newWorktreeFixture(t)
	path, _, err := Create(ctx, f.repo, f.wtRoot, "alpha", "busy", CreateOptions{From: "origin/main"})
	if err != nil {
		t.Fatal(err)
	}
	holder := exec.Command("sleep", "30")
	holder.Dir = path
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Process.Kill(); _ = holder.Wait() }()
	done, err := RunGC(ctx, f.wtRoot, f.repos(), "", GCOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := gotByEntry(done, "alpha/busy"); got != "skip: in use" {
		t.Fatalf("busy worktree = %q", got)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("busy worktree was removed")
	}
}

func TestNixOutLinksAndSiteBuildsAreRegenerable(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "site-dist", "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/nix/store/0000000000000000000000000000000a-x", filepath.Join(root, "result-web")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/elsewhere", filepath.Join(root, "result")); err != nil {
		t.Fatal(err)
	}
	if !onlyRegenerable(root, "site-dist/\nresult-web\n") {
		t.Fatal("site build and nix out-link should be regenerable")
	}
	if onlyRegenerable(root, "result\n") {
		t.Fatal("a result link outside /nix/store is user content")
	}
	if onlyRegenerable(root, "dist/\n") {
		t.Fatal("dist can hold evidence and must stay protected")
	}
}
