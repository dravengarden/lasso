package project

import "errors"

// Sentinel errors for the project package. Centralised so call sites
// can use errors.Is for matching and the err113 linter sees a static
// base for every dynamic fmt.Errorf.
var (
	// ErrInvalidName is returned when a project name fails the
	// validation regex.
	ErrInvalidName = errors.New("invalid project name")
	// ErrEmptyRepo is returned when a Spec lacks a Repo URL.
	ErrEmptyRepo = errors.New("spec.Repo is required")
	// ErrEmptyRepoURL is returned by RemoteDefaultBranchFromURL when
	// called with an empty URL.
	ErrEmptyRepoURL = errors.New("empty repo URL")
	// ErrIndexInsertionPoint is returned when docs/INDEX.md no longer carries
	// the stable row before which project registration inserts its anchor.
	ErrIndexInsertionPoint = errors.New("docs index insertion point not found")
	// ErrNoSymrefHead is returned when `git ls-remote --symref` output
	// has no `ref:` line.
	ErrNoSymrefHead = errors.New("ls-remote --symref returned no `ref:` line")
)
