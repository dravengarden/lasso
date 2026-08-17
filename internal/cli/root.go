// Package cli wires together the cobra command tree for `lasso`.
package cli

import (
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dravengarden/lasso/internal/config"
)

const APIVersion = 2

// Version and BuildRevision are set via -ldflags. Defaults identify a local
// development build without pretending it matches a deployed closure.
var (
	Version       = "dev"
	BuildRevision = "unknown"
)

// NewRoot builds and returns the root command. Each subcommand lives in
// its own file. Codex owns live execution state; Lasso retains project
// identity, stable checkout bootstrap, state-free worktree lifecycle, and
// durable coordination.
func NewRoot() *cobra.Command {
	var workspaceRoot string
	root := &cobra.Command{
		Use:   "lasso",
		Short: "Lasso monorepo CLI",
		Long: `lasso — agent-native monorepo workspace CLI.

Manage project identity, stable checkouts, state-free worktrees, durable work
items, and optional modules. The workspace root is identified by
project-defs/registry.toml. Resolution uses --root, then LASSO_ROOT, then walks
up from the current directory.`,
		Version:           Version,
		SilenceUsage:      true,
		SilenceErrors:     false,
		DisableAutoGenTag: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if strings.TrimSpace(workspaceRoot) == "" {
				return nil
			}
			return os.Setenv(config.RootEnv, workspaceRoot)
		},
	}
	root.PersistentFlags().StringVar(&workspaceRoot, "root", "", "workspace root (overrides LASSO_ROOT and cwd discovery)")
	root.AddCommand(
		newVersionCmd(),
		newInitCmd(),
		newSetupCmd(),
		newProjectCmd(),
		newModuleCmd(),
		newWorkItemCmd(),
		newWorktreeCmd(),
	)

	return root
}
