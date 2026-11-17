package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
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

// Rehomed from the retired internal/releasegate (V1-4805) for the hostile-regression
// matrix: extract, the live release-tar reader behind corvint-update, refuses a
// traversal member and an escaping symlink, alone or together, and writes nothing
// outside dest.
func TestTarArchiveRejectsTraversalAndSymlink(t *testing.T) {
	traversal := tar.Header{Name: "../escape.py", Mode: 0o644, Size: 1, Typeflag: tar.TypeReg}
	symlink := tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "../escape.py"}
	for name, headers := range map[string][]tar.Header{
		"traversal-and-symlink": {traversal, symlink},
		"traversal":             {traversal},
		"symlink":               {symlink},
	} {
		t.Run(name, func(t *testing.T) {
			var data bytes.Buffer
			gz := gzip.NewWriter(&data)
			tw := tar.NewWriter(gz)
			for i := range headers {
				if err := tw.WriteHeader(&headers[i]); err != nil {
					t.Fatal(err)
				}
				if headers[i].Size > 0 {
					if _, err := tw.Write([]byte("x")); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := tw.Close(); err != nil {
				t.Fatal(err)
			}
			if err := gz.Close(); err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			a := filepath.Join(dir, "a.tgz")
			if err := os.WriteFile(a, data.Bytes(), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := extract(a, filepath.Join(dir, "out")); err == nil {
				t.Fatal("unsafe tar accepted")
			}
			if _, err := os.Lstat(filepath.Join(dir, "escape.py")); !os.IsNotExist(err) {
				t.Fatalf("member wrote outside dest: %v", err)
			}
			if _, err := os.Lstat(filepath.Join(dir, "out", "link")); !os.IsNotExist(err) {
				t.Fatalf("symlink materialised: %v", err)
			}
		})
	}
}
