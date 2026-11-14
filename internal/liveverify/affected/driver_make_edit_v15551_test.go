// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// TestReachMakeRecipeShell_NCMV1019 pins V1-5551 in the reach: a recipe
// make hands to a SHELL the scan does not recognize, set for the file or for
// the target, or with .SHELLFLAGS it does not model, leaves the target's run
// set open with the recipe-shell reason, while a plain shell keeps it
// bounded. The planner's site stays the evaluated, uncertified one it was.
func TestReachMakeRecipeShell_NCMV1019(t *testing.T) {
	ctx := context.Background()
	open := `make target "check" runs a command or make goal that does not resolve: make:recipe-shell-uncertified`
	for _, tc := range []struct {
		name, makefile, reason string
	}{
		{"wrapper", "SHELL = ./tools/test-shell\ncheck:\n\techo hi\n", open},
		{"target-specific", "check: SHELL = ./tools/test-shell\ncheck:\n\techo hi\n", open},
		{"shell flags", ".SHELLFLAGS = -x script/rc -c\ncheck:\n\techo hi\n", open},
		{"plain shell", "SHELL = /bin/bash\ncheck:\n\techo hi\n", ""},
		{"default shell", "check:\n\techo hi\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := makeReachRepository(t, map[string]string{"Makefile": tc.makefile, "tools/test-shell": "#!/bin/sh\ngo test ./tool\n"})
			reach := ReachMakeTargets(ctx, root, "Makefile", []string{"check"})
			if reach.Reason != tc.reason {
				t.Fatalf("reason %q, want %q", reach.Reason, tc.reason)
			}
		})
	}
}

// TestReachMakeChdir_NCMV1019 pins V1-5552 in the reach: cd with `--`, -L
// or -P (alone or combined) changes to its operand, so the go selector after
// it names that directory's package; any other option, no operand, `-`, or
// a second operand leaves cd unfollowed, and so does pushd, which the closed
// rule never follows, whatever its arguments.
func TestReachMakeChdir_NCMV1019(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		cd, path, reason string
	}{
		{"cd -- tool", "tool/b", ""},
		{"cd -L tool", "tool/b", ""},
		{"cd -P tool", "tool/b", ""},
		{"cd -LP -- tool", "tool/b", ""},
		{"cd -- -tool", "-tool/b", ""},
		{"cd tool", "tool/b", ""},
		{"cd -e tool", "", "shell:unfollowed-command:cd"},
		{"cd -@ tool", "", "shell:unfollowed-command:cd"},
		{"cd", "", "shell:unfollowed-command:cd"},
		{"cd -", "", "shell:unfollowed-command:cd"},
		{"cd -- -", "", "shell:unfollowed-command:cd"},
		{"cd tool other", "", "shell:unfollowed-command:cd"},
		{"cd -- ''", "", "shell:unfollowed-command:cd"},
		{"pushd -- tool", "", "shell:unfollowed-command:pushd"},
	} {
		t.Run(tc.cd, func(t *testing.T) {
			root := makeReachRepository(t, map[string]string{"Makefile": "t:\n\t" + tc.cd + " && go test ./b\n"})
			reach := ReachMakeTargets(ctx, root, "Makefile", []string{"t"})
			if tc.reason != "" {
				if !strings.Contains(reach.Reason, tc.reason) {
					t.Fatalf("reason %q, want %q", reach.Reason, tc.reason)
				}
				return
			}
			if reach.Reason != "" {
				t.Fatalf("not bounded: %s", reach.Reason)
			}
			if !slices.Contains(reach.Paths, tc.path) || slices.ContainsFunc(reach.Paths, func(p string) bool { return strings.HasPrefix(p, "--") }) {
				t.Fatalf("paths %v, want %q and no `--` directory", reach.Paths, tc.path)
			}
			if want := []MakeSelectors{{{tc.path}}}; !slices.EqualFunc(reach.Selectors, want, func(a, b MakeSelectors) bool {
				return slices.EqualFunc(a, b, slices.Equal[[]string])
			}) {
				t.Fatalf("selectors %v, want %v", reach.Selectors, want)
			}
		})
	}
}

// TestDriverChdirPlanner_AFPV09051 pins that the planner reads cd's options
// the same way (V1-5552): `cd -- tool` records tool/b, never `--/b`, and a
// cd whose directory the scan cannot name leaves the directory unknown, so
// the go selector after it keeps the whole tree and the text is uncertified.
func TestDriverChdirPlanner_AFPV09051(t *testing.T) {
	for _, tc := range []struct {
		script, path string
		uncertified  bool
	}{
		{"cd -- tool && go test ./b", "tool/b", false},
		{"cd -e tool && go test ./b", "...", true},
		{"cd && go test ./b", "...", true},
		{"pushd +1 && go test ./b", "...", true},
	} {
		t.Run(tc.script, func(t *testing.T) {
			root := makeReachRepository(t, map[string]string{"tool/b/b.go": "package b\n"})
			scan := newDriverScan()
			scan.source = DiskSourceContext(context.Background(), root)
			scan.scanShell(tc.script, ".", "example.com/m")
			if !scan.paths[tc.path] {
				t.Fatalf("paths %v lack %q", scan.paths, tc.path)
			}
			for path := range scan.paths {
				if strings.HasPrefix(path, "--") {
					t.Fatalf("paths %v hold a `--` directory", scan.paths)
				}
			}
			if got := scan.uncertifiedBy["shell:cd-unresolved"]; got != tc.uncertified {
				t.Fatalf("cd-unresolved %v, want %v (uncertified by %v)", got, tc.uncertified, scan.uncertifiedBy)
			}
		})
	}
}

// TestReachMakeSelectorPasses_NCMV1013 pins the selector sets of a nested
// make file: the driver scans its recipe from its own directory and from the
// root, one set each, which the selector's missing-package check reads as
// alternatives (V1-5552).
func TestReachMakeSelectorPasses_NCMV1013(t *testing.T) {
	root := makeReachRepository(t, map[string]string{"Makefile": "all:\n\techo\n", "sub/Makefile": "check:\n\tgo test ./x ./...\n"})
	reach := ReachMakeTargets(context.Background(), root, "sub/Makefile", []string{"check"})
	if reach.Reason != "" {
		t.Fatalf("not bounded: %s", reach.Reason)
	}
	want := []MakeSelectors{{{"sub/...", "sub/x"}, {"x"}}}
	if !slices.EqualFunc(reach.Selectors, want, func(a, b MakeSelectors) bool { return slices.EqualFunc(a, b, slices.Equal[[]string]) }) {
		t.Fatalf("selectors %v, want %v", reach.Selectors, want)
	}
}

// TestReachMakeInvocationCommandLine_NCMV1019 pins the recursive make
// invocation's command line in the reach: a variable assignment and every
// option outside the closed harmless list leave the invocation unfollowed;
// the listed options, -C and -f stay bounded.
func TestReachMakeInvocationCommandLine_NCMV1019(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		call, reason string
	}{
		{"make SHELL=./tools/test-shell check", "make:command-line-assignment:SHELL"},
		{"make PKG=./tool check", "make:command-line-assignment:PKG"},
		{"make check PKG+=./tool", "make:command-line-assignment:PKG+"},
		{"make '--eval=x: ; go test ./tool' x", "make:unfollowed-option:--eval"},
		{"make --eval 'x: ; go test ./tool' x", "make:unfollowed-option:--eval"},
		{"make -e check", "make:unfollowed-option:-e"},
		{"make -n check", "make:unfollowed-option:-n"},
		{"make -I inc check", "make:unfollowed-option:-I"},
		{"make -o check check", "make:unfollowed-option:-o"},
		{"make -se check", "make:unfollowed-option:-se"},
		{"make --what-if=x check", "make:unfollowed-option:--what-if"},
		{"make -- SHELL=./tools/test-shell check", "make:command-line-assignment:SHELL"},
		{"make check", ""},
		{"make -s check", ""},
		{"make -ks check", ""},
		{"make --silent --quiet --keep-going --no-keep-going -S --stop check", ""},
		{"make --no-print-directory -w --print-directory check", ""},
		{"make -j 4 -j8 --jobs=2 --jobs 3 -j check", ""},
		{"make -l 2 -l2.5 --load-average=3 --max-load 4 check", ""},
		{"make -O -Oline --output-sync --output-sync=target check", ""},
		{"make -C . -f Makefile check", ""},
		{"make -- check", ""},
	} {
		t.Run(tc.call, func(t *testing.T) {
			root := makeReachRepository(t, map[string]string{"Makefile": "top:\n\t" + tc.call + "\ncheck:\n\techo hi\n", "tools/test-shell": "#!/bin/sh\ngo test ./tool\n"})
			reach := ReachMakeTargets(ctx, root, "Makefile", []string{"top"})
			if tc.reason == "" {
				if reach.Reason != "" {
					t.Fatalf("not bounded: %s", reach.Reason)
				}
				return
			}
			if want := `make target "top" runs a command or make goal that does not resolve: ` + tc.reason; reach.Reason != want {
				t.Fatalf("reason %q, want %q", reach.Reason, want)
			}
		})
	}
}

// TestDriverMakeAssignmentPath_NCMV1019 pins that the planner records no
// command-line assignment of a make invocation as a path: the word is not a
// file, and the scan still records the call itself.
func TestDriverMakeAssignmentPath_NCMV1019(t *testing.T) {
	scan := newDriverScan()
	scan.scanShell("make SHELL=./tools/test-shell PKG=./tool check", ".", "example.com/m")
	for path := range scan.paths {
		if strings.Contains(path, "=") {
			t.Fatalf("paths %v hold an assignment word", scan.paths)
		}
	}
	if len(scan.makes) != 1 || !slices.Equal(scan.makes[0].Targets, []string{"check"}) {
		t.Fatalf("makes %+v, want one call of check", scan.makes)
	}
	if !scan.openBy["make:command-line-assignment:SHELL"] || !scan.openBy["make:command-line-assignment:PKG"] {
		t.Fatalf("open by %v, want both assignments", scan.openBy)
	}
}

// TestDriverSubshellChdir_NCMV1019 pins the working directory of a subshell:
// a cd inside `( ... )` ends at its closing parenthesis, a cd inside a
// `{ ...; }` group does not, and a closing parenthesis the scan cannot pair
// leaves the directory unknown and the text uncertified.
func TestDriverSubshellChdir_NCMV1019(t *testing.T) {
	for _, tc := range []struct {
		script, path, absent string
		unpaired             bool
	}{
		{"(cd tool && go vet .); go test ./c", "c", "tool/c", false},
		{"(cd tool && go vet .)\ngo test ./c", "c", "tool/c", false},
		{"(cd tool; (cd c && go vet .); go test ./d); go test ./c", "tool/d", "tool/c/d", false},
		{"(cd tool; (cd c && go vet .); go test ./d); go test ./c", "c", "tool/c", false},
		{"{ cd tool && go vet .; }; go test ./c", "tool/c", "", false},
		{"cd tool ); go test ./c", "...", "tool/c", true},
	} {
		t.Run(tc.script, func(t *testing.T) {
			root := makeReachRepository(t, map[string]string{"tool/c/c.go": "package c\n", "c/c.go": "package c\n"})
			scan := newDriverScan()
			scan.source = DiskSourceContext(context.Background(), root)
			scan.scanShell(tc.script, ".", "example.com/m")
			if !scan.paths[tc.path] {
				t.Fatalf("paths %v lack %q", scan.paths, tc.path)
			}
			if tc.absent != "" && scan.paths[tc.absent] {
				t.Fatalf("paths %v hold %q", scan.paths, tc.absent)
			}
			if got := scan.uncertifiedBy["shell:unpaired-group"]; got != tc.unpaired {
				t.Fatalf("unpaired-group %v, want %v (uncertified by %v)", got, tc.unpaired, scan.uncertifiedBy)
			}
		})
	}
}
