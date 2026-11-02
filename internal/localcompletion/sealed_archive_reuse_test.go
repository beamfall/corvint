package localcompletion

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cli"
	"github.com/Beamfall/corvint/internal/dogfoodflow"
)

// sealArchiveFixture verifies the plan's one check at a bind commit, then
// seals it with the exact rename of the map, and returns the loaded state
// with its sealed continuation recorded as finish records it.
func sealArchiveFixture(t *testing.T, allow bool) (root string, repo *repository, saved *state, snap snapshot) {
	t.Helper()
	root, key, plan := fixture(t, []string{"true"})
	plan.Checks[0].AllowSealArchiveReuse = allow
	repo = beginFixture(t, root, key, plan)
	if err := os.WriteFile(filepath.Join(root, "source.go"), []byte("package fixture\nconst Answer = 2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	localGit(t, root, "add", "source.go")
	localGit(t, root, "commit", "-qm", "change")
	var out, errout strings.Builder
	if code := cli.Run(context.Background(), root, []string{"prepare", "--base", plan.Base, "--target", "HEAD"}, &out, &errout); code != 0 {
		t.Fatalf("prepare: %d %s %s", code, out.String(), errout.String())
	}
	localGit(t, root, "add", "-f", ".corvint/change.cem.json")
	localGit(t, root, "commit", "-qm", "bind")
	bind := localGit(t, root, "rev-parse", "HEAD")
	if _, err := Verify(context.Background(), root, key, "test"); err != nil {
		t.Fatal(err)
	}
	archive := ".corvint/changes/" + bind + ".cem.json"
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, archive)), 0755); err != nil {
		t.Fatal(err)
	}
	localGit(t, root, "mv", ".corvint/change.cem.json", archive)
	localGit(t, root, "commit", "-qm", "seal")
	var err error
	if snap, err = repo.snapshot(context.Background()); err != nil {
		t.Fatal(err)
	}
	binding, err := dogfoodflow.ResolveSealBinding(context.Background(), root, snap.target)
	if err != nil || binding.EvidenceTarget != bind {
		t.Fatalf("seal binding: %#v %v", binding, err)
	}
	if saved, err = repo.load(); err != nil {
		t.Fatal(err)
	}
	if len(saved.Observations) != 1 || saved.Observations[0].Target != bind {
		t.Fatalf("observations: %#v", saved.Observations)
	}
	saved.Sealed = &sealedContinuation{Binding: binding}
	return root, repo, saved, snap
}

// TestLCPV0920SealArchiveReuseNeedsTheExactSeal (LCP-V0-920,
// ci-keyed-finish-seal-compat): a check whose frozen plan declares
// allowSealArchiveReuse qualifies at the seal from its one observation at the
// evidence target, with no second run and the tested commit kept distinct from
// the seal; every other case reruns: no declaration, no recorded continuation,
// a continuation naming another target, a later failed observation, a missing
// log, and a descendant that is not the pure move. The field is omitted from a
// plan that leaves it false, so existing plan digests hold.
func TestLCPV0920SealArchiveReuseNeedsTheExactSeal(t *testing.T) {
	ctx := context.Background()
	root, repo, saved, snap := sealArchiveFixture(t, true)
	check := saved.Plan.Checks[0]
	bind := saved.Observations[0].Target
	if qualified, refused := repo.checkQualification(ctx, saved, check, snap); !qualified || refused != "" {
		t.Fatalf("declared check at its exact seal: qualified=%v refused=%q", qualified, refused)
	}
	if len(saved.Observations) != 1 || saved.Observations[0].Target != bind || bind == snap.target {
		t.Fatalf("reuse rewrote or added an observation: %#v (seal %s)", saved.Observations, snap.target)
	}

	undeclared := check
	undeclared.AllowSealArchiveReuse = false
	if repo.checkQualifies(ctx, saved, undeclared, snap) {
		t.Error("an undeclared check reused its pre-seal observation")
	}
	unsealed := *saved
	unsealed.Sealed = nil
	if repo.checkQualifies(ctx, &unsealed, check, snap) {
		t.Error("reuse without a recorded sealed continuation")
	}
	retargeted := *saved
	other := *saved.Sealed
	other.Binding.EvidenceTarget = saved.Plan.Base
	retargeted.Sealed = &other
	if repo.checkQualifies(ctx, &retargeted, check, snap) {
		t.Error("reuse under a continuation naming another evidence target")
	}
	failed := *saved
	later := saved.Observations[0]
	later.Passed, later.Exit = false, "1"
	failed.Observations = append(append([]observation{}, saved.Observations...), later)
	if repo.checkQualifies(ctx, &failed, check, snap) {
		t.Error("a later failed observation was masked by the earlier pass")
	}

	stdout := saved.Observations[0].Stdout.Path
	raw, err := os.ReadFile(stdout)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(stdout); err != nil {
		t.Fatal(err)
	}
	if repo.checkQualifies(ctx, saved, check, snap) {
		t.Error("reuse with a missing retained log")
	}
	if err = os.WriteFile(stdout, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if !repo.checkQualifies(ctx, saved, check, snap) {
		t.Fatal("the restored log did not qualify again")
	}

	if err = os.WriteFile(filepath.Join(root, "source.go"), []byte("package fixture\nconst Answer = 3\n"), 0600); err != nil {
		t.Fatal(err)
	}
	localGit(t, root, "add", "source.go")
	localGit(t, root, "commit", "-qm", "after seal")
	moved, err := repo.snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if repo.checkQualifies(ctx, saved, check, moved) {
		t.Error("reuse at a source change after the seal")
	}

	encoded, err := json.Marshal(Check{ID: "test", Argv: []string{"true"}, TimeoutSeconds: 5})
	if err != nil || strings.Contains(string(encoded), "allowSealArchiveReuse") {
		t.Fatalf("a false declaration changes the frozen plan bytes: %s %v", encoded, err)
	}
}

// TestLCPV0920UndeclaredCheckReruns (LCP-V0-920): without the declaration the
// same exact seal leaves the check unverified, so it reruns at the seal.
func TestLCPV0920UndeclaredCheckReruns(t *testing.T) {
	_, repo, saved, snap := sealArchiveFixture(t, false)
	if qualified, _ := repo.checkQualification(context.Background(), saved, saved.Plan.Checks[0], snap); qualified {
		t.Fatal("an undeclared check qualified at the seal")
	}
}
