package cmd

import (
	"net/http"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/hint"
	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/registry"
	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/response"
)

var queryCmd = &cobra.Command{
	Use:     "query",
	Short:   "Execute and explain SPL queries",
	Long:    "The unified read path for UModel, entities, and topology data.",
	GroupID: "service",
}

var queryRunCmd = &cobra.Command{
	Use:   "run [workspace] <spl>",
	Short: "Execute an SPL query",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ws, rest := resolveWorkspace(cmd, args)
		requireWorkspace(ws)
		if len(rest) == 0 {
			response.ExitWithError(response.ExitParam, "Missing SPL query", "Provide an SPL expression.")
		}
		spl := strings.Join(rest, " ")
		doRequest(http.MethodPost, "/api/v1/query/"+ws+"/execute", map[string]any{"query": spl},
			hint.ErrorContext{Command: "query", SubCommand: "run"})
	},
}

var queryExplainCmd = &cobra.Command{
	Use:   "explain [workspace] <spl>",
	Short: "Explain an SPL query plan",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ws, rest := resolveWorkspace(cmd, args)
		requireWorkspace(ws)
		if len(rest) == 0 {
			response.ExitWithError(response.ExitParam, "Missing SPL query", "Provide an SPL expression.")
		}
		spl := strings.Join(rest, " ")
		doRequest(http.MethodPost, "/api/v1/query/"+ws+"/explain", map[string]any{"query": spl},
			hint.ErrorContext{Command: "query", SubCommand: "explain"})
	},
}

var queryExamplesCmd = &cobra.Command{
	Use:   "examples",
	Short: "Show offline bootstrap SPL examples",
	Run: func(cmd *cobra.Command, args []string) {
		examples := []map[string]string{
			{"description": "List all UModel element kinds", "query": ".umodel with(kind='entity_set') | limit 20"},
			{"description": "List available entity-call methods", "query": ".entity_set with(domain='devops', name='devops.service') | entity-call __list_method__()"},
			{"description": "List data sets for an entity set", "query": ".entity_set with(domain='devops', name='devops.service') | entity-call list_data_set(['metric_set', 'log_set', 'event_set'], true)"},
			{"description": "Get logs for a specific entity", "query": ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_logs('devops', 'devops.log.service', query='level = \"ERROR\"')"},
			{"description": "Search entities by keyword", "query": ".entity with(domain='devops', name='devops.service', query='checkout') | limit 20"},
			{"description": "Get direct topology relations", "query": ".topo | graph-call getDirectRelations([(:'devops@devops.service' {__entity_id__: '10000000000000000000000000000101'})]) | limit 20"},
		}
		response.ExitWithSuccess(examples)
	},
}

func init() {
	queryCmd.AddCommand(queryRunCmd, queryExplainCmd, queryExamplesCmd)

	addWorkspaceFlag(queryRunCmd, queryExplainCmd, queryExamplesCmd)

	registry.Global.Register(registry.CommandMeta{
		Group: "service", Resource: "query", Action: "run",
		Description: "Execute an SPL query",
		Params: []registry.ParamMeta{
			{Name: "workspace", Type: "string", Flag: "--workspace", Short: "-w", Description: "Target workspace", Required: true},
			{Name: "spl", Type: "string", Description: "SPL query expression", Required: true},
		},
	})
	registry.Global.Register(registry.CommandMeta{
		Group: "service", Resource: "query", Action: "explain",
		Description: "Explain an SPL query plan",
		Params: []registry.ParamMeta{
			{Name: "workspace", Type: "string", Flag: "--workspace", Short: "-w", Description: "Target workspace", Required: true},
			{Name: "spl", Type: "string", Description: "SPL query expression", Required: true},
		},
	})
	registry.Global.Register(registry.CommandMeta{
		Group: "service", Resource: "query", Action: "examples",
		Description: "Show offline bootstrap SPL examples",
	})
}
