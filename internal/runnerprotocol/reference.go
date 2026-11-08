// SPDX-License-Identifier: Apache-2.0
package runnerprotocol

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/native"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

// Reference is a compiled-in, unqualified Go runner example. Its subprocess
// twin uses this exact SPI implementation. Inventory input is a core-captured
// {config_digest,tests:[{id,path}]} artifact; Parse reuses the existing Go parser.
// It always describes a superset command and produces absent native components.
// It is not registered into the production hot path or the release manifest.
type Reference struct{}

const ReferenceRunner = "reference-go-test"

func (Reference) Pieces() testrunner.RunnerPieces {
	p := testrunner.AbsentRunnerPieces(ReferenceRunner)
	p.Inventory = testrunner.RunnerPiece{Present: true}
	p.TestEvidence = testrunner.RunnerPiece{Present: true}
	return p
}

func readArtifact(path string) ([]byte, error) {
	f, err := openRegular(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err == nil && len(b) > 1<<20 {
		err = fmt.Errorf("artifact-cap-exceeded")
	}
	return b, err
}
func (Reference) Inventory(_ context.Context, r registry.InventoryRequest) (testrunner.RunnerInventory, error) {
	b, err := readArtifact(r.Output)
	if err != nil {
		return testrunner.RunnerInventory{}, err
	}
	var result inventoryResult
	if err = validatePayload("inventory", b, true, &result); err != nil {
		return testrunner.RunnerInventory{}, err
	}
	out := testrunner.RunnerInventory{Key: r.Key, Complete: true, Tests: []testrunner.InventoryTest{}}
	out.Key.ConfigDigest = result.ConfigDigest
	for _, t := range result.Tests {
		out.Tests = append(out.Tests, testrunner.InventoryTest{ID: t.ID, Path: t.Path})
	}
	return out, nil
}
func (Reference) Command(_ context.Context, r registry.CommandRequest) (registry.CommandResponse, error) {
	if r.Key.Runner != ReferenceRunner || r.RunDir == "" {
		return registry.CommandResponse{}, fmt.Errorf("reference-input-invalid")
	}
	if r.Purpose != "inventory" && r.Purpose != "run" && r.Purpose != "collect" {
		return registry.CommandResponse{}, fmt.Errorf("reference-purpose-invalid")
	}
	argv := []string{"go", "test", "-json", "./..."}
	if r.Purpose == "inventory" {
		argv = []string{"go", "test", "-list", ".", "./..."}
	}
	return registry.CommandResponse{Argv: argv, Cwd: ".", Env: map[string]string{}, Artifacts: []string{}, Superset: true}, nil
}
func (Reference) Parse(_ context.Context, r registry.ParseRequest) ([]testrunner.RunnerResult, error) {
	if r.Runner != ReferenceRunner {
		return nil, fmt.Errorf("undeclared-runner")
	}
	b, err := readArtifact(r.Report)
	if err != nil {
		return nil, err
	}
	observation, err := native.Parse(testrunner.Input{Runner: "go-test", Stdout: b})
	if err != nil {
		return nil, err
	}
	if !observation.Complete {
		// The report carries no exit code, so a failed test reads as an exit
		// contradiction; keep its validated failed rows alongside the error so a
		// passing retry cannot hide them. Passes and skips stay unobserved.
		failed := []testrunner.RunnerResult{}
		for _, test := range observation.Tests {
			if test.State == testrunner.Failed {
				failed = append(failed, testrunner.RunnerResult{ID: test.ID, Outcome: "failed", Reason: test.Message})
			}
		}
		return failed, fmt.Errorf("reference-report-incomplete")
	}
	results := []testrunner.RunnerResult{}
	for _, test := range observation.Tests {
		outcome := ""
		switch test.State {
		case testrunner.Passed:
			outcome = "passed"
		case testrunner.Failed:
			outcome = "failed"
		case testrunner.Skipped:
			outcome = "skipped"
		default:
			return nil, fmt.Errorf("reference-outcome-unknown")
		}
		reason := test.Message
		if outcome == "skipped" && reason == "" {
			reason = "native-skip"
		}
		results = append(results, testrunner.RunnerResult{ID: test.ID, Outcome: outcome, Reason: reason})
	}
	return results, nil
}
func (Reference) Evidence(_ context.Context, r registry.EvidenceRequest) ([]registry.NativeEvidence, error) {
	if r.Runner != ReferenceRunner || len(r.Artifacts) != 1 {
		return nil, fmt.Errorf("reference-artifacts-invalid")
	}
	// The artifact is the inventory, so every inventoried test explicitly keeps
	// its native components absent; the example cannot create false coverage.
	in, err := (Reference{}).Inventory(context.Background(), registry.InventoryRequest{Key: testrunner.InventoryKey{Runner: r.Runner}, Output: r.Artifacts[0]})
	if err != nil {
		return nil, err
	}
	executed, _ := json.Marshal(testevidence.Missing[testevidence.ExecutedFile]("reference-no-coverage"))
	regions, _ := json.Marshal(testevidence.Missing[testevidence.RegionsFile]("reference-no-regions"))
	loaded, _ := json.Marshal(testevidence.Missing[testevidence.Blob]("reference-no-build-graph"))
	out := []registry.NativeEvidence{}
	for _, test := range in.Tests {
		out = append(out, registry.NativeEvidence{Test: test.ID, Executed: executed, Regions: regions, Loaded: loaded})
	}
	return out, nil
}
