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

// TestReadDepthRejectsExplicitZeroUpperBound distinguishes an absent upper bound
// (`*1..`, open-ended → clamp to the ceiling) from an explicit zero upper bound
// (`*1..0`). An explicit zero is an empty/invalid range and must be rejected, not
// silently widened to the ceiling.
func TestReadDepthRejectsExplicitZeroUpperBound(t *testing.T) {
	for _, in := range []string{"1..0", "2..0", "..0"} {
		if mn, mx, _, err := readDepth(in); err == nil {
			t.Errorf("readDepth(%q) should reject an explicit zero upper bound, got %d..%d nil", in, mn, mx)
		}
	}
	// A bare "0" (single bound, no range) keeps its existing 1..1 normalization.
	if mn, mx, _, err := readDepth("0"); err != nil || mn != 1 || mx != 1 {
		t.Fatalf(`readDepth("0") = %d..%d, err=%v; want 1..1`, mn, mx, err)
	}
}
