package project

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestScanActiveWorkItemsForProjectFailsClosedOnCorruptMetadata(t *testing.T) {
	root := t.TempDir()
	brokenDir := filepath.Join(root, "work-items", "active", "broken")
	if err := os.MkdirAll(brokenDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(brokenDir, "item.yaml"), []byte("version: 1\nid: broken\n"), 0o600); err != nil {
		t.Fatalf("write work item: %v", err)
	}

	if _, err := scanActiveWorkItemsForProject(root, "heimdall"); err == nil {
		t.Fatal("project removal safety scan should fail on corrupt work-item metadata")
	}
}

func TestRemoveRejectsPathLikeProjectName(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "outside")
	if err := os.MkdirAll(target, 0o750); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	marker := filepath.Join(target, "keep")
	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	if _, err := Remove(root, "../outside", RemoveOptions{Force: true}); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("Remove error = %v, want ErrInvalidName", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("outside marker was touched: %v", err)
	}
}

func TestRemoveLeavesProjectDocsAlone(t *testing.T) {
	root := t.TempDir()
	defDir := filepath.Join(root, "project-defs", "alpha")
	if err := os.MkdirAll(defDir, 0o750); err != nil {
		t.Fatalf("mkdir registry: %v", err)
	}
	if err := os.WriteFile(filepath.Join(defDir, "project.toml"), []byte("kind = \"subdir\"\n"), 0o600); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	docsDir := filepath.Join(root, "docs", "projects", "alpha")
	if err := os.MkdirAll(docsDir, 0o750); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "README.md"), []byte("# alpha\n"), 0o600); err != nil {
		t.Fatalf("write docs: %v", err)
	}

	if _, err := Remove(root, "alpha", RemoveOptions{}); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(defDir); !os.IsNotExist(err) {
		t.Fatalf("registry still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(docsDir, "README.md")); err != nil {
		t.Fatalf("project docs were touched: %v", err)
	}
}
