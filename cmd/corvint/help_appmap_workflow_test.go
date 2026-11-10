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

// CAP-V0-030: `corvint flows appmap build --help` and `-h` print the build step's own usage,
// inputs, commits, options and example, and its place in the application-map chain, and exit 0.
func TestCAPV0030FlowsAppmapBuildHelp(t *testing.T) {
	t.Parallel()
	for _, word := range []string{"--help", "-h"} {
		got := runCatalogHelp(t, "flows", "appmap", "build", word)
		if got != helpText(flowsAppmapBuildHelpTopic) {
			t.Fatalf("flows appmap build %s does not print the build step help:\n%s", word, got)
		}
		for _, want := range []string{"\nUsage:\n  corvint [--root PATH] flows appmap build --manifest FILE", "\nInputs:\n", "\nCommit:\n",
			"\nOptions:\n  --manifest FILE", "\nExample:\n  corvint flows appmap build --manifest appmap.json", "application-map-manifest/0",
			"docs/specs/application-map-v0.md", "Step 2 of 2 (required; after manifest)", "Guide: docs/APPLICATION-MAP.md"} {
			if !strings.Contains(got, want) {
				t.Fatalf("flows appmap build %s lacks %q:\n%s", word, want, got)
			}
		}
		if strings.Contains(got, "corvint-test") {
			t.Fatalf("flows appmap build %s still names the retired corvint-test companion:\n%s", word, got)
		}
	}
	// The rest of the appmap group is corvint-docs's (decision 0595): corvint's node names the build step alone.
	if got := runCatalogHelp(t, "flows", "appmap", "--help"); !strings.Contains(got, "corvint [--root PATH] flows appmap build --manifest FILE") {
		t.Fatalf("flows appmap help does not name the build step:\n%s", got)
	}
}

// CAP-V0-030, CAP-V0-032: `corvint help --json` carries the application-map workflow, its
// required chain in adoption order, and every step's flags and help topic.
func TestCAPV0030HelpJSONCarriesTheAppmapWorkflow(t *testing.T) {
	t.Parallel()
	var catalog capcatalog.Catalog
	if err := json.Unmarshal([]byte(runCatalogHelp(t, "help", "--json")), &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Workflows) != 1 || catalog.Workflows[0].Name != "application-map" {
		t.Fatalf("workflows = %+v", catalog.Workflows)
	}
	workflow := catalog.Workflows[0]
	if !slices.Equal(workflow.Chain, []string{"manifest", "appmap-build"}) || workflow.Guide != "docs/APPLICATION-MAP.md" {
		t.Fatalf("chain = %v, guide = %q", workflow.Chain, workflow.Guide)
	}
	manifest, build := workflow.Steps[0], workflow.Steps[1]
	if manifest.Command != nil || manifest.HelpTopic != nil || len(manifest.Flags) != 0 {
		t.Fatalf("repository state manifest carries a command: %+v", manifest)
	}
	if build.Command == nil || *build.Command != "corvint flows appmap build" || *build.HelpTopic != "corvint flows appmap build --help" {
		t.Fatalf("appmap-build command = %v", build.Command)
	}
	if len(build.Usage) == 0 || len(build.Example) == 0 || !slices.Equal(build.RequiredFlags, []string{"--manifest"}) || slices.Contains(build.Flags, "--root") {
		t.Fatalf("appmap-build usage %v example %v required %v flags %v", build.Usage, build.Example, build.RequiredFlags, build.Flags)
	}
}

// CAP-V0-030: the adoption guide names every flag a command step of the chain declares, so the
// guide cannot drift from the step help (GitHub #1850).
func TestCAPV0030AdoptionGuideNamesStepFlags(t *testing.T) {
	t.Parallel()
	guide, err := os.ReadFile(filepath.Join(moduleRoot(t), filepath.FromSlash(appmapAdoptionGuide)))
	if err != nil {
		t.Fatal(err)
	}
	commands := 0
	for _, step := range capcatalog.BuildWorkflow(appmapAdoptionWorkflow).Steps {
		if step.Command == nil {
			continue
		}
		commands++
		for _, flag := range step.Flags {
			// A whole flag name: --repo must not be satisfied by --repo-revision.
			if !regexp.MustCompile(`(?:^|[^\w-])` + regexp.QuoteMeta(flag) + `(?:[^\w-]|$)`).Match(guide) {
				t.Errorf("%s does not name %s, which step %s (%s) declares", appmapAdoptionGuide, flag, step.ID, *step.Command)
			}
		}
	}
	if commands == 0 {
		t.Fatal("the application-map workflow declares no command step")
	}
}
