// SPDX-License-Identifier: Apache-2.0

package affected_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/shell"
)

// ownershipReaderFiles is a repository whose make targets run a script that
// reads a parameter-default path and a script that reads every tracked path,
// with an owner declaration that takes both data paths out of unknown.
func ownershipReaderFiles(declaration string) map[string]string {
	return map[string]string{
		"Makefile": "check: register-check tree-check\n\n" +
			"register-check:\n\t@script/check-register.sh\n\n" +
			"tree-check:\n\t@script/tree-check.sh\n",
		// AZU-V0-8920: the read path is only the default word of ${1:-...}.
		"script/check-register.sh": "#!/usr/bin/env bash\nfile=${1:-conformance/register.md}\nawk '/^# R-/ { n++ } END { print n }' \"$file\"\n",
		// The enumeration hides inside a perl program, which no shell rule
		// sees: only a declaration can name it (AZU-V0-8922).
		"script/tree-check.sh":         "#!/bin/sh\nperl - <<'PERL'\nopen my $l, '-|', 'git', 'ls-files', '-z' or die;\nPERL\n",
		"script/register_test.sh":      "#!/bin/sh\necho owned\n",
		"conformance/register.md":      "# R-1\n",
		"data/README.md":               "data\n",
		affected.DeclaredSelectionPath: declaration,
	}
}

const ownershipReaderOwners = `"owners":[{"paths":"conformance/**","tests":["script/register_test.sh"]},{"paths":"data/**","tests":["script/register_test.sh"]}]`

func ownershipReaderGraph(t *testing.T, declaration string) (*affected.Graph, error) {
	t.Helper()
	root := t.TempDir()
	writeFiles(t, root, ownershipReaderFiles(declaration))
	return affected.Build(context.Background(), root, shell.New(), affected.RegisteredDriver{})
}

// AZU-V0-8920 (class N2 of the V1-4685 reader audit): a script whose read
// path is the default word of a parameter expansion reads it, so the make
// target that runs the script is selected when the path, owned only by a
// declaration, changes alone. Before the rule the expansion was blanked and
// the target was left out while the owner took the path out of unknown.
func TestParameterDefaultPathSelectsItsMakeTarget_AZU_V0_8920(t *testing.T) {
	graph, err := ownershipReaderGraph(t, `{"profile":"corvint-test-selection/0",`+ownershipReaderOwners+`}`)
	if err != nil {
		t.Fatal(err)
	}
	unit, ok := graph.Unit("shell:script/check-register.sh")
	if !ok || !slices.Contains(unit.PathTokens, "conformance/register.md") {
		t.Fatalf("default word is no path token: %+v", unit.PathTokens)
	}
	plan := affected.Select(graph, []string{"conformance/register.md"})
	if slices.ContainsFunc(plan.Unknown, func(entry affected.Unknown) bool { return entry.Reason == affected.UnknownUnownedDirtyPath }) {
		t.Fatalf("owner left the path unknown: %+v", plan.Unknown)
	}
	wantDriverSelection(t, driverSelections(plan), []string{"driver:make:Makefile#register-check", "driver:make:Makefile#check"}, []string{"driver:make:Makefile#tree-check"})
}

// AZU-V0-8920: only value-supplying forms keep their word; `${N:?WORD}` is an
// error message, and other operators keep blanking the expansion.
func TestParameterDefaultFormsAreTokens_AZU_V0_8920(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"script/forms.sh": "#!/bin/sh\na=${1:-d/one.txt}\nb=${B-d/two.txt}\nc=${C:=d/three.txt}\nd=${D=d/four.txt}\ne=${E:+d/five.txt}\nf=${F+d/six.txt}\ng=${G:?d/seven.txt}\nh=${H#d/eight.txt}\ncat \"$a\" \"$b\" \"$c\" \"$d\" \"$e\" \"$f\" \"$g\" \"$h\"\n",
	})
	graph, err := affected.Build(context.Background(), root, shell.New())
	if err != nil {
		t.Fatal(err)
	}
	unit, _ := graph.Unit("shell:script/forms.sh")
	for _, token := range []string{"d/one.txt", "d/two.txt", "d/three.txt", "d/four.txt", "d/five.txt", "d/six.txt"} {
		if !slices.Contains(unit.PathTokens, token) {
			t.Errorf("value word %s is no token: %v", token, unit.PathTokens)
		}
	}
	for _, token := range []string{"d/seven.txt", "d/eight.txt"} {
		if slices.Contains(unit.PathTokens, token) {
			t.Errorf("non-value word %s became a token: %v", token, unit.PathTokens)
		}
	}
}

// AZU-V0-8921, AZU-V0-8922 (class W): a declared whole-tree reader makes
// every driver that runs it a PATH_GLOB_READER of every dirty path, and the
// drivers that depend on it follow; without the declaration the target is
// left out, because no rule sees the perl enumeration.
func TestDeclaredTreeReaderSelectsItsDrivers_AZU_V0_8922(t *testing.T) {
	graph, err := ownershipReaderGraph(t, `{"profile":"corvint-test-selection/0",`+ownershipReaderOwners+`}`)
	if err != nil {
		t.Fatal(err)
	}
	wantDriverSelection(t, driverSelections(affected.Select(graph, []string{"data/README.md"})), nil, []string{"driver:make:Makefile#tree-check"})

	graph, err = ownershipReaderGraph(t, `{"profile":"corvint-test-selection/2",`+ownershipReaderOwners+
		`,"treeReaders":[{"runs":"script/tree-check.sh","reason":"perl lists every tracked path"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	// The aggregate target runs the check as a prerequisite, so it is a
	// tree reader driver too.
	if got := graph.TreeReaderDrivers(); len(got) != 2 || got["driver:make:Makefile#tree-check"] != "script/tree-check.sh" || got["driver:make:Makefile#check"] != "script/tree-check.sh" {
		t.Fatalf("tree reader drivers = %v", got)
	}
	for _, dirty := range []string{"data/README.md", "conformance/register.md", "notes/new.txt"} {
		selected := driverSelections(affected.Select(graph, []string{dirty}))
		wantDriverSelection(t, selected, []string{"driver:make:Makefile#tree-check", "driver:make:Makefile#check"}, nil)
		if witness := selected["driver:make:Makefile#tree-check"].Witness; witness.Kind != affected.WitnessPathGlobReader || witness.Glob != "/*" || witness.DirtyPath != dirty {
			t.Errorf("%s: witness %+v", dirty, witness)
		}
	}
}

// AZU-V0-8921: treeReaders exists only under corvint-test-selection/2, and a
// full build refuses an entry that contradicts the observed graph.
func TestTreeReaderDeclarationRefusals_AZU_V0_8921(t *testing.T) {
	for label, tc := range map[string]struct{ declaration, want string }{
		"profile 0":        {`{"profile":"corvint-test-selection/0","treeReaders":[{"runs":"script/tree-check.sh","reason":"r"}]}`, "treeReaders needs profile corvint-test-selection/2"},
		"unowned file":     {`{"profile":"corvint-test-selection/2","treeReaders":[{"runs":"script/absent.sh","reason":"r"}]}`, `treeReaders runs "script/absent.sh" is no file`},
		"run by no driver": {`{"profile":"corvint-test-selection/2","treeReaders":[{"runs":"script/register_test.sh","reason":"r"}]}`, "run by no driver unit"},
		"directory":        {`{"profile":"corvint-test-selection/2","treeReaders":[{"runs":"script/","reason":"r"}]}`, `treeReaders runs "script/"`},
		"descending":       {`{"profile":"corvint-test-selection/2","treeReaders":[{"runs":"script/tree-check.sh","reason":"r"},{"runs":"script/check-register.sh","reason":"r"}]}`, `treeReaders runs "script/check-register.sh"`},
		"blank reason":     {`{"profile":"corvint-test-selection/2","treeReaders":[{"runs":"script/tree-check.sh","reason":""}]}`, "reason must be"},
		"other profile":    {`{"profile":"corvint-test-selection/3"}`, "profile must be"},
	} {
		if _, err := ownershipReaderGraph(t, tc.declaration); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", label, err, tc.want)
		}
	}
	// The /2 profile reads a /0 body unchanged.
	if _, err := ownershipReaderGraph(t, `{"profile":"corvint-test-selection/2",`+ownershipReaderOwners+`}`); err != nil {
		t.Fatalf("profile 2 without treeReaders: %v", err)
	}
}
