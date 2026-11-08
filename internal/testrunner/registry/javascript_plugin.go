// SPDX-License-Identifier: Apache-2.0
package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Beamfall/corvint/internal/langplugin/runnernames"
	"path/filepath"
	"sync"

	"github.com/Beamfall/corvint/internal/browsercov"
	"github.com/Beamfall/corvint/internal/jslinecov"
	"github.com/Beamfall/corvint/internal/proctrace"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/dynamic"
)

// JavaScriptPlugin binds the compiled-in SPI to core-captured configuration.
// Like ScriptPlugins it is constructed for an admitted invocation; the legacy
// SelectionPlugin lookup remains explicitly absent without captured inputs.
// No method starts a process or writes the evidence store.
type JavaScriptPlugin struct {
	discovery  jslinecov.Discovery
	executable string
	native     map[string]testevidence.Native
	mu         sync.Mutex
	inventory  testrunner.RunnerInventory
}

func NewJavaScriptPlugin(d jslinecov.Discovery, executable string, native map[string]testevidence.Native) (*JavaScriptPlugin, error) {
	if err := d.Check(); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(executable) {
		return nil, fmt.Errorf("runner-executable-unavailable")
	}
	d.Tracked = append([]string{}, d.Tracked...)
	if d.Files != nil {
		d.Files = append([]string{}, d.Files...)
	}
	// Freeze caller-owned native components; aliases cannot change later evidence.
	raw, err := json.Marshal(native)
	if err != nil || len(raw) > jslinecov.MaxRunnerArtifact {
		return nil, fmt.Errorf("native-evidence-cap")
	}
	captured := map[string]testevidence.Native{}
	if err = json.Unmarshal(raw, &captured); err != nil {
		return nil, err
	}
	return &JavaScriptPlugin{discovery: d, executable: executable, native: captured}, nil
}
func (p *JavaScriptPlugin) Pieces() testrunner.RunnerPieces {
	return testrunner.RunnerPieces{Inventory: testrunner.RunnerPiece{Present: true}, TestEvidence: testrunner.RunnerPiece{Present: true}, Qualification: testrunner.RunnerPiece{Reason: "unqualified"}}
}
func (p *JavaScriptPlugin) Inventory(ctx context.Context, r InventoryRequest) (testrunner.RunnerInventory, error) {
	if err := ctx.Err(); err != nil {
		return testrunner.RunnerInventory{Reason: "inventory-unavailable"}, err
	}
	if r.Key != p.discovery.Key {
		return testrunner.RunnerInventory{Reason: "inventory-key-mismatch"}, fmt.Errorf("inventory-key-mismatch")
	}
	var in testrunner.RunnerInventory
	var err error
	if r.Key.Runner == runnernames.RunnerPlaywright {
		in, err = browsercov.PlaywrightInventory(p.discovery, r.Output)
	} else {
		in, err = jslinecov.Inventory(p.discovery, r.Output)
	}
	p.mu.Lock()
	p.inventory = in
	p.inventory.Tests = append([]testrunner.InventoryTest{}, in.Tests...)
	p.mu.Unlock()
	return in, err
}

// Prepare reuses the existing runner's command builder and reporter assets.
// Core materializes Files in a fresh private RunDir before tracing Command;
// the plugin itself has no filesystem or executable admission capability.
func (p *JavaScriptPlugin) Prepare(r CommandRequest) (testrunner.Invocation, bool, error) {
	if r.Key != p.discovery.Key || !filepath.IsAbs(r.RunDir) || (r.Purpose != "run" && r.Purpose != "collect") {
		return testrunner.Invocation{}, false, fmt.Errorf("runner-command-invalid")
	}
	p.mu.Lock()
	in := p.inventory
	p.mu.Unlock()
	files, err := jslinecov.FilesFor(in, r.Tests)
	if err != nil {
		return testrunner.Invocation{}, false, err
	}
	superset := false
	if r.Tests != nil {
		switch r.Key.Runner {
		case runnernames.RunnerVitest:
			files = nil
			superset = true
		case runnernames.RunnerPlaywright:
			superset = true
		case runnernames.RunnerJest:
			_, _, superset, _ = jslinecov.RunnerVersion(runnernames.RunnerJest, p.discovery.Version)
		}
	}
	if files == nil {
		files = p.discovery.Files
	}
	// Vitest's native substring filter cannot express a guaranteed exact set;
	// its admitted config determines FULL. Explicit Vitest file filters remain
	// unsupported rather than being silently dropped.
	v, err := dynamic.Build(testrunner.Request{Runner: r.Key.Runner, Executable: p.executable, Root: p.discovery.Root, ReportDir: r.RunDir, Config: p.discovery.Config, Selectors: files})
	if err != nil {
		return v, false, err
	}
	v.Environment["NODE_OPTIONS"] = p.discovery.NodeOptions
	if r.Purpose == "collect" {
		// Every Node descendant, including Playwright's configured webServer,
		// inherits this scratch-only coverage destination inside the tracer.
		v.Environment["NODE_V8_COVERAGE"] = filepath.Join(r.RunDir, "node-coverage")
		if r.Key.Runner == runnernames.RunnerPlaywright {
			files, env, err := browsercov.RunnerCoveragePreparation(r.RunDir, p.discovery.NodeOptions)
			if err != nil {
				return v, false, err
			}
			for name, raw := range files {
				v.Files[name] = raw
			}
			for name, value := range env {
				v.Environment[name] = value
			}
			v.Argv = append(v.Argv, "--workers=1", "--retries=0")
		}
	}
	return v, superset, nil
}
func (p *JavaScriptPlugin) Command(ctx context.Context, r CommandRequest) (CommandResponse, error) {
	if err := ctx.Err(); err != nil {
		return CommandResponse{}, err
	}
	if r.Key != p.discovery.Key {
		return CommandResponse{}, fmt.Errorf("inventory-key-mismatch")
	}
	if r.Purpose == "inventory" {
		args, err := jslinecov.ListArgs(p.discovery)
		if err != nil {
			return CommandResponse{}, err
		}
		if r.Key.Runner == runnernames.RunnerNodeTest {
			return CommandResponse{}, nil
		}
		// Discovery loads the suite exactly as execution does, so it carries the
		// same NODE_OPTIONS (an empty value clears inherited preloads).
		return CommandResponse{Argv: append([]string{p.executable}, args...), Cwd: p.discovery.Root, Env: map[string]string{"NODE_OPTIONS": p.discovery.NodeOptions}}, nil
	}
	v, superset, err := p.Prepare(r)
	if err != nil {
		return CommandResponse{}, err
	}
	return CommandResponse{Argv: append([]string{p.executable}, v.Argv...), Cwd: p.discovery.Root, Env: v.Environment, Artifacts: v.ReportPaths, Superset: superset}, nil
}
func (p *JavaScriptPlugin) Parse(ctx context.Context, r ParseRequest) ([]testrunner.RunnerResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.Runner != p.discovery.Key.Runner {
		return nil, fmt.Errorf("runner-mismatch")
	}
	if r.Runner == runnernames.RunnerPlaywright {
		return browsercov.PlaywrightResults(p.discovery, r.Report)
	}
	return jslinecov.Results(p.discovery, r.Report)
}
func (p *JavaScriptPlugin) Evidence(ctx context.Context, r EvidenceRequest) ([]NativeEvidence, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.Runner != p.discovery.Key.Runner || len(r.Artifacts) != 0 {
		return nil, fmt.Errorf("native-capture-required")
	}
	p.mu.Lock()
	in := p.inventory
	p.mu.Unlock()
	if !in.Complete {
		return nil, fmt.Errorf("inventory-unavailable")
	}
	var out []NativeEvidence
	for _, test := range in.Tests {
		n, ok := p.native[test.ID]
		if !ok {
			n = jslinecov.NativeCoverage(jslinecov.CoverageCapture{Reason: "native-artifacts-absent"})
		}
		check := testevidence.New("check", test.ID, "unused")
		check.Executed, check.Regions, check.Loaded = n.Executed, n.Regions, n.Loaded
		if err := check.ValidateComponents(); err != nil {
			return nil, fmt.Errorf("native-components-malformed")
		}
		e, _ := json.Marshal(n.Executed)
		regions, _ := json.Marshal(n.Regions)
		l, _ := json.Marshal(n.Loaded)
		out = append(out, NativeEvidence{Test: test.ID, Executed: e, Regions: regions, Loaded: l})
	}
	return out, nil
}

// TraceJavaScriptCommand starts the runner itself in the core tracer. A
// Playwright webServer command is consequently a descendant, not a separately
// launched process. Reused/external servers are handled by JoinServer's core
// attribution facts and cannot become inside-tree merely from config presence.
func TraceJavaScriptCommand(ctx context.Context, cmd CommandResponse, q proctrace.Request, backend proctrace.Backend) (proctrace.Result, error) {
	if len(cmd.Argv) == 0 || cmd.Argv[0] != q.Executable {
		return proctrace.Result{}, fmt.Errorf("runner-trace-command-mismatch")
	}
	result, _, err := TraceScriptCommand(ctx, cmd, q, backend)
	return result, err
}

var _ RunnerPlugin = (*JavaScriptPlugin)(nil)
