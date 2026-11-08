// SPDX-License-Identifier: Apache-2.0

package swiftselection

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/platform"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

// SwiftSelectionOptions contains core-captured inputs for one checkout, runner
// and configuration. No method launches a tool, resolves PATH or reads a file.
// The core admits the pinned executable and verifies capture completeness before
// constructing this value. Unconfigured SelectionPlugin entries remain absent,
// just as script commands require their separately configured factory.
type SwiftSelectionOptions struct {
	Key              testrunner.InventoryKey
	Root, Executable string
	// Tests maps each exact --list-tests line to its native terminal identity
	// and tracked source path. SwiftPM's list alone has no source locations;
	// do not guess paths or remove Swift Testing's location suffix from IDs.
	Tests map[string]testrunner.InventoryTest
	// Sources maps llvm-cov source names to core-resolved immutable blobs.
	Sources map[string]testevidence.Blob
	// Reports and Coverage are keyed by core-owned capture handles, passed to
	// Parse and Evidence respectively. Input includes actual exit/lifecycle
	// facts; a native success message never supplies those facts itself.
	Reports  map[string]testrunner.Input
	Coverage map[string]SwiftCoverage
}

type swiftPlugin struct{ options SwiftSelectionOptions }

// NewSwiftSelectionPlugin configures the compiled-in XCTest or Swift Testing
// selection pieces (RSP-V0-001..005). This is not Xcode/device discovery and
// never confers qualification. Captures are copied before they can be used.
func NewSwiftSelectionPlugin(o SwiftSelectionOptions) (registry.RunnerPlugin, error) {
	if o.Key.Runner != "swift-xctest" && o.Key.Runner != "swift-testing" {
		return nil, fmt.Errorf("swift-runner-unsupported")
	}
	if (o.Key.OS != "darwin" && o.Key.OS != "linux") || (o.Key.Arch != "amd64" && o.Key.Arch != "arm64") {
		return nil, fmt.Errorf("swift-target-unsupported")
	}
	if !wire.IsGitOid(o.Key.Checkout) || !swiftAbsolute(o.Root) || !swiftAbsolute(o.Executable) {
		return nil, fmt.Errorf("swift-pinned-input-invalid")
	}
	if len(o.Tests) > testrunner.MaxTests || len(o.Sources) > testrunner.MaxTests || len(o.Reports) > testrunner.MaxReports || len(o.Coverage) > testrunner.MaxReports {
		return nil, fmt.Errorf("swift-capture-cap-exceeded")
	}
	copy := o
	copy.Tests = make(map[string]testrunner.InventoryTest, len(o.Tests))
	inv := testrunner.RunnerInventory{Key: o.Key, Complete: true}
	for name, test := range o.Tests {
		if !swiftText(name) || !swiftText(test.ID) || !strings.HasSuffix(test.Path, ".swift") || !strings.Contains(test.ID, ".") || (o.Key.Runner == "swift-xctest" && (!swiftXCTestID.MatchString(name) || name != test.ID)) {
			return nil, fmt.Errorf("swift-test-source-invalid")
		}
		copy.Tests[name] = test
		inv.Tests = append(inv.Tests, test)
	}
	if _, err := testrunner.CanonicalInventory(inv, o.Key); err != nil {
		return nil, err
	}
	copy.Sources = make(map[string]testevidence.Blob, len(o.Sources))
	paths := map[string]bool{}
	for name, blob := range o.Sources {
		if !swiftText(name) || paths[blob.Path.Lexical] {
			return nil, fmt.Errorf("swift-source-map-invalid")
		}
		check := testevidence.New("", "", "")
		check.Loaded = testevidence.Component[testevidence.Blob]{State: testevidence.Present, Items: []testevidence.Blob{blob}}
		if err := check.ValidateComponents(); err != nil {
			return nil, err
		}
		paths[blob.Path.Lexical] = true
		copy.Sources[name] = blob
	}
	copy.Reports = make(map[string]testrunner.Input, len(o.Reports))
	copy.Coverage = make(map[string]SwiftCoverage, len(o.Coverage))
	bytes := 0
	for handle, input := range o.Reports {
		if !swiftText(handle) || input.Runner != o.Key.Runner {
			return nil, fmt.Errorf("swift-report-binding-invalid")
		}
		if len(input.Reports) > testrunner.MaxReports {
			return nil, fmt.Errorf("swift-capture-cap-exceeded")
		}
		bytes += len(input.Stdout) + len(input.Stderr)
		for _, data := range input.Reports {
			bytes += len(data)
		}
		if bytes > testevidence.MaxRecordBytes {
			return nil, fmt.Errorf("swift-capture-cap-exceeded")
		}
		copy.Reports[handle] = cloneSwiftInput(input)
	}
	for handle, coverage := range o.Coverage {
		if !swiftText(handle) || len(coverage.Tests) > testrunner.MaxTests {
			return nil, fmt.Errorf("swift-coverage-handle-invalid")
		}
		bytes += len(coverage.LCOV)
		if bytes > testevidence.MaxRecordBytes {
			return nil, fmt.Errorf("swift-capture-cap-exceeded")
		}
		copy.Coverage[handle] = SwiftCoverage{LCOV: append([]byte(nil), coverage.LCOV...), Tests: append([]string(nil), coverage.Tests...), Complete: coverage.Complete}
	}
	return &swiftPlugin{options: copy}, nil
}

var swiftXCTestID = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)+/[A-Za-z_][A-Za-z0-9_]*$`)

func swiftText(s string) bool {
	return s != "" && len(s) <= 4096 && utf8.ValidString(s) && strings.TrimSpace(s) == s && strings.IndexFunc(s, unicode.IsControl) < 0
}
func swiftAbsolute(s string) bool { return swiftText(s) && filepath.IsAbs(s) && filepath.Clean(s) == s }

func (p *swiftPlugin) Pieces() testrunner.RunnerPieces {
	return testrunner.RunnerPieces{Inventory: testrunner.RunnerPiece{Present: true}, TestEvidence: testrunner.RunnerPiece{Present: true}, Qualification: testrunner.RunnerPiece{Reason: "unqualified"}}
}
func (p *swiftPlugin) Inventory(ctx context.Context, r registry.InventoryRequest) (testrunner.RunnerInventory, error) {
	fail := func(reason string) (testrunner.RunnerInventory, error) {
		return testrunner.RunnerInventory{Reason: reason}, fmt.Errorf("%s", reason)
	}
	if err := ctx.Err(); err != nil {
		return fail("swift-inventory-cancelled")
	}
	if r.Key != p.options.Key {
		return fail("swift-inventory-key-mismatch")
	}
	if len(r.Output) == 0 || len(r.Output) > testrunner.MaxReportBytes || !utf8.ValidString(r.Output) || !strings.HasSuffix(r.Output, "\n") {
		return fail("swift-inventory-missing-truncated-or-capped")
	}
	in := testrunner.RunnerInventory{Key: r.Key, Complete: true}
	seen := map[string]bool{}
	for _, name := range strings.Split(strings.TrimSuffix(r.Output, "\n"), "\n") {
		test, ok := p.options.Tests[name]
		if !ok || seen[name] {
			return fail("swift-inventory-unmapped-or-duplicate-test")
		}
		seen[name] = true
		in.Tests = append(in.Tests, test)
		if len(in.Tests) > testrunner.MaxTests {
			return fail("swift-inventory-cap-exceeded")
		}
	}
	// A complete core source map must agree with the entire list. This also
	// catches a capture cut at a line boundary, not only missing final newlines.
	if len(seen) != len(p.options.Tests) {
		return fail("swift-inventory-source-map-divergence")
	}
	return testrunner.CanonicalInventory(in, r.Key)
}
func (p *swiftPlugin) Command(ctx context.Context, r registry.CommandRequest) (registry.CommandResponse, error) {
	if err := ctx.Err(); err != nil {
		return registry.CommandResponse{}, err
	}
	if r.Key != p.options.Key || !swiftAbsolute(r.RunDir) || (r.Purpose != "inventory" && r.Purpose != "run" && r.Purpose != "collect") || len(r.Tests) > testrunner.MaxTests {
		return registry.CommandResponse{}, fmt.Errorf("swift-command-request-invalid")
	}
	if r.Tests != nil && (r.Purpose == "inventory" || len(r.Tests) == 0) {
		return registry.CommandResponse{}, fmt.Errorf("swift-command-empty-or-filtered-inventory")
	}
	known := map[string]bool{}
	for _, test := range p.options.Tests {
		known[test.ID] = true
	}
	seen := map[string]bool{}
	for _, id := range r.Tests {
		if !known[id] || seen[id] {
			return registry.CommandResponse{}, fmt.Errorf("swift-command-test-invalid")
		}
		seen[id] = true
	}
	argv := []string{p.options.Executable, "test", "--package-path", p.options.Root, "--disable-automatic-resolution", "--skip-update", "--no-parallel"}
	if p.options.Key.Runner == "swift-xctest" {
		argv = append(argv, "--enable-xctest", "--disable-swift-testing")
	} else {
		argv = append(argv, "--disable-xctest", "--enable-swift-testing")
	}
	out := registry.CommandResponse{Cwd: p.options.Root}
	if r.Purpose == "inventory" {
		argv = append(argv, "--list-tests")
	} else {
		if p.options.Key.Runner == "swift-xctest" {
			ids := append([]string(nil), r.Tests...)
			sort.Strings(ids)
			for _, id := range ids {
				argv = append(argv, "--filter", "^"+regexp.QuoteMeta(id)+"$")
			}
		} else {
			// Swift Testing's display names, location IDs and parameterized
			// cases need a qualified filter mapping. Run FULL until then.
			out.Superset = r.Tests != nil
			report := filepath.Join(r.RunDir, "swift-events.jsonl")
			argv = append(argv, "--event-stream-version", "0", "--event-stream-output-path", report)
			out.Artifacts = []string{report}
		}
		if r.Purpose == "collect" {
			argv = append(argv, "--enable-code-coverage")
			out.Env = map[string]string{"LLVM_PROFILE_FILE": filepath.Join(r.RunDir, "coverage-%p-%m.profraw")}
		}
	}
	out.Argv = argv
	return out, nil
}
func (p *swiftPlugin) Parse(ctx context.Context, r registry.ParseRequest) ([]testrunner.RunnerResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	input, ok := p.options.Reports[r.Report]
	if r.Runner != p.options.Key.Runner || !ok {
		return nil, fmt.Errorf("swift-report-unavailable")
	}
	// Use the frozen parser with the core's actual process observations. In
	// particular, never assume exit 0 from the absence of a failure message.
	o, err := platform.Parse(input)
	failures := map[string]bool{}
	for _, id := range platform.SwiftTerminalFailures(input) {
		failures[id] = true
	}
	skipReasons := map[string]string{}
	if err == nil && o.Complete && input.Runner == "swift-testing" {
		skipReasons = swiftSkipReasons(input.Reports["swift-events.jsonl"])
	}
	results := make([]testrunner.RunnerResult, 0, len(o.Tests))
	for _, test := range o.Tests {
		result := testrunner.RunnerResult{ID: test.ID}
		switch test.State {
		case testrunner.Passed:
			result.Outcome = "passed"
		case testrunner.Failed:
			result.Outcome = "failed"
		case testrunner.Skipped:
			result.Outcome, result.Reason = "skipped", test.Message
			if input.Runner == "swift-testing" {
				result.Reason = skipReasons[test.ID]
			}
			// A native skip without a reported reason stays nonterminal to
			// core reconciliation. Do not manufacture a success or a reason.
		}
		if failures[test.ID] {
			result.Outcome = "failed"
			delete(failures, test.ID)
		}
		results = append(results, result)
	}
	// Swift Testing may reject the run lifecycle before emitting any tests.
	var missingFailures []string
	for id := range failures {
		missingFailures = append(missingFailures, id)
	}
	sort.Strings(missingFailures)
	for _, id := range missingFailures {
		results = append(results, testrunner.RunnerResult{ID: id, Outcome: "failed"})
	}
	if err != nil {
		return results, fmt.Errorf("swift-report-incomplete:%w", err)
	}
	if len(input.ExecutionProblems) != 0 {
		return results, fmt.Errorf("swift-report-execution-problems")
	}
	if !o.Complete {
		return results, fmt.Errorf("swift-report-incomplete")
	}
	return results, nil
}
func cloneSwiftInput(in testrunner.Input) testrunner.Input {
	// Only fields consumed by platform.Parse for these runners are retained;
	// selection and terminal reconciliation remain with the core session.
	out := testrunner.Input{Runner: in.Runner, ExitCode: in.ExitCode, TimedOut: in.TimedOut, Interrupted: in.Interrupted, Overflow: in.Overflow, Stdout: append([]byte(nil), in.Stdout...), Stderr: append([]byte(nil), in.Stderr...)}
	out.Reports = make(map[string][]byte, len(in.Reports))
	for name, raw := range in.Reports {
		out.Reports[name] = append([]byte(nil), raw...)
	}
	out.ExecutionProblems = append([]testrunner.Problem(nil), in.ExecutionProblems...)
	return out
}

// Called only after the frozen event parser validates the stream. Preserve the
// native skip messages that the legacy observation does not expose.
func swiftSkipReasons(stream []byte) map[string]string {
	reasons := map[string]string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(stream), "\n"), "\n") {
		var record struct {
			Kind    string `json:"kind"`
			Payload struct {
				Kind     string `json:"kind"`
				TestID   string `json:"testID"`
				Messages []struct {
					Text string `json:"text"`
				} `json:"messages"`
			} `json:"payload"`
		}
		if json.Unmarshal([]byte(line), &record) != nil || record.Kind != "event" || record.Payload.Kind != "testSkipped" {
			continue
		}
		var messages []string
		for _, message := range record.Payload.Messages {
			if strings.TrimSpace(message.Text) != "" {
				messages = append(messages, message.Text)
			}
		}
		reasons[record.Payload.TestID] = strings.Join(messages, "\n")
	}
	return reasons
}
