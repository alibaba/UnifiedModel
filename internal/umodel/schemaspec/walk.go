package schemaspec

import (
	"fmt"
	"reflect"
)

func walk(value any, prop Property, path string, res *Result) {
	if prop.ReleaseStage == "experimental" || prop.ReleaseStage == "deprecated" {
		res.Warnings = append(res.Warnings, Issue{
			Path:   path,
			Reason: fmt.Sprintf("field is %s and may change in future versions", prop.ReleaseStage),
		})
	}

	if value == nil {
		if prop.Constraint != nil && prop.Constraint.Required {
			res.Errors = append(res.Errors, Issue{Path: path, Reason: "required field is missing"})
		}
		return
	}

	if prop.Type != "" && !typeMatches(value, prop.Type) {
		res.Errors = append(res.Errors, Issue{
			Path:   path,
			Reason: fmt.Sprintf("expected type %s, got %T", prop.Type, value),
		})
		return
	}

	if prop.Constraint != nil {
		applyConstraints(value, prop.Constraint, prop.Type, path, res)
	}

	switch prop.Type {
	case "object":
		obj, ok := value.(map[string]any)
		if !ok {
			return
		}
		walkObject(obj, prop, path, res)
	case "array":
		arr, ok := value.([]any)
		if !ok {
			return
		}
		if prop.Constraint != nil && prop.Constraint.Array != nil {
			for i, item := range arr {
				walk(item, prop.Constraint.Array.Item, fmt.Sprintf("%s[%d]", path, i), res)
			}
		}
	case "map":
		m, ok := value.(map[string]any)
		if !ok {
			return
		}
		if prop.Constraint != nil && prop.Constraint.Map != nil {
			for k, v := range m {
				walk(v, prop.Constraint.Map.Value, path+"."+k, res)
			}
		}
	}
}

func walkObject(obj map[string]any, prop Property, path string, res *Result) {
	for k, v := range obj {
		sub, defined := prop.Properties[k]
		if !defined {
			res.Warnings = append(res.Warnings, Issue{
				Path:   path + "." + k,
				Reason: "field is not defined in schema",
			})
			continue
		}
		walk(v, sub, path+"."+k, res)
	}
	for k, sub := range prop.Properties {
		if _, present := obj[k]; present {
			continue
		}
		if sub.Constraint != nil && sub.Constraint.Required {
			res.Errors = append(res.Errors, Issue{
				Path:   path + "." + k,
				Reason: "required field is missing",
			})
		}
	}
}

func applyConstraints(value any, c *Constraint, propType string, path string, res *Result) {
	if c == nil {
		return
	}
	if s, ok := value.(string); ok {
		if c.Pattern != nil && !c.Pattern.MatchString(s) {
			res.Errors = append(res.Errors, Issue{
				Path:   path,
				Reason: fmt.Sprintf("value %q does not match pattern %s", s, c.PatternStr),
			})
		}
		if c.MinLen != nil && len(s) < *c.MinLen {
			res.Errors = append(res.Errors, Issue{
				Path:   path,
				Reason: fmt.Sprintf("length %d is below min_len %d", len(s), *c.MinLen),
			})
		}
		if c.MaxLen != nil && len(s) > *c.MaxLen {
			res.Errors = append(res.Errors, Issue{
				Path:   path,
				Reason: fmt.Sprintf("length %d exceeds max_len %d", len(s), *c.MaxLen),
			})
		}
	}
	if arr, ok := value.([]any); ok && c.Array != nil {
		if c.Array.MinSize != nil && len(arr) < *c.Array.MinSize {
			res.Errors = append(res.Errors, Issue{
				Path:   path,
				Reason: fmt.Sprintf("array size %d below min %d", len(arr), *c.Array.MinSize),
			})
		}
		if c.Array.MaxSize != nil && len(arr) > *c.Array.MaxSize {
			res.Errors = append(res.Errors, Issue{
				Path:   path,
				Reason: fmt.Sprintf("array size %d above max %d", len(arr), *c.Array.MaxSize),
			})
		}
	}
	if m, ok := value.(map[string]any); ok && c.Map != nil {
		if c.Map.MinSize != nil && len(m) < *c.Map.MinSize {
			res.Errors = append(res.Errors, Issue{
				Path:   path,
				Reason: fmt.Sprintf("map size %d below min %d", len(m), *c.Map.MinSize),
			})
		}
		if c.Map.MaxSize != nil && len(m) > *c.Map.MaxSize {
			res.Errors = append(res.Errors, Issue{
				Path:   path,
				Reason: fmt.Sprintf("map size %d above max %d", len(m), *c.Map.MaxSize),
			})
		}
	}
	if len(c.Enum) > 0 {
		if c.DefaultValue != nil && reflect.DeepEqual(value, c.DefaultValue) {
			return
		}
		for _, e := range c.Enum {
			if reflect.DeepEqual(value, e) {
				return
			}
		}
		res.Errors = append(res.Errors, Issue{
			Path:   path,
			Reason: fmt.Sprintf("value %v is not in enum %v", value, c.Enum),
		})
	}
}

func typeMatches(value any, t string) bool {
	switch t {
	case "", "any":
		return true
	case "string", "enum", "time":
		_, ok := value.(string)
		return ok
	case "integer":
		switch x := value.(type) {
		case int, int64, uint64:
			return true
		case float64:
			return x == float64(int64(x))
		}
		return false
	case "number", "float":
		switch value.(type) {
		case int, int64, uint64, float64:
			return true
		}
		return false
	case "boolean", "bool":
		_, ok := value.(bool)
		return ok
	case "object", "map":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "semantic_string":
		switch value.(type) {
		case string, map[string]any:
			return true
		}
		return false
	case "json", "json_object", "json_array":
		return true
	}
	return true
}
