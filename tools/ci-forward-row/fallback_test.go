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

// TestAFPV0047NoPlanFallbackIsAFullRow_CINV07921 pins the V1-5216 fallback under CIN-V0-7921,
// which AFP-V0-047 defers to: a run whose every shard fell back to FULL because no plan was built
// measured no narrowing, so it is a complete row of class FULL (type skew), never CLEAN. S stays
// the selected packages only, so a queue failure counts as missed, yet the class is still FULL
// and the row is no narrowing evidence either way.
func TestAFPV0047NoPlanFallbackIsAFullRow_CINV07921(t *testing.T) {
	requireGitHubChange(t, ".github/forward-rows/row.jq", "CIN-V0-7921")
	f := queueFixture(t)
	for _, d := range f.sel {
		noPlanFallback(d)
	}
	row, _ := f.classify(t, nil)
	if row["class"] != "FULL" || len(strs(row["incomplete"])) != 0 {
		t.Fatalf("fallback row = %v %q, want FULL and complete", row["class"], row["incomplete"])
	}
	sel := sub(row, "selection")
	if full, _ := sel["full"].(doc); sel["scope"] != "FULL" || num(sel["selected"]) != 0 || full["type"] != "skew" || num(full["unplanned"]) != 8 {
		t.Fatalf("fallback selection = %v, want FULL of type skew over 8 unplanned shards, none selected", sel)
	}

	f = queueFixture(t)
	for _, d := range f.sel {
		noPlanFallback(d)
	}
	fail(f.out[7], flakyPkg, "fail", "RAN", "fail")
	row, _ = f.classify(t, nil)
	if c := sub(row, "counts"); row["class"] != "FULL" || num(c["missed"]) != 1 || num(c["selectedFailedInQueue"]) != 0 {
		t.Fatalf("fallback row with a queue failure = %v %v, want FULL with the failure missed", row["class"], c)
	}
}

// TestAFPV0047OtherPlanlessSelectionsFailClosed_CINV07921 is the fail-closed half: a document
// with no plan passes the shape only as a FULL fallback with a reason; a FULL run typed by its
// reasons (abstain for another reason, skew for a missing or malformed plan digest) is still a
// FULL row; and a fallback beside planned shards leaves the row incomplete.
func TestAFPV0047OtherPlanlessSelectionsFailClosed_CINV07921(t *testing.T) {
	requireGitHubChange(t, ".github/forward-rows/row.jq", "CIN-V0-7921")
	const shape = "8 selection document(s) fail the corvint-pr-selection/0 shape"
	for _, c := range []struct {
		name   string
		mutate func(doc)
	}{
		{"narrowed without a plan digest", func(d doc) { d["planDigest"] = "" }},
		{"FULL with no reason", func(d doc) { noPlanFallback(d); delete(d, "reason") }},
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
	for _, c := range []struct {
		name, fullType string
		mutate         func(doc)
	}{
		{"FULL with another reason", "abstain", func(d doc) { noPlanFallback(d); d["reason"] = "declared full scope" }},
		{"fallback with a null plan digest", "skew", func(d doc) { noPlanFallback(d); d["planDigest"] = nil }},
		{"fallback with a malformed plan digest", "skew", func(d doc) { noPlanFallback(d); d["planDigest"] = "sha256:x" }},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := queueFixture(t)
			for _, d := range f.sel {
				c.mutate(d)
			}
			row, _ := f.classify(t, nil)
			if full, _ := sub(row, "selection")["full"].(doc); row["class"] != "FULL" || len(strs(row["incomplete"])) != 0 || full["type"] != c.fullType {
				t.Fatalf("row = %v %q %v, want a complete FULL row of type %s", row["class"], row["incomplete"], full, c.fullType)
			}
		})
	}

	f := queueFixture(t)
	for k, d := range f.sel {
		if k != 3 {
			noPlanFallback(d)
		}
	}
	row, _ := f.classify(t, nil)
	wantIncomplete(t, row, "planned nothing (FULL: no plan: unsupported-affected-graph")
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
