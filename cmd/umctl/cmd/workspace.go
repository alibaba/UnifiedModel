package cmd

import (
	"net/http"

	"github.com/spf13/cobra"

	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/hint"
	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/registry"
	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/response"
)

var workspaceCmd = &cobra.Command{
	Use:     "workspace",
	Short:   "Manage workspaces",
	Long:    "Create, get, list, update, and delete workspaces. Workspaces are metadata-only containers.",
	GroupID: "service",
}

var workspaceCreateCmd = &cobra.Command{
	Use:   "create <id> [json-file-or-inline]",
	Short: "Create a new workspace",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		file, _ := cmd.Flags().GetString("file")
		if file == "" && len(args) >= 2 {
			file = args[1]
		}
		var payload any
		if file != "" {
			raw, err := readRawFile(file)
			if err != nil {
				hint.HandleInputError(err, file)
			}
			obj, err := parseJSONObject(string(raw))
			if err != nil {
				hint.HandleInputError(err, file)
			}
			if _, ok := obj["id"]; !ok {
				obj["id"] = args[0]
			}
			payload = obj
		} else {
			payload = map[string]any{"id": args[0]}
		}
		doRequest(http.MethodPost, "/api/v1/workspaces", payload, hint.ErrorContext{Command: "workspace", SubCommand: "create"})
	},
}

var workspaceGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get workspace details",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		doRequest(http.MethodGet, "/api/v1/workspaces/"+args[0], nil, hint.ErrorContext{Command: "workspace", SubCommand: "get"})
	},
}

var workspaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all workspaces",
	Run: func(cmd *cobra.Command, args []string) {
		doRequest(http.MethodGet, "/api/v1/workspaces", nil, hint.ErrorContext{Command: "workspace", SubCommand: "list"})
	},
}

var workspaceUpdateCmd = &cobra.Command{
	Use:   "update <id> [json-file-or-inline]",
	Short: "Update a workspace",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		file, _ := cmd.Flags().GetString("file")
		if file == "" && len(args) >= 2 {
			file = args[1]
		}
		if file == "" {
			response.ExitWithError(response.ExitParam, "Missing --file flag or inline JSON argument", "Provide a JSON file with workspace update payload.")
		}
		raw, err := readRawFile(file)
		if err != nil {
			hint.HandleInputError(err, file)
		}
		doRawRequest(http.MethodPut, "/api/v1/workspaces/"+args[0], raw, hint.ErrorContext{Command: "workspace", SubCommand: "update"})
	},
}

var workspaceDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a workspace",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		doRequest(http.MethodDelete, "/api/v1/workspaces/"+args[0], nil, hint.ErrorContext{Command: "workspace", SubCommand: "delete"})
	},
}

func init() {
	workspaceCmd.AddCommand(workspaceCreateCmd, workspaceGetCmd, workspaceListCmd, workspaceUpdateCmd, workspaceDeleteCmd)

	workspaceCreateCmd.Flags().StringP("file", "f", "", "JSON file with workspace payload")
	workspaceUpdateCmd.Flags().StringP("file", "f", "", "JSON file with workspace update payload")

	registry.Global.Register(registry.CommandMeta{
		Group: "service", Resource: "workspace", Action: "create",
		Description: "Create a new workspace",
		Params: []registry.ParamMeta{
			{Name: "id", Type: "string", Description: "Workspace ID", Required: true},
			{Name: "file", Type: "string", Flag: "--file", Short: "-f", Description: "JSON file with workspace payload"},
		},
	})
	registry.Global.Register(registry.CommandMeta{
		Group: "service", Resource: "workspace", Action: "get",
		Description: "Get workspace details",
		Params:      []registry.ParamMeta{{Name: "id", Type: "string", Description: "Workspace ID", Required: true}},
	})
	registry.Global.Register(registry.CommandMeta{
		Group: "service", Resource: "workspace", Action: "list",
		Description: "List all workspaces",
	})
	registry.Global.Register(registry.CommandMeta{
		Group: "service", Resource: "workspace", Action: "update",
		Description: "Update a workspace",
		Params: []registry.ParamMeta{
			{Name: "id", Type: "string", Description: "Workspace ID", Required: true},
			{Name: "file", Type: "string", Flag: "--file", Short: "-f", Description: "JSON file with update payload", Required: true},
		},
	})
	registry.Global.Register(registry.CommandMeta{
		Group: "service", Resource: "workspace", Action: "delete",
		Description: "Delete a workspace",
		Params:      []registry.ParamMeta{{Name: "id", Type: "string", Description: "Workspace ID", Required: true}},
	})
}
