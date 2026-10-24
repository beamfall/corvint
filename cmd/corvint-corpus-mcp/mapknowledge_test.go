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
		// The fixture has no setup records or static links.
		pinned := view == "setup" || view == "links" || strings.Contains(string(want), `"blob":`)
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
