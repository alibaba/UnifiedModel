package query

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

type Executor struct {
	graph graphStore
}

func NewExecutor(graph graphStore) *Executor {
	return &Executor{graph: graph}
}

func (e *Executor) Execute(ctx context.Context, workspace string, plan model.QueryPlan) (model.QueryResult, error) {
	plan.Workspace = workspace

	var result model.QueryResult
	var err error
	switch plan.Source {
	case ".umodel":
		result, err = e.executeUModel(ctx, workspace, plan)
	case ".entity_set":
		result, err = e.executeEntitySetCall(ctx, workspace, plan)
	case ".entity":
		result, err = e.graph.QueryEntities(ctx, model.EntityQueryPlan(plan))
	case ".topo":
		result, err = e.graph.QueryTopo(ctx, model.TopoQueryPlan(plan))
	default:
		return model.QueryResult{}, apperrors.New(apperrors.CodeQueryPlanError, "unsupported query source")
	}
	if err != nil {
		return model.QueryResult{}, err
	}

	rows, columns := applyPipeline(plan.Source, result.Rows, result.Columns, plan)
	result.Rows = rows
	result.Columns = columns
	result.Page = model.PageRequest{Limit: plan.Limit}
	return result, nil
}

func (e *Executor) executeUModel(ctx context.Context, workspace string, plan model.QueryPlan) (model.QueryResult, error) {
	snapshot, err := e.graph.GetUModelSnapshot(ctx, model.UModelSnapshotRequest{Workspace: workspace})
	if err != nil {
		return model.QueryResult{}, err
	}
	rows := make([]map[string]any, 0, len(snapshot.Elements))
	for _, element := range snapshot.Elements {
		rows = append(rows, map[string]any{
			"kind":    element.Kind,
			"domain":  element.Domain,
			"name":    element.Name,
			"version": element.Version,
			"spec":    element.Spec,
		})
	}
	return model.QueryResult{
		Columns: []string{"kind", "domain", "name", "version"},
		Rows:    rows,
		Page:    model.PageRequest{Limit: plan.Limit},
	}, nil
}

func (e *Executor) executeEntitySetCall(ctx context.Context, workspace string, plan model.QueryPlan) (model.QueryResult, error) {
	if plan.EntityCall == nil {
		return model.QueryResult{}, apperrors.New(apperrors.CodeQueryPlanError, ".entity_set requires entity-call")
	}
	switch plan.EntityCall.Name {
	case "__list_method__":
		return entitySetAssistantRawResponse(entityCallListMethodHeader(), entityCallListMethodRows()), nil
	case "list_data_set":
		return e.executeEntitySetListDataSet(ctx, workspace, plan)
	default:
		return model.QueryResult{}, apperrors.WithDetails(apperrors.CodeQueryPlanError, "unsupported entity-call method", map[string]string{"name": plan.EntityCall.Name})
	}
}

func entitySetAssistantRawResponse(header []string, data []map[string]any) model.QueryResult {
	return model.QueryResult{
		Columns: []string{"responseType", "query", "header", "data"},
		Rows: []map[string]any{{
			"responseType": 2,
			"query":        "",
			"header":       header,
			"data":         data,
		}},
	}
}

func (e *Executor) executeEntitySetListDataSet(ctx context.Context, workspace string, plan model.QueryPlan) (model.QueryResult, error) {
	snapshot, err := e.graph.GetUModelSnapshot(ctx, model.UModelSnapshotRequest{Workspace: workspace})
	if err != nil {
		return model.QueryResult{}, err
	}
	dataSetTypes := stringSet(stringSliceValue(plan.EntityCall.Parameters["data_set_types"]))
	detail := boolValue(plan.EntityCall.Parameters["detail"])
	rows := make([]map[string]any, 0)
	for _, link := range snapshot.Elements {
		if link.Kind != "data_link" {
			continue
		}
		src := refFromSpec(link.Spec, "src")
		dest := refFromSpec(link.Spec, "dest")
		if src.Kind != "entity_set" || src.Domain != stringFilter(plan.Filters["domain"]) || src.Name != stringFilter(plan.Filters["name"]) {
			continue
		}
		if len(dataSetTypes) > 0 {
			if _, ok := dataSetTypes[dest.Kind]; !ok {
				continue
			}
		}
		dataSet, ok := findUModelElement(snapshot.Elements, dest.Kind, dest.Domain, dest.Name)
		if !ok {
			continue
		}
		rows = append(rows, entityCallRowValues(listDataSetValues(snapshot.Elements, link, dataSet, detail)))
	}
	return entitySetAssistantRawResponse(listDataSetHeader(), rows), nil
}

type setRef struct {
	Domain string
	Kind   string
	Name   string
}

func entityCallRowValues(values []string) map[string]any {
	return map[string]any{"values": values}
}

func entityCallListMethodHeader() []string {
	return []string{"name", "display_name", "description", "params", "returns"}
}

func entityCallListMethodRows() []map[string]any {
	rows := []map[string]any{}
	for _, method := range []entityCallMethodInfo{
		methodInfoListMethods(),
		methodInfoListDataSet(),
	} {
		rows = append(rows, entityCallRowValues([]string{
			method.Name,
			method.DisplayName,
			method.Description,
			mustJSON(method.Params),
			mustJSON(method.Returns),
		}))
	}
	return rows
}

type entityCallMethodInfo struct {
	Name        string
	DisplayName string
	Description string
	Params      []assistantParamInfo
	Returns     []assistantReturnInfo
}

type assistantParamInfo struct {
	Key         string `json:"key"`
	Type        string `json:"type"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     any    `json:"default,omitempty"`
}

type assistantReturnInfo struct {
	Key         string `json:"key"`
	Type        string `json:"type"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
}

func methodInfoListMethods() entityCallMethodInfo {
	return entityCallMethodInfo{
		Name:        "__list_method__",
		DisplayName: "List Available Methods",
		Description: "Get all methods supported by current EntitySet",
		Returns: []assistantReturnInfo{
			{Key: "name", Type: "varchar", DisplayName: "Method Name"},
			{Key: "display_name", Type: "varchar", DisplayName: "Method Display Name"},
			{Key: "description", Type: "varchar", DisplayName: "Method Description"},
			{Key: "params", Type: "varchar", DisplayName: "Method Parameters (JSON)"},
			{Key: "returns", Type: "varchar", DisplayName: "Method Returns (JSON)"},
		},
	}
}

func methodInfoListDataSet() entityCallMethodInfo {
	return entityCallMethodInfo{
		Name:        "list_data_set",
		DisplayName: "List DataSets",
		Description: "Get DataSets(MetricSet, LogSet, TraceSet, EventSet...) related to EntitySet",
		Params: []assistantParamInfo{
			{Key: "data_set_types", Type: "array<varchar>", DisplayName: "Data Set Types, metric_set, log_set, trace_set, event_set"},
			{Key: "detail", Type: "boolean", DisplayName: "Detail Info, if true, return all fields of DataSet", Default: false},
		},
		Returns: returnsFromHeader(listDataSetHeader()),
	}
}

func returnsFromHeader(header []string) []assistantReturnInfo {
	out := make([]assistantReturnInfo, 0, len(header))
	for _, key := range header {
		out = append(out, assistantReturnInfo{Key: key, Type: "varchar", DisplayName: key})
	}
	return out
}

func listDataSetHeader() []string {
	return []string{"data_set_id", "type", "domain", "name", "fields_mapping", "filterable_fields", "fields", "storage_info", "storage_link_info", "data_link_detail", "data_set_detail", "storage_detail", "storage_link_detail"}
}

func listDataSetValues(elements []model.UModelElement, link model.UModelElement, dataSet model.UModelElement, detail bool) []string {
	storageInfo, storageLinkInfo, storageDetail, storageLinkDetail := storageDetailsForDataSet(elements, dataSet)
	dataLinkDetail := "{}"
	dataSetDetail := "{}"
	if detail {
		dataLinkDetail = mustJSON(link)
		dataSetDetail = mustJSON(dataSet)
	} else {
		storageDetail = []model.UModelElement{}
		storageLinkDetail = []model.UModelElement{}
	}
	fieldsMapping := mapValue(link.Spec["fields_mapping"])
	return []string{
		uniqueID(dataSet.Domain, dataSet.Kind, dataSet.Name),
		dataSet.Kind,
		dataSet.Domain,
		dataSet.Name,
		mustJSON(fieldsMapping),
		mustJSON(filterableFields(dataSet)),
		mustJSON(dataSet.Spec["fields"]),
		mustJSON(storageInfo),
		mustJSON(storageLinkInfo),
		dataLinkDetail,
		dataSetDetail,
		mustJSON(storageDetail),
		mustJSON(storageLinkDetail),
	}
}

func storageDetailsForDataSet(elements []model.UModelElement, dataSet model.UModelElement) ([]map[string]any, []map[string]any, []model.UModelElement, []model.UModelElement) {
	storageInfo := []map[string]any{}
	storageLinkInfo := []map[string]any{}
	storageDetail := []model.UModelElement{}
	storageLinkDetail := []model.UModelElement{}
	for _, link := range elements {
		if link.Kind != "storage_link" {
			continue
		}
		src := refFromSpec(link.Spec, "src")
		if src.Domain != dataSet.Domain || src.Kind != dataSet.Kind || src.Name != dataSet.Name {
			continue
		}
		dest := refFromSpec(link.Spec, "dest")
		if storage, ok := findUModelElement(elements, dest.Kind, dest.Domain, dest.Name); ok {
			storageInfo = append(storageInfo, map[string]any{
				"domain": storage.Domain,
				"type":   storage.Kind,
				"name":   storage.Name,
				"config": storage.Spec,
			})
			storageDetail = append(storageDetail, storage)
		}
		storageLinkInfo = append(storageLinkInfo, map[string]any{
			"domain": link.Domain,
			"name":   link.Name,
			"spec":   link.Spec,
		})
		storageLinkDetail = append(storageLinkDetail, link)
	}
	return storageInfo, storageLinkInfo, storageDetail, storageLinkDetail
}

func refFromSpec(spec map[string]any, key string) setRef {
	value, _ := spec[key].(map[string]any)
	return setRef{
		Domain: stringValue(value["domain"]),
		Kind:   stringValue(value["kind"]),
		Name:   stringValue(value["name"]),
	}
}

func findUModelElement(elements []model.UModelElement, kind, domain, name string) (model.UModelElement, bool) {
	for _, element := range elements {
		if element.Kind == kind && element.Domain == domain && element.Name == name {
			return element, true
		}
	}
	return model.UModelElement{}, false
}

func filterableFields(element model.UModelElement) []string {
	fields, ok := element.Spec["fields"].([]any)
	if !ok {
		return []string{}
	}
	out := []string{}
	for _, field := range fields {
		item, ok := field.(map[string]any)
		if !ok {
			continue
		}
		if boolValue(item["filterable"]) || boolValue(item["analysable"]) {
			out = append(out, stringValue(item["name"]))
		}
	}
	return out
}

func mapValue(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func stringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func uniqueID(domain, kind, name string) string {
	return strings.Join([]string{domain, kind, name}, "@")
}

func mustJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "null"
	}
	return string(encoded)
}

func applyPipeline(source string, rows []map[string]any, columns []string, plan model.QueryPlan) ([]map[string]any, []string) {
	if source == ".entity_set" {
		return limitRows(rows, plan.Limit), columns
	}
	if !hasOperator(plan.Pipeline, "with") && len(plan.Filters) > 0 {
		rows = filterRows(source, rows, plan.Filters)
	}

	for _, operator := range plan.Pipeline {
		switch operator.Name {
		case "with":
			rows = filterRows(source, rows, plan.Filters)
		case "where":
			if operator.Predicate != nil {
				rows = filterPredicate(rows, *operator.Predicate)
			}
		case "project":
			rows, columns = projectRows(rows, operator.Project)
		case "sort":
			if operator.Sort != nil {
				sortRows(rows, *operator.Sort)
			}
		case "limit":
			rows = limitRows(rows, operator.Limit)
		}
	}

	rows = limitRows(rows, plan.Limit)
	return rows, columns
}

func hasOperator(operators []model.QueryPipelineOperator, name string) bool {
	for _, operator := range operators {
		if operator.Name == name {
			return true
		}
	}
	return false
}

func filterRows(source string, rows []map[string]any, filters map[string]any) []map[string]any {
	if len(filters) == 0 {
		return rows
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if rowMatchesFilters(source, row, filters) {
			out = append(out, row)
		}
	}
	return out
}

func rowMatchesFilters(source string, row map[string]any, filters map[string]any) bool {
	switch source {
	case ".umodel":
		if _, ok := filters["id"]; ok {
			return false
		}
		if !stringMatches(rowString(row, "kind"), filters["kind"]) {
			return false
		}
		if !stringMatches(rowString(row, "domain"), filters["domain"]) {
			return false
		}
		if !stringMatches(rowString(row, "name"), filters["name"]) {
			return false
		}
	case ".entity":
		if !stringMatches(rowString(row, "__domain__"), filters["domain"]) {
			return false
		}
		if !stringMatches(rowString(row, "__entity_type__"), filters["name"]) {
			return false
		}
		if !matchesIDs(rowString(row, "__entity_id__"), filters["ids"]) {
			return false
		}
	case ".entity_set":
		if !stringMatches(rowString(row, "domain"), filters["domain"]) {
			return false
		}
		if !stringMatches(rowString(row, "name"), filters["name"]) {
			return false
		}
	case ".topo":
		relationType := rowString(row, "__relation_type__")
		if relationType == "" {
			relationType = rowString(row, "relation")
		}
		if !stringMatches(relationType, coalesce(filters["relation_type"], filters["type"])) {
			return false
		}
		if !stringMatches(rowString(row, "src"), filters["src"]) {
			return false
		}
		if !stringMatches(rowString(row, "dest"), filters["dest"]) {
			return false
		}
	}

	query := stringFilter(filters["query"])
	if query != "" && !rowContains(row, query) {
		return false
	}
	return true
}

func filterPredicate(rows []map[string]any, predicate model.QueryPredicate) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if predicateMatches(row, predicate) {
			out = append(out, row)
		}
	}
	return out
}

func predicateMatches(row map[string]any, predicate model.QueryPredicate) bool {
	left, ok := row[predicate.Field]
	if !ok {
		return false
	}
	switch predicate.Op {
	case "=", "==":
		return compareEqual(left, predicate.Value)
	case "!=":
		return !compareEqual(left, predicate.Value)
	case "contains", "~":
		return containsFold(stringValue(left), stringValue(predicate.Value))
	case ">", ">=", "<", "<=":
		return compareOrdered(left, predicate.Value, predicate.Op)
	default:
		return false
	}
}

func projectRows(rows []map[string]any, fields []string) ([]map[string]any, []string) {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		next := make(map[string]any, len(fields))
		for _, field := range fields {
			next[field] = row[field]
		}
		out = append(out, next)
	}
	return out, append([]string(nil), fields...)
}

func sortRows(rows []map[string]any, sortSpec model.QuerySort) {
	sort.SliceStable(rows, func(i, j int) bool {
		cmp := compareForSort(rows[i][sortSpec.Field], rows[j][sortSpec.Field])
		if sortSpec.Desc {
			return cmp > 0
		}
		return cmp < 0
	})
}

func limitRows(rows []map[string]any, limit int) []map[string]any {
	if limit <= 0 || len(rows) <= limit {
		return rows
	}
	return rows[:limit]
}

func compareEqual(left, right any) bool {
	if lf, ok := floatValue(left); ok {
		if rf, ok := floatValue(right); ok {
			return lf == rf
		}
	}
	return stringValue(left) == stringValue(right)
}

func compareOrdered(left, right any, op string) bool {
	lf, lok := floatValue(left)
	rf, rok := floatValue(right)
	if lok && rok {
		switch op {
		case ">":
			return lf > rf
		case ">=":
			return lf >= rf
		case "<":
			return lf < rf
		case "<=":
			return lf <= rf
		}
	}
	lv := stringValue(left)
	rv := stringValue(right)
	switch op {
	case ">":
		return lv > rv
	case ">=":
		return lv >= rv
	case "<":
		return lv < rv
	case "<=":
		return lv <= rv
	default:
		return false
	}
}

func compareForSort(left, right any) int {
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
	lv := stringValue(left)
	rv := stringValue(right)
	switch {
	case lv < rv:
		return -1
	case lv > rv:
		return 1
	default:
		return 0
	}
}

func floatValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case string:
		n, err := strconv.ParseFloat(typed, 64)
		return n, err == nil
	default:
		return 0, false
	}
}

func rowContains(row map[string]any, query string) bool {
	for _, value := range row {
		if containsFold(stringValue(value), query) {
			return true
		}
	}
	return false
}

func rowString(row map[string]any, key string) string {
	return stringValue(row[key])
}

func matchesIDs(value string, filter any) bool {
	if filter == nil {
		return true
	}
	switch ids := filter.(type) {
	case []string:
		for _, id := range ids {
			if id == value {
				return true
			}
		}
		return false
	case []any:
		for _, id := range ids {
			if stringValue(id) == value {
				return true
			}
		}
		return false
	default:
		return stringValue(filter) == "" || stringValue(filter) == value
	}
}

func stringMatches(value string, filter any) bool {
	expected := stringFilter(filter)
	if expected == "" || expected == "*" {
		return true
	}
	if strings.HasSuffix(expected, "*") {
		return strings.HasPrefix(value, strings.TrimSuffix(expected, "*"))
	}
	return value == expected
}

func stringFilter(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(value)
	}
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(typed, "true")
	default:
		return false
	}
}

func containsFold(value, query string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(query))
}

func coalesce(values ...any) any {
	for _, value := range values {
		if stringValue(value) != "" {
			return value
		}
	}
	return nil
}
