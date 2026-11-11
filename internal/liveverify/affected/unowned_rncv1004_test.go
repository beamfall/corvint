// SPDX-License-Identifier: Apache-2.0
package affected_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gateselect"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/sessiontest"
)

// unownedSession is selwideSession with the selector named: the planner's
// plan, the Go attribution verdict of selector and sessiontest.Build.
func unownedSession(t *testing.T, root string, graph *affected.Graph, selector func(gateselect.Receipt, string, string) []string, dirty ...string) sessiontest.Manifest {
	t.Helper()
	plan := affected.Select(graph, dirty)
	body, err := json.Marshal(map[string]any{"plan": plan})
	if err != nil {
		t.Fatal(err)
	}
	var receipt gateselect.Receipt
	if err := json.Unmarshal(body, &receipt); err != nil {
		t.Fatal(err)
	}
	receipt.Provider.Go.State = "RUNNABLE"
	input := sessiontest.Input{ShardCount: 1, RunnerProfile: sessiontest.ProfileLocal, GraphDigest: graph.Digest()}
	for _, id := range graph.UnitIDs() {
		if pkg, ok := strings.CutPrefix(id, "go:"); ok {
			input.Universe = append(input.Universe, pkg)
		}
	}
	for _, s := range plan.Selected {
		input.Selected = append(input.Selected, s.UnitID)
		if pkg, ok := strings.CutPrefix(s.UnitID, "go:"); ok {
			receipt.Provider.Go.Packages = append(receipt.Provider.Go.Packages, pkg)
		}
	}
	for _, u := range plan.Unknown {
		input.Unknown = append(input.Unknown, sessiontest.Unknown{Reason: u.Reason, Detail: u.Detail})
	}
	lines := selector(receipt, "example.test/m", root)
	input.GoSelection = lines[len(lines)-1]
	manifest, err := sessiontest.Build(input)
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

// TestUnownedPathRunsAllOnlyWhenNoRuleAttributesIt_RNCV1004: owner decision
// R32 runs every test only for a changed path no runner instance owns. A path
// a Go selection rule attributes (here a testdata file one package names) is
// owned by the Go instance and narrows to its attributed packages; a path no
// rule attributes runs every test with the named reason unowned-path; a
// go.sum-only change keeps the separate root-module fallback, never
// unowned-path, because the Go instance owns it. Both selectors agree: the
// qualified one `corvint test` reads and the structural one.
func TestUnownedPathRunsAllOnlyWhenNoRuleAttributesIt_RNCV1004(t *testing.T) {
	root, _ := selwideFiles(t, map[string]string{
		"go.mod":                    "module example.test/m\n\ngo 1.27.0\n",
		"go.sum":                    "",
		"fixtures/testdata/in.json": "{}\n",
		"notes.txt":                 "loose\n",
		"reader/reader_test.go":     "package reader\nimport \"testing\"\nfunc TestRead(t *testing.T) { _ = \"../fixtures/testdata/in.json\" }\n",
		"solo/solo.go":              "package solo\n",
		"solo/solo_test.go":         "package solo\nimport \"testing\"\nfunc TestSolo(t *testing.T) {}\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	units := func(m sessiontest.Manifest) []string {
		var ids []string
		for _, u := range m.Units {
			ids = append(ids, u.Unit)
		}
		return ids
	}
	for name, selector := range map[string]func(gateselect.Receipt, string, string) []string{"qualified": gateselect.SelectQualified, "structural": gateselect.SelectStructural} {
		t.Run(name+"/testdata read by one package", func(t *testing.T) {
			plan := affected.Select(graph, []string{"fixtures/testdata/in.json"})
			if !slices.ContainsFunc(plan.Unknown, func(u affected.Unknown) bool {
				return u.Reason == affected.UnknownUnownedDirtyPath && u.Detail == "fixtures/testdata/in.json"
			}) {
				t.Fatalf("the fixture is no longer UNOWNED_DIRTY_PATH, so it proves nothing: %+v", plan.Unknown)
			}
			m := unownedSession(t, root, graph, selector, "fixtures/testdata/in.json")
			if m.Scope != sessiontest.ScopeNarrowed {
				t.Fatalf("an attributed path forced %s: %q", m.Scope, m.Reason)
			}
			if got := units(m); !slices.Contains(got, "go:example.test/m/reader") || slices.Contains(got, "go:example.test/m/solo") {
				t.Fatalf("units = %v, want the reader alone", got)
			}
		})
		t.Run(name+"/path no rule attributes", func(t *testing.T) {
			m := unownedSession(t, root, graph, selector, "notes.txt")
			// The NCM-V1-001 non-code decision names the dirty path first,
			// with the same reason.
			if m.Scope != sessiontest.ScopeFull || m.Reason != gateselect.ReasonUnownedPath+" notes.txt" {
				t.Fatalf("scope %s reason %q, want FULL with %s", m.Scope, m.Reason, gateselect.ReasonUnownedPath)
			}
			if got := units(m); !slices.Contains(got, "go:example.test/m/solo") || !slices.Contains(got, "go:example.test/m/reader") {
				t.Fatalf("FULL units = %v", got)
			}
		})
		t.Run(name+"/go.sum only", func(t *testing.T) {
			m := unownedSession(t, root, graph, selector, "go.sum")
			if strings.HasPrefix(m.Reason, gateselect.ReasonUnownedPath) {
				t.Fatalf("go.sum, which the Go instance owns, ran every test as unowned: %q", m.Reason)
			}
			if m.Scope != sessiontest.ScopeFull || m.Reason != "root module definition is dirty: go.sum" {
				t.Fatalf("scope %s reason %q, want the root-module fallback", m.Scope, m.Reason)
			}
		})
	}
}

// TestUnownedPathWithoutGoUniverseRunsAll_RNCV1004: with no Go universe no Go
// rule can attribute a path, so an UNOWNED_DIRTY_PATH runs every test with
// unowned-path even though the verdict is NOTHING.
func TestUnownedPathWithoutGoUniverseRunsAll_RNCV1004(t *testing.T) {
	manifest, err := sessiontest.Build(sessiontest.Input{
		ShardCount: 1, RunnerProfile: sessiontest.ProfileLocal, GraphDigest: "g",
		GoSelection: "NOTHING no Go universe",
		Unknown:     []sessiontest.Unknown{{Reason: affected.UnknownUnownedDirtyPath, Detail: "notes.txt"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Scope != sessiontest.ScopeFull || manifest.Reason != gateselect.UnownedPathFallback+": notes.txt" {
		t.Fatalf("scope %s reason %q", manifest.Scope, manifest.Reason)
	}
}

// TestUnboundedReaderDoesNotAttributeAnUnownedPath_RNCV1004: the planner
// witnesses an unbounded reader (rule (d), here a test that reads a path built
// from os.Getwd) with whichever dirty path sorts first, whatever path it reads. A change of only the root notes.txt, which no
// package encloses or names, therefore selects the unbounded package with
// notes.txt as its witness; that selection attributes nothing, so every
// selector still runs every test with the named reason unowned-path instead of
// narrowing to the unbounded package. (A root Makefile, the original fixture,
// is on the NCM-V1-005 never-narrow list, which decides it first.)
func TestUnboundedReaderDoesNotAttributeAnUnownedPath_RNCV1004(t *testing.T) {
	root, _ := selwideFiles(t, map[string]string{
		"go.mod":                      "module example.test/m\n\ngo 1.27.0\n",
		"notes.txt":                   "notes\n",
		"unbounded/unbounded.go":      "package unbounded\n",
		"unbounded/unbounded_test.go": "package unbounded\nimport (\n\t\"os\"\n\t\"testing\"\n)\nfunc TestRead(t *testing.T) {\n\twd, _ := os.Getwd()\n\t_, _ = os.ReadFile(wd + \"/../in.json\")\n}\n",
		"solo/solo.go":                "package solo\n",
		"solo/solo_test.go":           "package solo\nimport \"testing\"\nfunc TestSolo(t *testing.T) {}\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"notes.txt"})
	if !slices.ContainsFunc(plan.Unknown, func(u affected.Unknown) bool {
		return u.Reason == affected.UnknownUnownedDirtyPath && u.Detail == "notes.txt"
	}) {
		t.Fatalf("notes.txt is no longer UNOWNED_DIRTY_PATH, so the fixture proves nothing: %+v", plan.Unknown)
	}
	if !slices.ContainsFunc(plan.Selected, func(s affected.Selection) bool {
		return s.UnitID == "go:example.test/m/unbounded" && s.Witness.Kind == affected.WitnessUnboundedReader && s.Witness.DirtyPath == "notes.txt"
	}) {
		t.Fatalf("the unbounded package is not witnessed by notes.txt, so the fixture proves nothing: %+v", plan.Selected)
	}
	want := gateselect.ReasonUnownedPath + " notes.txt"
	for name, selector := range map[string]func(gateselect.Receipt, string, string) []string{"select": gateselect.Select, "structural": gateselect.SelectStructural, "qualified": gateselect.SelectQualified} {
		t.Run(name, func(t *testing.T) {
			verdict := ""
			m := unownedSession(t, root, graph, func(receipt gateselect.Receipt, module, root string) []string {
				lines := selector(receipt, module, root)
				verdict = lines[len(lines)-1]
				return lines
			}, "notes.txt")
			if verdict != "FALLBACK "+want {
				t.Fatalf("verdict %q, want %q", verdict, "FALLBACK "+want)
			}
			if m.Scope != sessiontest.ScopeFull || m.Reason != want {
				t.Fatalf("scope %s reason %q, want FULL with %q", m.Scope, m.Reason, want)
			}
		})
	}
}
