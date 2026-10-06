package main

import "testing"

// guidancePrefilterHolds fails when guidanceMayMatch skips a line its rule's
// pattern matches, which would drop a feature from the receipt.
func guidancePrefilterHolds(t testing.TB, line string) {
	for _, rule := range []struct {
		id      string
		matches bool
	}{
		{"literal-marker", guidanceMarkers.MatchString(line)},
		{"literal-registration", guidanceRegistrations.MatchString(line)},
	} {
		if rule.matches && !guidanceMayMatch(rule.id, line) {
			t.Fatalf("%s prefilter skips a matching line %q", rule.id, line)
		}
	}
}

var guidancePrefilterSeeds = []string{
	"// Feature: cache demux", "# SCENARIO : retry backoff", "<!-- feature: docs -->", "/** Scenario: x */",
	"ſcenario: long s folds to s", "FEATURE:x", "feature:", "no colon feature here",
	`mux.HandleFunc("/v1/items", h)`, `r.Get( '/health' )`, `app.post("x")`, `server.AddTool("search", fn)`,
	`register_tool("a")`, `registerTool('b')`, `route("/")`, `tool("t")`, `Delete ("z")`, `patch("p")`,
	`fmt.Println("no registration")`, `Handle(x)`, `get()`, "",
}

// The prefilter skips no seed either pattern matches; the seeds exercise
// case folding and every registration name, and the fuzz target widens them.
func TestGuidancePrefilterIsANecessaryCondition(t *testing.T) {
	matched := 0
	for _, line := range guidancePrefilterSeeds {
		guidancePrefilterHolds(t, line)
		if guidanceMarkers.MatchString(line) || guidanceRegistrations.MatchString(line) {
			matched++
		}
	}
	if matched < 15 {
		t.Fatalf("only %d seeds match a pattern", matched)
	}
}

func FuzzGuidancePrefilter(f *testing.F) {
	for _, line := range guidancePrefilterSeeds {
		f.Add(line)
	}
	f.Fuzz(func(t *testing.T, line string) { guidancePrefilterHolds(t, line) })
}
