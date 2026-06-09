package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type cli struct {
	addr string
	out  io.Writer
	err  io.Writer
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(args []string, out, errOut io.Writer) error {
	flags := flag.NewFlagSet("umctl", flag.ContinueOnError)
	flags.SetOutput(errOut)
	addr := flags.String("addr", "http://localhost:8080", "UModel server address")
	if parseErr := flags.Parse(args); parseErr != nil {
		return parseErr
	}
	rest := flags.Args()
	c := cli{addr: strings.TrimRight(*addr, "/"), out: out, err: errOut}
	if len(rest) == 0 || rest[0] == "help" || rest[0] == "--help" {
		printHelp(out)
		return nil
	}

	switch rest[0] {
	case "workspace":
		return c.workspace(rest[1:])
	case "umodel":
		return c.umodel(rest[1:])
	case "entity":
		return c.entity(rest[1:])
	case "topo":
		return c.topo(rest[1:])
	case "query":
		return c.query(rest[1:])
	case "agent":
		return c.agent(rest[1:])
	case "serve":
		fmt.Fprintln(out, "Use `umodel-server --quickstart` for the in-memory demo, `--graphstore file.memory` for local persistence, or build with `-tags ladybug` for local.ladybug storage.")
		return nil
	default:
		return fmt.Errorf("unknown command group: %s", rest[0])
	}
}

func (c cli) workspace(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("workspace command is required")
	}
	if forbidden(args[0], "start", "stop", "backup", "restore", "export", "import") {
		return fmt.Errorf("workspace %s is forbidden; workspace is metadata only", args[0])
	}
	switch args[0] {
	case "create":
		if len(args) < 2 {
			return fmt.Errorf("usage: workspace create <id> [json-file-or-inline]")
		}
		payload := map[string]any{"id": args[1]}
		if len(args) >= 3 {
			custom, err := readJSONObjectArg(args[2])
			if err != nil {
				return err
			}
			payload = custom
			if _, ok := payload["id"]; !ok {
				payload["id"] = args[1]
			}
		}
		return c.doJSON(http.MethodPost, "/api/v1/workspaces", payload)
	case "get":
		if len(args) < 2 {
			return fmt.Errorf("usage: workspace get <id>")
		}
		return c.doJSON(http.MethodGet, "/api/v1/workspaces/"+args[1], nil)
	case "list":
		return c.doJSON(http.MethodGet, "/api/v1/workspaces", nil)
	case "delete":
		if len(args) < 2 {
			return fmt.Errorf("usage: workspace delete <id>")
		}
		return c.doJSON(http.MethodDelete, "/api/v1/workspaces/"+args[1], nil)
	case "update":
		if len(args) < 3 {
			return fmt.Errorf("usage: workspace update <id> <json-file-or-inline>")
		}
		payload, err := readJSONArg(args[2])
		if err != nil {
			return err
		}
		return c.doRaw(http.MethodPut, "/api/v1/workspaces/"+args[1], payload)
	default:
		return fmt.Errorf("unknown workspace command: %s", args[0])
	}
}

func (c cli) umodel(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("umodel command is required")
	}
	if forbidden(args[0], "get", "list", "graph") {
		return fmt.Errorf("umodel %s is forbidden; read UModel data through query run/explain", args[0])
	}
	switch args[0] {
	case "import":
		if len(args) < 3 {
			return fmt.Errorf("usage: umodel import <workspace> <yaml-json-file-or-directory>")
		}
		return c.doJSON(http.MethodPost, "/api/v1/umodel/"+args[1]+"/import", map[string]any{"path": args[2]})
	case "put", "validate":
		if len(args) < 3 {
			return fmt.Errorf("usage: umodel %s <workspace> <json-file-or-inline>", args[0])
		}
		payload, err := readJSONArg(args[2])
		if err != nil {
			return err
		}
		payload, err = wrapJSONPayload(payload, "elements")
		if err != nil {
			return err
		}
		action := "elements"
		if args[0] == "validate" {
			action = "validate"
		}
		return c.doRaw(http.MethodPost, "/api/v1/umodel/"+args[1]+"/"+action, payload)
	case "delete":
		if len(args) < 3 {
			return fmt.Errorf("usage: umodel delete <workspace> <ids-json-csv-or-payload>")
		}
		payload, err := idsPayload(args[2:], "")
		if err != nil {
			return err
		}
		return c.doRaw(http.MethodDelete, "/api/v1/umodel/"+args[1]+"/elements", payload)
	case "export":
		if len(args) < 2 {
			return fmt.Errorf("usage: umodel export <workspace> [limit]")
		}
		limit := 1000
		if len(args) >= 3 {
			parsed, err := strconv.Atoi(args[2])
			if err != nil {
				return fmt.Errorf("invalid limit %q", args[2])
			}
			limit = parsed
		}
		return c.doJSON(http.MethodPost, "/api/v1/query/"+args[1]+"/execute", map[string]any{
			"query": fmt.Sprintf(".umodel | limit %d", limit),
			"limit": limit,
		})
	default:
		return fmt.Errorf("unknown umodel command: %s", args[0])
	}
}

func (c cli) entity(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("entity command is required")
	}
	if forbidden(args[0], "get", "list", "search") {
		return fmt.Errorf("entity %s is forbidden; read entities through query run/explain", args[0])
	}
	switch args[0] {
	case "write":
		if len(args) < 3 {
			return fmt.Errorf("usage: entity write <workspace> <json-file-or-inline>")
		}
		payload, err := readJSONArg(args[2])
		if err != nil {
			return err
		}
		payload, err = wrapJSONPayload(payload, "entities")
		if err != nil {
			return err
		}
		return c.doRaw(http.MethodPost, "/api/v1/entitystore/"+args[1]+"/entities:write", payload)
	case "expire", "delete":
		if len(args) < 3 {
			return fmt.Errorf("usage: entity %s <workspace> <ids-json-csv-or-payload> [reason]", args[0])
		}
		reason := reasonFromArgs(args[0], args[3:])
		payload, err := idsPayload(args[2:3], reason)
		if err != nil {
			return err
		}
		return c.doRaw(http.MethodPost, "/api/v1/entitystore/"+args[1]+"/entities:expire", payload)
	default:
		return fmt.Errorf("unknown entity command: %s", args[0])
	}
}

func (c cli) topo(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("topo command is required")
	}
	if forbidden(args[0], "neighbors", "subgraph", "path") {
		return fmt.Errorf("topo %s is forbidden; read topology through query run/explain", args[0])
	}
	switch args[0] {
	case "write":
		if len(args) < 3 {
			return fmt.Errorf("usage: topo write <workspace> <json-file-or-inline>")
		}
		payload, err := readJSONArg(args[2])
		if err != nil {
			return err
		}
		payload, err = wrapJSONPayload(payload, "relations")
		if err != nil {
			return err
		}
		return c.doRaw(http.MethodPost, "/api/v1/entitystore/"+args[1]+"/relations:write", payload)
	case "expire", "delete":
		if len(args) < 3 {
			return fmt.Errorf("usage: topo %s <workspace> <ids-json-csv-or-payload> [reason]", args[0])
		}
		reason := reasonFromArgs(args[0], args[3:])
		payload, err := idsPayload(args[2:3], reason)
		if err != nil {
			return err
		}
		return c.doRaw(http.MethodPost, "/api/v1/entitystore/"+args[1]+"/relations:expire", payload)
	default:
		return fmt.Errorf("unknown topo command: %s", args[0])
	}
}

func (c cli) query(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("query command is required")
	}
	switch args[0] {
	case "run", "explain":
		if len(args) < 3 {
			return fmt.Errorf("usage: query %s <workspace> <spl>", args[0])
		}
		action := "execute"
		if args[0] == "explain" {
			action = "explain"
		}
		return c.doJSON(http.MethodPost, "/api/v1/query/"+args[1]+"/"+action, map[string]any{"query": strings.Join(args[2:], " ")})
	case "examples":
		fmt.Fprintln(c.out, `.umodel with(kind='entity_set') | limit 20`)
		fmt.Fprintln(c.out, `.entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call __list_method__()`)
		fmt.Fprintln(c.out, `.entity_set with(domain='devops', name='devops.service') | entity-call list_data_set(['metric_set', 'log_set', 'event_set'], true)`)
		fmt.Fprintln(c.out, `.entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_logs('devops', 'devops.log.service', query='level = "ERROR"')`)
		fmt.Fprintln(c.out, `.entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_metrics('devops', 'devops.metric.service', 'request_count', step='30s')`)
		fmt.Fprintln(c.out, `.entity with(domain='devops', name='devops.service', query='checkout') | limit 20`)
		fmt.Fprintln(c.out, `.topo | graph-call getDirectRelations([(:"devops@devops.service" {__entity_id__: '10000000000000000000000000000101'})]) | limit 20`)
		return nil
	case "history":
		return fmt.Errorf("query history is not wired in the current CLI")
	default:
		return fmt.Errorf("unknown query command: %s", args[0])
	}
}

func (c cli) agent(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent command is required")
	}
	switch args[0] {
	case "mcp":
		fmt.Fprintln(c.out, "Use `umodel-mcp` for the stdio MCP server.")
		return nil
	case "discover":
		if len(args) < 2 {
			return fmt.Errorf("usage: agent discover <workspace>")
		}
		return c.doJSON(http.MethodGet, "/api/v1/agent/"+args[1]+"/discover", nil)
	case "tool":
		if len(args) < 3 {
			return fmt.Errorf("usage: agent tool <workspace> <tool-name> [json-args]")
		}
		payload := map[string]any{"name": args[2]}
		if len(args) >= 4 {
			arguments, err := readJSONObjectArg(args[3])
			if err != nil {
				return err
			}
			payload["arguments"] = arguments
		}
		return c.doJSON(http.MethodPost, "/api/v1/agent/"+args[1]+"/tools:execute", payload)
	case "skill":
		return fmt.Errorf("agent skill command scaffold is present; use agent discover for MCP metadata")
	default:
		return fmt.Errorf("unknown agent command: %s", args[0])
	}
}

func (c cli) doJSON(method, path string, payload any) error {
	body := []byte(nil)
	if payload != nil {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(payload); err != nil {
			return err
		}
		body = buf.Bytes()
	}
	return c.doRaw(method, path, body)
}

func (c cli) doRaw(method, path string, body []byte) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, c.addr+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("server returned %s: %s", resp.Status, strings.TrimSpace(string(payload)))
	}
	_, err = c.out.Write(payload)
	return err
}

func readJSONArg(value string) ([]byte, error) {
	if strings.HasPrefix(strings.TrimSpace(value), "{") || strings.HasPrefix(strings.TrimSpace(value), "[") {
		return []byte(value), nil
	}
	return os.ReadFile(value)
}

func readJSONObjectArg(value string) (map[string]any, error) {
	payload, err := readJSONArg(value)
	if err != nil {
		return nil, err
	}
	var object map[string]any
	if err := json.Unmarshal(payload, &object); err != nil {
		return nil, fmt.Errorf("expected JSON object: %w", err)
	}
	return object, nil
}

func wrapJSONPayload(payload []byte, field string) ([]byte, error) {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty JSON payload")
	}

	var object map[string]any
	if trimmed[0] == '{' {
		if err := json.Unmarshal(trimmed, &object); err != nil {
			return nil, err
		}
		if _, ok := object[field]; ok {
			return trimmed, nil
		}
		return marshalJSON(map[string]any{field: []any{object}})
	}

	if trimmed[0] == '[' {
		var array []any
		if err := json.Unmarshal(trimmed, &array); err != nil {
			return nil, err
		}
		return marshalJSON(map[string]any{field: array})
	}

	return nil, fmt.Errorf("expected JSON object or array")
}

func idsPayload(args []string, reason string) ([]byte, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("ids are required")
	}
	first := strings.TrimSpace(args[0])
	if strings.HasPrefix(first, "{") {
		payload, err := readJSONArg(args[0])
		if err != nil {
			return nil, err
		}
		return payload, nil
	}
	if strings.HasPrefix(first, "[") {
		payload, err := readJSONArg(args[0])
		if err != nil {
			return nil, err
		}
		var ids []string
		if err := json.Unmarshal(payload, &ids); err != nil {
			return nil, fmt.Errorf("expected JSON string array for ids: %w", err)
		}
		return marshalIDsPayload(ids, reason)
	}
	if stat, err := os.Stat(args[0]); err == nil && !stat.IsDir() {
		payload, err := os.ReadFile(args[0])
		if err != nil {
			return nil, err
		}
		trimmed := bytes.TrimSpace(payload)
		if len(trimmed) == 0 {
			return nil, fmt.Errorf("empty JSON payload")
		}
		if trimmed[0] == '{' {
			return trimmed, nil
		}
		if trimmed[0] == '[' {
			var ids []string
			if err := json.Unmarshal(trimmed, &ids); err != nil {
				return nil, fmt.Errorf("expected JSON string array for ids: %w", err)
			}
			return marshalIDsPayload(ids, reason)
		}
		return nil, fmt.Errorf("expected JSON object or string array in %s", args[0])
	}

	ids := make([]string, 0, len(args))
	for _, arg := range args {
		for _, part := range strings.Split(arg, ",") {
			id := strings.TrimSpace(part)
			if id != "" {
				ids = append(ids, id)
			}
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("ids are required")
	}
	return marshalIDsPayload(ids, reason)
}

func reasonFromArgs(action string, args []string) string {
	if len(args) > 0 {
		return strings.Join(args, " ")
	}
	if action == "delete" {
		return "delete requested by umctl"
	}
	return "expire requested by umctl"
}

func marshalIDsPayload(ids []string, reason string) ([]byte, error) {
	payload := map[string]any{"ids": ids}
	if reason != "" {
		payload["reason"] = reason
	}
	return marshalJSON(payload)
}

func marshalJSON(payload any) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func forbidden(value string, forbiddenValues ...string) bool {
	for _, candidate := range forbiddenValues {
		if value == candidate {
			return true
		}
	}
	return false
}

func printHelp(out io.Writer) {
	fmt.Fprintln(out, `umctl

Command groups:
  workspace create|get|list|update|delete
  umodel put|delete|import|export|validate
  entity write|delete|expire
  topo write|delete|expire
  query run|explain|history|examples
  agent discover|skill|tool|mcp
  serve

This CLI intentionally keeps domain reads behind unified SPL. Use query run/explain.`)
}
