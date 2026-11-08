// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Beamfall/corvint/internal/langplugin/runnernames"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/proctrace"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/platform"
)

// ShellSelectionPlugin binds the selection SPI to one literal source file.
// The core owns the checkout/configuration/target key and admission of the
// runner executable and, for ShellSpec, the independently pinned shell path.
// ConfigDigest must bind that source, shell, runner version and configuration.
// Like ScriptPlugins, this factory leaves legacy SelectionPlugin declarations
// and execution wires unchanged. RSP-V0-001, RSP-V0-040: no qualification here.
func ShellSelectionPlugin(key testrunner.InventoryKey, source, shell string) (RunnerPlugin, error) {
	if key.Runner != runnernames.RunnerBats && key.Runner != runnernames.RunnerShellspec {
		return nil, fmt.Errorf("shell-runner-unsupported")
	}
	if !wire.IsGitOid(key.Checkout) {
		return nil, fmt.Errorf("shell-checkout-not-pinned")
	}
	if strings.ContainsAny(source, "*?[") {
		return nil, fmt.Errorf("shell-source-glob-unresolvable")
	}
	probe := testrunner.RunnerInventory{Key: key, Complete: true, Tests: []testrunner.InventoryTest{{ID: "source", Path: source}}}
	if _, err := testrunner.CanonicalInventory(probe, key); err != nil {
		return nil, err
	}
	if strings.HasPrefix(source, "-") || (key.Runner == runnernames.RunnerBats && (!strings.HasSuffix(source, ".bats") || shell != "")) || (key.Runner == runnernames.RunnerShellspec && (!strings.HasSuffix(source, ".sh") || !filepath.IsAbs(shell) || strings.ContainsAny(shell, "\x00\r\n"))) {
		return nil, fmt.Errorf("shell-source-or-tool-unsupported")
	}
	return &shellPlugin{key: key, source: source, shell: shell}, nil
}

type shellPlugin struct {
	key           testrunner.InventoryKey
	source, shell string
}

func shellPieces() testrunner.RunnerPieces {
	return testrunner.RunnerPieces{Inventory: testrunner.RunnerPiece{Present: true}, TestEvidence: testrunner.RunnerPiece{Present: true}, Qualification: testrunner.RunnerPiece{Reason: "unqualified"}}
}
func (p *shellPlugin) Pieces() testrunner.RunnerPieces { return shellPieces() }
func (p *shellPlugin) identity(id string) string       { return p.key.Runner + ":" + p.source + "::" + id }

// Reports use the existing Input serialization, only inside this compiled-in
// SPI. The core captures stdout AND process status; native output alone cannot
// invent a successful exit. Inventory consumes a FULL unfiltered run (Bats has
// no stable public name-listing command), never a static scan of shell syntax.
func (p *shellPlugin) report(ctx context.Context, report string) ([]testrunner.RunnerResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var in testrunner.Input
	if err := testrunner.DecodeDocument([]byte(report), &in); err != nil {
		return nil, fmt.Errorf("shell-report-invalid:%w", err)
	}
	// A missing/null process status must not decode to an invented successful
	// zero exit. These members are always emitted by the core capture helper.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(report), &fields); err != nil {
		return nil, err
	}
	for _, name := range []string{"runner", "sourceFile", "stdout", "exitCode", "overflow", "interrupted", "timedOut", "executionProblems"} {
		value, present := fields[name]
		if !present || (name != "executionProblems" && string(value) == "null") {
			return nil, fmt.Errorf("shell-report-field-missing:%s", name)
		}
	}
	if in.Runner != p.key.Runner || in.SourceFile != p.source || len(in.Selectors) != 0 || len(in.Expected) != 0 || in.ExpectedSelection != nil || len(in.SuccessExitCodes) != 0 || len(in.FailureExitCodes) != 0 || len(in.OutcomeNeutralExitCodes) != 0 {
		return nil, fmt.Errorf("shell-report-configuration-mismatch")
	}
	if in.Runner == runnernames.RunnerShellspec {
		in.FailureExitCodes = []int{101}
	}
	o, err := platform.ParseShellResults(in)
	results := make([]testrunner.RunnerResult, 0, len(o.Tests))
	// RSP-V0-003: validate each terminal independently. An invalid terminal
	// cannot certify completion or hide a later failure. Preserve the first
	// validation error so existing refusal text and precedence stay stable.
	var validationErr error
	invalid := func(reason string) {
		if validationErr == nil {
			validationErr = fmt.Errorf("%s", reason)
		}
	}
	for _, test := range o.Tests {
		if p.key.Runner == runnernames.RunnerShellspec && test.File != p.source {
			invalid("shell-report-source-mismatch")
			continue
		}
		result := testrunner.RunnerResult{ID: p.identity(test.ID), Reason: test.Message}
		switch test.State {
		case testrunner.Passed:
			result.Outcome = "passed"
		case testrunner.Failed:
			result.Outcome = "failed"
		case testrunner.Skipped:
			result.Outcome = "skipped"
			if strings.TrimSpace(result.Reason) == "" {
				invalid("shell-skip-reason-missing")
				continue
			}
		default:
			invalid("shell-terminal-result-unknown")
			continue
		}
		results = append(results, result)
	}
	if validationErr != nil {
		return results, validationErr
	}
	if err != nil {
		return results, err
	}
	if !o.Complete || len(in.ExecutionProblems) != 0 {
		return results, fmt.Errorf("shell-report-incomplete")
	}
	probe := testrunner.RunnerInventory{Key: p.key, Complete: true}
	for _, result := range results {
		probe.Tests = append(probe.Tests, testrunner.InventoryTest{ID: result.ID, Path: p.source})
	}
	if _, err := testrunner.CanonicalInventory(probe, p.key); err != nil {
		return results, err
	}
	return results, nil
}

// RSP-V0-002: completeness comes from the native plan/count and process result.
func (p *shellPlugin) Inventory(ctx context.Context, r InventoryRequest) (testrunner.RunnerInventory, error) {
	if r.Key != p.key {
		return testrunner.RunnerInventory{}, fmt.Errorf("shell-inventory-key-mismatch")
	}
	results, err := p.report(ctx, r.Output)
	if err != nil {
		return testrunner.RunnerInventory{Reason: err.Error()}, err
	}
	in := testrunner.RunnerInventory{Key: p.key, Complete: true}
	for _, result := range results {
		in.Tests = append(in.Tests, testrunner.InventoryTest{ID: result.ID, Path: p.source})
	}
	return testrunner.CanonicalInventory(in, r.Key)
}

// RSP-V0-004: always execute the whole source file. Unsupported native filters
// cannot silently omit a requested test; T3 reconciles every planned identity.
func (p *shellPlugin) Command(ctx context.Context, r CommandRequest) (CommandResponse, error) {
	if err := ctx.Err(); err != nil {
		return CommandResponse{}, err
	}
	if r.Key != p.key || (r.Purpose != "inventory" && r.Purpose != "run" && r.Purpose != "collect") || !filepath.IsAbs(r.RunDir) || strings.ContainsRune(r.RunDir, 0) || len(r.Tests) > testrunner.MaxTests {
		return CommandResponse{}, fmt.Errorf("shell-command-request-invalid")
	}
	argv := []string{runnernames.RunnerBats, "--formatter", "tap", filepath.Join(r.RunDir, p.source)}
	if p.key.Runner == runnernames.RunnerShellspec {
		argv = []string{runnernames.RunnerShellspec, "--shell", p.shell, "--format", "junit", "--no-color", "--no-banner", p.source}
	}
	return CommandResponse{Argv: argv, Cwd: r.RunDir, Superset: r.Tests != nil}, nil
}
func (p *shellPlugin) Parse(ctx context.Context, r ParseRequest) ([]testrunner.RunnerResult, error) {
	if r.Runner != p.key.Runner {
		return nil, fmt.Errorf("shell-report-runner-mismatch")
	}
	return p.report(ctx, r.Report)
}

func shellNative() testevidence.Native {
	return testevidence.Native{Executed: testevidence.Missing[testevidence.ExecutedFile]("shell-no-line-coverage"), Regions: testevidence.Missing[testevidence.RegionsFile]("shell-no-line-coverage"), Loaded: testevidence.Missing[testevidence.Blob]("shell-no-line-coverage")}
}

// RSP-V0-005: no shell coverage or fabricated regions. The sole artifact is the
// core-captured native report; trace components are supplied separately below.
func (p *shellPlugin) Evidence(ctx context.Context, r EvidenceRequest) ([]NativeEvidence, error) {
	if r.Runner != p.key.Runner || len(r.Artifacts) != 1 {
		return nil, fmt.Errorf("shell-native-artifacts-unsupported")
	}
	results, err := p.report(ctx, r.Artifacts[0])
	if err != nil {
		return nil, err
	}
	native := shellNative()
	executed, _ := json.Marshal(native.Executed)
	regions, _ := json.Marshal(native.Regions)
	loaded, _ := json.Marshal(native.Loaded)
	out := make([]NativeEvidence, 0, len(results))
	for _, result := range results {
		out = append(out, NativeEvidence{Test: result.ID, Executed: append(json.RawMessage(nil), executed...), Regions: append(json.RawMessage(nil), regions...), Loaded: append(json.RawMessage(nil), loaded...)})
	}
	return out, nil
}

// ShellTraceRecords attributes the whole file's process tree to EVERY test in
// that run's inventory. There are no qualified per-test markers. The caller
// supplies core trace observations and one bound header per inventoried test;
// no partial header set may manufacture narrower per-test observations.
func ShellTraceRecords(ctx context.Context, in testrunner.RunnerInventory, headers []testevidence.Record, trace proctrace.Result, repo *gitauth.Repository) ([]testevidence.Record, error) {
	inv, err := testrunner.CanonicalInventory(in, in.Key)
	if err != nil {
		return nil, err
	}
	if (in.Key.Runner != runnernames.RunnerBats && in.Key.Runner != runnernames.RunnerShellspec) || len(headers) != len(inv.Tests) || len(headers) == 0 {
		return nil, fmt.Errorf("shell-trace-inventory-mismatch")
	}
	expected := make(map[string]string, len(inv.Tests))
	for _, test := range inv.Tests {
		expected[test.ID] = test.Path
	}
	seen := map[string]bool{}
	out := make([]testevidence.Record, 0, len(headers))
	for _, header := range headers {
		source, ok := expected[header.Test]
		if !ok || seen[header.Test] || !strings.HasPrefix(header.Test, in.Key.Runner+":"+source+"::") || header.Runner.Plugin != in.Key.Runner || header.Commit != in.Key.Checkout || header.Runner.ConfigurationDigest != in.Key.ConfigDigest || header.Execution.OS != in.Key.OS || header.Execution.Architecture != in.Key.Arch || header.Execution.TracingBackend != trace.Backend {
			return nil, fmt.Errorf("shell-trace-header-mismatch")
		}
		seen[header.Test] = true
		record, err := fileTraceRecord(ctx, header, trace, repo, shellNative())
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, nil
}
