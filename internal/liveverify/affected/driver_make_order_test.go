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

// TestDriverUnfollowedCommand_NCMV1019 pins the closed rule of which recipe
// commands the reach follows: make, go whose first argument is test, vet,
// build or list with no -exec, -toolexec or -vettool program, and the
// builtins echo, printf, true, false, :, test, [, cd, exit, and set with -e,
// -u, -x and -o pipefail only. Every other command, each program the round
// 3 allowlist called pure or bounded, sh -c and find included, leaves the
// run set open with a reason naming it, as does any command substitution.
func TestDriverUnfollowedCommand_NCMV1019(t *testing.T) {
	for _, command := range []string{
		"go test ./tool",
		"go test -race -count=1 ./tool/...",
		"go vet ./tool",
		"go build ./tool",
		"go list ./tool",
		"make check",
		"gmake check",
		"echo done",
		"printf '%s\\n' done",
		"true",
		"false",
		":",
		"test -f go.mod",
		"[ -f go.mod ]",
		"cd tool && go test ./b",
		"exit 0",
		"set -e",
		"set -eux",
		"set -euo pipefail",
		"set +x",
		"if test -f go.mod; then go test ./tool; fi",
	} {
		t.Run("followed/"+command, func(t *testing.T) {
			scan := newDriverScan()
			scan.scanShell(command, ".", "example.test/m")
			if scan.runOpen {
				t.Fatalf("run set open by %v, want it followed", driverSorted(scan.openBy))
			}
		})
	}
	type open struct{ command, reason string }
	cases := []open{
		// The review's two cases (round 4).
		{"go -C . generate ./gen", "shell:unfollowed-command:go -C"},
		{"sort -S 1 --compress-program=script/check.sh data/lines.txt", "shell:unfollowed-command:sort"},
		// go outside its four subcommands, or after a global flag.
		{"go", "shell:unfollowed-command:go"},
		{"go run ./tool", "shell:unfollowed-command:go run"},
		{"go generate ./tool", "shell:unfollowed-command:go generate"},
		{"go tool cover -html=c.out", "shell:unfollowed-command:go tool"},
		{"go install ./tool", "shell:unfollowed-command:go install"},
		{"go env GOPATH", "shell:unfollowed-command:go env"},
		{"go mod tidy", "shell:unfollowed-command:go mod"},
		{"go -C tool test ./b", "shell:unfollowed-command:go -C"},
		{"go -modfile=x.mod test ./tool", "shell:unfollowed-command:go -modfile=x.mod"},
		{"go test -exec script/wrap.sh ./tool", "shell:unfollowed-command:go test -exec"},
		{"go test --exec=script/wrap.sh ./tool", "shell:unfollowed-command:go test --exec=script/wrap.sh"},
		{"go build -toolexec=script/wrap.sh ./tool", "shell:unfollowed-command:go build -toolexec=script/wrap.sh"},
		{"go vet -vettool=bin/vet ./tool", "shell:unfollowed-command:go vet -vettool=bin/vet"},
		// Programs the round 3 allowlist followed whatever their options.
		{"sed -n 1p go.mod", "shell:unfollowed-command:sed"},
		{"awk '{print}' go.mod", "shell:unfollowed-command:awk"},
		{"find . -name x.go", "shell:unfollowed-command:find"},
		{`sh -c "go test ./tool"`, "shell:unfollowed-command:sh"},
		{`bash -c "echo hi"`, "shell:unfollowed-command:bash"},
		{"echo ./tool | xargs go test", "shell:unfollowed-command:xargs"},
		{"env A=1 go test ./tool", "shell:unfollowed-command:env"},
		{"nice go test ./tool", "shell:unfollowed-command:nice"},
		{"timeout 5 go test ./tool", "shell:unfollowed-command:timeout"},
		{"time go test ./tool", "shell:unfollowed-command:time"},
		{"exec go test ./tool", "shell:unfollowed-command:exec"},
		{"command -v go", "shell:unfollowed-command:command"},
		// Builtins outside the closed list, or set with another option.
		{"set -f", "shell:unfollowed-command:set"},
		{"set -v", "shell:unfollowed-command:set"},
		{"set -o posix", "shell:unfollowed-command:set"},
		{"export A=1", "shell:builtin-export"},
		{"read line", "shell:unfollowed-command:read"},
		{"shift", "shell:unfollowed-command:shift"},
		{"wait", "shell:unfollowed-command:wait"},
		{"pushd tool", "shell:unfollowed-command:pushd"},
		// A command substitution in any word, even of a followed command.
		{"echo $(echo ./tool)", "shell:command-substitution:echo"},
		{"cd $(echo tool) && go test ./b", "shell:command-substitution:cd"},
		{`echo "$(echo x)"`, "shell:command-substitution:echo"},
		{"echo \"`echo x`\"", "shell:command-substitution:echo"},
		{"echo `echo x`", "shell:command-substitution:`"},
		// Scripts, binaries, interpreters and deferred or sourced code.
		{"script/check.sh --race", "shell:unfollowed-command:script/check.sh"},
		{"./script/check.sh", "shell:unfollowed-command:./script/check.sh"},
		{"bin/tool run", "shell:unfollowed-command:bin/tool"},
		{"/usr/bin/go test ./tool", "shell:unfollowed-command:/usr/bin/go"},
		{"bash script/check.sh", "shell:unfollowed-command:bash"},
		{"python3 tools/run.py", "shell:unfollowed-command:python3"},
		{". script/env.sh", "shell:unfollowed-command:."},
		{"trap 'go test ./tool' EXIT", "shell:unfollowed-command:trap"},
		{"gotestsum -- ./tool", "shell:unfollowed-command:gotestsum"},
		{"git status", "shell:unfollowed-command:git"},
	}
	// Every program the round 3 allowlist called pure or bounded, except
	// the followed builtins echo, printf, true and false, is now open.
	for _, table := range []map[string]bool{driverPureCommands, driverBoundedCommands} {
		for _, name := range driverSorted(table) {
			if !driverFollowedBuiltins[name] {
				cases = append(cases, open{name + " go.mod", "shell:unfollowed-command:" + name})
			}
		}
	}
	for _, tc := range cases {
		t.Run("open/"+tc.command, func(t *testing.T) {
			scan := newDriverScan()
			scan.scanShell(tc.command, ".", "example.test/m")
			if !scan.runOpen {
				t.Fatalf("run set followed, want it open by %q", tc.reason)
			}
			if !scan.openBy[tc.reason] {
				t.Fatalf("open by %v, want a reason %q naming the command", driverSorted(scan.openBy), tc.reason)
			}
		})
	}
	// The reviewer's case: check's recipe runs a script that runs
	// `go test ./tool`, which the reach would otherwise not select.
	// The round 4 review's cases: a go global flag before generate, and a
	// bounded program's option that runs a script, each of which runs
	// `go test ./tool`; and make with a file the driver does not model.
	file := "check:\n\tscript/check.sh --race\nplain:\n\tgo test -race ./tool\n" +
		"gen:\n\tgo -C . generate ./gen\n" +
		"sorted:\n\tsort -S 1 --compress-program=script/check.sh data/lines.txt\n" +
		"nested:\n\t$(MAKE) plain\nsay:\n\techo hi\nsub:\n\tcd b && go test ./c\n" +
		"other:\n\t$(MAKE) -f missing.mk x\n"
	root := makeReachRepository(t, map[string]string{
		"Makefile":        file,
		"script/check.sh": "#!/bin/sh\ngo test ./tool\n",
		"tool/tool.go":    "package tool\n",
		"gen/gen.go":      "package gen\n\n//go:generate go test ./tool\n",
		"b/c/c.go":        "package c\n",
		"data/lines.txt":  "x\n",
	})
	ctx := context.Background()
	for target, command := range map[string]string{
		"check":  "shell:unfollowed-command:script/check.sh",
		"gen":    "shell:unfollowed-command:go -C",
		"sorted": "shell:unfollowed-command:sort",
		"other":  "make:unmodelled-file:missing.mk",
	} {
		want := `make target "` + target + `" runs a command or make goal that does not resolve: ` + command
		if reach := ReachMakeTargets(ctx, root, "Makefile", []string{target}); reach.Reason != want {
			t.Fatalf("%s: paths %v reason %q, want %q", target, reach.Paths, reach.Reason, want)
		}
	}
	for target, path := range map[string]string{"plain": "tool", "nested": "tool", "say": "", "sub": "b/c"} {
		reach := ReachMakeTargets(ctx, root, "Makefile", []string{target})
		if reach.Reason != "" || path != "" && !slices.Contains(reach.Paths, path) {
			t.Fatalf("%s: paths %v reason %q, want it bounded to %q", target, reach.Paths, reach.Reason, path)
		}
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
