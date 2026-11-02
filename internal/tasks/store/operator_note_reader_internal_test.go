package store

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestONV0786_PinnedEvidenceReadsMatchPerPathReads pins ON-V0-786: the
// pinned evidence reader returns the bytes and the error of a per-path
// intent.ReadFile for a present, missing, symlinked, oversized and directory
// event (a missing one by class, since OperatorNoteHistory words its own
// MISSING_EVIDENCE refusal), and an evidence directory it cannot pin reads
// per path.
func TestONV0786_PinnedEvidenceReadsMatchPerPathReads(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("safeopen pins directories on linux and darwin only")
	}
	evidence := filepath.Join(t.TempDir(), "evidence")
	if err := os.Mkdir(evidence, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name string, raw []byte) {
		if err := os.WriteFile(filepath.Join(evidence, name), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("present", []byte("event\n"))
	write("oversized", bytes.Repeat([]byte("x"), ticket.MaxOperatorNoteBytes+1))
	if err := os.Symlink(filepath.Join(evidence, "present"), filepath.Join(evidence, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(evidence, "directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	same := func(dir, name string, read func(wire.Digest) ([]byte, error)) {
		t.Helper()
		got, gotErr := read(wire.Digest(name))
		want, wantErr := intent.ReadFile(filepath.Join(dir, name), ticket.MaxOperatorNoteBytes)
		if !bytes.Equal(got, want) || errors.Is(gotErr, fs.ErrNotExist) != errors.Is(wantErr, fs.ErrNotExist) ||
			wire.CodeOf(gotErr) != wire.CodeOf(wantErr) || (gotErr == nil) != (wantErr == nil) ||
			(gotErr != nil && !errors.Is(wantErr, fs.ErrNotExist) && gotErr.Error() != wantErr.Error()) {
			t.Fatalf("%s: pinned %q, %v; per path %q, %v", name, got, gotErr, want, wantErr)
		}
	}
	read, done := operatorNoteEvidenceReader(evidence)
	for _, name := range []string{"present", "missing", "link", "oversized", "directory"} {
		same(evidence, name, read)
	}
	done()
	absent := filepath.Join(t.TempDir(), "evidence")
	read, done = operatorNoteEvidenceReader(absent)
	defer done()
	same(absent, "present", read)
}
