package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/mutate"
)

// TAT-V0-001, TAT-V0-007: execution requires the explicit range and mutation flags.
func TestProveAttackAdmission(t *testing.T) {
	base := strings.Repeat("a", 40)
	root := cliRepository(t)
	for _, args := range [][]string{
		{"prove", "--attack-tests", "pkg/calc/calc.go"},
		{"prove", "--base", base, "--attack-tests"},
		{"prove", "--mutate", "--attack-tests", "pkg/calc/calc.go"},
		{"prove", "--base", base, "--mutate", "--attack-tests", "--attack-tests"},
		{"prove", "--task", "calc", "--mutate", "--attack-tests"},
		{"prove", "--checkpoint", "checkpoint.json", "--attack-tests"},
		{"prove", "--cem", "map.json", "--attack-tests"},
	} {
		if _, _, err := parseProveInvocation(append([]string{"--root", root}, args...)); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	options, handled, err := parseProveInvocation([]string{"--root", root, "prove", "--base", base, "--mutate", "--attack-tests"})
	if err != nil || !handled || !options.proveAttackTests {
		t.Fatalf("admitted: %+v %v", options, err)
	}
	kept, count := withoutAttackTestsFlag([]string{"--", "--attack-tests"})
	if count != 0 || len(kept) != 2 {
		t.Fatalf("positional flag stripped: %v %d", kept, count)
	}
}

func attackFixture(t *testing.T, assertion string) (string, string) {
	t.Helper()
	source := "package calc\nimport \"testing\"\nfunc TestRange(t *testing.T) { " + assertion + " }\n"
	root := proveMutateRepository(t, source)
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	commitCalc(t, root, strings.Replace(proveMutateCalc, "if n > 0 {", "if n > 0 && n < 10 {", 1), "bound condition")
	return root, base
}

// TAT-V0-002, TAT-V0-003, TAT-V0-005: a kill must not hide a surviving
// changed-condition mutant; adding boundary assertions kills the survivor.
func TestProveAttackFindsWeakConditionAndStrongTestKillsIt(t *testing.T) {
	for _, test := range []struct {
		name, assertion string
		survives        bool
	}{
		{"weak", `if !IsPositive(5) { t.Fatal("inside") }`, true},
		{"strong", `if !IsPositive(5) || IsPositive(-1) || IsPositive(11) { t.Fatal("range") }`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, base := attackFixture(t, test.assertion)
			before := treeDigest(t, root)
			receipt, _, stderr, code := runProveCLI(t, root, "--base", base, "--mutate", "--attack-tests")
			if code != 0 {
				t.Fatalf("exit %d: %s", code, stderr)
			}
			row := affectedTestRow(t, receipt)
			attack := row["attack"].(map[string]any)
			if row["falsified"] != falsifiedPass || attack["status"] != "COMPLETED" || attack["selected_mutant_set_complete"] != true {
				t.Fatalf("row: %v", row)
			}
			if (attack["survived"].(float64) > 0) != test.survives || attack["killed"].(float64) == 0 {
				t.Fatalf("attack: %v", attack)
			}
			survivors := attack["survivors"].([]any)
			if len(survivors) != int(attack["survived"].(float64)) {
				t.Fatalf("survivors missing: %v", attack)
			}
			for _, value := range survivors {
				survivor := value.(map[string]any)
				span := survivor["byte_span"].(map[string]any)
				source, err := os.ReadFile(filepath.Join(root, "pkg/calc/calc.go"))
				if err != nil {
					t.Fatal(err)
				}
				start, end := int(span["start"].(float64)), int(span["end"].(float64))
				if survivor["path"] != "pkg/calc/calc.go" || survivor["mutation_operator"] != "swap-binary" || string(source[start:end]) != "&&" {
					t.Fatalf("unexpected survivor: %v", survivor)
				}
				blob := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD:pkg/calc/calc.go"))
				if survivor["blob"] != blob || attack["checkout_revision"] != strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD")) {
					t.Fatalf("unbound: %v", attack)
				}
			}
			if after := treeDigest(t, root); after != before {
				t.Fatal("attack wrote repository")
			}
		})
	}
}

// TAT-V0-004: failures and invocation exhaustion remain explicit, and the
// legacy no-mutation profile never gains attack fields.
func TestProveAttackUnavailableAndDefault(t *testing.T) {
	root, base := attackFixture(t, `t.Fatal("baseline red")`)
	receipt, _, stderr, code := runProveCLI(t, root, "--base", base, "--mutate", "--attack-tests")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	row := affectedTestRow(t, receipt)
	if row["falsified"] != falsifiedNotRun || row["attack"].(map[string]any)["status"] != "NOT_RUN" {
		t.Fatalf("baseline: %v", row)
	}
	ctx := proveBoundsContext(func(b *proveBounds) { b.mutationBudget = time.Nanosecond })
	receipt, _, stderr, code = runProveCLIContext(t, ctx, root, "--base", base, "--mutate", "--attack-tests")
	if code != 0 {
		t.Fatalf("budget exit %d: %s", code, stderr)
	}
	row = affectedTestRow(t, receipt)
	if row["falsified"] != falsifiedNotRun || row["attack"].(map[string]any)["status"] != "NOT_RUN" {
		t.Fatalf("budget: %v", row)
	}
	receipt, _, stderr, code = runProveCLI(t, root, "--base", base)
	if code != 0 {
		t.Fatalf("default exit %d: %s", code, stderr)
	}
	for _, row := range proofRows(t, receipt) {
		if _, present := row["attack"]; present {
			t.Fatalf("default attack: %v", row)
		}
	}
}

// TAT-V0-004: a partially observed set retains all counters and survivors
// while the legacy verdict remains NOT_RUN. This is report projection only.
func TestProveAttackPreservesPartialReport(t *testing.T) {
	report := mutate.Report{Verdict: mutate.BudgetExceeded, Mutants: 5, Killed: 1, Survived: 1, Uncompilable: 1, Skipped: 2,
		Survivors: []mutate.Survivor{{Operator: "swap-binary", Line: 7, Start: 40, End: 42}}, Detail: "budget exhausted after 3 of 5 mutants"}
	attack := attackReport(report, "pkg/a.go", "commit", map[string]citedBlob{"pkg/a.go": {oid: "blob", objectType: "blob"}})
	if mutationFalsified(report.Verdict) != falsifiedNotRun || attack.Status != "PARTIAL" || attack.SelectedMutantSetComplete || attack.Killed != 1 || attack.Survived != 1 || attack.Uncompilable != 1 || attack.Skipped != 2 || len(attack.Survivors) != 1 {
		t.Fatalf("partial lost: %+v", attack)
	}
	for _, verdict := range []mutate.Verdict{mutate.Unsupported, mutate.NoMutants, mutate.BudgetExceeded} {
		attack := attackReport(mutate.Report{Verdict: verdict, Detail: "unavailable"}, "a.go", "commit", nil)
		if attack.Status != "NOT_RUN" || attack.SelectedMutantSetComplete || attack.Detail != "unavailable" {
			t.Fatalf("unrun: %+v", attack)
		}
	}
}

// TAT-V0-001, TAT-V0-004: unsupported languages and dirty inputs cannot run.
func TestProveAttackPreflight(t *testing.T) {
	for _, test := range []struct {
		name, changed, testPath string
		dirty                   bool
	}{
		{"python", "a.py", "test_a.py", false}, {"dirty", "a.go", "a_test.go", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			row := proveRow{Path: test.testPath, BlobHash: "blob", reason: "same-package test for " + test.changed}
			cited := map[string]citedBlob{test.testPath: {oid: "blob", objectType: "blob"}}
			dirty := map[string]struct{}{}
			if test.dirty {
				dirty[test.changed] = struct{}{}
			}
			judged, err := judgeMutationMode(context.Background(), nil, row, "commit", cited, dirty, map[string][]mutate.LineSpan{test.changed: {{Start: 1, End: 1}}}, true)
			if err != nil || judged.falsified != falsifiedNotRun {
				t.Fatalf("preflight: %+v %v", judged, err)
			}
		})
	}
}
