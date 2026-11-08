// SPDX-License-Identifier: Apache-2.0
//go:build darwin || linux

package testevidencedoc

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
)

func gitShim(t *testing.T, body string) string {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	script := "#!/bin/sh\n" + body + "\nexec '" + strings.ReplaceAll(realGit, "'", `'\''`) + "' \"$@\"\n"
	if err := execfile.WriteFile(filepath.Join(dir, "git"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	return dir + string(os.PathListSeparator) + os.Getenv("PATH")
}

// RSP-V0-051: the environment is an allowlist, including variables not known today.
func TestRSPV0051ReadUnknownGitEnvironment(t *testing.T) {
	root := repository(t)
	q := Query{Explain: "suite/test", Base: "HEAD", Revision: "HEAD"}
	want := Read(context.Background(), root, q)
	t.Setenv("PATH", gitShim(t, `if [ "${GIT_FUTURE_REDIRECT+x}" = x ]; then exit 97; fi`))
	t.Setenv("GIT_FUTURE_REDIRECT", "untrusted")
	if got := Read(context.Background(), root, q); !reflect.DeepEqual(got, want) {
		t.Fatalf("unknown Git environment reached child: %+v", got)
	}
}

// RSP-V0-051: a missing tree must remain unknown without contacting a promisor
// or writing objects, even when Git does not implement GIT_NO_LAZY_FETCH.
func TestRSPV0051ReadMissingPromisorTree(t *testing.T) {
	source := repository(t)
	if err := os.WriteFile(filepath.Join(source, "src.go"), []byte("package fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, source, "add", "src.go")
	fixtureGit(t, source, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-qm", "base")
	base := fixtureGit(t, source, "rev-parse", "HEAD")
	tree := fixtureGit(t, source, "rev-parse", "HEAD^{tree}")
	if err := os.WriteFile(filepath.Join(source, "src.go"), []byte("package changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, source, "add", "src.go")
	fixtureGit(t, source, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-qm", "head")
	fixtureGit(t, source, "config", "uploadpack.allowfilter", "true")
	partial := filepath.Join(t.TempDir(), "partial")
	fixtureGit(t, source, "clone", "-q", "--no-local", "--no-checkout", "--filter=tree:0", "file://"+source, partial)
	sentinel := filepath.Join(t.TempDir(), "remote-contacted")
	fixtureGit(t, partial, "config", "remote.origin.uploadpack", "touch '"+strings.ReplaceAll(sentinel, "'", `'\''`)+"' && git-upload-pack")
	// Prove the fixture is missing this tree without permitting a setup lazy fetch.
	probe := exec.Command("git", "-C", partial, "cat-file", "-e", tree)
	probe.Env = append(os.Environ(), "GIT_NO_LAZY_FETCH=1", "GIT_ALLOW_PROTOCOL=")
	if err := probe.Run(); err == nil {
		t.Fatal("base tree unexpectedly present")
	}
	retainFixture(t, partial)
	for _, drop := range []bool{false, true} {
		t.Run(map[bool]string{false: "guarded", true: "guard-ignored"}[drop], func(t *testing.T) {
			if drop {
				t.Setenv("PATH", gitShim(t, "unset GIT_NO_LAZY_FETCH"))
			}
			before, sourceBefore := snapshot(t, partial), snapshot(t, source)
			got := Read(context.Background(), partial, Query{Explain: "suite/test", Base: base, Revision: "HEAD"})
			if got.Status != "UNKNOWN" {
				t.Fatal(got)
			}
			if got.Explanation == nil || len(got.Explanation.Runners) != 1 || got.Explanation.Runners[0].Mode != "full" || !slices.Contains(got.Explanation.Runners[0].Reasons, "diff-incomplete:diff-unresolved") {
				t.Fatal(got)
			}
			if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
				t.Fatalf("remote contacted: %v", err)
			}
			if snapshot(t, partial) != before || snapshot(t, source) != sourceBefore {
				t.Fatal("read mutated repository")
			}
		})
	}
}
