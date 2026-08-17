package project

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dravengarden/lasso/internal/config"
)

// AddResult enumerates the artefacts touched, for the CLI to print
// back as a "what just happened" summary.
type AddResult struct {
	RegistryUpdated bool   // project-defs/<name>/project.toml was created
	DefPath         string // the created project.toml path, relative to root
}

// Add validates the spec, scaffolds the project's registry file
// (project-defs/<name>/project.toml — its existence IS the
// registration).
//
// The directory set under project-defs/ is the single source of truth;
// there is no aggregator file to edit.
func Add(harnessRoot string, spec Spec) (*AddResult, error) {
	if err := validateSpec(spec); err != nil {
		return nil, err
	}

	res := &AddResult{}

	defPath := config.ProjectFilePath(harnessRoot, spec.Name)
	if _, err := os.Stat(defPath); err == nil {
		return res, ExistsError{Name: spec.Name}
	} else if !os.IsNotExist(err) {
		return res, fmt.Errorf("stat %s: %w", defPath, err)
	}

	if err := os.MkdirAll(filepath.Dir(defPath), 0o755); err != nil { //nolint:gosec // G301: user-readable registry dir
		return res, fmt.Errorf("mkdir %s: %w", filepath.Dir(defPath), err)
	}
	if err := os.WriteFile(defPath, []byte(renderProjectTOML(spec)), 0o644); err != nil { //nolint:gosec // G306: user-readable registry file
		return res, fmt.Errorf("write %s: %w", defPath, err)
	}

	res.RegistryUpdated = true
	res.DefPath = filepath.Join(config.ProjectDefsDir, spec.Name, "project.toml")

	return res, nil
}

func validateSpec(s Spec) error {
	if err := ValidateName(s.Name); err != nil {
		return err
	}

	if s.Repo == "" {
		return ErrEmptyRepo
	}

	return nil
}

// ValidateName rejects path-like or non-canonical project identifiers before
// any filesystem path is constructed.
func ValidateName(name string) error {
	if !config.IsProjectName(name) {
		return fmt.Errorf("%w %q (must match %s)", ErrInvalidName, name, config.ProjectNamePattern)
	}
	return nil
}

// renderProjectTOML produces the complete minimal registry entry. Operational
// commands and deployment metadata live with their owning project or host.
func renderProjectTOML(spec Spec) string {
	return fmt.Sprintf("kind = %q\nrepo = %q\ndefault_branch = %q\n", "external", spec.Repo, spec.DefaultBranch)
}
