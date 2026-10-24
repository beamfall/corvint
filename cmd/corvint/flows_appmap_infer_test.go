package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/doccorpus"
)

// inferCLIRepo commits a Rails resource router and one Playwright spec, and builds its map.
func inferCLIRepo(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"appmap.json":              `{"schema":"application-map-manifest/0","app":"shop","routers":[{"path":"config/routes.rb","dialect":"rails-routes/0"}],"tests":{"root":"e2e","specs":["e2e/specs"],"page_objects":["e2e/pages"]}}`,
		"e2e/pages/orders.page.ts": "export const orders = '/orders';\n",
		"config/routes.rb":         "Rails.application.routes.draw do\n  resources :orders\nend\n",
		"e2e/specs/ship.spec.ts": "import { test, expect } from '@playwright/test';\n" +
			"test('ships', async ({ page }) => {\n  await page.goto('/orders');\n  await page.getByRole('button', { name: 'Ship' }).click();\n" +
			"  await page.goto('/orders/1');\n  await expect(page.getByText('Shipped')).toBeVisible();\n});\n",
		"flows/retired.json": `{"schema":"application-flow-retired/1","flow_ids":[],"variation_ids":[]}` + "\n",
	}
	for name, text := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "fixture"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = root, append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, b)
		}
	}
	code, out, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json")
	if code != 0 {
		t.Fatalf("build: %s", diag)
	}
	filename := filepath.Join(t.TempDir(), "map.json")
	if err := os.WriteFile(filename, []byte(out), 0o600); err != nil {
		t.Fatal(err)
	}
	return root, filename
}

// SKH-V0-051 SKH-V0-054 SKH-V0-120 SKH-V0-125 SKH-V0-126: infer prints the candidate report and,
// with --emit request, an adapter request; both rerun byte-identically and change no repository
// file; flows import of the request writes one proposed intent; invalid input exits 2.
func TestSKHV0_126_InferCLIAndImport(t *testing.T) {
	root, filename := inferCLIRepo(t)
	code, out, diag := runFlowsCLI(root, "appmap", "infer", "--map", filename, "--source", "e2e")
	var doc struct {
		Schema     string
		Candidates []struct {
			FlowID string `json:"flow_id"`
			Emit   string
		}
	}
	if code != 0 || json.Unmarshal([]byte(out), &doc) != nil || doc.Schema != appmap.FlowsSchema || len(doc.Candidates) != 1 ||
		doc.Candidates[0].Emit != "emitted" || duplicateTopLevelKey(out) != "" {
		t.Fatalf("infer: %d %s %s", code, diag, out)
	}
	if _, again, _ := runFlowsCLI(root, "appmap", "infer", "--map", filename, "--source", "e2e"); again != out {
		t.Fatal("infer rerun differs")
	}
	code, request, diag := runFlowsCLI(root, "appmap", "infer", "--map", filename, "--source", "e2e", "--emit", "request")
	if code != 0 || !strings.Contains(request, `"schema":"`+doccorpus.BehaviorAdapterRequestSchema+`"`) {
		t.Fatalf("request: %d %s %s", code, diag, request)
	}
	if _, again, _ := runFlowsCLI(root, "appmap", "infer", "--map", filename, "--source", "e2e", "--emit", "request"); again != request {
		t.Fatal("request rerun differs")
	}
	assertCleanRepo(t, root)
	for _, args := range [][]string{
		{"--map", filename},
		{"--map", filename, "--source", "e2e", "--emit", "intents"},
		{"--map", filename, "--source", "e2e", "--budget", "0"},
		{"--map", filename, "--source", "e2e", "extra"},
		{"--map", filename, "--source", "e2e", "--corpus", "missing.json"},
	} {
		if code, out, _ := runFlowsCLI(root, append([]string{"appmap", "infer"}, args...)...); code != 2 || out != "" {
			t.Fatalf("admitted %v: %d %s", args, code, out)
		}
	}
	from := filepath.Join(t.TempDir(), "request.json")
	if err := os.WriteFile(from, []byte(request), 0o600); err != nil {
		t.Fatal(err)
	}
	code, written, diag := runFlowsCLI(root, "import", "--flows", "flows", "--from", from, "--format", "behavior-adapter-request")
	if code != 0 || strings.TrimSpace(written) != "flows/"+doc.Candidates[0].FlowID+".json" {
		t.Fatalf("import: %d %s %s", code, written, diag)
	}
	raw, err := os.ReadFile(filepath.Join(root, "flows", doc.Candidates[0].FlowID+".json"))
	var intent struct {
		Proposed bool
		Steps    []struct{ Action string }
	}
	if err != nil || json.Unmarshal(raw, &intent) != nil || !intent.Proposed || len(intent.Steps) != 3 {
		t.Fatalf("imported intent %s %v", raw, err)
	}
}

// SKH-V0-126 SKH-V0-052: only joined records on the corpus repository become trust inputs, with
// their test anchor span, trust level, verdict, selection and gap.
func TestSKHV0_126_TrustFromJoins(t *testing.T) {
	repo := strings.Repeat("a", 40)
	record := func(path string, repository string) doccorpus.TestKnowledgeRecord {
		return doccorpus.TestKnowledgeRecord{Test: doccorpus.Anchor{Repository: repository, Path: path, Start: 3, End: 9}, Gap: "g"}
	}
	got := trustFromJoins([]doccorpus.TestKnowledgeJoin{
		{ID: "tk:a", Status: "joined", TrustLevel: "run+reviewed", Verdict: "partial", Selection: "candidate", Record: record("e2e/a.spec.ts", repo)},
		{ID: "tk:stale", Status: "stale", Record: record("e2e/a.spec.ts", repo)},
		{ID: "tk:other", Status: "joined", Record: record("e2e/a.spec.ts", strings.Repeat("b", 40))},
	}, repo)
	want := []appmap.TestTrust{{ID: "tk:a", Path: "e2e/a.spec.ts", Start: 3, End: 9, TrustLevel: "run+reviewed", Verdict: "partial", Selection: "candidate", Gap: "g"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("trust = %+v", got)
	}
}
