package wire

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCAXV0031PairedEvidenceIsClosedAndCanonical checks CAX-V0-031 (#1941):
// a cem-paired-evidence/1 document round-trips through its canonical bytes,
// binds only to the map whose change and hunks it names, and is refused whole
// when it carries an unknown member, non-canonical or unsorted bytes, the
// wrong profile, an invalid range, or a citation that breaks its bounds.
func TestCAXV0031PairedEvidenceIsClosedAndCanonical(t *testing.T) {
	mapBytes, err := os.ReadFile(filepath.Join(committedChanges, firstSealed(t)))
	if err != nil {
		t.Fatal(err)
	}
	stable, err := ParseStable(mapBytes)
	if err != nil {
		t.Fatal(err)
	}
	hunk := stable.Change.Hunks[0].ID
	citation := func(path string, start int64) PairedCitation {
		return PairedCitation{BlobOid: strings.Repeat("b", 40), HunkID: hunk, Path: path, Relation: "test-claim",
			Span: PairedSpan{Start: start, End: start + 8}, SpanSha256: strings.Repeat("c", 64)}
	}
	valid := PairedEvidence{
		Change:    PairedChange{BaseRevision: stable.Change.BaseRevision, PatchSha256: stable.Change.PatchSha256},
		Citations: []PairedCitation{citation("tests/z_test.go", 0), citation("tests/a_test.go", 4)},
		Profile:   PairedEvidenceProfile,
		Range:     PairedRange{BaseRevision: strings.Repeat("1", 40), HeadRevision: strings.Repeat("2", 40), Name: "tests"},
	}
	raw, err := EncodePairedEvidence(valid)
	if err != nil {
		t.Fatal(err)
	}
	document, err := ParsePairedEvidence(raw)
	if err != nil {
		t.Fatal(err)
	}
	if document.Citations[0].Path != "tests/a_test.go" {
		t.Fatalf("citations are not sorted: %+v", document.Citations)
	}
	if again, _ := EncodePairedEvidence(*document); !bytes.Equal(again, raw) {
		t.Fatal("paired evidence does not round-trip")
	}
	if _, err := BindPairedEvidence(document, mapBytes); err != nil {
		t.Fatal(err)
	}

	mutate := func(edit func(*PairedEvidence)) []byte {
		edited := valid
		edited.Citations = append([]PairedCitation(nil), valid.Citations...)
		edit(&edited)
		encoded, err := EncodePairedEvidence(edited)
		if err != nil {
			t.Fatal(err)
		}
		return encoded
	}
	unsorted := bytes.Replace(raw, []byte(`tests/a_test.go`), []byte(`tests/zz_test.go`), 1)
	for name, candidate := range map[string][]byte{
		"unknown member":     bytes.Replace(raw, []byte(`{"change":`), []byte(`{"extra":1,"change":`), 1),
		"non-canonical":      append([]byte(" "), raw...),
		"unsorted":           unsorted,
		"wrong profile":      mutate(func(d *PairedEvidence) { d.Profile = "cem-paired-evidence/2" }),
		"bad range name":     mutate(func(d *PairedEvidence) { d.Range.Name = "Tests" }),
		"base equals head":   mutate(func(d *PairedEvidence) { d.Range.HeadRevision = d.Range.BaseRevision }),
		"short commit":       mutate(func(d *PairedEvidence) { d.Range.BaseRevision = "1234567" }),
		"no citations":       mutate(func(d *PairedEvidence) { d.Citations = nil }),
		"duplicate citation": mutate(func(d *PairedEvidence) { d.Citations[1] = d.Citations[0] }),
		"empty span":         mutate(func(d *PairedEvidence) { d.Citations[0].Span.End = d.Citations[0].Span.Start }),
		"unknown relation":   mutate(func(d *PairedEvidence) { d.Citations[0].Relation = "vibes" }),
		"escaping path":      mutate(func(d *PairedEvidence) { d.Citations[0].Path = "../x_test.go" }),
	} {
		if _, err := ParsePairedEvidence(candidate); err == nil {
			t.Fatalf("%s: paired evidence was accepted", name)
		}
	}
	other := mutate(func(d *PairedEvidence) { d.Change.PatchSha256 = strings.Repeat("d", 64) })
	parsed, err := ParsePairedEvidence(other)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BindPairedEvidence(parsed, mapBytes); err == nil {
		t.Fatal("paired evidence bound to another change")
	}
	stray := mutate(func(d *PairedEvidence) { d.Citations[0].HunkID = "no-such-hunk" })
	if parsed, err = ParsePairedEvidence(stray); err != nil {
		t.Fatal(err)
	}
	if _, err := BindPairedEvidence(parsed, mapBytes); err == nil {
		t.Fatal("paired evidence bound a hunk the map does not hold")
	}
}
