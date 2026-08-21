package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dravengarden/lasso/internal/config"
)

func TestWorkspaceRootAcceptsLegacyColumbusEnv(t *testing.T) {
	t.Setenv(config.RootEnv, "")
	t.Setenv(config.LegacyRootEnv, "")
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "project-defs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "project-defs", "registry.toml"), []byte("version = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.LegacyRootEnv, dir)
	root, err := config.WorkspaceRoot()
	if err != nil {
		t.Fatal(err)
	}
	got, err := filepath.EvalSymlinks(root)
	if err != nil {
		got = root
	}
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		want = dir
	}
	if got != want {
		t.Fatalf("WorkspaceRoot()=%q want %q", got, want)
	}
}
