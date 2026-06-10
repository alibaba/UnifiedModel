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
