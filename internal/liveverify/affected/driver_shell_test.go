// SPDX-License-Identifier: Apache-2.0

package affected

import "testing"

// AZU-V0-001: truncated wrapper operands must retain uncertainty instead
// of panicking or silently claiming the command is fully understood.
func TestDriverTruncatedWrapperOptions_AZU_V0_001(t *testing.T) {
	for _, command := range []string{
		"env -u", "env -C", "env -S", "env NAME=value -u",
		"nice -n", "timeout -k", "xargs -I", "ionice -c", "taskset -c",
		"env -u NAME nice -n",
	} {
		t.Run(command, func(t *testing.T) {
			scan := newDriverScan()
			scan.scanShell(command, ".", "example.test/m")
			if !scan.computed {
				t.Fatal("malformed wrapper lost uncertainty")
			}
		})
	}
	for _, command := range []string{"env -u NAME go test ./core/...", "nice -n 10 go test ./core/..."} {
		t.Run(command, func(t *testing.T) {
			scan := newDriverScan()
			scan.scanShell(command, ".", "example.test/m")
			if scan.computed || !scan.paths["core/..."] {
				t.Fatalf("valid wrapper failed: %+v", scan)
			}
		})
	}
}

// AZU-V0-001: implicit packages honor cwd, and unresolved directories
// import every owner rather than claiming an empty dependency set. The
// build-only commands record them as build path imports (AFP-V0-2951), every
// other command as path imports.
func TestDriverImplicitGoPackageDirectory_AZU_V0_001(t *testing.T) {
	for _, command := range []string{"test", "build", "vet", "install", "generate", "list", "fix"} {
		for _, flags := range []string{"", " -race", " -tags custom", " -args -custom"} {
			for _, directory := range []string{".", "core", ""} {
				scan := newDriverScan()
				scan.scanShell("go "+command+flags, directory, "example.test/m")
				pattern := directory
				if directory == "" || directory == "." {
					pattern = "..."
				}
				recorded, other := scan.paths, scan.buildPaths
				if command == "vet" || command == "list" {
					recorded, other = other, recorded
				}
				if !recorded[pattern] || other[pattern] {
					t.Errorf("go %s%s cwd=%q lost package %q or recorded it in the wrong edge class: %+v", command, flags, directory, pattern, scan)
				}
			}
		}
	}
}

// AFP-V0-2951: a build-only command's package patterns are build path
// imports only; its flag values, a pattern it cannot resolve, and every word
// of any other command stay path imports.
func TestDriverBuildOnlyPatterns_AFPV02951(t *testing.T) {
	for _, tc := range []struct {
		script      string
		paths       []string
		buildPaths  []string
		absentPaths []string
	}{
		{script: "go vet ./cmd/tool", buildPaths: []string{"cmd/tool"}, absentPaths: []string{"cmd/tool"}},
		{script: "go list -f '{{.Dir}}' ./...", buildPaths: []string{"..."}, absentPaths: []string{"..."}},
		{script: "go -C sub vet ./x", buildPaths: []string{"sub/x"}},
		{script: "go vet -vettool ./bin/vt ./cmd/tool", paths: []string{"bin/vt"}, buildPaths: []string{"cmd/tool"}, absentPaths: []string{"cmd/tool"}},
		{script: "go vet internal/x", paths: []string{"internal/x"}},
		{script: "go vet ./cmd/tool && go run ./cmd/tool", paths: []string{"cmd/tool"}, buildPaths: []string{"cmd/tool"}},
		{script: "go build -o bin/tool ./cmd/tool", paths: []string{"cmd/tool"}},
		{script: "go test ./cmd/tool", paths: []string{"cmd/tool"}},
	} {
		scan := newDriverScan()
		scan.scanShell(tc.script, ".", "example.test/m")
		for _, value := range tc.paths {
			if !scan.paths[value] {
				t.Errorf("%q: path import %q missing: %+v", tc.script, value, scan)
			}
		}
		for _, value := range tc.buildPaths {
			if !scan.buildPaths[value] {
				t.Errorf("%q: build path import %q missing: %+v", tc.script, value, scan)
			}
		}
		for _, value := range tc.absentPaths {
			if scan.paths[value] {
				t.Errorf("%q: a build-only pattern %q is also a path import: %+v", tc.script, value, scan)
			}
		}
	}
}

// AFP-V0-2956 (V1-4283, review fix 2): a build-only command whose standard
// output feeds another command (a pipe, a command or process substitution)
// records its package patterns as path imports, since the consumer may run
// what it lists. AFP-V0-2957: a go package command that xargs, parallel or
// find -exec runs takes arguments the scanner cannot read, so it imports
// every owner; a make call it runs is computed. The review's case is the
// first: `go list ./lib | xargs -I PKG go test PKG`.
func TestDriverListingOutputKeepsExecution_AFPV02956(t *testing.T) {
	for _, tc := range []struct {
		script   string
		paths    []string
		computed bool
	}{
		{script: "go list ./lib | xargs -I PKG go test PKG", paths: []string{"lib", "..."}},
		{script: "go list ./lib |& xargs go test", paths: []string{"lib", "..."}},
		{script: "cd sub && go list ../lib | xargs go test", paths: []string{"lib", "..."}},
		{script: "go test $(go list ./lib)", paths: []string{"lib", "..."}},
		{script: "go test \"$(go list ./lib)\"", paths: []string{"lib", "..."}},
		{script: "for p in `go list ./lib`; do go test \"$p\"; done", paths: []string{"lib", "..."}},
		{script: "go list ./lib | while read -r p; do go test \"$p\"; done", paths: []string{"lib", "..."}},
		{script: "while read -r p; do go test \"$p\"; done < <(go list ./lib)", paths: []string{"lib", "..."}},
		{script: "go vet ./lib 2>&1 | tee vet.log", paths: []string{"lib"}},
		{script: "sh -c 'go list ./lib' | xargs go test", paths: []string{"lib", "..."}},
		{script: "xargs -a pkgs.txt go test", paths: []string{"..."}},
		{script: "parallel go test ::: a b", paths: []string{"..."}},
		{script: "find . -name '*_test.go' -execdir go test \\;", paths: []string{"..."}},
		{script: "xargs -n1 go vet < pkgs.txt", computed: false},
		{script: "go list ./lib | xargs -I T make T", computed: false},
	} {
		scan := newDriverScan()
		scan.scanShell(tc.script, ".", "example.test/m")
		for _, value := range tc.paths {
			if !scan.paths[value] {
				t.Errorf("%q: path import %q missing (paths %v, build paths %v)", tc.script, value, scan.paths, scan.buildPaths)
			}
		}
	}
	// A vet run that xargs feeds keeps the build-only class, now over every
	// owner, and a make call it feeds is computed.
	scan := newDriverScan()
	scan.scanShell("xargs -n1 go vet < pkgs.txt", ".", "example.test/m")
	if !scan.buildPaths["..."] || scan.paths["..."] {
		t.Errorf("xargs go vet: want the every-owner build pattern only, got paths %v build paths %v", scan.paths, scan.buildPaths)
	}
	scan = newDriverScan()
	scan.scanShell("go list ./lib | xargs -I T make T", ".", "example.test/m")
	if len(scan.makes) != 1 || !scan.makes[0].Computed {
		t.Errorf("a make call xargs runs is not computed: %+v", scan.makes)
	}
	// Controls: output that feeds no command keeps the build-only edge.
	for _, script := range []string{"go list ./lib", "go list ./lib > pkgs.txt", "go list ./lib || true", "go vet ./lib && go list ./x"} {
		scan := newDriverScan()
		scan.scanShell(script, ".", "example.test/m")
		if scan.paths["lib"] || !scan.buildPaths["lib"] {
			t.Errorf("%q: want a build-only edge to lib, got paths %v build paths %v", script, scan.paths, scan.buildPaths)
		}
	}
}

// AFP-V0-2957, AFP-V0-2958: an interpreter whose program or script operand
// arrives on standard input or through a feeder's arguments is a computed
// invocation: `| sh`, a heredoc-fed shell (the scanner skips the body),
// `bash -s`, and an interpreter or a command word holding the replacement
// string that xargs, parallel or find -exec runs. A shell running a named
// script or an inline program is not.
func TestDriverStdinProgramIsComputed_AFPV02958(t *testing.T) {
	for _, script := range []string{
		"echo 'go test ./lib' | sh",
		"bash <<'EOF'\ngo test ./lib\nEOF",
		"bash -s -- arg < run.txt",
		"ls scripts | xargs -n1 bash",
		"ls scripts | xargs -I F bash F",
		"ls bin | xargs -I CMD CMD --check",
		"find . -name run -exec {} \\;",
		"parallel {} ::: ./a ./b",
	} {
		scan := newDriverScan()
		scan.scanShell(script, ".", "example.test/m")
		if !scan.computed {
			t.Errorf("%q: a program fed through standard input or a feeder is not a computed invocation", script)
		}
	}
	for _, script := range []string{"bash script/x.sh", "sh -c 'go test ./lib'", "bash -eu script/x.sh", "go list ./lib | xargs go test"} {
		scan := newDriverScan()
		scan.scanShell(script, ".", "example.test/m")
		if scan.computed {
			t.Errorf("%q: a named script or inline program became a computed invocation", script)
		}
	}
}
