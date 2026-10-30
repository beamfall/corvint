package appmap

import (
	"encoding/json"
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
