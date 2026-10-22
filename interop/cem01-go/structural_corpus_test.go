package main

// The structural corpus (decision 0551, CEM-V1-050): one observation line per
// Go source file — scanner, parser, printer and formatter digests — plus
// derived perturbation pairs judged by all four portable structural
// predicates in both directions. A pair is recorded as segments of its base
// file so the native consumer can rebuild the same image and must reach the
// same verdicts (internal/cem/verify/structural_corpus_test.go).

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"
	"unicode"
)

// corpusReasons fixes the verdict-bit order: each reason forward (base to
// image), then reverse (image to base).
var corpusReasons = []string{"formatter-only", "import-reorder", "move", "rename"}

type corpusSegment struct {
	Start int     `json:"s"`
	End   int     `json:"e"`
	Text  *string `json:"t,omitempty"`
}

type corpusPair struct {
	Kind     string          `json:"kind"`
	Segments []corpusSegment `json:"segments"`
	Sha256   string          `json:"sha256"`
	Verdicts string          `json:"verdicts"`
}

type corpusObservation struct {
	Path             string       `json:"path"`
	Sha256           string       `json:"sha256"`
	Scan             string       `json:"scan"`
	Parse            string       `json:"parse"`
	Print            string       `json:"print"`
	Format           string       `json:"format"`
	FormatIdempotent bool         `json:"formatIdempotent"`
	Pairs            []corpusPair `json:"pairs"`
}

// TestStructuralCorpus observes every regular file under
// CEM_STRUCTURAL_CORPUS and writes one JSON line per file, in path order, to
// CEM_STRUCTURAL_CORPUS_OUT. It runs only when both are set.
func TestStructuralCorpus(t *testing.T) {
	dir, out := os.Getenv("CEM_STRUCTURAL_CORPUS"), os.Getenv("CEM_STRUCTURAL_CORPUS_OUT")
	if dir == "" || out == "" {
		t.Skip("CEM_STRUCTURAL_CORPUS and CEM_STRUCTURAL_CORPUS_OUT name the corpus extract and the observation file")
	}
	lines, err := corpusObserve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) == 0 {
		t.Fatalf("%s holds no regular file; an empty corpus proves nothing", dir)
	}
	if err := os.WriteFile(out, bytes.Join(lines, nil), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(bytes.Join(lines, nil))
	t.Logf("%s %s/%s: %d files, observations sha256 %s", runtime.Version(), runtime.GOOS, runtime.GOARCH, len(lines), hex.EncodeToString(sum[:]))
}

// emptyCases are the zero-byte cases. CEM 1.0 cannot bind an empty-file
// creation (CEM-V1-300), so they are declared here instead of committed and
// observed as zero bytes at their sorted path.
var emptyCases = []string{"empty-file.go.txt"}

// TestStructuralCorpusCases reobserves the committed version-sensitive cases
// and requires the qualified digest whenever the running toolchain is an
// admitted structural tuple: an admitted tuple shares the qualified parser
// closure, so it must observe exactly what the qualification observed.
func TestStructuralCorpusCases(t *testing.T) {
	root := filepath.Join("..", "..", "protocol", "cem-1.0", "structural-corpus")
	raw, err := os.ReadFile(filepath.Join(root, "QUALIFICATION.json"))
	if err != nil {
		t.Fatal(err)
	}
	var q struct {
		AdmittedTuples []string `json:"admittedTuples"`
		Cases          struct {
			ObservationsSha256 string `json:"observationsSha256"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &q); err != nil {
		t.Fatal(err)
	}
	if !sameTuples(q.AdmittedTuples) {
		t.Fatalf("structuralTuples %v differ from QUALIFICATION.json admittedTuples %v", structuralTuples, q.AdmittedTuples)
	}
	lines, err := corpusObserve(filepath.Join(root, "cases"), emptyCases...)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(bytes.Join(lines, nil))
	got := hex.EncodeToString(sum[:])
	if !structuralTuples[runtime.Version()] {
		t.Skipf("%s is not an admitted structural tuple; cases observe %s", runtime.Version(), got)
	}
	if got != q.Cases.ObservationsSha256 {
		t.Fatalf("%s observes the cases as %s, qualified %s", runtime.Version(), got, q.Cases.ObservationsSha256)
	}
}

// sameTuples reports whether admitted lists exactly the structuralTuples.
func sameTuples(admitted []string) bool {
	seen := map[string]bool{}
	for _, tuple := range admitted {
		if !structuralTuples[tuple] || seen[tuple] {
			return false
		}
		seen[tuple] = true
	}
	return len(seen) == len(structuralTuples)
}

func corpusObserve(dir string, empty ...string) ([][]byte, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			rel, err := filepath.Rel(dir, p)
			if err != nil {
				return err
			}
			paths = append(paths, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	paths = append(paths, empty...)
	sort.Strings(paths)
	lines := make([][]byte, 0, len(paths))
	for _, rel := range paths {
		src := []byte{}
		if !slices.Contains(empty, rel) {
			if src, err = readCorpusFile(dir, rel); err != nil {
				return nil, err
			}
		}
		line, err := json.Marshal(observeSource(rel, src))
		if err != nil {
			return nil, err
		}
		lines = append(lines, append(line, '\n'))
	}
	return lines, nil
}

// readCorpusFile reads one corpus file. A "*.crlf.go.txt" case is committed
// with LF endings, as the repository's end-of-line policy requires, and is
// observed with every LF widened to CRLF.
func readCorpusFile(dir, rel string) ([]byte, error) {
	src, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err == nil && strings.HasSuffix(rel, ".crlf.go.txt") {
		src = bytes.ReplaceAll(src, []byte("\n"), []byte("\r\n"))
	}
	return src, err
}

func corpusDigest(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// astFilter drops the deprecated object resolution: Scope is a map whose dump
// order is random, and no structural predicate reads Obj or Unresolved.
func astFilter(name string, v reflect.Value) bool {
	return name != "Scope" && name != "Obj" && name != "Unresolved" && ast.NotNilFilter(name, v)
}

func observeSource(rel string, src []byte) corpusObservation {
	o := corpusObservation{Path: rel, Sha256: corpusDigest(src), Pairs: []corpusPair{}}

	var scanned bytes.Buffer
	fset := token.NewFileSet()
	file := fset.AddFile(rel, -1, len(src))
	var s scanner.Scanner
	s.Init(file, src, func(pos token.Position, msg string) { fmt.Fprintf(&scanned, "error %d %s\x00", pos.Offset, msg) }, scanner.ScanComments)
	for {
		pos, kind, lit := s.Scan()
		if kind == token.EOF {
			break
		}
		fmt.Fprintf(&scanned, "%d %s %q\x00", file.Offset(pos), kind, lit)
	}
	o.Scan = corpusDigest(scanned.Bytes())

	pset := token.NewFileSet()
	parsed, parseErr := parser.ParseFile(pset, rel, src, parser.ParseComments|parser.AllErrors)
	if parseErr != nil {
		o.Parse = "error:" + corpusDigest([]byte(parseErr.Error()))
	} else {
		var dump bytes.Buffer
		if err := ast.Fprint(&dump, pset, parsed, astFilter); err != nil {
			o.Parse = "dump-error:" + corpusDigest([]byte(err.Error()))
		} else {
			o.Parse = corpusDigest(dump.Bytes())
		}
		var printed bytes.Buffer
		cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}
		if err := cfg.Fprint(&printed, pset, parsed); err != nil {
			o.Print = "error:" + corpusDigest([]byte(err.Error()))
		} else {
			o.Print = corpusDigest(printed.Bytes())
		}
	}

	formatted, err := format.Source(src)
	if err != nil {
		o.Format = "error:" + corpusDigest([]byte(err.Error()))
	} else {
		o.Format = corpusDigest(formatted)
		again, err := format.Source(formatted)
		o.FormatIdempotent = err == nil && bytes.Equal(again, formatted)
	}

	if parseErr == nil {
		for _, pair := range corpusPairs(src, pset, parsed) {
			image := buildImage(src, pair.Segments)
			pair.Sha256 = corpusDigest(image)
			var v strings.Builder
			for _, reason := range corpusReasons {
				v.WriteByte(corpusBit(structuralPredicate03(reason, src, image)))
				v.WriteByte(corpusBit(structuralPredicate03(reason, image, src)))
			}
			pair.Verdicts = v.String()
			o.Pairs = append(o.Pairs, pair)
		}
	}
	return o
}

func corpusBit(b bool) byte {
	if b {
		return '1'
	}
	return '0'
}

func buildImage(src []byte, segments []corpusSegment) []byte {
	var b bytes.Buffer
	for _, s := range segments {
		if s.Text != nil {
			b.WriteString(*s.Text)
		} else {
			b.Write(src[s.Start:s.End])
		}
	}
	return b.Bytes()
}

func corpusSpan(a, b int) corpusSegment { return corpusSegment{Start: a, End: b} }

func corpusLiteral(text string) corpusSegment { return corpusSegment{Text: &text} }

// swapRanges exchanges two ordered, disjoint byte ranges of src.
func swapRanges(n, a0, a1, b0, b1 int) []corpusSegment {
	return []corpusSegment{corpusSpan(0, a0), corpusSpan(b0, b1), corpusSpan(a1, b0), corpusSpan(a0, a1), corpusSpan(b1, n)}
}

// corpusPairs derives the perturbations of one parsed file. Each rule is
// syntactic and picks its target without consulting any predicate:
//
//   - formatter-only: two blank lines after the package clause line;
//   - import-reorder: the first two specs of the first parenthesized import
//     declaration swapped, when neither carries a comment;
//   - move: the first two adjacent top-level functions swapped, doc comments
//     included;
//   - rename: the first unexported top-level function without a receiver
//     renamed to a fresh name in every identifier token and every
//     non-directive comment;
//   - partial-rename (a near-miss): only that function's declaring name
//     renamed, when the name is used again.
func corpusPairs(src []byte, fset *token.FileSet, f *ast.File) []corpusPair {
	n := len(src)
	off := func(p token.Pos) int { return fset.Position(p).Offset }
	var pairs []corpusPair

	if nl := bytes.IndexByte(src[off(f.Name.End()):], '\n'); nl >= 0 {
		k := off(f.Name.End()) + nl + 1
		pairs = append(pairs, corpusPair{Kind: "formatter-only", Segments: []corpusSegment{corpusSpan(0, k), corpusLiteral("\n\n"), corpusSpan(k, n)}})
	}

	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.IMPORT {
			continue
		}
		if g.Lparen.IsValid() && len(g.Specs) >= 2 {
			a, b := g.Specs[0].(*ast.ImportSpec), g.Specs[1].(*ast.ImportSpec)
			if a.Doc == nil && a.Comment == nil && b.Doc == nil && b.Comment == nil {
				pairs = append(pairs, corpusPair{Kind: "import-reorder", Segments: swapRanges(n, off(a.Pos()), off(a.End()), off(b.Pos()), off(b.End()))})
			}
		}
		break
	}

	start := func(fn *ast.FuncDecl) int {
		if fn.Doc != nil {
			return off(fn.Doc.Pos())
		}
		return off(fn.Pos())
	}
	for i := 0; i+1 < len(f.Decls); i++ {
		a, okA := f.Decls[i].(*ast.FuncDecl)
		b, okB := f.Decls[i+1].(*ast.FuncDecl)
		if okA && okB {
			pairs = append(pairs, corpusPair{Kind: "move", Segments: swapRanges(n, start(a), off(a.End()), start(b), off(b.End()))})
			break
		}
	}

	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Name.Name == "init" || fn.Name.Name == "main" || fn.Name.Name == "_" || ast.IsExported(fn.Name.Name) {
			continue
		}
		from := fn.Name.Name
		to := "cemCorpusRenamed"
		for i := 0; wordIn(src, to); i++ {
			to = fmt.Sprintf("cemCorpusRenamed%d", i)
		}
		full, partial := renameSegments(src, from, to, off(fn.Name.Pos()))
		if full != nil {
			pairs = append(pairs, corpusPair{Kind: "rename", Segments: full})
		}
		if partial != nil {
			pairs = append(pairs, corpusPair{Kind: "partial-rename", Segments: partial})
		}
		break
	}
	return pairs
}

func wordIn(src []byte, word string) bool {
	for i := bytes.Index(src, []byte(word)); i >= 0; {
		end := i + len(word)
		before := i == 0 || !identRune(rune(src[i-1]))
		after := end == len(src) || !identRune(rune(src[end]))
		if before && after {
			return true
		}
		next := bytes.Index(src[i+1:], []byte(word))
		if next < 0 {
			return false
		}
		i += 1 + next
	}
	return false
}

func identRune(r rune) bool {
	return r == '_' || r >= 0x80 || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func corpusDirective(comment string) bool {
	return strings.HasPrefix(comment, "//go:") || strings.HasPrefix(comment, "//line ") || strings.HasPrefix(comment, "//export ")
}

// renameSegments returns the full rename of from and, when from occurs more
// than once, the rename of the declaring occurrence at decl alone.
func renameSegments(src []byte, from, to string, decl int) (full, partial []corpusSegment) {
	fset := token.NewFileSet()
	file := fset.AddFile("", -1, len(src))
	var s scanner.Scanner
	bad := false
	s.Init(file, src, func(token.Position, string) { bad = true }, scanner.ScanComments)
	cursor, uses := 0, 0
	for {
		pos, kind, lit := s.Scan()
		if kind == token.EOF {
			break
		}
		at := file.Offset(pos)
		switch {
		case kind == token.IDENT && lit == from:
			full = append(full, corpusSpan(cursor, at), corpusLiteral(to))
			cursor = at + len(lit)
			uses++
		case kind == token.COMMENT && !corpusDirective(lit):
			if replaced := replaceWordCorpus(lit, from, to); replaced != lit {
				full = append(full, corpusSpan(cursor, at), corpusLiteral(replaced))
				cursor = at + len(lit)
			}
		}
	}
	if bad || uses == 0 {
		return nil, nil
	}
	full = append(full, corpusSpan(cursor, len(src)))
	if uses > 1 {
		partial = []corpusSegment{corpusSpan(0, decl), corpusLiteral(to), corpusSpan(decl+len(from), len(src))}
	}
	return full, partial
}

func replaceWordCorpus(text, from, to string) string {
	var out, word strings.Builder
	flush := func() {
		if word.String() == from {
			out.WriteString(to)
		} else {
			out.WriteString(word.String())
		}
		word.Reset()
	}
	for _, r := range text {
		if r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			word.WriteRune(r)
			continue
		}
		flush()
		out.WriteRune(r)
	}
	flush()
	return out.String()
}
