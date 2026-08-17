package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dravengarden/lasso/internal/cliformat"
	"github.com/dravengarden/lasso/internal/config"
	"github.com/dravengarden/lasso/internal/ui"
	"github.com/dravengarden/lasso/internal/workitems"
)

func newWorkItemCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "work-item",
		Aliases: []string{"wi"},
		Short:   "Track exceptional repository-auditable coordination",
		Long: `A work item is not an execution state machine. The active agent runtime
owns the current task, goal, plan, and review; Codex is the Lasso baseline,
and Claude follower state remains Claude-local. Create a work item only when a
blocker, decision, migration, runbook, or handoff needs a durable Git artifact
independently of the current task. Multiple sessions or projects alone are not enough.`,
	}
	cmd.AddCommand(
		workItemNewCmd(),
		workItemListCmd(),
		workItemShowCmd(),
		workItemPathCmd(),
		workItemValidateCmd(),
		workItemRemoveCmd(),
	)

	return cmd
}

func workItemNewCmd() *cobra.Command {
	var (
		id       string
		title    string
		projects []string
		recipe   string
	)
	c := &cobra.Command{
		Use:   "new",
		Short: "Create minimal durable metadata without allocating a checkout",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			registry, root, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			if len(projects) == 0 {
				project, inferErr := inferCurrentProject(cmd.Context(), registry, root)
				if inferErr != nil {
					return inferErr
				}
				if project != "" {
					projects = []string{project}
				}
			}
			for _, project := range projects {
				if _, err := registry.MustGet(project); err != nil {
					return err
				}
			}

			path, err := workitems.NewStore(root).Create(workitems.Item{
				ID:       id,
				Title:    title,
				Projects: projects,
				Recipe:   recipe,
			})
			if err != nil {
				return err
			}
			ui.OKf("created %s", path)
			if len(projects) > 0 {
				ui.Dimf("project scope: %s", strings.Join(projects, ", "))
			}
			ui.Dimf("coordination repository: %s", root)
			ui.Dimf("add only the durable artifact selected by the work-item skill; the active agent runtime owns the live task and worktree")
			return nil
		},
	}
	c.Flags().StringVar(&id, "id", "", "stable work item id (required)")
	c.Flags().StringVar(&title, "title", "", "one-line outcome (required)")
	c.Flags().StringArrayVar(&projects, "project", nil, "registered project in scope (repeatable)")
	c.Flags().StringVar(&recipe, "recipe", "", "open-vocabulary workflow hint such as bug, design, or migration")
	_ = c.MarkFlagRequired("id")
	_ = c.MarkFlagRequired("title")

	return c
}

type workItemListEntry struct {
	ID       string   `json:"id" yaml:"id"`
	Title    string   `json:"title" yaml:"title"`
	Projects []string `json:"projects,omitempty" yaml:"projects,omitempty"`
	Recipe   string   `json:"recipe,omitempty" yaml:"recipe,omitempty"`
	Blocked  bool     `json:"blocked" yaml:"blocked"`
}

func workItemListCmd() *cobra.Command {
	var (
		format  cliformat.Format
		current bool
	)
	c := &cobra.Command{
		Use:   "list",
		Short: "List active durable work items",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			registry, root, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			items, err := workitems.NewStore(root).ListActive()
			if err != nil {
				return err
			}
			if current {
				project, inferErr := inferCurrentProject(cmd.Context(), registry, root)
				if inferErr != nil {
					return inferErr
				}
				if project == "" {
					return fmt.Errorf("current directory is the Lasso workspace, not a registered project")
				}
				items = filterWorkItemsByProject(items, project)
			}
			entries := make([]workItemListEntry, 0, len(items))
			for _, item := range items {
				entries = append(entries, workItemListEntry{
					ID: item.ID, Title: item.Title, Projects: item.Projects,
					Recipe:  item.Recipe,
					Blocked: item.BlockedReason != "",
				})
			}
			if format.IsStructured() {
				return cliformat.Print(cmd.OutOrStdout(), format, entries)
			}
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no active work items)")
				return nil
			}
			table := ui.Table("Active work items")
			table.AppendHeader(rowOf("id", "recipe", "projects", "blocked"))
			for _, entry := range entries {
				table.AppendRow(rowOf(entry.ID, entry.Recipe, strings.Join(entry.Projects, ", "), entry.Blocked))
			}
			table.Render()
			return nil
		},
	}
	cliformat.Bind(c, &format)
	c.Flags().BoolVar(&current, "current", false, "show only work items involving the current registered project")

	return c
}

func filterWorkItemsByProject(items []*workitems.Item, project string) []*workitems.Item {
	filtered := make([]*workitems.Item, 0, len(items))
	for _, item := range items {
		if slices.Contains(item.Projects, project) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func workItemShowCmd() *cobra.Command {
	var format cliformat.Format
	c := &cobra.Command{
		Use:   "show <id>",
		Short: "Show one work item's durable metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, root, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			item, err := workitems.NewStore(root).Load(args[0])
			if err != nil {
				return err
			}
			if format == cliformat.FormatTable {
				format = cliformat.FormatYAML
			}
			return cliformat.Print(cmd.OutOrStdout(), format, item)
		},
	}
	cliformat.Bind(c, &format)

	return c
}

func workItemPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path <id>",
		Short: "Print the active work item directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, root, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			store := workitems.NewStore(root)
			if _, err := store.Load(args[0]); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), store.ActivePath(args[0]))
			return nil
		},
	}
}

func workItemValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate active work item metadata and project references",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			registry, root, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			issues, err := workitems.NewStore(root).Validate(registry.Projects)
			if err != nil {
				return err
			}
			if len(issues) == 0 {
				ui.OKf("clean")
				return nil
			}
			for _, issue := range issues {
				ui.Errf("%s", issue)
			}
			return fmt.Errorf("%w: %d issue(s)", ErrValidationIssues, len(issues))
		},
	}
}

func workItemRemoveCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:   "rm <id>",
		Short: "Permanently remove one active work item; never touches Git",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("permanent removal requires --yes: %w", ErrAborted)
			}
			_, root, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			store := workitems.NewStore(root)
			path := store.ActivePath(args[0])
			if err := store.RemoveActive(args[0]); err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				rel = path
			}
			fmt.Fprintf(os.Stderr, "removed %s\n", rel)
			return nil
		},
	}
	c.Flags().BoolVar(&yes, "yes", false, "confirm permanent deletion")

	return c
}
