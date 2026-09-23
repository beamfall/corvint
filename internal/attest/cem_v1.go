package attest

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// CEMPredicateTypeV1 is the versioned in-toto predicateType URI for an
// attestation about one Change Evidence Map (docs/specs/falsifiable-packet-v0.md,
// FPK-V0-033, experimental prototype). Its predicate names the map as an
// in-toto ResourceDescriptor and also binds the base revision and patch the map
// covers; CEMPredicateType is unchanged.
const CEMPredicateTypeV1 = "https://corvint-context.dev/attestation/cem/v1"

type gitCommitDigest struct {
	GitCommit string `json:"gitCommit"`
}

type cemV1Base struct {
	Digest gitCommitDigest `json:"digest"`
}

type cemV1Patch struct {
	Digest cemDigest `json:"digest"`
}

type cemV1Predicate struct {
	Base  cemV1Base  `json:"base"`
	CEM   cemSubject `json:"cem"`
	Patch cemV1Patch `json:"patch"`
	Size  int64      `json:"size"`
	Spec  string     `json:"spec"`
}

type cemV1Statement struct {
	Type      string         `json:"_type"`
	Predicate cemV1Predicate `json:"predicate"`
	Subject   []cemSubject   `json:"subject"`
}

type predicateTypeHeader struct {
	PredicateType string `json:"predicateType"`
}

// cemClaimParsers maps each predicateType VerifyCEMPredicate accepts to the
// parser of its statement.
var cemClaimParsers = map[string]func([]byte) (CEMVerification, error){
	CEMPredicateType:   parseCEMStatement,
	CEMPredicateTypeV1: parseCEMStatementV1,
}

// CEMStatementV1 builds the canonical in-toto Statement v1 of FPK-V0-033: the
// subject CEMStatement writes, and the predicate
// {"base":{"digest":{"gitCommit":OID}},"cem":{"digest":{"sha256":HEX},"name":NAME},
// "patch":{"digest":{"sha256":HEX}},"size":BYTES,"spec":SPEC}, with base and
// patch read from the map. It refuses what CEMStatement refuses.
func CEMStatementV1(name string, cem []byte) ([]byte, error) {
	if name == "" {
		return nil, errors.New("attest: CEM name must be non-empty")
	}
	document, err := wire.ParseMap(cem)
	if err != nil {
		return nil, fmt.Errorf("attest: parse CEM: %w", err)
	}
	sum := sha256.Sum256(cem)
	resource := map[string]any{"name": name, "digest": map[string]any{"sha256": hex.EncodeToString(sum[:])}}
	statement := map[string]any{
		"_type":         statementType,
		"subject":       []any{resource},
		"predicateType": CEMPredicateTypeV1,
		"predicate": map[string]any{
			"base":  map[string]any{"digest": map[string]any{"gitCommit": document.BaseRevision}},
			"cem":   resource,
			"patch": map[string]any{"digest": map[string]any{"sha256": document.PatchSha256}},
			"size":  json.Number(strconv.Itoa(len(cem))),
			"spec":  document.Spec,
		},
	}
	return canonicalMarshal(statement)
}

// VerifyCEMPredicate is VerifyCEM for a statement of either CEMPredicateType
// or CEMPredicateTypeV1, reporting which in PredicateType. For a v1 claim the
// supplied bytes must also be a CEM whose baseRevision and patchSha256 equal
// the signed ones.
func VerifyCEMPredicate(envelope []byte, publicKey ed25519.PublicKey, cem []byte) (CEMVerification, error) {
	payload, err := Verify(envelope, publicKey)
	if err != nil {
		return CEMVerification{}, err
	}
	var header predicateTypeHeader
	if err := strictUnmarshal(payload, &header); err != nil {
		return CEMVerification{}, fmt.Errorf("attest: parse CEM statement: %w", err)
	}
	parse := cemClaimParsers[header.PredicateType]
	if parse == nil {
		return CEMVerification{}, fmt.Errorf("attest: predicateType %q is not a CEM predicate", header.PredicateType)
	}
	claim, err := parse(payload)
	if err != nil {
		return CEMVerification{}, err
	}
	claim.PredicateType = header.PredicateType
	return checkCEMBytes(claim, cem)
}

// parseCEMStatementV1 reads a verified payload whose predicateType is
// CEMPredicateTypeV1 under the member-name rules of parseCEMStatement, and
// refuses a claim CEMStatementV1 could not have produced.
func parseCEMStatementV1(payload []byte) (CEMVerification, error) {
	var statement cemV1Statement
	if err := strictUnmarshal(payload, &statement); err != nil {
		return CEMVerification{}, fmt.Errorf("attest: parse CEM statement: %w", err)
	}
	if statement.Type != statementType {
		return CEMVerification{}, fmt.Errorf("attest: statement _type %q, want %q", statement.Type, statementType)
	}
	if len(statement.Subject) != 1 {
		return CEMVerification{}, fmt.Errorf("attest: CEM statement has %d subjects, want exactly 1", len(statement.Subject))
	}
	predicate := statement.Predicate
	if statement.Subject[0] != predicate.CEM {
		return CEMVerification{}, errors.New("attest: CEM subject and predicate disagree")
	}
	if !wire.IsGitOid(predicate.Base.Digest.GitCommit) {
		return CEMVerification{}, errors.New("attest: CEM claim base is not a full lowercase Git commit OID")
	}
	if !lowerHexSHA256(predicate.Patch.Digest.SHA256) {
		return CEMVerification{}, errors.New("attest: CEM claim patch sha256 is not 64 lowercase hex digits")
	}
	return checkCEMClaim(CEMVerification{
		Name: predicate.CEM.Name, SHA256: predicate.CEM.Digest.SHA256, Size: predicate.Size, Spec: predicate.Spec,
		BaseRevision: predicate.Base.Digest.GitCommit, PatchSHA256: predicate.Patch.Digest.SHA256,
	})
}

// checkCEMV1Map refuses cem bytes, already matched to the signed digest, that
// are not a CEM or whose baseRevision, patchSha256, or spec differ from the
// signed claim: the signer attested a binding the map does not carry.
func checkCEMV1Map(claim CEMVerification, cem []byte) error {
	document, err := wire.ParseMap(cem)
	if err != nil {
		return fmt.Errorf("attest: attested CEM bytes are not a CEM: %w", err)
	}
	if document.BaseRevision != claim.BaseRevision || document.PatchSha256 != claim.PatchSHA256 || document.Spec != claim.Spec {
		return errors.New("attest: signed base, patch, or spec differ from the attested CEM")
	}
	return nil
}
