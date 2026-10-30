//go:build darwin || linux

package worksource

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testsupport"
)

func privateTestParent(t *testing.T) string {
	t.Helper()
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	return parent
}

// WQO-V0-094: ambient TMPDIR alone never moves the parent; a safe override is
// honored for acquisition scratch, which Close removes.
func TestScratchParentOverride(t *testing.T) {
	if RunParentEnvironment != testsupport.WorkRunParentEnvironment {
		t.Fatal("testsupport repeats a different override name")
	}
	t.Setenv(RunParentEnvironment, "")
	os.Unsetenv(RunParentEnvironment)
	t.Setenv("TMPDIR", privateTestParent(t))
	if parent, err := ScratchParent(); err != nil || parent != "/tmp" {
		t.Fatalf("TMPDIR moved the parent: %q %v", parent, err)
	}

	parent := privateTestParent(t)
	t.Setenv(RunParentEnvironment, parent)
	if got, err := ScratchParent(); err != nil || got != parent {
		t.Fatalf("override not honored: %q %v", got, err)
	}
	source, err := Acquire(context.Background(), fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(source.temporary) != parent || !strings.HasPrefix(filepath.Base(source.temporary), "corvint-work-source-") {
		source.Close()
		t.Fatalf("acquisition scratch %s is not under %s", source.temporary, parent)
	}
	source.Close()
	if entries, err := os.ReadDir(parent); err != nil || len(entries) != 0 {
		t.Fatalf("acquisition scratch remains: %v %v", entries, err)
	}
}

// WQO-V0-094: an unsafe override is refused, never replaced by /tmp.
func TestScratchParentOverrideRefusals(t *testing.T) {
	base := privateTestParent(t)
	directory := func(name string, mode os.FileMode) string {
		path := filepath.Join(base, name)
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
		return path
	}
	private := directory("private", 0700)
	if err := os.Mkdir(filepath.Join(private, "inner"), 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(private, link); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(base, "file")
	if err := os.WriteFile(file, nil, 0700); err != nil {
		t.Fatal(err)
	}
	shared := directory("shared", 0777)
	if err := os.Mkdir(filepath.Join(shared, "inner"), 0700); err != nil {
		t.Fatal(err)
	}
	sticky := directory("sticky", 0777|os.ModeSticky)
	if err := os.Mkdir(filepath.Join(sticky, "inner"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv(RunParentEnvironment, filepath.Join(sticky, "inner"))
	if _, err := ScratchParent(); err != nil {
		t.Fatalf("sticky shared ancestor refused: %v", err)
	}
	cases := map[string]string{
		"open ancestor": filepath.Join(shared, "inner"),
		"empty":         "",
		"relative":      "private",
		"unclean":       private + "/",
		"dot segment":   base + "/./private",
		"missing":       filepath.Join(base, "missing"),
		"symlink":       link,
		"through link":  filepath.Join(link, "inner"),
		"regular file":  file,
		"group open":    directory("group", 0750),
		"world open":    directory("world", 0777),
		"owner limited": directory("limited", 0500),
	}
	if os.Getuid() == 0 {
		other := directory("other", 0700)
		if err := os.Chown(other, 65534, 65534); err != nil {
			t.Fatal(err)
		}
		cases["other owner"] = other
	} else {
		t.Log("other-owner refusal needs root to chown; covered by the owner comparison only")
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv(RunParentEnvironment, value)
			if parent, err := ScratchParent(); err == nil || parent != "" {
				t.Fatalf("unsafe override accepted: %q", parent)
			}
			if source, err := newSource(); err == nil {
				source.Close()
				t.Fatal("acquisition used an unsafe override")
			}
		})
	}
}
