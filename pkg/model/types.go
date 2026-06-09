package model

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type PageRequest struct {
	Limit     int    `json:"limit,omitempty"`
	PageToken string `json:"page_token,omitempty"`
}

type Page[T any] struct {
	Items     []T    `json:"items"`
	NextToken string `json:"next_token,omitempty"`
}

type TimeRange struct {
	From *time.Time `json:"from,omitempty"`
	To   *time.Time `json:"to,omitempty"`
}

type WorkspaceStatus string

const (
	WorkspaceStatusActive     WorkspaceStatus = "active"
	WorkspaceStatusDeleted    WorkspaceStatus = "deleted"
	WorkspaceStatusConflicted WorkspaceStatus = "conflicted"
)

type WorkspacePaths struct {
	Root string `json:"root"`
	Tmp  string `json:"tmp,omitempty"`
}

type WorkspaceMetadata struct {
	ID              string                    `json:"id"`
	Name            string                    `json:"name"`
	Description     string                    `json:"description,omitempty"`
	Labels          map[string]string         `json:"labels,omitempty"`
	Config          map[string]map[string]any `json:"config,omitempty"`
	Paths           WorkspacePaths            `json:"paths"`
	Status          WorkspaceStatus           `json:"status"`
	ResourceVersion uint64                    `json:"resource_version"`
	CreatedAt       time.Time                 `json:"created_at"`
	UpdatedAt       time.Time                 `json:"updated_at"`
	DeletedAt       *time.Time                `json:"deleted_at,omitempty"`
}

type CreateWorkspaceRequest struct {
	ID          string                    `json:"id"`
	Name        string                    `json:"name,omitempty"`
	Description string                    `json:"description,omitempty"`
	Labels      map[string]string         `json:"labels,omitempty"`
	Config      map[string]map[string]any `json:"config,omitempty"`
}

type UpdateWorkspaceRequest struct {
	Name           *string                   `json:"name,omitempty"`
	Description    *string                   `json:"description,omitempty"`
	Labels         map[string]string         `json:"labels,omitempty"`
	Config         map[string]map[string]any `json:"config,omitempty"`
	IfMatchVersion uint64                    `json:"if_match_version,omitempty"`
	ReplaceLabels  bool                      `json:"replace_labels,omitempty"`
	ReplaceConfig  bool                      `json:"replace_config,omitempty"`
}

type WorkspaceListRequest struct {
	PageRequest
	IncludeDeleted   bool              `json:"include_deleted,omitempty"`
	IncludeConflicts bool              `json:"include_conflicts,omitempty"`
	LabelSelector    map[string]string `json:"label_selector,omitempty"`
}

type ErrorDetail struct {
	Field  string `json:"field,omitempty"`
	Reason string `json:"reason,omitempty"`
	Limit  string `json:"limit,omitempty"`
}

type BatchItemResult struct {
	ID      string        `json:"id,omitempty"`
	OK      bool          `json:"ok"`
	Code    string        `json:"code,omitempty"`
	Message string        `json:"message,omitempty"`
	Details []ErrorDetail `json:"details,omitempty"`
}

type WriteResult struct {
	Accepted int               `json:"accepted"`
	Failed   int               `json:"failed"`
	Items    []BatchItemResult `json:"items,omitempty"`
	Warnings []ErrorDetail     `json:"warnings,omitempty"`
}

type UModelElement struct {
	Kind    string         `json:"kind"`
	Domain  string         `json:"domain"`
	Name    string         `json:"name"`
	Version string         `json:"version,omitempty"`
	Spec    map[string]any `json:"spec,omitempty"`
}

func UModelElementKey(element UModelElement) string {
	return UModelElementRefKey(element.Domain, element.Name, element.Kind)
}

func UModelElementRefKey(domain, name, kind string) string {
	if domain == "" || name == "" || kind == "" {
		return ""
	}
	return strings.Join([]string{domain, name, kind}, "/")
}

type UModelElementBatch struct {
	Workspace string          `json:"workspace"`
	Elements  []UModelElement `json:"elements"`
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
	Errors    []ErrorDetail   `json:"errors,omitempty"`
}

type SampleImportResult struct {
	Workspace     string             `json:"workspace"`
	Sample        string             `json:"sample"`
	UModel        UModelImportResult `json:"umodel"`
	Entities      WriteResult        `json:"entities"`
	Relations     WriteResult        `json:"relations"`
	EntityCount   int                `json:"entity_count"`
	RelationCount int                `json:"relation_count"`
}

type UModelSnapshotRequest struct {
	Workspace string `json:"workspace"`
	Version   string `json:"version,omitempty"`
}

type UModelSnapshot struct {
	Workspace string          `json:"workspace"`
	Version   string          `json:"version"`
	Elements  []UModelElement `json:"elements"`
}

type ValidationResult struct {
	Valid    bool          `json:"valid"`
	Errors   []ErrorDetail `json:"errors,omitempty"`
	Warnings []ErrorDetail `json:"warnings,omitempty"`
}

type EntityTypeRef struct {
	Domain string `json:"domain"`
	Name   string `json:"name"`
}

type RelationTypeRef struct {
	Domain string `json:"domain,omitempty"`
	Type   string `json:"type"`
}

type EntitySetSchema struct {
	Ref    EntityTypeRef  `json:"ref"`
	Fields map[string]any `json:"fields,omitempty"`
}

type RelationSchema struct {
	Ref    RelationTypeRef `json:"ref"`
	Fields map[string]any  `json:"fields,omitempty"`
}

type SchemaVersion struct {
	Workspace string `json:"workspace"`
	Version   string `json:"version"`
}

type EntityPayload map[string]any

type RelationPayload map[string]any

type EntityRecord struct {
	Key        string        `json:"key"`
	Domain     string        `json:"domain"`
	Type       string        `json:"type"`
	ID         string        `json:"id"`
	Method     string        `json:"method,omitempty"`
	Deleted    bool          `json:"deleted,omitempty"`
	Properties EntityPayload `json:"properties,omitempty"`
}

type RelationRecord struct {
	Key        string          `json:"key"`
	Source     EntityRecord    `json:"source"`
	Dest       EntityRecord    `json:"dest"`
	Type       string          `json:"type"`
	Method     string          `json:"method,omitempty"`
	Deleted    bool            `json:"deleted,omitempty"`
	Properties RelationPayload `json:"properties,omitempty"`
}

type EntityWriteBatch struct {
	Workspace      string          `json:"workspace"`
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
	PartialSuccess bool            `json:"partial_success,omitempty"`
	Entities       []EntityPayload `json:"entities"`
}

type RelationWriteBatch struct {
	Workspace      string            `json:"workspace"`
	IdempotencyKey string            `json:"idempotency_key,omitempty"`
	PartialSuccess bool              `json:"partial_success,omitempty"`
	Relations      []RelationPayload `json:"relations"`
}

func EntityStableKey(payload EntityPayload) string {
	return strings.Join([]string{
		payloadString(payload["__domain__"]),
		payloadString(payload["__entity_type__"]),
		payloadString(payload["__entity_id__"]),
	}, "/")
}

func RelationStableKey(payload RelationPayload) string {
	return strings.Join([]string{
		payloadString(payload["__src_domain__"]),
		payloadString(payload["__src_entity_type__"]),
		payloadString(payload["__src_entity_id__"]),
		payloadString(payload["__relation_type__"]),
		payloadString(payload["__dest_domain__"]),
		payloadString(payload["__dest_entity_type__"]),
		payloadString(payload["__dest_entity_id__"]),
	}, "/")
}

func payloadString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(value)
	}
}

type ExpireRequest struct {
	Workspace string   `json:"workspace"`
	IDs       []string `json:"ids"`
	Reason    string   `json:"reason,omitempty"`
}

type QueryRequest struct {
	Query            string         `json:"query"`
	TimeRange        TimeRange      `json:"time_range,omitempty"`
	Format           string         `json:"format,omitempty"`
	Limit            int            `json:"limit,omitempty"`
	TimeoutMS        int            `json:"timeout_ms,omitempty"`
	Params           map[string]any `json:"parameters,omitempty"`
	EntityData       *EntityData    `json:"entity_data,omitempty"`
	EntityDataCamel  *EntityData    `json:"entityData,omitempty"`
	FilterByEntities *EntityData    `json:"filterByEntities,omitempty"`
}

func (r QueryRequest) EntityFilterData() *EntityData {
	for _, data := range []*EntityData{r.FilterByEntities, r.EntityDataCamel, r.EntityData} {
		if data != nil && !data.Empty() {
			return data
		}
	}
	return nil
}

type EntityData struct {
	Version int        `json:"version,omitempty"`
	Header  []string   `json:"header,omitempty"`
	Data    [][]string `json:"data,omitempty"`
}

func (e EntityData) Empty() bool {
	return len(e.Header) == 0 || len(e.Data) == 0
}

func (e EntityData) ToArrayMap() []map[string]string {
	if e.Empty() {
		return nil
	}
	rows := make([]map[string]string, 0, len(e.Data))
	for _, row := range e.Data {
		item := make(map[string]string, len(e.Header))
		for idx, field := range e.Header {
			if field == "" || idx >= len(row) {
				continue
			}
			item[field] = row[idx]
		}
		rows = append(rows, item)
	}
	return rows
}

func (e *EntityData) UnmarshalJSON(raw []byte) error {
	if e == nil {
		return nil
	}
	if string(raw) == "null" {
		*e = EntityData{}
		return nil
	}
	var table struct {
		Version int             `json:"version,omitempty"`
		Header  []string        `json:"header,omitempty"`
		Data    json.RawMessage `json:"data,omitempty"`
	}
	if err := json.Unmarshal(raw, &table); err == nil && (len(table.Header) > 0 || len(table.Data) > 0) {
		rows, header, err := decodeEntityDataRows(table.Data, table.Header)
		if err != nil {
			return err
		}
		*e = EntityData{Version: table.Version, Header: header, Data: rows}
		return nil
	}
	rows, header, err := decodeEntityDataRows(raw, nil)
	if err != nil {
		return err
	}
	*e = EntityData{Header: header, Data: rows}
	return nil
}

func decodeEntityDataRows(raw json.RawMessage, header []string) ([][]string, []string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, append([]string(nil), header...), nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, nil, err
	}
	outHeader := append([]string(nil), header...)
	if len(outHeader) == 0 {
		outHeader = inferEntityDataHeader(items)
	}
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		row, err := decodeEntityDataRow(item, outHeader)
		if err != nil {
			return nil, nil, err
		}
		if len(row) != len(outHeader) {
			return nil, nil, fmt.Errorf("entity_data row length %d does not match header length %d", len(row), len(outHeader))
		}
		rows = append(rows, row)
	}
	return rows, outHeader, nil
}

func inferEntityDataHeader(items []json.RawMessage) []string {
	keys := map[string]struct{}{}
	for _, item := range items {
		var row map[string]any
		if err := json.Unmarshal(item, &row); err != nil {
			continue
		}
		if _, ok := row["values"]; ok {
			continue
		}
		for key := range row {
			keys[key] = struct{}{}
		}
	}
	header := make([]string, 0, len(keys))
	for key := range keys {
		header = append(header, key)
	}
	sort.Strings(header)
	return header
}

func decodeEntityDataRow(raw json.RawMessage, header []string) ([]string, error) {
	var values []any
	if err := json.Unmarshal(raw, &values); err == nil {
		return entityDataValuesToStrings(values), nil
	}
	var wrapped struct {
		Values []any `json:"values"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && wrapped.Values != nil {
		return entityDataValuesToStrings(wrapped.Values), nil
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	row := make([]string, 0, len(header))
	for _, field := range header {
		row = append(row, entityDataCellString(object[field]))
	}
	return row, nil
}

func entityDataValuesToStrings(values []any) []string {
	row := make([]string, 0, len(values))
	for _, value := range values {
		row = append(row, entityDataCellString(value))
	}
	return row
}

func entityDataCellString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprint(typed)
	}
}

type QueryResult struct {
	Columns []string         `json:"columns"`
	Rows    []map[string]any `json:"rows"`
	Page    PageRequest      `json:"page"`
	Explain *QueryExplain    `json:"explain,omitempty"`
}

type QueryExecuteResponse struct {
	Code    string           `json:"code"`
	Data    QueryExecuteData `json:"data"`
	Message string           `json:"message"`
	Success bool             `json:"success"`
}

type QueryExecuteData struct {
	Data           [][]any             `json:"data"`
	Header         []string            `json:"header"`
	ResponseStatus QueryResponseStatus `json:"responseStatus"`
}

type QueryResponseStatus struct {
	Result      string `json:"result"`
	RetryPolicy string `json:"retryPolicy"`
	Level       string `json:"level"`
	StatusItem  []any  `json:"statusItem"`
}

func NewQueryExecuteResponse(result QueryResult) QueryExecuteResponse {
	header := queryMatrixHeader(result.Columns, result.Rows)
	return QueryExecuteResponse{
		Code:    "200",
		Message: "successful",
		Success: true,
		Data: QueryExecuteData{
			Header: header,
			Data:   queryRowsAsMatrix(header, result.Rows),
			ResponseStatus: QueryResponseStatus{
				Result:      "Success",
				RetryPolicy: "None",
				Level:       "Info",
				StatusItem:  []any{},
			},
		},
	}
}

func queryMatrixHeader(columns []string, rows []map[string]any) []string {
	header := append([]string(nil), columns...)
	seen := make(map[string]struct{}, len(header))
	for _, column := range header {
		seen[column] = struct{}{}
	}
	extras := map[string]struct{}{}
	for _, row := range rows {
		for column := range row {
			if _, ok := seen[column]; ok {
				continue
			}
			extras[column] = struct{}{}
		}
	}
	extraColumns := make([]string, 0, len(extras))
	for column := range extras {
		extraColumns = append(extraColumns, column)
	}
	sort.Strings(extraColumns)
	return append(header, extraColumns...)
}

func queryRowsAsMatrix(columns []string, rows []map[string]any) [][]any {
	out := make([][]any, 0, len(rows))
	for _, row := range rows {
		values := make([]any, 0, len(columns))
		for _, column := range columns {
			values = append(values, row[column])
		}
		out = append(out, values)
	}
	return out
}

type QueryExplain struct {
	Source           string          `json:"source"`
	Provider         string          `json:"provider,omitempty"`
	StorageProvider  string          `json:"storage_provider,omitempty"`
	SearchProvider   string          `json:"search_provider,omitempty"`
	EmbedModel       string          `json:"embed_model,omitempty"`
	SearchMode       string          `json:"search_mode,omitempty"`
	CypherDialect    string          `json:"cypher_dialect,omitempty"`
	CypherEngine     string          `json:"cypher_engine,omitempty"`
	EntityCall       *EntityCallPlan `json:"entity_call,omitempty"`
	Pushdown         []string        `json:"pushdown,omitempty"`
	Fallback         []string        `json:"fallback,omitempty"`
	Operators        []string        `json:"operators,omitempty"`
	Depth            int             `json:"depth,omitempty"`
	Limit            int             `json:"limit,omitempty"`
	TimeoutMS        int             `json:"timeout_ms,omitempty"`
	TimeRangeApplied bool            `json:"time_range_applied"`
}

type QueryPredicate struct {
	Field string `json:"field,omitempty"`
	Op    string `json:"op,omitempty"`
	Value any    `json:"value,omitempty"`
}

type QuerySort struct {
	Field string `json:"field,omitempty"`
	Desc  bool   `json:"desc,omitempty"`
}

type GraphNodeSelector struct {
	Variable   string         `json:"variable,omitempty"`
	Label      string         `json:"label,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
	Raw        string         `json:"raw,omitempty"`
}

type GraphCallPlan struct {
	Name      string              `json:"name,omitempty"`
	Type      string              `json:"type,omitempty"`
	Direction string              `json:"direction,omitempty"`
	Depth     int                 `json:"depth,omitempty"`
	SeedIDs   []string            `json:"seed_ids,omitempty"`
	Nodes     []GraphNodeSelector `json:"nodes,omitempty"`
	Cypher    string              `json:"cypher,omitempty"`
}

type EntityCallParam struct {
	Key         string `json:"key,omitempty"`
	Type        string `json:"type,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Default     any    `json:"default,omitempty"`
}

type EntityCallPlan struct {
	Name           string            `json:"name,omitempty"`
	Arguments      []any             `json:"arguments,omitempty"`
	NamedArguments map[string]any    `json:"named_arguments,omitempty"`
	Parameters     map[string]any    `json:"parameters,omitempty"`
	Signature      []EntityCallParam `json:"signature,omitempty"`
}

type QueryPipelineOperator struct {
	Name       string          `json:"name,omitempty"`
	Expression string          `json:"expression,omitempty"`
	Predicate  *QueryPredicate `json:"predicate,omitempty"`
	Project    []string        `json:"project,omitempty"`
	Sort       *QuerySort      `json:"sort,omitempty"`
	GraphCall  *GraphCallPlan  `json:"graph_call,omitempty"`
	EntityCall *EntityCallPlan `json:"entity_call,omitempty"`
	Limit      int             `json:"limit,omitempty"`
}

type QueryPlan struct {
	Workspace  string
	Source     string
	Query      string
	Filters    map[string]any
	Operators  []string
	Pipeline   []QueryPipelineOperator
	Predicates []QueryPredicate
	Project    []string
	Sort       []QuerySort
	GraphCall  *GraphCallPlan
	EntityCall *EntityCallPlan
	TopK       int
	TimeRange  TimeRange
	Params     map[string]any
	EntityData *EntityData
	Limit      int
	Depth      int
	TimeoutMS  int
}

type EntityQueryPlan = QueryPlan

type TopoQueryPlan = QueryPlan

type GraphStoreCapabilities struct {
	EntitySearch       bool   `json:"entity_search"`
	GraphMatch         bool   `json:"graph_match"`
	GraphCallNeighbors bool   `json:"graph_call_neighbors"`
	ControlledCypher   bool   `json:"controlled_cypher"`
	TimeVisibility     bool   `json:"time_visibility"`
	ServerSideFilter   bool   `json:"server_side_filter"`
	MaxDepth           int    `json:"max_depth"`
	MaxLimit           int    `json:"max_limit"`
	Timeout            string `json:"timeout"`
}

type GraphStoreHealth struct {
	Provider string `json:"provider"`
	Status   string `json:"status"`
	Message  string `json:"message,omitempty"`
}

// SearchRequest is the public input to SearchService.
// It mirrors the SLS USearch SPL `with(...)` parameter set so the same SPL
// can flow through both the open-source engine and the SLS backend.
type SearchRequest struct {
	Workspace  string             `json:"workspace,omitempty"`
	Source     string             `json:"source"`
	Domain     string             `json:"domain,omitempty"`
	Kinds      []string           `json:"kinds,omitempty"`
	Names      []string           `json:"names,omitempty"`
	Query      string             `json:"query,omitempty"`
	EmbedModel string             `json:"embed_model,omitempty"`
	TopK       int                `json:"topk,omitempty"`
	Origin     string             `json:"origin,omitempty"`
	Filters    map[string]any     `json:"filters,omitempty"`
	HybridK    int                `json:"hybrid_k,omitempty"`
	Weights    map[string]float64 `json:"weights,omitempty"`
}

// SearchResult is the public output of SearchService.
type SearchResult struct {
	Rows []SearchRow `json:"rows"`
}

// SearchRow mirrors the SLS USearch result row shape. Field tags use the
// `__field__` convention required by the SLS USearch contract.
type SearchRow struct {
	Type       string         `json:"__type__,omitempty"`
	Domain     string         `json:"__domain__,omitempty"`
	Kind       string         `json:"kind,omitempty"`
	Name       string         `json:"name,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Spec       map[string]any `json:"spec,omitempty"`
	Score      float64        `json:"__score__"`
	Provider   string         `json:"__provider__,omitempty"`
	EmbedModel string         `json:"__embedding_model__,omitempty"`
}

// AsMap renders a SearchRow as a map for QueryResult.Rows.
func (r SearchRow) AsMap() map[string]any {
	m := map[string]any{
		"__score__": r.Score,
	}
	if r.Type != "" {
		m["__type__"] = r.Type
	}
	if r.Domain != "" {
		m["__domain__"] = r.Domain
	}
	if r.Kind != "" {
		m["kind"] = r.Kind
	}
	if r.Name != "" {
		m["name"] = r.Name
	}
	if len(r.Metadata) > 0 {
		m["metadata"] = r.Metadata
	}
	if len(r.Spec) > 0 {
		m["spec"] = r.Spec
	}
	if r.Provider != "" {
		m["__provider__"] = r.Provider
	}
	if r.EmbedModel != "" {
		m["__embedding_model__"] = r.EmbedModel
	}
	return m
}

// SearchCapabilities advertises what a SearchService implementation supports.
type SearchCapabilities struct {
	VectorSearch         bool   `json:"vector_search"`
	HybridSearch         bool   `json:"hybrid_search"`
	FilteredVectorSearch bool   `json:"filtered_vector_search"`
	RRF                  bool   `json:"rrf"`
	ChunkChasing         bool   `json:"chunk_chasing"`
	EmbedderType         string `json:"embedder_type,omitempty"`
	MaxDim               int    `json:"max_dim,omitempty"`
}

// SearchHealth reports the runtime status of a SearchService implementation.
type SearchHealth struct {
	Provider string `json:"provider"`
	Status   string `json:"status"`
	Message  string `json:"message,omitempty"`
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

type AgentToolCallRequest struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type AgentToolCallResult struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Output any    `json:"output,omitempty"`
	Error  any    `json:"error,omitempty"`
}

type AgentResourceReadRequest struct {
	URI string `json:"uri"`
}

type AgentResourceReadResult struct {
	URI      string `json:"uri"`
	MIMEType string `json:"mime_type"`
	Content  any    `json:"content"`
}

type AgentDiscovery struct {
	Workspace   string            `json:"workspace"`
	Tools       []AgentTool       `json:"tools"`
	Resources   []AgentResource   `json:"resources"`
	NextActions []AgentNextAction `json:"next_actions,omitempty"`
}
