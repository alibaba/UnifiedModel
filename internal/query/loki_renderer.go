package query

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alibaba/UnifiedModel/internal/query/planrender"
	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
)

const maxLokiPlanQueries = 100

// lokiRenderer produces LogQL plans for both Loki logs and log-derived metrics.
// It only accepts filters that can be expressed as indexed stream labels.
type lokiRenderer struct{}

func (lokiRenderer) Family() string { return "logql" }

func (lokiRenderer) SupportsMethod(m planrender.Method) bool {
	return m == planrender.MethodGetLogs || m == planrender.MethodGetMetrics
}

func (lokiRenderer) Render(req planrender.Request) (map[string]any, error) {
	config, err := lokiConfig(req)
	if err != nil {
		return nil, err
	}
	mode := "range"
	if req.Method == planrender.MethodGetMetrics {
		if stringValue(req.DataSet.Spec["query_type"]) != "logql" {
			return nil, lokiError("Loki metric_set.query_type must be logql")
		}
		mode = firstNonEmpty(req.QueryType, defaultMetricQueryMode(req.Metrics), stringValue(req.Storage.Spec["default_query_type"]), "instant")
		if mode != "instant" && mode != "range" {
			return nil, lokiError("query_type must be instant or range")
		}
	}
	if req.TimeRange.To == nil || (mode == "range" && req.TimeRange.From == nil) {
		return nil, lokiError("time_range.to is required; range queries also require time_range.from")
	}
	if req.TimeRange.From != nil && !req.TimeRange.From.Before(*req.TimeRange.To) {
		return nil, lokiError("time_range.from must precede time_range.to")
	}

	base, err := lokiBaseConstraints(req)
	if err != nil {
		return nil, err
	}
	rows, err := lokiEntityRows(req)
	if err != nil {
		return nil, err
	}
	queries := make([]map[string]any, 0)
	seen := map[string]struct{}{}
	for _, row := range rows {
		constraints := append(append([]Constraint{}, base...), row...)
		if req.Method == planrender.MethodGetLogs {
			logql, err := renderLokiLogQL("{}", constraints)
			if err != nil {
				return nil, err
			}
			if _, ok := seen[logql]; !ok {
				seen[logql] = struct{}{}
				queries = append(queries, map[string]any{"logql": logql})
			}
		} else {
			for _, metric := range req.Metrics {
				generator := strings.TrimSpace(stringValue(metric["generator"]))
				if generator == "" {
					return nil, lokiError("Loki metrics require a LogQL generator")
				}
				logql, err := renderLokiLogQL(generator, constraints)
				if err != nil {
					return nil, err
				}
				key := stringValue(metric["name"]) + "\x00" + logql
				if _, ok := seen[key]; !ok {
					seen[key] = struct{}{}
					item := metricQueryItem(metric)
					item["logql"] = logql
					queries = append(queries, item)
				}
			}
		}
		if len(queries) > maxLokiPlanQueries {
			return nil, lokiError("Loki plan exceeds maximum query count")
		}
	}
	if len(queries) == 0 {
		return nil, lokiError("Loki plan has no matching entity rows")
	}
	config["dialect"] = "loki_logql"
	config["query_type"] = mode
	config["queries"] = queries
	if req.Method == planrender.MethodGetLogs {
		size := intValue(req.Storage.Spec["default_size"])
		if size == 0 {
			size = 100
		}
		maxSize := intValue(req.Storage.Spec["max_size"])
		if maxSize == 0 {
			maxSize = 1000
		}
		if size < 1 || maxSize < 1 || size > maxSize {
			return nil, lokiError("default_size and max_size must be positive, with default_size <= max_size")
		}
		if req.LimitExplicit {
			size = req.Limit
		}
		if size < 1 || size > maxSize {
			return nil, lokiError("log limit must be positive and not exceed max_size")
		}
		config["size"] = size
		switch firstNonEmpty(stringValue(req.DataSet.Spec["default_order"]), "desc") {
		case "desc":
			config["direction"] = "backward"
		case "asc":
			config["direction"] = "forward"
		default:
			return nil, lokiError("log_set.default_order must be asc or desc")
		}
	} else if mode == "range" {
		step := firstNonEmpty(req.Step, stringValue(req.Storage.Spec["default_step"]))
		if step != "" {
			d, err := time.ParseDuration(step)
			if err != nil || d <= 0 {
				return nil, lokiError("metric step must be a positive duration")
			}
			config["step"] = step
		}
	}
	return config, nil
}

func lokiError(message string) error {
	return apperrors.New(apperrors.CodeQueryPlanError, message)
}

func validLokiLabel(name string) bool {
	ok, err := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, name)
	return err == nil && ok
}

func lokiConfig(req planrender.Request) (map[string]any, error) {
	spec := req.Storage.Spec
	endpoint := stringValue(spec["endpoint"])
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return nil, lokiError("Loki endpoint must be an HTTP or HTTPS URL without embedded credentials")
	}
	prefix := firstNonEmpty(stringValue(spec["api_prefix"]), "/loki/api/v1")
	if !strings.HasPrefix(prefix, "/") || strings.Contains(prefix, "?") || strings.Contains(prefix, "#") {
		return nil, lokiError("Loki api_prefix must be an absolute path")
	}
	out := map[string]any{"endpoint": endpoint, "api_prefix": prefix, "tls_verify": true}
	if verify, ok := spec["tls_verify"].(bool); ok {
		out["tls_verify"] = verify
	}
	tenant := stringValue(spec["tenant"])
	tenantHeader := firstNonEmpty(stringValue(spec["tenant_header"]), "X-Scope-OrgID")
	if tenant != "" {
		if !validHTTPHeaderName(tenantHeader) {
			return nil, lokiError("invalid Loki tenant_header")
		}
		out["tenant"] = tenant
		out["tenant_header"] = tenantHeader
	}
	if raw := spec["headers"]; raw != nil {
		headers := mapValue(raw)
		for key, value := range headers {
			if !validHTTPHeaderName(key) || strings.EqualFold(key, "Authorization") || strings.EqualFold(key, tenantHeader) || strings.ContainsAny(stringValue(value), "\r\n") {
				return nil, lokiError("invalid or reserved Loki header")
			}
		}
		out["headers"] = headers
	}
	if raw := spec["auth"]; raw != nil {
		auth := mapValue(raw)
		typ := stringValue(auth["type"])
		var required []string
		switch typ {
		case "bearer":
			required = []string{"token_env"}
		case "basic":
			required = []string{"username_env", "password_env"}
		default:
			return nil, lokiError("Loki auth.type must be bearer or basic")
		}
		for _, key := range required {
			if !validEnvName(stringValue(auth[key])) {
				return nil, lokiError("Loki auth requires valid environment variable names")
			}
		}
		for _, key := range []string{"token_env", "username_env", "password_env"} {
			if stringValue(auth[key]) != "" && !containsString(required, key) {
				return nil, lokiError("Loki auth contains fields from another authentication type")
			}
		}
		out["auth"] = auth
	}
	return out, nil
}

func validHTTPHeaderName(name string) bool {
	ok, err := regexp.MatchString(`^[A-Za-z0-9-]+$`, name)
	return err == nil && ok
}

func validEnvName(name string) bool {
	ok, err := regexp.MatchString(`^[A-Za-z_][A-Za-z0-9_]*$`, name)
	return err == nil && ok
}

func lokiBaseConstraints(req planrender.Request) ([]Constraint, error) {
	out := []Constraint{}
	hasRowID := false
	if req.EntityData != nil {
		hasRowID = containsString(req.EntityData.Header, "id")
	}
	if len(req.EntityIDs) > 0 && !hasRowID {
		field, err := strictLokiEntityField("id", req.DataLinkMapping, req.StorageLinkMapping)
		if err != nil {
			return nil, err
		}
		out = append(out, Constraint{Field: field, Op: "in", Values: req.EntityIDs})
	}
	filters := []struct {
		value  string
		mapper func(string) (string, error)
	}{
		{firstNonEmpty(stringValue(req.Storage.Spec["search_filter"]), stringValue(req.Storage.Spec["default_filter"]), stringValue(req.Storage.Spec["query_filter"])), func(field string) (string, error) { return field, nil }},
		{req.DataFilter, func(field string) (string, error) { return strictLokiDataSetField(field, req.StorageLinkMapping) }},
		{req.EntityQuery, func(field string) (string, error) {
			return strictLokiEntityField(field, req.DataLinkMapping, req.StorageLinkMapping)
		}},
		{req.MethodQuery, func(field string) (string, error) { return strictLokiDataSetField(field, req.StorageLinkMapping) }},
	}
	for _, filter := range filters {
		cs, err := lokiFilterConstraints(filter.value, filter.mapper)
		if err != nil {
			return nil, err
		}
		out = append(out, cs...)
	}
	return out, nil
}

func strictLokiEntityField(field string, dataLink, storageLink map[string]any) (string, error) {
	dataSetField := stringValue(dataLink[field])
	if dataSetField == "" {
		return "", lokiError("entity field has no DataLink mapping: " + field)
	}
	return strictLokiDataSetField(dataSetField, storageLink)
}

func strictLokiDataSetField(field string, storageLink map[string]any) (string, error) {
	label := stringValue(storageLink[field])
	if !validLokiLabel(label) {
		return "", lokiError("dataset field has no valid Loki label mapping: " + field)
	}
	return label, nil
}

func lokiFilterConstraints(raw string, mapper func(string) (string, error)) ([]Constraint, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "*" {
		return nil, nil
	}
	node, err := parseLogFilterExpression(raw)
	if err != nil {
		return nil, lokiError("unsupported Loki filter expression")
	}
	cs, ok := nodeToConstraints(node, nil)
	if !ok {
		return nil, lokiError("Loki filter cannot be expressed as stream labels")
	}
	for i := range cs {
		cs[i].Field, err = mapper(cs[i].Field)
		if err != nil || !validLokiLabel(cs[i].Field) {
			return nil, lokiError("Loki filter field has no valid label mapping")
		}
	}
	return cs, nil
}

func lokiEntityRows(req planrender.Request) ([][]Constraint, error) {
	if req.EntityData == nil || req.EntityData.Empty() {
		return [][]Constraint{{}}, nil
	}
	rows := [][]Constraint{}
	hasID := containsString(req.EntityData.Header, "id")
	for _, row := range req.EntityData.Data {
		constraints := []Constraint{}
		rowID := ""
		skip := false
		for i, field := range req.EntityData.Header {
			if i >= len(row) || row[i] == "" {
				return nil, lokiError("entity_data row contains an empty mapped attribute")
			}
			label, err := strictLokiEntityField(field, req.DataLinkMapping, req.StorageLinkMapping)
			if err != nil {
				return nil, err
			}
			if field == "id" && len(req.EntityIDs) > 0 && !containsString(req.EntityIDs, row[i]) {
				skip = true
				break
			}
			if field == "id" {
				rowID = row[i]
			}
			constraints = append(constraints, Constraint{Field: label, Op: "eq", Values: []string{row[i]}})
		}
		if skip {
			continue
		}
		if hasID && rowID == "" {
			return nil, lokiError("entity_data row has no id")
		}
		if len(constraints) > 0 {
			rows = append(rows, constraints)
		}
	}
	if len(rows) == 0 {
		return nil, lokiError("entity_data contains no matching mapped rows")
	}
	return rows, nil
}

type lokiMatcher struct {
	label string
	op    string
	value string
}

func renderLokiLogQL(generator string, constraints []Constraint) (string, error) {
	open, close, err := lokiSelectorBounds(generator)
	if err != nil {
		return "", err
	}
	matchers, err := parseLokiMatchers(generator[open+1 : close])
	if err != nil {
		return "", err
	}
	for _, c := range constraints {
		matcher, err := constraintToLokiMatcher(c)
		if err != nil {
			return "", err
		}
		found := false
		for i := range matchers {
			if matchers[i].label != matcher.label {
				continue
			}
			if matchers[i].value == "$"+matcher.label && matchers[i].op == "=" {
				matchers[i] = matcher
			} else if matchers[i] != matcher {
				return "", lokiError("conflicting Loki label conditions: " + matcher.label)
			}
			found = true
			break
		}
		if !found {
			matchers = append(matchers, matcher)
		}
	}
	if len(matchers) == 0 {
		return "", lokiError("Loki stream selector requires at least one label matcher")
	}
	validSelector := false
	for _, matcher := range matchers {
		if strings.HasPrefix(matcher.value, "$") {
			return "", lokiError("unbound Loki label placeholder: " + matcher.label)
		}
		if (matcher.op == "=" || matcher.op == "=~") && matcher.value != "" {
			validSelector = true
		}
	}
	if !validSelector {
		return "", lokiError("Loki stream selector needs a non-empty positive matcher")
	}
	sort.Slice(matchers, func(i, j int) bool { return matchers[i].label < matchers[j].label })
	parts := make([]string, 0, len(matchers))
	for _, m := range matchers {
		parts = append(parts, m.label+m.op+strconv.Quote(m.value))
	}
	return generator[:open] + "{" + strings.Join(parts, ",") + "}" + generator[close+1:], nil
}

func lokiSelectorBounds(query string) (int, int, error) {
	open, close := -1, -1
	quote := byte(0)
	escape := false
	for i := 0; i < len(query); i++ {
		c := query[i]
		if quote != 0 {
			if escape {
				escape = false
			} else if c == '\\' && quote == '"' {
				escape = true
			} else if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '`' {
			quote = c
			continue
		}
		if c == '{' {
			if open >= 0 {
				return 0, 0, lokiError("Loki generator must contain exactly one stream selector")
			}
			open = i
		} else if c == '}' {
			if open < 0 || close >= 0 {
				return 0, 0, lokiError("invalid Loki stream selector")
			}
			close = i
		}
	}
	if open < 0 || close < open || quote != 0 {
		return 0, 0, lokiError("Loki generator requires a valid stream selector")
	}
	return open, close, nil
}

func parseLokiMatchers(selector string) ([]lokiMatcher, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil, nil
	}
	parts := []string{}
	start, quoted, escaped := 0, false, false
	for i := range selector {
		switch selector[i] {
		case '\\':
			if quoted && !escaped {
				escaped = true
				continue
			}
		case '"':
			if !escaped {
				quoted = !quoted
			}
		case ',':
			if !quoted {
				parts = append(parts, selector[start:i])
				start = i + 1
			}
		}
		escaped = false
	}
	parts = append(parts, selector[start:])
	matchers := make([]lokiMatcher, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		var label, op, literal string
		for _, candidate := range []string{"=~", "!~", "!=", "="} {
			if idx := strings.Index(part, candidate); idx > 0 {
				label, op, literal = strings.TrimSpace(part[:idx]), candidate, strings.TrimSpace(part[idx+len(candidate):])
				break
			}
		}
		if !validLokiLabel(label) || len(literal) < 2 || literal[0] != '"' {
			return nil, lokiError("invalid Loki generator label matcher")
		}
		value, err := strconv.Unquote(literal)
		if err != nil {
			return nil, lokiError("invalid Loki generator label string")
		}
		matchers = append(matchers, lokiMatcher{label, op, value})
	}
	return matchers, nil
}

func constraintToLokiMatcher(c Constraint) (lokiMatcher, error) {
	if !validLokiLabel(c.Field) || len(c.Values) == 0 {
		return lokiMatcher{}, lokiError("invalid Loki label constraint")
	}
	values := append([]string{}, c.Values...)
	sort.Strings(values)
	switch c.Op {
	case "eq", "in":
		if len(values) == 1 {
			return lokiMatcher{c.Field, "=", values[0]}, nil
		}
		return lokiMatcher{c.Field, "=~", lokiRegexAlternatives(values)}, nil
	case "neq", "notin":
		if len(values) == 1 {
			return lokiMatcher{c.Field, "!=", values[0]}, nil
		}
		return lokiMatcher{c.Field, "!~", lokiRegexAlternatives(values)}, nil
	default:
		return lokiMatcher{}, lokiError(fmt.Sprintf("unsupported Loki label operator %q", c.Op))
	}
}

func lokiRegexAlternatives(values []string) string {
	escaped := make([]string, 0, len(values))
	for _, value := range values {
		escaped = append(escaped, regexp.QuoteMeta(value))
	}
	return strings.Join(escaped, "|")
}
