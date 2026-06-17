package query

import (
	"fmt"
	"strings"

	"github.com/alibaba/UnifiedModel/internal/query/planrender"
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
	where := constraintsToSQLWhere(constraints)

	table := firstNonEmpty(stringValue(req.Storage.Spec["table"]), stringValue(req.DataSet.Spec["table"]), req.DataSet.Name)
	timeCol := firstNonEmpty(stringValue(req.Storage.Spec["time_column"]), "timestamp")
	valueCol := firstNonEmpty(stringValue(req.Storage.Spec["value_column"]), "value")
	metricCol := firstNonEmpty(stringValue(req.Storage.Spec["metric_column"]), "metric_name")
	queryType := firstNonEmpty(req.QueryType, defaultMetricQueryMode(req.Metrics), stringValue(req.Storage.Spec["default_query_type"]), "range")
	step := firstNonEmpty(req.Step, stringValue(req.Storage.Spec["default_step"]), "1m")

	queries := []map[string]any{}
	for _, metric := range req.Metrics {
		name := stringValue(metric["name"])
		item := metricQueryItem(metric)
		item["sql"] = buildSQLMetricQuery(name, table, timeCol, valueCol, metricCol, where, queryType, step, req.Limit)
		queries = append(queries, item)
	}

	out := map[string]any{
		"dialect":       "clickhouse_sql",
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
	if len(raw) > 0 {
		out["raw_filters"] = raw
	}
	return out, nil
}

// buildSQLMetricQuery renders one metric's SQL. A range query downsamples with
// toStartOfInterval + avg (ClickHouse idiom); an instant query takes the latest
// point. {from} / {to} are filled by the caller from the plan's time_range.
func buildSQLMetricQuery(metric, table, timeCol, valueCol, metricCol, where, queryType, step string, limit int) string {
	predicates := []string{}
	if where != "" {
		predicates = append(predicates, where)
	}
	if metric != "" {
		predicates = append(predicates, fmt.Sprintf("%s = %s", metricCol, sqlQuote(metric)))
	}
	predicates = append(predicates, fmt.Sprintf("%s BETWEEN {from} AND {to}", timeCol))
	whereClause := strings.Join(predicates, " AND ")

	if queryType == "instant" {
		sql := fmt.Sprintf("SELECT %s AS value FROM %s WHERE %s ORDER BY %s DESC LIMIT 1", valueCol, table, whereClause, timeCol)
		return sql
	}
	sql := fmt.Sprintf(
		"SELECT toStartOfInterval(%s, INTERVAL %s) AS bucket, avg(%s) AS value FROM %s WHERE %s GROUP BY bucket ORDER BY bucket",
		timeCol, sqlInterval(step), valueCol, table, whereClause,
	)
	if limit > 0 {
		sql += fmt.Sprintf(" LIMIT %d", limit)
	}
	return sql
}

// constraintsToSQLWhere formats the IR as a SQL boolean expression. Values are
// single-quoted with embedded quotes doubled.
func constraintsToSQLWhere(constraints []Constraint) string {
	parts := []string{}
	for _, c := range constraints {
		if c.Field == "" || len(c.Values) == 0 {
			continue
		}
		switch c.Op {
		case "eq":
			parts = append(parts, fmt.Sprintf("%s = %s", c.Field, sqlQuote(c.Values[0])))
		case "neq":
			parts = append(parts, fmt.Sprintf("%s != %s", c.Field, sqlQuote(c.Values[0])))
		case "in":
			if len(c.Values) == 1 {
				parts = append(parts, fmt.Sprintf("%s = %s", c.Field, sqlQuote(c.Values[0])))
			} else {
				parts = append(parts, fmt.Sprintf("%s IN (%s)", c.Field, sqlQuoteList(c.Values)))
			}
		case "notin":
			if len(c.Values) == 1 {
				parts = append(parts, fmt.Sprintf("%s != %s", c.Field, sqlQuote(c.Values[0])))
			} else {
				parts = append(parts, fmt.Sprintf("%s NOT IN (%s)", c.Field, sqlQuoteList(c.Values)))
			}
		}
	}
	return strings.Join(parts, " AND ")
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

// sqlInterval maps a Prometheus-style step (1m, 30s, 5m) to a ClickHouse INTERVAL
// expression, defaulting to seconds when no unit is recognized.
func sqlInterval(step string) string {
	step = strings.TrimSpace(step)
	if step == "" {
		return "1 MINUTE"
	}
	units := []struct {
		suffix string
		unit   string
	}{
		{"ms", "MILLISECOND"},
		{"s", "SECOND"},
		{"m", "MINUTE"},
		{"h", "HOUR"},
		{"d", "DAY"},
		{"w", "WEEK"},
	}
	for _, u := range units {
		if strings.HasSuffix(step, u.suffix) {
			n := strings.TrimSuffix(step, u.suffix)
			if n == "" {
				n = "1"
			}
			return n + " " + u.unit
		}
	}
	return step + " SECOND"
}
