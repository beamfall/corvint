package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/mutate"
	"github.com/Beamfall/corvint/internal/liveverify/rbmutate"
	"github.com/Beamfall/corvint/internal/liveverify/rbmutate/minirspec"
)

// TestFPKV0074NamedSourceSelectorParses pins the selector grammar and its
// refusals (FPK-V0-074).
func TestFPKV0074NamedSourceSelectorParses(t *testing.T) {
	t.Parallel()
	root := cliRepository(t)
	parsed, handled, err := parseProveInvocation([]string{"--root", root, "prove", "--mutate",
		"--mutate-source=app/models/scoring.rb:3-5,14", "--mutate-example", "spec/models/scoring_spec.rb:5:2",
		"--mutate-example", "spec/models/scoring_spec.rb:2", "--mutate-example=spec/other_spec.rb:7"})
	if !handled || err != nil {
		t.Fatalf("handled %v err %v", handled, err)
	}
	named := parsed.prove.named
	if named == nil || named.source != "app/models/scoring.rb" || named.lineBasis() != rbmutate.LineBasisCallerSpans {
		t.Fatalf("named: %+v", named)
	}
	if !reflect.DeepEqual(named.spans, []mutate.LineSpan{{Start: 3, End: 5}, {Start: 14, End: 14}}) {
		t.Fatalf("spans: %v", named.spans)
	}
	if !reflect.DeepEqual(named.examples, map[string][]int{"spec/models/scoring_spec.rb": {2, 5}, "spec/other_spec.rb": {7}}) {
		t.Fatalf("examples: %v", named.examples)
	}
	if parsed.proveMode != "impact" || !reflect.DeepEqual(parsed.impactPaths, []string{"app/models/scoring.rb"}) {
		t.Fatalf("the named source must join the impact paths: %s %v", parsed.proveMode, parsed.impactPaths)
	}
	rows := namedSourceRows(named)
	if len(rows) != 2 || rows[0].Path != "spec/models/scoring_spec.rb" || rows[0].Result != "spec/models/scoring_spec.rb:2:5" || rows[0].Line != 2 {
		t.Fatalf("rows: %+v", rows)
	}
	if rows[0].Kind != kindNamedSource || rows[0].Falsifier != falsifierMutant || rows[0].Trust == "" || rows[0].Refusal != "" {
		t.Fatalf("row claim: %+v", rows[0])
	}
	if changed, ok := testClaimTarget(rows[1].reason); !ok || changed != "app/models/scoring.rb" {
		t.Fatalf("claim target %q %v", changed, ok)
	}
	base := strings.Repeat("a", 40)
	for _, args := range [][]string{
		{"prove", "--mutate-source", "lib/a.rb", "--mutate-example", "spec/a_spec.rb:1", "lib/a.rb"},
		{"prove", "--mutate", "--mutate-example", "spec/a_spec.rb:1", "lib/a.rb"},
		{"prove", "--mutate", "--mutate-source", "lib/a.rb", "lib/a.rb"},
		{"prove", "--mutate", "--mutate-source", "lib/a.rb", "--mutate-source", "lib/b.rb", "--mutate-example", "spec/a_spec.rb:1"},
		{"prove", "--mutate", "--mutate-source", "lib/a.py", "--mutate-example", "spec/a_spec.rb:1"},
		{"prove", "--mutate", "--mutate-source", "spec/a_spec.rb", "--mutate-example", "spec/a_spec.rb:1"},
		{"prove", "--mutate", "--mutate-source", "../a.rb", "--mutate-example", "spec/a_spec.rb:1"},
		{"prove", "--mutate", "--mutate-source", "lib/a.rb:5-3", "--mutate-example", "spec/a_spec.rb:1"},
		{"prove", "--mutate", "--mutate-source", "lib/a.rb:0", "--mutate-example", "spec/a_spec.rb:1"},
		{"prove", "--mutate", "--mutate-source", "lib/a.rb", "--mutate-example", "spec/a_spec.rb"},
		{"prove", "--mutate", "--mutate-source", "lib/a.rb", "--mutate-example", "spec/a_spec.rb:0"},
		{"prove", "--mutate", "--mutate-source", "lib/a.rb", "--mutate-example", "test/a_test.rb:3"},
		{"prove", "--mutate", "--mutate-source", "lib/a.rb", "--mutate-example"},
		{"prove", "--base", base, "--mutate", "--attack-tests", "--mutate-source", "lib/a.rb", "--mutate-example", "spec/a_spec.rb:1"},
		{"prove", "--task", "calc", "--mutate", "--mutate-source", "lib/a.rb", "--mutate-example", "spec/a_spec.rb:1"},
		{"prove", "--mutate", "--mutate-source", "lib/a.rb", "--mutate-example", "spec/a_spec.rb:1", "--mutate-example", "spec/b_spec.rb:1",
			"--mutate-example", "spec/c_spec.rb:1", "--mutate-example", "spec/d_spec.rb:1", "--mutate-example", "spec/e_spec.rb:1"},
	} {
		if _, _, err := parseProveInvocation(append([]string{"--root", root}, args...)); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
	changeMode, _, err := parseProveInvocation([]string{"--root", root, "prove", "--base", base, "--mutate", "--mutate-source", "lib/a.rb", "--mutate-example", "spec/a_spec.rb:4"})
	if err != nil || changeMode.proveMode != "change" || changeMode.prove.named == nil || len(changeMode.impactPaths) != 0 || changeMode.prove.named.lineBasis() != rbmutate.LineBasisWholeFile {
		t.Fatalf("change mode: %+v %v", changeMode, err)
	}
}

// TestFPKV0075NamedUnchangedSourceGetsKillsMutantRows pins FPK-V0-074 to
// FPK-V0-076 end to end: change mode on a range that leaves the Ruby source
// untouched, plus a named source and three cited examples, yields one
// test-kills-mutant row whose every mutant has a verdict and a receipt, whose
// record names the RSpec selector and the whole-file basis, under the
// 12-mutant cap, and which writes nothing to the repository.
func TestFPKV0075NamedUnchangedSourceGetsKillsMutantRows(t *testing.T) {
	t.Parallel()
	requireNestableSandbox(t)
	ruby, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby is not available")
	}
	shims := t.TempDir()
	if _, err := minirspec.Install(shims); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for name, body := range map[string]string{"app/models/scoring.rb": proveScoringBase, "spec/models/scoring_spec.rb": proveScoringWeakSpec, "README.md": "scoring\n"} {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"}, {"add", "."}, {"commit", "-qm", "base"},
	} {
		affectedGit(t, root, arguments...)
	}
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("scoring bands\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "commit", "-qam", "docs only")
	before := treeDigest(t, root)
	environment := []string{"PATH=" + shims + string(os.PathListSeparator) + filepath.Dir(ruby) + string(os.PathListSeparator) + "/usr/bin:/bin"}
	receipt, _, stderr, code := runProveCLIEnvironment(t, environment, root, "--base", base, "--mutate",
		"--mutate-source", "app/models/scoring.rb", "--mutate-example", "spec/models/scoring_spec.rb:4:7", "--mutate-example", "spec/models/scoring_spec.rb:10")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	var row map[string]any
	for _, candidate := range proofRows(t, receipt) {
		if stringAt(candidate, "kind") == kindNamedSource {
			if row != nil {
				t.Fatalf("one spec file must give one row: %v", receipt["proof"])
			}
			row = candidate
		}
	}
	if row == nil {
		t.Fatalf("no named-source row: %v", receipt["proof"])
	}
	if row["falsifier"] != falsifierMutant || row["path"] != "spec/models/scoring_spec.rb" || row["result"] != "spec/models/scoring_spec.rb:4:7:10" || stringAt(row, "blob_hash") == "" {
		t.Fatalf("row: %v", row)
	}
	witness, _ := row["witness"].(map[string]any)
	if witness["status"] != "NOT_PRODUCED" || witness["reason"] != rubyWitnessNotProduced {
		t.Fatalf("witness: %v row %v", witness, row)
	}
	mutation, _ := row["mutation"].(map[string]any)
	baseline, _ := mutation["baseline"].(map[string]any)
	mutants, _ := mutation["mutants"].([]any)
	if mutation["target"] != "spec/models/scoring_spec.rb:4:7:10" || mutation["line_basis"] != rbmutate.LineBasisWholeFile || mutation["stage"] != "mutants" {
		t.Fatalf("mutation record: %v", mutation)
	}
	if mutation["planned"] != float64(15) || mutation["selected"] != float64(12) || len(mutants) != 12 || baseline["examples"] != float64(3) {
		t.Fatalf("whole-file plan under the cap: %v", mutation)
	}
	verdicts := map[string]int{}
	for _, entry := range mutants {
		mutant, _ := entry.(map[string]any)
		receiptOf, _ := mutant["receipt"].(map[string]any)
		verdicts[stringAt(mutant, "outcome")]++
		if receiptOf["examples"] != float64(3) || stringAt(receiptOf, "source_sha256") == "" {
			t.Fatalf("mutant receipt: %v", mutant)
		}
	}
	if verdicts["killed"] == 0 || verdicts["survived"] == 0 || verdicts["killed"]+verdicts["survived"]+verdicts["unloadable"]+verdicts["timed-out"] != 12 {
		t.Fatalf("verdicts: %v", verdicts)
	}
	if row["falsified"] != falsifiedPass || !strings.Contains(stringAt(row, "detail"), "survived spec/models/scoring_spec.rb:4:7:10") {
		t.Fatalf("row verdict: %v %q", row["falsified"], stringAt(row, "detail"))
	}
	// Impact mode with caller spans: the source joins the impact paths, and
	// the record names the caller-spans basis and stays inside lines 3 to 6.
	receipt, _, stderr, code = runProveCLIEnvironment(t, environment, root, "--mutate",
		"--mutate-source", "app/models/scoring.rb:3-6", "--mutate-example", "spec/models/scoring_spec.rb:4",
		"--mutate-example", "spec/models/absent_spec.rb:1")
	if code != 0 {
		t.Fatalf("impact exit %d: %s", code, stderr)
	}
	row = nil
	for _, candidate := range proofRows(t, receipt) {
		switch {
		case stringAt(candidate, "kind") != kindNamedSource:
		case candidate["path"] == "spec/models/absent_spec.rb":
			witness, _ := candidate["witness"].(map[string]any)
			if candidate["falsified"] != falsifiedFail || witness["reason"] != "mutation-input-unavailable" {
				t.Fatalf("absent spec file row: %v", candidate)
			}
		default:
			row = candidate
		}
	}
	mutation, _ = row["mutation"].(map[string]any)
	mutants, _ = mutation["mutants"].([]any)
	if mutation["line_basis"] != rbmutate.LineBasisCallerSpans || mutation["target"] != "spec/models/scoring_spec.rb:4" || len(mutants) == 0 {
		t.Fatalf("impact caller-spans record: %v", row)
	}
	for _, entry := range mutants {
		mutant, _ := entry.(map[string]any)
		if line, _ := mutant["line"].(float64); line < 3 || line > 6 {
			t.Fatalf("mutant outside the caller spans: %v", mutant)
		}
	}
	if after := treeDigest(t, root); after != before {
		t.Fatal("prove --mutate wrote the repository")
	}
}
