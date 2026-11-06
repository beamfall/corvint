// SPDX-License-Identifier: Apache-2.0
package browsercov

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestHookRendererSwapIncomplete(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	dir := t.TempDir()
	hook, err := filepath.Abs("hook.cjs")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(node, "hook_fixture.cjs", hook, dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("hook fixture: %v\n%s", err, out)
	}
	c := &collector{}
	obs, reason := c.observe(dir, "app.cy.js")
	if len(obs) != 0 || reason != "hook: cdp-detached: a page closed before its coverage was taken" {
		t.Fatalf("observe=%v reason=%q", obs, reason)
	}
}
