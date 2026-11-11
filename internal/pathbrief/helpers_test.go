// SPDX-License-Identifier: Apache-2.0

package pathbrief

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testsupport"
)

func TestMain(m *testing.M) { testsupport.IsolateGit(); os.Exit(m.Run()) }

// git runs one fixture git command in root and returns its trimmed output.
func git(t *testing.T, root string, arguments ...string) string {
	t.Helper()
	return strings.TrimSpace(testsupport.RunGit(t, "git", root, arguments...))
}

// newRepo makes a temporary repository holding files in one commit and returns its root.
func newRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q")
	commitAll(t, root, files)
	return root
}

// commitAll writes files (a nil value deletes the file), commits everything and returns the new
// commit.
func commitAll(t *testing.T, root string, files map[string]string) string {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if content == deleted {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "--allow-empty", "-m", "fixture")
	return git(t, root, "rev-parse", "HEAD")
}

// deleted marks a file for removal in commitAll.
const deleted = "\x00deleted"

func blobAt(t *testing.T, root, spec string) string {
	t.Helper()
	return git(t, root, "rev-parse", spec)
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// build runs Build for already-normalised paths and checks the invariants of every brief.
func build(t *testing.T, root string, sources Sources, paths ...string) Brief {
	t.Helper()
	brief, err := Build(context.Background(), root, paths, sources)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	checkInvariants(t, brief)
	return brief
}

var sectionNames = []string{"instructions", "owners", "tests", "flows", "requirements", "tickets"}

// checkInvariants holds for every brief: TBC-V1-010 order, limit and items; TBC-V1-011 state rules.
func checkInvariants(t *testing.T, brief Brief) {
	t.Helper()
	for _, p := range brief.Paths {
		if len(p.Sections) != len(sectionNames) {
			t.Fatalf("%s: %d sections", p.Path, len(p.Sections))
		}
		for i, s := range p.Sections {
			where := fmt.Sprintf("%s/%s", p.Path, s.Name)
			switch {
			case s.Name != sectionNames[i]:
				t.Errorf("%s: section %d is %q", p.Path, i, s.Name)
			case s.Items == nil:
				t.Errorf("%s: nil Items", where)
			case len(s.Items) > SectionLimit || s.Truncated != (s.Total > len(s.Items)):
				t.Errorf("%s: items %d total %d truncated %v", where, len(s.Items), s.Total, s.Truncated)
			case s.State == StateObserved && len(s.MissingSources) != 0:
				t.Errorf("%s: OBSERVED with missing %v", where, s.MissingSources)
			case s.State == StateIncomplete && s.Total == 0:
				t.Errorf("%s: INCOMPLETE with no items", where)
			case s.Name == "requirements" && s.State == StateObserved:
				t.Errorf("%s: OBSERVED", where)
			case s.State != StateObserved && s.State != StateIncomplete && s.State != StateNotObserved:
				t.Errorf("%s: state %q", where, s.State)
			case s.Reason == "":
				t.Errorf("%s: empty reason", where)
			}
			for _, source := range s.MissingSources {
				if !strings.Contains(s.Reason, source) {
					t.Errorf("%s: reason %q does not name %s", where, s.Reason, source)
				}
			}
		}
	}
}

// pick returns the named section of path index i.
func pick(t *testing.T, brief Brief, i int, name string) Section {
	t.Helper()
	for _, s := range brief.Paths[i].Sections {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("no section %s", name)
	return Section{}
}

func itemPaths(items []Item) []string {
	out := []string{}
	for _, item := range items {
		out = append(out, item.Path)
	}
	return out
}

func equal(t *testing.T, what string, got, want any) {
	t.Helper()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

func hasPrefixAndNames(t *testing.T, s Section, prefix string, names ...string) {
	t.Helper()
	if !strings.HasPrefix(s.Reason, prefix) {
		t.Errorf("%s reason %q does not start with %q", s.Name, s.Reason, prefix)
	}
	for _, name := range names {
		if !strings.Contains(s.Reason, name) {
			t.Errorf("%s reason %q does not name %s", s.Name, s.Reason, name)
		}
	}
}
