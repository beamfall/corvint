package appmap

import (
	"context"
	"encoding/json"
	"testing"
)

type statsDoc struct {
	Total, Resolved, Unknown int
	Omitted                  int
	Reasons                  []statsReason
	Edges                    []statsCount
}

func statsParse(t *testing.T, raw []byte) (map[string]any, []statsReason) {
	t.Helper()
	var doc struct {
		Reasons []statsReason `json:"unknown_reasons"`
	}
	var head map[string]any
	if json.Unmarshal(raw, &doc) != nil || json.Unmarshal(raw, &head) != nil {
		t.Fatalf("bad json: %s", raw)
	}
	return head, doc.Reasons
}

// AMAP-V0-060 AMAP-V0-061 AMAP-V0-062 AMAP-V0-063: counts reconcile with the map, reasons are
// ordered and capped, and the report is deterministic and reads no repository.
func TestAMAPV0060StatsReconcile(t *testing.T) {
	_, m := entriesFixture(t)
	raw, err := ProjectStats(context.Background(), m, Options{Full: true})
	if err != nil {
		t.Fatal(err)
	}
	again, _ := ProjectStats(context.Background(), m, Options{Full: true})
	if string(raw) != string(again) {
		t.Fatal("not deterministic")
	}
	head, reasons := statsParse(t, raw)
	resolved := 0
	for _, s := range m.Screens {
		if s.Status == StatusResolved {
			resolved++
		}
	}
	if int(head["screens_total"].(float64)) != len(m.Screens) || int(head["screens_resolved"].(float64)) != resolved ||
		int(head["screens_unknown"].(float64)) != len(m.Screens)-resolved {
		t.Fatalf("screens: %v", head)
	}
	want := len(m.Unknowns)
	if int(head["unknown_total"].(float64)) != want {
		t.Fatalf("unknown_total %v want %d", head["unknown_total"], want)
	}
	sum := 0
	for i, r := range reasons {
		sum += r.Count
		if i > 0 && reasons[i-1].Count < r.Count {
			t.Fatalf("order: %+v", reasons)
		}
		if len(r.Examples) == 0 || len(r.Examples) > statsExamplesEach {
			t.Fatalf("examples: %+v", r)
		}
	}
	if len(reasons) > statsTopReasons {
		t.Fatalf("not capped: %d", len(reasons))
	}
	if omitted := int(head["unknown_reasons_omitted"].(float64)); omitted == 0 && sum != want {
		t.Fatalf("reasons %d != unknown_total %d", sum, want)
	}
	edges := 0
	var doc struct {
		Edges []statsCount `json:"edges_by_basis"`
	}
	_ = json.Unmarshal(raw, &doc)
	for _, e := range doc.Edges {
		edges += e.Count
	}
	if edges != len(m.Edges) || int(head["edges_total"].(float64)) != len(m.Edges) {
		t.Fatalf("edges %d/%d", edges, len(m.Edges))
	}
	if head["evaluated_revision"] != "NOT_READ" {
		t.Fatalf("read the repository: %v", head["evaluated_revision"])
	}
	capped, err := ProjectStats(context.Background(), m, Options{Budget: 2048})
	if err != nil || len(capped) > 2048 {
		t.Fatalf("budget: %d %v", len(capped), err)
	}
	for _, o := range []Options{{Budget: 255}, {Budget: 65537}, {Budget: 4096, Full: true}} {
		if _, err := ProjectStats(context.Background(), m, o); err == nil {
			t.Fatalf("admitted %+v", o)
		}
	}
}

// AMAP-V0-064: the dominant reasons and an empty map.
func TestAMAPV0064StatsSummary(t *testing.T) {
	_, m := entriesFixture(t)
	s := Stats(m)
	if s.Total != len(m.Screens) || s.Resolved >= s.Total || len(s.Dominant) == 0 || len(s.Dominant) > statsTopReasons {
		t.Fatalf("%+v", s)
	}
	if got := Stats(&Map{}).ResolvedPercent(); got != 0 {
		t.Fatalf("empty map %v", got)
	}
}
