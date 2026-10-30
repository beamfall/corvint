package capcatalog

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

var testWorkflow = WorkflowSource{
	Name: "demo", Summary: "a demo chain", Maturity: "experimental", Specs: []string{"DEMO-V0"}, Guide: "docs/DEMO.md",
	Steps: []WorkflowStepSource{
		{ID: "state", Help: "Write demo.json (demo-manifest/0).\n\nCommit:\n  demo.json at the root.\n"},
		{ID: "build", Command: "corvint demo build", Requires: []string{"state"}, Help: "Build the demo.\n\nUsage:\n  corvint [--root PATH] demo build --manifest FILE [--revision REV]\n      [--repo ALIAS=ROOT]\n  corvint demo build --manifest FILE --strict\n\nOptions:\n  --manifest FILE   the manifest\n\nExample:\n  corvint demo build --manifest demo.json\n"},
		{ID: "peek", Command: "corvint-test demo peek", Optional: true, Requires: []string{"build"}, Help: "Peek (demo-peek/0).\n\nUsage:\n  corvint-test demo peek --rev REV [--root DIR]\n"},
	},
}

// CAP-V0-030, CAP-V0-031: a workflow entry is derived from its source declaration and step help
// only: the chain holds the required steps in order, a repository state has no command, required
// flags are the options every usage form names outside brackets, and --root is a companion flag
// but corvint's global option.
func TestCAPV0031WorkflowDerivedFromStepHelp(t *testing.T) {
	workflow := BuildWorkflow(testWorkflow)
	if !slices.Equal(workflow.Chain, []string{"state", "build"}) || len(workflow.Steps) != 3 {
		t.Fatalf("chain = %v steps = %d", workflow.Chain, len(workflow.Steps))
	}
	state, build, peek := workflow.Steps[0], workflow.Steps[1], workflow.Steps[2]
	if state.Command != nil || state.HelpTopic != nil || len(state.Flags) != 0 || !slices.Equal(state.Commit, []string{"demo.json at the root."}) ||
		!slices.Equal(state.SchemaIDs, []string{"demo-manifest/0"}) {
		t.Fatalf("state step = %+v", state)
	}
	if *build.Command != "corvint demo build" || *build.HelpTopic != "corvint demo build --help" || build.Order != 2 ||
		!slices.Equal(build.RequiredFlags, []string{"--manifest"}) || slices.Contains(build.Flags, "--root") ||
		!slices.Contains(build.Flags, "--repo") || !slices.Contains(build.Flags, "--strict") || len(build.Usage) != 3 {
		t.Fatalf("build step = %+v", build)
	}
	if !peek.Optional || !slices.Contains(peek.Flags, "--root") || !slices.Equal(peek.RequiredFlags, []string{"--rev"}) {
		t.Fatalf("peek step = %+v", peek)
	}
	data, err := json.Marshal(workflow)
	if err != nil {
		t.Fatal(err)
	}
	for _, member := range []string{`"command":null`, `"helpTopic":null`, `"requires":[]`, `"options":[]`} {
		if !strings.Contains(string(data), member) {
			t.Fatalf("workflow JSON lacks %s: %s", member, data)
		}
	}
}

// CAP-V0-031: StepHelp prints the step's sections, the caller's options only when the step has
// none, and the step's place in the chain with the guide; an unknown step prints nothing.
func TestCAPV0031StepHelpRendersTheStep(t *testing.T) {
	workflow := BuildWorkflow(testWorkflow)
	build := StepHelp(workflow, "build", "  --ignored  not printed\n")
	for _, want := range []string{"Build the demo.\n\nUsage:\n  corvint [--root PATH] demo build", "\nOptions:\n  --manifest FILE   the manifest\n",
		"\nExample:\n  corvint demo build --manifest demo.json\n", "\nStep 2 of 3 (required; after state) in the demo chain:\n  state -> build\n",
		"Every step: corvint help --json (workflows). Guide: docs/DEMO.md\n"} {
		if !strings.Contains(build, want) {
			t.Fatalf("build help lacks %q:\n%s", want, build)
		}
	}
	if strings.Contains(build, "--ignored") {
		t.Fatal("step options were replaced by the caller's")
	}
	peek := StepHelp(workflow, "peek", "  --rev string\n    \trevision\n")
	if !strings.Contains(peek, "\nOptions:\n  --rev string\n") || !strings.Contains(peek, "(optional; after build)") {
		t.Fatalf("peek help:\n%s", peek)
	}
	if StepHelp(workflow, "absent", "") != "" {
		t.Fatal("an unknown step printed help")
	}
}

// CAP-V0-032: in the committed catalog every workflow step requires only earlier steps, so the
// order is an adoption order, and every required step after the first requires one.
func TestCAPV0032ShippedWorkflowRequiresEarlierSteps(t *testing.T) {
	var catalog Catalog
	if err := json.Unmarshal(Snapshot(), &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Workflows) == 0 {
		t.Fatal("the committed catalog has no workflows")
	}
	for _, workflow := range catalog.Workflows {
		seen := map[string]bool{}
		for index, step := range workflow.Steps {
			if step.Order != index+1 || seen[step.ID] {
				t.Fatalf("%s step %s order %d", workflow.Name, step.ID, step.Order)
			}
			if index > 0 && len(step.Requires) == 0 {
				t.Fatalf("%s step %s requires nothing", workflow.Name, step.ID)
			}
			for _, required := range step.Requires {
				if !seen[required] {
					t.Fatalf("%s step %s requires %s, which is not an earlier step", workflow.Name, step.ID, required)
				}
			}
			seen[step.ID] = true
		}
	}
}
