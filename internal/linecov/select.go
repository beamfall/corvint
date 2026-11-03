package linecov

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"path"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
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
	// Blocks holds every block whose tests Tests gained by line, the join's
	// input (MCJ-V0-004). Direct holds the tests whose own source changed.
	Blocks map[BlockRef]bool
	Direct map[int32]bool
}

// Analyze maps a diff to its effect. read returns a file's content on the
// diff's old side. Only an edit inside one function body of an instrumented
// file whose old blob the index recorded is mapped by line; anything else is
// a fallback or, for a test file, the whole package.
func (ix *Index) Analyze(diffs []FileDiff, read func(path string) ([]byte, error)) Effect {
	e := Effect{Tests: map[int32]bool{}, Whole: map[string]bool{}, Touched: map[string]bool{}, Blocks: map[BlockRef]bool{}, Direct: map[int32]bool{}}
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
		fileIdx := -1
		if !test {
			i, ok := ix.byPath[d.Path]
			if !ok {
				return fail("uninstrumented file %s", d.Path)
			}
			file, fileIdx = &ix.Files[i], i
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
		var code []span
		if !test {
			code = codeTokens(src)
		}
		for _, h := range d.Hunks {
			lo, hi := h.Start, h.End
			insert := hi < lo
			if insert { // either neighbouring line may run it
				hi = lo + 1
			}
			fn := enclosingFunc(fset, parsed, lo, hi, insert, test)
			switch {
			case fn == nil && test:
				e.Whole[pkg] = true
			case fn == nil:
				return fail("declaration-level edit %s:%d", d.Path, h.Start)
			case test:
				if name, ok := topLevelTest(fn); ok {
					if id, ok := ix.byName[pkg+"\x00"+name]; ok {
						e.Tests[id] = true
						e.Direct[id] = true
					}
					// An unrecorded test is kept by Narrow anyway.
				} else {
					e.Whole[pkg] = true // TestMain, init or a helper
				}
			default:
				ix.selectLines(int32(fileIdx), file, code, fset, fn, lo, hi, e)
			}
		}
	}
	for id := range e.Tests {
		if t := ix.Tests[id]; t.Name == InOrder {
			e.Whole[t.Package] = true
		}
	}
	return e
}

// span is one source token: where it starts and the line it ends on.
type span struct{ line, col, endLine int32 }

// codeTokens lists src's tokens that are code: not a comment, brace or
// semicolon, which no coverage block needs to hold.
func codeTokens(src []byte) []span {
	fset := token.NewFileSet()
	tf := fset.AddFile("", -1, len(src))
	var sc scanner.Scanner
	sc.Init(tf, src, nil, scanner.ScanComments)
	var out []span
	for {
		pos, tok, lit := sc.Scan()
		if tok == token.EOF {
			return out
		}
		switch tok {
		case token.COMMENT, token.LBRACE, token.RBRACE, token.SEMICOLON:
			continue
		}
		p := tf.Position(pos)
		out = append(out, span{int32(p.Line), int32(p.Column), int32(p.Line + strings.Count(lit, "\n"))})
	}
}

// selectLines adds the tests that executed a block touching [lo, hi]. A code
// token of the span inside no block (a case expression, an else, a
// declaration a block does not hold) adds every test that executed a block of
// the enclosing function, since running it requires entering the function.
// The check is by column: a block sharing a line with such a token does not
// cover it (LPCV-V0-952).
func (ix *Index) selectLines(fi int32, f *File, code []span, fset *token.FileSet, fn *ast.FuncDecl, lo, hi int, e Effect) {
	for bi, b := range f.Blocks {
		if int(b.Start) > hi {
			break
		}
		if int(b.End) < lo {
			continue
		}
		e.Blocks[BlockRef{fi, int32(bi)}] = true
		for _, t := range b.Tests {
			e.Tests[t] = true
		}
	}
	for _, c := range code {
		if int(c.endLine) < lo || int(c.line) > hi || f.covers(c.line, c.col) {
			continue
		}
		first, last := fset.Position(fn.Pos()).Line, fset.Position(fn.End()).Line
		for bi, b := range f.Blocks {
			if int(b.Start) >= first && int(b.End) <= last {
				e.Blocks[BlockRef{fi, int32(bi)}] = true
				for _, t := range b.Tests {
					e.Tests[t] = true
				}
			}
		}
		return
	}
}

// covers reports whether a block holds the byte at line, col.
func (f *File) covers(line, col int32) bool {
	for _, b := range f.Blocks {
		if b.Start > line {
			return false
		}
		after := line > b.Start || col >= b.StartCol
		before := line < b.End || line == b.End && col < b.EndCol
		if after && before {
			return true
		}
	}
	return false
}

// enclosingFunc returns the function whose body strictly contains [lo, hi]; a
// span touching a signature or brace, or outside every body, is nil. An
// insertion between lo and hi = lo+1 is inside a body whose braces lie on or
// outside those lines: it runs only once the function is entered. With
// whole, the span may cover the declaration's own lines: a test function's
// signature or one-line body changes only that test, and a renamed test is
// unrecorded at the head and kept by Narrow.
func enclosingFunc(fset *token.FileSet, f *ast.File, lo, hi int, insert, whole bool) *ast.FuncDecl {
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		open, close := fset.Position(fn.Body.Lbrace).Line, fset.Position(fn.Body.Rbrace).Line
		if lo > open && hi < close || insert && lo >= open && hi <= close {
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
		rest, ok := strings.CutPrefix(name, prefix)
		if !ok {
			continue
		}
		// As go test: TestX and Test_x run, Testhelper does not.
		r, _ := utf8.DecodeRuneInString(rest)
		return name, rest == "" || !unicode.IsLower(r)
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
	unordered := map[string]bool{}
	for _, pkg := range ix.Unordered {
		unordered[pkg] = true
	}
	for _, pkg := range static {
		// An unordered package's order-only blocks may have no isolated owner,
		// so no selected test reveals that a change reaches them (LPCV-V0-962).
		if reason != "" || drift.Whole[pkg] || change.Whole[pkg] || unordered[pkg] {
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
