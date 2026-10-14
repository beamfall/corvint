package behaviorfalsify

import (
	"encoding/json"
	"errors"
	"sort"
)

// MaxHistoryReceipts bounds the retained receipts one inert-control history reads (BBF-V0-017).
const MaxHistoryReceipts = 64

// ReasonRepeatedNotApplied flags a control that was NOT-APPLIED at two or more revisions.
const ReasonRepeatedNotApplied = "repeated-not-applied"

// HistoryReceipt is one caller-supplied retained evidence receipt with the plan and executable
// digests the caller pins for it. The history is exactly these receipts; nothing is discovered,
// stored or written.
type HistoryReceipt struct {
	Receipt    EvidenceReceipt
	PlanDigest string
	ToolDigest string
}

// ControlIdentity is "the same control" across revisions: its contract, criterion, assertion,
// test, project, control ID, kind and declared marker. Revisions, hook bytes and perturbation
// digests are deliberately excluded so an unchanged control stays one identity as code moves.
type ControlIdentity struct {
	ContractID  string            `json:"contract_id"`
	CriterionID string            `json:"criterion_id"`
	AssertionID string            `json:"assertion_id"`
	TestID      string            `json:"test_id"`
	Project     string            `json:"project"`
	ControlID   string            `json:"control_id"`
	Kind        ControlKind       `json:"kind"`
	Marker      ApplicationMarker `json:"marker"`
}

// Revision is the application and test revision pair a receipt's plan pins.
type Revision struct {
	Application string `json:"application_revision"`
	Test        string `json:"test_revision"`
}

// ControlDefect is one control that stayed inert at two or more distinct revisions.
type ControlDefect struct {
	Control   ControlIdentity `json:"control"`
	Reason    string          `json:"reason"`
	Revisions []Revision      `json:"revisions"`
}

// InertControlDefects verifies every supplied receipt against its pinned digests and flags each
// control identity whose NOT-APPLIED results span at least two distinct revisions. One
// unverifiable receipt refuses the whole history rather than silently narrowing it.
func InertControlDefects(history []HistoryReceipt) ([]ControlDefect, error) {
	if len(history) == 0 || len(history) > MaxHistoryReceipts {
		return nil, errors.New("history-bound-exceeded")
	}
	inert := map[ControlIdentity]map[Revision]bool{}
	for _, entry := range history {
		if VerifyReceipt(entry.Receipt, entry.PlanDigest, entry.ToolDigest) != nil {
			return nil, errors.New("history-receipt-invalid")
		}
		var plan Plan
		if Decode(entry.Receipt.PlanPreimage, &plan) != nil {
			return nil, errors.New("history-receipt-invalid")
		}
		target := plan.Request.Target
		revision := Revision{Application: target.ApplicationRevision, Test: target.TestRevision}
		for index, result := range entry.Receipt.Report.Results {
			control := plan.Controls[index]
			if result.Application != ApplicationNotApplied || control.ApplicationMarker == nil {
				continue
			}
			identity := ControlIdentity{
				ContractID: target.ContractID, CriterionID: target.CriterionID, AssertionID: target.AssertionID,
				TestID: target.TestID, Project: target.Project, ControlID: control.ID, Kind: control.Kind,
				Marker: *control.ApplicationMarker,
			}
			if inert[identity] == nil {
				inert[identity] = map[Revision]bool{}
			}
			inert[identity][revision] = true
		}
	}
	defects := []ControlDefect{}
	for identity, revisions := range inert {
		if len(revisions) < 2 {
			continue
		}
		defect := ControlDefect{Control: identity, Reason: ReasonRepeatedNotApplied}
		for revision := range revisions {
			defect.Revisions = append(defect.Revisions, revision)
		}
		sort.Slice(defect.Revisions, func(i, j int) bool {
			a, b := defect.Revisions[i], defect.Revisions[j]
			return a.Application < b.Application || a.Application == b.Application && a.Test < b.Test
		})
		defects = append(defects, defect)
	}
	sort.Slice(defects, func(i, j int) bool { return identityKey(defects[i].Control) < identityKey(defects[j].Control) })
	return defects, nil
}

func identityKey(identity ControlIdentity) string {
	raw, _ := json.Marshal(identity)
	return string(raw)
}
