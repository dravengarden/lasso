package modules

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// AddResult is the outcome of installing one module.
type AddResult struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Path    string `json:"path"`
	State   string `json:"state"` // added | already
}

// RemoveResult is the outcome of removing one module.
type RemoveResult struct {
	ID    string `json:"id"`
	State string `json:"state"` // removed | absent
}

// Add copies a module from the kit into the workspace and refreshes lock + marketplace.
func Add(kitRoot, workspace, id string) (*AddResult, error) {
	cat, err := LoadCatalog(kitRoot)
	if err != nil {
		return nil, err
	}
	mod, err := cat.Get(id)
	if err != nil {
		return nil, err
	}
	for _, req := range mod.Requires {
		lock, err := LoadLock(workspace)
		if err != nil {
			return nil, err
		}
		if _, ok := lock.Modules[req]; !ok && req != "core" {
			return nil, fmt.Errorf("module %q requires %q; install it first", id, req)
		}
	}

	src := filepath.Join(kitRoot, mod.Path)
	if info, err := os.Stat(src); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("module source missing: %s", src)
	}
	dst := InstalledPath(workspace, id)
	if info, err := os.Stat(dst); err == nil && info.IsDir() {
		lock, err := LoadLock(workspace)
		if err != nil {
			return nil, err
		}
		if lock.Modules[id] == mod.Version {
			return &AddResult{ID: id, Version: mod.Version, Path: dst, State: "already"}, nil
		}
		if err := os.RemoveAll(dst); err != nil {
			return nil, err
		}
	}

	if err := copyDir(src, dst); err != nil {
		return nil, fmt.Errorf("install module %q: %w", id, err)
	}
	// Drop kit-only metadata that should not be treated as workspace content.
	_ = os.Remove(filepath.Join(dst, "module.toml"))

	lock, err := LoadLock(workspace)
	if err != nil {
		return nil, err
	}
	lock.Modules[id] = mod.Version
	if err := SaveLock(workspace, lock); err != nil {
		return nil, err
	}
	if err := refreshMarketplaces(kitRoot, workspace, lock); err != nil {
		return nil, err
	}
	if err := mergeConventions(workspace, id, dst); err != nil {
		return nil, err
	}
	return &AddResult{ID: id, Version: mod.Version, Path: dst, State: "added"}, nil
}

// Remove uninstalls a module from the workspace.
func Remove(kitRoot, workspace, id string) (*RemoveResult, error) {
	if id == "core" {
		return nil, fmt.Errorf("cannot remove core; it is part of workspace init")
	}
	lock, err := LoadLock(workspace)
	if err != nil {
		return nil, err
	}
	// Refuse removal when another installed module requires this id.
	cat, err := LoadCatalog(kitRoot)
	if err != nil {
		return nil, err
	}
	for otherID := range lock.Modules {
		if otherID == id {
			continue
		}
		other, err := cat.Get(otherID)
		if err != nil {
			continue
		}
		for _, req := range other.Requires {
			if req == id {
				return nil, fmt.Errorf("module %q is required by installed module %q", id, otherID)
			}
		}
	}

	dst := InstalledPath(workspace, id)
	if _, err := os.Stat(dst); err != nil {
		if os.IsNotExist(err) {
			delete(lock.Modules, id)
			_ = SaveLock(workspace, lock)
			_ = refreshMarketplaces(kitRoot, workspace, lock)
			return &RemoveResult{ID: id, State: "absent"}, nil
		}
		return nil, err
	}
	if err := os.RemoveAll(dst); err != nil {
		return nil, err
	}
	// Drop materialized plugin tree when present.
	_ = os.RemoveAll(filepath.Join(workspace, "plugins", "lasso-"+id))
	// Best-effort: remove convention files that this module exclusively owned.
	_ = removeModuleConventions(workspace, id)

	delete(lock.Modules, id)
	if err := SaveLock(workspace, lock); err != nil {
		return nil, err
	}
	if err := refreshMarketplaces(kitRoot, workspace, lock); err != nil {
		return nil, err
	}
	return &RemoveResult{ID: id, State: "removed"}, nil
}

// ListInstalled returns sorted installed module ids from the lockfile.
func ListInstalled(workspace string) ([]string, error) {
	lock, err := LoadLock(workspace)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(lock.Modules))
	for id := range lock.Modules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func mergeConventions(workspace, id, moduleDir string) error {
	src := filepath.Join(moduleDir, "conventions")
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	dst := filepath.Join(workspace, "conventions")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		// Namespace language packs under conventions/code/<id>/ when nested.
		return copyFile(path, target)
	})
}

func removeModuleConventions(workspace, id string) error {
	// Convention modules ship under conventions/; remove known language pack paths.
	candidates := []string{
		filepath.Join(workspace, "conventions", "code", strings.TrimPrefix(id, "lang-")),
		filepath.Join(workspace, "conventions", id+".md"),
	}
	for _, p := range candidates {
		_ = os.RemoveAll(p)
	}
	return nil
}

type marketplace struct {
	Name      string              `json:"name"`
	Interface map[string]any      `json:"interface,omitempty"`
	Plugins   []marketplacePlugin `json:"plugins"`
}

type marketplacePlugin struct {
	Name     string         `json:"name"`
	Source   map[string]any `json:"source"`
	Policy   map[string]any `json:"policy,omitempty"`
	Category string         `json:"category,omitempty"`
}

func refreshMarketplaces(kitRoot, workspace string, lock *Lockfile) error {
	existingCodex := readMarketplace(filepath.Join(workspace, MarketplaceCodex))
	existingClaude := readMarketplace(filepath.Join(workspace, MarketplaceClaude))

	// Instance workspaces (e.g. Columbus) keep their marketplace identity and
	// non-Lasso plugins. Lasso only owns lasso-core plus lasso-<module> entries.
	name := "lasso"
	iface := map[string]any{"displayName": "Lasso"}
	if existingCodex != nil {
		if existingCodex.Name != "" {
			name = existingCodex.Name
		}
		if existingCodex.Interface != nil {
			iface = existingCodex.Interface
		}
	}

	plugins := preservedForeignPlugins(existingCodex, existingClaude)

	// Prefer an instance harness plugin over vendoring a second skill tree.
	instanceHarness := hasInstanceHarnessPlugin(workspace)
	if !instanceHarness {
		if err := ensureCorePlugin(kitRoot, workspace); err != nil {
			return err
		}
		plugins = append(plugins, marketplacePlugin{
			Name: "lasso-core",
			Source: map[string]any{
				"source": "local",
				"path":   "./plugins/lasso-core",
			},
			Policy: map[string]any{
				"installation":   "AVAILABLE",
				"authentication": "ON_INSTALL",
			},
			Category: "Developer Tools",
		})
	}

	ids := make([]string, 0, len(lock.Modules))
	for id := range lock.Modules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		pluginDir := filepath.Join(InstalledPath(workspace, id), "plugin")
		if info, err := os.Stat(pluginDir); err == nil && info.IsDir() {
			dest := filepath.Join(workspace, "plugins", "lasso-"+id)
			_ = os.RemoveAll(dest)
			if err := copyDir(pluginDir, dest); err != nil {
				return fmt.Errorf("materialize plugin for %s: %w", id, err)
			}
			_ = ensurePluginManifests(dest, "lasso-"+id, lock.Modules[id])
			plugins = append(plugins, marketplacePlugin{
				Name: "lasso-" + id,
				Source: map[string]any{
					"source": "local",
					"path":   "./plugins/lasso-" + id,
				},
				Policy: map[string]any{
					"installation":   "AVAILABLE",
					"authentication": "ON_INSTALL",
				},
				Category: "Developer Tools",
			})
		}
	}

	codex := marketplace{Name: name, Interface: iface, Plugins: plugins}
	claude := marketplace{Name: name, Plugins: plugins}
	if existingClaude != nil && existingClaude.Interface != nil {
		claude.Interface = existingClaude.Interface
	}
	if err := writeJSON(filepath.Join(workspace, MarketplaceCodex), codex); err != nil {
		return err
	}
	return writeJSON(filepath.Join(workspace, MarketplaceClaude), claude)
}

func readMarketplace(path string) *marketplace {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var market marketplace
	if err := json.Unmarshal(data, &market); err != nil {
		return nil
	}
	return &market
}

func isLassoManagedPlugin(name string) bool {
	return name == "lasso-core" || strings.HasPrefix(name, "lasso-")
}

func preservedForeignPlugins(markets ...*marketplace) []marketplacePlugin {
	seen := map[string]bool{}
	var out []marketplacePlugin
	for _, market := range markets {
		if market == nil {
			continue
		}
		for _, plugin := range market.Plugins {
			if isLassoManagedPlugin(plugin.Name) || seen[plugin.Name] {
				continue
			}
			seen[plugin.Name] = true
			out = append(out, plugin)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func hasInstanceHarnessPlugin(workspace string) bool {
	// Columbus and similar instances already ship a harness plugin; avoid a
	// duplicate skill tree from lasso-core until they retire the fork.
	candidates := []string{
		filepath.Join(workspace, "plugins", "columbus-harness"),
		filepath.Join(workspace, "plugins", "harness-core"),
	}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

func ensureCorePlugin(kitRoot, workspace string) error {
	src := filepath.Join(kitRoot, "plugins", "lasso-core")
	dst := filepath.Join(workspace, "plugins", "lasso-core")
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("kit missing plugins/lasso-core: %w", err)
	}
	_ = os.RemoveAll(dst)
	return copyDir(src, dst)
}

func ensurePluginManifests(pluginDir, name, version string) error {
	codexPath := filepath.Join(pluginDir, ".codex-plugin", "plugin.json")
	claudePath := filepath.Join(pluginDir, ".claude-plugin", "plugin.json")
	// Preserve author-supplied manifests (e.g. hooks modules declare hooks/).
	if _, err := os.Stat(codexPath); err == nil {
		if _, err := os.Stat(claudePath); err == nil {
			return nil
		}
	}
	_ = os.MkdirAll(filepath.Dir(codexPath), 0o755)
	_ = os.MkdirAll(filepath.Dir(claudePath), 0o755)
	manifest := map[string]any{
		"name":        name,
		"version":     version,
		"description": "Lasso module plugin " + name,
		"skills":      "./skills/",
	}
	if err := writeJSON(codexPath, manifest); err != nil {
		return err
	}
	return writeJSON(claudePath, manifest)
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
