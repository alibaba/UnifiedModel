package cmd

import (
	"net/http"

	"github.com/spf13/cobra"

	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/hint"
	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/registry"
	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/response"
)

var agentCmd = &cobra.Command{
	Use:     "agent",
	Short:   "Interact with the Agent Gateway",
	Long:    "Discover agent tools and resources, execute tools, and read resources.",
	GroupID: "service",
}

var agentDiscoverCmd = &cobra.Command{
	Use:   "discover [workspace]",
	Short: "Discover available agent tools and resources",
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		ws, _ := resolveWorkspace(cmd, args)
		requireWorkspace(ws)
		doRequest(http.MethodGet, "/api/v1/agent/"+ws+"/discover", nil,
			hint.ErrorContext{Command: "agent", SubCommand: "discover"})
	},
}

var agentToolCmd = &cobra.Command{
	Use:   "tool [workspace] <name> [json-args]",
	Short: "Execute an agent tool",
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		ws, rest := resolveWorkspace(cmd, args)
		requireWorkspace(ws)
		if len(rest) == 0 {
			response.ExitWithError(response.ExitParam, "Missing tool name", "Provide the name of the agent tool to execute.")
		}
		toolName := rest[0]
		payload := map[string]any{"name": toolName}

		argsJSON, _ := cmd.Flags().GetString("args")
		if argsJSON == "" && len(rest) >= 2 {
			argsJSON = rest[1]
		}
		if argsJSON != "" {
			arguments, err := parseJSONObject(argsJSON)
			if err != nil {
				response.ExitWithError(response.ExitInputError, "Invalid tool arguments JSON: "+err.Error(), "Provide valid JSON object for tool arguments.")
			}
			payload["arguments"] = arguments
		}
		doRequest(http.MethodPost, "/api/v1/agent/"+ws+"/tools:execute", payload,
			hint.ErrorContext{Command: "agent", SubCommand: "tool"})
	},
}

var agentResourceCmd = &cobra.Command{
	Use:   "resource [workspace] <uri>",
	Short: "Read an agent resource",
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		ws, rest := resolveWorkspace(cmd, args)
		requireWorkspace(ws)
		if len(rest) == 0 {
			response.ExitWithError(response.ExitParam, "Missing resource URI", "Provide the URI of the agent resource to read.")
		}
		doRequest(http.MethodPost, "/api/v1/agent/"+ws+"/resources:read", map[string]any{"uri": rest[0]},
			hint.ErrorContext{Command: "agent", SubCommand: "resource"})
	},
}

func init() {
	agentCmd.AddCommand(agentDiscoverCmd, agentToolCmd, agentResourceCmd)

	addWorkspaceFlag(agentDiscoverCmd, agentToolCmd, agentResourceCmd)
	agentToolCmd.Flags().String("args", "", "JSON object with tool arguments")

	registry.Global.Register(registry.CommandMeta{
		Group: "service", Resource: "agent", Action: "discover",
		Description: "Discover available agent tools and resources",
		Params: []registry.ParamMeta{
			{Name: "workspace", Type: "string", Flag: "--workspace", Short: "-w", Description: "Target workspace", Required: true},
		},
	})
	registry.Global.Register(registry.CommandMeta{
		Group: "service", Resource: "agent", Action: "tool",
		Description: "Execute an agent tool",
		Params: []registry.ParamMeta{
			{Name: "workspace", Type: "string", Flag: "--workspace", Short: "-w", Description: "Target workspace", Required: true},
			{Name: "name", Type: "string", Description: "Tool name", Required: true},
			{Name: "args", Type: "string", Flag: "--args", Description: "JSON object with tool arguments"},
		},
	})
	registry.Global.Register(registry.CommandMeta{
		Group: "service", Resource: "agent", Action: "resource",
		Description: "Read an agent resource",
		Params: []registry.ParamMeta{
			{Name: "workspace", Type: "string", Flag: "--workspace", Short: "-w", Description: "Target workspace", Required: true},
			{Name: "uri", Type: "string", Description: "Resource URI", Required: true},
		},
	})
}
