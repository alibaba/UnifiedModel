package payload

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alibaba/UnifiedModel/pkg/model"
	"gopkg.in/yaml.v3"
)

// ParseElementFile reads one UModel source file and normalizes its
// schema/metadata envelope into the public UModelElement shape.
func ParseElementFile(path string) (model.UModelElement, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return model.UModelElement{}, err
	}
	return ParseElementBytes(body, filepath.Ext(path))
}

// ParseElementBytes normalizes one JSON or YAML UModel source document.
func ParseElementBytes(body []byte, ext string) (model.UModelElement, error) {
	var payload map[string]any
	switch strings.ToLower(ext) {
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
	return elementFromMap(normalizeMap(payload))
}

func elementFromMap(payload map[string]any) (model.UModelElement, error) {
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
