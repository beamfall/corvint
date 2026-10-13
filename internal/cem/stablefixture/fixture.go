// Package stablefixture builds real cem/1.0 fixtures for consumer tests. It is
// imported only by tests: it converts one committed, verified cem/1.0 producer
// change into a stable map with a closed criterion/receipt/link graph, writes that
// graph's six artifact files under a new explicit artifact root, and commits
// exactly the stable bytes as the sidecar at a new target.
package stablefixture

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// Stable is one converted change.
type Stable struct {
	// Map is the exact cem/1.0 bytes committed as the sidecar at Target.
	Map []byte
	// Target is the commit that adds only the stable sidecar.
	Target string
	// ArtifactRoot is the explicit directory holding ArtifactPaths.
	ArtifactRoot  string
	ArtifactPaths []string
	CriterionID   string
}

// RequireStableHost skips on hosts where the stable verifier's owned Git
// lifecycle is not observed.
func RequireStableHost(t testing.TB) {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("NOT_RUN: the stable verifier's owned Git lifecycle is observed on Darwin and Linux only")
	}
}

// Digest is the lowercase SHA-256 hex digest of data.
func Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Stabilize converts producerRaw, the cem/1.0 producer map `cem prepare`
// writes (CEM-V1-214), verified at the repository's current HEAD, into a
// cem/1.0 map whose one criterion links linkedHunk, and commits it with git, a
// caller-owned runner that must disable automatic maintenance. The change
// mechanics are kept byte for byte.
func Stabilize(t testing.TB, root string, producerRaw []byte, linkedHunk string, git func(args ...string) string) Stable {
	t.Helper()
	RequireStableHost(t)
	producer, err := changeMechanics(producerRaw)
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	out := Stable{ArtifactRoot: artifacts}
	artifact := func(kind string) (string, wire.StableArtifact) {
		path := "refs/" + kind + ".json"
		data := []byte(`{"fixture":"` + kind + `"}` + "\n")
		write(t, artifacts, path, data)
		out.ArtifactPaths = append(out.ArtifactPaths, path)
		return Digest(data), wire.StableArtifact{Kind: kind, Path: path, Sha256: Digest(data)}
	}
	capture, captureArtifact := artifact("tasks-capture")
	verification, verificationArtifact := artifact("tasks-verification")
	claimed, claimedArtifact := artifact("tasks-claimed-ticket")
	snapshot, snapshotArtifact := artifact("tasks-snapshot-head-receipt")
	plan, planArtifact := artifact("runner-plan")
	receipt, receiptArtifact := artifact("runner-receipt")
	criterion := wire.StableCriterion{
		TicketID: "ticket:beamfall:corvint:V1-0632", AcceptanceRevision: "1", CriterionIndex: 0,
		AcceptanceSha256: Digest([]byte("acceptance")), CriterionSha256: Digest([]byte("criterion")),
		CaptureSha256: capture, VerificationSha256: verification, ClaimTicketSha256: claimed,
		SnapshotHeadReceiptSha256: Digest([]byte("snapshot-receipt")), SnapshotHeadArtifactSha256: snapshot,
	}
	criterion.ID = criterionID(t, criterion)
	out.CriterionID = criterion.ID
	change := wire.StableChange(*producer)
	change.Spec = wire.StableSpec
	var evidenceIDs []string
	for _, hunk := range producer.Hunks {
		if hunk.ID == linkedHunk {
			for _, basis := range hunk.Basis {
				evidenceIDs = append(evidenceIDs, basis.EvidenceID)
			}
		}
	}
	if len(evidenceIDs) == 0 {
		t.Fatalf("linked hunk %s has no evidence basis", linkedHunk)
	}
	document := &wire.StableMap{
		Change:   change,
		Criteria: []wire.StableCriterion{criterion},
		Receipts: []wire.StableReceipt{{
			Sha256: receipt, Profile: "corvint-test-runner-receipt/0", PlanSha256: plan, SourceGitBinding: "NOT_OBSERVED",
			ExecutionAuthority: "CALLER_OBSERVED", DependencyClosure: "NOT_OBSERVED",
		}},
		Links: []wire.StableLink{{
			CriterionID: criterion.ID, HunkIDs: []string{linkedHunk}, EvidenceIDs: evidenceIDs, RunnerReceiptSha256s: []string{receipt},
		}},
		Artifacts: []wire.StableArtifact{captureArtifact, verificationArtifact, claimedArtifact, snapshotArtifact, planArtifact, receiptArtifact},
	}
	if out.Map, err = wire.EncodeStable(document); err != nil {
		t.Fatal(err)
	}
	if !wire.DeclaresStable(out.Map) {
		t.Fatal("fixture map does not declare cem/1.0")
	}
	write(t, root, wire.ExcludedCEMPath, out.Map)
	git("add", "-f", wire.ExcludedCEMPath)
	git("commit", "-qm", "stable sidecar")
	out.Target = git("rev-parse", "HEAD")
	return out
}

// changeMechanics parses the cem/1.0 change Stabilize converts through the
// stable decoder.
func changeMechanics(raw []byte) (*wire.Map, error) {
	document, err := wire.ParseStable(raw)
	if err != nil {
		return nil, err
	}
	change := wire.Map(document.Change)
	return &change, nil
}

// RebindOCM rewrites only an OCM's target and CEM map digest and returns its
// canonical bytes, so the same obligations bind the stable map and target.
func RebindOCM(t testing.TB, raw []byte, target, mapSHA256 string) []byte {
	t.Helper()
	value, err := wire.Parse(raw)
	if err != nil || value.Kind != wire.KindObject {
		t.Fatalf("OCM: %v", err)
	}
	value.Obj.Values["targetRevision"] = wire.Value{Kind: wire.KindString, Str: target}
	value.Obj.Values["cem"].Obj.Values["mapSha256"] = wire.Value{Kind: wire.KindString, Str: mapSHA256}
	return append(wire.CanonicalValue(value), '\n')
}

func criterionID(t testing.TB, criterion wire.StableCriterion) string {
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
	return wire.CandidateCriterionPrefix + Digest(wire.CanonicalValue(wire.Value{Kind: wire.KindObject, Obj: trimmed}))
}

func write(t testing.TB, root, path string, data []byte) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
