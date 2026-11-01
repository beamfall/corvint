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
// The repository root stays resolved, since local state refuses a symlinked
// root by design.
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
