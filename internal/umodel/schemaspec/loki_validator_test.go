package schemaspec_test

import (
	"testing"

	"github.com/alibaba/UnifiedModel/internal/umodel/schemaspec"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

func TestLokiSchemaValidatesAuthenticationAndSize(t *testing.T) {
	validator := schemaspec.DefaultValidator()
	base := map[string]any{"endpoint": "http://localhost:3100", "default_size": 100, "max_size": 1000}
	for name, auth := range map[string]map[string]any{
		"bearer": {"type": "bearer", "token_env": "LOKI_TOKEN"},
		"basic":  {"type": "basic", "username_env": "LOKI_USER", "password_env": "LOKI_PASSWORD"},
	} {
		t.Run(name, func(t *testing.T) {
			spec := map[string]any{"endpoint": base["endpoint"], "auth": auth}
			res, err := validator.Validate(model.UModelElement{Kind: "loki", Spec: spec})
			if err != nil || len(res.Errors) > 0 {
				t.Fatalf("valid Loki config rejected: %+v, %v", res.Errors, err)
			}
		})
	}
	bad := []map[string]any{
		{"endpoint": base["endpoint"], "auth": map[string]any{"type": "bearer", "token_env": "LOKI_TOKEN", "password_env": "PASSWORD"}},
		{"endpoint": base["endpoint"], "auth": map[string]any{"type": "basic", "username_env": "LOKI_USER"}},
		{"endpoint": base["endpoint"], "default_size": 101, "max_size": 100},
	}
	for _, spec := range bad {
		res, err := validator.Validate(model.UModelElement{Kind: "loki", Spec: spec})
		if err != nil || len(res.Errors) == 0 {
			t.Fatalf("invalid Loki config accepted: %#v, %+v, %v", spec, res.Errors, err)
		}
	}
}
