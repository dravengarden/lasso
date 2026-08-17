package project

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dravengarden/lasso/internal/config"
	"github.com/dravengarden/lasso/internal/workitems"
)

// RemoveOptions tunes Remove behaviour.
type RemoveOptions struct {
	// Force skips the active work-item safety check.
	Force bool
}

// RemoveResult enumerates the artefacts touched.
type RemoveResult struct {
	RegistryUpdated bool
	// ActiveWorkItems lists ids in work-items/active/ that include this
	// project. Populated even when Force=false; the
	// caller surfaces them.
	ActiveWorkItems []string
}

// ActiveWorkItemsBlockingError is returned when durable active work items
// reference the project being removed and Force is false.
type ActiveWorkItemsBlockingError struct {
	Project string
	Items   []string
}

func (e ActiveWorkItemsBlockingError) Error() string {
	return fmt.Sprintf(
		"%d active work item(s) reference project %q: %v. "+
			"Archive or reassign them first, or pass --force.",
		len(e.Items), e.Project, e.Items)
}

// Remove drops a project by deleting its project-defs/<name>/ registry
// directory.
//
// Refuses to run if any active work item includes the project, unless
// Force is true. The local checkout under projects/<name>/ is NEVER touched.
func Remove(harnessRoot, name string, opts RemoveOptions) (*RemoveResult, error) {
	res := &RemoveResult{}
	if err := ValidateName(name); err != nil {
		return res, err
	}

	defDir := filepath.Join(config.ProjectDefsPath(harnessRoot), name)
	if _, err := os.Stat(defDir); err != nil {
		if os.IsNotExist(err) {
			return res, NotFoundError{Name: name}
		}

		return res, fmt.Errorf("stat %s: %w", defDir, err)
	}

	matches, err := scanActiveWorkItemsForProject(harnessRoot, name)
	if err != nil {
		return nil, err
	}

	res.ActiveWorkItems = matches
	if len(matches) > 0 && !opts.Force {
		return res, ActiveWorkItemsBlockingError{Project: name, Items: matches}
	}

	if err := os.RemoveAll(defDir); err != nil {
		return res, fmt.Errorf("remove %s: %w", defDir, err)
	}

	res.RegistryUpdated = true

	return res, nil
}

// scanActiveWorkItemsForProject fails closed on corrupt metadata so project
// removal cannot silently orphan a durable reference.
func scanActiveWorkItemsForProject(harnessRoot, name string) ([]string, error) {
	store := workitems.NewStore(harnessRoot)

	all, err := store.ListActive()
	if err != nil {
		return nil, fmt.Errorf("list active work items: %w", err)
	}

	var matches []string

	for _, item := range all {
		for _, project := range item.Projects {
			if project == name {
				matches = append(matches, item.ID)
				break
			}
		}
	}

	return matches, nil
}
