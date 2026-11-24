package schemaspec

import (
	"fmt"
	"regexp"

	"github.com/alibaba/UnifiedModel/pkg/model"
)

type Validator struct {
	registry *Registry
}

func NewValidator(reg *Registry) *Validator {
	return &Validator{registry: reg}
}

func DefaultValidator() *Validator {
	return NewValidator(Default())
}

// NewNoopValidator returns a validator that accepts every element. Intended for
// test code that needs to exercise downstream behavior without supplying a
// schema-complete spec.
func NewNoopValidator() *Validator {
	return &Validator{}
}

// Validate returns the result of validating element.Spec against the schema for
// element.Kind. The returned error is non-nil only when the kind itself is
// unknown to the registry; field-level problems are reported in Result.
//
// TODO(v2): also validate the kind/schema/metadata envelope (currently
// elementFromPayload validates the minimum imperatively before the element
// reaches this layer).
func (v *Validator) Validate(element model.UModelElement) (Result, error) {
	if v == nil || v.registry == nil {
		return Result{}, nil
	}
	s := v.registry.Lookup(element.Kind)
	if s == nil {
		return Result{}, fmt.Errorf("unknown kind %q", element.Kind)
	}
	if element.Version != "" && !v.registry.AcceptsVersion(element.Version) {
		return Result{Errors: []Issue{{
			Path:   "version",
			Reason: fmt.Sprintf("unsupported model envelope version %q", element.Version),
		}}}, nil
	}
	var res Result
	walk(element.Spec, s.Spec, "spec", &res)
	if element.Kind == "loki" {
		validateLokiSpec(element.Spec, &res)
	}
	return res, nil
}

func validateLokiSpec(spec map[string]any, res *Result) {
	if raw, present := spec["auth"]; present {
		auth, ok := raw.(map[string]any)
		if !ok {
			return // the schema walker reports the type error
		}
		typ, _ := auth["type"].(string)
		required := []string{}
		switch typ {
		case "bearer":
			required = []string{"token_env"}
		case "basic":
			required = []string{"username_env", "password_env"}
		default:
			res.Errors = append(res.Errors, Issue{Path: "spec.auth.type", Reason: "must be bearer or basic"})
		}
		for _, key := range required {
			name, _ := auth[key].(string)
			valid, err := regexp.MatchString(`^[A-Za-z_][A-Za-z0-9_]*$`, name)
			if err != nil || !valid {
				res.Errors = append(res.Errors, Issue{Path: "spec.auth." + key, Reason: "must name a non-empty environment variable"})
			}
		}
		for _, key := range []string{"token_env", "username_env", "password_env"} {
			if value, exists := auth[key]; exists && value != nil && !containsSchemaString(required, key) {
				res.Errors = append(res.Errors, Issue{Path: "spec.auth." + key, Reason: "not allowed for this authentication type"})
			}
		}
	}
	defaultSize := 100
	if value, present := spec["default_size"]; present {
		if n, ok := intFromAny(value); ok {
			defaultSize = n
		}
	}
	maxSize := 1000
	if value, present := spec["max_size"]; present {
		if n, ok := intFromAny(value); ok {
			maxSize = n
		}
	}
	if defaultSize < 1 || maxSize < 1 || defaultSize > maxSize {
		res.Errors = append(res.Errors, Issue{Path: "spec.default_size", Reason: "default_size and max_size must be positive, with default_size <= max_size"})
	}
}

func containsSchemaString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
