package modules_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestReleaseModulesMaterializeIndependentPlugins(t *testing.T) {
	kitRoot := findKitRoot(t)
	ws := newWorkspace(t)

	expected := map[string][]string{
		"npm-release":   {"release-npm-package", "maintain-npm-package-page"},
		"pypi-release":  {"release-pypi-package", "maintain-pypi-package-page"},
		"cargo-release": {"release-cargo-package", "maintain-crates-io-package-page"},
	}
	for id, skills := range expected {
		if _, err := modules.Add(kitRoot, ws, id); err != nil {
			t.Fatalf("add %s: %v", id, err)
		}
		plugin := filepath.Join(ws, "plugins", "lasso-"+id)
		for _, skill := range skills {
			if _, err := os.Stat(filepath.Join(plugin, "skills", skill, "SKILL.md")); err != nil {
				t.Fatalf("%s missing skill %s: %v", id, skill, err)
			}
		}
	}

	data, err := os.ReadFile(filepath.Join(ws, modules.MarketplaceCodex))
	if err != nil {
		t.Fatal(err)
	}
	var market struct {
		Plugins []struct {
			Name string `json:"name"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(data, &market); err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(market.Plugins))
	for _, plugin := range market.Plugins {
		names = append(names, plugin.Name)
	}
	joined := strings.Join(names, " ")
	for id := range expected {
		if !strings.Contains(joined, "lasso-"+id) {
			t.Fatalf("marketplace missing lasso-%s: %s", id, joined)
		}
	}

	if _, err := modules.Remove(kitRoot, ws, "npm-release"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ws, "plugins", "lasso-npm-release")); !os.IsNotExist(err) {
		t.Fatalf("npm plugin remains after module removal: %v", err)
	}
}

func findKitRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "modules", "catalog.toml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("lasso kit root not found")
		}
		dir = parent
	}
}

func newWorkspace(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "project-defs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "project-defs", "registry.toml"), []byte("version = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := modules.SaveWorkspaceConfig(ws, &modules.WorkspaceConfig{Name: "test", Runtimes: []string{"codex"}}); err != nil {
		t.Fatal(err)
	}
	if err := modules.SaveLock(ws, &modules.Lockfile{Core: "0.1.0", Modules: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	return ws
}
