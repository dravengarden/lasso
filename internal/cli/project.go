package cli

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/spf13/cobra"

	"github.com/dravengarden/lasso/internal/cliformat"
	"github.com/dravengarden/lasso/internal/config"
	"github.com/dravengarden/lasso/internal/project"
	"github.com/dravengarden/lasso/internal/ui"
)

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Inspect the project registry",
	}
	cmd.AddCommand(projectListCmd())
	cmd.AddCommand(projectAddCmd(), projectRmCmd(), projectUpdateCmd(), projectPathCmd())

	return cmd
}

// ── list ─────────────────────────────────────────────────────────────

// projectListEntry is the JSON shape emitted by `project list --format=json`.
// Matches the table columns one-for-one so nu pipelines can rely on it.
type projectListEntry struct {
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	DefaultBranch string `json:"default_branch"`
	Present       bool   `json:"present"`
}

func projectListCmd() *cobra.Command {
	var format cliformat.Format

	c := &cobra.Command{
		Use:   "list",
		Short: "List all registered projects",
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, root, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			names := r.Names()
			slices.Sort(names)

			entries := make([]projectListEntry, 0, len(names))
			for _, name := range names {
				p := r.Projects[name]
				_, statErr := os.Stat(r.ProjectDir(root, name))
				entries = append(entries, projectListEntry{
					Name:          name,
					Kind:          p.Kind,
					DefaultBranch: p.DefaultBranch,
					Present:       statErr == nil,
				})
			}

			if format.IsStructured() {
				return cliformat.Print(cmd.OutOrStdout(), format, entries)
			}

			t := ui.Table("Projects")
			t.AppendHeader(rowOf("name", "kind", "default branch", "present"))
			for _, e := range entries {
				t.AppendRow(rowOf(e.Name, e.Kind, e.DefaultBranch, checkOrDot(e.Present)))
			}
			t.Render()
			return nil
		},
	}
	cliformat.Bind(c, &format)

	return c
}

func checkOrDot(b bool) string {
	if b {
		return "✓"
	}
	return "·"
}

// ── update ───────────────────────────────────────────────────────────

func projectUpdateCmd() *cobra.Command {
	var (
		projectName string
		allProjects bool
		noFetch     bool
		pullFFOnly  bool
		checkOnly   bool
	)

	c := &cobra.Command{
		Use:   "update",
		Short: "Reconcile a project's metadata and stable checkout with its remote",
		Long: `Update walks one (or every, with --all-projects) project and:

  - re-detects the remote's default branch (git ls-remote --symref)
  - fetches the stable Local checkout and reports ahead / behind / diverged
  - reports dirty, detached, and wrong-branch safety conditions
  - with --pull-ff-only (default), fast-forwards a clean expected branch after fetch
  - writes a changed default_branch to project.toml

Pass --check to report without writing project metadata or fast-forwarding the checkout.

If the remote default branch changed, checkout status is evaluated against the
detected branch before registry metadata is reconciled. The command never
switches branches implicitly.

Idempotent: rerunning when nothing changed produces no writes.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if projectName != "" && allProjects {
				return fmt.Errorf("--project and --all-projects are mutually exclusive")
			}
			r, root, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			opts := project.UpdateOptions{
				Fetch:      !noFetch,
				PullFFOnly: pullFFOnly,
				CheckOnly:  checkOnly,
			}

			var names []string

			switch {
			case allProjects:
				names = r.Names()
				slices.Sort(names)
			case projectName != "":
				names = []string{projectName}
			default:
				return ErrAmbiguous
			}

			anyChanged := false
			failures := 0

			for _, name := range names {
				res, err := project.Update(cmd.Context(), root, name, opts)
				if err != nil {
					ui.Errf("%s: %v", name, err)
					failures++
					continue
				}

				if printUpdateResult(res) {
					anyChanged = true
				}
			}

			if failures > 0 {
				return fmt.Errorf("%w: %d project(s)", ErrProjectUpdateFailures, failures)
			}

			if !anyChanged {
				ui.OKf("nothing to update")
			}

			return nil
		},
	}
	c.Flags().StringVar(&projectName, "project", "", "harness project name to update (mutually exclusive with --all-projects)")
	c.Flags().BoolVar(&allProjects, "all-projects", false, "update every project under project-defs/")
	c.Flags().BoolVar(&noFetch, "no-fetch", false, "skip git fetch (use cached refs only)")
	c.Flags().BoolVar(&pullFFOnly, "pull-ff-only", true, "fast-forward a clean stable checkout after fetching origin (default true; use --pull-ff-only=false to opt out)")
	c.Flags().BoolVar(&checkOnly, "check", false, "report drift without writing metadata or fast-forwarding the checkout")

	return c
}

// printUpdateResult emits a per-project block. Returns true when the
// project saw an actual change (used by the caller to decide whether
// to print the global "nothing to update" hint at the end).
func printUpdateResult(r *project.UpdateResult) bool {
	if r.NotApplicable {
		ui.Dimf("· %s — not applicable", r.Name)
		return false
	}
	changed := r.DefaultBranchChanged
	if r.Checkout != nil {
		if r.Checkout.State != "up-to-date" && r.Checkout.State != "no upstream" {
			changed = true
		}
		if r.Checkout.Dirty || r.Checkout.Branch != r.Checkout.ExpectedBranch {
			changed = true
		}
	}

	if !changed {
		ui.Dimf("· %s — clean", r.Name)
		return false
	}

	ui.OKf("%s", r.Name)

	if r.DefaultBranchChanged {
		ui.Dimf("    default_branch: %s → %s", r.OldDefaultBranch, r.NewDefaultBranch)
	}

	if r.RegistryUpdated {
		ui.Dimf("    project-defs/%s/project.toml reconciled", r.Name)
	}

	if r.Checkout != nil {
		state := r.Checkout.State
		if !r.Checkout.FetchAttempted {
			state += " (cached refs)"
		}
		if r.Checkout.Dirty {
			state += ", dirty"
		}
		if r.Checkout.Branch != "" && r.Checkout.Branch != r.Checkout.ExpectedBranch {
			state += fmt.Sprintf(", expected branch %s", r.Checkout.ExpectedBranch)
		}
		if r.Checkout.PulledFastForward != nil {
			if *r.Checkout.PulledFastForward {
				state += " → fast-forwarded"
			} else {
				state += " → fast-forward failed"
			}
		} else if r.Checkout.PullBlockedReason != "" {
			state += " → fast-forward blocked: " + r.Checkout.PullBlockedReason
		}

		ui.Dimf("    checkout %-30s %s", r.Checkout.Branch, state)
	}

	return true
}

// ── add ──────────────────────────────────────────────────────────────

func projectAddCmd() *cobra.Command {
	var (
		projectName   string
		repoURL       string
		defaultBranch string
		noClone       bool
	)

	c := &cobra.Command{
		Use:   "add",
		Short: "Register a new project and create its stable ordinary clone",
		Long: `Add a project to the harness. By default touches:

  - project registry                    (new project-defs/<n>/project.toml)
  - projects/<n>/                       (ordinary Git clone)

The default branch comes from the upstream's HEAD via
'git ls-remote --symref <repo-url> HEAD' — we don't guess. Pass
--default-branch only to override the remote (rare). If the remote
can't be reached, falls back to "main" with a warning so the entry
can still be added offline.

Pass --no-clone for a registry-only add. The clone can be created later via
'lasso setup --only=<name>'.

Deployment and service health belong to the owning host configuration, not
the Lasso project registry.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if projectName == "" {
				return fmt.Errorf("--project is required")
			}

			_, root, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			// Trust the remote first. If the user passed
			// --default-branch explicitly, that wins. Otherwise ask
			// `git ls-remote --symref` and fall back to "main" only
			// if we genuinely can't reach the remote.
			if defaultBranch == "" {
				if remote, rerr := project.RemoteDefaultBranchFromURL(cmd.Context(), repoURL); rerr == nil && remote != "" {
					ui.Dimf("upstream default branch: %s", remote)

					defaultBranch = remote
				} else {
					ui.Warnf("could not query remote default branch (%v); falling back to \"main\"", rerr)

					defaultBranch = "main"
				}
			}

			spec := project.Spec{
				Name:          projectName,
				Repo:          repoURL,
				DefaultBranch: defaultBranch,
			}

			res, err := project.Add(root, spec)
			if err != nil {
				return fmt.Errorf("add project: %w", err)
			}

			ui.OKf("project %q added", spec.Name)

			if res.RegistryUpdated {
				ui.Dimf("  %s created", res.DefPath)
			}

			if noClone {
				ui.Dimf("  next: bin/lasso setup --only=%s", spec.Name)
				return nil
			}

			// Reload registry so EnsureCheckout sees the entry we just wrote.
			r2, _, err := config.Load()
			if err != nil {
				return fmt.Errorf("reload config after add: %w", err)
			}

			checkout, cloneErr := EnsureCheckout(cmd.Context(), r2, root, spec.Name)
			if cloneErr != nil {
				ui.Warnf("clone failed: %v", cloneErr)
				ui.Dimf("  registry entries are in place; rerun: bin/lasso setup --only=%s", spec.Name)

				return cloneErr
			}

			if checkout.Cloned {
				ui.Dimf("  ordinary clone created")
			}

			ui.Dimf("  checkout ready: %s", checkout.Path)

			return nil
		},
	}
	c.Flags().StringVar(&projectName, "project", "", "new harness project name (required)")
	c.Flags().StringVar(&repoURL, "repo-url", "", "git remote URL (required)")
	c.Flags().StringVar(&defaultBranch, "default-branch", "",
		"override the upstream default branch (auto-detected via `git ls-remote --symref` when omitted)")
	c.Flags().BoolVar(&noClone, "no-clone", false, "registry-only add; skip ordinary clone creation")

	for _, flag := range []string{"project", "repo-url"} {
		if err := c.MarkFlagRequired(flag); err != nil {
			panic(err) // programming bug: flag wasn't registered
		}
	}

	return c
}

// ── rm ───────────────────────────────────────────────────────────────

func projectRmCmd() *cobra.Command {
	var (
		projectName string
		force       bool
		yes         bool
	)

	c := &cobra.Command{
		Use:   "rm",
		Short: "Unregister a project from the project registry",
		Long: `Remove a project from the harness. Touches:

  - project registry                    (entry removed)

Refuses to run if an active work item includes <name>. Pass --force to
override; the item must then be reassigned or removed by hand.

Does NOT touch projects/<n>/ or its stable checkout. Removing local files is a
separate user decision.

Permanent registry removal requires --yes.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if projectName == "" {
				return fmt.Errorf("--project is required")
			}
			if !yes {
				return fmt.Errorf("permanent project removal requires --yes: %w", ErrAborted)
			}

			_, root, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			res, err := project.Remove(root, projectName, project.RemoveOptions{
				Force: force,
			})
			if err != nil {
				var blocking project.ActiveWorkItemsBlockingError
				if errors.As(err, &blocking) {
					ui.Errf("%s", err)
					return ErrBlocked
				}

				return fmt.Errorf("remove project: %w", err)
			}

			ui.OKf("project %q removed", projectName)

			if res.RegistryUpdated {
				ui.Dimf("  project-defs/%s/ removed", projectName)
			}

			if len(res.ActiveWorkItems) > 0 {
				ui.Warnf("  %d work item(s) still referenced this project (kept due to --force): %v",
					len(res.ActiveWorkItems), res.ActiveWorkItems)
			}

			ui.Dimf("  next: remove projects/%s separately if you want to drop the local checkout", projectName)

			return nil
		},
	}
	c.Flags().StringVar(&projectName, "project", "", "harness project name (required)")
	c.Flags().BoolVar(&force, "force", false, "ignore active work-item references")
	c.Flags().BoolVar(&yes, "yes", false, "confirm permanent registry removal")
	_ = c.MarkFlagRequired("project")

	return c
}

// ── path ─────────────────────────────────────────────────────────────

func projectPathCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "path <name>",
		Short: "Print a project's checked-out path",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			registry, root, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			path, err := registry.CheckoutDir(root, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		},
	}
	return c
}

// rowOf is a tiny helper since go-pretty wants table.Row ([]any).
func rowOf(cells ...any) []any { return cells }
