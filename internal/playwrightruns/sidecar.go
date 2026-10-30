package playwrightruns

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
)

// Identity sidecars attach a corvintRunIdentity declaration to a report that does not embed one
// (PWR-V0-064..067, GitHub #1896).
const (
	SidecarProfile  = "corvint-run-identity-sidecar/0"
	MaxSidecarBytes = 8 << 20

	IdentityEmbedded    = "EMBEDDED"
	IdentitySidecar     = "SIDECAR"
	IdentityNotDeclared = "NOT_DECLARED"
)

// Sidecar refusal reasons; each error's text is its reason code (PWR-V0-064, PWR-V0-066).
var (
	ErrSidecarUnreadable = errors.New("IDENTITY_SIDECAR_UNREADABLE")
	ErrSidecarTooLarge   = errors.New("IDENTITY_SIDECAR_BOUND_EXCEEDED")
	ErrSidecarInvalid    = errors.New("IDENTITY_SIDECAR_INVALID")
	ErrSidecarMismatch   = errors.New("IDENTITY_SIDECAR_MISMATCH")
	ErrSidecarConflict   = errors.New("IDENTITY_SIDECAR_CONFLICT")
)

// ReadSidecarFile reads one sidecar's bytes: only a regular file, without blocking on a FIFO, and
// at most MaxSidecarBytes (PWR-V0-064).
func ReadSidecarFile(name string) ([]byte, error) {
	f, err := openNonblocking(name)
	if err != nil {
		return nil, ErrSidecarUnreadable
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, ErrSidecarUnreadable
	}
	raw, err := io.ReadAll(io.LimitReader(f, MaxSidecarBytes+1))
	if err != nil {
		return nil, ErrSidecarUnreadable
	}
	if len(raw) > MaxSidecarBytes {
		return nil, ErrSidecarTooLarge
	}
	return raw, nil
}

// MarkIdentitySource states where run's declaration came from once the caller asked for it
// (PWR-V0-067): EMBEDDED when the report carries a corvintRunIdentity member, else NOT_DECLARED.
// A source already stated is kept.
func MarkIdentitySource(run *Run) {
	if run.IdentitySource != "" {
		return
	}
	run.IdentitySource = IdentityNotDeclared
	if run.identityEmbedded {
		run.IdentitySource = IdentityEmbedded
	}
}

// ApplySidecar binds the sidecar raw to run (PWR-V0-064, PWR-V0-066): the sidecar must name the
// report's SHA-256, and a report that embeds a declaration keeps it, refusing a sidecar whose
// declaration differs. On success the run states identitySource SIDECAR, or EMBEDDED for an equal
// repeat of the embedded declaration. The report's corvintRunCategory is never replaced.
func ApplySidecar(run *Run, raw []byte) error {
	if len(raw) > MaxSidecarBytes {
		return ErrSidecarTooLarge
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var doc struct {
		Profile      *string         `json:"profile"`
		ReportSha256 *string         `json:"reportSha256"`
		Identity     json.RawMessage `json:"corvintRunIdentity"`
	}
	if err := dec.Decode(&doc); err != nil {
		return ErrSidecarInvalid
	}
	if _, err := dec.Token(); err != io.EOF {
		return ErrSidecarInvalid
	}
	if doc.Profile == nil || *doc.Profile != SidecarProfile || doc.ReportSha256 == nil || !validHex64(*doc.ReportSha256) ||
		len(doc.Identity) == 0 || doc.Identity[0] != '{' {
		return ErrSidecarInvalid
	}
	if *doc.ReportSha256 != run.Sha256 {
		return ErrSidecarMismatch
	}
	meta, err := json.Marshal(map[string]json.RawMessage{"corvintRunIdentity": doc.Identity})
	if err != nil {
		return ErrSidecarInvalid
	}
	inputs, declared := declaredInputs(meta)
	testRevision, testRootPath, _ := declaredRunFacts(meta)
	if run.identityEmbedded {
		if declared != run.InputsDeclared || !reflect.DeepEqual(inputs, run.Inputs) ||
			testRevision != run.TestRevision || testRootPath != run.TestRootPath {
			return ErrSidecarConflict
		}
		run.IdentitySource = IdentityEmbedded
		return nil
	}
	run.Inputs, run.InputsDeclared = inputs, declared
	run.TestRevision, run.TestRootPath = testRevision, testRootPath
	run.IdentitySource = IdentitySidecar
	return nil
}
