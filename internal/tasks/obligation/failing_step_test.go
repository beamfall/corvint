package obligation

import (
	"regexp"
	"testing"
)

// TestTCDV0800_FailingLineSkipsMakeWrapper reproduces V1-3940: a nested make
// failure ends in make's own wrapper lines, which are actionable by the
// default pattern but name only the target. failingLine must be the step
// runner's FAIL line; a log with only wrapper lines falls back to the last
// line.
func TestTCDV0800_FailingLineSkipsMakeWrapper(t *testing.T) {
	log := "go test ./internal/gate\n" +
		"FAIL pinned FFmpeg tone-map conformance\n" +
		"rerun: make conformance-tonemap; log at out/tonemap.log\n" +
		"make[2]: *** [Makefile:41: conformance] Error 1\n" +
		"make[1]: Leaving directory '/src/core'\n" +
		"make[1]: *** [Makefile:12: test] Error 2\n" +
		"make: *** [gate] Error 2\n"
	if _, failing := FailureTail(log, 20); failing != "FAIL pinned FFmpeg tone-map conformance" {
		t.Fatalf("failing %q", failing)
	}
	if _, failing := FailureTail("building\nmake: *** [gate] Error 1\n", 20); failing != "make: *** [gate] Error 1" {
		t.Fatalf("wrapper-only failing %q", failing)
	}
}

// TestTCDV0801_FailurePatternPicksTheLine: a policy failurePattern picks the
// last tail line it matches over the default choice; a pattern that matches
// nothing keeps the default choice.
func TestTCDV0801_FailurePatternPicksTheLine(t *testing.T) {
	log := "step conformance: BAD frame 12\nFAIL summary 1 of 9\nmake: *** [gate] Error 2\n"
	if _, failing := FailureTailMatching(log, 20, regexp.MustCompile(`^step [a-z]+: BAD`)); failing != "step conformance: BAD frame 12" {
		t.Fatalf("patterned failing %q", failing)
	}
	if _, failing := FailureTailMatching(log, 20, regexp.MustCompile(`NEVER`)); failing != "FAIL summary 1 of 9" {
		t.Fatalf("unmatched pattern failing %q", failing)
	}
}
