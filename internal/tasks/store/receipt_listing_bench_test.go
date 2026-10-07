//go:build darwin || linux

package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
)

// BenchmarkCALV0138_ListsPrefix measures the carried fold's listing binding
// at 13,000 receipts against a names-only listing (the c8315a93 cost floor)
// and a ReadDir through the Root-opened directory, which fstatats every
// entry (20cf49c4).
func BenchmarkCALV0138_ListsPrefix(b *testing.B) {
	const n = 13000
	dir := b.TempDir()
	for seq := uint64(1); seq <= n; seq++ {
		name, err := snapshot.ReceiptName(seq)
		if err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}\n"), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	r := &receiptFiles{dir: dir}
	if _, err := r.read(1); err != nil {
		b.Fatal(err)
	}
	defer r.close(false)
	b.Run("names", func(b *testing.B) {
		for b.Loop() {
			d, err := r.root.Open(".")
			if err != nil {
				b.Fatal(err)
			}
			names, err := d.Readdirnames(-1)
			d.Close()
			if err != nil || len(names) != n {
				b.Fatal(err, len(names))
			}
		}
	})
	b.Run("rootReadDir", func(b *testing.B) {
		for b.Loop() {
			d, err := r.root.Open(".")
			if err != nil {
				b.Fatal(err)
			}
			entries, err := d.ReadDir(-1)
			d.Close()
			if err != nil || len(entries) != n || !entries[0].Type().IsRegular() {
				b.Fatal(err, len(entries))
			}
		}
	})
	b.Run("listsPrefix", func(b *testing.B) {
		for b.Loop() {
			if !r.listsPrefix(n) {
				b.Fatal("listing refused")
			}
		}
	})
}
