package appmap

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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
	doc, raw, err := LoadTraceZip(z, "spec.ts:checkout")
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Events) != 4 || doc.TestKey != "spec.ts:checkout" {
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
		"two contexts": {"0-trace.trace": body, "1-trace.trace": body},
		"nested only":  {"dir/trace.trace": body},
		"multi page":   {"trace.trace": body + traceLines(snap("call@9", "before", "https://x.test/home", "p2"))},
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
	if doc, _, err := LoadTraceZip(traceZip(t, map[string]string{"trace.trace": custom}), "k"); err != nil || len(doc.Events) != 2 || doc.Events[1].Action != nil {
		t.Fatal("custom phase attributed", doc, err)
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
