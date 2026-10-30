package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/mutate"
)

const proveMutateCalc = "package calc\n\nfunc Add(a, b int) int { return a + b }\n\nfunc IsPositive(n int) bool {\n\tif n > 0 {\n\t\treturn true\n\t}\n\treturn false\n}\n"

const proveMutateKillingTest = "package calc\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(2, 3) != 5 {\n\t\tt.Fatalf(\"Add(2, 3) = %d, want 5\", Add(2, 3))\n\t}\n\tif !IsPositive(1) || IsPositive(-1) {\n\t\tt.Fatal(\"IsPositive\")\n\t}\n}\n"

const proveMutateEmptyTest = "package calc\n\nimport \"testing\"\n\nfunc TestNothing(t *testing.T) {}\n"

// proveMutateExternalEmptyTest is an external test package: the same file is
// a `test` result and a `reverse-import` result under one id.
const proveMutateExternalEmptyTest = "package calc_test\n\nimport (\n\t\"testing\"\n\n\t\"example.test/prove/pkg/calc\"\n)\n\nfunc TestNothing(t *testing.T) { _ = calc.Add }\n"

// proveMutateRepository is one Go package with a changed file and its
// same-package test, so `impact pkg/calc/calc.go` yields exactly one
// `test|test-convention` row.
func proveMutateRepository(t *testing.T, testSource string) string {
	t.Helper()
	requireNestableSandbox(t)
	root := cliRepository(t)
	files := map[string]string{
		"go.mod":                "module example.test/prove\n\ngo 1.27.0\n",
		"pkg/calc/calc.go":      proveMutateCalc,
		"pkg/calc/calc_test.go": testSource,
		".gitignore":            ".corvint/\n",
	}
	for relative, content := range files {
		file := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "mutate fixture")
	return root
}

// sandboxProbes runs a no-op under each host sandbox the mutation runner knows.
var sandboxProbes = map[string][]string{
	"sandbox-exec": {"/usr/bin/sandbox-exec", "-p", "(version 1)(allow default)", "/usr/bin/true"},
	"bwrap":        {"bwrap", "--ro-bind", "/", "/", "--unshare-net", "--", "true"},
}

// requireNestableSandbox skips when the mutation runner could not start its
// own sandbox here: a host without one, or a test already running inside a
// sandbox that denies nesting, as when the runner judges this very file.
func requireNestableSandbox(t *testing.T) {
	t.Helper()
	name, err := mutate.HostSandbox()
	if err != nil {
		t.Skipf("no sandbox on this host: %v", err)
	}
	probe := sandboxProbes[name]
	output, err := exec.Command(probe[0], probe[1:]...).CombinedOutput()
	if err != nil {
		t.Skipf("host sandbox %s cannot start inside the current sandbox: %v: %s", name, err, strings.TrimSpace(string(output)))
	}
}

func proveTestRow(t *testing.T, receipt map[string]any) map[string]any {
	t.Helper()
	for _, row := range proofRows(t, receipt) {
		if stringAt(row, "result") == "pkg/calc/calc_test.go" {
			return row
		}
	}
	t.Fatalf("no test row: %v", receipt["proof"])
	return nil
}

func TestProveImpactMutationIsOptInAndKillsWithAStrongTest(t *testing.T) {
	t.Parallel()
	root := proveMutateRepository(t, proveMutateKillingTest)
	before := treeDigest(t, root)
	receipt, _, stderr, code := runProveCLI(t, root, "pkg/calc/calc.go")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	row := proveTestRow(t, receipt)
	if row["falsifier"] != falsifierMutant || row["falsified"] != falsifiedNotRun || row["line"] != float64(1) {
		t.Fatalf("without --mutate: %v", row)
	}
	if _, present := row["detail"]; present {
		t.Fatalf("without --mutate the row must carry no detail: %v", row)
	}
	if _, present := row["witness"]; present {
		t.Fatalf("without --mutate the row must carry no witness: %v", row)
	}
	receipt, _, stderr, code = runProveCLI(t, root, "--mutate", "pkg/calc/calc.go")
	if code != 0 {
		t.Fatalf("--mutate exit %d: %s", code, stderr)
	}
	row = proveTestRow(t, receipt)
	if row["falsifier"] != falsifierMutant || row["falsified"] != falsifiedPass || !strings.Contains(stringAt(row, "detail"), "killed") {
		t.Fatalf("with --mutate: %v", row)
	}
	if receipt["state"] != "READY" || proofField(t, receipt, "failed_results") != 0 {
		t.Fatalf("state %v proof %v", receipt["state"], receipt["proof"])
	}
	if after := treeDigest(t, root); after != before {
		t.Fatal("prove --mutate changed the repository")
	}
}

// TestProveMutationWitnessReplaysKillOnSecondCheckout pins FPK-V0-028: the
// proof row identifies one exact mutant and named killing test, and those
// recorded inputs reproduce the behavioural kill in an independent checkout.
func TestProveMutationWitnessReplaysKillOnSecondCheckout(t *testing.T) {
	t.Parallel()
	root := proveMutateRepository(t, proveMutateKillingTest)
	receipt, _, stderr, code := runProveCLI(t, root, "--mutate", "pkg/calc/calc.go")
	if code != 0 {
		t.Fatalf("--mutate exit %d: %s", code, stderr)
	}
	row := proveTestRow(t, receipt)
	witness, _ := row["witness"].(map[string]any)
	mutant, _ := witness["mutant"].(map[string]any)
	span, _ := mutant["byte_span"].(map[string]any)
	if stringAt(witness, "status") != "PRODUCED" || stringAt(witness, "observed_kill") != "KILLED" {
		t.Fatalf("witness status: %v", witness)
	}
	if stringAt(mutant, "path") != "pkg/calc/calc.go" || stringAt(mutant, "blob") == "" || stringAt(mutant, "mutation_operator") == "" {
		t.Fatalf("mutant identity: %v", mutant)
	}
	if stringAt(witness, "checkout_revision") == "" || stringAt(witness, "killing_test") == "" || integerAt(span, "end") <= integerAt(span, "start") {
		t.Fatalf("replay inputs: %v", witness)
	}

	replay := filepath.Join(t.TempDir(), "replay")
	clone := exec.Command("git", "clone", "-q", "--no-hardlinks", root, replay)
	if output, err := clone.CombinedOutput(); err != nil {
		t.Fatalf("clone second checkout: %v\n%s", err, output)
	}
	revision := stringAt(witness, "checkout_revision")
	affectedGit(t, replay, "checkout", "-q", "--detach", revision)
	if blob := strings.TrimSpace(affectedGit(t, replay, "rev-parse", revision+":"+stringAt(mutant, "path"))); blob != stringAt(mutant, "blob") {
		t.Fatalf("second-checkout blob = %s, want %s", blob, mutant["blob"])
	}
	test := stringAt(witness, "killing_test")
	if output, err := replayGoTest(replay, test); err != nil {
		t.Fatalf("baseline %s failed: %v\n%s", test, err, output)
	}
	applyRecordedMutation(t, replay, mutant, span)
	output, err := replayGoTest(replay, test)
	if err == nil || !strings.Contains(string(output), "--- FAIL: "+test) || strings.Contains(string(output), "[build failed]") {
		t.Fatalf("recorded mutant did not reproduce a behavioural kill: %v\n%s", err, output)
	}
}

func TestMutationWitnessDisclosesNotProducedWhenRunnerUnavailable(t *testing.T) {
	t.Parallel()
	witness := mutationWitnessFromReport(mutate.Report{Verdict: mutate.Unsupported, Detail: "cited tests cannot be sandboxed: sandbox-exec is not available"}, "pkg/calc/calc.go", strings.Repeat("a", 40), nil)
	if witness.Status != "NOT_PRODUCED" || witness.Reason != "mutation-runner-unavailable" || witness.Mutant != nil {
		t.Fatalf("unavailable runner witness = %+v", witness)
	}
	baseline := mutationWitnessFromReport(mutate.Report{Verdict: mutate.Unsupported, Detail: "baseline tests fail"}, "pkg/calc/calc.go", strings.Repeat("a", 40), nil)
	if baseline.Reason != "no-replayable-kill-observed" {
		t.Fatalf("red baseline misclassified as unavailable runner: %+v", baseline)
	}
}

func TestMutationBudgetExhaustionDisclosesTypedWitness(t *testing.T) {
	t.Parallel()
	verdicts := exhaustedVerdicts([]proveRow{{Falsifier: falsifierMutant}})
	verdict := verdicts[0]
	if verdict.falsified != falsifiedNotRun || verdict.witness == nil || verdict.witness.Status != "NOT_PRODUCED" || verdict.witness.Reason != "mutation-budget-exhausted" {
		t.Fatalf("budget verdict = %+v", verdict)
	}
}

func TestMutationInputUnavailableDisclosesTypedWitness(t *testing.T) {
	t.Parallel()
	row := proveRow{Falsifier: falsifierMutant, Path: "pkg/calc/calc_test.go", reason: "same-package test for pkg/calc/calc.go"}
	verdict, err := judgeMutation(context.Background(), nil, row, "", nil, map[string]struct{}{"pkg/calc/calc.go": {}}, nil)
	if err != nil || verdict.falsified != falsifiedNotRun || verdict.witness == nil || verdict.witness.Status != "NOT_PRODUCED" || verdict.witness.Reason != "mutation-input-unavailable" {
		t.Fatalf("input verdict = %+v, err = %v", verdict, err)
	}
}

func TestNoMutantInRangeDisclosesTypedWitness(t *testing.T) {
	t.Parallel()
	row := proveRow{Falsifier: falsifierMutant, Path: "pkg/calc/calc_test.go", BlobHash: "test", reason: "same-package test for pkg/calc/calc.go"}
	cited := map[string]citedBlob{"pkg/calc/calc_test.go": {oid: "test", objectType: "blob"}}
	verdict, err := judgeMutation(context.Background(), nil, row, "", cited, nil, map[string][]mutate.LineSpan{})
	if err != nil || verdict.falsified != falsifiedNotRun || verdict.witness == nil || verdict.witness.Status != "NOT_PRODUCED" || verdict.witness.Reason != "no-mutant-in-range" {
		t.Fatalf("range verdict = %+v, err = %v", verdict, err)
	}
}

func replayGoTest(root, name string) ([]byte, error) {
	command := exec.Command("go", "test", "-count=1", "-run", "^"+name+"$", "./pkg/calc")
	command.Dir = root
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOENV=off", "GOPROXY=off", "GOSUMDB=off")
	return command.CombinedOutput()
}

func applyRecordedMutation(t *testing.T, root string, mutant, span map[string]any) {
	t.Helper()
	file := filepath.Join(root, filepath.FromSlash(stringAt(mutant, "path")))
	source, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	start, end := integerAt(span, "start"), integerAt(span, "end")
	if start < 0 || end <= start || end > len(source) {
		t.Fatalf("byte span %d:%d is outside %d-byte blob", start, end, len(source))
	}
	original := string(source[start:end])
	replacement := recordedReplacement(t, stringAt(mutant, "mutation_operator"), original)
	changed := append(append(append([]byte{}, source[:start]...), replacement...), source[end:]...)
	if err := os.WriteFile(file, changed, 0o644); err != nil {
		t.Fatal(err)
	}
}

func recordedReplacement(t *testing.T, operator, original string) string {
	t.Helper()
	switch operator {
	case "negate-condition":
		return "!(" + original + ")"
	case "swap-binary":
		swaps := map[string]string{"==": "!=", "!=": "==", "<": ">=", ">=": "<", ">": "<=", "<=": ">", "+": "-", "-": "+", "&&": "||", "||": "&&"}
		if replacement, ok := swaps[original]; ok {
			return replacement
		}
	case "replace-literal":
		if original == "0" {
			return "1"
		}
		if original == `""` {
			return `"mutant"`
		}
		if strings.HasPrefix(original, `"`) {
			return `""`
		}
		return "0"
	case "delete-statement":
		return ""
	}
	t.Fatalf("cannot apply %s to %q", operator, original)
	return ""
}

func TestProveImpactMutationFailsAnAssertionFreeTest(t *testing.T) {
	t.Parallel()
	root := proveMutateRepository(t, proveMutateEmptyTest)
	receipt, _, stderr, code := runProveCLI(t, root, "--mutate", "pkg/calc/calc.go")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	row := proveTestRow(t, receipt)
	if row["falsified"] != falsifiedFail || !strings.Contains(stringAt(row, "detail"), "survived") {
		t.Fatalf("row: %v", row)
	}
	if proofField(t, receipt, "failed_results") != 1 {
		t.Fatalf("proof: %v", receipt["proof"])
	}
}

func TestProveResultsSharingAnIDAnswerForTheirOwnRows(t *testing.T) {
	t.Parallel()
	root := proveMutateRepository(t, proveMutateExternalEmptyTest)
	receipt, _, stderr, code := runProveCLI(t, root, "--mutate", "--limit", "20", "pkg/calc/calc.go")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	verdicts := map[string]string{}
	for _, row := range proofRows(t, receipt) {
		if stringAt(row, "result") == "pkg/calc/calc_test.go" {
			verdicts[stringAt(row, "kind")] = stringAt(row, "falsified")
		}
	}
	if verdicts["test"] != falsifiedFail || verdicts["reverse-import"] != falsifiedPass {
		t.Fatalf("rows by kind: %v", verdicts)
	}
	if proofField(t, receipt, "failed_results") != 1 {
		t.Fatalf("one failing row must fail one result: %v", receipt["proof"])
	}
}

func TestProveMutateFlagIsImpactOnly(t *testing.T) {
	t.Parallel()
	root := proveMutateRepository(t, proveMutateKillingTest)
	for _, test := range []struct {
		arguments []string
		code      string
	}{
		{[]string{"--mutate", "--task", "calc"}, "invalid-arguments"},
		{[]string{"--mutate", "--mutate", "pkg/calc/calc.go"}, "invalid-arguments"},
		// After `--` the token is a path, never authorization to run tests.
		{[]string{"pkg/calc/calc.go", "--", "--mutate"}, "unsupported-impact-path-suffix"},
	} {
		_, stdout, stderr, code := runProveCLI(t, root, test.arguments...)
		if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, test.code) {
			t.Fatalf("%v: exit=%d stdout=%q stderr=%q", test.arguments, code, stdout, stderr)
		}
	}
	if kept, count := withoutMutateFlag([]string{"--mutate", "a.go", "--", "--mutate"}); count != 1 || strings.Join(kept, " ") != "a.go -- --mutate" {
		t.Fatalf("kept %v count %d", kept, count)
	}
}

func TestJudgeMutationFailsAMisplacedTestBlobWithoutRunning(t *testing.T) {
	t.Parallel()
	row := proveRow{Falsifier: falsifierMutant, Path: "pkg/calc/calc_test.go", BlobHash: "0000000000000000000000000000000000000000", reason: "same-package test for pkg/calc/calc.go"}
	cited := map[string]citedBlob{"pkg/calc/calc_test.go": {oid: "1111111111111111111111111111111111111111", objectType: "blob"}}
	judged, err := judgeMutation(context.Background(), nil, row, "", cited, map[string]struct{}{}, nil)
	if err != nil || judged.falsified != falsifiedFail || !strings.Contains(judged.detail, "cited test blob") {
		t.Fatalf("judged %+v err %v", judged, err)
	}
	if judged, _ := judgeMutation(context.Background(), nil, row, "", cited, map[string]struct{}{"pkg/calc/calc.go": {}}, nil); judged.falsified != falsifiedNotRun {
		t.Fatalf("dirty changed path must be NOT_RUN: %+v", judged)
	}
}

func TestMutationVerdictVocabulary(t *testing.T) {
	t.Parallel()
	for verdict, want := range map[mutate.Verdict]string{
		mutate.Killed: falsifiedPass, mutate.Survived: falsifiedFail,
		mutate.NoMutants: falsifiedNotRun, mutate.BudgetExceeded: falsifiedNotRun, mutate.Unsupported: falsifiedNotRun,
	} {
		if got := mutationFalsified(verdict); got != want {
			t.Errorf("%s: got %s want %s", verdict, got, want)
		}
	}
	for path, want := range map[string]string{"pkg/calc/calc_test.go": falsifierMutant, "pkg/calc/test_calc.py": falsifierMutant, "pkg/calc/calc.go": falsifierNone} {
		if got := falsifierFor(proveRow{Kind: "test", Authority: "test-convention", Path: path}); got != want {
			t.Errorf("%s: got %s want %s", path, got, want)
		}
	}
}
