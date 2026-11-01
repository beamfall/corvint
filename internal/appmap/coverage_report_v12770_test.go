package appmap

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

func v12770Map() *Map {
	screen := func(id, path, status string, abstract bool, bases ...string) Screen {
		s := Screen{ID: id, State: id, Status: status, Abstract: abstract, Anchor: Anchor{Path: path, Start: 1, End: 1}}
		if status != StatusResolved {
			s.Reason = "non-literal-name"
		}
		for _, b := range bases {
			s.Specs = append(s.Specs, Attribution{File: "e2e/" + id + ".spec.ts", Basis: b})
		}
		return s
	}
	return &Map{App: "fixture", Screens: []Screen{
		screen("screen:admin.home", "src/admin/routes.ts", StatusResolved, false, "goto", "import"),
		screen("screen:admin.users", "src/admin/routes.ts", StatusResolved, false),
		screen("screen:admin.root", "src/admin/routes.ts", StatusResolved, true),
		screen("screen:shop.cart", "src/shop/routes.ts", StatusResolved, false, "goto", "goto"),
		screen("screen:shop.audit", "src/shop/routes.ts", StatusResolved, false),
		screen("screen:shop.unknown", "src/shop/routes.ts", StatusUnknown, false),
	}, Files: []TestFile{{ID: "test:e2e/a.spec.ts", Path: "e2e/a.spec.ts", Role: "spec", Join: StatusResolved}}}
}

type v12770Doc struct {
	Schema   string            `json:"schema"`
	TestJoin string            `json:"test_join"`
	Sources  map[string]string `json:"count_sources"`
	Total    int               `json:"screens_total"`
	Reached  int               `json:"screens_reached"`
	Unknown  int               `json:"screens_unknown"`
	Modules  []coverageModule  `json:"modules"`
}

func v12770Project(t *testing.T, m *Map) v12770Doc {
	t.Helper()
	raw, err := ProjectCoverage(context.Background(), m, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var doc v12770Doc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("%v: %s", err, raw)
	}
	return doc
}

func TestAMAPV0601CoverageReport_V12770(t *testing.T) {
	doc := v12770Project(t, v12770Map())
	if doc.Schema != CoverageSchema || doc.TestJoin != StatusResolved || doc.Total != 4 || doc.Reached != 2 || doc.Unknown != 1 {
		t.Fatalf("head: %+v", doc)
	}
	for _, k := range []string{"module", "total", "reached", "reached_by_basis", "unreached", "unknown", "unreached_status"} {
		if doc.Sources[k] == "" {
			t.Fatalf("count %s cites no source: %v", k, doc.Sources)
		}
	}
	want := []coverageModule{
		{Module: "src/admin/routes.ts", Total: 2, Reached: 1, ReachedByBasis: map[string]int{"goto": 1, "import": 1}, Abstract: 1,
			UnreachedStatus: StatusResolved, Unreached: []string{"screen:admin.users"}, Unknown: []coverageUnknown{}},
		{Module: "src/shop/routes.ts", Total: 2, Reached: 1, ReachedByBasis: map[string]int{"goto": 1},
			UnreachedStatus: StatusResolved, Unreached: []string{"screen:shop.audit"},
			Unknown: []coverageUnknown{{Screen: "screen:shop.unknown", Status: StatusUnknown, Reason: "non-literal-name"}}},
	}
	if !reflect.DeepEqual(doc.Modules, want) {
		t.Fatalf("modules:\n got %+v\nwant %+v", doc.Modules, want)
	}

	m := v12770Map()
	m.Files[0].Join = StatusUnknown
	doc = v12770Project(t, m)
	if doc.TestJoin != StatusUnknown {
		t.Fatalf("test_join: %s", doc.TestJoin)
	}
	for _, row := range doc.Modules {
		if row.UnreachedStatus != StatusUnknown || row.UnreachedReason != "test-join-unknown" || len(row.Unreached) != 1 {
			t.Fatalf("unknown join keeps unreached definite: %+v", row)
		}
	}

	if doc = v12770Project(t, &Map{App: "empty"}); len(doc.Modules) != 0 || doc.Total != 0 {
		t.Fatalf("empty map: %+v", doc)
	}
}
