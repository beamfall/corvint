package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func benchReceiptDir(b *testing.B, n int) *nativeRead {
	b.Helper()
	dir, err := filepath.EvalSymlinks(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	receipts := filepath.Join(dir, "receipts")
	if err := os.MkdirAll(receipts, 0700); err != nil {
		b.Fatal(err)
	}
	for i := 1; i <= n; i++ {
		name, _ := snapshot.ReceiptName(uint64(i))
		if err := os.WriteFile(filepath.Join(receipts, name), nil, 0600); err != nil {
			b.Fatal(err)
		}
	}
	src := newNativeRead(Native{StateDir: dir, PrimaryWorktree: dir})
	b.Cleanup(func() {
		if err := src.close(); err != nil {
			b.Fatal(err)
		}
	})
	return src
}

// BenchmarkScanReceiptNames measures one writer-audit receipts/ scan,
// listing included, at two history sizes (V1-1410).
func BenchmarkScanReceiptNames(b *testing.B) {
	for _, n := range []int{3000, 13000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			src := benchReceiptDir(b, n)
			r := Reader{Source: src}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				o := &observation{files: map[string]os.FileInfo{}}
				remaining := wire.MaxArchiveScanEntries
				if err := r.scanReceiptNames(o, &remaining); err != nil || len(o.receipts) != n {
					b.Fatal(err, len(o.receipts))
				}
			}
		})
	}
}

// BenchmarkListReceiptNames measures the bare receipts/ listing the scan
// pays before any per-name work, for subtraction.
func BenchmarkListReceiptNames(b *testing.B) {
	for _, n := range []int{3000, 13000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			src := benchReceiptDir(b, n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, names, err := src.Names("receipts", wire.MaxArchiveScanEntries); err != nil || len(names) != n {
					b.Fatal(err, len(names))
				}
			}
		})
	}
}
