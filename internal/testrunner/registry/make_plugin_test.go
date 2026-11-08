// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/proctrace"
	"github.com/Beamfall/corvint/internal/sessiontest"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
)

const makeDeclaration = `{"profile":"corvint-test-commands/0","go":{"env":{"GOTOOLCHAIN":"local"},"flags":["-count=1"]},"languages":{"shell":["make","shell-test"],"typescript":["make","host-adapter-test"],"ruby":{"none":"not present"}}}`

func makeFixture(t *testing.T) (string, string, *MakePlugins) {
	t.Helper()
	root := t.TempDir()
	scriptGit(t, root, "init", "-q")
	if err := os.MkdirAll(filepath.Join(root, ".github/cishards"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, sessiontest.DeclarationPath), []byte(makeDeclaration), 0600); err != nil {
		t.Fatal(err)
	}
	// This would fail if inventory interpreted or executed a Makefile.
	if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte("$(error make must not be interpreted for inventory)\n"), 0600); err != nil {
		t.Fatal(err)
	}
	scriptGit(t, root, "add", ".")
	scriptGit(t, root, "commit", "-qm", "declared make fixture")
	head := scriptGit(t, root, "rev-parse", "HEAD")
	p, err := ReadMakePlugins(context.Background(), root, head)
	if err != nil {
		t.Fatal(err)
	}
	return root, head, p
}

// RSP-V0-001 / RSP-V0-002: one deterministic script instance per declared target.
func TestRSPV0002MakeTargetInventory(t *testing.T) {
	root, head, plugins := makeFixture(t)
	if !slices.Equal(plugins.IDs(), []string{"script:make:host-adapter-test", "script:make:shell-test"}) {
		t.Fatal(plugins.IDs())
	}
	for _, id := range plugins.IDs() {
		p, key := plugins.Plugin(id, "linux", "amd64")
		if key.Runner != "script" || key.Checkout != head {
			t.Fatal(key)
		}
		a, err := p.Inventory(context.Background(), InventoryRequest{Key: key})
		if err != nil {
			t.Fatal(err)
		}
		b, err := p.Inventory(context.Background(), InventoryRequest{Key: key})
		if err != nil {
			t.Fatal(err)
		}
		ab, err := testrunner.InventoryBytes(a, key)
		if err != nil {
			t.Fatal(err)
		}
		bb, err := testrunner.InventoryBytes(b, key)
		if err != nil {
			t.Fatal(err)
		}
		if string(ab) != string(bb) || len(a.Tests) != 1 || a.Tests[0].ID != id || a.Tests[0].Path != sessiontest.DeclarationPath {
			t.Fatal(a, b)
		}
		cmd, err := p.Command(context.Background(), CommandRequest{Key: key, Purpose: "run", RunDir: root})
		if err != nil || !slices.Equal(cmd.Argv, []string{"make", strings.TrimPrefix(id, "script:make:")}) {
			t.Fatal(cmd, err)
		}
	}
	// A dirty declaration cannot replace the pinned blob or change its digest.
	if err := os.WriteFile(filepath.Join(root, sessiontest.DeclarationPath), []byte("malformed"), 0600); err != nil {
		t.Fatal(err)
	}
	again, err := ReadMakePlugins(context.Background(), root, head)
	if err != nil || !slices.Equal(again.IDs(), plugins.IDs()) {
		t.Fatal(again, err)
	}
}

// RSP-V0-002: untracked, malformed and unsupported targets are explicit absence.
func TestRSPV0002MakeTargetRefusals(t *testing.T) {
	for name, raw := range map[string]string{"bad json": "{", "duplicate": strings.Replace(makeDeclaration, `"shell":["make","shell-test"]`, `"shell":["make","shell-test"],"shell":["make","other"]`, 1), "multiple goals": strings.Replace(makeDeclaration, `["make","shell-test"]`, `["make","a","b"]`, 1), "option": strings.Replace(makeDeclaration, `"shell-test"`, `"-f"`, 1), "variable": strings.Replace(makeDeclaration, `"shell-test"`, `"X=value"`, 1), "pattern": strings.Replace(makeDeclaration, `"shell-test"`, `"*.sh"`, 1), "none": strings.ReplaceAll(makeDeclaration, `"make"`, `"tool"`)} {
		t.Run(name, func(t *testing.T) {
			scripts, err := parseScriptPlugins(strings.Repeat("a", 40), sessiontest.DeclarationPath, []byte(raw))
			if err == nil {
				_, err = makePlugins(scripts)
			}
			if err == nil {
				t.Fatal("unsupported declaration admitted")
			}
		})
	}
	root, head, _ := makeFixture(t)
	if _, err := ReadMakePlugins(context.Background(), root, "HEAD"); err == nil {
		t.Fatal("movable ref admitted")
	}
	scriptGit(t, root, "rm", sessiontest.DeclarationPath)
	scriptGit(t, root, "commit", "-qm", "remove declaration")
	missing := scriptGit(t, root, "rev-parse", "HEAD")
	if _, err := ReadMakePlugins(context.Background(), root, missing); err == nil {
		t.Fatal("missing declaration admitted")
	}
	if _, err := ReadMakePlugins(context.Background(), root, head); err != nil {
		t.Fatal("historical declaration lost", err)
	}
}

// RSP-V0-003: target exit status is reconciled with its own single inventory ID.
func TestRSPV0003MakeTerminalResults(t *testing.T) {
	root, _, plugins := makeFixture(t)
	p, key := plugins.Plugin("script:make:shell-test", "linux", "amd64")
	session := NewSelectionSession(p, key, false)
	in := session.Inventory(context.Background(), "")
	for _, code := range []string{"0", "1"} {
		run := session.Run(context.Background(), in, nil, nil, root, false, func(context.Context, CommandResponse) (string, error) { return code, nil })
		if run.Succeeded != (code == "0") || len(run.Attempts) != 1 || len(run.Attempts[0].Results) != 1 {
			t.Fatalf("%s: %+v", code, run)
		}
	}
}
func TestRSPV0003MakeMissingResult(t *testing.T) {
	root, _, plugins := makeFixture(t)
	p, key := plugins.Plugin("script:make:shell-test", "linux", "amd64")
	session := NewSelectionSession(p, key, false)
	in := session.Inventory(context.Background(), "")
	run := session.Run(context.Background(), in, nil, nil, root, false, func(context.Context, CommandResponse) (string, error) { return "", nil })
	if run.Succeeded || !run.NeedsRequalification || !slices.Contains(run.Attempts[0].Reconciliation.Missing, "script:make:shell-test") {
		t.Fatalf("%+v", run)
	}
}

// RSP-V0-004: the exact target is indivisible; unsupported subsets are refused.
func TestRSPV0004MakeWholeTarget(t *testing.T) {
	root, _, plugins := makeFixture(t)
	id := "script:make:shell-test"
	p, key := plugins.Plugin(id, "linux", "amd64")
	full, err := p.Command(context.Background(), CommandRequest{Key: key, Purpose: "run", RunDir: root})
	if err != nil {
		t.Fatal(err)
	}
	selected, err := p.Command(context.Background(), CommandRequest{Key: key, Purpose: "run", RunDir: root, Tests: []string{id}})
	if err != nil || !slices.Equal(full.Argv, selected.Argv) {
		t.Fatal(selected, err)
	}
}
func TestRSPV0004MakeSubsetRefused(t *testing.T) {
	root, _, plugins := makeFixture(t)
	p, key := plugins.Plugin("script:make:shell-test", "linux", "amd64")
	for _, tests := range [][]string{{}, {"script:make:undeclared"}, {"script:make:shell-test", "extra"}} {
		if _, err := p.Command(context.Background(), CommandRequest{Key: key, Purpose: "run", RunDir: root, Tests: tests}); err == nil {
			t.Fatal("unsupported subset accepted")
		}
	}
}

// RSP-V0-040: script-backed targets do not enable step 4 narrowing by declaration.
func TestRSPV0040MakeUnqualified(t *testing.T) {
	_, _, plugins := makeFixture(t)
	for _, id := range plugins.IDs() {
		p, key := plugins.Plugin(id, "linux", "amd64")
		if !p.Pieces().Inventory.Present || !p.Pieces().TestEvidence.Present || p.Pieces().Qualification.Reason != "unqualified" {
			t.Fatal(p.Pieces())
		}
		session := NewSelectionSession(p, key, true)
		in := session.Inventory(context.Background(), "")
		plan := session.Plan(in, []testrunner.Exclusion{{ID: id, Fresh: true}}, nil)
		if !plan.Full || !strings.Contains(plan.Reason, "unqualified") || !slices.Equal(plan.Tests, []string{id}) {
			t.Fatal(plan)
		}
	}
}
func TestRSPV0040MakeUndeclared(t *testing.T) {
	_, _, plugins := makeFixture(t)
	p, key := plugins.Plugin("script:make:invented", "linux", "amd64")
	session := NewSelectionSession(p, key, true)
	in := session.Inventory(context.Background(), "")
	if in.Complete || !session.Plan(in, nil, nil).Full {
		t.Fatal(in)
	}
}

// RSP-V0-005: make uses the existing script trace executor and file record adapter.
func TestRSPV0005MakeFileEvidence(t *testing.T) {
	root, head, plugins := makeFixture(t)
	p, key := plugins.Plugin("script:make:shell-test", "linux", "amd64")
	cmd, err := p.Command(context.Background(), CommandRequest{Key: key, Purpose: "collect", RunDir: root})
	if err != nil {
		t.Fatal(err)
	}
	trace := scriptObservation()
	trace.Reads.Entries = []proctrace.Path{scriptPath(sessiontest.DeclarationPath)}
	trace.Execs.Entries = []proctrace.Exec{{Path: scriptPath("Makefile"), Digest: strings.Repeat("c", 64)}}
	backend := &scriptBackend{result: trace}
	captured, report, err := TraceScriptCommand(context.Background(), cmd, proctrace.Request{Executable: filepath.Join(root, "pinned-make"), Args: cmd.Argv[1:], Dir: root, Repositories: []proctrace.Repository{{ID: "main", Root: root}}, EventCap: 100, PathCap: 100}, backend)
	if err != nil || report != "0" {
		t.Fatal(report, err)
	}
	results, err := p.Parse(context.Background(), ParseRequest{Runner: "script", Report: report})
	if err != nil || len(results) != 1 || results[0].ID != "script:make:shell-test" {
		t.Fatal(results, err)
	}
	repo, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		t.Fatal(err)
	}
	header := scriptHeader(head)
	header.Test = results[0].ID
	header.Runner.ConfigurationDigest = key.ConfigDigest
	record, err := ScriptTraceRecord(context.Background(), header, captured, repo)
	if err != nil || record.Executed.State != testevidence.Absent || record.Reads.State != testevidence.Present || len(record.Reads.Items) != 1 || record.Execs.State != testevidence.Present || len(record.Execs.Items) != 1 {
		t.Fatalf("%+v %v", record, err)
	}
	native, err := p.Evidence(context.Background(), EvidenceRequest{Runner: "script"})
	if err != nil || len(native) != 1 || native[0].Test != header.Test {
		t.Fatal(native, err)
	}
}

// RSP-V0-005: unsupported artifacts cannot supply fabricated make coverage.
func TestRSPV0005MakeEvidenceRefusals(t *testing.T) {
	_, _, plugins := makeFixture(t)
	p, _ := plugins.Plugin("script:make:shell-test", "linux", "amd64")
	if _, err := p.Evidence(context.Background(), EvidenceRequest{Runner: "script", Artifacts: []string{"invented coverage"}}); err == nil {
		t.Fatal("native coverage admitted")
	}
}
