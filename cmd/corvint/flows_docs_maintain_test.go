package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
)

func maintenanceArgs() []string {
	return []string{"docs", "maintain", "--flows", "flows", "--page", "docs/flows.md", "--claims", "docs/claims.json", "--docs-root", "docs"}
}
func maintenancePreview(t *testing.T, root string, args []string) appflows.DocProposal {
	t.Helper()
	code, out, diagnostic := runFlowsCLI(root, append(slices.Clone(args), "--preview")...)
	if code != 0 {
		t.Fatalf("preview %d %s %s", code, out, diagnostic)
	}
	var p appflows.DocProposal
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		t.Fatal(err)
	}
	return p
}
func maintenanceApply(t *testing.T, root string, args []string, p appflows.DocProposal) string {
	t.Helper()
	code, out, diagnostic := runFlowsCLI(root, append(slices.Clone(args), "--apply", "--expected-proposal", p.Digest)...)
	if code != 0 {
		t.Fatalf("apply %d %s %s", code, out, diagnostic)
	}
	var receipt appflows.DocMaintenanceReceipt
	if err := json.Unmarshal([]byte(out), &receipt); err != nil || receipt.Schema != appflows.DocMaintenanceSchema {
		t.Fatalf("receipt %s %v", out, err)
	}
	return out
}
func maintenanceFixture(t *testing.T) (shopFixture, []string, string) {
	t.Helper()
	fx := newShopFixture(t)
	shopWrite(t, fx.root, map[string]string{"docs/guide.md": "# Authored companion\nKeep this exact prose.\n"})
	shopCommit(t, fx.root, "authored companion")
	args := maintenanceArgs()
	p := maintenancePreview(t, fx.root, args)
	if _, err := os.Stat(filepath.Join(fx.root, "docs/flows.md")); !os.IsNotExist(err) {
		t.Fatal("preview wrote output")
	}
	receipt := maintenanceApply(t, fx.root, args, p)
	shopWrite(t, fx.root, map[string]string{"docs/receipt.json": receipt})
	shopCommit(t, fx.root, "pair and receipt")
	return fx, append(args, "--receipt", "docs/receipt.json"), receipt
}

func TestFlowDocMaintenanceLifecycle(t *testing.T) {
	t.Run("AFU-V1-030 maintained template", func(t *testing.T) {
		fx, args, receipt := maintenanceFixture(t)
		p := maintenancePreview(t, fx.root, args)
		if !p.Noop {
			t.Fatal("repeat not noop")
		}
		if maintenanceApply(t, fx.root, args, p) != receipt {
			t.Fatal("noop receipt changed")
		}
		shopWrite(t, fx.root, map[string]string{"unrelated.txt": "unrelated HEAD movement"})
		shopCommit(t, fx.root, "unrelated")
		p = maintenancePreview(t, fx.root, args)
		if !p.Noop || maintenanceApply(t, fx.root, args, p) != receipt {
			t.Fatal("unrelated HEAD changed receipt")
		}
		changeDeclaredOutcome(t, fx.root)
		old := p
		p = maintenancePreview(t, fx.root, args)
		if p.Noop || p.Page == old.Page {
			t.Fatal("change not rendered")
		}
		if code, out, _ := runFlowsCLI(fx.root, append(slices.Clone(args), "--apply", "--expected-proposal", old.Digest)...); code == 0 || out != "" {
			t.Fatal("stale proposal accepted")
		}
		receipt = maintenanceApply(t, fx.root, args, p)
		shopWrite(t, fx.root, map[string]string{"docs/receipt.json": receipt})
		shopCommit(t, fx.root, "updated pair")
		check := []string{"docs", "--flows", "flows", "--page", "docs/flows.md", "--claims", "docs/claims.json", "--docs-root", "docs", "--check"}
		code, out, diag := runFlowsCLI(fx.root, check...)
		if code != 0 || !strings.Contains(out, `"status":"pass"`) {
			t.Fatalf("new committed pair check %d %s %s", code, out, diag)
		}
		guide, _ := os.ReadFile(filepath.Join(fx.root, "docs/guide.md"))
		if string(guide) != "# Authored companion\nKeep this exact prose.\n" {
			t.Fatal("authored prose changed")
		}
	})
}

// TestFlowDocMaintenanceRefusals tampers after the fixture's committed pair and
// re-previews, so each case must be refused by its own named check rather than
// by the stale proposal digest (FDM-V0-002, FDM-V0-003).
func TestFlowDocMaintenanceRefusals(t *testing.T) {
	const stale = "maintenance proposal changed; preview again"
	for _, tc := range []struct{ kind, reason string }{
		{"dirty-source", "maintenance generation input differs from committed bytes"},
		{"hidden-dirty-source", "maintenance generation input differs from committed bytes"},
		{"untracked-source", "maintenance inputs or outputs are dirty"},
		{"deleted-pair", "maintenance inputs or outputs are dirty"},
		{"dirty-output", "maintenance inputs or outputs are dirty"},
		{"hidden-output", "maintenance outputs must match committed bytes"},
		{"committed-output", "maintenance refuses authored or invalid generated outputs"},
		{"dirty-receipt", "maintenance receipt must match committed bytes"},
		{"forged-receipt", "invalid maintenance receipt integrity or profile"},
		{"stale-receipt", "maintenance receipt output mismatch"},
		{"mixed", "mixed maintenance destination existence"},
		{"alias", "aliased maintenance destinations"},
		{"symlink", "flow output parent must be a real directory"},
		// Changed committed inputs derive a new proposal; only the digest refuses the old one.
		{"committed-source", stale},
		{"evidence", stale},
		{"noop-stale-digest", stale},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			fx, args, receipt := maintenanceFixture(t)
			p := maintenancePreview(t, fx.root, args)
			switch tc.kind {
			case "dirty-source", "hidden-dirty-source", "committed-source":
				raw, _ := os.ReadFile(filepath.Join(fx.root, "flows/checkout.json"))
				if tc.kind == "hidden-dirty-source" {
					shopGit(t, fx.root, "update-index", "--assume-unchanged", "flows/checkout.json")
				}
				raw = append(raw, ' ')
				shopWrite(t, fx.root, map[string]string{"flows/checkout.json": string(raw)})
				if tc.kind == "committed-source" {
					shopCommit(t, fx.root, "source bytes changed")
				}
			case "untracked-source":
				shopWrite(t, fx.root, map[string]string{"flows/untracked.json": "{}"})
			case "deleted-pair":
				for _, n := range []string{"docs/flows.md", "docs/claims.json"} {
					if err := os.Remove(filepath.Join(fx.root, n)); err != nil {
						t.Fatal(err)
					}
				}
				args = args[:len(args)-2]
			case "dirty-output", "committed-output":
				shopWrite(t, fx.root, map[string]string{"docs/flows.md": "# Human replacement\n"})
				if tc.kind == "committed-output" {
					shopCommit(t, fx.root, "replace output")
				}
			case "hidden-output":
				// Git status hides the edit; the bytes stay generated-shaped.
				shopGit(t, fx.root, "update-index", "--assume-unchanged", "docs/flows.md")
				raw, _ := os.ReadFile(filepath.Join(fx.root, "docs/flows.md"))
				shopWrite(t, fx.root, map[string]string{"docs/flows.md": string(raw) + "\nHidden edit.\n"})
			case "dirty-receipt":
				shopWrite(t, fx.root, map[string]string{"docs/receipt.json": receipt + " "})
			case "forged-receipt":
				shopWrite(t, fx.root, map[string]string{"docs/receipt.json": strings.Replace(receipt, `"profile":"fixed-flow-template/0"`, `"profile":"forged"`, 1)})
				shopCommit(t, fx.root, "forged receipt")
			case "stale-receipt":
				// A valid receipt whose recorded outputs are no longer the committed pair.
				changeDeclaredOutcome(t, fx.root)
				next := maintenancePreview(t, fx.root, args)
				maintenanceApply(t, fx.root, args, next)
				shopCommit(t, fx.root, "pair without its receipt")
			case "evidence":
				e := filepath.Join(t.TempDir(), "new.jsonl")
				if err := os.WriteFile(e, nil, 0600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--evidence", e)
			case "noop-stale-digest":
				// The current state is a no-op; a stale digest must not return its receipt.
				p.Digest = strings.Repeat("0", 64)
			case "mixed":
				if err := os.Remove(filepath.Join(fx.root, "docs/claims.json")); err != nil {
					t.Fatal(err)
				}
			case "alias":
				if err := os.Remove(filepath.Join(fx.root, "docs/claims.json")); err != nil {
					t.Fatal(err)
				}
				if err := os.Link(filepath.Join(fx.root, "docs/flows.md"), filepath.Join(fx.root, "docs/claims.json")); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Rename(filepath.Join(fx.root, "docs"), filepath.Join(fx.root, "real-docs")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("real-docs", filepath.Join(fx.root, "docs")); err != nil {
					t.Fatal(err)
				}
			}
			before := maintenanceOutputs(t, fx.root)
			code, out, diagnostic := runFlowsCLI(fx.root, append(slices.Clone(args), "--preview")...)
			if tc.reason == stale {
				var fresh appflows.DocProposal
				if code != 0 || json.Unmarshal([]byte(out), &fresh) != nil || fresh.Digest == p.Digest {
					t.Fatalf("changed inputs kept the proposal %d %s %s", code, out, diagnostic)
				}
			} else if code == 0 || out != "" || !strings.Contains(diagnostic, tc.reason) {
				t.Fatalf("preview %d %s %q, want %q", code, out, diagnostic, tc.reason)
			}
			code, out, diagnostic = runFlowsCLI(fx.root, append(slices.Clone(args), "--apply", "--expected-proposal", p.Digest)...)
			if code == 0 || out != "" || !strings.Contains(diagnostic, tc.reason) {
				t.Fatalf("apply %d %s %q, want %q", code, out, diagnostic, tc.reason)
			}
			if after := maintenanceOutputs(t, fx.root); after != before {
				t.Fatalf("refused apply changed outputs %q -> %q", before, after)
			}
		})
	}
}

func changeDeclaredOutcome(t *testing.T, root string) {
	t.Helper()
	raw, _ := os.ReadFile(filepath.Join(root, "flows/checkout.json"))
	var flow appflows.FlowIntent
	if err := json.Unmarshal(raw, &flow); err != nil {
		t.Fatal(err)
	}
	flow.Outcomes[0].Behavior = "A changed declared outcome"
	changed, _ := json.Marshal(flow)
	shopWrite(t, root, map[string]string{"flows/checkout.json": string(changed)})
	shopCommit(t, root, "changed outcome")
}

func maintenanceOutputs(t *testing.T, root string) string {
	t.Helper()
	page, _ := os.ReadFile(filepath.Join(root, "docs/flows.md"))
	claims, _ := os.ReadFile(filepath.Join(root, "docs/claims.json"))
	return string(page) + "\x00" + string(claims)
}

func TestFlowDocMaintenanceAdoption(t *testing.T) {
	fx, args, _ := maintenanceFixture(t)
	args = args[:len(args)-2]
	if code, _, _ := runFlowsCLI(fx.root, append(slices.Clone(args), "--preview")...); code == 0 {
		t.Fatal("receipt-less overwrite")
	}
	args = append(args, "--adopt")
	p := maintenancePreview(t, fx.root, args)
	receipt := maintenanceApply(t, fx.root, args, p)
	if !strings.Contains(receipt, `"historical_provenance":"UNKNOWN"`) {
		t.Fatal("invented historical provenance")
	}
	for _, bad := range [][]string{
		{"docs", "maintain", "--flows", "flows", "--page", "docs/flows.md", "--claims", "docs/flows.md", "--preview"},
		append(maintenanceArgs(), "--preview", "--apply"), append(maintenanceArgs(), "--apply"), append(maintenanceArgs(), "--preview", "--check"),
	} {
		if code, _, _ := runFlowsCLI(fx.root, bad...); code == 0 {
			t.Fatalf("bad args accepted %v", bad)
		}
	}
}

type failingMaintenanceWriter struct{}

func (failingMaintenanceWriter) Write([]byte) (int, error) { return 0, errors.New("stdout closed") }

// TestFlowDocMaintenanceCLIGuards covers the maintain-mode argument guard, the
// maintenance-flag guard on plain `flows docs` and the receipt-delivery report
// on a failed stdout write (FDM-V0-005).
func TestFlowDocMaintenanceCLIGuards(t *testing.T) {
	fx := newShopFixture(t)
	shopWrite(t, fx.root, map[string]string{"docs/guide.md": "# Authored companion\n"})
	shopCommit(t, fx.root, "companion")
	page := filepath.Join(fx.root, "docs/flows.md")
	const mode = "flows docs maintain needs --preview or --apply --expected-proposal SHA256"
	for _, extra := range [][]string{{"--preview", "--apply"}, {"--apply"}, {"--preview", "--check"}, {"--preview", "--check", "--waivers", "w.json"}, {"--preview", "--expected-proposal", strings.Repeat("0", 64)}} {
		if code, _, diagnostic := runFlowsCLI(fx.root, append(maintenanceArgs(), extra...)...); code == 0 || !strings.Contains(diagnostic, mode) {
			t.Fatalf("maintain %v accepted %d %q", extra, code, diagnostic)
		}
	}
	plain := []string{"docs", "--flows", "flows", "--page", "docs/flows.md", "--claims", "docs/claims.json"}
	for _, extra := range [][]string{{"--preview"}, {"--apply"}, {"--expected-proposal", strings.Repeat("0", 64)}, {"--receipt", "docs/receipt.json"}, {"--adopt"}} {
		code, _, diagnostic := runFlowsCLI(fx.root, append(slices.Clone(plain), extra...)...)
		if code == 0 || !strings.Contains(diagnostic, "maintenance flags require flows docs maintain") {
			t.Fatalf("plain docs %v accepted %d %q", extra, code, diagnostic)
		}
		if _, err := os.Lstat(page); !os.IsNotExist(err) {
			t.Fatalf("plain docs %v wrote the page", extra)
		}
	}
	p := maintenancePreview(t, fx.root, maintenanceArgs())
	err := runFlowsDocs(context.Background(), fx.root, append(maintenanceArgs()[1:], "--apply", "--expected-proposal", p.Digest), failingMaintenanceWriter{})
	if err == nil || !strings.Contains(err.Error(), "receipt delivery failed") {
		t.Fatalf("failed receipt delivery not reported: %v", err)
	}
	if raw, e := os.ReadFile(page); e != nil || string(raw) != p.Page {
		t.Fatalf("published page missing after delivery failure: %v", e)
	}
}

func TestFlowDocMaintenanceCompiled(t *testing.T) {
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
