package errors

import (
	stderrors "errors"
	"fmt"
)

type Code string

const (
	CodeInvalidArgument     Code = "INVALID_ARGUMENT"
	CodeAlreadyExists       Code = "ALREADY_EXISTS"
	CodeNotFound            Code = "NOT_FOUND"
	CodeConflict            Code = "CONFLICT"
	CodePartialFailed       Code = "PARTIAL_FAILED"
	CodeVersionConflict     Code = "VERSION_CONFLICT"
	CodeWorkspaceTombstoned Code = "WORKSPACE_TOMBSTONED"
	CodeWorkspaceConflicted Code = "WORKSPACE_CONFLICTED"
	CodeProviderUnsupported Code = "PROVIDER_UNSUPPORTED"
	CodeProviderUnavailable Code = "PROVIDER_UNAVAILABLE"
	CodeValidationFailed    Code = "VALIDATION_FAILED"
	CodeQueryParseError     Code = "QUERY_PARSE_ERROR"
	CodeQueryPlanError      Code = "QUERY_PLAN_ERROR"
	CodeToolDisabled        Code = "TOOL_DISABLED"
	CodeToolNotFound        Code = "TOOL_NOT_FOUND"
	CodeTimeout             Code = "TIMEOUT"
	CodeInternal            Code = "INTERNAL"
	CodeNotImplemented      Code = "NOT_IMPLEMENTED"
)

type Error struct {
	Code      Code              `json:"code"`
	Message   string            `json:"message"`
	Retryable bool              `json:"retryable"`
	Details   map[string]string `json:"details,omitempty"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// retryableCodes marks the error classes where retrying the same request may
// succeed because the failure is transient: a provider that is momentarily
// unavailable, or a query that exceeded its deadline. Every other class —
// bad input, not found, conflicts, unsupported, internal bugs — fails again on
// an identical retry, so it is not retryable. Retryability is intrinsic to the
// code, not the call site, so the Retryable field is always consistent.
var retryableCodes = map[Code]bool{
	CodeTimeout:             true,
	CodeProviderUnavailable: true,
}

func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message, Retryable: retryableCodes[code]}
}

func WithDetails(code Code, message string, details map[string]string) *Error {
	return &Error{Code: code, Message: message, Retryable: retryableCodes[code], Details: details}
}

// Retryable builds an *Error for the given code.
//
// Deprecated: retryability is now derived from the code (see New), so this is
// equivalent to New(code, message) — it no longer force-marks the error
// retryable regardless of code. Prefer New or WithDetails. Retained because
// pkg/errors is a public, stable contract and dropping the exported symbol would
// break external Go callers.
func Retryable(code Code, message string) *Error {
	return New(code, message)
}

func As(err error) (*Error, bool) {
	var target *Error
	if stderrors.As(err, &target) {
		return target, true
	}
	return nil, false
}

func IsCode(err error, code Code) bool {
	if target, ok := As(err); ok {
		return target.Code == code
	}
	return false
}

// IsRetryable reports whether err is an *Error whose code is retryable. It
// unwraps like IsCode, so a wrapped transient error is still recognized.
// Non-*Error errors, and nil, are treated as not retryable.
func IsRetryable(err error) bool {
	if target, ok := As(err); ok {
		return target.Retryable
	}
	return false
}
