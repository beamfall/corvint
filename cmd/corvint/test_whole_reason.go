package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/Beamfall/corvint/internal/sessiontest"
)

// testWholeReasons prints, on stderr, one ready HEAVY_WHOLE_REASON line for
// each planned go test that runs packages whole (no -run). script/heavy.sh
// refuses such a run without a reason (ASD-V0-030); the reason here is the
// plan's own, so a lane copying the plan into heavy.sh needs no hand-written
// one, while a hand-typed whole run is still refused (STR-V0-073, V1-3120).
// stdout's JSON plan is unchanged.
func testWholeReasons(planned []sessiontest.Planned, fallback string, stderr io.Writer) {
	for _, invocation := range planned {
		if !wholeGoTest(invocation.Argv) {
			continue
		}
		reasons, seen := []string{}, map[string]bool{}
		for _, unit := range invocation.Units {
			reason := unit.Reason
			if reason == "" {
				reason = fallback
			}
			if reason != "" && !seen[reason] {
				seen[reason] = true
				reasons = append(reasons, reason)
			}
		}
		units := make([]string, len(invocation.Units))
		for index, unit := range invocation.Units {
			units[index] = strings.TrimPrefix(unit.Unit, "go:")
		}
		named := strings.Join(units, " ")
		if len(units) > 3 {
			named = fmt.Sprintf("%d packages", len(units))
		}
		reason := "corvint test selected " + named + " whole"
		if len(reasons) != 0 {
			reason += ": " + strings.Join(reasons, "; ")
		}
		_, _ = fmt.Fprintf(stderr, "corvint-test: whole-package go test planned; under script/heavy.sh run it with HEAVY_WHOLE_REASON=%s (STR-V0-073)\n", shellQuoteJoin([]string{reason}))
	}
}

// wholeGoTest reports whether command is a go test without a -run selection. It names its
// parameter command, not argv, so the command-tree scanner (CAP-V0-047) does not read these
// go-toolchain words as corvint dispatch.
func wholeGoTest(command []string) bool {
	if len(command) < 2 || command[0] != "go" || command[1] != "test" {
		return false
	}
	for _, word := range command[2:] {
		if word == "-run" || strings.HasPrefix(word, "-run=") {
			return false
		}
	}
	return true
}
