package worktree

import (
	"os"
	"path/filepath"
	"strings"
)

// liveProcessPaths returns the working directories and open file paths of the
// processes this user can inspect through /proc. Platforms without /proc
// return an empty set; GC then relies on its other guards.
func liveProcessPaths() []string {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var paths []string
	for _, entry := range entries {
		pid := entry.Name()
		if strings.Trim(pid, "0123456789") != "" {
			continue
		}
		base := filepath.Join("/proc", pid)
		if target, err := os.Readlink(filepath.Join(base, "cwd")); err == nil {
			paths = append(paths, target)
		}
		fds, err := os.ReadDir(filepath.Join(base, "fd"))
		if err != nil {
			continue
		}
		for _, fd := range fds {
			if target, err := os.Readlink(filepath.Join(base, "fd", fd.Name())); err == nil &&
				strings.HasPrefix(target, "/") {
				paths = append(paths, strings.TrimSuffix(target, " (deleted)"))
			}
		}
	}
	return paths
}

// inUse reports whether any live path lies inside the worktree, comparing
// both the registered and the symlink-resolved worktree path.
func inUse(live []string, path string) bool {
	roots := []string{filepath.Clean(path)}
	if resolved, err := filepath.EvalSymlinks(path); err == nil && resolved != roots[0] {
		roots = append(roots, resolved)
	}
	for _, candidate := range live {
		for _, root := range roots {
			if candidate == root || strings.HasPrefix(candidate, root+string(filepath.Separator)) {
				return true
			}
		}
	}
	return false
}
