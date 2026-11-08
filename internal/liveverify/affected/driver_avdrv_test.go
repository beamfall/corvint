// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

// These fixtures return declared units and never read ambient files.
type driverFixtureLanguage struct{ fake }

func (language driverFixtureLanguage) UnitsSource(*Source) (Result, error) {
	return language.Units(context.Background(), "")
}

// Before AFP-V0-9400 only a leading interpreter was retained, even though
// a wrapper can pass the skipped body to one.
func TestDriverHeredocConsumers_AFPV09400(t *testing.T) {
	for _, script := range []string{
		"docker exec -i c sh <<'EOF'\ngo test ./lib\nEOF",
		"docker exec -i c python3 <<'EOF'\nimport subprocess\nsubprocess.run(['go', 'test', './lib'])\nEOF",
		"wrapper <<'EOF'\ngo test ./lib\nEOF",
		"cat <<'EOF' > generated.sh\ngo test ./lib\nEOF\nsh generated.sh",
		"cat <<EOF\n$(go test ./lib)\nEOF",
		// V1-5034's exact nested case is already computed on the base;
		// the wrapper variant was not. Both must remain conservative.
		"sh <<'A' | sh\ncat <<'X'\ngo test ./lib\nX\nA",
		"docker exec -i c sh <<'A' | wrapper\ncat <<'X'\ngo test ./lib\nX\nA",
		"docker exec -i c sh <<'EOF'\ngo test ./lib", // Unterminated body.
	} {
		t.Run(script, func(t *testing.T) {
			scan := newDriverScan()
			scan.scanShell(script, ".", "example.test/m")
			if !scan.computed {
				t.Fatalf("skipped executable input has no computed fallback: %+v", scan)
			}
			// Exercise the workflow parser and selection, not only the
			// scanner flag: an unrelated/unindexed dirty path must reach
			// the job that may run the hidden program.
			workflow := "jobs:\n  check:\n    steps:\n      - run: |\n          " + strings.ReplaceAll(script, "\n", "\n          ") + "\n"
			graph, err := BuildFS(fstest.MapFS{".github/workflows/ci.yml": {Data: []byte(workflow)}}, Driver{})
			if err != nil {
				t.Fatal(err)
			}
			const id = "driver:ci:.github/workflows/ci.yml#check"
			selected := witnesses(Select(graph, []string{"unindexed/new.fixture"}))
			if selected[id].Kind != WitnessFrontierSite {
				t.Fatalf("hidden program's job lost its fallback: %+v", selected)
			}
		})
	}
	// Tabs are valid shell heredoc input but outside the workflow parser's
	// YAML subset. Exercise the scanner directly for the <<- spelling.
	scan := newDriverScan()
	scan.scanShell("docker exec -i c sh <<-'EOF'\n\tgo test ./lib\n\tEOF", ".", "example.test/m")
	if !scan.computed {
		t.Fatal("tab-stripped body lost its computed fallback")
	}
	for _, script := range []string{"echo '<<EOF'", "sh -c 'go test ./lib'"} {
		scan := newDriverScan()
		scan.scanShell(script, ".", "example.test/m")
		if scan.computed {
			t.Errorf("literal text or a modelled inline program became computed: %q", script)
		}
	}
}

func driverEnumerationFiles(recipe string) fstest.MapFS {
	return fstest.MapFS{
		"go.mod":                   {Data: []byte("module example.test/m\n\ngo 1.27.0\n")},
		"Makefile":                 {Data: []byte("check:\n\t" + recipe + "\nall: check\nother:\n\techo done\n")},
		".github/workflows/ci.yml": {Data: []byte("jobs:\n  caller:\n    steps:\n      - run: make all\n  direct:\n    steps:\n      - run: " + strings.ReplaceAll(recipe, "$$", "$") + "\n  other:\n    steps:\n      - run: echo done\n")},
		"owned.g":                  {Data: []byte("source")},
		"owned_test.g":             {Data: []byte("test")},
	}
}

// Every dirty path includes its new/deleted membership, not only the paths
// present while building the graph. A tested owner must not terminate the
// driver read before its make/workflow callers (AFP-V0-9402).
func TestDriverEnumerationSelectsFilesAndCallers_AFPV09401(t *testing.T) {
	for _, recipe := range []string{
		"git ls-files -z", "git -C sub ls-files", "git grep pattern",
		"find . -type f", "find ./sub -name '*.txt'", "find -L link -type f",
		"rg --files", "rg pattern .", "rg --files $$ROOT",
		"cd $$WHERE && rg --files",
		"go run ./tools/check", "go -C sub run ./tools/check",
		"go -C $$WHERE run ./tools/check",
		"go run tools/check.go", "go run ./tools/check --root=$$ROOT",
		"env go run ./tools/check", "sh -c 'git ls-files'",
	} {
		t.Run(recipe, func(t *testing.T) {
			owner := fake{name: "g", units: []Unit{{ID: "g:owned", Sources: []string{"owned.g"}, Tests: []string{"owned_test.g"}}}}
			graph, err := BuildFS(driverEnumerationFiles(recipe), driverFixtureLanguage{owner}, Driver{})
			if err != nil {
				t.Fatal(err)
			}
			for _, dirty := range []string{"new.fixture", "removed/file.txt", "unindexed/new.go", ".corvint/changes/new.cem.json", "owned.g", "owned_test.g"} {
				plan := Select(graph, []string{dirty})
				selected := witnesses(plan)
				for _, id := range []string{"driver:make:Makefile#check", "driver:make:Makefile#all", "driver:ci:.github/workflows/ci.yml#caller", "driver:ci:.github/workflows/ci.yml#direct"} {
					if _, found := selected[id]; !found {
						t.Errorf("%s lost %s: %+v", dirty, id, plan)
					}
				}
				for _, id := range []string{"driver:make:Makefile#check", "driver:ci:.github/workflows/ci.yml#direct"} {
					unit, _ := graph.Unit(id)
					if !slices.Contains(unit.PathGlobs, "/*") || len(unit.Frontier) != 0 {
						t.Errorf("%s needs a file-set read edge: %+v", id, unit)
					}
					// An unresolved go -C also imports every existing owner;
					// that earlier dependency witness retains precedence. The
					// absent/unowned paths must use the new file-set edge.
					if dirty != "owned.g" && dirty != "owned_test.g" && (selected[id].Kind != WitnessPathGlobReader || selected[id].Glob != "/*") {
						t.Errorf("%s needs a glob witness on %s: %+v", id, dirty, selected[id])
					}
				}
				for _, id := range []string{"driver:make:Makefile#other", "driver:ci:.github/workflows/ci.yml#other"} {
					if _, found := selected[id]; found {
						t.Errorf("%s selected unrelated driver %s", dirty, id)
					}
				}
			}
			if got := Select(graph, nil); len(got.Selected) != 0 {
				t.Errorf("an empty change selected a file-set reader: %+v", got)
			}
		})
	}
}

func TestDriverEnumerationKeepsComputedFallback_AFPV09402(t *testing.T) {
	for _, recipe := range []string{
		"$(if $(CI),git ls-files,find .)",
		"$(if $(CI),go run ./tools/check,go run ./tools/other)",
		"$$RUNNER $(if $(CI),a,b)",
	} {
		graph, err := BuildFS(driverEnumerationFiles(recipe), Driver{})
		if err != nil {
			t.Fatal(err)
		}
		const id = "driver:make:Makefile#check"
		unit, _ := graph.Unit(id)
		if !slices.Contains(unit.Frontier, FrontierDriverComputedInvocation) {
			t.Fatalf("computed invocation was certified as bounded: %+v", unit)
		}
		if selected := witnesses(Select(graph, []string{"new.fixture"})); selected[id].Kind != WitnessFrontierSite {
			t.Errorf("existing fallback witness changed: %+v", selected)
		}
	}
}

func TestDriverEnumerationDoesNotInventReaders_AFPV09401(t *testing.T) {
	for _, recipe := range []string{"go vet ./lib", "go test ./lib", "go build ./tools/check", "echo rg", "printf find", "echo 'git ls-files'"} {
		graph, err := BuildFS(driverEnumerationFiles(recipe), Driver{})
		if err != nil {
			t.Fatal(err)
		}
		selected := witnesses(Select(graph, []string{"new.fixture"}))
		if _, found := selected["driver:make:Makefile#check"]; found {
			t.Errorf("%q invented a file-set read: %+v", recipe, selected)
		}
	}
}

// A symlink's target can be outside the literal scan directory. The tree
// edge must keep that target even when the scanner never indexes the link.
func TestDriverEnumerationSymlinkTargets_AFPV09401(t *testing.T) {
	for _, directory := range []bool{false, true} {
		t.Run(map[bool]string{false: "file", true: "directory"}[directory], func(t *testing.T) {
			root := t.TempDir()
			for name, file := range driverEnumerationFiles("rg --follow pattern link") {
				full := filepath.Join(root, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, file.Data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			target := "target.txt"
			if directory {
				target = "target"
				if err := os.Mkdir(filepath.Join(root, target), 0o755); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(filepath.Join(root, target), []byte("data"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(root, "link")); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			graph, err := Build(context.Background(), root, Driver{})
			if err != nil {
				t.Fatal(err)
			}
			if directory {
				target += "/new.txt"
			}
			selected := witnesses(Select(graph, []string{target}))
			if selected["driver:make:Makefile#check"].Kind != WitnessPathGlobReader {
				t.Errorf("symlink target %s lost its enumerating reader: %+v", target, selected)
			}
		})
	}
}

// A directory edge proves only regular glob matches, never their linked targets.
func TestDriverGlobProvenance_AFPV09051(t *testing.T) {
	for _, tc := range []struct{ name, operand, link string }{
		{"linked-file", "data/*", "../lib/lib.go"},
		{"linked-directory", "data/*", "../lib"},
		{"unknown-glob-form", "data/?*", ""},
		{"plain-directory-match", "data/*", "directory"},
		{"regular-control", "data/*", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			files := map[string]string{
				"Makefile":       "check:; $(if yes,cat,cat) " + tc.operand + "\n",
				"data/plain.txt": "plain", "lib/lib.go": "package lib\n",
			}
			for name, body := range files {
				full := filepath.Join(root, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if tc.link == "directory" {
				if err := os.Mkdir(filepath.Join(root, "data", "sub"), 0o755); err != nil {
					t.Fatal(err)
				}
			} else if tc.link != "" {
				if err := os.Symlink(tc.link, filepath.Join(root, "data", "alias.txt")); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			graph, err := Build(t.Context(), root, Driver{})
			if err != nil {
				t.Fatal(err)
			}
			const id = "driver:make:Makefile#check"
			selected := witnesses(Select(graph, []string{"lib/lib.go"}))
			if tc.name == "regular-control" {
				if _, found := selected[id]; found {
					t.Fatalf("regular glob overselected: %+v", selected)
				}
			} else if selected[id].Kind != WitnessFrontierSite {
				t.Fatalf("unproven glob lost its frontier: %+v", selected)
			}
		})
	}
}

func TestDriverNestedWildcardDirectory_AFPV03121(t *testing.T) {
	for _, function := range []string{"wildcard marker", "future-filesystem-function marker"} {
		t.Run(function, func(t *testing.T) {
			files := fstest.MapFS{
				"build/Makefile": {Data: []byte("check:; go test ./$(if $(" + function + "),left,right)_pkg\n")},
				"build/marker":   {Data: []byte("present only beside Makefile")},
			}
			// Exercise the reported path through the builder directly:
			// the generic discovery walk excludes build/ independently.
			parsed := parseDriverMake("build/Makefile", files["build/Makefile"].Data)
			parsed.source = FSSource(files)
			registry := driverRegistry{source: parsed.source}
			units := registry.makeUnits(parsed, files["build/Makefile"].Data)
			graph, err := BuildFS(files, driverFixtureLanguage{fake{name: DriverNamespace, units: units, owns: DriverInput}})
			if err != nil {
				t.Fatal(err)
			}
			for _, dirty := range []string{"left_pkg/pkg.go", "right_pkg/pkg.go", "unknown/new.fixture"} {
				selected := witnesses(Select(graph, []string{dirty}))
				if selected["driver:make:build/Makefile#check"].Kind != WitnessFrontierSite {
					t.Fatalf("%s lost unknown invocation-directory claim: %+v", dirty, selected)
				}
			}
			expander := parsed.expander(nil)
			values := expander.expandAll("go test ./$(if $(" + function + "),left,right)_pkg")
			for _, want := range []string{"go test ./left_pkg", "go test ./right_pkg"} {
				if !slices.Contains(values, want) {
					t.Fatalf("missing alternative %q: %v", want, values)
				}
			}
		})
	}
}

func TestDriverExecutableProvenance_AFPV03128(t *testing.T) {
	for _, head := range []string{"/repo/tools/echo", "/repo/tools/cat", "/repo/tools/go", "/repo/tools/future-utility", "./tools/echo", "future-utility"} {
		t.Run(head, func(t *testing.T) {
			files := fstest.MapFS{"Makefile": {Data: []byte("check:; $(if yes," + head + "," + head + ")\n")}}
			graph, err := BuildFS(files, Driver{})
			if err != nil {
				t.Fatal(err)
			}
			selected := witnesses(Select(graph, []string{"unindexed/new.go"}))
			if selected["driver:make:Makefile#check"].Kind != WitnessFrontierSite {
				t.Fatalf("unproven executable inherited utility certification: %+v", selected)
			}
		})
	}
}

func TestDriverSharedEnumeration_AFPV09401(t *testing.T) {
	for _, rule := range []string{".DEFAULT", ".SUFFIXES", ".PHONY"} {
		t.Run(rule, func(t *testing.T) {
			files := fstest.MapFS{"Makefile": {Data: []byte("check: missing\n\techo done\n" + rule + ":\n\trg --files\n")}}
			graph, err := BuildFS(files, Driver{})
			if err != nil {
				t.Fatal(err)
			}
			for _, dirty := range []string{"new.fixture", "unindexed/new.go", ".hidden/new"} {
				selected := witnesses(Select(graph, []string{dirty}))
				if selected["driver:make:Makefile#check"].Kind != WitnessPathGlobReader {
					t.Fatalf("shared enumeration lost file-set read for %s: %+v", dirty, selected)
				}
			}
		})
	}
}
