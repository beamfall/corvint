package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// railsTestsRepoCLI returns a Rails application repository and a second, test repository that holds
// the manifest (tests.repo "tests"), the map built with --manifest-repo tests --repo tests=ROOT, and
// a workspace declaration binding the same alias.
func railsTestsRepoCLI(t *testing.T) (app, tests, mapFile, workspaceFile string) {
	t.Helper()
	write := func(root string, files map[string]string) {
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
	}
	app, tests = t.TempDir(), t.TempDir()
	write(app, map[string]string{
		"config/routes.rb":                     "Rails.application.routes.draw do\n  resources :orders, only: %i[index]\nend\n",
		"app/controllers/orders_controller.rb": "class OrdersController < ActionController::Base\n  def index\n  end\nend\n",
		"app/views/orders/index.html.erb":      "<a href=\"/orders\">Orders</a>\n",
	})
	write(tests, map[string]string{
		"appmap.json":              `{"schema":"application-map-manifest/0","app":"shop","routers":[{"path":"config/routes.rb","dialect":"rails-routes/0"}],"tests":{"repo":"tests","root":"e2e","specs":["e2e/specs"],"page_objects":["e2e/pages"]}}`,
		"e2e/specs/orders.spec.ts": "test('orders', async ({ page }) => { await page.goto('/orders'); });\n",
		"e2e/pages/orders.page.ts": "export class OrdersPage {\n  constructor(page) { this.page = page; }\n  async open() { await this.page.goto('/orders'); }\n}\n",
	})
	code, out, diag := runFlowsCLI(app, "appmap", "build", "--manifest", "appmap.json", "--manifest-repo", "tests", "--repo", "tests="+tests)
	if code != 0 {
		t.Fatalf("build: %s", diag)
	}
	dir := t.TempDir()
	mapFile, workspaceFile = filepath.Join(dir, "map.json"), filepath.Join(dir, "workspace.json")
	if err := os.WriteFile(mapFile, []byte(out), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workspaceFile, []byte(`{"profile":"corvint-workspace/0","repos":{"tests":{"root":"`+tests+`"}},"manifest_repo":"tests"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return app, tests, mapFile, workspaceFile
}

// WSD-V0-145 SKH-V0-329 SKH-V0-330 (#1933): every source verb reads a map whose manifest lives in
// the aliased test repository (links and setup included), takes --workspace FILE or a discovered
// workspace in place of --repo with the same answer, refuses --workspace with --repo, and parses
// --full after --source ALIAS:DIR.
func TestWSDV0145SourceVerbsWorkspace(t *testing.T) {
	app, tests, mapFile, workspaceFile := railsTestsRepoCLI(t)
	for _, verb := range []string{"refs", "pageobjects", "steps", "setup", "links", "infer", "how"} {
		tail := []string{}
		if verb == "how" {
			tail = []string{"open orders"}
		}
		run := func(args ...string) (int, string, string) {
			return runFlowsCLI(app, append(append(append([]string{"appmap", verb, "--map", mapFile, "--source", "tests:e2e"}, args...), "--full"), tail...)...)
		}
		code, want, diag := run("--repo", "tests="+tests)
		if code != 0 && code != 1 || want == "" {
			t.Fatalf("%s --repo then --full: %d %s", verb, code, diag)
		}
		if code, out, diag := run("--workspace", workspaceFile); out != want {
			t.Fatalf("%s --workspace: %d %s %s", verb, code, out, diag)
		}
		code, _, diag = run("--workspace", workspaceFile, "--repo", "tests="+tests)
		if code != 2 || !strings.Contains(diag, "invalid-arguments") || !strings.Contains(diag, "does not combine") {
			t.Fatalf("%s --workspace with --repo: %d %s", verb, code, diag)
		}
	}
	// A workspace discovered under --root binds the alias; --no-workspace leaves it unbound.
	if err := os.MkdirAll(filepath.Join(app, ".corvint"), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(workspaceFile)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(app, ".corvint", "workspace.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out, diag := runFlowsCLI(app, "appmap", "pageobjects", "--map", mapFile, "--source", "tests:e2e"); code != 0 || out == "" {
		t.Fatalf("discovered workspace: %d %s", code, diag)
	}
	code, _, diag := runFlowsCLI(app, "appmap", "pageobjects", "--map", mapFile, "--source", "tests:e2e", "--no-workspace")
	if code != 2 || !strings.Contains(diag, "appmap-root-unavailable") || !strings.Contains(diag, "--repo tests=") {
		t.Fatalf("--no-workspace: %d %s", code, diag)
	}
}

// SKH-V0-329: links and setup over a map whose manifest is in an aliased root that is not bound
// refuse naming the alias and the flag that binds it, instead of a bare revision error; actions,
// which binds no aliased root, refuses naming the root that holds the manifest.
func TestSKHV0329AliasedManifestUnbound(t *testing.T) {
	app, _, mapFile, _ := railsTestsRepoCLI(t)
	for _, verb := range []string{"links", "setup"} {
		code, _, diag := runFlowsCLI(app, "appmap", verb, "--map", mapFile, "--source", "app/views")
		if code != 2 || !strings.Contains(diag, "appmap-root-unavailable") || !strings.Contains(diag, "--repo tests=ABSOLUTE_ROOT") {
			t.Fatalf("%s: %d %s", verb, code, diag)
		}
	}
	// A verb that binds no aliased root names the root holding the manifest.
	code, _, diag := runFlowsCLI(app, "appmap", "actions", "--map", mapFile)
	if code != 2 || !strings.Contains(diag, "appmap-root-unavailable") || !strings.Contains(diag, "root tests holds the map manifest, and this caller reads --root only") {
		t.Fatalf("actions: %d %s", code, diag)
	}
}

// SKH-V0-330 (#1933): a source verb's flags parse in any order, how's included after its GOAL, and
// a refusal names the flag or argument that failed before the usage.
func TestSKHV0330SourceVerbFlagOrder(t *testing.T) {
	app, tests, mapFile, _ := railsTestsRepoCLI(t)
	repo := "tests=" + tests
	code, want, diag := runFlowsCLI(app, "appmap", "how", "--full", "--map", mapFile, "--repo", repo, "--source", "tests:e2e", "open orders")
	if code != 0 || want == "" {
		t.Fatalf("how flags first: %d %s", code, diag)
	}
	if code, out, diag := runFlowsCLI(app, "appmap", "how", "--map", mapFile, "open orders", "--source", "tests:e2e", "--repo", repo, "--full"); code != 0 || out != want {
		t.Fatalf("how flags after GOAL: %d %s", code, diag)
	}
	for _, tc := range []struct {
		verb  string
		args  []string
		named string
	}{
		{"pageobjects", []string{"--map", mapFile, "--source", "tests:e2e", "--repo", "tests", "--full"}, "--repo"},
		{"steps", []string{"--map", mapFile, "--source", "tests:e2e", "--fulll"}, "--fulll"},
		{"links", []string{"--map", mapFile, "--source", "tests:e2e", "stray"}, "unexpected argument"},
		{"refs", []string{"--source", "tests:e2e", "--full"}, "missing --map"},
		{"how", []string{"--map", mapFile, "--source", "tests:e2e", "--full"}, "missing GOAL"},
		{"how", []string{"--map", mapFile, "a", "b"}, "unexpected argument"},
	} {
		code, _, diag := runFlowsCLI(app, append([]string{"appmap", tc.verb}, tc.args...)...)
		if code != 2 || !strings.Contains(diag, "invalid-arguments") || !strings.Contains(diag, tc.named) || !strings.Contains(diag, "requires --map FILE") {
			t.Fatalf("%s %v: %d %s", tc.verb, tc.args, code, diag)
		}
	}
}
