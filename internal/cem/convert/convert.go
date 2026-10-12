// Package convert is the deterministic cem/0.x -> cem/1.0 map converter
// (V1-1275, decision 0525 §4, CEM-V1-096..105). Its output is a function of the
// source map bytes and the bind commit alone; the only Git read is the
// report-side source binding check, behind SourceReader.
package convert

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// Converter is the provenance converter string (CEM-V1-098).
const Converter = "corvint-cem-convert/1"

// ReportProfile names the closed report object (CEM-V1-102).
const ReportProfile = "corvint-cem-convert-report/1"

// Refusal codes owned by CEM-V1-101. Source-map wire refusals keep the 0.x
// reader's own code.
const (
	CodeSourceIsStable        = "convert-source-is-stable"
	CodeEmptyHunks            = "convert-empty-hunks"
	CodeBindCommitUnavailable = "convert-bind-commit-unavailable"
	CodeSourceBindingMismatch = "convert-source-binding-mismatch"
	CodeEncodeRefused         = "convert-encode-refused"
)

// Source binding states (CEM-V1-099).
const (
	BindingExact       = "EXACT"
	BindingAbsent      = "ABSENT"
	BindingUnavailable = "UNAVAILABLE"
	BindingMismatch    = "MISMATCH"
	BindingNotChecked  = "NOT_CHECKED"
)

// SourceReader is the one Git read the converter makes: the blob the bind
// commit holds at the excluded sidecar path. present is false when the commit
// resolves but holds no regular file there; err means the commit or its trees
// could not be read.
type SourceReader interface {
	SidecarBlob(commit string) (oid string, present bool, err error)
}

// Inputs are the converter's non-byte inputs. BindCommit is required (40
// lowercase hex). Git may be nil, in which case the binding is NOT_CHECKED.
type Inputs struct {
	BindCommit string
	Git        SourceReader
}

// Report is the closed corvint-cem-convert-report/1 object.
type Report struct {
	Profile       string                 `json:"profile"`
	Status        string                 `json:"status"`
	Code          *string                `json:"code"`
	Converter     string                 `json:"converter"`
	SourceSpec    *string                `json:"sourceSpec"`
	SourceBinding string                 `json:"sourceBinding"`
	Provenance    *wire.StableProvenance `json:"provenance"`
	HunkCount     int                    `json:"hunkCount"`
	EvidenceCount int                    `json:"evidenceCount"`
	OutputSha256  *string                `json:"outputSha256"`
}

func newReport() Report {
	return Report{Profile: ReportProfile, Status: "REFUSED", Converter: Converter, SourceBinding: BindingNotChecked}
}

func (r *Report) refuse(code string, cause error) error {
	r.Status = "REFUSED"
	r.Code = &code
	r.Provenance = nil
	r.OutputSha256 = nil
	if cause == nil {
		return cemcode.New(code, "%s", code)
	}
	return &cemcode.Error{Code: code, Message: cause.Error()}
}

// BindCommitFromName returns the bind commit a corpus file name pins
// (`<40 lowercase hex>.cem.json`), or "" when the name pins none.
func BindCommitFromName(name string) string {
	base := path.Base(strings.ReplaceAll(name, "\\", "/"))
	commit, ok := strings.CutSuffix(base, ".cem.json")
	if !ok || !sha1Hex(commit) {
		return ""
	}
	return commit
}

// BlobOid is the Git SHA-1 blob object name of exactly b.
func BlobOid(b []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(b))
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

// Convert maps the exact bytes of a cem/0.1, cem/0.2 or cem/0.3 map to the
// EncodeStable bytes of the cem/1.0 map CEM-V1-097 defines. It never invents a
// member: anything it cannot derive from src and in.BindCommit is a refusal.
func Convert(src []byte, in Inputs) ([]byte, Report, error) {
	r := newReport()
	if wire.DeclaresStable(src) {
		spec := wire.StableSpec
		r.SourceSpec = &spec
		return nil, r, r.refuse(CodeSourceIsStable, nil)
	}
	source, err := wire.ParseMap(src)
	if err != nil {
		code := cemcode.CodeOf(err)
		if code == "" {
			code = cemcode.InvalidJSON
		}
		return nil, r, r.refuse(code, err)
	}
	r.SourceSpec = &source.Spec
	r.HunkCount, r.EvidenceCount = len(source.Hunks), len(source.Evidence)
	if len(source.Hunks) == 0 {
		return nil, r, r.refuse(CodeEmptyHunks, nil)
	}
	if !sha1Hex(in.BindCommit) {
		return nil, r, r.refuse(CodeBindCommitUnavailable, nil)
	}
	blob := BlobOid(src)
	digest := sha256.Sum256(src)
	if in.Git != nil {
		oid, present, gitErr := in.Git.SidecarBlob(in.BindCommit)
		switch {
		case gitErr != nil:
			r.SourceBinding = BindingUnavailable
		case !present:
			r.SourceBinding = BindingAbsent
		case oid == blob:
			r.SourceBinding = BindingExact
		default:
			r.SourceBinding = BindingMismatch
			return nil, r, r.refuse(CodeSourceBindingMismatch, nil)
		}
	}
	change := wire.StableChange(*source)
	change.Spec = wire.StableSpec
	change.ExcludedPath = wire.ExcludedCEMPath
	stable := &wire.StableMap{
		Change:    change,
		Criteria:  []wire.StableCriterion{},
		Receipts:  []wire.StableReceipt{},
		Links:     []wire.StableLink{},
		Artifacts: []wire.StableArtifact{},
		Provenance: &wire.StableProvenance{
			SourceSpec:    source.Spec,
			SourceBlobOid: blob,
			SourceSha256:  hex.EncodeToString(digest[:]),
			BindCommit:    in.BindCommit,
			Converter:     Converter,
		},
	}
	out, err := wire.EncodeStable(stable)
	if err != nil {
		return nil, r, r.refuse(CodeEncodeRefused, errors.Join(errors.New(cemcode.CodeOf(err)), err))
	}
	sum := sha256.Sum256(out)
	outSha := hex.EncodeToString(sum[:])
	r.Status = "CONVERTED"
	r.Provenance = stable.Provenance
	r.OutputSha256 = &outSha
	return out, r, nil
}

func sha1Hex(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
