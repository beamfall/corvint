package wire

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strconv"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// Runner receipt authentication (CEM-V1-084, decision 0523 P1).
const (
	// StableAuthenticationNone and StableAuthenticationDSSE are the two
	// receipt authentication methods.
	StableAuthenticationNone = "none"
	StableAuthenticationDSSE = "dsse-ed25519"
	// StableLegacyAuthentication is the sealed S0E string form, read as
	// method none and echoed unchanged.
	StableLegacyAuthentication = "NOT_OBSERVED"
	// ReceiptAttestationKind is the artifact kind an envelopeSha256 names.
	ReceiptAttestationKind = "receipt-attestation"
	// ReceiptAttestationPayloadType is the DSSE payloadType of the envelope.
	ReceiptAttestationPayloadType = "application/vnd.in-toto+json"
	// ReceiptAttestationStatementType is the in-toto Statement v1 type.
	ReceiptAttestationStatementType = "https://in-toto.io/Statement/v1"
	// ReceiptAttestationPredicateType names the empty receipt predicate.
	ReceiptAttestationPredicateType = "https://corvint-context.dev/attestation/cem-receipt/v1"
	// TrustedKeysProfile is the closed consumer trusted-keys file profile.
	TrustedKeysProfile = "corvint-trusted-keys/1"
	// MaxTrustedKeysBytes bounds the trusted-keys file.
	MaxTrustedKeysBytes = 64 << 10
	// MaxTrustedKeys bounds the number of trusted keys.
	MaxTrustedKeys = 64
)

// ReceiptAttestationSubjects are the exact subject names, in order, and the
// digest algorithm each carries.
var ReceiptAttestationSubjects = [3][2]string{{"runner-receipt", "sha256"}, {"runner-plan", "sha256"}, {"target-tree", "gitTree"}}

// StableAuthentication is a runner receipt's authentication member. The zero
// value is the legacy sealed string "NOT_OBSERVED"; Method "none" is
// {"method":"none"}; Method "dsse-ed25519" carries KeyID and EnvelopeSha256.
type StableAuthentication struct {
	Method         string
	KeyID          string
	EnvelopeSha256 string
}

// DSSE reports whether the receipt names a signed attestation.
func (a StableAuthentication) DSSE() bool { return a.Method == StableAuthenticationDSSE }

// String renders the member for human views.
func (a StableAuthentication) String() string {
	switch a.Method {
	case "":
		return StableLegacyAuthentication
	case StableAuthenticationDSSE:
		return a.Method + " keyid " + a.KeyID + " envelope " + a.EnvelopeSha256
	}
	return a.Method
}

func (a StableAuthentication) MarshalJSON() ([]byte, error) {
	switch a.Method {
	case "":
		return json.Marshal(StableLegacyAuthentication)
	case StableAuthenticationDSSE:
		return []byte(`{"envelopeSha256":` + strconv.Quote(a.EnvelopeSha256) + `,"keyid":` + strconv.Quote(a.KeyID) + `,"method":"dsse-ed25519"}`), nil
	}
	return []byte(`{"method":` + strconv.Quote(a.Method) + `}`), nil
}

func (a *StableAuthentication) UnmarshalJSON(b []byte) error {
	v, err := Parse(b)
	if err != nil {
		return err
	}
	parsed, err := parseStableAuthentication(v)
	if err != nil {
		return err
	}
	*a = parsed
	return nil
}

// parseStableAuthentication applies the member's closed forms. The method
// value selects the key set, so the refusal order stays missing-field, then
// unknown-field, then invalid-field.
func parseStableAuthentication(v Value) (StableAuthentication, error) {
	if v.Kind == KindString {
		if v.Str != StableLegacyAuthentication {
			return StableAuthentication{}, fieldError("runner authentication")
		}
		return StableAuthentication{}, nil
	}
	if v.Kind != KindObject {
		return StableAuthentication{}, fieldError("runner authentication")
	}
	method, _ := v.Obj.Get("method")
	keys := []string{"method"}
	if method.Kind == KindString && method.Str == StableAuthenticationDSSE {
		keys = []string{"method", "keyid", "envelopeSha256"}
	}
	if err := requireClosedKeys(v.Obj, keys); err != nil {
		return StableAuthentication{}, err
	}
	if method.Kind != KindString || method.Str != StableAuthenticationNone && method.Str != StableAuthenticationDSSE {
		return StableAuthentication{}, fieldError("runner authentication method")
	}
	if method.Str == StableAuthenticationNone {
		return StableAuthentication{Method: StableAuthenticationNone}, nil
	}
	keyID, _ := v.Obj.Get("keyid")
	envelope, _ := v.Obj.Get("envelopeSha256")
	if keyID.Kind != KindString || !IsSha256(keyID.Str) {
		return StableAuthentication{}, fieldError("runner authentication keyid")
	}
	if envelope.Kind != KindString || !IsSha256(envelope.Str) {
		return StableAuthentication{}, fieldError("runner authentication envelope digest")
	}
	return StableAuthentication{Method: StableAuthenticationDSSE, KeyID: keyID.Str, EnvelopeSha256: envelope.Str}, nil
}

// ReceiptAttestation is one parsed receipt-attestation envelope: the single
// DSSE signature and the three Statement subjects, in their fixed order.
type ReceiptAttestation struct {
	KeyID     string
	Signature []byte
	Payload   []byte
	Subjects  [3]string
}

func attestationMalformed(what string) error {
	return cemcode.New("attestation-malformed", "receipt attestation %s", what)
}

// ParseReceiptAttestation reads a closed DSSE envelope over a closed in-toto
// Statement v1 with the receipt predicate. It checks shape only; the caller
// binds keyid, subjects and signature.
func ParseReceiptAttestation(raw []byte) (*ReceiptAttestation, error) {
	env, err := Parse(raw)
	if err != nil || env.Kind != KindObject || requireClosedKeys(env.Obj, []string{"payload", "payloadType", "signatures"}) != nil {
		return nil, attestationMalformed("envelope")
	}
	payloadType, _ := env.Obj.Get("payloadType")
	encoded, _ := env.Obj.Get("payload")
	signatures, _ := env.Obj.Get("signatures")
	if payloadType.Kind != KindString || payloadType.Str != ReceiptAttestationPayloadType || encoded.Kind != KindString || signatures.Kind != KindArray || len(signatures.Arr) != 1 {
		return nil, attestationMalformed("envelope")
	}
	sig := signatures.Arr[0]
	if sig.Kind != KindObject || requireClosedKeys(sig.Obj, []string{"keyid", "sig"}) != nil {
		return nil, attestationMalformed("signature")
	}
	keyID, _ := sig.Obj.Get("keyid")
	sigText, _ := sig.Obj.Get("sig")
	if keyID.Kind != KindString || !IsSha256(keyID.Str) || sigText.Kind != KindString {
		return nil, attestationMalformed("signature")
	}
	signature, err := base64.StdEncoding.Strict().DecodeString(sigText.Str)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return nil, attestationMalformed("signature")
	}
	payload, err := base64.StdEncoding.Strict().DecodeString(encoded.Str)
	if err != nil {
		return nil, attestationMalformed("payload")
	}
	st, err := Parse(payload)
	if err != nil || st.Kind != KindObject || requireClosedKeys(st.Obj, []string{"_type", "subject", "predicateType", "predicate"}) != nil {
		return nil, attestationMalformed("statement")
	}
	typ, _ := st.Obj.Get("_type")
	predicateType, _ := st.Obj.Get("predicateType")
	predicate, _ := st.Obj.Get("predicate")
	subjects, _ := st.Obj.Get("subject")
	if typ.Kind != KindString || typ.Str != ReceiptAttestationStatementType || predicateType.Kind != KindString || predicateType.Str != ReceiptAttestationPredicateType || predicate.Kind != KindObject || len(predicate.Obj.Keys) != 0 || subjects.Kind != KindArray || len(subjects.Arr) != len(ReceiptAttestationSubjects) {
		return nil, attestationMalformed("statement")
	}
	out := &ReceiptAttestation{KeyID: keyID.Str, Signature: signature, Payload: payload}
	for i, want := range ReceiptAttestationSubjects {
		s := subjects.Arr[i]
		if s.Kind != KindObject || requireClosedKeys(s.Obj, []string{"name", "digest"}) != nil {
			return nil, attestationMalformed("subject")
		}
		name, _ := s.Obj.Get("name")
		digest, _ := s.Obj.Get("digest")
		if name.Kind != KindString || name.Str != want[0] || digest.Kind != KindObject || requireClosedKeys(digest.Obj, []string{want[1]}) != nil {
			return nil, attestationMalformed("subject")
		}
		value, _ := digest.Obj.Get(want[1])
		if value.Kind != KindString || want[1] == "sha256" && !IsSha256(value.Str) || want[1] == "gitTree" && !IsGitOid(value.Str) {
			return nil, attestationMalformed("subject")
		}
		out.Subjects[i] = value.Str
	}
	return out, nil
}

// ReceiptAttestationPAE is the DSSE v1 pre-authentication encoding the
// signature covers.
func ReceiptAttestationPAE(payload []byte) []byte {
	pae := "DSSEv1 " + strconv.Itoa(len(ReceiptAttestationPayloadType)) + " " + ReceiptAttestationPayloadType + " " + strconv.Itoa(len(payload)) + " "
	return append([]byte(pae), payload...)
}

// Ed25519KeyID is the lowercase hex SHA-256 of the raw 32-byte public key.
func Ed25519KeyID(publicKey ed25519.PublicKey) string {
	sum := sha256.Sum256(publicKey)
	return hex.EncodeToString(sum[:])
}

// ParseTrustedKeys reads the closed consumer trusted-keys file:
// {"profile":"corvint-trusted-keys/1","keys":[{"keyid":K,"ed25519PublicKey":B64}]}
// with 1..64 keys, each keyid the Ed25519KeyID of its standard-base64 raw
// 32-byte key and no keyid repeated.
func ParseTrustedKeys(raw []byte) (map[string]ed25519.PublicKey, error) {
	bad := func(what string) error { return cemcode.New("invalid-arguments", "trusted keys %s", what) }
	if len(raw) > MaxTrustedKeysBytes {
		return nil, bad("file bound")
	}
	v, err := Parse(raw)
	if err != nil || v.Kind != KindObject || requireClosedKeys(v.Obj, []string{"profile", "keys"}) != nil {
		return nil, bad("document")
	}
	profile, _ := v.Obj.Get("profile")
	keys, _ := v.Obj.Get("keys")
	if profile.Kind != KindString || profile.Str != TrustedKeysProfile || keys.Kind != KindArray || len(keys.Arr) == 0 || len(keys.Arr) > MaxTrustedKeys {
		return nil, bad("document")
	}
	out := map[string]ed25519.PublicKey{}
	for _, k := range keys.Arr {
		if k.Kind != KindObject || requireClosedKeys(k.Obj, []string{"keyid", "ed25519PublicKey"}) != nil {
			return nil, bad("entry")
		}
		id, _ := k.Obj.Get("keyid")
		key, _ := k.Obj.Get("ed25519PublicKey")
		if id.Kind != KindString || key.Kind != KindString {
			return nil, bad("entry")
		}
		b, err := base64.StdEncoding.Strict().DecodeString(key.Str)
		if err != nil || len(b) != ed25519.PublicKeySize || Ed25519KeyID(b) != id.Str {
			return nil, bad("entry")
		}
		if _, dup := out[id.Str]; dup {
			return nil, bad("duplicate keyid")
		}
		out[id.Str] = ed25519.PublicKey(b)
	}
	return out, nil
}
