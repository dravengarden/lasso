package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dravengarden/lasso/internal/cliformat"
	"github.com/dravengarden/lasso/internal/config"
	"github.com/dravengarden/lasso/internal/gitx"
	"github.com/dravengarden/lasso/internal/ui"
	"github.com/dravengarden/lasso/internal/worktree"
)

func newWorktreeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worktree",
		Short: "Manage canonical task worktrees without owning task state",
		Long: `Manage task worktrees in a canonical location (<root>/<project>/<name>)
so every agent runtime can find, reuse, and safely garbage-collect them.

The canonical create root is $LASSO_WORKTREE_ROOT or $HOME/worktrees.
list, remove, and gc also treat $HOME/worktrees as a task root when the
environment override points elsewhere, so existing trees stay visible.
Native agent runtimes still own the live task: this command never persists
task state, infers an owner from a branch, or touches work-item metadata.
Git remains the only source of truth.`,
	}
	cmd.AddCommand(
		worktreeCreateCmd(),
		worktreeListCmd(),
		worktreeRemoveCmd(),
		worktreeGcCmd(),
	)
	return cmd
}

func worktreeCreateCmd() *cobra.Command {
	var (
		projectName string
		branch      string
		from        string
		noFetch     bool
	)
	c := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a canonical task worktree from origin/<default-branch>",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if projectName == "" {
				return fmt.Errorf("--project is required")
			}
			registry, root, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			repo, err := worktreeRepoFor(cmd.Context(), registry, root, projectName)
			if err != nil {
				return err
			}
			wtRoot, err := worktree.Root()
			if err != nil {
				return err
			}
			if from == "" {
				from = "origin/" + repo.DefaultBranch
			}
			path, branchName, err := worktree.Create(cmd.Context(), repo.Repo, wtRoot, repo.Project, name, worktree.CreateOptions{
				Branch: branch,
				From:   from,
				Fetch:  !noFetch,
			})
			if err != nil {
				return err
			}
			ui.OKf("worktree %s/%s created", repo.Project, name)
			ui.Dimf("  path: %s", path)
			ui.Dimf("  branch: %s", branchName)
			return nil
		},
	}
	c.Flags().StringVar(&projectName, "project", "", "registered project that owns the worktree (required)")
	c.Flags().StringVar(&branch, "branch", "", "local branch to create (default: codex/<name>)")
	c.Flags().StringVar(&from, "from", "", "source ref (default: origin/<default-branch>)")
	c.Flags().BoolVar(&noFetch, "no-fetch", false, "skip git fetch before creating")
	_ = c.MarkFlagRequired("project")
	return c
}

func worktreeListCmd() *cobra.Command {
	var (
		format        cliformat.Format
		projectFilter string
	)
	c := &cobra.Command{
		Use:   "list",
		Short: "List worktrees of registered repositories",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			registry, root, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			wtRoot, err := worktree.Root()
			if err != nil {
				return err
			}
			var repos []worktree.Repo
			if projectFilter != "" {
				repo, repoErr := worktreeRepoFor(cmd.Context(), registry, root, projectFilter)
				if repoErr != nil {
					return repoErr
				}
				repos = []worktree.Repo{repo}
			} else {
				repos = allWorktreeRepos(cmd.Context(), registry, root)
			}
			entries, err := worktree.List(cmd.Context(), wtRoot, repos, projectFilter)
			if err != nil {
				return err
			}
			if format.IsStructured() {
				return cliformat.Print(cmd.OutOrStdout(), format, entries)
			}
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no worktrees)")
				return nil
			}
			table := ui.Table("Worktrees")
			table.AppendHeader(rowOf("project", "name", "kind", "branch", "dirty", "merged", "age(d)"))
			for _, e := range entries {
				table.AppendRow(rowOf(
					e.Project, e.Name, string(e.Kind), e.Branch,
					checkOrDot(e.Dirty), checkOrDot(e.Merged), fmt.Sprintf("%d", e.AgeDays),
				))
			}
			table.Render()
			return nil
		},
	}
	cliformat.Bind(c, &format)
	c.Flags().StringVar(&projectFilter, "project", "", "show only one registered project")
	return c
}

func worktreeRemoveCmd() *cobra.Command {
	var (
		projectName  string
		force        bool
		deleteBranch bool
		yes          bool
	)
	c := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove one canonical task worktree",
		Long: `Remove <root>/<project>/<name> and optionally its local branch.

Safety: refuses dirty or unmerged worktrees unless --force is passed, and
refuses to run at all without --yes. It removes only this worktree: stable
checkouts, sessions, other worktrees, and work-item metadata are never
touched.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("worktree removal requires --yes: %w", ErrAborted)
			}
			if projectName == "" {
				return fmt.Errorf("--project is required")
			}
			registry, root, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			repo, err := worktreeRepoFor(cmd.Context(), registry, root, projectName)
			if err != nil {
				return err
			}
			wtRoot, err := worktree.Root()
			if err != nil {
				return err
			}
			if err := worktree.Remove(cmd.Context(), repo.Repo, repo.DefaultBranch, wtRoot, repo.Project, args[0], worktree.RemoveOptions{
				Force:        force,
				DeleteBranch: deleteBranch,
			}); err != nil {
				return err
			}
			ui.OKf("removed worktree %s/%s", repo.Project, args[0])
			if !deleteBranch {
				ui.Dimf("local branch untouched; use --delete-branch next time or `git branch -d`")
			}
			return nil
		},
	}
	c.Flags().StringVar(&projectName, "project", "", "registered project that owns the worktree (required)")
	c.Flags().BoolVar(&force, "force", false, "discard uncommitted changes and remove unmerged worktrees")
	c.Flags().BoolVar(&deleteBranch, "delete-branch", false, "also delete the local branch after removal")
	c.Flags().BoolVar(&yes, "yes", false, "confirm the destructive removal")
	_ = c.MarkFlagRequired("project")
	return c
}

func worktreeGcCmd() *cobra.Command {
	var (
		projectName  string
		minAge       time.Duration
		deleteBranch bool
		yes          bool
	)
	c := &cobra.Command{
		Use:   "gc",
		Short: "Garbage-collect clean, merged, aged canonical task worktrees",
		Long: `Plan or remove canonical task worktrees whose branch is already merged
into origin/<default-branch>, whose checkout is clean, and whose age exceeds
--min-age. Without --yes this is a dry run. Sessions and stable checkouts are
never candidates.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			registry, root, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			wtRoot, err := worktree.Root()
			if err != nil {
				return err
			}
			var repos []worktree.Repo
			if projectName != "" {
				repo, repoErr := worktreeRepoFor(cmd.Context(), registry, root, projectName)
				if repoErr != nil {
					return repoErr
				}
				repos = []worktree.Repo{repo}
			} else {
				repos = allWorktreeRepos(cmd.Context(), registry, root)
			}
			opts := worktree.GCOptions{MinAge: minAge, DeleteBranch: deleteBranch}

			var actions []worktree.Action
			if yes {
				actions, err = worktree.RunGC(cmd.Context(), wtRoot, repos, projectName, opts)
			} else {
				actions, err = worktree.PlanGC(cmd.Context(), wtRoot, repos, projectName, opts)
			}
			if err != nil {
				return err
			}
			for _, action := range actions {
				switch action.Action {
				case "remove":
					ui.OKf("remove %s/%s (%s)", action.Entry.Project, action.Entry.Name, action.Entry.Branch)
				default:
					ui.Dimf("%s %s/%s", action.Action, action.Entry.Project, action.Entry.Name)
				}
			}
			if !yes {
				ui.Dimf("dry run: pass --yes to remove the %q entries", "remove")
			}
			return nil
		},
	}
	c.Flags().StringVar(&projectName, "project", "", "limit GC to one registered project")
	c.Flags().DurationVar(&minAge, "min-age", 24*time.Hour, "minimum worktree age accepted for removal")
	c.Flags().BoolVar(&deleteBranch, "delete-branch", false, "delete the local branch after removing the worktree")
	c.Flags().BoolVar(&yes, "yes", false, "actually remove instead of dry-run")
	return c
}

func worktreeRepoFor(ctx context.Context, registry *config.Root, root, projectName string) (worktree.Repo, error) {
	mainRoot := root
	if main, err := mainWorktreePath(ctx, root); err == nil && main != "" {
		mainRoot = main
	}
	if projectName == "lasso" {
		return worktree.Repo{
			Project:       "lasso",
			Repo:          root,
			Stable:        mainRoot,
			DefaultBranch: "main",
		}, nil
	}
	p, err := registry.MustGet(projectName)
	if err != nil {
		return worktree.Repo{}, err
	}
	branch := p.DefaultBranch
	if branch == "" {
		branch = "main"
	}
	if p.IsExternal() {
		stable := registry.ProjectDir(root, projectName)
		if !isNormalCheckout(stable) {
			candidate := filepath.Join(mainRoot, config.ProjectsRoot, projectName)
			if isNormalCheckout(candidate) {
				stable = candidate
			}
		}
		if !isNormalCheckout(stable) {
			return worktree.Repo{}, fmt.Errorf("%s has no stable checkout; run `lasso setup --only=%s`", projectName, projectName)
		}
		return worktree.Repo{Project: projectName, Repo: stable, Stable: stable, DefaultBranch: branch}, nil
	}
	return worktree.Repo{Project: projectName, Repo: root, DefaultBranch: branch}, nil
}

func allWorktreeRepos(ctx context.Context, registry *config.Root, root string) []worktree.Repo {
	mainRoot := root
	if main, err := mainWorktreePath(ctx, root); err == nil && main != "" {
		mainRoot = main
	}
	repos := []worktree.Repo{{
		Project:       "lasso",
		Repo:          root,
		Stable:        mainRoot,
		DefaultBranch: "main",
	}}
	for _, name := range registry.Names() {
		p := registry.Projects[name]
		if !p.IsExternal() {
			continue
		}
		stable := registry.ProjectDir(root, name)
		if !isNormalCheckout(stable) {
			candidate := filepath.Join(mainRoot, config.ProjectsRoot, name)
			if isNormalCheckout(candidate) {
				stable = candidate
			}
		}
		if !isNormalCheckout(stable) {
			continue
		}
		branch := p.DefaultBranch
		if branch == "" {
			branch = "main"
		}
		repos = append(repos, worktree.Repo{
			Project:       name,
			Repo:          stable,
			Stable:        stable,
			DefaultBranch: branch,
		})
	}
	return repos
}

// mainWorktreePath returns the main checkout of a repository, even when root
// is a linked worktree. For linked worktrees, --git-common-dir points at the
// main repository's .git directory; for a main checkout it is <root>/.git.
func mainWorktreePath(ctx context.Context, repo string) (string, error) {
	common, err := gitx.Out(ctx, repo, "git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if strings.HasSuffix(common, "/.git") {
		return strings.TrimSuffix(common, "/.git"), nil
	}
	return "", fmt.Errorf("unexpected git-common-dir %q", common)
}
