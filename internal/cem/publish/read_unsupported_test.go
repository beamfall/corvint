//go:build !darwin && !linux && !windows

package publish

import (
	"os"
	"path/filepath"
	"testing"
)

// PRS-V1-004: compilation does not qualify descriptor acquisition.
func TestOpenBoundedInputUnsupportedRefusesRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input")
	if err := os.WriteFile(path, []byte("bounded"), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := openBoundedInput(path)
	if file != nil || err == nil {
		t.Fatalf("unsupported acquisition returned file=%v err=%v", file, err)
	}
}
