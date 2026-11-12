package jstestprovider

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"sort"

	"github.com/Beamfall/corvint/internal/jstestreceipt"
)

// Keep-reporters qualification (PWP-V0-014..018, proposed; V1-1028, GitHub
// #686). The qualification command runs the same external configuration twice,
// replace-only and keep-reporters, on the caller's host and compares what the
// provider observed. The closed record names the kept entries, every config
// input of the keep run (so a change to any file a kept reporter or the config
// loads requires a new qualification) and the runtime it was qualified on. It
// never admits a runtime tuple and never observes what a kept reporter does.

// KeepReportersQualificationProfile is jstestreceipt.KeepReportersQualificationProfile, kept under its old name.
const KeepReportersQualificationProfile = jstestreceipt.KeepReportersQualificationProfile

// KeepReportersQualified is jstestreceipt.KeepReportersQualified, kept under its old name.
const KeepReportersQualified = jstestreceipt.KeepReportersQualified

// KeepReportersNotQualified is jstestreceipt.KeepReportersNotQualified, kept under its old name.
const KeepReportersNotQualified = jstestreceipt.KeepReportersNotQualified

// KeepReportersNotRun is jstestreceipt.KeepReportersNotRun, kept under its old name.
const KeepReportersNotRun = jstestreceipt.KeepReportersNotRun

// MaxKeepReportersQualificationSize is jstestreceipt.MaxKeepReportersQualificationSize, kept under its old name.
const MaxKeepReportersQualificationSize = jstestreceipt.MaxKeepReportersQualificationSize

// KeepReportersQualification is jstestreceipt.KeepReportersQualification, kept under its old name.
type KeepReportersQualification = jstestreceipt.KeepReportersQualification

// QualifyKeepReporters derives the record from a replace-only control run and
// a keep-reporters run of the same configuration (PWP-V0-015). A run that
// returned an error, was cancelled, failed at run level or cannot be encoded
// is incomplete and the verdict is not-run; nothing is inferred from it.
func QualifyKeepReporters(control Receipt, controlErr error, keep Receipt, keepErr error) KeepReportersQualification {
	q := KeepReportersQualification{Profile: KeepReportersQualificationProfile, ConfigInputs: map[string]string{}}
	reasons := map[string]bool{}
	var controlComplete, keepComplete bool
	q.ControlReceiptSHA256, controlComplete = completeRunDigest(control, controlErr, false)
	q.KeepReceiptSHA256, keepComplete = completeRunDigest(keep, keepErr, true)
	if !controlComplete {
		reasons["keep-reporters-control-run-incomplete"] = true
	}
	if !keepComplete {
		reasons["keep-reporters-keep-run-incomplete"] = true
	}
	if keepComplete {
		q.ReceiptProfile, q.RunnerVersion, q.NodeVersion = keep.Profile, keep.Identity.RunnerVersion, keep.Identity.NodeVersion
		if keep.External != nil && keep.External.ProjectReporters != nil && keep.External.ProjectReporters.Entries != nil {
			q.Entries = append([]ProjectReporter{}, keep.External.ProjectReporters.Entries...)
		}
	}
	if controlComplete && keepComplete {
		compareKeepReportersRuns(&q, control, keep, reasons)
	}
	q.Reasons = make([]string, 0, len(reasons))
	for reason := range reasons {
		q.Reasons = append(q.Reasons, reason)
	}
	sort.Strings(q.Reasons)
	q.Verdict = jstestreceipt.KeepReportersVerdict(q.Reasons)
	return q
}

// completeRunDigest is the SHA-256 of the canonical retained document of a
// complete run, and whether the run is complete. A run whose config inputs
// the record could not bind is incomplete.
func completeRunDigest(r Receipt, err error, keep bool) (string, bool) {
	if err != nil || r.External == nil || r.Infrastructure != nil || r.Cancelled || (r.External.ProjectReporters != nil) != keep || len(r.Identity.ConfigInputDigests) > jstestreceipt.KeepReportersMaxConfigInputs {
		return "", false
	}
	for path, digest := range r.Identity.ConfigInputDigests {
		if !filepath.IsAbs(path) || len(path) > 4096 || !jstestreceipt.ReporterDigestPattern.MatchString(digest) {
			return "", false
		}
	}
	data, err := EncodeQualified(r)
	if err != nil {
		return "", false
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), true
}

func compareKeepReportersRuns(q *KeepReportersQualification, control, keep Receipt, reasons map[string]bool) {
	if !jstestreceipt.ProviderFirstConfig(keep.External.ConfigOverride) {
		reasons["keep-reporters-order-unsupported"] = true
	}
	for _, entry := range q.Entries {
		if entry.Module == "unknown" || entry.Options == "unknown" {
			reasons["keep-reporters-entries-unknown"] = true
		}
	}
	if q.Entries == nil {
		reasons["keep-reporters-entries-unknown"] = true
	}
	c, k := control.Identity, keep.Identity
	if control.Profile != keep.Profile || c.RunnerVersion != k.RunnerVersion || c.NodeVersion != k.NodeVersion || c.ConfigFile != k.ConfigFile || c.ConfigDigest != k.ConfigDigest || c.PackageDigest != k.PackageDigest || !jstestreceipt.SameStrings(c.TestFileDigests, k.TestFileDigests) || !jstestreceipt.SameStrings(c.Environment, k.Environment) {
		reasons["keep-reporters-inputs-differ"] = true
	}
	for path, digest := range c.ConfigInputDigests {
		if k.ConfigInputDigests[path] != digest {
			reasons["keep-reporters-inputs-differ"] = true
		}
	}
	for path, digest := range k.ConfigInputDigests {
		q.ConfigInputs[path] = digest
	}
	for _, t := range control.Tests {
		if jstestreceipt.QualifiedUnknown(control, t) {
			reasons["keep-reporters-control-unqualified"] = true
		}
	}
	for _, t := range keep.Tests {
		if jstestreceipt.QualifiedPrerequisitesUnknown(keep, t) {
			reasons["keep-reporters-keep-unqualified"] = true
		}
	}
	if len(control.Tests) == 0 {
		reasons["keep-reporters-no-tests"] = true
	}
	if !sameObservations(control.Tests, keep.Tests) {
		reasons["keep-reporters-observation-differs"] = true
	}
	q.Tests = len(control.Tests)
}

// observationKey identifies one outcome without the test ID, which binds the
// whole identity and so differs once kept reporter modules are config inputs.
func observationKey(t TestOutcome) (string, bool) {
	if t.Project == nil || t.Anchor == nil {
		return "", false
	}
	key, _ := json.Marshal([]any{t.Project.Name, t.FullName, t.Anchor.File, t.Anchor.Line})
	return string(key), true
}

// outcomeObservation is what the provider observed about one outcome; durations and
// failure text are excluded because they legitimately vary between runs.
func outcomeObservation(t TestOutcome) string {
	attempts := make([][]any, 0, len(t.Attempts))
	for _, a := range t.Attempts {
		attempts = append(attempts, []any{a.State, a.Retry, a.FailureKind})
	}
	data, _ := json.Marshal([]any{t.Name, t.State, t.Retries, attempts, t.Project.Browser, t.Project.Device, t.Project.ConfigDigest, t.Project.Use, len(t.Artifacts)})
	return string(data)
}

// sameObservations requires one-to-one outcomes with equal observations.
func sameObservations(control, keep []TestOutcome) bool {
	if len(control) != len(keep) {
		return false
	}
	observed := make(map[string]string, len(control))
	for _, t := range control {
		key, ok := observationKey(t)
		if _, duplicate := observed[key]; !ok || duplicate {
			return false
		}
		observed[key] = outcomeObservation(t)
	}
	for _, t := range keep {
		key, ok := observationKey(t)
		want, found := observed[key]
		if !ok || !found || want != outcomeObservation(t) {
			return false
		}
		delete(observed, key)
	}
	return len(observed) == 0
}

// EncodeKeepReportersQualification emits the canonical record bytes.
func EncodeKeepReportersQualification(q KeepReportersQualification) ([]byte, error) {
	if err := jstestreceipt.KeepReportersQualificationError(q); err != nil {
		return nil, err
	}
	data, err := json.Marshal(q)
	if err != nil {
		return nil, err
	}
	if len(data) >= MaxKeepReportersQualificationSize {
		return nil, errors.New(jstestreceipt.KeepReportersQualificationInvalid)
	}
	if err := jstestreceipt.ScreenQualifiedDocument(data, keepReportersIdentifiers); err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// DecodeKeepReportersQualification accepts only the exact canonical bytes of
// a closed record. Every refusal is the value-free
// keep-reporters-qualification-invalid.
func DecodeKeepReportersQualification(data []byte) (KeepReportersQualification, error) {
	invalid := errors.New(jstestreceipt.KeepReportersQualificationInvalid)
	if len(data) > MaxKeepReportersQualificationSize {
		return KeepReportersQualification{}, invalid
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var q KeepReportersQualification
	if err := decoder.Decode(&q); err != nil {
		return KeepReportersQualification{}, invalid
	}
	canonical, err := EncodeKeepReportersQualification(q)
	if err != nil || !bytes.Equal(canonical, data) {
		return KeepReportersQualification{}, invalid
	}
	return q, nil
}
