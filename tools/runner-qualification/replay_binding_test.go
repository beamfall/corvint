// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	q "github.com/Beamfall/corvint/internal/runnerqualification"
	s "github.com/Beamfall/corvint/internal/runnerselection"
	"github.com/Beamfall/corvint/internal/testrunner"
)

// RSP-V0-034: verifying merge ancestry cannot supply a missing capture's plan base.
func TestRSPV0034CommandRefusesUnpinnedReplayPlan(t *testing.T) {
	root, replay := pinnedFixture(t)
	d := replay.Request.Declaration
	m := replay.Request.Merges[0]
	for _, experiment := range []q.Experiment{
		{Commit: d.Corpus.Commit},
		{Commit: d.Corpus.Commit, Control: s.DataEdit},
		{Commit: m.Base},
		{Commit: m.Head}, // legacy capture lacks Base despite valid Git merge pins
	} {
		o := q.Observation{
			Inventory: testrunner.RunnerInventory{
				Key:      testrunner.InventoryKey{Checkout: experiment.Commit, Runner: d.Runner, ConfigDigest: d.Identity.Runner.ConfigurationDigest, OS: d.Identity.Execution.OS, Arch: d.Identity.Execution.Architecture},
				Complete: true, Tests: []testrunner.InventoryTest{{ID: "a", Path: "a.test"}},
			},
			Results: []testrunner.RunnerResult{{ID: "a", Outcome: "passed"}},
			Plan:    testrunner.RunSet{Full: true}, TimingsObserved: true,
		}
		replay.Captures = append(replay.Captures, q.RetainedCapture{Experiment: experiment, Pair: q.Pair{Traced: o, Untraced: o}})
	}
	b, err := json.Marshal(replay)
	if err != nil {
		t.Fatal(err)
	}
	input, output := filepath.Join(t.TempDir(), "input.json"), filepath.Join(t.TempDir(), "output.json")
	if err = os.WriteFile(input, b, 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if rc := run(context.Background(), []string{"replay", "-input", input, "-checkout", root, "-out", output, "-budget", "1m"}, &stdout, &stderr); rc != 1 {
		t.Fatalf("unpinned replay exit %d: %s %s", rc, stdout.String(), stderr.String())
	}
	b, err = os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var record q.Record
	if err = json.Unmarshal(b, &record); err != nil {
		t.Fatal(err)
	}
	if len(record.Classes) != 1 || record.Classes[0].Passed || record.Classes[0].ReplayPassed || !slices.Contains(record.Classes[0].Reasons, "capture:retained-replay-base-unpinned") {
		t.Fatalf("unpinned head plan not refused: %+v", record)
	}
}
