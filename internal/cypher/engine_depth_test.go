package cypher

import "testing"

// TestReadDepthCaps guards the variable-length path depth (`*min..max`) against
// unbounded / overflowing bounds that previously bypassed the depth limit.
func TestReadDepthCaps(t *testing.T) {
	// Explicit in-range bounds pass unchanged.
	if mn, mx, _, err := readDepth("1..3"); err != nil || mn != 1 || mx != 3 {
		t.Fatalf(`readDepth("1..3") = %d..%d, err=%v; want 1..3`, mn, mx, err)
	}
	// An open-ended range is clamped to the ceiling, not rejected.
	if mn, mx, _, err := readDepth("1.."); err != nil || mn != 1 || mx != maxCypherPathDepth {
		t.Fatalf(`readDepth("1..") = %d..%d, err=%v; want 1..%d`, mn, mx, err, maxCypherPathDepth)
	}
	// A single bound at the ceiling passes; just above it is rejected.
	if _, _, _, err := readDepth("16"); err != nil {
		t.Fatalf(`readDepth("16") err=%v; want nil`, err)
	}
	// Explicit over-cap bounds and integer overflow are rejected.
	for _, in := range []string{"17", "20..30", "1..100", "1..99999999999999999999999", "99999999999999999999999"} {
		if _, _, _, err := readDepth(in); err == nil {
			t.Fatalf("readDepth(%q) should reject an over-cap depth, got nil error", in)
		}
	}
}
