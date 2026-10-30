package changee2e

import (
	"archive/tar"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func untarFixture(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, name := range names {
		if err := tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: 2}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte("ok")); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// V1-3179: untar joins every member through archivepath.Join. A local member
// lands in the disposable tree; an escaping one is refused e2e-tree-unavailable
// and writes nothing beside it.
func TestV13179UntarConfinesMembersToTree(t *testing.T) {
	base := t.TempDir()
	dest := filepath.Join(base, "tree")
	if err := untar(untarFixture(t, "e2e/index.html"), dest); err != nil {
		t.Fatalf("local member refused: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(dest, "e2e", "index.html")); err != nil || string(data) != "ok" {
		t.Fatalf("e2e/index.html = %q, %v", data, err)
	}
	escapes := []string{"../outside", "e2e/../../outside", "/outside"}
	if runtime.GOOS == "windows" {
		escapes = append(escapes, "NUL", "e2e/CON")
	}
	for _, name := range escapes {
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			err := untar(untarFixture(t, name), filepath.Join(base, "tree"))
			var refusal *Refusal
			if !errors.As(err, &refusal) || refusal.Code != "e2e-tree-unavailable" {
				t.Fatalf("untar(%q) = %v, want e2e-tree-unavailable", name, err)
			}
			if _, err := os.Stat(filepath.Join(base, "outside")); !os.IsNotExist(err) {
				t.Fatalf("member %q wrote outside the tree: %v", name, err)
			}
		})
	}
}
