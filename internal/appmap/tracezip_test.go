package appmap

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func traceZip(t *testing.T, entries map[string]string) string {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, body := range entries {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "trace.zip")
	if err := os.WriteFile(p, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func traceLines(recs ...map[string]any) string {
	var b strings.Builder
	for _, r := range recs {
		j, _ := json.Marshal(r)
		b.Write(j)
		b.WriteByte('\n')
	}
	return b.String()
}

func before(call, method, selector string) map[string]any {
	return map[string]any{"type": "before", "callId": call, "method": method, "params": map[string]any{"selector": selector, "value": "typed-secret"}}
}

func snap(call, phase, url, page string) map[string]any {
	return map[string]any{"type": "frame-snapshot", "snapshot": map[string]any{"callId": call, "snapshotName": phase + "@" + call, "frameUrl": url, "pageId": page, "isMainFrame": true, "html": []any{}}}
}

// ANL-V0-022: a trace zip is read in memory; main-frame URL changes inside an action are
// attributed to it, changes between actions read UNKNOWN; bounds and ambiguity refuse.
func TestANLV0022TraceZip(t *testing.T) {
	body := traceLines(
		map[string]any{"type": "context-options", "version": 8},
		before("call@1", "goto", ""), snap("call@1", "before", "about:blank", "p1"), snap("call@1", "after", "https://x.test/login?token=sekret1", "p1"),
		before("call@2", "click", "#go"), snap("call@2", "before", "https://x.test/login?token=sekret1", "p1"), snap("call@2", "input", "https://x.test/login?token=sekret1", "p1"), snap("call@2", "after", "https://x.test/home", "p1"),
		before("call@3", "isVisible", "#x"), snap("call@3", "before", "https://x.test/items/7", "p1"), snap("call@3", "after", "https://x.test/items/7", "p1"),
		before("call@4", "press", "#q"), snap("call@4", "before", "https://x.test/items/7", "p1"), snap("call@4", "after", "https://x.test/login", "p1"),
		map[string]any{"type": "frame-snapshot", "snapshot": map[string]any{"callId": "call@4", "snapshotName": "after@call@4", "frameUrl": "https://ads.test/", "pageId": "p1", "isMainFrame": false}},
	)
	z := traceZip(t, map[string]string{"0-trace.trace": body, "test.trace": "{}\n", "resources/a.png": "x", "../trace.trace": "{}\n"})
	docs, raw, err := LoadTraceZip(z, "spec.ts:checkout")
	if err != nil {
		t.Fatal(err)
	}
	doc := docs[0]
	if len(docs) != 1 || doc.Page != "" || len(doc.Events) != 4 || doc.TestKey != "spec.ts:checkout" {
		t.Fatal(doc.Events)
	}
	obs, out := observeOnceRaw(t, observeMap(""), doc, raw)
	tr := obs.Traces[0]
	want := []string{"click", StatusUnknown, "key press"}
	for i, e := range tr.Edges {
		if e.Trigger != want[i] {
			t.Fatalf("edge %d %+v", i, e)
		}
	}
	if tr.Edges[0].Selector != "#go" || tr.Visits[2].Screen != screenID("shop", "item") || tr.TraceDigest != digest(raw) {
		t.Fatal(tr)
	}
	if strings.Contains(string(out), "sekret1") || strings.Contains(string(out), "typed-secret") || strings.Contains(string(out), "ads.test") {
		t.Fatal("leak or subframe", string(out))
	}
	refuse := map[string]map[string]string{
		"nested only":  {"dir/trace.trace": body},
		"no snapshots": {"trace.trace": traceLines(before("call@1", "goto", ""))},
		"bad record":   {"trace.trace": body + "{not json\n"},
		"null record":  {"trace.trace": body + "null\n"},
		"no url":       {"trace.trace": `{"type":"frame-snapshot","snapshot":{"pageId":"p","isMainFrame":true}}` + "\n"},
	}
	for name, entries := range refuse {
		if _, _, err := LoadTraceZip(traceZip(t, entries), "k"); err == nil {
			t.Fatal(name, "accepted")
		}
	}
	if _, _, err := LoadTraceZip(z, "bad key"); err == nil {
		t.Fatal("invalid test key accepted")
	}
	notZip := filepath.Join(t.TempDir(), "x.zip")
	_ = os.WriteFile(notZip, []byte("PK"), 0o600)
	link := filepath.Join(t.TempDir(), "l.zip")
	_ = os.Symlink(z, link)
	for _, p := range []string{notZip, link} {
		if _, _, err := LoadTraceZip(p, "k"); err == nil {
			t.Fatal(p, "accepted")
		}
	}
	// Only input and after snapshots attribute; any other phase reads UNKNOWN.
	custom := traceLines(before("c1", "click", "#a"), snap("c1", "before", "https://x.test/home", "p1"), snap("c1", "custom", "https://x.test/login", "p1"))
	if docs, _, err := LoadTraceZip(traceZip(t, map[string]string{"trace.trace": custom}), "k"); err != nil || len(docs) != 1 || len(docs[0].Events) != 2 || docs[0].Events[1].Action != nil {
		t.Fatal("custom phase attributed", docs, err)
	}
	defer func(n int) { traceZipEntryLimit = n }(traceZipEntryLimit)
	traceZipEntryLimit = 3
	if _, _, err := LoadTraceZip(z, "k"); err == nil {
		t.Fatal("entry count bound not enforced")
	}
	traceZipEntryLimit = 65536
	defer func(a, b int) { traceEntryLimit, traceLineLimit = a, b }(traceEntryLimit, traceLineLimit)
	traceEntryLimit = 512
	if _, _, err := LoadTraceZip(z, "k"); err == nil {
		t.Fatal("entry bound not enforced")
	}
	traceEntryLimit, traceLineLimit = 64<<20, 128
	if _, _, err := LoadTraceZip(z, "k"); err == nil {
		t.Fatal("line bound not enforced")
	}
}

func observeOnceRaw(t *testing.T, m *Map, d NavigationEvents, raw []byte) (Observation, []byte) {
	t.Helper()
	out, err := Observe(m, []NavigationEvents{d}, [][]byte{raw}, "abc", Options{})
	if err != nil {
		t.Fatal(err)
	}
	var obs Observation
	if err = json.Unmarshal(out, &obs); err != nil {
		t.Fatal(err)
	}
	return obs, out
}

// pw164Shape is a hand-built trace laid out as Playwright 1.64.0 writes it (string call IDs such
// as `wcuw@676`, one `N-trace.trace`/`.network`/`.stacks` triple per browser context: an API
// request context with no page, a second user's page and the test page). It is not a captured
// trace: real-trace qualification stays NOT_RUN.
const pw164Shape = "testdata/playwright-1.64-shape"

// pw164Entries reads the fixture's zip entries; override replaces entries, an empty value drops one.
func pw164Entries(t *testing.T, override map[string]string) map[string]string {
	t.Helper()
	files, err := os.ReadDir(pw164Shape)
	if err != nil {
		t.Fatal(err)
	}
	entries := map[string]string{}
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(pw164Shape, f.Name()))
		if err != nil {
			t.Fatal(err)
		}
		entries[f.Name()] = string(b)
	}
	for k, v := range override {
		if v == "" {
			delete(entries, k)
		} else {
			entries[k] = v
		}
	}
	return entries
}

// ANL-V0-068: a multi-context trace zip is read whole. Every context trace is keyed by its entry
// prefix, visits are ordered per page (never across pages or contexts), and each page document
// names its page; a context without pages adds nothing.
func TestANLV0068MultiContextTraceZip(t *testing.T) {
	z := traceZip(t, pw164Entries(t, nil))
	docs, raw, err := LoadTraceZip(z, "login.spec.ts:two-users")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 || docs[0].Page != "1-page@b2p1" || docs[1].Page != "2-page@c3p1" {
		t.Fatalf("pages: %+v", docs)
	}
	for i, want := range [][]string{{"http://app.test/login", "http://app.test/home"}, {"http://app.test/login", "http://app.test/users/42"}} {
		ev := docs[i].Events
		if len(ev) != 2 || ev[0].URL != want[0] || ev[1].URL != want[1] || ev[0].Action == nil || ev[0].Action.Kind != "goto" || ev[1].Action == nil || ev[1].Action.Kind != "click" {
			t.Fatalf("page %d events: %+v", i, ev)
		}
	}
	out, err := Observe(observeMap(""), docs, [][]byte{raw, raw}, "abc", Options{})
	if err != nil {
		t.Fatal(err)
	}
	var obs Observation
	if err = json.Unmarshal(out, &obs); err != nil {
		t.Fatal(err)
	}
	if len(obs.Traces) != 2 {
		t.Fatal(obs.Traces)
	}
	for i, tr := range obs.Traces {
		if tr.Page != docs[i].Page || tr.TraceDigest != digest(raw) || len(tr.Visits) != 2 || len(tr.Edges) != 1 || tr.Edges[0].Trigger != "click" {
			t.Fatalf("trace %d: %+v", i, tr)
		}
	}
	if obs.Traces[0].Edges[0].To != screenID("shop", "home") || obs.Traces[1].Visits[1].Status != UnmappedURL {
		t.Fatalf("per-page edges: %+v", obs.Traces)
	}
	if !strings.Contains(string(out), `"page":"1-page@b2p1"`) {
		t.Fatal("multi-page observation does not name its page", string(out))
	}
	// One context with two pages (a popup) reads one document per page in first-snapshot order.
	popup := traceLines(before("c1", "goto", ""), snap("c1", "after", "https://x.test/login", "p1"),
		snap("c2", "before", "https://x.test/home", "p2"), snap("c1", "after", "https://x.test/home", "p1"))
	docs, _, err = LoadTraceZip(traceZip(t, map[string]string{"trace.trace": popup}), "k")
	if err != nil || len(docs) != 2 || docs[0].Page != "p1" || docs[1].Page != "p2" || len(docs[0].Events) != 2 || len(docs[1].Events) != 1 {
		t.Fatal("popup pages", docs, err)
	}
	// A call ID repeated in another context never attributes that context's change.
	cross := map[string]string{
		"trace.trace":   traceLines(before("call@1", "click", "#a"), snap("call@1", "before", "https://x.test/login", "p1")),
		"1-trace.trace": traceLines(snap("x", "before", "https://x.test/login", "p1"), snap("call@1", "after", "https://x.test/home", "p1")),
	}
	docs, _, err = LoadTraceZip(traceZip(t, cross), "k")
	if err != nil || len(docs) != 2 || docs[0].Page != "p1" || docs[1].Page != "1-p1" || docs[1].Events[1].Action != nil {
		t.Fatal("cross-context attribution", docs, err)
	}
	many := map[string]string{}
	for i := 0; i <= maxTraceContexts; i++ {
		many[strconv.Itoa(i)+"-trace.trace"] = traceLines(snap("c", "before", "https://x.test/login", "p"))
	}
	if _, _, err := LoadTraceZip(traceZip(t, many), "k"); err == nil {
		t.Fatal("more than 16 context traces accepted")
	}
	if _, _, err := LoadTraceZip(traceZip(t, pw164Entries(t, map[string]string{"1-trace.trace": "", "2-trace.trace": ""})), "k"); err == nil {
		t.Fatal("a zip whose contexts have no main-frame snapshot accepted")
	}
}

// orderedZip writes entries in the order given, so a test can repeat a name.
func orderedZip(t *testing.T, names, bodies []string) string {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for i, name := range names {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.Write([]byte(bodies[i])); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "trace.zip")
	if err := os.WriteFile(p, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// ANL-V0-068: context keys never collide, a before URL counts only on its own page, entries order
// by N without overflow, and repeated context names and too many pages refuse.
func TestANLV0068TraceZipKeysAndBounds(t *testing.T) {
	// Root page "1-p" and page "p" of 1-trace.trace are two pages, not one.
	clash := map[string]string{
		"trace.trace":   traceLines(snap("c", "before", "https://x.test/a", "1-p")),
		"1-trace.trace": traceLines(snap("c", "before", "https://x.test/b", "p")),
	}
	docs, _, err := LoadTraceZip(traceZip(t, clash), "k")
	if err != nil || len(docs) != 2 || len(docs[0].Events) != 1 || len(docs[1].Events) != 1 {
		t.Fatal("prefix-spelling page IDs merged", docs, err)
	}
	// Call c's before snapshot on p1 never attributes p2's change, even at the same URL.
	inter := traceLines(before("c", "click", "#a"), snap("x", "before", "https://x.test/login", "p2"),
		snap("c", "before", "https://x.test/login", "p1"), snap("c", "after", "https://x.test/home", "p2"))
	docs, _, err = LoadTraceZip(traceZip(t, map[string]string{"trace.trace": inter}), "k")
	if err != nil || len(docs) != 2 || docs[0].Page != "p2" || len(docs[0].Events) != 2 || docs[0].Events[1].Action != nil {
		t.Fatal("cross-page attribution", docs, err)
	}
	// A huge N sorts after small ones and never before the root.
	order := map[string]string{
		"99999999999999999999999-trace.trace": traceLines(snap("c", "before", "https://x.test/c", "p")),
		"2-trace.trace":                       traceLines(snap("c", "before", "https://x.test/b", "p")),
		"trace.trace":                         traceLines(snap("c", "before", "https://x.test/a", "p")),
	}
	docs, _, err = LoadTraceZip(traceZip(t, order), "k")
	if err != nil || len(docs) != 3 || docs[0].Page != "p" || docs[1].Page != "2-p" || docs[2].Page != "99999999999999999999999-p" {
		t.Fatal("entry order", docs, err)
	}
	one := traceLines(snap("c", "before", "https://x.test/a", "p"))
	if _, _, err = LoadTraceZip(orderedZip(t, []string{"1-trace.trace", "1-trace.trace"}, []string{one, one}), "k"); err == nil {
		t.Fatal("two context traces with one name accepted")
	}
	var recs []map[string]any
	for i := 0; i <= maxTraceFiles; i++ {
		recs = append(recs, snap("c", "before", "about:blank", "p"+strconv.Itoa(i)))
	}
	if _, _, err = LoadTraceZip(traceZip(t, map[string]string{"trace.trace": traceLines(recs...)}), "k"); err == nil {
		t.Fatal("more than 64 pages accepted")
	}
}
