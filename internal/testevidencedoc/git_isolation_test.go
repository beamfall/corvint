// SPDX-License-Identifier: Apache-2.0
package testevidencedoc

import (
	"context"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
)

func fixtureGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// RSP-V0-051: records and immutable query identities must share the requested root.
func TestRSPV0051ReadAmbientGitRedirects(t *testing.T) {
	root, other := repository(t), repository(t)
	retainFixture(t, root)
	fixtureGit(t, other, "-c", "user.name=Other", "-c", "user.email=other@example.test", "commit", "--allow-empty", "-qm", "other")
	q := Query{Explain: "suite/test", Base: "HEAD", Revision: "HEAD"}
	want := Read(context.Background(), root, q)
	if len(want.Records) != 1 || want.Query.Revision != fixtureGit(t, root, "rev-parse", "HEAD") {
		t.Fatal(want)
	}
	for key, value := range map[string]string{
		"GIT_DIR":              filepath.Join(other, ".git"),
		"GIT_COMMON_DIR":       filepath.Join(other, ".git"),
		"GIT_WORK_TREE":        other,
		"GIT_OBJECT_DIRECTORY": filepath.Join(other, ".git", "objects"),
		"GIT_INDEX_FILE":       filepath.Join(other, ".git", "index"),
	} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, value)
			before, otherBefore := snapshot(t, root), snapshot(t, other)
			if got := Read(context.Background(), root, q); !reflect.DeepEqual(got, want) {
				t.Fatalf("redirect changed binding: %+v", got)
			}
			if snapshot(t, root) != before || snapshot(t, other) != otherBefore {
				t.Fatal("query changed repository state")
			}
		})
	}
	t.Run("alternates-refused", func(t *testing.T) {
		t.Setenv("GIT_ALTERNATE_OBJECT_DIRECTORIES", filepath.Join(other, ".git", "objects"))
		before := snapshot(t, root)
		if got := Read(context.Background(), root, q); !slices.Contains(got.Reasons, "revision-unresolved") {
			t.Fatal(got)
		}
		if snapshot(t, root) != before {
			t.Fatal("query changed repository state")
		}
	})
}

// RSP-V0-051: only closed, understood entry transitions become decoded changes.
func TestRSPV0051ChangeKindClosedForms(t *testing.T) {
	regular := gitauth.TreeEntry{Mode: "100644", Type: "blob", OID: strings.Repeat("a", 40)}
	executable := regular
	executable.Mode = "100755"
	link := regular
	link.Mode = "120000"
	unknown := regular
	unknown.Mode = "170000"
	for _, tc := range []struct {
		name     string
		old, new gitauth.TreeEntry
		want     string
	}{
		{"add", gitauth.TreeEntry{}, regular, "add"},
		{"delete", regular, gitauth.TreeEntry{}, "delete"},
		{"modify-mode", regular, executable, "modify"},
		{"type-change", regular, link, ""},
		{"unknown-mode", regular, unknown, ""},
		{"unknown-add", gitauth.TreeEntry{}, unknown, ""},
		{"unknown-type", regular, gitauth.TreeEntry{Mode: "100644", Type: "future"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := changeKind(gitauth.DeltaPath{Old: tc.old, New: tc.new}); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
