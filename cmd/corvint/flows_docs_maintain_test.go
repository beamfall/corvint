package main

import (
	"encoding/json"
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
		name := filepath.Join(fx.root, "flows/checkout.json")
		raw, _ := os.ReadFile(name)
		var flow appflows.FlowIntent
		if err := json.Unmarshal(raw, &flow); err != nil {
			t.Fatal(err)
		}
		flow.Outcomes[0].Behavior = "A changed declared outcome"
		changed, _ := json.Marshal(flow)
		shopWrite(t, fx.root, map[string]string{"flows/checkout.json": string(changed)})
		shopCommit(t, fx.root, "changed outcome")
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

func TestFlowDocMaintenanceRefusals(t *testing.T) {
	for _, kind := range []string{"dirty-source", "hidden-dirty-source", "deleted-pair", "committed-source", "dirty-output", "committed-output", "dirty-receipt", "forged-receipt", "wrong-output-receipt", "evidence", "mixed", "alias", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			fx, args, receipt := maintenanceFixture(t)
			p := maintenancePreview(t, fx.root, args)
			switch kind {
			case "dirty-source", "hidden-dirty-source", "committed-source":
				raw, _ := os.ReadFile(filepath.Join(fx.root, "flows/checkout.json"))
				if kind == "hidden-dirty-source" {
					shopGit(t, fx.root, "update-index", "--assume-unchanged", "flows/checkout.json")
				}
				raw = append(raw, ' ')
				shopWrite(t, fx.root, map[string]string{"flows/checkout.json": string(raw)})
				if kind == "committed-source" {
					shopCommit(t, fx.root, "source bytes changed")
				}
			case "deleted-pair":
				for _, n := range []string{"docs/flows.md", "docs/claims.json"} {
					if err := os.Remove(filepath.Join(fx.root, n)); err != nil {
						t.Fatal(err)
					}
				}
				args = args[:len(args)-2]
			case "dirty-output", "committed-output":
				shopWrite(t, fx.root, map[string]string{"docs/flows.md": "# Human replacement\n"})
				if kind == "committed-output" {
					shopCommit(t, fx.root, "replace output")
				}
			case "dirty-receipt":
				shopWrite(t, fx.root, map[string]string{"docs/receipt.json": receipt + " "})
			case "forged-receipt":
				shopWrite(t, fx.root, map[string]string{"docs/receipt.json": strings.Replace(receipt, `"profile":"fixed-flow-template/0"`, `"profile":"forged"`, 1)})
				shopCommit(t, fx.root, "forged receipt")
			case "wrong-output-receipt":
				shopWrite(t, fx.root, map[string]string{"docs/receipt.json": strings.Replace(receipt, `docs/claims.json`, `docs/other.json`, 1)})
				shopCommit(t, fx.root, "wrong output")
			case "evidence":
				e := filepath.Join(t.TempDir(), "new.jsonl")
				if err := os.WriteFile(e, nil, 0600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--evidence", e)
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
			if code, out, _ := runFlowsCLI(fx.root, append(slices.Clone(args), "--apply", "--expected-proposal", p.Digest)...); code == 0 || out != "" {
				t.Fatalf("unsafe apply %d %s", code, out)
			}
		})
	}
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
