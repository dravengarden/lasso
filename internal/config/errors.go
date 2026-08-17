package config

import "errors"

// Sentinel errors for the config package.
var (
	// ErrProjectRegistryNotFound is returned when registry.toml is absent.
	ErrProjectRegistryNotFound = errors.New("project registry not found")
	// ErrUnknownProject is returned when MustGet is asked for a
	// project name that isn't in the project registry.
	ErrUnknownProject = errors.New("unknown project")
)
