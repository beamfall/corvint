package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const certainReply = "```json\n{\"claims\":[{\"kind\":\"source-file\",\"value\":\"port.go\",\"confidence\":\"certain\",\"evidence\":\"none\"}]}\n```\n"

// CEP-V0-004/005: on an already-fixed control every valid claim is FALSE, a
// certain one is a recorded failure, and a produced packet that does not
// abstain is a recorded failure even when the agent abstains.
func TestAlreadyFixedControlScoresAnyConfidentAnswerAsFailure(t *testing.T) {
	abstaining := `{"state":"NO_CANDIDATES","results":[]}`
	withheld := `{"state":"READY","results":[{"id":"AGENTS.md"}],"coverage":{"answerability":{"verdict":"unsupported-conjunction"}}}`
	answering := `{"state":"READY","results":[{"id":"port.go"}]}`
	cases := []struct {
		name   string
		record armRecord
		expect map[string]float64
		absent string
	}{
		{"certain claim fails", armRecord{Reply: certainReply}, map[string]float64{"confidently_wrong_task": 1, "abstained": 0, "success": 0, "control_failed": 1}, "packet_abstained"},
		{"likely claim is wrong but not confident", armRecord{Reply: strings.Replace(certainReply, "certain", "likely", 1)}, map[string]float64{"wrong_likely": 1, "unjudged": 0, "control_failed": 0}, "packet_abstained"},
		{"silence abstains", armRecord{Reply: "Already fixed at this revision."}, map[string]float64{"abstained": 1, "control_failed": 0}, "packet_abstained"},
		{"abstaining packet", armRecord{Reply: "```json\n{\"claims\":[]}\n```", Context: abstaining, ContextState: "NO_CANDIDATES"}, map[string]float64{"abstained": 1, "packet_abstained": 1, "control_failed": 0}, ""},
		{"withheld packet abstains", armRecord{Reply: "none", Context: withheld, ContextState: "READY"}, map[string]float64{"packet_abstained": 1, "control_failed": 0}, ""},
		{"answering packet fails", armRecord{Reply: "none", Context: answering, ContextState: "READY"}, map[string]float64{"abstained": 1, "packet_abstained": 0, "control_failed": 1}, ""},
		{"cut packet fails", armRecord{Reply: "none", Context: answering[:20], ContextState: "READY"}, map[string]float64{"packet_abstained": 0, "control_failed": 1}, ""},
		{"failed producer is unobserved", armRecord{Reply: "none", ContextState: "UNREADABLE", ContextError: "corvint: exit 2"}, map[string]float64{"control_failed": 0}, "packet_abstained"},
	}
	for _, item := range cases {
		record := item.record
		scoreArm(&record, &taskRecord{Control: controlAlreadyFixed})
		for key, want := range item.expect {
			if got, present := record.Metrics[key]; !present || got != want {
				t.Errorf("%s: %s = %v (present %v), want %v", item.name, key, got, present, want)
			}
		}
		if _, present := record.Metrics[item.absent]; item.absent != "" && present {
			t.Errorf("%s: %s must be absent: %v", item.name, item.absent, record.Metrics)
		}
	}
	plain := armRecord{Reply: certainReply}
	scoreArm(&plain, &taskRecord{})
	if _, present := plain.Metrics["control_failed"]; present || plain.Metrics["unjudged"] != 1 {
		t.Fatalf("a task without a control keeps its scoring: %v", plain.Metrics)
	}
}

func TestAlreadyFixedControlRefusesGoldAndUnknownControls(t *testing.T) {
	base := task{ID: "c", Kind: "retrieval", Text: "t", Control: controlAlreadyFixed}
	if err := validateTask(base); err != nil {
		t.Fatalf("a gold-free control: %v", err)
	}
	withGold := base
	withGold.Gold = map[string][]string{"source-file": {"port.go"}}
	if err := validateTask(withGold); err == nil {
		t.Fatal("an already-fixed control with gold must be refused")
	}
	unknown := base
	unknown.Control = "not-a-bug"
	if err := validateTask(unknown); err == nil {
		t.Fatal("an unknown control must be refused")
	}
}

// The committed fixture runs offline end to end: a certain claim is the arm's
// recorded control failure, and the summary counts it.
func TestAlreadyFixedFixtureRecordsAConfidentAnswerAsFailure(t *testing.T) {
	directory := filepath.Join("testdata", "already-fixed")
	configuration := options{
		tasks: filepath.Join(directory, "tasks.json"), corpus: filepath.Join(directory, "corpus"),
		arms: []string{"none", "grep"}, agent: "script", agentCommand: fakeAgent(t, certainReply, ""),
		access: "read-only", timeout: time.Minute, limit: 20,
	}
	document, err := trial(context.Background(), configuration, newAgent(configuration))
	if err != nil {
		t.Fatal(err)
	}
	item := document.Details[0]
	if item.Control != controlAlreadyFixed || len(item.Gold) != 0 {
		t.Fatalf("control record: %+v", item)
	}
	for _, name := range configuration.arms {
		arm := item.Arms[name]
		if arm.Error != "" || arm.Claims[0].Verdict != "FALSE" || arm.Metrics["control_failed"] != 1 {
			t.Fatalf("%s: %+v", name, arm)
		}
		summary := document.Arms[name].(map[string]any)["already_fixed"].(map[string]any)
		if summary["tasks"] != 1 || summary["control_failed"] != 1.0 || summary["packets"] != 0.0 {
			t.Fatalf("%s summary: %v", name, summary)
		}
	}
}
