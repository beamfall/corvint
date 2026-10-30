package corpusbridge

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

const compactFixture = `{
 "schema": "corvint-corpus-receipt/2",
 "capabilities": [{"name": "subjects", "state": "present"}],
 "limitations": ["page withheld", "shared"],
 "citations": [
  {"path": "a.go", "start_line": 1, "end_line": 2, "symbol": "A", "revision": "r1", "blob": "b1", "sha256": "s1", "span_sha256": "p1"},
  {"path": "b.go", "start_line": 3, "end_line": 4, "symbol": "B", "revision": "r1", "blob": "b2", "sha256": "s2", "span_sha256": "p2"}
 ],
 "results": [
  {"id": "x", "evidence": {"anchors": [
    {"path": "a.go", "start_line": 1, "end_line": 2, "symbol": "A", "revision": "r1", "blob": "b1", "sha256": "s1", "span_sha256": "p1"}],
   "limitations": ["declaration only", "shared"]}},
  {"id": "y", "evidence": {"anchors": [
    {"path": "b.go", "start_line": 3, "end_line": 4, "symbol": "B", "revision": "r1", "blob": "b2", "sha256": "s2", "span_sha256": "p2"},
    {"path": "c.md", "start_line": 9, "end_line": 9, "symbol": "", "revision": "r1", "blob": "b3", "sha256": "s3", "span_sha256": "p3"}],
   "limitations": ["declaration only"]}},
  {"id": "z", "count": 3}
 ],
 "result_ranks": {
  "y": {"rank": 2, "match_anchor": {"path": "c.md", "start_line": 9, "end_line": 9, "symbol": "", "revision": "r1", "blob": "b3", "sha256": "s3", "span_sha256": "p3"}},
  "x": {"rank": 1}
 },
 "symbol_anchors": [
  {"anchor": "a.go:1-2#A@r1", "state": "resolved", "reason": "", "live": {"revision": "r1", "path": "a.go", "blob": "b1", "sha256": "s1", "start_line": 1, "end_line": 2, "span_sha256": "p1"}},
  {"anchor": "b.go:3-4#B@r1", "state": "moved", "reason": "", "live": {"revision": "r2", "path": "b.go", "blob": "b9", "sha256": "s9", "start_line": 5, "end_line": 6, "span_sha256": "p2"}},
  {"anchor": "gone.go:1-1#G@r1", "state": "unresolved", "reason": "not cited"}
 ],
 "trust_envelope": {
  "freshness": "fresh",
  "citations": [
   {"path": "a.go", "start_line": 1, "end_line": 2, "symbol": "A", "revision": "r1", "blob": "b1", "sha256": "s1", "span_sha256": "p1", "anchor_state": "live"},
   {"path": "d.go", "start_line": 7, "end_line": 7, "symbol": "D", "revision": "r1", "blob": "b4", "sha256": "s4", "span_sha256": "p4", "anchor_state": "drifted"}
  ],
  "limitations": ["shared", "envelope only"]
 }
}`

// DCP-V1-653..655 (V1-3080, GitHub #1977): the compact receipt holds each
// anchor and each limitation once, names them by id and index everywhere
// else, omits capabilities unless asked, and loses no anchor or limitation of
// any holder: expanding it rebuilds the full receipt's anchors and
// limitations exactly.
func TestDCPV1653CompactReceipt(t *testing.T) {
	var full map[string]any
	if err := json.Unmarshal([]byte(compactFixture), &full); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(full)
	compact, err := CompactReceipt(full, false)
	if err != nil {
		t.Fatal(err)
	}
	if after, _ := json.Marshal(full); string(after) != string(before) {
		t.Fatal("the full receipt was modified")
	}
	if compact["receipt_view"] != "compact" || compact["capabilities"] != nil || compact["schema"] != "corvint-corpus-receipt/2" {
		t.Fatalf("view, capabilities or schema: %v", compact)
	}
	anchors := compact["anchors"].([]any)
	// a.go, b.go and c.md from citations and results, d.go from the envelope.
	if len(anchors) != 4 {
		t.Fatalf("anchors = %v", anchors)
	}
	byID := map[string]map[string]any{}
	for i, v := range anchors {
		a := v.(map[string]any)
		if want := "a" + string(rune('1'+i)); a["id"] != want {
			t.Fatalf("anchor %d id = %v, want %s", i, a["id"], want)
		}
		byID[a["id"].(string)] = a
	}
	if byID["a1"]["anchor_state"] != "live" || byID["a4"]["anchor_state"] != "drifted" || byID["a2"]["anchor_state"] != nil {
		t.Fatalf("anchor states: %v", anchors)
	}
	if !reflect.DeepEqual(compact["citations"], []any{"a1", "a2"}) {
		t.Fatalf("citations = %v", compact["citations"])
	}
	results := compact["results"].([]any)
	if !reflect.DeepEqual(results[1].(map[string]any)["evidence"].(map[string]any)["anchors"], []any{"a2", "a3"}) {
		t.Fatalf("result anchors = %v", results[1])
	}
	if !reflect.DeepEqual(results[2], map[string]any{"id": "z", "count": float64(3)}) {
		t.Fatalf("a result without evidence changed: %v", results[2])
	}
	if ranks := compact["result_ranks"].(map[string]any); ranks["y"].(map[string]any)["match_anchor"] != "a3" {
		t.Fatalf("match_anchor = %v", ranks["y"])
	}
	checks := compact["symbol_anchors"].([]any)
	if c := checks[0].(map[string]any); c["anchor"] != "a1" || c["live"] != "a1" {
		t.Fatalf("an unmoved symbol anchor should name its anchor: %v", c)
	}
	if c := checks[1].(map[string]any); c["anchor"] != "a2" || reflect.TypeOf(c["live"]).Kind() != reflect.Map {
		t.Fatalf("a moved symbol anchor keeps its live location: %v", c)
	}
	if c := checks[2].(map[string]any); c["anchor"] != "gone.go:1-1#G@r1" {
		t.Fatalf("an uncited symbol anchor should stay as it was: %v", c)
	}
	if !reflect.DeepEqual(compact["limitations"], []any{"page withheld", "shared", "declaration only", "envelope only"}) {
		t.Fatalf("limitations = %v", compact["limitations"])
	}
	// Expanding every reference rebuilds each holder's anchors and limitations.
	table := compact["limitations"].([]any)
	expandAnchors := func(refs any) []any {
		out := []any{}
		for _, id := range refs.([]any) {
			a := map[string]any{}
			for k, v := range byID[id.(string)] {
				if k != "id" && k != "anchor_state" {
					a[k] = v
				}
			}
			out = append(out, a)
		}
		return out
	}
	expandLimitations := func(refs any) []any {
		out := []any{}
		for _, i := range refs.([]any) {
			out = append(out, table[int(i.(float64))])
		}
		return out
	}
	// Index references are ints in memory; normalize through JSON first.
	var wire map[string]any
	encoded, _ := json.Marshal(compact)
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expandAnchors(wire["citations"]), full["citations"]) {
		t.Fatal("citations do not expand to the full receipt's")
	}
	for i, v := range wire["results"].([]any)[:2] {
		evidence := v.(map[string]any)["evidence"].(map[string]any)
		want := full["results"].([]any)[i].(map[string]any)["evidence"].(map[string]any)
		if !reflect.DeepEqual(expandAnchors(evidence["anchors"]), want["anchors"]) || !reflect.DeepEqual(expandLimitations(evidence["limitations"]), want["limitations"]) {
			t.Fatalf("result %d does not expand to the full receipt's", i)
		}
	}
	envelope := wire["trust_envelope"].(map[string]any)
	if !reflect.DeepEqual(expandLimitations(envelope["limitations"]), full["trust_envelope"].(map[string]any)["limitations"]) {
		t.Fatal("envelope limitations do not expand to the full receipt's")
	}
	for i, id := range envelope["citations"].([]any) {
		want := full["trust_envelope"].(map[string]any)["citations"].([]any)[i].(map[string]any)
		got := expandAnchors([]any{id})[0].(map[string]any)
		got["anchor_state"] = byID[id.(string)]["anchor_state"]
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("envelope citation %d = %v, want %v", i, got, want)
		}
	}
	// include ["capabilities"] keeps them.
	withCapabilities, err := CompactReceipt(full, true)
	if err != nil || !reflect.DeepEqual(withCapabilities["capabilities"], full["capabilities"]) {
		t.Fatalf("capabilities on request: %v %v", withCapabilities["capabilities"], err)
	}
	// The same receipt compacts to the same bytes.
	again, _ := CompactReceipt(full, false)
	if a, b := mustJSON(t, again), mustJSON(t, compact); a != b {
		t.Fatal("compaction is not deterministic")
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// DCP-V1-655: use the real receipt encoder, which retains scalar zero values
// under json/v2 omitempty. Only the two known zero defaults may be discarded.
func TestDCPV1655CompactReceiptEncodedSymbolLocation(t *testing.T) {
	anchor := doccorpus.Anchor{Repository: "repo", Revision: "r1", Path: "a.ts", Blob: "b1", SHA256: "s1", Start: 1, End: 2, SpanSHA256: "p1", Symbol: "A"}
	receipt := doccorpus.Receipt{
		Schema: "corvint-corpus-receipt/2", Operation: "search",
		Citations: []doccorpus.Anchor{anchor},
		SymbolAnchors: []doccorpus.SymbolAnchorCheck{{
			Anchor: doccorpus.FormatSymbolAnchor(anchor), State: doccorpus.SymbolAnchorLive,
			Live: &doccorpus.SymbolLocation{Revision: anchor.Revision, Path: anchor.Path, Blob: anchor.Blob, SHA256: anchor.SHA256, Start: anchor.Start, End: anchor.End, SpanSHA256: anchor.SpanSHA256},
		}},
	}
	raw, err := doccorpus.Encode(receipt)
	if err != nil {
		t.Fatal(err)
	}
	decode := func(t *testing.T) (map[string]any, map[string]any) {
		t.Helper()
		var full map[string]any
		if err := json.Unmarshal(raw, &full); err != nil {
			t.Fatal(err)
		}
		live := full["symbol_anchors"].([]any)[0].(map[string]any)["live"].(map[string]any)
		return full, live
	}
	_, encodedLive := decode(t)
	if encodedLive["declaration_line"] != float64(0) || encodedLive["estimated"] != false {
		t.Fatalf("fixture does not exercise encoded zero defaults: %v", encodedLive)
	}
	type testCase struct {
		name     string
		edit     func(map[string]any)
		collapse bool
	}
	cases := []testCase{
		{"encoded zeros", func(map[string]any) {}, true},
		{"absent defaults", func(l map[string]any) { delete(l, "declaration_line"); delete(l, "estimated") }, true},
		{"absent declaration", func(l map[string]any) { delete(l, "declaration_line") }, true},
		{"absent estimated", func(l map[string]any) { delete(l, "estimated") }, true},
		{"declaration", func(l map[string]any) { l["declaration_line"] = float64(1) }, false},
		{"estimated", func(l map[string]any) { l["estimated"] = true }, false},
		{"unknown zero", func(l map[string]any) { l["future"] = float64(0) }, false},
		{"null declaration", func(l map[string]any) { l["declaration_line"] = nil }, false},
		{"null estimated", func(l map[string]any) { l["estimated"] = nil }, false},
		{"string declaration", func(l map[string]any) { l["declaration_line"] = "0" }, false},
		{"string estimated", func(l map[string]any) { l["estimated"] = "false" }, false},
	}
	for _, key := range []string{"revision", "path", "blob", "sha256", "start_line", "end_line", "span_sha256"} {
		cases = append(cases, testCase{"missing " + key, func(l map[string]any) { delete(l, key) }, false})
		cases = append(cases, testCase{"changed " + key, func(l map[string]any) { l[key] = "different" }, false})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			full, live := decode(t)
			tc.edit(live)
			before := mustJSON(t, full)
			compact, err := CompactReceipt(full, false)
			if err != nil {
				t.Fatal(err)
			}
			check := compact["symbol_anchors"].([]any)[0].(map[string]any)
			if check["anchor"] != "a1" {
				t.Fatalf("anchor was not interned: %v", check)
			}
			if tc.collapse {
				if check["live"] != "a1" {
					t.Fatalf("unchanged live location did not collapse: %v", check["live"])
				}
			} else if !reflect.DeepEqual(check["live"], live) {
				t.Fatalf("live location lost information: %v, want %v", check["live"], live)
			}
			if mustJSON(t, full) != before {
				t.Fatal("full receipt was modified")
			}
		})
	}
}
