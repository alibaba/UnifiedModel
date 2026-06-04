package query

import (
	"strings"

	"github.com/alibaba/UnifiedModel/pkg/model"
)

func buildExplain(plan model.QueryPlan, caps model.GraphStoreCapabilities, health model.GraphStoreHealth) model.QueryExplain {
	provider := health.Provider
	explain := model.QueryExplain{
		Source:           plan.Source,
		Provider:         provider,
		StorageProvider:  provider,
		EntityCall:       plan.EntityCall,
		Pushdown:         pushdownForPlan(plan, caps),
		Fallback:         fallbackForPlan(plan, caps),
		Operators:        append([]string(nil), plan.Operators...),
		Depth:            plan.Depth,
		Limit:            plan.Limit,
		TimeoutMS:        plan.TimeoutMS,
		TimeRangeApplied: plan.TimeRange.From != nil || plan.TimeRange.To != nil,
	}
	if plan.GraphCall != nil && plan.GraphCall.Name == "cypher" && caps.ControlledCypher {
		explain.CypherDialect = "ladybug"
		explain.CypherEngine = "go"
	}
	return explain
}

func pushdownForPlan(plan model.QueryPlan, caps model.GraphStoreCapabilities) []string {
	pushdown := []string{}
	switch plan.Source {
	case ".entity":
		if caps.EntitySearch {
			pushdown = append(pushdown, "entity_search")
		}
	case ".topo":
		if plan.GraphCall != nil {
			if plan.GraphCall.Name == "cypher" && caps.ControlledCypher {
				pushdown = append(pushdown, "graph_call:cypher", "controlled_cypher")
			} else if caps.GraphCallNeighbors {
				pushdown = append(pushdown, "graph_call:"+plan.GraphCall.Name)
			}
		} else if containsString(plan.Operators, "graph-match") && caps.GraphMatch {
			pushdown = append(pushdown, "graph_match")
		}
	}
	return pushdown
}

func fallbackForPlan(plan model.QueryPlan, caps model.GraphStoreCapabilities) []string {
	fallback := []string{}
	if plan.Source == ".umodel" {
		fallback = append(fallback, "snapshot_filter")
	}
	if plan.Source == ".entity_set" {
		fallback = append(fallback, "entity_call_plan")
	}
	if plan.Source == ".entity" && !caps.ServerSideFilter {
		fallback = append(fallback, "application_filter")
	}
	for _, operator := range plan.Operators {
		switch {
		case operator == "where" && !containsString(fallback, "application_filter"):
			fallback = append(fallback, "application_filter")
		case operator == "project":
			fallback = append(fallback, "application_project")
		case operator == "sort":
			fallback = append(fallback, "application_sort")
		case strings.HasPrefix(operator, "graph-call:") && plan.GraphCall != nil && plan.GraphCall.Name == "cypher" && !caps.ControlledCypher:
			fallback = append(fallback, "unsupported_controlled_cypher")
		case strings.HasPrefix(operator, "graph-call:") && plan.GraphCall != nil && plan.GraphCall.Name != "cypher" && !caps.GraphCallNeighbors:
			fallback = append(fallback, "unsupported_graph_call")
		}
	}
	return fallback
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
