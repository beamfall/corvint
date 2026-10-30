package authoritystore

import (
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/localauthority"
)

const EvidenceProfile = "corvint-protected-git-evidence/0"
const EvidenceReferenceProfile = "corvint-protected-git-reference/0"
const MaxEvidenceObjects = 1024
const MaxEvidenceBytes = 16 << 20
const MaxEvidenceManifest = 1 << 20

// EvidenceReference contains no source bytes. The protected public publication
// binds the separately restricted evidence directory and its complete manifest.
type EvidenceReference struct {
	Profile          string `json:"profile"`
	EnrollmentHandle string `json:"enrollmentHandle"`
	ManifestSHA256   string `json:"manifestSHA256"`
}

type EvidenceFile struct {
	Path        string `json:"path"`
	Size        string `json:"size"`
	SHA256      string `json:"sha256"`
	OID         string `json:"oid,omitempty"`
	Kind        string `json:"kind,omitempty"`
	DecodedSize string `json:"decodedSize,omitempty"`
}

type EvidenceManifest struct {
	Profile          string         `json:"profile"`
	RootID           string         `json:"rootId"`
	Epoch            string         `json:"epoch"`
	Generation       string         `json:"generation"`
	EnrollmentHandle string         `json:"enrollmentHandle"`
	CapsuleSHA256    string         `json:"capsuleSHA256"`
	Base             string         `json:"base"`
	Target           string         `json:"target"`
	Tree             string         `json:"tree"`
	ObjectFormat     string         `json:"objectFormat"`
	ReaderGID        string         `json:"readerGid"`
	Files            []EvidenceFile `json:"files"`
}

func evidenceOID(value string, width int) bool {
	if len(value) != width {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// ValidateEvidenceManifest checks a closed deterministic namespace. It cannot
// admit a reader, authenticate ownership or substitute for the file audit.
func ValidateEvidenceManifest(m EvidenceManifest) error {
	width := 40
	if m.ObjectFormat == "sha256" {
		width = 64
	} else if m.ObjectFormat != "sha1" {
		return errUnavailable
	}
	if m.Profile != EvidenceProfile || m.RootID == "" || !hexDigest(m.EnrollmentHandle) || !hexDigest(m.CapsuleSHA256) || !evidenceOID(m.Base, width) || !evidenceOID(m.Target, width) || !evidenceOID(m.Tree, width) {
		return errUnavailable
	}
	for _, value := range []string{m.Epoch, m.Generation} {
		if _, ok := decimal(value); !ok {
			return errUnavailable
		}
	}
	if gid, ok := decimal(m.ReaderGID); !ok || gid == 0 || gid > 65535 {
		return errUnavailable
	}
	if len(m.Files) < 3 || len(m.Files) > MaxEvidenceObjects+2 || !sort.SliceIsSorted(m.Files, func(i, j int) bool { return m.Files[i].Path < m.Files[j].Path }) {
		return errUnavailable
	}
	count, total := 0, uint64(0)
	seen := map[string]bool{}
	objects := map[string]string{}
	for _, f := range m.Files {
		if seen[f.Path] || path.Clean(f.Path) != f.Path || strings.HasPrefix(f.Path, "/") || !hexDigest(f.SHA256) {
			return errUnavailable
		}
		seen[f.Path] = true
		n, ok := decimal(f.Size)
		if !ok || n > MaxEvidenceBytes+65536 {
			return errUnavailable
		}
		if f.Path == ".git/HEAD" || f.Path == ".git/config" {
			if f.OID != "" || f.Kind != "" || f.DecodedSize != "" || n > 1024 {
				return errUnavailable
			}
			continue
		}
		if !evidenceOID(f.OID, width) || f.Path != ".git/objects/"+f.OID[:2]+"/"+f.OID[2:] || (f.Kind != "commit" && f.Kind != "tree" && f.Kind != "blob") {
			return errUnavailable
		}
		size, ok := decimal(f.DecodedSize)
		if !ok || size > MaxEvidenceBytes {
			return errUnavailable
		}
		total += uint64(len(f.Kind)+1+len(strconv.FormatUint(size, 10))+1) + size
		if total > MaxEvidenceBytes {
			return errUnavailable
		}
		count++
		objects[f.OID] = f.Kind
	}
	if count == 0 || count > MaxEvidenceObjects || !seen[".git/HEAD"] || !seen[".git/config"] || objects[m.Base] != "commit" || objects[m.Target] != "commit" || objects[m.Tree] != "tree" {
		return errUnavailable
	}
	return nil
}

func validateEvidenceBinding(root RootDocument, handle string, enrollment localauthority.Enrollment, ref EvidenceReference, m EvidenceManifest) error {
	if ValidateEvidenceManifest(m) != nil || ref.Profile != EvidenceReferenceProfile || ref.EnrollmentHandle != handle || !hexDigest(ref.ManifestSHA256) || m.EnrollmentHandle != handle || m.RootID != root.RootID || m.Epoch != root.Epoch || m.Generation != root.Generation || m.ReaderGID != root.AuthorityGID || m.Base != enrollment.Binding.Base || m.Target != enrollment.Binding.Target || m.Tree != enrollment.Binding.Tree {
		return errUnavailable
	}
	return nil
}

func DecodeEvidenceManifest(raw []byte, result *EvidenceManifest) error {
	return localauthority.DecodeDocument(raw, result, MaxEvidenceManifest)
}
