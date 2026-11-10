// SPDX-License-Identifier: Apache-2.0
package forwardrow

import "testing"

// noPlanFallback turns selection D into the shadow tool's no-plan FULL fallback (V1-5216): an
// affected graph the planner cannot compile leaves no plan digest and only the declared whole
// Go suite, which selection.jq does not list as go: units.
func noPlanFallback(d doc) {
	d["scope"], d["reason"], d["planDigest"], d["selected"] = "FULL",
		"no plan: unsupported-affected-graph: the graph is not supported", "", []any{}
}

// TestAFPV0047NoPlanFullFallbackIsComplete pins the AFP-V0-047 amendment (V1-5216): a selection
// that fell back to FULL because no plan was built is a complete forward row whose S is every
// package with an outcome, so a queue failure is selected, never a MISS.
func TestAFPV0047NoPlanFullFallbackIsComplete(t *testing.T) {
	f := queueFixture(t)
	for _, d := range f.sel {
		noPlanFallback(d)
	}
	universe := map[string]bool{}
	for _, d := range f.out {
		for pkg := range sub(d, "outcomes") {
			universe[pkg] = true
		}
	}
	row, _ := f.classify(t, nil)
	if row["class"] != "CLEAN" || len(strs(row["incomplete"])) != 0 {
		t.Fatalf("fallback row = %v %q, want CLEAN and complete", row["class"], row["incomplete"])
	}
	if sel := sub(row, "selection"); sel["scope"] != "FULL" || num(sel["selected"]) != len(universe) {
		t.Fatalf("fallback selection = %v, want FULL over all %d packages", sel, len(universe))
	}

	f = queueFixture(t)
	for _, d := range f.sel {
		noPlanFallback(d)
	}
	fail(f.out[7], flakyPkg, "fail", "RAN", "fail")
	row, _ = f.classify(t, nil)
	if c := sub(row, "counts"); row["class"] != "CLEAN" || num(c["missed"]) != 0 || num(c["selectedFailedInQueue"]) != 1 {
		t.Fatalf("fallback row with a queue failure = %v %v, want CLEAN with it selected", row["class"], c)
	}
}

// TestAFPV0047OtherPlanlessSelectionsStayIncomplete is the fail-closed half: only the recognisable
// fallback may lack a plan digest, and a fallback mixed with planned shards is not one row.
func TestAFPV0047OtherPlanlessSelectionsStayIncomplete(t *testing.T) {
	const shape = "8 selection document(s) fail the corvint-pr-selection/0 shape"
	for _, c := range []struct {
		name   string
		mutate func(doc)
	}{
		{"narrowed without a plan digest", func(d doc) { d["planDigest"] = "" }},
		{"FULL with another reason", func(d doc) { noPlanFallback(d); d["reason"] = "declared full scope" }},
		{"FULL with no reason", func(d doc) { noPlanFallback(d); delete(d, "reason") }},
		{"fallback with a null plan digest", func(d doc) { noPlanFallback(d); d["planDigest"] = nil }},
		{"fallback with a malformed plan digest", func(d doc) { noPlanFallback(d); d["planDigest"] = "sha256:x" }},
		{"fallback scoped NARROWED", func(d doc) { noPlanFallback(d); d["scope"] = "NARROWED" }},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := queueFixture(t)
			for _, d := range f.sel {
				c.mutate(d)
			}
			row, _ := f.classify(t, nil)
			wantIncomplete(t, row, shape)
		})
	}

	f := queueFixture(t)
	for k, d := range f.sel {
		if k != 3 {
			noPlanFallback(d)
		}
	}
	row, _ := f.classify(t, nil)
	wantIncomplete(t, row, "selection shards mix a no-plan FULL fallback with a plan")
}

// TestAFPV0030RerunsOfOneRowCountOnce pins the bar.jq dedupe the V1-5216 report questioned: rows
// of the same pull request and tested commit, however many times uploaded, are one row, so one
// pull request's rerun INCOMPLETE rows never reach the 3-INCOMPLETE kill switch alone.
func TestAFPV0030RerunsOfOneRowCountOnce(t *testing.T) {
	names := []string{barNames("INCOMPLETE", 163, "1", "0"), barNames("INCOMPLETE", 163, "1", "0"),
		barNames("INCOMPLETE", 163, "1", "0"), barNames("INCOMPLETE", 164, "1", "0")}
	names = append(names, metNames()...)
	v := barJSON(t, "1", names)
	if w := sub(v, "window"); v["killSwitch"] != "clear" || num(w["incomplete"]) != 2 {
		t.Fatalf("verdict = %v, want 2 INCOMPLETE rows and a clear switch", v)
	}
	// Three distinct rows still trip it: the threshold is unchanged.
	names = append([]string{barNames("INCOMPLETE", 165, "1", "0")}, names...)
	if v := barJSON(t, "1", names); v["killSwitch"] != "TRIPPED" {
		t.Fatalf("verdict = %v, want TRIPPED", v)
	}
}
