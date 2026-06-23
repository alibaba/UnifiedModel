package query

import (
	"sort"
	"strings"

	"github.com/alibaba/UnifiedModel/internal/query/planrender"
)

// Constraint is a normalized predicate over a storage field, independent of any
// query syntax. It is the shared intermediate representation a family renderer
// formats into its dialect: a PromQL label matcher, an Elasticsearch term, or a
// SQL WHERE clause. Field is already resolved through the dataLink /
// storageLink fields_mapping, so renderers do not re-derive storage field names.
type Constraint struct {
	Field  string
	Op     string // "eq", "neq", "in", "notin"
	Values []string
}

// ResolveConstraints turns an entity-scoped request into normalized constraints,
// plus any filter expressions that could not be parsed (returned verbatim so a
// renderer can pass them through unrendered, as the Prometheus path does with
// raw_filters). It resolves the same inputs the Prometheus matcher path does —
// entity ids, entity data, and the search/data/entity/method filter sources —
// reusing the existing filter parser and field mappers, but yields the
// syntax-neutral IR instead of PromQL matchers.
func ResolveConstraints(req planrender.Request) ([]Constraint, []string) {
	constraints := []Constraint{}
	raw := []string{}

	if idField := mappedStorageField(req.DataLinkMapping, req.StorageLinkMapping, "id"); idField != "" && len(req.EntityIDs) > 0 {
		constraints = append(constraints, Constraint{Field: idField, Op: "in", Values: req.EntityIDs})
	}

	valuesByField := entityDataStorageValues(req.EntityData, req.DataLinkMapping, req.StorageLinkMapping)
	if len(valuesByField) > 0 {
		fields := make([]string, 0, len(valuesByField))
		for field := range valuesByField {
			fields = append(fields, field)
		}
		sort.Strings(fields)
		for _, field := range fields {
			constraints = append(constraints, Constraint{Field: field, Op: "in", Values: valuesByField[field]})
		}
	}

	for _, item := range []struct {
		raw    string
		mapper func(string) string
	}{
		{firstNonEmpty(stringValue(req.Storage.Spec["search_filter"]), stringValue(req.Storage.Spec["default_filter"]), stringValue(req.Storage.Spec["query_filter"])), storageFieldMapper()},
		{req.DataFilter, dataSetToStorageFieldMapper(req.StorageLinkMapping)},
		{req.EntityQuery, entityToStorageFieldMapper(req.DataLinkMapping, req.StorageLinkMapping)},
		{req.MethodQuery, dataSetToStorageFieldMapper(req.StorageLinkMapping)},
	} {
		cs, unsupported := filterToConstraints(item.raw, item.mapper)
		constraints = append(constraints, cs...)
		raw = append(raw, unsupported...)
	}

	return dedupeConstraints(constraints), raw
}

// filterToConstraints parses one filter expression into constraints. An empty or
// "*" filter yields nothing; an expression that fails to parse or uses an
// unsupported shape is returned in the raw slice so the caller can pass it
// through unrendered.
func filterToConstraints(raw string, fieldMapper func(string) string) ([]Constraint, []string) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "*" {
		return nil, nil
	}
	expr, err := parseLogFilterExpression(raw)
	if err != nil {
		return nil, []string{raw}
	}
	cs, ok := nodeToConstraints(expr, fieldMapper)
	if !ok {
		return nil, []string{raw}
	}
	return cs, nil
}

func nodeToConstraints(node *logFilterNode, fieldMapper func(string) string) ([]Constraint, bool) {
	if node == nil {
		return nil, true
	}
	switch node.Kind {
	case "and":
		out := []Constraint{}
		for _, child := range node.Children {
			cs, ok := nodeToConstraints(child, fieldMapper)
			if !ok {
				return nil, false
			}
			out = append(out, cs...)
		}
		return out, true
	case "comparison":
		field := node.Field
		if fieldMapper != nil {
			field = fieldMapper(field)
		}
		switch node.Operator {
		case "=", ":", "==":
			return []Constraint{{Field: field, Op: "eq", Values: []string{stringValue(node.Value)}}}, true
		case "!=":
			return []Constraint{{Field: field, Op: "neq", Values: []string{stringValue(node.Value)}}}, true
		case "in":
			return []Constraint{{Field: field, Op: "in", Values: stringSliceValue(node.Value)}}, true
		case "not in":
			return []Constraint{{Field: field, Op: "notin", Values: stringSliceValue(node.Value)}}, true
		default:
			return nil, false
		}
	default:
		return nil, false
	}
}

func dedupeConstraints(constraints []Constraint) []Constraint {
	out := []Constraint{}
	seen := map[string]struct{}{}
	for _, c := range constraints {
		if c.Field == "" || len(c.Values) == 0 {
			continue
		}
		key := c.Field + "\x00" + c.Op + "\x00" + strings.Join(c.Values, "\x01")
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, c)
	}
	return out
}
