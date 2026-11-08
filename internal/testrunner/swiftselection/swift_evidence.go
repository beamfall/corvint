// SPDX-License-Identifier: Apache-2.0

package swiftselection

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/linecov/v8"
	"github.com/Beamfall/corvint/internal/nativelinecov"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

// SwiftCoverage is a core-captured `llvm-cov export -format=lcov` line report.
// Complete covers exporter success and uncapped capture. Tests is the entire
// core-observed process-tree run set, not a plugin-inferred single-test label.
// With no qualified markers the same report is attributed to every such test.
// The core supplies trace components separately when assembling the record.
type SwiftCoverage struct {
	LCOV     []byte
	Tests    []string
	Complete bool
}

func (p *swiftPlugin) Evidence(ctx context.Context, r registry.EvidenceRequest) ([]registry.NativeEvidence, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.Runner != p.options.Key.Runner || len(r.Artifacts) == 0 || len(r.Artifacts) > testrunner.MaxReports {
		return nil, fmt.Errorf("swift-coverage-unavailable")
	}
	sources := make(map[string]string, len(p.options.Sources))
	blobs := make(map[string]testevidence.Blob, len(p.options.Sources))
	for name, blob := range p.options.Sources {
		sources[name], blobs[blob.Path.Lexical] = blob.Path.Lexical, blob
	}
	known := map[string]bool{}
	for _, test := range p.options.Tests {
		known[test.ID] = true
	}
	byTest := map[string]v8.Lines{}
	seen := map[string]bool{}
	edges := 0
	for _, handle := range r.Artifacts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		capture, ok := p.options.Coverage[handle]
		if !ok || seen[handle] || !capture.Complete || len(capture.Tests) == 0 || len(capture.Tests) > testrunner.MaxTests {
			return nil, fmt.Errorf("swift-coverage-capture-invalid")
		}
		seen[handle] = true
		if !utf8.Valid(capture.LCOV) || len(capture.LCOV) == 0 || capture.LCOV[len(capture.LCOV)-1] != '\n' {
			return nil, fmt.Errorf("swift-coverage-malformed-or-truncated")
		}
		lines, err := nativelinecov.Decode(nativelinecov.Report{Format: "lcov", Data: capture.LCOV}, sources)
		if err != nil {
			return nil, fmt.Errorf("swift-coverage-invalid:%w", err)
		}
		lineCount := 0
		for _, file := range lines {
			lineCount += len(file)
		}
		// Bound the union before copying a shared report to its whole tree.
		// A cap never produces partial exclusion evidence.
		if lineCount > 100000 || len(capture.Tests) > (100000-edges)/lineCount {
			return nil, fmt.Errorf("swift-coverage-edge-cap-exceeded")
		}
		edges += lineCount * len(capture.Tests)
		seenTests := map[string]bool{}
		for _, test := range capture.Tests {
			if !known[test] || seenTests[test] {
				return nil, fmt.Errorf("swift-coverage-attribution-invalid")
			}
			seenTests[test] = true
			if byTest[test] == nil {
				byTest[test] = v8.Lines{}
			}
			byTest[test].Merge(lines)
		}
	}
	ids := make([]string, 0, len(byTest))
	for test := range byTest {
		ids = append(ids, test)
	}
	sort.Strings(ids)
	out := make([]registry.NativeEvidence, 0, len(ids))
	outputBytes := 0
	for _, id := range ids {
		native := testevidence.Native{
			Executed: testevidence.Component[testevidence.ExecutedFile]{State: testevidence.Present},
			// LLVM DA records describe instrumented lines, not executable
			// bodies or a complete build/load graph. Never promote either
			// unknown component to present by guessing from line numbers.
			Regions: testevidence.Missing[testevidence.RegionsFile]("llvm-cov-lines-no-body-map"),
			Loaded:  testevidence.Missing[testevidence.Blob]("swift-build-graph-unavailable"),
		}
		paths := make([]string, 0, len(byTest[id]))
		for path := range byTest[id] {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		for _, path := range paths {
			file := testevidence.ExecutedFile{Blob: blobs[path]}
			lines := make([]int, 0, len(byTest[id][path]))
			for line := range byTest[id][path] {
				lines = append(lines, line)
			}
			sort.Ints(lines)
			for _, line := range lines {
				range_ := testevidence.Range{Start: int32(line), StartColumn: 1, End: int32(line + 1), EndColumn: 1}
				if byTest[id][path][line] {
					file.Executed = append(file.Executed, range_)
				} else {
					file.Unexecuted = append(file.Unexecuted, range_)
				}
			}
			native.Executed.Items = append(native.Executed.Items, file)
		}
		// Reuse T1's validator before producing the SPI's opaque payloads.
		check := testevidence.New("", "", "")
		check.Executed, check.Regions, check.Loaded = native.Executed, native.Regions, native.Loaded
		if err := check.ValidateComponents(); err != nil {
			return nil, err
		}
		executed, err := json.Marshal(native.Executed)
		if err != nil {
			return nil, err
		}
		regions, err := json.Marshal(native.Regions)
		if err != nil {
			return nil, err
		}
		loaded, err := json.Marshal(native.Loaded)
		if err != nil {
			return nil, err
		}
		outputBytes += len(executed) + len(regions) + len(loaded) + len(id)
		if outputBytes > testevidence.MaxRecordBytes {
			return nil, fmt.Errorf("swift-coverage-output-cap-exceeded")
		}
		out = append(out, registry.NativeEvidence{Test: id, Executed: executed, Regions: regions, Loaded: loaded})
	}
	return out, nil
}
