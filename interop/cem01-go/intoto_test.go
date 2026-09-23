package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// intotoFixtureEnvelope is the envelope Corvint's CEMStatementV1 and Envelope
// emit for ../cem-0.1/maps/valid/supported-sha256.json named
// .corvint/change.cem.json under a public test-only key; its sha256 is pinned
// here and in Corvint's TestCEMV1FixtureEnvelopeIsDigestPinned.
const intotoFixtureEnvelope = `{"payload":"eyJfdHlwZSI6Imh0dHBzOi8vaW4tdG90by5pby9TdGF0ZW1lbnQvdjEiLCJwcmVkaWNhdGUiOnsiYmFzZSI6eyJkaWdlc3QiOnsiZ2l0Q29tbWl0IjoiMmZiMDcyY2YwZjJhMDhmNDVjMjY1ZDE2ODVkNWZiMTFjNDMzN2E5M2EzZjcyMmI2YTdmZjQxZGM0NTcwZmU0ZiJ9fSwiY2VtIjp7ImRpZ2VzdCI6eyJzaGEyNTYiOiJmMGRkNGE4YWI5YWY5MzM1ZDI3M2VkNDQ3ZTc1MDBiMDg2OTI1ODVkMDcyZDQ0MTU3MjM4ZjFlZWQ5YTlmZWI4In0sIm5hbWUiOiIuY29ydmludC9jaGFuZ2UuY2VtLmpzb24ifSwicGF0Y2giOnsiZGlnZXN0Ijp7InNoYTI1NiI6ImFhYTkxNzU5ODU3YmZlNjhiZWI1OGM3NDBhNzdiYzUyOTJhM2FkZTE2ZDQxOGE0NGUxZWY0ZjdmZmRkNzA3YWQifX0sInNpemUiOjg2Mywic3BlYyI6ImNlbS8wLjEifSwicHJlZGljYXRlVHlwZSI6Imh0dHBzOi8vY29ydmludC1jb250ZXh0LmRldi9hdHRlc3RhdGlvbi9jZW0vdjEiLCJzdWJqZWN0IjpbeyJkaWdlc3QiOnsic2hhMjU2IjoiZjBkZDRhOGFiOWFmOTMzNWQyNzNlZDQ0N2U3NTAwYjA4NjkyNTg1ZDA3MmQ0NDE1NzIzOGYxZWVkOWE5ZmViOCJ9LCJuYW1lIjoiLmNvcnZpbnQvY2hhbmdlLmNlbS5qc29uIn1dfQ==","payloadType":"application/vnd.in-toto+json","signatures":[{"sig":"uHGXZlxb4QNSQeXV80Av5gfeXK9X0I5vVvybnCSbRnm9RkUsoMKTX0Al7FzniZSdxFj8g61N1uYRyclK5pwcDQ==","keyid":"435af94acc20f0d9c0e19cf3561bffaf1fdeac2979e7c1c20cba69542412e9b3"}]}`

const intotoFixtureEnvelopeSHA256 = "283792cd974edb5112edfe9e23df7f4b155148310850c1001ae6c9cd9c976b38"

const intotoFixturePublicKey = "-----BEGIN PUBLIC KEY-----\nMCowBQYDK2VwAyEAMAazj6KeKiuiTGuM1CPJzZIK4JHKwVd866XgzODWRNM=\n-----END PUBLIC KEY-----\n"

func intotoFixtureMap(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "cem-0.1", "maps", "valid", "supported-sha256.json"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestIntotoReadsTheDigestPinnedCorvintCEMAttestation checks FPK-V0-035: the
// embedded envelope has its pinned digest, verifies offline under the PKIX key,
// yields the cem/v1 claim this consumer derives from the map itself, and is
// VERIFIED with the map bytes or NOT_RUN without them.
func TestIntotoReadsTheDigestPinnedCorvintCEMAttestation(t *testing.T) {
	if got := shaHex([]byte(intotoFixtureEnvelope)); got != intotoFixtureEnvelopeSHA256 {
		t.Fatalf("fixture envelope sha256 = %s, want %s", got, intotoFixtureEnvelopeSHA256)
	}
	mapBytes := intotoFixtureMap(t)
	var document cemMap
	if err := strictDecode(mapBytes, &document); err != nil {
		t.Fatal(err)
	}
	want := cemAttestationClaim{
		Name: ".corvint/change.cem.json", SHA256: shaHex(mapBytes), Spec: document.Spec,
		BaseRevision: document.BaseRevision, PatchSHA256: document.PatchSHA256, Size: uint64(len(mapBytes)),
	}
	for _, test := range []struct {
		mapBytes []byte
		status   string
	}{{mapBytes, "VERIFIED"}, {nil, "NOT_RUN"}} {
		claim, status, err := readCEMAttestation([]byte(intotoFixtureEnvelope), []byte(intotoFixturePublicKey), test.mapBytes)
		if err != nil || claim != want || status != test.status {
			t.Errorf("readCEMAttestation = %#v, %q, %v; want %#v, %q", claim, status, err, want, test.status)
		}
	}
}

// TestIntotoRefusesWhatTheSignerDidNotAttest checks FPK-V0-035: changed map
// bytes, a different key, a changed payload, an unknown envelope member, a
// case-variant or repeated statement member, and another predicate type are
// refused.
func TestIntotoRefusesWhatTheSignerDidNotAttest(t *testing.T) {
	mapBytes := intotoFixtureMap(t)
	publicKey := []byte(intotoFixturePublicKey)
	otherKey := []byte("-----BEGIN PUBLIC KEY-----\nMCowBQYDK2VwAyEAGb9ECWmEzf6FQbrBZ9w7lshQhqowtrbLDFw4rXAxZuE=\n-----END PUBLIC KEY-----\n")
	var envelope map[string]any
	if err := json.Unmarshal([]byte(intotoFixtureEnvelope), &envelope); err != nil {
		t.Fatal(err)
	}
	payload, err := base64.StdEncoding.DecodeString(envelope["payload"].(string))
	if err != nil {
		t.Fatal(err)
	}
	rewrap := func(edit func(string) string) []byte {
		copied := map[string]any{"payloadType": envelope["payloadType"], "signatures": envelope["signatures"]}
		copied["payload"] = base64.StdEncoding.EncodeToString([]byte(edit(string(payload))))
		encoded, err := json.Marshal(copied)
		if err != nil {
			t.Fatal(err)
		}
		return encoded
	}
	for name, test := range map[string]struct {
		envelope, key, mapBytes []byte
	}{
		"changed map":      {[]byte(intotoFixtureEnvelope), publicKey, append(bytes.Clone(mapBytes), '\n')},
		"other key":        {[]byte(intotoFixtureEnvelope), otherKey, mapBytes},
		"changed payload":  {rewrap(func(s string) string { return strings.Replace(s, `"size":`, `"size":1`, 1) }), publicKey, mapBytes},
		"envelope member":  {[]byte(strings.Replace(intotoFixtureEnvelope, `{"payload"`, `{"extra":1,"payload"`, 1)), publicKey, nil},
		"repeated member":  {[]byte(strings.Replace(intotoFixtureEnvelope, `{"payload"`, `{"payloadType":"x","payload"`, 1)), publicKey, nil},
		"case variant key": {[]byte(strings.Replace(intotoFixtureEnvelope, `"keyid"`, `"KeyID":"x","keyid"`, 1)), publicKey, nil},
	} {
		if _, _, err := readCEMAttestation(test.envelope, test.key, test.mapBytes); err == nil {
			t.Errorf("%s: readCEMAttestation accepted it", name)
		}
	}
	for name, edit := range map[string]func(string) string{
		"statement case variant": func(s string) string { return strings.Replace(s, `"predicate":`, `"Predicate":{},"predicate":`, 1) },
		"predicate type":         func(s string) string { return strings.Replace(s, "attestation/cem/v1", "attestation/cem/0", 1) },
		"subject name": func(s string) string {
			return strings.Replace(s, `"name":".corvint/change.cem.json"}]`, `"name":"x"}]`, 1)
		},
	} {
		claim, err := intotoCEMClaim([]byte(edit(string(payload))))
		if err == nil {
			t.Errorf("%s: intotoCEMClaim accepted %#v", name, claim)
		}
	}
}
