package testsupport

import "testing"

// CORVINT_FORCE_TIMING_TESTS=0 must mean off, matching unset, not "any
// non-empty value forces" (owner ruling 2026-09-13).
func TestForceTimingTestsRequestedTreatsZeroAsOff(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{"", false},
		{"0", false},
		{"1", true},
		{"true", false},
	} {
		t.Setenv("CORVINT_FORCE_TIMING_TESTS", tc.value)
		if got := forceTimingTestsRequested(); got != tc.want {
			t.Fatalf("value=%q got=%v want=%v", tc.value, got, tc.want)
		}
	}
}
