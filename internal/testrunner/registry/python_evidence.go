// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/pythongrammar"
	"github.com/Beamfall/corvint/internal/pythonsyntax"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
)

// PythonSource is a core-captured immutable blob, never a dirty worktree read.
// Coverage's lexical path must equal Blob.Path.Lexical. Bytes are checked against
// the Git object hash before they supply regions or line bounds.
type PythonSource struct {
	Blob testevidence.Blob
	Text string
}

// PythonCoverage reads coverage.py JSON line records (format 2 and 3). It does
// not trust dynamic contexts as per-test attribution or infer loaded modules
// from the files for which coverage happened to report lines. Unknown format,
// excluded lines, malformed/capped data and missing blobs stay incomplete.
func PythonCoverage(raw []byte, sources map[string]PythonSource) testevidence.Native {
	out := testevidence.Native{Executed: testevidence.Missing[testevidence.ExecutedFile]("coverage-not-collected"), Regions: testevidence.Missing[testevidence.RegionsFile]("regions-not-collected"), Loaded: testevidence.Missing[testevidence.Blob]("python-loaded-unobserved")}
	fail := func(reason string) testevidence.Native {
		out.Executed = testevidence.Component[testevidence.ExecutedFile]{State: testevidence.Incomplete, Reason: reason}
		out.Regions = testevidence.Component[testevidence.RegionsFile]{State: testevidence.Incomplete, Reason: reason}
		return out
	}
	if len(raw) == 0 {
		return out
	}
	if len(raw) > testrunner.MaxReportBytes {
		return fail("python-coverage-cap")
	}
	// Native reports include version-specific summaries, branches and contexts.
	// Decode through v2 first to reject duplicate keys and invalid UTF-8 anywhere.
	var document map[string]json.RawMessage
	if err := jsonv2.Unmarshal(raw, &document); err != nil {
		return fail("python-coverage-malformed")
	}
	for key := range document {
		if key != "meta" && key != "files" && key != "totals" {
			return fail("python-coverage-member-unsupported")
		}
	}
	var meta struct {
		Format  int    `json:"format"`
		Version string `json:"version"`
	}
	if err := jsonv2.Unmarshal(document["meta"], &meta); err != nil || (meta.Format != 2 && meta.Format != 3) || meta.Version == "" {
		return fail("python-coverage-format")
	}
	var files map[string]json.RawMessage
	if err := jsonv2.Unmarshal(document["files"], &files); err != nil || len(files) == 0 || len(files) > testrunner.MaxTests {
		return fail("python-coverage-files")
	}
	out.Executed = testevidence.Component[testevidence.ExecutedFile]{State: testevidence.Present}
	out.Regions = testevidence.Component[testevidence.RegionsFile]{State: testevidence.Present}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	total := 0
	for _, name := range names {
		source, ok := sources[name]
		if !ok || source.Blob.Path.Lexical != name || !pythonPinned(source) {
			return fail("python-coverage-source-unpinned")
		}
		var lines struct {
			Executed []int `json:"executed_lines"`
			Missing  []int `json:"missing_lines"`
			Excluded []int `json:"excluded_lines"`
		}
		if err := jsonv2.Unmarshal(files[name], &lines); err != nil || lines.Executed == nil || lines.Missing == nil || lines.Excluded == nil {
			return fail("python-coverage-lines-malformed")
		}
		if len(lines.Excluded) > 0 {
			return fail("python-coverage-excluded-lines")
		}
		total += len(lines.Executed) + len(lines.Missing)
		if total > 100000 {
			return fail("python-coverage-line-cap")
		}
		maxLine := strings.Count(source.Text, "\n")
		if !strings.HasSuffix(source.Text, "\n") {
			maxLine++
		}
		sourceLines := strings.Split(source.Text, "\n")
		seen := map[int]bool{}
		convert := func(values []int) ([]testevidence.Range, bool) {
			values = append([]int(nil), values...)
			sort.Ints(values)
			ranges := []testevidence.Range{}
			for _, line := range values {
				if line <= 0 || line > maxLine || seen[line] {
					return nil, false
				}
				seen[line] = true
				ranges = append(ranges, testevidence.Range{Start: int32(line), StartColumn: 1, End: int32(line), EndColumn: int32(len(sourceLines[line-1]) + 1)})
			}
			return ranges, true
		}
		executed, ok := convert(lines.Executed)
		if !ok {
			return fail("python-coverage-line-invalid")
		}
		missing, ok := convert(lines.Missing)
		if !ok {
			return fail("python-coverage-line-invalid")
		}
		out.Executed.Items = append(out.Executed.Items, testevidence.ExecutedFile{Blob: source.Blob, Executed: executed, Unexecuted: missing})
		regions, reason := pythonRegions(name, source)
		if reason != "" {
			out.Regions.State = testevidence.Incomplete
			out.Regions.Reason = reason
		} else {
			out.Regions.Items = append(out.Regions.Items, regions)
		}
	}
	probe := testevidence.New("", "", "")
	probe.Executed = out.Executed
	probe.Regions = out.Regions
	probe.Loaded = out.Loaded
	if err := probe.ValidateComponents(); err != nil {
		return fail("python-coverage-component-invalid")
	}
	return out
}

func pythonPinned(source PythonSource) bool {
	if len(source.Text) == 0 || len(source.Text) > testrunner.MaxReportBytes || !utf8.ValidString(source.Text) {
		return false
	}
	raw := []byte(fmt.Sprintf("blob %d\x00%s", len(source.Text), source.Text))
	var oid string
	switch len(source.Blob.OID) {
	case 40:
		sum := sha1.Sum(raw)
		oid = hex.EncodeToString(sum[:])
	case 64:
		sum := sha256.Sum256(raw)
		oid = hex.EncodeToString(sum[:])
	default:
		return false
	}
	return oid == source.Blob.OID
}

type pythonSpans struct {
	bodies []testevidence.Body
	capped bool
}

func (s *pythonSpans) Add(_, _, _, _ string, _, _ int) pythongrammar.Failure { return "" }
func (s *pythonSpans) DefinitionEnd(start, end int) {
	if len(s.bodies) >= testrunner.MaxTests {
		s.capped = true
		return
	}
	s.bodies = append(s.bodies, testevidence.Body{ID: "definition:" + strconv.Itoa(start), Range: testevidence.Range{Start: int32(start), End: int32(end)}})
}
func pythonRegions(name string, source PythonSource) (testevidence.RegionsFile, string) {
	spans := &pythonSpans{}
	if !strings.HasSuffix(name, ".py") || !pythonsyntax.SourceSyntaxValid(name, []byte(source.Text)) || pythonsyntax.ParseValidated(name, []byte(source.Text), spans) != "" {
		return testevidence.RegionsFile{}, "python-regions-unsupported"
	}
	if spans.capped {
		return testevidence.RegionsFile{}, "python-regions-cap"
	}
	sort.Slice(spans.bodies, func(i, j int) bool { return spans.bodies[i].Range.Start < spans.bodies[j].Range.Start })
	sourceLines := strings.Split(source.Text, "\n")
	for i := range spans.bodies {
		spans.bodies[i].Range.StartColumn = 1
		spans.bodies[i].Range.EndColumn = int32(len(sourceLines[spans.bodies[i].Range.End-1]) + 1)
		for j := i - 1; j >= 0; j-- {
			if spans.bodies[j].Range.End >= spans.bodies[i].Range.End {
				spans.bodies[i].Parent = spans.bodies[j].ID
				break
			}
		}
	}
	return testevidence.RegionsFile{Blob: source.Blob, Bodies: spans.bodies}, ""
}
