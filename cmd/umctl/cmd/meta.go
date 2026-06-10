package cmd

import (
	"github.com/spf13/cobra"

	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/registry"
	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/response"
)

var metaCmd = &cobra.Command{
	Use:     "meta",
	Short:   "CLI metadata operations",
	GroupID: "config",
}

var metaExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export all command metadata as JSON for AI agent discovery",
	Run: func(cmd *cobra.Command, args []string) {
		response.ExitWithSuccess(registry.Global.All())
	},
}

func init() {
	metaCmd.AddCommand(metaExportCmd)
}
