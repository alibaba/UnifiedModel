package query

import (
	"context"
	"testing"
	"time"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

func TestParseQueryTimeout(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"", 0},
		{"bogus", 0},
		{"0s", 0},
		{"-5s", 0},
		{"10s", 10 * time.Second},
		{"1m", time.Minute},
	}
	for _, c := range cases {
		if got := parseQueryTimeout(c.in); got != c.want {
			t.Errorf("parseQueryTimeout(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestAsQueryTimeout(t *testing.T) {
	bg := context.Background()
	if asQueryTimeout(bg, nil) != nil {
		t.Fatal("nil error should pass through as nil")
	}
	other := apperrors.New(apperrors.CodeInvalidArgument, "bad")
	if got := asQueryTimeout(bg, other); got != other {
		t.Fatalf("non-ctx error should pass through unchanged, got %v", got)
	}
	for _, e := range []error{context.DeadlineExceeded, context.Canceled} {
		if got := asQueryTimeout(bg, e); !apperrors.IsCode(got, apperrors.CodeTimeout) {
			t.Fatalf("asQueryTimeout(%v) should be CodeTimeout, got %v", e, got)
		}
	}
}

// TestExecuteCancelledContextReturnsTimeout proves the end-to-end wiring: a
// cancelled context flows through Execute -> executor -> memory store, the store
// aborts, and the ctx error is mapped to CodeTimeout.
func TestExecuteCancelledContextReturnsTimeout(t *testing.T) {
	svc := NewService(graphstore.NewMemoryStore())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := svc.Execute(ctx, "demo", model.QueryRequest{Query: ".entity with(domain='devops', name='devops.service')"})
	if !apperrors.IsCode(err, apperrors.CodeTimeout) {
		t.Fatalf("Execute with cancelled ctx: got %v, want CodeTimeout", err)
	}
}
