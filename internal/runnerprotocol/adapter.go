// SPDX-License-Identifier: Apache-2.0
package runnerprotocol

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

type inventoryResult struct {
	ConfigDigest string          `json:"config_digest"`
	Tests        []inventoryTest `json:"tests"`
}
type inventoryTest struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}
type commandResult struct {
	Argv      []string          `json:"argv"`
	Cwd       string            `json:"cwd"`
	Env       map[string]string `json:"env"`
	Artifacts []string          `json:"artifacts"`
	Superset  bool              `json:"superset"`
}
type parseResult struct {
	Results []testResult `json:"results"`
}
type testResult struct {
	ID      string `json:"id"`
	Outcome string `json:"outcome"`
	Reason  string `json:"reason,omitempty"`
}
type evidenceResult struct {
	Records []nativeRecord `json:"records"`
}
type nativeRecord struct {
	Test     string                                            `json:"test"`
	Executed testevidence.Component[testevidence.ExecutedFile] `json:"executed"`
	Regions  testevidence.Component[testevidence.RegionsFile]  `json:"regions"`
	Loaded   testevidence.Component[testevidence.Blob]         `json:"loaded"`
}

func (a *adapter) Inventory(ctx context.Context, in registry.InventoryRequest) (testrunner.RunnerInventory, error) {
	s := a.session
	s.mu.Lock()
	defer s.mu.Unlock()
	empty := testrunner.RunnerInventory{}
	if in.Key.Runner != a.runner {
		return empty, s.fail(fmt.Errorf("undeclared-runner"))
	}
	if !s.allowedPath(in.Output) {
		return empty, s.fail(fmt.Errorf("schema-violation:output-path"))
	}
	var result inventoryResult
	if err := s.call(ctx, "inventory", map[string]any{"runner": a.runner, "output": in.Output}, &result); err != nil {
		return empty, err
	}
	out := testrunner.RunnerInventory{Key: in.Key, Complete: true, Tests: []testrunner.InventoryTest{}}
	out.Key.ConfigDigest = result.ConfigDigest
	for _, t := range result.Tests {
		out.Tests = append(out.Tests, testrunner.InventoryTest{ID: t.ID, Path: t.Path})
	}
	out, err := testrunner.CanonicalInventory(out, in.Key)
	if err != nil {
		return empty, s.fail(err)
	}
	return out, nil
}
func (a *adapter) Command(ctx context.Context, in registry.CommandRequest) (registry.CommandResponse, error) {
	s := a.session
	s.mu.Lock()
	defer s.mu.Unlock()
	empty := registry.CommandResponse{}
	if in.Key.Runner != a.runner {
		return empty, s.fail(fmt.Errorf("undeclared-runner"))
	}
	if filepath.Clean(in.RunDir) != s.options.RunDir {
		return empty, s.fail(fmt.Errorf("schema-violation:run-directory"))
	}
	params := map[string]any{"runner": a.runner, "purpose": in.Purpose, "run_dir": in.RunDir}
	if in.Tests != nil {
		params["tests"] = in.Tests
	}
	var result commandResult
	if err := s.call(ctx, "command", params, &result); err != nil {
		return empty, err
	}
	if len(result.Argv) == 0 || result.Argv[0] == "" || !s.allowedPath(result.Cwd) {
		return empty, s.fail(fmt.Errorf("schema-violation:command"))
	}
	for _, arg := range result.Argv {
		if strings.ContainsRune(arg, 0) {
			return empty, s.fail(fmt.Errorf("schema-violation:argv"))
		}
	}
	for k, v := range result.Env {
		if k == "" || strings.ContainsAny(k, "=\x00") || strings.ContainsRune(v, 0) {
			return empty, s.fail(fmt.Errorf("schema-violation:environment"))
		}
	}
	for _, path := range result.Artifacts {
		if !s.allowedPath(path) {
			return empty, s.fail(fmt.Errorf("schema-violation:artifact-path"))
		}
	}
	return registry.CommandResponse{Argv: result.Argv, Cwd: result.Cwd, Env: result.Env, Artifacts: result.Artifacts, Superset: result.Superset}, nil
}
func (a *adapter) Parse(ctx context.Context, in registry.ParseRequest) ([]testrunner.RunnerResult, error) {
	s := a.session
	s.mu.Lock()
	defer s.mu.Unlock()
	if in.Runner != a.runner {
		return nil, s.fail(fmt.Errorf("undeclared-runner"))
	}
	if !s.allowedPath(in.Report) {
		return nil, s.fail(fmt.Errorf("schema-violation:report-path"))
	}
	var result parseResult
	if err := s.call(ctx, "parse", map[string]any{"runner": a.runner, "report": in.Report}, &result); err != nil {
		return nil, err
	}
	if len(result.Results) > testrunner.MaxTests {
		return nil, s.fail(fmt.Errorf("schema-violation:result-cap"))
	}
	out := []testrunner.RunnerResult{}
	seen := map[string]bool{}
	for _, r := range result.Results {
		if r.ID == "" || seen[r.ID] || r.Outcome == "skipped" && r.Reason == "" {
			return nil, s.fail(fmt.Errorf("schema-violation:results"))
		}
		seen[r.ID] = true
		out = append(out, testrunner.RunnerResult{ID: r.ID, Outcome: r.Outcome, Reason: r.Reason})
	}
	return out, nil
}
func evidenceKey(runner string, artifacts []string) string {
	b, _ := json.Marshal(struct {
		Runner    string
		Artifacts []string
	}{runner, artifacts})
	return string(b)
}

// Evidence stages the reply and returns evidence-pending-session-close until
// successful shutdown. Calling with the same arguments after Close retrieves a
// deep copy. Failure at any point discards every runner's staged results.
func (a *adapter) Evidence(ctx context.Context, in registry.EvidenceRequest) ([]registry.NativeEvidence, error) {
	s := a.session
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failure != "" {
		return nil, fmt.Errorf("%s", s.failure)
	}
	if in.Runner != a.runner {
		return nil, s.fail(fmt.Errorf("undeclared-runner"))
	}
	key := evidenceKey(in.Runner, in.Artifacts)
	if s.closed {
		records, ok := s.staged[key]
		if !ok {
			return nil, fmt.Errorf("evidence-not-collected")
		}
		b, _ := json.Marshal(records)
		var copy []registry.NativeEvidence
		_ = json.Unmarshal(b, &copy)
		return copy, nil
	}
	artifacts := append([]string{}, in.Artifacts...)
	for _, path := range artifacts {
		if !s.allowedPath(path) {
			return nil, s.fail(fmt.Errorf("schema-violation:artifact-path"))
		}
	}
	var result evidenceResult
	if err := s.call(ctx, "evidence", map[string]any{"runner": a.runner, "artifacts": artifacts}, &result); err != nil {
		return nil, err
	}
	if len(result.Records) > testrunner.MaxTests {
		return nil, s.fail(fmt.Errorf("schema-violation:evidence-cap"))
	}
	records := []registry.NativeEvidence{}
	seen := map[string]bool{}
	for _, r := range result.Records {
		if r.Test == "" || seen[r.Test] {
			return nil, s.fail(fmt.Errorf("schema-violation:evidence-test"))
		}
		seen[r.Test] = true
		header := testevidence.New("repository", r.Test, strings.Repeat("a", 40))
		header.Executed, header.Regions, header.Loaded = r.Executed, r.Regions, r.Loaded
		if err := header.ValidateComponents(); err != nil {
			return nil, s.fail(fmt.Errorf("schema-violation:%w", err))
		}
		executed, _ := json.Marshal(r.Executed)
		regions, _ := json.Marshal(r.Regions)
		loaded, _ := json.Marshal(r.Loaded)
		records = append(records, registry.NativeEvidence{Test: r.Test, Executed: executed, Regions: regions, Loaded: loaded})
	}
	s.staged[key] = records
	return nil, fmt.Errorf("evidence-pending-session-close")
}
