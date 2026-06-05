package query

import (
	"fmt"
	"strings"
	"unicode"
)

type logFilterNode struct {
	Kind     string
	Field    string
	Operator string
	Value    any
	Children []*logFilterNode
}

type logExprToken struct {
	Kind  string
	Value string
}

type logExprParser struct {
	tokens []logExprToken
	pos    int
}

func parseLogFilterExpression(raw string) (*logFilterNode, error) {
	tokens, err := tokenizeLogFilterExpression(raw)
	if err != nil {
		return nil, err
	}
	parser := logExprParser{tokens: tokens}
	expr, err := parser.parseOr()
	if err != nil {
		return nil, err
	}
	if !parser.atEnd() {
		return nil, fmt.Errorf("unexpected token %q", parser.peek().Value)
	}
	return expr, nil
}

func tokenizeLogFilterExpression(raw string) ([]logExprToken, error) {
	tokens := []logExprToken{}
	for i := 0; i < len(raw); {
		ch := raw[i]
		if unicode.IsSpace(rune(ch)) {
			i++
			continue
		}
		switch ch {
		case '(', ')', '[', ']', ',':
			tokens = append(tokens, logExprToken{Kind: string(ch), Value: string(ch)})
			i++
			continue
		case '\'', '"', '`':
			end := i + 1
			escaped := false
			for end < len(raw) {
				next := raw[end]
				if ch == '`' && next == '`' && end+1 < len(raw) && raw[end+1] == '`' {
					end += 2
					continue
				}
				if escaped {
					escaped = false
					end++
					continue
				}
				if next == '\\' && ch != '`' {
					escaped = true
					end++
					continue
				}
				if next == ch {
					break
				}
				end++
			}
			if end >= len(raw) {
				return nil, fmt.Errorf("unterminated quoted value")
			}
			tokens = append(tokens, logExprToken{Kind: "value", Value: stringValue(parseValue(raw[i : end+1]))})
			i = end + 1
			continue
		case '!', '=', '>', '<', ':':
			if i+1 < len(raw) {
				op := raw[i : i+2]
				if op == "!=" || op == "==" || op == ">=" || op == "<=" {
					tokens = append(tokens, logExprToken{Kind: "op", Value: op})
					i += 2
					continue
				}
			}
			tokens = append(tokens, logExprToken{Kind: "op", Value: string(ch)})
			i++
			continue
		default:
			start := i
			for i < len(raw) && !unicode.IsSpace(rune(raw[i])) && !isLogExprDelimiter(raw[i]) {
				i++
			}
			tokens = append(tokens, logExprToken{Kind: "ident", Value: raw[start:i]})
		}
	}
	return tokens, nil
}

func isLogExprDelimiter(ch byte) bool {
	switch ch {
	case '(', ')', '[', ']', ',', '!', '=', '>', '<', ':', '\'', '"', '`':
		return true
	default:
		return false
	}
}

func (p *logExprParser) parseOr() (*logFilterNode, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.matchKeyword("or") {
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = mergeLogFilterNode("or", left, right)
	}
	return left, nil
}

func (p *logExprParser) parseAnd() (*logFilterNode, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.matchKeyword("and") {
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = mergeLogFilterNode("and", left, right)
	}
	return left, nil
}

func (p *logExprParser) parseNot() (*logFilterNode, error) {
	if p.matchKeyword("not") {
		child, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return &logFilterNode{Kind: "not", Children: []*logFilterNode{child}}, nil
	}
	return p.parsePrimary()
}

func (p *logExprParser) parsePrimary() (*logFilterNode, error) {
	if p.matchKind("(") {
		expr, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if !p.matchKind(")") {
			return nil, fmt.Errorf("missing closing parenthesis")
		}
		return expr, nil
	}
	return p.parseComparison()
}

func (p *logExprParser) parseComparison() (*logFilterNode, error) {
	field, err := p.consumeField()
	if err != nil {
		return nil, err
	}
	operator, err := p.consumeOperator()
	if err != nil {
		return nil, err
	}
	switch operator {
	case "in", "not in":
		values, err := p.consumeList()
		if err != nil {
			return nil, err
		}
		return &logFilterNode{Kind: "comparison", Field: field, Operator: operator, Value: values}, nil
	case "like", "not like", "=", "==", "!=", ":", ">", ">=", "<", "<=":
		value, err := p.consumeValue()
		if err != nil {
			return nil, err
		}
		if operator == "==" {
			operator = "="
		}
		return &logFilterNode{Kind: "comparison", Field: field, Operator: operator, Value: value}, nil
	default:
		return nil, fmt.Errorf("unsupported operator %q", operator)
	}
}

func (p *logExprParser) consumeField() (string, error) {
	if p.atEnd() {
		return "", fmt.Errorf("expected field")
	}
	token := p.advance()
	if token.Kind != "ident" && token.Kind != "value" {
		return "", fmt.Errorf("expected field")
	}
	if token.Value == "" {
		return "", fmt.Errorf("field cannot be empty")
	}
	return token.Value, nil
}

func (p *logExprParser) consumeOperator() (string, error) {
	if p.matchKeyword("not") {
		if p.matchKeyword("in") {
			return "not in", nil
		}
		if p.matchKeyword("like") {
			return "not like", nil
		}
		return "", fmt.Errorf("expected in or like after not")
	}
	if p.matchKeyword("in") {
		return "in", nil
	}
	if p.matchKeyword("like") {
		return "like", nil
	}
	if p.atEnd() {
		return "", fmt.Errorf("expected operator")
	}
	token := p.advance()
	if token.Kind != "op" {
		return "", fmt.Errorf("expected operator")
	}
	return token.Value, nil
}

func (p *logExprParser) consumeList() ([]string, error) {
	closeKind := ""
	switch {
	case p.matchKind("["):
		closeKind = "]"
	case p.matchKind("("):
		closeKind = ")"
	default:
		return nil, fmt.Errorf("expected list")
	}

	values := []string{}
	for !p.atEnd() && !p.matchKind(closeKind) {
		value, err := p.consumeValue()
		if err != nil {
			return nil, err
		}
		values = append(values, value)
		if p.matchKind(closeKind) {
			break
		}
		if !p.matchKind(",") {
			return nil, fmt.Errorf("expected comma in list")
		}
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("list cannot be empty")
	}
	return values, nil
}

func (p *logExprParser) consumeValue() (string, error) {
	if p.atEnd() {
		return "", fmt.Errorf("expected value")
	}
	token := p.advance()
	if token.Kind != "ident" && token.Kind != "value" {
		return "", fmt.Errorf("expected value")
	}
	if token.Value == "" {
		return "", fmt.Errorf("value cannot be empty")
	}
	return token.Value, nil
}

func (p *logExprParser) matchKeyword(keyword string) bool {
	if p.atEnd() {
		return false
	}
	token := p.peek()
	if token.Kind != "ident" || !strings.EqualFold(token.Value, keyword) {
		return false
	}
	p.pos++
	return true
}

func (p *logExprParser) matchKind(kind string) bool {
	if p.atEnd() || p.peek().Kind != kind {
		return false
	}
	p.pos++
	return true
}

func (p *logExprParser) peek() logExprToken {
	return p.tokens[p.pos]
}

func (p *logExprParser) advance() logExprToken {
	token := p.tokens[p.pos]
	p.pos++
	return token
}

func (p *logExprParser) atEnd() bool {
	return p.pos >= len(p.tokens)
}

func mergeLogFilterNode(kind string, left, right *logFilterNode) *logFilterNode {
	children := []*logFilterNode{}
	if left.Kind == kind {
		children = append(children, left.Children...)
	} else {
		children = append(children, left)
	}
	if right.Kind == kind {
		children = append(children, right.Children...)
	} else {
		children = append(children, right)
	}
	return &logFilterNode{Kind: kind, Children: children}
}

func logFilterToElasticsearch(node *logFilterNode, fieldMapper func(string) string) map[string]any {
	if node == nil {
		return map[string]any{"match_all": map[string]any{}}
	}
	switch node.Kind {
	case "and":
		filters := []map[string]any{}
		for _, child := range node.Children {
			filters = append(filters, logFilterToElasticsearch(child, fieldMapper))
		}
		return map[string]any{"bool": map[string]any{"filter": filters}}
	case "or":
		should := []map[string]any{}
		for _, child := range node.Children {
			should = append(should, logFilterToElasticsearch(child, fieldMapper))
		}
		return map[string]any{"bool": map[string]any{"should": should, "minimum_should_match": 1}}
	case "not":
		if len(node.Children) == 0 {
			return map[string]any{"match_all": map[string]any{}}
		}
		return map[string]any{"bool": map[string]any{"must_not": []map[string]any{logFilterToElasticsearch(node.Children[0], fieldMapper)}}}
	case "comparison":
		return logComparisonToElasticsearch(node, fieldMapper)
	default:
		return map[string]any{"match_all": map[string]any{}}
	}
}

func logComparisonToElasticsearch(node *logFilterNode, fieldMapper func(string) string) map[string]any {
	field := node.Field
	if fieldMapper != nil {
		field = fieldMapper(field)
	}
	switch node.Operator {
	case "=", ":", "==":
		return map[string]any{"term": map[string]any{field: node.Value}}
	case "!=":
		return map[string]any{"bool": map[string]any{"must_not": []map[string]any{{"term": map[string]any{field: node.Value}}}}}
	case "in":
		return map[string]any{"terms": map[string]any{field: node.Value}}
	case "not in":
		return map[string]any{"bool": map[string]any{"must_not": []map[string]any{{"terms": map[string]any{field: node.Value}}}}}
	case "like":
		return map[string]any{"wildcard": map[string]any{field: sqlLikeToWildcard(stringValue(node.Value))}}
	case "not like":
		return map[string]any{"bool": map[string]any{"must_not": []map[string]any{{"wildcard": map[string]any{field: sqlLikeToWildcard(stringValue(node.Value))}}}}}
	case ">", ">=", "<", "<=":
		rangeOp := map[string]string{">": "gt", ">=": "gte", "<": "lt", "<=": "lte"}[node.Operator]
		return map[string]any{"range": map[string]any{field: map[string]any{rangeOp: node.Value}}}
	default:
		return map[string]any{"query_string": map[string]any{"query": node.Field + " " + node.Operator + " " + stringValue(node.Value)}}
	}
}

func sqlLikeToWildcard(value string) string {
	value = strings.ReplaceAll(value, "%", "*")
	value = strings.ReplaceAll(value, "_", "?")
	return value
}
