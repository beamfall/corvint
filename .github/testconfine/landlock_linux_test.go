//go:build linux

package testconfine

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestMain doubles as the confined helper: with TESTCONFINE_HELPER_ROOT set it
// confines itself to the package "pkg" with the entry "docs/" and execs a
// shell that reads one file from each place, then reparents files: within and
// between granted trees, and out of a denied one.
func TestMain(m *testing.M) {
	root := os.Getenv("TESTCONFINE_HELPER_ROOT")
	if root == "" {
		os.Exit(m.Run())
	}
	rules, err := Rules(root, "pkg", []string{"docs/"})
	if err == nil {
		script := "for f in pkg/a docs/b secret/c; do if cat $f >/dev/null 2>&1; then echo read $f; else echo denied $f; fi; done; ls secret >/dev/null 2>&1 || echo denied-list secret; " +
			"mkdir pkg/m && mv pkg/a pkg/m/a && echo moved pkg/m/a; ln docs/b pkg/b2 && echo linked pkg/b2; mv secret/c pkg/c 2>/dev/null || echo denied-move secret/c"
		err = ExecConfined(rules, "/bin/sh", []string{"sh", "-c", script}, os.Environ())
	}
	os.Stderr.WriteString(err.Error() + "\n")
	os.Exit(3)
}

func TestExecConfinedDeniesUndeclaredRepositoryReads_AFPV0023(t *testing.T) {
	if ABI() < 2 {
		t.Skip("landlock ABI 2 unavailable on this kernel; the CI wrapper fails closed instead")
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "repo")
	for _, name := range []string{"pkg/a", "docs/b", "secret/c"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command(os.Args[0])
	command.Dir = root
	command.Env = append(os.Environ(), "TESTCONFINE_HELPER_ROOT="+root)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("helper: %v\n%s", err, output)
	}
	want := "read pkg/a\nread docs/b\ndenied secret/c\ndenied-list secret\nmoved pkg/m/a\nlinked pkg/b2\ndenied-move secret/c\n"
	if got := string(output); got != want {
		t.Fatalf("confined reads:\n%s\nwant:\n%s", got, want)
	}
}
