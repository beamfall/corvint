// SPDX-License-Identifier: Apache-2.0

package registry_test

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/rubylinecov"
	"github.com/Beamfall/corvint/internal/testevidence"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

func rubyArtifactRuntime(t *testing.T, runner string) string {
	t.Helper()
	exe, err := exec.LookPath("ruby")
	if err != nil {
		t.Skipf("Ruby unavailable: %v", err)
	}
	library := "minitest"
	if runner == "test-unit" {
		library = "test/unit"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, exe, "-e", "require '"+library+"'").CombinedOutput(); err != nil {
		t.Skipf("Ruby dependency unavailable: %v %s", err, out)
	}
	return exe
}

func rubyArtifactPlugin(t *testing.T, runner, exe, root string, capture registry.RubyCapture) (*registry.RubyPlugin, tr.InventoryKey, string) {
	t.Helper()
	reportDir := t.TempDir()
	r := tr.Request{Runner: runner, Root: root, Executable: exe, ReportDir: reportDir, Selectors: []string{"sample.rb"}}
	key := tr.InventoryKey{Checkout: strings.Repeat("a", 40), Runner: runner, ConfigDigest: registry.RubyConfigurationDigest(r), OS: "linux", Arch: "amd64"}
	p, err := registry.NewRubyPlugin(key, r, capture)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range p.Files() {
		if err := os.WriteFile(filepath.Join(reportDir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(p.CaptureDirectory(), 0700); err != nil {
		t.Fatal(err)
	}
	return p, key, reportDir
}

func rubyArtifactCommand(t *testing.T, p *registry.RubyPlugin, key tr.InventoryKey, root, purpose string, only *rubylinecov.Outcome) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	inv, err := p.Command(ctx, registry.CommandRequest{Key: key, RunDir: root, Purpose: purpose})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, inv.Argv[0], inv.Argv[1:]...)
	cmd.Dir = inv.Cwd
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if !strings.HasPrefix(k, "CORVINT_LINECOV_") && k != "RUBYOPT" {
			if _, replaced := inv.Env[k]; !replaced {
				cmd.Env = append(cmd.Env, kv)
			}
		}
	}
	if only != nil {
		inv.Env["CORVINT_LINECOV_ONLY"] = only.Name
		inv.Env["CORVINT_LINECOV_PACKAGE"] = only.Package
	}
	for k, v := range inv.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", purpose, err, out)
	}
}

// RSP-V0-003: the actual shipped reporters must emit usable skip reasons.
func TestRSPV0003RubyReporterSkipReasonsLive(t *testing.T) {
	for _, runner := range []string{"minitest", "rails-test", "test-unit"} {
		t.Run(runner, func(t *testing.T) {
			exe := rubyArtifactRuntime(t, runner)
			root := t.TempDir()
			source := "require 'minitest/autorun'\nclass Sample < Minitest::Test\n def test_skip; skip 'runner skip reason'; end\nend\n"
			report := "minitest.json"
			if runner == "test-unit" {
				source = "require 'test/unit'\nclass Sample < Test::Unit::TestCase\n def test_skip; omit 'runner skip reason'; end\n def test_pending; pend 'runner pending reason'; end\nend\n"
				report = "testunit.json"
			}
			if err := os.WriteFile(filepath.Join(root, "sample.rb"), []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			p, key, dir := rubyArtifactPlugin(t, runner, exe, root, nil)
			// Inventory is pinned to this fixture's declarations; only terminal
			// messages come from the real reporter under test.
			tests := []tr.InventoryTest{{ID: "Sample#test_skip", Path: "sample.rb"}}
			if runner == "test-unit" {
				tests = []tr.InventoryTest{{ID: "test_skip(Sample)", Path: "sample.rb"}, {ID: "test_pending(Sample)", Path: "sample.rb"}}
			}
			inventory, err := tr.CanonicalInventory(tr.RunnerInventory{Key: key, Complete: true, Tests: tests}, key)
			if err != nil {
				t.Fatal(err)
			}
			rubyArtifactCommand(t, p, key, root, "run", nil)
			data, err := os.ReadFile(filepath.Join(dir, report))
			if err != nil {
				t.Fatal(err)
			}
			rows, err := p.Parse(context.Background(), registry.ParseRequest{Runner: runner, Report: string(data)})
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) == 0 {
				t.Fatal("no terminal results")
			}
			for _, row := range rows {
				want := "runner skip reason"
				if strings.Contains(row.ID, "pending") {
					want = "runner pending reason"
				}
				if row.Outcome != "skipped" || row.Reason != want {
					t.Fatalf("actual reporter row: %+v", row)
				}
			}
			got := tr.ReconcileInventory(inventory, key, tr.PlanRunSet(inventory, key, nil, nil), rows, false)
			if !got.Succeeded || got.Full {
				t.Fatalf("valid skips retried or failed: %+v", got)
			}
		})
	}
}

// RSP-V0-005, RSP-V0-048: Command -> real artifacts -> exported reader -> SPI.
func TestRSPV0005RubyCommandArtifactsEvidenceLive(t *testing.T) {
	for _, runner := range []string{"minitest", "rails-test", "test-unit"} {
		t.Run(runner, func(t *testing.T) {
			exe := rubyArtifactRuntime(t, runner)
			root := t.TempDir()
			source := "require 'minitest/autorun'\nclass Sample < Minitest::Test\n def test_one; assert_equal 2, 1+1; end\n def test_two; assert_equal 3, 1+2; end\nend\n"
			if runner == "test-unit" {
				source = strings.ReplaceAll(strings.ReplaceAll(source, "minitest/autorun", "test/unit"), "Minitest::Test", "Test::Unit::TestCase")
			}
			if err := os.WriteFile(filepath.Join(root, "sample.rb"), []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			oid := fmt.Sprintf("%x", sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(source), source))))
			sources := map[string]rubylinecov.SelectionSource{"sample.rb": {OID: oid, Bytes: []byte(source)}}
			var isolated []string
			capture := func(_ context.Context, _ tr.InventoryKey, artifacts []string) ([]rubylinecov.SelectionNative, error) {
				run, err := rubylinecov.ReadSelectionArtifacts(artifacts[0], isolated)
				if err != nil {
					return nil, err
				}
				return rubylinecov.SelectionEvidence(run, "repo", sources)
			}
			p, key, _ := rubyArtifactPlugin(t, runner, exe, root, capture)
			rubyArtifactCommand(t, p, key, root, "collect", nil)
			ordinary, err := rubylinecov.ReadSelectionArtifacts(p.CaptureDirectory(), nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(ordinary.Outcomes) != 2 {
				t.Fatalf("%+v", ordinary.Outcomes)
			}
			for _, out := range ordinary.Outcomes {
				if out.Recorded {
					t.Fatal("ordinary run certified isolation")
				}
				one, oneKey, _ := rubyArtifactPlugin(t, runner, exe, root, nil)
				if oneKey != key {
					t.Fatal("fresh staging changed inventory key")
				}
				rubyArtifactCommand(t, one, key, root, "collect", &out)
				isolated = append(isolated, one.CaptureDirectory())
			}
			evidence, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: runner, Artifacts: []string{p.CaptureDirectory()}})
			if err != nil {
				t.Fatal(err)
			}
			if len(evidence) != 2 {
				t.Fatalf("%+v", evidence)
			}
			for _, e := range evidence {
				var executed testevidence.Component[testevidence.ExecutedFile]
				if err := json.Unmarshal(e.Executed, &executed); err != nil {
					t.Fatal(err)
				}
				if executed.State != testevidence.Present || len(executed.Items) != 1 || executed.Items[0].Blob.OID != oid || len(executed.Items[0].Executed) == 0 {
					t.Fatalf("%+v", executed)
				}
			}
		})
	}
}
