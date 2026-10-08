package jstestprovider

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"sort"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// Keep-reporters qualification (PWP-V0-014..018, proposed; V1-1028, GitHub
// #686). The qualification command runs the same external configuration twice,
// replace-only and keep-reporters, on the caller's host and compares what the
// provider observed. The closed record names the kept entries, every config
// input of the keep run (so a change to any file a kept reporter or the config
// loads requires a new qualification) and the runtime it was qualified on. It
// never admits a runtime tuple and never observes what a kept reporter does.

const (
	// KeepReportersQualificationProfile identifies the closed record.
	KeepReportersQualificationProfile = "corvint-playwright-keep-reporters-qualification/0"

	KeepReportersQualified    = "qualified"
	KeepReportersNotQualified = "not-qualified"
	KeepReportersNotRun       = "not-run"

	keepReportersQualificationInvalid = "keep-reporters-qualification-invalid"
	// MaxKeepReportersQualificationSize bounds the canonical record bytes.
	MaxKeepReportersQualificationSize = 256 << 10
)

// keepReportersQualificationReasons is the closed reason set and the verdict
// each reason forces. A not-run reason outranks every not-qualified reason.
var keepReportersQualificationReasons = map[string]string{
	"keep-reporters-control-run-incomplete": KeepReportersNotRun,
	"keep-reporters-keep-run-incomplete":    KeepReportersNotRun,
	"keep-reporters-control-unqualified":    KeepReportersNotQualified,
	"keep-reporters-keep-unqualified":       KeepReportersNotQualified,
	"keep-reporters-order-unsupported":      KeepReportersNotQualified,
	"keep-reporters-entries-unknown":        KeepReportersNotQualified,
	"keep-reporters-inputs-differ":          KeepReportersNotQualified,
	"keep-reporters-observation-differs":    KeepReportersNotQualified,
	"keep-reporters-no-tests":               KeepReportersNotQualified,
}

// KeepReportersQualification is the closed canonical qualification record.
// Reasons is empty exactly when Verdict is qualified. Entries is null when the
// keep run never reported them. ConfigInputs are every reporter-observed
// config input of the keep run, including the kept reporter modules and
// everything they loaded, whether or not the control run loaded it too.
type KeepReportersQualification struct {
	Profile              string            `json:"profile"`
	Verdict              string            `json:"verdict"`
	Reasons              []string          `json:"reasons"`
	ReceiptProfile       string            `json:"receiptProfile"`
	RunnerVersion        string            `json:"runnerVersion"`
	NodeVersion          string            `json:"nodeVersion"`
	Entries              []ProjectReporter `json:"entries"`
	ConfigInputs         map[string]string `json:"configInputs"`
	Tests                int               `json:"tests"`
	ControlReceiptSHA256 string            `json:"controlReceiptSha256"`
	KeepReceiptSHA256    string            `json:"keepReceiptSha256"`
}

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
	q.Verdict = keepReportersVerdict(q.Reasons)
	return q
}

// completeRunDigest is the SHA-256 of the canonical retained document of a
// complete run, and whether the run is complete. A run whose config inputs
// the record could not bind is incomplete.
func completeRunDigest(r Receipt, err error, keep bool) (string, bool) {
	if err != nil || r.External == nil || r.Infrastructure != nil || r.Cancelled || (r.External.ProjectReporters != nil) != keep || len(r.Identity.ConfigInputDigests) > externalMaxConfigInputs {
		return "", false
	}
	for path, digest := range r.Identity.ConfigInputDigests {
		if !filepath.IsAbs(path) || len(path) > 4096 || !reporterDigestPattern.MatchString(digest) {
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
	if !providerFirstConfig(keep.External.ConfigOverride) {
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
	if control.Profile != keep.Profile || c.RunnerVersion != k.RunnerVersion || c.NodeVersion != k.NodeVersion || c.ConfigFile != k.ConfigFile || c.ConfigDigest != k.ConfigDigest || c.PackageDigest != k.PackageDigest || !sameStrings(c.TestFileDigests, k.TestFileDigests) || !sameStrings(c.Environment, k.Environment) {
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
		if qualifiedUnknown(control, t) {
			reasons["keep-reporters-control-unqualified"] = true
		}
	}
	for _, t := range keep.Tests {
		if qualifiedPrerequisitesUnknown(keep, t) {
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

func sameStrings(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if other, ok := b[key]; !ok || other != value {
			return false
		}
	}
	return true
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

// observation is what the provider observed about one outcome; durations and
// failure text are excluded because they legitimately vary between runs.
func observation(t TestOutcome) string {
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
		observed[key] = observation(t)
	}
	for _, t := range keep {
		key, ok := observationKey(t)
		want, found := observed[key]
		if !ok || !found || want != observation(t) {
			return false
		}
		delete(observed, key)
	}
	return len(observed) == 0
}

func keepReportersVerdict(reasons []string) string {
	verdict := KeepReportersQualified
	for _, reason := range reasons {
		switch keepReportersQualificationReasons[reason] {
		case KeepReportersNotRun:
			return KeepReportersNotRun
		case KeepReportersNotQualified:
			verdict = KeepReportersNotQualified
		}
	}
	return verdict
}

// keepReportersQualificationError validates the closed record (PWP-V0-017).
func keepReportersQualificationError(q KeepReportersQualification) error {
	invalid := errors.New(keepReportersQualificationInvalid)
	if q.Profile != KeepReportersQualificationProfile || q.Reasons == nil || q.ConfigInputs == nil || q.Tests < 0 || len(q.Entries) > maxProjectReporters || len(q.ConfigInputs) > externalMaxConfigInputs {
		return invalid
	}
	if !sort.StringsAreSorted(q.Reasons) || keepReportersVerdict(q.Reasons) != q.Verdict {
		return invalid
	}
	for i, reason := range q.Reasons {
		if keepReportersQualificationReasons[reason] == "" || (i > 0 && q.Reasons[i-1] == reason) {
			return invalid
		}
	}
	if len(q.ReceiptProfile) > 128 || len(q.RunnerVersion) > 128 || len(q.NodeVersion) > 128 {
		return invalid
	}
	for _, digest := range []string{q.ControlReceiptSHA256, q.KeepReceiptSHA256} {
		if digest != "" && !reporterDigestPattern.MatchString(digest) {
			return invalid
		}
	}
	for path, digest := range q.ConfigInputs {
		if !filepath.IsAbs(path) || len(path) > 4096 || !reporterDigestPattern.MatchString(digest) {
			return invalid
		}
	}
	for _, entry := range q.Entries {
		if !validProjectReporter(entry) {
			return invalid
		}
	}
	if q.Verdict != KeepReportersQualified {
		return nil
	}
	if !isExternalProfile(q.ReceiptProfile) || q.RunnerVersion == "" || q.NodeVersion == "" || q.Entries == nil || q.Tests < 1 || q.ControlReceiptSHA256 == "" || q.KeepReceiptSHA256 == "" {
		return invalid
	}
	for _, entry := range q.Entries {
		if entry.Module == "unknown" || entry.Options == "unknown" || (entry.Module == "bound" && q.ConfigInputs[entry.Name] != entry.ModuleDigest) {
			return invalid
		}
	}
	return nil
}

// EncodeKeepReportersQualification emits the canonical record bytes.
func EncodeKeepReportersQualification(q KeepReportersQualification) ([]byte, error) {
	if err := keepReportersQualificationError(q); err != nil {
		return nil, err
	}
	data, err := json.Marshal(q)
	if err != nil {
		return nil, err
	}
	if len(data) >= MaxKeepReportersQualificationSize {
		return nil, errors.New(keepReportersQualificationInvalid)
	}
	if secretscreen.MatchString(string(data)) {
		return nil, errors.New("qualified-document-secret-shaped")
	}
	return append(data, '\n'), nil
}

// DecodeKeepReportersQualification accepts only the exact canonical bytes of
// a closed record. Every refusal is the value-free
// keep-reporters-qualification-invalid.
func DecodeKeepReportersQualification(data []byte) (KeepReportersQualification, error) {
	invalid := errors.New(keepReportersQualificationInvalid)
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

// keepReportersQualified reports a keep-reporters receipt whose carried
// qualification matches it exactly (PWP-V0-016): provider-first order, every
// entry known, the same profile, runner and Node versions, entries, and exactly
// the qualified config inputs: no input changed, added or removed.
func keepReportersQualified(r Receipt) bool {
	p := r.External.ProjectReporters
	q := p.Qualification
	if q == nil || q.Verdict != KeepReportersQualified || keepReportersQualificationError(*q) != nil || !providerFirstConfig(r.External.ConfigOverride) {
		return false
	}
	if p.Entries == nil || len(p.Entries) != len(q.Entries) || q.ReceiptProfile != r.Profile || q.RunnerVersion != r.Identity.RunnerVersion || q.NodeVersion != r.Identity.NodeVersion {
		return false
	}
	for i, entry := range p.Entries {
		if entry != q.Entries[i] {
			return false
		}
	}
	return sameStrings(r.Identity.ConfigInputDigests, q.ConfigInputs)
}
