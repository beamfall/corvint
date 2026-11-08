// SPDX-License-Identifier: Apache-2.0

package rubylinecov

import (
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/rubydialect"
	"github.com/Beamfall/corvint/internal/testevidence"
)

// SelectionSource is core-captured immutable Git content. The adapter checks
// the Git blob hash rather than binding coverage to a mutable worktree file.
type SelectionSource struct {
	OID   string
	Bytes []byte
}

// SelectionNative retains the collector identity without guessing its mapping
// to a runner inventory (notably parameterized Test::Unit cases).
type SelectionNative struct {
	Package, Name string
	Native        testevidence.Native
}

// SelectionEvidence adapts the existing Ruby Coverage collector, including its
// isolated-run checks, without executing Ruby or writing any record store.
// The core supplies identity, provenance and trace through testevidence.Assemble.
// Regions deliberately use a conservative whole-file enclosing body: the Ruby
// dialect is lexical and cannot certify precise method ends. Loaded remains
// absent because coverage shapes are not a complete loaded-feature inventory.
func SelectionEvidence(run *Run, repository string, sources map[string]SelectionSource) ([]SelectionNative, error) {
	if run == nil || repository == "" {
		return nil, fmt.Errorf("ruby-coverage-missing")
	}
	if len(run.records) > 4097 || len(run.Outcomes) > 4096 || len(run.files) > 4096 || len(sources) > 4096 {
		return nil, fmt.Errorf("ruby-coverage-cap-exceeded")
	}
	paths := make([]string, 0, len(run.files))
	for p := range run.files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	records := map[string]record{}
	var load record
	loadSeen := false
	for _, r := range run.records {
		if r.pkg == Load {
			if loadSeen {
				return nil, fmt.Errorf("ruby-load-coverage-duplicate")
			}
			loadSeen = true
			load = r
			continue
		}
		k := r.pkg + "\x00" + r.name
		if _, ok := records[k]; ok {
			return nil, fmt.Errorf("ruby-coverage-duplicate")
		}
		records[k] = r
	}
	var out []SelectionNative
	seen := map[string]bool{}
	for _, o := range run.Outcomes {
		k := o.Package + "\x00" + o.Name
		if o.Package == "" || o.Name == "" || seen[k] {
			return nil, fmt.Errorf("ruby-coverage-identity-invalid")
		}
		seen[k] = true
		n := testevidence.Native{
			Executed: testevidence.Component[testevidence.ExecutedFile]{State: testevidence.Present},
			Regions:  testevidence.Component[testevidence.RegionsFile]{State: testevidence.Present},
			Loaded:   testevidence.Missing[testevidence.Blob]("ruby-loaded-inventory-unavailable"),
		}
		incomplete := func(reason string) { n.Executed.State, n.Executed.Reason = testevidence.Incomplete, reason }
		r, ok := records[k]
		if !loadSeen {
			incomplete("ruby-load-coverage-missing")
		}
		if !run.isolated || !o.Recorded || o.Status != "passed" || !ok {
			incomplete("ruby-coverage-unattributed-or-incomplete")
		}
		if len(paths) == 0 {
			incomplete("ruby-coverage-files-missing")
		}
		for _, p := range paths {
			source, ok := sources[p]
			if !ok || !selectionBlob(source) {
				incomplete("ruby-source-unpinned")
				n.Regions = testevidence.Missing[testevidence.RegionsFile]("ruby-source-unpinned")
				continue
			}
			lines := strings.Split(string(source.Bytes), "\n")
			if len(lines) > 100000 {
				incomplete("ruby-source-line-cap")
				n.Regions = testevidence.Missing[testevidence.RegionsFile]("ruby-source-line-cap")
				continue
			}
			blob := testevidence.Blob{Path: testevidence.Path{Repository: repository, Lexical: p, Resolved: p}, OID: source.OID}
			shape := run.files[p]
			if len(shape.Lines) > 100000 || len(r.lines[p]) > 100000 || len(load.lines[p]) > 100000 {
				incomplete("ruby-coverage-line-cap")
				continue
			}
			covered := map[int32]bool{}
			for _, line := range append(append([]int32{}, r.lines[p]...), load.lines[p]...) {
				covered[line] = true
			}
			executable := map[int32]bool{}
			for _, line := range shape.Lines {
				executable[line] = true
			}
			// A counter outside the shape is never silently discarded.
			for line := range covered {
				if !executable[line] {
					incomplete("ruby-coverage-shape-mismatch")
				}
			}
			ef := testevidence.ExecutedFile{Blob: blob}
			ordered := make([]int, 0, len(executable))
			for line := range executable {
				ordered = append(ordered, int(line))
			}
			sort.Ints(ordered)
			for _, line := range ordered {
				if line < 1 || line > len(lines) {
					incomplete("ruby-coverage-line-invalid")
					continue
				}
				span := testevidence.Range{Start: int32(line), StartColumn: 1, End: int32(line), EndColumn: int32(max(2, len(lines[line-1])+1))}
				if covered[int32(line)] {
					ef.Executed = append(ef.Executed, span)
				} else {
					ef.Unexecuted = append(ef.Unexecuted, span)
				}
			}
			n.Executed.Items = append(n.Executed.Items, ef)
			extraction := rubydialect.Symbols(string(source.Bytes), 100000, 10000)
			if extraction.Stop != "" || extraction.UnterminatedComment || extraction.UnterminatedString {
				n.Regions = testevidence.Missing[testevidence.RegionsFile]("ruby-dialect-incomplete")
			} else if n.Regions.State == testevidence.Present {
				n.Regions.Items = append(n.Regions.Items, testevidence.RegionsFile{Blob: blob, Bodies: []testevidence.Body{{ID: "file", Range: testevidence.Range{Start: 1, StartColumn: 1, End: int32(len(lines)), EndColumn: int32(max(2, len(lines[len(lines)-1])+1))}}}})
			}
			// Conservative even for text in strings/comments: a lexical scanner is
			// not a proof that interpolation, aliases or dynamic reads are absent.
			// Actual trace edges remain core-owned; this never manufactures them.
			for _, marker := range []string{"ENV", "File", "IO", "Dir", "eval", "send", "autoload"} {
				if strings.Contains(string(source.Bytes), marker) {
					incomplete("ruby-computed-read-or-dispatch")
				}
			}
		}
		// Missing shapes for an observed executed file cannot certify a subset.
		for _, executed := range []map[string][]int32{r.lines, load.lines} {
			for p := range executed {
				if _, ok := run.files[p]; !ok {
					incomplete("ruby-coverage-shape-missing")
				}
			}
		}
		check := testevidence.New(repository, o.Name, "")
		check.Executed, check.Regions, check.Loaded = n.Executed, n.Regions, n.Loaded
		if err := check.ValidateComponents(); err != nil {
			return nil, fmt.Errorf("ruby-coverage-invalid:%w", err)
		}
		out = append(out, SelectionNative{Package: o.Package, Name: o.Name, Native: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Package == out[j].Package {
			return out[i].Name < out[j].Name
		}
		return out[i].Package < out[j].Package
	})
	return out, nil
}

func selectionBlob(s SelectionSource) bool {
	if len(s.Bytes) > 1<<20 || !utf8.Valid(s.Bytes) {
		return false
	}
	data := append([]byte(fmt.Sprintf("blob %d\x00", len(s.Bytes))), s.Bytes...)
	switch len(s.OID) {
	case 40:
		return fmt.Sprintf("%x", sha1.Sum(data)) == s.OID
	case 64:
		return fmt.Sprintf("%x", sha256.Sum256(data)) == s.OID
	}
	return false
}

// SelectionCollector returns the existing Coverage instrumentation for core staging.
// The core owns the fresh dump directory and captures its complete process tree.
func SelectionCollector() []byte { return append([]byte{}, script...) }
