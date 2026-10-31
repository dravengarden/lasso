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

// LegacyRootEnv is accepted while Columbus instances migrate onto Lasso.
const LegacyRootEnv = "COLUMBUS_WORKTREE_ROOT"

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
	Locked        bool   `json:"locked"`
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
	// MinAge is the minimum idle time accepted for automatic removal: the time
	// since the newest change to the worktree root, its Git metadata (index,
	// HEAD, reflog) or any tracked or untracked non-ignored file. Zero means no age requirement; the CLI layer owns
	// the user-facing default.
	MinAge time.Duration
	// DeleteBranch passes through to Remove after a successful removal.
	DeleteBranch bool
	// PruneRegenerable accepts ignored content when every ignored entry is a
	// regenerable dependency or build cache directory (RegenerableDirs).
	// Other ignored files still preserve the worktree.
	PruneRegenerable bool
}

// RegenerableDirs are ignored directory names whose contents a package manager
// or compiler recreates from tracked sources. Build outputs such as dist/ are
// deliberately absent: they can hold evidence that is not rebuilt.
var RegenerableDirs = map[string]bool{
	"node_modules":  true,
	"target":        true,
	"__pycache__":   true,
	".pytest_cache": true,
	".mypy_cache":   true,
	".ruff_cache":   true,
	".rumdl_cache":  true,
	".turbo":        true,
	".next":         true,
	".wrangler":     true,
	".worker-tools": true,
	"site-dist":     true,
}

// nixOutLink matches `nix build` out-links (result, result-web, ...). They
// are garbage-collector roots into /nix/store, never workspace content.
var nixOutLink = regexp.MustCompile(`^result(-[A-Za-z0-9._+-]+)?$`)

func isNixOutLink(path string) bool {
	if !nixOutLink.MatchString(filepath.Base(path)) {
		return false
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return false
	}
	target, err := os.Readlink(path)
	return err == nil && strings.HasPrefix(target, "/nix/store/")
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
	if v := strings.TrimSpace(os.Getenv(LegacyRootEnv)); v != "" {
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
				Merged:        isMerged(ctx, repo.Repo, raw.Branch, raw.Head, repo.DefaultBranch),
				AgeDays:       ageDays(raw.Path),
				DefaultBranch: repo.DefaultBranch,
				Locked:        raw.Locked,
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
	head, _ := gitx.Out(ctx, path, "git", "rev-parse", "HEAD")
	merged := isMerged(ctx, repo, branch, strings.TrimSpace(head), defaultBranch)
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
	live := liveProcessPaths()
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
		case e.Locked:
			actions = append(actions, Action{Entry: e, Action: "skip: locked"})
		case time.Duration(e.AgeDays)*24*time.Hour < opts.MinAge:
			actions = append(actions, Action{Entry: e, Action: "skip: too young"})
		case inUse(live, e.Path):
			actions = append(actions, Action{Entry: e, Action: "skip: in use"})
		default:
			reason := gcContentGuard(ctx, e.Path, opts.PruneRegenerable)
			if reason == "" {
				reason = "remove"
			}
			actions = append(actions, Action{Entry: e, Action: reason})
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
	var failures []error
	for i := range actions {
		if actions[i].Action != "remove" {
			continue
		}
		// Recheck ignored content before Remove: Git deliberately ignores it
		// when deciding whether a worktree is clean enough to delete.
		if reason := gcContentGuard(ctx, actions[i].Entry.Path, opts.PruneRegenerable); reason != "" {
			actions[i].Action = reason
			continue
		}
		if err := Remove(ctx, actions[i].Entry.Repo, actions[i].Entry.DefaultBranch, root,
			actions[i].Entry.Project, actions[i].Entry.Name,
			RemoveOptions{DeleteBranch: opts.DeleteBranch}); err != nil {
			actions[i].Action = "failed: " + err.Error()
			failures = append(failures, fmt.Errorf("%s/%s: %w", actions[i].Entry.Project, actions[i].Entry.Name, err))
		}
	}
	return actions, errors.Join(failures...)
}

// GC has no knowledge of build artifacts or live runtime ownership. Ignored
// files may be unique user data; only an explicit lifecycle owner may retire
// them. Submodule worktrees need their own teardown and are never forced.
func gcContentGuard(ctx context.Context, path string, pruneRegenerable bool) string {
	ignored, err := gitx.Out(ctx, path, "git", "ls-files", "--others", "--ignored", "--exclude-standard", "--directory")
	if err != nil {
		return "skip: cannot inspect ignored files"
	}
	if ignored != "" && !(pruneRegenerable && onlyRegenerable(path, ignored)) {
		return "skip: ignored content"
	}
	files, err := gitx.Out(ctx, path, "git", "ls-files", "--stage")
	if err != nil {
		return "skip: cannot inspect submodules"
	}
	for _, line := range strings.Split(files, "\n") {
		if strings.HasPrefix(line, "160000 ") {
			return "skip: submodules"
		}
	}
	return ""
}

type rawEntry struct {
	Path   string
	Head   string
	Branch string
	Locked bool
}

func parsePorcelain(out string) []rawEntry {
	var entries []rawEntry
	for _, block := range strings.Split(out, "\n\n") {
		var e rawEntry
		for _, line := range strings.Split(block, "\n") {
			switch {
			case line == "locked" || strings.HasPrefix(line, "locked "):
				e.Locked = true
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
	return err != nil || out != ""
}

var commitID = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

// isMerged reports whether every change on branch is already in
// origin/<defaultBranch>: either the branch is an ancestor, or merging it
// would leave the upstream tree unchanged (squash or rebase merges). A
// detached worktree (no branch) is judged by its HEAD commit.
func isMerged(ctx context.Context, repo, branch, head, defaultBranch string) bool {
	if defaultBranch == "" {
		return false
	}
	local := "refs/heads/" + branch
	if branch == "" {
		if !commitID.MatchString(head) {
			return false
		}
		local = head
	}
	upstream := "refs/remotes/origin/" + defaultBranch
	if !gitx.RefExists(ctx, repo, upstream) {
		return false
	}
	if _, err := gitx.Out(ctx, repo, "git", "merge-base", "--is-ancestor", local, upstream); err == nil {
		return true
	}
	// Tree equivalence only counts for a branch that changed files: an empty
	// or self-cancelling branch still carries commits upstream never saw.
	base, err := gitx.Out(ctx, repo, "git", "merge-base", upstream, local)
	if err != nil {
		return false
	}
	treeOf := func(rev string) string {
		out, err := gitx.Out(ctx, repo, "git", "rev-parse", strings.TrimSpace(rev)+"^{tree}")
		if err != nil {
			return ""
		}
		return strings.TrimSpace(out)
	}
	branchTree, baseTree, upstreamTree := treeOf(local), treeOf(base), treeOf(upstream)
	if branchTree == "" || upstreamTree == "" || branchTree == baseTree {
		return false
	}
	merged, err := gitx.Out(ctx, repo, "git", "merge-tree", "--write-tree", upstream, local)
	if err != nil {
		return false
	}
	return strings.TrimSpace(strings.SplitN(merged, "\n", 2)[0]) == upstreamTree
}

// onlyRegenerable reports whether every `ls-files --ignored --directory`
// entry under root is a RegenerableDirs directory. Git also lists an untracked
// directory holding only ignored content (web/ around web/node_modules/); such
// a directory qualifies only when each file in it lies below a regenerable
// directory.
func onlyRegenerable(root, ignored string) bool {
	for _, line := range strings.Split(strings.TrimSpace(ignored), "\n") {
		if !strings.HasSuffix(line, "/") {
			if isNixOutLink(filepath.Join(root, line)) {
				continue
			}
			return false
		}
		entry := strings.TrimSuffix(line, "/")
		if RegenerableDirs[filepath.Base(entry)] {
			continue
		}
		ok := true
		err := filepath.WalkDir(filepath.Join(root, entry), func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if RegenerableDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			ok = false
			return filepath.SkipAll
		})
		if err != nil || !ok {
			return false
		}
	}
	return true
}

// ageDays is the idle time in days: since the newest modification of the
// worktree root, its Git metadata, or any tracked or untracked non-ignored
// file. Ignored dependency and build caches do not count as activity.
func ageDays(path string) int {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	newest := info.ModTime()
	consider := func(candidate string) {
		if info, err := os.Lstat(candidate); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	ctx := context.Background()
	if gitDir, err := gitx.Out(ctx, path, "git", "rev-parse", "--absolute-git-dir"); err == nil {
		gitDir = strings.TrimSpace(gitDir)
		for _, name := range []string{"index", "HEAD", filepath.Join("logs", "HEAD")} {
			consider(filepath.Join(gitDir, name))
		}
	} else {
		return 0
	}
	files, err := gitx.Out(ctx, path, "git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return 0
	}
	for _, name := range strings.Split(files, "\x00") {
		if name != "" {
			consider(filepath.Join(path, name))
		}
	}
	return int(time.Since(newest).Hours() / 24)
}
