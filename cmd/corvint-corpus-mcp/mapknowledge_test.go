package main

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

// SKH-V0-013: corvint-corpus-mcp serves the page-object and step-IR projections of a configured map
// as the library's bytes (pinned to the map revision and source spans) inside the untrusted-data
// envelope, lists the tool only with maps, refuses arguments outside its schema, and reports
// projection refusals as tool errors.
func TestSKHV0013CorpusMCPMapKnowledge(t *testing.T) {
	root, maps := mapPlanRoot(t)
	registry, e := corpusbridge.New(root, "corpus.json")
	if e != nil {
		t.Fatal(e)
	}
	listed := func(h *toolHandler) bool {
		raw, _ := json.Marshal(h.listedTools())
		return strings.Contains(string(raw), `"name":"corvint.map_knowledge"`)
	}
	if listed(&toolHandler{registry: registry}) {
		t.Fatal("map_knowledge listed without maps")
	}
	planner, err := newMapPlanner(root, []string{"map-marketplace.json", "map-appmap.json"})
	if err != nil {
		t.Fatal(err)
	}
	h := &toolHandler{registry: registry, planner: planner}
	if !listed(h) {
		t.Fatal("map_knowledge not listed")
	}
	admin := maps[0]
	for view, project := range map[string]func(context.Context, *appmap.Map, []string, appmap.Options) ([]byte, error){
		"page_objects": appmap.PageObjects, "steps": appmap.Steps, "setup": appmap.Setup, "links": appmap.Links,
		"flow_candidates": func(ctx context.Context, m *appmap.Map, s []string, o appmap.Options) ([]byte, error) {
			return appmap.InferFlows(ctx, m, s, nil, o)
		},
	} {
		result, rpc := h.call(context.Background(), map[string]any{"name": mapKnowledgeTool,
			"arguments": map[string]any{"view": view, "sources": []any{"e2e"}, "app": "admin"}})
		if rpc != nil || result["isError"] != false {
			t.Fatalf("%s: %#v %v", view, result, rpc)
		}
		want, err := project(context.Background(), admin, []string{"e2e"}, appmap.Options{Root: root})
		if err != nil {
			t.Fatal(err)
		}
		var wantObject map[string]any
		_ = json.Unmarshal(want, &wantObject)
		if !reflect.DeepEqual(result["structuredContent"], wantObject) {
			t.Fatalf("%s: structured content differs from the library projection", view)
		}
		if framed, _ := repoenvelope.Frame(string(want)); result["content"].([]any)[0].(map[string]any)["text"] != framed {
			t.Fatalf("%s: text not framed as untrusted data", view)
		}
		// The fixture has no setup records, static links or candidate flows.
		pinned := view == "setup" || view == "links" || view == "flow_candidates" || strings.Contains(string(want), `"blob":`)
		if wantObject["map_revision"] != admin.Revision || wantObject["schema"] == nil || !pinned {
			t.Fatalf("%s: projection lacks its evidence pins: %s", view, want)
		}
	}
	for _, args := range []map[string]any{
		{"view": "page_objects", "sources": []any{"e2e"}},
		{"view": "page_objects", "sources": []any{"e2e"}, "app": "nope"},
		{"view": "page_objects", "sources": []any{"../e2e"}, "app": "admin"},
	} {
		result, rpc := h.call(context.Background(), map[string]any{"name": mapKnowledgeTool, "arguments": args})
		if rpc != nil || result["isError"] != true || !strings.Contains(result["content"].([]any)[0].(map[string]any)["text"].(string), "appmap-invalid-query") {
			t.Errorf("arguments %v: %#v %v", args, result, rpc)
		}
	}
	seventeen := []any{}
	for range 17 {
		seventeen = append(seventeen, "e2e")
	}
	for _, args := range []map[string]any{
		{},
		{"view": "page_objects"},
		{"sources": []any{"e2e"}},
		{"view": "flows", "sources": []any{"e2e"}},
		{"view": nil, "sources": []any{"e2e"}},
		{"view": "steps", "sources": []any{}},
		{"view": "steps", "sources": seventeen},
		{"view": "steps", "sources": []any{""}},
		{"view": "steps", "sources": []any{1}},
		{"view": "steps", "sources": []any{strings.Repeat("x", 513)}},
		{"view": "steps", "sources": []any{"e2e"}, "app": ""},
		{"view": "steps", "sources": []any{"e2e"}, "budget": 1.5},
		{"view": "steps", "sources": []any{"e2e"}, "budget": 1},
		{"view": "steps", "sources": []any{"e2e"}, "budget": "4096"},
		{"view": "steps", "sources": []any{"e2e"}, "extra": true},
	} {
		if _, rpc := h.call(context.Background(), map[string]any{"name": mapKnowledgeTool, "arguments": args}); rpc == nil {
			t.Errorf("arguments %v accepted", args)
		}
	}
}

// SKH-V0-060 SKH-V0-143: the how view serves the library's site-knowledge-how/0 bytes for a goal
// and its conditions, framed as untrusted data; goal is required for how and refused on any other
// view, and conditions outside the bounds are refused.
func TestSKHV0_143_MapKnowledgeHowView(t *testing.T) {
	root, maps := mapPlanRoot(t)
	registry, e := corpusbridge.New(root, "corpus.json")
	if e != nil {
		t.Fatal(e)
	}
	planner, err := newMapPlanner(root, []string{"map-marketplace.json", "map-appmap.json"})
	if err != nil {
		t.Fatal(err)
	}
	h := &toolHandler{registry: registry, planner: planner}
	result, rpc := h.call(context.Background(), map[string]any{"name": mapKnowledgeTool,
		"arguments": map[string]any{"view": "how", "sources": []any{"e2e"}, "app": "admin", "goal": "open the dashboard", "conditions": []any{"a user exists"}}})
	if rpc != nil || result["isError"] != false {
		t.Fatalf("how: %#v %v", result, rpc)
	}
	want, err := appmap.How(context.Background(), maps[0], []string{"e2e"}, "open the dashboard", []string{"a user exists"}, nil, nil, appmap.Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	var wantObject map[string]any
	_ = json.Unmarshal(want, &wantObject)
	if !reflect.DeepEqual(result["structuredContent"], wantObject) || wantObject["schema"] != "site-knowledge-how/0" {
		t.Fatalf("how view differs from the library answer: %s", want)
	}
	if framed, _ := repoenvelope.Frame(string(want)); result["content"].([]any)[0].(map[string]any)["text"] != framed {
		t.Fatal("how text not framed as untrusted data")
	}
	seventeen := []any{}
	for range 17 {
		seventeen = append(seventeen, "x")
	}
	for _, args := range []map[string]any{
		{"view": "how", "sources": []any{"e2e"}},
		{"view": "steps", "sources": []any{"e2e"}, "goal": "x"},
		{"view": "steps", "sources": []any{"e2e"}, "conditions": []any{"x"}},
		{"view": "how", "sources": []any{"e2e"}, "goal": ""},
		{"view": "how", "sources": []any{"e2e"}, "goal": strings.Repeat("x", 257)},
		{"view": "how", "sources": []any{"e2e"}, "goal": "x", "conditions": nil},
		{"view": "how", "sources": []any{"e2e"}, "goal": "x", "conditions": []any{""}},
		{"view": "how", "sources": []any{"e2e"}, "goal": "x", "conditions": seventeen},
	} {
		if _, rpc := h.call(context.Background(), map[string]any{"name": mapKnowledgeTool, "arguments": args}); rpc == nil {
			t.Errorf("arguments %v accepted", args)
		}
	}
}

// SKH-V0-162: corvint.map_plan with sources, request and draft returns the library's how draft,
// framed as untrusted data; sources without draft, with steps or revision, and conditions or app
// without sources are refused as invalid arguments.
func TestSKHV0_162_MapPlanHowDraft(t *testing.T) {
	root, maps := mapPlanRoot(t)
	registry, e := corpusbridge.New(root, "corpus.json")
	if e != nil {
		t.Fatal(e)
	}
	planner, err := newMapPlanner(root, []string{"map-marketplace.json", "map-appmap.json"})
	if err != nil {
		t.Fatal(err)
	}
	h := &toolHandler{registry: registry, planner: planner}
	result, rpc := h.call(context.Background(), map[string]any{"name": mapPlanTool,
		"arguments": map[string]any{"request": "open the dashboard", "draft": true, "sources": []any{"e2e"}, "app": "admin", "conditions": []any{"a user exists"}}})
	if rpc != nil || result["isError"] != false {
		t.Fatalf("draft: %#v %v", result, rpc)
	}
	want, err := appmap.HowDraft(context.Background(), maps[0], []string{"e2e"}, "open the dashboard", []string{"a user exists"}, nil, nil, appmap.Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	var wantObject map[string]any
	_ = json.Unmarshal(want, &wantObject)
	if !reflect.DeepEqual(result["structuredContent"], wantObject) || wantObject["draft"] == nil {
		t.Fatalf("map_plan draft differs from the library answer: %s", want)
	}
	if framed, _ := repoenvelope.Frame(string(want)); result["content"].([]any)[0].(map[string]any)["text"] != framed {
		t.Fatal("draft text not framed as untrusted data")
	}
	for _, args := range []map[string]any{
		{"request": "x", "sources": []any{"e2e"}},
		{"request": "x", "draft": false, "sources": []any{"e2e"}},
		{"steps": []any{"x"}, "draft": true, "sources": []any{"e2e"}},
		{"request": "x", "draft": true, "sources": []any{"e2e"}, "revision": "HEAD"},
		{"request": "x", "draft": true, "sources": []any{}},
		{"request": "x", "draft": true, "sources": []any{"e2e"}, "conditions": nil},
		{"request": "x", "conditions": []any{"y"}},
		{"request": "x", "app": "admin"},
		{"request": strings.Repeat("x", 257), "draft": true, "sources": []any{"e2e"}},
	} {
		if _, rpc := h.call(context.Background(), map[string]any{"name": mapPlanTool, "arguments": args}); rpc == nil {
			t.Errorf("arguments %v accepted", args)
		}
	}
}
