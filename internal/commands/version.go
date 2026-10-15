package commands

import (
	"encoding/json"
	"fmt"

	"github.com/rierek/lif/internal/version"
	"github.com/spf13/cobra"
)

var versionJSON bool

var versionCmd = &cobra.Command{
	Use:   "version",
	Args:  cobra.NoArgs,
	Short: "Show version information",
	RunE: func(cmd *cobra.Command, args []string) error {
		if versionJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(version.JSON())
		}
		fmt.Fprintln(cmd.OutOrStdout(), version.String())
		return nil
	},
}

func RegisterVersionCmd(root *cobra.Command) {
	versionCmd.Flags().BoolVar(&versionJSON, "json", false, "Output as JSON")
	root.AddCommand(versionCmd)
}
