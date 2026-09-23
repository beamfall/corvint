package attest

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// cemV1FixtureEnvelopeSHA256 pins the envelope TestCEMV1FixtureEnvelopeIsDigestPinned
// rebuilds; interop/cem01-go/intoto_test.go embeds those exact bytes and pins
// the same digest, so the independent consumer reads what Corvint emits.
const cemV1FixtureEnvelopeSHA256 = "283792cd974edb5112edfe9e23df7f4b155148310850c1001ae6c9cd9c976b38"

// cemV1FixtureKey is a public, test-only Ed25519 key derived from a fixed
// seed so the fixture envelope is reproducible; it signs nothing else.
func cemV1FixtureKey() ed25519.PrivateKey {
	seed := sha256.Sum256([]byte("corvint FPK-V0-033 fixture key"))
	return ed25519.NewKeyFromSeed(seed[:])
}

func signedCEMV1(t *testing.T, statement []byte) ([]byte, ed25519.PublicKey) {
	t.Helper()
	privateKey := cemV1FixtureKey()
	envelope, err := Envelope(statement, privateKey)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	return envelope, privateKey.Public().(ed25519.PublicKey)
}

func buildCEMV1Statement(t *testing.T) []byte {
	t.Helper()
	statement, err := CEMStatementV1(cemFixtureName, cemFixture(t))
	if err != nil {
		t.Fatalf("CEMStatementV1: %v", err)
	}
	return statement
}

// TestCEMV1StatementBindsBaseAndPatchAndVerifies checks FPK-V0-033 and
// FPK-V0-034: the v1 predicate carries the map's own base revision and patch
// digest beside the ResourceDescriptor, VerifyCEMPredicate reports VERIFIED or
// NOT_RUN with its predicate type, and a v0 envelope still verifies to the
// VerifyCEM claim.
func TestCEMV1StatementBindsBaseAndPatchAndVerifies(t *testing.T) {
	cem := cemFixture(t)
	document, err := wire.ParseMap(cem)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(cem)
	digest := hex.EncodeToString(sum[:])
	want := `{"_type":"https://in-toto.io/Statement/v1","predicate":{"base":{"digest":{"gitCommit":"` +
		document.BaseRevision + `"}},"cem":{"digest":{"sha256":"` + digest + `"},"name":"` + cemFixtureName +
		`"},"patch":{"digest":{"sha256":"` + document.PatchSha256 + `"}},"size":` + jsonInt(len(cem)) +
		`,"spec":"cem/0.1"},"predicateType":"https://corvint-context.dev/attestation/cem/v1","subject":[{"digest":{"sha256":"` +
		digest + `"},"name":"` + cemFixtureName + `"}]}`
	statement := buildCEMV1Statement(t)
	if string(statement) != want {
		t.Fatalf("CEMStatementV1 =\n%s\nwant\n%s", statement, want)
	}

	envelope, publicKey := signedCEMV1(t, statement)
	claim := CEMVerification{
		Name: cemFixtureName, SHA256: digest, Size: int64(len(cem)), Spec: "cem/0.1",
		PredicateType: CEMPredicateTypeV1, BaseRevision: document.BaseRevision, PatchSHA256: document.PatchSha256,
	}
	verified, notRun := claim, claim
	verified.Status = CEMVerified
	notRun.Status, notRun.Reason = CEMNotRun, cemBytesNotSupplied
	if got, err := VerifyCEMPredicate(envelope, publicKey, cem); err != nil || got != verified {
		t.Errorf("VerifyCEMPredicate(bytes) = %#v, %v; want %#v", got, err, verified)
	}
	if got, err := VerifyCEMPredicate(envelope, publicKey, nil); err != nil || got != notRun {
		t.Errorf("VerifyCEMPredicate(nil) = %#v, %v; want %#v", got, err, notRun)
	}
	if _, err := VerifyCEM(envelope, publicKey, cem); err == nil {
		t.Error("VerifyCEM accepted a v1 statement; FPK-V0-030 stays v0-only")
	}

	v0Envelope, v0Key := signedCEM(t, cem)
	v0Claim, err := VerifyCEM(v0Envelope, v0Key, cem)
	if err != nil {
		t.Fatal(err)
	}
	v0Claim.PredicateType = CEMPredicateType
	if got, err := VerifyCEMPredicate(v0Envelope, v0Key, cem); err != nil || got != v0Claim {
		t.Errorf("VerifyCEMPredicate(v0) = %#v, %v; want %#v", got, err, v0Claim)
	}
}

func jsonInt(value int) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

// TestCEMV1RefusesAClaimItCouldNotHaveProduced checks FPK-V0-034: a validly
// signed v1 statement with a malformed base or patch, a subject that disagrees
// with the cem descriptor, a case-variant member, or an unknown predicateType
// is refused with or without bytes; a well-formed base or patch the matching
// map does not carry is refused when the bytes are supplied, never as a byte
// mismatch.
func TestCEMV1RefusesAClaimItCouldNotHaveProduced(t *testing.T) {
	cem := cemFixture(t)
	original := string(buildCEMV1Statement(t))
	document, err := wire.ParseMap(cem)
	if err != nil {
		t.Fatal(err)
	}
	otherOID := strings.Repeat("a", len(document.BaseRevision))
	for name, test := range map[string]struct {
		edited    string
		withBytes bool
	}{
		"short base":       {strings.Replace(original, document.BaseRevision, "abc", 1), false},
		"uppercase patch":  {strings.Replace(original, document.PatchSha256, strings.ToUpper(document.PatchSha256), 1), false},
		"cem name":         {strings.Replace(original, `"name":"`+cemFixtureName+`"},"patch"`, `"name":"other"},"patch"`, 1), false},
		"case variant":     {strings.Replace(original, `"base":`, `"Base":{},"base":`, 1), false},
		"predicate type":   {strings.Replace(original, "attestation/cem/v1", "attestation/cem/v2", 1), false},
		"type case":        {strings.Replace(original, `"predicateType"`, `"PredicateType":"x","predicateType"`, 1), false},
		"other base":       {strings.Replace(original, document.BaseRevision, otherOID, 1), true},
		"other patch":      {strings.Replace(original, document.PatchSha256, strings.Repeat("b", 64), 1), true},
		"other spec":       {strings.Replace(original, `"spec":"cem/0.1"`, `"spec":"cem/0.2"`, 1), true},
		"repeated member":  {strings.Replace(original, `"size":`, `"size":1,"size":`, 1), false},
		"no subject":       {strings.Replace(original, `"subject":[`, `"subject":[],"x":[`, 1), false},
		"other _type":      {strings.Replace(original, "Statement/v1", "Statement/v0.1", 1), false},
		"unsupported base": {strings.Replace(original, `"gitCommit"`, `"sha1"`, 1), false},
	} {
		if test.edited == original {
			t.Fatalf("%s: edit did not apply", name)
		}
		envelope, publicKey := signedCEMV1(t, []byte(test.edited))
		inputs := [][]byte{cem, nil}
		if test.withBytes {
			inputs = [][]byte{cem}
		}
		for _, input := range inputs {
			got, err := VerifyCEMPredicate(envelope, publicKey, input)
			if err == nil || errors.Is(err, ErrCEMBytesMismatch) {
				t.Errorf("%s (bytes supplied: %t): VerifyCEMPredicate = %#v, %v; want a non-mismatch refusal", name, input != nil, got, err)
			}
		}
	}
}

// TestCEMV1FixtureEnvelopeIsDigestPinned checks FPK-V0-035: the envelope
// CEMStatementV1 and Envelope produce for the public CEM 0.1 fixture under the
// fixed test key has the digest the interop consumer's embedded copy pins.
func TestCEMV1FixtureEnvelopeIsDigestPinned(t *testing.T) {
	envelope, _ := signedCEMV1(t, buildCEMV1Statement(t))
	sum := sha256.Sum256(envelope)
	if got := hex.EncodeToString(sum[:]); got != cemV1FixtureEnvelopeSHA256 {
		t.Fatalf("fixture envelope sha256 = %s, want %s\nenvelope: %s\npublic key: %x", got, cemV1FixtureEnvelopeSHA256,
			envelope, []byte(cemV1FixtureKey().Public().(ed25519.PublicKey)))
	}
}
