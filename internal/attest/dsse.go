package attest

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// payloadType is the DSSE payloadType this package signs and verifies: an
// in-toto Statement encoded as JSON.
const payloadType = "application/vnd.in-toto+json"

// envelopeSignature is one entry of a DSSE envelope's "signatures" array.
type envelopeSignature struct {
	Sig   string `json:"sig"`
	KeyID string `json:"keyid"`
}

// dsseEnvelope is the DSSE envelope wire shape
// (https://github.com/secure-systems-lab/dsse).
type dsseEnvelope struct {
	Payload     string              `json:"payload"`
	PayloadType string              `json:"payloadType"`
	Signatures  []envelopeSignature `json:"signatures"`
}

// Envelope wraps statement in a DSSE envelope, signed with Ed25519 over the
// DSSE pre-authentication encoding (PAE) of (payloadType, statement). The
// envelope's payloadType is fixed at "application/vnd.in-toto+json". keyid
// is the lowercase hex sha256 of the raw Ed25519 public key bytes.
func Envelope(statement []byte, privateKey ed25519.PrivateKey) ([]byte, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("attest: private key has size %d, want %d", len(privateKey), ed25519.PrivateKeySize)
	}
	publicKey, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("attest: private key has no Ed25519 public key")
	}
	signature := ed25519.Sign(privateKey, preAuthenticationEncoding(payloadType, statement))
	envelope := dsseEnvelope{
		Payload:     base64.StdEncoding.EncodeToString(statement),
		PayloadType: payloadType,
		Signatures: []envelopeSignature{{
			Sig:   base64.StdEncoding.EncodeToString(signature),
			KeyID: keyID(publicKey),
		}},
	}
	return json.Marshal(envelope)
}

// Verify checks a DSSE envelope's single signature against publicKey and
// returns the statement payload. It rejects: a public key that is not 32
// bytes; a payloadType other than
// "application/vnd.in-toto+json"; zero or multiple signatures; a keyid that
// does not match the lowercase hex sha256 of publicKey; payload or signature
// bytes that are not valid base64; a signature that does not verify; any
// envelope member other than payload, payloadType, and signatures; and, in the
// envelope or its signature object, a repeated member name or one that differs
// from a defined member name only in case.
func Verify(envelope []byte, publicKey ed25519.PublicKey) ([]byte, error) {
	if len(publicKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("attest: public key has size %d, want %d", len(publicKey), ed25519.PublicKeySize)
	}
	var members map[string]jsontext.Value
	if err := jsonv2.Unmarshal(envelope, &members); err != nil {
		return nil, fmt.Errorf("attest: parse envelope: %w", err)
	}
	for member := range members {
		switch member {
		case "payload", "payloadType", "signatures":
		default:
			return nil, fmt.Errorf("attest: envelope has unknown member %q", member)
		}
	}

	var parsed dsseEnvelope
	if err := strictUnmarshal(envelope, &parsed); err != nil {
		return nil, fmt.Errorf("attest: parse envelope: %w", err)
	}
	if parsed.PayloadType != payloadType {
		return nil, fmt.Errorf("attest: envelope has payloadType %q, want %q", parsed.PayloadType, payloadType)
	}
	if len(parsed.Signatures) != 1 {
		return nil, fmt.Errorf("attest: envelope has %d signatures, want exactly 1", len(parsed.Signatures))
	}
	signature := parsed.Signatures[0]
	if signature.KeyID != keyID(publicKey) {
		return nil, errors.New("attest: signature keyid does not match the given public key")
	}
	payload, err := base64.StdEncoding.DecodeString(parsed.Payload)
	if err != nil {
		return nil, fmt.Errorf("attest: decode payload base64: %w", err)
	}
	signatureBytes, err := base64.StdEncoding.DecodeString(signature.Sig)
	if err != nil {
		return nil, fmt.Errorf("attest: decode signature base64: %w", err)
	}
	if !ed25519.Verify(publicKey, preAuthenticationEncoding(payloadType, payload), signatureBytes) {
		return nil, errors.New("attest: signature does not verify")
	}
	return payload, nil
}

// strictUnmarshal decodes data into the struct v points to with
// encoding/json/v2, which matches member names exactly and refuses a repeated
// name at any depth, and then refuses a member whose name differs from a
// field's json name only in case, which a case-insensitive reader would bind to
// that field. Members v does not define are ignored.
func strictUnmarshal(data []byte, v any) error {
	if err := jsonv2.Unmarshal(data, v); err != nil {
		return err
	}
	return refuseCaseVariantMembers(jsontext.Value(data), reflect.TypeOf(v).Elem())
}

// refuseCaseVariantMembers walks value, already decoded into typ, through its
// struct fields and slice elements.
func refuseCaseVariantMembers(value jsontext.Value, typ reflect.Type) error {
	switch typ.Kind() {
	case reflect.Slice:
		var elements []jsontext.Value
		if err := jsonv2.Unmarshal(value, &elements); err != nil {
			return err
		}
		for _, element := range elements {
			if err := refuseCaseVariantMembers(element, typ.Elem()); err != nil {
				return err
			}
		}
	case reflect.Struct:
		var members map[string]jsontext.Value
		if err := jsonv2.Unmarshal(value, &members); err != nil {
			return err
		}
		for field := range typ.Fields() {
			if err := refuseCaseVariantField(members, field); err != nil {
				return err
			}
		}
	}
	return nil
}

// refuseCaseVariantField refuses a member of members that names field in
// another case, and walks the exactly named member, when present.
func refuseCaseVariantField(members map[string]jsontext.Value, field reflect.StructField) error {
	name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
	for member := range members {
		if member != name && strings.EqualFold(member, name) {
			return fmt.Errorf("member %q differs from %q only in case", member, name)
		}
	}
	value, present := members[name]
	if !present {
		return nil
	}
	return refuseCaseVariantMembers(value, field.Type)
}

// preAuthenticationEncoding computes the DSSE PAE:
// "DSSEv1" SP len(payloadType) SP payloadType SP len(payload) SP payload.
func preAuthenticationEncoding(payloadType string, payload []byte) []byte {
	pae := make([]byte, 0, len(payloadType)+len(payload)+32)
	pae = append(pae, "DSSEv1 "...)
	pae = append(pae, strconv.Itoa(len(payloadType))...)
	pae = append(pae, ' ')
	pae = append(pae, payloadType...)
	pae = append(pae, ' ')
	pae = append(pae, strconv.Itoa(len(payload))...)
	pae = append(pae, ' ')
	pae = append(pae, payload...)
	return pae
}

// keyID is the lowercase hex sha256 of the raw Ed25519 public key bytes.
func keyID(publicKey ed25519.PublicKey) string {
	sum := sha256.Sum256(publicKey)
	return hex.EncodeToString(sum[:])
}
