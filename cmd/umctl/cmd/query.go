package cmd

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/hint"
	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/registry"
	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/response"
	"github.com/alibaba/UnifiedModel/pkg/model"
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
		body := map[string]any{"query": spl}
		from, _ := cmd.Flags().GetString("from")
		to, _ := cmd.Flags().GetString("to")
		if from != "" || to != "" {
			rangeBody := map[string]any{}
			for key, value := range map[string]string{"from": from, "to": to} {
				if value == "" {
					continue
				}
				if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
					response.ExitWithError(response.ExitParam, "Invalid timestamp", key+" must be RFC3339 with a time zone: "+err.Error())
				}
				rangeBody[key] = value
			}
			body["time_range"] = rangeBody
		}
		limit, _ := cmd.Flags().GetInt("limit")
		if limit < 0 {
			response.ExitWithError(response.ExitParam, "Invalid limit", "limit must be non-negative")
		}
		if limit > 0 {
			body["limit"] = limit
		}
		entityData, _ := cmd.Flags().GetString("entity-data")
		if entityData != "" {
			var data model.EntityData
			if err := json.Unmarshal([]byte(entityData), &data); err != nil {
				response.ExitWithError(response.ExitParam, "Invalid entity data", err.Error())
			}
			body["entity_data"] = data
		}
		format, _ := cmd.Flags().GetString("plan-format")
		path := "/api/v1/query/" + ws + "/execute"
		if format != "" {
			if format != "agent" {
				response.ExitWithError(response.ExitParam, "Invalid plan format", "plan-format must be agent")
			}
			path += "?format=" + url.QueryEscape(format)
		}
		doRequest(http.MethodPost, path, body,
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
			{"description": "List all UModel element kinds", "query": ".umodel with(kind='entity_set') | limit 1000"},
			{"description": "List available entity-call methods", "query": ".entity_set with(domain='devops', name='devops.service') | entity-call __list_method__()"},
			{"description": "List data sets for an entity set", "query": ".entity_set with(domain='devops', name='devops.service') | entity-call list_data_set(['metric_set', 'log_set', 'event_set'], true)"},
			{"description": "Get logs for a specific entity", "query": ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_logs('devops', 'devops.log.service', query='level = \"ERROR\"')"},
			{"description": "Search entities by keyword", "query": ".entity with(domain='devops', name='devops.service', query='checkout') | limit 1000"},
			{"description": "Get direct topology relations", "query": ".topo | graph-call getDirectRelations([(:'devops@devops.service' {__entity_id__: '10000000000000000000000000000101'})]) | limit 1000"},
		}
		response.ExitWithSuccess(examples)
	},
}

func init() {
	queryRunCmd.Flags().String("from", "", "Query range start (RFC3339 with time zone)")
	queryRunCmd.Flags().String("to", "", "Query range end or instant evaluation time (RFC3339 with time zone)")
	queryRunCmd.Flags().Int("limit", 0, "Explicit query or log entry limit")
	queryRunCmd.Flags().String("entity-data", "", "EntityData JSON table for mapped entity attributes")
	queryRunCmd.Flags().String("plan-format", "", "Query plan response format (agent)")
	queryCmd.AddCommand(queryRunCmd, queryExplainCmd, queryExamplesCmd)

	addWorkspaceFlag(queryRunCmd, queryExplainCmd, queryExamplesCmd)

	registry.Global.Register(registry.CommandMeta{
		Group: "service", Resource: "query", Action: "run",
		Description: "Execute an SPL query",
		Params: []registry.ParamMeta{
			{Name: "workspace", Type: "string", Flag: "--workspace", Short: "-w", Description: "Target workspace", Required: true},
			{Name: "spl", Type: "string", Description: "SPL query expression", Required: true},
			{Name: "from", Type: "string", Flag: "--from", Description: "Query range start in RFC3339"},
			{Name: "to", Type: "string", Flag: "--to", Description: "Query range end or instant evaluation time in RFC3339"},
			{Name: "limit", Type: "int", Flag: "--limit", Description: "Explicit query or log entry limit"},
			{Name: "entity_data", Type: "string", Flag: "--entity-data", Description: "EntityData JSON table"},
			{Name: "plan_format", Type: "string", Flag: "--plan-format", Description: "Use agent query-plan response format"},
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
