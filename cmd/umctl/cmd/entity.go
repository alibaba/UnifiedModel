package cmd

import (
	"strings"

	"net/http"

	"github.com/spf13/cobra"

	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/hint"
	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/registry"
	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/response"
)

var entityCmd = &cobra.Command{
	Use:     "entity",
	Short:   "Manage runtime entities",
	Long:    "Write, expire, and delete entities. Read entities through query run/explain.",
	GroupID: "service",
}

var entityWriteCmd = &cobra.Command{
	Use:   "write [workspace] --file <file>",
	Short: "Write entities",
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
		raw, err = wrapPayload(raw, "entities")
		if err != nil {
			hint.HandleInputError(err, file)
		}
		doRawRequest(http.MethodPost, "/api/v1/entitystore/"+ws+"/entities:write", raw,
			hint.ErrorContext{Command: "entity", SubCommand: "write"})
	},
}

var entityExpireCmd = &cobra.Command{
	Use:   "expire [workspace] --ids <ids>",
	Short: "Expire entities by IDs",
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
		doRawRequest(http.MethodPost, "/api/v1/entitystore/"+ws+"/entities:expire", payload,
			hint.ErrorContext{Command: "entity", SubCommand: "expire"})
	},
}

var entityDeleteCmd = &cobra.Command{
	Use:   "delete [workspace] --ids <ids>",
	Short: "Delete entities by IDs",
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
		doRawRequest(http.MethodPost, "/api/v1/entitystore/"+ws+"/entities:expire", payload,
			hint.ErrorContext{Command: "entity", SubCommand: "delete"})
	},
}

func init() {
	entityCmd.AddCommand(entityWriteCmd, entityExpireCmd, entityDeleteCmd)

	addWorkspaceFlag(entityWriteCmd, entityExpireCmd, entityDeleteCmd)
	entityWriteCmd.Flags().StringP("file", "f", "", "JSON file with entity payload")
	entityExpireCmd.Flags().String("ids", "", "Comma-separated entity IDs")
	entityDeleteCmd.Flags().String("ids", "", "Comma-separated entity IDs")

	registry.Global.Register(registry.CommandMeta{
		Group: "service", Resource: "entity", Action: "write",
		Description: "Write entities",
		Params: []registry.ParamMeta{
			{Name: "workspace", Type: "string", Flag: "--workspace", Short: "-w", Description: "Target workspace", Required: true},
			{Name: "file", Type: "string", Flag: "--file", Short: "-f", Description: "JSON file with entity payload", Required: true},
		},
	})
	registry.Global.Register(registry.CommandMeta{
		Group: "service", Resource: "entity", Action: "expire",
		Description: "Expire entities by IDs",
		Params: []registry.ParamMeta{
			{Name: "workspace", Type: "string", Flag: "--workspace", Short: "-w", Description: "Target workspace", Required: true},
			{Name: "ids", Type: "string", Flag: "--ids", Description: "Comma-separated entity IDs", Required: true},
		},
	})
	registry.Global.Register(registry.CommandMeta{
		Group: "service", Resource: "entity", Action: "delete",
		Description: "Delete entities by IDs",
		Params: []registry.ParamMeta{
			{Name: "workspace", Type: "string", Flag: "--workspace", Short: "-w", Description: "Target workspace", Required: true},
			{Name: "ids", Type: "string", Flag: "--ids", Description: "Comma-separated entity IDs", Required: true},
		},
	})
}

func requireIDs(ids string) {
	if ids == "" {
		response.ExitWithError(response.ExitParam, "Missing --ids flag", "Provide comma-separated IDs.")
	}
}
