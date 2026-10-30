package update

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// V1-3179: extract joins members through archivepath.Join. A local member
// extracts beneath dest; one that filepath.IsLocal rejects for this platform,
// including a Windows reserved device name that safeName alone admits, does not.
func TestV13179ExtractConfinesMembersToDest(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "ok.tgz")
	if err := os.WriteFile(a, archiveBytes(t, map[string][]byte{"bin/corvint": []byte("x")}), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := extract(a, out); err != nil {
		t.Fatalf("local member refused: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(out, "bin", "corvint")); err != nil || string(got) != "x" {
		t.Fatalf("extracted member = %q, %v", got, err)
	}
	escapes := []string{"../outside", "a/../../escape"}
	if runtime.GOOS == "windows" {
		escapes = append(escapes, "NUL", "a/CON")
	}
	for _, n := range escapes {
		t.Run(n, func(t *testing.T) {
			d := t.TempDir()
			a := filepath.Join(d, "a.tgz")
			if err := os.WriteFile(a, archiveBytes(t, map[string][]byte{n: []byte("x")}), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := extract(a, filepath.Join(d, "out")); err == nil {
				t.Fatalf("member %q accepted", n)
			}
			if _, err := os.Stat(filepath.Join(d, "outside")); !os.IsNotExist(err) {
				t.Fatalf("member %q wrote outside dest: %v", n, err)
			}
		})
	}
}
