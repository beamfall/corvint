package attest

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const cemFixtureName = ".corvint/change.cem.json"

func cemFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "interop", "cem-0.1", "maps", "valid", "supported-sha256.json"))
	if err != nil {
		t.Fatalf("read CEM fixture: %v", err)
	}
	return data
}

// legacyFixture is the base revision, patch digest and spec of the cem/0.1
// fixture, read as plain JSON: the cem/0 and cem/v1 producers refuse every map
// now (CEM-V1-210), so these tests sign the statements they used to emit.
type legacyFixture struct {
	BaseRevision string `json:"baseRevision"`
	PatchSha256  string `json:"patchSha256"`
	Spec         string `json:"spec"`
}

func readLegacyFixture(t *testing.T, cem []byte) legacyFixture {
	t.Helper()
	var fixture legacyFixture
	if err := json.Unmarshal(cem, &fixture); err != nil || fixture.Spec == "" {
		t.Fatalf("legacy fixture: %v", err)
	}
	return fixture
}

// legacyCEMStatement is the canonical cem/0 statement CEMStatement emitted
// for cem before cem/0.x was retired.
func legacyCEMStatement(t *testing.T, name string, cem []byte) []byte {
	t.Helper()
	sum := sha256.Sum256(cem)
	digest := map[string]any{"sha256": hex.EncodeToString(sum[:])}
	statement, err := canonicalMarshal(map[string]any{
		"_type":         statementType,
		"subject":       []any{map[string]any{"name": name, "digest": digest}},
		"predicateType": CEMPredicateType,
		"predicate":     map[string]any{"name": name, "digest": digest, "size": json.Number(strconv.Itoa(len(cem))), "spec": readLegacyFixture(t, cem).Spec},
	})
	if err != nil {
		t.Fatal(err)
	}
	return statement
}

func signedCEM(t *testing.T, cem []byte) ([]byte, ed25519.PublicKey) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	statement := legacyCEMStatement(t, cemFixtureName, cem)
	envelope, err := Envelope(statement, privateKey)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	return envelope, publicKey
}

// TestCEMAttestationRoundTripsThroughTheExistingEnvelope checks FPK-V0-030:
// the CEM statement signs and verifies through Envelope/Verify, its subject and
// predicate carry an independently computed digest, VerifyCEM reports the claim
// NOT_RUN without bytes, and it refuses the matching retired map bytes
// (CEM-V1-210).
func TestCEMAttestationRoundTripsThroughTheExistingEnvelope(t *testing.T) {
	cem := cemFixture(t)
	envelope, publicKey := signedCEM(t, cem)
	sum := sha256.Sum256(cem)
	wantDigest := hex.EncodeToString(sum[:])

	payload, err := Verify(envelope, publicKey)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	var statement map[string]any
	if err := json.Unmarshal(payload, &statement); err != nil {
		t.Fatalf("decode statement: %v", err)
	}
	if statement["predicateType"] != "https://corvint-context.dev/attestation/cem/0" {
		t.Errorf("predicateType = %v", statement["predicateType"])
	}
	subjects, _ := statement["subject"].([]any)
	subject, _ := subjects[0].(map[string]any)
	subjectDigest, _ := subject["digest"].(map[string]any)
	if len(subjects) != 1 || subject["name"] != cemFixtureName || subjectDigest["sha256"] != wantDigest {
		t.Errorf("subject = %#v, want one %s entry with sha256 %s", statement["subject"], cemFixtureName, wantDigest)
	}
	if _, embedded := statement["predicate"].(map[string]any)["cem"]; embedded {
		t.Error("predicate embeds CEM bytes; FPK-V0-030 carries a content-addressed reference only")
	}

	got, err := VerifyCEM(envelope, publicKey, nil)
	if err != nil {
		t.Fatalf("VerifyCEM: %v", err)
	}
	want := CEMVerification{Name: cemFixtureName, SHA256: wantDigest, Size: int64(len(cem)), Spec: "cem/0.1", Status: CEMNotRun, Reason: cemBytesNotSupplied}
	if got != want {
		t.Errorf("VerifyCEM = %#v, want %#v", got, want)
	}
	if got, err := VerifyCEM(envelope, publicKey, cem); cemcode.CodeOf(err) != cemcode.UnsupportedSpec || !strings.Contains(err.Error(), wire.RetiredSpecFix) {
		t.Errorf("VerifyCEM over the retired map = %#v, %v; want unsupported-spec with the recovery line", got, err)
	}
}

// TestCEMAttestationRefusesTamperedMapBytes checks FPK-V0-030: bytes that differ
// from the signed digest, including an empty map, are refused, and a non-CEM
// input is never attested.
func TestCEMAttestationRefusesTamperedMapBytes(t *testing.T) {
	cem := cemFixture(t)
	envelope, publicKey := signedCEM(t, cem)
	tampered := append([]byte{}, cem...)
	tampered[len(tampered)/2] ^= 0x01

	for name, candidate := range map[string][]byte{"flipped byte": tampered, "empty": {}, "appended": append(append([]byte{}, cem...), ' ')} {
		if _, err := VerifyCEM(envelope, publicKey, candidate); err == nil {
			t.Errorf("%s: VerifyCEM accepted CEM bytes that do not match the signed digest", name)
		}
	}
	if _, err := CEMStatement(cemFixtureName, []byte(`{"not":"a cem"}`)); err == nil {
		t.Error("CEMStatement attested bytes that are not a CEM")
	}
}

// TestCEMV1210LegacyPredicatesRefuseEveryMap checks CEM-V1-210: the cem/0 and
// cem/v1 producers refuse a cem/0.x map with the retired-spec fix and a cem/1.0
// map as unsupported-spec, and a signed v1 claim is never confirmed by a map.
func TestCEMV1210LegacyPredicatesRefuseEveryMap(t *testing.T) {
	legacy, stable := cemFixture(t), stableFixture(t)
	for _, produce := range []func(string, []byte) ([]byte, error){CEMStatement, CEMStatementV1} {
		if _, err := produce(cemFixtureName, legacy); cemcode.CodeOf(err) != cemcode.UnsupportedSpec || !strings.Contains(err.Error(), wire.RetiredSpecFix) {
			t.Errorf("0.x map: %v", err)
		}
		if _, err := produce(stableMapName, stable); cemcode.CodeOf(err) != cemcode.UnsupportedSpec {
			t.Errorf("1.0 map: %v", err)
		}
	}
	envelope, publicKey := signedCEMV1(t, buildCEMV1Statement(t))
	if _, err := VerifyCEMPredicate(envelope, publicKey, legacy); cemcode.CodeOf(err) != cemcode.UnsupportedSpec || errors.Is(err, ErrCEMBytesMismatch) {
		t.Errorf("v1 claim over the 0.x map: %v", err)
	}
}

// TestCEMAttestationRefusesBytesWhoseSizeDiffersFromTheSignedClaim checks
// FPK-V0-030: bytes matching the signed sha256 are still refused as a byte
// mismatch when the signed size disagrees with their length.
func TestCEMAttestationRefusesBytesWhoseSizeDiffersFromTheSignedClaim(t *testing.T) {
	cem := cemFixture(t)
	envelope, publicKey := signedEditedCEMStatement(t, func(statement map[string]any) {
		statement["predicate"].(map[string]any)["size"] = len(cem) + 1
	})
	if got, err := VerifyCEM(envelope, publicKey, cem); !errors.Is(err, ErrCEMBytesMismatch) {
		t.Fatalf("VerifyCEM = %#v, %v; want ErrCEMBytesMismatch", got, err)
	}
}

// TestCEMAttestationDisclosesMissingMapBytes checks FPK-V0-030: with no CEM
// bytes supplied the signed claim is returned as NOT_RUN with a reason, never
// as VERIFIED.
func TestCEMAttestationDisclosesMissingMapBytes(t *testing.T) {
	cem := cemFixture(t)
	envelope, publicKey := signedCEM(t, cem)

	got, err := VerifyCEM(envelope, publicKey, nil)
	if err != nil {
		t.Fatalf("VerifyCEM without bytes: %v", err)
	}
	if got.Status != CEMNotRun || got.Reason != "cem-bytes-not-supplied" {
		t.Errorf("VerifyCEM without bytes = %#v, want NOT_RUN with reason cem-bytes-not-supplied", got)
	}
	if got.Name != cemFixtureName || got.Size != int64(len(cem)) {
		t.Errorf("VerifyCEM without bytes lost the signed claim: %#v", got)
	}
}

// signedEditedCEMStatement signs, with a fresh key, the fixture's CEM statement
// after edit changes its decoded JSON, so a refusal is reached only past Verify.
func signedEditedCEMStatement(t *testing.T, edit func(statement map[string]any)) ([]byte, ed25519.PublicKey) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	statement := legacyCEMStatement(t, cemFixtureName, cemFixture(t))
	var decoded map[string]any
	if err := json.Unmarshal(statement, &decoded); err != nil {
		t.Fatalf("decode statement: %v", err)
	}
	edit(decoded)
	edited, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("encode statement: %v", err)
	}
	envelope, err := Envelope(edited, privateKey)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	return envelope, publicKey
}

// refuseSignedCEMClaim fails unless VerifyCEM refuses envelope both with the
// matching map bytes and without bytes, and never as a byte mismatch.
func refuseSignedCEMClaim(t *testing.T, name string, envelope []byte, publicKey ed25519.PublicKey) {
	t.Helper()
	for _, cem := range [][]byte{cemFixture(t), nil} {
		got, err := VerifyCEM(envelope, publicKey, cem)
		if err == nil || errors.Is(err, ErrCEMBytesMismatch) {
			t.Errorf("%s (bytes supplied: %t): VerifyCEM = %#v, %v; want a non-mismatch refusal", name, cem != nil, got, err)
		}
	}
}

// TestCEMAttestationRefusesASignedStatementThatIsNotACEMClaim checks FPK-V0-030:
// a validly signed statement with a different _type or predicateType, other
// than one subject, or a subject that disagrees with the predicate is refused.
func TestCEMAttestationRefusesASignedStatementThatIsNotACEMClaim(t *testing.T) {
	subjects := func(statement map[string]any) []any { return statement["subject"].([]any) }
	for name, edit := range map[string]func(map[string]any){
		"_type": func(statement map[string]any) { statement["_type"] = "https://in-toto.io/Statement/v0.1" },
		"predicateType": func(statement map[string]any) {
			statement["predicateType"] = "https://corvint-context.dev/attestation/prove/0"
		},
		"no subject": func(statement map[string]any) { statement["subject"] = []any{} },
		"two subjects": func(statement map[string]any) {
			statement["subject"] = append(subjects(statement), subjects(statement)[0])
		},
		"subject name": func(statement map[string]any) { subjects(statement)[0].(map[string]any)["name"] = "other.cem.json" },
		"subject digest": func(statement map[string]any) {
			subjects(statement)[0].(map[string]any)["digest"] = map[string]any{"sha256": strings.Repeat("0", 64)}
		},
	} {
		envelope, publicKey := signedEditedCEMStatement(t, edit)
		refuseSignedCEMClaim(t, name, envelope, publicKey)
	}
}

// TestCEMAttestationRefusesAMalformedSignedClaim checks FPK-V0-030: a validly
// signed claim CEMStatement could not have produced (sha256 not 64 lowercase
// hex, negative size, empty name or spec) is refused, not reported NOT_RUN.
func TestCEMAttestationRefusesAMalformedSignedClaim(t *testing.T) {
	setDigest := func(value string) func(map[string]any) {
		return func(statement map[string]any) {
			digest := map[string]any{"sha256": value}
			statement["predicate"].(map[string]any)["digest"] = digest
			statement["subject"].([]any)[0].(map[string]any)["digest"] = digest
		}
	}
	for name, edit := range map[string]func(map[string]any){
		"empty sha256":     setDigest(""),
		"short sha256":     setDigest(strings.Repeat("a", 63)),
		"uppercase sha256": setDigest(strings.Repeat("A", 64)),
		"negative size":    func(statement map[string]any) { statement["predicate"].(map[string]any)["size"] = -1 },
		"empty spec":       func(statement map[string]any) { statement["predicate"].(map[string]any)["spec"] = "" },
		"empty name": func(statement map[string]any) {
			statement["predicate"].(map[string]any)["name"] = ""
			statement["subject"].([]any)[0].(map[string]any)["name"] = ""
		},
	} {
		envelope, publicKey := signedEditedCEMStatement(t, edit)
		refuseSignedCEMClaim(t, name, envelope, publicKey)
	}
}

// signedRawCEMStatement signs the canonical CEM statement after edit rewrites
// its bytes, so a test can sign JSON a Go map cannot represent.
func signedRawCEMStatement(t *testing.T, edit func(statement string) string) ([]byte, ed25519.PublicKey) {
	t.Helper()
	statement := legacyCEMStatement(t, cemFixtureName, cemFixture(t))
	envelope, publicKey, _ := testEnvelope(t, []byte(edit(string(statement))))
	return envelope, publicKey
}

// TestCEMAttestationRefusesADuplicateStatementMember checks FPK-V0-030: a
// validly signed statement that repeats a member name is refused, so a strict
// in-toto reader and Corvint cannot read two different claims from one envelope.
func TestCEMAttestationRefusesADuplicateStatementMember(t *testing.T) {
	for name, edit := range map[string]func(string) string{
		"predicateType": func(statement string) string {
			return strings.Replace(statement, `"predicateType":`, `"predicateType":"https://example.com/other","predicateType":`, 1)
		},
		"predicate size": func(statement string) string { return strings.Replace(statement, `"size":`, `"size":1,"size":`, 1) },
	} {
		envelope, publicKey := signedRawCEMStatement(t, edit)
		refuseSignedCEMClaim(t, name, envelope, publicKey)
	}
}

// TestCEMAttestationRefusesACaseVariantStatementMember checks FPK-V0-030: a
// validly signed statement with a member whose name differs from a statement
// member only in case is refused, whichever value a reader would keep.
func TestCEMAttestationRefusesACaseVariantStatementMember(t *testing.T) {
	for name, edit := range map[string]func(string) string{
		"PredicateType": func(statement string) string {
			return strings.Replace(statement, `"predicateType":`, `"predicateType":"https://example.com/other","PredicateType":`, 1)
		},
		"predicate Spec": func(statement string) string {
			return strings.Replace(statement, `"spec":`, `"Spec":"other","spec":`, 1)
		},
	} {
		envelope, publicKey := signedRawCEMStatement(t, edit)
		refuseSignedCEMClaim(t, name, envelope, publicKey)
	}
}

// TestCEMAttestationAcceptsUnknownStatementMembers checks FPK-V0-030: members
// the statement does not define are ignored, as in-toto consumers must.
func TestCEMAttestationAcceptsUnknownStatementMembers(t *testing.T) {
	envelope, publicKey := signedRawCEMStatement(t, func(statement string) string {
		withExtension := strings.Replace(statement, `{"_type":`, `{"extension":{"a":1},"_type":`, 1)
		return strings.Replace(withExtension, `"size":`, `"note":"x","size":`, 1)
	})
	got, err := VerifyCEM(envelope, publicKey, nil)
	if err != nil || got.Status != CEMNotRun {
		t.Fatalf("VerifyCEM = %#v, %v; want the claim NOT_RUN", got, err)
	}
}
