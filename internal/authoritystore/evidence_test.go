package authoritystore

import (
	"bytes"
	"fmt"
	"github.com/Beamfall/corvint/internal/localauthority"
	"sort"
	"strings"
	"testing"
)

func manifestFixture() EvidenceManifest {
	a, b, c := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)
	d := strings.Repeat("d", 64)
	return EvidenceManifest{Profile: EvidenceProfile, RootID: "parser-test", Epoch: "1", Generation: "2", EnrollmentHandle: d, CapsuleSHA256: d, Base: a, Target: b, Tree: c, ObjectFormat: "sha1", ReaderGID: "600", Files: []EvidenceFile{
		{Path: ".git/HEAD", Size: "41", SHA256: d},
		{Path: ".git/config", Size: "38", SHA256: d},
		{Path: ".git/objects/aa/" + a[2:], Size: "12", SHA256: d, OID: a, Kind: "commit", DecodedSize: "0"},
		{Path: ".git/objects/bb/" + b[2:], Size: "12", SHA256: d, OID: b, Kind: "commit", DecodedSize: "0"},
		{Path: ".git/objects/cc/" + c[2:], Size: "12", SHA256: d, OID: c, Kind: "tree", DecodedSize: "0"},
	}}
}
func TestEvidenceManifestRejectsNamespaceAndObjectBudgetDrift(t *testing.T) {
	m := manifestFixture()
	if err := ValidateEvidenceManifest(m); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*EvidenceManifest){
		"alternate":   func(m *EvidenceManifest) { m.Files[2].Path = ".git/objects/info/alternates" },
		"duplicate":   func(m *EvidenceManifest) { m.Files[3] = m.Files[2] },
		"wrong-type":  func(m *EvidenceManifest) { m.Files[2].Kind = "blob" },
		"mixed-width": func(m *EvidenceManifest) { m.Files[2].OID += strings.Repeat("a", 24) },
		"bound":       func(m *EvidenceManifest) { m.Files[2].DecodedSize = "16777216" },
		"metadata":    func(m *EvidenceManifest) { m.Files[0].OID = m.Base },
		"missing":     func(m *EvidenceManifest) { m.Files = m.Files[:4] },
		"reader":      func(m *EvidenceManifest) { m.ReaderGID = "0" },
	} {
		t.Run(name, func(t *testing.T) {
			m := manifestFixture()
			change(&m)
			if ValidateEvidenceManifest(m) == nil {
				t.Fatal("accepted")
			}
		})
	}
}

func TestEvidenceManifestOwnDecodeBoundAndZeroFields(t *testing.T) {
	m := manifestFixture()
	m.Epoch = "0"
	m.Generation = "0"
	for i := 0; i < 600; i++ {
		oid := fmt.Sprintf("%040x", i+1)
		m.Files = append(m.Files, EvidenceFile{Path: ".git/objects/" + oid[:2] + "/" + oid[2:], OID: oid, Kind: "blob", Size: "12", DecodedSize: "0", SHA256: strings.Repeat("d", 64)})
	}
	sort.Slice(m.Files, func(i, j int) bool { return m.Files[i].Path < m.Files[j].Path })
	raw := encode(t, m)
	if len(raw) <= localauthority.MaxReceiptBytes {
		t.Fatal("fixture too small")
	}
	var decoded EvidenceManifest
	if err := DecodeEvidenceManifest(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := ValidateEvidenceManifest(decoded); err != nil {
		t.Fatal(err)
	}
	if localauthority.Decode(raw, &decoded) == nil {
		t.Fatal("receipt bound widened")
	}
	if DecodeEvidenceManifest(bytes.Repeat([]byte(" "), MaxEvidenceManifest+1), &decoded) == nil {
		t.Fatal("manifest wire bound")
	}
	root, floor := rootFixture()
	root.Epoch = "0"
	root.Generation = "0"
	floor.Epoch = "0"
	floor.Generation = "0"
	root.AuthorityGID = m.ReaderGID
	m.RootID = root.RootID
	if _, _, _, err := decodeRoot(encode(t, root), encode(t, floor)); err != nil {
		t.Fatal(err)
	}
	enrollment := localauthority.Enrollment{Binding: localauthority.Binding{Base: m.Base, Target: m.Target, Tree: m.Tree}}
	ref := EvidenceReference{Profile: EvidenceReferenceProfile, EnrollmentHandle: m.EnrollmentHandle, ManifestSHA256: strings.Repeat("d", 64)}
	if err := validateEvidenceBinding(root, m.EnrollmentHandle, enrollment, ref, m); err != nil {
		t.Fatal("zero root binding", err)
	}
}
