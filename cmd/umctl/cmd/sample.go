package cmd

import (
	"net/http"

	"github.com/spf13/cobra"

	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/hint"
	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/registry"
	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/response"
)

var sampleCmd = &cobra.Command{
	Use:     "sample",
	Short:   "Manage sample data",
	GroupID: "service",
}

var sampleImportCmd = &cobra.Command{
	Use:   "import [workspace] <name>",
	Short: "Import sample data into a workspace",
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		ws, rest := resolveWorkspace(cmd, args)
		requireWorkspace(ws)
		if len(rest) == 0 {
			response.ExitWithError(response.ExitParam, "Missing sample name", "Provide the name of the sample to import.")
		}
		doRequest(http.MethodPost, "/api/v1/samples/"+ws+"/"+rest[0]+":import", nil,
			hint.ErrorContext{Command: "sample", SubCommand: "import"})
	},
}

func init() {
	sampleCmd.AddCommand(sampleImportCmd)
	addWorkspaceFlag(sampleImportCmd)

	registry.Global.Register(registry.CommandMeta{
		Group: "service", Resource: "sample", Action: "import",
		Description: "Import sample data into a workspace",
		Params: []registry.ParamMeta{
			{Name: "workspace", Type: "string", Flag: "--workspace", Short: "-w", Description: "Target workspace", Required: true},
			{Name: "name", Type: "string", Description: "Sample name", Required: true},
		},
	})
}
