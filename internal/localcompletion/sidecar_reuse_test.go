package localcompletion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cli"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// requireStableHost skips where the stable verifier's owned Git lifecycle is
// not admitted; there every reuse is refused sidecar-unverifiable instead.
func requireStableHost(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("NOT_RUN: the stable verifier runs on Darwin and Linux only")
	}
}

func cemRun(t *testing.T, root string, args ...string) {
	t.Helper()
	var stdout, stderr strings.Builder
	if cli.Run(context.Background(), root, args, &stdout, &stderr) != 0 {
		t.Fatalf("cem %v: %s %s", args, stdout.String(), stderr.String())
	}
}

func commitSource(t *testing.T, root, body, message string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "source.go"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	localGit(t, root, "add", "source.go")
	localGit(t, root, "commit", "-qm", message)
	return localGit(t, root, "rev-parse", "HEAD")
}

func sidecarPath(root string) string {
	return filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath))
}

// commitSidecar commits exactly raw as the sidecar and nothing else.
func commitSidecar(t *testing.T, root string, raw []byte) string {
	t.Helper()
	if err := os.WriteFile(sidecarPath(root), raw, 0600); err != nil {
		t.Fatal(err)
	}
	localGit(t, root, "add", "-f", wire.ExcludedCEMPath)
	localGit(t, root, "commit", "-qm", "sidecar")
	return localGit(t, root, "rev-parse", "HEAD")
}

// prepareStable writes the default cem/1.0 producer map of base..HEAD.
func prepareStable(t *testing.T, root, base string) []byte {
	t.Helper()
	cemRun(t, root, "prepare", "--base", base, "--target", "HEAD", "--replace")
	raw, err := os.ReadFile(sidecarPath(root))
	if err != nil || !wire.DeclaresStable(raw) {
		t.Fatalf("prepare did not write cem/1.0: %v %.80s", err, raw)
	}
	return raw
}

// reuseFixture is a passing observation on a change commit followed by a
// commit that adds only the given sidecar bytes (nil: a fresh producer map).
func reuseFixture(t *testing.T, allow bool) (root, key string, plan Plan, repo *repository) {
	t.Helper()
	requireStableHost(t)
	root, key, plan = fixture(t, []string{"true"})
	plan.Checks[0].AllowCemSidecarOnlyReuse = allow
	commitSource(t, root, "package fixture\nconst Answer = 42\n", "change")
	repo = beginFixture(t, root, key, plan)
	if _, err := Verify(context.Background(), root, key, "test"); err != nil {
		t.Fatal(err)
	}
	return root, key, plan, repo
}

func evaluateReuse(t *testing.T, root, key string) Evaluation {
	t.Helper()
	result, err := Evaluate(context.Background(), root, key)
	if err != nil || len(result.Checks) != 1 {
		t.Fatalf("evaluate: %#v %v", result, err)
	}
	return result
}

func requireGranted(t *testing.T, result Evaluation) {
	t.Helper()
	check := result.Checks[0]
	if hasUnmet(result, "selected-check-unverified") || !check.Qualified || check.ReuseRefused != "" || check.TestedCommit == check.CurrentTarget {
		t.Fatalf("reuse not granted: %#v", result)
	}
}

func requireRefused(t *testing.T, result Evaluation, reason string) {
	t.Helper()
	check := result.Checks[0]
	if !hasUnmet(result, "selected-check-unverified") || check.Qualified || check.ReuseRefused != reason {
		t.Fatalf("reuse refusal = %q (want %q): %#v", check.ReuseRefused, reason, result)
	}
	want := []string{"corvint", "dogfood", "verify", "--session-key", HashSession(t.Name()), "--check", "test"}
	if len(result.NextActions) != 1 || strings.Join(result.NextActions[0], " ") != strings.Join(want, " ") {
		t.Fatalf("refused reuse does not rerun the check: %#v", result.NextActions)
	}
}

// directReuse calls the decision itself so the verifier code is visible.
func directReuse(t *testing.T, repo *repository) (string, string) {
	t.Helper()
	saved, err := repo.load()
	if err != nil {
		t.Fatal(err)
	}
	snap, err := repo.snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	reason, code := repo.stableSidecarReuse(context.Background(), saved, repo.latest(saved, "test"), snap)
	t.Logf("sidecar reuse: reason=%q code=%q", reason, code)
	return reason, code
}

// criterionID is the candidate record identity of a plain criterion.
func criterionID(t *testing.T, criterion wire.StableCriterion) string {
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
	for _, name := range value.Obj.Keys {
		if name != "id" {
			trimmed.Keys = append(trimmed.Keys, name)
			trimmed.Values[name] = value.Obj.Values[name]
		}
	}
	sum := sha256.Sum256(wire.CanonicalValue(wire.Value{Kind: wire.KindObject, Obj: trimmed}))
	return wire.CandidateCriterionPrefix + hex.EncodeToString(sum[:])
}

// stableMapWithArtifacts cites the producer map's hunk and adds one plain
// criterion whose runner plan and receipt are opaque artifacts under the
// ignored .corvint/artifacts directory of the repository root.
func stableMapWithArtifacts(t *testing.T, root, base string) ([]byte, []string) {
	t.Helper()
	prepareStable(t, root, base)
	cemRun(t, root, "cite", "--map", wire.ExcludedCEMPath, "--hunk", "1", "--evidence-path", "intent.md", "--lines", "1:5", "--relation", "specification")
	raw, err := os.ReadFile(sidecarPath(root))
	if err != nil {
		t.Fatal(err)
	}
	producer, err := wire.ParseStable(raw)
	if err != nil || len(producer.Change.Hunks) != 1 || len(producer.Change.Hunks[0].Basis) != 1 {
		t.Fatalf("cited producer map: %v", err)
	}
	hunk := producer.Change.Hunks[0]
	evidenceID := hunk.Basis[0].EvidenceID
	var spanDigest string
	for _, evidence := range producer.Change.Evidence {
		if evidence.ID == evidenceID {
			spanDigest = evidence.SpanSha256
		}
	}
	var paths []string
	artifact := func(kind string) string {
		path := ".corvint/artifacts/" + kind + ".json"
		data := []byte(`{"fixture":"` + kind + `"}` + "\n")
		if err := os.MkdirAll(filepath.Join(root, ".corvint", "artifacts"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), data, 0600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
		return digest(data)
	}
	planDigest, receiptDigest := artifact("runner-plan"), artifact("runner-receipt")
	criterion := wire.StableCriterion{CriterionIndex: 0, CriterionSha256: spanDigest, EvidenceID: evidenceID}
	criterion.ID = criterionID(t, criterion)
	document := &wire.StableMap{
		Change:   producer.Change,
		Criteria: []wire.StableCriterion{criterion},
		Receipts: []wire.StableReceipt{{Sha256: receiptDigest, Profile: "corvint-test-runner-receipt/0", PlanSha256: planDigest,
			SourceGitBinding: "NOT_OBSERVED", ExecutionAuthority: "CALLER_OBSERVED", DependencyClosure: "NOT_OBSERVED"}},
		Links: []wire.StableLink{{CriterionID: criterion.ID, HunkIDs: []string{hunk.ID}, EvidenceIDs: []string{evidenceID}, RunnerReceiptSha256s: []string{receiptDigest}}},
		Artifacts: []wire.StableArtifact{
			{Kind: "runner-plan", Path: paths[0], Sha256: planDigest},
			{Kind: "runner-receipt", Path: paths[1], Sha256: receiptDigest},
		},
	}
	encoded, err := wire.EncodeStable(document)
	if err != nil {
		t.Fatal(err)
	}
	return encoded, paths
}

// TestCEMV1205StableSidecarAcceptGrantsReuse (CEM-V1-205): a committed cem/1.0
// sidecar that the stable verifier accepts, as the only difference from the
// observed target, qualifies the check without a rerun; the tested commit and
// the reused target stay separate.
func TestCEMV1205StableSidecarAcceptGrantsReuse(t *testing.T) {
	root, key, plan, repo := reuseFixture(t, true)
	observed := localGit(t, root, "rev-parse", "HEAD")
	target := commitSidecar(t, root, prepareStable(t, root, plan.Base))
	result := evaluateReuse(t, root, key)
	requireGranted(t, result)
	if result.Checks[0].TestedCommit != observed || result.Checks[0].CurrentTarget != target {
		t.Fatalf("reuse rewrote the tested commit: %#v", result.Checks[0])
	}
	if reason, code := directReuse(t, repo); reason != "" || code != "" {
		t.Fatalf("direct decision = %q %q", reason, code)
	}
	saved, _ := repo.load()
	if len(saved.Observations) != 1 || saved.Observations[0].Target != observed {
		t.Fatal("reuse recorded a new observation")
	}
}

// TestCEMV1205ArtifactRootIsRepositoryRoot (CEM-V1-205, CEM-V1-207): the
// verifier reads the map's artifacts under the absolute repository root, so a
// map whose artifacts are present grants reuse and one whose artifacts are
// missing is unverifiable and refused.
func TestCEMV1205ArtifactRootIsRepositoryRoot(t *testing.T) {
	root, key, plan, repo := reuseFixture(t, true)
	raw, paths := stableMapWithArtifacts(t, root, plan.Base)
	commitSidecar(t, root, raw)
	requireGranted(t, evaluateReuse(t, root, key))
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(paths[1]))); err != nil {
		t.Fatal(err)
	}
	requireRefused(t, evaluateReuse(t, root, key), reuseUnverifiable)
	if reason, code := directReuse(t, repo); reason != reuseUnverifiable || code != "artifact-unavailable" {
		t.Fatalf("missing artifact = %q %q", reason, code)
	}
}

// TestCEMV1206EditedSidecarRefusesReuse (CEM-V1-206): the verifier reads the
// exact sidecar bytes. A committed edit of the map's patch digest is rejected,
// and so is a semantically equal re-spacing of the committed sidecar that was
// not committed: nothing is re-encoded or compared by meaning.
func TestCEMV1206EditedSidecarRefusesReuse(t *testing.T) {
	root, key, plan, repo := reuseFixture(t, true)
	valid := prepareStable(t, root, plan.Base)
	marker := `"patchSha256":"`
	at := strings.Index(string(valid), marker) + len(marker)
	if at < len(marker) {
		t.Fatalf("fixture map has no patch digest: %.200s", valid)
	}
	edited := append([]byte(nil), valid...)
	if edited[at] == '0' {
		edited[at] = '1'
	} else {
		edited[at] = '0'
	}
	commitSidecar(t, root, edited)
	requireRefused(t, evaluateReuse(t, root, key), reuseRejected)
	if reason, code := directReuse(t, repo); reason != reuseRejected || code != "patch-digest-mismatch" {
		t.Fatalf("committed edit = %q %q", reason, code)
	}
	commitSidecar(t, root, valid)
	requireGranted(t, evaluateReuse(t, root, key))
	respaced := []byte(strings.Replace(string(valid), marker, `"patchSha256": "`, 1))
	if err := os.WriteFile(sidecarPath(root), respaced, 0600); err != nil {
		t.Fatal(err)
	}
	if reason, code := directReuse(t, repo); reason != reuseRejected || code != "excluded-artifact-mismatch" {
		t.Fatalf("uncommitted re-spacing = %q %q", reason, code)
	}
}

// TestCEMV1207StaleSidecarRefusesReuse (CEM-V1-207): a sidecar prepared for an
// earlier target is rejected by the verifier, and a source change after the
// sidecar refuses before any verification.
func TestCEMV1207StaleSidecarRefusesReuse(t *testing.T) {
	requireStableHost(t)
	root, key, plan := fixture(t, []string{"true"})
	plan.Checks[0].AllowCemSidecarOnlyReuse = true
	commitSource(t, root, "package fixture\nconst Answer = 41\n", "earlier")
	stale := prepareStable(t, root, plan.Base)
	if err := os.Remove(sidecarPath(root)); err != nil {
		t.Fatal(err)
	}
	commitSource(t, root, "package fixture\nconst Answer = 42\n", "change")
	repo := beginFixture(t, root, key, plan)
	if _, err := Verify(context.Background(), root, key, "test"); err != nil {
		t.Fatal(err)
	}
	commitSidecar(t, root, stale)
	requireRefused(t, evaluateReuse(t, root, key), reuseRejected)
	if reason, code := directReuse(t, repo); reason != reuseRejected || code != "patch-digest-mismatch" {
		t.Fatalf("stale map = %q %q", reason, code)
	}
	commitSidecar(t, root, prepareStable(t, root, plan.Base))
	requireGranted(t, evaluateReuse(t, root, key))
	commitSource(t, root, "package fixture\nconst Answer = 43\n", "drift")
	requireRefused(t, evaluateReuse(t, root, key), reuseContentChanged)
}

// TestCEMV1207WrongBaseSidecarRefusesReuse (CEM-V1-207): a sidecar whose base
// is not the plan's base is rejected by the verifier's expected-base check.
func TestCEMV1207WrongBaseSidecarRefusesReuse(t *testing.T) {
	requireStableHost(t)
	root, key, plan := fixture(t, []string{"true"})
	plan.Checks[0].AllowCemSidecarOnlyReuse = true
	other := commitSource(t, root, "package fixture\nconst Answer = 41\n", "earlier")
	commitSource(t, root, "package fixture\nconst Answer = 42\n", "change")
	repo := beginFixture(t, root, key, plan)
	if _, err := Verify(context.Background(), root, key, "test"); err != nil {
		t.Fatal(err)
	}
	commitSidecar(t, root, prepareStable(t, root, other))
	requireRefused(t, evaluateReuse(t, root, key), reuseRejected)
	if reason, code := directReuse(t, repo); reason != reuseRejected || code != "base-revision-mismatch" {
		t.Fatalf("wrong base = %q %q", reason, code)
	}
}

// TestCEMV1207UnboundSidecarRefusesReuse (CEM-V1-207): an absent sidecar, an
// accepted map that is not committed at the target, and abbreviated object
// IDs each refuse with their closed reason.
func TestCEMV1207UnboundSidecarRefusesReuse(t *testing.T) {
	root, key, plan, repo := reuseFixture(t, true)
	localGit(t, root, "commit", "-q", "--allow-empty", "-m", "empty")
	requireRefused(t, evaluateReuse(t, root, key), reuseSidecarUnavailable)
	// .corvint/ is ignored, so an uncommitted map leaves the tree clean.
	prepareStable(t, root, plan.Base)
	requireRefused(t, evaluateReuse(t, root, key), reuseBindingMismatch)
	commitSidecar(t, root, prepareStable(t, root, plan.Base))
	requireGranted(t, evaluateReuse(t, root, key))
	saved, err := repo.load()
	if err != nil {
		t.Fatal(err)
	}
	snap, err := repo.snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	observed := repo.latest(saved, "test")
	saved.Plan.Base = saved.Plan.Base[:12]
	if reason, _ := repo.stableSidecarReuse(context.Background(), saved, observed, snap); reason != reuseIDsNotFull {
		t.Fatalf("abbreviated base = %q", reason)
	}
}

// TestCEMV1208SidecarReuseFailsClosedByDefault (CEM-V1-208): without the plan's
// allowCemSidecarOnlyReuse an accepted sidecar grants nothing and reports no
// reuse reason: the check reruns on the current target (CEM-V1-150).
func TestCEMV1208SidecarReuseFailsClosedByDefault(t *testing.T) {
	root, key, plan, _ := reuseFixture(t, false)
	commitSidecar(t, root, prepareStable(t, root, plan.Base))
	result := evaluateReuse(t, root, key)
	if !hasUnmet(result, "selected-check-unverified") || result.Checks[0].Qualified || result.Checks[0].ReuseRefused != "" {
		t.Fatalf("default plan granted reuse: %#v", result)
	}
}
