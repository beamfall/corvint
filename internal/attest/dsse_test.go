package attest

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func TestEnvelopeVerifyRoundTrip(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	statement := []byte(`{"hello":"world"}`)

	envelope, err := Envelope(statement, privateKey)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	payload, err := Verify(envelope, publicKey)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !bytes.Equal(payload, statement) {
		t.Errorf("Verify returned %q, want %q", payload, statement)
	}
}

// TestPreAuthenticationEncodingMatchesHandComputedValue checks the PAE
// against a value built independently in the test, character by character,
// per the DSSE spec: "DSSEv1" SP len(payloadType) SP payloadType SP
// len(payload) SP payload.
func TestPreAuthenticationEncodingMatchesHandComputedValue(t *testing.T) {
	tinyPayload := []byte("hi") // 2 bytes
	// payloadType is the fixed constant "application/vnd.in-toto+json",
	// independently counted at 28 bytes.
	const wantPayloadTypeLen = 28
	if len(payloadType) != wantPayloadTypeLen {
		t.Fatalf("payloadType %q has length %d, hand count says %d", payloadType, len(payloadType), wantPayloadTypeLen)
	}
	want := "DSSEv1 28 application/vnd.in-toto+json 2 hi"

	got := preAuthenticationEncoding(payloadType, tinyPayload)
	if string(got) != want {
		t.Errorf("preAuthenticationEncoding = %q, want %q", got, want)
	}
}

func TestKeyIDIsSHA256OfPublicKey(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	sum := sha256.Sum256(publicKey)
	want := hex.EncodeToString(sum[:])
	if got := keyID(publicKey); got != want {
		t.Errorf("keyID = %q, want %q", got, want)
	}
}

func TestEnvelopeSetsKeyIDToSHA256OfPublicKey(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	envelopeBytes, err := Envelope([]byte(`{"a":1}`), privateKey)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	var parsed dsseEnvelope
	if err := json.Unmarshal(envelopeBytes, &parsed); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	sum := sha256.Sum256(publicKey)
	want := hex.EncodeToString(sum[:])
	if len(parsed.Signatures) != 1 || parsed.Signatures[0].KeyID != want {
		t.Errorf("envelope keyid = %+v, want %q", parsed.Signatures, want)
	}
}

func testEnvelope(t *testing.T, statement []byte) ([]byte, ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	envelope, err := Envelope(statement, privateKey)
	if err != nil {
		t.Fatalf("Envelope: %v", err)
	}
	return envelope, publicKey, privateKey
}

func TestVerifyRejectsWrongPayloadType(t *testing.T) {
	envelope, publicKey, _ := testEnvelope(t, []byte(`{}`))
	var parsed map[string]any
	if err := json.Unmarshal(envelope, &parsed); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	parsed["payloadType"] = "application/json"
	mutated, err := json.Marshal(parsed)
	if err != nil {
		t.Fatalf("marshal mutated envelope: %v", err)
	}
	if _, err := Verify(mutated, publicKey); err == nil {
		t.Fatal("Verify accepted the wrong payloadType")
	}
}

func TestVerifyRejectsZeroSignatures(t *testing.T) {
	envelope, publicKey, _ := testEnvelope(t, []byte(`{}`))
	var parsed map[string]any
	if err := json.Unmarshal(envelope, &parsed); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	parsed["signatures"] = []any{}
	mutated, err := json.Marshal(parsed)
	if err != nil {
		t.Fatalf("marshal mutated envelope: %v", err)
	}
	if _, err := Verify(mutated, publicKey); err == nil {
		t.Fatal("Verify accepted zero signatures")
	}
}

func TestVerifyRejectsMultipleSignatures(t *testing.T) {
	envelope, publicKey, _ := testEnvelope(t, []byte(`{}`))
	var parsed map[string]any
	if err := json.Unmarshal(envelope, &parsed); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	signatures, _ := parsed["signatures"].([]any)
	parsed["signatures"] = append(signatures, signatures[0])
	mutated, err := json.Marshal(parsed)
	if err != nil {
		t.Fatalf("marshal mutated envelope: %v", err)
	}
	if _, err := Verify(mutated, publicKey); err == nil {
		t.Fatal("Verify accepted multiple signatures")
	}
}

func TestVerifyRejectsKeyIDMismatch(t *testing.T) {
	envelope, _, _ := testEnvelope(t, []byte(`{}`))
	otherPublicKey, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	if _, err := Verify(envelope, otherPublicKey); err == nil {
		t.Fatal("Verify accepted a keyid that does not match the given public key")
	}
}

// TestVerifyRejectsKeyIDThatDoesNotNameTheSigningKey checks FPK-V0-015: a
// signature that verifies under the given key is still refused when its keyid
// is not the SHA-256 of that key.
func TestVerifyRejectsKeyIDThatDoesNotNameTheSigningKey(t *testing.T) {
	envelope, publicKey, _ := testEnvelope(t, []byte(`{}`))
	var parsed map[string]any
	if err := json.Unmarshal(envelope, &parsed); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	parsed["signatures"].([]any)[0].(map[string]any)["keyid"] = strings.Repeat("0", 64)
	mutated, err := json.Marshal(parsed)
	if err != nil {
		t.Fatalf("marshal mutated envelope: %v", err)
	}
	if _, err := Verify(mutated, publicKey); err == nil {
		t.Fatal("Verify accepted a keyid that does not name the signing key")
	}
}

func TestVerifyRejectsBadPayloadBase64(t *testing.T) {
	envelope, publicKey, _ := testEnvelope(t, []byte(`{}`))
	var parsed map[string]any
	if err := json.Unmarshal(envelope, &parsed); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	parsed["payload"] = "not-valid-base64!!"
	mutated, err := json.Marshal(parsed)
	if err != nil {
		t.Fatalf("marshal mutated envelope: %v", err)
	}
	if _, err := Verify(mutated, publicKey); err == nil {
		t.Fatal("Verify accepted invalid payload base64")
	}
}

func TestVerifyRejectsBadSignature(t *testing.T) {
	envelope, publicKey, _ := testEnvelope(t, []byte(`{"a":1}`))
	var parsed map[string]any
	if err := json.Unmarshal(envelope, &parsed); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	// Replace the payload with different (still valid) bytes so the keyid
	// still matches but the signature no longer verifies over the new PAE.
	parsed["payload"] = base64.StdEncoding.EncodeToString([]byte(`{"a":2}`))
	mutated, err := json.Marshal(parsed)
	if err != nil {
		t.Fatalf("marshal mutated envelope: %v", err)
	}
	if _, err := Verify(mutated, publicKey); err == nil {
		t.Fatal("Verify accepted a tampered payload")
	}
}

func TestVerifyRejectsUnknownEnvelopeMember(t *testing.T) {
	envelope, publicKey, _ := testEnvelope(t, []byte(`{}`))
	var parsed map[string]any
	if err := json.Unmarshal(envelope, &parsed); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	parsed["extra"] = "surprise"
	mutated, err := json.Marshal(parsed)
	if err != nil {
		t.Fatalf("marshal mutated envelope: %v", err)
	}
	if _, err := Verify(mutated, publicKey); err == nil {
		t.Fatal("Verify accepted an envelope with an unknown member")
	}
}

// TestVerifyRejectsAPublicKeyOfTheWrongSize checks that a public key that is
// not 32 bytes is an error, not an ed25519.Verify panic, even when the
// envelope's keyid is the sha256 of that key.
func TestVerifyRejectsAPublicKeyOfTheWrongSize(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := Envelope([]byte(`{}`), privateKey)
	if err != nil {
		t.Fatal(err)
	}
	var parsed dsseEnvelope
	if err := json.Unmarshal(envelope, &parsed); err != nil {
		t.Fatal(err)
	}
	shortKey := ed25519.PublicKey{}
	parsed.Signatures[0].KeyID = keyID(shortKey)
	forged, err := json.Marshal(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(forged, shortKey); err == nil {
		t.Fatal("Verify accepted a zero-length public key")
	}
}

// TestVerifyRejectsDuplicateMemberNames checks FPK-V0-015: a signed envelope
// that repeats a member name, in the envelope or in its signature object, is
// refused rather than read by whichever occurrence a decoder keeps.
func TestVerifyRejectsDuplicateMemberNames(t *testing.T) {
	envelope, publicKey, _ := testEnvelope(t, []byte(`{}`))
	for name, mutated := range map[string]string{
		"envelope payload": strings.Replace(string(envelope), `"payload":`, `"payload":"e30=","payload":`, 1),
		"signature keyid":  strings.Replace(string(envelope), `"keyid":`, `"keyid":"00","keyid":`, 1),
	} {
		if _, err := Verify([]byte(mutated), publicKey); err == nil {
			t.Errorf("%s: Verify accepted a duplicate member name", name)
		}
	}
}

// TestVerifyRejectsCaseVariantSignatureMember checks FPK-V0-015: a signature
// member whose name differs from sig or keyid only in case is refused, alone
// or beside the exact name.
func TestVerifyRejectsCaseVariantSignatureMember(t *testing.T) {
	envelope, publicKey, _ := testEnvelope(t, []byte(`{}`))
	for name, mutated := range map[string]string{
		"alone":        strings.Replace(string(envelope), `"keyid":`, `"KeyID":`, 1),
		"beside exact": strings.Replace(string(envelope), `"keyid":`, `"KEYID":"00","keyid":`, 1),
	} {
		if _, err := Verify([]byte(mutated), publicKey); err == nil {
			t.Errorf("%s: Verify accepted a case-variant signature member", name)
		}
	}
}
