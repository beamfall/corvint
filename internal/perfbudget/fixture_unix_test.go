//go:build unix

package perfbudget

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// PRB-V0-023 (review of GitHub #1602): a directory whose emptiness cannot be proved (write and
// search permission, no read permission) is refused before anything is written.
func TestPRBV0023UninspectableDirectoryRefused(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a directory without read permission")
	}
	dir := filepath.Join(t.TempDir(), "fixture")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "keep"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o300); err != nil {
		t.Fatal(err)
	}
	_, err := Generate(context.Background(), dir, Smoke, LibraryBuilder)
	if chmodErr := os.Chmod(dir, 0o700); chmodErr != nil {
		t.Fatal(chmodErr)
	}
	if err == nil || !strings.Contains(err.Error(), "cannot prove") {
		t.Fatalf("Generate into an unreadable directory: %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 || entries[0].Name() != "keep" {
		t.Fatalf("refusal wrote %v", entries)
	}
}
