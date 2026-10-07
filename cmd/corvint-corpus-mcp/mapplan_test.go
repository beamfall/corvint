package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

// mapPlanRoot adds the application-map planner fixture to a corpus fixture repository, commits
// it, and writes both built maps beside it as untracked files.
func mapPlanRoot(t *testing.T) (string, []*appmap.Map) {
	t.Helper()
	root, _ := corpusServerFixture(t, "# Evidence\n\nOriginal source.\n")
	for _, dir := range []string{"fixture", "plan"} {
		src := filepath.Join("..", "..", "internal", "appmap", "testdata", dir)
		err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(src, p)
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			if err = os.MkdirAll(filepath.Join(root, filepath.Dir(rel)), 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(root, rel), data, 0o644)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"add", "-A", ":!corpus.json"}, {"-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-qm", "maps"}} {
		c := exec.Command("git", args...)
		c.Dir, c.Env = root, append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	maps := []*appmap.Map{}
	for _, manifest := range []string{"appmap.json", "marketplace.json"} {
		m, err := appmap.Build(context.Background(), root, manifest, "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		data, err := appmap.Encode(m)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(root, "map-"+manifest), data, 0o644); err != nil {
			t.Fatal(err)
		}
		maps = append(maps, m)
	}
	return root, maps
}

// AMSP-V0-010: corvint-corpus-mcp lists corvint.map_plan only when maps are configured, serves
// the library's plan bytes as structured content inside the untrusted-data envelope, refuses
// arguments outside its schema, and reports planner refusals as tool errors.
func TestAMSPV0010CorpusMCPMapPlan(t *testing.T) {
	root, maps := mapPlanRoot(t)
	for _, c := range []struct {
		args []string
		ok   bool
		maps int
	}{
		{[]string{"--root", root, "--artifact", "corpus.json"}, true, 0},
		{[]string{"--root", root, "--artifact", "corpus.json", "--map", "a.json", "--map", "b.json"}, true, 2},
		{[]string{"--root", root, "--artifact", "corpus.json", "--map"}, false, 0},
		{[]string{"--root", root, "--artifact", "corpus.json", "--map", ""}, false, 0},
		{[]string{"--root", root, "--artifact", "corpus.json", "--maps", "a.json"}, false, 0},
		{append([]string{"--root", root, "--artifact", "corpus.json"}, strings.Split(strings.Repeat("--map x ", 9), " ")[:18]...), false, 0},
	} {
		_, _, got, _, ok := parseArguments(c.args)
		if ok != c.ok || len(got) != c.maps {
			t.Errorf("parseArguments(%q) = %v %v", c.args, got, ok)
		}
	}
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.Symlink(filepath.Join(root, "map-appmap.json"), outside); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.json")); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "external.json")
	data, _ := os.ReadFile(filepath.Join(root, "map-appmap.json"))
	if err := os.WriteFile(external, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(root, "leak.json")); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"../map-appmap.json", filepath.Join(root, "map-appmap.json"), "missing.json", "leak.json", "corpus.json"} {
		if _, err := newMapPlanner(root, []string{bad}); err == nil {
			t.Errorf("map %q accepted", bad)
		}
	}
	var stderr bytes.Buffer
	if code := run(context.Background(), []string{"--root", root, "--artifact", "corpus.json", "--map", "missing.json"}, strings.NewReader(""), &bytes.Buffer{}, &stderr); code != 2 ||
		!strings.Contains(stderr.String(), "application map unavailable") {
		t.Fatalf("missing map: %d %s", code, stderr.String())
	}

	registry, e := corpusbridge.New(root, "corpus.json")
	if e != nil {
		t.Fatal(e)
	}
	listed := func(h *toolHandler) bool {
		result, rpc := h.Handle(context.Background(), protocol.Request{Method: "tools/list", Params: map[string]any{}}, nil)
		if rpc != nil {
			t.Fatal(rpc)
		}
		raw, _ := json.Marshal(result["tools"])
		return strings.Contains(string(raw), `"name":"corvint.map_plan"`)
	}
	if listed(&toolHandler{registry: registry}) {
		t.Fatal("map_plan listed without maps")
	}
	planner, err := newMapPlanner(root, []string{"map-marketplace.json", "map-appmap.json"})
	if err != nil {
		t.Fatal(err)
	}
	h := &toolHandler{registry: registry, planner: planner}
	if !listed(h) {
		t.Fatal("map_plan not listed")
	}
	request := "book a tee time; check the slot status; buy a gift card; change the club settings"
	result, rpc := h.call(context.Background(), map[string]any{"name": mapPlanTool, "arguments": map[string]any{"request": request, "draft": true}})
	if rpc != nil || result["isError"] != false {
		t.Fatalf("call: %#v %v", result, rpc)
	}
	want, err := appmap.Plan(context.Background(), maps, appmap.SplitRequest(request), appmap.PlanOptions{Options: appmap.Options{Root: root}, Draft: true})
	if err != nil {
		t.Fatal(err)
	}
	var wantObject map[string]any
	_ = json.Unmarshal(want, &wantObject)
	if !reflect.DeepEqual(result["structuredContent"], wantObject) {
		t.Fatal("structured content differs from the library plan")
	}
	framed, _ := repoenvelope.Frame(string(want))
	if result["content"].([]any)[0].(map[string]any)["text"] != framed {
		t.Fatal("plan text not framed as untrusted data")
	}
	if wantObject["status"] != "COMPLETE" || !strings.Contains(string(want), "await test.step(") {
		t.Fatalf("plan: %s", want)
	}
	seventeen := []any{}
	for range 17 {
		seventeen = append(seventeen, "book a tee time")
	}
	for _, args := range []map[string]any{
		{},
		{"request": request, "steps": []any{"x"}},
		{"request": request, "steps": []any{}},
		{"request": request, "steps": nil},
		{"request": request, "extra": true},
		{"request": request, "budget": 1.5},
		{"request": request, "budget": 0},
		{"request": request, "budget": 1},
		{"request": request, "budget": nil},
		{"request": request, "budget": "4096"},
		{"request": request, "full": true},
		{"request": request, "draft": nil},
		{"request": request, "revision": nil},
		{"request": nil},
		{"request": ""},
		{"steps": []any{}},
		{"steps": seventeen},
		{"steps": []any{""}},
		{"steps": []any{strings.Repeat("x", 513)}},
		{"steps": []any{1}},
		{"request": request, "revision": ""},
	} {
		if _, rpc := h.call(context.Background(), map[string]any{"name": mapPlanTool, "arguments": args}); rpc == nil {
			t.Errorf("arguments %v accepted", args)
		}
	}
	var tool map[string]any
	result, _ = h.Handle(context.Background(), protocol.Request{Method: "tools/list", Params: map[string]any{}}, nil)
	raw, _ := json.Marshal(result["tools"])
	var tools []map[string]any
	_ = json.Unmarshal(raw, &tools)
	for _, td := range tools {
		if td["name"] == mapPlanTool {
			tool = td
		}
	}
	if _, full := tool["inputSchema"].(map[string]any)["properties"].(map[string]any)["full"]; full {
		t.Fatal("map_plan advertises an unbounded full escape")
	}
	// The largest plan the tool can return, framed and copied into structured content, fits one
	// MCP message.
	big, rpc := h.call(context.Background(), map[string]any{"name": mapPlanTool, "arguments": map[string]any{"steps": seventeen[:16], "budget": appmap.MaxBudget, "draft": true}})
	if rpc != nil || big["isError"] != false {
		t.Fatalf("max-budget plan: %v %v", big["isError"], rpc)
	}
	if encoded, _ := json.Marshal(big); len(encoded) >= protocol.MaxMessageBytes {
		t.Fatalf("max-budget response is %d bytes", len(encoded))
	}
	refused, rpc := h.call(context.Background(), map[string]any{"name": mapPlanTool, "arguments": map[string]any{"request": request, "budget": 300}})
	if rpc != nil || refused["isError"] != true || refused["structuredContent"].(map[string]any)["code"] != "appmap-budget-too-small" {
		t.Fatalf("budget refusal: %#v %v", refused, rpc)
	}
}
