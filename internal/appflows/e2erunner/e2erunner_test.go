// SPDX-License-Identifier: Apache-2.0
package e2erunner

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/extevidence"
	"github.com/Beamfall/corvint/internal/jslinecov"
	"github.com/Beamfall/corvint/internal/runnerselection"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
)

const nativeReport = `{"config":{"rootDir":"/repo"},"suites":[{"specs":[{"id":"native","file":"a.spec.ts","title":"page","tests":[{"projectName":"chromium"}]}]}]}`

func runnerE2EFixture() Input {
	key := testrunner.InventoryKey{Checkout: strings.Repeat("a", 40), Runner: "playwright", ConfigDigest: strings.Repeat("b", 64), OS: "linux", Arch: "amd64"}
	d := jslinecov.Discovery{Key: key, Root: "/repo", Version: "1.63.0", Tracked: []string{"a.spec.ts"}, ConfigKnown: true, ListOK: true}
	r := runnerselection.Runner{Name: "playwright", Identity: testevidence.RunnerIdentity{Plugin: "playwright", PluginVersion: "1", RunnerVersion: d.Version, ConfigurationDigest: key.ConfigDigest, ExecutableDigest: strings.Repeat("c", 64)}, Execution: testevidence.ExecutionIdentity{OS: "linux", Architecture: "amd64"}, Pieces: testrunner.RunnerPieces{Inventory: testrunner.RunnerPiece{Present: true}, TestEvidence: testrunner.RunnerPiece{Present: true}, Qualification: testrunner.RunnerPiece{Reason: "unqualified"}}, Records: []runnerselection.Evidence{{Record: testevidence.New("repo", "missing-native", key.Checkout)}}}
	return Input{Discovery: d, Report: nativeReport, Selector: runnerselection.Input{Base: key.Checkout, Head: key.Checkout, Repository: "repo", DiffComplete: true, Runners: []runnerselection.Runner{r}}}
}

func selectFixture(in Input) appflows.E2ESelection {
	return appflows.SelectE2E(context.Background(), "/does-not-exist", appflows.E2EInput{Provider: "must-not-read-legacy-provider", Base: in.Selector.Base, RunnerEvidence: Bind(in)})
}

func TestRSPV0052E2ESafeConsumesRunnerRecords(t *testing.T) {
	out := selectFixture(runnerE2EFixture())
	if out.State != extevidence.SelectionFull || len(out.Selected) != 1 || out.Selected[0].Project != "chromium" || out.Selected[0].TestKey != jslinecov.TestID("playwright", "a.spec.ts", "chromium", "native") || len(out.OmittedTests) != 0 {
		t.Fatalf("%+v", out)
	}
	if len(out.Fallback) == 0 {
		t.Fatal("unqualified reason missing")
	}
}

// TestRSPV0052NoLegacyFallbackOnMalformedInventory also proves the one
// Playwright inventory is the only discovery: a separate or malformed list
// report is refused, never replaced by a legacy provider read.
func TestRSPV0052NoLegacyFallbackOnMalformedInventory(t *testing.T) {
	for _, raw := range []string{`{}`, `null`} {
		in := runnerE2EFixture()
		in.Report = raw
		out := selectFixture(in)
		if out.State != extevidence.SelectionFull || len(out.OmittedTests) != 0 || len(out.Selected) != 0 || len(out.Fallback) == 0 || out.Discovery.State != "MALFORMED" {
			t.Fatalf("%s: %+v", raw, out)
		}
	}
	in := runnerE2EFixture()
	in.Report = `{}`
	if out := selectFixture(in); out.Fallback[0].Subject != "list-output-malformed" {
		t.Fatalf("%+v", out)
	}
	in = runnerE2EFixture()
	in.Selector.Head = strings.Repeat("d", 40)
	if out := selectFixture(in); out.State != extevidence.SelectionFull || out.Fallback[0].Subject != "inventory-key-mismatch" {
		t.Fatal(out)
	}
}
