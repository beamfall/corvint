// SPDX-License-Identifier: Apache-2.0
package affected_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Beamfall/corvint/internal/gateselect"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/ruby"
	"github.com/Beamfall/corvint/internal/sessiontest"
)

func selwideFiles(t *testing.T, files map[string]string) (string, fstest.MapFS) {
	t.Helper()
	root, fsys := t.TempDir(), fstest.MapFS{}
	for name, body := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		fsys[name] = &fstest.MapFile{Data: []byte(body)}
	}
	return root, fsys
}

// Exercise the real Go selector and session allowlist, not just the graph's scope.
func selwideSession(t *testing.T, root string, graph *affected.Graph, dirty string) sessiontest.Manifest {
	t.Helper()
	plan := affected.Select(graph, []string{dirty})
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
	lines := gateselect.SelectStructural(receipt, "example.test/m", root)
	input.GoSelection = lines[len(lines)-1]
	manifest, err := sessiontest.Build(input)
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

// AFP-V0-9012: reuse PR #125's hidden discovery; both source universes keep
// the helper and importers. An injected unindexable source kind still forces FULL.
func TestHiddenGoSelection_AFPV09012(t *testing.T) {
	for _, directory := range []string{".github/cishards", ".github/testconfine", "nested/.helpers"} {
		t.Run(directory, func(t *testing.T) {
			root, fsys := selwideFiles(t, map[string]string{
				"go.mod":                      "module example.test/m\n\ngo 1.27.0\n",
				directory + "/helper.go":      "package helper\nfunc Value() int { return 1 }\n",
				directory + "/helper_test.go": "package helper\nimport \"testing\"\nfunc TestValue(t *testing.T) {}\n",
				"consumer/use.go":             "package consumer\nimport _ \"example.test/m/" + directory + "\"\n",
				"consumer/use_test.go":        "package consumer\nimport \"testing\"\nfunc TestUse(t *testing.T) {}\n",
				"solo/solo_test.go":           "package solo\nimport \"testing\"\nfunc TestSolo(t *testing.T) {}\n",
				".git/metadata.go":            "not Go source",
				affected.NonSourcePathsPath:   `{"profile":"corvint-non-source-paths/0","entries":[{"path":".evidence/","reason":"retained output"}]}`,
				".evidence/retained.go":       "not Go source",
			})
			graph, err := affected.Build(context.Background(), root, golang.New())
			if err != nil {
				t.Fatal(err)
			}
			pinned, err := affected.BuildFS(fsys, golang.New())
			if err != nil {
				t.Fatal(err)
			}
			if pinned.Digest() != graph.Digest() {
				t.Fatal("disk and immutable graphs differ")
			}
			plan := affected.Select(graph, []string{directory + "/helper.go"})
			want := []string{directory + "/helper_test.go", "consumer/use_test.go"}
			slices.Sort(want)
			if plan.Scope != affected.ScopeBounded || !slices.Equal(plan.SelectedTests(), want) {
				t.Fatalf("plan=%+v", plan)
			}
			if m := selwideSession(t, root, graph, directory+"/helper.go"); m.Scope == sessiontest.ScopeFull || len(m.Units) != 2 {
				t.Fatalf("session=%+v", m)
			}
			fault, err := affected.Build(context.Background(), root, golang.New(), selwideUnindexed{})
			if err != nil {
				t.Fatal(err)
			}
			if m := selwideSession(t, root, fault, directory+"/new.unmodelled"); m.Scope != sessiontest.ScopeFull || !strings.Contains(m.Reason, affected.UnknownUnindexedSourcePath) {
				t.Fatalf("injected source was narrowed: %+v", m)
			}
		})
	}
}

// AFP-V0-9012 / STR-V0-004: indexed hidden packages participate in real
// discovery and execution, including when an unrelated package is selected.
func TestHiddenGoExecution_AFPV09012(t *testing.T) {
	files := map[string]string{
		"go.mod":            "module example.test/m\n\ngo 1.27.3\n",
		"solo/solo_test.go": "package solo\nimport \"testing\"\nfunc TestSolo(t *testing.T) {}\n",
	}
	for _, directory := range []string{".github/cishards", ".github/testconfine/cmd", "nested/.helpers", "nested/_helpers"} {
		files[directory+"/helper_test.go"] = "package helper\nimport (\"testing\"; \"os\")\nfunc TestHidden(t *testing.T) { if os.Getenv(\"SELWIDE_FAIL_HIDDEN\") == \"1\" { t.Fatal(\"hidden test executed\") } }\n"
	}
	root, _ := selwideFiles(t, files)
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	var universe []string
	for _, id := range graph.UnitIDs() {
		universe = append(universe, strings.TrimPrefix(id, "go:"))
	}
	for _, dirty := range []string{"solo/solo_test.go", ".github/cishards/helper_test.go", ".github/testconfine/cmd/helper_test.go", "nested/.helpers/helper_test.go", "nested/_helpers/helper_test.go"} {
		t.Run(dirty, func(t *testing.T) {
			manifest := selwideSession(t, root, graph, dirty)
			digest, err := sessiontest.Digest(manifest)
			if err != nil {
				t.Fatal(err)
			}
			runner := sessiontest.Runner{Root: root, Universe: universe, Environ: os.Environ(), Budget: 2 * time.Minute, Output: io.Discard, Errors: io.Discard,
				Recheck: func(context.Context) (sessiontest.SourceIdentity, error) { return manifest.Source, nil }}
			receipt := runner.Execute(context.Background(), manifest, digest, 0)
			if receipt.Result != sessiontest.ResultPass || len(receipt.Invocations) == 0 {
				t.Fatalf("execution: %+v", receipt)
			}
			var outcomes []string
			for _, invocation := range receipt.Invocations {
				for _, outcome := range invocation.Outcomes {
					if outcome.Outcome != sessiontest.OutcomePass {
						t.Fatalf("test did not pass: %+v", outcome)
					}
					outcomes = append(outcomes, outcome.Unit)
				}
			}
			if !slices.Contains(outcomes, "go:example.test/m/"+path.Dir(dirty)) {
				t.Fatalf("selected package not executed: %v", outcomes)
			}
			if strings.HasPrefix(dirty, ".github/") {
				runner.Environ = append(os.Environ(), "SELWIDE_FAIL_HIDDEN=1")
				failed := runner.Execute(context.Background(), manifest, digest, 0)
				if failed.Result != sessiontest.ResultFail {
					t.Fatalf("hidden test failure was lost: %+v", failed)
				}
			}
		})
	}
}

type selwideUnindexed struct{}

func (selwideUnindexed) Name() string       { return "unmodelled" }
func (selwideUnindexed) Owns(p string) bool { return strings.HasSuffix(p, ".unmodelled") }
func (selwideUnindexed) Units(context.Context, string) (affected.Result, error) {
	return affected.Result{}, nil
}

// AFP-V0-9013: hosted unindexed fixtures retain readers without FULL; failed
// indexing of real compiled source retains FULL, even below a testdata directory.
// A hosted fixture no plugin indexes is a path no runner owns, which runs every
// test since RNC-V1-004 (owner decision R32).
func TestUnindexedFixtureSession_AFPV09013(t *testing.T) {
	root, _ := selwideFiles(t, map[string]string{
		"go.mod":                "module example.test/m\n\ngo 1.27.0\n",
		"host/host.go":          "package host\n",
		"host/host_test.go":     "package host\nimport \"testing\"\nfunc TestHost(t *testing.T) {}\n",
		"host/testdata/case.rb": "puts 1\n",
		"host/testdata/new.go":  "package fixture\n",
		"reader/reader_test.go": "package reader\nimport \"testing\"\nfunc TestRead(t *testing.T) { _ = \"../host/testdata/case.rb\" }\n",
		"solo/solo_test.go":     "package solo\nimport \"testing\"\nfunc TestSolo(t *testing.T) {}\n",
		"host/missed.go":        "package host\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New(), ruby.New())
	if err != nil {
		t.Fatal(err)
	}
	for _, dirty := range []string{"host/testdata/case.rb", "host/testdata/new.go"} {
		m := selwideSession(t, root, graph, dirty)
		// No plugin indexes a hosted fixture, so it is a path no runner owns
		// and the plan runs every test with the named reason, its readers
		// included (RNC-V1-004, owner decision R32).
		if m.Scope != sessiontest.ScopeFull || !strings.HasPrefix(m.Reason, "unowned-path: ") || !strings.HasSuffix(m.Reason, ": "+dirty) {
			t.Fatalf("unowned fixture narrowed: %+v", m)
		}
		var units []string
		for _, u := range m.Units {
			units = append(units, u.Unit)
		}
		if !slices.Contains(units, "go:example.test/m/host") || !slices.Contains(units, "go:example.test/m/solo") {
			t.Fatalf("FULL units=%v", units)
		}
		if strings.HasSuffix(dirty, ".rb") && !slices.Contains(units, "go:example.test/m/reader") {
			t.Fatalf("literal reader omitted: %v", units)
		}
	}
	// Inject the indexing fault after observing compiled source, preserving the
	// claimant and another indexed package under the fixture directory.
	for _, dirty := range []string{"host/missed.go", "host/testdata/new.go"} {
		fault, err := affected.Build(context.Background(), root, selwideMissedGo{})
		if err != nil {
			t.Fatal(err)
		}
		if m := selwideSession(t, root, fault, dirty); m.Scope != sessiontest.ScopeFull || !strings.Contains(m.Reason, affected.UnknownUnindexedSourcePath) {
			t.Fatalf("compiled source narrowed: %+v", m)
		}
	}
}

type selwideMissedGo struct{}

func (selwideMissedGo) Name() string       { return "go" }
func (selwideMissedGo) Owns(p string) bool { return strings.HasSuffix(p, ".go") }
func (selwideMissedGo) Units(context.Context, string) (affected.Result, error) {
	return affected.Result{Units: []affected.Unit{{ID: "go:example.test/m/host", Sources: []string{"host/host.go"}, Tests: []string{"host/host_test.go"}}, {ID: "go:example.test/m/compiled", Sources: []string{"host/testdata/compiled/kept.go"}}}}, nil
}
