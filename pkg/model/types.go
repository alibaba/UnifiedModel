package model

import (
	"fmt"
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
	Query     string         `json:"query"`
	TimeRange TimeRange      `json:"time_range,omitempty"`
	Format    string         `json:"format,omitempty"`
	Limit     int            `json:"limit,omitempty"`
	TimeoutMS int            `json:"timeout_ms,omitempty"`
	Params    map[string]any `json:"parameters,omitempty"`
}

type QueryResult struct {
	Columns []string         `json:"columns"`
	Rows    []map[string]any `json:"rows"`
	Page    PageRequest      `json:"page"`
	Explain *QueryExplain    `json:"explain,omitempty"`
}

type QueryExplain struct {
	Source           string   `json:"source"`
	Provider         string   `json:"provider,omitempty"`
	StorageProvider  string   `json:"storage_provider,omitempty"`
	CypherDialect    string   `json:"cypher_dialect,omitempty"`
	CypherEngine     string   `json:"cypher_engine,omitempty"`
	Pushdown         []string `json:"pushdown,omitempty"`
	Fallback         []string `json:"fallback,omitempty"`
	Operators        []string `json:"operators,omitempty"`
	Depth            int      `json:"depth,omitempty"`
	Limit            int      `json:"limit,omitempty"`
	TimeoutMS        int      `json:"timeout_ms,omitempty"`
	TimeRangeApplied bool     `json:"time_range_applied"`
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

type QueryPipelineOperator struct {
	Name       string          `json:"name,omitempty"`
	Expression string          `json:"expression,omitempty"`
	Predicate  *QueryPredicate `json:"predicate,omitempty"`
	Project    []string        `json:"project,omitempty"`
	Sort       *QuerySort      `json:"sort,omitempty"`
	GraphCall  *GraphCallPlan  `json:"graph_call,omitempty"`
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
	TopK       int
	TimeRange  TimeRange
	Params     map[string]any
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
