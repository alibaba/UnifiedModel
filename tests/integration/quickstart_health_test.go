package integration_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestQuickstartHealth(t *testing.T) {
	sampleDir := filepath.Join("..", "..", "examples", "quickstart-multidomain", "sample-data")

	var manifest struct {
		Sample       string            `json:"sample"`
		Domains      []string          `json:"domains"`
		SeedEntities map[string]string `json:"seed_entities"`
		Counts       struct {
			EntitySets     int `json:"entity_sets"`
			EntitySetLinks int `json:"entity_set_links"`
			Entities       int `json:"entities"`
			Relations      int `json:"relations"`
		} `json:"counts"`
	}
	readJSON(t, filepath.Join(sampleDir, "manifest.json"), &manifest)

	var entities []map[string]any
	readJSON(t, filepath.Join(sampleDir, "entities.json"), &entities)

	var relations []map[string]any
	readJSON(t, filepath.Join(sampleDir, "relations.json"), &relations)

	t.Run("ManifestMatchesData", func(t *testing.T) {
		if got := len(entities); got != manifest.Counts.Entities {
			t.Fatalf("manifest declares %d entities, file has %d", manifest.Counts.Entities, got)
		}
		if got := len(relations); got != manifest.Counts.Relations {
			t.Fatalf("manifest declares %d relations, file has %d", manifest.Counts.Relations, got)
		}
	})

	t.Run("AllDomainsPresent", func(t *testing.T) {
		entityDomains := make(map[string]bool)
		for _, e := range entities {
			if d, ok := e["__domain__"].(string); ok {
				entityDomains[d] = true
			}
		}
		for _, domain := range manifest.Domains {
			if !entityDomains[domain] {
				t.Fatalf("domain %q declared in manifest but not found in entity data", domain)
			}
		}
	})

	t.Run("EntityIDsAreUnique", func(t *testing.T) {
		seen := make(map[string]bool)
		for _, e := range entities {
			id, ok := e["__entity_id__"].(string)
			if !ok || id == "" {
				t.Fatalf("entity missing __entity_id__: %+v", e)
			}
			if seen[id] {
				t.Fatalf("duplicate entity ID: %s", id)
			}
			seen[id] = true
		}
	})

	t.Run("RelationEndpointsHaveEntityType", func(t *testing.T) {
		entityTypes := make(map[string]bool)
		for _, e := range entities {
			if et, ok := e["__entity_type__"].(string); ok {
				entityTypes[et] = true
			}
		}
		for _, r := range relations {
			srcType, _ := r["__src_entity_type__"].(string)
			destType, _ := r["__dest_entity_type__"].(string)
			if srcType == "" || destType == "" {
				t.Fatalf("relation missing entity type: %+v", r)
			}
			if !entityTypes[srcType] {
				t.Fatalf("relation src entity type %q has no entities in data", srcType)
			}
			if !entityTypes[destType] {
				t.Fatalf("relation dest entity type %q has no entities in data", destType)
			}
		}
	})

	t.Run("CrossDomainLinksExist", func(t *testing.T) {
		found := false
		for _, r := range relations {
			src, _ := r["__src_domain__"].(string)
			dest, _ := r["__dest_domain__"].(string)
			if src != "" && dest != "" && src != dest {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected at least one cross-domain relation")
		}
	})

	t.Run("SeedEntitiesExist", func(t *testing.T) {
		entityIDs := make(map[string]bool)
		for _, e := range entities {
			if id, ok := e["__entity_id__"].(string); ok {
				entityIDs[id] = true
			}
		}
		for name, id := range manifest.SeedEntities {
			if !entityIDs[id] {
				t.Fatalf("seed entity %q (id=%s) not found in entity data", name, id)
			}
		}
	})

	t.Run("EntitySetYAMLsExist", func(t *testing.T) {
		examplesDir := filepath.Join("..", "..", "examples", "quickstart-multidomain", "umodel")
		entityTypes := make(map[string]bool)
		for _, e := range entities {
			if et, ok := e["__entity_type__"].(string); ok {
				entityTypes[et] = true
			}
		}
		for et := range entityTypes {
			parts := splitEntityType(et)
			if len(parts) != 2 {
				t.Fatalf("unexpected entity type format: %s", et)
			}
			yamlPath := filepath.Join(examplesDir, parts[0], "entity_set", et+".yaml")
			if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
				t.Fatalf("entity type %s has no entity_set yaml at %s", et, yamlPath)
			}
		}
	})
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
}

func splitEntityType(et string) []string {
	for i := 0; i < len(et); i++ {
		if et[i] == '.' {
			return []string{et[:i], et[i+1:]}
		}
	}
	return []string{et}
}
