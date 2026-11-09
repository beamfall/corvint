//go:build darwin || linux

package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// swapParentIntoGitDir installs the afterExternalCheck seam: once the
// external directory has passed the location check, its parent is renamed
// aside and replaced by a symlink into the repository's Git directory, where
// a directory of the same name waits. A pathname re-resolution would follow
// it; a held handle of the checked directory cannot.
func swapParentIntoGitDir(t *testing.T, root, parent string) string {
	t.Helper()
	inside := filepath.Join(root, ".git", filepath.Base(parent), "bundle")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	afterExternalCheck = func(string) {
		if err := os.Rename(parent, parent+".aside"); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Dir(inside), parent); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { afterExternalCheck = nil })
	return inside
}

// externalParent returns a fresh resolved directory outside the repository
// holding parent/bundle, and the parent path.
func externalParent(t *testing.T) (string, string) {
	t.Helper()
	scratch, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(scratch, "parent")
	bundle := filepath.Join(parent, "bundle")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatal(err)
	}
	return parent, bundle
}

// CEM-V1-244: an absolute --map is read through a held handle of the
// directory that passed the location check. A parent swapped for a symlink
// into the Git directory between the check and the open is a typed
// map-unavailable refusal, never a read of the file it now points at.
func TestCEMV1244ExternalMapParentSwapRefused(t *testing.T) {
	root, base, target := makeRepo(t)
	parent, bundle := externalParent(t)
	if err := os.WriteFile(filepath.Join(bundle, "cem.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	inside := swapParentIntoGitDir(t, root, parent)
	if err := os.WriteFile(filepath.Join(inside, "cem.json"), []byte("redirected"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := openSession(t, root).Read(ctx(), "status", ReadOptions{
		MapPath: filepath.Join(bundle, "cem.json"), ExpectedBase: base, Target: target, ArtifactRoot: root, ArtifactsGiven: true,
	})
	if cemcode.CodeOf(err) != cemcode.MapUnavailable || !strings.Contains(err.Error(), "changed after it was checked") {
		t.Fatalf("swapped-parent map read = %v, want a map-unavailable refusal of the changed directory", err)
	}
}

// CEM-V1-244: an absolute --output is published through a held handle of
// the checked directory. A parent swapped into the Git directory between the
// check and the write is a typed publish-failed refusal and writes nothing
// there.
func TestCEMV1244ExternalReportParentSwapRefused(t *testing.T) {
	root, base, target := makeRepo(t)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	parent, bundle := externalParent(t)
	inside := swapParentIntoGitDir(t, root, parent)
	_, err := openSession(t, root).Read(ctx(), "report", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: target, ArtifactRoot: root, ArtifactsGiven: true,
		Output: filepath.Join(bundle, "report.md"),
	})
	if cemcode.CodeOf(err) != cemcode.PublishFailed || !strings.Contains(err.Error(), "changed after it was checked") {
		t.Fatalf("swapped-parent report = %v, want a publish-failed refusal of the changed directory", err)
	}
	if entries, _ := os.ReadDir(inside); len(entries) != 0 {
		t.Fatalf("report wrote into the Git directory: %v", entries)
	}
}

// CEM-V1-244: without a swap the held handle publishes an absolute report
// and reads the map back, so the hold changes only the race.
func TestCEMV1244ExternalReportPublishes(t *testing.T) {
	root, base, target := makeRepo(t)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	_, bundle := externalParent(t)
	output := filepath.Join(bundle, "report.md")
	result, err := openSession(t, root).Read(ctx(), "report", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: target, ArtifactRoot: root, ArtifactsGiven: true, Output: output,
	})
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(output)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || result["report"] != output {
		t.Fatalf("report %v at %s: info %v err %v", result["report"], output, info, err)
	}
	if entries, _ := os.ReadDir(bundle); len(entries) != 1 {
		t.Fatalf("publication left residue: %v", entries)
	}
}
