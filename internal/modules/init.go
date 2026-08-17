package modules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// InitOptions configures a new workspace.
type InitOptions struct {
	Name     string
	Path     string
	Runtimes []string // codex, claude, grok
	Modules  []string // module ids to install after scaffold
}

// InitResult summarizes workspace creation.
type InitResult struct {
	Path             string   `json:"path"`
	Name             string   `json:"name"`
	Runtimes         []string `json:"runtimes"`
	Modules          []string `json:"modules"`
	InstalledModules []string `json:"installed_modules"`
}

// Init scaffolds a new agent-native monorepo workspace from the kit template.
func Init(kitRoot string, opts InitOptions) (*InitResult, error) {
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		return nil, fmt.Errorf("workspace name is required")
	}
	path := strings.TrimSpace(opts.Path)
	if path == "" {
		path = name
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(abs); err == nil {
		entries, _ := os.ReadDir(abs)
		if info.IsDir() && len(entries) > 0 {
			// Allow re-init only when already a lasso workspace marker exists? Prefer fail.
			if _, err := os.Stat(filepath.Join(abs, WorkspaceFile)); err == nil {
				return nil, fmt.Errorf("workspace already exists at %s", abs)
			}
			return nil, fmt.Errorf("target directory is not empty: %s", abs)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	runtimes := opts.Runtimes
	if len(runtimes) == 0 {
		runtimes = []string{"codex"}
	}
	for _, r := range runtimes {
		switch r {
		case "codex", "claude", "grok":
		default:
			return nil, fmt.Errorf("unsupported runtime %q (codex|claude|grok)", r)
		}
	}

	tmplRoot := filepath.Join(kitRoot, "templates", "workspace")
	if info, err := os.Stat(tmplRoot); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("kit missing templates/workspace")
	}

	data := map[string]any{
		"Name":      name,
		"Runtimes":  runtimes,
		"HasClaude": contains(runtimes, "claude") || contains(runtimes, "grok"),
		"HasGrok":   contains(runtimes, "grok"),
	}
	if err := renderTree(tmplRoot, abs, data); err != nil {
		return nil, fmt.Errorf("render template: %w", err)
	}

	cfg := &WorkspaceConfig{Name: name, Runtimes: runtimes}
	if err := SaveWorkspaceConfig(abs, cfg); err != nil {
		return nil, err
	}
	lock := &Lockfile{Core: "0.1.0", Modules: map[string]string{}}
	if err := SaveLock(abs, lock); err != nil {
		return nil, err
	}
	if err := ensureCorePlugin(kitRoot, abs); err != nil {
		return nil, err
	}
	if err := refreshMarketplaces(kitRoot, abs, lock); err != nil {
		return nil, err
	}

	// Default modules from catalog + user request.
	cat, err := LoadCatalog(kitRoot)
	if err != nil {
		return nil, err
	}
	want := map[string]bool{}
	for _, m := range cat.Modules {
		if m.Default {
			want[m.ID] = true
		}
	}
	for _, id := range opts.Modules {
		want[id] = true
	}
	installed := []string{}
	// Install in catalog order to satisfy simple requires.
	for _, m := range cat.Modules {
		if !want[m.ID] {
			continue
		}
		res, err := Add(kitRoot, abs, m.ID)
		if err != nil {
			return nil, fmt.Errorf("install default/requested module %q: %w", m.ID, err)
		}
		installed = append(installed, res.ID)
	}

	return &InitResult{
		Path:             abs,
		Name:             name,
		Runtimes:         runtimes,
		Modules:          opts.Modules,
		InstalledModules: installed,
	}, nil
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func renderTree(src, dst string, data map[string]any) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		// Template files use .tmpl suffix; strip on write.
		outRel := rel
		isTmpl := false
		if strings.HasSuffix(rel, ".tmpl") {
			outRel = strings.TrimSuffix(rel, ".tmpl")
			isTmpl = true
		}
		target := filepath.Join(dst, outRel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if isTmpl {
			t, err := template.New(filepath.Base(path)).Parse(string(body))
			if err != nil {
				return fmt.Errorf("parse template %s: %w", rel, err)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.Create(target)
			if err != nil {
				return err
			}
			defer f.Close()
			return t.Execute(f, data)
		}
		return copyFile(path, target)
	})
}
