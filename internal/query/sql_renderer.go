package query

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/alibaba/UnifiedModel/internal/query/planrender"
	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
)

// sqlTableRenderer renders the sql-table query family: metric backends queried
// with SQL over a table (ClickHouse and other SQL-compatible stores). It is the
// first renderer for a query model genuinely different from PromQL, and it
// consumes the shared Constraint IR (ResolveConstraints) rather than
// re-implementing entity binding and filter parsing — the WHERE clause is just
// the IR formatted as SQL.
//
// The service returns a plan, not a result, so the SQL carries {from} / {to}
// placeholders the caller fills from the plan envelope's time_range (mirroring
// how the PromQL plan pairs a query string with a separate time_range/step).
type sqlTableRenderer struct{}

func (sqlTableRenderer) Family() string { return "sql-table" }

func (sqlTableRenderer) SupportsMethod(m planrender.Method) bool {
	return m == planrender.MethodGetMetrics
}

func (sqlTableRenderer) Render(req planrender.Request) (map[string]any, error) {
	constraints, raw := ResolveConstraints(req)
	// Fail closed: the sql-table renderer can only express the constraint shapes
	// the IR understands. If ResolveConstraints could not parse a filter, it is
	// returned in raw; emitting SQL anyway would silently drop that predicate and
	// run a broader query than asked, so refuse to render instead of producing an
	// under-constrained plan. (The executor propagates this error rather than
	// falling back to an unrendered passthrough.)
	if len(raw) > 0 {
		return nil, apperrors.WithDetails(apperrors.CodeQueryPlanError,
			"sql-table renderer cannot express some filters; refusing to emit an under-constrained query",
			map[string]string{"unsupported_filters": strings.Join(raw, " ; ")})
	}

	database := firstNonEmpty(stringValue(req.Storage.Spec["database"]), stringValue(req.DataSet.Spec["database"]))
	table := firstNonEmpty(stringValue(req.Storage.Spec["table"]), stringValue(req.DataSet.Spec["table"]), req.DataSet.Name)
	timeCol := firstNonEmpty(stringValue(req.Storage.Spec["time_column"]), "timestamp")
	valueCol := firstNonEmpty(stringValue(req.Storage.Spec["value_column"]), "value")
	metricCol := firstNonEmpty(stringValue(req.Storage.Spec["metric_column"]), "metric_name")

	// Identifiers come from storage/dataset config; values come from the query.
	// Validate and backtick-quote every identifier so a malformed or hostile
	// mapping cannot inject SQL through an identifier position.
	qTable, err := quoteSQLIdent(table)
	if err != nil {
		return nil, sqlIdentError("table", table, err)
	}
	qTimeCol, err := quoteSQLIdent(timeCol)
	if err != nil {
		return nil, sqlIdentError("time_column", timeCol, err)
	}
	qValueCol, err := quoteSQLIdent(valueCol)
	if err != nil {
		return nil, sqlIdentError("value_column", valueCol, err)
	}
	qMetricCol, err := quoteSQLIdent(metricCol)
	if err != nil {
		return nil, sqlIdentError("metric_column", metricCol, err)
	}

	// Qualify the table with its database when present, so the plan targets
	// `db`.`table` rather than whichever default database the executing connection
	// happens to use.
	qFrom := qTable
	if database != "" {
		qDatabase, err := quoteSQLIdent(database)
		if err != nil {
			return nil, sqlIdentError("database", database, err)
		}
		qFrom = qDatabase + "." + qTable
	}

	where, err := constraintsToSQLWhere(constraints)
	if err != nil {
		return nil, err
	}

	queryType := firstNonEmpty(req.QueryType, defaultMetricQueryMode(req.Metrics), stringValue(req.Storage.Spec["default_query_type"]), "range")
	// Normalize/validate: only instant and range produce well-defined SQL. Anything
	// else (a bad query param, mapping, or metric query_mode) must fail closed rather
	// than silently fall into the range branch while the plan echoes the bogus value.
	if queryType != "instant" && queryType != "range" {
		return nil, apperrors.WithDetails(apperrors.CodeQueryParseError,
			"sql-table renderer rejected the query_type",
			map[string]string{"query_type": queryType, "reason": "must be 'instant' or 'range'"})
	}
	step := firstNonEmpty(req.Step, stringValue(req.Storage.Spec["default_step"]), "1m")
	interval, err := sqlInterval(step)
	if err != nil {
		return nil, apperrors.WithDetails(apperrors.CodeQueryParseError,
			"sql-table renderer rejected the step", map[string]string{"step": step, "reason": err.Error()})
	}

	queries := []map[string]any{}
	for _, metric := range req.Metrics {
		name := stringValue(metric["name"])
		item := metricQueryItem(metric)
		item["sql"] = buildSQLMetricQuery(name, qFrom, qTimeCol, qValueCol, qMetricCol, where, queryType, interval, req.Limit)
		queries = append(queries, item)
	}

	out := map[string]any{
		"dialect":       "clickhouse_sql",
		"database":      database,
		"table":         table,
		"time_column":   timeCol,
		"value_column":  valueCol,
		"metric_column": metricCol,
		"where":         where,
		"constraints":   echoConstraints(constraints),
		"metrics":       metricQueryItems(req.Metrics),
		"queries":       queries,
		"query_type":    queryType,
		"step":          step,
		"limit":         req.Limit,
	}
	return out, nil
}

// buildSQLMetricQuery renders one metric's SQL from already-validated, quoted
// identifiers and a validated INTERVAL operand. A range query downsamples with
// toStartOfInterval + avg (ClickHouse idiom); an instant query takes the latest
// point. {from} / {to} are filled by the caller from the plan's time_range.
func buildSQLMetricQuery(metric, qFrom, qTimeCol, qValueCol, qMetricCol, where, queryType, interval string, limit int) string {
	predicates := []string{}
	if where != "" {
		predicates = append(predicates, where)
	}
	if metric != "" {
		predicates = append(predicates, fmt.Sprintf("%s = %s", qMetricCol, sqlQuote(metric)))
	}
	predicates = append(predicates, fmt.Sprintf("%s BETWEEN {from} AND {to}", qTimeCol))
	whereClause := strings.Join(predicates, " AND ")

	if queryType == "instant" {
		return fmt.Sprintf("SELECT %s AS value FROM %s WHERE %s ORDER BY %s DESC LIMIT 1", qValueCol, qFrom, whereClause, qTimeCol)
	}
	sql := fmt.Sprintf(
		"SELECT toStartOfInterval(%s, INTERVAL %s) AS bucket, avg(%s) AS value FROM %s WHERE %s GROUP BY bucket ORDER BY bucket",
		qTimeCol, interval, qValueCol, qFrom, whereClause,
	)
	if limit > 0 {
		sql += fmt.Sprintf(" LIMIT %d", limit)
	}
	return sql
}

// constraintsToSQLWhere formats the IR as a SQL boolean expression. Field names
// are validated and backtick-quoted as identifiers; values are single-quoted with
// embedded quotes doubled.
func constraintsToSQLWhere(constraints []Constraint) (string, error) {
	parts := []string{}
	for _, c := range constraints {
		if c.Field == "" || len(c.Values) == 0 {
			continue
		}
		qField, err := quoteSQLIdent(c.Field)
		if err != nil {
			return "", sqlIdentError("filter field", c.Field, err)
		}
		switch c.Op {
		case "eq":
			parts = append(parts, fmt.Sprintf("%s = %s", qField, sqlQuote(c.Values[0])))
		case "neq":
			parts = append(parts, fmt.Sprintf("%s != %s", qField, sqlQuote(c.Values[0])))
		case "in":
			if len(c.Values) == 1 {
				parts = append(parts, fmt.Sprintf("%s = %s", qField, sqlQuote(c.Values[0])))
			} else {
				parts = append(parts, fmt.Sprintf("%s IN (%s)", qField, sqlQuoteList(c.Values)))
			}
		case "notin":
			if len(c.Values) == 1 {
				parts = append(parts, fmt.Sprintf("%s != %s", qField, sqlQuote(c.Values[0])))
			} else {
				parts = append(parts, fmt.Sprintf("%s NOT IN (%s)", qField, sqlQuoteList(c.Values)))
			}
		}
	}
	return strings.Join(parts, " AND "), nil
}

func echoConstraints(constraints []Constraint) []map[string]any {
	out := make([]map[string]any, 0, len(constraints))
	for _, c := range constraints {
		out = append(out, map[string]any{"field": c.Field, "op": c.Op, "values": c.Values})
	}
	return out
}

func sqlQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func sqlQuoteList(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, v := range values {
		quoted = append(quoted, sqlQuote(v))
	}
	return strings.Join(quoted, ", ")
}

var sqlIdentSegment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// quoteSQLIdent validates a (possibly dotted) SQL identifier sourced from storage
// or mapping config and returns it backtick-quoted for the ClickHouse dialect.
// Each dot-separated segment must match [A-Za-z_][A-Za-z0-9_]*, so a malformed or
// hostile mapping cannot inject SQL through an identifier position.
func quoteSQLIdent(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("empty identifier")
	}
	segments := strings.Split(name, ".")
	quoted := make([]string, 0, len(segments))
	for _, seg := range segments {
		if !sqlIdentSegment.MatchString(seg) {
			return "", fmt.Errorf("identifier %q is not a valid SQL identifier", name)
		}
		quoted = append(quoted, "`"+seg+"`")
	}
	return strings.Join(quoted, "."), nil
}

func sqlIdentError(kind, value string, err error) error {
	return apperrors.WithDetails(apperrors.CodeQueryPlanError,
		"sql-table renderer rejected an invalid "+kind,
		map[string]string{kind: value, "reason": err.Error()})
}

var sqlStepPattern = regexp.MustCompile(`^([0-9]+)(ms|s|m|h|d|w)$`)

var sqlStepUnits = map[string]string{
	"ms": "MILLISECOND",
	"s":  "SECOND",
	"m":  "MINUTE",
	"h":  "HOUR",
	"d":  "DAY",
	"w":  "WEEK",
}

// sqlInterval parses a Prometheus-style step (e.g. "30s", "5m") into a validated
// ClickHouse INTERVAL operand. Only a positive integer followed by an allowlisted
// unit is accepted, so step (a query parameter) cannot inject SQL into the plan.
func sqlInterval(step string) (string, error) {
	step = strings.TrimSpace(step)
	m := sqlStepPattern.FindStringSubmatch(step)
	if m == nil {
		return "", fmt.Errorf("step %q must be a positive integer followed by one of ms, s, m, h, d, w", step)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n <= 0 {
		return "", fmt.Errorf("step %q must be a positive integer", step)
	}
	return fmt.Sprintf("%d %s", n, sqlStepUnits[m[2]]), nil
}
