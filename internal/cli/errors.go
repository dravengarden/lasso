package cli

import "errors"

// Sentinel errors for the cli package. err113 wants dynamic
// fmt.Errorf strings to wrap a static error so callers can use
// errors.Is for matching.
var (
	// ErrSetupFailures is returned by `harness setup` when one or
	// more projects failed.
	ErrSetupFailures = errors.New("setup failures")
	// ErrProjectUpdateFailures is returned after project update reports one or
	// more per-project reconciliation failures.
	ErrProjectUpdateFailures = errors.New("project update failures")
	// ErrAmbiguous is returned when an update scope is required and
	// neither --project nor --all-projects was provided.
	ErrAmbiguous = errors.New("either pass --project=<name> or --all-projects")
	// ErrBlocked is returned by project rm when active work items reference
	// the project (the underlying ActiveWorkItemsBlockingError is logged).
	ErrBlocked = errors.New("blocked")
	// ErrValidationIssues is returned by `work-item validate` when one or
	// more issues were reported.
	ErrValidationIssues = errors.New("validation issues")
	// ErrNoRepoURL is returned when a project's registry entry has no
	// `repo` field.
	ErrNoRepoURL = errors.New("project has no `repo` in its project.toml")
	// ErrAborted is returned when an interactive command is aborted.
	ErrAborted = errors.New("aborted")
)
