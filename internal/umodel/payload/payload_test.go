package payload

import "testing"

func TestParseElementBytesNormalizesSchemaStyleYAML(t *testing.T) {
	element, err := ParseElementBytes([]byte(`
kind: entity_set
schema:
  version: v0.1.0
metadata:
  name: devops.service
  domain: devops
spec:
  fields:
    - name: id
      type: string
`), ".yaml")
	if err != nil {
		t.Fatalf("parse yaml: %v", err)
	}
	if element.Kind != "entity_set" || element.Domain != "devops" || element.Name != "devops.service" || element.Version != "v0.1.0" {
		t.Fatalf("unexpected element envelope: %+v", element)
	}
	fields, ok := element.Spec["fields"].([]any)
	if !ok || len(fields) != 1 {
		t.Fatalf("expected spec.fields to survive normalization, got %+v", element.Spec)
	}
}

func TestParseElementBytesInfersDomainFromName(t *testing.T) {
	element, err := ParseElementBytes([]byte(`
kind: entity_set
metadata:
  name: devops.service
`), ".yaml")
	if err != nil {
		t.Fatalf("parse yaml: %v", err)
	}
	if element.Domain != "devops" {
		t.Fatalf("domain = %q, want devops", element.Domain)
	}
}
