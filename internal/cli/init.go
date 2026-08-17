package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dravengarden/lasso/internal/kit"
	"github.com/dravengarden/lasso/internal/modules"
)

func newInitCmd() *cobra.Command {
	var (
		name     string
		runtimes []string
		mods     []string
		format   string
	)
	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Create a new agent-native monorepo workspace",
		Long: `init scaffolds a Lasso workspace from the product template.

It writes project-defs/, work-items/, AGENTS.md, runtime adapters, the core
plugin, lasso.toml, and lasso.lock.toml. Optional modules can be installed in
the same step with --module.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			kitRoot, err := kit.Root()
			if err != nil {
				return err
			}
			path := "."
			if len(args) == 1 {
				path = args[0]
			}
			if strings.TrimSpace(name) == "" {
				base := filepath.Base(path)
				if base == "." || base == string(filepath.Separator) {
					cwd, err := os.Getwd()
					if err != nil {
						return err
					}
					base = filepath.Base(cwd)
				}
				name = base
			}
			res, err := modules.Init(kitRoot, modules.InitOptions{
				Name:     name,
				Path:     path,
				Runtimes: runtimes,
				Modules:  mods,
			})
			if err != nil {
				return err
			}
			if format == "json" {
				return json.NewEncoder(os.Stdout).Encode(res)
			}
			fmt.Printf("created workspace %s at %s\n", res.Name, res.Path)
			fmt.Printf("runtimes: %s\n", strings.Join(res.Runtimes, ", "))
			if len(res.InstalledModules) > 0 {
				fmt.Printf("modules:  %s\n", strings.Join(res.InstalledModules, ", "))
			}
			fmt.Println("next:")
			fmt.Println("  cd", res.Path)
			fmt.Println("  lasso project add --project=<name> --repo-url=<url>")
			fmt.Println("  codex plugin marketplace add . && codex plugin add lasso-core@lasso")
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "workspace name (default: directory basename)")
	cmd.Flags().StringSliceVar(&runtimes, "runtime", []string{"codex"}, "agent runtimes: codex, claude, grok")
	cmd.Flags().StringSliceVar(&mods, "module", nil, "module ids to install (repeatable)")
	cmd.Flags().StringVar(&format, "format", "text", "text|json")
	return cmd
}
