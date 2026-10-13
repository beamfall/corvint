package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/attest"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// maxAttestationEnvelopeBytes bounds the envelope verify-cem mode reads; it
// matches the prove-observe stdin bound (FPK-V0-031, experimental).
const maxAttestationEnvelopeBytes = 8 << 20

// cemAttestation is the --attest-cem-v2 statement input: the map bytes and the
// target the stable verifier accepted.
type cemAttestation struct {
	name   string
	bytes  []byte
	target attest.CEMTarget
}

// cemAttestationInput returns the CEM statement input when a CEM attestation
// flag was given, and nil otherwise so the FPK-V0-015 output is untouched.
func cemAttestationInput(prove proveOptions, mapBytes []byte, target attest.CEMTarget) *cemAttestation {
	if !prove.attestCEMV2 {
		return nil
	}
	return &cemAttestation{
		name: filepath.ToSlash(prove.mapPath), bytes: mapBytes,
		target: target,
	}
}

// proveVerifiesCEMAttestation reports whether --verify-cem-attestation appears
// before --, which selects verify-cem mode wherever it is placed.
func proveVerifiesCEMAttestation(rest []string) bool {
	for _, argument := range rest {
		if argument == "--" {
			return false
		}
		if argument == "--verify-cem-attestation" || strings.HasPrefix(argument, "--verify-cem-attestation=") {
			return true
		}
	}
	return false
}

// parseProveVerifyCEMArguments reads `--verify-cem-attestation ENVELOPE
// --attest-public-key PEM [--cem MAP] [--artifacts ABS] [--target REV]`; every
// flag takes one value and none repeats. The root is resolved only when a map
// or a target is to be read, and --artifacts needs --cem (CEM-V1-065).
func parseProveVerifyCEMArguments(rootArguments, rest []string) (options, error) {
	result := options{command: "prove", proveMode: "verify-cem"}
	root := "."
	for index := 0; index < len(rootArguments); index++ {
		if rootArguments[index] == "--root" {
			root, index = rootArguments[index+1], index+1
		} else {
			root = strings.TrimPrefix(rootArguments[index], "--root=")
		}
	}
	fields := map[string]*string{
		"--verify-cem-attestation": &result.prove.envelopePath,
		"--attest-public-key":      &result.prove.publicKeyPath,
		"--cem":                    &result.prove.mapPath,
		"--artifacts":              &result.prove.artifacts,
		"--target":                 &result.prove.target,
	}
	seen := map[string]bool{}
	for index := 0; index < len(rest); {
		name, value, inline := strings.Cut(rest[index], "=")
		field := fields[name]
		if field == nil {
			return result, argumentError("unrecognized arguments: " + rest[index])
		}
		if seen[name] {
			return result, argumentError("argument " + name + ": may not be repeated")
		}
		seen[name] = true
		if !inline {
			if index+1 >= len(rest) || argparseOptionLike(rest[index+1]) {
				return result, argumentError("argument " + name + ": expected one argument")
			}
			value, index = rest[index+1], index+2
		} else {
			index++
		}
		if value == "" {
			return result, argumentError("argument " + name + ": expected one argument")
		}
		*field = value
	}
	if result.prove.envelopePath == "" {
		return result, argumentError("argument --verify-cem-attestation: expected one argument")
	}
	if result.prove.publicKeyPath == "" {
		return result, argumentError("argument --attest-public-key: expected one argument")
	}
	if result.prove.artifacts != "" && result.prove.mapPath == "" {
		return result, argumentError("argument --artifacts: requires --cem")
	}
	if result.prove.artifacts != "" && !filepath.IsAbs(result.prove.artifacts) {
		return result, argumentError("argument --artifacts: must be an absolute path")
	}
	if result.prove.mapPath == "" && result.prove.target == "" {
		return result, nil
	}
	resolved, err := resolveExplicitRoot(root)
	result.root = resolved
	return result, err
}

// cemAttestationReceipt is the verify-cem output. ArtifactsStatus and
// TargetStatus appear only for a cem/v2 claim (CEM-V1-065).
type cemAttestationReceipt struct {
	ArtifactsStatus string              `json:"artifactsStatus,omitempty"`
	CEM             cemAttestationClaim `json:"cem"`
	Mutates         bool                `json:"mutates"`
	OK              bool                `json:"ok"`
	PredicateType   string              `json:"predicateType"`
	Reason          string              `json:"reason,omitempty"`
	Status          string              `json:"status"`
	TargetStatus    string              `json:"targetStatus,omitempty"`
	Tool            string              `json:"tool"`
}

// cemAttestationClaim is the signed claim. A cem/v2 claim signs no size, so
// Size is omitted unless a v2 map was supplied; Artifacts and Target appear
// only for cem/v2.
type cemAttestationClaim struct {
	Artifacts    []cemAttestationArtifact `json:"artifacts,omitempty"`
	BaseRevision string                   `json:"baseRevision,omitempty"`
	Digest       map[string]string        `json:"digest"`
	Name         string                   `json:"name"`
	PatchSHA256  string                   `json:"patchSha256,omitempty"`
	Size         *int64                   `json:"size,omitempty"`
	Spec         string                   `json:"spec"`
	Target       *cemAttestationTarget    `json:"target,omitempty"`
}

type cemAttestationArtifact struct {
	Digest map[string]string `json:"digest"`
	Name   string            `json:"name"`
}

type cemAttestationTarget struct {
	GitCommit string `json:"gitCommit"`
	GitTree   string `json:"gitTree"`
}

// runVerifyCEMAttestation verifies a CEM DSSE envelope offline and prints the
// signed claim with VERIFIED or, without --cem, NOT_RUN. It runs Git only to
// resolve --target, and writes nothing.
func runVerifyCEMAttestation(ctx context.Context, options options, stdout, stderr io.Writer) int {
	verification, err := verifyCEMAttestation(ctx, options)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	encoded, err := gokernel.CanonicalJSON(cemAttestationReceiptFor(verification))
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot encode the CEM attestation verification"})
		return 2
	}
	if _, err := stdout.Write(append(encoded, '\n')); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write the CEM attestation verification"})
		return 2
	}
	return 0
}

// cemAttestationReceiptFor renders a cem/v2 verification.
func cemAttestationReceiptFor(verification attest.CEMVerification) cemAttestationReceipt {
	size := verification.Size
	receipt := cemAttestationReceipt{
		CEM: cemAttestationClaim{
			Digest: map[string]string{"sha256": verification.SHA256}, Name: verification.Name,
			Size: &size, Spec: verification.Spec,
			BaseRevision: verification.BaseRevision, PatchSHA256: verification.PatchSHA256,
		},
		OK: true, PredicateType: verification.PredicateType, Reason: verification.Reason,
		Status: verification.Status, Tool: "prove",
	}
	if v2 := verification.V2; v2 != nil {
		if verification.Status != attest.CEMVerified {
			receipt.CEM.Size = nil
		}
		receipt.CEM.Artifacts = make([]cemAttestationArtifact, 0, len(v2.Artifacts))
		for _, artifact := range v2.Artifacts {
			receipt.CEM.Artifacts = append(receipt.CEM.Artifacts, cemAttestationArtifact{
				Digest: map[string]string{"sha256": artifact.SHA256}, Name: artifact.Name,
			})
		}
		receipt.CEM.Target = &cemAttestationTarget{GitCommit: v2.Target.GitCommit, GitTree: v2.Target.GitTree}
		receipt.ArtifactsStatus, receipt.TargetStatus = v2.ArtifactsStatus, v2.TargetStatus
	}
	return receipt
}

func verifyCEMAttestation(ctx context.Context, options options) (attest.CEMVerification, error) {
	publicKey, err := attest.ReadPublicKey(options.prove.publicKeyPath)
	if err != nil {
		return attest.CEMVerification{}, attestPublicKeyRefusal(options.prove.publicKeyPath)
	}
	envelope, err := readBoundedFile(options.prove.envelopePath, maxAttestationEnvelopeBytes)
	if err != nil {
		return attest.CEMVerification{}, attestEnvelopeRefusal(options.prove.envelopePath)
	}
	mapBytes, err := readAttestedMap(options.root, options.prove.mapPath)
	if err != nil {
		return attest.CEMVerification{}, err
	}
	check := attest.CEMCheck{Map: mapBytes}
	if options.prove.artifacts != "" {
		check.Artifact = cemArtifactReader(options.prove.artifacts)
	}
	if options.prove.target != "" {
		target, err := resolveCEMTarget(ctx, options.root, options.prove.target)
		if err != nil {
			return attest.CEMVerification{}, err
		}
		check.Target = &target
	}
	verification, err := attest.VerifyCEMPredicateWith(envelope, publicKey, check)
	if errors.Is(err, attest.ErrCEMBytesMismatch) {
		return attest.CEMVerification{}, &gokernel.Error{Code: "attest-cem-mismatch", Message: "the CEM bytes differ from the attested sha256 or size"}
	}
	if errors.Is(err, attest.ErrCEMSubjectMismatch) {
		return attest.CEMVerification{}, &gokernel.Error{Code: "attest-cem-subject-mismatch", Message: "an artifact is not a signed subject of the CEM attestation: " + err.Error()}
	}
	if errors.Is(err, attest.ErrCEMTargetMismatch) {
		return attest.CEMVerification{}, &gokernel.Error{Code: "attest-cem-target-mismatch", Message: "the target is not the signed target of the CEM attestation: " + err.Error()}
	}
	if errors.Is(err, attest.ErrCEMPredicateRetired) {
		return attest.CEMVerification{}, &gokernel.Error{Code: "attest-verification-failed", Message: "the envelope's CEM predicate is retired; " + attest.CEMPredicateTypeV2 + " is the only CEM predicate"}
	}
	if err != nil {
		return attest.CEMVerification{}, &gokernel.Error{Code: "attest-verification-failed", Message: "the envelope is not a verified CEM attestation for this key"}
	}
	return verification, nil
}

// readAttestedMap reads the map bytes to check, unparsed so a changed map is
// reported as a digest mismatch; no --cem returns nil, which VerifyCEMPredicate
// discloses as NOT_RUN.
func readAttestedMap(root, mapPath string) ([]byte, error) {
	if mapPath == "" {
		return nil, nil
	}
	return readCEMMapBytes(root, mapPath)
}

// readCEMMapBytes is the one bounded map read prove --cem and the attestation verifier share: a
// path that is absolute, climbs out of the root or names .git is refused before any read.
func readCEMMapBytes(root, mapPath string) ([]byte, error) {
	if !containedRelativePath(mapPath) {
		return nil, cemMapPathRefusal(mapPath)
	}
	data, err := readBoundedFileUnderRoot(root, mapPath, wire.MaxMapBytes)
	if err != nil {
		return nil, cemMapReadRefusal(mapPath)
	}
	return data, nil
}

// statement builds the `cem/v2` CEM statement (CEM-V1-065); cem/0 and cem/v1
// are retired (CEM-V1-218).
func (cem *cemAttestation) statement() ([]byte, error) {
	return attest.CEMStatementV2(cem.name, cem.bytes, cem.target)
}

// cemArtifactReader reads a declared artifact path under the absolute
// artifact root the way the stable verifier's --artifacts does: contained,
// with no symlinked component, regular, and bounded.
func cemArtifactReader(artifactRoot string) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		if !containedRelativePath(path) {
			return nil, errors.New("artifact path is not contained in the artifact root")
		}
		return readBoundedFileUnderRoot(artifactRoot, path, wire.CandidateMaxArtifactBytes)
	}
}

// resolveCEMTarget resolves revision in root to a commit and that commit's
// tree, as full object IDs.
func resolveCEMTarget(ctx context.Context, root, revision string) (attest.CEMTarget, error) {
	refusal := &gokernel.Error{Code: "unsupported-prove-revision", Message: "--target " + revision + " does not name a commit in the repository"}
	if strings.HasPrefix(revision, "-") {
		return attest.CEMTarget{}, refusal
	}
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		return attest.CEMTarget{}, proveGitExecutableRefusal()
	}
	deadline, cancel := context.WithTimeout(ctx, proveGitDeadline)
	defer cancel()
	identities := make([]string, 0, 2)
	ref := revision + "^{commit}"
	for range 2 {
		output, err := hermeticGitCommand(deadline, gitExecutable, root, "rev-parse", "--verify", "--quiet", ref).Output()
		identity := string(bytes.TrimSpace(output))
		if err != nil || !validGitObjectID(identity) {
			return attest.CEMTarget{}, refusal
		}
		identities = append(identities, identity)
		ref = identity + "^{tree}"
	}
	return attest.CEMTarget{GitCommit: identities[0], GitTree: identities[1]}, nil
}

// compileStableCEMProof is compileCEMProof for --attest-cem-v2 (CEM-V1-065):
// the map must declare exactly cem/1.0, the embedded document is what `cem
// verify --artifacts` emits for the same inputs, and the stable verifier must
// accept the map before its target is attested. The bracket is the same as
// the 0.x path's.
func compileStableCEMProof(ctx context.Context, gitExecutable string, options options, dirtyBefore []string) (proveReceipt, error) {
	mapBytes, err := readCEMMapBytes(options.root, options.prove.mapPath)
	if err != nil {
		return proveReceipt{}, err
	}
	if !wire.DeclaresStable(mapBytes) {
		return proveReceipt{}, &gokernel.Error{Code: "attest-cem-v2-requires-stable-map", Message: "--attest-cem-v2 attests only a cem/1.0 stable map; cem/0.x is retired; fix: " + wire.RetiredSpecFix}
	}
	treeBefore, err := proveTreeRevision(ctx, gitExecutable, options.root)
	if err != nil {
		return proveReceipt{}, err
	}
	session, err := workflow.Open(options.root)
	if err != nil {
		return proveReceipt{}, cemProveError(err)
	}
	packet, err := session.Read(ctx, "verify", workflow.ReadOptions{
		MapPath: options.prove.mapPath, PatchGiven: options.prove.patchGiven,
		ExpectedBase: options.prove.expectedBase, Target: options.prove.target,
		ArtifactRoot: options.prove.artifacts, ArtifactsGiven: options.prove.artifacts != "",
	})
	if err != nil {
		return proveReceipt{}, cemProveError(err)
	}
	dirtyAfter, err := affected.DirtyPaths(ctx, gitExecutable, options.root)
	if err != nil {
		return proveReceipt{}, &gokernel.Error{Code: "unsupported-prove-history", Message: "cannot read the worktree status"}
	}
	mapAfter, err := readCEMMapBytes(options.root, options.prove.mapPath)
	if err != nil {
		return proveReceipt{}, err
	}
	treeAfter, err := proveTreeRevision(ctx, gitExecutable, options.root)
	if err != nil {
		return proveReceipt{}, err
	}
	if treeBefore != treeAfter || !equalStringSlices(dirtyBefore, dirtyAfter) || !bytes.Equal(mapBytes, mapAfter) {
		return proveReceipt{}, &gokernel.Error{Code: "unsupported-prove-drift", Message: "the worktree, HEAD tree, or map changed while the map was being proved"}
	}
	stable, _ := packet["stableVerification"].(workflow.StableVerification)
	result := stable.Result
	if result.Outcome != "ACCEPT" || result.Accept == nil || !*result.Accept || result.TargetRevision == nil {
		code := "none"
		if result.Code != nil {
			code = *result.Code
		}
		return proveReceipt{}, &gokernel.Error{Code: "attest-cem-v2-not-accepted", Message: "the stable verifier did not accept the map (outcome " + result.Outcome + ", code " + code + "); cem/v2 attests only an accepted map"}
	}
	document, err := wire.ParseStable(mapBytes)
	if err != nil {
		return proveReceipt{}, cemProveError(err)
	}
	target, err := resolveCEMTarget(ctx, options.root, *result.TargetRevision)
	if err != nil || target.GitCommit != *result.TargetRevision {
		return proveReceipt{}, &gokernel.Error{Code: "unsupported-prove-drift", Message: "the accepted target no longer resolves to the same commit"}
	}
	drift := make([]any, 0, len(result.Drift))
	for _, item := range result.Drift {
		drift = append(drift, map[string]any{"evidenceId": item.EvidenceID, "status": string(item.Status)})
	}
	verification := map[string]any{"valid": true, "drift": drift, "targetRevision": target.GitCommit}
	legacy := wire.Map(document.Change)
	rows := cemRows(&legacy)
	for index := range rows {
		rows[index].Falsified = judgeVerifier(rows[index], verification)
	}
	summary := summarizeCEMProof(rows)
	return proveReceipt{
		Mutates: false, OK: true, Packet: packet, Profile: proveProfile, Proof: summary,
		Revision: target.GitCommit, State: cemState(verification, summary), Tool: "prove",
		subjects: cemSubjects(options.prove.mapPath, mapBytes, target.GitCommit), mapBytes: mapBytes, cemTarget: target,
	}, nil
}
