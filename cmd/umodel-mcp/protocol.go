package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"

	"github.com/alibaba/UnifiedModel/internal/bootstrap"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

const (
	currentProtocolVersion = "2025-06-18"
	toonMimeType           = "text/toon"
)

var supportedProtocolVersions = map[string]bool{
	"2025-06-18": true,
	"2025-03-26": true,
	"2024-11-05": true,
}

type rpcRequest struct {
	JSONRPC string         `json:"jsonrpc,omitempty"`
	ID      any            `json:"id,omitempty"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
	hasID   bool
}

type rpcResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func handleRawRPC(ctx context.Context, app *bootstrap.App, defaultWorkspace string, payload []byte) (rpcResponse, bool) {
	req, err := parseRPCRequest(payload)
	if err != nil {
		return rpcResponse{
			JSONRPC: "2.0",
			Error:   &rpcError{Code: -32700, Message: "Parse error", Data: err.Error()},
		}, true
	}
	return handleRPCRequest(ctx, app, defaultWorkspace, req)
}

func parseRPCRequest(payload []byte) (rpcRequest, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err != nil {
		return rpcRequest{}, err
	}
	var req rpcRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return rpcRequest{}, err
	}
	_, req.hasID = raw["id"]
	if req.JSONRPC == "" {
		req.JSONRPC = "2.0"
	}
	return req, nil
}

func handleRPCRequest(ctx context.Context, app *bootstrap.App, defaultWorkspace string, req rpcRequest) (rpcResponse, bool) {
	if req.Method == "" {
		return errorResponse(req, -32600, "Invalid request", "method is required"), true
	}
	result, rpcErr := handleRequest(ctx, app, defaultWorkspace, req)
	if !req.hasID {
		return rpcResponse{}, false
	}
	if rpcErr != nil {
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: rpcErr}, true
	}
	return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: result}, true
}

func errorResponse(req rpcRequest, code int, message string, data any) rpcResponse {
	return rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: code, Message: message, Data: data}}
}

func handleRequest(ctx context.Context, app *bootstrap.App, defaultWorkspace string, req rpcRequest) (any, *rpcError) {
	workspace := defaultWorkspace
	if value := stringParam(req.Params, "workspace"); value != "" {
		workspace = value
	}

	switch req.Method {
	case "initialize":
		return initializeResult(ctx, app, workspace, req)
	case "notifications/initialized", "notifications/cancelled":
		return map[string]any{}, nil
	case "ping", "logging/setLevel":
		return map[string]any{}, nil
	case "tools/list":
		tools, err := app.AgentGateway.Tools(ctx)
		if err != nil {
			return nil, internalRPCError(err)
		}
		return map[string]any{"tools": mcpTools(tools)}, nil
	case "tools/call":
		return callTool(ctx, app, workspace, req.Params)
	case "resources/list":
		discovery, err := app.AgentGateway.Discover(ctx, workspace)
		if err != nil {
			return nil, internalRPCError(err)
		}
		return map[string]any{"resources": mcpResources(discovery.Resources)}, nil
	case "resources/templates/list":
		return map[string]any{"resourceTemplates": mcpResourceTemplates(workspace)}, nil
	case "resources/read":
		uri := stringParam(req.Params, "uri")
		if uri == "" {
			return nil, invalidParams("uri param is required")
		}
		result, err := app.AgentGateway.ReadResource(ctx, workspace, model.AgentResourceReadRequest{URI: uri})
		if err != nil {
			return nil, mapServiceError(err)
		}
		return map[string]any{
			"contents": []map[string]any{
				{
					"uri":      result.URI,
					"mimeType": toonMimeType,
					"text":     encodeTOON(result.Content),
					"_meta": map[string]any{
						"sourceMimeType": result.MIMEType,
						"format":         "toon",
					},
				},
			},
		}, nil
	case "prompts/list":
		return map[string]any{"prompts": mcpPrompts()}, nil
	case "prompts/get":
		return getPrompt(workspace, req.Params)
	case "completion/complete":
		return completionResult(workspace, req.Params), nil
	case "discovery", "umodel/discovery":
		discovery, err := app.AgentGateway.Discover(ctx, workspace)
		if err != nil {
			return nil, internalRPCError(err)
		}
		return discovery, nil
	default:
		return nil, &rpcError{Code: -32601, Message: "Method not found", Data: req.Method}
	}
}

func initializeResult(ctx context.Context, app *bootstrap.App, workspace string, req rpcRequest) (any, *rpcError) {
	protocolVersion := currentProtocolVersion
	if requested := stringParam(req.Params, "protocolVersion"); requested != "" && supportedProtocolVersions[requested] {
		protocolVersion = requested
	}
	discovery, err := app.AgentGateway.Discover(ctx, workspace)
	if err != nil {
		return nil, internalRPCError(err)
	}
	return map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities": map[string]any{
			"tools":       map[string]any{"listChanged": false},
			"resources":   map[string]any{"listChanged": false},
			"prompts":     map[string]any{"listChanged": false},
			"completions": map[string]any{},
			"logging":     map[string]any{},
		},
		"serverInfo": map[string]string{
			"name":    "umodel-mcp",
			"title":   "UModel MCP Server",
			"version": "0.1.0",
		},
		"instructions": "Use UModel query tools for .umodel, .entity, and .topo reads. Tool and resource text payloads are encoded as TOON while the MCP JSON-RPC envelope remains JSON.",
		"discovery":    discovery,
		"_meta": map[string]any{
			"workspace":    workspace,
			"outputFormat": toonMimeType,
			"protocols":    []string{"stdio", "streamable-http", "http+sse"},
		},
	}, nil
}

func callTool(ctx context.Context, app *bootstrap.App, workspace string, params map[string]any) (any, *rpcError) {
	name := stringParam(params, "name")
	if name == "" {
		return nil, invalidParams("name param is required")
	}
	arguments, err := mapParam(params, "arguments")
	if err != nil {
		return nil, invalidParams(err.Error())
	}
	result, err := app.AgentGateway.ExecuteTool(ctx, workspace, model.AgentToolCallRequest{Name: name, Arguments: arguments})
	if err != nil {
		return toolErrorResult(name, err), nil
	}
	structured := map[string]any{
		"name":   result.Name,
		"ok":     result.OK,
		"output": result.Output,
	}
	return map[string]any{
		"content": []map[string]any{
			{
				"type": "text",
				"text": encodeTOON(structured),
				"_meta": map[string]any{
					"format":   "toon",
					"mimeType": toonMimeType,
				},
			},
		},
		"structuredContent": structured,
		"isError":           false,
	}, nil
}

func toolErrorResult(name string, err error) map[string]any {
	payload := map[string]any{
		"name":  name,
		"ok":    false,
		"error": err.Error(),
	}
	if coded, ok := apperrors.As(err); ok {
		payload["code"] = coded.Code
	}
	return map[string]any{
		"content": []map[string]any{
			{
				"type": "text",
				"text": encodeTOON(payload),
				"_meta": map[string]any{
					"format":   "toon",
					"mimeType": toonMimeType,
				},
			},
		},
		"structuredContent": payload,
		"isError":           true,
	}
}

func mcpTools(tools []model.AgentTool) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		item := map[string]any{
			"name":        tool.Name,
			"title":       titleFromName(tool.Name),
			"description": tool.Description,
			"inputSchema": schemaObject(tool.InputSchema),
			"outputSchema": map[string]any{
				"type":        "object",
				"description": "The structuredContent field mirrors this JSON shape. The content text block is encoded as TOON.",
			},
			"annotations": map[string]any{
				"title":           titleFromName(tool.Name),
				"readOnlyHint":    !tool.RequiresExplicitWriteEnable,
				"destructiveHint": tool.RequiresExplicitWriteEnable,
			},
			"_meta": map[string]any{
				"enabledByDefault":            tool.Enabled,
				"requiresExplicitWriteEnable": tool.RequiresExplicitWriteEnable,
				"outputFormat":                toonMimeType,
				"legacyInputSchema":           tool.InputSchema,
				"legacyOutputSchema":          tool.OutputSchema,
			},
		}
		out = append(out, item)
	}
	return out
}

func schemaObject(schema any) map[string]any {
	if value, ok := schema.(map[string]any); ok {
		return value
	}
	return map[string]any{"type": "object", "additionalProperties": true}
}

func mcpResources(resources []model.AgentResource) []map[string]any {
	out := make([]map[string]any, 0, len(resources))
	for _, resource := range resources {
		out = append(out, map[string]any{
			"uri":         resource.URI,
			"name":        resource.Name,
			"title":       titleFromName(resource.Name),
			"description": resource.Description,
			"mimeType":    toonMimeType,
			"_meta": map[string]any{
				"kind":           resource.Kind,
				"readOnly":       resource.ReadOnly,
				"sourceMimeType": resource.MIMEType,
			},
		})
	}
	return out
}

func mcpResourceTemplates(workspace string) []map[string]any {
	templates := []string{"overview", "schema-index", "query-templates", "tool-capability-metadata"}
	out := make([]map[string]any, 0, len(templates))
	for _, name := range templates {
		out = append(out, map[string]any{
			"name":        name,
			"title":       titleFromName(name),
			"uriTemplate": "umodel://workspace/{workspace}/" + name,
			"description": fmt.Sprintf("Read %s metadata for a UModel workspace.", name),
			"mimeType":    toonMimeType,
			"_meta": map[string]any{
				"defaultWorkspace": workspace,
			},
		})
	}
	return out
}

func mcpPrompts() []map[string]any {
	return []map[string]any{
		{
			"name":        "umodel_query_context",
			"title":       "UModel Query Context",
			"description": "Prepare a UModel query task using .umodel, .entity, and .topo surfaces.",
			"arguments": []map[string]any{
				{"name": "workspace", "description": "Workspace name.", "required": false},
				{"name": "query", "description": "Optional SPL query to inspect.", "required": false},
			},
		},
		{
			"name":        "umodel_object_graph_review",
			"title":       "UModel Object Graph Review",
			"description": "Review model, entity, and topology context before using runtime query tools.",
			"arguments": []map[string]any{
				{"name": "workspace", "description": "Workspace name.", "required": false},
				{"name": "focus", "description": "Review focus.", "required": false},
			},
		},
	}
}

func getPrompt(defaultWorkspace string, params map[string]any) (any, *rpcError) {
	name := stringParam(params, "name")
	if name == "" {
		return nil, invalidParams("name param is required")
	}
	args, err := mapParam(params, "arguments")
	if err != nil {
		return nil, invalidParams(err.Error())
	}
	workspace := defaultWorkspace
	if value := stringArg(args, "workspace"); value != "" {
		workspace = value
	}
	switch name {
	case "umodel_query_context":
		query := stringArg(args, "query")
		if query == "" {
			query = ".umodel | limit 20"
		}
		return promptResult("Use UModel Query Service", fmt.Sprintf("Workspace: %s\nRun or refine this UModel SPL through query_spl_execute or query_spl_explain:\n%s\n\nPrefer .umodel, .entity_set, .entity, .topo, and .runbook_set as the public read sources. Tool/resource data returned by this server is encoded as TOON.", workspace, query)), nil
	case "umodel_object_graph_review":
		focus := stringArg(args, "focus")
		if focus == "" {
			focus = "model definitions, runtime entities, and topology relations"
		}
		return promptResult("Review UModel Object Graph Context", fmt.Sprintf("Workspace: %s\nFocus: %s\nUse resources for metadata, then query tools for runtime rows. Keep resources metadata-only and use Query Service for .umodel, .entity_set, .entity, .topo, and .runbook_set reads.", workspace, focus)), nil
	default:
		return nil, invalidParams("unknown prompt: " + name)
	}
}

func promptResult(description, text string) map[string]any {
	return map[string]any{
		"description": description,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": map[string]any{
					"type": "text",
					"text": text,
				},
			},
		},
	}
}

func completionResult(workspace string, params map[string]any) map[string]any {
	ref, _ := mapParam(params, "ref")
	argument, _ := mapParam(params, "argument")
	value := strings.ToLower(stringArg(argument, "value"))
	values := []string{}
	switch stringArg(ref, "type") {
	case "ref/resource":
		for _, name := range []string{"overview", "schema-index", "query-templates", "tool-capability-metadata"} {
			candidate := "umodel://workspace/" + workspace + "/" + name
			if value == "" || strings.Contains(strings.ToLower(candidate), value) || strings.Contains(name, value) {
				values = append(values, candidate)
			}
		}
	case "ref/prompt":
		for _, candidate := range []string{"umodel_query_context", "umodel_object_graph_review", workspace, ".umodel | limit 20", ".entity | limit 20", ".topo | limit 20"} {
			if value == "" || strings.Contains(strings.ToLower(candidate), value) {
				values = append(values, candidate)
			}
		}
	default:
		if value == "" || strings.Contains(strings.ToLower(workspace), value) {
			values = append(values, workspace)
		}
	}
	if len(values) > 100 {
		values = values[:100]
	}
	return map[string]any{
		"completion": map[string]any{
			"values":  values,
			"total":   len(values),
			"hasMore": false,
		},
	}
}

func invalidParams(message string) *rpcError {
	return &rpcError{Code: -32602, Message: "Invalid params", Data: message}
}

func internalRPCError(err error) *rpcError {
	return &rpcError{Code: -32603, Message: "Internal error", Data: err.Error()}
}

func mapServiceError(err error) *rpcError {
	coded, ok := apperrors.As(err)
	if !ok {
		return internalRPCError(err)
	}
	switch coded.Code {
	case apperrors.CodeNotFound:
		return &rpcError{Code: -32002, Message: "Resource not found", Data: err.Error()}
	default:
		return internalRPCError(err)
	}
}

func stringParam(params map[string]any, key string) string {
	if params == nil {
		return ""
	}
	value, _ := params[key].(string)
	return value
}

func mapParam(params map[string]any, key string) (map[string]any, error) {
	if params == nil || params[key] == nil {
		return map[string]any{}, nil
	}
	value, ok := params[key].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s param must be an object", key)
	}
	return value, nil
}

func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	if value, ok := args[key].(string); ok {
		return value
	}
	return ""
}

func titleFromName(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == '-' || r == '/'
	})
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, " ")
}
