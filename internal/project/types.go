// Package project owns mutations to project registry entries and their remote
// reconciliation. Project documentation belongs to the owning repository and
// is deliberately independent from registration lifecycle.
package project

import "strings"

// Spec is the inputs for a new project entry. Mirrors config.Project
// but lives here so the add/remove flow doesn't drag CLI types into
// the config package.
type Spec struct {
	Name          string
	Repo          string
	DefaultBranch string
}

// ExistsError is returned when Add is called for a project whose
// project-defs/<name>/project.toml already exists. Used by the CLI
// layer to format a friendly hint.
type ExistsError struct{ Name string }

func (e ExistsError) Error() string {
	return "project " + strings.TrimSpace(e.Name) + " already exists (project-defs/" + strings.TrimSpace(e.Name) + "/)"
}

// NotFoundError is returned when Remove / Update can't find the named
// project under project-defs/.
type NotFoundError struct{ Name string }

func (e NotFoundError) Error() string {
	return "project " + strings.TrimSpace(e.Name) + " not found under project-defs/"
}
