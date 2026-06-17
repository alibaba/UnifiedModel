package query

import (
	"testing"

	"github.com/alibaba/UnifiedModel/pkg/model"
)

// FuzzParse hardens the SPL parser, an untrusted-input surface. The invariant is
// simple but important: Parse must never panic on arbitrary input — it may
// return an error, but it must not crash the server. The seed corpus runs on
// every `go test`; `go test -fuzz=FuzzParse` explores further.
func FuzzParse(f *testing.F) {
	seeds := []string{
		"",
		".entity with(domain='devops', name='devops.service')",
		".entity with(domain='a', name='b', query='x', topk=5, mode='vector')",
		".umodel with(kind='entity_set') | project domain,name | sort domain | limit 10",
		".entity_set with(domain='d', name='n', ids=['1']) | entity-call get_metrics('d','m','x', step='30s')",
		".topo | graph-call getNeighborNodes('full', 2, [(:\"d@n\" {__entity_id__: '1'})]) | limit 5",
		".topo | graph-call cypher(`MATCH (n) RETURN n LIMIT 1`)",
		"not a query",
		".entity with(",
		".entity with(domain='x'",
		".topo | graph-call getNeighborNodes(",
		"| | |",
		".entity with(domain='a', name='b') | project | sort | limit",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, query string) {
		// Must not panic. An error return is acceptable.
		_, _ = Parse(model.QueryRequest{Query: query})
	})
}
