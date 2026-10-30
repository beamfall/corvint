package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/flowdocs"
)

// TestFlowDocsCLIInventory (AFU-V1-101, AFU-V1-105, V1-2313): generate
// --flows writes exactly the declared flows and check reruns against the
// retained inventory.
func TestFlowDocsCLIInventory(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		out, e := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if e != nil {
			t.Fatalf("git: %v %s", e, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	git("config", "user.name", "Fixture")
	git("config", "user.email", "fixture@example.invalid")
	if err = os.Mkdir(filepath.Join(root, "app"), 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"app/pay.rb": "class Pay\n  def create\n    authorize(user)\n  end\n  def helper\n  end\nend\n", "app/view.js": "function view() {\n  fetch('/pay');\n}\n"}
	for p, v := range files {
		if err = os.WriteFile(filepath.Join(root, p), []byte(v), 0600); err != nil {
			t.Fatal(err)
		}
	}
	git("add", ".")
	git("commit", "-qm", "source")
	source := git("rev-parse", "HEAD")
	scratch, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	inventory := filepath.Join(scratch, "flows.json")
	if err = os.WriteFile(inventory, []byte(`{"schema":"corvint-flow-inventory/1","flows":[{"id":"pay","title":"Pay","members":[{"path":"app/pay.rb","symbol":"Pay#create"},{"path":"app/view.js"}]}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (int, []byte) {
		t.Helper()
		var out, stderr bytes.Buffer
		code := runContext(context.Background(), append([]string{"--root", root, "docs", "flows"}, args...), strings.NewReader(""), &out, &stderr)
		if code == 2 {
			t.Fatalf("CLI refused: %s", stderr.String())
		}
		return code, out.Bytes()
	}
	destination := filepath.Join(root, "generated")
	_, raw := run("generate", "--revision", source, "--scope", "app", "--flows", inventory, "--output-dir", destination)
	var m flowdocs.Manifest
	if err = json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Flows) != 1 || m.Flows[0].Name != "Pay" || m.Inventory == nil || len(m.Inventory.Unassigned) != 1 || m.Inventory.Unassigned[0].Symbol != "Pay#helper" {
		t.Fatalf("declared inventory not honoured: %d flows, %+v", len(m.Flows), m.Inventory)
	}
	if code, _ := run("check", "--revision", source, "--previous", filepath.Join(destination, "generation.json"), "--output-dir", destination); code != 0 {
		t.Fatal("inventory generation is not fresh at its own revision")
	}
}
