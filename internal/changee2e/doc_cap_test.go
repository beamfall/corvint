package changee2e

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func withDocByteCap(t *testing.T, n int) {
	t.Helper()
	old := docByteCap
	docByteCap = n
	t.Cleanup(func() { docByteCap = old })
}

// CCE-V0-014, CCE-V0-040 (V1-2378): an untruncated document carries no truncated count; an
// over-cap one keeps the longest deterministic prefix that fits, names the dropped count and keeps
// every total computed over the complete data; a document that cannot fit refuses.
func TestCCEV0014DocumentByteCap(t *testing.T) {
	data, err := json.Marshal(gateWindow(30))
	if err != nil {
		t.Fatal(err)
	}
	full, err := Shadow(data)
	if err != nil {
		t.Fatal(err)
	}
	var whole map[string]json.RawMessage
	if err := json.Unmarshal(full, &whole); err != nil {
		t.Fatal(err)
	}
	if _, ok := whole["truncated"]; ok || len(full) > DocumentByteCap {
		t.Fatalf("untruncated document carries a truncated count or exceeds the default cap (%d bytes)", len(full))
	}

	limit := len(full) / 2
	withDocByteCap(t, limit)
	capped, err := Shadow(data)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := Shadow(data)
	if len(capped) > limit || string(capped) != string(again) {
		t.Fatalf("capped document is %d bytes over %d or not deterministic", len(capped), limit)
	}
	var cut struct {
		Entries   []json.RawMessage `json:"entries"`
		Totals    json.RawMessage   `json:"totals"`
		Truncated int               `json:"truncated"`
	}
	var all struct {
		Entries []json.RawMessage `json:"entries"`
		Totals  json.RawMessage   `json:"totals"`
	}
	if err := json.Unmarshal(capped, &cut); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(full, &all); err != nil {
		t.Fatal(err)
	}
	if cut.Truncated == 0 || len(cut.Entries)+cut.Truncated != len(all.Entries) {
		t.Fatalf("kept %d, truncated %d of %d", len(cut.Entries), cut.Truncated, len(all.Entries))
	}
	if !reflect.DeepEqual(cut.Entries, all.Entries[:len(cut.Entries)]) || string(cut.Totals) != string(all.Totals) {
		t.Fatal("truncation must keep a prefix and leave the totals computed over the complete window")
	}

	withDocByteCap(t, 64)
	var r *Refusal
	if _, err := Shadow(data); !errors.As(err, &r) || r.Code != "e2e-document-over-cap" {
		t.Fatalf("over cap with no entries: %v", err)
	}
}

// CCE-V0-014: a consumer refuses an input assembled from a truncated document.
func TestCCEV0014TruncatedInputRefused(t *testing.T) {
	w := gateWindow(30)
	w.Truncated = 3
	data, _ := json.Marshal(w)
	var r *Refusal
	if _, err := Shadow(data); !errors.As(err, &r) || r.Code != "e2e-input-truncated" || !strings.Contains(r.Detail, "3") {
		t.Fatalf("truncated window: %v", err)
	}
}

// CCE-V0-040: capList keeps the longest fitting prefix across every list length.
func TestCCEV0040CapListPrefix(t *testing.T) {
	type doc struct {
		Items     []string `json:"items"`
		Truncated int      `json:"truncated,omitempty"`
	}
	for n := 0; n <= 12; n++ {
		d := doc{}
		for i := range n {
			d.Items = append(d.Items, strings.Repeat("x", i+1))
		}
		items := append([]string(nil), d.Items...)
		withDocByteCap(t, 40)
		out, err := capList(&d.Items, &d.Truncated, func() ([]byte, error) { return json.Marshal(d) })
		if err != nil {
			t.Fatalf("n=%d: %v", n, err)
		}
		if len(out) > 40 || len(d.Items)+d.Truncated != n || !reflect.DeepEqual(d.Items, items[:len(d.Items)]) {
			t.Fatalf("n=%d: %s", n, out)
		}
		if d.Truncated > 0 {
			longer := doc{Items: items[:len(d.Items)+1], Truncated: d.Truncated - 1}
			if b, _ := json.Marshal(longer); len(b) <= 40 {
				t.Fatalf("n=%d: a longer prefix fits: %s", n, b)
			}
		}
	}
}
