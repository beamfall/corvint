package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/gokernel"
)

const gateTestTasksSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// gateReport builds a complete two-arm report over n tasks whose gold is
// `gold/N.go`: the corvint packet lists gold on the first treatmentHits tasks,
// the grep listing on the first controlHits tasks.
func gateReport(n, treatmentHits, controlHits int) *report {
	document := &report{
		Profile: profile, Partition: "unseen", TasksSHA256: gateTestTasksSHA, Tasks: n, Limit: 20,
		Access: "none", Corvint: map[string]any{"sha256": "bb", "version": "test"},
	}
	for index := 0; index < n; index++ {
		gold := fmt.Sprintf("gold/%d.go", index)
		packetRow, grepRow := "other/a.go", "other/b.go"
		if index < treatmentHits {
			packetRow = gold
		}
		if index < controlHits {
			grepRow = gold
		}
		document.Details = append(document.Details, taskRecord{
			ID: fmt.Sprintf("t%d", index), Kind: "retrieval", Gold: map[string][]string{"source-file": {gold}},
			Arms: map[string]*armRecord{
				"corvint": {Context: fmt.Sprintf(`{"results":[{"id":%q}]}`, packetRow), Reply: "```json\n{\"claims\":[]}\n```"},
				"grep":    {Context: "score=2 " + grepRow + "\n", Reply: "```json\n{\"claims\":[]}\n```"},
			},
		})
	}
	finalize(document)
	return document
}

func writeGateInputs(t *testing.T, document *report, mutate func(map[string]any)) (string, string) {
	t.Helper()
	dir := t.TempDir()
	encoded, err := gokernel.CanonicalJSON(document)
	if err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(dir, "report.json")
	if err := os.WriteFile(reportPath, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	spec := map[string]any{
		"profile": gateProfile, "name": "test-gate", "purpose": "test", "tasks_sha256": gateTestTasksSHA,
		"tasks": len(document.Details), "access": "none", "limit": 20, "treatment": "corvint", "control": "grep",
		"metrics": []string{"packet_top_10", "gold_as_result"}, "margin": 0, "max_treatment_context_failed": 1,
	}
	if mutate != nil {
		mutate(spec)
	}
	gateData, _ := json.Marshal(spec)
	gatePath := filepath.Join(dir, "gate.json")
	if err := os.WriteFile(gatePath, gateData, 0o644); err != nil {
		t.Fatal(err)
	}
	return reportPath, gatePath
}

func gateOutcome(t *testing.T, reportPath, gatePath string) (int, gateVerdict) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runGate([]string{"--report", reportPath, "--gate", gatePath}, &stdout, &stderr)
	var verdict gateVerdict
	if code != 2 {
		if err := json.Unmarshal(stdout.Bytes(), &verdict); err != nil {
			t.Fatalf("verdict: %v (%s)", err, stderr.String())
		}
	}
	return code, verdict
}

func failedRules(verdict gateVerdict) []string {
	var failed []string
	for _, rule := range verdict.Rules {
		if !rule.Pass {
			failed = append(failed, rule.ID)
		}
	}
	return failed
}

// TestGateAdmitsANonInferiorTreatmentAndRefusesAWorseOne covers CWT-V0-015:
// the paired rule over every task admits a treatment at least as good as the
// control on each frozen metric and refuses one that loses on any.
func TestGateAdmitsANonInferiorTreatmentAndRefusesAWorseOne(t *testing.T) {
	reportPath, gatePath := writeGateInputs(t, gateReport(10, 6, 6), nil)
	code, verdict := gateOutcome(t, reportPath, gatePath)
	if code != 0 || verdict.Verdict != "ADMIT" || len(failedRules(verdict)) != 0 {
		t.Fatalf("tie: code %d verdict %s failed %v", code, verdict.Verdict, failedRules(verdict))
	}
	if verdict.GateSHA256 == "" || verdict.ReportSHA256 == "" || verdict.TasksSHA256 != gateTestTasksSHA {
		t.Fatalf("verdict does not bind its inputs: %+v", verdict)
	}
	reportPath, gatePath = writeGateInputs(t, gateReport(10, 5, 6), nil)
	code, verdict = gateOutcome(t, reportPath, gatePath)
	if code != 1 || verdict.Verdict != "REFUSE" {
		t.Fatalf("worse treatment: code %d verdict %s", code, verdict.Verdict)
	}
	if got := strings.Join(failedRules(verdict), ","); got != "paired-packet_top_10,paired-gold_as_result" {
		t.Fatalf("failed rules = %s", got)
	}
	// A margin admits a bounded loss, and only that.
	reportPath, gatePath = writeGateInputs(t, gateReport(10, 5, 6), func(spec map[string]any) { spec["margin"] = 0.1 })
	if code, verdict = gateOutcome(t, reportPath, gatePath); code != 0 {
		t.Fatalf("within margin: code %d failed %v", code, failedRules(verdict))
	}
}

// TestGateRefusesAReportItWasNotFrozenFor covers CWT-V0-015's identity rule:
// another task set, a partial or truncated run, an unobserved binary, or a
// different access/limit refuses whatever the metrics say.
func TestGateRefusesAReportItWasNotFrozenFor(t *testing.T) {
	cases := map[string]struct {
		document func() *report
		mutate   func(map[string]any)
	}{
		"tasks digest": {func() *report { return gateReport(4, 4, 0) }, func(spec map[string]any) { spec["tasks_sha256"] = strings.Repeat("c", 64) }},
		"partial":      {func() *report { d := gateReport(4, 4, 0); d.Partial = true; return d }, nil},
		"missing task": {func() *report { d := gateReport(4, 4, 0); d.Details = d.Details[:3]; return d }, nil},
		"missing arm": {func() *report {
			d := gateReport(4, 4, 0)
			delete(d.Details[0].Arms, "grep")
			return d
		}, nil},
		"corvint unobserved": {func() *report { d := gateReport(4, 4, 0); d.Corvint = notObserved; return d }, nil},
		"access":             {func() *report { d := gateReport(4, 4, 0); d.Access = "read-only"; return d }, nil},
		"limit":              {func() *report { d := gateReport(4, 4, 0); d.Limit = 10; return d }, nil},
	}
	for name, test := range cases {
		document := test.document()
		mutate := test.mutate
		if name == "missing task" {
			mutate = func(spec map[string]any) { spec["tasks"] = 4 }
		}
		reportPath, gatePath := writeGateInputs(t, document, mutate)
		code, verdict := gateOutcome(t, reportPath, gatePath)
		if code != 1 || failedRules(verdict)[0] != "identity" {
			t.Errorf("%s: code %d failed %v", name, code, failedRules(verdict))
		}
	}
}

// TestGateCountsTreatmentFailuresAsMissesAndBoundsThem covers CWT-V0-015: a
// failed corvint context stays in the denominator as a miss, and more
// failures than the frozen bound refuse.
func TestGateCountsTreatmentFailuresAsMissesAndBoundsThem(t *testing.T) {
	document := gateReport(4, 4, 3)
	for index := 0; index < 2; index++ {
		document.Details[index].Arms["corvint"].ContextError = "corvint context: exit status 1"
		document.Details[index].Arms["corvint"].Context = ""
	}
	finalize(document)
	reportPath, gatePath := writeGateInputs(t, document, nil)
	code, verdict := gateOutcome(t, reportPath, gatePath)
	if code != 1 {
		t.Fatalf("code %d", code)
	}
	if got := strings.Join(failedRules(verdict), ","); got != "treatment-context,paired-packet_top_10,paired-gold_as_result" {
		t.Fatalf("failed rules = %s", got)
	}
}

// TestGateRebuildsMetricsAndRefusesUnreadableGates covers CWT-V0-015: a
// report whose recorded metric was edited is judged on the metric its raw
// context yields, and a gate with an unknown field or metric is an input
// error, not a verdict.
func TestGateRebuildsMetricsAndRefusesUnreadableGates(t *testing.T) {
	document := gateReport(4, 2, 3)
	for index := range document.Details {
		document.Details[index].Arms["corvint"].Metrics["packet_top_10"] = 1
		document.Details[index].Arms["corvint"].Metrics["gold_as_result"] = 1
	}
	reportPath, gatePath := writeGateInputs(t, document, nil)
	if code, _ := gateOutcome(t, reportPath, gatePath); code != 1 {
		t.Fatalf("edited metrics were trusted: code %d", code)
	}
	for name, mutate := range map[string]func(map[string]any){
		"unknown field":  func(spec map[string]any) { spec["verdict"] = "ADMIT" },
		"unknown metric": func(spec map[string]any) { spec["metrics"] = []string{"mean_f1"} },
		"same arms":      func(spec map[string]any) { spec["control"] = "corvint" },
		"profile":        func(spec map[string]any) { spec["profile"] = "x" },
	} {
		reportPath, gatePath := writeGateInputs(t, gateReport(2, 1, 1), mutate)
		if code := runGate([]string{"--report", reportPath, "--gate", gatePath}, io.Discard, io.Discard); code != 2 {
			t.Errorf("%s: code %d, want 2", name, code)
		}
	}
}

// TestFrozenBeamfallOrientationGateNamesTheFrozenSet covers CWT-V0-016: the
// committed gate parses and binds the committed task set's digest.
func TestFrozenBeamfallOrientationGateNamesTheFrozenSet(t *testing.T) {
	dir := filepath.Join("testdata", "beamfall-roadmap-v1")
	spec, _, err := readGate(filepath.Join(dir, "orientation-gate.json"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var tasks manifest
	if err := json.Unmarshal(data, &tasks); err != nil {
		t.Fatal(err)
	}
	if spec.TasksSHA256 != sha256Hex(data) || spec.Tasks != len(tasks.Tasks) {
		t.Fatalf("gate binds %s/%d, set is %s/%d", spec.TasksSHA256, spec.Tasks, sha256Hex(data), len(tasks.Tasks))
	}
}

// TestRelativeAgentCommandResolvesAgainstTheLaunchDirectory covers CWT-V0-016:
// the documented `--agent-command tools/cw-trial/testdata/abstain-agent.sh`
// must name the same file when the agent runs from an empty directory.
func TestRelativeAgentCommandResolvesAgainstTheLaunchDirectory(t *testing.T) {
	tasks := filepath.Join("testdata", "beamfall-roadmap-v1", "tasks.json")
	configuration, err := parseRunOptions([]string{"--tasks", tasks, "--arms", "grep", "--access", "none",
		"--agent", "script", "--agent-command", filepath.Join("testdata", "abstain-agent.sh")})
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.Abs(filepath.Join("testdata", "abstain-agent.sh"))
	if configuration.agentCommand != want {
		t.Fatalf("agent command %q, want %q", configuration.agentCommand, want)
	}
	if runtime.GOOS == "windows" {
		t.Skip("the abstain agent is a POSIX shell script")
	}
	result, err := scriptAgent{command: configuration.agentCommand}.run(context.Background(), t.TempDir(), "prompt", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if claims, state := extractClaims(result.reply); len(claims) != 0 || state != "PRESENT" {
		t.Fatalf("abstain agent replied %q (%s)", result.reply, state)
	}
}

// TestCommittedOrientationVerdictReproducesAndAdmits covers CWT-V0-017: the
// skill and DOGFOOD route task-start orientation to `context --task` on the
// strength of the committed verdict, so that verdict must be exactly what the
// frozen gate yields over the committed run report, and it must admit.
func TestCommittedOrientationVerdictReproducesAndAdmits(t *testing.T) {
	results := filepath.Join("..", "..", "benchmarks", "results")
	recorded, err := os.ReadFile(filepath.Join(results, "orientation-gate-beamfall-roadmap-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	code := runGate([]string{
		"--report", filepath.Join(results, "cw-trial-beamfall-roadmap-v1-orientation-run-1.json"),
		"--gate", filepath.Join("testdata", "beamfall-roadmap-v1", "orientation-gate.json"),
	}, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("gate exit %d, routing requires ADMIT", code)
	}
	if !bytes.Equal(recorded, stdout.Bytes()) {
		t.Fatalf("committed verdict does not reproduce:\n%s", stdout.String())
	}
}
