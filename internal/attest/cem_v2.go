package attest

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// CEMPredicateTypeV2 is the in-toto predicateType URI for an attestation about
// one cem/1.0 stable map, its declared artifacts and the target commit it was
// verified against (docs/specs/cem-stable-v1.md, CEM-V1-065). It is the
// only CEM predicate: cem/0 and cem/v1 are retired (CEM-V1-218).
const CEMPredicateTypeV2 = "https://corvint-context.dev/attestation/cem/v2"

var (
	// ErrCEMRequiresStableMap is returned by CEMStatementV2 for bytes that do
	// not declare exactly cem/1.0.
	ErrCEMRequiresStableMap = errors.New("attest: cem/v2 attests only a cem/1.0 stable map")
	// ErrCEMSubjectMismatch is wrapped when a declared artifact, or an artifact
	// file, is not one of the signed subjects, or when an artifact check is
	// asked of a claim that has no artifact subjects.
	ErrCEMSubjectMismatch = errors.New("attest: CEM artifacts do not match the attested subjects")
	// ErrCEMTargetMismatch is wrapped when the supplied target commit or tree
	// differs from the signed target, or when a target check is asked of a
	// claim that signs no target.
	ErrCEMTargetMismatch = errors.New("attest: target does not match the attested target")
	// ErrCEMPredicateRetired is wrapped when a verified envelope carries a
	// retired cem/0 or cem/v1 predicateType (CEM-V1-218).
	ErrCEMPredicateRetired = errors.New("attest: retired CEM predicate")
)

// CEMTarget is a commit and its tree, as full lowercase Git object IDs.
type CEMTarget struct {
	GitCommit string
	GitTree   string
}

// CEMArtifactSubject is one signed artifact subject: the declared path and its
// sha256.
type CEMArtifactSubject struct {
	Name   string
	SHA256 string
}

// CEMV2Claim is the part of a CEMPredicateTypeV2 claim beyond the map subject.
// ArtifactsStatus and TargetStatus are CEMVerified when that check ran and
// CEMNotRun otherwise.
type CEMV2Claim struct {
	Target          CEMTarget
	Artifacts       []CEMArtifactSubject
	ArtifactsStatus string
	TargetStatus    string
	// Counts and Ceilings are the signed CAX-V0-001 members of a cem/v3
	// claim, and nil for cem/v2.
	Counts   *CEMCounts
	Ceilings *CEMCeilings
	// Paired is the optional signed CAX-V0-034 pairedRange member of a cem/v3
	// claim, and nil when none was signed.
	Paired *CEMPairedRange
}

// CEMCheck is what VerifyCEMPredicateWith checks a claim against. A nil Map
// leaves the map unchecked (NOT_RUN). A non-nil Artifact reads a declared
// artifact by its declared path; it needs Map. A non-nil Target is compared
// with the signed target.
type CEMCheck struct {
	Map      []byte
	Artifact func(path string) ([]byte, error)
	Target   *CEMTarget
}

type cemV2Target struct {
	GitCommit string `json:"gitCommit"`
	GitTree   string `json:"gitTree"`
}

type cemV2Predicate struct {
	BaseRevision string      `json:"baseRevision"`
	PatchSha256  string      `json:"patchSha256"`
	Target       cemV2Target `json:"target"`
}

type cemV2Statement struct {
	Type          string         `json:"_type"`
	Predicate     cemV2Predicate `json:"predicate"`
	PredicateType string         `json:"predicateType"`
	Subject       []cemSubject   `json:"subject"`
}

// CEMStatementV2 builds the canonical in-toto Statement v1 of CEM-V1-065. Its
// subjects are the map, named name with the sha256 of the exact cem bytes,
// followed by every declared artifact in map order, named by its declared path
// with its declared sha256. Its predicate is the closed object
// {"baseRevision":OID,"patchSha256":HEX,"target":{"gitCommit":OID,"gitTree":OID}},
// with base and patch read from the map. It refuses an empty name, bytes that
// are not a valid cem/1.0 map, a target that is not two full Git object IDs of
// one length, and a subject name that repeats.
func CEMStatementV2(name string, cem []byte, target CEMTarget) ([]byte, error) {
	if name == "" {
		return nil, errors.New("attest: CEM name must be non-empty")
	}
	if !wire.DeclaresStable(cem) {
		return nil, ErrCEMRequiresStableMap
	}
	document, err := wire.ParseStable(cem)
	if err != nil {
		return nil, fmt.Errorf("attest: parse stable CEM: %w", err)
	}
	if err := checkCEMTarget(target); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(cem)
	subjects := []any{map[string]any{"name": name, "digest": map[string]any{"sha256": hex.EncodeToString(sum[:])}}}
	seen := map[string]bool{name: true}
	for _, artifact := range document.Artifacts {
		if seen[artifact.Path] {
			return nil, fmt.Errorf("attest: subject name %q repeats", artifact.Path)
		}
		seen[artifact.Path] = true
		subjects = append(subjects, map[string]any{"name": artifact.Path, "digest": map[string]any{"sha256": artifact.Sha256}})
	}
	statement := map[string]any{
		"_type":         statementType,
		"subject":       subjects,
		"predicateType": CEMPredicateTypeV2,
		"predicate": map[string]any{
			"baseRevision": document.Change.BaseRevision,
			"patchSha256":  document.Change.PatchSha256,
			"target":       map[string]any{"gitCommit": target.GitCommit, "gitTree": target.GitTree},
		},
	}
	return canonicalMarshal(statement)
}

func checkCEMTarget(target CEMTarget) error {
	if !wire.IsGitOid(target.GitCommit) || !wire.IsGitOid(target.GitTree) || len(target.GitCommit) != len(target.GitTree) {
		return errors.New("attest: CEM target is not a full lowercase Git commit and tree OID of one length")
	}
	return nil
}

// VerifyCEMPredicateWith verifies envelope with Verify, requires a
// CEMPredicateTypeV2 or CEMPredicateTypeV3 statement, and checks it against
// check; a v3 claim's counts are also compared with the supplied map. A retired cem/0 or
// cem/v1 envelope is refused by name (CEM-V1-218), as is any other
// predicateType.
func VerifyCEMPredicateWith(envelope []byte, publicKey ed25519.PublicKey, check CEMCheck) (CEMVerification, error) {
	payload, err := Verify(envelope, publicKey)
	if err != nil {
		return CEMVerification{}, err
	}
	var header predicateTypeHeader
	if err := strictUnmarshal(payload, &header); err != nil {
		return CEMVerification{}, fmt.Errorf("attest: parse CEM statement: %w", err)
	}
	if retiredCEMPredicateTypes[header.PredicateType] {
		return CEMVerification{}, fmt.Errorf("%w: predicateType %q is retired; %s is the only CEM predicate", ErrCEMPredicateRetired, header.PredicateType, CEMPredicateTypeV2)
	}
	var claim CEMVerification
	switch header.PredicateType {
	case CEMPredicateTypeV2:
		claim, err = parseCEMStatementV2(payload)
	case CEMPredicateTypeV3:
		claim, err = parseCEMStatementV3(payload)
	default:
		return CEMVerification{}, fmt.Errorf("attest: predicateType %q is not a CEM predicate", header.PredicateType)
	}
	if err != nil {
		return CEMVerification{}, err
	}
	claim.PredicateType = header.PredicateType
	if claim, err = checkCEMV2(claim, check); err != nil {
		return CEMVerification{}, err
	}
	return checkCEMV3Counts(claim, check)
}

// parseCEMStatementV2 reads a verified payload whose predicateType is
// CEMPredicateTypeV2 under the member-name rules of parseCEMStatement, refuses
// any predicate member CEMStatementV2 does not write, and refuses a claim
// CEMStatementV2 could not have produced.
func parseCEMStatementV2(payload []byte) (CEMVerification, error) {
	var statement cemV2Statement
	if err := strictUnmarshal(payload, &statement); err != nil {
		return CEMVerification{}, fmt.Errorf("attest: parse CEM statement: %w", err)
	}
	var raw struct {
		Predicate jsontext.Value `json:"predicate"`
	}
	if err := jsonv2.Unmarshal(payload, &raw); err != nil {
		return CEMVerification{}, fmt.Errorf("attest: parse CEM statement: %w", err)
	}
	var closed cemV2Predicate
	if err := jsonv2.Unmarshal(raw.Predicate, &closed, jsonv2.RejectUnknownMembers(true)); err != nil {
		return CEMVerification{}, fmt.Errorf("attest: cem/v2 predicate is not closed: %w", err)
	}
	if statement.Type != statementType {
		return CEMVerification{}, fmt.Errorf("attest: statement _type %q, want %q", statement.Type, statementType)
	}
	if len(statement.Subject) == 0 {
		return CEMVerification{}, errors.New("attest: CEM statement has no subjects")
	}
	predicate := statement.Predicate
	if !wire.IsGitOid(predicate.BaseRevision) {
		return CEMVerification{}, errors.New("attest: CEM claim base is not a full lowercase Git commit OID")
	}
	if !lowerHexSHA256(predicate.PatchSha256) {
		return CEMVerification{}, errors.New("attest: CEM claim patch sha256 is not 64 lowercase hex digits")
	}
	target := CEMTarget{GitCommit: predicate.Target.GitCommit, GitTree: predicate.Target.GitTree}
	if err := checkCEMTarget(target); err != nil {
		return CEMVerification{}, err
	}
	seen := map[string]bool{}
	artifacts := make([]CEMArtifactSubject, 0, len(statement.Subject)-1)
	for index, subject := range statement.Subject {
		if subject.Name == "" || seen[subject.Name] {
			return CEMVerification{}, fmt.Errorf("attest: CEM subject %d name is empty or repeats", index)
		}
		seen[subject.Name] = true
		if !lowerHexSHA256(subject.Digest.SHA256) {
			return CEMVerification{}, fmt.Errorf("attest: CEM subject %d sha256 is not 64 lowercase hex digits", index)
		}
		if index > 0 {
			artifacts = append(artifacts, CEMArtifactSubject{Name: subject.Name, SHA256: subject.Digest.SHA256})
		}
	}
	return checkCEMClaim(CEMVerification{
		Name: statement.Subject[0].Name, SHA256: statement.Subject[0].Digest.SHA256, Spec: wire.StableSpec,
		BaseRevision: predicate.BaseRevision, PatchSHA256: predicate.PatchSha256,
		V2: &CEMV2Claim{Target: target, Artifacts: artifacts, ArtifactsStatus: CEMNotRun, TargetStatus: CEMNotRun},
	})
}

// checkCEMV2 checks a parsed v2 claim: the map bytes against the map subject
// and the signed base and patch, each declared artifact against the artifact
// subjects and its file, and the target against the signed target.
func checkCEMV2(claim CEMVerification, check CEMCheck) (CEMVerification, error) {
	if check.Map == nil {
		if check.Artifact != nil {
			return CEMVerification{}, errors.New("attest: an artifact check needs the CEM bytes")
		}
		claim.Status, claim.Reason = CEMNotRun, cemBytesNotSupplied
	} else {
		sum := sha256.Sum256(check.Map)
		if hex.EncodeToString(sum[:]) != claim.SHA256 {
			return CEMVerification{}, fmt.Errorf("%w: sha256", ErrCEMBytesMismatch)
		}
		claim.Size = int64(len(check.Map))
		document, err := wire.ParseStable(check.Map)
		if err != nil || !wire.DeclaresStable(check.Map) {
			return CEMVerification{}, fmt.Errorf("attest: attested CEM bytes are not a cem/1.0 stable map: %v", err)
		}
		if document.Change.BaseRevision != claim.BaseRevision || document.Change.PatchSha256 != claim.PatchSHA256 {
			return CEMVerification{}, errors.New("attest: signed base or patch differ from the attested CEM")
		}
		claim.Status = CEMVerified
		if check.Artifact != nil {
			if err := checkCEMArtifacts(claim.V2.Artifacts, document.Artifacts, check.Artifact); err != nil {
				return CEMVerification{}, err
			}
			claim.V2.ArtifactsStatus = CEMVerified
		}
	}
	if check.Target != nil {
		if check.Target.GitCommit != claim.V2.Target.GitCommit {
			return CEMVerification{}, fmt.Errorf("%w: commit %s is not the signed %s", ErrCEMTargetMismatch, check.Target.GitCommit, claim.V2.Target.GitCommit)
		}
		if check.Target.GitTree != claim.V2.Target.GitTree {
			return CEMVerification{}, fmt.Errorf("%w: tree %s is not the signed %s", ErrCEMTargetMismatch, check.Target.GitTree, claim.V2.Target.GitTree)
		}
		claim.V2.TargetStatus = CEMVerified
	}
	return claim, nil
}

// checkCEMArtifacts requires the declared artifacts and the artifact subjects
// to be the same (path, sha256) pairs, and every artifact file read by its
// declared path to carry its declared sha256.
func checkCEMArtifacts(subjects []CEMArtifactSubject, declared []wire.StableArtifact, read func(string) ([]byte, error)) error {
	signed := make(map[CEMArtifactSubject]bool, len(subjects))
	for _, subject := range subjects {
		signed[subject] = true
	}
	if len(declared) != len(subjects) {
		return fmt.Errorf("%w: the map declares %d artifacts, the statement signs %d", ErrCEMSubjectMismatch, len(declared), len(subjects))
	}
	for _, artifact := range declared {
		if !signed[CEMArtifactSubject{Name: artifact.Path, SHA256: artifact.Sha256}] {
			return fmt.Errorf("%w: declared artifact %s is not a subject", ErrCEMSubjectMismatch, artifact.Path)
		}
	}
	for _, artifact := range declared {
		data, err := read(artifact.Path)
		if err != nil {
			return fmt.Errorf("%w: artifact %s is unreadable: %v", ErrCEMSubjectMismatch, artifact.Path, err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != artifact.Sha256 {
			return fmt.Errorf("%w: artifact file %s is not the signed subject", ErrCEMSubjectMismatch, artifact.Path)
		}
	}
	return nil
}
