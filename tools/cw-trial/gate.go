package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Beamfall/corvint/internal/gokernel"
)

const (
	gateProfile        = "corvint-cw-trial-gate/0"
	gateVerdictProfile = "corvint-cw-trial-gate-verdict/0"
	maxGateBytes       = 64 << 10
)

// gateSpec is a frozen paired-trial gate (CWT-V0-015): it names the task set
// it applies to, the two arms it compares, the per-task metrics read, and the
// thresholds. It is committed before the run it judges and never edited after;
// the verdict records its digest.
type gateSpec struct {
	Profile                   string   `json:"profile"`
	Name                      string   `json:"name"`
	Purpose                   string   `json:"purpose"`
	TasksSHA256               string   `json:"tasks_sha256"`
	Tasks                     int      `json:"tasks"`
	Access                    string   `json:"access"`
	Limit                     int      `json:"limit"`
	Treatment                 string   `json:"treatment"`
	Control                   string   `json:"control"`
	Metrics                   []string `json:"metrics"`
	Margin                    float64  `json:"margin"`
	MaxTreatmentContextFailed int      `json:"max_treatment_context_failed"`
}

type gateRule struct {
	ID        string         `json:"id"`
	Pass      bool           `json:"pass"`
	Observed  map[string]any `json:"observed"`
	Threshold string         `json:"threshold"`
}

type gateVerdict struct {
	Profile      string     `json:"profile"`
	Gate         string     `json:"gate"`
	GateSHA256   string     `json:"gate_sha256"`
	ReportSHA256 string     `json:"report_sha256"`
	TasksSHA256  string     `json:"tasks_sha256"`
	Corvint      any        `json:"corvint"`
	Treatment    string     `json:"treatment"`
	Control      string     `json:"control"`
	Rules        []gateRule `json:"rules"`
	Verdict      string     `json:"verdict"`
}

// runGate applies a frozen gate to a report: exit 0 when it admits, 1 when it
// refuses, 2 when an input cannot be read. The report's derived fields are
// rebuilt from its raw ones first, so an edited metric cannot pass the gate.
func runGate(arguments []string, stdout, stderr io.Writer) int {
	var reportPath, gatePath, output string
	flags := flag.NewFlagSet("cw-trial gate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&reportPath, "report", "", "cw-trial report to judge")
	flags.StringVar(&gatePath, "gate", "", "frozen gate file")
	flags.StringVar(&output, "output", "", "verdict path (default stdout)")
	if err := flags.Parse(arguments); err != nil || reportPath == "" || gatePath == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "cw-trial: gate takes --report FILE --gate FILE and optionally --output FILE")
		return 2
	}
	verdict, err := judgeGate(reportPath, gatePath)
	if err != nil {
		fmt.Fprintln(stderr, "cw-trial:", err)
		return 2
	}
	encoded, err := gokernel.CanonicalJSON(verdict)
	if err != nil {
		fmt.Fprintln(stderr, "cw-trial: cannot encode the verdict")
		return 2
	}
	encoded = append(encoded, '\n')
	if output == "" {
		_, err = stdout.Write(encoded)
	} else {
		err = os.WriteFile(output, encoded, 0o644)
	}
	if err != nil {
		fmt.Fprintln(stderr, "cw-trial: cannot write the verdict")
		return 2
	}
	if verdict.Verdict != "ADMIT" {
		return 1
	}
	return 0
}

func readGate(path string) (gateSpec, string, error) {
	data, err := readBounded(path, maxGateBytes)
	if err != nil {
		return gateSpec{}, "", err
	}
	var spec gateSpec
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&spec); err != nil {
		return gateSpec{}, "", fmt.Errorf("%s: %w", path, err)
	}
	switch {
	case spec.Profile != gateProfile:
		return gateSpec{}, "", fmt.Errorf("%s: profile %q is not %s", path, spec.Profile, gateProfile)
	case spec.TasksSHA256 == "" || spec.Tasks <= 0 || spec.Limit <= 0 || spec.Access == "":
		return gateSpec{}, "", fmt.Errorf("%s: tasks_sha256, tasks, limit and access are required", path)
	case spec.Treatment == "" || spec.Control == "" || spec.Treatment == spec.Control:
		return gateSpec{}, "", fmt.Errorf("%s: treatment and control must be two distinct arms", path)
	case len(spec.Metrics) == 0:
		return gateSpec{}, "", fmt.Errorf("%s: metrics is empty", path)
	case spec.Margin < 0 || spec.MaxTreatmentContextFailed < 0:
		return gateSpec{}, "", fmt.Errorf("%s: margin and max_treatment_context_failed cannot be negative", path)
	}
	for _, metric := range spec.Metrics {
		if !gateMetrics[metric] {
			return gateSpec{}, "", fmt.Errorf("%s: metric %q is not a per-task retrieval metric the gate reads", path, metric)
		}
	}
	return spec, sha256Hex(data), nil
}

// gateMetrics are the per-task 0/1 metrics a gate may compare; every one is
// derived from the record alone (CWT-V0-013), so `score` reproduces it.
var gateMetrics = map[string]bool{
	"packet_top_1": true, "packet_top_3": true, "packet_top_5": true, "packet_top_10": true,
	"gold_in_context": true, "gold_as_result": true, "success": true,
}

func judgeGate(reportPath, gatePath string) (*gateVerdict, error) {
	spec, gateDigest, err := readGate(gatePath)
	if err != nil {
		return nil, err
	}
	data, err := readBounded(reportPath, maxOutputBytes)
	if err != nil {
		return nil, err
	}
	document, err := readReport(reportPath)
	if err != nil {
		return nil, err
	}
	finalize(document)
	verdict := &gateVerdict{
		Profile: gateVerdictProfile, Gate: spec.Name, GateSHA256: gateDigest,
		ReportSHA256: sha256Hex(data), TasksSHA256: document.TasksSHA256, Corvint: document.Corvint,
		Treatment: spec.Treatment, Control: spec.Control,
	}
	verdict.Rules = append(verdict.Rules, identityRule(spec, document))
	treatmentFailed := 0
	for _, item := range document.Details {
		if record := item.Arms[spec.Treatment]; record == nil || record.ContextError != "" || record.Error != "" {
			treatmentFailed++
		}
	}
	verdict.Rules = append(verdict.Rules, gateRule{
		ID:        "treatment-context",
		Pass:      treatmentFailed <= spec.MaxTreatmentContextFailed,
		Observed:  map[string]any{"failed": treatmentFailed},
		Threshold: fmt.Sprintf("failed <= %d", spec.MaxTreatmentContextFailed),
	})
	for _, metric := range spec.Metrics {
		verdict.Rules = append(verdict.Rules, pairedRule(spec, document, metric))
	}
	verdict.Verdict = "ADMIT"
	for _, rule := range verdict.Rules {
		if !rule.Pass {
			verdict.Verdict = "REFUSE"
		}
	}
	return verdict, nil
}

// identityRule binds the report to the gate's frozen task set and run shape:
// the manifest digest, every task scored exactly once in both arms, a
// complete (not partial, not pilot) run, and an observed corvint binary.
func identityRule(spec gateSpec, document *report) gateRule {
	seen := map[string]bool{}
	missing, duplicate := 0, 0
	for _, item := range document.Details {
		if seen[item.ID] {
			duplicate++
		}
		seen[item.ID] = true
		if item.Arms[spec.Treatment] == nil || item.Arms[spec.Control] == nil {
			missing++
		}
	}
	corvintObserved := false
	if identity, ok := document.Corvint.(map[string]any); ok {
		_, corvintObserved = identity["sha256"].(string)
	}
	observed := map[string]any{
		"tasks_sha256": document.TasksSHA256, "tasks": len(document.Details), "duplicate_tasks": duplicate,
		"tasks_missing_an_arm": missing, "partial": document.Partial, "pilot": document.Pilot,
		"access": document.Access, "limit": document.Limit, "corvint_observed": corvintObserved,
	}
	pass := document.TasksSHA256 == spec.TasksSHA256 && document.Tasks == spec.Tasks &&
		len(document.Details) == spec.Tasks && duplicate == 0 && missing == 0 && !document.Partial &&
		!document.Pilot && document.Access == spec.Access && document.Limit == spec.Limit && corvintObserved
	return gateRule{
		ID: "identity", Pass: pass, Observed: observed,
		Threshold: fmt.Sprintf("tasks_sha256 %s, %d tasks each with %s and %s, access %s, limit %d, complete, non-pilot, corvint observed",
			spec.TasksSHA256, spec.Tasks, spec.Treatment, spec.Control, spec.Access, spec.Limit),
	}
}

// pairedRule compares one per-task metric between the arms over every task
// of the set: a task whose arm failed or lacks the metric counts as a miss
// for that arm, so failures stay in the denominator.
func pairedRule(spec gateSpec, document *report, metric string) gateRule {
	wins, losses, ties := 0, 0, 0
	treatmentHits, controlHits := 0.0, 0.0
	for _, item := range document.Details {
		treatment, control := armMetric(item.Arms[spec.Treatment], metric), armMetric(item.Arms[spec.Control], metric)
		treatmentHits += treatment
		controlHits += control
		switch {
		case treatment > control:
			wins++
		case treatment < control:
			losses++
		default:
			ties++
		}
	}
	tasks := float64(spec.Tasks)
	treatmentRate, controlRate := round(treatmentHits/tasks), round(controlHits/tasks)
	return gateRule{
		ID:   "paired-" + metric,
		Pass: treatmentRate-controlRate >= -spec.Margin,
		Observed: map[string]any{
			spec.Treatment: treatmentRate, spec.Control: controlRate, "difference": round(treatmentRate - controlRate),
			"treatment_wins": wins, "treatment_losses": losses, "ties": ties,
		},
		Threshold: fmt.Sprintf("%s rate - %s rate >= -%g over %d tasks", spec.Treatment, spec.Control, spec.Margin, spec.Tasks),
	}
}

func armMetric(record *armRecord, metric string) float64 {
	if record == nil || record.Error != "" || record.Metrics == nil {
		return 0
	}
	return record.Metrics[metric]
}

func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
