// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	_ "embed"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"
	"github.com/Beamfall/corvint/internal/langplugin/runnernames"
	"path"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
)

//go:embed python/capture.py
var pythonCaptureProgram string

// PythonCapture contains only core-captured artifacts and pinned source bytes.
// It is an in-process input, not a wire contract. Tests must name every test in
// the captured process tree: coverage contexts are never trusted for attribution.
// The core retains execution provenance, supplies trace components and admits
// records; this factory neither runs processes nor reads or writes any files.
type PythonCapture struct {
	Tests    []string
	Coverage []byte
	Sources  map[string]PythonSource
}

type pythonPlugin struct {
	runner  string
	capture PythonCapture
}

// PythonPlugin constructs the fixture-facing pieces without replacing the
// frozen legacy registry declarations. The caller executes Command under core
// containment and captures stdout; capture.py serializes the EXISTING internal
// RunnerInventory / RunnerResult types, never a new public runner protocol.
// No qualification record is bundled: every boundary class remains unqualified.
func PythonPlugin(runner string, capture PythonCapture) (RunnerPlugin, bool) {
	if runner != runnernames.RunnerPytest && runner != runnernames.RunnerUnittest {
		return nil, false
	}
	copy := PythonCapture{Tests: append([]string(nil), capture.Tests...), Coverage: append([]byte(nil), capture.Coverage...), Sources: map[string]PythonSource{}}
	for name, source := range capture.Sources {
		copy.Sources[name] = source
	}
	return &pythonPlugin{runner, copy}, true
}

func (p *pythonPlugin) Pieces() testrunner.RunnerPieces {
	return testrunner.RunnerPieces{Inventory: testrunner.RunnerPiece{Present: true}, TestEvidence: testrunner.RunnerPiece{Present: true}, Qualification: testrunner.RunnerPiece{Reason: "unqualified"}}
}

func pythonDecode(raw []byte, into any) error {
	if len(raw) == 0 || len(raw) > testrunner.MaxReportBytes {
		return fmt.Errorf("python-artifact-size")
	}
	// v2 rejects duplicate members and invalid UTF-8; unknown capture fields are
	// refused as well. Native coverage has its own explicitly bounded projection.
	if err := jsonv2.Unmarshal(raw, into, jsonv2.RejectUnknownMembers(true)); err != nil {
		return fmt.Errorf("python-artifact-invalid:%w", err)
	}
	return nil
}

func (p *pythonPlugin) Inventory(ctx context.Context, r InventoryRequest) (testrunner.RunnerInventory, error) {
	fail := func(err error) (testrunner.RunnerInventory, error) {
		return testrunner.RunnerInventory{Reason: err.Error()}, err
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if r.Key.Runner != p.runner {
		return fail(fmt.Errorf("python-runner-mismatch"))
	}
	var in testrunner.RunnerInventory
	if err := pythonDecode([]byte(r.Output), &in); err != nil {
		return fail(err)
	}
	if in.Tests == nil {
		return fail(fmt.Errorf("python-inventory-tests-absent"))
	}
	for _, test := range in.Tests {
		if !pythonTestID(p.runner, test.ID) || !strings.HasSuffix(test.Path, ".py") {
			return fail(fmt.Errorf("python-inventory-identity"))
		}
	}
	out, err := testrunner.CanonicalInventory(in, r.Key)
	if err != nil {
		return fail(err)
	}
	return out, nil
}

func pythonTestID(runner, id string) bool {
	if !utf8.ValidString(id) || strings.IndexFunc(id, unicode.IsControl) >= 0 || id == "" || len(id) > 4096 || strings.ContainsAny(id, "\x00\r\n\t") || strings.HasPrefix(id, "-") || strings.HasPrefix(id, "@") {
		return false
	}
	if runner == runnernames.RunnerPytest {
		source, tail, ok := strings.Cut(id, "::")
		return ok && tail != "" && strings.HasSuffix(source, ".py") && source != ".py" && path.Clean(source) == source && !strings.ContainsAny(source, "\\:") && !strings.HasPrefix(source, "/") && !strings.HasPrefix(source, "../") && !strings.Contains(source, "/../")
	}
	parts := strings.Split(id, ".")
	if len(parts) < 3 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for i, c := range part {
			if !(c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && c >= '0' && c <= '9') {
				return false
			}
		}
	}
	return true
}

func (p *pythonPlugin) Command(ctx context.Context, r CommandRequest) (CommandResponse, error) {
	if err := ctx.Err(); err != nil {
		return CommandResponse{}, err
	}
	if r.Key.Runner != p.runner || !filepath.IsAbs(r.RunDir) || strings.ContainsRune(r.RunDir, 0) {
		return CommandResponse{}, fmt.Errorf("python-command-request-invalid")
	}
	if _, err := testrunner.CanonicalInventory(testrunner.RunnerInventory{Key: r.Key, Complete: true}, r.Key); err != nil {
		return CommandResponse{}, err
	}
	if r.Purpose != "inventory" && r.Purpose != "run" && r.Purpose != "collect" {
		return CommandResponse{}, fmt.Errorf("python-command-purpose")
	}
	if r.Tests != nil && (len(r.Tests) == 0 || r.Purpose == "inventory") {
		return CommandResponse{}, fmt.Errorf("python-empty-or-filtered-discovery")
	}
	if len(r.Tests) > testrunner.MaxTests {
		return CommandResponse{}, fmt.Errorf("python-test-cap")
	}
	seen := map[string]bool{}
	for _, id := range r.Tests {
		if !pythonTestID(p.runner, id) || seen[id] {
			return CommandResponse{}, fmt.Errorf("python-filter-unsupported")
		}
		seen[id] = true
	}
	key, _ := json.Marshal(r.Key)
	tests, _ := json.Marshal(r.Tests)
	return CommandResponse{Argv: []string{"python3", "-c", pythonCaptureProgram, p.runner, r.Purpose, string(key), string(tests), filepath.Join(r.RunDir, "coverage.json")}, Cwd: ".", Artifacts: pythonArtifacts(r.Purpose, r.RunDir)}, nil
}
func pythonArtifacts(purpose, dir string) []string {
	if purpose == "collect" {
		return []string{filepath.Join(dir, "coverage.json")}
	}
	return nil
}

func (p *pythonPlugin) Parse(ctx context.Context, r ParseRequest) ([]testrunner.RunnerResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.Runner != p.runner {
		return nil, fmt.Errorf("python-runner-mismatch")
	}
	var results []testrunner.RunnerResult
	if err := pythonDecode([]byte(r.Report), &results); err != nil {
		return nil, err
	}
	if results == nil || len(results) > testrunner.MaxTests {
		return nil, fmt.Errorf("python-result-cap-or-absent")
	}
	seen := map[string]bool{}
	for _, result := range results {
		if !pythonTestID(p.runner, result.ID) || seen[result.ID] {
			return results, fmt.Errorf("python-result-identity")
		}
		seen[result.ID] = true
		switch result.Outcome {
		case "passed", "failed":
		case "skipped":
			if strings.TrimSpace(result.Reason) == "" {
				return results, fmt.Errorf("python-skip-reason-missing")
			}
		default:
			return results, fmt.Errorf("python-result-nonterminal")
		}
	}
	return results, nil
}

func (p *pythonPlugin) Evidence(ctx context.Context, r EvidenceRequest) ([]NativeEvidence, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.Runner != p.runner || len(r.Artifacts) != 0 {
		return nil, fmt.Errorf("python-use-captured-artifacts")
	}
	if len(p.capture.Tests) == 0 || len(p.capture.Tests) > testrunner.MaxTests {
		return nil, fmt.Errorf("python-attribution-missing-or-capped")
	}
	seen := map[string]bool{}
	for _, id := range p.capture.Tests {
		if !pythonTestID(p.runner, id) || seen[id] {
			return nil, fmt.Errorf("python-attribution-invalid")
		}
		seen[id] = true
	}
	native := PythonCoverage(p.capture.Coverage, p.capture.Sources)
	executed, _ := json.Marshal(native.Executed)
	regions, _ := json.Marshal(native.Regions)
	loaded, _ := json.Marshal(native.Loaded)
	out := make([]NativeEvidence, 0, len(p.capture.Tests))
	// No per-context narrowing: every test of the tree gets the entire union.
	for _, id := range p.capture.Tests {
		out = append(out, NativeEvidence{Test: id, Executed: append(json.RawMessage(nil), executed...), Regions: append(json.RawMessage(nil), regions...), Loaded: append(json.RawMessage(nil), loaded...)})
	}
	return out, nil
}

// PythonRecord joins only core-owned trace components with native coverage.
// Assembly validates the existing record and does not persist it.
func PythonRecord(header testevidence.Record, native testevidence.Native, trace testevidence.Trace) (testevidence.Record, error) {
	if header.Runner.Plugin != runnernames.RunnerPytest && header.Runner.Plugin != runnernames.RunnerUnittest {
		return testevidence.Record{}, fmt.Errorf("python-record-runner-mismatch")
	}
	return testevidence.Assemble(header, native, trace)
}
