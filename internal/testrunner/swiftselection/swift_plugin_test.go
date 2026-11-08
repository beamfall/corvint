// SPDX-License-Identifier: Apache-2.0

package swiftselection_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/langplugin"
	"github.com/Beamfall/corvint/internal/testevidence"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
	"github.com/Beamfall/corvint/internal/testrunner/swiftselection"
)

func swiftFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/swift/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func swiftOptions(t *testing.T, runner string) swiftselection.SwiftSelectionOptions {
	t.Helper()
	o := swiftselection.SwiftSelectionOptions{
		Key:  tr.InventoryKey{Checkout: strings.Repeat("a", 40), Runner: runner, ConfigDigest: strings.Repeat("b", 64), OS: "darwin", Arch: "arm64"},
		Root: "/fixture", Executable: "/tools/swift",
		Tests:   map[string]tr.InventoryTest{},
		Sources: map[string]testevidence.Blob{"/fixture/Sources/Logic.swift": {Path: testevidence.Path{Repository: "primary", Lexical: "Sources/Logic.swift", Resolved: "Sources/Logic.swift"}, OID: strings.Repeat("c", 40)}},
		Reports: map[string]tr.Input{}, Coverage: map[string]swiftselection.SwiftCoverage{},
	}
	if runner == "swift-xctest" {
		for _, suffix := range []string{"Pass", "Fail", "Skip"} {
			id := "ProofTests.Proof/test" + suffix
			o.Tests[id] = tr.InventoryTest{ID: id, Path: "Tests/ProofTests/Proof.swift"}
		}
		o.Reports["pass"] = tr.Input{Runner: runner, Stdout: swiftFixture(t, "xctest-pass.txt"), ExitCode: 0}
		o.Reports["mixed"] = tr.Input{Runner: runner, Stdout: swiftFixture(t, "xctest-mixed.txt"), ExitCode: 1}
	} else {
		for name, line := range map[string]string{"pass": "2", "fail": "3", "skip": "4"} {
			name = "ProofTests." + name + "Criterion()"
			o.Tests[name] = tr.InventoryTest{ID: name + "/Proof.swift:" + line + ":2", Path: "Tests/ProofTests/Proof.swift"}
		}
		o.Reports["mixed"] = tr.Input{Runner: runner, Reports: map[string][]byte{"swift-events.jsonl": swiftFixture(t, "swift-testing.jsonl")}, ExitCode: 1}
	}
	var ids []string
	for _, test := range o.Tests {
		ids = append(ids, test.ID)
	}
	slices.Sort(ids)
	o.Coverage["coverage"] = swiftselection.SwiftCoverage{LCOV: swiftFixture(t, "coverage.lcov"), Tests: ids, Complete: true}
	return o
}
func swiftPlugin(t *testing.T, o swiftselection.SwiftSelectionOptions) registry.RunnerPlugin {
	t.Helper()
	p, err := swiftselection.NewSwiftSelectionPlugin(o)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func swiftList(t *testing.T, runner string) string {
	t.Helper()
	if runner == "swift-xctest" {
		return string(swiftFixture(t, "xctest-list.txt"))
	}
	return string(swiftFixture(t, "swift-testing-list.txt"))
}
func swiftInventory(t *testing.T, p registry.RunnerPlugin, o swiftselection.SwiftSelectionOptions) tr.RunnerInventory {
	t.Helper()
	in, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: o.Key, Output: swiftList(t, o.Key.Runner)})
	if err != nil || !in.Complete || len(in.Tests) != 3 {
		t.Fatalf("initial inventory: %+v %v", in, err)
	}
	return in
}
func TestRSPV0001SwiftConfiguredPieces(t *testing.T) {
	for _, runner := range []string{"swift-testing", "swift-xctest"} {
		t.Run(runner, func(t *testing.T) {
			o := swiftOptions(t, runner)
			p := swiftPlugin(t, o)
			in := swiftInventory(t, p, o)
			pieces := p.Pieces()
			if !pieces.Inventory.Present || !pieces.TestEvidence.Present || pieces.Qualification.Present || pieces.Qualification.Reason != "unqualified" {
				t.Fatalf("%+v", pieces)
			}
			for _, m := range langplugin.All() {
				if m.Name == "swift" {
					own := make(map[string]tr.RunnerPieces, len(m.RunnerPieces))
					for k, v := range m.RunnerPieces {
						own[k] = v
					}
					m.RunnerPieces = own
					m.RunnerPieces[runner] = pieces
					if err := langplugin.Validate(m); err != nil {
						t.Fatal(err)
					}
					pieces.Qualification = tr.RunnerPiece{}
					m.RunnerPieces[runner] = pieces
					if err := langplugin.Validate(m); err == nil {
						t.Fatal("unexplained missing qualification accepted")
					}
				}
			}
			for _, qualified := range []bool{false, true} {
				plan := registry.NewSelectionSession(p, o.Key, qualified).Plan(in, []tr.Exclusion{{ID: in.Tests[0].ID, Fresh: true}}, nil)
				if !plan.Full || !strings.Contains(plan.Reason, "unqualified") || len(plan.Tests) != 3 {
					t.Fatalf("%+v", plan)
				}
			}
		})
	}
}
func TestRSPV0001SwiftUnsupportedInputs(t *testing.T) {
	valid := swiftOptions(t, "swift-xctest")
	swiftInventory(t, swiftPlugin(t, valid), valid)
	for name, change := range map[string]func(*swiftselection.SwiftSelectionOptions){
		"runner":   func(o *swiftselection.SwiftSelectionOptions) { o.Key.Runner = "xcode-xctest" },
		"target":   func(o *swiftselection.SwiftSelectionOptions) { o.Key.OS = "unknown" },
		"checkout": func(o *swiftselection.SwiftSelectionOptions) { o.Key.Checkout = "HEAD" },
		"tool":     func(o *swiftselection.SwiftSelectionOptions) { o.Executable = "swift" },
		"config":   func(o *swiftselection.SwiftSelectionOptions) { o.Key.ConfigDigest = "unknown" },
		"source": func(o *swiftselection.SwiftSelectionOptions) {
			o.Tests["ProofTests.Proof/testPass"] = tr.InventoryTest{ID: "ProofTests.Proof/testPass", Path: "../escape.swift"}
		},
		"blob": func(o *swiftselection.SwiftSelectionOptions) {
			b := o.Sources["/fixture/Sources/Logic.swift"]
			b.OID = "unresolved"
			o.Sources["/fixture/Sources/Logic.swift"] = b
		},
	} {
		t.Run(name, func(t *testing.T) {
			o := swiftOptions(t, "swift-xctest")
			change(&o)
			if p, err := swiftselection.NewSwiftSelectionPlugin(o); err == nil || p != nil {
				t.Fatal("invalid factory admitted")
			}
		})
	}
	for _, runner := range []string{"swift-testing", "swift-xctest"} {
		p, ok := registry.SelectionPlugin(runner)
		if !ok || p.Pieces().FullReason(true) == "" {
			t.Fatal("unconfigured runner narrowed")
		}
	}
}
func TestRSPV0002SwiftDeterministicInventory(t *testing.T) {
	for _, runner := range []string{"swift-testing", "swift-xctest"} {
		t.Run(runner, func(t *testing.T) {
			o := swiftOptions(t, runner)
			p := swiftPlugin(t, o)
			in := swiftInventory(t, p, o)
			lines := strings.Split(strings.TrimSuffix(swiftList(t, runner), "\n"), "\n")
			slices.Reverse(lines)
			other, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: o.Key, Output: strings.Join(lines, "\n") + "\n"})
			if err != nil {
				t.Fatal(err)
			}
			a, err := tr.InventoryBytes(in, o.Key)
			if err != nil {
				t.Fatal(err)
			}
			b, err := tr.InventoryBytes(other, o.Key)
			if err != nil || !bytes.Equal(a, b) {
				t.Fatalf("inventory drift: %v", err)
			}
			for name := range o.Tests {
				delete(o.Tests, name)
			}
			if got := swiftInventory(t, p, o); !reflect.DeepEqual(got, in) {
				t.Fatal("caller mutated inventory snapshot")
			}
		})
	}
}
func TestRSPV0002SwiftIncompleteInventoryFull(t *testing.T) {
	for _, runner := range []string{"swift-testing", "swift-xctest"} {
		t.Run(runner, func(t *testing.T) {
			o := swiftOptions(t, runner)
			p := swiftPlugin(t, o)
			swiftInventory(t, p, o)
			list := swiftList(t, runner)
			rows := strings.Split(strings.TrimSuffix(list, "\n"), "\n")
			for name, output := range map[string]string{"empty": "", "truncated": strings.TrimSuffix(list, "\n"), "line-boundary": rows[0] + "\n", "unknown": list + "unknown\n", "duplicate": list + rows[0] + "\n", "invalid-utf8": list + string([]byte{255}) + "\n", "capped": strings.Repeat("x", tr.MaxReportBytes+1)} {
				t.Run(name, func(t *testing.T) {
					in, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: o.Key, Output: output})
					if err == nil || in.Complete || in.Reason == "" {
						t.Fatalf("%+v %v", in, err)
					}
					if plan := registry.NewSelectionSession(p, o.Key, false).Plan(in, nil, nil); !plan.Full {
						t.Fatalf("%+v", plan)
					}
				})
			}
			foreign := o.Key
			foreign.Arch = "amd64"
			if _, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: foreign, Output: list}); err == nil {
				t.Fatal("foreign target inventory admitted")
			}
		})
	}
}
func TestRSPV0003SwiftTerminalResults(t *testing.T) {
	for _, runner := range []string{"swift-testing", "swift-xctest"} {
		t.Run(runner, func(t *testing.T) {
			o := swiftOptions(t, runner)
			p := swiftPlugin(t, o)
			in := swiftInventory(t, p, o)
			results, err := p.Parse(context.Background(), registry.ParseRequest{Runner: runner, Report: "mixed"})
			if err != nil || len(results) != 3 {
				t.Fatalf("%+v %v", results, err)
			}
			counts := map[string]int{}
			for _, result := range results {
				counts[result.Outcome]++
				if result.Outcome == "skipped" && result.Reason == "" {
					t.Fatal("native skip reason lost")
				}
			}
			if counts["passed"] != 1 || counts["failed"] != 1 || counts["skipped"] != 1 {
				t.Fatal(counts)
			}
			rec := tr.ReconcileInventory(in, o.Key, tr.RunSet{Full: true}, results, false)
			if rec.Succeeded || rec.NeedsRequalification || len(rec.Missing) != 0 {
				t.Fatalf("failure result was changed: %+v", rec)
			}
			// The shared core also admits a narrowed set with every terminal present.
			var passed tr.RunnerResult
			for _, result := range results {
				if result.Outcome == "passed" {
					passed = result
				}
			}
			rec = tr.ReconcileInventory(in, o.Key, tr.RunSet{Tests: []string{passed.ID}}, []tr.RunnerResult{passed}, false)
			if !rec.Succeeded || rec.NeedsRequalification {
				t.Fatalf("%+v", rec)
			}
		})
	}
}
func TestRSPV0003SwiftMissingAndUnexpectedResults(t *testing.T) {
	o := swiftOptions(t, "swift-xctest")
	p := swiftPlugin(t, o)
	in := swiftInventory(t, p, o)
	s := registry.NewSelectionSession(p, o.Key, false)
	calls := 0
	out := s.Run(context.Background(), in, nil, nil, "/run", true, func(_ context.Context, c registry.CommandResponse) (string, error) {
		calls++
		if slices.Contains(c.Argv, "--filter") {
			t.Fatal("FULL retry filtered")
		}
		if calls == 1 {
			return "pass", nil
		}
		return "mixed", nil
	})
	if calls != 2 || len(out.Attempts) != 2 || !out.NeedsRequalification || len(out.Attempts[0].Reconciliation.Missing) != 2 || out.Succeeded {
		t.Fatalf("%+v", out)
	}
	if !strings.Contains(s.Plan(in, nil, nil).Reason, "inventory-divergence") {
		t.Fatal("divergence lost")
	}
	result, err := p.Parse(context.Background(), registry.ParseRequest{Runner: o.Key.Runner, Report: "pass"})
	if err != nil {
		t.Fatal(err)
	}
	input := o.Reports["pass"]
	input.Stdout = bytes.ReplaceAll(input.Stdout, []byte("testPass"), []byte("testAlien"))
	o.Reports["alien"] = input
	result, err = swiftPlugin(t, o).Parse(context.Background(), registry.ParseRequest{Runner: o.Key.Runner, Report: "alien"})
	if err != nil || len(result) != 1 || result[0].ID != "ProofTests.Proof/testAlien" {
		t.Fatalf("unexpected native result lost: %+v %v", result, err)
	}
	rec := tr.ReconcileInventory(in, o.Key, tr.RunSet{Full: true}, result, false)
	if rec.Succeeded || !rec.NeedsRequalification || !slices.Contains(rec.Unexpected, "ProofTests.Proof/testAlien") {
		t.Fatalf("%+v", rec)
	}
	// Caller forbidding a retry gets failure and the exact missing identities.
	calls = 0
	out = s.Run(context.Background(), in, nil, nil, "/run", false, func(context.Context, registry.CommandResponse) (string, error) { calls++; return "pass", nil })
	if calls != 1 || out.Succeeded || len(out.Attempts[0].Reconciliation.Missing) != 2 {
		t.Fatalf("%+v", out)
	}
}
func TestRSPV0003SwiftBadCaptureCannotSucceed(t *testing.T) {
	for _, runner := range []string{"swift-testing", "swift-xctest"} {
		t.Run(runner, func(t *testing.T) {
			valid := swiftOptions(t, runner)
			p := swiftPlugin(t, valid)
			in := swiftInventory(t, p, valid)
			if _, err := p.Parse(context.Background(), registry.ParseRequest{Runner: runner, Report: "mixed"}); err != nil {
				t.Fatal(err)
			}
			for _, mode := range []string{"exit", "timeout", "interrupted", "overflow", "execution-problem", "truncated"} {
				t.Run(mode, func(t *testing.T) {
					o := swiftOptions(t, runner)
					input := o.Reports["mixed"]
					switch mode {
					case "exit":
						input.ExitCode = 99
					case "timeout":
						input.TimedOut = true
					case "interrupted":
						input.Interrupted = true
					case "overflow":
						input.Overflow = true
					case "execution-problem":
						input.ExecutionProblems = []tr.Problem{{Code: "capture-failed"}}
					case "truncated":
						if runner == "swift-xctest" {
							input.Stdout = input.Stdout[:len(input.Stdout)/2]
						} else {
							input.Reports["swift-events.jsonl"] = input.Reports["swift-events.jsonl"][:100]
						}
					}
					o.Reports["mixed"] = input
					results, err := swiftPlugin(t, o).Parse(context.Background(), registry.ParseRequest{Runner: runner, Report: "mixed"})
					if err == nil {
						t.Fatal("bad capture succeeded")
					}
					rec := tr.ReconcileInventory(in, o.Key, tr.RunSet{Full: true}, results, false)
					if rec.Succeeded {
						t.Fatalf("%+v", rec)
					}
				})
			}
		})
	}
}
func TestRSPV0004SwiftRunSetAndNativeFilters(t *testing.T) {
	for _, runner := range []string{"swift-testing", "swift-xctest"} {
		t.Run(runner, func(t *testing.T) {
			o := swiftOptions(t, runner)
			p := swiftPlugin(t, o)
			in := swiftInventory(t, p, o)
			plan := tr.PlanRunSet(in, o.Key, []tr.Exclusion{{ID: in.Tests[0].ID, Fresh: true}, {ID: in.Tests[1].ID, Fresh: false}}, nil)
			if plan.Full || slices.Contains(plan.Tests, in.Tests[0].ID) || !slices.Contains(plan.Tests, in.Tests[1].ID) || !slices.Contains(plan.Tests, in.Tests[2].ID) {
				t.Fatalf("%+v", plan)
			}
			mandated := tr.PlanRunSet(in, o.Key, []tr.Exclusion{{ID: in.Tests[0].ID, Fresh: true}}, []string{in.Tests[0].ID})
			if len(mandated.Tests) != 3 {
				t.Fatalf("%+v", mandated)
			}
			c, err := p.Command(context.Background(), registry.CommandRequest{Key: o.Key, Purpose: "run", RunDir: "/run", Tests: plan.Tests})
			if err != nil {
				t.Fatal(err)
			}
			if runner == "swift-testing" {
				if !c.Superset || slices.Contains(c.Argv, "--filter") {
					t.Fatalf("%+v", c)
				}
			} else {
				for i, arg := range c.Argv {
					if arg == "--filter" {
						pattern := c.Argv[i+1]
						re, err := regexp.Compile(pattern)
						if err != nil || !strings.HasPrefix(pattern, "^") || !strings.HasSuffix(pattern, "$") {
							t.Fatal(pattern)
						}
						for _, test := range in.Tests {
							if re.MatchString(test.ID) && !slices.Contains(plan.Tests, test.ID) {
								t.Fatal("excluded test selected by filter")
							}
						}
						if re.MatchString("Prefix" + plan.Tests[0]) {
							t.Fatal("unanchored filter")
						}
					}
				}
			}
			for _, purpose := range []string{"inventory", "collect"} {
				c, err := p.Command(context.Background(), registry.CommandRequest{Key: o.Key, Purpose: purpose, RunDir: "/run"})
				if err != nil || slices.Contains(c.Argv, "--filter") {
					t.Fatalf("%+v %v", c, err)
				}
				want := "--list-tests"
				if purpose == "collect" {
					want = "--enable-code-coverage"
				}
				if !slices.Contains(c.Argv, want) {
					t.Fatal(c.Argv)
				}
			}
		})
	}
}
func TestRSPV0004SwiftInvalidSelectionNeverRunsSubset(t *testing.T) {
	o := swiftOptions(t, "swift-xctest")
	p := swiftPlugin(t, o)
	swiftInventory(t, p, o)
	for _, ids := range [][]string{{}, {"unknown"}, {"ProofTests.Proof/testPass", "ProofTests.Proof/testPass"}} {
		if _, err := p.Command(context.Background(), registry.CommandRequest{Key: o.Key, Purpose: "run", RunDir: "/run", Tests: ids}); err == nil {
			t.Fatal("invalid set admitted")
		}
	}
	for _, purpose := range []string{"unknown", "inventory"} {
		if _, err := p.Command(context.Background(), registry.CommandRequest{Key: o.Key, Purpose: purpose, RunDir: "/run", Tests: []string{"ProofTests.Proof/testPass"}}); err == nil {
			t.Fatal("invalid purpose/filter admitted")
		}
	}
}
func TestRSPV0005SwiftLLVMLineEvidence(t *testing.T) {
	for _, runner := range []string{"swift-testing", "swift-xctest"} {
		t.Run(runner, func(t *testing.T) {
			o := swiftOptions(t, runner)
			p := swiftPlugin(t, o)
			swiftInventory(t, p, o)
			// The factory retains an immutable copy of captures and source bindings.
			b := o.Sources["/fixture/Sources/Logic.swift"]
			b.OID = "dirty"
			o.Sources["/fixture/Sources/Logic.swift"] = b
			o.Coverage["coverage"].LCOV[0] = '!'
			records, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: runner, Artifacts: []string{"coverage"}})
			if err != nil || len(records) != 3 {
				t.Fatalf("%+v %v", records, err)
			}
			var first json.RawMessage
			for _, record := range records {
				r := testevidence.New("primary", record.Test, o.Key.Checkout)
				r.Executed = testevidence.Component[testevidence.ExecutedFile]{}
				r.Regions = testevidence.Component[testevidence.RegionsFile]{}
				r.Loaded = testevidence.Component[testevidence.Blob]{}
				if err := json.Unmarshal(record.Executed, &r.Executed); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(record.Regions, &r.Regions); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(record.Loaded, &r.Loaded); err != nil {
					t.Fatal(err)
				}
				if err := r.ValidateComponents(); err != nil {
					t.Fatal(err)
				}
				if r.Executed.State != testevidence.Present || len(r.Executed.Items) != 1 {
					t.Fatalf("%+v", r.Executed)
				}
				file := r.Executed.Items[0]
				if file.Blob.OID != strings.Repeat("c", 40) || len(file.Executed) != 1 || file.Executed[0].Start != 2 || len(file.Unexecuted) != 1 || file.Unexecuted[0].Start != 3 {
					t.Fatalf("%+v", file)
				}
				if r.Regions.State != testevidence.Absent || r.Regions.Reason != "llvm-cov-lines-no-body-map" || r.Loaded.State != testevidence.Absent || r.Loaded.Reason != "swift-build-graph-unavailable" {
					t.Fatal("line records fabricated bodies/build graph")
				}
				if r.Reads.State != testevidence.Absent || r.External.State != testevidence.Absent {
					t.Fatal("plugin supplied core trace components")
				}
				if first != nil && !bytes.Equal(first, record.Executed) {
					t.Fatal("shared-tree coverage not attributed to every test")
				}
				first = record.Executed
			}
		})
	}
}
func TestRSPV0005SwiftInvalidEvidencePublishesNothing(t *testing.T) {
	for _, runner := range []string{"swift-testing", "swift-xctest"} {
		t.Run(runner, func(t *testing.T) {
			valid := swiftOptions(t, runner)
			p := swiftPlugin(t, valid)
			if _, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: runner, Artifacts: []string{"coverage"}}); err != nil {
				t.Fatal(err)
			}
			for _, mode := range []string{"truncated", "incomplete", "unknown-source", "negative-count", "zero-line", "malformed", "capped", "unknown-test", "no-tests", "duplicate-test"} {
				t.Run(mode, func(t *testing.T) {
					o := swiftOptions(t, runner)
					capture := o.Coverage["coverage"]
					switch mode {
					case "truncated":
						capture.LCOV = capture.LCOV[:len(capture.LCOV)-4]
					case "incomplete":
						capture.Complete = false
					case "unknown-source":
						capture.LCOV = bytes.ReplaceAll(capture.LCOV, []byte("/fixture/Sources/Logic.swift"), []byte("/outside/Unknown.swift"))
					case "negative-count":
						capture.LCOV = bytes.ReplaceAll(capture.LCOV, []byte("DA:2,1"), []byte("DA:2,-1"))
					case "zero-line":
						capture.LCOV = bytes.ReplaceAll(capture.LCOV, []byte("DA:2,1"), []byte("DA:0,1"))
					case "malformed":
						capture.LCOV = []byte("not LLVM coverage\n")
					case "capped":
						capture.LCOV = append(bytes.Repeat([]byte("x"), 4<<20), '\n')
					case "unknown-test":
						capture.Tests = []string{"unknown"}
					case "no-tests":
						capture.Tests = nil
					case "duplicate-test":
						capture.Tests = append(capture.Tests, capture.Tests[0])
					}
					o.Coverage["coverage"] = capture
					records, err := swiftPlugin(t, o).Evidence(context.Background(), registry.EvidenceRequest{Runner: runner, Artifacts: []string{"coverage"}})
					if err == nil || len(records) != 0 {
						t.Fatalf("partial evidence admitted: %+v %v", records, err)
					}
					if mode == "capped" && !strings.Contains(err.Error(), "native coverage report size") {
						t.Fatalf("cap path not reached: %v", err)
					}
				})
			}
			for _, handles := range [][]string{nil, {"missing"}, {"coverage", "coverage"}, {"coverage", "missing"}} {
				records, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: runner, Artifacts: handles})
				if err == nil || len(records) != 0 {
					t.Fatal("missing/duplicate artifact admitted")
				}
			}
		})
	}
}

func TestRSPV0003SwiftSkipWithoutReasonDiverges(t *testing.T) {
	o := swiftOptions(t, "swift-testing")
	p := swiftPlugin(t, o)
	in := swiftInventory(t, p, o)
	if results, err := p.Parse(context.Background(), registry.ParseRequest{Runner: o.Key.Runner, Report: "mixed"}); err != nil || len(results) != 3 {
		t.Fatalf("initial parse: %+v %v", results, err)
	}
	input := o.Reports["mixed"]
	var lines [][]byte
	for _, line := range bytes.Split(bytes.TrimSuffix(input.Reports["swift-events.jsonl"], []byte("\n")), []byte("\n")) {
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatal(err)
		}
		payload := record["payload"].(map[string]any)
		if record["kind"] == "event" && payload["kind"] == "testSkipped" {
			payload["messages"] = []any{}
		}
		raw, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, raw)
	}
	input.Reports["swift-events.jsonl"] = append(bytes.Join(lines, []byte("\n")), '\n')
	o.Reports["mixed"] = input
	results, err := swiftPlugin(t, o).Parse(context.Background(), registry.ParseRequest{Runner: o.Key.Runner, Report: "mixed"})
	if err != nil {
		t.Fatal(err)
	}
	rec := tr.ReconcileInventory(in, o.Key, tr.RunSet{Full: true}, results, false)
	if !rec.NeedsRequalification || len(rec.Missing) != 1 || !strings.Contains(strings.Join(rec.Reasons, ","), "skip-reason-missing") {
		t.Fatalf("%+v", rec)
	}
}
func TestRSPV0005SwiftUnionPreservesExecutedAndUnexecuted(t *testing.T) {
	o := swiftOptions(t, "swift-xctest")
	first := o.Coverage["coverage"]
	second := first
	second.LCOV = bytes.ReplaceAll(first.LCOV, []byte("DA:3,0"), []byte("DA:3,4"))
	o.Coverage["second"] = second
	p := swiftPlugin(t, o)
	for _, handles := range [][]string{{"coverage", "second"}, {"second", "coverage"}} {
		records, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: o.Key.Runner, Artifacts: handles})
		if err != nil || len(records) != 3 {
			t.Fatalf("%+v %v", records, err)
		}
		for _, record := range records {
			var c testevidence.Component[testevidence.ExecutedFile]
			if err := json.Unmarshal(record.Executed, &c); err != nil {
				t.Fatal(err)
			}
			if len(c.Items) != 1 || len(c.Items[0].Executed) != 2 || len(c.Items[0].Unexecuted) != 0 {
				t.Fatalf("%+v", c)
			}
		}
	}
}
func TestRSPV0005SwiftCancelledRequestsProduceNothing(t *testing.T) {
	o := swiftOptions(t, "swift-xctest")
	p := swiftPlugin(t, o)
	swiftInventory(t, p, o)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if in, err := p.Inventory(ctx, registry.InventoryRequest{Key: o.Key, Output: swiftList(t, o.Key.Runner)}); err == nil || in.Complete || in.Reason == "" {
		t.Fatal("cancelled inventory admitted")
	}
	if _, err := p.Command(ctx, registry.CommandRequest{Key: o.Key, Purpose: "run", RunDir: "/run"}); err == nil {
		t.Fatal("cancelled command admitted")
	}
	if records, err := p.Parse(ctx, registry.ParseRequest{Runner: o.Key.Runner, Report: "mixed"}); err == nil || len(records) != 0 {
		t.Fatal("cancelled parse admitted")
	}
	if records, err := p.Evidence(ctx, registry.EvidenceRequest{Runner: o.Key.Runner, Artifacts: []string{"coverage"}}); err == nil || len(records) != 0 {
		t.Fatal("cancelled evidence admitted")
	}
}

// RSP-V0-003: a complete passing FULL retry cannot erase an earlier failure.
func TestRSPV0003SwiftIncompleteFailureSurvivesRetry(t *testing.T) {
	for _, runner := range []string{"swift-xctest", "swift-testing"} {
		for _, mode := range []string{"missing-end", "truncated", "malformed", "invalid-utf8", "timeout", "interrupted", "overflow"} {
			t.Run(runner+"/"+mode, func(t *testing.T) {
				o := swiftOptions(t, runner)
				initial := o.Reports["mixed"]
				passing := tr.Input{Runner: runner, ExitCode: 0}
				var prefix []byte
				if runner == "swift-xctest" {
					lines := bytes.SplitAfter(initial.Stdout, []byte("\n"))
					var good []byte
					for _, line := range lines {
						if bytes.Contains(line, []byte("error: -[")) || bytes.HasPrefix(line, []byte("Note:")) || bytes.HasPrefix(line, []byte("  - ")) {
							continue
						}
						line = bytes.ReplaceAll(line, []byte(" failed"), []byte(" passed"))
						line = bytes.ReplaceAll(line, []byte("1 failure ("), []byte("0 failures ("))
						good = append(good, line...)
					}
					passing.Stdout = good
					end := bytes.Index(initial.Stdout, []byte("Test Case '-[ProofTests.Proof testPass]' started."))
					if end < 0 {
						t.Fatal("failure boundary missing")
					}
					prefix = append([]byte(nil), initial.Stdout[:end]...)
				} else {
					var good []byte
					for _, line := range bytes.SplitAfter(initial.Reports["swift-events.jsonl"], []byte("\n")) {
						if bytes.Contains(line, []byte(`"kind":"issueRecorded"`)) {
							continue
						}
						line = bytes.ReplaceAll(line, []byte(`"symbol":"fail"`), []byte(`"symbol":"pass"`))
						good = append(good, line...)
					}
					passing.Reports = map[string][]byte{"swift-events.jsonl": good}
					raw := initial.Reports["swift-events.jsonl"]
					end := bytes.Index(raw, []byte(`{"kind":"event","payload":{"_comments":["not available"]`))
					if end < 0 {
						t.Fatal("failure boundary missing")
					}
					prefix = append([]byte(nil), raw[:end]...)
				}
				switch mode {
				case "truncated":
					prefix = append(prefix, []byte(`{"partial":`)...)
				case "invalid-utf8":
					prefix = append(prefix, 0xff, '\n')
				case "malformed":
					prefix = append(prefix, []byte("Test Case malformed\n")...)
				case "timeout":
					initial.TimedOut = true
				case "interrupted":
					initial.Interrupted = true
				case "overflow":
					initial.Overflow = true
				}
				if runner == "swift-xctest" {
					initial.Stdout = prefix
				} else {
					initial.Reports["swift-events.jsonl"] = prefix
				}
				o.Reports["initial"], o.Reports["retry"] = initial, passing
				p := swiftPlugin(t, o)
				in := swiftInventory(t, p, o)
				// Prove the retry fixture is independently complete and successful.
				results, err := p.Parse(context.Background(), registry.ParseRequest{Runner: runner, Report: "retry"})
				if err != nil || !tr.ReconcileInventory(in, o.Key, tr.RunSet{Full: true}, results, false).Succeeded {
					t.Fatalf("invalid passing retry fixture: %+v %v", results, err)
				}
				calls := 0
				out := registry.NewSelectionSession(p, o.Key, false).Run(context.Background(), in, nil, nil, "/run", true,
					func(_ context.Context, command registry.CommandResponse) (string, error) {
						calls++
						if slices.Contains(command.Argv, "--filter") {
							t.Fatal("FULL retry was filtered")
						}
						if calls == 1 {
							return "initial", nil
						}
						return "retry", nil
					})
				if calls != 2 || len(out.Attempts) != 2 || !out.Attempts[1].Reconciliation.Succeeded || out.Succeeded || !out.NeedsRequalification {
					t.Fatalf("reported failure disappeared across retry: %+v", out)
				}
				failed := 0
				for _, result := range out.Attempts[0].Results {
					if result.Outcome == "failed" {
						failed++
					}
				}
				if failed != 1 {
					t.Fatalf("validated initial failure lost: %+v", out.Attempts[0])
				}
			})
		}
	}
}
