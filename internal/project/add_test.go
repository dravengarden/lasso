package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dravengarden/lasso/internal/config"
)

func TestAddWritesOnlyRegistryEntry(t *testing.T) {
	root := t.TempDir()
	spec := Spec{
		Name:          "alpha",
		Repo:          "git@github.com:org/alpha.git",
		DefaultBranch: "main",
	}

	res, err := Add(root, spec)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !res.RegistryUpdated {
		t.Fatal("registry was not updated")
	}
	if _, err := os.Stat(config.ProjectFilePath(root, spec.Name)); err != nil {
		t.Fatalf("registry entry: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs")); !os.IsNotExist(err) {
		t.Fatalf("Add unexpectedly touched docs: %v", err)
	}
}
