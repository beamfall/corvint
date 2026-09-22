//go:build darwin || linux

package trace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStableMetadataIncludesChangeTime(t *testing.T) {
	name := filepath.Join(t.TempDir(), "trace.jsonl")
	if err := os.WriteFile(name, []byte("trace\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(name, 0o400); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(name, 0o600); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(name)
	if err != nil {
		t.Fatal(err)
	}
	if sameMetadata(before, after) {
		t.Fatal("ctime-only metadata change accepted")
	}
}
