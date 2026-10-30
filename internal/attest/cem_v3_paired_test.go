package attest

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

func pairedFixtureDocument(t *testing.T, cem []byte, edit func(*wire.PairedEvidence)) []byte {
	t.Helper()
	stable, err := wire.ParseStable(cem)
	if err != nil {
		t.Fatal(err)
	}
	document := wire.PairedEvidence{
		Change: wire.PairedChange{BaseRevision: stable.Change.BaseRevision, PatchSha256: stable.Change.PatchSha256},
		Citations: []wire.PairedCitation{{BlobOid: strings.Repeat("b", 40), HunkID: stable.Change.Hunks[0].ID,
			Path: "tests/calc_test.go", Relation: "test-claim", Span: wire.PairedSpan{End: 8}, SpanSha256: strings.Repeat("c", 64)}},
		Profile: wire.PairedEvidenceProfile,
		Range:   wire.PairedRange{BaseRevision: strings.Repeat("1", 40), HeadRevision: strings.Repeat("2", 40), Name: "tests"},
	}
	if edit != nil {
		edit(&document)
	}
	raw, err := wire.EncodePairedEvidence(document)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// TestCAXV0034StatementV3SignsPairedRange checks CAX-V0-034 (#1941): a cem/v3
// statement built with a cem-paired-evidence/1 document signs pairedRange as
// the range pins plus the document's SHA-256, verification returns it, a
// statement without the document carries no member, a document bound to
// another change is refused at build time, and a malformed signed member is
// refused at verification.
func TestCAXV0034StatementV3SignsPairedRange(t *testing.T) {
	cem := stableFixture(t)
	publicKey := cemFixtureKey().Public().(ed25519.PublicKey)
	paired := pairedFixtureDocument(t, cem, nil)
	statement, err := CEMStatementV3Paired(stableMapName, cem, fixtureV2Target, CEMCeilings{}, paired)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(paired)
	want := CEMPairedRange{Name: "tests", BaseRevision: strings.Repeat("1", 40), HeadRevision: strings.Repeat("2", 40),
		EvidenceSha256: hex.EncodeToString(digest[:])}
	envelope, _ := signedFixtureStatement(t, statement)
	got, err := VerifyCEMPredicate(envelope, publicKey, cem)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != CEMVerified || got.V2 == nil || got.V2.Paired == nil || *got.V2.Paired != want {
		t.Fatalf("verification = %#v (%#v)", got, got.V2)
	}
	plain, _ := signedCEMV3(t, cem, CEMCeilings{})
	if strings.Contains(string(plain), "pairedRange") {
		t.Fatal("a cem/v3 statement without paired evidence carries pairedRange")
	}
	other := pairedFixtureDocument(t, cem, func(d *wire.PairedEvidence) { d.Change.PatchSha256 = strings.Repeat("d", 64) })
	if _, err := CEMStatementV3Paired(stableMapName, cem, fixtureV2Target, CEMCeilings{}, other); err == nil {
		t.Fatal("paired evidence bound to another change was signed")
	}
	for name, edit := range map[string]func(string) string{
		"short head":       func(s string) string { return strings.Replace(s, strings.Repeat("2", 40), "2222222", 1) },
		"bad name":         func(s string) string { return strings.Replace(s, `"name":"tests"`, `"name":"Tests"`, 1) },
		"short digest":     func(s string) string { return strings.Replace(s, want.EvidenceSha256, "abc", 1) },
		"unknown member":   func(s string) string { return strings.Replace(s, `"pairedRange":{`, `"pairedRange":{"extra":1,`, 1) },
		"base equals head": func(s string) string { return strings.Replace(s, strings.Repeat("2", 40), strings.Repeat("1", 40), 1) },
	} {
		edited := edit(string(statement))
		if edited == string(statement) {
			t.Fatalf("%s: the edit changed nothing", name)
		}
		forged, _ := signedFixtureStatement(t, []byte(edited))
		if _, err := VerifyCEMPredicate(forged, publicKey, cem); err == nil {
			t.Errorf("%s: a malformed pairedRange verified", name)
		}
	}
}
