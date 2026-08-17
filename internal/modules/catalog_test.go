package modules_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dravengarden/lasso/internal/modules"
)

func TestLoadCatalogAndAddRemove(t *testing.T) {
	kitRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	// tests run with package dir = internal/modules; kit is repo root two up
	// but go test cwd is package dir. Discover via marker walk.
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "modules", "catalog.toml")); err == nil {
			kitRoot = dir
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("not running inside lasso kit")
		}
		dir = parent
	}

	cat, err := modules.LoadCatalog(kitRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Modules) < 1 {
		t.Fatal("expected modules")
	}

	ws := t.TempDir()
	// minimal workspace markers
	if err := os.MkdirAll(filepath.Join(ws, "project-defs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "project-defs", "registry.toml"), []byte("version = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := modules.SaveWorkspaceConfig(ws, &modules.WorkspaceConfig{Name: "t", Runtimes: []string{"codex"}}); err != nil {
		t.Fatal(err)
	}
	if err := modules.SaveLock(ws, &modules.Lockfile{Core: "0.1.0", Modules: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	// core plugin required by marketplace refresh
	if err := os.MkdirAll(filepath.Join(kitRoot, "plugins", "lasso-core"), 0o755); err != nil {
		t.Fatal(err)
	}

	res, err := modules.Add(kitRoot, ws, "lang-go")
	if err != nil {
		t.Fatal(err)
	}
	if res.State != "added" {
		t.Fatalf("state=%s", res.State)
	}
	lock, err := modules.LoadLock(ws)
	if err != nil {
		t.Fatal(err)
	}
	if lock.Modules["lang-go"] == "" {
		t.Fatal("expected lang-go pin")
	}
	if _, err := modules.Remove(kitRoot, ws, "lang-go"); err != nil {
		t.Fatal(err)
	}
}
