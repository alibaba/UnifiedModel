package schemaspec

import (
	"fmt"

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
// TODO(v2): respect element.Version; today we always use versions[0].
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
	var res Result
	walk(element.Spec, s.Spec, "spec", &res)
	return res, nil
}
