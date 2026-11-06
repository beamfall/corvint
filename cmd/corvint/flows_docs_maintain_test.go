package main

import (
	"encoding/json"
	"os/exec"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
)

// Handler-level flows tests live in internal/command/flows. These remain here because they exercise
// root dispatch, the `how` alias, root help or the compiled binary.

func TestFlowDocMaintenanceCompiled(t *testing.T) {
	t.Parallel()
	t.Run("AFU-V1-030 fixed template and AFU-V1-032 committed check", func(t *testing.T) {
		fx := newShopFixture(t)
		shopWrite(t, fx.root, map[string]string{"docs/guide.md": "# Authored companion\n"})
		shopCommit(t, fx.root, "companion")
		binary := workBoundCorvint(t)
		run := func(args ...string) []byte {
			t.Helper()
			cmd := exec.Command(binary, append([]string{"flows"}, args...)...)
			cmd.Dir = fx.root
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("compiled %v: %v %s", args, err, out)
			}
			return out
		}
		args := maintenanceArgs()
		var p appflows.DocProposal
		if err := json.Unmarshal(run(append(slices.Clone(args), "--preview")...), &p); err != nil {
			t.Fatal(err)
		}
		receipt := run(append(slices.Clone(args), "--apply", "--expected-proposal", p.Digest)...)
		shopWrite(t, fx.root, map[string]string{"docs/receipt.json": string(receipt)})
		shopCommit(t, fx.root, "compiled pair")
		out := run("docs", "--flows", "flows", "--page", "docs/flows.md", "--claims", "docs/claims.json", "--docs-root", "docs", "--check")
		var check appflows.DocCheck
		if err := json.Unmarshal(out, &check); err != nil || check.Status != "pass" {
			t.Fatalf("new HEAD check %s %v", out, err)
		}
	})
}
