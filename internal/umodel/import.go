package umodel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
	"gopkg.in/yaml.v3"
)

// Import loads UModel elements from an operator-provided path. The path is
// confined to the configured import root (WithImportRoot; the current working
// directory by default) so an API caller cannot read arbitrary server files.
func (s *Service) Import(ctx context.Context, workspace string, req model.UModelImportRequest) (model.UModelImportResult, error) {
	if req.Path != "" {
		confined, err := s.confineImportPath(req.Path)
		if err != nil {
			return model.UModelImportResult{Workspace: workspace, Source: req.Path}, err
		}
		req.Path = confined
	}
	return s.importInternal(ctx, workspace, req)
}

// ImportTrusted loads UModel elements from a path the caller has already
// vouched for (e.g. a bundled sample pack resolved from the repository).
// It skips import-root confinement and must never be reached by user input.
func (s *Service) ImportTrusted(ctx context.Context, workspace string, req model.UModelImportRequest) (model.UModelImportResult, error) {
	return s.importInternal(ctx, workspace, req)
}

func (s *Service) importInternal(ctx context.Context, workspace string, req model.UModelImportRequest) (model.UModelImportResult, error) {
	if workspace == "" {
		return model.UModelImportResult{}, apperrors.New(apperrors.CodeInvalidArgument, "workspace is required")
	}
	if req.Path == "" {
		return model.UModelImportResult{}, apperrors.New(apperrors.CodeInvalidArgument, "import path is required")
	}

	result := model.UModelImportResult{Workspace: workspace, Source: req.Path}
	elements, err := s.loadCommonSchemaPacks(ctx, workspace, req.CommonSchemaPacks)
	if err != nil {
		return result, err
	}

	paths, skipped, err := collectImportFiles(req.Path)
	if err != nil {
		return result, err
	}
	result.Skipped = skipped

	for _, path := range paths {
		element, err := parseUModelElementFile(path)
		if err != nil {
			return result, apperrors.WithDetails(apperrors.CodeValidationFailed, "umodel import failed", map[string]string{
				"path":   path,
				"reason": err.Error(),
			})
		}
		elements = append(elements, element)
	}

	write, err := s.PutElements(ctx, model.UModelElementBatch{Workspace: workspace, Elements: elements})
	if err != nil {
		return result, err
	}
	result.Imported = write.Accepted
	result.Elements = elements
	for _, item := range write.Items {
		if item.OK {
			continue
		}
		result.Errors = append(result.Errors, model.ErrorDetail{Field: item.ID, Reason: item.Message})
	}
	return result, nil
}

// confineImportPath resolves p and verifies it stays within the import root,
// returning the cleaned absolute path. The root defaults to the current
// working directory; "/" effectively disables confinement. This is the
// barrier that stops an API-provided path from escaping to arbitrary files.
func (s *Service) confineImportPath(p string) (string, error) {
	root := s.importRoot
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", apperrors.WithDetails(apperrors.CodeInternal, "cannot resolve import root", map[string]string{"reason": err.Error()})
		}
		root = wd
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", apperrors.WithDetails(apperrors.CodeInternal, "invalid import root", map[string]string{"reason": err.Error()})
	}
	rootAbs = filepath.Clean(rootAbs)

	abs, err := filepath.Abs(p)
	if err != nil {
		return "", apperrors.WithDetails(apperrors.CodeInvalidArgument, "invalid import path", map[string]string{"path": p})
	}
	abs = filepath.Clean(abs)

	rel, err := filepath.Rel(rootAbs, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", apperrors.WithDetails(apperrors.CodeInvalidArgument,
			"import path is outside the allowed import root",
			map[string]string{"import_root": rootAbs})
	}
	return abs, nil
}

func (s *Service) loadCommonSchemaPacks(ctx context.Context, workspace string, packs []string) ([]model.UModelElement, error) {
	if len(packs) == 0 {
		return nil, nil
	}
	s.mu.RLock()
	loader := s.commonSchemaLoader
	s.mu.RUnlock()
	if loader == nil {
		return nil, apperrors.New(apperrors.CodeNotImplemented, "common schema pack loader hook is reserved but not wired")
	}
	return loader.LoadCommonSchemaPacks(ctx, workspace, packs)
}

func collectImportFiles(root string) ([]string, int, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, 0, apperrors.WithDetails(apperrors.CodeInvalidArgument, "import path is not accessible", map[string]string{
			"path": root,
		})
	}
	if !info.IsDir() {
		if !isImportFile(root) {
			return nil, 1, apperrors.WithDetails(apperrors.CodeInvalidArgument, "import file must be yaml, yml, or json", map[string]string{
				"path": root,
			})
		}
		return []string{root}, 0, nil
	}

	files := []string{}
	skipped := 0
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && shouldSkipDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if isImportFile(path) {
			files = append(files, path)
			return nil
		}
		skipped++
		return nil
	})
	if err != nil {
		return nil, skipped, err
	}
	sort.Strings(files)
	return files, skipped, nil
}

func shouldSkipDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case "node_modules", "vendor", "target", "dist", "build", "sample-data":
		return true
	default:
		return false
	}
}

func isImportFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml", ".json":
		return true
	default:
		return false
	}
}

func parseUModelElementFile(path string) (model.UModelElement, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return model.UModelElement{}, err
	}
	var payload map[string]any
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		if err := json.Unmarshal(body, &payload); err != nil {
			return model.UModelElement{}, err
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(body, &payload); err != nil {
			return model.UModelElement{}, err
		}
	default:
		return model.UModelElement{}, fmt.Errorf("unsupported file extension")
	}
	return elementFromPayload(normalizeMap(payload))
}

func elementFromPayload(payload map[string]any) (model.UModelElement, error) {
	metadata := nestedMap(payload, "metadata")
	schema := nestedMap(payload, "schema")

	kind := firstString(payload["kind"])
	name := firstString(metadata["name"], payload["name"])
	domain := firstString(metadata["domain"], payload["domain"])
	version := firstString(schema["version"], payload["version"])
	if domain == "" {
		domain = inferDomain(name)
	}
	if kind == "" {
		return model.UModelElement{}, fmt.Errorf("kind is required")
	}
	if domain == "" {
		return model.UModelElement{}, fmt.Errorf("domain or metadata.domain is required")
	}
	if name == "" {
		return model.UModelElement{}, fmt.Errorf("name or metadata.name is required")
	}

	spec := nestedMap(payload, "spec")
	return model.UModelElement{
		Kind:    kind,
		Domain:  domain,
		Name:    name,
		Version: version,
		Spec:    spec,
	}, nil
}

func normalizeMap(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	out := make(map[string]any, len(source))
	for key, value := range source {
		out[key] = normalizeValue(value)
	}
	return out
}

func normalizeValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return normalizeMap(typed)
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[fmt.Sprint(key)] = normalizeValue(value)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = normalizeValue(item)
		}
		return out
	default:
		return typed
	}
}

func nestedMap(source map[string]any, key string) map[string]any {
	value, ok := source[key].(map[string]any)
	if !ok {
		return nil
	}
	return value
}

func firstString(values ...any) string {
	for _, value := range values {
		text, ok := value.(string)
		if ok && text != "" {
			return text
		}
	}
	return ""
}

func inferDomain(name string) string {
	if before, _, ok := strings.Cut(name, "."); ok {
		return before
	}
	return ""
}
