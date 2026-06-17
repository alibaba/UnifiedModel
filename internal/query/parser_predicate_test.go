package query

import "testing"

// TestParsePredicateQuoteAware guards the `where` operator scan against quoted
// values that contain operator characters — previously `strings.Index` matched
// the operator inside the quotes and produced a wrong predicate.
func TestParsePredicateQuoteAware(t *testing.T) {
	p, err := parsePredicate(`path = "a<=b>=c"`)
	if err != nil {
		t.Fatalf("parsePredicate: %v", err)
	}
	if p.Field != "path" || p.Op != "=" {
		t.Fatalf("quoted value: got field=%q op=%q, want path / =", p.Field, p.Op)
	}
	if s, ok := p.Value.(string); !ok || s != "a<=b>=c" {
		t.Fatalf("quoted value: got value=%#v, want \"a<=b>=c\"", p.Value)
	}

	// Normal predicates still resolve to the correct (longest-match) operator.
	cases := []struct{ expr, field, op string }{
		{`status = active`, "status", "="},
		{`status != active`, "status", "!="},
		{`count >= 5`, "count", ">="},
		{`count <= 5`, "count", "<="},
		{`a == b`, "a", "="}, // == normalizes to =
		{`x ~ y`, "x", "~"},
	}
	for _, c := range cases {
		got, err := parsePredicate(c.expr)
		if err != nil {
			t.Fatalf("parsePredicate(%q): %v", c.expr, err)
		}
		if got.Field != c.field || got.Op != c.op {
			t.Fatalf("parsePredicate(%q): got field=%q op=%q, want %q / %q", c.expr, got.Field, got.Op, c.field, c.op)
		}
	}
}
