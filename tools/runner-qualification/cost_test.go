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
	"time"

	q "github.com/Beamfall/corvint/internal/runnerqualification"
	s "github.com/Beamfall/corvint/internal/runnerselection"
	"github.com/Beamfall/corvint/internal/testrunner"
)

type costDriver struct{ captures []q.RetainedCapture }

func (*costDriver) Verify(context.Context, q.Declaration, string, []q.Merge) error { return nil }
func (*costDriver) Population(context.Context, q.Declaration, string, s.Class) (q.Population, error) {
	return q.FilePopulation([]q.FileTarget{{Path: "data", Bytes: []byte("x"), Class: s.DataEdit, Kind: "data"}}), nil
}
func (d *costDriver) Capture(_ context.Context, decl q.Declaration, _ string, e q.Experiment) (q.Pair, error) {
	outcome := "passed"
	if e.Mutant != nil {
		outcome = "failed"
	}
	o := q.Observation{
		Inventory: testrunner.RunnerInventory{
			Key:      testrunner.InventoryKey{Checkout: e.Commit, Runner: decl.Runner, ConfigDigest: decl.Identity.Runner.ConfigurationDigest, OS: decl.Identity.Execution.OS, Arch: decl.Identity.Execution.Architecture},
			Complete: true, Tests: []testrunner.InventoryTest{{ID: "a", Path: "a.test"}},
		},
		Results: []testrunner.RunnerResult{{ID: "a", Outcome: outcome}}, Plan: testrunner.RunSet{Full: true},
		TimingsObserved: true, InventoryWall: time.Millisecond, CollectionWall: time.Millisecond,
	}
	p := q.Pair{Traced: o, Untraced: o}
	d.captures = append(d.captures, q.RetainedCapture{Experiment: e, Pair: p})
	return p, nil
}

// RSP-V0-036/056: the trusted CLI ceiling must not overwrite retained costs.
func TestRSPV0056CommandPreservesExecutionBudget(t *testing.T) {
	root, replay := pinnedFixture(t)
	driver := &costDriver{}
	live := q.Run(context.Background(), replay.Request, driver)
	if !live.Complete || len(live.Reasons) != 0 || len(live.Classes) != 1 || !live.Classes[0].Passed {
		t.Fatalf("invalid fixture: %+v", live)
	}
	population, err := driver.Population(context.Background(), replay.Request.Declaration, root, s.DataEdit)
	if err != nil {
		t.Fatal(err)
	}
	replay.Populations = []q.ClassPopulation{{Class: s.DataEdit, Population: population}}
	replay.Captures = driver.captures
	for _, tc := range []struct {
		name, reason       string
		budget, collection time.Duration
		rc                 int
	}{
		{"within-ceiling", "", 30 * time.Second, time.Millisecond, 0},
		{"original-budget-over-ceiling", "qualification-cost-ceiling-exceeded", time.Hour, time.Millisecond, 1},
		{"original-wall-over-budget", "qualification-budget-exceeded", time.Minute, time.Hour, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inputReplay := replay
			inputReplay.Request.Budget = tc.budget
			inputReplay.Captures = slices.Clone(replay.Captures)
			inputReplay.Captures[0].Pair.Traced.CollectionWall = tc.collection
			b, err := json.Marshal(inputReplay)
			if err != nil {
				t.Fatal(err)
			}
			input, output := filepath.Join(t.TempDir(), "input.json"), filepath.Join(t.TempDir(), "output.json")
			if err = os.WriteFile(input, b, 0600); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			if rc := run(context.Background(), []string{"replay", "-input", input, "-checkout", root, "-out", output, "-budget", "1m"}, &stdout, &stderr); rc != tc.rc || stderr.Len() != 0 {
				t.Fatalf("rc=%d want=%d out=%s err=%s", rc, tc.rc, &stdout, &stderr)
			}
			b, err = os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			var record q.Record
			if err = json.Unmarshal(b, &record); err != nil {
				t.Fatal(err)
			}
			wantWall := time.Duration(len(replay.Captures))*4*time.Millisecond + tc.collection - time.Millisecond
			if record.Budget != tc.budget || record.Wall != wantWall || (tc.reason != "" && !slices.Contains(record.Reasons, tc.reason)) {
				t.Fatalf("execution cost replaced: budget=%s wall=%s reasons=%v", record.Budget, record.Wall, record.Reasons)
			}
		})
	}
}
