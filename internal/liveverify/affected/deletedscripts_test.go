// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// shellFake is a fake shell plugin: it claims every `.sh` path and every
// extensionless path under script/, as the shell plugin claims scripts by
// location.
func shellFake(units ...Unit) fake {
	return fake{name: "shell", units: units, owns: func(path string) bool {
		return strings.HasSuffix(path, ".sh") || strings.HasPrefix(path, "script/") && filepath.Ext(path) == ""
	}}
}

// AFP-V0-9992 (V1-5191): a deleted shell script that only the shell plugin
// claims, absent from the checkout, outside every directory the plugin does
// not observe, in a checkout that is not sparse, is an admitted deletion: the
// plan raises no UNINDEXED_SOURCE_PATH, selects its head-tree literal reader
// and selects no unrelated unit. Each control keeps the entry: the file
// present on disk, an extensionless path, a sparse checkout, a directory the
// plugin does not observe, and a second claimant.
func TestDeletedShellScriptAdmission_AFPV09992(t *testing.T) {
	const deleted = "script/check-old.sh"
	units := []Unit{
		{ID: "shell:script/reader_test.sh", Tests: []string{"script/reader_test.sh"}, PathTokens: []string{deleted}},
		{ID: "shell:script/other_test.sh", Tests: []string{"script/other_test.sh"}, PathTokens: []string{"docs/x.md"}},
	}
	plain := shellFake(units...)
	unobserving := shellFake(units...)
	unobserving.unobserved = []string{"script"}
	second := fake{name: "y", owns: func(path string) bool { return path == deleted }}
	cases := []struct {
		name      string
		languages []Language
		dirty     string
		onDisk    bool
		sparse    bool
		want      string // "" for an admitted deletion
	}{
		{"deleted script", []Language{plain}, deleted, false, false, ""},
		{"present on disk", []Language{plain}, deleted, true, false, UnknownUnindexedSourcePath},
		{"extensionless script", []Language{plain}, "script/check-old", false, false, UnknownUnindexedSourcePath},
		{"sparse checkout", []Language{plain}, deleted, false, true, UnknownUnindexedSourcePath},
		{"directory the plugin does not observe", []Language{unobserving}, deleted, false, false, UnknownLanguageFrontier},
		{"second claimant", []Language{plain, second}, deleted, false, false, UnknownUnindexedSourcePath},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.onDisk {
				if err := os.MkdirAll(filepath.Join(root, "script"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(tc.dirty)), []byte("true\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			graph, err := Build(context.Background(), root, tc.languages...)
			if err != nil {
				t.Fatal(err)
			}
			graph.sparse = tc.sparse
			plan := Select(graph, []string{tc.dirty})
			got := ""
			for _, unknown := range plan.Unknown {
				if strings.Contains(unknown.Detail, tc.dirty) || strings.Contains(unknown.Detail, filepath.Base(tc.dirty)) {
					got = unknown.Reason
				}
			}
			if got != tc.want {
				t.Fatalf("unknowns %+v, want %q on %s", plan.Unknown, tc.want, tc.dirty)
			}
			if tc.want != "" {
				return
			}
			if len(plan.Unknown) != 0 {
				t.Fatalf("unknowns %+v, want none", plan.Unknown)
			}
			if tests := plan.SelectedTests(); !slices.Equal(tests, []string{"script/reader_test.sh"}) {
				t.Fatalf("selected %v, want only the literal reader", tests)
			}
		})
	}
}

// AFP-V0-9993 and AFP-V0-9994 (V1-5191): each branch of the observer
// relation selects its unit for an admitted deleted script with a
// DELETED_SCRIPT_OBSERVER witness, and the matching control selects nothing:
// a walk directory holding the path (a different one does not), a walk
// opacity reason, computed run names matching the file name (others do not),
// a lone directory token (another directory does not), a dropped token set,
// and a declared read scope that excludes the path, which clears even an
// opaque unit. A test-less observer is carried to the test that runs it, its
// frontier bears, and Readers names the observers too. A root-walking test
// stays an UNBOUNDED_READER. Without deletedScriptObserversOf none of the
// observers is selected.
func TestDeletedScriptObservers_AFPV09994(t *testing.T) {
	const deleted = "script/sub/check-old.sh"
	units := []Unit{
		{ID: "shell:t/walk_test.sh", Tests: []string{"t/walk_test.sh"}, WalkDirectories: []string{"script"}},
		{ID: "shell:t/walksub_test.sh", Tests: []string{"t/walksub_test.sh"}, WalkDirectories: []string{"script/sub"}},
		{ID: "shell:t/walkother_test.sh", Tests: []string{"t/walkother_test.sh"}, WalkDirectories: []string{"docs", "sub/script"}},
		{ID: "shell:t/opaque_test.sh", Tests: []string{"t/opaque_test.sh"}, WalkOpaque: "computed-walk-operand"},
		{ID: "shell:t/run_test.sh", Tests: []string{"t/run_test.sh"}, RunNames: []string{"check-*.sh"}},
		{ID: "shell:t/runother_test.sh", Tests: []string{"t/runother_test.sh"}, RunNames: []string{"lint-*.sh", "old.sh"}},
		{ID: "shell:t/lone_test.sh", Tests: []string{"t/lone_test.sh"}, PathTokens: []string{"sub/"}},
		{ID: "shell:t/loneother_test.sh", Tests: []string{"t/loneother_test.sh"}, PathTokens: []string{"docs/"}},
		{ID: "shell:t/bounded_test.sh", Tests: []string{"t/bounded_test.sh"}, PathTokensBounded: true},
		{ID: "shell:t/scoped_test.sh", Tests: []string{"t/scoped_test.sh"}, WalkOpaque: "eval", ReadScoped: true, ReadScope: []string{"docs/"}},
		{ID: "shell:t/tree_test.sh", Tests: []string{"t/tree_test.sh"}, UnboundedReads: "shell:tree-walk", WalkOpaque: "tree-walk"},
		{ID: "shell:tools/helper.sh", Sources: []string{"tools/helper.sh"}, WalkDirectories: []string{"script"}, Frontier: []string{"shell:foreign-invocation-unmodelled"}, FrontierBears: FrontierBearsOnDependents},
		{ID: "shell:tools/helper_test.sh", Tests: []string{"tools/helper_test.sh"}, Imports: []string{"shell:tools/helper.sh"}},
		{ID: "shell:t/plain_test.sh", Tests: []string{"t/plain_test.sh"}, PathTokens: []string{"docs/x.md"}},
	}
	graph, err := Build(context.Background(), t.TempDir(), shellFake(units...))
	if err != nil {
		t.Fatal(err)
	}
	plan := Select(graph.WithChangeStatus(map[string]ChangeStatus{deleted: ChangeDeleted}), []string{deleted})
	kinds := map[string]string{}
	for _, selection := range plan.Selected {
		kinds[selection.UnitID] = selection.Witness.Kind
	}
	want := map[string]string{
		"shell:t/walk_test.sh":       WitnessDeletedScriptObserver,
		"shell:t/walksub_test.sh":    WitnessDeletedScriptObserver,
		"shell:t/opaque_test.sh":     WitnessDeletedScriptObserver,
		"shell:t/run_test.sh":        WitnessDeletedScriptObserver,
		"shell:t/lone_test.sh":       WitnessDeletedScriptObserver,
		"shell:t/bounded_test.sh":    WitnessDeletedScriptObserver,
		"shell:t/tree_test.sh":       WitnessUnboundedReader,
		"shell:tools/helper_test.sh": WitnessDependency,
	}
	for id, kind := range want {
		if kinds[id] != kind {
			t.Errorf("%s witness %q, want %q", id, kinds[id], kind)
		}
	}
	for id := range kinds {
		if _, wanted := want[id]; !wanted {
			t.Errorf("%s selected (%s), want it not selected", id, kinds[id])
		}
	}
	bears := false
	for _, unknown := range plan.Unknown {
		if unknown.Reason == UnknownUnindexedSourcePath {
			t.Errorf("unknown %+v", unknown)
		}
		bears = bears || unknown.Detail == "shell:foreign-invocation-unmodelled"
	}
	if !bears {
		t.Errorf("the observing helper's frontier does not bear: %+v", plan.Unknown)
	}
	readers := graph.Readers([]string{deleted})
	for _, id := range []string{"shell:t/walk_test.sh", "shell:t/opaque_test.sh", "shell:tools/helper.sh"} {
		if readers[id].Kind != WitnessDeletedScriptObserver {
			t.Errorf("Readers %s = %+v, want an observer", id, readers[id])
		}
	}
	// An edit of a present, owned script admits nothing and selects no
	// observer.
	edit := Select(graph, []string{"tools/helper.sh"})
	for _, selection := range edit.Selected {
		if selection.Witness.Kind == WitnessDeletedScriptObserver {
			t.Errorf("edit selected observer %s", selection.UnitID)
		}
	}
}

// AFP-V0-9993: matchesRunName reads "*" as any text, and walksPath matches a
// run of directory components at any depth, never the file name itself.
func TestDeletedScriptRelationHelpers_AFPV09993(t *testing.T) {
	for _, tc := range []struct {
		pattern, base string
		want          bool
	}{
		{"check-*.sh", "check-old.sh", true},
		{"*.sh", "x.sh", true},
		{"a*b*c.sh", "aXbYc.sh", true},
		{"a*b*c.sh", "acb.sh", false},
		{"check-*.sh", "lint-old.sh", false},
		{"old.sh", "old.sh", true},
		{"old.sh", "check-old.sh", false},
	} {
		if got := matchesRunName(tc.pattern, tc.base); got != tc.want {
			t.Errorf("matchesRunName(%q, %q) = %v", tc.pattern, tc.base, got)
		}
	}
	for _, tc := range []struct {
		directories []string
		path        string
		want        bool
	}{
		{[]string{"script"}, "script/a.sh", true},
		{[]string{"script"}, "x/script/y/a.sh", true},
		{[]string{"script/sub"}, "script/sub/a.sh", true},
		{[]string{"script/sub"}, "script/a.sh", false},
		{[]string{"a.sh"}, "script/a.sh", false},
		{[]string{"scrip"}, "script/a.sh", false},
		{nil, "script/a.sh", false},
	} {
		if got := walksPath(tc.directories, tc.path); got != tc.want {
			t.Errorf("walksPath(%q, %q) = %v", tc.directories, tc.path, got)
		}
	}
}

// AFP-V0-9995: each walk fact is part of the graph digest, and a graph whose
// units carry none projects none, so it keeps the digest it had before the
// facts existed.
func TestGraphDigestWalkFacts_AFPV09995(t *testing.T) {
	digest := func(unit Unit) (string, string) {
		graph, err := Build(context.Background(), t.TempDir(), shellFake(unit))
		if err != nil {
			t.Fatal(err)
		}
		body, err := graph.Canonical()
		if err != nil {
			t.Fatal(err)
		}
		return graph.Digest(), string(body)
	}
	base := Unit{ID: "shell:t_test.sh", Tests: []string{"t_test.sh"}}
	plain, body := digest(base)
	if strings.Contains(body, "walk") || strings.Contains(body, "runNames") {
		t.Fatalf("a graph without walk facts projects them: %s", body)
	}
	for name, unit := range map[string]Unit{
		"directories": {ID: base.ID, Tests: base.Tests, WalkDirectories: []string{"script"}},
		"opaque":      {ID: base.ID, Tests: base.Tests, WalkOpaque: "eval"},
		"runs":        {ID: base.ID, Tests: base.Tests, RunNames: []string{"check-*.sh"}},
	} {
		if got, _ := digest(unit); got == plain {
			t.Errorf("%s: digest unchanged", name)
		}
	}
}
