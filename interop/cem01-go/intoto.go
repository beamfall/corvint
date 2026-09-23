package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
)

// This file reads a Corvint CEM attestation from its published wire alone: a
// DSSE envelope (payloadType application/vnd.in-toto+json, one Ed25519
// signature over the DSSE pre-authentication encoding) whose payload is an
// in-toto Statement v1 with the versioned predicateType below. It uses only the
// standard library, runs no Git, and opens no network connection.
const (
	intotoPayloadType   = "application/vnd.in-toto+json"
	intotoStatementType = "https://in-toto.io/Statement/v1"
	cemPredicateTypeV1  = "https://corvint-context.dev/attestation/cem/v1"
)

// cemAttestationClaim is what a verified cem/v1 statement asserts about a map.
type cemAttestationClaim struct {
	Name, SHA256, Spec, BaseRevision, PatchSHA256 string
	Size                                          uint64
}

// readCEMAttestation verifies envelope against the PKIX Ed25519 public key in
// publicKeyPEM and returns the signed cem/v1 claim with status VERIFIED when
// mapBytes has the signed sha256, size, spec, baseRevision and patchSha256, or
// NOT_RUN when mapBytes is nil.
func readCEMAttestation(envelope, publicKeyPEM, mapBytes []byte) (cemAttestationClaim, string, error) {
	publicKey, err := intotoPublicKey(publicKeyPEM)
	if err != nil {
		return cemAttestationClaim{}, "", err
	}
	payload, err := intotoVerifyEnvelope(envelope, publicKey)
	if err != nil {
		return cemAttestationClaim{}, "", err
	}
	claim, err := intotoCEMClaim(payload)
	if err != nil {
		return cemAttestationClaim{}, "", err
	}
	if mapBytes == nil {
		return claim, "NOT_RUN", nil
	}
	if err := intotoCheckMap(claim, mapBytes); err != nil {
		return cemAttestationClaim{}, "", err
	}
	return claim, "VERIFIED", nil
}

func intotoPublicKey(publicKeyPEM []byte) (ed25519.PublicKey, error) {
	block, rest := pem.Decode(publicKeyPEM)
	if block == nil || block.Type != "PUBLIC KEY" || len(bytes.TrimSpace(rest)) != 0 {
		return nil, errors.New("public key is not one PKIX PEM block")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	publicKey, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("public key is not Ed25519")
	}
	return publicKey, nil
}

// intotoVerifyEnvelope accepts exactly the envelope members payload,
// payloadType and signatures, one signature whose keyid is the lowercase hex
// sha256 of the raw public key, and returns the payload once the signature
// verifies over PAE(payloadType, payload).
func intotoVerifyEnvelope(envelope []byte, publicKey ed25519.PublicKey) ([]byte, error) {
	members, err := intotoObject(envelope, "payload", "payloadType", "signatures")
	if err != nil {
		return nil, fmt.Errorf("envelope: %w", err)
	}
	if len(members) != 3 {
		return nil, errors.New("envelope members are not exactly payload, payloadType, signatures")
	}
	payloadType, err := intotoString(members, "payloadType")
	if err != nil || payloadType != intotoPayloadType {
		return nil, errors.New("envelope payloadType is not " + intotoPayloadType)
	}
	var signatures []json.RawMessage
	if err := json.Unmarshal(members["signatures"], &signatures); err != nil || len(signatures) != 1 {
		return nil, errors.New("envelope does not carry exactly one signature")
	}
	signature, err := intotoObject(signatures[0], "keyid", "sig")
	if err != nil {
		return nil, fmt.Errorf("signature: %w", err)
	}
	keyID, err := intotoString(signature, "keyid")
	if err != nil || keyID != shaHex(publicKey) {
		return nil, errors.New("signature keyid is not the sha256 of the public key")
	}
	payload, err := intotoBase64(members, "payload")
	if err != nil {
		return nil, err
	}
	sig, err := intotoBase64(signature, "sig")
	if err != nil {
		return nil, err
	}
	pae := fmt.Sprintf("DSSEv1 %d %s %d ", len(payloadType), payloadType, len(payload))
	if !ed25519.Verify(publicKey, append([]byte(pae), payload...), sig) {
		return nil, errors.New("signature does not verify")
	}
	return payload, nil
}

// intotoCEMClaim reads the statement's defined members, ignoring others as an
// in-toto consumer must, and refuses a claim that is not one cem/v1 subject
// equal to the predicate's cem descriptor with well-formed values.
func intotoCEMClaim(payload []byte) (cemAttestationClaim, error) {
	statement, err := intotoObject(payload, "_type", "subject", "predicateType", "predicate")
	if err != nil {
		return cemAttestationClaim{}, fmt.Errorf("statement: %w", err)
	}
	statementType, _ := intotoString(statement, "_type")
	predicateType, _ := intotoString(statement, "predicateType")
	if statementType != intotoStatementType || predicateType != cemPredicateTypeV1 {
		return cemAttestationClaim{}, errors.New("statement is not an in-toto v1 cem/v1 statement")
	}
	var subjects []json.RawMessage
	if err := json.Unmarshal(statement["subject"], &subjects); err != nil || len(subjects) != 1 {
		return cemAttestationClaim{}, errors.New("statement does not have exactly one subject")
	}
	predicate, err := intotoObject(statement["predicate"], "base", "cem", "patch", "size", "spec")
	if err != nil {
		return cemAttestationClaim{}, fmt.Errorf("predicate: %w", err)
	}
	subjectName, subjectDigest, err := intotoDescriptor(subjects[0], "sha256")
	if err != nil {
		return cemAttestationClaim{}, fmt.Errorf("subject: %w", err)
	}
	var claim cemAttestationClaim
	claim.Name, claim.SHA256, err = intotoDescriptor(predicate["cem"], "sha256")
	if err != nil {
		return cemAttestationClaim{}, fmt.Errorf("predicate cem: %w", err)
	}
	if subjectName != claim.Name || subjectDigest != claim.SHA256 {
		return cemAttestationClaim{}, errors.New("subject and predicate cem disagree")
	}
	if _, claim.BaseRevision, err = intotoDescriptor(predicate["base"], "gitCommit"); err != nil {
		return cemAttestationClaim{}, fmt.Errorf("predicate base: %w", err)
	}
	if _, claim.PatchSHA256, err = intotoDescriptor(predicate["patch"], "sha256"); err != nil {
		return cemAttestationClaim{}, fmt.Errorf("predicate patch: %w", err)
	}
	if claim.Size, err = parseWireUint(predicate["size"]); err != nil {
		return cemAttestationClaim{}, errors.New("predicate size is not a wire integer")
	}
	claim.Spec, _ = intotoString(predicate, "spec")
	return claim, intotoClaimShape(claim)
}

func intotoClaimShape(claim cemAttestationClaim) error {
	if claim.Name == "" || claim.Spec == "" {
		return errors.New("claim name or spec is empty")
	}
	if !validDigest(claim.SHA256) || !validDigest(claim.PatchSHA256) {
		return errors.New("claim sha256 is not 64 lowercase hex digits")
	}
	if !validOID(claim.BaseRevision) {
		return errors.New("claim base is not a full lowercase Git commit OID")
	}
	return nil
}

// intotoCheckMap compares the supplied map with the signed claim: its digest
// and size, then the map's own spec, baseRevision and patchSha256 members.
func intotoCheckMap(claim cemAttestationClaim, mapBytes []byte) error {
	if shaHex(mapBytes) != claim.SHA256 || uint64(len(mapBytes)) != claim.Size {
		return errors.New("map bytes differ from the signed sha256 or size")
	}
	document, err := intotoObject(mapBytes, "spec", "baseRevision", "patchSha256")
	if err != nil {
		return fmt.Errorf("map: %w", err)
	}
	spec, _ := intotoString(document, "spec")
	base, _ := intotoString(document, "baseRevision")
	patch, _ := intotoString(document, "patchSha256")
	if spec != claim.Spec || base != claim.BaseRevision || patch != claim.PatchSHA256 {
		return errors.New("map spec, baseRevision or patchSha256 differ from the signed claim")
	}
	return nil
}

// intotoObject decodes raw as one JSON object with unique member names and
// refuses a member name that differs from a defined one only in case, so a
// case-insensitive reader could not see a different claim.
func intotoObject(raw []byte, defined ...string) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := scanJSONValue(decoder); err != nil {
		return nil, err
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil || members == nil {
		return nil, errors.New("not one JSON object")
	}
	for name := range members {
		for _, want := range defined {
			if name != want && strings.EqualFold(name, want) {
				return nil, fmt.Errorf("member %q differs from %q only in case", name, want)
			}
		}
	}
	return members, nil
}

func intotoString(members map[string]json.RawMessage, name string) (string, error) {
	var value string
	err := json.Unmarshal(members[name], &value)
	return value, err
}

func intotoBase64(members map[string]json.RawMessage, name string) ([]byte, error) {
	encoded, err := intotoString(members, name)
	if err != nil {
		return nil, fmt.Errorf("%s is not a string", name)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("%s is not base64: %w", name, err)
	}
	return decoded, nil
}

// intotoDescriptor reads an in-toto ResourceDescriptor's optional name and the
// digest value for algorithm; other digest algorithms are ignored.
func intotoDescriptor(raw json.RawMessage, algorithm string) (string, string, error) {
	descriptor, err := intotoObject(raw, "name", "digest")
	if err != nil {
		return "", "", err
	}
	digest, err := intotoObject(descriptor["digest"], algorithm)
	if err != nil {
		return "", "", fmt.Errorf("digest: %w", err)
	}
	value, err := intotoString(digest, algorithm)
	if err != nil {
		return "", "", fmt.Errorf("digest has no %s string", algorithm)
	}
	name := ""
	if _, present := descriptor["name"]; present {
		if name, err = intotoString(descriptor, "name"); err != nil {
			return "", "", errors.New("name is not a string")
		}
	}
	return name, value, nil
}
