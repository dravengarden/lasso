// Package modules installs optional capability packs into a Lasso workspace.
package modules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	CatalogRel        = "modules/catalog.toml"
	InstalledDir      = ".lasso/modules"
	LockFile          = "lasso.lock.toml"
	WorkspaceFile     = "lasso.toml"
	MarketplaceCodex  = ".agents/plugins/marketplace.json"
	MarketplaceClaude = ".claude-plugin/marketplace.json"
)

// Catalog is the kit-side module index.
type Catalog struct {
	Modules []Module `toml:"module"`
}

// Module describes one installable capability pack.
type Module struct {
	ID          string   `toml:"id" json:"id"`
	Version     string   `toml:"version" json:"version"`
	Kind        string   `toml:"kind" json:"kind"` // plugin | convention | capability
	Description string   `toml:"description" json:"description"`
	Path        string   `toml:"path" json:"path"`
	Skills      []string `toml:"skills,omitempty" json:"skills,omitempty"`
	Requires    []string `toml:"requires,omitempty" json:"requires,omitempty"`
	Default     bool     `toml:"default,omitempty" json:"default,omitempty"`
}

// Lockfile pins installed modules in a workspace.
type Lockfile struct {
	Core    string            `toml:"core"`
	Modules map[string]string `toml:"modules"`
}

// WorkspaceConfig is the small workspace-level product config.
type WorkspaceConfig struct {
	Name     string   `toml:"name"`
	Runtimes []string `toml:"runtimes"`
}

// LoadCatalog reads modules/catalog.toml from a kit root.
func LoadCatalog(kitRoot string) (*Catalog, error) {
	path := filepath.Join(kitRoot, CatalogRel)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read catalog: %w", err)
	}
	var cat Catalog
	if err := toml.Unmarshal(data, &cat); err != nil {
		return nil, fmt.Errorf("decode catalog: %w", err)
	}
	if len(cat.Modules) == 0 {
		return nil, fmt.Errorf("catalog %s has no modules", path)
	}
	seen := map[string]bool{}
	for _, m := range cat.Modules {
		if strings.TrimSpace(m.ID) == "" {
			return nil, fmt.Errorf("catalog entry missing id")
		}
		if seen[m.ID] {
			return nil, fmt.Errorf("duplicate module id %q", m.ID)
		}
		seen[m.ID] = true
		if strings.TrimSpace(m.Path) == "" {
			return nil, fmt.Errorf("module %q missing path", m.ID)
		}
	}
	return &cat, nil
}

// Get returns a module by id.
func (c *Catalog) Get(id string) (Module, error) {
	for _, m := range c.Modules {
		if m.ID == id {
			return m, nil
		}
	}
	return Module{}, fmt.Errorf("unknown module %q", id)
}

// LoadLock reads lasso.lock.toml from a workspace (empty if missing).
func LoadLock(workspace string) (*Lockfile, error) {
	path := filepath.Join(workspace, LockFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Lockfile{Core: "0.1.0", Modules: map[string]string{}}, nil
		}
		return nil, err
	}
	var lock Lockfile
	if err := toml.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("decode lockfile: %w", err)
	}
	if lock.Modules == nil {
		lock.Modules = map[string]string{}
	}
	if lock.Core == "" {
		lock.Core = "0.1.0"
	}
	return &lock, nil
}

// SaveLock writes lasso.lock.toml.
func SaveLock(workspace string, lock *Lockfile) error {
	if lock.Modules == nil {
		lock.Modules = map[string]string{}
	}
	data, err := toml.Marshal(lock)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(workspace, LockFile), data, 0o644)
}

// LoadWorkspaceConfig reads lasso.toml when present.
func LoadWorkspaceConfig(workspace string) (*WorkspaceConfig, error) {
	path := filepath.Join(workspace, WorkspaceFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &WorkspaceConfig{Runtimes: []string{"codex"}}, nil
		}
		return nil, err
	}
	var cfg WorkspaceConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("decode %s: %w", WorkspaceFile, err)
	}
	if len(cfg.Runtimes) == 0 {
		cfg.Runtimes = []string{"codex"}
	}
	return &cfg, nil
}

// SaveWorkspaceConfig writes lasso.toml.
func SaveWorkspaceConfig(workspace string, cfg *WorkspaceConfig) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(workspace, WorkspaceFile), data, 0o644)
}

// InstalledPath is where a module lives inside the workspace after add.
func InstalledPath(workspace, id string) string {
	return filepath.Join(workspace, InstalledDir, id)
}
