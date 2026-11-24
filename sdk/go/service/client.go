package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		HTTPClient: http.DefaultClient,
	}
}

type CreateWorkspaceRequest struct {
	ID          string                    `json:"id"`
	Name        string                    `json:"name,omitempty"`
	Description string                    `json:"description,omitempty"`
	Labels      map[string]string         `json:"labels,omitempty"`
	Config      map[string]map[string]any `json:"config,omitempty"`
}

type WorkspaceMetadata struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Description     string            `json:"description,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Status          string            `json:"status"`
	ResourceVersion uint64            `json:"resource_version"`
}

type UModelElement struct {
	Kind    string         `json:"kind"`
	Domain  string         `json:"domain"`
	Name    string         `json:"name"`
	Version string         `json:"version,omitempty"`
	Spec    map[string]any `json:"spec,omitempty"`
}

type WriteResult struct {
	Accepted int               `json:"accepted"`
	Failed   int               `json:"failed"`
	Items    []BatchItemResult `json:"items,omitempty"`
}

type BatchItemResult struct {
	ID      string `json:"id,omitempty"`
	OK      bool   `json:"ok"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type ValidationResult struct {
	Valid bool `json:"valid"`
}

type UModelImportRequest struct {
	Path              string   `json:"path"`
	CommonSchemaPacks []string `json:"common_schema_packs,omitempty"`
}

type UModelImportResult struct {
	Workspace string          `json:"workspace"`
	Source    string          `json:"source"`
	Imported  int             `json:"imported"`
	Skipped   int             `json:"skipped"`
	Elements  []UModelElement `json:"elements,omitempty"`
}

type QueryRequest struct {
	Query     string         `json:"query"`
	Limit     int            `json:"limit,omitempty"`
	TimeoutMS int            `json:"timeout_ms,omitempty"`
	Format    string         `json:"format,omitempty"`
	Params    map[string]any `json:"parameters,omitempty"`
}

type QueryResult struct {
	Columns []string         `json:"columns"`
	Rows    []map[string]any `json:"rows"`
	Explain *QueryExplain    `json:"explain,omitempty"`
}

type QueryExplain struct {
	Source           string   `json:"source"`
	Provider         string   `json:"provider,omitempty"`
	StorageProvider  string   `json:"storage_provider,omitempty"`
	Pushdown         []string `json:"pushdown,omitempty"`
	Fallback         []string `json:"fallback,omitempty"`
	Operators        []string `json:"operators,omitempty"`
	Depth            int      `json:"depth,omitempty"`
	Limit            int      `json:"limit,omitempty"`
	TimeoutMS        int      `json:"timeout_ms,omitempty"`
	TimeRangeApplied bool     `json:"time_range_applied"`
}

type AgentTool struct {
	Name                        string `json:"name"`
	Description                 string `json:"description"`
	Enabled                     bool   `json:"enabled"`
	RequiresExplicitWriteEnable bool   `json:"requires_explicit_write_enable,omitempty"`
	InputSchema                 any    `json:"input_schema,omitempty"`
	OutputSchema                any    `json:"output_schema,omitempty"`
}

type AgentResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
	MIMEType    string `json:"mime_type"`
	ReadOnly    bool   `json:"read_only"`
}

type AgentQueryAction struct {
	Method string       `json:"method"`
	Path   string       `json:"path"`
	Body   QueryRequest `json:"body"`
}

type AgentNextAction struct {
	ID          string           `json:"id"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Tool        string           `json:"tool"`
	QueryAPI    AgentQueryAction `json:"query_api"`
}

type AgentDiscovery struct {
	Workspace   string            `json:"workspace"`
	Tools       []AgentTool       `json:"tools"`
	Resources   []AgentResource   `json:"resources"`
	NextActions []AgentNextAction `json:"next_actions,omitempty"`
}

func (c *Client) CreateWorkspace(ctx context.Context, req CreateWorkspaceRequest) (WorkspaceMetadata, error) {
	var out WorkspaceMetadata
	err := c.do(ctx, http.MethodPost, "/api/v1/workspaces", req, &out)
	return out, err
}

func (c *Client) PutUModelElements(ctx context.Context, workspace string, elements []UModelElement) (WriteResult, error) {
	var out WriteResult
	err := c.do(ctx, http.MethodPost, "/api/v1/umodel/"+escape(workspace)+"/elements", map[string]any{"elements": elements}, &out)
	return out, err
}

func (c *Client) ValidateUModel(ctx context.Context, workspace string, elements []UModelElement) (ValidationResult, error) {
	var out ValidationResult
	err := c.do(ctx, http.MethodPost, "/api/v1/umodel/"+escape(workspace)+"/validate", map[string]any{"elements": elements}, &out)
	return out, err
}

func (c *Client) ImportUModel(ctx context.Context, workspace string, req UModelImportRequest) (UModelImportResult, error) {
	var out UModelImportResult
	err := c.do(ctx, http.MethodPost, "/api/v1/umodel/"+escape(workspace)+"/import", req, &out)
	return out, err
}

func (c *Client) WriteEntities(ctx context.Context, workspace string, entities []map[string]any) (WriteResult, error) {
	var out WriteResult
	err := c.do(ctx, http.MethodPost, "/api/v1/entitystore/"+escape(workspace)+"/entities:write", map[string]any{"entities": entities}, &out)
	return out, err
}

func (c *Client) ExpireEntities(ctx context.Context, workspace string, ids []string, reason string) (WriteResult, error) {
	var out WriteResult
	err := c.do(ctx, http.MethodPost, "/api/v1/entitystore/"+escape(workspace)+"/entities:expire", expirePayload(ids, reason), &out)
	return out, err
}

func (c *Client) WriteRelations(ctx context.Context, workspace string, relations []map[string]any) (WriteResult, error) {
	var out WriteResult
	err := c.do(ctx, http.MethodPost, "/api/v1/entitystore/"+escape(workspace)+"/relations:write", map[string]any{"relations": relations}, &out)
	return out, err
}

func (c *Client) ExpireRelations(ctx context.Context, workspace string, ids []string, reason string) (WriteResult, error) {
	var out WriteResult
	err := c.do(ctx, http.MethodPost, "/api/v1/entitystore/"+escape(workspace)+"/relations:expire", expirePayload(ids, reason), &out)
	return out, err
}

func (c *Client) Query(ctx context.Context, workspace string, req QueryRequest) (QueryResult, error) {
	var raw json.RawMessage
	if err := c.do(ctx, http.MethodPost, "/api/v1/query/"+escape(workspace)+"/execute", req, &raw); err != nil {
		return QueryResult{}, err
	}
	var response struct {
		QueryResult
		Data *struct {
			Header []string `json:"header"`
			Data   [][]any  `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return QueryResult{}, err
	}
	if response.Data == nil {
		return response.QueryResult, nil
	}
	out := QueryResult{Columns: response.Data.Header, Rows: make([]map[string]any, 0, len(response.Data.Data))}
	for _, values := range response.Data.Data {
		if len(values) != len(out.Columns) {
			return QueryResult{}, fmt.Errorf("query response row length does not match header")
		}
		row := make(map[string]any, len(values))
		for i, column := range out.Columns {
			row[column] = values[i]
		}
		out.Rows = append(out.Rows, row)
	}
	return out, nil
}

func (c *Client) Explain(ctx context.Context, workspace string, req QueryRequest) (QueryExplain, error) {
	var out QueryExplain
	err := c.do(ctx, http.MethodPost, "/api/v1/query/"+escape(workspace)+"/explain", req, &out)
	return out, err
}

func (c *Client) Discover(ctx context.Context, workspace string) (AgentDiscovery, error) {
	var out AgentDiscovery
	err := c.do(ctx, http.MethodGet, "/api/v1/agent/"+escape(workspace)+"/discover", nil, &out)
	return out, err
}

func (c *Client) do(ctx context.Context, method, path string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(payload); err != nil {
			return err
		}
		body = &buf
	}

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("content-type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return readErr
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("umodel service returned %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	if out == nil || len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode %s %s response: %w", method, path, err)
	}
	return nil
}

func expirePayload(ids []string, reason string) map[string]any {
	payload := map[string]any{"ids": ids}
	if reason != "" {
		payload["reason"] = reason
	}
	return payload
}

func escape(value string) string {
	return url.PathEscape(value)
}
