//go:build !darwin && !linux && !windows

package analyzerswift

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// SAC-002 / PRS-V1-004: even a regular descriptor is refused without acquisition support.
func TestReadDescriptorOtherRefusesRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "descriptor")
	if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	value, err := readDescriptor(path)
	if value != nil || !errors.Is(err, errDescriptorUnsafe) {
		t.Fatalf("unsupported acquisition returned value=%q err=%v", value, err)
	}
}
