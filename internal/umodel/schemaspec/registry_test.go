package schemaspec

import (
	"testing"
)

func TestDefaultRegistryLoadsEveryKindFromManifest(t *testing.T) {
	reg := Default()
	expected := []string{
		"entity_set", "entity_source", "metric_set", "log_set", "event_set",
		"trace_set", "profile_set", "runbook_set", "explorer",
		"sls_logstore", "sls_metricstore", "sls_entitystore",
		"external_storage", "aliyun_prometheus",
		"entity_set_link", "entity_source_link", "data_link",
		"storage_link", "runbook_link", "explorer_link",
	}
	for _, kind := range expected {
		if reg.Lookup(kind) == nil {
			t.Errorf("expected schema for kind %q to be loaded", kind)
		}
	}
	got := reg.Kinds()
	if len(got) != len(expected) {
		t.Fatalf("expected %d kinds, got %d: %v", len(expected), len(got), got)
	}
}

func TestRegistryLookupReturnsNilForUnknownKind(t *testing.T) {
	if Default().Lookup("not_a_real_kind") != nil {
		t.Fatal("unknown kind should return nil")
	}
}

func TestEntitySetLinkSchemaCarriesRequiredEntityLinkType(t *testing.T) {
	s := Default().Lookup("entity_set_link")
	if s == nil {
		t.Fatal("entity_set_link schema missing")
	}
	prop, ok := s.Spec.Properties["entity_link_type"]
	if !ok {
		t.Fatal("entity_link_type property not present in spec")
	}
	if prop.Constraint == nil || !prop.Constraint.Required {
		t.Fatalf("entity_link_type must be required, got constraint=%+v", prop.Constraint)
	}
}

func TestEntitySetLinkSchemaCarriesSrcAndDestEndpoints(t *testing.T) {
	s := Default().Lookup("entity_set_link")
	for _, endpoint := range []string{"src", "dest"} {
		prop, ok := s.Spec.Properties[endpoint]
		if !ok {
			t.Fatalf("%s endpoint property is missing", endpoint)
		}
		for _, field := range []string{"domain", "kind", "name"} {
			sub, ok := prop.Properties[field]
			if !ok {
				t.Errorf("%s.%s is missing", endpoint, field)
				continue
			}
			if sub.Constraint == nil || !sub.Constraint.Required {
				t.Errorf("%s.%s should be required, got constraint=%+v", endpoint, field, sub.Constraint)
			}
		}
	}
}

func TestEntitySetSchemaCarriesFieldsArrayWithRequiredItemNameAndType(t *testing.T) {
	s := Default().Lookup("entity_set")
	if s == nil {
		t.Fatal("entity_set schema missing")
	}
	fields, ok := s.Spec.Properties["fields"]
	if !ok {
		t.Fatal("fields property missing")
	}
	if fields.Type != "array" {
		t.Fatalf("fields.type expected array, got %q", fields.Type)
	}
	if fields.Constraint == nil || fields.Constraint.Array == nil {
		t.Fatal("fields.constraint.array missing")
	}
	item := fields.Constraint.Array.Item
	if item.Type != "object" {
		t.Fatalf("fields[].type expected object, got %q", item.Type)
	}
	for _, want := range []string{"name", "type"} {
		sub, ok := item.Properties[want]
		if !ok {
			t.Errorf("fields[].%s missing", want)
			continue
		}
		if sub.Constraint == nil || !sub.Constraint.Required {
			t.Errorf("fields[].%s should be required, got constraint=%+v", want, sub.Constraint)
		}
	}
}
