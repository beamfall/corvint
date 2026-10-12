package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"strconv"
)

// Runner receipt authentication (CEM-V1-084, decision 0523 P1), mirrored from
// the native verifier with the standard library only.
const (
	stableAuthNone            = "none"
	stableAuthDSSE            = "dsse-ed25519"
	stableAuthLegacy          = "NOT_OBSERVED"
	stableAttestationKind     = "receipt-attestation"
	stableAttestationPayload  = "application/vnd.in-toto+json"
	stableAttestationType     = "https://in-toto.io/Statement/v1"
	stableAttestationPredType = "https://corvint-context.dev/attestation/cem-receipt/v1"
	stableTrustedKeysProfile  = "corvint-trusted-keys/1"
	stableTrustedKeysBytes    = 64 << 10
	stableTrustedKeysMax      = 64
)

// stableAttestationSubjects are the exact subject names, in order, and the
// digest algorithm each carries.
var stableAttestationSubjects = [3][2]string{{"runner-receipt", "sha256"}, {"runner-plan", "sha256"}, {"target-tree", "gitTree"}}

// stableAuthentication is a receipt's authentication member; the zero value
// is the legacy sealed string "NOT_OBSERVED".
type stableAuthentication struct {
	Method         string
	KeyID          string
	EnvelopeSHA256 string
}

func (a stableAuthentication) dsse() bool { return a.Method == stableAuthDSSE }

func (a stableAuthentication) MarshalJSON() ([]byte, error) {
	switch a.Method {
	case "":
		return json.Marshal(stableAuthLegacy)
	case stableAuthDSSE:
		return []byte(`{"envelopeSha256":` + strconv.Quote(a.EnvelopeSHA256) + `,"keyid":` + strconv.Quote(a.KeyID) + `,"method":"dsse-ed25519"}`), nil
	}
	return []byte(`{"method":` + strconv.Quote(a.Method) + `}`), nil
}

// UnmarshalJSON reads any closed form, so an emitted result round-trips.
func (a *stableAuthentication) UnmarshalJSON(raw []byte) error {
	parsed, e := decodeStableAuthentication(raw)
	if e != nil {
		return e
	}
	*a = parsed
	return nil
}

// decodeStableAuthentication applies the closed forms: the legacy string, or
// an object whose method value selects its key set, so the refusal order is
// missing-field, unknown-field, then invalid-field.
func decodeStableAuthentication(raw json.RawMessage) (stableAuthentication, *cemError) {
	if s, ok := text03(raw); ok {
		if s != stableAuthLegacy {
			return stableAuthentication{}, invalid("invalid-field")
		}
		return stableAuthentication{}, nil
	}
	if len(raw) == 0 || bytes.TrimSpace(raw)[0] != '{' {
		return stableAuthentication{}, invalid("invalid-field")
	}
	var probe map[string]json.RawMessage
	if json.Unmarshal(raw, &probe) != nil {
		return stableAuthentication{}, invalid("invalid-field")
	}
	method, _ := text03(probe["method"])
	keys := []string{"method"}
	if method == stableAuthDSSE {
		keys = []string{"method", "keyid", "envelopeSha256"}
	}
	o, e := object03(raw, keys, nil)
	if e != nil {
		return stableAuthentication{}, e
	}
	if m, ok := text03(o["method"]); !ok || m != stableAuthNone && m != stableAuthDSSE {
		return stableAuthentication{}, invalid("invalid-field")
	}
	if method == stableAuthNone {
		return stableAuthentication{Method: stableAuthNone}, nil
	}
	a := stableAuthentication{Method: stableAuthDSSE}
	if a.KeyID, e = stableString(o, "keyid", validDigest); e != nil {
		return stableAuthentication{}, e
	}
	if a.EnvelopeSHA256, e = stableString(o, "envelopeSha256", validDigest); e != nil {
		return stableAuthentication{}, e
	}
	return a, nil
}

type stableAttestation struct {
	keyID     string
	signature []byte
	payload   []byte
	subjects  [3]string
}

// closedJSON03 decodes one strict JSON object with exactly the named keys.
func closedJSON03(raw []byte, keys ...string) (map[string]json.RawMessage, bool) {
	if strictJSON03(raw) != nil {
		return nil, false
	}
	o, e := object03(raw, keys, nil)
	return o, e == nil
}

// parseStableAttestation reads a closed DSSE envelope over a closed in-toto
// Statement v1 with the empty receipt predicate. Any shape failure is
// attestation-malformed; the caller binds keyid, subjects and signature.
func parseStableAttestation(raw []byte) (*stableAttestation, *cemError) {
	bad := invalid("attestation-malformed")
	env, ok := closedJSON03(raw, "payload", "payloadType", "signatures")
	if !ok {
		return nil, bad
	}
	payloadType, ok1 := text03(env["payloadType"])
	encoded, ok2 := text03(env["payload"])
	sigs, e := list03(env["signatures"], 1)
	if !ok1 || !ok2 || payloadType != stableAttestationPayload || e != nil || len(sigs) != 1 {
		return nil, bad
	}
	sig, e := object03(sigs[0], []string{"keyid", "sig"}, nil)
	if e != nil {
		return nil, bad
	}
	keyID, ok1 := text03(sig["keyid"])
	sigText, ok2 := text03(sig["sig"])
	if !ok1 || !ok2 || !validDigest(keyID) {
		return nil, bad
	}
	signature, err := base64.StdEncoding.Strict().DecodeString(sigText)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return nil, bad
	}
	payload, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil {
		return nil, bad
	}
	st, ok := closedJSON03(payload, "_type", "subject", "predicateType", "predicate")
	if !ok {
		return nil, bad
	}
	typ, ok1 := text03(st["_type"])
	predicateType, ok2 := text03(st["predicateType"])
	predicate, e1 := object03(st["predicate"], nil, nil)
	subjects, e2 := list03(st["subject"], len(stableAttestationSubjects))
	if !ok1 || !ok2 || typ != stableAttestationType || predicateType != stableAttestationPredType || e1 != nil || len(predicate) != 0 || e2 != nil || len(subjects) != len(stableAttestationSubjects) {
		return nil, bad
	}
	out := &stableAttestation{keyID: keyID, signature: signature, payload: payload}
	for i, want := range stableAttestationSubjects {
		s, e := object03(subjects[i], []string{"name", "digest"}, nil)
		if e != nil {
			return nil, bad
		}
		name, ok := text03(s["name"])
		if !ok || name != want[0] {
			return nil, bad
		}
		digest, e := object03(s["digest"], []string{want[1]}, nil)
		if e != nil {
			return nil, bad
		}
		value, ok := text03(digest[want[1]])
		if !ok || want[1] == "sha256" && !validDigest(value) || want[1] == "gitTree" && !validOID(value) {
			return nil, bad
		}
		out.subjects[i] = value
	}
	return out, nil
}

// stableAttestationPAE is the DSSE v1 pre-authentication encoding.
func stableAttestationPAE(payload []byte) []byte {
	pae := "DSSEv1 " + strconv.Itoa(len(stableAttestationPayload)) + " " + stableAttestationPayload + " " + strconv.Itoa(len(payload)) + " "
	return append([]byte(pae), payload...)
}

// parseStableTrustedKeys reads the closed consumer trusted-keys file:
// {"profile":"corvint-trusted-keys/1","keys":[{"keyid":K,"ed25519PublicKey":B64}]}
// with 1..64 keys, each keyid the lowercase hex sha256 of its standard-base64
// raw 32-byte key, and no keyid repeated.
func parseStableTrustedKeys(raw []byte) (map[string]ed25519.PublicKey, bool) {
	if len(raw) > stableTrustedKeysBytes {
		return nil, false
	}
	doc, ok := closedJSON03(raw, "profile", "keys")
	if !ok {
		return nil, false
	}
	profile, ok := text03(doc["profile"])
	keys, e := list03(doc["keys"], stableTrustedKeysMax)
	if !ok || profile != stableTrustedKeysProfile || e != nil || len(keys) == 0 {
		return nil, false
	}
	out := map[string]ed25519.PublicKey{}
	for _, k := range keys {
		entry, e := object03(k, []string{"keyid", "ed25519PublicKey"}, nil)
		if e != nil {
			return nil, false
		}
		id, ok1 := text03(entry["keyid"])
		text, ok2 := text03(entry["ed25519PublicKey"])
		if !ok1 || !ok2 {
			return nil, false
		}
		b, err := base64.StdEncoding.Strict().DecodeString(text)
		if err != nil || len(b) != ed25519.PublicKeySize || shaHex(b) != id {
			return nil, false
		}
		if _, dup := out[id]; dup {
			return nil, false
		}
		out[id] = ed25519.PublicKey(b)
	}
	return out, true
}

func stableHasSignedReceipt(m *stableMap) bool {
	for _, r := range m.RunnerReceipts {
		if r.Authentication.dsse() {
			return true
		}
	}
	return false
}

// stableAuthenticate binds every dsse-ed25519 receipt to its verified
// receipt-attestation bytes and returns the authentication axis. Per receipt,
// in map order: attestation-malformed, attestation-key-mismatch,
// attestation-subject-mismatch, then, only for a trusted keyid,
// attestation-signature-invalid. A key the map or its artifacts carry is never
// trusted; only the consumer file can raise the axis.
func stableAuthenticate(m *stableMap, contents map[string][]byte, targetTree string, trusted map[string]ed25519.PublicKey) (string, *cemError) {
	signed, unsigned, untrusted := 0, 0, false
	for _, r := range m.RunnerReceipts {
		if !r.Authentication.dsse() {
			unsigned++
			continue
		}
		a, e := parseStableAttestation(contents[r.Authentication.EnvelopeSHA256])
		if e != nil {
			return "", e
		}
		if a.keyID != r.Authentication.KeyID {
			return "", invalid("attestation-key-mismatch")
		}
		if a.subjects != [3]string{r.SHA256, r.PlanSHA256, targetTree} {
			return "", invalid("attestation-subject-mismatch")
		}
		key, ok := trusted[r.Authentication.KeyID]
		if !ok {
			untrusted = true
			continue
		}
		if !ed25519.Verify(key, stableAttestationPAE(a.payload), a.signature) {
			return "", invalid("attestation-signature-invalid")
		}
		signed++
	}
	switch {
	case trusted == nil || signed == 0 && !untrusted:
		return "NOT_OBSERVED", nil
	case untrusted:
		return "UNTRUSTED_KEY", nil
	case unsigned > 0:
		return "NOT_OBSERVED", nil
	}
	return "VERIFIED", nil
}

// failStableAuthentication records an attestation refusal: reference
// integrity and authentication both fail at the artifacts stage.
func failStableAuthentication(r *stableResult, e *cemError) (stableResult, int) {
	r.Axes["authentication"] = "FAILED"
	return failStable(r, "artifacts", e, true)
}

// readStableTrustedKeys reads an optional --trusted-keys path once as a
// bounded regular file at an absolute path; "" means no file was supplied.
func readStableTrustedKeys(ctx context.Context, path string) (map[string]ed25519.PublicKey, bool) {
	if path == "" {
		return nil, true
	}
	if !filepath.IsAbs(path) {
		return nil, false
	}
	raw, e := readBounded(ctx, path, stableTrustedKeysBytes)
	if e != nil {
		return nil, false
	}
	return parseStableTrustedKeys(raw)
}
