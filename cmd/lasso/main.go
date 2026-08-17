// Command harness is the CLI entry point for the lasso harness monorepo.
// It dispatches to the thin workspace subcommands defined in internal/cli,
// including project, setup, version, work-item, and worktree operations.
package main

import (
	"fmt"
	"os"

	"github.com/dravengarden/lasso/internal/cli"
)

func main() {
	if err := cli.NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr)
		os.Exit(1)
	}
}
