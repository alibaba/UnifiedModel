package cypher

import (
	"fmt"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

type Node struct {
	ID         string
	Labels     []string
	Properties map[string]any
}

type Edge struct {
	ID         string
	From       string
	To         string
	Type       string
	Properties map[string]any
}

type Path struct {
	Nodes []Node
	Rels  []Edge
}

type Graph struct {
	Nodes map[string]Node
	Edges []Edge
}

type Result struct {
	Columns []string
	Rows    []map[string]any
	Limit   int
}

type Options struct {
	Limit int
}

type graphIndex struct {
	nodes     map[string]Node
	edges     []Edge
	adjacency map[string][]Edge
}

type clause struct {
	kind string
	body string
}

type row map[string]any

type projection struct {
	expr      string
	alias     string
	aggregate string
	distinct  bool
}

type nodePattern struct {
	variable   string
	labels     []string
	properties map[string]string
}

type relPattern struct {
	variable string
	types    []string
	min      int
	max      int
	dir      string
}

type pathPattern struct {
	pathVar string
	nodes   []nodePattern
	rels    []relPattern
}

var entityIDLiteralPattern = regexp.MustCompile(`(?is)(?:^|[.\s{,])__entity_id__\s*(?::|=)\s*['"]([^'"]+)['"]`)

func Execute(query string, graph Graph, params map[string]any, options Options) (Result, error) {
	if err := ValidateReadOnly(query); err != nil {
		return Result{}, err
	}
	query = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(query), ";"))
	limit := options.Limit
	if limit <= 0 {
		limit = 100
	}
	idx := newGraphIndex(graph)
	parts := splitUnion(query)
	allRows := []map[string]any{}
	var columns []string
	for _, part := range parts {
		rows, cols, err := executePart(part.query, idx, params)
		if err != nil {
			return Result{}, err
		}
		if columns == nil {
			columns = cols
		}
		allRows = append(allRows, rows...)
		if !part.all {
			allRows = distinctRows(allRows, columns)
		}
	}
	if len(allRows) > limit {
		allRows = allRows[:limit]
	}
	return Result{Columns: columns, Rows: allRows, Limit: limit}, nil
}

func ValidateReadOnly(query string) error {
	trimmed := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(query), ";"))
	if trimmed == "" {
		return apperrors.New(apperrors.CodeQueryPlanError, "cypher query is empty")
	}
	first := strings.ToLower(firstKeyword(trimmed))
	switch first {
	case "match", "optional", "with", "unwind":
	default:
		return apperrors.New(apperrors.CodeQueryPlanError, "cypher query must start with MATCH, OPTIONAL MATCH, WITH, or UNWIND")
	}
	for _, token := range readOnlyTokens(trimmed) {
		switch token {
		case "call", "create", "delete", "detach", "drop", "load", "merge", "remove", "set":
			return apperrors.WithDetails(apperrors.CodeQueryPlanError, "cypher query must be read-only", map[string]string{"keyword": token})
		}
	}
	for _, match := range entityIDLiteralPattern.FindAllStringSubmatch(trimmed, -1) {
		if len(match) > 1 && !model.IsEntityID(match[1]) {
			return apperrors.New(apperrors.CodeQueryPlanError, "cypher __entity_id__ must be a 128-bit lowercase hex string")
		}
	}
	return nil
}

func newGraphIndex(graph Graph) graphIndex {
	idx := graphIndex{
		nodes:     map[string]Node{},
		edges:     append([]Edge(nil), graph.Edges...),
		adjacency: map[string][]Edge{},
	}
	for key, node := range graph.Nodes {
		if node.ID == "" {
			node.ID = key
		}
		node.Properties = cloneMap(node.Properties)
		idx.nodes[node.ID] = node
	}
	sort.SliceStable(idx.edges, func(i, j int) bool {
		return idx.edges[i].ID < idx.edges[j].ID
	})
	for _, edge := range idx.edges {
		edge.Properties = cloneMap(edge.Properties)
		idx.adjacency[edge.From] = append(idx.adjacency[edge.From], edge)
		idx.adjacency[edge.To] = append(idx.adjacency[edge.To], edge)
	}
	return idx
}

type unionPart struct {
	query string
	all   bool
}

func splitUnion(query string) []unionPart {
	matches := topLevelKeywordPositions(query, []string{"union all", "union"})
	if len(matches) == 0 {
		return []unionPart{{query: query, all: true}}
	}
	parts := []unionPart{}
	start := 0
	nextAll := true
	for _, match := range matches {
		parts = append(parts, unionPart{query: strings.TrimSpace(query[start:match.pos]), all: nextAll})
		nextAll = match.keyword == "union all"
		start = match.pos + len(match.keyword)
	}
	parts = append(parts, unionPart{query: strings.TrimSpace(query[start:]), all: nextAll})
	return parts
}

func executePart(query string, graph graphIndex, params map[string]any) ([]map[string]any, []string, error) {
	clauses, err := parseClauses(query)
	if err != nil {
		return nil, nil, err
	}
	rows := []row{{}}
	columns := []string{}
	for i := 0; i < len(clauses); i++ {
		clause := clauses[i]
		switch clause.kind {
		case "match", "optional match":
			optional := clause.kind == "optional match"
			patterns, err := parsePatterns(clause.body)
			if err != nil {
				return nil, nil, err
			}
			rows, err = executeMatch(rows, graph, params, patterns, optional)
			if err != nil {
				return nil, nil, err
			}
		case "where":
			filtered := rows[:0]
			for _, r := range rows {
				ok, err := evalPredicate(clause.body, r, params)
				if err != nil {
					return nil, nil, err
				}
				if ok {
					filtered = append(filtered, r)
				}
			}
			rows = filtered
		case "with", "return":
			projections, distinct, err := parseProjectionList(clause.body)
			if err != nil {
				return nil, nil, err
			}
			rows, columns, err = projectRows(rows, projections, distinct, params)
			if err != nil {
				return nil, nil, err
			}
		case "unwind":
			rows, err = executeUnwind(rows, clause.body, params)
			if err != nil {
				return nil, nil, err
			}
		case "order by":
			sortRows(rows, clause.body, params)
		case "skip":
			offset, err := positiveInt(clause.body, "SKIP")
			if err != nil {
				return nil, nil, err
			}
			if offset >= len(rows) {
				rows = nil
			} else {
				rows = rows[offset:]
			}
		case "limit":
			limit, err := positiveInt(clause.body, "LIMIT")
			if err != nil {
				return nil, nil, err
			}
			if len(rows) > limit {
				rows = rows[:limit]
			}
		default:
			return nil, nil, apperrors.WithDetails(apperrors.CodeQueryParseError, "unsupported cypher clause", map[string]string{"clause": clause.kind})
		}
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any(r))
	}
	if columns == nil {
		columns = []string{}
	}
	return out, columns, nil
}

func parseClauses(query string) ([]clause, error) {
	keywords := []string{"optional match", "order by", "match", "with", "where", "return", "unwind", "skip", "limit"}
	positions := topLevelKeywordPositions(query, keywords)
	if len(positions) == 0 {
		return nil, apperrors.New(apperrors.CodeQueryParseError, "cypher query requires clauses")
	}
	sort.SliceStable(positions, func(i, j int) bool { return positions[i].pos < positions[j].pos })
	positions = removeOverlappingKeywords(positions)
	clauses := make([]clause, 0, len(positions))
	for i, pos := range positions {
		end := len(query)
		if i+1 < len(positions) {
			end = positions[i+1].pos
		}
		body := strings.TrimSpace(query[pos.pos+len(pos.keyword) : end])
		if body == "" && pos.keyword != "match" {
			return nil, apperrors.WithDetails(apperrors.CodeQueryParseError, "cypher clause is empty", map[string]string{"clause": pos.keyword})
		}
		clauses = append(clauses, clause{kind: pos.keyword, body: body})
	}
	if clauses[len(clauses)-1].kind != "return" && !containsClause(clauses, "return") {
		return nil, apperrors.New(apperrors.CodeQueryParseError, "cypher query requires RETURN")
	}
	return clauses, nil
}

func removeOverlappingKeywords(positions []keywordPos) []keywordPos {
	filtered := make([]keywordPos, 0, len(positions))
	lastEnd := -1
	for _, pos := range positions {
		if pos.pos < lastEnd {
			continue
		}
		filtered = append(filtered, pos)
		lastEnd = pos.pos + len(pos.keyword)
	}
	return filtered
}

func containsClause(clauses []clause, kind string) bool {
	for _, clause := range clauses {
		if clause.kind == kind {
			return true
		}
	}
	return false
}

type keywordPos struct {
	keyword string
	pos     int
}

func topLevelKeywordPositions(text string, keywords []string) []keywordPos {
	lower := strings.ToLower(text)
	positions := []keywordPos{}
	depth := 0
	quote := rune(0)
	for i, r := range text {
		if quote != 0 {
			if r == quote {
				if quote == '`' && i+1 < len(text) && rune(text[i+1]) == '`' {
					continue
				}
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
		if depth != 0 || !keywordBoundary(lower, i-1) {
			continue
		}
		for _, keyword := range keywords {
			if strings.HasPrefix(lower[i:], keyword) && keywordBoundary(lower, i+len(keyword)) {
				positions = append(positions, keywordPos{keyword: keyword, pos: i})
				break
			}
		}
	}
	return positions
}

func keywordBoundary(text string, idx int) bool {
	if idx < 0 || idx >= len(text) {
		return true
	}
	r := rune(text[idx])
	return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
}

func parsePatterns(body string) ([]pathPattern, error) {
	parts := splitTopLevel(body, ',')
	patterns := make([]pathPattern, 0, len(parts))
	for _, part := range parts {
		pattern, err := parsePattern(part)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, pattern)
	}
	return patterns, nil
}

func parsePattern(raw string) (pathPattern, error) {
	text := strings.TrimSpace(raw)
	if idx := topLevelEquals(text); idx >= 0 {
		left := strings.TrimSpace(text[:idx])
		if isIdentifier(left) {
			text = strings.TrimSpace(text[idx+1:])
			pattern, err := parsePattern(text)
			if err != nil {
				return pathPattern{}, err
			}
			pattern.pathVar = left
			return pattern, nil
		}
	}
	pattern := pathPattern{}
	for {
		start := strings.Index(text, "(")
		if start < 0 {
			break
		}
		end := matchingIndex(text, start, '(', ')')
		if end < 0 {
			return pathPattern{}, apperrors.New(apperrors.CodeQueryParseError, "cypher node pattern is malformed")
		}
		node, err := parseNodePattern(text[start+1 : end])
		if err != nil {
			return pathPattern{}, err
		}
		pattern.nodes = append(pattern.nodes, node)
		text = text[end+1:]
		next := strings.Index(text, "(")
		if next < 0 {
			break
		}
		rel, err := parseRelPattern(text[:next])
		if err != nil {
			return pathPattern{}, err
		}
		pattern.rels = append(pattern.rels, rel)
		text = text[next:]
	}
	if len(pattern.nodes) == 0 {
		return pathPattern{}, apperrors.New(apperrors.CodeQueryParseError, "cypher MATCH requires a node pattern")
	}
	if len(pattern.rels) != len(pattern.nodes)-1 {
		return pathPattern{}, apperrors.New(apperrors.CodeQueryParseError, "cypher relationship pattern is malformed")
	}
	return pattern, nil
}

func parseNodePattern(raw string) (nodePattern, error) {
	text := strings.TrimSpace(raw)
	properties := ""
	if idx := strings.Index(text, "{"); idx >= 0 {
		end := matchingIndex(text, idx, '{', '}')
		if end < 0 {
			return nodePattern{}, apperrors.New(apperrors.CodeQueryParseError, "cypher node properties are malformed")
		}
		properties = text[idx+1 : end]
		text = strings.TrimSpace(text[:idx])
	}
	variable := ""
	labels := []string{}
	for strings.TrimSpace(text) != "" {
		text = strings.TrimSpace(text)
		if strings.HasPrefix(text, ":") {
			label, rest := readCypherName(strings.TrimSpace(text[1:]))
			labels = append(labels, label)
			text = rest
			continue
		}
		name, rest := readCypherName(text)
		if name == "" {
			return nodePattern{}, apperrors.WithDetails(apperrors.CodeQueryParseError, "cypher node pattern has unexpected token", map[string]string{"token": text})
		}
		if variable == "" {
			variable = name
		} else {
			labels = append(labels, name)
		}
		text = rest
	}
	return nodePattern{variable: variable, labels: labels, properties: parsePropertyMap(properties)}, nil
}

func parseRelPattern(raw string) (relPattern, error) {
	text := strings.TrimSpace(raw)
	dir := "both"
	if strings.HasSuffix(text, "->") {
		dir = "out"
	}
	if strings.HasPrefix(text, "<-") {
		dir = "in"
	}
	start := strings.Index(text, "[")
	end := strings.LastIndex(text, "]")
	rel := relPattern{min: 1, max: 1, dir: dir}
	if start < 0 || end < start {
		return rel, nil
	}
	inside := strings.TrimSpace(text[start+1 : end])
	for inside != "" {
		inside = strings.TrimSpace(inside)
		switch {
		case strings.HasPrefix(inside, ":"):
			types, rest := readRelTypes(inside[1:])
			rel.types = append(rel.types, types...)
			inside = rest
		case strings.HasPrefix(inside, "*"):
			min, max, rest, err := readDepth(inside[1:])
			if err != nil {
				return relPattern{}, err
			}
			rel.min, rel.max = min, max
			inside = rest
		default:
			name, rest := readCypherName(inside)
			if name == "" {
				return relPattern{}, apperrors.WithDetails(apperrors.CodeQueryParseError, "cypher relationship pattern has unexpected token", map[string]string{"token": inside})
			}
			rel.variable = name
			inside = rest
		}
	}
	return rel, nil
}

func readRelTypes(text string) ([]string, string) {
	types := []string{}
	for {
		name, rest := readCypherName(text)
		if name == "" {
			return types, rest
		}
		types = append(types, name)
		rest = strings.TrimSpace(rest)
		if !strings.HasPrefix(rest, "|") {
			return types, rest
		}
		text = strings.TrimSpace(rest[1:])
	}
}

// maxCypherPathDepth bounds variable-length relationship patterns (`*min..max`)
// so a single query cannot drive unbounded path expansion / recursion. An
// open-ended range (`*1..`) is clamped to this ceiling; an explicit bound that
// exceeds it (or overflows int) is rejected.
const maxCypherPathDepth = 16

func readDepth(text string) (int, int, string, error) {
	text = strings.TrimSpace(text)
	// readNumber returns the leading run of digits, whether any were present,
	// and whether they overflowed int (strconv.Atoi clamps to MaxInt on a range
	// error, so the discarded error previously hid an unbounded upper bound).
	readNumber := func(s string) (n int, rest string, present, overflow bool) {
		i := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == 0 {
			return 0, s, false, false
		}
		v, err := strconv.Atoi(s[:i])
		if err != nil {
			return 0, s[i:], true, true
		}
		return v, s[i:], true, false
	}
	depthExceeded := func() error {
		return apperrors.WithDetails(apperrors.CodeQueryParseError,
			"cypher relationship depth exceeds the maximum",
			map[string]string{"max_depth": strconv.Itoa(maxCypherPathDepth)})
	}

	min, rest, _, minOverflow := readNumber(text)
	if minOverflow {
		return 0, 0, "", depthExceeded()
	}
	max := min
	hasRange := false
	if strings.HasPrefix(rest, "..") {
		hasRange = true
		rest = rest[2:]
		if parsed, next, present, overflow := readNumber(rest); overflow {
			return 0, 0, "", depthExceeded()
		} else if present {
			// Explicit upper bound, including 0. A zero (`*1..0`) is kept as-is so
			// the max < min check below rejects it instead of silently widening it
			// to the ceiling.
			max = parsed
			rest = next
		} else {
			// Open-ended range (`*1..`): bound to the maximum.
			max = maxCypherPathDepth
		}
	}
	if min == 0 {
		min = 1
	}
	// Only default a missing upper bound from min. When a range was given, max is
	// already determined (an explicit value, or the open-ended ceiling), so an
	// explicit `..0` must not be normalized away here.
	if max == 0 && !hasRange {
		max = min
	}
	if max < min {
		return 0, 0, "", apperrors.New(apperrors.CodeQueryParseError, "cypher relationship depth is invalid")
	}
	if min > maxCypherPathDepth || max > maxCypherPathDepth {
		return 0, 0, "", depthExceeded()
	}
	return min, max, rest, nil
}

func parsePropertyMap(raw string) map[string]string {
	props := map[string]string{}
	for _, item := range splitTopLevel(raw, ',') {
		key, value, ok := strings.Cut(item, ":")
		if !ok {
			continue
		}
		props[cleanName(key)] = strings.TrimSpace(value)
	}
	return props
}

func executeMatch(rows []row, graph graphIndex, params map[string]any, patterns []pathPattern, optional bool) ([]row, error) {
	current := rows
	var err error
	for _, pattern := range patterns {
		current, err = executePattern(current, graph, params, pattern, optional)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}

func executePattern(rows []row, graph graphIndex, params map[string]any, pattern pathPattern, optional bool) ([]row, error) {
	out := []row{}
	for _, base := range rows {
		matches, err := expandPattern(base, graph, params, pattern)
		if err != nil {
			return nil, err
		}
		if len(matches) == 0 && optional {
			next := cloneRow(base)
			for _, node := range pattern.nodes {
				if node.variable != "" {
					if _, ok := next[node.variable]; !ok {
						next[node.variable] = nil
					}
				}
			}
			for _, rel := range pattern.rels {
				if rel.variable != "" {
					if _, ok := next[rel.variable]; !ok {
						next[rel.variable] = nil
					}
				}
			}
			if pattern.pathVar != "" {
				next[pattern.pathVar] = nil
			}
			out = append(out, next)
			continue
		}
		out = append(out, matches...)
	}
	return out, nil
}

func expandPattern(base row, graph graphIndex, params map[string]any, pattern pathPattern) ([]row, error) {
	if len(pattern.rels) == 0 {
		candidates := candidateNodes(base, graph, pattern.nodes[0])
		out := []row{}
		for _, node := range candidates {
			ok, err := nodeMatches(node, pattern.nodes[0], base, params)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			next := cloneRow(base)
			if pattern.nodes[0].variable != "" {
				next[pattern.nodes[0].variable] = node
			}
			out = append(out, next)
		}
		return out, nil
	}
	startCandidates := candidateNodes(base, graph, pattern.nodes[0])
	out := []row{}
	for _, start := range startCandidates {
		ok, err := nodeMatches(start, pattern.nodes[0], base, params)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		next := cloneRow(base)
		if pattern.nodes[0].variable != "" {
			next[pattern.nodes[0].variable] = start
		}
		expanded, err := expandRelStep(next, graph, params, pattern, 0, start, []Node{start}, nil, map[string]struct{}{})
		if err != nil {
			return nil, err
		}
		out = append(out, expanded...)
	}
	return out, nil
}

func expandRelStep(base row, graph graphIndex, params map[string]any, pattern pathPattern, index int, current Node, nodes []Node, rels []Edge, used map[string]struct{}) ([]row, error) {
	if index == len(pattern.rels) {
		next := cloneRow(base)
		if pattern.pathVar != "" {
			next[pattern.pathVar] = Path{Nodes: append([]Node(nil), nodes...), Rels: append([]Edge(nil), rels...)}
		}
		return []row{next}, nil
	}
	relPattern := pattern.rels[index]
	targetPattern := pattern.nodes[index+1]
	out := []row{}
	paths := findRelPaths(graph, current, relPattern, used)
	for _, p := range paths {
		target := p.nodes[len(p.nodes)-1]
		ok, err := nodeMatches(target, targetPattern, base, params)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		next := cloneRow(base)
		if targetPattern.variable != "" {
			next[targetPattern.variable] = target
		}
		if relPattern.variable != "" {
			if len(p.rels) == 1 && relPattern.min == 1 && relPattern.max == 1 {
				next[relPattern.variable] = p.rels[0]
			} else {
				next[relPattern.variable] = append([]Edge(nil), p.rels...)
			}
		}
		nextNodes := append(append([]Node(nil), nodes...), p.nodes[1:]...)
		nextRels := append(append([]Edge(nil), rels...), p.rels...)
		nextUsed := cloneStringSet(used)
		for _, rel := range p.rels {
			nextUsed[rel.ID] = struct{}{}
		}
		more, err := expandRelStep(next, graph, params, pattern, index+1, target, nextNodes, nextRels, nextUsed)
		if err != nil {
			return nil, err
		}
		out = append(out, more...)
	}
	return out, nil
}

type partialPath struct {
	nodes []Node
	rels  []Edge
}

func findRelPaths(graph graphIndex, start Node, pattern relPattern, used map[string]struct{}) []partialPath {
	paths := []partialPath{}
	var walk func(node Node, depth int, nodes []Node, rels []Edge, usedEdges map[string]struct{})
	walk = func(node Node, depth int, nodes []Node, rels []Edge, usedEdges map[string]struct{}) {
		if depth >= pattern.min {
			paths = append(paths, partialPath{nodes: append([]Node(nil), nodes...), rels: append([]Edge(nil), rels...)})
		}
		if depth == pattern.max {
			return
		}
		for _, edge := range graph.adjacency[node.ID] {
			if _, ok := usedEdges[edge.ID]; ok {
				continue
			}
			if len(pattern.types) > 0 && !containsString(pattern.types, edge.Type) {
				continue
			}
			nextID, ok := nextNodeID(node.ID, edge, pattern.dir)
			if !ok {
				continue
			}
			next, ok := graph.nodes[nextID]
			if !ok {
				continue
			}
			usedEdges[edge.ID] = struct{}{}
			walk(next, depth+1, append(nodes, next), append(rels, edge), usedEdges)
			delete(usedEdges, edge.ID)
		}
	}
	walk(start, 0, []Node{start}, nil, cloneStringSet(used))
	sort.SliceStable(paths, func(i, j int) bool {
		if len(paths[i].rels) != len(paths[j].rels) {
			return len(paths[i].rels) < len(paths[j].rels)
		}
		return paths[i].nodes[len(paths[i].nodes)-1].ID < paths[j].nodes[len(paths[j].nodes)-1].ID
	})
	return paths
}

func nextNodeID(current string, edge Edge, direction string) (string, bool) {
	switch direction {
	case "out":
		if edge.From == current {
			return edge.To, true
		}
	case "in":
		if edge.To == current {
			return edge.From, true
		}
	default:
		if edge.From == current {
			return edge.To, true
		}
		if edge.To == current {
			return edge.From, true
		}
	}
	return "", false
}

func candidateNodes(base row, graph graphIndex, pattern nodePattern) []Node {
	if pattern.variable != "" {
		if bound, ok := base[pattern.variable]; ok {
			if node, ok := bound.(Node); ok {
				return []Node{node}
			}
			return nil
		}
	}
	keys := make([]string, 0, len(graph.nodes))
	for key := range graph.nodes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	nodes := make([]Node, 0, len(keys))
	for _, key := range keys {
		nodes = append(nodes, graph.nodes[key])
	}
	return nodes
}

func nodeMatches(node Node, pattern nodePattern, base row, params map[string]any) (bool, error) {
	if pattern.variable != "" {
		if bound, ok := base[pattern.variable]; ok {
			boundNode, ok := bound.(Node)
			if !ok || boundNode.ID != node.ID {
				return false, nil
			}
		}
	}
	for _, label := range pattern.labels {
		if !nodeHasLabel(node, label) {
			return false, nil
		}
	}
	for key, raw := range pattern.properties {
		expected, err := evalExpr(raw, base, params)
		if err != nil {
			return false, err
		}
		if key == "__entity_id__" {
			if value := stringValue(expected); value != "" && !model.IsEntityID(value) {
				return false, apperrors.New(apperrors.CodeQueryPlanError, "cypher __entity_id__ must be a 128-bit lowercase hex string")
			}
		}
		if !valuesEqual(node.Properties[key], expected) {
			return false, nil
		}
	}
	return true, nil
}

func nodeHasLabel(node Node, label string) bool {
	for _, candidate := range node.Labels {
		if candidate == label {
			return true
		}
	}
	domain := stringValue(node.Properties["__domain__"])
	entityType := stringValue(node.Properties["__entity_type__"])
	return label == entityType || label == domain+"@"+entityType
}

func executeUnwind(rows []row, body string, params map[string]any) ([]row, error) {
	expr, alias, ok := splitAlias(body)
	if !ok {
		return nil, apperrors.New(apperrors.CodeQueryParseError, "UNWIND requires AS")
	}
	out := []row{}
	for _, r := range rows {
		value, err := evalExpr(expr, r, params)
		if err != nil {
			return nil, err
		}
		values := toSlice(value)
		for _, item := range values {
			next := cloneRow(r)
			next[alias] = item
			out = append(out, next)
		}
	}
	return out, nil
}

func parseProjectionList(body string) ([]projection, bool, error) {
	body = strings.TrimSpace(body)
	distinct := false
	if strings.HasPrefix(strings.ToLower(body), "distinct ") {
		distinct = true
		body = strings.TrimSpace(body[len("distinct "):])
	}
	if body == "*" {
		return []projection{{expr: "*", alias: "*"}}, distinct, nil
	}
	parts := splitTopLevel(body, ',')
	projections := make([]projection, 0, len(parts))
	for _, part := range parts {
		expr, alias, ok := splitAlias(part)
		if !ok {
			expr = strings.TrimSpace(part)
			alias = defaultAlias(expr)
		}
		if expr == "" {
			continue
		}
		proj := projection{expr: expr, alias: alias}
		if agg := aggregateName(expr); agg != "" {
			proj.aggregate = agg
		}
		projections = append(projections, proj)
	}
	if len(projections) == 0 {
		return nil, false, apperrors.New(apperrors.CodeQueryParseError, "projection requires expressions")
	}
	return projections, distinct, nil
}

func projectRows(rows []row, projections []projection, distinct bool, params map[string]any) ([]row, []string, error) {
	if len(projections) == 1 && projections[0].expr == "*" {
		out := make([]row, 0, len(rows))
		columns := []string{}
		seen := map[string]struct{}{}
		for _, r := range rows {
			next := cloneRow(r)
			out = append(out, next)
			for key := range next {
				if _, ok := seen[key]; !ok {
					seen[key] = struct{}{}
					columns = append(columns, key)
				}
			}
		}
		sort.Strings(columns)
		return out, columns, nil
	}
	hasAggregate := false
	for _, proj := range projections {
		if proj.aggregate != "" {
			hasAggregate = true
			break
		}
	}
	columns := make([]string, 0, len(projections))
	for _, proj := range projections {
		columns = append(columns, proj.alias)
	}
	var out []row
	var err error
	if hasAggregate {
		out, err = projectAggregateRows(rows, projections, params)
	} else {
		out, err = projectPlainRows(rows, projections, params)
	}
	if err != nil {
		return nil, nil, err
	}
	if distinct {
		out = rowsToInternal(distinctRows(internalToMaps(out), columns))
	}
	return out, columns, nil
}

func projectPlainRows(rows []row, projections []projection, params map[string]any) ([]row, error) {
	out := make([]row, 0, len(rows))
	for _, r := range rows {
		next := row{}
		for _, proj := range projections {
			value, err := evalExpr(proj.expr, r, params)
			if err != nil {
				return nil, err
			}
			next[proj.alias] = value
		}
		out = append(out, next)
	}
	return out, nil
}

func projectAggregateRows(rows []row, projections []projection, params map[string]any) ([]row, error) {
	groups := map[string][]row{}
	keys := []string{}
	for _, r := range rows {
		values := []any{}
		for _, proj := range projections {
			if proj.aggregate == "" {
				value, err := evalExpr(proj.expr, r, params)
				if err != nil {
					return nil, err
				}
				values = append(values, value)
			}
		}
		key := rowKey(values)
		if _, ok := groups[key]; !ok {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], r)
	}
	sort.Strings(keys)
	out := []row{}
	for _, key := range keys {
		group := groups[key]
		next := row{}
		for _, proj := range projections {
			if proj.aggregate == "" {
				value, err := evalExpr(proj.expr, group[0], params)
				if err != nil {
					return nil, err
				}
				next[proj.alias] = value
				continue
			}
			value, err := aggregateValue(proj.aggregate, proj.expr, group, params)
			if err != nil {
				return nil, err
			}
			next[proj.alias] = value
		}
		out = append(out, next)
	}
	return out, nil
}

func aggregateValue(name, expr string, rows []row, params map[string]any) (any, error) {
	inner := functionInner(expr)
	switch name {
	case "count":
		if strings.TrimSpace(inner) == "*" {
			return len(rows), nil
		}
		count := 0
		for _, r := range rows {
			value, err := evalExpr(inner, r, params)
			if err != nil {
				return nil, err
			}
			if value != nil {
				count++
			}
		}
		return count, nil
	case "collect":
		values := make([]any, 0, len(rows))
		for _, r := range rows {
			value, err := evalExpr(inner, r, params)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		return values, nil
	case "sum", "avg", "min", "max":
		values := []float64{}
		for _, r := range rows {
			value, err := evalExpr(inner, r, params)
			if err != nil {
				return nil, err
			}
			if f, ok := floatValue(value); ok {
				values = append(values, f)
			}
		}
		if len(values) == 0 {
			return nil, nil
		}
		switch name {
		case "sum":
			sum := 0.0
			for _, value := range values {
				sum += value
			}
			return sum, nil
		case "avg":
			sum := 0.0
			for _, value := range values {
				sum += value
			}
			return sum / float64(len(values)), nil
		case "min":
			min := values[0]
			for _, value := range values[1:] {
				min = math.Min(min, value)
			}
			return min, nil
		case "max":
			max := values[0]
			for _, value := range values[1:] {
				max = math.Max(max, value)
			}
			return max, nil
		}
	}
	return nil, apperrors.WithDetails(apperrors.CodeQueryPlanError, "unsupported aggregate", map[string]string{"function": name})
}

func sortRows(rows []row, body string, params map[string]any) {
	items := splitTopLevel(body, ',')
	sort.SliceStable(rows, func(i, j int) bool {
		for _, item := range items {
			fields := strings.Fields(strings.TrimSpace(item))
			if len(fields) == 0 {
				continue
			}
			expr := fields[0]
			desc := len(fields) > 1 && strings.EqualFold(fields[1], "desc")
			left, _ := evalExpr(expr, rows[i], params)
			right, _ := evalExpr(expr, rows[j], params)
			cmp := compareValue(left, right)
			if cmp == 0 {
				continue
			}
			if desc {
				return cmp > 0
			}
			return cmp < 0
		}
		return false
	})
}

func evalPredicate(expr string, r row, params map[string]any) (bool, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return true, nil
	}
	if wrapped(expr) {
		return evalPredicate(expr[1:len(expr)-1], r, params)
	}
	for _, part := range splitTopLevelKeyword(expr, "or") {
		if part == expr {
			break
		}
		ok, err := evalPredicate(part, r, params)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	orParts := splitTopLevelKeyword(expr, "or")
	if len(orParts) > 1 {
		return false, nil
	}
	andParts := splitTopLevelKeyword(expr, "and")
	if len(andParts) > 1 {
		for _, part := range andParts {
			ok, err := evalPredicate(part, r, params)
			if err != nil || !ok {
				return ok, err
			}
		}
		return true, nil
	}
	lower := strings.ToLower(expr)
	if strings.HasPrefix(lower, "not ") {
		ok, err := evalPredicate(expr[4:], r, params)
		return !ok, err
	}
	if strings.HasSuffix(lower, " is null") {
		value, err := evalExpr(strings.TrimSpace(expr[:len(expr)-len(" is null")]), r, params)
		return value == nil, err
	}
	if strings.HasSuffix(lower, " is not null") {
		value, err := evalExpr(strings.TrimSpace(expr[:len(expr)-len(" is not null")]), r, params)
		return value != nil, err
	}
	if idx := topLevelKeywordIndex(expr, "in"); idx >= 0 {
		left, err := evalExpr(expr[:idx], r, params)
		if err != nil {
			return false, err
		}
		right, err := evalExpr(expr[idx+len("in"):], r, params)
		if err != nil {
			return false, err
		}
		for _, item := range toSlice(right) {
			if valuesEqual(left, item) {
				return true, nil
			}
		}
		return false, nil
	}
	for _, op := range []string{"<=", ">=", "<>", "!=", "=", "<", ">"} {
		if idx := topLevelOperatorIndex(expr, op); idx >= 0 {
			left, err := evalExpr(expr[:idx], r, params)
			if err != nil {
				return false, err
			}
			right, err := evalExpr(expr[idx+len(op):], r, params)
			if err != nil {
				return false, err
			}
			switch op {
			case "=":
				return valuesEqual(left, right), nil
			case "<>", "!=":
				return !valuesEqual(left, right), nil
			case "<":
				return compareValue(left, right) < 0, nil
			case "<=":
				return compareValue(left, right) <= 0, nil
			case ">":
				return compareValue(left, right) > 0, nil
			case ">=":
				return compareValue(left, right) >= 0, nil
			}
		}
	}
	value, err := evalExpr(expr, r, params)
	if err != nil {
		return false, err
	}
	return truthy(value), nil
}

func evalExpr(expr string, r row, params map[string]any) (any, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, nil
	}
	if wrapped(expr) {
		return evalExpr(expr[1:len(expr)-1], r, params)
	}
	lower := strings.ToLower(expr)
	switch lower {
	case "null":
		return nil, nil
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	if strings.HasPrefix(expr, "$") {
		name := strings.TrimPrefix(expr, "$")
		value, ok := params[name]
		if !ok {
			return nil, apperrors.WithDetails(apperrors.CodeQueryPlanError, "cypher parameter is missing", map[string]string{"parameter": name})
		}
		return value, nil
	}
	if isQuoted(expr, '\'') || isQuoted(expr, '"') || isQuoted(expr, '`') {
		return unquote(expr), nil
	}
	if strings.HasPrefix(expr, "[") && strings.HasSuffix(expr, "]") {
		inner := strings.TrimSpace(expr[1 : len(expr)-1])
		if variable, source, body, ok := parseListComprehension(inner); ok {
			value, err := evalExpr(source, r, params)
			if err != nil {
				return nil, err
			}
			out := []any{}
			for _, item := range toSlice(value) {
				next := cloneRow(r)
				next[variable] = item
				projected, err := evalExpr(body, next, params)
				if err != nil {
					return nil, err
				}
				out = append(out, projected)
			}
			if stringsOnly(out) {
				values := make([]string, 0, len(out))
				for _, item := range out {
					values = append(values, item.(string))
				}
				return values, nil
			}
			return out, nil
		}
		items := splitTopLevel(inner, ',')
		out := make([]any, 0, len(items))
		for _, item := range items {
			value, err := evalExpr(item, r, params)
			if err != nil {
				return nil, err
			}
			out = append(out, value)
		}
		return out, nil
	}
	if i, err := strconv.ParseInt(expr, 10, 64); err == nil {
		return i, nil
	}
	if f, err := strconv.ParseFloat(expr, 64); err == nil && strings.Contains(expr, ".") {
		return f, nil
	}
	if fn, inner, ok := functionCall(expr); ok {
		return evalFunction(fn, inner, r, params)
	}
	if value, ok := r[expr]; ok {
		return unwrapValue(value), nil
	}
	if variable, property, ok := strings.Cut(expr, "."); ok {
		value, ok := r[strings.TrimSpace(variable)]
		if !ok || value == nil {
			return nil, nil
		}
		return propertyValue(value, cleanName(property)), nil
	}
	return expr, nil
}

func evalFunction(name, inner string, r row, params map[string]any) (any, error) {
	name = strings.ToLower(name)
	args := splitTopLevel(inner, ',')
	switch name {
	case "type":
		if len(args) != 1 {
			return nil, apperrors.New(apperrors.CodeQueryParseError, "type() requires one argument")
		}
		value, ok := r[strings.TrimSpace(args[0])]
		if !ok {
			var err error
			value, err = evalExpr(args[0], r, params)
			if err != nil {
				return nil, err
			}
		}
		if edge, ok := value.(Edge); ok {
			return edge.Type, nil
		}
		return nil, nil
	case "relationships":
		if len(args) != 1 {
			return nil, apperrors.New(apperrors.CodeQueryParseError, "relationships() requires one argument")
		}
		value, err := evalExpr(args[0], r, params)
		if err != nil {
			return nil, err
		}
		if path, ok := value.(Path); ok {
			return path.Rels, nil
		}
		return []Edge{}, nil
	case "nodes":
		value, err := evalExpr(args[0], r, params)
		if err != nil {
			return nil, err
		}
		if path, ok := value.(Path); ok {
			return path.Nodes, nil
		}
		return []Node{}, nil
	case "size":
		value, err := evalExpr(args[0], r, params)
		if err != nil {
			return nil, err
		}
		return len(toSlice(value)), nil
	case "coalesce":
		for _, arg := range args {
			value, err := evalExpr(arg, r, params)
			if err != nil {
				return nil, err
			}
			if value != nil {
				return value, nil
			}
		}
		return nil, nil
	case "labels":
		value, err := evalFunctionTarget(args[0], r, params)
		if err != nil {
			return nil, err
		}
		if node, ok := value.(Node); ok {
			return append([]string(nil), node.Labels...), nil
		}
		return []string{}, nil
	case "properties":
		value, err := evalFunctionTarget(args[0], r, params)
		if err != nil {
			return nil, err
		}
		switch typed := value.(type) {
		case Node:
			return cloneMap(typed.Properties), nil
		case Edge:
			return cloneMap(typed.Properties), nil
		}
		return nil, nil
	case "id":
		value, err := evalFunctionTarget(args[0], r, params)
		if err != nil {
			return nil, err
		}
		switch typed := value.(type) {
		case Node:
			return typed.ID, nil
		case Edge:
			return typed.ID, nil
		}
		return nil, nil
	case "tostring":
		value, err := evalExpr(args[0], r, params)
		return stringValue(value), err
	case "tointeger":
		value, err := evalExpr(args[0], r, params)
		if err != nil {
			return nil, err
		}
		if f, ok := floatValue(value); ok {
			return int64(f), nil
		}
		return nil, nil
	default:
		return nil, apperrors.WithDetails(apperrors.CodeQueryPlanError, "unsupported cypher function", map[string]string{"function": name})
	}
}

func evalFunctionTarget(expr string, r row, params map[string]any) (any, error) {
	expr = strings.TrimSpace(expr)
	if value, ok := r[expr]; ok {
		return value, nil
	}
	return evalExpr(expr, r, params)
}

func propertyValue(value any, property string) any {
	switch typed := value.(type) {
	case Node:
		return typed.Properties[property]
	case Edge:
		return typed.Properties[property]
	case map[string]any:
		return typed[property]
	default:
		return nil
	}
}

func parseListComprehension(inner string) (string, string, string, bool) {
	pipe := topLevelRune(inner, '|')
	if pipe < 0 {
		return "", "", "", false
	}
	left := strings.TrimSpace(inner[:pipe])
	body := strings.TrimSpace(inner[pipe+1:])
	idx := topLevelKeywordIndex(left, "in")
	if idx < 0 {
		return "", "", "", false
	}
	variable := strings.TrimSpace(left[:idx])
	source := strings.TrimSpace(left[idx+len("in"):])
	return variable, source, body, variable != "" && source != "" && body != ""
}

func splitAlias(text string) (string, string, bool) {
	idx := lastTopLevelKeywordIndex(text, "as")
	if idx < 0 {
		return "", "", false
	}
	expr := strings.TrimSpace(text[:idx])
	alias := strings.TrimSpace(text[idx+len("as"):])
	return expr, cleanName(alias), expr != "" && alias != ""
}

func defaultAlias(expr string) string {
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(strings.ToLower(expr), "count(") {
		return "count"
	}
	if variable, property, ok := strings.Cut(expr, "."); ok && variable != "" {
		return cleanName(property)
	}
	return cleanName(expr)
}

func aggregateName(expr string) string {
	fn, _, ok := functionCall(expr)
	if !ok {
		return ""
	}
	switch strings.ToLower(fn) {
	case "count", "collect", "sum", "avg", "min", "max":
		return strings.ToLower(fn)
	default:
		return ""
	}
}

func functionInner(expr string) string {
	_, inner, _ := functionCall(expr)
	return inner
}

func functionCall(expr string) (string, string, bool) {
	open := strings.Index(expr, "(")
	if open <= 0 || !strings.HasSuffix(strings.TrimSpace(expr), ")") {
		return "", "", false
	}
	name := strings.TrimSpace(expr[:open])
	if !isIdentifier(name) {
		return "", "", false
	}
	close := matchingIndex(expr, open, '(', ')')
	if close != len(expr)-1 {
		return "", "", false
	}
	return name, expr[open+1 : close], true
}

func positiveInt(body, clause string) (int, error) {
	value := strings.TrimSpace(strings.TrimSuffix(body, ";"))
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0, apperrors.WithDetails(apperrors.CodeQueryParseError, clause+" requires a non-negative integer", map[string]string{"value": value})
	}
	return n, nil
}

func readCypherName(text string) (string, string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", ""
	}
	if strings.HasPrefix(text, "`") {
		var b strings.Builder
		for i := 1; i < len(text); i++ {
			if text[i] == '`' {
				if i+1 < len(text) && text[i+1] == '`' {
					b.WriteByte('`')
					i++
					continue
				}
				return b.String(), text[i+1:]
			}
			b.WriteByte(text[i])
		}
		return "", text
	}
	i := 0
	for i < len(text) {
		r := rune(text[i])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' || r == '@' || r == '-' {
			i++
			continue
		}
		break
	}
	return text[:i], text[i:]
}

func topLevelEquals(text string) int {
	return topLevelOperatorIndex(text, "=")
}

func topLevelOperatorIndex(text, op string) int {
	depth := 0
	quote := rune(0)
	for i, r := range text {
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		switch r {
		case '\'', '"', '`':
			quote = r
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && strings.HasPrefix(text[i:], op) {
				return i
			}
		}
	}
	return -1
}

func topLevelKeywordIndex(text, keyword string) int {
	positions := topLevelKeywordPositions(text, []string{keyword})
	if len(positions) == 0 {
		return -1
	}
	return positions[0].pos
}

func lastTopLevelKeywordIndex(text, keyword string) int {
	positions := topLevelKeywordPositions(text, []string{keyword})
	if len(positions) == 0 {
		return -1
	}
	return positions[len(positions)-1].pos
}

func topLevelRune(text string, needle rune) int {
	depth := 0
	quote := rune(0)
	for i, r := range text {
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		switch r {
		case '\'', '"', '`':
			quote = r
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && r == needle {
				return i
			}
		}
	}
	return -1
}

func matchingIndex(text string, start int, open, close rune) int {
	depth := 0
	quote := rune(0)
	for i, r := range text[start:] {
		pos := start + i
		if quote != 0 {
			if r == quote {
				if quote == '`' && pos+1 < len(text) && rune(text[pos+1]) == '`' {
					continue
				}
				quote = 0
			}
			continue
		}
		switch r {
		case '\'', '"', '`':
			quote = r
		default:
			if r == open {
				depth++
			} else if r == close {
				depth--
				if depth == 0 {
					return pos
				}
			}
		}
	}
	return -1
}

func splitTopLevel(text string, sep rune) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	parts := []string{}
	depth := 0
	quote := rune(0)
	start := 0
	for i, r := range text {
		if quote != 0 {
			if r == quote {
				if quote == '`' && i+1 < len(text) && rune(text[i+1]) == '`' {
					continue
				}
				quote = 0
			}
			continue
		}
		switch r {
		case '\'', '"', '`':
			quote = r
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && r == sep {
				parts = append(parts, strings.TrimSpace(text[start:i]))
				start = i + len(string(r))
			}
		}
	}
	parts = append(parts, strings.TrimSpace(text[start:]))
	return parts
}

func splitTopLevelKeyword(text, keyword string) []string {
	positions := topLevelKeywordPositions(text, []string{keyword})
	if len(positions) == 0 {
		return []string{text}
	}
	parts := []string{}
	start := 0
	for _, pos := range positions {
		parts = append(parts, strings.TrimSpace(text[start:pos.pos]))
		start = pos.pos + len(keyword)
	}
	parts = append(parts, strings.TrimSpace(text[start:]))
	return parts
}

func wrapped(expr string) bool {
	expr = strings.TrimSpace(expr)
	return strings.HasPrefix(expr, "(") && matchingIndex(expr, 0, '(', ')') == len(expr)-1
}

func isQuoted(text string, quote byte) bool {
	return len(text) >= 2 && text[0] == quote && text[len(text)-1] == quote
}

func unquote(text string) string {
	text = strings.TrimSpace(text)
	if len(text) < 2 {
		return text
	}
	if isQuoted(text, '\'') || isQuoted(text, '"') {
		return text[1 : len(text)-1]
	}
	if isQuoted(text, '`') {
		return strings.ReplaceAll(text[1:len(text)-1], "``", "`")
	}
	return text
}

func cleanName(text string) string {
	return unquote(strings.TrimSpace(text))
}

func firstKeyword(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func readOnlyTokens(text string) []string {
	tokens := []string{}
	var b strings.Builder
	quote := rune(0)
	flush := func() {
		if b.Len() > 0 {
			tokens = append(tokens, strings.ToLower(b.String()))
			b.Reset()
		}
	}
	for _, r := range text {
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' || r == '`' {
			flush()
			quote = r
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			b.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return tokens
}

func isIdentifier(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	for i, r := range text {
		if i == 0 && !(unicode.IsLetter(r) || r == '_') {
			return false
		}
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			return false
		}
	}
	return true
}

func toSlice(value any) []any {
	switch typed := value.(type) {
	case nil:
		return nil
	case []any:
		return typed
	case []string:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out
	case []Edge:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out
	case []Node:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out
	default:
		rv := reflect.ValueOf(value)
		if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
			out := make([]any, 0, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				out = append(out, rv.Index(i).Interface())
			}
			return out
		}
		return []any{value}
	}
}

func stringsOnly(values []any) bool {
	for _, value := range values {
		if _, ok := value.(string); !ok {
			return false
		}
	}
	return true
}

func unwrapValue(value any) any {
	switch typed := value.(type) {
	case Node:
		return cloneMap(typed.Properties)
	case Edge:
		return cloneMap(typed.Properties)
	case Path:
		return typed
	default:
		return value
	}
}

func truthy(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case bool:
		return typed
	case string:
		return typed != ""
	default:
		return true
	}
}

func valuesEqual(left, right any) bool {
	if lf, ok := floatValue(left); ok {
		if rf, ok := floatValue(right); ok {
			return lf == rf
		}
	}
	return fmt.Sprint(left) == fmt.Sprint(right)
}

func compareValue(left, right any) int {
	if left == nil && right == nil {
		return 0
	}
	if left == nil {
		return -1
	}
	if right == nil {
		return 1
	}
	if lf, ok := floatValue(left); ok {
		if rf, ok := floatValue(right); ok {
			switch {
			case lf < rf:
				return -1
			case lf > rf:
				return 1
			default:
				return 0
			}
		}
	}
	ls := fmt.Sprint(left)
	rs := fmt.Sprint(right)
	switch {
	case ls < rs:
		return -1
	case ls > rs:
		return 1
	default:
		return 0
	}
}

func floatValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	case string:
		f, err := strconv.ParseFloat(typed, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func cloneMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func cloneRow(input row) row {
	out := make(row, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func cloneStringSet(input map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(input))
	for key := range input {
		out[key] = struct{}{}
	}
	return out
}

func rowKey(values []any) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, fmt.Sprintf("%#v", value))
	}
	return strings.Join(parts, "\x00")
}

func distinctRows(rows []map[string]any, columns []string) []map[string]any {
	seen := map[string]struct{}{}
	out := []map[string]any{}
	for _, row := range rows {
		values := []any{}
		if len(columns) == 0 {
			keys := make([]string, 0, len(row))
			for key := range row {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				values = append(values, row[key])
			}
		} else {
			for _, key := range columns {
				values = append(values, row[key])
			}
		}
		key := rowKey(values)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, row)
	}
	return out
}

func rowsToInternal(rows []map[string]any) []row {
	out := make([]row, 0, len(rows))
	for _, r := range rows {
		out = append(out, row(r))
	}
	return out
}

func internalToMaps(rows []row) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any(r))
	}
	return out
}
