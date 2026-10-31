package testvaliditybridge

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// PRS-V1-920 (V1-2683): the bridge reports the receipt platform refusal as
// the typed platform code, not as an unreadable receipt; other read failures
// keep invalid-test-validity-receipt.
func TestPRSV1920BridgeReportsThePlatformRefusal(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "receipt.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	identity, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	registry := &Registry{root: root, rootIdentity: identity}
	saved := readReceiptFile
	t.Cleanup(func() { readReceiptFile = saved })
	for _, tc := range []struct {
		cause      error
		code, text string
	}{
		{fmt.Errorf("open: %w", testvaliditydoc.ErrPlatformUnsupported), testvaliditydoc.PlatformUnsupportedCode, testvaliditydoc.ErrPlatformUnsupported.Error()},
		{errors.New("not a regular file"), "invalid-test-validity-receipt", "receipt is not a readable regular file"},
	} {
		readReceiptFile = func(*os.Root, string) ([]byte, error) { return nil, tc.cause }
		_, failure := registry.readResolved("receipt.json")
		if failure == nil || failure.Code != tc.code || failure.Message != tc.text {
			t.Fatalf("cause %v: got %+v, want %s %q", tc.cause, failure, tc.code, tc.text)
		}
	}
}
