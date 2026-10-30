package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/capcatalog"
)

// CCE-V0-118: `corvint-test e2e SUBCOMMAND --help` and `-h` print that step's usage, inputs,
// commits, options and example from the committed catalog, and exit 0. Every option the
// subcommand's flag set declares is a flag of its catalog step, and the other way round, and every
// schema the printed help names is one of the step's schemaIds, so the help, the parser and
// `corvint help --json` cannot drift apart. how-accept and how-evidence, outside the chain, print
// their own usage, inputs, options and example.
func TestCCEV0118SubcommandHelp(t *testing.T) {
	var catalog capcatalog.Catalog
	if err := json.Unmarshal(capcatalog.Snapshot(), &catalog); err != nil {
		t.Fatal(err)
	}
	steps := map[string]capcatalog.WorkflowStep{}
	for _, step := range catalog.Workflows[0].Steps {
		steps[step.ID] = step
	}
	option := regexp.MustCompile(`(?m)^  (--[a-z][a-z0-9-]*)`)
	schema := regexp.MustCompile(`(?:^|[\s(,])([a-z][a-z0-9]*(?:[-.][a-z0-9]+)*/[0-9]+(?:\.[0-9]+)?)(?:$|[\s),;.])`)
	for _, sub := range []string{"catalog", "harvest", "obligations", "compile", "prove", "shadow"} {
		step := steps["e2e-"+sub]
		for _, word := range []string{"--help", "-h"} {
			var out bytes.Buffer
			if err := run(context.Background(), []string{"e2e", sub, word}, &out); err != nil {
				t.Fatalf("e2e %s %s: %v", sub, word, err)
			}
			got := out.String()
			for _, want := range []string{"\nUsage:\n  corvint-test e2e " + sub, "\nInputs:\n", "\nProduces:\n", "\nOptions:\n",
				"\nExample:\n  corvint-test e2e " + sub, "chain:\n  manifest -> appmap-build", "Guide: docs/E2E-ADOPTION.md"} {
				if !strings.Contains(got, want) {
					t.Fatalf("e2e %s %s lacks %q:\n%s", sub, word, want, got)
				}
			}
			_, options, _ := strings.Cut(got, "\nOptions:\n")
			options, _, _ = strings.Cut(options, "\n\n")
			var declared []string
			for _, match := range option.FindAllStringSubmatch(options, -1) {
				declared = append(declared, match[1])
			}
			slices.Sort(declared)
			want := slices.Clone(step.Flags)
			slices.Sort(want)
			if !slices.Equal(declared, want) {
				t.Fatalf("e2e %s: flag set declares %v, catalog step names %v", sub, declared, want)
			}
			for _, match := range schema.FindAllStringSubmatch(got, -1) {
				if !slices.Contains(step.SchemaIDs, match[1]) {
					t.Fatalf("e2e %s help names %s, catalog step schemaIds are %v", sub, match[1], step.SchemaIDs)
				}
			}
		}
	}
	for _, sub := range []string{"how-accept", "how-evidence"} {
		for _, word := range []string{"--help", "-h"} {
			var out bytes.Buffer
			if err := run(context.Background(), []string{"e2e", sub, word}, &out); err != nil {
				t.Fatalf("e2e %s %s: %v", sub, word, err)
			}
			got := out.String()
			for _, want := range []string{"\nUsage:\n  corvint-test e2e " + sub + " --answer FILE", "\nInputs:\n", "\nProduces:\n",
				"\nExample:\n  corvint-test e2e " + sub, "\nOptions:\n  --answer string"} {
				if !strings.Contains(got, want) {
					t.Fatalf("e2e %s %s lacks %q:\n%s", sub, word, want, got)
				}
			}
		}
	}
	for _, args := range [][]string{{"--help"}, {"-h"}, {"e2e", "--help"}, {"e2e", "-h"}} {
		var out bytes.Buffer
		if err := run(context.Background(), args, &out); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if got := out.String(); !strings.HasPrefix(got, usage) || !strings.Contains(got, "manifest -> appmap-build -> commit-manifest -> e2e-catalog") ||
			!strings.Contains(got, "Guide: docs/E2E-ADOPTION.md") {
			t.Fatalf("%v group help:\n%s", args, got)
		}
	}
}

// CCE-V0-121: the catalog and obligations/compile help, and the adoption guide, answer the gaps an
// adopter otherwise reads from source: the manifest is not a built map, flows supply outcomes,
// exit 0 on UNKNOWN/INCOMPLETE means read `.state`.
func TestCCEV0121HelpAnswersAdopterGaps(t *testing.T) {
	helpOf := func(sub string) string {
		var out bytes.Buffer
		if err := run(context.Background(), []string{"e2e", sub, "--help"}, &out); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	catalog := strings.Join(strings.Fields(helpOf("catalog")), " ")
	for _, want := range []string{"not a built", "application-flow-intent/1", "outcome blocks", "UNKNOWN or INCOMPLETE"} {
		if !strings.Contains(catalog, want) {
			t.Fatalf("catalog help lacks %q:\n%s", want, catalog)
		}
	}
	for _, sub := range []string{"obligations", "compile"} {
		// V1-2734 words the obligations step as gating CI on the top-level .state.
		got := strings.Join(strings.Fields(helpOf(sub)), " ")
		readsState := strings.Contains(got, "Read .state") || strings.Contains(got, "Gate CI on the top-level .state")
		if !readsState || !strings.Contains(got, "UNKNOWN") || !strings.Contains(got, "exits 0") || !strings.Contains(got, "exit code") {
			t.Fatalf("%s help lacks the .state reading:\n%s", sub, got)
		}
	}
	guide, err := os.ReadFile("../../docs/E2E-ADOPTION.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"corvint affected", "flows appmap impact", "read `.state`", "not a built map"} {
		if !strings.Contains(string(guide), want) {
			t.Fatalf("guide lacks %q", want)
		}
	}
}
