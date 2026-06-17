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
	// Deadline expiry is a provider timeout.
	if got := asQueryTimeout(bg, context.DeadlineExceeded); !apperrors.IsCode(got, apperrors.CodeTimeout) {
		t.Fatalf("DeadlineExceeded should map to CodeTimeout, got %v", got)
	}
	// Cancellation is NOT a timeout: it must pass through unchanged so a
	// client disconnect / parent cancellation is not reported as a (retryable)
	// provider timeout.
	if got := asQueryTimeout(bg, context.Canceled); got != context.Canceled {
		t.Fatalf("context.Canceled should pass through unchanged, got %v", got)
	}
}

// TestExecuteDeadlineExceededReturnsTimeout proves the end-to-end wiring: an
// expired deadline flows through Execute -> executor -> memory store, the store
// aborts, and the ctx error is mapped to CodeTimeout.
func TestExecuteDeadlineExceededReturnsTimeout(t *testing.T) {
	svc := NewService(graphstore.NewMemoryStore())
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Hour))
	defer cancel()
	_, err := svc.Execute(ctx, "demo", model.QueryRequest{Query: ".entity with(domain='devops', name='devops.service')"})
	if !apperrors.IsCode(err, apperrors.CodeTimeout) {
		t.Fatalf("Execute with expired deadline: got %v, want CodeTimeout", err)
	}
}

// TestExecuteCancelledContextIsNotTimeout guards against misreporting: a cancelled
// request must surface an error, but NOT CodeTimeout — it is not a provider
// timeout and must not look retryable.
func TestExecuteCancelledContextIsNotTimeout(t *testing.T) {
	svc := NewService(graphstore.NewMemoryStore())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := svc.Execute(ctx, "demo", model.QueryRequest{Query: ".entity with(domain='devops', name='devops.service')"})
	if err == nil {
		t.Fatal("Execute with cancelled ctx should return an error")
	}
	if apperrors.IsCode(err, apperrors.CodeTimeout) {
		t.Fatalf("cancelled ctx must not map to CodeTimeout, got %v", err)
	}
}
