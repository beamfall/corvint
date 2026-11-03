package linecov

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"sort"
	"strings"
)

// Hunk is the base-side line span one diff hunk replaces. A pure insertion
// after line n has Start n and End n-1.
type Hunk struct{ Start, End int }

// FileDiff is one changed path. OldOID is empty for an added file and NewOID
// for a deleted one.
type FileDiff struct {
	Path           string
	OldOID, NewOID string
	Hunks          []Hunk
}

// Effect is what one diff does to the index: the tests it reaches by line,
// the packages it reaches whole, the packages it changes, or a fallback that
// keeps the static plan (LPCV-V0-952, LPCV-V0-953).
type Effect struct {
	Fallback string
	Tests    map[int32]bool
	Whole    map[string]bool // package directories every test of which runs
	Touched  map[string]bool // package directories with a changed Go file
}

// Analyze maps a diff to its effect. read returns a file's content on the
// diff's old side. Only an edit inside one function body of an instrumented
// file whose old blob the index recorded is mapped by line; anything else is
// a fallback or, for a test file, the whole package.
func (ix *Index) Analyze(diffs []FileDiff, read func(path string) ([]byte, error)) Effect {
	e := Effect{Tests: map[int32]bool{}, Whole: map[string]bool{}, Touched: map[string]bool{}}
	fail := func(format string, args ...any) Effect {
		return Effect{Fallback: fmt.Sprintf(format, args...)}
	}
	for _, d := range diffs {
		if !strings.HasSuffix(d.Path, ".go") {
			return fail("non-Go input %s", d.Path)
		}
		pkg := path.Dir(d.Path)
		e.Touched[pkg] = true
		test := strings.HasSuffix(d.Path, "_test.go")
		if d.OldOID == "" || d.NewOID == "" {
			if test {
				e.Whole[pkg] = true
				continue
			}
			return fail("added or deleted file %s", d.Path)
		}
		var file *File
		if !test {
			i, ok := ix.byPath[d.Path]
			if !ok {
				return fail("uninstrumented file %s", d.Path)
			}
			file = &ix.Files[i]
			if file.OID != d.OldOID {
				return fail("index records %s at another blob", d.Path)
			}
		}
		src, err := read(d.Path)
		if err != nil {
			return fail("unreadable %s: %v", d.Path, err)
		}
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, d.Path, src, parser.SkipObjectResolution)
		if err != nil {
			return fail("unparsable %s", d.Path)
		}
		for _, h := range d.Hunks {
			lo, hi := h.Start, h.End
			if hi < lo { // insertion: either neighbouring line may run it
				hi = lo + 1
			}
			fn := enclosingFunc(fset, parsed, lo, hi, test)
			switch {
			case fn == nil && test:
				e.Whole[pkg] = true
			case fn == nil:
				return fail("declaration-level edit %s:%d", d.Path, h.Start)
			case test:
				if name, ok := topLevelTest(fn); ok {
					if id, ok := ix.byName[pkg+"\x00"+name]; ok {
						e.Tests[id] = true
					}
					// An unrecorded test is kept by Narrow anyway.
				} else {
					e.Whole[pkg] = true // TestMain, init or a helper
				}
			default:
				ix.selectLines(file, fset, fn, lo, hi, e.Tests)
			}
		}
	}
	return e
}

// selectLines adds the tests that executed a block touching [lo, hi]. A line
// of the span inside no block (a case expression, an else-if condition, a
// comment) adds every test that executed a block of the enclosing function,
// since running that line requires entering the function.
func (ix *Index) selectLines(f *File, fset *token.FileSet, fn *ast.FuncDecl, lo, hi int, out map[int32]bool) {
	covered := make([]bool, hi-lo+1)
	for _, b := range f.Blocks {
		if int(b.Start) > hi {
			break
		}
		if int(b.End) < lo {
			continue
		}
		for _, t := range b.Tests {
			out[t] = true
		}
		for l := max(lo, int(b.Start)); l <= min(hi, int(b.End)); l++ {
			covered[l-lo] = true
		}
	}
	for _, c := range covered {
		if c {
			continue
		}
		first, last := fset.Position(fn.Pos()).Line, fset.Position(fn.End()).Line
		for _, b := range f.Blocks {
			if int(b.Start) >= first && int(b.End) <= last {
				for _, t := range b.Tests {
					out[t] = true
				}
			}
		}
		return
	}
}

// enclosingFunc returns the function whose body strictly contains [lo, hi]; a
// span touching a signature or brace, or outside every body, is nil. With
// whole, the span may cover the declaration's own lines: a test function's
// signature or one-line body changes only that test, and a renamed test is
// unrecorded at the head and kept by Narrow.
func enclosingFunc(fset *token.FileSet, f *ast.File, lo, hi int, whole bool) *ast.FuncDecl {
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if lo > fset.Position(fn.Body.Lbrace).Line && hi < fset.Position(fn.Body.Rbrace).Line {
			return fn
		}
		if whole && lo >= fset.Position(fn.Pos()).Line && hi <= fset.Position(fn.End()).Line {
			return fn
		}
	}
	return nil
}

// topLevelTest names fn when `go test` runs it as a top-level test.
func topLevelTest(fn *ast.FuncDecl) (string, bool) {
	name := fn.Name.Name
	if fn.Recv != nil || name == "TestMain" {
		return "", false
	}
	for _, prefix := range []string{"Test", "Example", "Fuzz"} {
		if strings.HasPrefix(name, prefix) {
			return name, true
		}
	}
	return "", false
}

// Mode is how one statically selected package runs.
type Mode string

const (
	Whole   Mode = "whole"   // every test
	Tests   Mode = "tests"   // only Plan.Tests
	Compile Mode = "compile" // build and vet, no test: a changed package no test reaches
	Exclude Mode = "exclude" // nothing: no kept test, no changed file
)

// PackagePlan is one package's decision with the reason for each kept test.
type PackagePlan struct {
	Mode  Mode
	Tests []string          // Mode Tests: sorted top-level names
	Why   map[string]string // kept test -> selected|stale|unrecorded
}

// Narrow decides each statically selected package directory. headTests lists
// the top-level tests each package declares at the change head. drift is the
// effect of the history between the index commit and the change base, change
// the effect of the change itself. A fallback in either keeps every package
// whole (LPCV-V0-951, LPCV-V0-953).
func (ix *Index) Narrow(static []string, headTests map[string][]string, drift, change Effect) (map[string]PackagePlan, string) {
	plans := map[string]PackagePlan{}
	reason := drift.Fallback
	if reason != "" {
		reason = "index drift: " + reason
	} else {
		reason = change.Fallback
	}
	for _, pkg := range static {
		if reason != "" || drift.Whole[pkg] || change.Whole[pkg] {
			plans[pkg] = PackagePlan{Mode: Whole}
			continue
		}
		names := headTests[pkg]
		p := PackagePlan{Why: map[string]string{}}
		for _, name := range names {
			id, ok := ix.byName[pkg+"\x00"+name]
			switch {
			case !ok:
				p.Why[name] = "unrecorded"
			case change.Tests[id]:
				p.Why[name] = "selected"
			case drift.Tests[id]:
				p.Why[name] = "stale"
			default:
				continue
			}
			p.Tests = append(p.Tests, name)
		}
		sort.Strings(p.Tests)
		switch {
		case len(p.Tests) > 0 && len(p.Tests) == len(names):
			p.Mode = Whole
		case len(p.Tests) > 0:
			p.Mode = Tests
		case change.Touched[pkg]:
			p.Mode = Compile
		default:
			p.Mode = Exclude
		}
		plans[pkg] = p
	}
	return plans, reason
}
