package publish

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

func openRoot(t *testing.T) *Root {
	t.Helper()
	root, err := OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func noResidue(t *testing.T, root *Root) {
	t.Helper()
	err := filepath.WalkDir(root.Path(), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.Contains(entry.Name(), ".tmp-") || strings.Contains(entry.Name(), ".bak-") {
			t.Errorf("temporary residue %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertPrivateFileMode(t *testing.T, mode os.FileMode) {
	t.Helper()
	t.Run("private file mode", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows does not expose POSIX file-mode semantics")
		}
		if mode.Perm() != 0o600 {
			t.Fatalf("permissions %v, want 0600", mode.Perm())
		}
	})
}

func TestPublishAtomicPrivate(t *testing.T) {
	root := openRoot(t)
	if err := Publish(Output{Root: root, Relative: "a/b/map.json", Data: []byte("{}")}); err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(root.Path(), "a", "b", "map.json")
	info, err := os.Lstat(full)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("published output: %v", err)
	}
	assertPrivateFileMode(t, info.Mode())
	noResidue(t, root)
}

func TestPublishSecureAtomicPrivate(t *testing.T) {
	root := openRoot(t)
	if err := PublishSecure(context.Background(), Output{Root: root, Relative: "a/b/report.md", Data: []byte("report\n")}, true); err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(root.Path(), "a", "b", "report.md")
	data, err := os.ReadFile(full)
	if err != nil || string(data) != "report\n" {
		t.Fatalf("published output=%q err=%v", data, err)
	}
	info, err := os.Lstat(full)
	if err != nil {
		t.Fatal(err)
	}
	assertPrivateFileMode(t, info.Mode())
	noResidue(t, root)
}

func TestPublishSecureRequiresExistingParentWhenCreationDisabled(t *testing.T) {
	root := openRoot(t)
	err := PublishSecure(context.Background(), Output{Root: root, Relative: "missing/report.md", Data: []byte("report\n")}, false)
	if cemcode.CodeOf(err) != cemcode.PublishFailed {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root.Path(), "missing")); !os.IsNotExist(err) {
		t.Fatal("secure publication created a disabled parent")
	}
}

func TestPublishSecureRefusesSymlinkComponents(t *testing.T) {
	root := openRoot(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root.Path(), "linked")); err != nil {
		t.Fatal(err)
	}
	err := PublishSecure(context.Background(), Output{Root: root, Relative: "linked/report.md", Data: []byte("attack")}, false)
	if cemcode.CodeOf(err) != cemcode.PublishFailed {
		t.Fatalf("got %v", err)
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("outside entries=%v err=%v", entries, err)
	}
}

func TestPublishSecureRefusesReplacedRoot(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "root")
	outside := filepath.Join(base, "outside")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, filepath.Join(base, "moved")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}
	err = PublishSecure(context.Background(), Output{Root: root, Relative: "report.md", Data: []byte("attack")}, false)
	if cemcode.CodeOf(err) != cemcode.PublishFailed {
		t.Fatalf("got %v", err)
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("outside entries=%v err=%v", entries, err)
	}
}

func TestPublishSecureHonorsPreCancelledContext(t *testing.T) {
	root := openRoot(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := PublishSecure(ctx, Output{Root: root, Relative: "a/b/report.md", Data: []byte("report")}, true)
	if cemcode.CodeOf(err) != cemcode.PublishFailed {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root.Path(), "a")); !os.IsNotExist(err) {
		t.Fatal("cancelled secure publication changed the directory tree")
	}
}

func TestOutputPathContainment(t *testing.T) {
	root := openRoot(t)
	for _, relative := range []string{"../escape", "/abs", "a/../../b", "a\\b"} {
		err := Publish(Output{Root: root, Relative: relative, Data: []byte("x")})
		if err == nil {
			t.Errorf("%q accepted", relative)
		}
	}
}

func TestSymlinkFinalRefused(t *testing.T) {
	root := openRoot(t)
	outside := filepath.Join(t.TempDir(), "victim")
	if err := os.WriteFile(outside, []byte("safe"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root.Path(), "link.json")); err != nil {
		t.Fatal(err)
	}
	err := Publish(Output{Root: root, Relative: "link.json", Data: []byte("attack")})
	if cemcode.CodeOf(err) != cemcode.PublishFailed {
		t.Fatalf("got %v", err)
	}
	data, _ := os.ReadFile(outside)
	if string(data) != "safe" {
		t.Fatal("symlink target mutated")
	}
}

func TestSymlinkAncestorRefused(t *testing.T) {
	root := openRoot(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root.Path(), "dir")); err != nil {
		t.Fatal(err)
	}
	err := Publish(Output{Root: root, Relative: "dir/map.json", Data: []byte("x")})
	if cemcode.CodeOf(err) != cemcode.PublishFailed {
		t.Fatalf("got %v", err)
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatal("escaped through symlinked ancestor")
	}
}

func TestPairRollbackRestoresPrior(t *testing.T) {
	root := openRoot(t)
	if err := Publish(Output{Root: root, Relative: "map.json", Data: []byte("prior")}); err != nil {
		t.Fatal(err)
	}
	// Force the second output to fail: its final path is a symlink.
	if err := os.Symlink(filepath.Join(t.TempDir(), "x"), filepath.Join(root.Path(), "cache.patch")); err != nil {
		t.Fatal(err)
	}
	err := PublishPair(
		Output{Root: root, Relative: "map.json", Data: []byte("new")},
		Output{Root: root, Relative: "cache.patch", Data: []byte("patch")})
	if cemcode.CodeOf(err) != cemcode.PublishFailed {
		t.Fatalf("got %v", err)
	}
	data, readErr := os.ReadFile(filepath.Join(root.Path(), "map.json"))
	if readErr != nil || string(data) != "prior" {
		t.Fatalf("first output not rolled back: %q %v", data, readErr)
	}
	noResidue(t, root)
}

func TestPairRollbackRemovesFreshFirst(t *testing.T) {
	root := openRoot(t)
	if err := os.Symlink(filepath.Join(t.TempDir(), "x"), filepath.Join(root.Path(), "cache.patch")); err != nil {
		t.Fatal(err)
	}
	err := PublishPair(
		Output{Root: root, Relative: "fresh.json", Data: []byte("new")},
		Output{Root: root, Relative: "cache.patch", Data: []byte("patch")})
	if err == nil {
		t.Fatal("accepted")
	}
	if _, statErr := os.Lstat(filepath.Join(root.Path(), "fresh.json")); !os.IsNotExist(statErr) {
		t.Fatal("fresh first output left behind")
	}
	noResidue(t, root)
}

func TestPairSuccess(t *testing.T) {
	root := openRoot(t)
	err := PublishPair(
		Output{Root: root, Relative: ".corvint/change.cem.json", Data: []byte("map")},
		Output{Root: root, Relative: "corvint/change.patch", Data: []byte("patch")})
	if err != nil {
		t.Fatal(err)
	}
	for relative, want := range map[string]string{".corvint/change.cem.json": "map", "corvint/change.patch": "patch"} {
		data, readErr := os.ReadFile(filepath.Join(root.Path(), filepath.FromSlash(relative)))
		if readErr != nil || string(data) != want {
			t.Fatalf("%s: %q %v", relative, data, readErr)
		}
	}
	noResidue(t, root)
}

// TestPairSuccessSyncsFinalsBeforeRemovingBackup is CEM-PILOT-013: the
// prior-file backup must remain durable until both new final directory entries
// are synced, and its later removal must itself be synced.
func TestPairSuccessSyncsFinalsBeforeRemovingBackup(t *testing.T) {
	root := openRoot(t)
	if err := Publish(Output{Root: root, Relative: "map.json", Data: []byte("prior")}); err != nil {
		t.Fatal(err)
	}
	previousSync := syncOutputDirectory
	var backupPresentAtSync []bool
	syncOutputDirectory = func(path string) error {
		backups, err := filepath.Glob(filepath.Join(root.Path(), "map.json.bak-*"))
		if err != nil {
			t.Fatal(err)
		}
		backupPresentAtSync = append(backupPresentAtSync, len(backups) == 1)
		return syncDirectory(path)
	}
	t.Cleanup(func() { syncOutputDirectory = previousSync })
	if err := PublishPair(
		Output{Root: root, Relative: "map.json", Data: []byte("new")},
		Output{Root: root, Relative: "cache.patch", Data: []byte("patch")}); err != nil {
		t.Fatal(err)
	}
	want := []bool{true, true, true, false}
	if len(backupPresentAtSync) != len(want) {
		t.Fatalf("sync observations = %v, want %v", backupPresentAtSync, want)
	}
	for i := range want {
		if backupPresentAtSync[i] != want[i] {
			t.Fatalf("sync observations = %v, want %v", backupPresentAtSync, want)
		}
	}
	noResidue(t, root)
}

func TestPairFinalSyncFailureRetainsBackup(t *testing.T) {
	root := openRoot(t)
	if err := Publish(Output{Root: root, Relative: "map.json", Data: []byte("prior")}); err != nil {
		t.Fatal(err)
	}
	previousSync := syncOutputDirectory
	syncCount := 0
	syncOutputDirectory = func(path string) error {
		syncCount++
		if syncCount == 2 {
			return os.ErrPermission
		}
		return syncDirectory(path)
	}
	t.Cleanup(func() { syncOutputDirectory = previousSync })
	pairErr := PublishPair(
		Output{Root: root, Relative: "map.json", Data: []byte("new")},
		Output{Root: root, Relative: "cache.patch", Data: []byte("patch")})
	if cemcode.CodeOf(pairErr) != cemcode.PublishFailed {
		t.Fatalf("got %v", pairErr)
	}
	backups, err := filepath.Glob(filepath.Join(root.Path(), "map.json.bak-*"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("backup evidence = %v, %v; want one retained backup", backups, err)
	}
}

func TestReadBoundedRefusesSymlinkAndOversize(t *testing.T) {
	root := openRoot(t)
	if err := Publish(Output{Root: root, Relative: "ok.json", Data: []byte("{}")}); err != nil {
		t.Fatal(err)
	}
	data, err := root.ReadBounded("ok.json", 10, cemcode.MapUnavailable)
	if err != nil || string(data) != "{}" {
		t.Fatalf("%q %v", data, err)
	}
	if _, err := root.ReadBounded("ok.json", 1, cemcode.MapUnavailable); cemcode.CodeOf(err) != cemcode.MapUnavailable {
		t.Fatalf("oversize: %v", err)
	}
	if err := os.Symlink(filepath.Join(root.Path(), "ok.json"), filepath.Join(root.Path(), "alias.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := root.ReadBounded("alias.json", 10, cemcode.MapUnavailable); cemcode.CodeOf(err) != cemcode.MapUnavailable {
		t.Fatalf("symlink input: %v", err)
	}
}

// failSecondRename makes every rename succeed except the one landing at the
// given final path, injecting the failure only after both stages and the
// first final rename completed.
func failSecondRename(t *testing.T, finalSecond string) {
	t.Helper()
	previous := renameOutput
	renameOutput = func(oldPath, newPath string) error {
		if newPath == finalSecond {
			return os.ErrPermission
		}
		return os.Rename(oldPath, newPath)
	}
	t.Cleanup(func() { renameOutput = previous })
}

// TestPairSecondRenameFailureRestoresExactPrior injects a failure into the
// SECOND final rename itself — after both outputs staged and the first
// landed — and requires the first output's exact prior inode back with no
// residue. The pre-seam best-effort rollback fails this test.
func TestPairSecondRenameFailureRestoresExactPrior(t *testing.T) {
	root := openRoot(t)
	if err := Publish(Output{Root: root, Relative: "map.json", Data: []byte("prior")}); err != nil {
		t.Fatal(err)
	}
	prior, err := os.Stat(filepath.Join(root.Path(), "map.json"))
	if err != nil {
		t.Fatal(err)
	}
	failSecondRename(t, filepath.Join(root.Path(), "cache.patch"))
	pairErr := PublishPair(
		Output{Root: root, Relative: "map.json", Data: []byte("new")},
		Output{Root: root, Relative: "cache.patch", Data: []byte("patch")})
	if cemcode.CodeOf(pairErr) != cemcode.PublishFailed {
		t.Fatalf("got %v", pairErr)
	}
	data, readErr := os.ReadFile(filepath.Join(root.Path(), "map.json"))
	if readErr != nil || string(data) != "prior" {
		t.Fatalf("first output not restored: %q %v", data, readErr)
	}
	restored, err := os.Stat(filepath.Join(root.Path(), "map.json"))
	if err != nil || !os.SameFile(prior, restored) {
		t.Fatalf("restoration did not return the exact prior file: %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(root.Path(), "cache.patch")); !os.IsNotExist(statErr) {
		t.Fatal("second output landed despite the injected failure")
	}
	noResidue(t, root)
}

// TestPairRollbackSyncsRestoredDirectory is CEM-PILOT-013: restoring the
// prior inode after the second publication fails is not durable until the
// restored directory entry is synced.
func TestPairRollbackSyncsRestoredDirectory(t *testing.T) {
	root := openRoot(t)
	if err := Publish(Output{Root: root, Relative: "map.json", Data: []byte("prior")}); err != nil {
		t.Fatal(err)
	}
	finalSecond := filepath.Join(root.Path(), "cache.patch")
	previousRename := renameOutput
	previousSync := syncOutputDirectory
	secondFailed := false
	restoredSynced := false
	renameOutput = func(oldPath, newPath string) error {
		if newPath == finalSecond {
			secondFailed = true
			return os.ErrPermission
		}
		return os.Rename(oldPath, newPath)
	}
	syncOutputDirectory = func(path string) error {
		if secondFailed && path == root.Path() {
			restoredSynced = true
		}
		return syncDirectory(path)
	}
	t.Cleanup(func() {
		renameOutput = previousRename
		syncOutputDirectory = previousSync
	})
	pairErr := PublishPair(
		Output{Root: root, Relative: "map.json", Data: []byte("new")},
		Output{Root: root, Relative: "cache.patch", Data: []byte("patch")})
	if cemcode.CodeOf(pairErr) != cemcode.PublishFailed {
		t.Fatalf("got %v", pairErr)
	}
	if !restoredSynced {
		t.Fatal("restored directory entry was not synced")
	}
}

// TestPairSecondRenameFailureRemovesFreshFirst covers the same injection when
// no prior first output existed: the fresh first file must be removed.
func TestPairSecondRenameFailureRemovesFreshFirst(t *testing.T) {
	root := openRoot(t)
	failSecondRename(t, filepath.Join(root.Path(), "cache.patch"))
	pairErr := PublishPair(
		Output{Root: root, Relative: "fresh.json", Data: []byte("new")},
		Output{Root: root, Relative: "cache.patch", Data: []byte("patch")})
	if cemcode.CodeOf(pairErr) != cemcode.PublishFailed {
		t.Fatalf("got %v", pairErr)
	}
	if _, statErr := os.Lstat(filepath.Join(root.Path(), "fresh.json")); !os.IsNotExist(statErr) {
		t.Fatal("fresh first output left behind")
	}
	noResidue(t, root)
}

// TestPairNonRegularSecondFinalRefusedAtStage keeps the stage-time guard: a
// directory at a final path is refused before any rename.
func TestPairNonRegularSecondFinalRefusedAtStage(t *testing.T) {
	root := openRoot(t)
	if err := os.Mkdir(filepath.Join(root.Path(), "cache.patch"), 0o755); err != nil {
		t.Fatal(err)
	}
	pairErr := PublishPair(
		Output{Root: root, Relative: "fresh.json", Data: []byte("new")},
		Output{Root: root, Relative: "cache.patch", Data: []byte("patch")})
	if cemcode.CodeOf(pairErr) != cemcode.PublishFailed {
		t.Fatalf("got %v", pairErr)
	}
	if _, statErr := os.Lstat(filepath.Join(root.Path(), "fresh.json")); !os.IsNotExist(statErr) {
		t.Fatal("fresh first output left behind")
	}
	if err := os.Remove(filepath.Join(root.Path(), "cache.patch")); err != nil {
		t.Fatal(err)
	}
	noResidue(t, root)
}

// TestPairSameTargetRefused: a pair must never publish both outputs onto one
// file — the second rename would silently clobber the first.
func TestPairSameTargetRefused(t *testing.T) {
	root := openRoot(t)
	err := PublishPair(
		Output{Root: root, Relative: "same.json", Data: []byte("map")},
		Output{Root: root, Relative: "same.json", Data: []byte("patch")})
	if cemcode.CodeOf(err) != cemcode.PublishFailed {
		t.Fatalf("aliased pair accepted: %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(root.Path(), "same.json")); !os.IsNotExist(statErr) {
		t.Fatal("aliased pair left an output behind")
	}
	noResidue(t, root)
}

// TestReadBoundedFileDescriptorIdentity freezes the bounded-read contract:
// an exact-bound regular file reads fully, one byte over rejects, and a
// symlink at the path is refused before any read.
func TestReadBoundedFileDescriptorIdentity(t *testing.T) {
	dir := t.TempDir()
	exact := filepath.Join(dir, "exact")
	if err := os.WriteFile(exact, []byte("12345678"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := ReadBoundedFile(exact, 8, cemcode.PatchUnavailable)
	if err != nil || string(data) != "12345678" {
		t.Fatalf("exact bound: %q %v", data, err)
	}
	if _, err := ReadBoundedFile(exact, 7, cemcode.PatchUnavailable); cemcode.CodeOf(err) != cemcode.PatchUnavailable {
		t.Fatalf("over bound accepted: %v", err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(exact, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadBoundedFile(link, 8, cemcode.PatchUnavailable); cemcode.CodeOf(err) != cemcode.PatchUnavailable {
		t.Fatalf("symlink accepted: %v", err)
	}
}

// caseInsensitiveVolume probes whether the directory folds case.
func caseInsensitiveVolume(t *testing.T, dir string) bool {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "probe-case"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := os.Lstat(filepath.Join(dir, "PROBE-CASE"))
	return err == nil
}

// TestPairPhysicalAliasRefused: on a case-insensitive volume two spellings of
// one path are one physical file; the pair must refuse rather than let the
// second output overwrite the first, and must restore the prior state.
func TestPairPhysicalAliasRefused(t *testing.T) {
	root := openRoot(t)
	if !caseInsensitiveVolume(t, root.Path()) {
		t.Skip("volume is case-sensitive")
	}
	if err := Publish(Output{Root: root, Relative: "map.json", Data: []byte("prior")}); err != nil {
		t.Fatal(err)
	}
	pairErr := PublishPair(
		Output{Root: root, Relative: "map.json", Data: []byte("new")},
		Output{Root: root, Relative: "MAP.json", Data: []byte("patch")})
	if cemcode.CodeOf(pairErr) != cemcode.PublishFailed {
		t.Fatalf("aliased pair accepted: %v", pairErr)
	}
	data, readErr := os.ReadFile(filepath.Join(root.Path(), "map.json"))
	if readErr != nil || string(data) != "prior" {
		t.Fatalf("prior state not restored: %q %v", data, readErr)
	}
	noResidue(t, root)
}

// TestReadBoundedFileRejectsSwappedPath injects the race the descriptor
// identity check exists for: between the pre-open metadata capture and the
// open, the path is swapped for a symlink to a different file and then
// swapped back. The read must be refused with no bytes returned — an
// implementation that re-checks path metadata only after reading returns the
// other file's bytes here.
func TestReadBoundedFileRejectsSwappedPath(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(dir, "victim")
	other := filepath.Join(dir, "other")
	if err := os.WriteFile(victim, []byte("expected"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte("secret!!"), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := openInputFile
	openInputFile = func(path string) (*os.File, error) {
		if path != victim {
			return os.Open(path)
		}
		if err := os.Rename(victim, victim+".aside"); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(other, victim); err != nil {
			t.Fatal(err)
		}
		file, err := os.Open(path)
		if removeErr := os.Remove(victim); removeErr != nil {
			t.Fatal(removeErr)
		}
		if renameErr := os.Rename(victim+".aside", victim); renameErr != nil {
			t.Fatal(renameErr)
		}
		return file, err
	}
	t.Cleanup(func() { openInputFile = previous })
	data, err := ReadBoundedFile(victim, 64, cemcode.PatchUnavailable)
	if cemcode.CodeOf(err) != cemcode.PatchUnavailable || data != nil {
		t.Fatalf("swapped path read: %q %v", data, err)
	}
}

// TestReadBoundedFileRejectsGrowthAfterSizeCheck grows the file between the
// size check and the open; the bound-plus-one limited read must refuse it.
func TestReadBoundedFileRejectsGrowthAfterSizeCheck(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "grows")
	if err := os.WriteFile(target, []byte("12345678"), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := openInputFile
	openInputFile = func(path string) (*os.File, error) {
		if path == target {
			file, err := os.OpenFile(target, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := file.WriteString("overflow"); err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
		}
		return os.Open(path)
	}
	t.Cleanup(func() { openInputFile = previous })
	data, err := ReadBoundedFile(target, 8, cemcode.PatchUnavailable)
	if cemcode.CodeOf(err) != cemcode.PatchUnavailable || data != nil {
		t.Fatalf("grown file read: %q %v", data, err)
	}
}
