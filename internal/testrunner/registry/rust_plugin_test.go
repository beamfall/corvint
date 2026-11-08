// SPDX-License-Identifier: Apache-2.0

package registry_test

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

func rustOptions(runner string) registry.RustPluginOptions {
	root := filepath.Join(string(filepath.Separator), "fixture", "rust")
	abs := func(name string) string { return filepath.Join(root, filepath.FromSlash(name)) }
	sources := map[string]registry.RustSource{}
	for name, text := range map[string]string{
		"Cargo.toml":    "[package]\nname='probe'\nversion='0.1.0'\n",
		"Cargo.lock":    "version = 4\n",
		"src/lib.rs":    "pub fn value() -> bool {\n    true\n}\npub fn other() -> bool {\n    false\n}\n",
		"src/helper.rs": "pub fn helper() {}\n",
		"build.rs":      "fn main() {}\n",
		"schema/input":  "schema\n",
	} {
		h := sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(text), text)))
		sources[name] = registry.RustSource{Data: []byte(text), Blob: testevidence.Blob{Path: testevidence.Path{Repository: "repo", Lexical: name, Resolved: name}, OID: hex.EncodeToString(h[:])}}
	}
	id := func(name string) string {
		if runner == "nextest" {
			return "probe::" + name
		}
		return name
	}
	meta, _ := json.Marshal(map[string]any{
		"version": 1, "workspace_root": root, "workspace_members": []string{"probe"},
		"packages": []any{map[string]any{"id": "probe", "source": nil, "manifest_path": abs("Cargo.toml"), "targets": []any{
			map[string]any{"kind": []string{"lib"}, "src_path": abs("src/lib.rs")},
			map[string]any{"kind": []string{"custom-build"}, "src_path": abs("build.rs")},
		}}},
	})
	build, _ := json.Marshal(map[string]any{"reason": "build-script-executed", "package_id": "probe", "out_dir": abs("target/build/out"), "linked_paths": []string{}})
	return registry.RustPluginOptions{
		CompilerEnvironment: map[string]string{"RUSTFLAGS": ""},
		Key:                 testrunner.InventoryKey{Checkout: strings.Repeat("a", 40), Runner: runner, ConfigDigest: strings.Repeat("b", 64), OS: "linux", Arch: "amd64"},
		Root:                root, Manifest: "Cargo.toml", Sources: sources, SourcesComplete: true, CoverageComplete: true,
		TestSources:   map[string]string{id("a"): "src/lib.rs", id("b"): "src/lib.rs", id("c"): "src/lib.rs"},
		NativeSources: map[string]string{abs("src/lib.rs"): "src/lib.rs"}, Metadata: meta,
		BuildMessages: append(build, []byte("\n{\"reason\":\"build-finished\",\"success\":true}\n")...),
		BuildScripts:  map[string][]byte{"probe": []byte("cargo::rerun-if-changed=schema\n")},
		Artifacts:     map[string][]byte{"lines": []byte("SF:" + abs("src/lib.rs") + "\nDA:2,1\nDA:5,0\nend_of_record\n")},
		Tests:         []string{id("a"), id("b")},
	}
}

func rustNew(t *testing.T, o registry.RustPluginOptions) registry.RunnerPlugin {
	t.Helper()
	p, err := registry.NewRustPlugin(o)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func rustListing(runner string) string {
	if runner == "cargo-test" {
		return "b: test\na: test\nc: test\n\n3 tests, 0 benchmarks\n"
	}
	return `{"rust-build-meta":{},"test-count":3,"rust-suites":{"probe":{"binary-id":"probe","kind":"lib","status":"listed","testcases":{"b":{"ignored":false,"filter-match":{"status":"matches"}},"a":{"ignored":false,"filter-match":{"status":"matches"}},"c":{"ignored":true,"filter-match":{"status":"matches"}}}}}}`
}

func rustReport(runner string, ids ...string) string {
	if runner == "nextest" {
		s := `<testsuites><testsuite name="probe" tests="` + fmt.Sprint(len(ids)) + `">`
		for _, id := range ids {
			s += `<testcase classname="probe" name="` + id + `"/>`
		}
		return s + `</testsuite></testsuites>`
	}
	s := fmt.Sprintf("running %d tests\n", len(ids))
	for _, id := range ids {
		s += "test " + id + " ... ok\n"
	}
	return s + fmt.Sprintf("test result: ok. %d passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.00s\n", len(ids))
}

func rustNativeRecord(t *testing.T, n registry.NativeEvidence) testevidence.Record {
	t.Helper()
	r := testevidence.New("repo", n.Test, strings.Repeat("a", 40))
	// Decode complete components into zero values: omitted optional reasons
	// must not retain New's initial missing-component reasons.
	r.Executed = testevidence.Component[testevidence.ExecutedFile]{}
	r.Regions = testevidence.Component[testevidence.RegionsFile]{}
	r.Loaded = testevidence.Component[testevidence.Blob]{}
	if err := json.Unmarshal(n.Executed, &r.Executed); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(n.Regions, &r.Regions); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(n.Loaded, &r.Loaded); err != nil {
		t.Fatal(err)
	}
	if err := r.ValidateComponents(); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRSPV0001RustPieces(t *testing.T) {
	for _, runner := range []string{"cargo-test", "nextest"} {
		o := rustOptions(runner)
		p := rustNew(t, o)
		pieces := p.Pieces()
		if !pieces.Inventory.Present || !pieces.TestEvidence.Present || pieces.Qualification.Present || pieces.Qualification.Reason != "unqualified" {
			t.Fatalf("%s: %+v", runner, pieces)
		}
		s := registry.NewSelectionSession(p, o.Key, false)
		in := s.Inventory(context.Background(), rustListing(runner))
		plan := s.Plan(in, nil, nil)
		if !plan.Full || !strings.Contains(plan.Reason, "unqualified") || len(plan.Tests) != 3 {
			t.Fatalf("%s: %+v", runner, plan)
		}
	}
}

func TestRSPV0001RustCannotSelfQualify(t *testing.T) {
	for _, runner := range []string{"cargo-test", "nextest"} {
		o := rustOptions(runner)
		p := rustNew(t, o)
		s := registry.NewSelectionSession(p, o.Key, true)
		in := s.Inventory(context.Background(), rustListing(runner))
		var excluded []testrunner.Exclusion
		for _, test := range in.Tests {
			excluded = append(excluded, testrunner.Exclusion{ID: test.ID, Fresh: true})
		}
		if plan := s.Plan(in, excluded, nil); !plan.Full || len(plan.Tests) != 3 || !strings.Contains(plan.Reason, "unqualified") {
			t.Fatalf("%s: %+v", runner, plan)
		}
	}
	o := rustOptions("cargo-doctest")
	if _, err := registry.NewRustPlugin(o); err == nil {
		t.Fatal("unsupported runner acquired pieces")
	}
}

func TestRSPV0002RustDeterministicInventory(t *testing.T) {
	for _, runner := range []string{"cargo-test", "nextest"} {
		o := rustOptions(runner)
		p := rustNew(t, o)
		req := registry.InventoryRequest{Key: o.Key, Output: rustListing(runner)}
		a, err := p.Inventory(context.Background(), req)
		if err != nil || !a.Complete || len(a.Tests) != 3 {
			t.Fatalf("%s: %+v %v", runner, a, err)
		}
		b, err := p.Inventory(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		ab, err := testrunner.InventoryBytes(a, o.Key)
		if err != nil {
			t.Fatal(err)
		}
		bb, err := testrunner.InventoryBytes(b, o.Key)
		if err != nil || string(ab) != string(bb) {
			t.Fatalf("noncanonical: %v", err)
		}
		for _, test := range a.Tests {
			if test.Path != "src/lib.rs" {
				t.Fatal(test)
			}
		}
		foreign := o.Key
		foreign.Arch = "arm64"
		if _, err := testrunner.CanonicalInventory(a, foreign); err == nil {
			t.Fatal("cross-target inventory admitted")
		}
		cmd, err := p.Command(context.Background(), registry.CommandRequest{Key: o.Key, Purpose: "inventory"})
		if err != nil || cmd.Cwd != o.Root || !slices.Contains(cmd.Argv, "--locked") {
			t.Fatalf("command: %+v %v", cmd, err)
		}
		if runner == "cargo-test" && !slices.Contains(cmd.Argv, "--list") || runner == "nextest" && !slices.Contains(cmd.Argv, "json") {
			t.Fatal(cmd)
		}
	}
}

func TestRSPV0002RustIncompleteInventory(t *testing.T) {
	for _, runner := range []string{"cargo-test", "nextest"} {
		o := rustOptions(runner)
		p := rustNew(t, o)
		bad := []string{"", "a: test\n", "a: test\na: test\n2 tests, 0 benchmarks\n", "a: benchmark\n0 tests, 1 benchmark\n", "0 tests, 0 benchmarks\ntrailing", "unknown: test\n1 test, 0 benchmarks\n", strings.Repeat("x", testrunner.MaxReportBytes+1)}
		if runner == "nextest" {
			bad = append(bad, `{"test-count":1,"rust-suites":{}}`, `{"test-count":0,"test-count":0,"rust-suites":{}}`, strings.Replace(rustListing(runner), `"listed"`, `"skipped"`, 1), strings.Replace(rustListing(runner), `"matches"`, `"mismatch"`, 1), strings.Replace(rustListing(runner), `"ignored":false`, `"ignored":null`, 1))
		}
		for i, raw := range bad {
			in, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: o.Key, Output: raw})
			if err == nil || in.Complete || in.Reason == "" {
				t.Fatalf("%s/%d narrowed: %+v %v", runner, i, in, err)
			}
		}
		key := o.Key
		key.ConfigDigest = strings.Repeat("c", 64)
		if in, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: key, Output: rustListing(runner)}); err == nil || in.Complete {
			t.Fatal("configuration mismatch admitted")
		}
	}
}

func TestRSPV0003RustTerminalResults(t *testing.T) {
	for _, runner := range []string{"cargo-test", "nextest"} {
		o := rustOptions(runner)
		p := rustNew(t, o)
		in, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: o.Key, Output: rustListing(runner)})
		if err != nil {
			t.Fatal(err)
		}
		results, err := p.Parse(context.Background(), registry.ParseRequest{Runner: runner, Report: rustReport(runner, "a", "b", "c")})
		if err != nil {
			t.Fatal(err)
		}
		plan := testrunner.PlanRunSet(in, o.Key, nil, nil)
		if r := testrunner.ReconcileInventory(in, o.Key, plan, results, false); !r.Succeeded || r.Full {
			t.Fatalf("%s: %+v", runner, r)
		}
	}
	p := rustNew(t, rustOptions("cargo-test"))
	report := "running 2 tests\ntest a ... FAILED\ntest b ... ignored, unavailable fixture\ntest result: FAILED. 0 passed; 1 failed; 1 ignored; 0 measured; 0 filtered out; finished in 0.00s\n"
	results, err := p.Parse(context.Background(), registry.ParseRequest{Runner: "cargo-test", Report: report})
	if err != nil || len(results) != 2 || results[0].Outcome != "failed" || results[1].Reason != "ignored, unavailable fixture" {
		t.Fatalf("%+v %v", results, err)
	}
	p = rustNew(t, rustOptions("nextest"))
	report = `<testsuites><testsuite tests="2"><testcase classname="probe" name="a"><failure message="assertion failed"/></testcase><testcase classname="probe" name="b"><skipped message="fixture unavailable"/></testcase></testsuite></testsuites>`
	results, err = p.Parse(context.Background(), registry.ParseRequest{Runner: "nextest", Report: report})
	if err != nil || len(results) != 2 || results[0].Outcome != "failed" || results[1].Reason != "fixture unavailable" {
		t.Fatalf("%+v %v", results, err)
	}
}

// RSP-V0-003: an unsupported nextest row sorted first cannot hide a later
// failed row; the failure survives alongside the nonterminal error.
func TestRSPV0003RustNonterminalRowKeepsLaterFailure(t *testing.T) {
	p := rustNew(t, rustOptions("nextest"))
	report := `<testsuites><testsuite tests="2"><testcase classname="probe" name="a_crash"><error message="process aborted"/></testcase><testcase classname="probe" name="z_fail"><failure message="assertion failed"/></testcase></testsuite></testsuites>`
	results, err := p.Parse(context.Background(), registry.ParseRequest{Runner: "nextest", Report: report})
	if err == nil || !strings.Contains(err.Error(), "rust-report-nonterminal") {
		t.Fatalf("unsupported row accepted: %+v %v", results, err)
	}
	if len(results) != 1 || !strings.HasSuffix(results[0].ID, "z_fail") || results[0].Outcome != "failed" {
		t.Fatalf("later failure lost: %+v %v", results, err)
	}
}

func TestRSPV0003RustDivergenceRerunsFull(t *testing.T) {
	for _, runner := range []string{"cargo-test", "nextest"} {
		o := rustOptions(runner)
		p := rustNew(t, o)
		s := registry.NewSelectionSession(p, o.Key, false)
		in := s.Inventory(context.Background(), rustListing(runner))
		calls := 0
		run := s.Run(context.Background(), in, nil, nil, o.Root, true, func(_ context.Context, cmd registry.CommandResponse) (string, error) {
			calls++
			if slices.Contains(cmd.Argv, "--exact") {
				t.Fatal("FULL retry acquired filter")
			}
			if calls == 1 {
				return rustReport(runner, "a", "b"), nil
			}
			return rustReport(runner, "a", "b", "c"), nil
		})
		if calls != 2 || !run.Succeeded || !run.NeedsRequalification || len(run.Attempts[0].Reconciliation.Missing) != 1 {
			t.Fatalf("%s: %+v", runner, run)
		}
		if plan := s.Plan(in, nil, nil); !plan.Full || !strings.Contains(plan.Reason, "requalification-required") {
			t.Fatal(plan)
		}
		results, err := p.Parse(context.Background(), registry.ParseRequest{Runner: runner, Report: rustReport(runner, "unknown")})
		if err != nil {
			t.Fatal(err)
		}
		if r := testrunner.ReconcileInventory(in, o.Key, testrunner.PlanRunSet(in, o.Key, nil, nil), results, true); r.Succeeded || !r.NeedsRequalification || len(r.Unexpected) != 1 {
			t.Fatal(r)
		}
		if _, err := p.Parse(context.Background(), registry.ParseRequest{Runner: runner, Report: "running 1 test\ntest a ... ok\n"}); err == nil {
			t.Fatal("truncated results admitted")
		}
	}
}

func TestRSPV0004RustRunSetAndSuperset(t *testing.T) {
	for _, runner := range []string{"cargo-test", "nextest"} {
		o := rustOptions(runner)
		p := rustNew(t, o)
		in, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: o.Key, Output: rustListing(runner)})
		if err != nil {
			t.Fatal(err)
		}
		ids := []string{in.Tests[0].ID, in.Tests[1].ID, in.Tests[2].ID}
		plan := testrunner.PlanRunSet(in, o.Key, []testrunner.Exclusion{{ID: ids[0], Fresh: true}, {ID: ids[1], Fresh: false}}, nil)
		if plan.Full || !slices.Equal(plan.Tests, ids[1:]) {
			t.Fatal(plan)
		}
		// The head-new identity c and the stale identity b both stay selected.
		cmd, err := p.Command(context.Background(), registry.CommandRequest{Key: o.Key, Purpose: "run", Tests: plan.Tests, RunDir: o.Root})
		if err != nil || !cmd.Superset || slices.Contains(cmd.Argv, "--exact") || slices.Contains(cmd.Argv, "-E") {
			t.Fatalf("%s: %+v %v", runner, cmd, err)
		}
		plan = testrunner.PlanRunSet(in, o.Key, []testrunner.Exclusion{{ID: ids[0], Fresh: true}}, []string{ids[0]})
		if !slices.Equal(plan.Tests, ids) {
			t.Fatal("mandate dropped", plan)
		}
		cmd, err = p.Command(context.Background(), registry.CommandRequest{Key: o.Key, Purpose: "collect", Tests: ids[:1], RunDir: o.Root})
		if err != nil || cmd.Env["RUSTFLAGS"] != "-C instrument-coverage" || !strings.Contains(cmd.Env["LLVM_PROFILE_FILE"], "%p-%m") {
			t.Fatalf("%+v %v", cmd, err)
		}
		if runner == "cargo-test" && (cmd.Superset || !slices.Contains(cmd.Argv, "--exact")) {
			t.Fatal(cmd)
		}
		files, err := registry.RustRunFiles(runner, o.Manifest, o.Root)
		if err != nil {
			t.Fatal(err)
		}
		if runner == "nextest" && (!strings.Contains(string(files["nextest.toml"]), "retries=0") || !strings.Contains(string(files["nextest.toml"]), "report-skipped=\"all\"")) {
			t.Fatal(files)
		}
	}
}

func TestRSPV0004RustRefusesUnsafeRequests(t *testing.T) {
	o := rustOptions("cargo-test")
	p := rustNew(t, o)
	for _, req := range []registry.CommandRequest{
		{Key: o.Key, Purpose: "run", Tests: []string{}}, {Key: o.Key, Purpose: "run", Tests: []string{"unknown"}},
		{Key: o.Key, Purpose: "collect", RunDir: "relative"}, {Key: o.Key, Purpose: "collect", RunDir: filepath.Join(o.Root, "bad%p")},
		{Key: o.Key, Purpose: "inventory", Tests: []string{"a"}}, {Key: o.Key, Purpose: "unknown"},
	} {
		if _, err := p.Command(context.Background(), req); err == nil {
			t.Fatalf("unsafe command: %+v", req)
		}
	}
	key := o.Key
	key.Checkout = strings.Repeat("c", 40)
	if _, err := p.Command(context.Background(), registry.CommandRequest{Key: key, Purpose: "run"}); err == nil {
		t.Fatal("wrong checkout admitted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Command(ctx, registry.CommandRequest{Key: o.Key, Purpose: "run"}); err == nil {
		t.Fatal("cancelled command admitted")
	}
}

func TestRSPV0005RustNativeEvidence(t *testing.T) {
	for _, runner := range []string{"cargo-test", "nextest"} {
		o := rustOptions(runner)
		p := rustNew(t, o)
		native, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: runner, Artifacts: []string{"lines"}})
		if err != nil || len(native) != 2 {
			t.Fatalf("%s: %v", runner, err)
		}
		a := rustNativeRecord(t, native[0])
		b := rustNativeRecord(t, native[1])
		if a.Executed.State != testevidence.Present || a.Regions.State != testevidence.Present || a.Loaded.State != testevidence.Present {
			t.Fatalf("%+v", a)
		}
		if !reflect.DeepEqual(a.Executed, b.Executed) || !reflect.DeepEqual(a.Regions, b.Regions) || !reflect.DeepEqual(a.Loaded, b.Loaded) {
			t.Fatal("shared process lost conservative attribution")
		}
		f := a.Executed.Items[0]
		if len(f.Executed) != 1 || f.Executed[0].Start != 2 || len(f.Unexecuted) != 1 || f.Unexecuted[0].Start != 5 || len(a.Regions.Items[0].Bodies) != 2 {
			t.Fatalf("%+v %+v", f, a.Regions)
		}
		loaded := map[string]bool{}
		for _, blob := range a.Loaded.Items {
			loaded[blob.Path.Resolved] = true
		}
		for _, name := range []string{"Cargo.toml", "Cargo.lock", "src/lib.rs", "src/helper.rs", "build.rs", "schema/input"} {
			if !loaded[name] {
				t.Fatalf("missing loaded input %s", name)
			}
		}
		// Raw artifacts and factory source bytes cannot be mutated afterward.
		o.Artifacts["lines"][0] = '!'
		o.Sources["src/lib.rs"].Data[0] = '!'
		o.TestSources["a"] = "missing"
		again, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: runner, Artifacts: []string{"lines"}})
		if err != nil || !reflect.DeepEqual(native, again) {
			t.Fatal("caller mutated captured evidence", err)
		}
	}
}

func TestRSPV0005RustUnknownArtifactsWiden(t *testing.T) {
	for name, edit := range map[string]func(*registry.RustPluginOptions){
		"missing-coverage":   func(o *registry.RustPluginOptions) { delete(o.Artifacts, "lines") },
		"truncated-coverage": func(o *registry.RustPluginOptions) { o.Artifacts["lines"] = []byte("SF:src/lib.rs\nDA:2,1\n") },
		"unmapped-coverage":  func(o *registry.RustPluginOptions) { o.NativeSources = map[string]string{} },
		"process-gap":        func(o *registry.RustPluginOptions) { o.CoverageComplete = false },
		"line-outside-blob": func(o *registry.RustPluginOptions) {
			o.Artifacts["lines"] = []byte("SF:" + filepath.Join(o.Root, "src/lib.rs") + "\nDA:999,1\nend_of_record\n")
		},
	} {
		t.Run(name, func(t *testing.T) {
			o := rustOptions("cargo-test")
			edit(&o)
			p := rustNew(t, o)
			native, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: o.Key.Runner, Artifacts: []string{"lines"}})
			if err != nil {
				t.Fatal(err)
			}
			r := rustNativeRecord(t, native[0])
			if r.Executed.State == testevidence.Present || r.Executed.Reason == "" {
				t.Fatalf("uncertainty narrowed: %+v", r.Executed)
			}
		})
	}
	o := rustOptions("cargo-test")
	p := rustNew(t, o)
	native, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: o.Key.Runner})
	if err != nil {
		t.Fatal(err)
	}
	if r := rustNativeRecord(t, native[0]); r.Executed.State != testevidence.Absent || r.Regions.State != testevidence.Absent {
		t.Fatal(r)
	}
	if _, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: "nextest", Artifacts: []string{"lines"}}); err == nil {
		t.Fatal("foreign runner supplied evidence")
	}
	o.Sources["src/lib.rs"] = registry.RustSource{Blob: o.Sources["src/lib.rs"].Blob, Data: []byte("dirty bytes")}
	if _, err := registry.NewRustPlugin(o); err == nil {
		t.Fatal("stale blob binding admitted")
	}
}

func TestRSPV0005RustBuildGraphFailuresWiden(t *testing.T) {
	for name, edit := range map[string]func(*registry.RustPluginOptions){
		"no-metadata":        func(o *registry.RustPluginOptions) { o.Metadata = nil },
		"malformed-metadata": func(o *registry.RustPluginOptions) { o.Metadata = []byte(`{"version":1,"version":1}`) },
		"snapshot-gap":       func(o *registry.RustPluginOptions) { o.SourcesComplete = false },
		"unknown-package-source": func(o *registry.RustPluginOptions) {
			o.Metadata = []byte(strings.Replace(string(o.Metadata), `"packages":[`, `"packages":[{"id":"external","source":"unknown","manifest_path":"/outside/Cargo.toml","targets":[]},`, 1))
		},
		"build-gap": func(o *registry.RustPluginOptions) { o.BuildMessages = nil },
		"failed-build": func(o *registry.RustPluginOptions) {
			o.BuildMessages = []byte("{\"reason\":\"build-finished\",\"success\":false}\n")
		},
		"unknown-build": func(o *registry.RustPluginOptions) {
			o.BuildMessages = []byte("{\"reason\":\"unknown\"}\n{\"reason\":\"build-finished\",\"success\":true}\n")
		},
		"missing-script": func(o *registry.RustPluginOptions) { o.BuildScripts = nil },
		"native-input": func(o *registry.RustPluginOptions) {
			o.BuildScripts["probe"] = []byte("cargo::rustc-link-search=native=/outside\n")
		},
		"unknown-directive": func(o *registry.RustPluginOptions) { o.BuildScripts["probe"] = []byte("cargo::future-directive=x\n") },
		"outside-script-input": func(o *registry.RustPluginOptions) {
			o.BuildScripts["probe"] = []byte("cargo::rerun-if-changed=../../outside\n")
		},
	} {
		t.Run(name, func(t *testing.T) {
			o := rustOptions("cargo-test")
			edit(&o)
			p := rustNew(t, o)
			native, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: o.Key.Runner, Artifacts: []string{"lines"}})
			if err != nil {
				t.Fatal(err)
			}
			r := rustNativeRecord(t, native[0])
			if r.Loaded.State == testevidence.Present || r.Loaded.Reason == "" {
				t.Fatalf("build uncertainty narrowed: %+v", r.Loaded)
			}
			if name == "unknown-package-source" && r.Loaded.Reason != "rust-cargo-package-source-unsupported" {
				t.Fatalf("unknown source guard not reached: %+v", r.Loaded)
			}
		})
	}
}

func TestRSPV0005RustBuildTraceRemainsCoreOwned(t *testing.T) {
	o := rustOptions("cargo-test")
	p := rustNew(t, o)
	native, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: o.Key.Runner, Artifacts: []string{"lines"}})
	if err != nil {
		t.Fatal(err)
	}
	r := rustNativeRecord(t, native[0])
	r.Runner = testevidence.RunnerIdentity{Plugin: "fixture", PluginVersion: "1", ExecutableDigest: strings.Repeat("b", 64), RunnerVersion: "1", ConfigurationDigest: o.Key.ConfigDigest}
	r.Execution = testevidence.ExecutionIdentity{OS: o.Key.OS, Architecture: o.Key.Arch, TracingBackend: "fixture", TracingVersion: "1", EnvironmentDigest: strings.Repeat("c", 64)}
	r.Sources = []testevidence.Producer{{CoreDigest: strings.Repeat("d", 64), VerifierRevision: o.Key.Checkout, ConfigurationDigest: o.Key.ConfigDigest, Ref: "refs/heads/main", Run: "fixture", ContentDigest: strings.Repeat("e", 64)}}
	r.CollectionCount = 1
	trace := testevidence.Trace{
		Reads:    testevidence.Component[testevidence.Blob]{State: testevidence.Present, Items: []testevidence.Blob{o.Sources["src/lib.rs"].Blob}},
		Listings: testevidence.Component[testevidence.Path]{State: testevidence.Present},
		Lookups:  testevidence.Component[testevidence.Path]{State: testevidence.Present},
		Writes:   testevidence.Component[testevidence.Write]{State: testevidence.Present},
		Execs:    testevidence.Component[testevidence.Executable]{State: testevidence.Present},
		External: testevidence.Component[testevidence.Contact]{State: testevidence.Present},
	}
	r, err = testevidence.Assemble(r, testevidence.Native{Executed: r.Executed, Regions: r.Regions, Loaded: r.Loaded}, trace)
	if err != nil {
		t.Fatal(err)
	}
	for _, modeled := range []bool{false, true} {
		records, err := testevidence.Attribute(r, testevidence.Attribution{Tests: o.Tests, BuildPhase: true, BuildGraphModeled: modeled, BuildLoaded: r.Loaded})
		if err != nil || len(records) != 2 {
			t.Fatalf("%+v %v", records, err)
		}
		for _, record := range records {
			if !reflect.DeepEqual(record.Reads, trace.Reads) || record.Executed.Items[0].Unexecuted[0].Start != 5 {
				t.Fatal("build read dropped because the test has coverage", record)
			}
		}
	}
}
