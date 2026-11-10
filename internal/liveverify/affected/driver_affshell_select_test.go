// SPDX-License-Identifier: Apache-2.0

package affected_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// writeFiles writes the fixture files below root.
func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// AFP-V0-9050: `or` skips an empty argument and `and` returns empty on one,
// as GNU make 4.4.1 does (`make -n` of `echo $(if $(and ,yes),ci,local)`
// prints local and of `$(or ,yes)` prints ci). The chosen branch is the only
// package the target selects.
func TestMakeOrAndEmptyArguments_AFP_V0_9050(t *testing.T) {
	for _, tc := range []struct{ condition, branch string }{
		{"$(or ,yes)", "a"}, {"$(or , ,yes)", "a"}, {"$(or yes,)", "a"}, {"$(or ,)", "b"}, {"$(or ,$(empty))", "b"},
		{"$(and ,yes)", "b"}, {"$(and yes,)", "b"}, {"$(and yes,yes)", "a"},
	} {
		t.Run(tc.condition, func(t *testing.T) {
			files := boundFiles("empty :=\ncheck:; go test ./$(if " + tc.condition + ",a,b)\nall: check\n")
			files["a/a.go"] = "package a\n"
			files["b/b.go"] = "package b\n"
			graph := driverGraph(t, files, nil)
			if computedSite(t, graph, "driver:make:Makefile#check") {
				t.Fatal("literal condition left the target computed")
			}
			other := "b"
			if tc.branch == "b" {
				other = "a"
			}
			units := []string{"driver:make:Makefile#check", makeAll, jobUnit}
			wantDriverSelection(t, driverSelections(affected.Select(graph, []string{tc.branch + "/" + tc.branch + ".go"})), units, nil)
			wantDriverSelection(t, driverSelections(affected.Select(graph, []string{other + "/" + other + ".go"})), nil, units)
		})
	}
}

// AFP-V0-9131: an absolute command word below the repository root is the
// repository program at that path: it records the path edge and keeps the
// fallback. Any other absolute word, a system path included, is certified for
// nothing and records no edge.
func TestDriverAbsoluteExecutable_AFP_V0_9131(t *testing.T) {
	for _, tc := range []struct {
		name, head      string
		edge, certified bool
	}{
		{"repository", "ROOT/tools/cat", true, false},
		// A repository shell test with literal arguments certifies as its
		// relative spelling does: the shell plugin selects it for itself.
		{"repository-test", "ROOT/script/audit_test.sh", true, true},
		{"bin", "/bin/cat", false, false},
		{"usr-bin", "/usr/bin/cat", false, false},
		{"unknown", "/opt/tools/cat", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			head := tc.head
			if rest, found := slicesCutPrefix(head, "ROOT"); found {
				head = filepath.ToSlash(root) + rest
			}
			files := boundFiles("check:; $(if yes," + head + " LICENSE,/bin/true)\nall: check\n")
			files["go.mod"] = "module example.test/m\n\ngo 1.27\n"
			files["tools/cat"] = "#!/bin/sh\ncat \"$@\"\n"
			files["LICENSE"] = "Apache-2.0\n"
			writeFiles(t, root, files)
			graph, err := affected.Build(t.Context(), root, golang.Language{}, affected.Driver{})
			if err != nil {
				t.Fatal(err)
			}
			if got := computedSite(t, graph, "driver:make:Makefile#check"); got == tc.certified {
				t.Fatalf("computed site %v; want %v", got, !tc.certified)
			}
			unit, _ := graph.Unit("driver:make:Makefile#check")
			relative := head
			if tc.edge {
				relative = head[len(filepath.ToSlash(root))+1:]
			}
			if got := slices.Contains(unit.PathImports, relative); got != tc.edge {
				t.Fatalf("path edge to %s = %v, imports %v; want %v", relative, got, unit.PathImports, tc.edge)
			}
			units := []string{"driver:make:Makefile#check", makeAll, jobUnit}
			selected := driverSelections(affected.Select(graph, []string{sealedCEM}))
			if tc.certified {
				wantDriverSelection(t, selected, nil, units)
			} else {
				wantDriverSelection(t, selected, units, nil)
			}
			if tc.edge {
				wantDriverSelection(t, driverSelections(affected.Select(graph, []string{relative})), units, nil)
			}
		})
	}
}

func slicesCutPrefix(text, prefix string) (string, bool) {
	if len(text) >= len(prefix) && text[:len(prefix)] == prefix {
		return text[len(prefix):], true
	}
	return "", false
}

// AFP-V0-9051: a glob operand whose match is a symlink into another
// repository file keeps the frontier, so editing the link target selects the
// target; a glob of regular files records the directory edge instead.
func TestDriverGlobSymlinkMatch_AFP_V0_9051(t *testing.T) {
	for _, linked := range []bool{true, false} {
		t.Run(map[bool]string{true: "symlink", false: "regular"}[linked], func(t *testing.T) {
			root := t.TempDir()
			files := boundFiles("check:; $(if yes,grep,grep) KEEP inputs/*.txt\nall: check\n")
			files["go.mod"] = "module example.test/m\n\ngo 1.27\n"
			files["inputs/plain.txt"] = "KEEP\n"
			if !linked {
				files["inputs/value.txt"] = "KEEP\n"
			}
			writeFiles(t, root, files)
			if linked {
				if err := os.Symlink("../tools/local/main.go", filepath.Join(root, "inputs", "value.txt")); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			graph, err := affected.Build(t.Context(), root, golang.Language{}, affected.Driver{})
			if err != nil {
				t.Fatal(err)
			}
			if got := computedSite(t, graph, "driver:make:Makefile#check"); got != linked {
				t.Fatalf("computed site %v; want %v", got, linked)
			}
			units := []string{"driver:make:Makefile#check", makeAll, jobUnit}
			wantDriverSelection(t, driverSelections(affected.Select(graph, []string{"inputs/plain.txt"})), units, nil)
			selected := driverSelections(affected.Select(graph, []string{"tools/local/main.go"}))
			if linked {
				wantDriverSelection(t, selected, units, nil)
			} else {
				wantDriverSelection(t, selected, nil, units)
			}
		})
	}
}
