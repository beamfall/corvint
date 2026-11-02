package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// writeExecutable writes body to path with mode through a short-lived
// /bin/sh child, so this test binary never holds a write descriptor on a file
// it later executes. With t.Parallel another test can fork while such a
// descriptor is open; the forked child keeps it until its own exec, and an
// exec of the file in that window fails ETXTBSY (V1-2587, TCL-V0-827).
func writeExecutable(t testing.TB, path string, body []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	c := exec.Command("/bin/sh", "-c", `cat > "$1" && chmod "$2" "$1"`, "sh", path, strconv.FormatUint(uint64(mode.Perm()), 8))
	c.Stdin = bytes.NewReader(body)
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("write executable %s: %v: %s", path, err, out)
	}
}
