package store

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-135: the whole-history binding fold reads every receipt beneath one
// pinned receipts/ descriptor with the same bytes, refusals and absence as the
// per-path reader, and refuses a receipts/ directory replaced during the fold.
func TestCALV0135_ReceiptFoldPinnedReader(t *testing.T) {
	t.Parallel()
	repo := historyStore(t, 80)
	files := newReceiptFiles(repo)
	for seq := uint64(1); seq <= 80; seq++ {
		pinned, err := files.read(seq)
		if err != nil {
			t.Fatalf("seq %d: %v", seq, err)
		}
		named, err := readReceiptBytes(repo, seq)
		if err != nil || !bytes.Equal(pinned, named) {
			t.Fatalf("seq %d: pinned bytes differ from the per-path reader: %v", seq, err)
		}
	}
	if _, err := files.read(81); !os.IsNotExist(err) {
		t.Fatalf("absent receipt: %v", err)
	}
	if err := files.close(true); err != nil {
		t.Fatal(err)
	}
	if err := FoldReceiptBindings(repo, 80, nil); err != nil {
		t.Fatalf("fold: %v", err)
	}

	dir := filepath.Join(repo.StateDir, "receipts")
	name, err := snapshot.ReceiptName(7)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), name)
	historyWrite(t, outside, raw)
	if err := os.Remove(filepath.Join(dir, name)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, name)); err != nil {
		t.Fatal(err)
	}
	if err := FoldReceiptBindings(repo, 80, nil); wire.CodeOf(err) != wire.CodeUnsupportedFilesystem {
		t.Fatalf("symlinked receipt: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, name)); err != nil {
		t.Fatal(err)
	}
	historyWrite(t, filepath.Join(dir, name), raw)

	files = newReceiptFiles(repo)
	if _, err := files.read(1); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(dir, dir+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := files.close(true); wire.CodeOf(err) != wire.CodeSnapshotMoved {
		t.Fatalf("replaced receipts directory: %v", err)
	}
}

// BenchmarkCALV0135_FoldReceiptBindings guards the per-receipt cost of the
// `receipt audit` binding fold (one openat per receipt, not a traversal).
func BenchmarkCALV0135_FoldReceiptBindings(b *testing.B) {
	repo := historyStore(b, 2000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := FoldReceiptBindings(repo, 2000, nil); err != nil {
			b.Fatal(err)
		}
	}
}
