package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// railsCLIRepo commits a small Rails application whose orders controller lacks the create action
// the routes declare, and returns its root with a built map file.
func railsCLIRepo(t *testing.T, create bool) (string, string) {
	t.Helper()
	root := t.TempDir()
	controller := "class OrdersController < ActionController::Base\n  def index\n    head :forbidden if @order.locked\n  end\n"
	if create {
		controller += "\n  def create\n  end\n"
	}
	files := map[string]string{
		"appmap.json":                          `{"schema":"application-map-manifest/0","app":"shop","routers":[{"path":"config/routes.rb","dialect":"rails-routes/0"}],"tests":{"root":"e2e","specs":["e2e/specs"],"page_objects":["e2e/pages"]}}`,
		"config/routes.rb":                     "Rails.application.routes.draw do\n  resources :orders, only: %i[index create]\nend\n",
		"app/controllers/orders_controller.rb": controller + "end\n",
		"app/models/order.rb":                  "class Order < ActiveRecord::Base\nend\n",
		"spec/factories/orders.rb":             "FactoryBot.define do\n  factory :order do\n    locked { true }\n  end\nend\n",
		"e2e/specs/orders.spec.ts":             "test('orders', async ({ page }) => { await page.goto('/orders'); });\n",
		"e2e/pages/orders.page.ts":             "export const orders = '/orders';\n",
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

func assertCleanRepo(t *testing.T, root string) {
	t.Helper()
	cmd := exec.Command("git", "status", "--porcelain", "--ignored")
	cmd.Dir = root
	if status, err := cmd.Output(); err != nil || len(status) != 0 {
		t.Fatalf("mutated repo: %s %v", status, err)
	}
}

// AMAP-V0-100 AMAP-V0-106: actions writes the bounded report, exits 1 on a ROUTE-ACTION-MISSING
// finding and 0 without one, refuses invalid input with exit 2 and changes no repository file.
func TestAMAPV0106ActionsCLI(t *testing.T) {
	root, filename := railsCLIRepo(t, false)
	code, out, diag := runFlowsCLI(root, "appmap", "actions", "--map", filename)
	var doc map[string]any
	if code != 1 || json.Unmarshal([]byte(out), &doc) != nil || doc["schema"] != "application-map-actions/0" || doc["missing"] != float64(1) ||
		len(out) > 6144 || !strings.Contains(diag, "1 ROUTE-ACTION-MISSING findings") || !strings.Contains(out, `"declaration":"config/routes.rb:2"`) {
		t.Fatalf("actions: %d %s %s", code, diag, out)
	}
	for _, tc := range []struct {
		code string
		args []string
	}{
		{"invalid-arguments", []string{}},
		{"appmap-invalid-query", []string{"--map", filename, "--budget", "1"}},
		{"appmap-invalid-query", []string{"--map", filename, "--budget", "0"}},
		{"appmap-invalid-query", []string{"--map", filename, "--full", "--budget", "4096"}},
		{"invalid-arguments", []string{"--map", filename, "extra"}},
	} {
		code, out, diag = runFlowsCLI(root, append([]string{"appmap", "actions"}, tc.args...)...)
		if code != 2 || out != "" || !strings.Contains(diag, tc.code) {
			t.Fatalf("admitted %v: %d %s %s", tc.args, code, out, diag)
		}
	}
	assertCleanRepo(t, root)
	root, filename = railsCLIRepo(t, true)
	if code, out, diag = runFlowsCLI(root, "appmap", "actions", "--map", filename, "--full"); code != 0 || !strings.Contains(out, `"missing":0`) {
		t.Fatalf("defined: %d %s %s", code, diag, out)
	}
}

// AMAP-V0-110 AMAP-V0-116: branches writes the bounded report and exits 0, refuses invalid cites
// with exit 2 and changes no repository file.
func TestAMAPV0116BranchesCLI(t *testing.T) {
	root, filename := railsCLIRepo(t, false)
	code, out, diag := runFlowsCLI(root, "appmap", "branches", "--map", filename, "--cite", "app/controllers/orders_controller.rb:2-4")
	var doc map[string]any
	if code != 0 || json.Unmarshal([]byte(out), &doc) != nil || doc["schema"] != "application-map-branches/0" || doc["branches_total"] != float64(1) ||
		!strings.Contains(out, `"note":"none at request"`) || !strings.Contains(out, `"name":"factory :order"`) {
		t.Fatalf("branches: %d %s %s", code, diag, out)
	}
	for _, tc := range []struct {
		code string
		args []string
	}{
		{"invalid-arguments", []string{"--map", filename}},
		{"invalid-arguments", []string{"--cite", "app/models/order.rb:1"}},
		{"appmap-invalid-query", []string{"--map", filename, "--cite", "../order.rb:1"}},
		{"appmap-invalid-query", []string{"--map", filename, "--cite", "app/models/order.rb:9"}},
		{"appmap-invalid-query", []string{"--map", filename, "--cite", "app/models/order.rb:1", "--budget", "1"}},
		{"invalid-arguments", []string{"--map", filename, "--cite", "app/models/order.rb:1", "extra"}},
	} {
		code, out, diag = runFlowsCLI(root, append([]string{"appmap", "branches"}, tc.args...)...)
		if code != 2 || out != "" || !strings.Contains(diag, tc.code) {
			t.Fatalf("admitted %v: %d %s %s", tc.args, code, out, diag)
		}
	}
	assertCleanRepo(t, root)
}
