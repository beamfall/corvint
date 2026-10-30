package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/capcatalog"
)

// CCE-V0-118: `corvint flows appmap build --help` and `-h` print the build step's own usage,
// inputs, commits, options and example, and its place in the E2E chain, and exit 0.
func TestCCEV0118FlowsAppmapBuildHelp(t *testing.T) {
	t.Parallel()
	for _, word := range []string{"--help", "-h"} {
		got := runCatalogHelp(t, "flows", "appmap", "build", word)
		if got != helpText(flowsAppmapBuildHelpTopic) {
			t.Fatalf("flows appmap build %s does not print the build step help:\n%s", word, got)
		}
		for _, want := range []string{"\nUsage:\n  corvint [--root PATH] flows appmap build --manifest FILE", "\nInputs:\n", "\nCommit:\n",
			"\nOptions:\n  --manifest FILE", "\nExample:\n  corvint flows appmap build --manifest appmap.json", "application-map-manifest/0",
			"docs/specs/application-map-v0.md", "Step 2 of 9 (required; after manifest)", "Guide: docs/E2E-ADOPTION.md"} {
			if !strings.Contains(got, want) {
				t.Fatalf("flows appmap build %s lacks %q:\n%s", word, want, got)
			}
		}
	}
	if got := runCatalogHelp(t, "flows", "appmap", "--help"); !strings.Contains(got, "corvint flows appmap build --help") {
		t.Fatal("flows appmap help does not point at the build step help")
	}
}

// CAP-V0-030, CAP-V0-032: `corvint help --json` carries the change-compiled E2E workflow, its
// required chain in adoption order, and every step's flags and help topic.
func TestCAPV0030HelpJSONCarriesTheE2EWorkflow(t *testing.T) {
	t.Parallel()
	var catalog capcatalog.Catalog
	if err := json.Unmarshal([]byte(runCatalogHelp(t, "help", "--json")), &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Workflows) != 1 || catalog.Workflows[0].Name != "change-compiled-e2e" {
		t.Fatalf("workflows = %+v", catalog.Workflows)
	}
	workflow := catalog.Workflows[0]
	wantChain := []string{"manifest", "appmap-build", "commit-manifest", "e2e-catalog", "e2e-obligations", "e2e-compile", "e2e-prove", "e2e-shadow"}
	if !slices.Equal(workflow.Chain, wantChain) || workflow.Guide != "docs/E2E-ADOPTION.md" {
		t.Fatalf("chain = %v, guide = %q", workflow.Chain, workflow.Guide)
	}
	commands := map[string]string{"appmap-build": "corvint flows appmap build", "e2e-catalog": "corvint-test e2e catalog",
		"e2e-harvest": "corvint-test e2e harvest", "e2e-obligations": "corvint-test e2e obligations",
		"e2e-compile": "corvint-test e2e compile", "e2e-prove": "corvint-test e2e prove", "e2e-shadow": "corvint-test e2e shadow"}
	for _, step := range workflow.Steps {
		want, isCommand := commands[step.ID]
		switch {
		case !isCommand && (step.Command != nil || step.HelpTopic != nil || len(step.Flags) != 0):
			t.Fatalf("repository state %s carries a command: %+v", step.ID, step)
		case isCommand && (step.Command == nil || *step.Command != want || *step.HelpTopic != want+" --help"):
			t.Fatalf("step %s command = %v", step.ID, step.Command)
		case isCommand && (len(step.Usage) == 0 || len(step.Example) == 0 || len(step.Flags) == 0):
			t.Fatalf("step %s lacks usage, example or flags: %+v", step.ID, step)
		}
	}
	build := workflow.Steps[1]
	if !slices.Equal(build.RequiredFlags, []string{"--manifest"}) || slices.Contains(build.Flags, "--root") {
		t.Fatalf("appmap-build required %v flags %v", build.RequiredFlags, build.Flags)
	}
	if !slices.Contains(workflow.Steps[7].Flags, "--hosted-test") || !slices.Equal(workflow.Steps[7].RequiredFlags, []string{"--base", "--head", "--out"}) {
		t.Fatalf("e2e-prove flags %v required %v", workflow.Steps[7].Flags, workflow.Steps[7].RequiredFlags)
	}
}

// CCE-V0-120: the adoption guide names every flag a command step of the E2E chain declares, so
// the guide cannot drift from the step help again (GitHub #1850).
func TestCCEV0120AdoptionGuideNamesStepFlags(t *testing.T) {
	t.Parallel()
	guide, err := os.ReadFile(filepath.Join(moduleRoot(t), filepath.FromSlash(e2eAdoptionGuide)))
	if err != nil {
		t.Fatal(err)
	}
	commands := 0
	for _, step := range capcatalog.BuildWorkflow(e2eAdoptionWorkflow).Steps {
		if step.Command == nil {
			continue
		}
		commands++
		for _, flag := range step.Flags {
			// A whole flag name: --repo must not be satisfied by --repo-revision.
			if !regexp.MustCompile(`(?:^|[^\w-])` + regexp.QuoteMeta(flag) + `(?:[^\w-]|$)`).Match(guide) {
				t.Errorf("%s does not name %s, which step %s (%s) declares", e2eAdoptionGuide, flag, step.ID, *step.Command)
			}
		}
	}
	if commands == 0 {
		t.Fatal("the E2E workflow declares no command step")
	}
}

// V1-2734 (CCE-V0-004, CCE-V0-019): the obligations step help gates CI on the document's
// top-level .state, the only member that reads UNKNOWN, as docs/E2E-ADOPTION.md does; it no
// longer implies a per-obligation UNKNOWN state.
func TestV12734E2EObligationsHelpGatesOnTheTopLevelState(t *testing.T) {
	t.Parallel()
	var help string
	for _, step := range e2eAdoptionWorkflow.Steps {
		if step.ID == "e2e-obligations" {
			help = step.Help
		}
	}
	for _, want := range []string{"Gate CI on the top-level\n  .state (REACHED, NONE or UNKNOWN), never on the exit code",
		"Only the top-level .state reads UNKNOWN."} {
		if !strings.Contains(help, want) {
			t.Fatalf("e2e obligations help lacks %q:\n%s", want, help)
		}
	}
	if strings.Contains(help, "each obligation's state") {
		t.Fatalf("e2e obligations help still names a per-obligation state:\n%s", help)
	}
	guide, err := os.ReadFile(filepath.Join("..", "..", "docs", "E2E-ADOPTION.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(guide), "gate on the document's top-level `state`") {
		t.Fatal("docs/E2E-ADOPTION.md no longer gates CI on the top-level state")
	}
}
