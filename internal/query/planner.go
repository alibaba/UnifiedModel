package query

import (
	"fmt"
	"strings"

	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

type Planner struct{}

func (Planner) Plan(req model.QueryRequest, caps model.GraphStoreCapabilities) (model.QueryPlan, error) {
	plan, err := Parse(req)
	if err != nil {
		return model.QueryPlan{}, err
	}
	plan, err = validatePlan(plan, caps)
	if err != nil {
		return model.QueryPlan{}, err
	}
	return plan, nil
}

func validatePlan(plan model.QueryPlan, caps model.GraphStoreCapabilities) (model.QueryPlan, error) {
	maxLimit := caps.MaxLimit
	if maxLimit <= 0 {
		maxLimit = 1000
	}
	if plan.Limit > maxLimit {
		return model.QueryPlan{}, apperrors.WithDetails(apperrors.CodeValidationFailed, "query limit exceeds provider capability", map[string]string{
			"limit":     fmt.Sprint(plan.Limit),
			"max_limit": fmt.Sprint(maxLimit),
		})
	}
	if plan.TopK > maxLimit {
		return model.QueryPlan{}, apperrors.WithDetails(apperrors.CodeValidationFailed, "query topk exceeds provider capability", map[string]string{
			"topk":      fmt.Sprint(plan.TopK),
			"max_limit": fmt.Sprint(maxLimit),
		})
	}

	maxDepth := caps.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 1
	}
	if plan.Depth > maxDepth {
		return model.QueryPlan{}, apperrors.WithDetails(apperrors.CodeValidationFailed, "query depth exceeds provider capability", map[string]string{
			"depth":     fmt.Sprint(plan.Depth),
			"max_depth": fmt.Sprint(maxDepth),
		})
	}

	for idx, operator := range plan.Operators {
		switch {
		case strings.HasPrefix(operator, "entity-call:"):
			if plan.Source != ".entity_set" {
				return model.QueryPlan{}, apperrors.New(apperrors.CodeQueryPlanError, "entity-call is only supported for .entity_set")
			}
			if plan.EntityCall == nil || plan.EntityCall.Name == "" {
				return model.QueryPlan{}, apperrors.New(apperrors.CodeQueryPlanError, "entity-call requires a method name")
			}
			entityCall, err := normalizeEntityCall(*plan.EntityCall)
			if err != nil {
				return model.QueryPlan{}, err
			}
			plan.EntityCall = &entityCall
			if idx < len(plan.Pipeline) && plan.Pipeline[idx].EntityCall != nil {
				plan.Pipeline[idx].EntityCall = &entityCall
			}
		case operator == "graph-match" && !caps.GraphMatch:
			return model.QueryPlan{}, apperrors.New(apperrors.CodeProviderUnsupported, "graph-match is not supported by provider")
		case strings.HasPrefix(operator, "graph-call:"):
			if plan.Source != ".topo" {
				return model.QueryPlan{}, apperrors.New(apperrors.CodeQueryPlanError, "graph-call is only supported for .topo")
			}
			if plan.GraphCall == nil || !isAllowedGraphCall(plan.GraphCall.Name) {
				return model.QueryPlan{}, apperrors.WithDetails(apperrors.CodeQueryPlanError, "unsupported graph-call", map[string]string{"name": graphCallName(plan.GraphCall)})
			}
			if plan.GraphCall.Name == "cypher" {
				if !caps.ControlledCypher {
					return model.QueryPlan{}, apperrors.New(apperrors.CodeProviderUnsupported, "controlled cypher is not supported by provider")
				}
				continue
			}
			if !caps.GraphCallNeighbors {
				return model.QueryPlan{}, apperrors.New(apperrors.CodeProviderUnsupported, "graph-call is not supported by provider")
			}
		}
	}

	if plan.Source == ".entity_set" && plan.EntityCall == nil {
		return model.QueryPlan{}, apperrors.New(apperrors.CodeQueryPlanError, ".entity_set requires entity-call")
	}

	return plan, nil
}

func isAllowedGraphCall(name string) bool {
	return name == "getNeighborNodes" || name == "getDirectRelations" || name == "cypher"
}

type entityCallMethodSpec struct {
	Name   string
	Params []model.EntityCallParam
}

func normalizeEntityCall(call model.EntityCallPlan) (model.EntityCallPlan, error) {
	spec, ok := entityCallMethodSpecFor(call.Name)
	if !ok {
		return model.EntityCallPlan{}, apperrors.WithDetails(apperrors.CodeQueryPlanError, "unsupported entity-call method", map[string]string{"name": call.Name})
	}
	if len(call.Arguments) > len(spec.Params) {
		return model.EntityCallPlan{}, apperrors.WithDetails(apperrors.CodeQueryPlanError, "entity-call has too many arguments", map[string]string{
			"method": call.Name,
			"max":    fmt.Sprint(len(spec.Params)),
		})
	}

	paramIndex := map[string]int{}
	for idx, param := range spec.Params {
		paramIndex[param.Key] = idx
	}
	for key := range call.NamedArguments {
		if _, exists := paramIndex[key]; !exists {
			return model.EntityCallPlan{}, apperrors.WithDetails(apperrors.CodeQueryPlanError, "entity-call named argument is not supported", map[string]string{
				"method":   call.Name,
				"argument": key,
			})
		}
	}

	parameters := make(map[string]any, len(spec.Params))
	for idx, value := range call.Arguments {
		param := spec.Params[idx]
		if _, exists := call.NamedArguments[param.Key]; exists {
			return model.EntityCallPlan{}, apperrors.WithDetails(apperrors.CodeQueryPlanError, "entity-call argument is provided twice", map[string]string{
				"method":   call.Name,
				"argument": param.Key,
			})
		}
		if err := validateEntityCallArgument(call.Name, param, value); err != nil {
			return model.EntityCallPlan{}, err
		}
		parameters[param.Key] = value
	}
	for key, value := range call.NamedArguments {
		param := spec.Params[paramIndex[key]]
		if err := validateEntityCallArgument(call.Name, param, value); err != nil {
			return model.EntityCallPlan{}, err
		}
		parameters[param.Key] = value
	}
	for _, param := range spec.Params {
		if _, exists := parameters[param.Key]; exists {
			continue
		}
		if param.Required {
			return model.EntityCallPlan{}, apperrors.WithDetails(apperrors.CodeQueryPlanError, "entity-call required argument is missing", map[string]string{
				"method":   call.Name,
				"argument": param.Key,
			})
		}
		parameters[param.Key] = param.Default
	}

	call.Signature = append([]model.EntityCallParam(nil), spec.Params...)
	call.Parameters = parameters
	call.Name = spec.Name
	return call, nil
}

func entityCallMethodSpecFor(name string) (entityCallMethodSpec, bool) {
	switch name {
	case "__list_method__":
		return entityCallMethodSpec{Name: "__list_method__"}, true
	case "list_dataset", "list_data_set":
		return entityCallMethodSpec{
			Name: "list_data_set",
			Params: []model.EntityCallParam{
				{Key: "data_set_types", Type: "array<varchar>", DisplayName: "Data Set Types, metric_set, log_set, trace_set, event_set"},
				{Key: "detail", Type: "boolean", DisplayName: "Detail Info, if true, return all fields of DataSet", Default: false},
			},
		}, true
	case "get_log", "get_logs":
		return entityCallMethodSpec{
			Name: "get_logs",
			Params: []model.EntityCallParam{
				{Key: "domain", Type: "varchar", DisplayName: "log_set Domain", Required: true},
				{Key: "name", Type: "varchar", DisplayName: "log_set Name", Required: true},
				{
					Key:         "query",
					Type:        "varchar",
					DisplayName: "Query expression for the log set",
					Description: "Basic SPL where syntax, for example service_id = 'service_a' and level in ['ERROR', 'WARN'].",
				},
				{Key: "storage_domain", Type: "varchar", DisplayName: "Storage Domain", Description: "Optional storage domain used to select a specific StorageLink target. Echoed into the plan; not consumed by the open-source planner."},
				{Key: "storage_name", Type: "varchar", DisplayName: "Storage Name", Description: "Optional storage name used to select a specific StorageLink target. Echoed into the plan; not consumed by the open-source planner."},
				{Key: "storage_kind", Type: "varchar", DisplayName: "Storage Kind", Description: "Optional storage kind used to select a specific StorageLink target. Echoed into the plan; not consumed by the open-source planner."},
			},
		}, true
	case "get_metric", "get_metrics":
		return entityCallMethodSpec{
			Name: "get_metrics",
			Params: []model.EntityCallParam{
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
				{Key: "aggregate", Type: "boolean", DisplayName: "Aggregate time series", Description: "Whether to aggregate the time series. Echoed into the plan; not consumed by the open-source planner.", Default: true},
				{Key: "storage_domain", Type: "varchar", DisplayName: "Storage Domain", Description: "Optional storage domain used to select a specific StorageLink target. Echoed into the plan; not consumed by the open-source planner."},
				{Key: "storage_name", Type: "varchar", DisplayName: "Storage Name", Description: "Optional storage name used to select a specific StorageLink target. Echoed into the plan; not consumed by the open-source planner."},
				{Key: "storage_kind", Type: "varchar", DisplayName: "Storage Kind", Description: "Optional storage kind used to select a specific StorageLink target. Echoed into the plan; not consumed by the open-source planner."},
			},
		}, true
	default:
		return entityCallMethodSpec{}, false
	}
}

func validateEntityCallArgument(method string, param model.EntityCallParam, value any) error {
	switch param.Type {
	case "varchar":
		if _, ok := value.(string); ok {
			return nil
		}
	case "boolean":
		if _, ok := value.(bool); ok {
			return nil
		}
	case "array<varchar>":
		if _, ok := value.([]string); ok {
			return nil
		}
		if values, ok := value.([]any); ok {
			for _, item := range values {
				if _, ok := item.(string); !ok {
					return entityCallTypeError(method, param, value)
				}
			}
			return nil
		}
	default:
		return nil
	}
	return entityCallTypeError(method, param, value)
}

func entityCallTypeError(method string, param model.EntityCallParam, value any) error {
	return apperrors.WithDetails(apperrors.CodeQueryPlanError, "entity-call argument has invalid type", map[string]string{
		"method":   method,
		"argument": param.Key,
		"expected": param.Type,
		"actual":   fmt.Sprintf("%T", value),
	})
}

func graphCallName(call *model.GraphCallPlan) string {
	if call == nil {
		return ""
	}
	return call.Name
}
