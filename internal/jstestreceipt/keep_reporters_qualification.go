package jstestreceipt

import (
	"errors"
	"path/filepath"
	"sort"
)

const (
	// KeepReportersQualificationProfile identifies the closed record.
	KeepReportersQualificationProfile = "corvint-playwright-keep-reporters-qualification/0"

	KeepReportersQualified    = "qualified"
	KeepReportersNotQualified = "not-qualified"
	KeepReportersNotRun       = "not-run"

	KeepReportersQualificationInvalid = "keep-reporters-qualification-invalid"
	// MaxKeepReportersQualificationSize bounds the canonical record bytes.
	MaxKeepReportersQualificationSize = 256 << 10
	// KeepReportersMaxConfigInputs keeps the record's own 256-input bound
	// (PWP-V0-017), which fits MaxKeepReportersQualificationSize; it does not
	// follow the receipt's raised externalMaxConfigInputs (PWP-V0-021).
	KeepReportersMaxConfigInputs = 256
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

func SameStrings(a, b map[string]string) bool {
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

func KeepReportersVerdict(reasons []string) string {
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

// KeepReportersQualificationError validates the closed record (PWP-V0-017).
func KeepReportersQualificationError(q KeepReportersQualification) error {
	invalid := errors.New(KeepReportersQualificationInvalid)
	if q.Profile != KeepReportersQualificationProfile || q.Reasons == nil || q.ConfigInputs == nil || q.Tests < 0 || len(q.Entries) > MaxProjectReporters || len(q.ConfigInputs) > KeepReportersMaxConfigInputs {
		return invalid
	}
	if !sort.StringsAreSorted(q.Reasons) || KeepReportersVerdict(q.Reasons) != q.Verdict {
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
		if digest != "" && !ReporterDigestPattern.MatchString(digest) {
			return invalid
		}
	}
	for path, digest := range q.ConfigInputs {
		if !filepath.IsAbs(path) || len(path) > 4096 || !ReporterDigestPattern.MatchString(digest) {
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
	if !IsExternalProfile(q.ReceiptProfile) || q.RunnerVersion == "" || q.NodeVersion == "" || q.Entries == nil || q.Tests < 1 || q.ControlReceiptSHA256 == "" || q.KeepReceiptSHA256 == "" {
		return invalid
	}
	for _, entry := range q.Entries {
		if entry.Module == "unknown" || entry.Options == "unknown" || (entry.Module == "bound" && q.ConfigInputs[entry.Name] != entry.ModuleDigest) {
			return invalid
		}
	}
	return nil
}

// keepReportersQualified reports a keep-reporters receipt whose carried
// qualification matches it exactly (PWP-V0-016): provider-first order, every
// entry known, the same profile, runner and Node versions, entries, and exactly
// the qualified config inputs: no input changed, added or removed.
func keepReportersQualified(r Receipt) bool {
	p := r.External.ProjectReporters
	q := p.Qualification
	if q == nil || q.Verdict != KeepReportersQualified || KeepReportersQualificationError(*q) != nil || !ProviderFirstConfig(r.External.ConfigOverride) {
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
	return SameStrings(r.Identity.ConfigInputDigests, q.ConfigInputs)
}
