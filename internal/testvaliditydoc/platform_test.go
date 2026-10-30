package testvaliditydoc

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// PRS-V1-022 (V1-2484): receipt reads are qualified on darwin and linux only;
// everywhere else PlatformRefusal and every read return the one typed
// refusal, never a generic open failure.
func TestPRSV1022ReceiptPlatformRefusalMatchesTheQualifiedPlatforms(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a", "r.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	_, readErr := ReadFile(dir, filepath.Join("a", "r.json"))
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		if PlatformRefusal() != nil || readErr != nil {
			t.Fatalf("%s: refusal=%v read=%v, want a qualified read", runtime.GOOS, PlatformRefusal(), readErr)
		}
		return
	}
	if !errors.Is(PlatformRefusal(), ErrPlatformUnsupported) || !errors.Is(readErr, ErrPlatformUnsupported) {
		t.Fatalf("%s: refusal=%v read=%v, want %v", runtime.GOOS, PlatformRefusal(), readErr, ErrPlatformUnsupported)
	}
}
