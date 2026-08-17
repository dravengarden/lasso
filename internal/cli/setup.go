package cli

import (
	"context"
	"fmt"
	"os"
	"slices"

	"github.com/spf13/cobra"

	"github.com/dravengarden/lasso/internal/config"
	"github.com/dravengarden/lasso/internal/ui"
)

// setupResult is the per-project outcome surfaced in the summary table.
type setupResult struct {
	name   string
	state  string // cloned | already | not-applicable | error
	detail string
}

func newSetupCmd() *cobra.Command {
	var (
		only []string
	)

	c := &cobra.Command{
		Use:   "setup",
		Short: "Bootstrap the workspace with one ordinary clone per external project",
		Long: "setup brings a fresh harness clone to a usable state.\n\n" +
			"For each registered project (project-defs/<name>/):\n" +
			"  - if projects/<name> is missing: clone the repository there\n" +
			"  - if an ordinary clone is already present: no-op\n\n" +
			"Network failures on individual projects don't abort the run.\n" +
			"A summary is printed at the end.\n\n" +
			"Run again any time — it's idempotent.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, root, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			names := r.Names()
			slices.Sort(names)
			names, err = filterSetupNames(names, only)
			if err != nil {
				return err
			}

			results, failures := runSetup(cmd.Context(), r, root, names)

			fmt.Fprintln(os.Stderr)
			renderSetupSummary(results)

			if failures > 0 {
				return fmt.Errorf("%w: %d project(s) failed", ErrSetupFailures, failures)
			}

			return nil
		},
	}
	c.Flags().StringSliceVar(&only, "only", nil,
		"comma-separated subset of projects to setup (default: all)")

	return c
}

// filterSetupNames trims names down to the user-specified --only set.
// Returns the input unchanged when only is empty.
func filterSetupNames(names, only []string) ([]string, error) {
	if len(only) == 0 {
		return names, nil
	}

	known := make(map[string]bool, len(names))
	for _, name := range names {
		known[name] = true
	}
	want := map[string]bool{}
	for _, n := range only {
		if !known[n] {
			return nil, fmt.Errorf("unknown project in --only: %s", n)
		}
		want[n] = true
	}

	filtered := names[:0]
	for _, n := range names {
		if want[n] {
			filtered = append(filtered, n)
		}
	}

	return filtered, nil
}

// runSetup processes every project, returning the per-project results
// plus a count of error rows.
func runSetup(ctx context.Context, r *config.Root, root string, names []string) ([]setupResult, int) {
	results := make([]setupResult, 0, len(names))
	failures := 0

	for _, name := range names {
		fmt.Fprintln(os.Stderr)
		ui.Dimf("=== %s ===", name)

		res, isFailure := setupOne(ctx, r, root, name)
		results = append(results, res)

		if isFailure {
			failures++
		}
	}

	return results, failures
}

// setupOne handles a single project and ensures its stable Local checkout.
func setupOne(ctx context.Context, r *config.Root, root, name string) (setupResult, bool) {
	project, err := r.MustGet(name)
	if err != nil {
		return setupResult{name, "error", err.Error()}, true
	}
	if !project.IsExternal() {
		return setupResult{name, "not-applicable", project.Kind + " project"}, false
	}

	res, err := EnsureCheckout(ctx, r, root, name)
	if err != nil {
		ui.Errf("%s: %v", name, err)
		return setupResult{name, "error", err.Error()}, true
	}

	switch {
	case res.Cloned:
		return setupResult{name, "ok-cloned", "ordinary clone " + res.Branch}, false
	case res.AlreadyExisted:
		return setupResult{name, "already", fmt.Sprintf("checkout %s already present", res.Branch)}, false
	default:
		return setupResult{name, "ok-checkout", "created checkout " + res.Branch}, false
	}
}

// renderSetupSummary prints the table of per-project setup results.
func renderSetupSummary(results []setupResult) {
	t := ui.Table("Setup summary")
	t.AppendHeader(rowOf("project", "state", "detail"))

	for _, r := range results {
		t.AppendRow(rowOf(r.name, r.state, r.detail))
	}

	t.Render()
}
