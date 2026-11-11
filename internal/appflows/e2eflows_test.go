// SPDX-License-Identifier: Apache-2.0

package appflows

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
)

// e2eFlowsRepo is a module with one command, cmd/tool, built from lib and
// other, two consumer scripts that run it (web/run-a.mjs with a runner,
// web/run-b.mjs without), and two flows: tool-run links lib and the command
// and is bound to run-a; other-flow links other and is bound to run-b. Every
// declared link is reviewed at the first commit unless reviewToolRun is false,
// which leaves tool-run's links unreviewed.
func e2eFlowsRepo(t *testing.T, reviewToolRun bool) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                     "module example.test/m\n",
		"lib/lib.go":                 "package lib\n\nfunc Help() string { return \"help\" }\n",
		"other/other.go":             "package other\n\nfunc Other() string { return \"other\" }\n",
		"cmd/tool/main.go":           "package main\n\nimport (\n\t\"example.test/m/lib\"\n\t\"example.test/m/other\"\n)\n\nfunc main() { println(lib.Help(), other.Other()) }\n",
		"web/run-a.mjs":              "// runs the built tool binary\n",
		"web/run-b.mjs":              "// runs the built tool binary\n",
		affected.BinaryConsumersPath: `{"profile":"corvint-test-binary-consumers/0","consumers":{"web/run-a.mjs":{"commands":["cmd/tool"],"run":["node","web/run-a.mjs"]},"web/run-b.mjs":["cmd/tool"]}}`,
		E2EFlowsPath:                 `{"profile":"corvint-e2e-flows/0","flows":"flows","consumers":{"web/run-a.mjs":{"flows":["tool-run"]},"web/run-b.mjs":{"flows":["other-flow"]}}}`,
	}
	for name, body := range files {
		writeRaw(t, root, name, []byte(body))
	}
	toolRun := FlowIntent{Schema: FlowIntentSchema, FlowID: "tool-run", Revision: 1, Kind: "api", Actor: "operator", Preconditions: []string{},
		Steps:      []FlowStep{{StepID: "run", Action: "run the tool"}},
		Outcomes:   []FlowOutcome{{OutcomeID: "help", Behavior: "help is printed", Matcher: "toContain", Locator: "stdout", Value: "help"}},
		Variations: []FlowVariation{{VariationID: "tool-run.default", Preconditions: []string{}, Steps: []string{"run"}, ObservableFacts: []string{}, Outcomes: []string{"help"}, Projects: []string{"node"}}},
		Links: []FlowLink{
			{From: "run", Basis: "declared", Target: LinkTarget{Type: "source", Path: "cmd/tool/main.go"}},
			{From: "run", Basis: "declared", Target: LinkTarget{Type: "source", Path: "lib/lib.go"}},
			{From: "tool-run.default", Basis: "declared", Target: LinkTarget{Type: "test", Path: "web/run-a.mjs", TestKey: "web/run-a.mjs"}},
		}}
	otherFlow := FlowIntent{Schema: FlowIntentSchema, FlowID: "other-flow", Revision: 1, Kind: "api", Actor: "operator", Preconditions: []string{},
		Steps:      []FlowStep{{StepID: "other", Action: "ask for other"}},
		Outcomes:   []FlowOutcome{{OutcomeID: "printed", Behavior: "other is printed", Matcher: "toContain", Locator: "stdout", Value: "other"}},
		Variations: []FlowVariation{{VariationID: "other-flow.default", Preconditions: []string{}, Steps: []string{"other"}, ObservableFacts: []string{}, Outcomes: []string{"printed"}, Projects: []string{"node"}}},
		Links: []FlowLink{
			{From: "other", Basis: "declared", Target: LinkTarget{Type: "source", Path: "other/other.go"}},
			{From: "other-flow.default", Basis: "declared", Target: LinkTarget{Type: "test", Path: "web/run-b.mjs", TestKey: "web/run-b.mjs"}},
		}}
	writeIntent(t, root, "flows/tool-run.json", toolRun)
	writeIntent(t, root, "flows/other-flow.json", otherFlow)
	gitTest(t, root, "init", "-q")
	anchor := commitAll(t, root, "intents")
	for i := range otherFlow.Links {
		otherFlow.Links[i].ReviewedAt = anchor
	}
	if reviewToolRun {
		for i := range toolRun.Links {
			toolRun.Links[i].ReviewedAt = anchor
		}
	}
	writeIntent(t, root, "flows/tool-run.json", toolRun)
	writeIntent(t, root, "flows/other-flow.json", otherFlow)
	commitAll(t, root, "review anchors")
	return root
}

func e2eFlowsPlan(t *testing.T, root string, dirty ...string) (affected.Plan, affected.Plan) {
	t.Helper()
	graph, err := affected.Build(context.Background(), root, languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	before := affected.Select(graph, dirty)
	return before, BindE2EFlows(context.Background(), root, graph, before)
}

func consumerSelection(plan affected.Plan, file string) (affected.Selection, bool) {
	for _, s := range plan.Selected {
		if s.Kind == "DECLARED_CONSUMER" && s.UnitID == "declared-consumer:"+file {
			return s, true
		}
	}
	return affected.Selection{}, false
}

func unknownDetails(plan affected.Plan, reason string) []string {
	var out []string
	for _, u := range plan.Unknown {
		if u.Reason == reason {
			out = append(out, u.Detail)
		}
	}
	return out
}

// AFU-V1-1651, AFU-V1-1653: a change reaching a bound flow's link target
// keeps that consumer and names the reached flow; the other consumer, whose
// flows the change does not reach, is left out when a reviewed link explains
// every changed path reaching the command, and the plan records why.
func TestAFUV11651ReachedFlowKeepsAndReviewedLinksOmit(t *testing.T) {
	root := e2eFlowsRepo(t, true)
	before, plan := e2eFlowsPlan(t, root, "lib/lib.go")
	if _, ok := consumerSelection(before, "web/run-a.mjs"); !ok || !reflect.DeepEqual(unknownDetails(before, affected.UnknownUnselectedBinaryConsumer), []string{"web/run-b.mjs"}) {
		t.Fatalf("fixture plan: selected=%+v unknown=%+v", before.Selected, before.Unknown)
	}
	kept, ok := consumerSelection(plan, "web/run-a.mjs")
	if !ok || !reflect.DeepEqual(kept.Flows, []string{"tool-run"}) || !reflect.DeepEqual(kept.Run, []string{"node", "web/run-a.mjs"}) {
		t.Fatalf("run-a: %+v", kept)
	}
	if got := unknownDetails(plan, affected.UnknownUnselectedBinaryConsumer); got != nil {
		t.Fatalf("run-b still named: %v", got)
	}
	want := []affected.E2EOmission{{File: "web/run-b.mjs", Flows: []string{"other-flow"}, Commands: []string{"go:example.test/m/cmd/tool"}, Basis: E2EBasisReviewedLinks,
		Mapped: []affected.E2EMapping{{Path: "lib/lib.go", Flow: "tool-run", Target: "cmd/tool/main.go"}}}}
	if !reflect.DeepEqual(plan.E2EOmitted, want) || len(plan.Unknown) != 0 || plan.Scope != affected.ScopeBounded {
		t.Fatalf("omitted=%+v unknown=%+v scope=%s", plan.E2EOmitted, plan.Unknown, plan.Scope)
	}
	// The other change: other-flow is reached, and tool-run too, because it
	// anchors the command's own main.go and the Go graph reaches a target by
	// its owning package (AFU-V1-1651 granularity): nothing is left out, and
	// a consumer without a runner keeps its unknown with the reached flows.
	_, plan = e2eFlowsPlan(t, root, "other/other.go")
	if kept, ok := consumerSelection(plan, "web/run-a.mjs"); !ok || !reflect.DeepEqual(kept.Flows, []string{"tool-run"}) {
		t.Fatalf("run-a for other/other.go: ok=%v %+v", ok, kept)
	}
	if got := unknownDetails(plan, affected.UnknownUnselectedBinaryConsumer); !reflect.DeepEqual(got, []string{"web/run-b.mjs"}) || !strings.Contains(plan.Unknown[0].Hint, "bound flows reached: other-flow") || len(plan.E2EOmitted) != 0 {
		t.Fatalf("run-b: unknown=%+v omitted=%+v", plan.Unknown, plan.E2EOmitted)
	}
	// The consumer's own test file is a link target: a change to it reaches
	// the flow through the target itself.
	_, plan = e2eFlowsPlan(t, root, "lib/lib.go", "web/run-b.mjs")
	if got := unknownDetails(plan, affected.UnknownUnselectedBinaryConsumer); !reflect.DeepEqual(got, []string{"web/run-b.mjs"}) || len(plan.E2EOmitted) != 0 {
		t.Fatalf("run-b with its own file changed: unknown=%+v omitted=%+v", plan.Unknown, plan.E2EOmitted)
	}
}

// AFU-V1-1652: a changed path that reaches the command while no reviewed
// link explains it keeps the consumer and widens the plan with
// E2E_UNMAPPED_CHANGE naming the path; an unreviewed declared link still adds
// (run-a's flow is reached) but has no narrowing authority (run-b stays).
func TestAFUV11652UnmappedChangeKeepsTheConsumer(t *testing.T) {
	root := e2eFlowsRepo(t, false)
	_, plan := e2eFlowsPlan(t, root, "lib/lib.go")
	kept, ok := consumerSelection(plan, "web/run-a.mjs")
	if !ok || !reflect.DeepEqual(kept.Flows, []string{"tool-run"}) {
		t.Fatalf("run-a: ok=%v %+v", ok, kept)
	}
	if got := unknownDetails(plan, affected.UnknownE2EUnmappedChange); !reflect.DeepEqual(got, []string{"lib/lib.go"}) || plan.Scope != affected.ScopeUnknown || len(plan.E2EOmitted) != 0 {
		t.Fatalf("unknown=%+v scope=%s omitted=%+v", plan.Unknown, plan.Scope, plan.E2EOmitted)
	}
	if got := unknownDetails(plan, affected.UnknownUnselectedBinaryConsumer); !reflect.DeepEqual(got, []string{"web/run-b.mjs"}) {
		t.Fatalf("run-b: unknown=%+v", plan.Unknown)
	}
	if hint := plan.Unknown[len(plan.Unknown)-1].Hint; !strings.Contains(hint, "web/run-b.mjs runs") || !strings.Contains(hint, "AFU-V1-1652") {
		t.Fatalf("hint=%q", hint)
	}
}

// AFU-V1-1652: a changed file the command embeds has no owning unit, so the
// import walk does not list it; it still rebuilds the command, and no link
// explains it. Beside a changed path a reviewed link does explain, it keeps
// the consumer instead of letting the explained path alone leave it out.
func TestAFUV11652EmbeddedChangeKeepsTheConsumer(t *testing.T) {
	root := e2eFlowsRepo(t, true)
	writeRaw(t, root, "cmd/tool/asset.txt", []byte("asset\n"))
	writeRaw(t, root, "cmd/tool/asset.go", []byte("package main\n\nimport _ \"embed\"\n\n//go:embed asset.txt\nvar asset string\n"))
	commitAll(t, root, "embed an asset")
	_, plan := e2eFlowsPlan(t, root, "lib/lib.go")
	if len(plan.E2EOmitted) != 1 || plan.E2EOmitted[0].File != "web/run-b.mjs" {
		t.Fatalf("fixture: lib/lib.go alone omitted=%+v unknown=%+v", plan.E2EOmitted, plan.Unknown)
	}
	_, plan = e2eFlowsPlan(t, root, "cmd/tool/asset.txt", "lib/lib.go")
	if got := unknownDetails(plan, affected.UnknownE2EUnmappedChange); !reflect.DeepEqual(got, []string{"cmd/tool/asset.txt"}) || len(plan.E2EOmitted) != 0 || plan.Scope != affected.ScopeUnknown {
		t.Fatalf("unknown=%+v omitted=%+v scope=%s", plan.Unknown, plan.E2EOmitted, plan.Scope)
	}
	if got := unknownDetails(plan, affected.UnknownUnselectedBinaryConsumer); !reflect.DeepEqual(got, []string{"web/run-b.mjs"}) {
		t.Fatalf("run-b: unknown=%+v", plan.Unknown)
	}
}

// AFU-V1-1654: a global path runs every bound consumer the change reaches;
// an invalid declaration narrows nothing and is named; a plan without the
// declaration is returned unchanged.
func TestAFUV11654GlobalPathsInvalidDeclarationAndAbsence(t *testing.T) {
	root := e2eFlowsRepo(t, true)
	// `.env` is a global path: beside a change that reaches the command, no
	// consumer is left out, and the one no flow reaches names all its flows.
	_, plan := e2eFlowsPlan(t, root, ".env", "lib/lib.go")
	if kept, ok := consumerSelection(plan, "web/run-a.mjs"); !ok || !reflect.DeepEqual(kept.Flows, []string{"tool-run"}) {
		t.Fatalf("global run-a: ok=%v %+v", ok, kept)
	}
	if got := unknownDetails(plan, affected.UnknownUnselectedBinaryConsumer); !reflect.DeepEqual(got, []string{"web/run-b.mjs"}) || len(plan.E2EOmitted) != 0 {
		t.Fatalf("global run-b: unknown=%+v omitted=%+v", plan.Unknown, plan.E2EOmitted)
	}
	for _, u := range plan.Unknown {
		if u.Detail == "web/run-b.mjs" && !strings.Contains(u.Hint, "bound flows reached: other-flow") {
			t.Fatalf("global run-b hint=%q", u.Hint)
		}
	}

	writeRaw(t, root, E2EFlowsPath, []byte(`{"profile":"corvint-e2e-flows/0","flows":"flows","consumers":{"web/run-a.mjs":{"flows":["missing"]}}}`))
	before, plan := e2eFlowsPlan(t, root, "lib/lib.go")
	if got := unknownDetails(plan, affected.UnknownE2EFlowsInvalid); !reflect.DeepEqual(got, []string{E2EFlowsPath}) || !strings.Contains(plan.Unknown[len(plan.Unknown)-1].Hint, `binds flow "missing"`) {
		t.Fatalf("invalid: unknown=%+v", plan.Unknown)
	}
	if !reflect.DeepEqual(plan.Selected, before.Selected) || len(plan.E2EOmitted) != 0 || plan.Scope != affected.ScopeUnknown {
		t.Fatalf("invalid declaration narrowed: selected=%+v omitted=%+v", plan.Selected, plan.E2EOmitted)
	}
	for _, body := range []string{
		`{"profile":"corvint-e2e-flows/1","flows":"flows","consumers":{"web/run-a.mjs":{"flows":["tool-run"]}}}`,
		`{"profile":"corvint-e2e-flows/0","flows":"flows","consumers":{"web/none.mjs":{"flows":["tool-run"]}}}`,
		`{"profile":"corvint-e2e-flows/0","flows":"flows","consumers":{"web/run-a.mjs":{"flows":["tool-run"],"run":["x"]}}}`,
		`{"profile":"corvint-e2e-flows/0","flows":"flows","consumers":{"web/run-a.mjs":{"flows":["tool-run","alpha"]}}}`,
		`{"profile":"corvint-e2e-flows/0","flows":"../flows","consumers":{"web/run-a.mjs":{"flows":["tool-run"]}}}`,
	} {
		writeRaw(t, root, E2EFlowsPath, []byte(body))
		_, plan := e2eFlowsPlan(t, root, "lib/lib.go")
		if got := unknownDetails(plan, affected.UnknownE2EFlowsInvalid); !reflect.DeepEqual(got, []string{E2EFlowsPath}) {
			t.Errorf("%s: unknown=%+v", body, plan.Unknown)
		}
	}

	writeRaw(t, root, E2EFlowsPath, []byte(`{"profile":"corvint-e2e-flows/0","flows":"flows","consumers":{"web/run-a.mjs":{"flows":["tool-run"]},"web/run-b.mjs":{"flows":["other-flow"]}}}`))
	gitTest(t, root, "rm", "-q", "--cached", E2EFlowsPath)
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(E2EFlowsPath))); err != nil {
		t.Fatal(err)
	}
	before, plan = e2eFlowsPlan(t, root, "lib/lib.go")
	beforeJSON, _ := json.Marshal(before)
	planJSON, _ := json.Marshal(plan)
	if string(beforeJSON) != string(planJSON) {
		t.Fatalf("absent declaration changed the plan:\n%s\n%s", beforeJSON, planJSON)
	}
}
