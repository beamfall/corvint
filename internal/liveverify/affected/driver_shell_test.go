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
