// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/Beamfall/corvint/internal/langplugin/runnernames"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/rubylinecov"
	"github.com/Beamfall/corvint/internal/testevidence"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/dynamic"
)

// RubyCapture is a core-owned capture reader: it admits bounded artifact names,
// pins source blobs to key.Checkout, and calls rubylinecov.SelectionEvidence.
// It cannot supply trace or qualification components. The core assembles those
// independently using testevidence.Assemble. A nil reader means absent evidence.
type RubyCapture func(context.Context, tr.InventoryKey, []string) ([]rubylinecov.SelectionNative, error)

// RubyPlugin is the bound selection extension of the existing Ruby profiles.
// Global lookup exposes discovery and parsing; execution and evidence require
// the core-bound factory. Legacy Build/Parse behavior remains unchanged.
type RubyPlugin struct {
	key             tr.InventoryKey
	root, reportDir string
	run, inventory  tr.Invocation
	capture         RubyCapture
}

// unboundRubyPlugin is the dynamic family's selection factory (AFP-V0-9985):
// it serves discovery and parsing; execution and evidence need the core-bound
// NewRubyPlugin.
func unboundRubyPlugin(runner string) RunnerPlugin {
	return &RubyPlugin{key: tr.InventoryKey{Runner: runner}}
}

// RubyConfigurationDigest binds runner configuration independently of the fresh
// report staging directory. The core still admits executable and configuration pins.
func RubyConfigurationDigest(r tr.Request) string {
	r.ReportDir = "" // execution staging is not runner configuration (RSP-V0-002)
	b, _ := json.Marshal(r)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

func NewRubyPlugin(key tr.InventoryKey, r tr.Request, capture RubyCapture) (*RubyPlugin, error) {
	if !wire.IsGitOid(key.Checkout) || key.Runner != r.Runner || key.ConfigDigest != RubyConfigurationDigest(r) || !filepath.IsAbs(r.Root) || strings.ContainsAny(r.Root+r.ReportDir+r.Executable, "\x00\r\n") {
		return nil, fmt.Errorf("ruby-plugin-binding-invalid")
	}
	if _, err := tr.CanonicalInventory(tr.RunnerInventory{Key: key, Complete: true}, key); err != nil {
		return nil, err
	}
	run, err := dynamic.RubySelectionInvocation(r, false)
	if err != nil {
		return nil, err
	}
	inventory, err := dynamic.RubySelectionInvocation(r, true)
	if err != nil {
		return nil, err
	}
	// Build's argv contains arguments; the core-owned SPI includes executable.
	run.Argv = append([]string{r.Executable}, run.Argv...)
	inventory.Argv = append([]string{r.Executable}, inventory.Argv...)
	// Every request runs a known superset. A test ID is never interpreted as a
	// Ruby regular expression or passed through a framework substring filter.
	return &RubyPlugin{key: key, root: r.Root, reportDir: r.ReportDir, run: run, inventory: inventory, capture: capture}, nil
}

func (p *RubyPlugin) Pieces() tr.RunnerPieces {
	return tr.RunnerPieces{Inventory: tr.RunnerPiece{Present: true}, TestEvidence: tr.RunnerPiece{Present: true}, Qualification: tr.RunnerPiece{Reason: "unqualified"}}
}

// Files returns the existing core-staged reporter assets plus the list adapter.
// Copies ensure a caller cannot alter subsequent command descriptions.
func (p *RubyPlugin) Files() map[string][]byte {
	out := map[string][]byte{"ruby_linecov.rb": rubylinecov.SelectionCollector()}
	for _, inv := range []tr.Invocation{p.run, p.inventory} {
		for name, b := range inv.Files {
			out[name] = append([]byte{}, b...)
		}
	}
	return out
}

func (p *RubyPlugin) Inventory(ctx context.Context, r InventoryRequest) (tr.RunnerInventory, error) {
	fail := func(err error) (tr.RunnerInventory, error) { return tr.RunnerInventory{Reason: err.Error()}, err }
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if r.Key.Runner != p.key.Runner || p.key.Checkout != "" && r.Key != p.key {
		return fail(fmt.Errorf("ruby-inventory-key-mismatch"))
	}
	rows, err := dynamic.RubySelectionReport(p.key.Runner, r.Output)
	if err != nil {
		return fail(err)
	}
	in := tr.RunnerInventory{Key: r.Key, Complete: true}
	for _, row := range rows {
		in.Tests = append(in.Tests, tr.InventoryTest{ID: row.ID, Path: strings.TrimPrefix(row.File, "./")})
	}
	in, err = tr.CanonicalInventory(in, r.Key)
	if err != nil {
		return fail(err)
	}
	return in, nil
}

func (p *RubyPlugin) Command(ctx context.Context, r CommandRequest) (CommandResponse, error) {
	if err := ctx.Err(); err != nil {
		return CommandResponse{}, err
	}
	if p.key.Checkout == "" || r.Key != p.key || r.RunDir != p.root || len(r.Tests) > tr.MaxTests {
		return CommandResponse{}, fmt.Errorf("ruby-command-binding-invalid")
	}
	inv := p.run
	switch r.Purpose {
	case "inventory":
		inv = p.inventory
	case "run":
	case "collect":
		// RUBYOPT is parsed by Ruby; refuse ambiguous staging paths.
		if strings.ContainsAny(p.reportDir, " \t\\\"'") {
			return CommandResponse{}, fmt.Errorf("ruby-collector-path-unsupported")
		}
	default:
		return CommandResponse{}, fmt.Errorf("ruby-command-purpose-unsupported")
	}
	env := map[string]string{}
	for k, v := range inv.Environment {
		env[k] = v
	}
	artifacts := append([]string{}, inv.ReportPaths...)
	if r.Purpose == "collect" {
		env["RUBYOPT"] = "-r" + filepath.Join(p.reportDir, "ruby_linecov.rb")
		env["CORVINT_LINECOV_ROOT"] = p.root
		env["CORVINT_LINECOV_DIR"] = p.CaptureDirectory()
		env["CORVINT_LINECOV_ONLY"] = ""
		env["CORVINT_LINECOV_PACKAGE"] = ""
		env["CORVINT_LINECOV_TEST"] = "load"
		env["CORVINT_LINECOV_LAUNCH"] = ""
		env["PARALLEL_WORKERS"] = "1"
		artifacts = append(artifacts, "ruby-coverage")
	}
	return CommandResponse{Argv: append([]string{}, inv.Argv...), Cwd: p.root, Env: env, Artifacts: artifacts, Superset: r.Tests != nil}, nil
}

func (p *RubyPlugin) Parse(ctx context.Context, r ParseRequest) ([]tr.RunnerResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.Runner != p.key.Runner {
		return nil, fmt.Errorf("ruby-report-runner-mismatch")
	}
	// RSP-V0-003: rows returned with an error are retained failures; keep them.
	rows, err := dynamic.RubySelectionReport(r.Runner, r.Report)
	out := make([]tr.RunnerResult, 0, len(rows))
	for _, row := range rows {
		out = append(out, tr.RunnerResult{ID: row.ID, Outcome: strings.ToLower(row.State), Reason: row.Message})
	}
	return out, err
}

func (p *RubyPlugin) Evidence(ctx context.Context, r EvidenceRequest) ([]NativeEvidence, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.Runner != p.key.Runner || p.capture == nil || len(r.Artifacts) == 0 || len(r.Artifacts) > tr.MaxReports {
		return nil, fmt.Errorf("ruby-native-capture-unavailable")
	}
	native, err := p.capture(ctx, p.key, append([]string{}, r.Artifacts...))
	if err != nil {
		return nil, err
	}
	if len(native) == 0 || len(native) > tr.MaxTests {
		return nil, fmt.Errorf("ruby-native-capture-count-invalid")
	}
	out := make([]NativeEvidence, 0, len(native))
	seen := map[string]bool{}
	for _, n := range native {
		id := n.Name
		if p.key.Runner == runnernames.RunnerTestUnit {
			// The collector preserves decorated/data-driven names inside brackets.
			// Only the exact simple form can be mapped without additional metadata.
			klass, method, ok := strings.Cut(n.Name, "#")
			if !ok || klass == "" || method == "" || strings.ContainsAny(method, "#[]()") {
				return nil, fmt.Errorf("ruby-native-identity-unsupported")
			}
			id = method + "(" + klass + ")"
		}
		if _, err := tr.CanonicalInventory(tr.RunnerInventory{Key: p.key, Complete: true, Tests: []tr.InventoryTest{{ID: id, Path: strings.TrimPrefix(n.Package, "./")}}}, p.key); err != nil {
			return nil, err
		}
		if seen[id] {
			return nil, fmt.Errorf("ruby-native-identity-duplicate")
		}
		seen[id] = true
		record := testevidence.New("", id, p.key.Checkout)
		record.Executed, record.Regions, record.Loaded = n.Native.Executed, n.Native.Regions, n.Native.Loaded
		if err := record.ValidateComponents(); err != nil {
			return nil, err
		}
		executed, _ := json.Marshal(n.Native.Executed)
		regions, _ := json.Marshal(n.Native.Regions)
		loaded, _ := json.Marshal(n.Native.Loaded)
		out = append(out, NativeEvidence{Test: id, Executed: executed, Regions: regions, Loaded: loaded})
	}
	return out, nil
}

// CaptureDirectory names the fresh directory the core must create beside its
// staged reporter files before collection and capture completely afterwards.
// As in rubylinecov.Collect, core execution must remove inherited CORVINT_LINECOV_*
// settings before applying Command.Env, especially CORVINT_LINECOV_OWNER.
// The capture reader must keep non-isolated or incomplete runs incomplete.
// The plugin never creates it or reads a live mutable worktree.
func (p *RubyPlugin) CaptureDirectory() string { return filepath.Join(p.reportDir, "ruby-coverage") }
