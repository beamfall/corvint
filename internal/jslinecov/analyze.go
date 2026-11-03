// Package jslinecov is the JavaScript and TypeScript line-level coverage
// selection of docs/specs/live-proof-carrying-verification-v0.md
// (LPCV-V0-965..969): per-test V8 line records from the node runners, kept in
// the shared internal/linecov index, and the analysis that maps a change to
// the tests it can affect. A record's Package is the repository-relative test
// file and its Name the test's full title.
package jslinecov

import (
	"fmt"
	"path"
	"strings"

	"github.com/Beamfall/corvint/internal/linecov"
)

// sourceExt are the suffixes whose bodies the scanner reads.
var sourceExt = []string{".js", ".mjs", ".cjs", ".jsx", ".ts", ".mts", ".cts", ".tsx"}

// IsSource reports whether p is a JavaScript or TypeScript source.
func IsSource(p string) bool {
	for _, x := range sourceExt {
		if strings.HasSuffix(p, x) {
			return true
		}
	}
	return false
}

// IsTestFile reports whether p is named like a test file of the supported
// runners' default discovery.
func IsTestFile(p string) bool {
	base := path.Base(p)
	return strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") ||
		strings.Contains(base, "_test.") || strings.Contains("/"+p, "/__tests__/")
}

// Analyze maps a diff to its effect on a JavaScript index, as
// linecov.Index.Analyze does for Go. read returns a file's old-side content.
// A changed recorded test file, or an added test file, keeps that file whole;
// an unrecorded existing one may be a helper other tests load, so it falls
// back. A change to any other file is
// mapped by line only when it lies inside one function body of a file the
// index recorded at the diff's old blob; anything else is a fallback, so the
// static plan stands (LPCV-V0-966).
func Analyze(ix *linecov.Index, diffs []linecov.FileDiff, read func(string) ([]byte, error)) linecov.Effect {
	e := linecov.Effect{Tests: map[int32]bool{}, Whole: map[string]bool{}, Touched: map[string]bool{}}
	fail := func(format string, args ...any) linecov.Effect {
		return linecov.Effect{Fallback: fmt.Sprintf(format, args...)}
	}
	recorded := map[string]bool{}
	for _, t := range ix.Tests {
		recorded[t.Package] = true
	}
	for _, p := range ix.Unordered {
		recorded[p] = true
	}
	files := map[string]*linecov.File{}
	for i := range ix.Files {
		files[ix.Files[i].Path] = &ix.Files[i]
	}
	for _, d := range diffs {
		if !IsSource(d.Path) {
			return fail("non-JavaScript input %s", d.Path)
		}
		e.Touched[d.Path] = true
		if recorded[d.Path] || IsTestFile(d.Path) && d.OldOID == "" {
			if f, ok := files[d.Path]; ok && loadedElsewhere(ix, f) {
				return fail("test file %s runs under other test files", d.Path)
			}
			e.Whole[d.Path] = true
			continue
		}
		if d.OldOID == "" || d.NewOID == "" {
			return fail("added or deleted file %s", d.Path)
		}
		f, ok := files[d.Path]
		if !ok {
			return fail("uninstrumented file %s", d.Path)
		}
		if f.OID != d.OldOID {
			return fail("index records %s at another blob", d.Path)
		}
		src, err := read(d.Path)
		if err != nil {
			return fail("unreadable %s: %v", d.Path, err)
		}
		bs, err := bodies(src)
		if err != nil {
			return fail("unscannable %s: %v", d.Path, err)
		}
		lines := lineTests(f)
		for _, h := range d.Hunks {
			lo, hi := h.Start, h.End
			insert := hi < lo
			if insert { // either neighbouring line may run it
				hi = lo + 1
			}
			b, ok := innermost(bs, lo, hi, insert)
			if !ok {
				return fail("declaration-level edit %s:%d", d.Path, h.Start)
			}
			if !selectLines(lines, bs, b, lo, hi, e.Tests) {
				return fail("function body without instrumented line %s:%d", d.Path, b.open)
			}
		}
	}
	for id := range e.Tests {
		if t := ix.Tests[id]; t.Name == linecov.InOrder {
			e.Whole[t.Package] = true
		}
	}
	return e
}

// loadedElsewhere reports whether a test of another file executed a line of
// test file f, which then is a module those tests load: rerunning f's own
// tests would miss them.
func loadedElsewhere(ix *linecov.Index, f *linecov.File) bool {
	for _, b := range f.Blocks {
		for _, id := range b.Tests {
			if int(id) < len(ix.Tests) && ix.Tests[id].Package != f.Path {
				return true
			}
		}
	}
	return false
}

// lineTests maps each instrumented line of f to the tests that executed it.
// The JavaScript collector writes one block per line (LPCV-V0-965).
func lineTests(f *linecov.File) map[int][]int32 {
	out := map[int][]int32{}
	for _, b := range f.Blocks {
		for l := b.Start; l < b.End || l == b.Start; l++ {
			out[int(l)] = append(out[int(l)], b.Tests...)
		}
	}
	return out
}

// innermost is the narrowest body strictly containing [lo, hi], or, for an
// insertion between lo and hi, whose braces lie on or outside those lines.
func innermost(bs []body, lo, hi int, insert bool) (body, bool) {
	var best body
	found := false
	for _, b := range bs {
		inside := b.open < lo && hi < b.close
		if insert {
			inside = b.open <= lo && hi <= b.close
		}
		if inside && (!found || b.close-b.open < best.close-best.open) {
			best, found = b, true
		}
	}
	return best, found
}

// selectLines adds the tests that executed a line of [lo, hi]. A line no
// block holds (blank, a comment, a brace, a type) adds every test that
// executed a line of body b, since running code there requires entering b.
// So does the opening or closing line of a body nested in b: its count is
// the inner function's, while code edited there may run as part of b. It
// reports false when b holds no instrumented line, so nothing proves which
// tests enter it.
func selectLines(lines map[int][]int32, bs []body, b body, lo, hi int, out map[int32]bool) bool {
	edge := map[int]bool{}
	for _, n := range bs {
		if b.open <= n.open && n.close <= b.close && n != b {
			edge[n.open], edge[n.close] = true, true
		}
	}
	whole := false
	for l := lo; l <= hi; l++ {
		ts, ok := lines[l]
		if !ok || edge[l] {
			whole = true
		}
		for _, t := range ts {
			out[t] = true
		}
	}
	if !whole {
		return true
	}
	any := false
	for l := b.open; l <= b.close; l++ {
		ts, ok := lines[l]
		any = any || ok
		for _, t := range ts {
			out[t] = true
		}
	}
	return any
}
