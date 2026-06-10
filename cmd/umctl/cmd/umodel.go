package cmd

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/hint"
	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/registry"
	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/response"
)

var umodelCmd = &cobra.Command{
	Use:     "umodel",
	Short:   "Manage UModel elements",
	Long:    "Import, validate, put, delete, and export UModel elements. Read UModel data through query run/explain.",
	GroupID: "service",
}

var umodelImportCmd = &cobra.Command{
	Use:   "import [workspace] --path <path>",
	Short: "Import UModel from a YAML/JSON file or directory",
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		ws, rest := resolveWorkspace(cmd, args)
		requireWorkspace(ws)
		path, _ := cmd.Flags().GetString("path")
		if path == "" && len(rest) > 0 {
			path = rest[0]
		}
		if path == "" {
			response.ExitWithError(response.ExitParam, "Missing --path flag", "Provide a path to YAML/JSON files or directory.")
		}
		doRequest(http.MethodPost, "/api/v1/umodel/"+ws+"/import", map[string]any{"path": path},
			hint.ErrorContext{Command: "umodel", SubCommand: "import"})
	},
}

var umodelValidateCmd = &cobra.Command{
	Use:   "validate [workspace] --file <file>",
	Short: "Validate UModel elements",
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
		raw, err = wrapPayload(raw, "elements")
		if err != nil {
			hint.HandleInputError(err, file)
		}
		doRawRequest(http.MethodPost, "/api/v1/umodel/"+ws+"/validate", raw,
			hint.ErrorContext{Command: "umodel", SubCommand: "validate"})
	},
}

var umodelPutCmd = &cobra.Command{
	Use:   "put [workspace] --file <file>",
	Short: "Put UModel elements",
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
		raw, err = wrapPayload(raw, "elements")
		if err != nil {
			hint.HandleInputError(err, file)
		}
		doRawRequest(http.MethodPost, "/api/v1/umodel/"+ws+"/elements", raw,
			hint.ErrorContext{Command: "umodel", SubCommand: "put"})
	},
}

var umodelDeleteCmd = &cobra.Command{
	Use:   "delete [workspace] --ids <ids>",
	Short: "Delete UModel elements by IDs",
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		ws, rest := resolveWorkspace(cmd, args)
		requireWorkspace(ws)
		ids, _ := cmd.Flags().GetString("ids")
		if ids == "" && len(rest) > 0 {
			ids = strings.Join(rest, ",")
		}
		if ids == "" {
			response.ExitWithError(response.ExitParam, "Missing --ids flag", "Provide comma-separated element IDs.")
		}
		payload := buildIDsPayload(ids, "")
		doRawRequest(http.MethodDelete, "/api/v1/umodel/"+ws+"/elements", payload,
			hint.ErrorContext{Command: "umodel", SubCommand: "delete"})
	},
}

var umodelExportCmd = &cobra.Command{
	Use:   "export [workspace] [--limit N]",
	Short: "Export UModel elements via query",
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		ws, rest := resolveWorkspace(cmd, args)
		requireWorkspace(ws)
		limit, _ := cmd.Flags().GetInt("limit")
		if len(rest) > 0 {
			if v, err := strconv.Atoi(rest[0]); err == nil {
				limit = v
			}
		}
		doRequest(http.MethodPost, "/api/v1/query/"+ws+"/execute", map[string]any{
			"query": fmt.Sprintf(".umodel | limit %d", limit),
			"limit": limit,
		}, hint.ErrorContext{Command: "umodel", SubCommand: "export"})
	},
}

func init() {
	umodelCmd.AddCommand(umodelImportCmd, umodelValidateCmd, umodelPutCmd, umodelDeleteCmd, umodelExportCmd)

	addWorkspaceFlag(umodelImportCmd, umodelValidateCmd, umodelPutCmd, umodelDeleteCmd, umodelExportCmd)

	umodelImportCmd.Flags().String("path", "", "Path to YAML/JSON file or directory")
	umodelValidateCmd.Flags().StringP("file", "f", "", "JSON file with UModel elements")
	umodelPutCmd.Flags().StringP("file", "f", "", "JSON file with UModel elements")
	umodelDeleteCmd.Flags().String("ids", "", "Comma-separated element IDs")
	umodelExportCmd.Flags().Int("limit", 1000, "Maximum number of elements to export")

	registry.Global.Register(registry.CommandMeta{
		Group: "service", Resource: "umodel", Action: "import",
		Description: "Import UModel from YAML/JSON file or directory",
		Params: []registry.ParamMeta{
			{Name: "workspace", Type: "string", Flag: "--workspace", Short: "-w", Description: "Target workspace", Required: true},
			{Name: "path", Type: "string", Flag: "--path", Description: "Path to YAML/JSON files or directory", Required: true},
		},
	})
	registry.Global.Register(registry.CommandMeta{
		Group: "service", Resource: "umodel", Action: "validate",
		Description: "Validate UModel elements",
		Params: []registry.ParamMeta{
			{Name: "workspace", Type: "string", Flag: "--workspace", Short: "-w", Description: "Target workspace", Required: true},
			{Name: "file", Type: "string", Flag: "--file", Short: "-f", Description: "JSON file with elements", Required: true},
		},
	})
	registry.Global.Register(registry.CommandMeta{
		Group: "service", Resource: "umodel", Action: "put",
		Description: "Put UModel elements",
		Params: []registry.ParamMeta{
			{Name: "workspace", Type: "string", Flag: "--workspace", Short: "-w", Description: "Target workspace", Required: true},
			{Name: "file", Type: "string", Flag: "--file", Short: "-f", Description: "JSON file with elements", Required: true},
		},
	})
	registry.Global.Register(registry.CommandMeta{
		Group: "service", Resource: "umodel", Action: "delete",
		Description: "Delete UModel elements by IDs",
		Params: []registry.ParamMeta{
			{Name: "workspace", Type: "string", Flag: "--workspace", Short: "-w", Description: "Target workspace", Required: true},
			{Name: "ids", Type: "string", Flag: "--ids", Description: "Comma-separated element IDs", Required: true},
		},
	})
	registry.Global.Register(registry.CommandMeta{
		Group: "service", Resource: "umodel", Action: "export",
		Description: "Export UModel elements via query",
		Params: []registry.ParamMeta{
			{Name: "workspace", Type: "string", Flag: "--workspace", Short: "-w", Description: "Target workspace", Required: true},
			{Name: "limit", Type: "int", Flag: "--limit", Description: "Maximum number of elements", Default: "1000"},
		},
	})
}

func buildIDsPayload(ids, reason string) []byte {
	idList := strings.Split(ids, ",")
	cleaned := make([]string, 0, len(idList))
	for _, id := range idList {
		id = strings.TrimSpace(id)
		if id != "" {
			cleaned = append(cleaned, id)
		}
	}
	payload := map[string]any{"ids": cleaned}
	if reason != "" {
		payload["reason"] = reason
	}
	b, _ := marshalJSONBytes(payload)
	return b
}

func requireWorkspace(ws string) {
	if ws == "" {
		response.ExitWithError(response.ExitParam, "Missing --workspace / -w flag", "Specify the target workspace.")
	}
}

func requireFile(file string) {
	if file == "" {
		response.ExitWithError(response.ExitParam, "Missing --file / -f flag", "Provide a JSON file path.")
	}
}

func addWorkspaceFlag(cmds ...*cobra.Command) {
	for _, cmd := range cmds {
		cmd.Flags().StringP("workspace", "w", "", "Target workspace")
	}
}
