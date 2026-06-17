package errors

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"testing"
)

func TestNewSeedsRetryableFromCode(t *testing.T) {
	cases := []struct {
		code Code
		want bool
	}{
		{CodeTimeout, true},
		{CodeProviderUnavailable, true},
		{CodeInvalidArgument, false},
		{CodeNotFound, false},
		{CodeConflict, false},
		{CodeInternal, false},
		{CodeNotImplemented, false},
	}
	for _, tc := range cases {
		if got := New(tc.code, "boom").Retryable; got != tc.want {
			t.Errorf("New(%s).Retryable = %v, want %v", tc.code, got, tc.want)
		}
		// WithDetails must seed retryability identically and keep details.
		err := WithDetails(tc.code, "boom", map[string]string{"k": "v"})
		if err.Retryable != tc.want {
			t.Errorf("WithDetails(%s).Retryable = %v, want %v", tc.code, err.Retryable, tc.want)
		}
		if err.Details["k"] != "v" {
			t.Errorf("WithDetails(%s) dropped details: %#v", tc.code, err.Details)
		}
	}
}

func TestIsRetryable(t *testing.T) {
	if !IsRetryable(New(CodeTimeout, "deadline")) {
		t.Error("timeout error should be retryable")
	}
	if IsRetryable(New(CodeNotFound, "missing")) {
		t.Error("not-found error should not be retryable")
	}
	// Unwraps like IsCode: a wrapped transient error stays retryable.
	wrapped := fmt.Errorf("query failed: %w", New(CodeProviderUnavailable, "down"))
	if !IsRetryable(wrapped) {
		t.Error("wrapped provider-unavailable error should be retryable")
	}
	// Non-*Error and nil are not retryable.
	if IsRetryable(stderrors.New("plain")) {
		t.Error("plain error should not be retryable")
	}
	if IsRetryable(nil) {
		t.Error("nil should not be retryable")
	}
}

// TestRetryableDeprecatedDelegatesToNew pins the deprecated constructor's new
// behavior: it is kept for source compatibility but now derives retryability
// from the code (like New), rather than force-marking every error retryable.
func TestRetryableDeprecatedDelegatesToNew(t *testing.T) {
	if got := Retryable(CodeTimeout, "x"); !got.Retryable {
		t.Error("Retryable(CodeTimeout) should be retryable")
	}
	if got := Retryable(CodeNotFound, "x"); got.Retryable {
		t.Error("Retryable(CodeNotFound) should not be retryable (now derived from code)")
	}
	if got := Retryable(CodeInvalidArgument, "msg"); got.Code != CodeInvalidArgument || got.Message != "msg" {
		t.Errorf("Retryable should preserve code and message, got %+v", got)
	}
}

// TestRetryableSerialized guards the JSON contract: the field clients read must
// carry the correct value, true for transient codes and false otherwise.
func TestRetryableSerialized(t *testing.T) {
	for code, want := range map[Code]bool{CodeTimeout: true, CodeNotFound: false} {
		blob, err := json.Marshal(New(code, "x"))
		if err != nil {
			t.Fatalf("marshal %s: %v", code, err)
		}
		var decoded struct {
			Retryable bool `json:"retryable"`
		}
		if err := json.Unmarshal(blob, &decoded); err != nil {
			t.Fatalf("unmarshal %s: %v", code, err)
		}
		if decoded.Retryable != want {
			t.Errorf("%s serialized retryable = %v, want %v (%s)", code, decoded.Retryable, want, blob)
		}
	}
}
