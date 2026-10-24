package journal

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestLGLV0009_SealedBaseCloseFailureIsCleanup covers LGL-V0-009 and
// PSR-V0-057 (proposed, V1-1702): SealedResumeBase reads the sealed base
// through a read lifetime of its own, and a failed close of that lifetime is
// cleanup (UNSUPPORTED_FILESYSTEM, `read lifetime`) that terminates the
// command, never a decline the caller falls back from, even when the base
// itself also declined. Here the manifest the head binds is read and fails
// to decode (a decline), and the lifetime's close fails.
func TestLGLV0009_SealedBaseCloseFailureIsCleanup(t *testing.T) {
	if !haveChangeTime {
		t.Skip("platform reports no change time")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	stateDir, primary := filepath.Join(root, "state"), filepath.Join(root, "worktree")
	const through = 1024
	name := snapshot.SegmentManifestName(through)
	raw := []byte("{}\n")
	if err := os.MkdirAll(filepath.Join(stateDir, filepath.Dir(filepath.FromSlash(name))), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(primary, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, filepath.FromSlash(name)), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	queue, err := wire.ParseQueueID("queue", "queue:test:main")
	if err != nil {
		t.Fatal(err)
	}
	header := Checkpoint{QueueID: queue, PrimaryWorktree: primary, Seq: wire.SizeOf(through)}
	w := &WriterCheckpoint{Checkpoint: header, FullSeq: through, base: &baseTable{seq: through, meta: baseMeta{header: header}}}
	head := &snapshot.Head{QueueID: queue, LastSeq: wire.SizeOf(through + 1), Segments: &snapshot.SegmentBinding{Through: wire.SizeOf(through), ManifestSha256: wire.Sum(raw)}}
	r := Reader{Source: Native{StateDir: stateDir, PrimaryWorktree: primary}, QueueID: queue, PrimaryWorktree: primary}

	// Without an injected failure the undecodable manifest is a decline.
	if base, decline, cleanup := r.SealedResumeBase(w, head); base != nil || decline == nil || cleanup != nil {
		t.Fatalf("undecodable manifest: base %v decline %v cleanup %v", base, decline, cleanup)
	}

	savedFile, savedRoot := closeReadFile, closeReadRoot
	t.Cleanup(func() { closeReadFile, closeReadRoot = savedFile, savedRoot })
	failed := false
	closeReadFile = func(f *os.File) error {
		failed = true
		return errors.Join(savedFile(f), errors.New("injected close failure"))
	}
	closeReadRoot = func(rt *os.Root) error {
		failed = true
		return errors.Join(savedRoot(rt), errors.New("injected close failure"))
	}
	base, decline, cleanup := r.SealedResumeBase(w, head)
	if !failed {
		t.Fatal("the sealed base read closed nothing")
	}
	if base != nil || decline != nil || wire.CodeOf(cleanup) != wire.CodeUnsupportedFilesystem {
		t.Fatalf("sealed base over a failed close: base %v decline %v cleanup %v", base, decline, cleanup)
	}
}
