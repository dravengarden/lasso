package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dravengarden/lasso/internal/config"
	"github.com/dravengarden/lasso/internal/gitx"
)

// UpdateOptions controls Update behaviour.
type UpdateOptions struct {
	// Fetch the stable checkout from origin before computing status.
	// Default: true. Disable when offline.
	Fetch bool
	// PullFFOnly attempts to fast-forward the stable checkout to the fetched
	// origin branch. It never merges divergent history or touches a dirty,
	// detached, or wrong-branch checkout. Default: true at the CLI layer so
	// agents start from fresh main unless they opt out.
	PullFFOnly bool
	// CheckOnly reports drift without writing project metadata or fast-forwarding
	// the checkout. Fetch may still refresh remote-tracking refs unless disabled.
	CheckOnly bool
}

// UpdateResult is the per-project diff between what the registry
// said and what the world actually shows. Returned even when nothing
// changed so callers can render a stable "what I checked" report.
type UpdateResult struct {
	Name string
	// NotApplicable is true for subdir projects without a remote
	// checkout lifecycle.
	NotApplicable bool

	OldDefaultBranch, NewDefaultBranch string
	DefaultBranchChanged               bool

	RegistryUpdated bool

	Checkout *CheckoutStatus
}

// CheckoutStatus describes the stable Local checkout's relationship to its
// upstream branch.
type CheckoutStatus struct {
	Branch         string
	ExpectedBranch string
	Path           string
	State          string // "up-to-date" | "ahead N" | "behind N" | "diverged A/B" | "no upstream" | "fetch-failed" | "status-failed"
	Dirty          bool
	FetchAttempted bool
	FetchOK        bool
	// PulledFastForward is true when PullFFOnly succeeded; absent
	// when not requested or when a safety precondition blocked it.
	PulledFastForward *bool
	// PullBlockedReason explains why an explicitly requested fast-forward was
	// not attempted. It is empty when no fast-forward was requested or needed.
	PullBlockedReason string
}

// Update reconciles a project's stored metadata and stable checkout with
// what its remote actually shows.
//
// Steps:
//  1. Detect remote's HEAD branch (via `git ls-remote --symref`)
//  2. Fetch + report the stable checkout's ahead/behind/diverged status
//  3. With PullFFOnly, fast-forward a clean checkout on its expected branch
//  4. If default_branch drifted, write the reconciled value to project.toml.
//
// Update is idempotent: rerunning when nothing has changed produces
// no writes.
func Update(ctx context.Context, harnessRoot, name string, opts UpdateOptions) (*UpdateResult, error) {
	cur, err := loadProject(harnessRoot, name)
	if err != nil {
		return nil, err
	}

	res := &UpdateResult{
		Name:             name,
		OldDefaultBranch: cur.DefaultBranch,
	}
	if cur.Kind != config.KindExternal {
		res.NewDefaultBranch = cur.DefaultBranch
		res.NotApplicable = true
		return res, nil
	}

	pdir := filepath.Join(harnessRoot, "projects", name)
	checkout := stableCheckoutPath(pdir)

	if opts.Fetch {
		updateMetadata(ctx, cur, res)
	} else {
		// --no-fetch is a fully cached/offline inspection, including remote
		// default-branch metadata.
		res.NewDefaultBranch = cur.DefaultBranch
	}

	if checkout != "" {
		status := checkoutStatus(ctx, checkout, res.NewDefaultBranch, opts)
		res.Checkout = &status
	}

	if err := patchRegistryMetadata(harnessRoot, cur, opts, res); err != nil {
		return res, err
	}

	return res, nil
}

func patchRegistryMetadata(harnessRoot string, cur projectEntry, opts UpdateOptions, res *UpdateResult) error {
	if opts.CheckOnly || !res.DefaultBranchChanged {
		return nil
	}
	path := config.ProjectFilePath(harnessRoot, res.Name)
	body := renderProjectTOML(Spec{
		Name:          res.Name,
		Repo:          cur.Repo,
		DefaultBranch: res.NewDefaultBranch,
	})
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil { //nolint:gosec // G306: user-readable registry file
		return fmt.Errorf("write %s: %w", path, err)
	}
	res.RegistryUpdated = true
	return nil
}

// updateMetadata fills NewDefaultBranch on res based on the remote.
// Pure mutation of res — no I/O failures escape (offline collapses
// to "no change").
func updateMetadata(ctx context.Context, cur projectEntry, res *UpdateResult) {
	remoteDefault, err := RemoteDefaultBranchFromURL(ctx, cur.Repo)
	if err != nil {
		// Offline or inaccessible remote: preserve the registry value.
		res.NewDefaultBranch = cur.DefaultBranch
		return
	}

	res.NewDefaultBranch = remoteDefault
	if remoteDefault != cur.DefaultBranch {
		res.DefaultBranchChanged = true
	}
}

func stableCheckoutPath(projectDir string) string {
	if info, err := os.Stat(filepath.Join(projectDir, ".git")); err == nil && info.IsDir() {
		return projectDir
	}
	return ""
}

// checkoutStatus computes the stable checkout's status, optionally fetching
// and fast-forwarding it per opts. The remote-tracking ref is explicitly
// reported as cached when Fetch is false.
func checkoutStatus(ctx context.Context, path, expectedBranch string, opts UpdateOptions) CheckoutStatus {
	branch, _ := gitx.CurrentBranch(ctx, path) // detached checkout is reported below
	status := CheckoutStatus{
		Branch:         branch,
		ExpectedBranch: expectedBranch,
		Path:           path,
	}

	dirty, err := gitx.Out(ctx, path, "git", "status", "--porcelain=v1", "--untracked-files=normal")
	if err != nil {
		status.State = "status-failed"
		return status
	}
	status.Dirty = dirty != ""

	if opts.Fetch {
		status.FetchAttempted = true
		if err := gitx.Run(ctx, path, true, "git", "fetch", "--prune", "origin"); err != nil {
			status.State = "fetch-failed"
			return status
		}

		status.FetchOK = true
	}

	if branch == "" {
		status.State = "detached"
		if opts.PullFFOnly && !opts.CheckOnly {
			status.PullBlockedReason = "checkout is detached"
		}
		return status
	}
	status.State = computeAheadBehind(ctx, path, expectedBranch)
	if opts.PullFFOnly && !opts.CheckOnly && strings.HasPrefix(status.State, "behind") {
		switch {
		case branch != expectedBranch:
			status.PullBlockedReason = fmt.Sprintf("branch %s is not expected branch %s", branch, expectedBranch)
		case status.Dirty:
			status.PullBlockedReason = "checkout is dirty"
		case !status.FetchAttempted || !status.FetchOK:
			status.PullBlockedReason = "origin refs were not refreshed"
		case gitx.Run(ctx, path, true, "git", "merge", "--ff-only", "origin/"+expectedBranch) == nil:
			t := true
			status.PulledFastForward = &t
			status.State = computeAheadBehind(ctx, path, expectedBranch)
		default:
			f := false
			status.PulledFastForward = &f
		}
	}

	return status
}

// loadProject reads one project entry into a tiny struct.
type projectEntry struct {
	Kind          string
	Repo          string
	DefaultBranch string
}

func loadProject(harnessRoot, name string) (projectEntry, error) {
	root, err := config.LoadFromRoot(harnessRoot)
	if err != nil {
		return projectEntry{}, err
	}

	p, ok := root.Projects[name]
	if !ok {
		return projectEntry{}, NotFoundError{Name: name}
	}

	return projectEntry{
		Kind:          p.Kind,
		Repo:          p.Repo,
		DefaultBranch: p.DefaultBranch,
	}, nil
}

// RemoteDefaultBranchFromURL queries the remote directly via URL, so `project add`
// can record the upstream's chosen default branch without first
// cloning. Requires network reachability to the URL.
func RemoteDefaultBranchFromURL(ctx context.Context, repoURL string) (string, error) {
	if repoURL == "" {
		return "", ErrEmptyRepoURL
	}

	out, err := gitx.Out(ctx, "", "git", "ls-remote", "--symref", repoURL, "HEAD")
	if err != nil {
		return "", fmt.Errorf("ls-remote %s HEAD: %w", repoURL, err)
	}

	return parseSymrefHead(out)
}

// parseSymrefHead extracts the branch name from `git ls-remote --symref`
// output. Expected first line: `ref: refs/heads/<branch>\tHEAD`.
func parseSymrefHead(out string) (string, error) {
	for ln := range strings.SplitSeq(out, "\n") {
		if after, ok := strings.CutPrefix(ln, "ref: "); ok {
			ref := strings.SplitN(after, "\t", 2)[0]

			return strings.TrimPrefix(ref, "refs/heads/"), nil
		}
	}

	return "", ErrNoSymrefHead
}

// computeAheadBehind compares the checkout's HEAD against
// origin/<branch>. Returns one of:
//
//	"up-to-date"     — same commit
//	"ahead N"        — local has N commits not on origin
//	"behind N"       — origin has N commits not on local
//	"diverged A/B"   — local has A, remote has B (true fork)
//	"no upstream"    — origin/<branch> doesn't exist
//
// Errors collapse to "no upstream" rather than failing — a project
// with a stale local branch shouldn't break the report.
func computeAheadBehind(ctx context.Context, checkout, branch string) string {
	if !gitx.RefExists(ctx, checkout, "refs/remotes/origin/"+branch) {
		return "no upstream"
	}

	out, err := gitx.Out(ctx, checkout, "git", "rev-list", "--left-right", "--count",
		"HEAD..."+"origin/"+branch)
	if err != nil {
		return "no upstream"
	}

	parts := strings.Fields(strings.TrimSpace(out))
	if len(parts) != 2 {
		return "unknown"
	}

	ahead, _ := strconv.Atoi(parts[0])  //nolint:errcheck // git rev-list --count always emits ints; bad parse → 0
	behind, _ := strconv.Atoi(parts[1]) //nolint:errcheck // git rev-list --count always emits ints; bad parse → 0

	switch {
	case ahead == 0 && behind == 0:
		return "up-to-date"
	case ahead == 0:
		return fmt.Sprintf("behind %d", behind)
	case behind == 0:
		return fmt.Sprintf("ahead %d", ahead)
	default:
		return fmt.Sprintf("diverged %d/%d", ahead, behind)
	}
}
