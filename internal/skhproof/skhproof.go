// Package skhproof is the site knowledge stage 8 acceptance pipeline (SKH-V0-080..089,
// SKH-V0-165..176): it turns the guarded how draft into the executable candidate by one recorded
// transform, and assesses recorded live-run, validity, provider, binding, control, prover and
// oracle evidence for those exact bytes into one acceptance receipt. It runs only in the
// corvint-test companion (SKH-V0-083); core corvint never imports it. V0 reads recorded evidence:
// it starts no browser and runs no test.
package skhproof

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
)

const (
	ReceiptSchema  = "site-knowledge-acceptance/0"
	EvidenceSchema = "site-knowledge-proof-evidence/0"
	OracleSchema   = "site-knowledge-accepted-oracle/0"

	Proven            = "PROVEN"
	ExecutionVerified = "EXECUTION_VERIFIED"
	Blocked           = "BLOCKED"

	// The closed first-failing codes of SKH-V0-081, in assessment order.
	CodeGenerateIncomplete  = "generate-incomplete"
	CodeRunFailed           = "run-failed"
	CodeValidityUnknown     = "validity-unestablished"
	CodeValidityDefect      = "validity-defect"
	CodeAcceptanceBlocked   = "acceptance-blocked"
	CodeBindingUnverified   = "binding-unverified"
	CodeControlSurvived     = "control-survived"
	CodeNotDiscriminating   = "not-discriminating"
	CodeDiscriminationUnk   = "discrimination-inconclusive"
	CodeInfrastructure      = "infrastructure"
	CodeOracleUnaccepted    = "oracle-unaccepted"
	CodeOracleMismatch      = "oracle-mismatch"
	TransformRemoveGuard    = "remove-guard-line"
	minRepeats, maxRepeats  = 2, 100 // NEA-V0-002
	maxEvidenceBytes        = 1 << 20
	maxRecordedListElements = 256
)

// proverAccepted is false until the V1-1733 prover contract is accepted: until then no verdict
// satisfies condition 4, so nothing reads PROVEN or EXECUTION_VERIFIED (SKH-V0-085). Tests flip
// it to exercise the full ladder.
var proverAccepted = false

// requiredAxes are the NEA-V0 assessment axes that must each read OBSERVED (SKH-V0-082).
var requiredAxes = []string{"association", "hygiene", "freshness", "execution", "cleanup"}

// Transform is the one recorded draft-to-candidate transform (SKH-V0-088).
type Transform struct {
	Kind           string `json:"kind"`
	Line           int    `json:"line"`
	RemovedSHA256  string `json:"removed_sha256"`
	DraftDigest    string `json:"draft_digest"`
	CandidateSHA   string `json:"candidate_sha256"`
	CandidateBlob  string `json:"candidate_blob"`
	ProposedPath   string `json:"proposed_path"`
	SourceTest     string `json:"source_test"`
	MapRevision    string `json:"map_revision"`
	draftStatus    string
	draftUnchecked bool
}

// Condition is one of the six conditions of "confidently", with the code that failed it.
type Condition struct {
	Condition int    `json:"condition"`
	Name      string `json:"name"`
	Holds     bool   `json:"holds"`
	Code      string `json:"code,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

// Receipt is the one acceptance receipt of SKH-V0-080.
type Receipt struct {
	Schema         string      `json:"schema"`
	Status         string      `json:"status"`
	Code           string      `json:"code,omitempty"`
	Detail         string      `json:"detail,omitempty"`
	OracleCode     string      `json:"oracle_code,omitempty"`
	Evidence       string      `json:"evidence"`
	Transform      *Transform  `json:"transform"`
	AppRevision    string      `json:"app_revision,omitempty"`
	Conditions     []Condition `json:"conditions"`
	BlockedAxes    []string    `json:"blocked_axes"`
	Unverified     []string    `json:"unverified"`
	Verdict        string      `json:"verdict,omitempty"`
	ProverAccepted bool        `json:"prover_accepted"`
	Mutants        []string    `json:"mutants"`
	Controls       []Control   `json:"controls"`
}

// Evidence is the recorded live-run evidence for one candidate (site-knowledge-proof-evidence/0).
type Evidence struct {
	Schema          string            `json:"schema"`
	CandidateSHA256 string            `json:"candidate_sha256"`
	AppRevision     string            `json:"app_revision"`
	Infrastructure  string            `json:"infrastructure"`
	Runs            []Run             `json:"runs"`
	Validity        Validity          `json:"validity"`
	Assessment      map[string]string `json:"assessment"`
	Bindings        []Binding         `json:"bindings"`
	Controls        []Control         `json:"controls"`
	Verdict         Verdict           `json:"verdict"`
	Assertion       *Oracle           `json:"assertion"`
}

// Run is one live repeat as the provider observed it.
type Run struct {
	Status  string `json:"status"`
	Retries int    `json:"retries"`
}

// Validity is the recorded `corvint tests validity` result for the candidate (TSA-V0-081).
type Validity struct {
	Uncertain bool      `json:"uncertain"`
	Examples  []Example `json:"examples"`
	Defects   []string  `json:"defects"`
}

// Example is one located test example.
type Example struct {
	Status  string `json:"status"`
	Blob    string `json:"blob"`
	Line    int    `json:"line"`
	Hygiene string `json:"hygiene"`
}

// Binding is one step, selector or method of the candidate and its behavioural witness.
type Binding struct {
	Element string `json:"element"`
	Kind    string `json:"kind"`
	Witness string `json:"witness"`
}

// Control is one approved negative control and its observed result.
type Control struct {
	ID     string `json:"id"`
	Result string `json:"result"`
}

// Verdict is the V1-1733 prover's CCE-V0-009 verdict with the mutant identities it ran.
type Verdict struct {
	Verdict string   `json:"verdict"`
	Mutants []string `json:"mutants"`
}

// Oracle is an outcome assertion; as an accepted oracle it also names its origin.
type Oracle struct {
	Schema   string `json:"schema,omitempty"`
	Origin   string `json:"origin,omitempty"`
	Commit   string `json:"commit,omitempty"`
	Screen   string `json:"screen"`
	Locator  string `json:"locator"`
	Matcher  string `json:"matcher"`
	Expected string `json:"expected"`
}

// Refusal is a coded input refusal.
type Refusal struct{ Code, Detail string }

func (r *Refusal) Error() string { return r.Code + ": " + r.Detail }

func refuse(code, format string, args ...any) error {
	return &Refusal{Code: code, Detail: fmt.Sprintf(format, args...)}
}

// Candidate checks the how answer's draft and applies the one transform: it removes the guard
// line, and nothing else, from the draft's lines (SKH-V0-165, SKH-V0-166).
func Candidate(answer []byte) ([]byte, *Transform, error) {
	var doc struct {
		Schema   string          `json:"schema"`
		Revision string          `json:"map_revision"`
		Draft    *appmap.SKDraft `json:"draft"`
	}
	if len(answer) > maxEvidenceBytes || json.Unmarshal(answer, &doc) != nil || doc.Schema != appmap.HowSchema || doc.Draft == nil {
		return nil, nil, refuse("skh-answer-invalid", "not a %s answer with a draft", appmap.HowSchema)
	}
	d := doc.Draft
	t := &Transform{Kind: TransformRemoveGuard, DraftDigest: d.Digest, ProposedPath: d.ProposedPath, SourceTest: d.Source,
		MapRevision: doc.Revision, draftStatus: d.Status}
	if d.Status != appmap.HowComplete {
		return nil, t, nil // generate-incomplete: there is no candidate
	}
	if digestOf([]byte(strings.Join(d.Lines, "\n")+"\n")) != d.Digest {
		return nil, nil, refuse("skh-answer-invalid", "draft digest does not match its lines")
	}
	if d.GuardLine < 1 || d.GuardLine > len(d.Lines) || d.Lines[d.GuardLine-1] != appmap.DraftGuardLine {
		return nil, nil, refuse("skh-answer-invalid", "draft guard line %d is not the guard", d.GuardLine)
	}
	for i, line := range d.Lines {
		if i != d.GuardLine-1 && strings.Contains(line, "test.fixme(") {
			return nil, nil, refuse("skh-answer-invalid", "draft line %d holds a second fixme", i+1)
		}
	}
	kept := slices.Concat(d.Lines[:d.GuardLine-1], d.Lines[d.GuardLine:])
	candidate := []byte(strings.Join(kept, "\n") + "\n")
	t.Line = d.GuardLine
	t.RemovedSHA256 = digestOf([]byte(appmap.DraftGuardLine))
	t.CandidateSHA = digestOf(candidate)
	t.CandidateBlob = gitBlob(candidate)
	return candidate, t, nil
}

// DecodeEvidence reads recorded evidence strictly: a closed schema, bounded lists.
func DecodeEvidence(data []byte) (*Evidence, error) {
	var e Evidence
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if len(data) > maxEvidenceBytes || dec.Decode(&e) != nil || e.Schema != EvidenceSchema {
		return nil, refuse("skh-evidence-invalid", "not a %s document", EvidenceSchema)
	}
	for _, n := range []int{len(e.Runs), len(e.Validity.Examples), len(e.Validity.Defects), len(e.Bindings), len(e.Controls), len(e.Verdict.Mutants)} {
		if n > maxRecordedListElements {
			return nil, refuse("skh-evidence-invalid", "a recorded list exceeds %d entries", maxRecordedListElements)
		}
	}
	return &e, nil
}

// DecodeOracle reads an accepted outcome oracle (SKH-V0-087).
func DecodeOracle(data []byte) (*Oracle, error) {
	var o Oracle
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if len(data) > maxEvidenceBytes || dec.Decode(&o) != nil || o.Schema != OracleSchema {
		return nil, refuse("skh-oracle-invalid", "not a %s document", OracleSchema)
	}
	return &o, nil
}

// Assess evaluates the six conditions over recorded evidence for the transform's candidate and
// returns the receipt naming the first failing condition (SKH-V0-081, SKH-V0-167..176). A nil
// evidence document means the live run was not recorded: the receipt blocks as infrastructure.
func Assess(t *Transform, e *Evidence, oracle *Oracle) Receipt {
	r := Receipt{Schema: ReceiptSchema, Evidence: "recorded", Transform: t, ProverAccepted: proverAccepted,
		Conditions: []Condition{}, BlockedAxes: []string{}, Unverified: []string{}, Mutants: []string{}, Controls: []Control{}}
	fail := func(code, detail string) Receipt {
		r.Status, r.Code, r.Detail = Blocked, code, detail
		return r
	}
	if t.draftStatus != appmap.HowComplete {
		return fail(CodeGenerateIncomplete, "draft reads "+t.draftStatus)
	}
	if e == nil {
		r.Evidence = "none"
		return fail(CodeInfrastructure, "live-run-not-recorded: the pinned Playwright host did not run the candidate")
	}
	r.AppRevision = e.AppRevision
	if e.Infrastructure != "" {
		return fail(CodeInfrastructure, e.Infrastructure)
	}
	// Every check runs on the exact candidate bytes; other bytes void the evidence.
	if e.CandidateSHA256 != t.CandidateSHA {
		return fail(CodeInfrastructure, "candidate-bytes-differ: evidence names "+e.CandidateSHA256)
	}
	if e.AppRevision == "" {
		return fail(CodeRunFailed, "no pinned application revision")
	}
	add := func(n int, name, code, detail string) bool {
		r.Conditions = append(r.Conditions, Condition{Condition: n, Name: name, Holds: code == "", Code: code, Detail: detail})
		return code == ""
	}
	// 081 order: run, validity, acceptance, binding, controls, discrimination, then the oracle.
	if code, detail := runs(e.Runs); !add(1, "ran-and-passed", code, detail) {
		return fail(code, detail)
	}
	if code, detail := validity(e.Validity, t.CandidateBlob); !add(2, "valid-source", code, detail) {
		return fail(code, detail)
	}
	for _, axis := range requiredAxes {
		if v := e.Assessment[axis]; v != "OBSERVED" {
			r.BlockedAxes = append(r.BlockedAxes, axis+"="+cmpOr(v, "UNKNOWN"))
		}
	}
	if len(r.BlockedAxes) > 0 && !add(3, "accepted-by-provider", CodeAcceptanceBlocked, strings.Join(r.BlockedAxes, ", ")) {
		return fail(CodeAcceptanceBlocked, strings.Join(r.BlockedAxes, ", "))
	}
	add(3, "accepted-by-provider", "", "")
	r.Unverified = unverified(e.Bindings)
	if len(e.Bindings) == 0 {
		r.Unverified = []string{"no-bindings-recorded"}
	}
	if len(r.Unverified) > 0 {
		add(5, "anchored-and-witnessed", CodeBindingUnverified, strings.Join(r.Unverified, ", "))
		return fail(CodeBindingUnverified, strings.Join(r.Unverified, ", "))
	}
	add(5, "anchored-and-witnessed", "", "")
	r.Controls = append(r.Controls, e.Controls...)
	if code, detail := controls(e.Controls); code != "" {
		add(4, "discriminates", code, detail)
		return fail(code, detail)
	}
	r.Verdict, r.Mutants = e.Verdict.Verdict, append(r.Mutants, e.Verdict.Mutants...)
	if code, detail := verdict(e.Verdict); !add(4, "discriminates", code, detail) {
		return fail(code, detail)
	}
	code, detail := oracleCode(oracle, e.Assertion)
	if !add(6, "demonstrates-the-request", code, detail) {
		r.Status, r.OracleCode, r.Detail = ExecutionVerified, code, detail
		return r
	}
	r.Status = Proven
	return r
}

// runs holds when 2 to 100 repeats all passed with retries zero; a fixme or skipped run of the
// guarded draft, or any flaky or failed repeat, reads run-failed (SKH-V0-082, SKH-V0-088).
func runs(rs []Run) (string, string) {
	if len(rs) < minRepeats || len(rs) > maxRepeats {
		return CodeRunFailed, "repeats=" + strconv.Itoa(len(rs)) + " outside 2..100"
	}
	for i, run := range rs {
		if run.Retries != 0 {
			return CodeRunFailed, fmt.Sprintf("repeat %d retried %d times", i+1, run.Retries)
		}
		if run.Status != "passed" {
			return CodeRunFailed, fmt.Sprintf("repeat %d %s", i+1, cmpOr(run.Status, "unknown"))
		}
	}
	return "", ""
}

// validity holds only with exactly one LOCATED, ELIGIBLE example in the candidate's blob and no
// defect; an empty finding list is never validity (SKH-V0-086).
func validity(v Validity, blob string) (string, string) {
	if len(v.Defects) > 0 {
		return CodeValidityDefect, strings.Join(v.Defects, ", ")
	}
	switch {
	case v.Uncertain:
		return CodeValidityUnknown, "uncertain file"
	case len(v.Examples) != 1:
		return CodeValidityUnknown, strconv.Itoa(len(v.Examples)) + " examples"
	case v.Examples[0].Status != "LOCATED":
		return CodeValidityUnknown, "example " + cmpOr(v.Examples[0].Status, "UNKNOWN")
	case v.Examples[0].Blob != blob:
		return CodeValidityUnknown, "example read other bytes: " + v.Examples[0].Blob
	case v.Examples[0].Line < 1:
		return CodeValidityUnknown, "example has no callee line"
	case v.Examples[0].Hygiene != "ELIGIBLE":
		return CodeValidityUnknown, "hygiene " + cmpOr(v.Examples[0].Hygiene, "UNKNOWN")
	}
	return "", ""
}

// unverified lists the elements with no behavioural witness. Selector and method witnesses need
// the PWP-V0, RVN-V0 and AMSP-V0 amendments of SKH-V0-089, which have not landed, so V0 reads
// every selector and method unverified whatever the recorded witness says.
func unverified(bs []Binding) []string {
	out := []string{}
	for _, b := range bs {
		switch {
		case b.Kind == "selector" || b.Kind == "method":
			out = append(out, b.Element+" ("+b.Kind+": witness contract not amended)")
		case b.Kind != "step":
			out = append(out, b.Element+" (unknown kind "+b.Kind+")")
		case b.Witness == "":
			out = append(out, b.Element+" (step: no witness)")
		}
	}
	return out
}

// controls holds when at least one approved control ran and every one reads killed.
func controls(cs []Control) (string, string) {
	if len(cs) == 0 {
		return CodeControlSurvived, "no approved control recorded"
	}
	for _, c := range cs {
		if c.Result != "killed" {
			return CodeControlSurvived, c.ID + " " + cmpOr(c.Result, "unknown")
		}
	}
	return "", ""
}

// verdict applies the SKH-V0-085 allowlist to the prover's CCE-V0-009 verdict.
func verdict(v Verdict) (string, string) {
	switch v.Verdict {
	case "DISCRIMINATES", "NEW_REACH":
		if len(v.Mutants) == 0 {
			return CodeDiscriminationUnk, v.Verdict + " names no mutant"
		}
		if !proverAccepted {
			return CodeDiscriminationUnk, "prover-contract-unaccepted (V1-1733)"
		}
		return "", ""
	case "PRESERVES":
		return CodeNotDiscriminating, v.Verdict
	case "CONTROL_SURVIVED":
		return CodeControlSurvived, v.Verdict
	case "REGRESSION_DETECTED", "SCENARIO_BROKEN":
		return CodeRunFailed, v.Verdict
	}
	return CodeDiscriminationUnk, cmpOr(v.Verdict, "no verdict")
}

// oracleCode compares the candidate's outcome assertion with the accepted oracle on all four
// fields; a learned or proposed oracle never qualifies (SKH-V0-087).
func oracleCode(accepted, assertion *Oracle) (string, string) {
	if accepted == nil || accepted.Commit == "" || (accepted.Origin != "owner-request-set" && accepted.Origin != "afu-v1-flow-intent") {
		return CodeOracleUnaccepted, "no accepted oracle for the request"
	}
	if assertion == nil || assertion.Screen != accepted.Screen || assertion.Locator != accepted.Locator ||
		assertion.Matcher != accepted.Matcher || assertion.Expected != accepted.Expected {
		return CodeOracleMismatch, "outcome assertion differs from the accepted oracle"
	}
	return "", ""
}

// Encode returns the receipt's canonical bytes with a final newline.
func (r Receipt) Encode() []byte {
	data, _ := json.Marshal(r)
	return append(data, '\n')
}

func cmpOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func digestOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// gitBlob is the Git blob ID of data, the identity TSA-V0 validity locates examples in.
func gitBlob(data []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(data))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
