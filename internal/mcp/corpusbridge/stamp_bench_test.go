// SPDX-License-Identifier: Apache-2.0

package corpusbridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkDCRV1847CheckoutStamp captures the stamp of the DCP-V1-317 fixture's three checkouts
// (17,001 files), as one warm corpus MCP call does.
func BenchmarkDCRV1847CheckoutStamp(b *testing.B) {
	saved := stampRacyWindow
	b.Cleanup(func() { stampRacyWindow = saved })
	stampRacyWindow = 0
	root, extra := multiRepositoryFixture(b, [3]int{4000, 12000, 1000})
	dirs := append([]string{root}, extra...)
	if checkoutStamp(context.Background(), dirs) == "" {
		b.Fatal("fixture has no stamp")
	}
	b.ResetTimer()
	for b.Loop() {
		if checkoutStamp(context.Background(), dirs) == "" {
			b.Fatal("stamp lost")
		}
	}
}

// TestDCRV1847ChunkedStatMatchesSerialDigest: describing a large directory's entries in chunks on
// other goroutines gives the digest of the serial walk for every chunk size.
func TestDCRV1847ChunkedStatMatchesSerialDigest(t *testing.T) {
	supportedStampPlatform(t)
	shortenStampWindow(t)
	dir := stampRepository(t)
	for _, sub := range []string{"docs/flat", "docs/flat/nested"} {
		if err := os.MkdirAll(filepath.Join(dir, filepath.FromSlash(sub)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for n := range 2500 {
		if err := os.WriteFile(filepath.Join(dir, "docs", "flat", fmt.Sprintf("f%04d.md", n)), []byte("x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "flat", "nested", "deep.md"), []byte("y\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	saved := stampStatChunk
	t.Cleanup(func() { stampStatChunk = saved })
	stampStatChunk = 1 << 30
	serial := checkoutStamp(context.Background(), []string{dir})
	if serial == "" {
		t.Fatal("no serial stamp")
	}
	for _, chunk := range []int{1, 7, 1024, 2499, 2500} {
		stampStatChunk = chunk
		for range 3 {
			if got := checkoutStamp(context.Background(), []string{dir}); got != serial {
				t.Fatalf("chunk %d: digest %q, serial %q", chunk, got, serial)
			}
		}
	}
	// A change in the nested directory reached only after the chunked parent still changes it.
	if err := os.WriteFile(filepath.Join(dir, "docs", "flat", "nested", "deep.md"), []byte("yy\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stampStatChunk = 7
	if got := checkoutStamp(context.Background(), []string{dir}); got == serial || got == "" {
		t.Fatalf("nested edit: digest %q, before %q", got, serial)
	}
}
