package query

import (
	"strconv"
	"strings"
	"unicode"

	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

const defaultLimit = 100

func Parse(req model.QueryRequest) (model.QueryPlan, error) {
	ast, err := ParseAST(req.Query)
	if err != nil {
		return model.QueryPlan{}, err
	}
	return planFromAST(req, ast), nil
}

func ParseAST(query string) (AST, error) {
	queryText := strings.TrimSpace(query)
	source, err := detectSource(queryText)
	if err != nil {
		return AST{}, err
	}

	ast := AST{
		Source:  source,
		Query:   query,
		Filters: map[string]any{},
	}

	segments := splitTopLevel(queryText, '|')
	if len(segments) == 0 {
		return AST{}, apperrors.New(apperrors.CodeQueryParseError, "query is empty")
	}

	first := strings.TrimSpace(segments[0])
	first = strings.TrimSpace(strings.TrimPrefix(first, source))
	if first != "" {
		if err := parsePipelineSegment(&ast, first); err != nil {
			return AST{}, err
		}
	}
	for _, segment := range segments[1:] {
		if err := parsePipelineSegment(&ast, strings.TrimSpace(segment)); err != nil {
			return AST{}, err
		}
	}

	if err := validateAST(ast); err != nil {
		return AST{}, err
	}

	return ast, nil
}

func planFromAST(req model.QueryRequest, ast AST) model.QueryPlan {
	filters := make(map[string]any, len(ast.Filters))
	for key, value := range ast.Filters {
		filters[key] = resolveParamValue(value, req.Params)
	}

	pipeline := append([]model.QueryPipelineOperator(nil), ast.Operators...)
	operators := make([]string, 0, len(pipeline))
	predicates := []model.QueryPredicate{}
	project := []string{}
	sortSpecs := []model.QuerySort{}
	var graphCall *model.GraphCallPlan
	var entityCall *model.EntityCallPlan
	topk := intFilter(filters["topk"])

	for idx, operator := range pipeline {
		if operator.Predicate != nil {
			predicate := *operator.Predicate
			predicate.Value = resolveParamValue(predicate.Value, req.Params)
			pipeline[idx].Predicate = &predicate
			operator.Predicate = &predicate
		}
		operators = append(operators, operator.Name)
		if operator.Predicate != nil {
			predicates = append(predicates, *operator.Predicate)
		}
		if len(operator.Project) > 0 {
			project = append([]string(nil), operator.Project...)
		}
		if operator.Sort != nil {
			sortSpecs = append(sortSpecs, *operator.Sort)
		}
		if operator.GraphCall != nil {
			copy := *operator.GraphCall
			graphCall = &copy
		}
		if operator.EntityCall != nil {
			copy := *operator.EntityCall
			copy.Arguments = resolveArgumentValues(copy.Arguments, req.Params)
			copy.NamedArguments = resolveNamedArgumentValues(copy.NamedArguments, req.Params)
			pipeline[idx].EntityCall = &copy
			operator.EntityCall = &copy
			entityCall = &copy
		}
	}

	limit := defaultLimit
	if topk > 0 {
		limit = topk
	}
	if ast.Limit > 0 {
		limit = ast.Limit
	}
	if req.Limit > 0 {
		limit = req.Limit
	}

	depth := ast.Depth
	if graphCall != nil && graphCall.Depth > 0 {
		depth = graphCall.Depth
	}

	return model.QueryPlan{
		Source:      ast.Source,
		Query:       req.Query,
		Filters:     filters,
		Operators:   operators,
		Pipeline:    pipeline,
		Predicates:  predicates,
		Project:     project,
		Sort:        sortSpecs,
		GraphCall:   graphCall,
		EntityCall:  entityCall,
		TopK:        topk,
		TimeRange:   req.TimeRange,
		Params:      req.Params,
		EntityData:  req.EntityFilterData(),
		Limit:       limit,
		Depth:       depth,
		TimeoutMS:   req.TimeoutMS,
		Format:      req.Format,
		IncludeSpec: req.IncludeSpec,
	}
}

func resolveParamValue(value any, params map[string]any) any {
	switch typed := value.(type) {
	case string:
		name, ok := paramName(typed)
		if !ok || params == nil {
			return typed
		}
		if replacement, exists := params[name]; exists {
			return replacement
		}
		return typed
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			resolved := resolveParamValue(item, params)
			text, ok := resolved.(string)
			if !ok {
				return typed
			}
			out = append(out, text)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, resolveParamValue(item, params))
		}
		return out
	default:
		return value
	}
}

func resolveArgumentValues(args []any, params map[string]any) []any {
	if len(args) == 0 {
		return nil
	}
	out := make([]any, 0, len(args))
	for _, item := range args {
		out = append(out, resolveParamValue(item, params))
	}
	return out
}

func resolveNamedArgumentValues(args map[string]any, params map[string]any) map[string]any {
	if len(args) == 0 {
		return nil
	}
	out := make(map[string]any, len(args))
	for key, value := range args {
		out[key] = resolveParamValue(value, params)
	}
	return out
}

func paramName(value string) (string, bool) {
	if !strings.HasPrefix(value, "$") || len(value) == 1 {
		return "", false
	}
	name := strings.TrimPrefix(value, "$")
	for _, r := range name {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			return "", false
		}
	}
	return name, true
}

func detectSource(queryText string) (string, error) {
	for _, source := range []string{".umodel", ".entity_set", ".entity", ".topo", ".runbook_set"} {
		if strings.HasPrefix(queryText, source) && hasSourceBoundary(queryText, len(source)) {
			return source, nil
		}
	}
	return "", apperrors.New(apperrors.CodeQueryParseError, "query must start with .umodel, .entity_set, .entity, .topo, or .runbook_set")
}

func validateAST(ast AST) error {
	if ast.Source != ".entity_set" {
		return nil
	}
	if stringFilter(ast.Filters["domain"]) == "" {
		return apperrors.New(apperrors.CodeQueryParseError, ".entity_set requires with(domain=...)")
	}
	if stringFilter(ast.Filters["name"]) == "" {
		return apperrors.New(apperrors.CodeQueryParseError, ".entity_set requires with(name=...)")
	}
	for _, operator := range ast.Operators {
		if operator.EntityCall != nil {
			return nil
		}
	}
	return apperrors.New(apperrors.CodeQueryParseError, ".entity_set requires entity-call __list_method__(), list_data_set(...), get_logs(...), or get_metrics(...)")
}

func hasSourceBoundary(queryText string, pos int) bool {
	if pos >= len(queryText) {
		return true
	}
	next := rune(queryText[pos])
	return unicode.IsSpace(next) || next == '|'
}

func parsePipelineSegment(ast *AST, segment string) error {
	segment = strings.Trim(strings.TrimSpace(segment), ";")
	if segment == "" {
		return nil
	}

	switch {
	case strings.HasPrefix(segment, "with("):
		filters, err := parseWithFilters(segment)
		if err != nil {
			return err
		}
		for key, value := range filters {
			ast.Filters[key] = value
		}
		ast.Operators = append(ast.Operators, model.QueryPipelineOperator{Name: "with"})
	case strings.HasPrefix(segment, "where "):
		predicate, err := parsePredicate(strings.TrimSpace(strings.TrimPrefix(segment, "where")))
		if err != nil {
			return err
		}
		ast.Operators = append(ast.Operators, model.QueryPipelineOperator{Name: "where", Predicate: &predicate})
	case strings.HasPrefix(segment, "project "):
		fields := parseCSVFields(strings.TrimSpace(strings.TrimPrefix(segment, "project")))
		if len(fields) == 0 {
			return apperrors.New(apperrors.CodeQueryParseError, "project requires at least one field")
		}
		ast.Operators = append(ast.Operators, model.QueryPipelineOperator{Name: "project", Project: fields})
	case strings.HasPrefix(segment, "sort "):
		sortSpec, err := parseSort(strings.TrimSpace(strings.TrimPrefix(segment, "sort")))
		if err != nil {
			return err
		}
		ast.Operators = append(ast.Operators, model.QueryPipelineOperator{Name: "sort", Sort: &sortSpec})
	case strings.HasPrefix(segment, "limit "):
		limit, err := parsePositiveInt(strings.TrimSpace(strings.TrimPrefix(segment, "limit")), "limit")
		if err != nil {
			return err
		}
		ast.Limit = limit
		ast.Operators = append(ast.Operators, model.QueryPipelineOperator{Name: "limit", Limit: limit})
	case strings.HasPrefix(segment, "entity-call "):
		if ast.Source != ".entity_set" {
			return apperrors.New(apperrors.CodeQueryParseError, "entity-call is only supported for .entity_set")
		}
		entityCall, err := parseEntityCall(strings.TrimSpace(strings.TrimPrefix(segment, "entity-call")))
		if err != nil {
			return err
		}
		ast.Operators = append(ast.Operators, model.QueryPipelineOperator{Name: "entity-call:" + entityCall.Name, EntityCall: &entityCall})
	case strings.HasPrefix(segment, "graph-call "):
		if ast.Source != ".topo" {
			return apperrors.New(apperrors.CodeQueryParseError, "graph-call is only supported for .topo")
		}
		graphCall, err := parseGraphCall(strings.TrimSpace(strings.TrimPrefix(segment, "graph-call")))
		if err != nil {
			return err
		}
		if graphCall.Depth > 0 {
			ast.Depth = graphCall.Depth
		}
		ast.Operators = append(ast.Operators, model.QueryPipelineOperator{Name: "graph-call:" + graphCall.Name, GraphCall: &graphCall})
	case strings.HasPrefix(segment, "graph-match"):
		if ast.Source != ".topo" {
			return apperrors.New(apperrors.CodeQueryParseError, "graph-match is only supported for .topo")
		}
		expression := strings.TrimSpace(strings.TrimPrefix(segment, "graph-match"))
		if expression == "" {
			return apperrors.New(apperrors.CodeQueryParseError, "graph-match requires a path expression")
		}
		ast.Operators = append(ast.Operators, model.QueryPipelineOperator{Name: "graph-match", Expression: expression})
	default:
		return apperrors.WithDetails(apperrors.CodeQueryParseError, "unsupported query operator", map[string]string{"operator": firstWord(segment)})
	}
	return nil
}

func parseWithFilters(segment string) (map[string]any, error) {
	start := strings.Index(segment, "with(")
	if start < 0 {
		return map[string]any{}, nil
	}
	start += len("with(")
	end := matchingParen(segment, start-1)
	if end < 0 {
		return nil, apperrors.New(apperrors.CodeQueryParseError, "with(...) is not closed")
	}

	filters := map[string]any{}
	for _, part := range splitTopLevel(segment[start:end], ',') {
		if strings.TrimSpace(part) == "" {
			continue
		}
		key, value, ok := cutTopLevel(part, '=')
		if !ok {
			return nil, apperrors.New(apperrors.CodeQueryParseError, "with(...) filters must use key=value")
		}
		filters[strings.TrimSpace(key)] = parseValue(strings.TrimSpace(value))
	}
	return filters, nil
}

func parsePredicate(expression string) (model.QueryPredicate, error) {
	expression = strings.TrimSpace(expression)
	if strings.HasPrefix(expression, "contains(") {
		start := len("contains(")
		end := matchingParen(expression, start-1)
		if end < 0 {
			return model.QueryPredicate{}, apperrors.New(apperrors.CodeQueryParseError, "contains(...) is not closed")
		}
		args := splitTopLevel(expression[start:end], ',')
		if len(args) != 2 {
			return model.QueryPredicate{}, apperrors.New(apperrors.CodeQueryParseError, "contains(...) requires field and value")
		}
		return model.QueryPredicate{Field: strings.TrimSpace(args[0]), Op: "contains", Value: parseValue(args[1])}, nil
	}

	ops := []string{"==", "!=", ">=", "<=", "=", "~", ">", "<"}
	if idx, op := topLevelOperatorIndex(expression, ops); idx >= 0 {
		field := strings.TrimSpace(expression[:idx])
		value := strings.TrimSpace(expression[idx+len(op):])
		if field == "" || value == "" {
			return model.QueryPredicate{}, apperrors.New(apperrors.CodeQueryParseError, "where requires field, operator, and value")
		}
		if op == "==" {
			op = "="
		}
		return model.QueryPredicate{Field: field, Op: op, Value: parseValue(value)}, nil
	}
	return model.QueryPredicate{}, apperrors.New(apperrors.CodeQueryParseError, "unsupported where predicate")
}

func parseSort(expression string) (model.QuerySort, error) {
	parts := strings.Fields(expression)
	if len(parts) == 0 {
		return model.QuerySort{}, apperrors.New(apperrors.CodeQueryParseError, "sort requires a field")
	}
	desc := false
	if len(parts) > 1 {
		switch strings.ToLower(parts[1]) {
		case "desc":
			desc = true
		case "asc":
			desc = false
		default:
			return model.QuerySort{}, apperrors.New(apperrors.CodeQueryParseError, "sort direction must be asc or desc")
		}
	}
	return model.QuerySort{Field: parts[0], Desc: desc}, nil
}

func parseEntityCall(expression string) (model.EntityCallPlan, error) {
	open := strings.Index(expression, "(")
	if open < 0 {
		return model.EntityCallPlan{}, apperrors.New(apperrors.CodeQueryParseError, "entity-call requires a function call")
	}
	name := strings.TrimSpace(expression[:open])
	end := matchingParen(expression, open)
	if !isValidCallName(name) || end < 0 || strings.TrimSpace(expression[end+1:]) != "" {
		return model.EntityCallPlan{}, apperrors.New(apperrors.CodeQueryParseError, "entity-call is malformed")
	}

	rawArgs := strings.TrimSpace(expression[open+1 : end])
	args := []any{}
	namedArgs := map[string]any{}
	if rawArgs != "" {
		for _, rawArg := range splitTopLevel(rawArgs, ',') {
			rawArg = strings.TrimSpace(rawArg)
			if rawArg == "" {
				return model.EntityCallPlan{}, apperrors.New(apperrors.CodeQueryParseError, "entity-call arguments cannot be empty")
			}
			if key, value, ok := cutTopLevel(rawArg, '='); ok && isValidArgumentName(strings.TrimSpace(key)) {
				key = strings.TrimSpace(key)
				if _, exists := namedArgs[key]; exists {
					return model.EntityCallPlan{}, apperrors.WithDetails(apperrors.CodeQueryParseError, "entity-call named argument is duplicated", map[string]string{"argument": key})
				}
				namedArgs[key] = parseValue(strings.TrimSpace(value))
				continue
			}
			args = append(args, parseValue(rawArg))
		}
	}
	if len(namedArgs) == 0 {
		namedArgs = nil
	}
	return model.EntityCallPlan{Name: name, Arguments: args, NamedArguments: namedArgs}, nil
}

func isValidCallName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if unicode.IsSpace(r) || r == '|' || r == '(' || r == ')' {
			return false
		}
	}
	return true
}

func isValidArgumentName(name string) bool {
	if name == "" {
		return false
	}
	for idx, r := range name {
		if idx == 0 && !(unicode.IsLetter(r) || r == '_') {
			return false
		}
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			return false
		}
	}
	return true
}

func parseGraphCall(expression string) (model.GraphCallPlan, error) {
	open := strings.Index(expression, "(")
	if open < 0 {
		return model.GraphCallPlan{}, apperrors.New(apperrors.CodeQueryParseError, "graph-call requires a function call")
	}
	name := strings.TrimSpace(expression[:open])
	end := matchingParen(expression, open)
	if name == "" || end < 0 || strings.TrimSpace(expression[end+1:]) != "" {
		return model.GraphCallPlan{}, apperrors.New(apperrors.CodeQueryParseError, "graph-call is malformed")
	}
	args := splitTopLevel(expression[open+1:end], ',')

	switch name {
	case "getNeighborNodes":
		if len(args) != 3 {
			return model.GraphCallPlan{}, apperrors.New(apperrors.CodeQueryParseError, "getNeighborNodes requires type, depth, and nodeList")
		}
		traversalType := stringValue(parseValue(args[0]))
		if !isAllowedNeighborTraversalType(traversalType) {
			return model.GraphCallPlan{}, apperrors.New(apperrors.CodeQueryParseError, "getNeighborNodes type must be sequence, sequence_in, sequence_out, or full")
		}
		depth, ok := parseIntValue(args[1])
		if !ok || depth <= 0 {
			return model.GraphCallPlan{}, apperrors.New(apperrors.CodeQueryParseError, "graph-call depth requires a positive integer")
		}
		nodes, err := parseGraphNodeList(args[2])
		if err != nil {
			return model.GraphCallPlan{}, err
		}
		return model.GraphCallPlan{
			Name:    name,
			Type:    traversalType,
			Depth:   depth,
			Nodes:   nodes,
			SeedIDs: nodeSeedIDs(nodes),
		}, nil
	case "getDirectRelations":
		if len(args) != 1 {
			return model.GraphCallPlan{}, apperrors.New(apperrors.CodeQueryParseError, "getDirectRelations requires nodeList")
		}
		nodes, err := parseGraphNodeList(args[0])
		if err != nil {
			return model.GraphCallPlan{}, err
		}
		return model.GraphCallPlan{
			Name:    name,
			Depth:   1,
			Nodes:   nodes,
			SeedIDs: nodeSeedIDs(nodes),
		}, nil
	case "cypher":
		if len(args) != 1 {
			return model.GraphCallPlan{}, apperrors.New(apperrors.CodeQueryParseError, "cypher requires one query string")
		}
		cypher, ok := parseBacktickValue(args[0])
		if !ok || strings.TrimSpace(cypher) == "" {
			return model.GraphCallPlan{}, apperrors.New(apperrors.CodeQueryParseError, "cypher requires a backtick query string")
		}
		return model.GraphCallPlan{Name: name, Cypher: cypher}, nil
	default:
		return model.GraphCallPlan{Name: name}, nil
	}
}

func isAllowedNeighborTraversalType(value string) bool {
	switch value {
	case "sequence", "sequence_in", "sequence_out", "full":
		return true
	default:
		return false
	}
}

func parseGraphNodeList(raw string) ([]model.GraphNodeSelector, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "[") || !strings.HasSuffix(raw, "]") {
		return nil, apperrors.New(apperrors.CodeQueryParseError, "nodeList must be an array of node selectors")
	}
	content := strings.TrimSpace(raw[1 : len(raw)-1])
	if content == "" {
		return nil, apperrors.New(apperrors.CodeQueryParseError, "nodeList requires at least one node selector")
	}
	parts := splitTopLevel(content, ',')
	nodes := make([]model.GraphNodeSelector, 0, len(parts))
	for _, part := range parts {
		node, err := parseGraphNodeSelector(part)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func parseGraphNodeSelector(raw string) (model.GraphNodeSelector, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "(") || !strings.HasSuffix(raw, ")") {
		return model.GraphNodeSelector{}, apperrors.New(apperrors.CodeQueryParseError, "node selector must use (...) syntax")
	}
	inner := strings.TrimSpace(raw[1 : len(raw)-1])
	node := model.GraphNodeSelector{Raw: raw, Properties: map[string]any{}}

	propertyStart := topLevelIndex(inner, '{')
	labelPart := inner
	if propertyStart >= 0 {
		propertyEnd := matchingBrace(inner, propertyStart)
		if propertyEnd < 0 || strings.TrimSpace(inner[propertyEnd+1:]) != "" {
			return model.GraphNodeSelector{}, apperrors.New(apperrors.CodeQueryParseError, "node selector properties are malformed")
		}
		labelPart = strings.TrimSpace(inner[:propertyStart])
		properties, err := parseGraphProperties(inner[propertyStart+1 : propertyEnd])
		if err != nil {
			return model.GraphNodeSelector{}, err
		}
		node.Properties = properties
	}

	if labelPart != "" {
		colon := topLevelIndex(labelPart, ':')
		if colon < 0 {
			return model.GraphNodeSelector{}, apperrors.New(apperrors.CodeQueryParseError, "node selector requires a label")
		}
		node.Variable = strings.TrimSpace(labelPart[:colon])
		label, ok := parseQuotedLabel(strings.TrimSpace(labelPart[colon+1:]))
		if !ok || label == "" {
			return model.GraphNodeSelector{}, apperrors.New(apperrors.CodeQueryParseError, "node selector label must be quoted")
		}
		node.Label = label
	}

	id := stringValue(node.Properties["__entity_id__"])
	if node.Label == "" || id == "" {
		return model.GraphNodeSelector{}, apperrors.New(apperrors.CodeQueryParseError, "node selector requires label and __entity_id__")
	}
	if !model.IsEntityID(id) {
		return model.GraphNodeSelector{}, apperrors.New(apperrors.CodeQueryParseError, "node selector __entity_id__ must be a 128-bit lowercase hex string")
	}
	return node, nil
}

func parseGraphProperties(raw string) (map[string]any, error) {
	properties := map[string]any{}
	for _, part := range splitTopLevel(raw, ',') {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, ok := cutTopLevel(part, ':')
		if !ok {
			return nil, apperrors.New(apperrors.CodeQueryParseError, "node selector properties must use key: value")
		}
		key = strings.TrimSpace(key)
		if label, ok := parseQuotedLabel(key); ok {
			key = label
		}
		if key == "" {
			return nil, apperrors.New(apperrors.CodeQueryParseError, "node selector property key is empty")
		}
		properties[key] = parseValue(value)
	}
	return properties, nil
}

func parseQuotedLabel(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 {
		return "", false
	}
	switch raw[0] {
	case '"', '\'':
		if raw[len(raw)-1] != raw[0] {
			return "", false
		}
		return unescapeSearchString(raw[1:len(raw)-1], raw[0]), true
	case '`':
		value, ok := parseBacktickValue(raw)
		return value, ok
	default:
		return "", false
	}
}

func nodeSeedIDs(nodes []model.GraphNodeSelector) []string {
	ids := make([]string, 0, len(nodes))
	for _, node := range nodes {
		id := stringValue(node.Properties["__entity_id__"])
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func parsePositiveInt(raw, field string) (int, error) {
	parts := strings.Fields(strings.Trim(raw, ";"))
	if len(parts) == 0 {
		return 0, apperrors.WithDetails(apperrors.CodeQueryParseError, field+" requires a positive integer", map[string]string{"field": field})
	}
	n, err := strconv.Atoi(strings.Trim(parts[0], ";"))
	if err != nil || n <= 0 {
		return 0, apperrors.WithDetails(apperrors.CodeQueryParseError, field+" requires a positive integer", map[string]string{"field": field})
	}
	return n, nil
}

func parseCSVFields(expression string) []string {
	fields := []string{}
	for _, part := range splitTopLevel(expression, ',') {
		field := strings.TrimSpace(strings.Trim(part, ";"))
		if field != "" {
			fields = append(fields, field)
		}
	}
	return fields
}

func matchingParen(text string, open int) int {
	depth := 0
	quote := rune(0)
	for i, r := range text[open:] {
		pos := open + i
		if quote != 0 {
			if quote == '`' && r == '`' && isDoubledBacktick(text, pos) {
				continue
			}
			if r == quote && !isEscaped(text, pos) {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' || r == '`' {
			quote = r
			continue
		}
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return pos
			}
		}
	}
	return -1
}

func splitTopLevel(text string, sep rune) []string {
	parts := []string{}
	depth := 0
	quote := rune(0)
	start := 0
	for i, r := range text {
		if quote != 0 {
			if quote == '`' && r == '`' && isDoubledBacktick(text, i) {
				continue
			}
			if r == quote && !isEscaped(text, i) {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' || r == '`' {
			quote = r
			continue
		}
		switch r {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		default:
			if r == sep && depth == 0 {
				parts = append(parts, text[start:i])
				start = i + len(string(r))
			}
		}
	}
	parts = append(parts, text[start:])
	return parts
}

func matchingBrace(text string, open int) int {
	depth := 0
	quote := rune(0)
	for i, r := range text[open:] {
		pos := open + i
		if quote != 0 {
			if quote == '`' && r == '`' && isDoubledBacktick(text, pos) {
				continue
			}
			if r == quote && !isEscaped(text, pos) {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' || r == '`' {
			quote = r
			continue
		}
		switch r {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return pos
			}
		}
	}
	return -1
}

func topLevelIndex(text string, target rune) int {
	depth := 0
	quote := rune(0)
	for i, r := range text {
		if quote != 0 {
			if quote == '`' && r == '`' && isDoubledBacktick(text, i) {
				continue
			}
			if r == quote && !isEscaped(text, i) {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' || r == '`' {
			quote = r
			continue
		}
		switch r {
		case '(', '[', '{':
			if r == target && depth == 0 {
				return i
			}
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		default:
			if r == target && depth == 0 {
				return i
			}
		}
	}
	return -1
}

func cutTopLevel(text string, sep rune) (string, string, bool) {
	idx := topLevelIndex(text, sep)
	if idx < 0 {
		return "", "", false
	}
	return text[:idx], text[idx+len(string(sep)):], true
}

// topLevelOperatorIndex returns the byte offset and matched operator of the
// first operator in ops that sits outside any quoted string or bracket group,
// so a quoted value containing operator characters (e.g. "a<=b") is not
// mistaken for the predicate's operator. List multi-character operators before
// their single-character prefixes (== before =, >= before >) so the longest
// match wins at a given position.
func topLevelOperatorIndex(text string, ops []string) (int, string) {
	depth := 0
	quote := rune(0)
	for i, r := range text {
		if quote != 0 {
			if quote == '`' && r == '`' && isDoubledBacktick(text, i) {
				continue
			}
			if r == quote && !isEscaped(text, i) {
				quote = 0
			}
			continue
		}
		switch r {
		case '\'', '"', '`':
			quote = r
			continue
		case '(', '[', '{':
			depth++
			continue
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth != 0 {
			continue
		}
		for _, op := range ops {
			if strings.HasPrefix(text[i:], op) {
				return i, op
			}
		}
	}
	return -1, ""
}

func parseValue(value string) any {
	value = strings.TrimSpace(strings.Trim(value, ";"))
	if len(value) >= 2 {
		if (value[0] == '\'' && value[len(value)-1] == '\'') || (value[0] == '"' && value[len(value)-1] == '"') {
			return unescapeSearchString(value[1:len(value)-1], value[0])
		}
		if parsed, ok := parseBacktickValue(value); ok {
			return parsed
		}
	}
	if tuple, ok := parseTupleValue(value); ok {
		return tuple
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		rawItems := splitTopLevel(strings.TrimSpace(value[1:len(value)-1]), ',')
		items := make([]any, 0, len(rawItems))
		for _, item := range rawItems {
			items = append(items, parseValue(strings.TrimSpace(item)))
		}
		if stringsOnly(items) {
			out := make([]string, 0, len(items))
			for _, item := range items {
				out = append(out, item.(string))
			}
			return out
		}
		return items
	}
	if n, err := strconv.Atoi(value); err == nil {
		return n
	}
	if strings.Contains(value, ".") {
		if n, err := strconv.ParseFloat(value, 64); err == nil {
			return n
		}
	}
	if strings.EqualFold(value, "true") {
		return true
	}
	if strings.EqualFold(value, "false") {
		return false
	}
	return strings.TrimFunc(value, unicode.IsSpace)
}

func parseTupleValue(value string) ([]string, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "(") || !strings.HasSuffix(value, ")") {
		return nil, false
	}
	items := splitTopLevel(strings.TrimSpace(value[1:len(value)-1]), ',')
	if len(items) == 0 {
		return nil, false
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		parsed := parseValue(item)
		text, ok := parsed.(string)
		if !ok {
			return nil, false
		}
		out = append(out, text)
	}
	return out, true
}

func parseBacktickValue(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if len(value) < 2 || value[0] != '`' || value[len(value)-1] != '`' {
		return "", false
	}
	return strings.ReplaceAll(value[1:len(value)-1], "``", "`"), true
}

func unescapeSearchString(value string, quote byte) string {
	var builder strings.Builder
	escaped := false
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if escaped {
			switch ch {
			case '\\', byte(quote):
				builder.WriteByte(ch)
			default:
				builder.WriteByte('\\')
				builder.WriteByte(ch)
			}
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		builder.WriteByte(ch)
	}
	if escaped {
		builder.WriteByte('\\')
	}
	return builder.String()
}

func isEscaped(text string, pos int) bool {
	count := 0
	for i := pos - 1; i >= 0 && text[i] == '\\'; i-- {
		count++
	}
	return count%2 == 1
}

func isDoubledBacktick(text string, pos int) bool {
	return (pos > 0 && text[pos-1] == '`') || (pos+1 < len(text) && text[pos+1] == '`')
}

func stringsOnly(items []any) bool {
	for _, item := range items {
		if _, ok := item.(string); !ok {
			return false
		}
	}
	return true
}

func parseIntValue(value string) (int, bool) {
	parsed := parseValue(value)
	switch typed := parsed.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case string:
		n, err := strconv.Atoi(typed)
		return n, err == nil
	default:
		return 0, false
	}
}

func intFilter(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		n, err := strconv.Atoi(typed)
		if err == nil {
			return n
		}
	}
	return 0
}

func stringSliceValue(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			out = append(out, stringValue(item))
		}
		return out
	case string:
		if typed == "" {
			return nil
		}
		return []string{typed}
	default:
		return nil
	}
}

func firstWord(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
