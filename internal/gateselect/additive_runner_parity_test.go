// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// additiveRunnerFixture is the review's case for the CI selector
// (AFP-V0-3143): command, a command that locates nothing, is run from the
// code of harness, consumer calls harness.Run, and the change edits command
// and adds an unused exported function to harness. It returns the base and
// head trees.
func additiveRunnerFixture(harness, command, harnessSource string, extra map[string]string) (base, head map[string]string) {
	quiet := func(word string) string {
		return "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"" + word + "\") }\n"
	}
	base = map[string]string{
		"go.mod":                  "module example.com/fixture\n\ngo 1.22\n",
		command + "/main.go":      quiet("a"),
		command + "/main_test.go": "package main\n",
		harness + "/h.go":         harnessSource,
		harness + "/h_test.go":    "package harness\n",
		"consumer/c.go":           "package consumer\n\nimport \"example.com/fixture/" + harness + "\"\n\n// Use runs the command through harness.\nfunc Use() error { return harness.Run() }\n",
		"consumer/c_test.go":      "package consumer\n",
	}
	for relative, body := range extra {
		base[relative] = body
	}
	head = map[string]string{}
	for relative, body := range base {
		head[relative] = body
	}
	head[command+"/main.go"] = quiet("b")
	head[harness+"/h.go"] = harnessSource + "\n// Fresh is new and unused.\nfunc Fresh() {}\n"
	return base, head
}

// narrowedHarnessPlan is the receipt the planner emitted before AFP-V0-3140:
// harness's record NARROWED with consumer unreferenced.
func narrowedHarnessPlan(harness string, dirty ...string) Receipt {
	var plan Receipt
	plan.Provider.Go.State = "RUNNABLE"
	plan.Plan.Dirty = dirty
	plan.Plan.GoAdditive = append(plan.Plan.GoAdditive, struct {
		Unit         string   `json:"unit"`
		State        string   `json:"state"`
		Paths        []string `json:"paths"`
		Unreferenced []string `json:"unreferenced"`
	}{Unit: "go:" + fixtureModule + "/" + harness, State: "NARROWED", Paths: []string{harness + "/h.go"}, Unreferenced: []string{"go:" + fixtureModule + "/consumer"}})
	return plan
}

// plannerNarrowedSelection returns the Go package directories the planner
// selects for dirty over root with the plugin's additive records measured
// from base.
func plannerNarrowedSelection(t *testing.T, root string, base map[string]string, dirty []string) []string {
	t.Helper()
	baseFS := fstest.MapFS{}
	for relative, body := range base {
		baseFS[relative] = &fstest.MapFile{Data: []byte(body)}
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	records := golang.AdditiveRecords(baseFS, os.DirFS(root), graph, dirty)
	var selected []string
	for _, item := range affected.Select(graph.WithGoAdditive(records), dirty).Selected {
		if rest, ok := strings.CutPrefix(item.UnitID, "go:"+fixtureModule+"/"); ok {
			selected = append(selected, rest)
		}
	}
	slices.Sort(selected)
	return selected
}

// TestNarrowedRunnerFollowsTheChangedCommand_AFPV03143 holds the CI
// selector's `corvint test` form (SelectStructural) at least as wide as the
// planner on the review's case. Rules (a) and (b) walk a command's code
// runners as they walk its importers and namers (AFP-V0-3143, mirroring
// AFP-V0-2993), so the edit of the command reaches harness's importers
// whatever harness's own path narrows to. Before the fix a run declared in
// .corvint/test-binary-execs.json was no namer edge, and consumer was
// missing under a NARROWED record; a module-path literal was already a
// namer. (A relative literal in library code is a rule (d) read, which the
// plan's UNBOUNDED_READER selections carry, so it is no case here.) Select
// keeps the whole attribution in every case.
func TestNarrowedRunnerFollowsTheChangedCommand_AFPV03143(t *testing.T) {
	cases := []struct {
		name, harness, command, source string
		extra                          map[string]string
	}{
		{
			name: "module-path go run", harness: "harness", command: "quiet",
			source: "package harness\n\nimport \"os/exec\"\n\n// Run runs quiet.\nfunc Run() error { return exec.Command(\"go\", \"run\", \"example.com/fixture/quiet\").Run() }\n",
		},
		{
			name: "declared binary exec", harness: "harness", command: "quiet",
			source: "package harness\n\n// Run runs quiet, as .corvint/test-binary-execs.json declares; no literal names it.\nfunc Run() error { return nil }\n",
			extra: map[string]string{
				".corvint/test-binary-execs.json": "{\"profile\":\"corvint-test-binary-execs/0\",\"packages\":{\"harness\":[\"quiet\"]}}\n", //corvint:fixture-path
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base, head := additiveRunnerFixture(tc.harness, tc.command, tc.source, tc.extra)
			root := writeFixture(t, head)
			dirty := []string{tc.harness + "/h.go", tc.command + "/main.go"}
			planner := plannerNarrowedSelection(t, root, base, dirty)
			if !slices.Contains(planner, "consumer") {
				t.Fatalf("the planner leaves consumer out: %v", planner)
			}
			structural, lines := structuralVerdict(narrowedHarnessPlan(tc.harness, dirty...), root)
			for _, directory := range planner {
				if !strings.Contains(structural+" ", " "+fixtureModule+"/"+directory+" ") {
					t.Errorf("planner package %s missing from the structural verdict:\n%s", directory, strings.Join(lines, "\n"))
				}
			}
			if got := verdict(narrowedHarnessPlan(tc.harness, dirty...), root); !strings.Contains(got+" ", " "+fixtureModule+"/consumer ") {
				t.Errorf("Select verdict %q leaves consumer out", got)
			}
		})
	}
}

// TestNarrowedRunnerKeepsItsNarrowing_AFPV03143 is the control: harness's
// additive change alone keeps consumer out of both the planner's and the
// structural selection, so following code runners narrows nothing less than
// the change requires.
func TestNarrowedRunnerKeepsItsNarrowing_AFPV03143(t *testing.T) {
	source := "package harness\n\n// Run runs quiet, as .corvint/test-binary-execs.json declares.\nfunc Run() error { return nil }\n"
	declaration := map[string]string{
		".corvint/test-binary-execs.json": "{\"profile\":\"corvint-test-binary-execs/0\",\"packages\":{\"harness\":[\"quiet\"]}}\n", //corvint:fixture-path
	}
	base, head := additiveRunnerFixture("harness", "quiet", source, declaration)
	head["quiet/main.go"] = base["quiet/main.go"]
	root := writeFixture(t, head)
	dirty := []string{"harness/h.go"}
	if planner := plannerNarrowedSelection(t, root, base, dirty); slices.Contains(planner, "consumer") {
		t.Errorf("the planner selects consumer for an additive change alone: %v", planner)
	}
	if got, lines := structuralVerdict(narrowedHarnessPlan("harness", dirty...), root); strings.Contains(got+" ", " "+fixtureModule+"/consumer ") {
		t.Errorf("the structural verdict selects consumer for an additive change alone:\n%s", strings.Join(lines, "\n"))
	}
}

// TestNarrowedCommandKeepsItsRunners_AFPV03143: a narrowed record prunes
// import edges only. The plugin never narrows a `main` package (ADN-V0-004),
// so the planner selects the packages whose code runs the command, and their
// importers, the ordinary way; a receipt that names a NARROWED record for the
// command anyway (stale or forged) must not narrow them away from the
// structural verdict, so attributeNarrowed starts from the command's code
// runners too. The run is declared, so no namer edge stands in for it.
func TestNarrowedCommandKeepsItsRunners_AFPV03143(t *testing.T) {
	source := "package harness\n\n// Run runs quiet, as .corvint/test-binary-execs.json declares.\nfunc Run() error { return nil }\n"
	declaration := map[string]string{
		".corvint/test-binary-execs.json": "{\"profile\":\"corvint-test-binary-execs/0\",\"packages\":{\"harness\":[\"quiet\"]}}\n", //corvint:fixture-path
	}
	base, head := additiveRunnerFixture("harness", "quiet", source, declaration)
	head["harness/h.go"] = base["harness/h.go"]
	head["quiet/main.go"] = base["quiet/main.go"] + "\n// Fresh is new and unused.\nfunc Fresh() {}\n"
	root := writeFixture(t, head)
	dirty := []string{"quiet/main.go"}
	planner := plannerNarrowedSelection(t, root, base, dirty)
	if !slices.Contains(planner, "consumer") {
		t.Fatalf("the planner leaves consumer out: %v", planner)
	}
	var plan Receipt
	plan.Provider.Go.State = "RUNNABLE"
	plan.Plan.Dirty = dirty
	plan.Plan.GoAdditive = append(plan.Plan.GoAdditive, struct {
		Unit         string   `json:"unit"`
		State        string   `json:"state"`
		Paths        []string `json:"paths"`
		Unreferenced []string `json:"unreferenced"`
	}{Unit: "go:" + fixtureModule + "/quiet", State: "NARROWED", Paths: dirty, Unreferenced: []string{}})
	structural, lines := structuralVerdict(plan, root)
	for _, directory := range planner {
		if !strings.Contains(structural+" ", " "+fixtureModule+"/"+directory+" ") {
			t.Errorf("planner package %s missing from the structural verdict:\n%s", directory, strings.Join(lines, "\n"))
		}
	}
}
