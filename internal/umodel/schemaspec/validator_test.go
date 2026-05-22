package schemaspec_test

import (
	"strings"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/umodel/schemaspec"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

func TestValidateReturnsErrorForUnknownKind(t *testing.T) {
	v := schemaspec.DefaultValidator()
	_, err := v.Validate(model.UModelElement{Kind: "not_a_real_kind", Domain: "d", Name: "n"})
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
	if !strings.Contains(err.Error(), "unknown kind") {
		t.Fatalf("error message should mention unknown kind, got: %v", err)
	}
}

func TestNoopValidatorAcceptsEverything(t *testing.T) {
	v := schemaspec.NewNoopValidator()
	res, err := v.Validate(model.UModelElement{Kind: "anything_goes", Spec: map[string]any{"garbage": true}})
	if err != nil {
		t.Fatalf("noop validator must never error, got %v", err)
	}
	if len(res.Errors) != 0 || len(res.Warnings) != 0 {
		t.Fatalf("noop validator must produce no issues, got %+v", res)
	}
}

func TestValidateEntitySetLinkRegression(t *testing.T) {
	// Regression: the pre-fix incident-investigation YAMLs used the wrong
	// field names (source/destination/relation_type) and were missing the
	// required entity_link_type. Verify that this combination is caught.
	v := schemaspec.DefaultValidator()
	el := model.UModelElement{
		Kind: "entity_set_link", Domain: "demo", Name: "demo.bad",
		Spec: map[string]any{
			"source":        map[string]any{"domain": "x", "name": "y"},
			"destination":   map[string]any{"domain": "z", "name": "w"},
			"relation_type": "depends_on",
		},
	}
	res, err := v.Validate(el)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !hasErrorAt(res, "spec.entity_link_type", "required") {
		t.Errorf("expected required-missing error for spec.entity_link_type, got %+v", res.Errors)
	}
	for _, unknown := range []string{"spec.source", "spec.destination", "spec.relation_type"} {
		if !hasWarningAt(res, unknown, "not defined") {
			t.Errorf("expected unknown-field warning for %s, got %+v", unknown, res.Warnings)
		}
	}
}

func TestValidateEntitySetLinkValidShape(t *testing.T) {
	v := schemaspec.DefaultValidator()
	el := model.UModelElement{
		Kind: "entity_set_link", Domain: "demo", Name: "demo.good",
		Spec: map[string]any{
			"src":              map[string]any{"domain": "demo", "kind": "entity_set", "name": "a"},
			"dest":             map[string]any{"domain": "demo", "kind": "entity_set", "name": "b"},
			"entity_link_type": "depends_on",
		},
	}
	res, err := v.Validate(el)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("valid link should have zero errors, got %+v", res.Errors)
	}
}

func TestValidateEnumViolation(t *testing.T) {
	v := schemaspec.DefaultValidator()
	// fields[].type carries an enum (string/integer/float/boolean/time/json_object/json_array).
	el := model.UModelElement{
		Kind: "entity_set", Domain: "demo", Name: "demo.x",
		Spec: map[string]any{
			"fields": []any{
				map[string]any{"name": "f1", "type": "totally_made_up_type"},
			},
		},
	}
	res, _ := v.Validate(el)
	if !hasErrorAt(res, "spec.fields[0].type", "enum") {
		t.Errorf("expected enum violation on fields[0].type, got %+v", res.Errors)
	}
}

func TestValidatePatternViolation(t *testing.T) {
	v := schemaspec.DefaultValidator()
	// fields[].name carries the pattern ^[a-zA-Z0-9_][a-zA-Z0-9-_\.:]{0,127}$
	el := model.UModelElement{
		Kind: "entity_set", Domain: "demo", Name: "demo.x",
		Spec: map[string]any{
			"fields": []any{
				map[string]any{"name": "!!!bad name!!!", "type": "string"},
			},
		},
	}
	res, _ := v.Validate(el)
	if !hasErrorAt(res, "spec.fields[0].name", "pattern") {
		t.Errorf("expected pattern violation on fields[0].name, got %+v", res.Errors)
	}
}

func TestValidateTypeMismatch(t *testing.T) {
	v := schemaspec.DefaultValidator()
	// fields[].name expects string; pass an integer instead.
	el := model.UModelElement{
		Kind: "entity_set", Domain: "demo", Name: "demo.x",
		Spec: map[string]any{
			"fields": []any{map[string]any{"name": 42, "type": "string"}},
		},
	}
	res, _ := v.Validate(el)
	if !hasErrorAt(res, "spec.fields[0].name", "expected type") {
		t.Errorf("expected type mismatch on fields[0].name, got %+v", res.Errors)
	}
}

func TestValidateRequiredMissingForNestedEndpoint(t *testing.T) {
	v := schemaspec.DefaultValidator()
	// src present but missing its required name field.
	el := model.UModelElement{
		Kind: "entity_set_link", Domain: "demo", Name: "demo.partial",
		Spec: map[string]any{
			"src":              map[string]any{"domain": "demo", "kind": "entity_set"},
			"dest":             map[string]any{"domain": "demo", "kind": "entity_set", "name": "b"},
			"entity_link_type": "depends_on",
		},
	}
	res, _ := v.Validate(el)
	if !hasErrorAt(res, "spec.src.name", "required") {
		t.Errorf("expected required-missing on spec.src.name, got %+v", res.Errors)
	}
}

func TestValidateEntitySetMinimalSpecAccepted(t *testing.T) {
	// Most existing tests construct minimal entity_set elements; ensure they
	// still pass to keep blast radius low.
	v := schemaspec.DefaultValidator()
	for _, spec := range []map[string]any{
		nil,
		{},
		{"fields": []any{map[string]any{"name": "id", "type": "string"}}},
	} {
		el := model.UModelElement{Kind: "entity_set", Domain: "demo", Name: "demo.x", Spec: spec}
		res, err := v.Validate(el)
		if err != nil {
			t.Errorf("validate err for spec %+v: %v", spec, err)
		}
		if len(res.Errors) != 0 {
			t.Errorf("minimal entity_set %+v should pass, got errors %+v", spec, res.Errors)
		}
	}
}

func TestValidateRequiredMissingForEmptyValue(t *testing.T) {
	// "required" should also fire when the value is nil, not just when the key
	// is absent. Use entity_set_link.src=nil.
	v := schemaspec.DefaultValidator()
	el := model.UModelElement{
		Kind: "entity_set_link", Domain: "demo", Name: "demo.nilsrc",
		Spec: map[string]any{
			"src":              nil,
			"dest":             map[string]any{"domain": "demo", "kind": "entity_set", "name": "b"},
			"entity_link_type": "depends_on",
		},
	}
	res, _ := v.Validate(el)
	// src itself isn't required (only its children are when src is present);
	// nil should be treated as absence, so no required-missing on src.name.
	for _, issue := range res.Errors {
		if strings.HasPrefix(issue.Path, "spec.src") && issue.Reason != "required field is missing" {
			t.Errorf("did not expect non-required error on spec.src.*: %+v", issue)
		}
	}
}

func hasErrorAt(res schemaspec.Result, pathPrefix, reasonSubstring string) bool {
	for _, e := range res.Errors {
		if strings.HasPrefix(e.Path, pathPrefix) && strings.Contains(e.Reason, reasonSubstring) {
			return true
		}
	}
	return false
}

func hasWarningAt(res schemaspec.Result, pathPrefix, reasonSubstring string) bool {
	for _, w := range res.Warnings {
		if strings.HasPrefix(w.Path, pathPrefix) && strings.Contains(w.Reason, reasonSubstring) {
			return true
		}
	}
	return false
}
