package appmap

import (
	"reflect"
	"strings"
	"testing"
)

// AMAP-V0-210: each map unknown a declaration can resolve names its manifest key with the
// candidates the map infers, offered and never chosen; anything else states its uncertainty and
// names no key, so no remedy is invented.
func TestAMAPV0210UnknownRemedy(t *testing.T) {
	m := &Map{Manifest: Anchor{Path: "appmap.json"},
		Screens: []Screen{{ID: "screen:a:home", State: "home"}, {ID: "screen:a:x", State: "x"}},
		Files: []TestFile{
			{Path: "e2e/pages/both.page.ts", Gotos: []Goto{{Line: 3, Screen: "screen:a:x"}, {Line: 5, Screen: "screen:a:home"}}},
			{Path: "e2e/pages/lost.page.ts", Gotos: []Goto{{Line: 4, Screen: "screen:a:home"}, {Line: 9}}},
			{Path: "e2e/pages/none.page.ts"},
		}}
	for _, c := range []struct {
		u    Unknown
		want Remedy
	}{
		{Unknown{Kind: "page-object", Reason: "page-object-ambiguous", Path: "e2e/pages/both.page.ts"}, Remedy{File: "appmap.json", Key: "page_object_screens",
			Candidates: []string{`{"path":"e2e/pages/both.page.ts","state":"home"}`, `{"path":"e2e/pages/both.page.ts","state":"x"}`},
			Basis:      "resolved navigation at lines 3,5", Uncertainty: "the page object reaches 2 screens; declare the one it models"}},
		{Unknown{Kind: "page-object", Reason: "page-object-unresolved-target", Path: "e2e/pages/lost.page.ts"}, Remedy{File: "appmap.json", Key: "page_object_screens",
			Candidates: []string{`{"path":"e2e/pages/lost.page.ts","state":"home"}`}, Basis: "resolved navigation at lines 4",
			Uncertainty: "navigation at lines 9 resolves to no screen; the screen it models is not inferred"}},
		{Unknown{Kind: "page-object", Reason: "page-object-unbound", Path: "e2e/pages/none.page.ts"}, Remedy{File: "appmap.json", Key: "page_object_screens",
			Uncertainty: "the page object navigates nowhere the map resolves; the screen it models is not inferred"}},
		{Unknown{Kind: "page-object", Reason: "page-object-unbound", Path: "e2e/pages/gone.page.ts"}, Remedy{File: "appmap.json", Key: "page_object_screens",
			Uncertainty: "page_object_screens declares e2e/pages/gone.page.ts but the map holds no such page object; correct or remove the entry"}},
		{Unknown{Kind: "router", Reason: "router-not-declared", Path: "app/more.routes.js"}, Remedy{File: "appmap.json", Key: "routers",
			Candidates: []string{"app/more.routes.js"}, Basis: "a router_globs file the declared routers do not name",
			Uncertainty: "the router dialect of app/more.routes.js is not inferred; declare it or narrow router_globs"}},
		{Unknown{Kind: "screen", Reason: "non-literal-url", Path: "app/routes.js"}, Remedy{Uncertainty: "no declaration change is inferred for screen non-literal-url"}},
	} {
		if got := UnknownRemedy(m, c.u); !reflect.DeepEqual(*got, c.want) {
			t.Errorf("%s %s:\n got %+v\nwant %+v", c.u.Reason, c.u.Path, *got, c.want)
		}
	}
}

// AMAP-V0-210: an impact unknown carries a remedy; an unmapped path takes the keyed remedy of a
// map unknown at the same path, and every other reason states its uncertainty without a key.
func TestAMAPV0210ImpactRemedy(t *testing.T) {
	m := &Map{Manifest: Anchor{Path: "appmap.json"}, Unknowns: []Unknown{{Kind: "router", Reason: "router-not-declared", Path: "app/more.routes.js"}}}
	if r := impactRemedy(m, ImpactUnknown{Path: "app/more.routes.js", Reason: "unmapped-path"}); r.Key != "routers" || r.Candidates[0] != "app/more.routes.js" {
		t.Fatalf("router remedy %+v", r)
	}
	for _, reason := range []string{"manifest-changed", "non-ui-path", "test-support-unreached", "test-import-graph-incomplete", "unmapped-path", "unresolved-reference", "reference-graph-bound"} {
		r := impactRemedy(m, ImpactUnknown{Path: "app/x.html", Reason: reason, Detail: "d"})
		if r.Key != "" || r.File != "" || len(r.Candidates) != 0 || r.Uncertainty == "" {
			t.Errorf("%s: %+v", reason, r)
		}
	}
	if r := impactRemedy(m, ImpactUnknown{Path: "app/x.html", Reason: "unresolved-reference", Detail: "app/a.js:3"}); !strings.Contains(r.Uncertainty, "app/a.js:3") {
		t.Fatalf("detail not carried: %+v", r)
	}
}
