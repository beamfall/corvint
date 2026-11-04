// SPDX-License-Identifier: Apache-2.0

package rubylinecov

import (
	"fmt"
	"path"
	"strings"

	"github.com/Beamfall/corvint/internal/linecov"
)

// IsTestFile reports whether p is a test file by the RSpec, Minitest and
// test-unit naming conventions.
func IsTestFile(p string) bool {
	base := path.Base(p)
	return strings.HasSuffix(base, "_spec.rb") || strings.HasSuffix(base, "_test.rb") ||
		strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".rb")
}

// Analyze maps a diff to its effect on a Ruby index. Only an edit strictly
// inside one method body (between its def and end lines) of an instrumented
// file whose old blob the index recorded is mapped, and it selects every test
// that called the method or ran any of its body lines: Ruby's line counts
// are not a safe per-line owner (a `when` condition is evaluated without its
// line being counted). A changed test file runs whole. Every other change,
// including one reaching code a file load or a window outside every test
// ran, is a fallback (LPCV-V0-952, LPCV-V0-953).
func Analyze(ix *linecov.Index, diffs []linecov.FileDiff) linecov.Effect {
	e := linecov.Effect{Tests: map[int32]bool{}, Whole: map[string]bool{}, Touched: map[string]bool{}}
	fail := func(format string, args ...any) linecov.Effect {
		return linecov.Effect{Fallback: fmt.Sprintf(format, args...)}
	}
	testFiles := map[string]bool{}
	load := int32(-1)
	for i, t := range ix.Tests {
		if t.Package == Load {
			load = int32(i)
		} else {
			testFiles[t.Package] = true
		}
	}
	for _, d := range diffs {
		if !strings.HasSuffix(d.Path, ".rb") {
			return fail("non-Ruby input %s", d.Path)
		}
		if testFiles[d.Path] || IsTestFile(d.Path) {
			e.Whole[d.Path] = true
			continue
		}
		if dir, _, _ := strings.Cut(d.Path, "/"); dir == "spec" || dir == "test" {
			return fail("test helper %s", d.Path)
		}
		if d.OldOID == "" || d.NewOID == "" {
			return fail("added or deleted file %s", d.Path)
		}
		f, ok := ix.FileByPath(d.Path)
		if !ok {
			return fail("uninstrumented file %s", d.Path)
		}
		if len(ix.TestsOfFile(d.Path)) == 0 {
			return fail("unexecuted file %s", d.Path)
		}
		if f.OID != d.OldOID {
			return fail("index records %s at another blob", d.Path)
		}
		for _, h := range d.Hunks {
			if !selectHunk(ix, f, h, e.Tests) {
				return fail("edit outside a method body %s:%d", d.Path, h.Start)
			}
		}
	}
	if load >= 0 && e.Tests[load] {
		return fail("the change reaches load-time code")
	}
	return e
}

// selectHunk adds the tests of the innermost method whose body holds the
// hunk, and reports whether there is one.
func selectHunk(ix *linecov.Index, f *linecov.File, h linecov.Hunk, out map[int32]bool) bool {
	lo, hi := int32(h.Start), int32(h.End)
	var method *linecov.Block
	candidates := ix.BlocksAtLine(f.Path, int32(h.Start))
	for i := range candidates {
		b := &candidates[i]
		if b.StartCol == 0 { // an executable line, not a method
			continue
		}
		inside := b.Start < lo && hi < b.End
		if hi < lo { // an insertion between lines lo and lo+1
			inside = b.Start <= lo && lo+1 <= b.End
		}
		if inside && (method == nil || b.End-b.Start < method.End-method.Start) {
			method = b
		}
	}
	if method == nil || len(method.Tests) == 0 {
		return false
	}
	for _, t := range method.Tests {
		out[t] = true
	}
	for i := range f.Blocks {
		if b := &f.Blocks[i]; b.StartCol == 0 && method.Start < b.Start && b.Start < method.End {
			for _, t := range b.Tests {
				out[t] = true
			}
		}
	}
	return true
}

// Plan decides each test file of static from a Ruby index: run whole, run
// only the selected tests, or exclude. drift is the effect of the history
// between the index commit and the change base. A test file with no record,
// or one Index listed as unordered (today every recorded file), runs whole
// (LPCV-V0-951, LPCV-V0-962).
func Plan(ix *linecov.Index, static []string, drift, change linecov.Effect) (map[string]linecov.PackagePlan, string) {
	head := map[string][]string{}
	for _, t := range ix.Tests {
		if t.Package != Load {
			head[t.Package] = append(head[t.Package], t.Name)
		}
	}
	plans, reason := ix.Narrow(static, head, drift, change)
	for _, file := range static {
		if head[file] == nil {
			plans[file] = linecov.PackagePlan{Mode: linecov.Whole}
		}
	}
	return plans, reason
}
