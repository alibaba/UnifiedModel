package entitystore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	searchcontract "github.com/alibaba/UnifiedModel/internal/search/contract"
	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

type graphStore interface {
	WriteEntities(ctx context.Context, batch model.EntityWriteBatch) (model.WriteResult, error)
	WriteRelations(ctx context.Context, batch model.RelationWriteBatch) (model.WriteResult, error)
}

type schemaResolver interface {
	ValidateEntityPayload(ctx context.Context, payload model.EntityPayload) (model.ValidationResult, error)
	ValidateRelationPayload(ctx context.Context, payload model.RelationPayload) (model.ValidationResult, error)
}

type searchIndexer interface {
	Index(ctx context.Context, workspace string, chunks []searchcontract.Chunk) error
	DeleteByDocID(ctx context.Context, workspace string, docIDs []string) error
}

type Option func(*Service)

func WithSearchIndexer(indexer searchIndexer) Option {
	return func(s *Service) { s.search = indexer }
}

type Service struct {
	graph    graphStore
	resolver schemaResolver
	search   searchIndexer

	mu                  sync.Mutex
	entityIdempotency   map[string]model.WriteResult
	relationIdempotency map[string]model.WriteResult
}

func NewService(graph graphStore, resolver schemaResolver, opts ...Option) *Service {
	s := &Service{
		graph:               graph,
		resolver:            resolver,
		entityIdempotency:   make(map[string]model.WriteResult),
		relationIdempotency: make(map[string]model.WriteResult),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) WriteEntities(ctx context.Context, workspace string, batch model.EntityWriteBatch) (model.WriteResult, error) {
	batch.Workspace = workspace
	if cached, ok := s.cachedEntityResult(workspace, batch.IdempotencyKey); ok {
		return cached, nil
	}

	var validationResult model.WriteResult
	valid := model.EntityWriteBatch{
		Workspace:      workspace,
		IdempotencyKey: batch.IdempotencyKey,
		PartialSuccess: batch.PartialSuccess,
		Entities:       make([]model.EntityPayload, 0, len(batch.Entities)),
	}
	for _, payload := range batch.Entities {
		result, err := s.resolver.ValidateEntityPayload(ctx, payload)
		if err != nil {
			return model.WriteResult{}, err
		}
		if !result.Valid {
			item := failedItem(model.EntityStableKey(payload), apperrors.CodeValidationFailed, "entity payload validation failed", result.Errors)
			if !batch.PartialSuccess {
				return model.WriteResult{}, validationError("entity payload validation failed", result.Errors)
			}
			validationResult.Failed++
			validationResult.Items = append(validationResult.Items, item)
			continue
		}
		valid.Entities = append(valid.Entities, payload)
	}

	graphResult := model.WriteResult{}
	if len(valid.Entities) > 0 {
		var err error
		graphResult, err = s.graph.WriteEntities(ctx, valid)
		if err != nil {
			return model.WriteResult{}, err
		}
		s.indexEntities(ctx, workspace, valid.Entities, &graphResult)
	}
	result := mergeWriteResults(graphResult, validationResult)
	if result.Failed > 0 && !batch.PartialSuccess {
		return result, itemFailureError("entity", result.Failed)
	}
	s.rememberEntityResult(workspace, batch.IdempotencyKey, result)
	return result, nil
}

func (s *Service) WriteRelations(ctx context.Context, workspace string, batch model.RelationWriteBatch) (model.WriteResult, error) {
	batch.Workspace = workspace
	if cached, ok := s.cachedRelationResult(workspace, batch.IdempotencyKey); ok {
		return cached, nil
	}

	var validationResult model.WriteResult
	valid := model.RelationWriteBatch{
		Workspace:      workspace,
		IdempotencyKey: batch.IdempotencyKey,
		PartialSuccess: batch.PartialSuccess,
		Relations:      make([]model.RelationPayload, 0, len(batch.Relations)),
	}
	for _, payload := range batch.Relations {
		result, err := s.resolver.ValidateRelationPayload(ctx, payload)
		if err != nil {
			return model.WriteResult{}, err
		}
		if !result.Valid {
			item := failedItem(model.RelationStableKey(payload), apperrors.CodeValidationFailed, "relation payload validation failed", result.Errors)
			if !batch.PartialSuccess {
				return model.WriteResult{}, validationError("relation payload validation failed", result.Errors)
			}
			validationResult.Failed++
			validationResult.Items = append(validationResult.Items, item)
			continue
		}
		valid.Relations = append(valid.Relations, payload)
	}

	graphResult := model.WriteResult{}
	if len(valid.Relations) > 0 {
		var err error
		graphResult, err = s.graph.WriteRelations(ctx, valid)
		if err != nil {
			return model.WriteResult{}, err
		}
	}
	result := mergeWriteResults(graphResult, validationResult)
	if result.Failed > 0 && !batch.PartialSuccess {
		return result, itemFailureError("relation", result.Failed)
	}
	s.rememberRelationResult(workspace, batch.IdempotencyKey, result)
	return result, nil
}

func (s *Service) ExpireEntities(ctx context.Context, workspace string, req model.ExpireRequest) (model.WriteResult, error) {
	req.Workspace = workspace
	now := time.Now().Unix()
	var parseResult model.WriteResult
	payloads := make([]model.EntityPayload, 0, len(req.IDs))
	for _, id := range req.IDs {
		payload, ok := entityPayloadFromStableKey(id, now)
		if !ok {
			parseResult.Failed++
			parseResult.Items = append(parseResult.Items, failedItem(id, apperrors.CodeValidationFailed, "entity id must be a stable key", nil))
			continue
		}
		payloads = append(payloads, payload)
	}
	graphResult := model.WriteResult{}
	if len(payloads) > 0 {
		var err error
		graphResult, err = s.graph.WriteEntities(ctx, model.EntityWriteBatch{Workspace: workspace, PartialSuccess: true, Entities: payloads})
		if err != nil {
			return model.WriteResult{}, err
		}
		s.deleteEntitiesFromSearch(ctx, workspace, req.IDs, &graphResult)
	}
	return mergeWriteResults(graphResult, parseResult), nil
}

func (s *Service) ExpireRelations(ctx context.Context, workspace string, req model.ExpireRequest) (model.WriteResult, error) {
	req.Workspace = workspace
	now := time.Now().Unix()
	var parseResult model.WriteResult
	payloads := make([]model.RelationPayload, 0, len(req.IDs))
	for _, id := range req.IDs {
		payload, ok := relationPayloadFromStableKey(id, now)
		if !ok {
			parseResult.Failed++
			parseResult.Items = append(parseResult.Items, failedItem(id, apperrors.CodeValidationFailed, "relation id must be a stable key", nil))
			continue
		}
		payloads = append(payloads, payload)
	}
	graphResult := model.WriteResult{}
	if len(payloads) > 0 {
		var err error
		graphResult, err = s.graph.WriteRelations(ctx, model.RelationWriteBatch{Workspace: workspace, PartialSuccess: true, Relations: payloads})
		if err != nil {
			return model.WriteResult{}, err
		}
	}
	return mergeWriteResults(graphResult, parseResult), nil
}

func (s *Service) RunTTL(ctx context.Context, workspace string, now time.Time) (model.WriteResult, error) {
	if workspace == "" {
		return model.WriteResult{}, apperrors.New(apperrors.CodeInvalidArgument, "workspace is required")
	}
	return model.WriteResult{}, nil
}

func (s *Service) cachedEntityResult(workspace, key string) (model.WriteResult, bool) {
	if key == "" {
		return model.WriteResult{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	result, ok := s.entityIdempotency[idempotencyKey(workspace, key)]
	return cloneWriteResult(result), ok
}

func (s *Service) cachedRelationResult(workspace, key string) (model.WriteResult, bool) {
	if key == "" {
		return model.WriteResult{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	result, ok := s.relationIdempotency[idempotencyKey(workspace, key)]
	return cloneWriteResult(result), ok
}

func (s *Service) rememberEntityResult(workspace, key string, result model.WriteResult) {
	if key == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entityIdempotency[idempotencyKey(workspace, key)] = cloneWriteResult(result)
}

func (s *Service) rememberRelationResult(workspace, key string, result model.WriteResult) {
	if key == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.relationIdempotency[idempotencyKey(workspace, key)] = cloneWriteResult(result)
}

func idempotencyKey(workspace, key string) string {
	return workspace + "\x00" + key
}

func validationError(message string, details []model.ErrorDetail) error {
	fields := map[string]string{}
	if len(details) > 0 {
		fields["field"] = details[0].Field
	}
	return apperrors.WithDetails(apperrors.CodeValidationFailed, message, fields)
}

func failedItem(id string, code apperrors.Code, message string, details []model.ErrorDetail) model.BatchItemResult {
	return model.BatchItemResult{
		ID:      id,
		OK:      false,
		Code:    string(code),
		Message: message,
		Details: cloneErrorDetails(details),
	}
}

func (s *Service) indexEntities(ctx context.Context, workspace string, payloads []model.EntityPayload, result *model.WriteResult) {
	if s.search == nil {
		return
	}
	chunks, deleteIDs := entitySearchIndexBatch(payloads)
	if len(deleteIDs) > 0 {
		s.deleteEntitiesFromSearch(ctx, workspace, deleteIDs, result)
	}
	if len(chunks) == 0 {
		return
	}
	if err := s.search.Index(ctx, workspace, chunks); err != nil {
		result.Warnings = append(result.Warnings, model.ErrorDetail{
			Field:  "search_index",
			Reason: err.Error(),
		})
	}
}

func (s *Service) deleteEntitiesFromSearch(ctx context.Context, workspace string, ids []string, result *model.WriteResult) {
	if s.search == nil || len(ids) == 0 {
		return
	}
	docIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		docIDs = append(docIDs, "entity/"+id)
	}
	if len(docIDs) == 0 {
		return
	}
	if err := s.search.DeleteByDocID(ctx, workspace, docIDs); err != nil {
		result.Warnings = append(result.Warnings, model.ErrorDetail{
			Field:  "search_index",
			Reason: err.Error(),
		})
	}
}

func entitySearchIndexBatch(payloads []model.EntityPayload) ([]searchcontract.Chunk, []string) {
	chunks := make([]searchcontract.Chunk, 0, len(payloads))
	deleteIDs := make([]string, 0)
	for _, payload := range payloads {
		id := model.EntityStableKey(payload)
		if id == "" {
			continue
		}
		if entitySearchDeletes(payload) {
			deleteIDs = append(deleteIDs, id)
			continue
		}
		domain := entityPayloadString(payload["__domain__"])
		entityType := entityPayloadString(payload["__entity_type__"])
		entityID := entityPayloadString(payload["__entity_id__"])
		method := entityPayloadString(payload["__method__"])
		chunks = append(chunks, searchcontract.Chunk{
			DocID:  "entity/" + id,
			Source: ".entity",
			Kind:   entityType,
			Domain: domain,
			Name:   entityType,
			Text:   searchText(domain, entityType, entityID, method, payload),
			Attrs: map[string]any{
				"__entity_id__": entityID,
				"__method__":    method,
			},
			Metadata: map[string]any{
				"__entity_id__": entityID,
				"__method__":    method,
			},
			Spec: payload,
		})
	}
	return chunks, deleteIDs
}

func entitySearchDeletes(payload model.EntityPayload) bool {
	switch strings.ToLower(entityPayloadString(payload["__method__"])) {
	case "expire", "delete":
		return true
	default:
		return false
	}
}

func entityPayloadString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

func searchText(parts ...any) string {
	var builder strings.Builder
	for _, part := range parts {
		if part == nil {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteByte(' ')
		}
		switch typed := part.(type) {
		case string:
			builder.WriteString(typed)
		default:
			body, err := json.Marshal(typed)
			if err != nil {
				builder.WriteString(fmt.Sprint(typed))
				continue
			}
			builder.Write(body)
		}
	}
	return builder.String()
}

func mergeWriteResults(results ...model.WriteResult) model.WriteResult {
	var merged model.WriteResult
	for _, result := range results {
		merged.Accepted += result.Accepted
		merged.Failed += result.Failed
		merged.Items = append(merged.Items, result.Items...)
		merged.Warnings = append(merged.Warnings, result.Warnings...)
	}
	return merged
}

func entityPayloadFromStableKey(key string, now int64) (model.EntityPayload, bool) {
	parts := strings.Split(key, "/")
	if len(parts) != 3 {
		return nil, false
	}
	for _, part := range parts {
		if part == "" {
			return nil, false
		}
	}
	if !model.IsEntityID(parts[2]) {
		return nil, false
	}
	return model.EntityPayload{
		"__domain__":              parts[0],
		"__entity_type__":         parts[1],
		"__entity_id__":           parts[2],
		"__method__":              "Expire",
		"__first_observed_time__": int64(0),
		"__last_observed_time__":  now,
	}, true
}

func relationPayloadFromStableKey(key string, now int64) (model.RelationPayload, bool) {
	parts := strings.Split(key, "/")
	if len(parts) != 7 {
		return nil, false
	}
	for _, part := range parts {
		if part == "" {
			return nil, false
		}
	}
	if !model.IsEntityID(parts[2]) || !model.IsEntityID(parts[6]) {
		return nil, false
	}
	return model.RelationPayload{
		"__src_domain__":          parts[0],
		"__src_entity_type__":     parts[1],
		"__src_entity_id__":       parts[2],
		"__relation_type__":       parts[3],
		"__dest_domain__":         parts[4],
		"__dest_entity_type__":    parts[5],
		"__dest_entity_id__":      parts[6],
		"__method__":              "Expire",
		"__first_observed_time__": int64(0),
		"__last_observed_time__":  now,
	}, true
}

func cloneWriteResult(result model.WriteResult) model.WriteResult {
	result.Items = append([]model.BatchItemResult(nil), result.Items...)
	for i := range result.Items {
		result.Items[i].Details = cloneErrorDetails(result.Items[i].Details)
	}
	result.Warnings = cloneErrorDetails(result.Warnings)
	return result
}

func cloneErrorDetails(details []model.ErrorDetail) []model.ErrorDetail {
	if details == nil {
		return nil
	}
	return append([]model.ErrorDetail(nil), details...)
}

func itemFailureError(entity string, failed int) error {
	return apperrors.New(apperrors.CodePartialFailed, fmt.Sprintf("%s batch contains %d failed item(s)", entity, failed))
}
