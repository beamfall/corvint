// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// makeOrderBase reads PKG immediately into RUN between two assignments of
// PKG, so `make t` runs `go test ./a`; all keeps the default goal.
const makeOrderBase = "all:\n\techo all\nPKG = ./a\nRUN := go test $(PKG)\nPKG = ./b\nt:\n\t$(RUN)\nu: $(PKG)\n\techo u\n"

// TestAttributeMakeEditOrder_NCMV1018 pins that an edit which moves a
// statement relative to the others refuses, although every per-variable
// assignment list and every rule block stays the same: moving RUN after
// both PKG assignments makes t run `go test ./b`, and moving u's header
// changes what its prerequisite list expands to. A statement added beside
// unmoved ones is still attributed.
func TestAttributeMakeEditOrder_NCMV1018(t *testing.T) {
	for _, tc := range []struct {
		name, head, reason string
		want               []string
	}{
		{"immediate assignment moved", "all:\n\techo all\nPKG = ./a\nPKG = ./b\nRUN := go test $(PKG)\nt:\n\t$(RUN)\nu: $(PKG)\n\techo u\n",
			`the order of make file statements changed at "assign`, nil},
		{"rule moved across an assignment", "all:\n\techo all\nPKG = ./a\nRUN := go test $(PKG)\nu: $(PKG)\n\techo u\nPKG = ./b\nt:\n\t$(RUN)\n",
			"the order of make file statements changed", nil},
		{"rules swapped", "all:\n\techo all\nPKG = ./a\nRUN := go test $(PKG)\nPKG = ./b\nu: $(PKG)\n\techo u\nt:\n\t$(RUN)\n",
			`the order of make file statements changed at "rule`, nil},
		{"default goal moved", "u: $(PKG)\n\techo u\nall:\n\techo all\nPKG = ./a\nRUN := go test $(PKG)\nPKG = ./b\nt:\n\t$(RUN)\n",
			"default goal changed", nil},
		{"rule added between", "all:\n\techo all\nPKG = ./a\nRUN := go test $(PKG)\nv:\n\techo v\nPKG = ./b\nt:\n\t$(RUN)\nu: $(PKG)\n\techo u\n",
			"", []string{"v"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			edit := AttributeMakeEdit("Makefile", []byte(makeOrderBase), []byte(tc.head))
			if tc.reason == "" {
				if edit.Reason != "" || !slices.Equal(edit.Targets, tc.want) {
					t.Fatalf("targets %v reason %q, want %v", edit.Targets, edit.Reason, tc.want)
				}
				return
			}
			if !strings.Contains(edit.Reason, tc.reason) || edit.Targets != nil {
				t.Fatalf("targets %v reason %q, want a refusal naming %q", edit.Targets, edit.Reason, tc.reason)
			}
		})
	}
}

// TestDriverUnfollowedCommand_NCMV1019 pins which recipe commands leave a
// make target's run set open because the scan does not follow what they run
// in turn: a repository script or binary, an interpreter given a file, a
// sourced file, a trap, xargs, go generate or go tool, a go test with an
// -exec program, and any program outside the pure, bounded, go and make
// commands. A direct go or make invocation, a pure or bounded command,
// `sh -c` over such commands and a find that runs nothing stay resolved.
func TestDriverUnfollowedCommand_NCMV1019(t *testing.T) {
	for _, tc := range []struct {
		command string
		open    bool
	}{
		{"go test ./tool", false},
		{"go test -race -count=1 ./tool/...", false},
		{"go vet ./tool", false},
		{"go run ./tool", false},
		{"make check", false},
		{"echo done", false},
		{"mkdir -p out", false},
		{"cat go.mod", false},
		{`sh -c "go test ./tool"`, false},
		{"find . -name x.go", false},
		{"script/check.sh --race", true},
		{"./script/check.sh", true},
		{"bin/tool run", true},
		{"bash script/check.sh", true},
		{"python3 tools/run.py", true},
		{". script/env.sh", true},
		{"trap 'go test ./tool' EXIT", true},
		{"echo ./tool | xargs go test", true},
		{"go generate ./tool", true},
		{"go tool cover -html=c.out", true},
		{"go test -exec script/wrap.sh ./tool", true},
		{"gotestsum -- ./tool", true},
		{"git status", true},
		{"find . -name x.go -exec go test {} ;", true},
		{`sh -c "script/check.sh"`, true},
	} {
		t.Run(tc.command, func(t *testing.T) {
			scan := newDriverScan()
			scan.scanShell(tc.command, ".", "example.test/m")
			if scan.runOpen != tc.open {
				t.Fatalf("run set open %v, want %v", scan.runOpen, tc.open)
			}
		})
	}
	// The reviewer's case: check's recipe runs a script that runs
	// `go test ./tool`, which the reach would otherwise not select.
	file := "check:\n\tscript/check.sh --race\nplain:\n\tgo test -race ./tool\n"
	root := makeReachRepository(t, map[string]string{
		"Makefile":        file,
		"script/check.sh": "#!/bin/sh\ngo test ./tool\n",
		"tool/tool.go":    "package tool\n",
	})
	ctx := context.Background()
	if reach := ReachMakeTargets(ctx, root, "Makefile", []string{"check"}); !strings.Contains(reach.Reason, `make target "check" runs a command or make goal that does not resolve`) {
		t.Fatalf("check: paths %v reason %q, want it unresolved", reach.Paths, reach.Reason)
	}
	if reach := ReachMakeTargets(ctx, root, "Makefile", []string{"plain"}); reach.Reason != "" || !slices.Contains(reach.Paths, "tool") {
		t.Fatalf("plain: paths %v reason %q, want it bounded to tool", reach.Paths, reach.Reason)
	}
	// The planner's graph is unchanged: a followed-or-not mark alone makes
	// no computed site.
	units, err := Driver{}.Units(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range units.Units {
		if unit.ID == "driver:make:Makefile#plain" && slices.Contains(unit.Frontier, FrontierDriverComputedInvocation) {
			t.Fatalf("plain frontier %v: the mark made a computed site", unit.Frontier)
		}
	}
}
