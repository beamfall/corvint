//go:build !darwin && !linux

package testvaliditydoc

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// PRS-V1-022 (V1-2484): on windows and every other unqualified platform each
// receipt open path (file, directory, top directory, stream and absolute read,
// evidence discovery) refuses with ErrPlatformUnsupported. make windows-vet
// type-checks this file; it runs on a native Windows host.
func TestPRSV1022UnqualifiedPlatformRefusesEveryReceiptOpen(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a", "r.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "top.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	_, fileErr := openReceiptFile(dir, "top.json")
	_, dirErr := openReceiptDirectory(dir, "a")
	_, topErr := openTopDirectory("a")
	_, readErr := ReadFile(dir, "top.json")
	_, nestedErr := ReadFile(dir, filepath.Join("a", "r.json"))
	streamErr := StreamFileBounded(dir, "top.json", 16, func(r io.Reader) error { _, err := io.ReadAll(r); return err })
	_, absoluteErr := ReadAbsoluteFileBounded(filepath.Join(root, "a", "r.json"), 16)
	for name, err := range map[string]error{"file": fileErr, "directory": dirErr, "top": topErr, "read": readErr, "nested": nestedErr, "stream": streamErr, "absolute": absoluteErr} {
		if !errors.Is(err, ErrPlatformUnsupported) {
			t.Fatalf("%s: %v, want %v", name, err, ErrPlatformUnsupported)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(EvidenceDirectory)), 0o755); err != nil {
		t.Fatal(err)
	}
	var refusal *DiscoveryError
	if _, err := Discover(root); !errors.As(err, &refusal) || refusal.Code != "invalid-test-evidence-location" || refusal.Message != ErrPlatformUnsupported.Error() {
		t.Fatalf("discover: %v, want the platform refusal as the evidence-location reason", err)
	}
}
