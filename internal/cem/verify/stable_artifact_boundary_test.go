//go:build darwin || linux

package verify

import (
	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"os"
	"path/filepath"
	"testing"
)

func stableRealTemp(t *testing.T) string {
	t.Helper()
	p, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestStableArtifactRejectsStaticConfinedLinks(t *testing.T) {
	base := stableRealTemp(t)
	rootPath := filepath.Join(base, "root")
	if e := os.MkdirAll(filepath.Join(rootPath, "actual"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(rootPath, "actual", "receipt"), []byte("same exact bytes"), 0600); e != nil {
		t.Fatal(e)
	}
	for _, pair := range [][2]string{{"actual/receipt", "leaf"}, {"actual", "directory"}} {
		if e := os.Symlink(pair[0], filepath.Join(rootPath, pair[1])); e != nil {
			t.Fatal(e)
		}
	}
	root, closeRoot, e := stableOpenRoot(rootPath)
	if e != nil {
		t.Fatal(e)
	}
	defer closeRoot()
	if b, e := stableReadArtifact(root, "actual/receipt", 1024); e != nil || string(b) != "same exact bytes" {
		t.Fatalf("regular control %q %v", b, e)
	}
	for _, p := range []string{"leaf", "directory/receipt"} {
		if _, e := stableReadArtifact(root, p, 1024); cemcode.CodeOf(e) != "artifact-unavailable" {
			t.Fatalf("confined link %s accepted: %v", p, e)
		}
	}
	if e = os.Symlink("root", filepath.Join(base, "root-link")); e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{filepath.Join(base, "root-link"), filepath.Join(base, "root-link", "actual")} {
		if r, c, e := stableOpenRoot(p); e == nil {
			c()
			t.Fatalf("root/ancestry link accepted: %v", r)
		}
	}
}
func TestStableArtifactRootIdentityReplacementRefuses(t *testing.T) {
	base := stableRealTemp(t)
	named := filepath.Join(base, "root")
	if e := os.Mkdir(named, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(named, "receipt"), []byte("same"), 0600); e != nil {
		t.Fatal(e)
	}
	root, closeRoot, e := stableOpenRoot(named)
	if e != nil {
		t.Fatal(e)
	}
	defer closeRoot()
	if e = os.Rename(named, named+"-old"); e != nil {
		t.Fatal(e)
	}
	if e = os.Mkdir(named, 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(named, "receipt"), []byte("same"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = stableReadArtifact(root, "receipt", 1024); cemcode.CodeOf(e) != "artifact-unavailable" {
		t.Fatalf("replaced named root admitted: %v", e)
	}
}
