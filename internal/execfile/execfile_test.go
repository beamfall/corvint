package execfile

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// WQO-V0-093: WriteFile keeps os.WriteFile's contract (content, mode,
// truncation of an existing file) and Guard returns the callback's error.
func TestWriteFileMatchesOSWriteFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "script")
	if err := WriteFile(path, []byte("#!/bin/sh\nexit 3\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "#!/bin/sh\nexit 0\n" {
		t.Fatalf("rewrite did not truncate: %q", raw)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Windows has no executable bit: Go reports writable regular files as 0666 there.
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o100 == 0 {
		t.Fatalf("executable bit missing: %v", info.Mode())
	}
	sentinel := errors.New("sentinel")
	if err := Guard(func() error { return sentinel }); !errors.Is(err, sentinel) {
		t.Fatalf("Guard returned %v, want the callback's error", err)
	}
	if err := WriteFile(filepath.Join(path, "child"), nil, 0o700); err == nil {
		t.Fatal("write under a regular file succeeded")
	}
}
