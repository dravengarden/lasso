package cli

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/dravengarden/lasso/internal/config"
	"github.com/dravengarden/lasso/internal/gitx"
	"github.com/dravengarden/lasso/internal/ui"
)

const fallbackDefaultBranch = "main"

// EnsureCheckoutResult describes the stable Local checkout for one project.
type EnsureCheckoutResult struct {
	Project        string
	Branch         string
	Path           string
	Cloned         bool
	AlreadyExisted bool
}

// EnsureCheckout creates one ordinary clone at projects/<name>. Additional
// worktrees are deliberately outside the Lasso contract and are created and
// managed by Codex.
func EnsureCheckout(ctx context.Context, registry *config.Root, root, projectName string) (EnsureCheckoutResult, error) {
	res := EnsureCheckoutResult{Project: projectName}
	project, err := registry.MustGet(projectName)
	if err != nil {
		return res, fmt.Errorf("lookup %s: %w", projectName, err)
	}
	if !project.IsExternal() {
		return res, fmt.Errorf("%s is not an external project", projectName)
	}
	if project.Repo == "" {
		return res, fmt.Errorf("%w for %s", ErrNoRepoURL, projectName)
	}

	branch := project.DefaultBranch
	if branch == "" {
		branch = fallbackDefaultBranch
	}
	res.Branch = branch

	projectDir := registry.ProjectDir(root, projectName)
	target := projectDir
	absTarget, absErr := filepath.Abs(target)
	if absErr == nil {
		res.Path = absTarget
	} else {
		res.Path = target
	}

	if isNormalCheckout(target) {
		if err := validateExistingCheckout(ctx, target, filepath.Dir(projectDir), project.Repo, branch); err != nil {
			return res, err
		}
		res.AlreadyExisted = true
		return res, nil
	}
	if _, statErr := os.Stat(target); statErr == nil {
		return res, fmt.Errorf("checkout path exists but is not a normal Git clone: %s", target)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return res, fmt.Errorf("inspect checkout %s: %w", target, statErr)
	}

	projectsDir := filepath.Dir(projectDir)
	if err := os.MkdirAll(projectsDir, 0o750); err != nil {
		return res, fmt.Errorf("mkdir %s: %w", projectsDir, err)
	}
	staging, err := os.MkdirTemp(projectsDir, ".checkout-next-"+projectName+"-")
	if err != nil {
		return res, fmt.Errorf("create checkout staging directory in %s: %w", projectsDir, err)
	}
	if err := os.Remove(staging); err != nil {
		return res, fmt.Errorf("prepare checkout staging path %s: %w", staging, err)
	}
	defer os.RemoveAll(staging) //nolint:errcheck // best-effort cleanup of a path created by this call

	ui.Dimf("cloning %s into %s...", projectName, target)
	if err := gitx.Run(ctx, projectsDir, false, "git", "clone", "--branch", branch, project.Repo, staging); err != nil {
		return res, fmt.Errorf("git clone %s: %w", projectName, err)
	}
	if !isNormalCheckout(staging) {
		return res, fmt.Errorf("clone did not create a normal Git checkout: %s", staging)
	}
	if err := os.Rename(staging, target); err != nil {
		return res, fmt.Errorf("publish checkout %s: %w", target, err)
	}
	res.Cloned = true
	return res, nil
}

func validateExistingCheckout(ctx context.Context, checkout, projectDir, repo, branch string) error {
	origin, err := gitx.Out(ctx, checkout, "git", "remote", "get-url", "origin")
	if err != nil {
		return fmt.Errorf("inspect checkout origin %s: %w", checkout, err)
	}
	want, err := normalizeRepoIdentity(projectDir, repo)
	if err != nil {
		return fmt.Errorf("normalize registered repository %q: %w", repo, err)
	}
	got, err := normalizeRepoIdentity(checkout, strings.TrimSpace(origin))
	if err != nil {
		return fmt.Errorf("normalize checkout origin %q: %w", strings.TrimSpace(origin), err)
	}
	if got != want {
		return fmt.Errorf("checkout origin mismatch at %s: registered %q, found %q", checkout, repo, strings.TrimSpace(origin))
	}

	current, err := gitx.Out(ctx, checkout, "git", "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || strings.TrimSpace(current) != branch {
		return fmt.Errorf("checkout branch mismatch at %s: expected %q", checkout, branch)
	}
	return nil
}

func normalizeRepoIdentity(base, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrNoRepoURL
	}
	if parsed, err := url.Parse(raw); err == nil && parsed.Scheme != "" {
		if parsed.Scheme == "file" {
			return normalizeLocalRepo(base, parsed.Path)
		}
		if parsed.Hostname() != "" {
			return strings.ToLower(parsed.Host) + "/" + normalizeRepoPath(parsed.Path), nil
		}
	}
	if colon := strings.IndexByte(raw, ':'); colon > 0 && !strings.Contains(raw[:colon], "/") {
		host := raw[:colon]
		if at := strings.LastIndexByte(host, '@'); at >= 0 {
			host = host[at+1:]
		}
		return strings.ToLower(host) + "/" + normalizeRepoPath(raw[colon+1:]), nil
	}
	return normalizeLocalRepo(base, raw)
}

func normalizeLocalRepo(base, path string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func normalizeRepoPath(path string) string {
	return strings.TrimSuffix(strings.Trim(strings.TrimSpace(path), "/"), ".git")
}

func isNormalCheckout(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil && info.IsDir()
}
