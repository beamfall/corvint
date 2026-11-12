// SPDX-License-Identifier: Apache-2.0
package shell

import (
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AFP-V0-9990: the shell plugin records, for every unit, the literal
// directories its own commands list or walk, the first construct whose
// listing, walk or run it cannot bound, and the file-name patterns of the
// scripts it runs by a computed name. Each row is a helper (a non-test
// source), so a walk of the working directory is recorded as an opacity
// reason rather than read as a test's unbounded reads.
func TestShellWalkFacts_AFPV09990(t *testing.T) {
	cases := []struct {
		name, text string
		dirs       []string
		opaque     string
		runs       []string
	}{
		{"bare lister", "ls script | wc -l\n", []string{"script"}, "", nil},
		{"dot slash walker", "find ./script/sub -name '*.sh'\n", []string{"script/sub"}, "", nil},
		{"trailing slash search", "grep -rn FIXME docs/\n", []string{"docs"}, "", nil},
		{"recursive grep pattern skipped", "grep -r -- TODO internal\n", []string{"internal"}, "", nil},
		{"for-in glob", "for f in script/*.sh; do echo \"$f\"; done\n", []string{"script"}, "", nil},
		{"two walkers", "ls script\ndu -s tools/x\n", []string{"script", "tools/x"}, "", nil},
		{"absolute operand", "ls /usr/share\n", nil, "", nil},
		{"computed walk operand", "find \"$dir\" -name x\n", nil, OpaqueComputedWalkOperand, nil},
		{"positional walk operand", "ls \"$1\"\n", nil, OpaqueComputedWalkOperand, nil},
		{"climbing operand", "find ../x -name y\n", nil, OpaqueClimbingWalkOperand, nil},
		{"root operand", "du -s /\n", nil, OpaqueClimbingWalkOperand, nil},
		{"home operand", "ls ~/x\n", nil, OpaqueClimbingWalkOperand, nil},
		{"tree walk", "find . -name x\n", nil, OpaqueTreeWalk, nil},
		{"tree walk no operand", "grep -r FIXME\n", nil, OpaqueTreeWalk, nil},
		{"git lister", "git ls-files\n", nil, OpaqueTreeWalk, nil},
		{"forwarded command", "\"$@\"\n", nil, OpaqueForwardedCommand, nil},
		{"forwarded positional", "$1 script\n", nil, OpaqueForwardedCommand, nil},
		{"computed command", "\"$tool\" --version\n", nil, OpaqueComputedCommand, nil},
		{"eval", "eval \"$x\"\n", nil, OpaqueEval, nil},
		{"xargs", "echo a | xargs ls\n", nil, OpaqueXargs, nil},
		{"inline python", "python3 -c 'print(1)'\n", nil, OpaqueInterpreter, nil},
		{"python from stdin", "python3 - <<'EOF'\nprint(1)\nEOF\n", nil, OpaqueInterpreter, nil},
		{"deno program file", "deno run tools/x.ts\n", nil, OpaqueInterpreter, nil},
		{"bun program file", "bun tools/x.ts\n", nil, "", nil},
		{"python is a toolchain", "python3 tools/x.py\n", nil, OpaqueToolchain, nil},
		{"perl program file", "perl tools/x.pl\n", nil, OpaqueInterpreter, nil},
		{"awk system", "awk '{system(\"ls\")}' f.txt\n", nil, OpaqueAwkCommand, nil},
		{"awk print", "awk '{print $1}' f.txt\n", nil, "", nil},
		{"working directory ls", "ls | wc -l\n", nil, OpaqueWorkingDirectoryListing, nil},
		{"working directory glob", "for f in *.sh; do echo \"$f\"; done\n", nil, OpaqueWorkingDirectoryListing, nil},
		{"computed glob", "for f in \"$d\"/*.sh; do echo \"$f\"; done\n", nil, OpaqueComputedGlob, nil},
		{"helper toolchain", "go test ./...\n", nil, OpaqueToolchain, nil},
		{"computed run name", "bash \"$dir/check-$name.sh\"\n", nil, "", []string{"check-*.sh"}},
		{"test command is no glob", "if [ -f \"$x\" ]; then echo y; fi\nwhile [ \"$a\" -le \"$((b * 4))\" ]; do a=1; done\n", nil, "", nil},
		{"conditional pattern is no glob", "if [[ $x == *.sh ]]; then echo y; fi\n", nil, "", nil},
		{"test operand globs", "if [ \"$f\" = *.sh ]; then echo y; fi\n", nil, OpaqueWorkingDirectoryListing, nil},
		{"parameter pattern is no glob", "echo \"${x#*/}\" ${y%%*}\n", nil, "", nil},
		{"quoted pattern is no glob", "grep -q 'a*b' f.txt\n", nil, "", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "tools/x.sh", tc.text)
			unit, ok := build(t, root).Unit("shell:tools/x.sh")
			if !ok {
				t.Fatal("no unit")
			}
			if !slices.Equal(unit.WalkDirectories, tc.dirs) || unit.WalkOpaque != tc.opaque || !slices.Equal(unit.RunNames, tc.runs) {
				t.Fatalf("walk facts dirs=%q opaque=%q runs=%q; want dirs=%q opaque=%q runs=%q", unit.WalkDirectories, unit.WalkOpaque, unit.RunNames, tc.dirs, tc.opaque, tc.runs)
			}
		})
	}
}

// AFP-V0-9990: a walk of a private temporary root names no repository
// directory and holds no opacity reason.
func TestShellWalkFactsTempRoot_AFPV09990(t *testing.T) {
	root := t.TempDir()
	write(t, root, "tools/x.sh", "tmp=$(mktemp -d)\nfind \"$tmp\" -name x\nls \"$tmp/sub\"\n")
	unit, _ := build(t, root).Unit("shell:tools/x.sh")
	if unit.WalkOpaque != "" || len(unit.WalkDirectories) != 0 {
		t.Fatalf("temp-root walk facts dirs=%q opaque=%q; want none", unit.WalkDirectories, unit.WalkOpaque)
	}
}

// AFP-V0-9991 (V1-5193): a test's literal walker operand spelled with a slash
// names its directory as a "/dir/" fragment, as a bare operand does
// (AFP-V0-9133), so a file added to or deleted from the directory selects the
// test and a file elsewhere does not.
func TestShellWalkOperandFragments_AFPV09991(t *testing.T) {
	root := t.TempDir()
	write(t, root, "script/a.sh", "true\n")
	write(t, root, "script/sub/b.sh", "true\n")
	write(t, root, "docs/x.md", "x\n")
	write(t, root, "slash_test.sh", "test \"$(ls script/ | wc -l)\" -eq 2\n")
	write(t, root, "dot_test.sh", "find ./script -name '*.sh' | wc -l\n")
	write(t, root, "sub_test.sh", "ls script/sub | wc -l\n")
	graph := build(t, root)
	for id, fragment := range map[string]string{"shell:slash_test.sh": "/script/", "shell:dot_test.sh": "/script/", "shell:sub_test.sh": "/script/sub/"} {
		unit, _ := graph.Unit(id)
		if !slices.Contains(unit.PathTokens, fragment) {
			t.Errorf("%s tokens %q; want %s", id, unit.PathTokens, fragment)
		}
	}
	added := affected.Select(graph.WithChangeStatus(map[string]affected.ChangeStatus{"script/sub/new.sh": affected.ChangeAdded}), []string{"script/sub/new.sh"}).SelectedTests()
	for _, test := range []string{"slash_test.sh", "dot_test.sh", "sub_test.sh"} {
		if !slices.Contains(added, test) {
			t.Errorf("added script/sub/new.sh: %s not selected: %v", test, added)
		}
	}
	elsewhere := affected.Select(graph, []string{"docs/x.md"}).SelectedTests()
	if len(elsewhere) != 0 {
		t.Errorf("docs/x.md selected %v; want none", elsewhere)
	}
	top := affected.Select(graph.WithChangeStatus(map[string]affected.ChangeStatus{"script/new.sh": affected.ChangeAdded}), []string{"script/new.sh"}).SelectedTests()
	if slices.Contains(top, "sub_test.sh") || !slices.Contains(top, "slash_test.sh") {
		t.Errorf("added script/new.sh selected %v; want slash_test.sh and not sub_test.sh", top)
	}
}

// V1-5193 acceptance through the shell plugin and the planner: `ls script |
// wc -l` is selected for a file added to or deleted from script/ (AC1), not
// for an edit of an existing script/ file (AC2, AFP-V0-9962), and a deleted
// script below the listed directory is admitted with the lister selected
// (AC3, AFP-V0-9992).
func TestBareOperandListerAcceptance_V15193(t *testing.T) {
	root := t.TempDir()
	write(t, root, "script/a.sh", "true\n")
	write(t, root, "script/b.sh", "true\n")
	write(t, root, "lister_test.sh", "ls script | wc -l\n")
	write(t, root, "finder_test.sh", "find script -name '*.sh' -exec sh {} \\;\n")
	graph := build(t, root)
	cases := []struct {
		path   string
		status affected.ChangeStatus
		lister bool
	}{
		{"script/new.sh", affected.ChangeAdded, true},
		{"script/gone.sh", affected.ChangeDeleted, true},
		{"script/a.sh", affected.ChangeModified, false},
	}
	for _, tc := range cases {
		plan := affected.Select(graph.WithChangeStatus(map[string]affected.ChangeStatus{tc.path: tc.status}), []string{tc.path})
		selected := plan.SelectedTests()
		if slices.Contains(selected, "lister_test.sh") != tc.lister {
			t.Errorf("%s %s: lister_test.sh selected=%v, want %v (%v)", tc.status, tc.path, !tc.lister, tc.lister, selected)
		}
		// find -exec runs what it finds, a content read decided by any edit.
		if !slices.Contains(selected, "finder_test.sh") {
			t.Errorf("%s %s: finder_test.sh not selected: %v", tc.status, tc.path, selected)
		}
		for _, unknown := range plan.Unknown {
			if unknown.Reason == affected.UnknownUnindexedSourcePath {
				t.Errorf("%s %s: unknown %+v", tc.status, tc.path, unknown)
			}
		}
	}
}

// V1-5194 acceptance: the ticket's failing inputs. Each unbounded walker test
// is selected for its input with the base change status the CLI passes
// (AC1), and a name-only walker is not selected for an edit, which cannot
// change a name listing (AC2, AFP-V0-9963).
func TestUnboundedWalkerAcceptance_V15194(t *testing.T) {
	root := t.TempDir()
	write(t, root, "script/a.sh", "true\n")
	write(t, root, "internal/x.go", "package x\n")
	write(t, root, "docs/a.md", "a\n")
	write(t, root, "count_test.sh", "test \"$(find . -name '*.sh' | wc -l)\" -eq 2\n")
	write(t, root, "fixme_test.sh", "! grep -rn FIXME .\n")
	write(t, root, "md_test.sh", "find . -name '*.md' | wc -l\n")
	graph := build(t, root)
	cases := []struct {
		test, path string
		status     affected.ChangeStatus
		want       bool
	}{
		{"count_test.sh", "script/new.sh", affected.ChangeAdded, true},
		{"fixme_test.sh", "internal/x.go", affected.ChangeModified, true},
		{"md_test.sh", "docs/new.md", affected.ChangeAdded, true},
		{"count_test.sh", "internal/x.go", affected.ChangeModified, false},
		{"md_test.sh", "docs/a.md", affected.ChangeModified, false},
		{"md_test.sh", "docs/gone.md", affected.ChangeDeleted, true},
	}
	for _, tc := range cases {
		selected := affected.Select(graph.WithChangeStatus(map[string]affected.ChangeStatus{tc.path: tc.status}), []string{tc.path}).SelectedTests()
		if slices.Contains(selected, tc.test) != tc.want {
			t.Errorf("%s %s: %s selected=%v, want %v (%v)", tc.status, tc.path, tc.test, !tc.want, tc.want, selected)
		}
	}
}

// V1-5191 through the shell plugin (AFP-V0-9992 to AFP-V0-9994): a deleted
// script raises no UNINDEXED_SOURCE_PATH; its literal reader, the lister of
// its directory, a helper that lists the directory together with the test
// that runs that helper, a test that runs a script by a computed name the
// deleted name matches and a test whose walk operand is computed are
// selected; a test that reads only an unrelated file is not.
func TestDeletedScriptObserversEndToEnd_AFPV09994(t *testing.T) {
	root := t.TempDir()
	write(t, root, "script/a.sh", "true\n")
	write(t, root, "docs/x.md", "x\n")
	write(t, root, "script/reader_test.sh", "grep -q x script/check-old.sh\n")
	write(t, root, "script/lister_test.sh", "test \"$(ls script | wc -l)\" -eq 2\n")
	write(t, root, "tools/helper.sh", "ls script | wc -l\n")
	write(t, root, "tools/helper_test.sh", "sh tools/helper.sh\n")
	write(t, root, "tools/runner_test.sh", "bash \"$dir/check-$1.sh\"\n")
	write(t, root, "tools/opaque_test.sh", "find \"$1\" -name '*.sh'\n")
	write(t, root, "tools/plain_test.sh", "cat docs/x.md\n")
	graph := build(t, root)
	const deleted = "script/check-old.sh"
	plan := affected.Select(graph.WithChangeStatus(map[string]affected.ChangeStatus{deleted: affected.ChangeDeleted}), []string{deleted})
	for _, unknown := range plan.Unknown {
		if unknown.Reason == affected.UnknownUnindexedSourcePath {
			t.Fatalf("unknown %+v", unknown)
		}
	}
	witnesses := map[string]string{}
	for _, selection := range plan.Selected {
		witnesses[selection.UnitID] = selection.Witness.Kind
	}
	want := map[string]string{
		"shell:script/reader_test.sh": affected.WitnessPathLiteralReader,
		"shell:script/lister_test.sh": affected.WitnessPathLiteralReader,
		"shell:tools/runner_test.sh":  affected.WitnessDeletedScriptObserver,
		"shell:tools/opaque_test.sh":  affected.WitnessDeletedScriptObserver,
	}
	for id, kind := range want {
		if witnesses[id] != kind {
			t.Errorf("%s witness %q, want %q (%v)", id, witnesses[id], kind, witnesses)
		}
	}
	// The helper has no test of its own, so the plan lists the test that
	// runs it, reached through the helper.
	for _, selection := range plan.Selected {
		if selection.UnitID == "shell:tools/helper_test.sh" && (selection.Witness.DirtyPath != deleted || !slices.Contains(selection.Witness.Via, "shell:tools/helper.sh")) {
			t.Errorf("helper_test.sh witness %+v; want one through shell:tools/helper.sh for %s", selection.Witness, deleted)
		}
	}
	if _, ok := witnesses["shell:tools/helper_test.sh"]; !ok {
		t.Errorf("the test that runs the observing helper is not selected: %v", witnesses)
	}
	if _, ok := witnesses["shell:tools/plain_test.sh"]; ok {
		t.Errorf("plain_test.sh selected: %v", witnesses)
	}
}
