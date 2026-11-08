// SPDX-License-Identifier: Apache-2.0
package jslinecov

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/linecov/v8"
	"github.com/Beamfall/corvint/internal/testevidence"
)

// Source is original immutable content, not generated JS substituted for TS.
type Source struct {
	OID  string
	Text []byte
}

// CoverageCapture is the core's bounded, source-mapped native collection. The
// caller supplies only proved whole-line states in Lines; partial or mapped
// summaries without full coverage boundaries must be omitted. Completeness is lost
// on any artifact, map, binding or cap failure. Loaded includes modules with no
// instrumented lines. Missing loader accounting must leave LoadedComplete false.
type CoverageCapture struct {
	Repository               string
	Sources                  map[string]Source
	Lines                    v8.Lines
	Loaded                   []string
	Complete, LoadedComplete bool
	Reason                   string
}

func missingNative(reason string) testevidence.Native {
	return testevidence.Native{Executed: testevidence.Missing[testevidence.ExecutedFile](reason), Regions: testevidence.Missing[testevidence.RegionsFile](reason), Loaded: testevidence.Missing[testevidence.Blob](reason)}
}

func SourceMatches(s Source) bool {
	prefix := fmt.Appendf(nil, "blob %d\x00", len(s.Text))
	raw := append(prefix, s.Text...)
	if len(s.OID) == 40 {
		h := sha1.Sum(raw)
		return s.OID == hex.EncodeToString(h[:])
	}
	if len(s.OID) == 64 {
		h := sha256.Sum256(raw)
		return s.OID == hex.EncodeToString(h[:])
	}
	return false
}

// NativeCoverage reuses T1's native components. Unsupported syntax never
// becomes an empty complete region map; exact columns aren't inferred from the
// line scanner, so only whole lines strictly between braces become regions.
func NativeCoverage(c CoverageCapture) testevidence.Native {
	if !c.Complete {
		r := c.Reason
		if r == "" {
			r = "coverage-incomplete"
		}
		return missingNative(r)
	}
	if c.Repository == "" || len(c.Sources) > 4096 || len(c.Lines) > 4096 || len(c.Loaded) > 4096 {
		return missingNative("coverage-cap-or-identity")
	}
	out := testevidence.Native{Executed: testevidence.Component[testevidence.ExecutedFile]{State: testevidence.Present}, Regions: testevidence.Component[testevidence.RegionsFile]{State: testevidence.Present}, Loaded: testevidence.Missing[testevidence.Blob]("loader-accounting-absent")}
	blob := func(p string) (testevidence.Blob, bool) {
		s, ok := c.Sources[p]
		return testevidence.Blob{Path: testevidence.Path{Repository: c.Repository, Lexical: p, Resolved: p}, OID: s.OID}, ok && SafeRunnerPath(p) && len(s.Text) <= MaxRunnerArtifact && SourceMatches(s)
	}
	paths := []string{}
	for p := range c.Lines {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	count := 0
	for _, p := range paths {
		b, ok := blob(p)
		if !ok {
			return missingNative("coverage-source-unbound")
		}
		source := c.Sources[p].Text
		nlines := strings.Count(string(source), "\n") + 1
		f := testevidence.ExecutedFile{Blob: b}
		lines := []int{}
		for l := range c.Lines[p] {
			lines = append(lines, l)
		}
		sort.Ints(lines)
		for _, l := range lines {
			count++
			if count > 65536 || l < 1 || l > nlines {
				return missingNative("coverage-range-invalid")
			}
			r := testevidence.Range{Start: int32(l), StartColumn: 1, End: int32(l + 1), EndColumn: 1}
			if c.Lines[p][l] {
				f.Executed = append(f.Executed, r)
			} else {
				f.Unexecuted = append(f.Unexecuted, r)
			}
		}
		out.Executed.Items = append(out.Executed.Items, f)
		bs, err := bodies(source)
		if err != nil {
			out.Regions.State, out.Regions.Reason = testevidence.Incomplete, "regions-syntax-unsupported"
			continue
		}
		region := testevidence.RegionsFile{Blob: b}
		for i, body := range bs {
			if body.close <= body.open+1 {
				continue
			}
			region.Bodies = append(region.Bodies, testevidence.Body{ID: strconv.Itoa(i), Range: testevidence.Range{Start: int32(body.open + 1), StartColumn: 1, End: int32(body.close), EndColumn: 1}})
		}
		for i := range region.Bodies {
			child := &region.Bodies[i]
			best := int32(1 << 30)
			for j, parent := range region.Bodies {
				if i != j && parent.Range.Start < child.Range.Start && parent.Range.End > child.Range.End && parent.Range.End-parent.Range.Start < best {
					child.Parent = parent.ID
					best = parent.Range.End - parent.Range.Start
				}
			}
		}
		out.Regions.Items = append(out.Regions.Items, region)
	}
	if c.LoadedComplete {
		out.Loaded = testevidence.Component[testevidence.Blob]{State: testevidence.Present}
		seen := map[string]bool{}
		for _, p := range append(append([]string{}, c.Loaded...), paths...) {
			if seen[p] {
				continue
			}
			seen[p] = true
			b, ok := blob(p)
			if !ok {
				return missingNative("loaded-source-unbound")
			}
			out.Loaded.Items = append(out.Loaded.Items, b)
		}
		sort.Slice(out.Loaded.Items, func(i, j int) bool { return out.Loaded.Items[i].Path.Lexical < out.Loaded.Items[j].Path.Lexical })
	}
	check := testevidence.New(c.Repository, "native", "unused")
	check.Executed, check.Regions, check.Loaded = out.Executed, out.Regions, out.Loaded
	if check.ValidateComponents() != nil {
		return missingNative("native-components-malformed")
	}
	return out
}
