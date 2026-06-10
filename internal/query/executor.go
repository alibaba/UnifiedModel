package query

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
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
	case "get_logs":
		return e.executeEntitySetGetLogs(ctx, workspace, plan)
	case "get_metrics":
		return e.executeEntitySetGetMetrics(ctx, workspace, plan)
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

func entitySetAssistantQueryResponse(query string) model.QueryResult {
	return model.QueryResult{
		Columns: []string{"responseType", "query", "header", "data"},
		Rows: []map[string]any{{
			"responseType": 1,
			"query":        query,
			"header":       []string{},
			"data":         []map[string]any{},
		}},
	}
}

func (e *Executor) executeEntitySetListDataSet(ctx context.Context, workspace string, plan model.QueryPlan) (model.QueryResult, error) {
	snapshot, err := e.graph.GetUModelSnapshot(ctx, model.UModelSnapshotRequest{Workspace: workspace})
	if err != nil {
		return model.QueryResult{}, err
	}
	dataSetTypes := dataSetTypeSet(stringSliceValue(plan.EntityCall.Parameters["data_set_types"]))
	detail := boolValue(plan.EntityCall.Parameters["detail"])
	rows := make([]map[string]any, 0)
	for _, related := range relatedDataSetsForEntitySet(snapshot.Elements, stringFilter(plan.Filters["domain"]), stringFilter(plan.Filters["name"]), dataSetTypes, plan.EntityData) {
		rows = append(rows, entityCallRowValues(listDataSetValues(snapshot.Elements, related, detail, plan.EntityData)))
	}
	return entitySetAssistantRawResponse(listDataSetHeader(), rows), nil
}

func (e *Executor) executeEntitySetGetLogs(ctx context.Context, workspace string, plan model.QueryPlan) (model.QueryResult, error) {
	snapshot, err := e.graph.GetUModelSnapshot(ctx, model.UModelSnapshotRequest{Workspace: workspace})
	if err != nil {
		return model.QueryResult{}, err
	}
	domain := stringFilter(plan.EntityCall.Parameters["domain"])
	name := stringFilter(plan.EntityCall.Parameters["name"])
	dataLink, logSet, ok := findRelatedDataSet(snapshot.Elements, stringFilter(plan.Filters["domain"]), stringFilter(plan.Filters["name"]), "log_set", domain, name, plan.EntityData)
	if !ok {
		return model.QueryResult{}, apperrors.WithDetails(apperrors.CodeQueryPlanError, "related log_set not found", map[string]string{
			"domain": domain,
			"name":   name,
		})
	}
	bindings := storageBindingsForDataSet(snapshot.Elements, logSet, plan.EntityData)
	if len(bindings) == 0 {
		return model.QueryResult{}, apperrors.WithDetails(apperrors.CodeQueryPlanError, "log_set storage not found", map[string]string{
			"domain": domain,
			"name":   name,
		})
	}

	queryPlan := logQueryPlan(plan, dataLink, logSet, bindings[0])
	return entitySetAssistantQueryResponse(mustJSON(queryPlan)), nil
}

func (e *Executor) executeEntitySetGetMetrics(ctx context.Context, workspace string, plan model.QueryPlan) (model.QueryResult, error) {
	snapshot, err := e.graph.GetUModelSnapshot(ctx, model.UModelSnapshotRequest{Workspace: workspace})
	if err != nil {
		return model.QueryResult{}, err
	}
	domain := stringFilter(plan.EntityCall.Parameters["domain"])
	name := stringFilter(plan.EntityCall.Parameters["name"])
	dataLink, metricSet, ok := findRelatedDataSet(snapshot.Elements, stringFilter(plan.Filters["domain"]), stringFilter(plan.Filters["name"]), "metric_set", domain, name, plan.EntityData)
	if !ok {
		return model.QueryResult{}, apperrors.WithDetails(apperrors.CodeQueryPlanError, "related metric_set not found", map[string]string{
			"domain": domain,
			"name":   name,
		})
	}
	bindings := storageBindingsForDataSet(snapshot.Elements, metricSet, plan.EntityData)
	if len(bindings) == 0 {
		return model.QueryResult{}, apperrors.WithDetails(apperrors.CodeQueryPlanError, "metric_set storage not found", map[string]string{
			"domain": domain,
			"name":   name,
		})
	}
	metrics, err := selectedMetricSpecs(metricSet, stringFilter(plan.EntityCall.Parameters["metric"]))
	if err != nil {
		return model.QueryResult{}, err
	}

	queryPlan := metricQueryPlan(plan, dataLink, metricSet, bindings[0], metrics)
	return entitySetAssistantQueryResponse(mustJSON(queryPlan)), nil
}

type setRef struct {
	Domain string
	Kind   string
	Name   string
}

type storageBinding struct {
	Link    model.UModelElement
	Storage model.UModelElement
}

type relatedDataSet struct {
	Link                  model.UModelElement
	HasLink               bool
	DataSet               model.UModelElement
	FilterStorageByEntity bool
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
		methodInfoGetLogs(),
		methodInfoGetMetrics(),
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

func methodInfoGetLogs() entityCallMethodInfo {
	return entityCallMethodInfo{
		Name:        "get_logs",
		DisplayName: "Get Logs",
		Description: "Get log query plan from a LogSet",
		Params: []assistantParamInfo{
			{Key: "domain", Type: "varchar", DisplayName: "log_set Domain", Required: true},
			{Key: "name", Type: "varchar", DisplayName: "log_set Name", Required: true},
			{
				Key:         "query",
				Type:        "varchar",
				DisplayName: "Query expression for the log set",
				Description: "Basic SPL where syntax, for example service_id = 'service_a' and level in ['ERROR', 'WARN'].",
			},
			{Key: "storage_domain", Type: "varchar", DisplayName: "Storage Domain", Description: "Optional storage domain used to select a specific StorageLink target."},
			{Key: "storage_name", Type: "varchar", DisplayName: "Storage Name", Description: "Optional storage name used to select a specific StorageLink target."},
			{Key: "storage_kind", Type: "varchar", DisplayName: "Storage Kind", Description: "Optional storage kind used to select a specific StorageLink target."},
		},
		Returns: []assistantReturnInfo{
			{Key: "query", Type: "varchar", DisplayName: "Log query plan"},
		},
	}
}

func methodInfoGetMetrics() entityCallMethodInfo {
	return entityCallMethodInfo{
		Name:        "get_metrics",
		DisplayName: "Get Metrics",
		Description: "Get metric query plan from a MetricSet",
		Params: []assistantParamInfo{
			{Key: "domain", Type: "varchar", DisplayName: "metric_set Domain", Required: true},
			{Key: "name", Type: "varchar", DisplayName: "metric_set Name", Required: true},
			{
				Key:         "metric",
				Type:        "varchar",
				DisplayName: "Metric name",
				Description: "Optional metric name. When omitted, all metrics in the MetricSet are planned.",
			},
			{
				Key:         "query",
				Type:        "varchar",
				DisplayName: "Query expression for metric labels",
				Description: "Basic SPL where syntax, for example service_id = 'service_a' and environment = 'prod'.",
			},
			{Key: "query_type", Type: "varchar", DisplayName: "Prometheus query type", Description: "range or instant. Defaults to the MetricSet/storage preference."},
			{Key: "step", Type: "varchar", DisplayName: "Range query step", Description: "Range query step, for example 1m."},
			{Key: "aggregate", Type: "boolean", DisplayName: "Aggregate time series", Description: "Whether to aggregate the time series.", Default: true},
			{Key: "storage_domain", Type: "varchar", DisplayName: "Storage Domain", Description: "Optional storage domain used to select a specific StorageLink target."},
			{Key: "storage_name", Type: "varchar", DisplayName: "Storage Name", Description: "Optional storage name used to select a specific StorageLink target."},
			{Key: "storage_kind", Type: "varchar", DisplayName: "Storage Kind", Description: "Optional storage kind used to select a specific StorageLink target."},
		},
		Returns: []assistantReturnInfo{
			{Key: "query", Type: "varchar", DisplayName: "Metric query plan"},
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

func listDataSetValues(elements []model.UModelElement, related relatedDataSet, detail bool, entityData *model.EntityData) []string {
	link := related.Link
	dataSet := related.DataSet
	storageInfo, storageLinkInfo, storageDetail, storageLinkDetail := storageDetailsForDataSet(elements, dataSet, entityData, related.FilterStorageByEntity)
	dataLinkDetail := "{}"
	dataSetDetail := "{}"
	if detail {
		if related.HasLink {
			dataLinkDetail = mustJSON(link)
		} else {
			dataLinkDetail = "null"
		}
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
		mustJSON(dataSetFields(dataSet)),
		mustJSON(storageInfo),
		mustJSON(storageLinkInfo),
		dataLinkDetail,
		dataSetDetail,
		mustJSON(storageDetail),
		mustJSON(storageLinkDetail),
	}
}

func relatedDataSetsForEntitySet(elements []model.UModelElement, entityDomain, entityName string, dataSetTypes map[string]struct{}, entityData *model.EntityData) []relatedDataSet {
	out := []relatedDataSet{}
	seen := map[string]struct{}{}
	for _, link := range elements {
		if link.Kind != "data_link" {
			continue
		}
		src := refFromSpec(link.Spec, "src")
		dest := refFromSpec(link.Spec, "dest")
		if src.Kind != "entity_set" || src.Domain != entityDomain || src.Name != entityName {
			continue
		}
		if !dataSetTypeAllowed(dataSetTypes, dest.Kind) {
			continue
		}
		dataSet, ok := findUModelElement(elements, dest.Kind, dest.Domain, dest.Name)
		if !ok {
			continue
		}
		if !filterByEntityAllows(filterByEntityExpression(link.Spec), entityData) {
			continue
		}
		out = append(out, relatedDataSet{Link: link, HasLink: true, DataSet: dataSet, FilterStorageByEntity: true})
		seen[uniqueID(dataSet.Domain, dataSet.Kind, dataSet.Name)] = struct{}{}
	}

	for _, dataSet := range elements {
		if dataSet.Domain != "default" || !dataSetTypeAllowed(dataSetTypes, dataSet.Kind) {
			continue
		}
		id := uniqueID(dataSet.Domain, dataSet.Kind, dataSet.Name)
		if _, exists := seen[id]; exists {
			continue
		}
		out = append(out, relatedDataSet{DataSet: dataSet})
		seen[id] = struct{}{}
	}
	return out
}

func storageDetailsForDataSet(elements []model.UModelElement, dataSet model.UModelElement, entityData *model.EntityData, filterByEntity bool) ([]map[string]any, []map[string]any, []model.UModelElement, []model.UModelElement) {
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
		if filterByEntity && !filterByEntityAllows(filterByEntityExpression(link.Spec), entityData) {
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

func findRelatedDataSet(elements []model.UModelElement, entityDomain, entityName, dataSetKind, dataSetDomain, dataSetName string, entityData *model.EntityData) (model.UModelElement, model.UModelElement, bool) {
	for _, link := range elements {
		if link.Kind != "data_link" {
			continue
		}
		src := refFromSpec(link.Spec, "src")
		dest := refFromSpec(link.Spec, "dest")
		if src.Kind != "entity_set" || src.Domain != entityDomain || src.Name != entityName {
			continue
		}
		if dest.Kind != dataSetKind || dest.Domain != dataSetDomain || dest.Name != dataSetName {
			continue
		}
		if !filterByEntityAllows(filterByEntityExpression(link.Spec), entityData) {
			continue
		}
		dataSet, ok := findUModelElement(elements, dest.Kind, dest.Domain, dest.Name)
		if !ok {
			continue
		}
		return link, dataSet, true
	}
	return model.UModelElement{}, model.UModelElement{}, false
}

func storageBindingsForDataSet(elements []model.UModelElement, dataSet model.UModelElement, entityData *model.EntityData) []storageBinding {
	out := []storageBinding{}
	for _, link := range elements {
		if link.Kind != "storage_link" {
			continue
		}
		src := refFromSpec(link.Spec, "src")
		if src.Domain != dataSet.Domain || src.Kind != dataSet.Kind || src.Name != dataSet.Name {
			continue
		}
		if !filterByEntityAllows(filterByEntityExpression(link.Spec), entityData) {
			continue
		}
		dest := refFromSpec(link.Spec, "dest")
		storage, ok := findUModelElement(elements, dest.Kind, dest.Domain, dest.Name)
		if !ok {
			continue
		}
		out = append(out, storageBinding{Link: link, Storage: storage})
	}
	return out
}

func logQueryPlan(plan model.QueryPlan, dataLink model.UModelElement, logSet model.UModelElement, binding storageBinding) map[string]any {
	dataLinkMapping := mapValue(dataLink.Spec["fields_mapping"])
	storageLinkMapping := mapValue(binding.Link.Spec["fields_mapping"])
	entityIDs := stringSliceValue(plan.Filters["ids"])
	entityQuery := stringFilter(plan.Filters["query"])
	dataFilter := stringValue(dataLink.Spec["data_filter"])
	methodQuery := stringFilter(plan.EntityCall.Parameters["query"])

	queryPlan := map[string]any{
		"mode":         "plan",
		"version":      "v1",
		"operation":    "get_logs",
		"description":  describeLogPlan(logSet, binding.Storage, methodQuery),
		"next_action":  nextActionForwardToExecutor,
		"source_query": plan.Query,
		"data_source": map[string]any{
			"data_set": map[string]any{
				"domain": logSet.Domain,
				"kind":   logSet.Kind,
				"name":   logSet.Name,
			},
			"storage": map[string]any{
				"domain": binding.Storage.Domain,
				"type":   binding.Storage.Kind,
				"name":   binding.Storage.Name,
				"config": binding.Storage.Spec,
			},
			"data_link": map[string]any{
				"domain": dataLink.Domain,
				"name":   dataLink.Name,
				"spec":   dataLink.Spec,
			},
			"storage_link": map[string]any{
				"domain": binding.Link.Domain,
				"name":   binding.Link.Name,
				"spec":   binding.Link.Spec,
			},
		},
		"params_echo": echoParams(plan.EntityCall.Parameters),
		"query":       buildLogStorageQuery(logSet, binding.Storage, dataLinkMapping, storageLinkMapping, entityIDs, entityQuery, dataFilter, methodQuery, plan.EntityData, plan.Limit),
	}
	if plan.TimeRange.From != nil || plan.TimeRange.To != nil {
		queryPlan["time_range"] = plan.TimeRange
	}
	return queryPlan
}

func metricQueryPlan(plan model.QueryPlan, dataLink model.UModelElement, metricSet model.UModelElement, binding storageBinding, metrics []map[string]any) map[string]any {
	dataLinkMapping := mapValue(dataLink.Spec["fields_mapping"])
	storageLinkMapping := mapValue(binding.Link.Spec["fields_mapping"])
	entityIDs := stringSliceValue(plan.Filters["ids"])
	entityQuery := stringFilter(plan.Filters["query"])
	dataFilter := stringValue(dataLink.Spec["data_filter"])
	methodQuery := stringFilter(plan.EntityCall.Parameters["query"])
	queryType := stringFilter(plan.EntityCall.Parameters["query_type"])
	step := stringFilter(plan.EntityCall.Parameters["step"])

	metricName := stringFilter(plan.EntityCall.Parameters["metric"])
	queryPlan := map[string]any{
		"mode":         "plan",
		"version":      "v1",
		"operation":    "get_metrics",
		"description":  describeMetricPlan(metricSet, binding.Storage, metricName, methodQuery, queryType, step),
		"next_action":  nextActionForwardToExecutor,
		"source_query": plan.Query,
		"data_source": map[string]any{
			"data_set": map[string]any{
				"domain": metricSet.Domain,
				"kind":   metricSet.Kind,
				"name":   metricSet.Name,
			},
			"storage": map[string]any{
				"domain": binding.Storage.Domain,
				"type":   binding.Storage.Kind,
				"name":   binding.Storage.Name,
				"config": binding.Storage.Spec,
			},
			"data_link": map[string]any{
				"domain": dataLink.Domain,
				"name":   dataLink.Name,
				"spec":   dataLink.Spec,
			},
			"storage_link": map[string]any{
				"domain": binding.Link.Domain,
				"name":   binding.Link.Name,
				"spec":   binding.Link.Spec,
			},
		},
		"params_echo": echoParams(plan.EntityCall.Parameters),
		"query":       buildMetricStorageQuery(metricSet, binding.Storage, dataLinkMapping, storageLinkMapping, metrics, entityIDs, entityQuery, dataFilter, methodQuery, plan.EntityData, queryType, step, plan.Limit),
	}
	if plan.TimeRange.From != nil || plan.TimeRange.To != nil {
		queryPlan["time_range"] = plan.TimeRange
	}
	return queryPlan
}

// nextActionForwardToExecutor is the canonical "next_action" hint embedded in
// every plan-mode response. An AI agent that receives this plan should not
// try to execute the inner storage query itself; the canonical path is to
// forward the plan to a UModel data executor (e.g. umodel-assistant) that
// turns it into rows.
const nextActionForwardToExecutor = "forward_to_executor"

// describeMetricPlan returns a one-line human-readable summary of what the
// metric plan does, so an AI agent can render or relay it to a user without
// having to reverse-engineer the inner storage query.
func describeMetricPlan(metricSet, storage model.UModelElement, metricName, filter, queryType, step string) string {
	metricLabel := metricName
	if metricLabel == "" {
		metricLabel = "all metrics"
	} else {
		metricLabel = fmt.Sprintf("metric %q", metricName)
	}
	parts := []string{fmt.Sprintf("Retrieve %s from MetricSet %s/%s", metricLabel, metricSet.Domain, metricSet.Name)}
	if filter != "" {
		parts = append(parts, fmt.Sprintf("filtered by [%s]", filter))
	}
	if queryType != "" {
		parts = append(parts, fmt.Sprintf("as %s query", queryType))
	}
	if step != "" {
		parts = append(parts, fmt.Sprintf("with step %s", step))
	}
	parts = append(parts, fmt.Sprintf("(storage: %s/%s).", storage.Kind, storage.Name))
	parts = append(parts, "Forward this plan to a UModel data executor (e.g. umodel-assistant) to fetch real time series.")
	return strings.Join(parts, " ")
}

// describeLogPlan returns a one-line human-readable summary of what the log
// plan does, parallel to describeMetricPlan.
func describeLogPlan(logSet, storage model.UModelElement, filter string) string {
	parts := []string{fmt.Sprintf("Retrieve logs from LogSet %s/%s", logSet.Domain, logSet.Name)}
	if filter != "" {
		parts = append(parts, fmt.Sprintf("filtered by [%s]", filter))
	}
	parts = append(parts, fmt.Sprintf("(storage: %s/%s).", storage.Kind, storage.Name))
	parts = append(parts, "Forward this plan to a UModel data executor (e.g. umodel-assistant) to fetch real log rows.")
	return strings.Join(parts, " ")
}

// echoParams returns the entity-call parameters that the caller actually
// supplied, with nil and empty-string values stripped. Plan v1 includes this
// echo so an executor (e.g. umodel-assistant) can recover the full call
// context — including parameters declared in the method signature but not
// consumed by the open-source planner (aggregate, storage_*).
func echoParams(params map[string]any) map[string]any {
	if len(params) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(params))
	for k, v := range params {
		switch val := v.(type) {
		case nil:
			continue
		case string:
			if val == "" {
				continue
			}
			out[k] = val
		default:
			out[k] = v
		}
	}
	return out
}

func selectedMetricSpecs(metricSet model.UModelElement, metricName string) ([]map[string]any, error) {
	items, ok := metricSet.Spec["metrics"].([]any)
	if !ok || len(items) == 0 {
		return nil, apperrors.WithDetails(apperrors.CodeQueryPlanError, "metric_set metrics not found", map[string]string{
			"domain": metricSet.Domain,
			"name":   metricSet.Name,
		})
	}
	out := []map[string]any{}
	for _, item := range items {
		metric, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if metricName != "" && stringValue(metric["name"]) != metricName {
			continue
		}
		out = append(out, metric)
	}
	if len(out) == 0 {
		return nil, apperrors.WithDetails(apperrors.CodeQueryPlanError, "metric not found in metric_set", map[string]string{
			"domain": metricSet.Domain,
			"name":   metricSet.Name,
			"metric": metricName,
		})
	}
	return out, nil
}

func buildMetricStorageQuery(metricSet model.UModelElement, storage model.UModelElement, dataLinkMapping, storageLinkMapping map[string]any, metrics []map[string]any, entityIDs []string, entityQuery, dataFilter, methodQuery string, entityData *model.EntityData, queryType, step string, limit int) map[string]any {
	switch storage.Kind {
	case "prometheus", "aliyun_prometheus":
		return prometheusMetricQuery(metricSet, storage, dataLinkMapping, storageLinkMapping, metrics, entityIDs, entityQuery, dataFilter, methodQuery, entityData, queryType, step, limit)
	default:
		return map[string]any{
			"dialect":      storage.Kind,
			"metrics":      metricQueryItems(metrics),
			"entity_ids":   entityIDs,
			"entity_data":  entityDataSummary(entityData),
			"entity_query": entityQuery,
			"data_filter":  dataFilter,
			"query":        methodQuery,
			"query_type":   firstNonEmpty(queryType, defaultMetricQueryMode(metrics), stringValue(storage.Spec["default_query_type"])),
			"step":         firstNonEmpty(step, stringValue(storage.Spec["default_step"])),
			"limit":        limit,
		}
	}
}

type prometheusLabelMatcher struct {
	Label    string `json:"label"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

func prometheusMetricQuery(metricSet model.UModelElement, storage model.UModelElement, dataLinkMapping, storageLinkMapping map[string]any, metrics []map[string]any, entityIDs []string, entityQuery, dataFilter, methodQuery string, entityData *model.EntityData, queryType, step string, limit int) map[string]any {
	matchers, rawFilters := prometheusQueryMatchers(storage, dataLinkMapping, storageLinkMapping, entityIDs, entityQuery, dataFilter, methodQuery, entityData)
	queries := []map[string]any{}
	for _, metric := range metrics {
		name := stringValue(metric["name"])
		promQL := firstNonEmpty(stringValue(metric["generator"]), name)
		item := metricQueryItem(metric)
		item["promql"] = renderPromQL(promQL, matchers)
		queries = append(queries, item)
	}
	out := map[string]any{
		"dialect":        "prometheus_promql",
		"endpoint":       storage.Spec["endpoint"],
		"api_prefix":     firstNonEmpty(stringValue(storage.Spec["api_prefix"]), "/api/v1"),
		"query_type":     firstNonEmpty(queryType, defaultMetricQueryMode(metrics), stringValue(storage.Spec["default_query_type"]), "range"),
		"step":           firstNonEmpty(step, stringValue(storage.Spec["default_step"])),
		"lookback_delta": storage.Spec["lookback_delta"],
		"metrics":        metricQueryItems(metrics),
		"queries":        queries,
		"label_matchers": matchers,
		"limit":          limit,
	}
	if len(rawFilters) > 0 {
		out["raw_filters"] = rawFilters
	}
	if tenant := stringValue(storage.Spec["tenant"]); tenant != "" {
		out["tenant"] = tenant
	}
	if tenantHeader := stringValue(storage.Spec["tenant_header"]); tenantHeader != "" {
		out["tenant_header"] = tenantHeader
	}
	if externalLabels, ok := storage.Spec["external_labels"].(map[string]any); ok && len(externalLabels) > 0 {
		out["external_labels"] = externalLabels
	}
	if queryFamily := stringValue(metricSet.Spec["query_type"]); queryFamily != "" {
		out["query_family"] = queryFamily
	}
	return out
}

func metricQueryItems(metrics []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(metrics))
	for _, metric := range metrics {
		out = append(out, metricQueryItem(metric))
	}
	return out
}

func metricQueryItem(metric map[string]any) map[string]any {
	item := map[string]any{
		"name": stringValue(metric["name"]),
	}
	for _, key := range []string{"unit", "data_format", "type", "query_mode", "aggregator", "display_type"} {
		if value := stringValue(metric[key]); value != "" {
			item[key] = value
		}
	}
	if value, ok := metric["golden_metric"].(bool); ok {
		item["golden_metric"] = value
	}
	return item
}

func defaultMetricQueryMode(metrics []map[string]any) string {
	for _, metric := range metrics {
		mode := stringValue(metric["query_mode"])
		if mode != "" && mode != "both" {
			return mode
		}
	}
	for _, metric := range metrics {
		if stringValue(metric["query_mode"]) == "both" {
			return "range"
		}
	}
	return ""
}

func prometheusQueryMatchers(storage model.UModelElement, dataLinkMapping, storageLinkMapping map[string]any, entityIDs []string, entityQuery, dataFilter, methodQuery string, entityData *model.EntityData) ([]prometheusLabelMatcher, []string) {
	matchers := []prometheusLabelMatcher{}
	rawFilters := []string{}
	if idField := mappedStorageField(dataLinkMapping, storageLinkMapping, "id"); idField != "" && len(entityIDs) > 0 {
		matchers = append(matchers, prometheusValuesMatcher(idField, entityIDs, false))
	}
	matchers = append(matchers, entityDataPrometheusMatchers(entityData, dataLinkMapping, storageLinkMapping)...)
	for _, item := range []struct {
		raw    string
		mapper func(string) string
	}{
		{firstNonEmpty(stringValue(storage.Spec["search_filter"]), stringValue(storage.Spec["default_filter"]), stringValue(storage.Spec["query_filter"])), storageFieldMapper()},
		{dataFilter, dataSetToStorageFieldMapper(storageLinkMapping)},
		{entityQuery, entityToStorageFieldMapper(dataLinkMapping, storageLinkMapping)},
		{methodQuery, dataSetToStorageFieldMapper(storageLinkMapping)},
	} {
		filterMatchers, unsupported := prometheusMatchersFromFilter(item.raw, item.mapper)
		matchers = append(matchers, filterMatchers...)
		rawFilters = append(rawFilters, unsupported...)
	}
	return dedupePrometheusMatchers(matchers), rawFilters
}

func prometheusMatchersFromFilter(raw string, fieldMapper func(string) string) ([]prometheusLabelMatcher, []string) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "*" {
		return nil, nil
	}
	expr, err := parseLogFilterExpression(raw)
	if err != nil {
		return nil, []string{raw}
	}
	matchers, ok := logFilterToPrometheusMatchers(expr, fieldMapper)
	if !ok {
		return nil, []string{raw}
	}
	return matchers, nil
}

func logFilterToPrometheusMatchers(node *logFilterNode, fieldMapper func(string) string) ([]prometheusLabelMatcher, bool) {
	if node == nil {
		return nil, true
	}
	switch node.Kind {
	case "and":
		out := []prometheusLabelMatcher{}
		for _, child := range node.Children {
			matchers, ok := logFilterToPrometheusMatchers(child, fieldMapper)
			if !ok {
				return nil, false
			}
			out = append(out, matchers...)
		}
		return out, true
	case "comparison":
		field := node.Field
		if fieldMapper != nil {
			field = fieldMapper(field)
		}
		switch node.Operator {
		case "=", ":", "==":
			return []prometheusLabelMatcher{{Label: field, Operator: "=", Value: stringValue(node.Value)}}, true
		case "!=":
			return []prometheusLabelMatcher{{Label: field, Operator: "!=", Value: stringValue(node.Value)}}, true
		case "in":
			return []prometheusLabelMatcher{prometheusValuesMatcher(field, stringSliceValue(node.Value), false)}, true
		case "not in":
			return []prometheusLabelMatcher{prometheusValuesMatcher(field, stringSliceValue(node.Value), true)}, true
		default:
			return nil, false
		}
	default:
		return nil, false
	}
}

func prometheusValuesMatcher(label string, values []string, negative bool) prometheusLabelMatcher {
	if len(values) == 1 {
		operator := "="
		if negative {
			operator = "!="
		}
		return prometheusLabelMatcher{Label: label, Operator: operator, Value: values[0]}
	}
	escaped := make([]string, 0, len(values))
	for _, value := range values {
		escaped = append(escaped, regexp.QuoteMeta(value))
	}
	operator := "=~"
	if negative {
		operator = "!~"
	}
	return prometheusLabelMatcher{Label: label, Operator: operator, Value: strings.Join(escaped, "|")}
}

func entityDataPrometheusMatchers(entityData *model.EntityData, dataLinkMapping, storageLinkMapping map[string]any) []prometheusLabelMatcher {
	valuesByField := entityDataStorageValues(entityData, dataLinkMapping, storageLinkMapping)
	if len(valuesByField) == 0 {
		return nil
	}
	fields := make([]string, 0, len(valuesByField))
	for field := range valuesByField {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	matchers := make([]prometheusLabelMatcher, 0, len(fields))
	for _, field := range fields {
		matchers = append(matchers, prometheusValuesMatcher(field, valuesByField[field], false))
	}
	return matchers
}

func dedupePrometheusMatchers(matchers []prometheusLabelMatcher) []prometheusLabelMatcher {
	out := []prometheusLabelMatcher{}
	seen := map[string]struct{}{}
	for _, matcher := range matchers {
		if matcher.Label == "" || matcher.Value == "" {
			continue
		}
		key := matcher.Label + "\x00" + matcher.Operator + "\x00" + matcher.Value
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, matcher)
	}
	return out
}

func renderPromQL(promQL string, matchers []prometheusLabelMatcher) string {
	remaining := []prometheusLabelMatcher{}
	for _, matcher := range matchers {
		placeholder := "$" + matcher.Label
		if strings.Contains(promQL, placeholder) {
			if matcher.Operator == "=" {
				promQL = strings.ReplaceAll(promQL, placeholder, escapePromQLStringContent(matcher.Value))
				continue
			}
			pattern := matcher.Label + `="` + placeholder + `"`
			if strings.Contains(promQL, pattern) {
				promQL = strings.ReplaceAll(promQL, pattern, matcher.Label+matcher.Operator+strconv.Quote(matcher.Value))
				continue
			}
		}
		if promQLSelectorHasLabel(promQL, matcher.Label) {
			continue
		}
		remaining = append(remaining, matcher)
	}
	return injectPromQLMatchers(promQL, remaining)
}

func promQLSelectorHasLabel(promQL, label string) bool {
	for _, op := range []string{"=~", "!~", "!=", "="} {
		if strings.Contains(promQL, label+op) {
			return true
		}
	}
	return false
}

func injectPromQLMatchers(promQL string, matchers []prometheusLabelMatcher) string {
	if len(matchers) == 0 {
		return promQL
	}
	matcherText := prometheusMatcherText(matchers)
	open := strings.Index(promQL, "{")
	if open < 0 {
		return promQL
	}
	if open+1 < len(promQL) && promQL[open+1] == '}' {
		return promQL[:open+1] + matcherText + promQL[open+1:]
	}
	return promQL[:open+1] + matcherText + "," + promQL[open+1:]
}

func prometheusMatcherText(matchers []prometheusLabelMatcher) string {
	parts := make([]string, 0, len(matchers))
	for _, matcher := range matchers {
		parts = append(parts, matcher.Label+matcher.Operator+strconv.Quote(matcher.Value))
	}
	return strings.Join(parts, ",")
}

func escapePromQLStringContent(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	return replacer.Replace(value)
}

func buildLogStorageQuery(logSet model.UModelElement, storage model.UModelElement, dataLinkMapping, storageLinkMapping map[string]any, entityIDs []string, entityQuery, dataFilter, methodQuery string, entityData *model.EntityData, limit int) map[string]any {
	switch storage.Kind {
	case "elasticsearch":
		return elasticsearchLogQuery(logSet, storage, dataLinkMapping, storageLinkMapping, entityIDs, entityQuery, dataFilter, methodQuery, entityData, limit)
	default:
		return map[string]any{
			"dialect":      storage.Kind,
			"entity_ids":   entityIDs,
			"entity_data":  entityDataSummary(entityData),
			"entity_query": entityQuery,
			"data_filter":  dataFilter,
			"query":        methodQuery,
			"limit":        limit,
		}
	}
}

func elasticsearchLogQuery(logSet model.UModelElement, storage model.UModelElement, dataLinkMapping, storageLinkMapping map[string]any, entityIDs []string, entityQuery, dataFilter, methodQuery string, entityData *model.EntityData, limit int) map[string]any {
	dataSetMapper := dataSetToStorageFieldMapper(storageLinkMapping)
	timeField := stringValue(storage.Spec["time_field"])
	if timeField == "" {
		timeField = dataSetMapper(firstNonEmpty(stringValue(logSet.Spec["time_field"]), "timestamp"))
	}
	size := intValue(storage.Spec["default_size"])
	if limit > 0 && (size == 0 || limit < size) {
		size = limit
	}
	if size <= 0 {
		size = 1000
	}

	filters := []map[string]any{}
	entityMapper := entityToStorageFieldMapper(dataLinkMapping, storageLinkMapping)
	storageMapper := storageFieldMapper()
	idField := mappedStorageField(dataLinkMapping, storageLinkMapping, "id")
	if idField != "" && len(entityIDs) > 0 {
		if len(entityIDs) == 1 {
			filters = append(filters, map[string]any{"term": map[string]any{idField: entityIDs[0]}})
		} else {
			filters = append(filters, map[string]any{"terms": map[string]any{idField: entityIDs}})
		}
	}
	if entityFilter := entityDataElasticsearchFilter(entityData, dataLinkMapping, storageLinkMapping); entityFilter != nil {
		filters = append(filters, entityFilter)
	}
	filters = appendLogQueryFilter(filters, firstNonEmpty(stringValue(storage.Spec["search_filter"]), stringValue(storage.Spec["default_filter"]), stringValue(storage.Spec["query_filter"])), storageMapper)
	filters = appendLogQueryFilter(filters, dataFilter, dataSetMapper)
	filters = appendLogQueryFilter(filters, entityQuery, entityMapper)
	filters = appendLogQueryFilter(filters, methodQuery, dataSetMapper)

	query := map[string]any{"match_all": map[string]any{}}
	if len(filters) > 0 {
		query = map[string]any{"bool": map[string]any{"filter": filters}}
	}
	body := map[string]any{
		"size":  size,
		"query": query,
		"sort":  []map[string]any{{timeField: map[string]any{"order": firstNonEmpty(stringValue(logSet.Spec["default_order"]), "desc")}}},
	}
	if fields := mappedLogOutputFields(logSet, storageLinkMapping); len(fields) > 0 {
		body["_source"] = fields
	}
	return map[string]any{
		"dialect": "elasticsearch_dsl",
		"index":   storage.Spec["index"],
		"body":    body,
	}
}

func mappedStorageField(dataLinkMapping, storageLinkMapping map[string]any, entityField string) string {
	dataSetField := stringValue(dataLinkMapping[entityField])
	if dataSetField == "" {
		dataSetField = entityField
	}
	storageField := stringValue(storageLinkMapping[dataSetField])
	if storageField == "" {
		return dataSetField
	}
	return storageField
}

func mappedStorageFieldForEntityData(dataLinkMapping, storageLinkMapping map[string]any, entityField string) string {
	if stringValue(dataLinkMapping[entityField]) == "" {
		return ""
	}
	return mappedStorageField(dataLinkMapping, storageLinkMapping, entityField)
}

func entityDataStorageValues(entityData *model.EntityData, dataLinkMapping, storageLinkMapping map[string]any) map[string][]string {
	if entityData == nil || entityData.Empty() {
		return nil
	}
	valuesByField := map[string]map[string]struct{}{}
	for idx, entityField := range entityData.Header {
		storageField := mappedStorageFieldForEntityData(dataLinkMapping, storageLinkMapping, entityField)
		if storageField == "" {
			continue
		}
		if _, ok := valuesByField[storageField]; !ok {
			valuesByField[storageField] = map[string]struct{}{}
		}
		for _, row := range entityData.Data {
			if idx >= len(row) || row[idx] == "" {
				continue
			}
			valuesByField[storageField][row[idx]] = struct{}{}
		}
	}
	out := make(map[string][]string, len(valuesByField))
	for field, set := range valuesByField {
		values := make([]string, 0, len(set))
		for value := range set {
			values = append(values, value)
		}
		sort.Strings(values)
		out[field] = values
	}
	return out
}

func entityDataElasticsearchFilter(entityData *model.EntityData, dataLinkMapping, storageLinkMapping map[string]any) map[string]any {
	if entityData == nil || entityData.Empty() {
		return nil
	}
	fieldMappings := entityDataFieldMappings(entityData.Header, dataLinkMapping, storageLinkMapping)
	if len(fieldMappings) == 0 {
		return nil
	}
	rowFilters := []map[string]any{}
	for _, row := range entityData.Data {
		fieldFilters := []map[string]any{}
		for idx, storageField := range fieldMappings {
			if idx >= len(row) || row[idx] == "" {
				continue
			}
			fieldFilters = append(fieldFilters, map[string]any{"term": map[string]any{storageField: row[idx]}})
		}
		if len(fieldFilters) == 0 {
			continue
		}
		if len(fieldFilters) == 1 {
			rowFilters = append(rowFilters, fieldFilters[0])
			continue
		}
		rowFilters = append(rowFilters, map[string]any{"bool": map[string]any{"filter": fieldFilters}})
	}
	if len(rowFilters) == 0 {
		return nil
	}
	if len(rowFilters) == 1 {
		return rowFilters[0]
	}
	return map[string]any{"bool": map[string]any{"should": rowFilters, "minimum_should_match": 1}}
}

func entityDataFieldMappings(header []string, dataLinkMapping, storageLinkMapping map[string]any) map[int]string {
	out := map[int]string{}
	for idx, entityField := range header {
		if storageField := mappedStorageFieldForEntityData(dataLinkMapping, storageLinkMapping, entityField); storageField != "" {
			out[idx] = storageField
		}
	}
	return out
}

func entityDataSummary(entityData *model.EntityData) map[string]any {
	if entityData == nil || entityData.Empty() {
		return nil
	}
	return map[string]any{
		"header": entityData.Header,
		"rows":   len(entityData.Data),
	}
}

func filterByEntityAllows(raw string, entityData *model.EntityData) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || entityData == nil || entityData.Empty() {
		return true
	}
	expr, err := parseLogFilterExpression(raw)
	if err != nil {
		return false
	}
	for _, row := range entityData.ToArrayMap() {
		if evalFilterByEntity(expr, row) {
			return true
		}
	}
	return false
}

func filterByEntityExpression(spec map[string]any) string {
	for _, key := range []string{"filter_by_entity", "filterByEntity", "FilterByEntity"} {
		if raw := strings.TrimSpace(stringValue(spec[key])); raw != "" {
			return raw
		}
	}
	src := mapValue(spec["src"])
	for _, key := range []string{"filter", "Filter"} {
		if raw := strings.TrimSpace(stringValue(src[key])); raw != "" {
			return raw
		}
	}
	return ""
}

func evalFilterByEntity(node *logFilterNode, row map[string]string) bool {
	if node == nil {
		return true
	}
	switch node.Kind {
	case "and":
		for _, child := range node.Children {
			if !evalFilterByEntity(child, row) {
				return false
			}
		}
		return true
	case "or":
		for _, child := range node.Children {
			if evalFilterByEntity(child, row) {
				return true
			}
		}
		return false
	case "not":
		return len(node.Children) == 0 || !evalFilterByEntity(node.Children[0], row)
	case "comparison":
		return evalFilterByEntityComparison(node, row)
	default:
		return false
	}
}

func evalFilterByEntityComparison(node *logFilterNode, row map[string]string) bool {
	value, ok := row[node.Field]
	if !ok {
		return false
	}
	expected := stringValue(node.Value)
	switch node.Operator {
	case "=", "==", ":":
		return value == expected
	case "!=":
		return value != expected
	case "in":
		return containsString(stringSliceValue(node.Value), value)
	case "not in":
		return !containsString(stringSliceValue(node.Value), value)
	default:
		return false
	}
}

func appendLogQueryFilter(filters []map[string]any, raw string, fieldMapper func(string) string) []map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "*" {
		return filters
	}
	expr, err := parseLogFilterExpression(raw)
	if err != nil {
		return append(filters, map[string]any{"query_string": map[string]any{"query": raw}})
	}
	return append(filters, logFilterToElasticsearch(expr, fieldMapper))
}

func entityToStorageFieldMapper(dataLinkMapping, storageLinkMapping map[string]any) func(string) string {
	return func(field string) string {
		return mappedStorageField(dataLinkMapping, storageLinkMapping, field)
	}
}

func dataSetToStorageFieldMapper(storageLinkMapping map[string]any) func(string) string {
	return func(field string) string {
		if mapped := stringValue(storageLinkMapping[field]); mapped != "" {
			return mapped
		}
		return field
	}
}

func storageFieldMapper() func(string) string {
	return func(field string) string {
		return field
	}
}

func mappedLogOutputFields(logSet model.UModelElement, storageLinkMapping map[string]any) []string {
	fields, ok := logSet.Spec["fields"].([]any)
	if !ok {
		return []string{}
	}
	mapper := dataSetToStorageFieldMapper(storageLinkMapping)
	out := []string{}
	seen := map[string]struct{}{}
	for _, field := range fields {
		item, ok := field.(map[string]any)
		if !ok {
			continue
		}
		name := stringValue(item["name"])
		if name == "" {
			continue
		}
		mapped := mapper(name)
		if _, exists := seen[mapped]; exists {
			continue
		}
		seen[mapped] = struct{}{}
		out = append(out, mapped)
	}
	return out
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
	if element.Kind == "metric_set" {
		labels := mapValue(element.Spec["labels"])
		keys, ok := labels["keys"].([]any)
		if !ok {
			return []string{}
		}
		out := []string{}
		for _, field := range keys {
			item, ok := field.(map[string]any)
			if !ok {
				continue
			}
			if name := stringValue(item["name"]); name != "" {
				out = append(out, name)
			}
		}
		return out
	}
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

func dataSetFields(element model.UModelElement) any {
	if element.Kind == "metric_set" {
		metrics, ok := element.Spec["metrics"].([]any)
		if !ok {
			return nil
		}
		out := make([]map[string]any, 0, len(metrics))
		for _, metric := range metrics {
			item, ok := metric.(map[string]any)
			if !ok {
				continue
			}
			field := map[string]any{
				"name": stringValue(item["name"]),
				"type": "metric",
			}
			copyFieldInfo(field, item)
			out = append(out, field)
		}
		return out
	}
	fields, ok := element.Spec["fields"].([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(fields))
	for _, raw := range fields {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		field := map[string]any{
			"name": stringValue(item["name"]),
			"type": stringValue(item["type"]),
		}
		copyFieldInfo(field, item)
		out = append(out, field)
	}
	return out
}

func copyFieldInfo(dst, src map[string]any) {
	for _, key := range []string{"display_name", "description", "data_format", "unit"} {
		if value, ok := src[key]; ok && stringValue(value) != "" {
			dst[key] = value
		}
	}
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

func dataSetTypeSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return stringSet([]string{"metric_set", "log_set", "trace_set", "event_set", "profile_set"})
	}
	return stringSet(values)
}

func dataSetTypeAllowed(typeFilter map[string]struct{}, kind string) bool {
	_, ok := typeFilter[kind]
	return ok
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

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case int32:
		return int(typed)
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case string:
		n, err := strconv.Atoi(typed)
		if err == nil {
			return n
		}
	}
	return 0
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
