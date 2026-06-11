package query

import (
	"context"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

func TestExecuteAcceptsEmptyModeAsPlan(t *testing.T) {
	svc := NewService(graphstore.NewMemoryStore())
	_, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
		Query: ".umodel | limit 1",
		Mode:  "",
	})
	if err != nil {
		t.Fatalf("empty mode should default to plan, got error: %v", err)
	}
}

func TestExecuteAcceptsExplicitPlanMode(t *testing.T) {
	svc := NewService(graphstore.NewMemoryStore())
	_, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
		Query: ".umodel | limit 1",
		Mode:  "plan",
	})
	if err != nil {
		t.Fatalf("mode=plan should be accepted, got error: %v", err)
	}
}

func TestExecuteRejectsDataMode(t *testing.T) {
	svc := NewService(graphstore.NewMemoryStore())
	_, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
		Query: ".umodel | limit 1",
		Mode:  "data",
	})
	if !apperrors.IsCode(err, apperrors.CodeNotImplemented) {
		t.Fatalf("mode=data should return CodeNotImplemented, got: %v", err)
	}
}

func TestExecuteRejectsUnknownMode(t *testing.T) {
	svc := NewService(graphstore.NewMemoryStore())
	_, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
		Query: ".umodel | limit 1",
		Mode:  "bogus",
	})
	if !apperrors.IsCode(err, apperrors.CodeNotImplemented) {
		t.Fatalf("unknown mode should return CodeNotImplemented, got: %v", err)
	}
}

func TestExplainAlsoValidatesMode(t *testing.T) {
	svc := NewService(graphstore.NewMemoryStore())
	_, err := svc.Explain(context.Background(), "demo", model.QueryRequest{
		Query: ".umodel | limit 1",
		Mode:  "data",
	})
	if !apperrors.IsCode(err, apperrors.CodeNotImplemented) {
		t.Fatalf("Explain should reject mode=data symmetrically, got: %v", err)
	}
}

// TestModeRejectionCarriesMigrationDetails verifies that the mode=data error
// includes structured migration_* keys an AI agent can act on without having
// to parse the error message.
func TestModeRejectionCarriesMigrationDetails(t *testing.T) {
	svc := NewService(graphstore.NewMemoryStore())
	_, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
		Query: ".umodel | limit 1",
		Mode:  "data",
	})
	target, ok := apperrors.As(err)
	if !ok {
		t.Fatalf("expected *apperrors.Error, got: %v", err)
	}
	for _, key := range []string{"requested_mode", "supported_modes", "migration_service", "migration_action", "migration_docs_url"} {
		if _, ok := target.Details[key]; !ok {
			t.Fatalf("error details missing %q: %+v", key, target.Details)
		}
	}
	if target.Details["migration_service"] != "umodel-assistant" {
		t.Fatalf("migration_service = %q, want umodel-assistant", target.Details["migration_service"])
	}
}
