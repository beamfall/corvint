package appflows

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/secretscreen"
	"github.com/Beamfall/corvint/internal/tcq"
)

// Wire identity, authorities and bounds of test-run-evidence/0 (AFU-V1-011, AFU-V1-014, AFU-V1-037).
const (
	RunEvidenceSchema        = "test-run-evidence/0"
	AuthorityIngested        = "INGESTED"
	AuthorityLocallyObserved = "LOCALLY_OBSERVED"
	AuthorityStatic          = "STATIC"
	maxRunRecords            = 8192
	maxRunAttempts           = 32
	maxRunLinks              = 256
	maxRunDepth              = 64
)

// Named codes of an incomplete run-evidence ingest; each names the bound that was exceeded (AFU-V1-037).
const (
	BoundBytes     = "run-evidence-byte-bound"
	BoundRecords   = "run-evidence-record-bound"
	BoundAttempts  = "run-evidence-attempt-bound"
	BoundLinks     = "run-evidence-link-bound"
	BoundTraversal = "run-evidence-traversal-bound"
)

var (
	attemptOutcomes = []string{"passed", "failed", "timedOut", "skipped", "interrupted"}
	cleanupStates   = []string{"done", "failed", "not-declared"}
	// runResults are the per-test classifications: an attempt outcome, flaky, or not-run.
	runResults = append(slices.Clone(attemptOutcomes), "flaky", "not-run")
)

// TestRunEvidence is one closed test-run-evidence/0 record: one test key in one run (AFU-V1-011).
type TestRunEvidence struct {
	Schema              string            `json:"schema"`
	Authority           string            `json:"authority"`
	RunID               string            `json:"run_id"`
	Runner              RunRunner         `json:"runner"`
	Source              RunSource         `json:"source"`
	BuildArtifactDigest string            `json:"build_artifact_digest"`
	Environment         RunDigestRef      `json:"environment"`
	Fixture             RunDigestRef      `json:"fixture"`
	TestKey             string            `json:"test_key"`
	Project             string            `json:"project,omitempty"`
	Attempts            []RunAttempt      `json:"attempts"`
	Cleanup             string            `json:"cleanup"`
	NegativeControls    []NegativeControl `json:"negative_controls"`
}

type RunRunner struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type RunSource struct {
	Commit string `json:"commit"`
	Tree   string `json:"tree"`
	Clean  bool   `json:"clean"`
}

type RunDigestRef struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

// RunAttempt is one attempt, in run order, with its ordinal starting at 1 (AFU-V1-011, AFU-V1-012).
type RunAttempt struct {
	Ordinal          int             `json:"ordinal"`
	Outcome          string          `json:"outcome"`
	DurationMS       int64           `json:"duration_ms"`
	AssertionAnchors []RunAnchor     `json:"assertion_anchors"`
	Failure          string          `json:"failure,omitempty"`
	Attachments      []RunAttachment `json:"attachments"`
}

type RunAnchor struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}

// RunAttachment is the path the runner recorded, never the attachment's content.
type RunAttachment struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// NegativeControl is a control test with the outcome it must show and the classification it showed.
type NegativeControl struct {
	TestKey  string `json:"test_key"`
	Expected string `json:"expected"`
	Observed string `json:"observed"`
}

// ValidateRunEvidence checks one record's closed shape (AFU-V1-011, AFU-V1-014).
func ValidateRunEvidence(r TestRunEvidence) error {
	if r.Schema != RunEvidenceSchema {
		return errors.New("schema must be " + RunEvidenceSchema)
	}
	if !flowText(r.TestKey) || (r.Project != "" && !flowText(r.Project)) {
		return errors.New("invalid test_key or project")
	}
	if r.Attempts == nil || r.NegativeControls == nil {
		return errors.New("attempts and negative_controls must be arrays")
	}
	if r.Authority == AuthorityStatic {
		return validateStatic(r)
	}
	if r.Authority != AuthorityIngested && r.Authority != AuthorityLocallyObserved {
		return errors.New("authority must be INGESTED, LOCALLY_OBSERVED or STATIC")
	}
	if err := validateRunHeader(r); err != nil {
		return err
	}
	if len(r.Attempts) < 1 || len(r.Attempts) > maxRunAttempts {
		return errors.New("attempt count outside bound")
	}
	for i, a := range r.Attempts {
		if err := validateAttempt(a, i+1); err != nil {
			return fmt.Errorf("attempts[%d]: %v", i, err)
		}
	}
	return validateControls(r.NegativeControls)
}

// validateStatic accepts a source mapping alone: a test key with no run, attempt or control (AFU-V1-014).
func validateStatic(r TestRunEvidence) error {
	bad := errors.New("a STATIC record carries no run, attempt, cleanup or negative control")
	if len(r.Attempts) != 0 || len(r.NegativeControls) != 0 || r.Cleanup != "not-declared" {
		return bad
	}
	if r.RunID != "" || r.Runner != (RunRunner{}) || r.Source != (RunSource{}) || r.BuildArtifactDigest != "" {
		return bad
	}
	if r.Environment != (RunDigestRef{}) || r.Fixture != (RunDigestRef{}) {
		return bad
	}
	return nil
}

func validateRunHeader(r TestRunEvidence) error {
	if !memberPattern.MatchString(r.RunID) {
		return errors.New("invalid run_id")
	}
	if !flowText(r.Runner.Name) || !flowText(r.Runner.Version) {
		return errors.New("invalid runner")
	}
	if !oidPattern.MatchString(r.Source.Commit) || !oidPattern.MatchString(r.Source.Tree) {
		return errors.New("source commit and tree must be full object IDs")
	}
	if !sha256Pattern.MatchString(r.BuildArtifactDigest) {
		return errors.New("invalid build_artifact_digest")
	}
	if !validDigestRef(r.Environment) || !validDigestRef(r.Fixture) {
		return errors.New("invalid environment or fixture")
	}
	if !slices.Contains(cleanupStates, r.Cleanup) {
		return errors.New("cleanup must be done, failed or not-declared")
	}
	return nil
}

func validDigestRef(d RunDigestRef) bool {
	return memberPattern.MatchString(d.ID) && sha256Pattern.MatchString(d.Digest)
}

func validateAttempt(a RunAttempt, ordinal int) error {
	if a.Ordinal != ordinal {
		return errors.New("ordinals must run 1..n in order")
	}
	if !slices.Contains(attemptOutcomes, a.Outcome) {
		return errors.New("outcome must be passed, failed, timedOut, skipped or interrupted")
	}
	if a.DurationMS < 0 {
		return errors.New("duration_ms must not be negative")
	}
	if a.AssertionAnchors == nil || a.Attachments == nil || len(a.AssertionAnchors)+len(a.Attachments) > maxRunLinks {
		return errors.New("assertion_anchors and attachments must be arrays within the link bound")
	}
	if !utf8.ValidString(a.Failure) || strings.ContainsRune(a.Failure, 0) {
		return errors.New("invalid failure detail")
	}
	for _, anchor := range a.AssertionAnchors {
		if !flowText(anchor.Path) || anchor.Line < 1 {
			return errors.New("invalid assertion anchor")
		}
	}
	for _, attachment := range a.Attachments {
		if !flowText(attachment.Name) || !flowText(attachment.Path) {
			return errors.New("invalid attachment")
		}
	}
	return nil
}

func validateControls(controls []NegativeControl) error {
	for _, c := range controls {
		if !flowText(c.TestKey) || !slices.Contains(attemptOutcomes, c.Expected) || !slices.Contains(runResults, c.Observed) {
			return fmt.Errorf("negative control %q: invalid test_key, expected or observed outcome", c.TestKey)
		}
	}
	return nil
}

// EncodeRunEvidence is the canonical encoding and the last step before any write: it validates the
// record, then refuses secret-shaped content and an encoding over the byte bound (AFU-V1-037, AFU-V1-038).
func EncodeRunEvidence(r TestRunEvidence) ([]byte, error) {
	if err := ValidateRunEvidence(r); err != nil {
		return nil, err
	}
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if err := e.Encode(r); err != nil {
		return nil, err
	}
	if b.Len() > MaxBytes {
		return nil, errors.New("run evidence exceeds byte limit")
	}
	if secretscreen.MatchString(b.String()) {
		return nil, errors.New("run evidence contains secret-shaped data")
	}
	return b.Bytes(), nil
}

// DecodeRunEvidence reads one record strictly and accepts only its canonical encoding.
func DecodeRunEvidence(raw []byte) (TestRunEvidence, error) {
	var r TestRunEvidence
	if err := Decode(raw, &r); err != nil {
		return r, err
	}
	canonical, err := EncodeRunEvidence(r)
	if err != nil {
		return r, err
	}
	if !bytes.Equal(canonical, raw) {
		return r, errors.New("run evidence is not canonically encoded")
	}
	return r, nil
}

// tcqReport maps an attempt outcome onto the TCQ report vocabulary of the shared tcq.Flaky rule.
var tcqReport = map[string]string{"passed": tcq.ReportPassed, "failed": tcq.ReportFailed, "timedOut": tcq.ReportError, "skipped": tcq.ReportSkipped, "interrupted": tcq.ReportSkipped}

// Classify is the per-test result of one run: the last attempt's outcome, except that a passed last
// attempt after a failed or timed-out one is flaky, never passed (AFU-V1-013).
func Classify(attempts []RunAttempt) string {
	if len(attempts) == 0 {
		return "not-run"
	}
	statuses := []string{}
	for _, a := range attempts {
		statuses = append(statuses, tcqReport[a.Outcome])
	}
	last := attempts[len(attempts)-1].Outcome
	if last == "passed" && tcq.Flaky(statuses) {
		return "flaky"
	}
	return last
}

// Verified reports whether a record may be shown as verified. A STATIC record never is (AFU-V1-014);
// any other needs a passed classification, cleanup done and every negative control as expected.
func Verified(r TestRunEvidence) bool {
	if r.Authority == AuthorityStatic {
		return false
	}
	if Classify(r.Attempts) != "passed" || r.Cleanup != "done" {
		return false
	}
	for _, c := range r.NegativeControls {
		if c.Observed != c.Expected {
			return false
		}
	}
	return true
}
