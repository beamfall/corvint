package main

import (
	"encoding/json"
	"errors"
	"fmt"
)

// controlAlreadyFixed marks a task whose issue is already resolved at its
// pinned revision (CEP-V0-004, arXiv 2603.25764): nothing is left to change,
// so the right reply is an abstention and the right packet abstains too.
const controlAlreadyFixed = "already-fixed"

// nothingToFix judges every claim kind with no true value, so any valid claim
// on an already-fixed control is FALSE, never UNJUDGED.
var nothingToFix = map[string][]string{"test-file": {}, "source-file": {}, "answer": {}}

// validateControl admits only the already-fixed control, and only without
// gold: a control that names a right answer is not a control.
func validateControl(item task) error {
	if item.Control == "" {
		return nil
	}
	if item.Control != controlAlreadyFixed {
		return fmt.Errorf("control must be %s, not %q", controlAlreadyFixed, item.Control)
	}
	if len(item.Gold) > 0 {
		return errors.New("an already-fixed control carries no gold")
	}
	return nil
}

// scoringGold is the gold the claims are judged against.
func scoringGold(item *taskRecord) map[string][]string {
	if item.Control == controlAlreadyFixed {
		return nothingToFix
	}
	return item.Gold
}

// controlMetrics adds the already-fixed outcome (CEP-V0-005): control_failed
// is the agent's own failure, a certain claim, so every arm is compared on it.
// packet_abstained is recorded apart, and only for a produced corvint packet.
func controlMetrics(metrics map[string]float64, record *armRecord) {
	metrics["control_failed"] = metrics["confidently_wrong_task"]
	if record.ContextState != "" && record.ContextError == "" {
		metrics["packet_abstained"] = boolFloat(packetAbstained(record.Context))
	}
}

// packetAbstained reads a task-context packet as tools/retrieval-bench does:
// NO_CANDIDATES or OUT_OF_SCOPE, no result row, or an unsupported-conjunction
// answerability verdict (TCP-V0-016) is an abstention. A packet that does not
// parse (cut at the context bound) did not abstain.
func packetAbstained(contextText string) bool {
	var packet struct {
		State    string            `json:"state"`
		Results  []json.RawMessage `json:"results"`
		Coverage struct {
			Answerability struct {
				Verdict string `json:"verdict"`
			} `json:"answerability"`
		} `json:"coverage"`
	}
	if err := json.Unmarshal([]byte(contextText), &packet); err != nil {
		return false
	}
	withheld := packet.Coverage.Answerability.Verdict == "unsupported-conjunction"
	return packet.State == "NO_CANDIDATES" || packet.State == "OUT_OF_SCOPE" || withheld || len(packet.Results) == 0
}

// controlSummary counts one arm's already-fixed controls, which the arm's
// other aggregates leave out; it is nil when the arm ran none, so a report
// without controls keeps its bytes.
func controlSummary(details []taskRecord, name string) map[string]any {
	tasks, errored, failed, observed, abstained := 0, 0, 0.0, 0.0, 0.0
	for _, item := range details {
		arm := item.Arms[name]
		if item.Control != controlAlreadyFixed || arm == nil {
			continue
		}
		tasks++
		if arm.Metrics == nil {
			errored++
			continue
		}
		failed += arm.Metrics["control_failed"]
		if value, present := arm.Metrics["packet_abstained"]; present {
			observed++
			abstained += value
		}
	}
	if tasks == 0 {
		return nil
	}
	return map[string]any{"tasks": tasks, "errors": errored, "control_failed": failed, "packets": observed, "packet_abstained": abstained}
}

func boolFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
