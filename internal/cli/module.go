package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/dravengarden/lasso/internal/config"
	"github.com/dravengarden/lasso/internal/kit"
	"github.com/dravengarden/lasso/internal/modules"
)

func newModuleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "module",
		Short: "List, add, or remove optional Lasso modules",
	}
	cmd.AddCommand(newModuleListCmd(), newModuleAddCmd(), newModuleRemoveCmd())
	return cmd
}

func newModuleListCmd() *cobra.Command {
	var (
		format    string
		installed bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available kit modules or modules installed in this workspace",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			kitRoot, err := kit.Root()
			if err != nil {
				return err
			}
			if installed {
				root, err := config.WorkspaceRoot()
				if err != nil {
					return err
				}
				lock, err := modules.LoadLock(root)
				if err != nil {
					return err
				}
				if format == "json" {
					return json.NewEncoder(os.Stdout).Encode(lock)
				}
				w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
				fmt.Fprintln(w, "ID\tVERSION")
				for id, ver := range lock.Modules {
					fmt.Fprintf(w, "%s\t%s\n", id, ver)
				}
				return w.Flush()
			}
			cat, err := modules.LoadCatalog(kitRoot)
			if err != nil {
				return err
			}
			if format == "json" {
				return json.NewEncoder(os.Stdout).Encode(cat.Modules)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tVERSION\tKIND\tDEFAULT\tDESCRIPTION")
			for _, m := range cat.Modules {
				fmt.Fprintf(w, "%s\t%s\t%s\t%t\t%s\n", m.ID, m.Version, m.Kind, m.Default, m.Description)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "table|json")
	cmd.Flags().BoolVar(&installed, "installed", false, "list modules installed in the current workspace")
	return cmd
}

func newModuleAddCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "add <id>",
		Short: "Install a module into the current workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			kitRoot, err := kit.Root()
			if err != nil {
				return err
			}
			root, err := config.WorkspaceRoot()
			if err != nil {
				return err
			}
			res, err := modules.Add(kitRoot, root, args[0])
			if err != nil {
				return err
			}
			if format == "json" {
				return json.NewEncoder(os.Stdout).Encode(res)
			}
			fmt.Printf("%s module %s@%s -> %s\n", res.State, res.ID, res.Version, res.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "text", "text|json")
	return cmd
}

func newModuleRemoveCmd() *cobra.Command {
	var (
		format string
		yes    bool
	)
	cmd := &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove an installed module from the current workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return fmt.Errorf("refusing to remove module without --yes")
			}
			kitRoot, err := kit.Root()
			if err != nil {
				return err
			}
			root, err := config.WorkspaceRoot()
			if err != nil {
				return err
			}
			res, err := modules.Remove(kitRoot, root, args[0])
			if err != nil {
				return err
			}
			if format == "json" {
				return json.NewEncoder(os.Stdout).Encode(res)
			}
			fmt.Printf("%s module %s\n", res.State, res.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "text", "text|json")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm removal")
	return cmd
}
