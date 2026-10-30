package attest

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

// retiredCEMPredicateTypes are the cem/0 (FPK-V0-030) and cem/v1 (FPK-V0-033)
// predicate URIs. They attested a retired cem/0.x map, so their builders and
// readers are removed and a signed envelope of either is refused by name
// (CEM-V1-218); cem/v2 is the only CEM predicate.
var retiredCEMPredicateTypes = map[string]bool{
	"https://corvint-context.dev/attestation/cem/0":  true,
	"https://corvint-context.dev/attestation/cem/v1": true,
}

// CEM verification statuses. NOT_RUN means the envelope and the signed claim
// verified but no CEM bytes were supplied, so the map itself was not checked.
const (
	CEMVerified = "VERIFIED"
	CEMNotRun   = "NOT_RUN"
)

// ErrCEMBytesMismatch is wrapped by the verification error for supplied CEM
// bytes whose sha256 differs from the signed claim, so a caller can tell a
// changed map from an envelope that does not verify.
var ErrCEMBytesMismatch = errors.New("attest: CEM bytes do not match the attested claim")

// cemBytesNotSupplied is the Reason of a NOT_RUN CEMVerification.
const cemBytesNotSupplied = "cem-bytes-not-supplied"

// CEMVerification is what VerifyCEMPredicateWith established about a signed
// cem/v2 claim.
type CEMVerification struct {
	Name          string
	SHA256        string
	Size          int64
	Spec          string
	Status        string
	Reason        string
	PredicateType string
	BaseRevision  string
	PatchSHA256   string
	V2            *CEMV2Claim
}

type cemDigest struct {
	SHA256 string `json:"sha256"`
}

type cemSubject struct {
	Digest cemDigest `json:"digest"`
	Name   string    `json:"name"`
}

type predicateTypeHeader struct {
	PredicateType string `json:"predicateType"`
}

// VerifyCEMPredicate is VerifyCEMPredicateWith checking only the map bytes; it
// checks no artifact or target.
func VerifyCEMPredicate(envelope []byte, publicKey ed25519.PublicKey, cem []byte) (CEMVerification, error) {
	return VerifyCEMPredicateWith(envelope, publicKey, CEMCheck{Map: cem})
}

// checkCEMClaim refuses a claim CEMStatementV2 could not have produced: a
// sha256 that is not 64 lowercase hex digits, a negative size, or an empty
// name or spec.
func checkCEMClaim(claim CEMVerification) (CEMVerification, error) {
	if !lowerHexSHA256(claim.SHA256) {
		return CEMVerification{}, errors.New("attest: CEM claim sha256 is not 64 lowercase hex digits")
	}
	if claim.Size < 0 {
		return CEMVerification{}, errors.New("attest: CEM claim size is negative")
	}
	if claim.Name == "" {
		return CEMVerification{}, errors.New("attest: CEM claim name is empty")
	}
	if claim.Spec == "" {
		return CEMVerification{}, errors.New("attest: CEM claim spec is empty")
	}
	return claim, nil
}

// lowerHexSHA256 reports whether digest is the 64 lowercase hex digits
// CEMStatementV2 writes for a sha256.
func lowerHexSHA256(digest string) bool {
	if len(digest) != hex.EncodedLen(sha256.Size) {
		return false
	}
	return strings.Trim(digest, "0123456789abcdef") == ""
}
