// SPDX-License-Identifier: Apache-2.0

package shaderrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/proctrace"
	"github.com/Beamfall/corvint/internal/runnerselection"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

const declaration = `{"profile":"corvint-test-commands/0","go":{"env":{},"flags":[]},"languages":{"shader":["xcrun","metal","-c","shaders/main.metal"],"shader-tests":["tools/shader-test","--fixture","shaders/main.metal"]}}`

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root, "-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, "GIT_") {
			cmd.Env = append(cmd.Env, v)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_AUTHOR_NAME=Fixture", "GIT_COMMITTER_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
func fixture(t *testing.T, files map[string]string) (string, string, *gitauth.Repository) {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q")
	write := func(name, text string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(registry.ScriptDeclarationPath, declaration)
	for name, text := range files {
		write(name, text)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "shader fixture")
	head := git(t, root, "rev-parse", "HEAD")
	repo, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		t.Fatal(err)
	}
	return root, head, repo
}
func basicFiles() map[string]string {
	return map[string]string{
		"shaders/main.metal": "#include \"common.h\"\nkernel void shade(uint i [[thread_position_in_grid]]) { helper(); }\n",
		"shaders/other.hlsl": "#include <common.h>\n[numthreads(1,1,1)] void shade(uint3 i : SV_DispatchThreadID) { helper(); }\n",
		"shaders/common.h":   "#include \"nested.h\"\nfloat helper() { return value; }\n",
		"shaders/nested.h":   "const float value = 1.0;\n",
	}
}
func plugin(t *testing.T, root, head, id string) (registry.RunnerPlugin, testrunner.InventoryKey) {
	t.Helper()
	p, key, err := Read(context.Background(), root, head, registry.ScriptDeclarationPath, id, "darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	return p, key
}

// RSP-V0-001: present inventory/evidence pieces and explicit unqualified FULL.
func TestRSPV0001ShaderPieces(t *testing.T) {
	root, head, _ := fixture(t, basicFiles())
	for _, id := range []string{"script:shader", "script:shader-tests"} {
		p, key := plugin(t, root, head, id)
		pieces := p.Pieces()
		if !pieces.Inventory.Present || !pieces.TestEvidence.Present || pieces.Qualification.Present || pieces.Qualification.Reason != "unqualified" {
			t.Fatal(pieces)
		}
		session := registry.NewSelectionSession(p, key, false)
		inv := session.Inventory(context.Background(), "")
		plan := session.Plan(inv, []testrunner.Exclusion{{ID: id, Fresh: true}}, nil)
		if !plan.Full || !strings.Contains(plan.Reason, "unqualified") || !slices.Equal(plan.Tests, []string{id}) {
			t.Fatal(plan)
		}
	}
}

// RSP-V0-001: no guessed or undeclared command can supply pieces/inventory.
func TestRSPV0001UndeclaredShader(t *testing.T) {
	root, head, _ := fixture(t, basicFiles())
	if p, _, err := Read(context.Background(), root, head, registry.ScriptDeclarationPath, "script:unknown", "darwin", "arm64"); err == nil || p != nil {
		t.Fatal("undeclared command admitted")
	}
}

// RSP-V0-002: inventories are byte equal and pinned to declaration/config/target.
func TestRSPV0002DeterministicInventory(t *testing.T) {
	root, head, _ := fixture(t, basicFiles())
	p, key := plugin(t, root, head, "script:shader")
	a, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: key})
	if err != nil {
		t.Fatal(err)
	}
	b, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: key})
	if err != nil {
		t.Fatal(err)
	}
	x, err := testrunner.InventoryBytes(a, key)
	if err != nil {
		t.Fatal(err)
	}
	y, err := testrunner.InventoryBytes(b, key)
	if err != nil {
		t.Fatal(err)
	}
	if string(x) != string(y) || !reflect.DeepEqual(a.Tests, []testrunner.InventoryTest{{ID: "script:shader", Path: registry.ScriptDeclarationPath}}) {
		t.Fatalf("%s %s", x, y)
	}
	if err := os.WriteFile(filepath.Join(root, registry.ScriptDeclarationPath), []byte("truncated"), 0600); err != nil {
		t.Fatal(err)
	}
	again, _ := plugin(t, root, head, "script:shader")
	c, err := again.Inventory(context.Background(), registry.InventoryRequest{Key: key})
	if err != nil || !reflect.DeepEqual(a, c) {
		t.Fatalf("dirty bytes substituted: %+v %v", c, err)
	}
	_, other := plugin(t, root, head, "script:shader-tests")
	if key.ConfigDigest == other.ConfigDigest {
		t.Fatal("declaration entries share config key")
	}
}

// RSP-V0-002: failed, malformed, capped and mismatched inventory inputs fail closed.
func TestRSPV0002InvalidInventory(t *testing.T) {
	root, head, _ := fixture(t, basicFiles())
	p, key := plugin(t, root, head, "script:shader")
	wrong := key
	wrong.OS = "linux"
	if inv, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: wrong}); err == nil || inv.Complete {
		t.Fatal("cross-target inventory admitted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if inv, err := p.Inventory(ctx, registry.InventoryRequest{Key: key}); err == nil || inv.Complete {
		t.Fatal("cancelled inventory admitted")
	}
	for _, raw := range []string{declaration[:len(declaration)-1], declaration + "{}", strings.Replace(declaration, `"profile":`, `"extra":true,"profile":`, 1), strings.Repeat(" ", 1<<20)} {
		badRoot, bad, _ := fixture(t, map[string]string{registry.ScriptDeclarationPath: raw})
		if p, _, err := Read(context.Background(), badRoot, bad, registry.ScriptDeclarationPath, "script:shader", "darwin", "arm64"); err == nil || p != nil {
			t.Fatal("invalid discovery admitted")
		}
	}
	if p, _, err := Read(context.Background(), root, "HEAD", registry.ScriptDeclarationPath, "script:shader", "darwin", "arm64"); err == nil || p != nil {
		t.Fatal("movable ref admitted")
	}
}

// RSP-V0-003: core-owned decimal exit observations reconcile the whole command.
func TestRSPV0003TerminalResults(t *testing.T) {
	root, head, _ := fixture(t, basicFiles())
	p, key := plugin(t, root, head, "script:shader")
	for report, success := range map[string]bool{"0": true, "1": false} {
		session := registry.NewSelectionSession(p, key, false)
		inv := session.Inventory(context.Background(), "")
		run := session.Run(context.Background(), inv, nil, nil, root, false, func(_ context.Context, cmd registry.CommandResponse) (string, error) {
			if !slices.Equal(cmd.Argv, []string{"xcrun", "metal", "-c", "shaders/main.metal"}) {
				t.Fatal(cmd)
			}
			return report, nil
		})
		if run.Succeeded != success || len(run.Attempts) != 1 || len(run.Attempts[0].Results) != 1 {
			t.Fatal(run)
		}
	}
}

// RSP-V0-003: missing/invalid native results fail, retry FULL once, and stay divergent.
func TestRSPV0003MissingResultFullRetry(t *testing.T) {
	root, head, _ := fixture(t, basicFiles())
	p, key := plugin(t, root, head, "script:shader")
	session := registry.NewSelectionSession(p, key, false)
	inv := session.Inventory(context.Background(), "")
	calls := 0
	run := session.Run(context.Background(), inv, nil, nil, root, true, func(context.Context, registry.CommandResponse) (string, error) {
		calls++
		if calls == 1 {
			return "", nil
		}
		return "0", nil
	})
	if len(run.Attempts) != 2 || !run.NeedsRequalification || !slices.Contains(run.Attempts[0].Reconciliation.Missing, "script:shader") {
		t.Fatalf("%+v", run)
	}
	if plan := session.Plan(inv, nil, nil); !plan.Full || !strings.Contains(plan.Reason, "inventory-divergence") {
		t.Fatal(plan)
	}
	for _, report := range []string{"", `{"status":"passed"}`, "00", "256", "-1", "0\n"} {
		if results, err := p.Parse(context.Background(), registry.ParseRequest{Runner: "script", Report: report}); err == nil || len(results) != 0 {
			t.Fatalf("accepted %q", report)
		}
	}
	extra := testrunner.ReconcileInventory(inv, key, testrunner.PlanRunSet(inv, key, nil, nil), []testrunner.RunnerResult{{ID: "script:other", Outcome: "passed"}}, false)
	if extra.Succeeded || !extra.NeedsRequalification {
		t.Fatal(extra)
	}
}

// RSP-V0-004: aggregate commands express their exact inventory unit or FULL.
func TestRSPV0004ExactAggregateCommand(t *testing.T) {
	root, head, _ := fixture(t, basicFiles())
	p, key := plugin(t, root, head, "script:shader")
	for _, tests := range [][]string{nil, {"script:shader"}} {
		cmd, err := p.Command(context.Background(), registry.CommandRequest{Key: key, Purpose: "run", Tests: tests, RunDir: root})
		if err != nil || cmd.Superset || !slices.Equal(cmd.Argv, []string{"xcrun", "metal", "-c", "shaders/main.metal"}) {
			t.Fatalf("%+v %v", cmd, err)
		}
	}
	inv, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: key})
	if err != nil {
		t.Fatal(err)
	}
	plan := testrunner.PlanRunSet(inv, key, []testrunner.Exclusion{{ID: "script:shader", Fresh: true}}, nil)
	if len(plan.Tests) != 0 {
		t.Fatal(plan)
	}
}

// RSP-V0-004: stale/new inventory identities and mandates are never excluded.
func TestRSPV0004UnknownAndStaleSelected(t *testing.T) {
	root, head, _ := fixture(t, basicFiles())
	p, key := plugin(t, root, head, "script:shader")
	inv, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: key})
	if err != nil {
		t.Fatal(err)
	}
	for _, exclusions := range [][]testrunner.Exclusion{nil, {{ID: "script:shader", Fresh: false}}} {
		if plan := testrunner.PlanRunSet(inv, key, exclusions, nil); !slices.Equal(plan.Tests, []string{"script:shader"}) {
			t.Fatal(plan)
		}
	}
	if plan := testrunner.PlanRunSet(inv, key, []testrunner.Exclusion{{ID: "script:shader", Fresh: true}}, []string{"script:shader"}); !slices.Equal(plan.Tests, []string{"script:shader"}) {
		t.Fatal(plan)
	}
	for _, tests := range [][]string{{}, {"script:other"}, {"script:shader", "script:other"}} {
		if _, err := p.Command(context.Background(), registry.CommandRequest{Key: key, Purpose: "run", Tests: tests, RunDir: root}); err == nil {
			t.Fatal("inexact aggregate command admitted")
		}
	}
}

func header(head string) testevidence.Record {
	r := testevidence.New("main", "script:shader", head)
	d := strings.Repeat("a", 64)
	r.Runner = testevidence.RunnerIdentity{Plugin: "script", PluginVersion: "1", RunnerVersion: "fixture", ExecutableDigest: d, ConfigurationDigest: d}
	r.Execution = testevidence.ExecutionIdentity{OS: "darwin", Architecture: "arm64", TracingBackend: "fixture", TracingVersion: "1", EnvironmentDigest: d}
	r.Sources = []testevidence.Producer{{CoreDigest: d, VerifierRevision: head, ConfigurationDigest: d, Ref: "local", Run: "fixture", ContentDigest: d}}
	r.CollectionCount = 1
	return r
}
func trace() proctrace.Result {
	return proctrace.Result{Backend: "fixture", Reads: proctrace.Component[proctrace.Path]{State: proctrace.Present}, Listings: proctrace.Component[proctrace.Path]{State: proctrace.Present}, Lookups: proctrace.Component[proctrace.Path]{State: proctrace.Present}, Writes: proctrace.Component[proctrace.Write]{State: proctrace.Present}, Execs: proctrace.Component[proctrace.Exec]{State: proctrace.Present}, External: proctrace.Component[string]{State: proctrace.Present}, Outcome: proctrace.Outcome{Started: true}}
}

// RSP-V0-005: transitive include reads pin headers and retain native regions,
// without claiming line coverage or replacing independently supplied traces.
func TestRSPV0005IncludeEvidence(t *testing.T) {
	root, head, repo := fixture(t, basicFiles())
	r, err := Record(context.Background(), header(head), trace(), repo, []string{"shaders/main.metal", "shaders/other.hlsl"}, []string{"shaders"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Reads.State != testevidence.Present || len(r.Reads.Items) != 4 || r.Regions.State != testevidence.Present || len(r.Regions.Items) != 2 || r.Executed.State != testevidence.Absent || r.Executed.Reason != "shader-no-line-coverage" {
		t.Fatalf("%+v", r)
	}
	if _, err := testevidence.Encode(r); err != nil {
		t.Fatal(err)
	}
	p, key := plugin(t, root, head, "script:shader")
	native, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: key.Runner})
	if err != nil || len(native) != 1 {
		t.Fatalf("%v %v", native, err)
	}
	var executed testevidence.Component[testevidence.ExecutedFile]
	if err := json.Unmarshal(native[0].Executed, &executed); err != nil || executed.State != testevidence.Absent {
		t.Fatal("script native coverage invented")
	}
}

// RSP-V0-005: unknown include graphs and incomplete traces can never certify reads.
func TestRSPV0005IncompleteGraphAndTrace(t *testing.T) {
	for name, source := range map[string]string{
		"missing": "#include \"missing.h\"\nvoid main() {}", "macro": "#include HEADER\nvoid main() {}", "outside": "#include \"../../outside.h\"\nvoid main() {}", "conditional": "#if FEATURE\n#include \"x.h\"\n#endif\nvoid main() {}", "cycle": "#include \"main.metal\"\nvoid main() {}", "malformed": "void main() {",
	} {
		t.Run(name, func(t *testing.T) {
			_, head, repo := fixture(t, map[string]string{"shaders/main.metal": source})
			r, err := Record(context.Background(), header(head), trace(), repo, []string{"shaders/main.metal"}, nil)
			if err != nil || r.Reads.State != testevidence.Incomplete || r.Reads.Reason == "" || r.Regions.State != testevidence.Incomplete {
				t.Fatalf("%+v %v", r, err)
			}
		})
	}
	_, head, repo := fixture(t, basicFiles())
	tr := trace()
	tr.Reads = proctrace.Component[proctrace.Path]{State: proctrace.Absent, Reasons: []string{"backend-unavailable"}}
	r, err := Record(context.Background(), header(head), tr, repo, []string{"shaders/main.metal"}, nil)
	if err != nil || r.Reads.State != testevidence.Incomplete || !strings.Contains(r.Reads.Reason, "backend-unavailable") || len(r.Reads.Items) != 3 {
		t.Fatalf("%+v %v", r, err)
	}
	for _, sources := range [][]string{nil, {"../outside.metal"}, {"shaders/missing.metal"}, {"shaders/common.h"}} {
		if g := Includes(context.Background(), repo, "main", head, sources, nil); g.Reads.State != testevidence.Incomplete || g.Reads.Reason == "" {
			t.Fatal(g)
		}
	}
}

// RSP-V0-005: bounded graph traversal and immutable regular-file reads.
func TestRSPV0005GraphLimitsAndPinnedBytes(t *testing.T) {
	files := basicFiles()
	for i := 0; i < MaxDepth+2; i++ {
		files[fmt.Sprintf("deep/%d.h", i)] = fmt.Sprintf("#include \"%d.h\"\n", i+1)
	}
	files["deep/root.metal"] = "#include \"0.h\"\nvoid main() {}"
	root, head, repo := fixture(t, files)
	g := Includes(context.Background(), repo, "main", head, []string{"deep/root.metal"}, nil)
	if g.Reads.State != testevidence.Incomplete || g.Reads.Reason != "shader-depth-cap" {
		t.Fatal(g)
	}
	if g := Includes(context.Background(), repo, "main", head, make([]string, MaxFiles+1), nil); g.Reads.Reason != "shader-file-cap" {
		t.Fatal(g)
	}
	if err := os.WriteFile(filepath.Join(root, "shaders/common.h"), []byte("#include \"untracked.h\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	g = Includes(context.Background(), repo, "main", head, []string{"shaders/main.metal"}, nil)
	if g.Reads.State != testevidence.Present || len(g.Reads.Items) != 3 {
		t.Fatal(g)
	}
	if err := os.Symlink("nested.h", filepath.Join(root, "shaders/link.h")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "shaders/main.metal"), []byte("#include \"link.h\"\nvoid main() {}"), 0600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "linked include")
	head = git(t, root, "rev-parse", "HEAD")
	g = Includes(context.Background(), repo, "main", head, []string{"shaders/main.metal"}, nil)
	if g.Reads.State != testevidence.Incomplete || g.Reads.Reason != "shader-include-not-regular" {
		t.Fatal(g)
	}
}

// RSP-V0-005: the existing language-neutral selector sees the include header
// edge. Synthetic core admission below tests interoperability only; it is not
// a retained qualification record and does not enable this runner.
func TestRSPV0005HeaderEditSelectsDependants(t *testing.T) {
	_, head, repo := fixture(t, basicFiles())
	r, err := Record(context.Background(), header(head), trace(), repo, []string{"shaders/main.metal"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	key := testrunner.InventoryKey{Checkout: head, Runner: "script", ConfigDigest: r.Runner.ConfigurationDigest, OS: r.Execution.OS, Arch: r.Execution.Architecture}
	snapshot := testevidence.Snapshot{Blobs: map[testevidence.PathKey]string{}, Executables: map[testevidence.PathKey]string{}}
	for _, b := range r.Reads.Items {
		snapshot.Blobs[testevidence.PathKey{Repository: "main", Path: b.Path.Resolved}] = b.OID
	}
	pieces := testrunner.RunnerPieces{Inventory: testrunner.RunnerPiece{Present: true}, TestEvidence: testrunner.RunnerPiece{Present: true}, Qualification: testrunner.RunnerPiece{Present: true}}
	runner := runnerselection.Runner{Name: "script", Identity: r.Runner, Execution: r.Execution, Pieces: pieces,
		Inventory:     testrunner.RunnerInventory{Key: key, Complete: true, Tests: []testrunner.InventoryTest{{ID: r.Test, Path: registry.ScriptDeclarationPath}}},
		Qualification: runnerselection.Qualification{Admitted: true, Passed: true, SamplingEnabled: true, SingleCollectionQualified: true, Model: "traced", Classes: []runnerselection.Class{runnerselection.DataEdit}, Runner: r.Runner, Execution: r.Execution},
		Records:       []runnerselection.Evidence{{Record: r, Admitted: true}}, IndexBase: head, IndexVerified: true}
	input := runnerselection.Input{Base: head, Head: head, Repository: "main", Repositories: map[string]bool{"main": true}, DiffComplete: true, DirectoriesComplete: true,
		BaseDirectories: map[testevidence.PathKey]bool{{Repository: "main", Path: "."}: true, {Repository: "main", Path: "shaders"}: true}, Snapshot: snapshot,
		Changes: []runnerselection.Change{{Repository: "main", Path: "shaders/nested.h", Kind: "modify", Decoded: true}}, Runners: []runnerselection.Runner{runner}}
	plan := runnerselection.Select(input).Runners[0]
	if len(plan.Selected) != 1 || !slices.Contains(plan.Selected[0].Rules, "rule:d") || len(plan.Excluded) != 0 {
		t.Fatalf("header dependant lost: %+v", plan)
	}
}

// RSP-V0-005: plugins cannot turn a store path into a native artifact or write it.
func TestRSPV0005StoreWriteRefused(t *testing.T) {
	root, head, _ := fixture(t, basicFiles())
	p, key := plugin(t, root, head, "script:shader")
	target := filepath.Join(root, ".corvint", "forbidden-record.json")
	if records, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: key.Runner, Artifacts: []string{target}}); err == nil || len(records) != 0 {
		t.Fatal("store path admitted")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("plugin wrote store: %v", err)
	}
}
