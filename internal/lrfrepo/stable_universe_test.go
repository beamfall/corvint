package lrfrepo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/lrf"
)

// stableUniverseFixture is one real cem/1.0 change with a closed reference
// graph, its exact sidecar committed at the stable target, its six artifact
// files, and an OCM with one linked and one still-unknown obligation.
type stableUniverseFixture struct {
	root, artifacts           string
	base, legacyTarget        string // legacyTarget is the change without the sidecar
	target                    string // target adds only the exact stable sidecar
	stable, stableOCM         []byte
	legacyCEM, legacyOCM      []byte
	artifactPaths             []string
	linkedHunk, unknownHunkID string
}

func stableDigestHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func makeStableUniverseFixture(t *testing.T) stableUniverseFixture {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("NOT_RUN: the stable verifier's owned Git lifecycle is observed on Darwin and Linux only")
	}
	ctx := context.Background()
	f := stableUniverseFixture{}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f.root = root
	gitOCMPrepare(t, root, "init", "-q", "-b", "main")
	writeOCMPrepareFile(t, root, "docs/specs/intent.md", "# Intent\n\n## Requirements\n\n- `OCM-TEST-001`: preserve the review.\n- `OCM-TEST-002`: keep the gap visible.\n\n## Non-goals\n")
	writeOCMPrepareFile(t, root, "internal/example.txt", "before\n")
	gitOCMPrepare(t, root, "add", ".")
	gitOCMPrepare(t, root, "commit", "-qm", "base")
	f.base = gitOCMPrepare(t, root, "rev-parse", "HEAD")
	const testPath = "example_test.go"
	source := "package example\nimport \"testing\"\nfunc TestAnchors(t *testing.T) {\n t.Run(\"OCM-TEST-001 exact anchor\", func(t *testing.T) {})\n}\n"
	writeOCMPrepareFile(t, root, "internal/example.txt", "after\n")
	writeOCMPrepareFile(t, root, testPath, source)
	gitOCMPrepare(t, root, "add", ".")
	gitOCMPrepare(t, root, "commit", "-qm", "change")
	f.legacyTarget = gitOCMPrepare(t, root, "rev-parse", "HEAD")

	// The cem/0.2 producer flow supplies real hunks, evidence and an OCM.
	prepareCEM(t, root, f.base, f.legacyTarget, false)
	session, err := workflow.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	legacyRaw, err := os.ReadFile(filepath.Join(root, wire.ExcludedCEMPath))
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := wire.ParseMap(legacyRaw)
	if err != nil || len(legacy.Hunks) != 2 {
		t.Fatalf("legacy map: hunks=%d err=%v", len(legacy.Hunks), err)
	}
	citeOrdinal := "1"
	if legacy.Hunks[0].Path != "internal/example.txt" {
		citeOrdinal = "2"
	}
	if _, err := session.Cite(ctx, workflow.CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: citeOrdinal, EvidencePath: "docs/specs/intent.md", Lines: "5:5", Relation: "specification",
	}); err != nil {
		t.Fatal(err)
	}
	if f.legacyCEM, err = os.ReadFile(filepath.Join(root, wire.ExcludedCEMPath)); err != nil {
		t.Fatal(err)
	}
	if legacy, err = wire.ParseMap(f.legacyCEM); err != nil {
		t.Fatal(err)
	}
	for _, hunk := range legacy.Hunks {
		if hunk.Disposition == "supported" {
			f.linkedHunk = hunk.ID
		} else {
			f.unknownHunkID = hunk.ID
		}
	}
	const ocmPath = ".corvint/change.ocm.json"
	if _, err := PrepareOCM(ctx, root, PrepareOptions{
		MapPath: ocmPath, CEMPath: wire.ExcludedCEMPath, IntentPath: "docs/specs/intent.md", ExpectedBase: f.base, Target: f.legacyTarget,
	}); err != nil {
		t.Fatal(err)
	}
	blobOID := gitOCMPrepare(t, root, "rev-parse", f.legacyTarget+":"+testPath)
	claims, err := enumerateClaims(testPath, blobOID, []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LinkOCM(ctx, root, LinkOptions{
		MapPath: ocmPath, CEMPath: wire.ExcludedCEMPath, Obligation: "OCM-TEST-001", Hunks: []string{f.linkedHunk},
		TestPath: testPath, Claims: []string{caseClaim(t, claims).selector}, ExpectedBase: f.base, Target: f.legacyTarget,
	}); err != nil {
		t.Fatal(err)
	}
	if f.legacyOCM, err = os.ReadFile(filepath.Join(root, ocmPath)); err != nil {
		t.Fatal(err)
	}

	// The stable map keeps the same change mechanics and adds one closed
	// criterion/receipt/link graph whose six artifacts live under an
	// explicit root outside the repository.
	artifacts, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f.artifacts = artifacts
	digest := func(kind string) (string, wire.StableArtifact) {
		path := "refs/" + kind + ".json"
		data := []byte(`{"fixture":"` + kind + `"}` + "\n")
		writeOCMPrepareFile(t, artifacts, path, string(data))
		f.artifactPaths = append(f.artifactPaths, path)
		return stableDigestHex(data), wire.StableArtifact{Kind: kind, Path: path, Sha256: stableDigestHex(data)}
	}
	capture, captureArtifact := digest("tasks-capture")
	verification, verificationArtifact := digest("tasks-verification")
	claimed, claimedArtifact := digest("tasks-claimed-ticket")
	snapshot, snapshotArtifact := digest("tasks-snapshot-head-receipt")
	plan, planArtifact := digest("runner-plan")
	receipt, receiptArtifact := digest("runner-receipt")
	criterion := wire.StableCriterion{
		TicketID: "ticket:beamfall:corvint:V1-0632", AcceptanceRevision: "1", CriterionIndex: 0,
		AcceptanceSha256: stableDigestHex([]byte("acceptance")), CriterionSha256: stableDigestHex([]byte("criterion")),
		CaptureSha256: capture, VerificationSha256: verification, ClaimTicketSha256: claimed,
		SnapshotHeadReceiptSha256: stableDigestHex([]byte("snapshot-receipt")), SnapshotHeadArtifactSha256: snapshot,
	}
	criterion.ID = stableCriterionID(t, criterion)
	change := wire.StableChange(*legacy)
	change.Spec = wire.StableSpec
	var evidenceIDs []string
	for _, hunk := range legacy.Hunks {
		if hunk.ID == f.linkedHunk {
			for _, basis := range hunk.Basis {
				evidenceIDs = append(evidenceIDs, basis.EvidenceID)
			}
		}
	}
	document := &wire.StableMap{
		Change:   change,
		Criteria: []wire.StableCriterion{criterion},
		Receipts: []wire.StableReceipt{{
			Sha256: receipt, Profile: "corvint-test-runner-receipt/0", PlanSha256: plan, SourceGitBinding: "NOT_OBSERVED",
			ExecutionAuthority: "CALLER_OBSERVED", DependencyClosure: "NOT_OBSERVED", Authentication: "NOT_OBSERVED",
		}},
		Links: []wire.StableLink{{
			CriterionID: criterion.ID, HunkIDs: []string{f.linkedHunk}, EvidenceIDs: evidenceIDs, RunnerReceiptSha256s: []string{receipt},
		}},
		Artifacts: []wire.StableArtifact{captureArtifact, verificationArtifact, claimedArtifact, snapshotArtifact, planArtifact, receiptArtifact},
	}
	if f.stable, err = wire.EncodeStable(document); err != nil {
		t.Fatal(err)
	}
	if !wire.DeclaresStable(f.stable) {
		t.Fatal("fixture map does not declare cem/1.0")
	}

	// The stable target commits the exact stable bytes as the sidecar and
	// nothing else, so its canonical patch is the change's own patch.
	writeOCMPrepareFile(t, root, wire.ExcludedCEMPath, string(f.stable))
	gitOCMPrepare(t, root, "add", "-f", wire.ExcludedCEMPath)
	gitOCMPrepare(t, root, "commit", "-qm", "sidecar")
	f.target = gitOCMPrepare(t, root, "rev-parse", "HEAD")
	f.stableOCM = rebindOCM(t, f.legacyOCM, f.target, stableDigestHex(f.stable))
	return f
}

func stableCriterionID(t *testing.T, criterion wire.StableCriterion) string {
	t.Helper()
	raw, err := json.Marshal(criterion)
	if err != nil {
		t.Fatal(err)
	}
	value, err := wire.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	trimmed := &wire.Object{Values: map[string]wire.Value{}}
	for _, key := range value.Obj.Keys {
		if key != "id" {
			trimmed.Keys = append(trimmed.Keys, key)
			trimmed.Values[key] = value.Obj.Values[key]
		}
	}
	return wire.CandidateCriterionPrefix + stableDigestHex(wire.CanonicalValue(wire.Value{Kind: wire.KindObject, Obj: trimmed}))
}

// rebindOCM rewrites only the OCM target and CEM map digest and returns the
// canonical bytes, so the same obligations bind a different map or target.
func rebindOCM(t *testing.T, raw []byte, target, mapSHA256 string) []byte {
	t.Helper()
	value, err := wire.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	value.Obj.Values["targetRevision"] = stringValue(target)
	value.Obj.Values["cem"].Obj.Values["mapSha256"] = stringValue(mapSHA256)
	return canonicalOCMBytes(value)
}

func (f stableUniverseFixture) options() StableUniverseOptions {
	return StableUniverseOptions{Root: f.root, ExpectedBase: f.base, Target: f.target, ArtifactRoot: f.artifacts}
}

func requireStableRefusal(t *testing.T, universe *StableUniverse, err error, outcome, code string) *StableRefusal {
	t.Helper()
	if universe != nil {
		t.Fatalf("refusal yielded a universe: %+v", universe)
	}
	var refusal *StableRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("err=%v, want a retained stable refusal", err)
	}
	if refusal.Result.Outcome != outcome || CodeOf(err) != code || refusal.Result.Accept != nil && *refusal.Result.Accept {
		t.Fatalf("outcome=%s code=%s result=%+v", refusal.Result.Outcome, CodeOf(err), refusal.Result)
	}
	if refusal.Result.Spec != wire.StableSpec || refusal.Result.Profile != "cem-stable-verification/1" {
		t.Fatalf("refusal lost the stable profile: %+v", refusal.Result)
	}
	return refusal
}

func requireNoUniverse(t *testing.T, universe *StableUniverse, err error, code string) {
	t.Helper()
	if universe != nil || CodeOf(err) != code {
		t.Fatalf("universe=%v code=%q err=%v, want %s", universe != nil, CodeOf(err), err, code)
	}
}

// TestCEMV1051StableUniverseAccepts (CEM-V1-051): accepts a verified cem/1.0 change with its exact digest and full stable result.
func TestCEMV1051StableUniverseAccepts(t *testing.T) {
	f := makeStableUniverseFixture(t)
	ctx := context.Background()
	universe, err := VerifyStableUniverse(ctx, f.stable, f.stableOCM, f.options())
	if err != nil {
		t.Fatal(err)
	}
	digest := stableDigestHex(f.stable)
	result := universe.Verification
	if universe.Spec != wire.StableSpec || universe.MapSHA256 != digest || result.Outcome != "ACCEPT" ||
		result.MapSha256 == nil || *result.MapSha256 != digest || result.Sidecar != "EXACT" || result.Stage != "complete" {
		t.Fatalf("universe identity: spec=%s map=%s result=%+v", universe.Spec, universe.MapSHA256, result)
	}
	if universe.BaseRevision != f.base || universe.TargetRevision != f.target || universe.ObjectFormat != "sha1" ||
		universe.PatchSHA256 != *result.PatchSha256 || universe.OCMSHA256 != stableDigestHex(f.stableOCM) {
		t.Fatalf("universe binding: %+v", universe)
	}
	if !bytes.Equal(universe.OriginalBytes(), f.stable) {
		t.Fatal("original bytes are not the exact authenticated input")
	}
	context := universe.LRFRequest.Context
	if context.CEMSpec != wire.StableSpec || context.CEMMapSHA256 != digest || context.OCMMapSHA256 == nil ||
		*context.OCMMapSHA256 != universe.OCMSHA256 || context.TargetRevision == nil || *context.TargetRevision != f.target {
		t.Fatalf("LRF projection context: %+v", context)
	}
	if err := lrf.ValidateStableRequest(universe.LRFRequest); err != nil {
		t.Fatal(err)
	}
}

// TestCEMV1052StableUniverseAuthorityArguments (CEM-V1-052): authenticates artifacts under the explicit root and retains NOT_OBSERVED authority.
func TestCEMV1052StableUniverseAuthorityArguments(t *testing.T) {
	f := makeStableUniverseFixture(t)
	ctx := context.Background()
	universe, err := VerifyStableUniverse(ctx, f.stable, f.stableOCM, f.options())
	if err != nil {
		t.Fatal(err)
	}
	result := universe.Verification
	if len(result.ArtifactChecks) != len(f.artifactPaths) || result.Axes["referenceIntegrity"] != "REFERENCE_INTEGRITY_ONLY" {
		t.Fatalf("artifact checks=%+v axes=%v", result.ArtifactChecks, result.Axes)
	}
	for _, axis := range []string{"nativeAuthority", "authentication", "runnerExecution", "executionAtCommit", "criterionAdequacy"} {
		if result.Axes[axis] != "NOT_OBSERVED" {
			t.Fatalf("axis %s=%s, want NOT_OBSERVED", axis, result.Axes[axis])
		}
	}
	for _, tc := range []struct {
		name   string
		mutate func(*StableUniverseOptions)
		code   string
	}{
		{"missing artifact root", func(o *StableUniverseOptions) { o.ArtifactRoot = "" }, "artifacts-required"},
		{"missing expected base", func(o *StableUniverseOptions) { o.ExpectedBase = "" }, "expected-base-required"},
		{"missing target", func(o *StableUniverseOptions) { o.Target = "" }, "target-required"},
		{"missing repository root", func(o *StableUniverseOptions) { o.Root = "" }, "invalid-arguments"},
	} {
		options := f.options()
		tc.mutate(&options)
		universe, err := VerifyStableUniverse(ctx, f.stable, f.stableOCM, options)
		refusal := requireStableRefusal(t, universe, err, "REJECT", tc.code)
		if refusal.Exit != 2 || refusal.Result.Stage != "authority-arguments" {
			t.Fatalf("%s: exit=%d stage=%s", tc.name, refusal.Exit, refusal.Result.Stage)
		}
	}
}

// TestCEMV1053StableUniverseRetainsUnsatisfiedObligations (CEM-V1-053): retains unsatisfied obligations in OCM order.
func TestCEMV1053StableUniverseRetainsUnsatisfiedObligations(t *testing.T) {
	f := makeStableUniverseFixture(t)
	ctx := context.Background()
	universe, err := VerifyStableUniverse(ctx, f.stable, f.stableOCM, f.options())
	if err != nil {
		t.Fatal(err)
	}
	if len(universe.Obligations) != 2 {
		t.Fatalf("obligations=%+v", universe.Obligations)
	}
	linked, open := universe.Obligations[0], universe.Obligations[1]
	if linked.ID != "OCM-TEST-001" || linked.Disposition != "linked" || len(linked.HunkIDs) != 1 || linked.HunkIDs[0] != f.linkedHunk {
		t.Fatalf("linked obligation=%+v", linked)
	}
	if open.ID != "OCM-TEST-002" || open.Disposition != "unknown" || open.Reason != "unassessed" || len(open.HunkIDs) != 0 {
		t.Fatalf("unsatisfied obligation was not retained: %+v", open)
	}
	if len(universe.LRFRequest.Obligations) != 1 || universe.LRFRequest.Obligations[0].ID != "OCM-TEST-001" {
		t.Fatalf("LRF obligations=%+v", universe.LRFRequest.Obligations)
	}
}

// TestCEMV1054StableUniverseReadOnlyDocument (CEM-V1-054): the change view is read-only and keeps the full stable document.
func TestCEMV1054StableUniverseReadOnlyDocument(t *testing.T) {
	f := makeStableUniverseFixture(t)
	ctx := context.Background()
	universe, err := VerifyStableUniverse(ctx, f.stable, f.stableOCM, f.options())
	if err != nil {
		t.Fatal(err)
	}
	document, err := universe.Document()
	if err != nil {
		t.Fatal(err)
	}
	if document.Change.Spec != wire.StableSpec || len(document.Criteria) != 1 || len(document.Receipts) != 1 ||
		len(document.Links) != 1 || len(document.Artifacts) != 6 || document.StableOriginalDigest() != universe.MapSHA256 {
		t.Fatalf("document lost stable members: %+v", document)
	}
	document.Change.Hunks = nil
	document.Artifacts = nil
	copied := universe.OriginalBytes()
	copied[0] = 'X'
	again, err := universe.Document()
	if err != nil || len(again.Change.Hunks) != 2 || len(again.Artifacts) != 6 || !bytes.Equal(universe.OriginalBytes(), f.stable) {
		t.Fatalf("consumer edits reached the universe: err=%v", err)
	}
	if _, err := wire.EncodeStable(again); err != nil {
		t.Fatal(err)
	}
}

// TestCEMV1055StableLRFRequestAdmission (CEM-V1-055): lrf/0 stays frozen and the stable request has its own admission.
func TestCEMV1055StableLRFRequestAdmission(t *testing.T) {
	f := makeStableUniverseFixture(t)
	ctx := context.Background()
	universe, err := VerifyStableUniverse(ctx, f.stable, f.stableOCM, f.options())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lrf.Evaluate(universe.LRFRequest); CodeOf(err) != "unsupported-lrf-context" {
		t.Fatalf("lrf/0 admitted a cem/1.0 context: %v", err)
	}
	legacy, err := VerifyUniverse(ctx, f.root, f.legacyCEM, f.legacyOCM, f.base, f.legacyTarget)
	if err != nil {
		t.Fatal(err)
	}
	if err := lrf.ValidateStableRequest(legacy.LRFRequest); CodeOf(err) != "unsupported-lrf-context" {
		t.Fatalf("stable admission accepted a cem/0.2 context: %v", err)
	}
	relabeled := universe.LRFRequest
	relabeled.Context.CEMSpec = wire.Spec02
	if err := lrf.ValidateStableRequest(relabeled); CodeOf(err) != "unsupported-lrf-context" {
		t.Fatalf("stable admission accepted a relabeled context: %v", err)
	}
	missingTarget := universe.LRFRequest
	missingTarget.Context.TargetRevision = nil
	if err := lrf.ValidateStableRequest(missingTarget); CodeOf(err) != "unsupported-lrf-context" {
		t.Fatalf("stable admission accepted a context without target: %v", err)
	}
}

// TestCEMV1056StableUniverseRefusesStaleOrTamperedInput (CEM-V1-056): stale, swapped or tampered inputs refuse and keep their evidence.
func TestCEMV1056StableUniverseRefusesStaleOrTamperedInput(t *testing.T) {
	f := makeStableUniverseFixture(t)
	ctx := context.Background()
	// Tampered artifact bytes: REJECT with the full retained result.
	tampered := filepath.Join(f.artifacts, filepath.FromSlash(f.artifactPaths[0]))
	original, err := os.ReadFile(tampered)
	if err != nil {
		t.Fatal(err)
	}
	writeOCMPrepareFile(t, f.artifacts, f.artifactPaths[0], `{"fixture":"swapped"}`+"\n")
	universe, err := VerifyStableUniverse(ctx, f.stable, f.stableOCM, f.options())
	refusal := requireStableRefusal(t, universe, err, "REJECT", "artifact-digest-mismatch")
	if refusal.Exit != 1 || refusal.Result.Stage != "artifacts" || refusal.Result.MapSha256 == nil ||
		*refusal.Result.MapSha256 != stableDigestHex(f.stable) || refusal.Result.Axes["referenceIntegrity"] != "FAILED" {
		t.Fatalf("tampered artifact result=%+v", refusal.Result)
	}
	if err := os.Remove(tampered); err != nil {
		t.Fatal(err)
	}
	universe, err = VerifyStableUniverse(ctx, f.stable, f.stableOCM, f.options())
	requireStableRefusal(t, universe, err, "UNSUPPORTED", "artifact-unavailable")
	writeOCMPrepareFile(t, f.artifacts, f.artifactPaths[0], string(original))

	// A swapped map: different stable bytes than the committed sidecar.
	document, err := wire.ParseStable(f.stable)
	if err != nil {
		t.Fatal(err)
	}
	document.Criteria[0].AcceptanceRevision = "2"
	document.Criteria[0].ID = stableCriterionID(t, document.Criteria[0])
	document.Links[0].CriterionID = document.Criteria[0].ID
	swapped, err := wire.EncodeStable(document)
	if err != nil {
		t.Fatal(err)
	}
	universe, err = VerifyStableUniverse(ctx, swapped, rebindOCM(t, f.legacyOCM, f.target, stableDigestHex(swapped)), f.options())
	requireStableRefusal(t, universe, err, "REJECT", cemcode.ExcludedArtifactMismatch)

	// The OCM still bound to another map digest, or to a stale target.
	universe, err = VerifyStableUniverse(ctx, f.stable, rebindOCM(t, f.legacyOCM, f.target, stableDigestHex(f.legacyCEM)), f.options())
	requireNoUniverse(t, universe, err, "cem-map-digest-mismatch")
	universe, err = VerifyStableUniverse(ctx, f.stable, rebindOCM(t, f.legacyOCM, f.legacyTarget, stableDigestHex(f.stable)), f.options())
	requireNoUniverse(t, universe, err, "target-mismatch")
	stale := f.options()
	stale.Target = f.legacyTarget
	universe, err = VerifyStableUniverse(ctx, f.stable, f.stableOCM, stale)
	requireNoUniverse(t, universe, err, "target-mismatch")

	// A stale expected base refuses inside the verifier.
	stale = f.options()
	stale.ExpectedBase = f.legacyTarget
	universe, err = VerifyStableUniverse(ctx, f.stable, f.stableOCM, stale)
	if universe != nil || err == nil {
		t.Fatalf("stale base yielded a universe: %v", err)
	}
	var refusal2 *StableRefusal
	if !errors.As(err, &refusal2) || refusal2.Result.Outcome == "ACCEPT" {
		t.Fatalf("stale base err=%v", err)
	}

	// An OCM whose obligation was removed no longer equals the intent.
	value, err := wire.Parse(f.stableOCM)
	if err != nil {
		t.Fatal(err)
	}
	obligations := value.Obj.Values["obligations"]
	obligations.Arr = obligations.Arr[:1]
	value.Obj.Values["obligations"] = obligations
	universe, err = VerifyStableUniverse(ctx, f.stable, canonicalOCMBytes(value), f.options())
	if universe != nil || err == nil {
		t.Fatal("an OCM missing an unsatisfied obligation yielded a universe")
	}
}

// TestCEMV1057StableUniverseProfileSwapsRefuse (CEM-V1-057): profile swaps refuse in both directions.
func TestCEMV1057StableUniverseProfileSwapsRefuse(t *testing.T) {
	f := makeStableUniverseFixture(t)
	ctx := context.Background()
	universe, err := VerifyStableUniverse(ctx, f.legacyCEM, f.legacyOCM, StableUniverseOptions{
		Root: f.root, ExpectedBase: f.base, Target: f.legacyTarget, ArtifactRoot: f.artifacts,
	})
	requireNoUniverse(t, universe, err, cemcode.UnsupportedSpec)
	legacy, err := VerifyUniverse(ctx, f.root, f.stable, f.stableOCM, f.base, f.target)
	if legacy != nil || CodeOf(err) != cemcode.UnsupportedSpec || !strings.Contains(err.Error(), "stable universe verification") {
		t.Fatalf("legacy universe admitted cem/1.0: %v", err)
	}
	relabeled := bytes.Replace(f.stable, []byte(`"spec":"cem/1.0"`), []byte(`"spec":"cem/0.2"`), 1)
	universe, err = VerifyStableUniverse(ctx, relabeled, f.stableOCM, f.options())
	requireNoUniverse(t, universe, err, cemcode.UnsupportedSpec)
}

// TestCEMV1058LegacyOCMLRFAdmissionUnchanged (CEM-V1-058): old OCM/LRF admission is unchanged and never reads a stable map.
func TestCEMV1058LegacyOCMLRFAdmissionUnchanged(t *testing.T) {
	f := makeStableUniverseFixture(t)
	ctx := context.Background()
	legacy, err := VerifyUniverse(ctx, f.root, f.legacyCEM, f.legacyOCM, f.base, f.legacyTarget)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.CEM.Spec != wire.Spec02 || legacy.LRFRequest.Context.CEMSpec != wire.Spec02 || len(legacy.Obligations) != 2 {
		t.Fatalf("legacy universe changed: %+v", legacy.LRFRequest.Context)
	}
	if _, err := lrf.Evaluate(legacy.LRFRequest); err != nil {
		t.Fatal(err)
	}
	writeOCMPrepareFile(t, f.root, ".corvint/stable.cem.json", string(f.stable))
	writeOCMPrepareFile(t, f.root, ".corvint/stable.ocm.json", string(f.stableOCM))
	result, err := ReadOCM(ctx, f.root, OCMReadOptions{
		OCMPath: ".corvint/stable.ocm.json", CEMPath: ".corvint/stable.cem.json",
		ExpectedBase: f.base, Target: f.target, ExpectedBaseGiven: true, TargetGiven: true,
	})
	if result != nil || CodeOf(err) != cemcode.UnsupportedSpec {
		t.Fatalf("ocm read reduced a stable map: result=%+v err=%v", result, err)
	}
	before, err := os.ReadFile(filepath.Join(f.root, ".corvint/stable.ocm.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareOCM(ctx, f.root, PrepareOptions{
		MapPath: ".corvint/stable.ocm.json", CEMPath: ".corvint/stable.cem.json", IntentPath: "docs/specs/intent.md",
		ExpectedBase: f.base, Target: f.target, Replace: true,
	}); CodeOf(err) != cemcode.UnsupportedSpec {
		t.Fatalf("ocm prepare wrote from a stable map: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(f.root, ".corvint/stable.ocm.json"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("refused prepare changed the OCM map: %v", err)
	}
	if _, err := LinkOCM(ctx, f.root, LinkOptions{
		MapPath: ".corvint/stable.ocm.json", CEMPath: ".corvint/stable.cem.json", Obligation: "OCM-TEST-002",
		Hunks: []string{f.linkedHunk}, ExpectedBase: f.base, Target: f.target,
	}); CodeOf(err) != cemcode.UnsupportedSpec {
		t.Fatalf("ocm link wrote from a stable map: %v", err)
	}
	if after, err := os.ReadFile(filepath.Join(f.root, ".corvint/stable.ocm.json")); err != nil || !bytes.Equal(before, after) {
		t.Fatalf("refused link changed the OCM map: %v", err)
	}
	if _, err := Evaluate(ctx, f.root, Options{CEMPath: ".corvint/stable.cem.json", ExpectedBase: f.base, Target: f.target}); CodeOf(err) != cemcode.UnsupportedSpec {
		t.Fatalf("lrf admitted a stable map: %v", err)
	}
}

// TestCEMV1059StableUniverseReadDoesNotMutate (CEM-V1-059): a read leaves the repository, map and artifacts unchanged.
func TestCEMV1059StableUniverseReadDoesNotMutate(t *testing.T) {
	f := makeStableUniverseFixture(t)
	ctx := context.Background()
	status := gitOCMPrepare(t, f.root, "status", "--porcelain", "--untracked-files=all")
	head := gitOCMPrepare(t, f.root, "rev-parse", "HEAD")
	snapshot := map[string][]byte{}
	for _, path := range f.artifactPaths {
		data, err := os.ReadFile(filepath.Join(f.artifacts, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		snapshot[path] = data
	}
	cemCopy, ocmCopy := append([]byte(nil), f.stable...), append([]byte(nil), f.stableOCM...)
	if _, err := VerifyStableUniverse(ctx, cemCopy, ocmCopy, f.options()); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(cemCopy, f.stable) || !bytes.Equal(ocmCopy, f.stableOCM) {
		t.Fatal("verification mutated its input bytes")
	}
	if gitOCMPrepare(t, f.root, "status", "--porcelain", "--untracked-files=all") != status || gitOCMPrepare(t, f.root, "rev-parse", "HEAD") != head {
		t.Fatal("verification changed repository state")
	}
	for path, data := range snapshot {
		now, err := os.ReadFile(filepath.Join(f.artifacts, filepath.FromSlash(path)))
		if err != nil || !bytes.Equal(now, data) {
			t.Fatalf("verification changed artifact %s", path)
		}
	}
}
