package skhproof

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
)

// The live-run outputs Record reads (SKH-V0-180).
const (
	ProveSchema    = "e2e-prove/0"
	ValiditySchema = "corvint-test-source-validity/0"
	// AxisObserved is the only assessment value condition 3 accepts.
	AxisObserved = "OBSERVED"
)

// ReadFile returns the committed bytes of path at rev.
type ReadFile func(rev, path string) ([]byte, error)

// proveDoc is the part of an e2e-prove/0 report Record reads.
type proveDoc struct {
	Schema    string       `json:"schema"`
	Head      string       `json:"head"`
	Retries   int          `json:"retries"`
	Scenarios []proveScene `json:"scenarios"`
}

type proveScene struct {
	Placement string      `json:"placement"`
	Hosted    bool        `json:"hosted"`
	Verdict   string      `json:"verdict"`
	Head      proveSide   `json:"head"`
	Base      proveSide   `json:"base"`
	Controls  []proveCtrl `json:"controls"`
}

type proveSide struct {
	ApplicationRevision string `json:"application_revision"`
	TestRevision        string `json:"test_revision"`
	Qualified           bool   `json:"qualified"`
	Reason              string `json:"reason"`
	Stability           string `json:"stability"`
	Outcomes            []struct {
		State string `json:"state"`
	} `json:"outcomes"`
}

type proveCtrl struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// validityDoc is the part of a corvint-test-source-validity/0 report Record reads.
type validityDoc struct {
	Schema   string `json:"schema"`
	Revision string `json:"revision"`
	Files    []struct {
		Path      string `json:"path"`
		Uncertain bool   `json:"uncertain"`
		Findings  []struct {
			Rule     string `json:"rule"`
			Severity string `json:"severity"`
		} `json:"findings"`
	} `json:"files"`
	Tests []struct {
		Test    string `json:"test"`
		Hygiene struct {
			State string `json:"state"`
		} `json:"hygiene"`
		Strength *json.RawMessage `json:"strength"`
	} `json:"tests"`
}

// answerDoc is the part of the how answer Record reads for bindings and the outcome assertion.
type answerDoc struct {
	GoalScreen *struct {
		ID string `json:"id"`
	} `json:"goal_screen"`
	Oracle []struct {
		Locator  string `json:"locator"`
		Matcher  string `json:"matcher"`
		Expected string `json:"expected"`
	} `json:"oracle"`
	Steps []struct {
		Index  int    `json:"index"`
		Action string `json:"action"`
		Via    string `json:"via"`
	} `json:"steps"`
}

// Record maps the live run's own outputs for the answer's candidate into
// site-knowledge-proof-evidence/0 (SKH-V0-180..183): the hosted `corvint-test e2e prove` report
// for the candidate's proposed path, the `corvint tests validity` report for that path, and the
// committed bytes read back through read. It copies observed facts only. A run the prover did not
// qualify is recorded as infrastructure; an axis, witness or mutant the run did not observe is
// left unrecorded, so Assess blocks on it rather than reading it as held. Hygiene has a named
// source (SKH-V0-186); association, freshness and cleanup have none in these reports, so they
// stay unrecorded (SKH-V0-187).
func Record(answer, prove, validity []byte, read ReadFile) (*Evidence, error) {
	_, t, err := Candidate(answer)
	if err != nil {
		return nil, err
	}
	if t.draftStatus != appmap.HowComplete {
		return nil, refuse(CodeGenerateIncomplete, "the draft reads %s; there is no candidate to record", t.draftStatus)
	}
	var p proveDoc
	if len(prove) > maxEvidenceBytes || json.Unmarshal(prove, &p) != nil || p.Schema != ProveSchema {
		return nil, refuse("skh-evidence-invalid", "not a %s report", ProveSchema)
	}
	var v validityDoc
	if len(validity) > maxEvidenceBytes || json.Unmarshal(validity, &v) != nil || v.Schema != ValiditySchema {
		return nil, refuse("skh-evidence-invalid", "not a %s report", ValiditySchema)
	}
	var a answerDoc
	_ = json.Unmarshal(answer, &a) // Candidate already decoded the answer
	e := &Evidence{Schema: EvidenceSchema, Runs: []Run{}, Validity: Validity{Examples: []Example{}, Defects: []string{}},
		Assessment: map[string]string{}, Bindings: bindings(a), Controls: []Control{}, Verdict: Verdict{Mutants: []string{}},
		Assertion: assertion(a)}
	var scene *proveScene
	for i := range p.Scenarios {
		if s := &p.Scenarios[i]; s.Hosted && s.Placement == t.ProposedPath {
			if scene != nil {
				e.Infrastructure = "prove-scenario-ambiguous: more than one hosted scenario at " + t.ProposedPath
				return e, nil
			}
			scene = s
		}
	}
	if scene == nil {
		e.Infrastructure = "prove-scenario-missing: no hosted scenario at " + t.ProposedPath
		return e, nil
	}
	head := scene.Head
	if !head.Qualified {
		e.Infrastructure = "head-receipt-unqualified: " + cmpOr(head.Reason, "no reason recorded")
		return e, nil
	}
	if head.ApplicationRevision == "" || head.ApplicationRevision != p.Head || head.TestRevision != head.ApplicationRevision {
		e.Infrastructure = "head-revision-unpinned: the candidate did not run at the pinned application revision"
		return e, nil
	}
	ran, err := read(head.ApplicationRevision, t.ProposedPath)
	if err != nil {
		e.Infrastructure = "candidate-unreadable-at-head: " + err.Error()
		return e, nil
	}
	e.CandidateSHA256, e.AppRevision = digestOf(ran), head.ApplicationRevision
	passed := head.Stability == "DETERMINISTIC" && len(head.Outcomes) > 0
	for _, o := range head.Outcomes {
		e.Runs = append(e.Runs, Run{Status: o.State, Retries: p.Retries})
		passed = passed && o.State == "passed"
	}
	if passed && p.Retries == 0 {
		e.Assessment["execution"] = AxisObserved
	}
	for _, c := range scene.Controls {
		e.Controls = append(e.Controls, Control{ID: c.ID, Result: c.Status})
	}
	e.Verdict.Verdict = scene.Verdict
	if scene.Base.Qualified && scene.Base.ApplicationRevision != "" {
		e.Verdict.Mutants = append(e.Verdict.Mutants, "base:"+scene.Base.ApplicationRevision)
	}
	e.Validity, err = recordValidity(v, t.ProposedPath, read)
	if err != nil {
		e.Infrastructure = "candidate-unreadable-at-validity-revision: " + err.Error()
		return e, nil
	}
	// Hygiene is read only from a validity reading of exactly the bytes the run executed, with
	// one example in the file. Association is not recorded: the report's outcomes carry no
	// runner-observed test identity (SKH-V0-187).
	single := len(e.Validity.Examples) == 1 && e.Validity.Examples[0].Blob == gitBlob(ran) && !e.Validity.Uncertain
	if single && e.Validity.Examples[0].Hygiene == "ELIGIBLE" {
		e.Assessment["hygiene"] = AxisObserved
	}
	return e, nil
}

// recordValidity copies the candidate file's validity reading: its uncertainty, each invalid
// finding as a defect, and each example with its located state, line, hygiene and the Git blob of
// the bytes the report read (SKH-V0-181). A located example is one carrying its TSA-V0-081
// strength axis; any other example records its hygiene state as its status.
func recordValidity(v validityDoc, path string, read ReadFile) (Validity, error) {
	out := Validity{Examples: []Example{}, Defects: []string{}}
	found := false
	for _, f := range v.Files {
		if f.Path != path {
			continue
		}
		found = true
		out.Uncertain = f.Uncertain
		for _, finding := range f.Findings {
			if finding.Severity == "invalid" {
				out.Defects = append(out.Defects, finding.Rule)
			}
		}
	}
	if !found {
		return out, nil
	}
	data, err := read(v.Revision, path)
	if err != nil {
		return out, err
	}
	blob := gitBlob(data)
	for _, test := range v.Tests {
		rest, ok := strings.CutPrefix(test.Test, path+":")
		if !ok {
			continue
		}
		line, err := strconv.Atoi(rest)
		if err != nil {
			line = 0
		}
		status := cmpOr(test.Hygiene.State, "UNKNOWN")
		if test.Strength != nil {
			status = "LOCATED"
		}
		out.Examples = append(out.Examples, Example{Status: status, Blob: blob, Line: line, Hygiene: test.Hygiene.State})
	}
	return out, nil
}

// bindings lists the answer's steps and the page-object methods they call, each with no witness:
// the prove report records no per-element behavioural witness (SKH-V0-089, SKH-V0-182).
func bindings(a answerDoc) []Binding {
	out := []Binding{}
	for _, s := range a.Steps {
		out = append(out, Binding{Element: fmt.Sprintf("step:%d %s", s.Index, s.Action), Kind: "step"})
		if method, ok := strings.CutPrefix(s.Via, "page-object:"); ok {
			out = append(out, Binding{Element: method, Kind: "method"})
		}
	}
	return out
}

// assertion is the candidate's outcome assertion as the answer reads it: the goal screen and the
// one shared oracle; with no goal screen or not exactly one oracle there is none (SKH-V0-183).
func assertion(a answerDoc) *Oracle {
	if a.GoalScreen == nil || len(a.Oracle) != 1 {
		return nil
	}
	o := a.Oracle[0]
	return &Oracle{Screen: a.GoalScreen.ID, Locator: o.Locator, Matcher: o.Matcher, Expected: o.Expected}
}

// Encode returns the evidence's canonical bytes with a final newline.
func (e *Evidence) Encode() []byte {
	data, _ := json.Marshal(e)
	return append(data, '\n')
}
