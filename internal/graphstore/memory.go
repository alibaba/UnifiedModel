package graphstore

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

type MemoryStore struct {
	mu        sync.RWMutex
	umodels   map[string]map[string]model.UModelElement
	entities  map[string]map[string]model.EntityPayload
	relations map[string]map[string]model.RelationPayload
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		umodels:   make(map[string]map[string]model.UModelElement),
		entities:  make(map[string]map[string]model.EntityPayload),
		relations: make(map[string]map[string]model.RelationPayload),
	}
}

func (s *MemoryStore) OpenWorkspace(ctx context.Context, workspace model.WorkspaceMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureWorkspaceLocked(workspace.ID)
	return nil
}

func (s *MemoryStore) EnsureSchema(ctx context.Context, workspace string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureWorkspaceLocked(workspace)
	return nil
}

func (s *MemoryStore) PutUModelElements(ctx context.Context, batch model.UModelElementBatch) (model.WriteResult, error) {
	if batch.Workspace == "" {
		return model.WriteResult{}, fmt.Errorf("workspace is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureWorkspaceLocked(batch.Workspace)

	items := make([]model.BatchItemResult, 0, len(batch.Elements))
	for _, element := range batch.Elements {
		key := model.UModelElementKey(element)
		s.umodels[batch.Workspace][key] = cloneUModelElement(element)
		items = append(items, model.BatchItemResult{ID: key, OK: true})
	}
	return model.WriteResult{Accepted: len(batch.Elements), Items: items}, nil
}

func (s *MemoryStore) GetUModelSnapshot(ctx context.Context, req model.UModelSnapshotRequest) (model.UModelSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	source := s.umodels[req.Workspace]
	elements := make([]model.UModelElement, 0, len(source))
	for _, element := range source {
		elements = append(elements, cloneUModelElement(element))
	}
	sort.Slice(elements, func(i, j int) bool {
		return model.UModelElementKey(elements[i]) < model.UModelElementKey(elements[j])
	})

	version := req.Version
	if version == "" {
		version = "memory"
	}
	return model.UModelSnapshot{Workspace: req.Workspace, Version: version, Elements: elements}, nil
}

func (s *MemoryStore) WriteEntities(ctx context.Context, batch model.EntityWriteBatch) (model.WriteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureWorkspaceLocked(batch.Workspace)

	items := make([]model.BatchItemResult, 0, len(batch.Entities))
	for _, payload := range batch.Entities {
		key := EntityKey(payload)
		method := methodOf(payload)
		existing, exists := s.entities[batch.Workspace][key]

		switch method {
		case "Create":
			if exists && !boolValue(existing["__deleted__"]) {
				items = append(items, writeFailure(key, apperrors.CodeAlreadyExists, "entity already exists"))
				continue
			}
			next := cloneEntityPayload(payload)
			next["__deleted__"] = false
			s.entities[batch.Workspace][key] = next
		case "Delete":
			if !exists {
				items = append(items, writeFailure(key, apperrors.CodeNotFound, "entity not found"))
				continue
			}
			next := cloneEntityPayload(existing)
			applyLifecycle(next, payload, "Delete", true)
			s.entities[batch.Workspace][key] = next
		case "Expire":
			if !exists {
				items = append(items, writeFailure(key, apperrors.CodeNotFound, "entity not found"))
				continue
			}
			next := cloneEntityPayload(existing)
			applyLifecycle(next, payload, "Expire", true)
			s.entities[batch.Workspace][key] = next
		default:
			next := cloneEntityPayload(payload)
			if exists {
				if first, ok := existing["__first_observed_time__"]; ok {
					next["__first_observed_time__"] = first
				}
			}
			next["__deleted__"] = false
			s.entities[batch.Workspace][key] = next
		}
		items = append(items, model.BatchItemResult{ID: key, OK: true})
	}
	return summarizeItems(items), nil
}

func (s *MemoryStore) WriteRelations(ctx context.Context, batch model.RelationWriteBatch) (model.WriteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureWorkspaceLocked(batch.Workspace)

	items := make([]model.BatchItemResult, 0, len(batch.Relations))
	for _, payload := range batch.Relations {
		key := RelationKey(payload)
		method := methodOf(payload)
		existing, exists := s.relations[batch.Workspace][key]

		switch method {
		case "Create":
			if exists && !boolValue(existing["__deleted__"]) {
				items = append(items, writeFailure(key, apperrors.CodeAlreadyExists, "relation already exists"))
				continue
			}
			next := cloneRelationPayload(payload)
			next["__deleted__"] = false
			s.relations[batch.Workspace][key] = next
		case "Delete":
			if !exists {
				items = append(items, writeFailure(key, apperrors.CodeNotFound, "relation not found"))
				continue
			}
			next := cloneRelationPayload(existing)
			applyLifecycle(next, payload, "Delete", true)
			s.relations[batch.Workspace][key] = next
		case "Expire":
			if !exists {
				items = append(items, writeFailure(key, apperrors.CodeNotFound, "relation not found"))
				continue
			}
			next := cloneRelationPayload(existing)
			applyLifecycle(next, payload, "Expire", true)
			s.relations[batch.Workspace][key] = next
		default:
			next := cloneRelationPayload(payload)
			if exists {
				if first, ok := existing["__first_observed_time__"]; ok {
					next["__first_observed_time__"] = first
				}
			}
			next["__deleted__"] = false
			s.relations[batch.Workspace][key] = next
		}
		items = append(items, model.BatchItemResult{ID: key, OK: true})
	}
	return summarizeItems(items), nil
}

func (s *MemoryStore) QueryEntities(ctx context.Context, plan model.EntityQueryPlan) (model.QueryResult, error) {
	limit := normalizeLimit(plan.Limit, 100)

	s.mu.RLock()
	defer s.mu.RUnlock()

	rows := make([]map[string]any, 0, allocHint(limit))
	keys := make([]string, 0, len(s.entities[plan.Workspace]))
	for key := range s.entities[plan.Workspace] {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		payload := s.entities[plan.Workspace][key]
		if !entityMatches(payload, plan) {
			continue
		}
		rows = append(rows, entityRow(payload))
		if len(rows) == limit {
			break
		}
	}

	return model.QueryResult{
		Columns: []string{"__domain__", "__entity_type__", "__entity_id__", "__method__", "__deleted__"},
		Rows:    rows,
		Page:    model.PageRequest{Limit: limit},
	}, nil
}

func (s *MemoryStore) QueryTopo(ctx context.Context, plan model.TopoQueryPlan) (model.QueryResult, error) {
	limit := normalizeLimit(plan.Limit, 100)

	s.mu.RLock()
	defer s.mu.RUnlock()

	if plan.GraphCall != nil && plan.GraphCall.Name == "cypher" {
		return s.queryCypherLocked(plan, limit)
	}

	rows := make([]map[string]any, 0, allocHint(limit))
	keys := make([]string, 0, len(s.relations[plan.Workspace]))
	for key := range s.relations[plan.Workspace] {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		payload := s.relations[plan.Workspace][key]
		if !relationMatches(payload, plan) {
			continue
		}
		rows = append(rows, relationRow(payload))
		if len(rows) == limit {
			break
		}
	}

	return model.QueryResult{
		Columns: []string{"src", "relation", "dest", "__relation_type__", "__deleted__"},
		Rows:    rows,
		Page:    model.PageRequest{Limit: limit},
	}, nil
}

func (s *MemoryStore) Capabilities(ctx context.Context) (model.GraphStoreCapabilities, error) {
	return model.GraphStoreCapabilities{
		EntitySearch:       true,
		GraphMatch:         false,
		GraphCallNeighbors: true,
		ControlledCypher:   true,
		TimeVisibility:     true,
		ServerSideFilter:   false,
		MaxDepth:           2,
		// Memory backs --quickstart / MCP / demos and has no storage backend, so
		// it serves a generous row ceiling. Kept >= the default local.ladybug
		// provider's 1000 so any production-valid `limit` is also valid here.
		MaxLimit: 10000,
		Timeout:  "10s",
	}, nil
}

func (s *MemoryStore) Health(ctx context.Context) (model.GraphStoreHealth, error) {
	return model.GraphStoreHealth{Provider: ProviderTypeMemory, Status: "ok"}, nil
}

func (s *MemoryStore) ensureWorkspaceLocked(workspace string) {
	if _, ok := s.umodels[workspace]; !ok {
		s.umodels[workspace] = make(map[string]model.UModelElement)
	}
	if _, ok := s.entities[workspace]; !ok {
		s.entities[workspace] = make(map[string]model.EntityPayload)
	}
	if _, ok := s.relations[workspace]; !ok {
		s.relations[workspace] = make(map[string]model.RelationPayload)
	}
}

func EntityKey(payload model.EntityPayload) string {
	return model.EntityStableKey(payload)
}

func RelationKey(payload model.RelationPayload) string {
	return model.RelationStableKey(payload)
}

func entityRow(payload model.EntityPayload) map[string]any {
	row := cloneMap(map[string]any(payload))
	row["__deleted__"] = boolValue(payload["__deleted__"])
	return row
}

func relationRow(payload model.RelationPayload) map[string]any {
	row := cloneMap(map[string]any(payload))
	row["src"] = strings.Join([]string{
		stringValue(payload["__src_domain__"]),
		stringValue(payload["__src_entity_type__"]),
		stringValue(payload["__src_entity_id__"]),
	}, "/")
	row["dest"] = strings.Join([]string{
		stringValue(payload["__dest_domain__"]),
		stringValue(payload["__dest_entity_type__"]),
		stringValue(payload["__dest_entity_id__"]),
	}, "/")
	row["relation"] = stringValue(payload["__relation_type__"])
	row["__deleted__"] = boolValue(payload["__deleted__"])
	return row
}

func entityMatches(payload model.EntityPayload, plan model.QueryPlan) bool {
	if methodOf(payload) == "Delete" {
		return false
	}
	if boolValue(payload["__deleted__"]) && !hasTimeRange(plan.TimeRange) {
		return false
	}
	if !matchesFilter(stringValue(payload["__domain__"]), plan.Filters["domain"]) {
		return false
	}
	if !matchesFilter(stringValue(payload["__entity_type__"]), plan.Filters["name"]) {
		return false
	}
	if !matchesIDs(stringValue(payload["__entity_id__"]), plan.Filters["ids"]) {
		return false
	}
	if !matchesSearch(payload, plan.Filters["query"]) {
		return false
	}
	return visibleInRange(payload, plan.TimeRange)
}

func relationMatches(payload model.RelationPayload, plan model.QueryPlan) bool {
	if methodOf(payload) == "Delete" {
		return false
	}
	if boolValue(payload["__deleted__"]) && !hasTimeRange(plan.TimeRange) {
		return false
	}
	if !matchesFilter(stringValue(payload["__relation_type__"]), firstFilter(plan.Filters["relation_type"], plan.Filters["type"])) {
		return false
	}
	if !matchesFilter(relationEndpoint(payload, "src"), plan.Filters["src"]) {
		return false
	}
	if !matchesFilter(relationEndpoint(payload, "dest"), plan.Filters["dest"]) {
		return false
	}
	if plan.GraphCall != nil && len(plan.GraphCall.SeedIDs) > 0 {
		srcID := stringValue(payload["__src_entity_id__"])
		destID := stringValue(payload["__dest_entity_id__"])
		if !containsID(plan.GraphCall.SeedIDs, srcID) && !containsID(plan.GraphCall.SeedIDs, destID) {
			return false
		}
	}
	if !matchesSearch(payload, plan.Filters["query"]) {
		return false
	}
	return visibleInRange(payload, plan.TimeRange)
}

func hasTimeRange(timeRange model.TimeRange) bool {
	return timeRange.From != nil || timeRange.To != nil
}

func applyLifecycle(target map[string]any, source map[string]any, method string, deleted bool) {
	target["__method__"] = method
	target["__deleted__"] = deleted
	for _, field := range []string{"__last_observed_time__", "__keep_alive_seconds__"} {
		if value, ok := source[field]; ok {
			target[field] = value
		}
	}
	if _, ok := target["__first_observed_time__"]; !ok {
		if value, hasValue := source["__first_observed_time__"]; hasValue {
			target["__first_observed_time__"] = value
		}
	}
}

func summarizeItems(items []model.BatchItemResult) model.WriteResult {
	result := model.WriteResult{Items: items}
	for _, item := range items {
		if item.OK {
			result.Accepted++
		} else {
			result.Failed++
		}
	}
	return result
}

func writeFailure(id string, code apperrors.Code, message string) model.BatchItemResult {
	return model.BatchItemResult{ID: id, OK: false, Code: string(code), Message: message}
}

func relationEndpoint(payload model.RelationPayload, side string) string {
	return strings.Join([]string{
		stringValue(payload["__"+side+"_domain__"]),
		stringValue(payload["__"+side+"_entity_type__"]),
		stringValue(payload["__"+side+"_entity_id__"]),
	}, "/")
}

func visibleInRange(payload map[string]any, timeRange model.TimeRange) bool {
	if timeRange.From == nil && timeRange.To == nil {
		return true
	}

	first, hasFirst := int64Value(payload["__first_observed_time__"])
	last, hasLast := int64Value(payload["__last_observed_time__"])
	if !hasFirst || !hasLast {
		return true
	}

	keepAlive, _ := int64Value(payload["__keep_alive_seconds__"])
	from := int64(0)
	if timeRange.From != nil {
		from = timeRange.From.Unix()
	}
	to := time.Now().Add(100 * 365 * 24 * time.Hour).Unix()
	if timeRange.To != nil {
		to = timeRange.To.Unix()
	}
	if first >= to {
		return false
	}
	if boolValue(payload["__deleted__"]) {
		return last > from
	}
	return last+keepAlive > from
}

func matchesFilter(value string, filter any) bool {
	if filter == nil || stringValue(filter) == "" || stringValue(filter) == "*" {
		return true
	}
	pattern := stringValue(filter)
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(value, strings.TrimSuffix(pattern, "*"))
	}
	return value == pattern
}

func matchesIDs(value string, filter any) bool {
	if filter == nil {
		return true
	}
	switch ids := filter.(type) {
	case []string:
		for _, id := range ids {
			if id == value {
				return true
			}
		}
		return false
	case []any:
		for _, id := range ids {
			if stringValue(id) == value {
				return true
			}
		}
		return false
	default:
		return stringValue(filter) == "" || stringValue(filter) == value
	}
}

func matchesSearch(payload map[string]any, filter any) bool {
	query := strings.ToLower(stringValue(filter))
	if query == "" {
		return true
	}
	for _, value := range payload {
		if strings.Contains(strings.ToLower(stringValue(value)), query) {
			return true
		}
	}
	return false
}

func firstFilter(values ...any) any {
	for _, value := range values {
		if stringValue(value) != "" {
			return value
		}
	}
	return nil
}

func containsID(ids []string, value string) bool {
	for _, id := range ids {
		if id == value {
			return true
		}
	}
	return false
}

func methodOf(payload map[string]any) string {
	method := stringValue(payload["__method__"])
	if method == "" {
		return "Update"
	}
	return method
}

func normalizeLimit(limit, fallback int) int {
	if limit <= 0 {
		return fallback
	}
	return limit
}

// maxPreallocRows bounds the initial capacity we reserve for a result slice.
// The slice still grows via append, so this never truncates results — it only
// stops a caller-supplied limit from driving a huge up-front allocation
// (defense in depth; the planner already caps limit against provider
// capability before a request reaches the store).
const maxPreallocRows = 1024

func allocHint(limit int) int {
	if limit < 0 {
		return 0
	}
	if limit > maxPreallocRows {
		return maxPreallocRows
	}
	return limit
}

func stringValue(value any) string {
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

func int64Value(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case int32:
		return int64(typed), true
	case float64:
		return int64(typed), true
	case float32:
		return int64(typed), true
	case string:
		if typed == "" {
			return 0, false
		}
		var n int64
		_, err := fmt.Sscan(typed, &n)
		return n, err == nil
	default:
		return 0, false
	}
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(typed, "true")
	default:
		return false
	}
}

func cloneUModelElement(element model.UModelElement) model.UModelElement {
	element.Spec = cloneMap(element.Spec)
	return element
}

func cloneEntityPayload(payload model.EntityPayload) model.EntityPayload {
	return model.EntityPayload(cloneMap(map[string]any(payload)))
}

func cloneRelationPayload(payload model.RelationPayload) model.RelationPayload {
	return model.RelationPayload(cloneMap(map[string]any(payload)))
}

func cloneMap(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	target := make(map[string]any, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}
