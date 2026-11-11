// SPDX-License-Identifier: Apache-2.0
package shell

import (
	"reflect"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AFP-V0-9961: a closed list of command forms whose output depends only on
// entry names and kinds records a name-listing read; every other form, and
// an admitted form whose names can reach a command that opens files, stays a
// content read. Either way the token stays in PathTokens (AFP-V0-9133).
func TestNameListingForms_AFPV09961(t *testing.T) {
	admitted := map[string]string{
		"ls_count":       "ls script | wc -l\n",
		"ls_substituted": "test \"$(ls script | wc -l)\" -eq 3\n",
		"ls_a":           "ls -a script | wc -l\n",
		"ls_1A":          "ls -1A script | wc -l\n",
		"ls_R":           "ls -R script | wc -l\n",
		"ls_stdout":      "ls script\n",
		"ls_filters":     "ls script | grep -v x | sort -r | head -n 3\n",
		"ls_devnull":     "ls script 2>/dev/null | wc -l\n",
		"ls_if":          "if ls script | grep -q a.sh; then echo ok; fi\n",
		"find_bare":      "find script | wc -l\n",
		"find_name":      "find script -name '*.sh' | wc -l\n",
		"find_depth":     "find script -maxdepth 1 -type f -print0 | wc -c\n",
		"find_path":      "find script -path 'script/a.sh' -print | wc -l\n",
		"tree_bare":      "tree script | wc -l\n",
		// Commands whose every read a literal token bounds keep it admitted.
		"plain_rest": "set -eu\nls script | wc -l\ncat docs/x.md\nhead -n 2 docs/x.md >/dev/null\n[ -f script/a.sh ] || exit 1\nif grep -q x docs/x.md; then echo ok >&2; fi\nwhile read -r l; do echo \"$l\"; done < docs/x.md\n",
	}
	refused := map[string]string{
		"ls_l":          "ls -l script | wc -l\n",
		"ls_long":       "ls --all script | wc -l\n",
		"find_newer":    "find script -newer script/a.sh | wc -l\n",
		"find_exec":     "find script -name '*.sh' -exec cat {} \\; | wc -l\n",
		"find_size":     "find script -size +1k | wc -l\n",
		"find_or":       "find script -name a -o -name b | wc -l\n",
		"find_follow":   "find -L script -name '*.sh' | wc -l\n",
		"find_computed": "find script -name \"$pattern\" | wc -l\n",
		"tree_flag":     "tree -s script | wc -l\n",
		"du":            "du script | wc -l\n",
		"grep_r":        "grep -rn FIXME script | wc -l\n",
		"computed":      "ls script \"$extra\" | wc -l\n",
		"xargs":         "ls script | xargs cat\n",
		"wc_files0":     "find script -print0 | wc --files0-from=- -l\n",
		"loop":          "for f in $(ls script); do cat \"script/$f\"; done\n",
		"while":         "ls script | while read -r f; do cat \"script/$f\"; done\n",
		"group":         "{ ls script; } | xargs cat\n",
		"captured":      "names=$(ls script)\nfor f in $names; do cat \"script/$f\"; done\n",
		"redirect":      "ls script > names.txt\n",
		"tee":           "ls script | tee names.txt | wc -l\n",
		"run_prefix":    "run ls script\n",
		"env_prefix":    "env ls script | wc -l\n",
		"function":      "ls() { cat \"$@\"; }\nls script | wc -l\n",
		"filter_fn":     "wc() { cat \"$@\"; }\nls script | wc -l\n",
		"path":          "PATH=\"$PWD/stub:$PATH\"\nls script | wc -l\n",
		"sources":       ". ./lib.sh\nls script | wc -l\n",
		"exec":          "exec > names.txt\nls script\n",
		"glob":          "ls script | grep -c x; ls script/*.sh | wc -l\n",
		// The unit gate: another command that may read below the listed
		// directory by a name no token holds keeps the listing a content read.
		"computed_read": "ls script | wc -l\nfor f in a b; do cat \"script/$f\"; done\n",
		"cd":            "ls script | wc -l\ncd script && cat a.sh\n",
		"helper_run":    "ls script | wc -l\n./tools/run.sh\n",
		"untokened":     "ls script | wc -l\ncat README\n",
		"size_test":     "ls script | wc -l\n[ -s script/a.sh ]\n",
		"git":           "ls script | wc -l\ngit diff --quiet\n",
		"grep_e":        "ls script | wc -l\ngrep -e x script/a.sh\n",
		"diff_r":        "ls script | wc -l\ndiff -r script/x script/y\n",
		// A directory operand makes diff read every file below it, and a
		// checksum tool's check mode reads the files its sums file names;
		// neither read has a token of its own.
		"diff_dir":      "ls script | wc -l\ndiff script/ golden/\n",
		"diff_dot":      "ls script | wc -l\ndiff -q ./script ./golden\n",
		"diff_file_dir": "ls script | wc -l\ndiff docs/a.sh script/\n",
		"diff_files":    "ls script | wc -l\ndiff docs/a.sh docs/b.sh\n",
		"sha_check":     "ls script | wc -l\nsha256sum -c docs/sums.sha256\n",
		"shasum_check":  "ls script | wc -l\nshasum -c docs/sums.sha256\n",
		"md5_check":     "ls script | wc -l\nmd5sum -c docs/sums.md5\n",
		"sha_file":      "ls script | wc -l\nsha256sum docs/x.md\n",
		// BSD ls colours an entry by its mode bits when forced to.
		"clicolor":      "export CLICOLOR_FORCE=1 CLICOLOR=1\nls script\n",
		"clicolor_bare": "CLICOLOR_FORCE=1\nexport CLICOLOR_FORCE\nls script\n",
		"clicolor_cmd":  "CLICOLOR_FORCE=1 ls script | wc -l\n",
		// Every other way to set an environment-style name is refused too.
		"grep_options": "GREP_OPTIONS=-r\nls script | grep -c a\n",
		"export_name":  "preload=x\nexport preload\nls script\n",
		"read_env":     "read -r PATH < docs/x.md\nls script | wc -l\n",
		"for_env":      "for PATH in docs/x.md; do echo x; done\nls script | wc -l\n",
		"printf_env":   "printf -v PATH '%s' stub\nls script | wc -l\n",
		"default_env":  ": \"${CLICOLOR_FORCE:=1}\"\nls script\n",
		"input_var":    "ls script | wc -l\nwhile read -r l; do echo \"$l\"; done < \"$list\"\n",
		"subshell_fn":  "count() { ls script | wc -l; }\ntest \"$(count)\" -eq 1\n",
	}
	root := t.TempDir()
	write(t, root, "script/a.sh", "true\n")
	write(t, root, "lib.sh", "true\n")
	for name, body := range admitted {
		write(t, root, name+"_test.sh", body)
	}
	for name, body := range refused {
		write(t, root, name+"_test.sh", body)
	}
	// A helper's bare operand is not named at all (AFP-V0-9133), and a
	// helper holds no name-listing read.
	write(t, root, "tools/list-helper.sh", "find script -path 'script/a.sh' | wc -l\n")
	result := units(t, root)
	for name := range admitted {
		unit := unitOf(t, result, name+"_test.sh")
		if !slices.Contains(unit.PathTokens, "/script/") || !slices.Contains(unit.NameListings, "/script/") {
			t.Errorf("%s: tokens %v listings %v; want /script/ as a name-listing read", name, unit.PathTokens, unit.NameListings)
		}
	}
	for name := range refused {
		unit := unitOf(t, result, name+"_test.sh")
		if slices.Contains(unit.NameListings, "/script/") {
			t.Errorf("%s: listings %v; want /script/ kept a content read", name, unit.NameListings)
		}
	}
	if unit := unitOf(t, result, "tools/list-helper.sh"); len(unit.NameListings) != 0 || slices.Contains(unit.PathTokens, "/script/") {
		t.Errorf("helper: tokens %v listings %v; want neither", unit.PathTokens, unit.NameListings)
	}
	// A pattern or path argument of a name-only command is a name filter,
	// held by the name-listing read with the directory.
	if unit := unitOf(t, result, "find_path_test.sh"); !reflect.DeepEqual(unit.NameListings, []string{"/script/", "script/a.sh"}) {
		t.Errorf("find_path: listings %v; want /script/ and script/a.sh", unit.NameListings)
	}
}

// AFP-V0-9961: a unit with a content read and a name-listing read of the
// same directory keeps the content read; a content read of another path
// leaves the listing narrowed.
func TestNameListingContentDecides_AFPV09961(t *testing.T) {
	root := t.TempDir()
	write(t, root, "script/a.sh", "true\n")
	write(t, root, "both_test.sh", "ls script | wc -l\n! grep -rn FIXME script\n")
	write(t, root, "other_test.sh", "ls script | wc -l\ncat docs/x.md\n")
	// A data body's `/script/` fragment is a content read of the directory.
	write(t, root, "data_test.sh", "ls script | wc -l\ncat <<'EOF'\n$ROOT/script/\nEOF\n")
	result := units(t, root)
	if unit := unitOf(t, result, "data_test.sh"); len(unit.NameListings) != 0 || !slices.Contains(unit.PathTokens, "/script/") {
		t.Errorf("data: tokens %v listings %v; want /script/ a content read", unit.PathTokens, unit.NameListings)
	}
	if unit := unitOf(t, result, "both_test.sh"); len(unit.NameListings) != 0 || !slices.Contains(unit.PathTokens, "/script/") {
		t.Errorf("both: tokens %v listings %v; want /script/ a content read", unit.PathTokens, unit.NameListings)
	}
	if unit := unitOf(t, result, "other_test.sh"); !reflect.DeepEqual(unit.NameListings, []string{"/script/"}) || !slices.Contains(unit.PathTokens, "docs/x.md") {
		t.Errorf("other: tokens %v listings %v; want /script/ listed and docs/x.md content", unit.PathTokens, unit.NameListings)
	}
}

// statusPlan selects tests for one dirty path with the given status;
// affected.ChangeUnknown passes no status at all.
func statusPlan(graph *affected.Graph, dirty string, status affected.ChangeStatus) []string {
	if status != affected.ChangeUnknown {
		graph = graph.WithChangeStatus(map[string]affected.ChangeStatus{dirty: status})
	}
	return affected.Select(graph, []string{dirty}).SelectedTests()
}

// AFP-V0-9962: `ls script | wc -l` is selected on an added, deleted,
// type-changed or unknown path in script/, not on an edit of an existing
// file; a content read of the same directory, and a refused form, are
// selected on the edit. A missing status plans as before the status existed.
func TestNameListingSelection_AFPV09962(t *testing.T) {
	root := t.TempDir()
	write(t, root, "script/a.sh", "true\n")
	write(t, root, "script/new.sh", "true\n")
	write(t, root, "script/sub/deep.sh", "true\n")
	write(t, root, "docs/x.md", "x\n")
	write(t, root, "lister_test.sh", "test \"$(ls script | wc -l)\" -eq 2\n")
	write(t, root, "finder_test.sh", "find script -name '*.sh' | wc -l\n")
	write(t, root, "both_test.sh", "ls script | wc -l\n! grep -rn FIXME script\n")
	write(t, root, "long_test.sh", "ls -l script | wc -l\n")
	write(t, root, "piped_test.sh", "ls script | xargs cat\n")
	write(t, root, "helper_test.sh", ". ./lib.sh\nls script | wc -l\n")
	write(t, root, "lib.sh", "true\n")
	graph := build(t, root)
	narrowed := []string{"lister_test.sh", "finder_test.sh"}
	content := []string{"both_test.sh", "long_test.sh", "piped_test.sh", "helper_test.sh"}
	for _, row := range []struct {
		dirty  string
		status affected.ChangeStatus
	}{
		{"script/new.sh", affected.ChangeAdded},
		{"script/gone.sh", affected.ChangeDeleted},
		{"script/a.sh", affected.ChangeTypeChanged},
		{"script/a.sh", affected.ChangeUnknown},
		{"script/a.sh", "renamed"},
		// A deeper addition can add a directory to a direct listing.
		{"script/sub/deep.sh", affected.ChangeAdded},
	} {
		selected := statusPlan(graph, row.dirty, row.status)
		for _, test := range append(slices.Clone(narrowed), content...) {
			if !slices.Contains(selected, test) {
				t.Errorf("%s %s: %s not selected: %v", row.dirty, row.status, test, selected)
			}
		}
	}
	for _, dirty := range []string{"script/a.sh", "script/sub/deep.sh"} {
		selected := statusPlan(graph, dirty, affected.ChangeModified)
		for _, test := range narrowed {
			if slices.Contains(selected, test) {
				t.Errorf("%s modified: %s selected: %v", dirty, test, selected)
			}
		}
		for _, test := range content {
			if !slices.Contains(selected, test) {
				t.Errorf("%s modified: %s not selected: %v", dirty, test, selected)
			}
		}
	}
	// Elsewhere nothing changes: docs/x.md selects none of them.
	for _, status := range []affected.ChangeStatus{affected.ChangeAdded, affected.ChangeModified} {
		selected := statusPlan(graph, "docs/x.md", status)
		for _, test := range append(slices.Clone(narrowed), content...) {
			if slices.Contains(selected, test) {
				t.Errorf("docs/x.md %s: %s selected: %v", status, test, selected)
			}
		}
	}
	// Select without a status equals Select with every path unknown, and
	// with an empty status map (AFP-V0-9960).
	dirty := []string{"docs/x.md", "script/a.sh", "script/new.sh"}
	plain := affected.Select(graph, dirty)
	if empty := affected.Select(graph.WithChangeStatus(nil), dirty); !reflect.DeepEqual(plain, empty) {
		t.Errorf("an empty status changes the plan:\n%+v\n%+v", plain, empty)
	}
	unknown := map[string]affected.ChangeStatus{}
	for _, path := range dirty {
		unknown[path] = affected.ChangeUnknown
	}
	if all := affected.Select(graph.WithChangeStatus(unknown), dirty); !reflect.DeepEqual(plain, all) {
		t.Errorf("an all-unknown status changes the plan:\n%+v\n%+v", plain, all)
	}
	// One listing-changing path among edits keeps the lister selected.
	mixed := graph.WithChangeStatus(map[string]affected.ChangeStatus{"script/a.sh": affected.ChangeModified, "script/new.sh": affected.ChangeAdded})
	if selected := affected.Select(mixed, []string{"script/a.sh", "script/new.sh"}).SelectedTests(); !slices.Contains(selected, "lister_test.sh") {
		t.Errorf("an added path beside an edit: lister not selected: %v", selected)
	}
}

// AFP-V0-9963: a tree walker whose every walking command is name-only is an
// unbounded reader selected only on a status that can change a listing; one
// content-searching walker keeps the unit selected on every dirty path.
func TestNameOnlyTreeWalker_AFPV09963(t *testing.T) {
	names := map[string]string{
		"find_md":    "find . -name '*.md' | wc -l\n",
		"find_count": "test \"$(find . -name '*.sh' | wc -l)\" -eq 2\n",
		"ls_r":       "ls -R . | wc -l\n",
		"find_cwd":   "find -type f | wc -l\n",
	}
	contents := map[string]string{
		"grep_tree":  "! grep -rn TODO .\n",
		"both":       "find . -name '*.md' | wc -l\n! grep -rn TODO .\n",
		"du_cwd":     "du -sh\n",
		"find_newer": "find . -newer marker | wc -l\n",
		"find_exec":  "find . -name '*.md' -exec cat {} \\;\n",
		"find_loop":  "for f in $(find . -name '*.md'); do cat \"$f\"; done\n",
		// A name-only walk beside a computed read keeps the unbounded read.
		"find_computed_read": "find . -name '*.md' | wc -l\nfor f in $(git ls-files '*.md'); do cat \"$f\"; done\n",
		"find_cd":            "test \"$(find . -name '*.md' | wc -l)\" -eq 1\ncd docs && cat a.md\n",
		// A name-only walk beside a directory diff or a checksum check,
		// which read files no token names.
		"walk_diff": "find . -type f | wc -l\ndiff -q ./docs ./golden\n",
		"walk_sha":  "find . -name '*.md' | wc -l\nsha256sum -c sums.sha256\n",
	}
	root := t.TempDir()
	write(t, root, "internal/x.go", "package x\n")
	write(t, root, "docs/a.md", "x\n")
	for name, body := range names {
		write(t, root, name+"_test.sh", body)
	}
	for name, body := range contents {
		write(t, root, name+"_test.sh", body)
	}
	graph := build(t, root)
	for name := range names {
		unit, _ := graph.Unit("shell:" + name + "_test.sh")
		if unit.UnboundedReads != UnboundedTreeWalk || !unit.UnboundedNamesOnly {
			t.Errorf("%s: unbounded %q names-only %v; want a name-only tree walk", name, unit.UnboundedReads, unit.UnboundedNamesOnly)
		}
	}
	for name := range contents {
		unit, _ := graph.Unit("shell:" + name + "_test.sh")
		if unit.UnboundedReads != UnboundedTreeWalk || unit.UnboundedNamesOnly {
			t.Errorf("%s: unbounded %q names-only %v; want a content tree walk", name, unit.UnboundedReads, unit.UnboundedNamesOnly)
		}
	}
	for _, row := range []struct {
		dirty    string
		status   affected.ChangeStatus
		narrowed bool
	}{
		{"internal/x.go", affected.ChangeModified, true},
		{"docs/a.md", affected.ChangeModified, true},
		{"internal/x.go", affected.ChangeUnknown, false},
		{"internal/y.go", affected.ChangeAdded, false},
		{"docs/a.md", affected.ChangeDeleted, false},
		{"internal/x.go", affected.ChangeTypeChanged, false},
	} {
		selected := statusPlan(graph, row.dirty, row.status)
		for name := range names {
			if slices.Contains(selected, name+"_test.sh") == row.narrowed {
				t.Errorf("%s %s: %s selected=%v: %v", row.dirty, row.status, name, !row.narrowed, selected)
			}
		}
		for name := range contents {
			if !slices.Contains(selected, name+"_test.sh") {
				t.Errorf("%s %s: %s not selected: %v", row.dirty, row.status, name, selected)
			}
		}
	}
	// The witness is the smallest dirty path that can change a listing.
	mixed := graph.WithChangeStatus(map[string]affected.ChangeStatus{"docs/a.md": affected.ChangeModified, "internal/y.go": affected.ChangeAdded})
	plan := affected.Select(mixed, []string{"docs/a.md", "internal/y.go"})
	for _, selection := range plan.Selected {
		if slices.Contains(selection.Tests, "find_md_test.sh") {
			if selection.Witness.Kind != affected.WitnessUnboundedReader || selection.Witness.DirtyPath != "internal/y.go" {
				t.Errorf("find_md witness %+v; want UNBOUNDED_READER on internal/y.go", selection.Witness)
			}
			return
		}
	}
	t.Errorf("find_md not selected beside an added path: %v", plan.SelectedTests())
}
