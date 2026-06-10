package query

import (
	"context"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

func TestExecuteAcceptsEmptyFormat(t *testing.T) {
	svc := NewService(graphstore.NewMemoryStore())
	_, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
		Query:  ".umodel | limit 1",
		Format: "",
	})
	if err != nil {
		t.Fatalf("empty format should default to assistant envelope, got error: %v", err)
	}
}

func TestExecuteAcceptsAgentFormat(t *testing.T) {
	svc := NewService(graphstore.NewMemoryStore())
	_, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
		Query:  ".umodel | limit 1",
		Format: model.FormatAgent,
	})
	if err != nil {
		t.Fatalf("format=agent should be accepted, got error: %v", err)
	}
}

func TestExecuteRejectsUnknownFormat(t *testing.T) {
	svc := NewService(graphstore.NewMemoryStore())
	_, err := svc.Execute(context.Background(), "demo", model.QueryRequest{
		Query:  ".umodel | limit 1",
		Format: "bogus",
	})
	if !apperrors.IsCode(err, apperrors.CodeInvalidArgument) {
		t.Fatalf("unknown format should return INVALID_ARGUMENT, got: %v", err)
	}
	target, ok := apperrors.As(err)
	if !ok {
		t.Fatalf("expected *apperrors.Error, got: %v", err)
	}
	if target.Details["requested_format"] != "bogus" {
		t.Fatalf("error details should echo requested_format, got: %+v", target.Details)
	}
}

func TestExplainAlsoValidatesFormat(t *testing.T) {
	svc := NewService(graphstore.NewMemoryStore())
	_, err := svc.Explain(context.Background(), "demo", model.QueryRequest{
		Query:  ".umodel | limit 1",
		Format: "bogus",
	})
	if !apperrors.IsCode(err, apperrors.CodeInvalidArgument) {
		t.Fatalf("Explain should reject unknown format symmetrically, got: %v", err)
	}
}

func TestFormatThreadsToQueryPlan(t *testing.T) {
	// The parser must copy Format and IncludeSpec from the request into the
	// resulting plan so the executor can branch on them.
	planner := Planner{}
	plan, err := planner.Plan(model.QueryRequest{
		Query:       ".umodel | limit 1",
		Format:      model.FormatAgent,
		IncludeSpec: true,
	}, model.GraphStoreCapabilities{})
	if err != nil {
		t.Fatalf("planner.Plan: %v", err)
	}
	if plan.Format != model.FormatAgent {
		t.Fatalf("plan.Format = %q, want %q", plan.Format, model.FormatAgent)
	}
	if !plan.IncludeSpec {
		t.Fatalf("plan.IncludeSpec = false, want true")
	}
}
