package workflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestCEMV1933PrepareBindsModeChange is the V1-3404 regression (CEM-V1-933):
// prepare over a commit that makes a script executable and edits it succeeds,
// maps the textual hunk, and the native stable verifier accepts the map.
func TestCEMV1933PrepareBindsModeChange(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-q", "-b", "main")
	gitCmd(t, root, "config", "core.fileMode", "true")
	writeFile(t, root, "run.sh", "echo a\nkeep\n")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-qm", "base")
	base := gitCmd(t, root, "rev-parse", "HEAD")
	writeFile(t, root, "run.sh", "echo b\nkeep\n")
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "update-index", "--chmod=+x", "run.sh")
	gitCmd(t, root, "commit", "-qm", "target")
	target := gitCmd(t, root, "rev-parse", "HEAD")
	if mode := gitCmd(t, root, "ls-tree", "--format=%(objectmode)", target, "run.sh"); mode != "100755" {
		t.Fatalf("fixture target mode %q, want 100755", mode)
	}

	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatalf("Prepare over a mode change beside a text edit: %v", err)
	}
	_, document, err := openSession(t, root).readProducerMap(wire.ExcludedCEMPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Hunks) != 1 || document.Hunks[0].Path != "run.sh" {
		t.Fatalf("mapped hunks %+v, want only the run.sh hunk", document.Hunks)
	}
	raw, err := os.ReadFile(filepath.Join(root, wire.ExcludedCEMPath))
	if err != nil {
		t.Fatal(err)
	}
	verdict, _ := verify.Stable(ctx(), raw, verify.StableOptions{Repository: root, ExpectedBase: base, Target: target, ArtifactRoot: root})
	if verdict.Stage != "complete" || verdict.Outcome != "ACCEPT" || verdict.Code != nil {
		encoded, _ := json.Marshal(verdict)
		t.Fatalf("stable verify did not accept the map over a mode change: %s", encoded)
	}
}
