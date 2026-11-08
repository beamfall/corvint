package linecov

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
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
// for a deleted one. NewHunks and NewSource carry the immutable new side.
type FileDiff struct {
	Path           string
	OldOID, NewOID string
	Hunks          []Hunk
	NewHunks       []Hunk
	NewSource      []byte // immutable new-side blob; nil forbids narrowing
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

// The path classes of GLS-V0-012. ClassBody and ClassTestFunction are
// mapped by line; every other class is a static-fallback category of
// LPCV-V0-953.
const (
	ClassBody           = "body"
	ClassTestFunction   = "test-function"
	ClassNonGo          = "non-go"
	ClassAdded          = "added"
	ClassDeleted        = "deleted"
	ClassDeclaration    = "declaration"
	ClassUninstrumented = "uninstrumented"
	ClassBlobMismatch   = "blob-mismatch"
	ClassTestHelper     = "test-helper"
	ClassUnparsable     = "unparsable"
	ClassUnreadable     = "unreadable"
	ClassSectionInvalid = "index-section-invalid"
)

// PathResult is one changed path's class. Hunks are its base-side ranges
// when the class is mapped by line; Detail says why a fallback class applies.
type PathResult struct {
	Path   string
	Class  string
	Detail string
	Hunks  []Hunk
}

// Mapped reports whether the path was mapped by line.
func (r PathResult) Mapped() bool { return r.Class == ClassBody || r.Class == ClassTestFunction }

// AnalyzePaths classifies each diff (GLS-V0-012) and adds, for each body
// and test-function path, the tests it reaches to the effect. read returns a
// file's content on the diff's old side. A fallback path adds nothing: its
// static reach decides. Touched holds every package directory with a
// changed Go file, whatever its class.
func (ix *Index) AnalyzePaths(diffs []FileDiff, read func(path string) ([]byte, error)) ([]PathResult, Effect) {
	e := Effect{Tests: map[int32]bool{}, Whole: map[string]bool{}, Touched: map[string]bool{}, Blocks: map[BlockRef]bool{}, Direct: map[int32]bool{}}
	results := make([]PathResult, 0, len(diffs))
	for _, d := range diffs {
		r := ix.analyzePath(d, read, &e)
		results = append(results, r)
	}
	return results, e
}

func (ix *Index) analyzePath(d FileDiff, read func(path string) ([]byte, error), e *Effect) PathResult {
	res := PathResult{Path: d.Path}
	fail := func(class, format string, args ...any) PathResult {
		res.Class, res.Detail = class, fmt.Sprintf(format, args...)
		return res
	}
	if !strings.HasSuffix(d.Path, ".go") {
		return fail(ClassNonGo, "non-Go input %s", d.Path)
	}
	pkg := path.Dir(d.Path)
	e.Touched[pkg] = true
	test := strings.HasSuffix(d.Path, "_test.go")
	switch {
	case d.OldOID == "":
		return fail(ClassAdded, "added file %s", d.Path)
	case d.NewOID == "":
		return fail(ClassDeleted, "deleted file %s", d.Path)
	}
	var file *File
	fileIdx := -1
	if !test {
		i, ok := ix.byPath[d.Path]
		if !ok {
			return fail(ClassUninstrumented, "uninstrumented file %s", d.Path)
		}
		if ix.Files[i].OID != d.OldOID {
			return fail(ClassBlobMismatch, "index records %s at another blob", d.Path)
		}
		if err := ix.ensureFile(i); err != nil {
			return fail(ClassSectionInvalid, "%v", err)
		}
		file, fileIdx = &ix.Files[i], i
	}
	src, err := read(d.Path)
	if err != nil {
		return fail(ClassUnreadable, "unreadable %s: %v", d.Path, err)
	}
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, d.Path, src, parser.SkipObjectResolution)
	if err != nil {
		return fail(ClassUnparsable, "unparsable %s", d.Path)
	}
	if d.NewSource == nil || len(d.NewHunks) != len(d.Hunks) || len(d.Hunks) == 0 {
		return fail(ClassUnreadable, "missing new-side evidence %s", d.Path)
	}
	newSet := token.NewFileSet()
	newFile, err := parser.ParseFile(newSet, d.Path, d.NewSource, parser.SkipObjectResolution)
	if err != nil {
		return fail(ClassUnparsable, "unparsable new side %s", d.Path)
	}
	// A body-only edit preserves every declaration and signature. Strip
	// bodies before printing, keeping boundary changes out of line mapping.
	if !sameDeclarations(parsed, newFile, test) {
		class := ClassDeclaration
		if test {
			class = ClassTestHelper
		}
		return fail(class, "declarations or function boundaries changed %s", d.Path)
	}
	var code []span
	if !test {
		code = codeTokens(src)
	}
	// The path's tests join the effect only once every hunk is mapped.
	local := Effect{Tests: map[int32]bool{}, Blocks: map[BlockRef]bool{}, Direct: map[int32]bool{}}
	for i, h := range d.Hunks {
		lo, hi := h.Start, h.End
		insert := hi < lo
		if insert { // either neighbouring line may run it
			hi = lo + 1
		}
		fn := enclosingFunc(fset, parsed, lo, hi, insert, test)
		nh := d.NewHunks[i]
		nlo, nhi := nh.Start, nh.End
		ninsert := nhi < nlo
		if ninsert {
			nhi = nlo + 1
		}
		nfn := enclosingFunc(newSet, newFile, nlo, nhi, ninsert, test)
		if fn == nil || nfn == nil || funcOrdinal(parsed, fn) != funcOrdinal(newFile, nfn) {
			class := ClassDeclaration
			if test {
				class = ClassTestHelper
			}
			return fail(class, "edit crosses function boundaries %s:%d", d.Path, h.Start)
		}
		switch {
		case test:
			name, ok := topLevelTest(fn)
			if !ok {
				return fail(ClassTestHelper, "edit of %s, not a top-level test, %s:%d", fn.Name.Name, d.Path, h.Start)
			}
			if id, ok := ix.byName[pkg+"\x00"+name]; ok {
				local.Tests[id] = true
				local.Direct[id] = true
			}
			// The head name can already have a record (for example two tests
			// swapping names). Keep both sides of the edited test function.
			if headName, ok := topLevelTest(nfn); ok {
				if id, ok := ix.byName[pkg+"\x00"+headName]; ok {
					local.Tests[id], local.Direct[id] = true, true
				}
			} else {
				return fail(ClassTestHelper, "new side is not a top-level test %s:%d", d.Path, nh.Start)
			}
			// The edited test also runs in its package's in-order suite, where
			// state it leaves can change what a later test reaches; that run
			// attributes nothing to one test, so the edit reaches the
			// in-order record and keeps the package (GLS-V0-008).
			if id, ok := ix.byName[pkg+"\x00"+InOrder]; ok {
				local.Tests[id] = true
			}
			// An unrecorded test is kept by Narrow anyway.
		default:
			ix.selectLines(int32(fileIdx), file, code, fset, fn, lo, hi, local)
		}
	}
	for id := range local.Tests {
		e.Tests[id] = true
	}
	for id := range local.Direct {
		e.Direct[id] = true
	}
	for b := range local.Blocks {
		e.Blocks[b] = true
	}
	res.Class, res.Hunks = ClassBody, d.Hunks
	if test {
		res.Class = ClassTestFunction
	}
	return res
}

// Analyze maps a diff to its effect. read returns a file's content on the
// diff's old side; FileDiff carries the new side. Only an edit inside the
// same function body on both sides of an instrumented
// file whose old blob the index recorded is mapped by line; a test file's
// helper edit, addition or deletion keeps its whole package; anything else
// is a fallback that keeps the static plan (LPCV-V0-952, LPCV-V0-953).
func (ix *Index) Analyze(diffs []FileDiff, read func(path string) ([]byte, error)) Effect {
	results, e := ix.AnalyzePaths(diffs, read)
	for _, r := range results {
		test := strings.HasSuffix(r.Path, "_test.go")
		switch {
		case r.Mapped():
		case test && (r.Class == ClassAdded || r.Class == ClassDeleted || r.Class == ClassTestHelper):
			e.Whole[path.Dir(r.Path)] = true
		default:
			return Effect{Fallback: r.Detail}
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
	// Fast negative lookup: an unexecuted file still compiles when touched.
	if len(ix.TestsOfFile(f.Path)) == 0 {
		return
	}
	hits := map[[4]int32]bool{}
	for _, b := range ix.BlocksAtLine(f.Path, int32(lo)) {
		hits[[4]int32{b.Start, b.StartCol, b.End, b.EndCol}] = true
	}
	for _, b := range f.Blocks {
		if int(b.Start) > lo && int(b.Start) <= hi {
			hits[[4]int32{b.Start, b.StartCol, b.End, b.EndCol}] = true
		}
	}
	for bi, b := range f.Blocks {
		if int(b.Start) > hi {
			break
		}
		if !hits[[4]int32{b.Start, b.StartCol, b.End, b.EndCol}] {
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

// Narrow decides each statically selected package directory. environ is the
// environment the tests will run under: a test, or a package's in-order
// record, observed under bindings that environment does not hold is kept
// (GLS-V0-018), and an index that binds no environment keeps every package
// whole. A nil environ compares nothing, for a language shadow whose plan is
// a candidate only. headTests lists the top-level tests each package declares
// at the change head. drift is the effect of the history between the index
// commit and the change base, change the effect of the change itself. A
// fallback in either keeps every package whole (LPCV-V0-951, LPCV-V0-953).
func (ix *Index) Narrow(environ []string, static []string, headTests map[string][]string, drift, change Effect) (map[string]PackagePlan, string) {
	plans := map[string]PackagePlan{}
	reason := drift.Fallback
	if reason != "" {
		reason = "index drift: " + reason
	} else {
		reason = change.Fallback
	}
	if environ != nil && !ix.EnvBound && reason == "" {
		reason = ReasonEnvironmentUnbound + ": the index records no environment bindings"
	}
	values := EnvValues(environ)
	unbound := func(id int32) bool { return environ != nil && EnvUnbound(ix.envBindings(id), values) != "" }
	unordered := ix.unorderedSet()
	for _, pkg := range static {
		// An unordered package's order-only blocks may have no isolated owner,
		// so no selected test reveals that a change reaches them (LPCV-V0-962).
		if reason != "" || drift.Whole[pkg] || change.Whole[pkg] || unordered[pkg] {
			plans[pkg] = PackagePlan{Mode: Whole}
			continue
		}
		if id, ok := ix.byName[pkg+"\x00"+InOrder]; ok && unbound(id) {
			plans[pkg] = PackagePlan{Mode: Whole}
			continue
		}
		names := headTests[pkg]
		// A test binary without a named test runs only its TestMain, init
		// and package variables, which its in-order record alone shows: kept
		// whole without that record or when the change reaches it (GLS-V0-008).
		if ix.suiteUnrecorded(pkg, names) {
			plans[pkg] = PackagePlan{Mode: Whole}
			continue
		}
		if id, ok := ix.byName[pkg+"\x00"+InOrder]; ok && names != nil && len(names) == 0 && (change.Tests[id] || drift.Tests[id]) {
			plans[pkg] = PackagePlan{Mode: Whole}
			continue
		}
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
			case unbound(id):
				p.Why[name] = WhyEnvironment
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

// sameDeclarations compares syntax outside ordinary bodies. Test declarations
// may change signature/name because both versions still have to be tests.
func sameDeclarations(a, b *ast.File, tests bool) bool {
	outline := func(f *ast.File) string {
		clone := *f
		clone.Decls = append([]ast.Decl(nil), f.Decls...)
		for i, d := range clone.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok {
				c := *fn
				c.Body = nil
				if tests {
					if _, ok := topLevelTest(fn); ok {
						c.Name = ast.NewIdent("TestChanged")
						c.Type = &ast.FuncType{Params: &ast.FieldList{}}
					}
				}
				clone.Decls[i] = &c
			}
		}
		var buf bytes.Buffer
		if err := format.Node(&buf, token.NewFileSet(), &clone); err != nil {
			return ""
		}
		return buf.String()
	}
	x, y := outline(a), outline(b)
	return x != "" && x == y
}
func funcOrdinal(f *ast.File, fn *ast.FuncDecl) int {
	for i, d := range f.Decls {
		if d == fn {
			return i
		}
	}
	return -1
}
