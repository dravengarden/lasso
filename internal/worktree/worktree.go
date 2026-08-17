// Package worktree owns state-free task-worktree lifecycle helpers shared by
// every agent runtime. Native Codex/Claude worktrees remain the runtime-local
// execution surface; Lasso adds a canonical placement convention, a
// deterministic inventory, and conservative garbage collection for worktrees
// whose branch is already merged and whose checkout is clean.
//
// This package never persists task state: no registry, no ownership mapping,
// no branch-to-work-item inference. Git is the only source of truth.
package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/dravengarden/lasso/internal/gitx"
)

// RootEnv overrides the canonical worktree root. The default is
// $HOME/worktrees, matching the existing layout used by Lasso tasks.
const RootEnv = "LASSO_WORKTREE_ROOT"

// defaultRootName is the directory under $HOME used when RootEnv is unset
// and kept as an extra classification root after a host override.
const defaultRootName = "worktrees"

var (
	nameRE   = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	branchRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]{0,127}$`)
)

// Kind classifies a worktree's role for display and GC safety.
type Kind string

const (
	// KindTask is a canonical task worktree under <root>/<project>/<name>.
	KindTask Kind = "task"
	// KindStable is the ordinary stable checkout Lasso bootstraps.
	KindStable Kind = "stable"
	// KindSession is a runtime-owned session worktree (e.g. cowboy-machine).
	KindSession Kind = "session"
	// KindLegacy is any other worktree outside the canonical root.
	KindLegacy Kind = "legacy"
)

// Entry is one worktree as seen by `git worktree list --porcelain`, enriched
// with the deterministic safety facts GC needs.
type Entry struct {
	Project       string `json:"project"`
	Name          string `json:"name"`
	Kind          Kind   `json:"kind"`
	Path          string `json:"path"`
	Repo          string `json:"repo"`
	Branch        string `json:"branch"`
	Head          string `json:"head"`
	Dirty         bool   `json:"dirty"`
	Merged        bool   `json:"merged"`
	AgeDays       int    `json:"age_days"`
	DefaultBranch string `json:"default_branch"`
}

// Repo is a registered repository to scan. Project is the display owner;
// Stable is the ordinary checkout path (empty when the project lives inside
// the Lasso repository itself).
type Repo struct {
	Project       string
	Repo          string
	Stable        string
	DefaultBranch string
}

// CreateOptions controls Create.
type CreateOptions struct {
	// Branch is the local branch to create. Defaults to "codex/<name>".
	Branch string
	// From is the ref the branch is created from. Required.
	From string
	// Fetch refreshes origin before resolving From. Default: true.
	Fetch bool
}

// RemoveOptions controls Remove.
type RemoveOptions struct {
	// Force bypasses dirty and unmerged safety checks and passes --force
	// to `git worktree remove` / `git branch`.
	Force bool
	// DeleteBranch also deletes the local branch after the worktree is
	// removed. With Force=false the branch must be merged.
	DeleteBranch bool
}

// GCOptions controls PlanGC / RunGC.
type GCOptions struct {
	// MinAge is the minimum worktree age (modification time) accepted for
	// automatic removal. Zero means no age requirement; the CLI layer owns
	// the user-facing default.
	MinAge time.Duration
	// DeleteBranch passes through to Remove after a successful removal.
	DeleteBranch bool
}

// Action is one GC decision for a task worktree.
type Action struct {
	Entry  Entry  `json:"entry"`
	Action string `json:"action"` // "remove" or a skip reason
}

// Root returns the canonical worktree root used by Create.
func Root() (string, error) {
	if v := strings.TrimSpace(os.Getenv(RootEnv)); v != "" {
		return filepath.Abs(v)
	}
	return defaultHomeRoot()
}

func defaultHomeRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("worktree root: resolve home: %w", err)
	}
	return filepath.Join(home, defaultRootName), nil
}

// TaskRoots returns the create root plus the default $HOME/worktrees path
// when they differ. Create uses Root(); list, remove, and GC treat a path
// as KindTask if it lives under any of these roots so a host can move new
// worktrees onto scratch storage without orphaning existing trees.
func TaskRoots(primary string) []string {
	roots := make([]string, 0, 2)
	if cleaned := strings.TrimSpace(primary); cleaned != "" {
		if abs, err := filepath.Abs(cleaned); err == nil {
			roots = append(roots, abs)
		} else {
			roots = append(roots, cleaned)
		}
	}
	if home, err := defaultHomeRoot(); err == nil && !containsPath(roots, home) {
		roots = append(roots, home)
	}
	return roots
}

func containsPath(roots []string, candidate string) bool {
	for _, root := range roots {
		if samePath(root, candidate) {
			return true
		}
	}
	return false
}

func samePath(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return aa == bb
}

// CanonicalPath returns <root>/<project>/<name>.
func CanonicalPath(root, project, name string) string {
	return filepath.Join(root, project, name)
}

// ValidateName accepts a single lowercase worktree name segment that is safe
// as a directory name and as a branch slug.
func ValidateName(name string) error {
	if !nameRE.MatchString(name) || name == "." || name == ".." {
		return fmt.Errorf("invalid worktree name %q: use 1-64 lowercase letters, digits, '.', '_', '-'", name)
	}
	return nil
}

func validateBranch(branch string) error {
	if !branchRE.MatchString(branch) ||
		strings.HasSuffix(branch, "/") ||
		strings.Contains(branch, "//") ||
		strings.Contains(branch, "..") {
		return fmt.Errorf("invalid branch name %q", branch)
	}
	return nil
}

// Create adds a canonical task worktree at <root>/<project>/<name> and returns
// the created path and branch. It refuses to overwrite an existing path or
// branch and fetches origin first unless disabled.
func Create(ctx context.Context, repo, root, project, name string, opts CreateOptions) (string, string, error) {
	if err := ValidateName(name); err != nil {
		return "", "", err
	}
	if strings.TrimSpace(opts.From) == "" {
		return "", "", errors.New("worktree create requires a source ref (--from)")
	}
	branch := opts.Branch
	if branch == "" {
		branch = "codex/" + name
	}
	if err := validateBranch(branch); err != nil {
		return "", "", err
	}

	path := CanonicalPath(root, project, name)
	if _, err := os.Stat(path); err == nil {
		return "", "", fmt.Errorf("worktree path already exists: %s", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", "", fmt.Errorf("inspect %s: %w", path, err)
	}
	if gitx.RefExists(ctx, repo, "refs/heads/"+branch) {
		return "", "", fmt.Errorf("branch already exists: %s", branch)
	}
	if opts.Fetch {
		if err := gitx.Run(ctx, repo, true, "git", "fetch", "--prune", "origin"); err != nil {
			return "", "", fmt.Errorf("fetch origin: %w", err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", "", fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	if err := gitx.Run(ctx, repo, false, "git", "worktree", "add", "-b", branch, path, opts.From); err != nil {
		return "", "", fmt.Errorf("git worktree add: %w", err)
	}
	return path, branch, nil
}

// List inventories every worktree of the given repositories. Project filters
// by the resolved display project; empty means all.
func List(ctx context.Context, root string, repos []Repo, project string) ([]Entry, error) {
	seen := make(map[string]bool)
	entries := make([]Entry, 0)
	for _, repo := range repos {
		if repo.Repo == "" || seen[repo.Repo] {
			continue
		}
		seen[repo.Repo] = true

		out, err := gitx.Out(ctx, repo.Repo, "git", "worktree", "list", "--porcelain")
		if err != nil {
			return nil, fmt.Errorf("%s: git worktree list: %w", repo.Project, err)
		}
		roots := TaskRoots(root)
		for _, raw := range parsePorcelain(out) {
			projectName, name, kind := classify(roots, raw.Path, repo.Stable, repo.Project)
			if project != "" && projectName != project {
				continue
			}
			entries = append(entries, Entry{
				Project:       projectName,
				Name:          name,
				Kind:          kind,
				Path:          raw.Path,
				Repo:          repo.Repo,
				Branch:        raw.Branch,
				Head:          raw.Head,
				Dirty:         isDirty(ctx, raw.Path),
				Merged:        isMerged(ctx, repo.Repo, raw.Branch, repo.DefaultBranch),
				AgeDays:       ageDays(raw.Path),
				DefaultBranch: repo.DefaultBranch,
			})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Project != entries[j].Project {
			return entries[i].Project < entries[j].Project
		}
		if entries[i].Kind != entries[j].Kind {
			return entries[i].Kind < entries[j].Kind
		}
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

// Remove removes a canonical task worktree. Local branches are left untouched
// unless DeleteBranch is set; the command never touches other worktrees,
// stable checkouts, sessions, or work-item metadata.
func Remove(ctx context.Context, repo, defaultBranch, root, project, name string, opts RemoveOptions) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	path, err := resolveTaskPath(TaskRoots(root), project, name)
	if err != nil {
		return err
	}

	branch := currentBranch(ctx, path)
	dirty := isDirty(ctx, path)
	merged := isMerged(ctx, repo, branch, defaultBranch)
	if dirty && !opts.Force {
		return fmt.Errorf("worktree %s has uncommitted changes; pass --force to discard them", name)
	}
	if !merged && !opts.Force {
		return fmt.Errorf("branch %s is not merged into origin/%s; pass --force to remove anyway",
			branch, defaultBranch)
	}

	args := []string{"worktree", "remove"}
	if opts.Force {
		args = append(args, "--force")
	}
	args = append(args, path)
	if err := gitx.Run(ctx, repo, false, "git", args...); err != nil {
		return fmt.Errorf("git worktree remove: %w", err)
	}

	if opts.DeleteBranch {
		if branch == "" {
			return nil
		}
		branchArgs := []string{"branch"}
		if opts.Force {
			branchArgs = append(branchArgs, "-D")
		} else {
			branchArgs = append(branchArgs, "-d")
		}
		branchArgs = append(branchArgs, branch)
		if err := gitx.Run(ctx, repo, false, "git", branchArgs...); err != nil {
			return fmt.Errorf("git branch delete: %w", err)
		}
	}
	return nil
}

// PlanGC returns GC decisions for canonical task worktrees without mutating
// anything. RunGC performs the removals selected by the same rules.
func PlanGC(ctx context.Context, root string, repos []Repo, project string, opts GCOptions) ([]Action, error) {
	entries, err := List(ctx, root, repos, project)
	if err != nil {
		return nil, err
	}
	actions := make([]Action, 0)
	for _, e := range entries {
		if e.Kind != KindTask {
			continue
		}
		switch {
		case e.Dirty:
			actions = append(actions, Action{Entry: e, Action: "skip: dirty"})
		case !e.Merged:
			actions = append(actions, Action{Entry: e, Action: "skip: unmerged"})
		case time.Duration(e.AgeDays)*24*time.Hour < opts.MinAge:
			actions = append(actions, Action{Entry: e, Action: "skip: too young"})
		default:
			actions = append(actions, Action{Entry: e, Action: "remove"})
		}
	}
	return actions, nil
}

// RunGC executes the removals planned by PlanGC. It is only called after an
// explicit confirmation in the CLI layer.
func RunGC(ctx context.Context, root string, repos []Repo, project string, opts GCOptions) ([]Action, error) {
	actions, err := PlanGC(ctx, root, repos, project, opts)
	if err != nil {
		return nil, err
	}
	for i := range actions {
		if actions[i].Action != "remove" {
			continue
		}
		if err := Remove(ctx, actions[i].Entry.Repo, actions[i].Entry.DefaultBranch, root,
			actions[i].Entry.Project, actions[i].Entry.Name,
			RemoveOptions{DeleteBranch: opts.DeleteBranch}); err != nil {
			actions[i].Action = "failed: " + err.Error()
		}
	}
	return actions, nil
}

type rawEntry struct {
	Path   string
	Head   string
	Branch string
}

func parsePorcelain(out string) []rawEntry {
	var entries []rawEntry
	for _, block := range strings.Split(out, "\n\n") {
		var e rawEntry
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "worktree "):
				e.Path = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
			case strings.HasPrefix(line, "HEAD "):
				e.Head = strings.TrimSpace(strings.TrimPrefix(line, "HEAD "))
			case strings.HasPrefix(line, "branch refs/heads/"):
				e.Branch = strings.TrimSpace(strings.TrimPrefix(line, "branch refs/heads/"))
			}
		}
		if e.Path != "" {
			entries = append(entries, e)
		}
	}
	return entries
}

func classify(roots []string, path, stable, repoProject string) (project, name string, kind Kind) {
	if path == stable {
		return repoProject, "stable", KindStable
	}
	for _, root := range roots {
		rel, err := filepath.Rel(root, path)
		if err == nil && rel != "." && rel != ".." &&
			!strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			parts := strings.Split(rel, string(filepath.Separator))
			if len(parts) >= 2 {
				return parts[0], strings.Join(parts[1:], "/"), KindTask
			}
			return repoProject, filepath.Base(path), KindLegacy
		}
	}
	marker := string(filepath.Separator) + "cowboy-machine" + string(filepath.Separator) + "worktrees"
	if strings.Contains(path, marker) {
		return repoProject, filepath.Base(path), KindSession
	}
	return repoProject, filepath.Base(path), KindLegacy
}

func resolveTaskPath(roots []string, project, name string) (string, error) {
	missing := make([]string, 0, len(roots))
	for _, root := range roots {
		path := CanonicalPath(root, project, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect %s: %w", path, err)
		}
		missing = append(missing, path)
	}
	if len(missing) == 1 {
		return "", fmt.Errorf("worktree not found at %s", missing[0])
	}
	return "", fmt.Errorf("worktree not found at %s", strings.Join(missing, " or "))
}

func currentBranch(ctx context.Context, path string) string {
	branch, err := gitx.Out(ctx, path, "git", "branch", "--show-current")
	if err != nil {
		return ""
	}
	return branch
}

func isDirty(ctx context.Context, path string) bool {
	out, err := gitx.Out(ctx, path, "git", "status", "--porcelain=v1", "--untracked-files=normal")
	return err == nil && out != ""
}

func isMerged(ctx context.Context, repo, branch, defaultBranch string) bool {
	if branch == "" || defaultBranch == "" {
		return false
	}
	upstream := "refs/remotes/origin/" + defaultBranch
	if !gitx.RefExists(ctx, repo, upstream) {
		return false
	}
	_, err := gitx.Out(ctx, repo, "git", "merge-base", "--is-ancestor",
		"refs/heads/"+branch, upstream)
	return err == nil
}

func ageDays(path string) int {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return int(time.Since(info.ModTime()).Hours() / 24)
}
