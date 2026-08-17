package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/dravengarden/lasso/internal/config"
	"github.com/dravengarden/lasso/internal/gitx"
)

func inferCurrentProject(ctx context.Context, registry *config.Root, root string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	cwd = canonicalPath(cwd)

	var pathMatches []string
	for _, name := range registry.Names() {
		project := registry.Projects[name]
		projectDir := canonicalPath(registry.ProjectDir(root, name))
		if !project.IsExternal() {
			if pathWithin(projectDir, cwd) {
				pathMatches = append(pathMatches, name)
			}
			continue
		}
		if pathWithin(projectDir, cwd) {
			pathMatches = append(pathMatches, name)
		}
	}
	pathMatches = uniqueSorted(pathMatches)
	if len(pathMatches) == 1 {
		return pathMatches[0], nil
	}
	if len(pathMatches) > 1 {
		return "", fmt.Errorf("current checkout matches multiple projects: %v", pathMatches)
	}

	toplevel, topErr := gitx.Toplevel(ctx, cwd)
	if topErr == nil {
		remote, remoteErr := gitx.Out(ctx, toplevel, "git", "config", "--get", "remote.origin.url")
		if remoteErr == nil && remote != "" {
			var remoteMatches []string
			for name, project := range registry.Projects {
				if project.IsExternal() && normalizeRemote(project.Repo) == normalizeRemote(remote) {
					remoteMatches = append(remoteMatches, name)
				}
			}
			remoteMatches = uniqueSorted(remoteMatches)
			if len(remoteMatches) == 1 {
				return remoteMatches[0], nil
			}
			if len(remoteMatches) > 1 {
				return "", fmt.Errorf("current remote matches multiple projects: %v", remoteMatches)
			}
		}
	}

	if pathWithin(canonicalPath(root), cwd) {
		return "", nil
	}
	return "", fmt.Errorf("current checkout is not a registered Lasso project; pass --project")
}

func canonicalPath(path string) string {
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return filepath.Clean(path)
}

func pathWithin(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func normalizeRemote(remote string) string {
	return strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(remote), "/"), ".git")
}

func uniqueSorted(values []string) []string {
	slices.Sort(values)
	return slices.Compact(values)
}
