// SPDX-License-Identifier: Apache-2.0

package registry_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/rubylinecov"
	"github.com/Beamfall/corvint/internal/testevidence"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

var rubyRunners = []string{"minitest", "rails-test", "rspec", "test-unit"}

func rubyFixture(t *testing.T, runner string, capture registry.RubyCapture) (*registry.RubyPlugin, tr.InventoryKey, string) {
	t.Helper()
	root := t.TempDir()
	request := tr.Request{Runner: runner, Root: root, Executable: filepath.Join(root, "ruby"), ReportDir: filepath.Join(root, "reports"), Selectors: []string{"test/calc_test.rb"}}
	if runner == "rspec" {
		request.Executable = filepath.Join(root, "rspec")
		request.Selectors = []string{"spec/calc_spec.rb"}
	}
	key := tr.InventoryKey{Checkout: strings.Repeat("a", 40), Runner: runner, ConfigDigest: registry.RubyConfigurationDigest(request), OS: "linux", Arch: "amd64"}
	p, err := registry.NewRubyPlugin(key, request, capture)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join("testdata", "ruby-selection", runner+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return p, key, string(b)
}

func TestRSPV0001RubyPieces(t *testing.T) {
	for _, runner := range rubyRunners {
		t.Run(runner, func(t *testing.T) {
			p, _, _ := rubyFixture(t, runner, nil)
			pieces := p.Pieces()
			if !pieces.Inventory.Present || !pieces.TestEvidence.Present || pieces.Qualification.Present || pieces.Qualification.Reason != "unqualified" {
				t.Fatalf("%+v", pieces)
			}
			if pieces.FullReason(true) == "" || pieces.FullReason(false) == "" {
				t.Fatal("declaration conferred qualification")
			}
		})
	}
}
func TestRSPV0001RubyInvalidBinding(t *testing.T) {
	for _, runner := range rubyRunners {
		t.Run(runner, func(t *testing.T) {
			_, key, _ := rubyFixture(t, runner, nil)
			if _, err := registry.NewRubyPlugin(key, tr.Request{Runner: runner}, nil); err == nil {
				t.Fatal("unbound configuration admitted")
			}
		})
	}
}
func TestRSPV0002RubyDeterministicInventory(t *testing.T) {
	for _, runner := range rubyRunners {
		t.Run(runner, func(t *testing.T) {
			p, key, report := rubyFixture(t, runner, nil)
			var previous []byte
			for i := 0; i < 2; i++ {
				in, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: key, Output: report})
				if err != nil {
					t.Fatal(err)
				}
				if !in.Complete || len(in.Tests) != 2 {
					t.Fatalf("%+v", in)
				}
				b, err := tr.InventoryBytes(in, key)
				if err != nil {
					t.Fatal(err)
				}
				if i > 0 && string(b) != string(previous) {
					t.Fatal("inventory nondeterministic")
				}
				previous = b
			}
			cmd, err := p.Command(context.Background(), registry.CommandRequest{Key: key, Purpose: "inventory", RunDir: filepath.Dir(p.CaptureDirectory()) + string(filepath.Separator) + ".."})
			if err == nil || len(cmd.Argv) != 0 {
				t.Fatal("noncanonical execution root admitted")
			}
		})
	}
}
func TestRSPV0002RubyMalformedInventoryFull(t *testing.T) {
	for _, runner := range rubyRunners {
		t.Run(runner, func(t *testing.T) {
			p, key, report := rubyFixture(t, runner, nil)
			for _, bad := range []string{"", report[:len(report)/2], report + "{}", strings.ReplaceAll(report, "calc_test.rb", "../outside.rb"), strings.ReplaceAll(report, "calc_spec.rb", "../../outside.rb")} {
				if bad == report {
					continue
				}
				session := registry.NewSelectionSession(p, key, true)
				in := session.Inventory(context.Background(), bad)
				if in.Complete || in.Reason == "" {
					t.Fatalf("invalid report complete: %+v", in)
				}
				if plan := session.Plan(in, nil, nil); !plan.Full || plan.Reason == "" {
					t.Fatalf("%+v", plan)
				}
			}
		})
	}
}
func TestRSPV0003RubyTerminalResults(t *testing.T) {
	for _, runner := range rubyRunners {
		t.Run(runner, func(t *testing.T) {
			p, key, report := rubyFixture(t, runner, nil)
			in, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: key, Output: report})
			if err != nil {
				t.Fatal(err)
			}
			rows, err := p.Parse(context.Background(), registry.ParseRequest{Runner: runner, Report: report})
			if err != nil {
				t.Fatal(err)
			}
			result := tr.ReconcileInventory(in, key, tr.PlanRunSet(in, key, nil, nil), rows, false)
			if !result.Succeeded || result.Full {
				t.Fatalf("%+v", result)
			}
		})
	}
}
func TestRSPV0003RubyDroppedAndExtraResults(t *testing.T) {
	for _, runner := range rubyRunners {
		t.Run(runner, func(t *testing.T) {
			p, key, report := rubyFixture(t, runner, nil)
			in, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: key, Output: report})
			if err != nil {
				t.Fatal(err)
			}
			rows, err := p.Parse(context.Background(), registry.ParseRequest{Runner: runner, Report: report})
			if err != nil {
				t.Fatal(err)
			}
			for _, results := range [][]tr.RunnerResult{rows[:1], append(append([]tr.RunnerResult{}, rows...), tr.RunnerResult{ID: "unlisted", Outcome: "passed"}), {{ID: rows[0].ID, Outcome: "skipped"}, rows[1]}} {
				result := tr.ReconcileInventory(in, key, tr.PlanRunSet(in, key, nil, nil), results, true)
				if result.Succeeded || !result.Full || !result.NeedsRequalification {
					t.Fatalf("%+v", result)
				}
			}
			calls := 0
			run := tr.RunInventoried(in, key, tr.PlanRunSet(in, key, nil, nil), true, func(plan tr.RunSet) ([]tr.RunnerResult, bool, error) {
				calls++
				if calls == 1 {
					return rows[:1], false, nil
				}
				if !plan.Full {
					t.Fatal("retry not FULL")
				}
				return rows, false, nil
			})
			if !run.Succeeded || !run.NeedsRequalification || calls != 2 {
				t.Fatalf("%+v", run)
			}
		})
	}
}
func TestRSPV0004RubyFreshExclusionsAndMandates(t *testing.T) {
	for _, runner := range rubyRunners {
		t.Run(runner, func(t *testing.T) {
			p, key, report := rubyFixture(t, runner, nil)
			in, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: key, Output: report})
			if err != nil {
				t.Fatal(err)
			}
			plan := tr.PlanRunSet(in, key, []tr.Exclusion{{ID: in.Tests[0].ID, Fresh: true}}, nil)
			if plan.Full || !reflect.DeepEqual(plan.Tests, []string{in.Tests[1].ID}) {
				t.Fatalf("%+v", plan)
			}
			plan = tr.PlanRunSet(in, key, []tr.Exclusion{{ID: in.Tests[0].ID, Fresh: true}}, []string{in.Tests[0].ID})
			if len(plan.Tests) != 2 {
				t.Fatalf("%+v", plan)
			}
		})
	}
}
func TestRSPV0004RubyFilterWidensAndStaleStays(t *testing.T) {
	for _, runner := range rubyRunners {
		t.Run(runner, func(t *testing.T) {
			p, key, report := rubyFixture(t, runner, nil)
			in, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: key, Output: report})
			if err != nil {
				t.Fatal(err)
			}
			if plan := tr.PlanRunSet(in, key, []tr.Exclusion{{ID: in.Tests[0].ID, Fresh: false}}, nil); len(plan.Tests) != 2 {
				t.Fatalf("%+v", plan)
			}
			root := filepath.Dir(filepath.Dir(p.CaptureDirectory()))
			full, err := p.Command(context.Background(), registry.CommandRequest{Key: key, Purpose: "run", RunDir: root})
			if err != nil {
				t.Fatal(err)
			}
			for _, tests := range [][]string{{in.Tests[0].ID}, {}, {".*[unsafe]"}} {
				cmd, err := p.Command(context.Background(), registry.CommandRequest{Key: key, Purpose: "run", RunDir: root, Tests: tests})
				if err != nil {
					t.Fatal(err)
				}
				if !cmd.Superset || !reflect.DeepEqual(cmd.Argv, full.Argv) {
					t.Fatalf("unsafe subset: %+v", cmd)
				}
			}
		})
	}
}
func TestRSPV0005RubyNativeCapture(t *testing.T) {
	for _, runner := range rubyRunners {
		t.Run(runner, func(t *testing.T) {
			native := testevidence.Native{Executed: testevidence.Component[testevidence.ExecutedFile]{State: testevidence.Present}, Regions: testevidence.Missing[testevidence.RegionsFile]("fixture"), Loaded: testevidence.Missing[testevidence.Blob]("fixture")}
			calls := 0
			capture := func(ctx context.Context, key tr.InventoryKey, artifacts []string) ([]rubylinecov.SelectionNative, error) {
				calls++
				if key.Runner != runner || !reflect.DeepEqual(artifacts, []string{"captured"}) {
					t.Fatal("capture binding lost")
				}
				name := "CalcTest#test_add"
				if runner == "rspec" {
					name = "./spec/calc_spec.rb[1:1]"
				}
				return []rubylinecov.SelectionNative{{Package: "test/calc_test.rb", Name: name, Native: native}}, nil
			}
			p, _, _ := rubyFixture(t, runner, capture)
			result, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: runner, Artifacts: []string{"captured"}})
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 || len(result) != 1 {
				t.Fatalf("%+v", result)
			}
			var executed testevidence.Component[testevidence.ExecutedFile]
			if err := json.Unmarshal(result[0].Executed, &executed); err != nil || executed.State != testevidence.Present {
				t.Fatalf("%+v %v", executed, err)
			}
		})
	}
}
func TestRSPV0005RubyMissingAndInvalidCapture(t *testing.T) {
	for _, runner := range rubyRunners {
		t.Run(runner, func(t *testing.T) {
			for _, capture := range []registry.RubyCapture{nil, func(context.Context, tr.InventoryKey, []string) ([]rubylinecov.SelectionNative, error) {
				return nil, fmt.Errorf("truncated")
			}, func(context.Context, tr.InventoryKey, []string) ([]rubylinecov.SelectionNative, error) {
				return []rubylinecov.SelectionNative{{Package: "test/calc_test.rb", Name: "Calc#test"}}, nil
			}} {
				p, _, _ := rubyFixture(t, runner, capture)
				if result, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: runner, Artifacts: []string{"capture"}}); err == nil || result != nil {
					t.Fatal("incomplete native capture admitted")
				}
			}
		})
	}
}
func TestRSPV0048RubyDiscoveryAndCollectionPieces(t *testing.T) {
	for _, runner := range rubyRunners {
		t.Run(runner, func(t *testing.T) {
			p, key, _ := rubyFixture(t, runner, nil)
			root := filepath.Dir(filepath.Dir(p.CaptureDirectory()))
			discover, err := p.Command(context.Background(), registry.CommandRequest{Key: key, Purpose: "inventory", RunDir: root})
			if err != nil {
				t.Fatal(err)
			}
			joined := strings.Join(discover.Argv, " ")
			if runner == "rspec" {
				if !strings.Contains(joined, "--dry-run") {
					t.Fatal(joined)
				}
			} else {
				if !strings.Contains(joined, "ruby_inventory.rb") {
					t.Fatal(joined)
				}
			}
			collect, err := p.Command(context.Background(), registry.CommandRequest{Key: key, Purpose: "collect", RunDir: root})
			if err != nil {
				t.Fatal(err)
			}
			if collect.Env["CORVINT_LINECOV_DIR"] != p.CaptureDirectory() || collect.Env["PARALLEL_WORKERS"] != "1" || len(p.Files()["ruby_linecov.rb"]) == 0 {
				t.Fatalf("%+v", collect)
			}
			files := p.Files()
			files["ruby_linecov.rb"][0] = '!'
			if p.Files()["ruby_linecov.rb"][0] == '!' {
				t.Fatal("mutable shared collector")
			}
		})
	}
}
func TestRSPV0048RubyUnqualifiedFull(t *testing.T) {
	for _, runner := range rubyRunners {
		t.Run(runner, func(t *testing.T) {
			p, key, report := rubyFixture(t, runner, nil)
			session := registry.NewSelectionSession(p, key, true)
			in := session.Inventory(context.Background(), report)
			if !in.Complete {
				t.Fatalf("%+v", in)
			}
			plan := session.Plan(in, []tr.Exclusion{{ID: in.Tests[0].ID, Fresh: true}}, nil)
			if !plan.Full || !strings.HasSuffix(plan.Reason, ":unqualified") || len(plan.Tests) != 2 {
				t.Fatalf("%+v", plan)
			}
		})
	}
}

func TestRSPV0002RubyCountDuplicatesCapsAndCancellation(t *testing.T) {
	for _, runner := range rubyRunners {
		t.Run(runner, func(t *testing.T) {
			p, key, report := rubyFixture(t, runner, nil)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if in, err := p.Inventory(ctx, registry.InventoryRequest{Key: key, Output: report}); err == nil || in.Complete {
				t.Fatal("cancelled inventory accepted")
			}
			countKey := "Count"
			if runner == "rspec" {
				countKey = "example_count"
			}
			badCount := strings.Replace(report, `"`+countKey+`": 2`, `"`+countKey+`": 1`, 1)
			for _, bad := range []string{badCount, `{"version":"x","version":"y"}`, strings.Repeat(" ", tr.MaxReportBytes+1), string([]byte{0xff})} {
				in, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: key, Output: bad})
				if err == nil || in.Complete || in.Reason == "" {
					t.Fatalf("malformed inventory: %+v %v", in, err)
				}
			}
		})
	}
}

func TestRSPV0003RubyReportedSkipsAndFailures(t *testing.T) {
	for _, runner := range rubyRunners {
		t.Run(runner, func(t *testing.T) {
			p, key, report := rubyFixture(t, runner, nil)
			in, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: key, Output: report})
			if err != nil {
				t.Fatal(err)
			}
			skip := strings.Replace(report, `"State": "PASSED"`, `"State": "SKIPPED", "Message": "disabled by runner"`, 1)
			failed := strings.Replace(report, `"State": "PASSED"`, `"State": "FAILED"`, 1)
			if runner == "rspec" {
				skip = strings.Replace(report, `"status": "passed"`, `"status": "pending", "pending_message": "disabled by runner"`, 1)
				skip = strings.Replace(skip, `"pending_count": 0`, `"pending_count": 1`, 1)
				failed = strings.Replace(report, `"status": "passed"`, `"status": "failed"`, 1)
				failed = strings.Replace(failed, `"failure_count": 0`, `"failure_count": 1`, 1)
			}
			for i, raw := range []string{skip, failed} {
				rows, err := p.Parse(context.Background(), registry.ParseRequest{Runner: runner, Report: raw})
				if err != nil {
					t.Fatal(err)
				}
				result := tr.ReconcileInventory(in, key, tr.PlanRunSet(in, key, nil, nil), rows, false)
				if result.Full || result.Succeeded != (i == 0) {
					t.Fatalf("%+v", result)
				}
			}
		})
	}
}

func TestRSPV0002RubyConfigurationIgnoresStaging(t *testing.T) {
	for _, runner := range rubyRunners {
		t.Run(runner, func(t *testing.T) {
			root := t.TempDir()
			r := tr.Request{Runner: runner, Root: root, Executable: filepath.Join(root, "ruby"), ReportDir: filepath.Join(root, "first"), Selectors: []string{"sample.rb"}}
			key := tr.InventoryKey{Checkout: strings.Repeat("a", 40), Runner: runner, ConfigDigest: registry.RubyConfigurationDigest(r), OS: "linux", Arch: "amd64"}
			r.ReportDir = filepath.Join(root, "second")
			if registry.RubyConfigurationDigest(r) != key.ConfigDigest {
				t.Fatal("staging changed configuration identity")
			}
			p, err := registry.NewRubyPlugin(key, r, nil)
			if err != nil {
				t.Fatal(err)
			}
			if p.CaptureDirectory() != filepath.Join(r.ReportDir, "ruby-coverage") {
				t.Fatal("staging did not move")
			}
			r.ConfigSha256 = strings.Repeat("b", 64)
			if registry.RubyConfigurationDigest(r) == key.ConfigDigest {
				t.Fatal("configuration pin ignored")
			}
			if _, err := registry.NewRubyPlugin(key, r, nil); err == nil {
				t.Fatal("changed configuration admitted under old identity")
			}
		})
	}
}
