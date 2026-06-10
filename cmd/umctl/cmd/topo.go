package cmd

import (
	"net/http"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/hint"
	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/registry"
)

var topoCmd = &cobra.Command{
	Use:     "topo",
	Short:   "Manage topology relations",
	Long:    "Write, expire, and delete topology relations. Read topology through query run/explain.",
	GroupID: "service",
}

var topoWriteCmd = &cobra.Command{
	Use:   "write [workspace] --file <file>",
	Short: "Write topology relations",
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		ws, rest := resolveWorkspace(cmd, args)
		requireWorkspace(ws)
		file, _ := cmd.Flags().GetString("file")
		if file == "" && len(rest) > 0 {
			file = rest[0]
		}
		requireFile(file)
		raw, err := readRawFile(file)
		if err != nil {
			hint.HandleInputError(err, file)
		}
		raw, err = wrapPayload(raw, "relations")
		if err != nil {
			hint.HandleInputError(err, file)
		}
		doRawRequest(http.MethodPost, "/api/v1/entitystore/"+ws+"/relations:write", raw,
			hint.ErrorContext{Command: "topo", SubCommand: "write"})
	},
}

var topoExpireCmd = &cobra.Command{
	Use:   "expire [workspace] --ids <ids>",
	Short: "Expire topology relations by IDs",
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		ws, rest := resolveWorkspace(cmd, args)
		requireWorkspace(ws)
		ids, _ := cmd.Flags().GetString("ids")
		if ids == "" && len(rest) > 0 {
			ids = strings.Join(rest, ",")
		}
		requireIDs(ids)
		payload := buildIDsPayload(ids, "expire requested by umctl")
		doRawRequest(http.MethodPost, "/api/v1/entitystore/"+ws+"/relations:expire", payload,
			hint.ErrorContext{Command: "topo", SubCommand: "expire"})
	},
}

var topoDeleteCmd = &cobra.Command{
	Use:   "delete [workspace] --ids <ids>",
	Short: "Delete topology relations by IDs",
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		ws, rest := resolveWorkspace(cmd, args)
		requireWorkspace(ws)
		ids, _ := cmd.Flags().GetString("ids")
		if ids == "" && len(rest) > 0 {
			ids = strings.Join(rest, ",")
		}
		requireIDs(ids)
		payload := buildIDsPayload(ids, "delete requested by umctl")
		doRawRequest(http.MethodPost, "/api/v1/entitystore/"+ws+"/relations:expire", payload,
			hint.ErrorContext{Command: "topo", SubCommand: "delete"})
	},
}

func init() {
	topoCmd.AddCommand(topoWriteCmd, topoExpireCmd, topoDeleteCmd)

	addWorkspaceFlag(topoWriteCmd, topoExpireCmd, topoDeleteCmd)
	topoWriteCmd.Flags().StringP("file", "f", "", "JSON file with relation payload")
	topoExpireCmd.Flags().String("ids", "", "Comma-separated relation IDs")
	topoDeleteCmd.Flags().String("ids", "", "Comma-separated relation IDs")

	registry.Global.Register(registry.CommandMeta{
		Group: "service", Resource: "topo", Action: "write",
		Description: "Write topology relations",
		Params: []registry.ParamMeta{
			{Name: "workspace", Type: "string", Flag: "--workspace", Short: "-w", Description: "Target workspace", Required: true},
			{Name: "file", Type: "string", Flag: "--file", Short: "-f", Description: "JSON file with relation payload", Required: true},
		},
	})
	registry.Global.Register(registry.CommandMeta{
		Group: "service", Resource: "topo", Action: "expire",
		Description: "Expire topology relations by IDs",
		Params: []registry.ParamMeta{
			{Name: "workspace", Type: "string", Flag: "--workspace", Short: "-w", Description: "Target workspace", Required: true},
			{Name: "ids", Type: "string", Flag: "--ids", Description: "Comma-separated relation IDs", Required: true},
		},
	})
	registry.Global.Register(registry.CommandMeta{
		Group: "service", Resource: "topo", Action: "delete",
		Description: "Delete topology relations by IDs",
		Params: []registry.ParamMeta{
			{Name: "workspace", Type: "string", Flag: "--workspace", Short: "-w", Description: "Target workspace", Required: true},
			{Name: "ids", Type: "string", Flag: "--ids", Description: "Comma-separated relation IDs", Required: true},
		},
	})
}
