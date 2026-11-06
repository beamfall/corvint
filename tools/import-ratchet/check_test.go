package importratchet

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fixtureManifest() *manifest {
	return &manifest{
		Profile: manifestShape,
		Module:  "example.com/m",
		Areas: []area{
			{Name: "low", Layer: 0, Prefixes: []string{"internal/low", "internal/mid/wire"}},
			{Name: "mid", Layer: 1, Prefixes: []string{"internal/mid"}},
			{Name: "peer", Layer: 1, Prefixes: []string{"internal/peer"}},
			{Name: "entry", Layer: 2, Entry: true, Prefixes: []string{"cmd"}},
		},
	}
}

func fixtureGraph() map[string][]string {
	return map[string][]string{
		"internal/low":      {"internal/mid"}, // wrong way, baselined
		"internal/mid":      {"internal/low", "internal/mid/wire", "internal/mid/sub"},
		"internal/mid/sub":  {"internal/mid"},
		"internal/mid/wire": nil,
		"internal/peer":     {"internal/low"},
		"cmd/x":             {"internal/mid", "internal/peer"},
	}
}

func fixtureBaseline(edges ...string) *baseline {
	return &baseline{Profile: baselineShape, Edges: edges}
}

// IRT-V0-002: own-area, strictly-lower and entry imports pass; the longest
// prefix decides the area (internal/mid/wire is low, not mid).
func TestCheckAcceptsTheBaselinedGraph(t *testing.T) {
	t.Run("IRT-V0-002 own-area lower and entry imports pass", func(t *testing.T) {
		if got := check(fixtureManifest(), fixtureGraph(), fixtureBaseline("internal/low -> internal/mid")); len(got) != 0 {
			t.Fatalf("problems on an accepted graph: %q", got)
		}
	})
}

// IRT-V0-004: a new wrong-way import, including one between two areas of the
// same layer, is named with both areas and layers.
func TestCheckNamesNewWrongWayEdges(t *testing.T) {
	t.Run("IRT-V0-004 a new wrong-way import is named", func(t *testing.T) {
		g := fixtureGraph()
		g["internal/mid/wire"] = []string{"internal/low"} // same area: allowed
		g["internal/peer"] = append(g["internal/peer"], "internal/mid")
		g["internal/mid"] = append(g["internal/mid"], "cmd/x")
		got := check(fixtureManifest(), g, fixtureBaseline("internal/low -> internal/mid"))
		want := []string{
			"NEW_WRONG_WAY_EDGE internal/mid -> cmd/x: area mid (layer 1) may not import area entry (layer 2)",
			"NEW_WRONG_WAY_EDGE internal/peer -> internal/mid: area peer (layer 1) may not import area mid (layer 1)",
		}
		if len(got) != len(want) {
			t.Fatalf("got %q, want %d problems", got, len(want))
		}
		for i := range want {
			if !strings.HasPrefix(got[i], want[i]) || !strings.Contains(got[i], "Do not add it to "+baselinePath) {
				t.Errorf("problem %d = %q, want prefix %q and the no-baseline advice", i, got[i], want[i])
			}
		}
	})
}

// IRT-V0-004: a baseline entry whose import is gone, or no longer wrong-way,
// fails and says to delete it; unsorted and duplicate entries fail.
func TestCheckRefusesStaleAndMalformedBaselines(t *testing.T) {
	t.Run("IRT-V0-004 stale and malformed baseline entries fail", func(t *testing.T) {
		g := fixtureGraph()
		g["internal/low"] = nil
		got := check(fixtureManifest(), g, fixtureBaseline("internal/low -> internal/mid", "internal/mid -> internal/low"))
		if len(got) != 2 {
			t.Fatalf("got %q, want two stale entries", got)
		}
		for _, p := range got {
			if !strings.HasPrefix(p, "STALE_BASELINE_EDGE ") || !strings.Contains(p, "delete its line from "+baselinePath) {
				t.Errorf("stale problem lacks the delete advice: %q", p)
			}
		}
		got = check(fixtureManifest(), fixtureGraph(), fixtureBaseline("internal/low -> internal/mid", "internal/low -> internal/mid"))
		if len(got) != 1 || !strings.HasPrefix(got[0], "BASELINE_FORMAT duplicate edge") {
			t.Errorf("duplicate baseline entry: got %q", got)
		}
		g = fixtureGraph()
		g["internal/low"] = append(g["internal/low"], "internal/peer")
		got = check(fixtureManifest(), g, fixtureBaseline("internal/low -> internal/peer", "internal/low -> internal/mid"))
		if len(got) != 1 || !strings.HasPrefix(got[0], "BASELINE_FORMAT edge \"internal/low -> internal/mid\" is out of order") {
			t.Errorf("unsorted baseline: got %q", got)
		}
	})
}

// IRT-V0-001: every package maps to an area, every prefix matches a package,
// and the manifest itself is well formed.
func TestCheckRefusesManifestGaps(t *testing.T) {
	t.Run("IRT-V0-001 manifest gaps and defects fail", func(t *testing.T) {
		g := fixtureGraph()
		g["internal/orphan"] = []string{"internal/low"}
		m := fixtureManifest()
		m.Areas[2].Prefixes = append(m.Areas[2].Prefixes, "internal/gone")
		got := check(m, g, fixtureBaseline("internal/low -> internal/mid"))
		want := []string{
			"STALE_AREA_PREFIX internal/gone (area peer) matches no package: delete it from " + manifestPath,
			"UNMAPPED_PACKAGE internal/orphan: add a prefix for it to an area in " + manifestPath,
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %q, want %q", got, want)
		}
		m = fixtureManifest()
		m.Profile = "x"
		m.Areas = append(m.Areas, area{Name: "low", Layer: -1, Prefixes: []string{"internal/low", "bad/"}})
		if got := m.validate(); len(got) != 5 {
			t.Errorf("manifest defects: got %q, want profile, duplicate name, negative layer, malformed and duplicate prefix", got)
		}
	})
}

// IRT-V0-003: the scanner reads only production files some release platform
// selects, skips test data, hidden, underscore and nested-module directories,
// and still follows an import into a hidden directory.
func TestScanModuleFollowsGoListRules(t *testing.T) {
	t.Run("IRT-V0-003 the scan follows go list rules", func(t *testing.T) {
		root := t.TempDir()
		write := func(rel, body string) {
			t.Helper()
			path := filepath.Join(root, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		write("go.mod", "module example.com/m\n")
		write("a/a.go", "package a\nimport (\n\t\"fmt\"\n\t\"example.com/m/b\"\n\t\"example.com/m/.hidden/h\"\n)\nvar _ = fmt.Sprint\n")
		write("a/a_windows.go", "package a\nimport _ \"example.com/m/w\"\n")
		write("a/a_plan9.go", "package a\nimport _ \"example.com/m/plan9only\"\n")
		write("a/gen.go", "//go:build ignore\n\npackage main\nimport _ \"example.com/m/ignored\"\n")
		write("a/a_test.go", "package a\nimport _ \"example.com/m/testonly\"\n")
		write("b/b.go", "package b\n")
		write("w/w.go", "package w\n")
		write(".hidden/h/h.go", "package h\n")
		write(".hidden/unused/u.go", "package u\n")
		write("_skip/s.go", "package s\n")
		write("a/testdata/t.go", "package t\n")
		write("nested/go.mod", "module example.com/m/nested\n")
		write("nested/n.go", "package n\n")
		write("docs/readme.md", "not go\n")
		got, err := scanModule(root, "example.com/m")
		if err != nil {
			t.Fatal(err)
		}
		want := map[string][]string{
			"a":         {".hidden/h", "b", "w"},
			"b":         {},
			"w":         {},
			".hidden/h": {},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("graph = %v, want %v", got, want)
		}
	})
}
