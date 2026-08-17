package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dravengarden/lasso/internal/cliformat"
)

type versionInfo struct {
	Version       string `json:"version" yaml:"version"`
	APIVersion    int    `json:"api_version" yaml:"api_version"`
	BuildRevision string `json:"build_revision" yaml:"build_revision"`
}

func newVersionCmd() *cobra.Command {
	var format cliformat.Format
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print CLI, API, and source revision versions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			info := versionInfo{
				Version:       Version,
				APIVersion:    APIVersion,
				BuildRevision: BuildRevision,
			}
			if format.IsStructured() {
				return cliformat.Print(cmd.OutOrStdout(), format, info)
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "lasso %s (API %d, revision %s)\n",
				info.Version, info.APIVersion, info.BuildRevision)
			return err
		},
	}
	cliformat.Bind(cmd, &format)
	return cmd
}
