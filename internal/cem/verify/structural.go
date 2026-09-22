package verify

import (
	"bytes"
	"errors"
	"go/ast"
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/sim"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// mechanicalProofs indexes the parsed patch by hunk ID so each mapped
// mechanical claim is proved against its byte reason or its structural class.
type mechanicalProofs struct {
	hunks  map[string]*patch.Hunk
	groups map[string]*patch.Group
	source sim.BlobSource
}

func indexProofs(parsed *patch.Patch, source sim.BlobSource) mechanicalProofs {
	proofs := mechanicalProofs{hunks: map[string]*patch.Hunk{}, groups: map[string]*patch.Group{}, source: source}
	for _, group := range parsed.Groups {
		for _, hunk := range group.Hunks {
			proofs.hunks[hunk.ID] = hunk
			proofs.groups[hunk.ID] = group
		}
	}
	return proofs
}

func (p mechanicalProofs) proven(id, reason string) bool {
	if wire.StructuralReasons[reason] {
		return structuralProven(p.groups[id], p.hunks[id], reason, p.source)
	}
	return mechanicalProven(p.hunks[id], reason)
}

// structuralPredicates dispatch each cem/0.3 structural reason to the Go
// comparison that proves it (CEM-SM-001). Every predicate sees the base file
// and a derived image that differ, and answers false for anything it cannot
// parse, scan, or format (CEM-SM-005).
var structuralPredicates = map[string]func(old, new []byte) bool{
	"rename":         renameProven,
	"move":           moveProven,
	"import-reorder": importReorderProven,
	"formatter-only": formatterOnlyProven,
}

// structuralProven proves one structural reason from the base blob and the
// patch alone (CEM-SM-002). Creates and deletes have no Go file on one side
// and are refused.
func structuralProven(group *patch.Group, hunk *patch.Hunk, reason string, source sim.BlobSource) bool {
	if group == nil {
		return false
	}
	if group.Kind == patch.KindCreate || group.Kind == patch.KindDelete {
		return false
	}
	old, _, exists, err := source.BaseBlob(*group.OldPath)
	if err != nil || !exists {
		return false
	}
	image, err := structuralImage(group, hunk, reason, old)
	if err != nil || bytes.Equal(old, image) {
		return false
	}
	return structuralPredicates[reason](old, image)
}

// structuralImage derives the post-image a reason is judged against: the
// whole file group for move (CEM-SM-004), because a move is a paired removal
// and insertion, and this hunk alone for every other reason (CEM-SM-003).
func structuralImage(group *patch.Group, hunk *patch.Hunk, reason string, old []byte) ([]byte, error) {
	if reason == "move" {
		return sim.ApplyHunks(group, old)
	}
	return spliceHunk(old, hunk)
}

// spliceHunk applies one hunk to the base bytes in isolation. Simulation has
// already proved the hunk's old side against these bytes, so only the range
// bounds are re-checked here.
func spliceHunk(old []byte, hunk *patch.Hunk) ([]byte, error) {
	lines, err := patch.SplitLF(old)
	if err != nil {
		return nil, err
	}
	first := hunk.OldRange.Start - 1
	if hunk.OldRange.Count == 0 {
		first = hunk.OldRange.Start
	}
	last := first + hunk.OldRange.Count
	if first < 0 || last > int64(len(lines)) {
		return nil, errors.New("hunk old range exceeds the base file")
	}
	var out bytes.Buffer
	for _, line := range lines[:first] {
		out.Write(line)
	}
	for _, line := range hunk.Body {
		if line.Prefix != '-' {
			out.Write(line.NewPayload())
		}
	}
	for _, line := range lines[last:] {
		out.Write(line)
	}
	return out.Bytes(), nil
}

// formatterOnlyProven: both sides canonicalise to the same gofmt output, so
// the difference lies entirely in what gofmt controls (CEM-SM-010).
func formatterOnlyProven(old, new []byte) bool {
	oldCanonical, err := format.Source(old)
	if err != nil {
		return false
	}
	newCanonical, err := format.Source(new)
	if err != nil {
		return false
	}
	return bytes.Equal(oldCanonical, newCanonical)
}

// goToken is one scanned token with its byte offset in the file.
type goToken struct {
	offset int
	kind   token.Token
	lit    string
}

// goFile is one side of a structural comparison: the parsed file and its
// complete token stream, comments included.
type goFile struct {
	fset   *token.FileSet
	file   *ast.File
	tokens []goToken
}

type byteRange struct{ start, end int }

func parseGoFile(src []byte) (*goFile, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	tokens, err := scanGoTokens(src)
	if err != nil {
		return nil, err
	}
	return &goFile{fset: fset, file: file, tokens: tokens}, nil
}

func parseBoth(old, new []byte) (*goFile, *goFile, bool) {
	oldFile, err := parseGoFile(old)
	if err != nil {
		return nil, nil, false
	}
	newFile, err := parseGoFile(new)
	if err != nil {
		return nil, nil, false
	}
	return oldFile, newFile, true
}

func scanGoTokens(src []byte) ([]goToken, error) {
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	var scanErr error
	var s scanner.Scanner
	s.Init(file, src, func(_ token.Position, msg string) { scanErr = errors.New(msg) }, scanner.ScanComments)
	var out []goToken
	for {
		pos, kind, lit := s.Scan()
		if kind == token.EOF {
			break
		}
		out = append(out, goToken{offset: file.Offset(pos), kind: kind, lit: lit})
	}
	return out, scanErr
}

func (f *goFile) offset(pos token.Pos) int { return f.fset.Position(pos).Offset }

// outside returns the tokens that fall in none of the ascending ranges.
func outside(tokens []goToken, ranges []byteRange) []goToken {
	var out []goToken
	next := 0
	for _, tok := range tokens {
		for next < len(ranges) && tok.offset >= ranges[next].end {
			next++
		}
		if next < len(ranges) && tok.offset >= ranges[next].start {
			continue
		}
		out = append(out, tok)
	}
	return out
}

// inside renders the tokens of each range as one comparable string.
func inside(tokens []goToken, ranges []byteRange) []string {
	out := make([]string, len(ranges))
	for index, span := range ranges {
		var parts []string
		for _, tok := range tokens {
			if tok.offset >= span.start && tok.offset < span.end {
				parts = append(parts, tok.kind.String()+"\x00"+tok.lit)
			}
		}
		out[index] = strings.Join(parts, "\x01")
	}
	return out
}

func equalTokens(a, b []goToken) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index].kind != b[index].kind || a[index].lit != b[index].lit {
			return false
		}
	}
	return true
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}

func sorted(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

// importReorderProven: the import specs form the same multiset, the comments
// inside import declarations form the same multiset, and every token outside
// the import declarations is identical in order (CEM-SM-009).
func importReorderProven(old, new []byte) bool {
	oldFile, newFile, ok := parseBoth(old, new)
	if !ok {
		return false
	}
	oldSpecs, newSpecs := importSpecs(oldFile.file), importSpecs(newFile.file)
	if equalStrings(oldSpecs, newSpecs) || !equalStrings(sorted(oldSpecs), sorted(newSpecs)) {
		return false
	}
	oldRanges, newRanges := importRanges(oldFile), importRanges(newFile)
	if !equalStrings(rangeComments(oldFile.tokens, oldRanges), rangeComments(newFile.tokens, newRanges)) {
		return false
	}
	return equalTokens(outside(oldFile.tokens, oldRanges), outside(newFile.tokens, newRanges))
}

func importSpecs(file *ast.File) []string {
	var out []string
	for _, spec := range file.Imports {
		name := ""
		if spec.Name != nil {
			name = spec.Name.Name
		}
		out = append(out, name+" "+spec.Path.Value)
	}
	return out
}

func importRanges(f *goFile) []byteRange {
	var out []byteRange
	for _, decl := range f.file.Decls {
		general, ok := decl.(*ast.GenDecl)
		if !ok || general.Tok != token.IMPORT {
			continue
		}
		out = append(out, byteRange{f.offset(general.Pos()), f.offset(general.End())})
	}
	return out
}

func rangeComments(tokens []goToken, ranges []byteRange) []string {
	var out []string
	for _, tok := range tokens {
		if tok.kind != token.COMMENT {
			continue
		}
		for _, span := range ranges {
			if tok.offset >= span.start && tok.offset < span.end {
				out = append(out, tok.lit)
			}
		}
	}
	return sorted(out)
}

// moveProven: the top-level declarations (with their doc comments) form the
// same multiset in a different order, every token outside them is identical,
// and the relative order of order-sensitive declarations (var declarations
// and init functions) is unchanged (CEM-SM-008).
func moveProven(old, new []byte) bool {
	oldFile, newFile, ok := parseBoth(old, new)
	if !ok {
		return false
	}
	oldRanges, newRanges := declRanges(oldFile), declRanges(newFile)
	if !equalTokens(outside(oldFile.tokens, oldRanges), outside(newFile.tokens, newRanges)) {
		return false
	}
	oldDecls, newDecls := inside(oldFile.tokens, oldRanges), inside(newFile.tokens, newRanges)
	if equalStrings(oldDecls, newDecls) {
		return false
	}
	if !equalStrings(sorted(oldDecls), sorted(newDecls)) {
		return false
	}
	return equalStrings(orderSensitive(oldFile.file, oldDecls), orderSensitive(newFile.file, newDecls))
}

func declRanges(f *goFile) []byteRange {
	out := make([]byteRange, 0, len(f.file.Decls))
	for _, decl := range f.file.Decls {
		out = append(out, byteRange{f.offset(declStart(decl)), f.offset(decl.End())})
	}
	return out
}

func declStart(decl ast.Decl) token.Pos {
	switch typed := decl.(type) {
	case *ast.GenDecl:
		if typed.Doc != nil {
			return typed.Doc.Pos()
		}
	case *ast.FuncDecl:
		if typed.Doc != nil {
			return typed.Doc.Pos()
		}
	}
	return decl.Pos()
}

func orderSensitive(file *ast.File, rendered []string) []string {
	var out []string
	for index, decl := range file.Decls {
		if isOrderSensitive(decl) {
			out = append(out, rendered[index])
		}
	}
	return out
}

func isOrderSensitive(decl ast.Decl) bool {
	switch typed := decl.(type) {
	case *ast.GenDecl:
		return typed.Tok == token.VAR
	case *ast.FuncDecl:
		return typed.Recv == nil && typed.Name.Name == "init"
	}
	return false
}

// renameProven: the token streams differ only by one identifier pair
// from→to (and comments that differ only by that whole-word substitution);
// both names are unexported, `to` is fresh in the base file, and `from` is
// declared in the base file without any use the file cannot resolve alone
// (selector, composite-literal key, struct field, interface or concrete
// method, package name) (CEM-SM-007).
func renameProven(old, new []byte) bool {
	oldFile, newFile, ok := parseBoth(old, new)
	if !ok {
		return false
	}
	from, to, ok := identSubstitution(oldFile.tokens, newFile.tokens)
	if !ok {
		return false
	}
	if exported(from) || exported(to) {
		return false
	}
	if identOccurs(oldFile.tokens, to) {
		return false
	}
	return renameableDeclaration(oldFile.file, from)
}

// identSubstitution finds the single identifier pair by which two equal-length
// token streams differ; comments may differ only by that pair as whole words
// and never when they carry a compiler directive.
func identSubstitution(old, new []goToken) (string, string, bool) {
	if len(old) != len(new) {
		return "", "", false
	}
	from, to := "", ""
	var comments []int
	for index := range old {
		a, b := old[index], new[index]
		if a.kind != b.kind {
			return "", "", false
		}
		if a.lit == b.lit {
			continue
		}
		switch a.kind {
		case token.IDENT:
			if from != "" && (from != a.lit || to != b.lit) {
				return "", "", false
			}
			from, to = a.lit, b.lit
		case token.COMMENT:
			comments = append(comments, index)
		default:
			return "", "", false
		}
	}
	if from == "" {
		return "", "", false
	}
	for _, index := range comments {
		if directive(old[index].lit) || replaceWord(old[index].lit, from, to) != new[index].lit {
			return "", "", false
		}
	}
	return from, to, true
}

func directive(comment string) bool {
	return strings.HasPrefix(comment, "//go:") || strings.HasPrefix(comment, "//line ") ||
		strings.HasPrefix(comment, "//export ")
}

func identChar(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

// replaceWord substitutes whole identifier words only.
func replaceWord(text, from, to string) string {
	var out strings.Builder
	word := strings.Builder{}
	flush := func() {
		if word.String() == from {
			out.WriteString(to)
		} else {
			out.WriteString(word.String())
		}
		word.Reset()
	}
	for _, r := range text {
		if identChar(r) {
			word.WriteRune(r)
			continue
		}
		flush()
		out.WriteRune(r)
	}
	flush()
	return out.String()
}

func exported(name string) bool {
	first, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(first)
}

func identOccurs(tokens []goToken, name string) bool {
	for _, tok := range tokens {
		if tok.kind == token.IDENT && tok.lit == name {
			return true
		}
	}
	return false
}

// renameableDeclaration reports whether name is declared in the file and
// every use of it is one the file resolves by itself.
func renameableDeclaration(file *ast.File, name string) bool {
	if file.Name.Name == name {
		return false
	}
	declared, unresolvable := false, false
	ast.Inspect(file, func(node ast.Node) bool {
		declared = declared || declaresName(node, name)
		unresolvable = unresolvable || usesUnresolvably(node, name)
		return !unresolvable
	})
	return declared && !unresolvable
}

func declaresName(node ast.Node, name string) bool {
	switch typed := node.(type) {
	case *ast.FuncDecl:
		return (typed.Recv == nil && typed.Name.Name == name) || namesInclude(typed.Recv, name)
	case *ast.FuncType:
		return namesInclude(typed.Params, name) || namesInclude(typed.Results, name) || namesInclude(typed.TypeParams, name)
	case *ast.ValueSpec:
		return identsInclude(typed.Names, name)
	case *ast.TypeSpec:
		return typed.Name.Name == name || namesInclude(typed.TypeParams, name)
	case *ast.ImportSpec:
		return typed.Name != nil && typed.Name.Name == name
	case *ast.LabeledStmt:
		return typed.Label.Name == name
	case *ast.AssignStmt:
		return typed.Tok == token.DEFINE && exprsInclude(typed.Lhs, name)
	case *ast.RangeStmt:
		return typed.Tok == token.DEFINE && exprsInclude([]ast.Expr{typed.Key, typed.Value}, name)
	}
	return false
}

func usesUnresolvably(node ast.Node, name string) bool {
	switch typed := node.(type) {
	case *ast.SelectorExpr:
		return typed.Sel.Name == name
	case *ast.KeyValueExpr:
		return exprsInclude([]ast.Expr{typed.Key}, name)
	case *ast.StructType:
		return namesInclude(typed.Fields, name)
	case *ast.InterfaceType:
		return namesInclude(typed.Methods, name)
	case *ast.FuncDecl:
		return typed.Recv != nil && typed.Name.Name == name
	}
	return false
}

func namesInclude(fields *ast.FieldList, name string) bool {
	if fields == nil {
		return false
	}
	for _, field := range fields.List {
		if identsInclude(field.Names, name) {
			return true
		}
	}
	return false
}

func identsInclude(idents []*ast.Ident, name string) bool {
	for _, ident := range idents {
		if ident.Name == name {
			return true
		}
	}
	return false
}

func exprsInclude(exprs []ast.Expr, name string) bool {
	for _, expr := range exprs {
		if ident, ok := expr.(*ast.Ident); ok && ident.Name == name {
			return true
		}
	}
	return false
}
