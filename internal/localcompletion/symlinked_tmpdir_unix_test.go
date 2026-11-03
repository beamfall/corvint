//go:build darwin || linux

package localcompletion

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestLCPV0884SymlinkedTMPDIRPathsAreAdmitted (V1-3665): TestMain resolves
// TMPDIR for the package, so no other test sees a temp root behind a
// symlink, which is how the macOS /tmp -> /private/tmp bug slipped past
// agents whose TMPDIR was already /private/tmp. Here TMPDIR itself is a
// symlink, as on a stock macOS host, and every caller-supplied path (the plan
// read by ReadPlan, the export bundle and the import input) lives under it.
// Repository-root aliases have their own regression below; private local-state
// symlinks remain refused.
func TestLCPV0884SymlinkedTMPDIRPathsAreAdmitted(t *testing.T) {
	ctx := context.Background()
	root, key, plan := fixture(t, []string{"true"})
	restored := filepath.Join(t.TempDir(), "restored")
	localGit(t, root, "worktree", "add", "-q", "--detach", restored, "HEAD")
	restored, err := filepath.EvalSymlinks(restored)
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "tmp")
	if err = os.Symlink(t.TempDir(), link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", link)
	scratch, err := os.MkdirTemp("", "agent-")
	if err != nil {
		t.Fatal(err)
	}
	if resolved, _ := filepath.EvalSymlinks(scratch); resolved == scratch {
		t.Fatalf("scratch %s is not behind the TMPDIR symlink", scratch)
	}

	planPath := filepath.Join(scratch, "plan.json")
	encoded, _ := json.Marshal(plan)
	if err = os.WriteFile(planPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	raw, err := ReadPlan(planPath)
	if err != nil {
		t.Fatalf("plan under a symlinked TMPDIR: %v", err)
	}
	if result, err := Begin(ctx, root, key, raw); err != nil || result.Lifecycle != "active" {
		t.Fatalf("begin: %#v %v", result, err)
	}
	bundle := filepath.Join(scratch, "bundle.json")
	if _, err = Export(ctx, root, key, bundle); err != nil {
		t.Fatalf("export under a symlinked TMPDIR: %v", err)
	}
	if after, err := Import(ctx, restored, key, bundle); err != nil || after.Lifecycle != "active" {
		t.Fatalf("import under a symlinked TMPDIR: %#v %v", after, err)
	}
}

// LCP-V0-2160, V1-3665: exercise the repository itself through a symlinked
// temporary root even when the host's TMPDIR is already canonical.
func TestLCPV02160SymlinkedTempRootSharesEnrollment(t *testing.T) {
	ctx := context.Background()
	realRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "tmp")
	if err = os.Symlink(realRoot, root); err != nil {
		t.Fatal(err)
	}
	root, key, plan := fixtureAt(t, root, []string{"true"})
	if resolved, err := filepath.EvalSymlinks(root); err != nil || resolved == root || resolved != realRoot {
		t.Fatalf("root %q must remain a symlink to %q: %q %v", root, realRoot, resolved, err)
	}
	beginFixture(t, root, key, plan)
	for _, spelling := range []string{root, realRoot} {
		repo, err := open(spelling, key)
		if err != nil {
			t.Fatal(err)
		}
		if repo.auth.Root != realRoot || repo.directory != filepath.Join(realRoot, ".git", "corvint", "local-completion") {
			t.Fatalf("root %q resolved to %q, state %q", spelling, repo.auth.Root, repo.directory)
		}
		result, err := Evaluate(ctx, spelling, key)
		if err != nil || result.Lifecycle != "active" {
			t.Fatalf("evaluate through %q: %#v %v", spelling, result, err)
		}
	}
	if result, err := Verify(ctx, root, key, "test"); err != nil || result.Lifecycle != "active" || len(result.Checks) != 1 || !result.Checks[0].Qualified {
		t.Fatalf("verify through symlinked root: %#v %v", result, err)
	}
	if result, err := Evaluate(ctx, realRoot, key); err != nil || len(result.Checks) != 1 || !result.Checks[0].Qualified || result.Checks[0].TestedCommit != plan.Base {
		t.Fatalf("real root did not retain alias verification: %#v %v", result, err)
	}
	// Cancellation through the real path must be visible through the alias;
	// neither spelling creates a second enrollment.
	if result, err := Cancel(ctx, realRoot, key, "fixture complete"); err != nil || result.Lifecycle != "cancelled" {
		t.Fatalf("cancel through real root: %#v %v", result, err)
	}
	if result, err := Evaluate(ctx, root, key); err != nil || result.Lifecycle != "cancelled" {
		t.Fatalf("evaluate cancelled alias: %#v %v", result, err)
	}
}
