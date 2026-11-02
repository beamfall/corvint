package appmap

import (
	"encoding/json"
	"strings"
	"testing"
)

// OUE-V0-058 (V1-2739, #1852): real chromium traces written by Playwright 1.64.0 with the
// reporter's configuration (testdata/pw164/playwright.config.js.txt: trace mode on, DOM and aria
// snapshots, no screenshots, sources; spec in flows.spec.js.txt) are read whole. String stack
// IDs attach frames for `--records`, and the multi-context zip (an API request context, a second
// browser context and the test page) yields one observation per page for `--map --trace`.
func TestOUEV0058RealPlaywright164Traces(t *testing.T) {
	single, multi := "testdata/pw164/single-context.zip", "testdata/pw164/multi-context.zip"
	raw, err := ObserveUIRecords(uiTestMap(), nil, "", []UITraceInput{{TestKey: "single", File: single}, {TestKey: "multi", File: multi}}, uiTestRevision)
	if err != nil {
		t.Fatal(err)
	}
	var rec UIRecords
	if err = json.Unmarshal(raw, &rec); err != nil {
		t.Fatal(err)
	}
	if len(rec.Traces) != 2 || len(rec.Screens) == 0 || len(rec.Transitions) == 0 || len(rec.Endpoints) == 0 {
		t.Fatalf("records: %s", raw)
	}
	for _, tr := range rec.Traces {
		if tr.StackFrames == 0 || tr.Actions == 0 || tr.Navigations == 0 {
			t.Fatalf("trace %+v", tr)
		}
	}
	for _, u := range rec.Unknowns {
		if u.Reason == "trace-without-stacks" {
			t.Fatalf("stacks unread: %+v", u)
		}
	}

	docs, _, err := LoadTraceZip(single, "spec:single")
	if err != nil || len(docs) != 1 || len(docs[0].Events) != 2 {
		t.Fatalf("single: %v %+v", err, docs)
	}
	docs, _, err = LoadTraceZip(multi, "spec:multi")
	if err != nil || len(docs) != 2 || docs[0].Page == "" || docs[1].Page == "" {
		t.Fatalf("multi: %v %+v", err, docs)
	}
	visits := 0
	for _, d := range docs {
		visits += len(d.Events)
	}
	if visits != 3 {
		t.Fatalf("multi visits %d: %+v", visits, docs)
	}
}

// OUE-V0-685 (V1-2723): a real Playwright 1.63.0 chromium trace carries the ARIA YAML only as
// the after.result.snapshot of a Frame.ariaSnapshot call; it yields one scrubbed structure
// digest for the screen, and a trace without such a call yields none.
func TestOUEV0685RealPlaywrightAriaSnapshotResult(t *testing.T) {
	raw, err := ObserveUIRecords(uiTestMap(), nil, "", []UITraceInput{{TestKey: "aria", File: "testdata/pw163-aria/aria.zip"}}, uiTestRevision)
	if err != nil {
		t.Fatal(err)
	}
	var rec UIRecords
	if err = json.Unmarshal(raw, &rec); err != nil {
		t.Fatal(err)
	}
	if len(rec.Traces) != 1 || rec.Traces[0].AriaSnapshots != 1 || rec.Traces[0].AriaOversized != 0 {
		t.Fatalf("traces: %+v", rec.Traces)
	}
	if len(rec.Structures) != 1 || rec.Structures[0].Screen == "" || strings.HasPrefix(rec.Structures[0].Screen, "unknown:") {
		t.Fatalf("structures: %+v", rec.Structures)
	}
	if strings.Contains(string(raw), "Members") || strings.Contains(string(raw), "Save") {
		t.Fatalf("ARIA text leaked: %s", raw)
	}
	raw, err = ObserveUIRecords(uiTestMap(), nil, "", []UITraceInput{{TestKey: "single", File: "testdata/pw164/single-context.zip"}}, uiTestRevision)
	if err != nil {
		t.Fatal(err)
	}
	rec = UIRecords{}
	if err = json.Unmarshal(raw, &rec); err != nil {
		t.Fatal(err)
	}
	if len(rec.Structures) != 0 || rec.Traces[0].AriaSnapshots != 0 {
		t.Fatalf("pw164 single has no ariaSnapshot call: %+v", rec.Structures)
	}
}
