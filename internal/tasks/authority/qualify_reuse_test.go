//go:build darwin || linux

package authority

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// LGL-V0-890 (V1-2203): a root qualifies once per process while it is the
// same directory; a replaced root is probed again, and a failed qualification
// is never reused.
func TestLGLV0890_QualificationIsReusedWhileTheRootIsTheSameDirectory(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.Join(root, "common")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	probes, fail := 0, false
	prev := qualifyFresh
	t.Cleanup(func() { qualifyFresh = prev })
	qualifyFresh = func(r string) (*Qualification, error) {
		probes++
		if fail {
			return &Qualification{Root: r}, errors.New("injected qualification failure")
		}
		return prev(r)
	}
	for i := 0; i < 3; i++ {
		if q, err := QualifyReused(root); err != nil || !q.DirSync || !q.FullSync {
			t.Fatalf("call %d: %+v %v", i, q, err)
		}
	}
	if probes != 1 {
		t.Fatalf("%d probes for three writes over one directory, want 1", probes)
	}
	// A directory replaced at the same path is a new directory.
	if err := os.Rename(root, root+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := QualifyReused(root); err != nil || probes != 2 {
		t.Fatalf("replaced root: %v after %d probes, want a second probe", err, probes)
	}
	// A failure is reported and not kept: the next call probes again.
	other := root + ".other"
	if err := os.Mkdir(other, 0o700); err != nil {
		t.Fatal(err)
	}
	fail = true
	if _, err := QualifyReused(other); err == nil {
		t.Fatal("an injected failure qualified")
	}
	fail = false
	if _, err := QualifyReused(other); err != nil || probes != 4 {
		t.Fatalf("after a failure: %v after %d probes, want a fresh probe", err, probes)
	}
}
