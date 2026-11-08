// SPDX-License-Identifier: Apache-2.0
package appflows

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/extevidence"
	"github.com/Beamfall/corvint/internal/jslinecov"
	"github.com/Beamfall/corvint/internal/runnerselection"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
)

func runnerE2EFixture() E2EInput {
	key := testrunner.InventoryKey{Checkout: strings.Repeat("a", 40), Runner: "playwright", ConfigDigest: strings.Repeat("b", 64), OS: "linux", Arch: "amd64"}
	d := jslinecov.Discovery{Key: key, Root: "/repo", Version: "1.63.0", Tracked: []string{"a.spec.ts"}, ConfigKnown: true, ListOK: true}
	r := runnerselection.Runner{Name: "playwright", Identity: testevidence.RunnerIdentity{Plugin: "playwright", PluginVersion: "1", RunnerVersion: d.Version, ConfigurationDigest: key.ConfigDigest, ExecutableDigest: strings.Repeat("c", 64)}, Execution: testevidence.ExecutionIdentity{OS: "linux", Architecture: "amd64"}, Pieces: testrunner.RunnerPieces{Inventory: testrunner.RunnerPiece{Present: true}, TestEvidence: testrunner.RunnerPiece{Present: true}, Qualification: testrunner.RunnerPiece{Reason: "unqualified"}}, Records: []runnerselection.Evidence{{Record: testevidence.New("repo", "missing-native", key.Checkout)}}}
	return E2EInput{Provider: "must-not-read-legacy-provider", Base: key.Checkout, RunnerEvidence: &E2ERunnerInput{Discovery: d, Report: `{"config":{"rootDir":"/repo"},"suites":[{"specs":[{"id":"native","file":"a.spec.ts","title":"page","tests":[{"projectName":"chromium"}]}]}]}`, Selector: runnerselection.Input{Base: key.Checkout, Head: key.Checkout, Repository: "repo", DiffComplete: true, Runners: []runnerselection.Runner{r}}}}
}
func TestRSPV0052E2ESafeConsumesRunnerRecords(t *testing.T) {
	in := runnerE2EFixture()
	out := SelectE2E(context.Background(), "/does-not-exist", in)
	if out.State != extevidence.SelectionFull || len(out.Selected) != 1 || out.Selected[0].Project != "chromium" || out.Selected[0].TestKey != jslinecov.TestID("playwright", "a.spec.ts", "chromium", "native") || len(out.OmittedTests) != 0 {
		t.Fatalf("%+v", out)
	}
	if len(out.Fallback) == 0 {
		t.Fatal("unqualified reason missing")
	}
}
func TestRSPV0052NoLegacyFallbackOnMalformedInventory(t *testing.T) {
	in := runnerE2EFixture()
	in.RunnerEvidence.Report = `{}`
	out := SelectE2E(context.Background(), "/does-not-exist", in)
	if out.State != extevidence.SelectionFull || len(out.OmittedTests) != 0 || len(out.Fallback) == 0 || out.Fallback[0].Subject != "list-output-malformed" {
		t.Fatalf("%+v", out)
	}
	in = runnerE2EFixture()
	in.RunnerEvidence.Selector.Head = strings.Repeat("d", 40)
	out = SelectE2E(context.Background(), "/does-not-exist", in)
	if out.State != extevidence.SelectionFull || out.Fallback[0].Subject != "inventory-key-mismatch" {
		t.Fatal(out)
	}
}
