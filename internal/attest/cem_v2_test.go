package attest

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// stableCase is the public receipt-authentication stable case; its artifact
// paths are relative to this directory.
const stableCase = "../../protocol/cem-1.0/stable/receipt-authentication/"

const stableMapName = "stable.json"

// fixtureV2Target is a well-formed commit and tree pair; the statement only
// carries it.
var fixtureV2Target = CEMTarget{
	GitCommit: "1f181cd0843318b72bb6679cd19878c46eb95ada",
	GitTree:   "4b825dc642cb6eb9a060e54bf8d69288fbbc4904",
}

func stableFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(stableCase + stableMapName)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// caseArtifact reads a declared artifact from the case directory, or from
// overrides when present.
func caseArtifact(overrides map[string][]byte) func(string) ([]byte, error) {
	return func(name string) ([]byte, error) {
		if data, ok := overrides[name]; ok {
			return data, nil
		}
		return os.ReadFile(stableCase + path.Clean(name))
	}
}

// cemFixtureKey is a public, test-only Ed25519 key derived from a fixed seed
// so the fixture envelopes are reproducible; it signs nothing else.
func cemFixtureKey() ed25519.PrivateKey {
	seed := sha256.Sum256([]byte("corvint FPK-V0-033 fixture key"))
	return ed25519.NewKeyFromSeed(seed[:])
}

func signedFixtureStatement(t *testing.T, statement []byte) ([]byte, ed25519.PublicKey) {
	t.Helper()
	privateKey := cemFixtureKey()
	envelope, err := Envelope(statement, privateKey)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	return envelope, privateKey.Public().(ed25519.PublicKey)
}

func signedCEMV2(t *testing.T, cem []byte) ([]byte, []byte) {
	t.Helper()
	statement, err := CEMStatementV2(stableMapName, cem, fixtureV2Target)
	if err != nil {
		t.Fatalf("CEMStatementV2: %v", err)
	}
	envelope, _ := signedFixtureStatement(t, statement)
	return statement, envelope
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// TestCEMV1065StatementBytesAndSubjectOrder checks CEM-V1-065: the statement
// is deterministic canonical JSON whose first subject is the exact map bytes
// and whose further subjects are the declared artifacts in map order, under a
// predicate of exactly baseRevision, patchSha256 and target.
func TestCEMV1065StatementBytesAndSubjectOrder(t *testing.T) {
	cem := stableFixture(t)
	document, err := wire.ParseStable(cem)
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Artifacts) != 3 {
		t.Fatalf("fixture declares %d artifacts, want 3", len(document.Artifacts))
	}
	subjects := `{"digest":{"sha256":"` + sha256Hex(cem) + `"},"name":"stable.json"}`
	for _, artifact := range document.Artifacts {
		subjects += `,{"digest":{"sha256":"` + artifact.Sha256 + `"},"name":"` + artifact.Path + `"}`
	}
	want := `{"_type":"https://in-toto.io/Statement/v1","predicate":{"baseRevision":"` + document.Change.BaseRevision +
		`","patchSha256":"` + document.Change.PatchSha256 + `","target":{"gitCommit":"` + fixtureV2Target.GitCommit +
		`","gitTree":"` + fixtureV2Target.GitTree + `"}},"predicateType":"https://corvint-context.dev/attestation/cem/v2","subject":[` +
		subjects + `]}`
	first, _ := signedCEMV2(t, cem)
	second, _ := signedCEMV2(t, cem)
	if string(first) != want || !bytes.Equal(first, second) {
		t.Fatalf("CEMStatementV2 =\n%s\nwant\n%s", first, want)
	}
	for _, bad := range []CEMTarget{{}, {GitCommit: fixtureV2Target.GitCommit}, {GitCommit: fixtureV2Target.GitCommit, GitTree: strings.ToUpper(fixtureV2Target.GitTree)}} {
		if _, err := CEMStatementV2(stableMapName, cem, bad); err == nil {
			t.Errorf("CEMStatementV2 accepted target %+v", bad)
		}
	}
	if _, err := CEMStatementV2("", cem, fixtureV2Target); err == nil {
		t.Error("CEMStatementV2 accepted an empty name")
	}
	if _, err := CEMStatementV2("artifacts/runner-plan.json", cem, fixtureV2Target); err == nil {
		t.Error("CEMStatementV2 accepted a map name that repeats an artifact subject")
	}
}

// TestCEMV1065VerifiesMapArtifactsAndTarget checks CEM-V1-065: with the map,
// an artifact reader and the target, a v2 envelope verifies all three, and
// without them each check is reported NOT_RUN.
func TestCEMV1065VerifiesMapArtifactsAndTarget(t *testing.T) {
	cem := stableFixture(t)
	_, envelope := signedCEMV2(t, cem)
	publicKey := cemFixtureKey().Public().(ed25519.PublicKey)
	target := fixtureV2Target
	got, err := VerifyCEMPredicateWith(envelope, publicKey, CEMCheck{Map: cem, Artifact: caseArtifact(nil), Target: &target})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != CEMVerified || got.PredicateType != CEMPredicateTypeV2 || got.Spec != wire.StableSpec ||
		got.Name != stableMapName || got.Size != int64(len(cem)) || got.V2 == nil ||
		got.V2.ArtifactsStatus != CEMVerified || got.V2.TargetStatus != CEMVerified || len(got.V2.Artifacts) != 3 ||
		got.V2.Artifacts[0].Name != "artifacts/runner-plan.json" || got.V2.Target != target {
		t.Fatalf("verification = %#v (%#v)", got, got.V2)
	}
	notRun, err := VerifyCEMPredicate(envelope, publicKey, nil)
	if err != nil || notRun.Status != CEMNotRun || notRun.Reason != cemBytesNotSupplied ||
		notRun.V2.ArtifactsStatus != CEMNotRun || notRun.V2.TargetStatus != CEMNotRun {
		t.Fatalf("NOT_RUN verification = %#v, %v", notRun, err)
	}
	if _, err := VerifyCEMPredicateWith(envelope, publicKey, CEMCheck{Artifact: caseArtifact(nil)}); err == nil {
		t.Error("an artifact check without the map bytes verified")
	}
}

// TestCEMV1065RefusesSwappedReceiptArtifact is the CEM-V1-065 acceptance case:
// the receipt attestation is swapped for another valid envelope under the same
// path and the map re-issued with its artifacts[] and receipt envelope digests updated. The old
// envelope is refused against the new map, and against the old map with the
// swapped file.
func TestCEMV1065RefusesSwappedReceiptArtifact(t *testing.T) {
	cem := stableFixture(t)
	_, envelope := signedCEMV2(t, cem)
	publicKey := cemFixtureKey().Public().(ed25519.PublicKey)
	const swapped = "artifacts/receipt-attestation.json"
	original, err := os.ReadFile(stableCase + swapped)
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := os.ReadFile(stableCase + "artifacts/attestation-other-key.json")
	if err != nil {
		t.Fatal(err)
	}
	reissued := bytes.Replace(cem, []byte(sha256Hex(original)), []byte(sha256Hex(replacement)), -1)
	if bytes.Equal(reissued, cem) {
		t.Fatal("the fixture map does not declare the receipt attestation digest")
	}
	if _, err := wire.ParseStable(reissued); err != nil {
		t.Fatalf("the re-issued map is not a valid stable map: %v", err)
	}
	overrides := map[string][]byte{swapped: replacement}
	if _, err := VerifyCEMPredicateWith(envelope, publicKey, CEMCheck{Map: reissued, Artifact: caseArtifact(overrides)}); !errors.Is(err, ErrCEMBytesMismatch) {
		t.Errorf("old envelope against the re-issued map: %v, want ErrCEMBytesMismatch", err)
	}
	if _, err := VerifyCEMPredicateWith(envelope, publicKey, CEMCheck{Map: cem, Artifact: caseArtifact(overrides)}); !errors.Is(err, ErrCEMSubjectMismatch) {
		t.Errorf("old envelope with the swapped artifact file: %v, want ErrCEMSubjectMismatch", err)
	}
	// A statement over the re-issued map whose subjects omit an artifact the
	// map declares is refused as a subject mismatch.
	_, fresh := signedCEMV2(t, reissued)
	if _, err := VerifyCEMPredicateWith(fresh, publicKey, CEMCheck{Map: reissued, Artifact: caseArtifact(overrides)}); err != nil {
		t.Fatalf("a fresh envelope over the re-issued map: %v", err)
	}
	omitted, publicKey := signedEditedCEMV2(t, cem, func(statement string) string {
		return strings.Replace(statement, `,{"digest":{"sha256":"`+sha256Hex(original)+`"},"name":"`+swapped+`"}`, "", 1)
	})
	if _, err := VerifyCEMPredicateWith(omitted, publicKey, CEMCheck{Map: cem, Artifact: caseArtifact(nil)}); !errors.Is(err, ErrCEMSubjectMismatch) {
		t.Errorf("declared artifact missing from the subjects: %v, want ErrCEMSubjectMismatch", err)
	}
	unreadable := caseArtifact(nil)
	missing := func(name string) ([]byte, error) {
		if name == swapped {
			return nil, os.ErrNotExist
		}
		return unreadable(name)
	}
	if _, err := VerifyCEMPredicateWith(envelope, publicKey, CEMCheck{Map: cem, Artifact: missing}); !errors.Is(err, ErrCEMSubjectMismatch) {
		t.Errorf("missing artifact file: %v, want ErrCEMSubjectMismatch", err)
	}
}

// TestCEMV1065RefusesTargetThatIsNotTheSubject checks CEM-V1-065: a target
// commit or tree other than the signed one is refused.
func TestCEMV1065RefusesTargetThatIsNotTheSubject(t *testing.T) {
	cem := stableFixture(t)
	_, envelope := signedCEMV2(t, cem)
	publicKey := cemFixtureKey().Public().(ed25519.PublicKey)
	for _, target := range []CEMTarget{
		{GitCommit: fixtureV2Target.GitTree, GitTree: fixtureV2Target.GitTree},
		{GitCommit: fixtureV2Target.GitCommit, GitTree: fixtureV2Target.GitCommit},
	} {
		if _, err := VerifyCEMPredicateWith(envelope, publicKey, CEMCheck{Map: cem, Target: &target}); !errors.Is(err, ErrCEMTargetMismatch) {
			t.Errorf("target %+v: %v, want ErrCEMTargetMismatch", target, err)
		}
	}
}

// signedEditedCEMV2 signs the v2 statement over cem after a textual edit.
func signedEditedCEMV2(t *testing.T, cem []byte, edit func(string) string) ([]byte, ed25519.PublicKey) {
	t.Helper()
	statement, _ := signedCEMV2(t, cem)
	edited := edit(string(statement))
	if edited == string(statement) {
		t.Fatal("the edit changed nothing")
	}
	envelope, publicKey := signedFixtureStatement(t, []byte(edited))
	return envelope, publicKey
}

// TestCEMV1065PredicateIsClosed checks CEM-V1-065: an unknown or case-variant
// predicate member, a missing target, or a malformed subject is refused even
// when signed.
func TestCEMV1065PredicateIsClosed(t *testing.T) {
	cem := stableFixture(t)
	cases := map[string]func(string) string{
		"unknown predicate member": func(s string) string {
			return strings.Replace(s, `"predicate":{`, `"predicate":{"extra":1,`, 1)
		},
		"unknown target member": func(s string) string { return strings.Replace(s, `"target":{`, `"target":{"gitRef":"main",`, 1) },
		"case-variant member":   func(s string) string { return strings.Replace(s, `"baseRevision"`, `"BaseRevision"`, 1) },
		"repeated member": func(s string) string {
			return strings.Replace(s, `"predicate":{`, `"predicate":{"patchSha256":"`+strings.Repeat("0", 64)+`",`, 1)
		},
		"missing target": func(s string) string {
			start := strings.Index(s, `,"target":`)
			end := strings.Index(s, `}},"predicateType"`)
			return s[:start] + s[end+1:]
		},
		"uppercase subject digest": func(s string) string {
			return strings.Replace(s, sha256Hex(cem), strings.ToUpper(sha256Hex(cem)), 1)
		},
		"repeated subject name": func(s string) string {
			return strings.Replace(s, `"name":"artifacts/runner-receipt.json"`, `"name":"artifacts/runner-plan.json"`, 1)
		},
	}
	for name, edit := range cases {
		envelope, publicKey := signedEditedCEMV2(t, cem, edit)
		if _, err := VerifyCEMPredicate(envelope, publicKey, cem); err == nil {
			t.Errorf("%s: a signed statement verified", name)
		}
	}
}

// TestCEMV1218RetiredPredicatesRefuse checks CEM-V1-218 and CEM-V1-065: a
// cem/0.x map is never a cem/v2 subject, and a signed cem/0 or cem/v1
// envelope is refused by name rather than read as a claim.
func TestCEMV1218RetiredPredicatesRefuse(t *testing.T) {
	legacy := []byte(`{"spec":"cem/0.2"}`)
	if _, err := CEMStatementV2(stableMapName, legacy, fixtureV2Target); !errors.Is(err, ErrCEMRequiresStableMap) {
		t.Errorf("CEMStatementV2 on a 0.x map: %v, want ErrCEMRequiresStableMap", err)
	}
	stable := stableFixture(t)
	for _, predicateType := range []string{"https://corvint-context.dev/attestation/cem/0", "https://corvint-context.dev/attestation/cem/v1"} {
		statement, err := canonicalMarshal(map[string]any{
			"_type":         statementType,
			"subject":       []any{map[string]any{"name": stableMapName, "digest": map[string]any{"sha256": sha256Hex(stable)}}},
			"predicateType": predicateType,
			"predicate":     map[string]any{"name": stableMapName},
		})
		if err != nil {
			t.Fatal(err)
		}
		envelope, publicKey := signedFixtureStatement(t, statement)
		for _, cem := range [][]byte{nil, stable} {
			_, err := VerifyCEMPredicate(envelope, publicKey, cem)
			if err == nil || !strings.Contains(err.Error(), "is retired; "+CEMPredicateTypeV2+" is the only CEM predicate") {
				t.Errorf("%s envelope: %v, want the retired-predicate refusal", predicateType, err)
			}
		}
	}
}
