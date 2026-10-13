package attest

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// CEMPredicateType is the in-toto predicateType URI for an attestation about
// one Change Evidence Map (docs/specs/falsifiable-packet-v0.md, FPK-V0-030,
// experimental prototype).
const CEMPredicateType = "https://corvint-context.dev/attestation/cem/0"

// CEM verification statuses. NOT_RUN means the envelope and the signed claim
// verified but no CEM bytes were supplied, so the map itself was not checked.
const (
	CEMVerified = "VERIFIED"
	CEMNotRun   = "NOT_RUN"
)

// ErrCEMBytesMismatch is wrapped by the VerifyCEM error for supplied CEM bytes
// whose sha256 or size differ from the signed claim, so a caller can tell a
// changed map from an envelope that does not verify.
var ErrCEMBytesMismatch = errors.New("attest: CEM bytes do not match the attested claim")

// cemBytesNotSupplied is the Reason of a NOT_RUN CEMVerification.
const cemBytesNotSupplied = "cem-bytes-not-supplied"

// CEMVerification is what VerifyCEM established about a signed CEM claim.
type CEMVerification struct {
	Name   string
	SHA256 string
	Size   int64
	Spec   string
	Status string
	Reason string
	// PredicateType is set by VerifyCEMPredicate only; BaseRevision and
	// PatchSHA256 only for a CEMPredicateTypeV1 or CEMPredicateTypeV2 claim.
	PredicateType string
	BaseRevision  string
	PatchSHA256   string
	// V2 is set only for a CEMPredicateTypeV2 claim.
	V2 *CEMV2Claim
}

type cemDigest struct {
	SHA256 string `json:"sha256"`
}

type cemPredicate struct {
	Digest cemDigest `json:"digest"`
	Name   string    `json:"name"`
	Size   int64     `json:"size"`
	Spec   string    `json:"spec"`
}

type cemSubject struct {
	Digest cemDigest `json:"digest"`
	Name   string    `json:"name"`
}

type cemStatement struct {
	Type          string       `json:"_type"`
	Predicate     cemPredicate `json:"predicate"`
	PredicateType string       `json:"predicateType"`
	Subject       []cemSubject `json:"subject"`
}

// CEMStatement built an in-toto Statement v1 whose single subject is the CEM
// at name with the sha256 of cem, and whose predicate is a content-addressed
// reference to that map: name, sha256 digest, byte size, and CEM spec. The
// cem/0 predicate attests a cem/0.x map, which is retired, and never a cem/1.0
// map (CEM-V1-065), so it now refuses an empty name and every map
// (CEM-V1-210); VerifyCEM still verifies an existing signed claim, but refuses
// every supplied map.
func CEMStatement(name string, cem []byte) ([]byte, error) {
	if name == "" {
		return nil, errors.New("attest: CEM name must be non-empty")
	}
	return nil, retiredPredicateMap(cem)
}

// retiredPredicateMap refuses the map a cem/0 or cem/v1 predicate names: a
// cem/0.x map with the retired-spec fix (CEM-V1-200) and a cem/1.0 map because
// cem/v2 is its only predicate (CEM-V1-065).
func retiredPredicateMap(cem []byte) error {
	if _, err := wire.ParseStable(cem); err != nil {
		return fmt.Errorf("attest: parse CEM: %w", err)
	}
	return fmt.Errorf("attest: parse CEM: %w", cemcode.New(cemcode.UnsupportedSpec, "the cem/0 and cem/v1 predicates attest only a retired cem/0.x map; %s attests a cem/1.0 map", CEMPredicateTypeV2))
}

// VerifyCEM verifies envelope with Verify, requires its statement to be a
// CEMStatement whose subject and predicate agree, and then checks cem against
// the signed digest and size. A nil cem is not a failure: the claim is returned
// with Status NOT_RUN so the caller cannot mistake it for a checked map. A
// non-nil cem, including an empty one, that does not match is refused.
func VerifyCEM(envelope []byte, publicKey ed25519.PublicKey, cem []byte) (CEMVerification, error) {
	payload, err := Verify(envelope, publicKey)
	if err != nil {
		return CEMVerification{}, err
	}
	claim, err := parseCEMStatement(payload)
	if err != nil {
		return CEMVerification{}, err
	}
	return checkCEMBytes(claim, cem)
}

// checkCEMBytes returns claim NOT_RUN for nil cem. Supplied bytes must have the
// signed sha256 and size, and are then refused: a cem/0 or cem/v1 claim names
// a retired cem/0.x map, so neither is ever VERIFIED against map bytes
// (CEM-V1-210).
func checkCEMBytes(claim CEMVerification, cem []byte) (CEMVerification, error) {
	if cem == nil {
		claim.Status, claim.Reason = CEMNotRun, cemBytesNotSupplied
		return claim, nil
	}
	sum := sha256.Sum256(cem)
	if hex.EncodeToString(sum[:]) != claim.SHA256 {
		return CEMVerification{}, fmt.Errorf("%w: sha256", ErrCEMBytesMismatch)
	}
	if int64(len(cem)) != claim.Size {
		return CEMVerification{}, fmt.Errorf("%w: size", ErrCEMBytesMismatch)
	}
	return CEMVerification{}, checkRetiredClaimMap(cem)
}

// parseCEMStatement reads a verified payload as a CEM statement and returns
// its claim without a Status. A repeated member name, or one that differs from
// a statement member only in case, is refused; other members are ignored.
func parseCEMStatement(payload []byte) (CEMVerification, error) {
	var statement cemStatement
	if err := strictUnmarshal(payload, &statement); err != nil {
		return CEMVerification{}, fmt.Errorf("attest: parse CEM statement: %w", err)
	}
	if statement.Type != statementType {
		return CEMVerification{}, fmt.Errorf("attest: statement _type %q, want %q", statement.Type, statementType)
	}
	if statement.PredicateType != CEMPredicateType {
		return CEMVerification{}, fmt.Errorf("attest: predicateType %q, want %q", statement.PredicateType, CEMPredicateType)
	}
	if len(statement.Subject) != 1 {
		return CEMVerification{}, fmt.Errorf("attest: CEM statement has %d subjects, want exactly 1", len(statement.Subject))
	}
	predicate := statement.Predicate
	if statement.Subject[0] != (cemSubject{Digest: predicate.Digest, Name: predicate.Name}) {
		return CEMVerification{}, errors.New("attest: CEM subject and predicate disagree")
	}
	return checkCEMClaim(CEMVerification{
		Name: predicate.Name, SHA256: predicate.Digest.SHA256, Size: predicate.Size, Spec: predicate.Spec,
	})
}

// checkCEMClaim refuses a claim CEMStatement or CEMStatementV1 could not have
// produced: a sha256 that is not 64 lowercase hex digits, a negative size, or
// an empty name or spec.
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
// CEMStatement writes for a sha256.
func lowerHexSHA256(digest string) bool {
	if len(digest) != hex.EncodedLen(sha256.Size) {
		return false
	}
	return strings.Trim(digest, "0123456789abcdef") == ""
}
