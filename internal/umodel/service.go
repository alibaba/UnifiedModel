package umodel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	searchcontract "github.com/alibaba/UnifiedModel/internal/search/contract"
	"github.com/alibaba/UnifiedModel/internal/umodel/schemaspec"
	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

type graphStore interface {
	PutUModelElements(ctx context.Context, batch model.UModelElementBatch) (model.WriteResult, error)
	GetUModelSnapshot(ctx context.Context, req model.UModelSnapshotRequest) (model.UModelSnapshot, error)
}

type searchIndexer interface {
	Index(ctx context.Context, workspace string, chunks []searchcontract.Chunk) error
	DeleteByDocID(ctx context.Context, workspace string, docIDs []string) error
}

// SchemaValidator is the contract Service uses to enforce schema-driven
// validation on UModel elements. schemaspec.Validator implements it.
type SchemaValidator interface {
	Validate(element model.UModelElement) (schemaspec.Result, error)
}

type Service struct {
	graph              graphStore
	validator          SchemaValidator
	mu                 sync.RWMutex
	indexes            map[string]*schemaIndex
	commonSchemaLoader CommonSchemaLoader
	search             searchIndexer
}

// Option configures Service.
type Option func(*Service)

// WithValidator overrides the schema validator. Pass
// schemaspec.NewNoopValidator() in tests that intentionally exercise specs
// outside the schema (e.g. graphstore round-trip tests).
func WithValidator(v SchemaValidator) Option {
	return func(s *Service) { s.validator = v }
}

// WithSearchIndexer connects UModel writes to the runtime search index.
func WithSearchIndexer(indexer searchIndexer) Option {
	return func(s *Service) { s.search = indexer }
}

func NewService(graph graphStore, opts ...Option) *Service {
	s := &Service{
		graph:     graph,
		validator: schemaspec.DefaultValidator(),
		indexes:   make(map[string]*schemaIndex),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type CommonSchemaLoader interface {
	LoadCommonSchemaPacks(ctx context.Context, workspace string, packs []string) ([]model.UModelElement, error)
}

func (s *Service) SetCommonSchemaLoader(loader CommonSchemaLoader) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commonSchemaLoader = loader
}

func (s *Service) Validate(ctx context.Context, workspace string, elements []model.UModelElement) (model.ValidationResult, error) {
	var errors, warnings []model.ErrorDetail
	for i, element := range elements {
		if element.Kind == "" || element.Domain == "" || element.Name == "" {
			errors = append(errors, model.ErrorDetail{
				Field:  fmt.Sprintf("elements[%d].kind/domain/name", i),
				Reason: "umodel element kind, domain, and name are required",
			})
			continue
		}
		if s.validator == nil {
			continue
		}
		res, err := s.validator.Validate(element)
		if err != nil {
			errors = append(errors, model.ErrorDetail{
				Field:  fmt.Sprintf("elements[%d].kind", i),
				Reason: err.Error(),
			})
			continue
		}
		for _, e := range res.Errors {
			errors = append(errors, model.ErrorDetail{
				Field:  fmt.Sprintf("elements[%d].%s", i, e.Path),
				Reason: e.Reason,
			})
		}
		for _, w := range res.Warnings {
			warnings = append(warnings, model.ErrorDetail{
				Field:  fmt.Sprintf("elements[%d].%s", i, w.Path),
				Reason: w.Reason,
			})
		}
	}
	return model.ValidationResult{
		Valid:    len(errors) == 0,
		Errors:   errors,
		Warnings: warnings,
	}, nil
}

func (s *Service) PutElements(ctx context.Context, batch model.UModelElementBatch) (model.WriteResult, error) {
	if batch.Workspace == "" {
		return model.WriteResult{}, apperrors.New(apperrors.CodeInvalidArgument, "workspace is required")
	}
	validation, err := s.Validate(ctx, batch.Workspace, batch.Elements)
	if err != nil {
		return model.WriteResult{}, err
	}
	if !validation.Valid {
		details := map[string]string{
			"field":  validation.Errors[0].Field,
			"reason": validation.Errors[0].Reason,
		}
		if len(validation.Errors) > 1 {
			details["additional_errors"] = fmt.Sprintf("%d", len(validation.Errors)-1)
		}
		return model.WriteResult{}, apperrors.WithDetails(apperrors.CodeValidationFailed, "umodel validation failed", details)
	}
	result, err := s.graph.PutUModelElements(ctx, batch)
	if err != nil {
		return model.WriteResult{}, err
	}
	s.mergeIndex(batch.Workspace, batch.Elements)
	result.Warnings = append(result.Warnings, validation.Warnings...)
	if s.search != nil {
		if err := s.search.Index(ctx, batch.Workspace, umodelSearchChunks(batch.Elements)); err != nil {
			result.Warnings = append(result.Warnings, model.ErrorDetail{
				Field:  "search_index",
				Reason: err.Error(),
			})
		}
	}
	return result, nil
}

func (s *Service) DeleteElements(ctx context.Context, workspace string, ids []string) (model.WriteResult, error) {
	items := make([]model.BatchItemResult, 0, len(ids))
	for _, id := range ids {
		items = append(items, model.BatchItemResult{
			ID:      id,
			OK:      false,
			Code:    string(apperrors.CodeNotImplemented),
			Message: "delete dependency checks are not implemented in the current service",
		})
	}
	return model.WriteResult{Failed: len(items), Items: items}, nil
}

func (s *Service) RebuildIndex(ctx context.Context, workspace string) error {
	if workspace == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "workspace is required")
	}
	snapshot, err := s.graph.GetUModelSnapshot(ctx, model.UModelSnapshotRequest{Workspace: workspace})
	if err != nil {
		return err
	}
	s.replaceIndex(workspace, snapshot.Elements)
	return nil
}

func (s *Service) ResolveEntitySet(ctx context.Context, ref model.EntityTypeRef) (model.EntitySetSchema, error) {
	if ref.Domain == "" || ref.Name == "" {
		return model.EntitySetSchema{}, apperrors.New(apperrors.CodeInvalidArgument, "entity set ref requires domain and name")
	}
	if element, ok := s.findIndexedElement("entity_set", ref.Domain, ref.Name); ok {
		return model.EntitySetSchema{Ref: ref, Fields: fieldMapFromElement(element)}, nil
	}
	return model.EntitySetSchema{Ref: ref}, nil
}

func (s *Service) ResolveRelationType(ctx context.Context, ref model.RelationTypeRef) (model.RelationSchema, error) {
	if ref.Type == "" {
		return model.RelationSchema{}, apperrors.New(apperrors.CodeInvalidArgument, "relation type is required")
	}
	if element, ok := s.findIndexedRelation(ref); ok {
		return model.RelationSchema{Ref: ref, Fields: fieldMapFromElement(element)}, nil
	}
	return model.RelationSchema{Ref: ref}, nil
}

func (s *Service) ValidateEntityPayload(ctx context.Context, payload model.EntityPayload) (model.ValidationResult, error) {
	required := []string{"__domain__", "__entity_type__", "__entity_id__", "__method__", "__first_observed_time__", "__last_observed_time__"}
	for _, field := range required {
		if payload[field] == nil || payload[field] == "" {
			return model.ValidationResult{
				Valid:  false,
				Errors: []model.ErrorDetail{{Field: field, Reason: "required CMS 2.0 entity field is missing"}},
			}, nil
		}
	}
	if id, ok := payload["__entity_id__"].(string); !ok || !model.IsEntityID(id) {
		return model.ValidationResult{
			Valid:  false,
			Errors: []model.ErrorDetail{{Field: "__entity_id__", Reason: "entity id must be a 128-bit lowercase hex string"}},
		}, nil
	}
	if !validWriteMethod(payload["__method__"]) {
		return model.ValidationResult{
			Valid:  false,
			Errors: []model.ErrorDetail{{Field: "__method__", Reason: "method must be Create, Update, Expire, or Delete"}},
		}, nil
	}
	return model.ValidationResult{Valid: true}, nil
}

func (s *Service) ValidateRelationPayload(ctx context.Context, payload model.RelationPayload) (model.ValidationResult, error) {
	required := []string{"__src_domain__", "__src_entity_type__", "__src_entity_id__", "__dest_domain__", "__dest_entity_type__", "__dest_entity_id__", "__relation_type__", "__method__", "__first_observed_time__", "__last_observed_time__"}
	for _, field := range required {
		if payload[field] == nil || payload[field] == "" {
			return model.ValidationResult{
				Valid:  false,
				Errors: []model.ErrorDetail{{Field: field, Reason: "required CMS 2.0 relation field is missing"}},
			}, nil
		}
	}
	for _, field := range []string{"__src_entity_id__", "__dest_entity_id__"} {
		if id, ok := payload[field].(string); !ok || !model.IsEntityID(id) {
			return model.ValidationResult{
				Valid:  false,
				Errors: []model.ErrorDetail{{Field: field, Reason: "entity id must be a 128-bit lowercase hex string"}},
			}, nil
		}
	}
	if !validWriteMethod(payload["__method__"]) {
		return model.ValidationResult{
			Valid:  false,
			Errors: []model.ErrorDetail{{Field: "__method__", Reason: "method must be Create, Update, Expire, or Delete"}},
		}, nil
	}
	return model.ValidationResult{Valid: true}, nil
}

func (s *Service) SnapshotVersion(ctx context.Context, workspace string) (model.SchemaVersion, error) {
	snapshot, err := s.graph.GetUModelSnapshot(ctx, model.UModelSnapshotRequest{Workspace: workspace})
	if err != nil {
		return model.SchemaVersion{}, err
	}
	return model.SchemaVersion{Workspace: workspace, Version: snapshot.Version}, nil
}

func validWriteMethod(value any) bool {
	method, ok := value.(string)
	if !ok {
		return false
	}
	switch method {
	case "Create", "Update", "Expire", "Delete":
		return true
	default:
		return false
	}
}

func umodelSearchChunks(elements []model.UModelElement) []searchcontract.Chunk {
	chunks := make([]searchcontract.Chunk, 0, len(elements))
	for _, element := range elements {
		key := model.UModelElementKey(element)
		if key == "" {
			continue
		}
		chunks = append(chunks, searchcontract.Chunk{
			DocID:  "umodel/" + key,
			Source: ".umodel",
			Kind:   element.Kind,
			Domain: element.Domain,
			Name:   element.Name,
			Text:   searchText(element.Kind, element.Domain, element.Name, element.Version, element.Spec),
			Metadata: map[string]any{
				"version": element.Version,
			},
			Spec: element.Spec,
		})
		if element.Kind != "runbook_set" {
			continue
		}
		for _, section := range []string{"knowledge", "observations", "actions", "automations", "skills"} {
			value, ok := element.Spec[section]
			if !ok || value == nil {
				continue
			}
			chunks = append(chunks, searchcontract.Chunk{
				DocID:  "runbook_set/" + key + "/" + section,
				Source: ".runbook_set",
				Kind:   element.Kind,
				Domain: element.Domain,
				Name:   element.Name,
				Text:   searchText(element.Kind, element.Domain, element.Name, section, value),
				Attrs: map[string]any{
					"type": section,
				},
				Metadata: map[string]any{
					"type":    section,
					"version": element.Version,
				},
				Spec: map[string]any{
					section: value,
				},
			})
		}
	}
	return chunks
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
