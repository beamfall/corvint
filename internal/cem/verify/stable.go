package verify

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/sim"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

type StableOptions struct {
	Repository, ExpectedBase, Target, ArtifactRoot string
	// TrustedKeys is the consumer-supplied keyid to Ed25519 key set of
	// --trusted-keys; nil means no file was supplied (CEM-V1-084).
	TrustedKeys map[string]ed25519.PublicKey
	// ExternalRepositories maps each --external-repository name to its
	// absolute repository root (CEM-V1-333); nil means none was supplied.
	ExternalRepositories map[string]string
	// ScaledOperations opts into an operation bound of 1,024 plus 4 per map
	// record (DCW-V0-373). Off, the bound is the cem/1.0 stable protocol's
	// fixed 1,024 operations (CEM-V1-260); every other bound is unchanged.
	ScaledOperations bool
	// NonRegular opts into the additive cem-non-regular/1 profile
	// (CAX-V0-020): the canonical patch is derived without the sections of
	// changed paths that are not regular files, and those paths are returned
	// in StableResult.NonRegular as explicit unknowns. Off, a non-regular
	// entry refuses the range exactly as cem/1.0 does.
	NonRegular bool
	seam       *stableSeam
	// capture, set only by StableWithHunkRevert, receives the admitted
	// mechanics of an accepted call; it never changes the result.
	capture *stableCapture
}

// stableSeam is the in-package test seam of one Stable call. A nil seam is
// production: the real clock, the pinned Git binary and the real filesystem.
// git.Event also receives the verifier checkpoints metadata-admitted,
// artifacts-start, artifacts-complete, boundary-revalidated and final-check.
type stableSeam struct {
	git   gitrun.Seam
	fault gitauth.StableFault
	// operations, when positive, replaces the scaled operation bound so a
	// shared case can reach the operation limit with a small map
	// (CEM-V1-260).
	operations int
}

// StableOuterDeadline is the one wall deadline of a Stable verification.
const StableOuterDeadline = 30 * time.Minute

// StableResult is the closed cem-stable-verification/1 result. Citation is the
// CEM-V1-082 block over the admitted map and its resolved evidence spans and is
// non-null only on ACCEPT (CEM-V1-092): Refuse and reject clear it, so no
// refusal, rejection, cancellation or drift carries it.
type StableResult struct {
	Profile            string                `json:"profile"`
	Spec               string                `json:"spec"`
	VerificationMode   string                `json:"verificationMode"`
	Outcome            string                `json:"outcome"`
	Accept             *bool                 `json:"accept"`
	Stage              string                `json:"stage"`
	Code               *string               `json:"code"`
	IssueCodes         []string              `json:"issueCodes"`
	Assurance          *string               `json:"assurance"`
	ExpectedBase       *string               `json:"expectedBase"`
	TargetRevision     *string               `json:"targetRevision"`
	MapSha256          *string               `json:"mapSha256"`
	PatchSha256        *string               `json:"patchSha256"`
	Sidecar            string                `json:"sidecar"`
	Drift              []StableDrift         `json:"drift"`
	Hunks              []any                 `json:"hunks"`
	References         map[string]any        `json:"references"`
	ArtifactChecks     []StableArtifactCheck `json:"artifactChecks"`
	Citation           *Citation             `json:"citation"`
	Axes               map[string]string     `json:"axes"`
	Runtime            StableRuntime         `json:"runtime"`
	RepositoryEnvelope string                `json:"repositoryEnvelope"`
	// VerdictSha256 is recomputed by MarshalJSON on every encoding; a value
	// held in the struct is never emitted (CEM-V1-066).
	VerdictSha256 string `json:"verdictSha256"`
	// LimitCause is the bounded Git error behind an unsupported-resource-limit
	// refusal, naming the bound that fired and its value. It is never encoded,
	// so cem-stable-verification/1 bytes and verdictSha256 are unchanged
	// (OCM-V0-026; V1-2170).
	LimitCause string `json:"-"`
	// NonRegular lists the changed non-regular paths that StableOptions.NonRegular
	// left out of the derived patch (CAX-V0-020). It is never encoded, so
	// the cem-stable-verification/1 bytes and verdictSha256 are unchanged.
	NonRegular []gitauth.NonRegularEntry `json:"-"`
}
type StableDrift struct {
	EvidenceID    string           `json:"evidenceId"`
	Path          string           `json:"path"`
	BaseBlobOid   string           `json:"baseBlobOid"`
	Status        DriftStatus      `json:"status"`
	TargetBlobOid *string          `json:"targetBlobOid"`
	TargetSpan    map[string]int64 `json:"targetSpan"`
}
type StableRuntime struct {
	GoVersion               string `json:"goVersion"`
	StructuralQualification string `json:"structuralQualification"`
}
type StableArtifactCheck struct {
	Kind       string `json:"kind"`
	Path       string `json:"path"`
	Sha256     string `json:"sha256"`
	ByteLength int    `json:"byteLength"`
}

// structuralTuples are the exact Go releases whose go/parser, go/scanner and
// go/format behavior is qualified for structural mechanical proof
// (CEM-V1-050): go1.27.1 is the S0E prototype tuple, go1.27.2 is admitted by
// decision 0522 and go1.27.3 by decision 0547, each on its sealed
// protocol/cem-1.0/structural-tuples/<version> packet. Every other release
// refuses with unsupported-structural-runtime.
var structuralTuples = map[string]bool{"go1.27.1": true, "go1.27.2": true, "go1.27.3": true}

func structuralRuntimeAdmitted(version string) bool { return structuralTuples[version] }

// StructuralTupleAdmitted reports whether version is an admitted structural Go
// tuple, the only goVersion a QUALIFIED_EXACT_TUPLE result may name
// (CEM-V1-050, CEM-V1-570).
func StructuralTupleAdmitted(version string) bool { return structuralRuntimeAdmitted(version) }

func nullableStable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func stableDigest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func NewStableResult(o StableOptions) StableResult {
	axes := map[string]string{}
	for _, k := range []string{"changeIntegrity", "referenceIntegrity", "structuralProof", "coverageValidation", "discriminationValidation"} {
		axes[k] = "NOT_CHECKED"
	}
	for _, k := range []string{"nativeAuthority", "historicalValidity", "currentApplicability", "sourceGitBinding", "sourceInventoryCompleteness", "executionAtCommit", "runnerReceiptSemantics", "runnerExecution", "authentication", "dependencyClosure", "criterionAdequacy", "criterionDiscrimination", "externalInteroperability"} {
		axes[k] = "NOT_OBSERVED"
	}
	return StableResult{Profile: "cem-stable-verification/1", Spec: wire.StableSpec, VerificationMode: "canonical-and-reference-integrity", Outcome: "UNSUPPORTED", Stage: "arguments", IssueCodes: []string{}, ExpectedBase: nullableStable(o.ExpectedBase), TargetRevision: nullableStable(o.Target), Sidecar: "NOT_CHECKED", Drift: []StableDrift{}, Hunks: []any{}, References: map[string]any{"criterionBindings": []any{}, "runnerReceipts": []any{}, "criterionLinks": []any{}, "artifacts": []any{}}, ArtifactChecks: []StableArtifactCheck{}, Axes: axes, Runtime: StableRuntime{runtime.Version(), "NOT_REQUIRED"}, RepositoryEnvelope: "canonical-repository-bounded/1"}
}

// RelativePathFix names the recovery for an invalid-arguments refusal whose
// cause is a relative path option (CEM-V1-117). Each pair is an option name and
// its supplied value; empty values are absent. It returns "" when every
// supplied path is absolute. A relative path is refused, never resolved
// (CEM-V1-017), so the fix is the caller's.
func RelativePathFix(options ...[2]string) string {
	var relative []string
	for _, o := range options {
		if o[1] != "" && !filepath.IsAbs(o[1]) {
			relative = append(relative, o[0])
		}
	}
	if len(relative) == 0 {
		return ""
	}
	return "invalid-arguments: relative path option " + strings.Join(relative, ", ") + "; fix: pass each as an absolute path such as \"$PWD/<path>\"; a relative path is refused, never resolved"
}

func (r *StableResult) Refuse(stage, code string, unsupported bool) int {
	r.Citation = nil
	r.Stage = stage
	r.Code = nullableStable(code)
	r.Outcome = "REJECT"
	b := false
	r.Accept = &b
	if unsupported {
		r.Outcome = "UNSUPPORTED"
		r.Accept = nil
		r.Assurance = nil
	}
	return 2
}
func (r *StableResult) reject(stage, issue string) int {
	r.Citation = nil
	r.Stage = stage
	r.Outcome = "REJECT"
	b := false
	r.Accept = &b
	r.Code = nil
	r.IssueCodes = []string{issue}
	return 1
}

// Stable verifies exact stable wire, independent canonical Git content and two
// complete passes over opaque artifact bytes. Repository metadata is admitted
// by gitauth.OpenStable and every Git child runs under the owned runner: one
// thirty-minute wall deadline, one ten-second emergency retirement allowance,
// and an unobserved process cleanup overrides any provisional result.
func Stable(ctx context.Context, raw []byte, o StableOptions) (r StableResult, exit int) {
	r = NewStableResult(o)
	if len(raw) > wire.MaxMapBytes {
		exit = r.Refuse("input", "map-unavailable", true)
		return r, exit
	}
	r.MapSha256 = nullableStable(stableDigest(raw))
	d, err := wire.ParseStable(raw)
	if d != nil {
		r.Axes["coverageValidation"] = "ABSENT"
		r.Axes["discriminationValidation"] = "ABSENT"
		for _, h := range d.Change.Hunks {
			if h.Coverage != nil {
				r.Axes["coverageValidation"] = "WIRE_VALIDATED_ONLY"
			}
			if h.Discriminates != nil {
				r.Axes["discriminationValidation"] = "WIRE_VALIDATED_ONLY"
			}
		}
	}
	if err != nil {
		stage := "wire"
		if d != nil {
			stage = "references"
			r.Axes["referenceIntegrity"] = "FAILED"
		}
		exit = r.Refuse(stage, cemcode.CodeOf(err), false)
		return r, exit
	}
	change := wire.Map(d.Change) // Private mechanics view; never returned to a legacy writer.
	r.Hunks = d.StableHunks()
	r.References = d.StableReferences()
	if o.ExpectedBase == "" {
		exit = r.Refuse("authority-arguments", "expected-base-required", true)
		return r, exit
	}
	if o.Target == "" {
		exit = r.Refuse("authority-arguments", "target-required", true)
		return r, exit
	}
	if o.ArtifactRoot == "" {
		exit = r.Refuse("authority-arguments", "artifacts-required", true)
		return r, exit
	}
	if !wire.IsGitOid(o.ExpectedBase) || !wire.IsGitOid(o.Target) || len(o.ExpectedBase) != len(o.Target) || !filepath.IsAbs(o.Repository) || !filepath.IsAbs(o.ArtifactRoot) {
		exit = r.Refuse("authority-arguments", "invalid-arguments", true)
		return r, exit
	}
	mechanical, structural := false, false
	for _, h := range d.Change.Hunks {
		if h.Disposition == "mechanical" {
			mechanical = true
			structural = structural || wire.StructuralReasons[h.Reason]
		}
	}
	if structural {
		if !structuralRuntimeAdmitted(runtime.Version()) {
			r.Runtime.StructuralQualification = "UNSUPPORTED"
			r.Axes["structuralProof"] = "UNSUPPORTED_RUNTIME"
			exit = r.Refuse("runtime", "unsupported-structural-runtime", true)
			return r, exit
		}
		r.Runtime.StructuralQualification = "QUALIFIED_EXACT_TUPLE"
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		exit = r.Refuse("runtime", "unsupported-process-containment", true)
		return r, exit
	}
	seam := o.seam
	if seam == nil {
		seam = &stableSeam{}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	now := time.Now
	if seam.git.Now != nil {
		now = seam.git.Now
	}
	// The protocol bound is fixed; an opted-in caller scales it with the
	// admitted map, whose wire bounds cap it (DCW-V0-373).
	operations := gitrun.DefaultOperations
	if o.ScaledOperations {
		operations = gitrun.ScaledOperations(len(change.Hunks) + len(change.Evidence))
	}
	if seam.operations > 0 {
		operations = seam.operations
	}
	budget := gitrun.NewStableBudget(operations, now().Add(StableOuterDeadline), cancel, seam.git)
	if seam.git.Now == nil {
		timer := time.AfterFunc(StableOuterDeadline, func() { budget.OuterExpired() })
		defer timer.Stop()
	}
	defer context.AfterFunc(ctx, budget.AnchorEmergency)()
	timedOut := func() bool { return budget.OuterExpired() || ctx.Err() != nil }
	repo, boundary, err := gitauth.OpenStable(o.Repository, budget, seam.fault)
	if err != nil {
		exit = r.Refuse("repository", stableAdmissionCode(err), true)
		return r, exit
	}
	defer boundary.Close()
	if err = boundary.Validate(); err != nil {
		exit = r.Refuse("repository", "repository-object-unavailable", true)
		return r, exit
	}
	budget.Notify("metadata-admitted")
	// The session close is part of the result: a cleanup that was not observed
	// overrides whatever the verification provisionally concluded.
	release := repo.BeginCheckedObjectSession(ctx)
	released := false
	closeSession := func() error {
		if released {
			return nil
		}
		released = true
		return release()
	}
	defer func() {
		if e := closeSession(); e != nil || budget.Held() {
			r.IssueCodes = []string{}
			exit = r.Refuse("repository", gitrun.ProcessContainment, true)
		}
	}()
	fail := func(stage string, e error) int {
		code := cemcode.CodeOf(e)
		if code == gitrun.ProcessContainment || budget.Held() {
			return r.Refuse("repository", gitrun.ProcessContainment, true)
		}
		// A committed per-operation cause stands: only a cancellation that the
		// operation itself reported becomes the outer verification timeout.
		if code == "git-cancelled" || errors.Is(e, context.Canceled) || errors.Is(e, context.DeadlineExceeded) {
			return r.Refuse(stage, "verification-timeout", true)
		}
		switch code {
		case "git-output-exceeded", "git-budget-exceeded":
			r.LimitCause = e.Error()
			return r.Refuse("repository", "unsupported-resource-limit", true)
		case "git-start-failed", "git-exit-failure":
			return r.Refuse("repository", "git-read-failed", true)
		case "git-diff-timeout":
			return r.Refuse("repository", "git-timeout", true)
		case "unsupported-repository-attributes":
			return r.Refuse("repository", "unsupported-repository-envelope", true)
		case "unsupported-tree-mode":
			if stage == "repository" {
				return r.Refuse(stage, code, true)
			}
		case "repository-object-unavailable", "unsupported-object-alternates", "git-read-failed", "unsupported-resource-limit", "unsupported-repository-envelope", "unsupported-patch-inventory", "git-timeout", "verification-timeout", "git-diff-failed":
			if code == "unsupported-resource-limit" {
				r.LimitCause = e.Error()
			}
			return r.Refuse("repository", code, true)
		}
		r.Axes["changeIntegrity"] = "FAILED"
		return r.reject(stage, code)
	}
	if timedOut() {
		exit = r.Refuse("binding", "verification-timeout", true)
		return r, exit
	}
	r.Assurance = nullableStable("structural-only")
	if err = checkExpectedBase(ctx, repo, &change, o.ExpectedBase); err != nil {
		exit = fail("binding", err)
		return r, exit
	}
	target, err := repo.Resolve(ctx, o.Target)
	if err != nil {
		exit = fail("repository", err)
		return r, exit
	}
	if err = checkBaseSidecar(ctx, repo, d.Change.BaseRevision); err != nil {
		exit = fail("binding", err)
		return r, exit
	}
	entry, present, err := repo.LookupTreeEntry(ctx, target, wire.ExcludedCEMPath)
	if err != nil {
		exit = fail("repository", err)
		return r, exit
	}
	r.Sidecar = "ABSENT"
	if present {
		r.Sidecar = "MISMATCH"
		if entry.Type != "blob" || entry.Mode != "100644" {
			r.Sidecar = "UNSUPPORTED_KIND"
		}
		if err = checkTargetSidecar(ctx, repo, target, d.OriginalBytes()); err != nil {
			exit = fail("binding", err)
			return r, exit
		}
		r.Sidecar = "EXACT"
	}
	var patchBytes []byte
	var createDestinations map[string]gitauth.TreeEntry
	if o.NonRegular {
		patchBytes, createDestinations, r.NonRegular, err = repo.CanonicalRegularDiffWithCreateDestinations(ctx, d.Change.BaseRevision, target)
	} else {
		patchBytes, createDestinations, err = repo.CanonicalDiffWithCreateDestinations(ctx, d.Change.BaseRevision, target)
	}
	if err != nil {
		exit = fail("repository", err)
		return r, exit
	}
	r.PatchSha256 = nullableStable(stableDigest(patchBytes))
	r.Assurance = nullableStable("canonical")
	if err = checkPatchDigest(&change, patchBytes); err != nil {
		exit = fail("verification", err)
		return r, exit
	}
	parsed, err := patch.Parse(patchBytes)
	if err != nil {
		exit = fail("verification", stablePatchRefusal(err))
		return r, exit
	}
	source := &baseSource{ctx: ctx, repository: repo, base: d.Change.BaseRevision}
	createAbsent, createBlocked, err := stableAuthenticatedCreateAbsences(parsed, createDestinations)
	if err != nil {
		exit = fail("repository", err)
		return r, exit
	}
	simSource := &stableSimulationSource{source: source, authenticatedCreateAbsent: createAbsent, authenticatedCreateBlocked: createBlocked}
	if err = sim.Simulate(parsed, simSource); err != nil {
		exit = fail("verification", err)
		return r, exit
	}
	if len(change.Hunks) != len(parsed.Hunks) {
		r.Axes["changeIntegrity"] = "FAILED"
		exit = r.reject("verification", "patch-hunk-count")
		return r, exit
	}
	if err = checkHunkCoverage(&change, parsed); err != nil {
		exit = fail("verification", err)
		return r, exit
	}
	spans, err := checkEvidence(ctx, repo, &change, d.Change.BaseRevision)
	if err != nil {
		exit = fail("verification", err)
		return r, exit
	}
	if timedOut() {
		exit = r.Refuse("verification", "verification-timeout", true)
		return r, exit
	}
	proofSource := &stableProofSource{source: source}
	if err = checkMechanical(&change, parsed, proofSource); err != nil {
		if proofSource.err != nil {
			exit = fail("verification", proofSource.err)
			return r, exit
		}
		if timedOut() {
			exit = r.Refuse("verification", "verification-timeout", true)
			return r, exit
		}
		r.Axes["structuralProof"] = "FAILED"
		exit = fail("verification", err)
		return r, exit
	}
	if timedOut() {
		exit = r.Refuse("verification", "verification-timeout", true)
		return r, exit
	}
	r.Axes["structuralProof"] = "NOT_REQUIRED"
	if mechanical {
		r.Axes["structuralProof"] = "PROVEN_DECLARED_FILE_PREDICATES"
	}
	driftChange := change
	driftChange.Evidence = append([]wire.Evidence(nil), change.Evidence...)
	sort.Slice(driftChange.Evidence, func(i, j int) bool { return driftChange.Evidence[i].ID < driftChange.Evidence[j].ID })
	outcome := &Outcome{BaseRevision: d.Change.BaseRevision}
	err = checkDrift(ctx, repo, &driftChange, spans, target, outcome)
	evidence := map[string]wire.Evidence{}
	for _, e := range d.Change.Evidence {
		evidence[e.ID] = e
	}
	for _, v := range outcome.Drift {
		e := evidence[v.EvidenceID]
		var span map[string]int64
		if v.TargetSpan != nil {
			span = map[string]int64{"start": v.TargetSpan.Start, "end": v.TargetSpan.End}
		}
		r.Drift = append(r.Drift, StableDrift{v.EvidenceID, e.Path, e.BlobOid, v.Status, nullableStable(v.TargetBlobOid), span})
	}
	sort.Slice(r.Drift, func(i, j int) bool { return r.Drift[i].EvidenceID < r.Drift[j].EvidenceID })
	if err != nil {
		exit = fail("drift", err)
		return r, exit
	}
	r.Axes["changeIntegrity"] = "VERIFIED"
	budget.Notify("artifacts-start")
	if timedOut() {
		exit = r.Refuse("artifacts", "verification-timeout", true)
		return r, exit
	}
	checks, contents, err := stableArtifacts(ctx, o.ArtifactRoot, d.Artifacts)
	if err != nil {
		code := cemcode.CodeOf(err)
		if code == "artifact-digest-mismatch" || code == "artifact-changed-during-verification" {
			r.Axes["referenceIntegrity"] = "FAILED"
			exit = r.reject("artifacts", code)
			return r, exit
		}
		exit = r.Refuse("artifacts", code, true)
		return r, exit
	}
	if stableHasSignedReceipt(d) {
		tree, err := repo.CommitTree(ctx, target)
		if err != nil {
			exit = fail("repository", err)
			return r, exit
		}
		axis, err := stableAuthentication(d, contents, tree, o.TrustedKeys)
		if err != nil {
			r.Axes["referenceIntegrity"] = "FAILED"
			r.Axes["authentication"] = AuthenticationFailed
			exit = r.reject("artifacts", cemcode.CodeOf(err))
			return r, exit
		}
		r.Axes["authentication"] = axis
	}
	// CEM-V1-334: an external criterion is observed only from its supplied
	// repository; otherwise the verification abstains and never accepts.
	if err = checkStableExternal(ctx, repo, d.Criteria, o.ExternalRepositories, budget); err != nil {
		switch code := cemcode.CodeOf(err); code {
		case stableExternalMismatch:
			r.Axes["referenceIntegrity"] = "FAILED"
			exit = r.reject("artifacts", code)
		case gitrun.ProcessContainment:
			exit = r.Refuse("repository", code, true)
		default:
			exit = r.Refuse("artifacts", code, true)
		}
		return r, exit
	}
	r.ArtifactChecks = checks
	r.Axes["referenceIntegrity"] = "REFERENCE_INTEGRITY_ONLY"
	if len(d.Artifacts) == 0 {
		r.Axes["referenceIntegrity"] = "EMPTY_REFERENCE_SET"
	}
	// Terminal order: revalidate the retained repository boundary, complete
	// process teardown, sample cancellation and the deadline once more, publish.
	budget.Notify("artifacts-complete")
	if err = boundary.Validate(); err != nil {
		exit = r.Refuse("repository", "repository-object-unavailable", true)
		return r, exit
	}
	budget.Notify("boundary-revalidated")
	if err = closeSession(); err != nil || budget.Held() {
		exit = r.Refuse("repository", gitrun.ProcessContainment, true)
		return r, exit
	}
	r.Citation = ComputeCitation(&change, parsed, spans)
	budget.Notify("final-check")
	if timedOut() {
		exit = r.Refuse("verification", "verification-timeout", true)
		return r, exit
	}
	r.Outcome = "ACCEPT"
	b := true
	r.Accept = &b
	r.Stage = "complete"
	if o.capture != nil {
		*o.capture = stableCapture{change: &change, parsed: parsed, target: target}
	}
	return r, 0
}

// stableAdmissionCode maps a metadata admission refusal to its public code.
func stableAdmissionCode(err error) string {
	code := cemcode.CodeOf(err)
	if code == cemcode.UnsupportedRepositoryAttributes {
		return "unsupported-repository-envelope"
	}
	return code
}

// stableRepositoryEnvelope reports whether root passes the stable metadata
// admission. Stable itself retains the admitted boundary for the whole call.
func stableRepositoryEnvelope(root string) error {
	_, boundary, err := gitauth.OpenStable(root, nil, nil)
	if err != nil {
		return cemcode.New(stableAdmissionCode(err), "repository outside the stable envelope")
	}
	boundary.Close()
	return nil
}

// Structural predicates deliberately return bool; retain source failures at the
// boundary so unavailable bytes cannot become an observed predicate rejection.
type stableProofSource struct {
	source sim.BlobSource
	err    error
}

func (s *stableProofSource) BaseBlob(path string) ([]byte, string, bool, error) {
	b, m, ok, e := s.source.BaseBlob(path)
	if e != nil {
		s.err = e
	}
	return b, m, ok, e
}

type stableSimulationSource struct {
	source                     sim.BlobSource
	authenticatedCreateAbsent  map[string]bool
	authenticatedCreateBlocked map[string]bool
}

func stableAuthenticatedCreateAbsences(parsed *patch.Patch, createDestinations map[string]gitauth.TreeEntry) (map[string]bool, map[string]bool, error) {
	absent := map[string]bool{}
	blocked := map[string]bool{}
	for _, group := range parsed.Groups {
		if group.Kind != patch.KindCreate || group.NewPath == nil {
			continue
		}
		path := *group.NewPath
		entry, ok := createDestinations[path]
		if !ok {
			return nil, nil, cemcode.New("unsupported-patch-inventory", "create destination %q has no authenticated base entry proof", path)
		}
		if entry.OID == "" {
			absent[path] = true
			continue
		}
		blocked[path] = true
	}
	return absent, blocked, nil
}

func (s *stableSimulationSource) BaseBlob(path string) ([]byte, string, bool, error) {
	if s.authenticatedCreateBlocked[path] {
		return nil, "", false, cemcode.New(cemcode.InvalidField, "create destination %q exists in the base tree", path)
	}
	if s.authenticatedCreateAbsent[path] {
		return nil, "", false, nil
	}
	return s.source.BaseBlob(path)
}

// stablePatchRefusal keeps the frozen stable result vocabulary closed: the
// stable profile's code and issueCodes enums have no metadata-only code, so a
// metadata-only-change-unsupported parse refusal reports the pre-V1-0609
// malformed-patch code and the stable result bytes stay unchanged
// (CEM-V1-302).
func stablePatchRefusal(err error) error {
	if cemcode.CodeOf(err) != cemcode.MetadataOnlyChangeUnsupported {
		return err
	}
	return cemcode.New(cemcode.MalformedPatch, "%s", cemcode.MessageOf(err))
}
