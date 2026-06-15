package sampledata

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/alibaba/UnifiedModel/internal/entitystore"
	"github.com/alibaba/UnifiedModel/internal/umodel"
	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

const (
	MultiDomainQuickStartSample = "multi-domain-quickstart"
)

type sampleDefinition struct {
	Name         string
	Aliases      []string
	SchemaRoot   string
	EntityFile   string
	RelationFile string
}

var sampleCatalog = []sampleDefinition{
	{
		Name:         MultiDomainQuickStartSample,
		Aliases:      []string{"quickstart-multidomain", "quickstart"},
		SchemaRoot:   "examples/quickstart-multidomain",
		EntityFile:   "examples/quickstart-multidomain/sample-data/entities.json",
		RelationFile: "examples/quickstart-multidomain/sample-data/relations.json",
	},
	{
		Name:         "incident-investigation",
		Aliases:      []string{"incident-inv", "payment-gateway-demo", "examples/incident-investigation"},
		SchemaRoot:   "examples/incident-investigation",
		EntityFile:   "examples/incident-investigation/sample-data/entities.json",
		RelationFile: "examples/incident-investigation/sample-data/relations.json",
	},
	{
		Name:         "service-localization",
		Aliases:      []string{"bottleneck-localization", "examples/service-localization"},
		SchemaRoot:   "examples/service-localization",
		EntityFile:   "examples/service-localization/sample-data/entities.json",
		RelationFile: "examples/service-localization/sample-data/relations.json",
	},
}

type Service struct {
	umodel      *umodel.Service
	entityStore *entitystore.Service
}

func NewService(umodelSvc *umodel.Service, entitySvc *entitystore.Service) *Service {
	return &Service{
		umodel:      umodelSvc,
		entityStore: entitySvc,
	}
}

func (s *Service) Import(ctx context.Context, workspace string, sample string) (model.SampleImportResult, error) {
	def, ok := lookupSample(sample)
	if !ok {
		return model.SampleImportResult{}, apperrors.WithDetails(apperrors.CodeNotFound, "sample not found", map[string]string{
			"sample":    sample,
			"available": strings.Join(AvailableSamples(), ","),
		})
	}
	return s.importPack(ctx, workspace, def)
}

func (s *Service) ImportMultiDomainQuickStart(ctx context.Context, workspace string) (model.SampleImportResult, error) {
	def, _ := lookupSample(MultiDomainQuickStartSample)
	return s.importPack(ctx, workspace, def)
}

func AvailableSamples() []string {
	names := make([]string, 0, len(sampleCatalog))
	for _, def := range sampleCatalog {
		names = append(names, def.Name)
	}
	return names
}

func lookupSample(sample string) (sampleDefinition, bool) {
	normalized := normalizeSampleName(sample)
	for _, def := range sampleCatalog {
		if normalizeSampleName(def.Name) == normalized {
			return def, true
		}
		for _, alias := range def.Aliases {
			if normalizeSampleName(alias) == normalized {
				return def, true
			}
		}
	}
	return sampleDefinition{}, false
}

func normalizeSampleName(sample string) string {
	return strings.ToLower(strings.TrimSpace(sample))
}

func (s *Service) importPack(ctx context.Context, workspace string, def sampleDefinition) (model.SampleImportResult, error) {
	if workspace == "" {
		return model.SampleImportResult{}, apperrors.New(apperrors.CodeInvalidArgument, "workspace is required")
	}
	schemaRoot, err := repoPath(def.SchemaRoot)
	if err != nil {
		return model.SampleImportResult{}, err
	}
	entityPath, err := repoPath(def.EntityFile)
	if err != nil {
		return model.SampleImportResult{}, err
	}
	topoPath, err := repoPath(def.RelationFile)
	if err != nil {
		return model.SampleImportResult{}, err
	}

	result := model.SampleImportResult{
		Workspace: workspace,
		Sample:    def.Name,
	}
	// Bundled sample packs are resolved from the repository (repoPath); the
	// path is trusted, so it bypasses import-root confinement and loads
	// regardless of the server's working directory.
	umodelResult, err := s.umodel.ImportTrusted(ctx, workspace, model.UModelImportRequest{Path: schemaRoot})
	if err != nil {
		return result, err
	}
	result.UModel = umodelResult

	entities, err := loadPayloads[model.EntityPayload](entityPath)
	if err != nil {
		return result, err
	}
	entityResult, err := s.entityStore.WriteEntities(ctx, workspace, model.EntityWriteBatch{
		IdempotencyKey: def.Name + ":entities:v1",
		Entities:       entities,
	})
	if err != nil {
		return result, err
	}
	result.Entities = entityResult
	result.EntityCount = len(entities)
	if entityResult.Failed > 0 {
		return result, writeFailureError("sample entity import failed", entityResult)
	}

	relations, err := loadPayloads[model.RelationPayload](topoPath)
	if err != nil {
		return result, err
	}
	relationResult, err := s.entityStore.WriteRelations(ctx, workspace, model.RelationWriteBatch{
		IdempotencyKey: def.Name + ":relations:v1",
		Relations:      relations,
	})
	if err != nil {
		return result, err
	}
	result.Relations = relationResult
	result.RelationCount = len(relations)
	if relationResult.Failed > 0 {
		return result, writeFailureError("sample topology import failed", relationResult)
	}
	return result, nil
}

func loadPayloads[T ~map[string]any](path string) ([]T, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, apperrors.WithDetails(apperrors.CodeInvalidArgument, "sample data is not accessible", map[string]string{
			"path": path,
		})
	}
	var payloads []T
	if err := json.Unmarshal(body, &payloads); err != nil {
		return nil, apperrors.WithDetails(apperrors.CodeValidationFailed, "sample data is invalid json", map[string]string{
			"path":   path,
			"reason": err.Error(),
		})
	}
	return payloads, nil
}

func repoPath(rel string) (string, error) {
	if _, err := os.Stat(rel); err == nil {
		return rel, nil
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", apperrors.New(apperrors.CodeInternal, "cannot resolve sample data path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if _, err := os.Stat(abs); err != nil {
		return "", apperrors.WithDetails(apperrors.CodeInvalidArgument, "sample data is not accessible", map[string]string{
			"path": rel,
		})
	}
	return abs, nil
}

func writeFailureError(message string, result model.WriteResult) error {
	details := map[string]string{
		"accepted": fmt.Sprint(result.Accepted),
		"failed":   fmt.Sprint(result.Failed),
	}
	if len(result.Items) > 0 {
		details["item"] = result.Items[0].ID
		details["reason"] = result.Items[0].Message
	}
	return apperrors.WithDetails(apperrors.CodeValidationFailed, message, details)
}
