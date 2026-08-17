// Package kit locates the Lasso product (kit) tree that ships modules and
// workspace templates. A workspace is a user monorepo; the kit is the product.
package kit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// KitRootEnv points at the Lasso product checkout (this repository).
	KitRootEnv = "LASSO_KIT_ROOT"
	// Marker is the file that identifies a kit root.
	Marker = "modules/catalog.toml"
)

// Root resolves the kit tree: LASSO_KIT_ROOT, then walk-up from cwd, then the
// directory containing the running binary's sibling checkout heuristics.
func Root() (string, error) {
	if configured := strings.TrimSpace(os.Getenv(KitRootEnv)); configured != "" {
		root, err := filepath.Abs(configured)
		if err != nil {
			return "", fmt.Errorf("resolve %s: %w", KitRootEnv, err)
		}
		if !isKit(root) {
			return "", fmt.Errorf("%s=%s is not a Lasso kit (missing %s)", KitRootEnv, root, Marker)
		}
		return root, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if isKit(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}

	// When the binary lives in <kit>/bin/lasso during development.
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		binDir := filepath.Dir(exe)
		candidate := filepath.Dir(binDir)
		if isKit(candidate) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("lasso kit not found (set %s or run inside the lasso product checkout)", KitRootEnv)
}

func isKit(root string) bool {
	info, err := os.Stat(filepath.Join(root, Marker))
	return err == nil && !info.IsDir()
}
