package contextindex

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
)

// SymbolExtent is one declaration an index symbol extractor names in a single
// source, with the 1-based inclusive line range its text occupies and that
// text. It exists for know-how symbol anchors (KHN-V0-016), which pin a
// declaration's content rather than a whole file.
type SymbolExtent struct {
	Kind, Name string
	Start, End int
	Content    string
}

// SymbolExtents returns the declarations the index's own extractors name in
// one source, in source order. It adds no language and no parser: a Go source
// goes through go/parser with goDeclaredNames' naming, and a Python, Rust, C#,
// Swift, Kotlin, Ruby or web source through the same extractor the index build
// runs. ok is false when the path has no extractor, the bytes are not text, or
// the extractor refused or truncated the source, so a caller never mistakes a
// partial walk for the whole file.
//
// A Go method is named Receiver.Method, because the index model's bare method
// name is ambiguous across receivers; its extent runs from its doc comment to
// its closing brace, and a member of a parenthesized group spans its own spec.
// Every other extent is the declaration's reported end when the grammar
// reports one, else the line before the source's next symbol, else the last
// line, with trailing blank lines trimmed: the span ranker's rule without its
// display clip.
func SymbolExtents(path string, data []byte) ([]SymbolExtent, bool) {
	if !textDecodable(data) {
		return nil, false
	}
	text := string(data)
	if strings.HasSuffix(path, ".go") {
		return goSymbolExtents(text)
	}
	source := Source{Path: path}
	var symbols []Symbol
	switch {
	case strings.HasSuffix(path, ".py"):
		found, _, refusal := pythonSourceFacts(source, text)
		if refusal != "" {
			return nil, false
		}
		symbols = found
	default:
		text = normalizeLineEndings(text)
		found, notes, handled := languageSymbols(source, text)
		if !handled || len(notes) > 0 {
			return nil, false
		}
		symbols = found
	}
	extents := newExtentSource(text)
	// The next symbol's line, for an extent the grammar gives no end, is a
	// binary search over the sorted symbol lines: O(S log S), not all pairs.
	starts := make([]int, 0, len(symbols))
	for _, s := range symbols {
		starts = append(starts, s.Line)
	}
	slices.Sort(starts)
	out := make([]SymbolExtent, 0, len(symbols))
	for _, s := range symbols {
		end := len(extents.lines)
		if s.EndLine > 0 {
			end = s.EndLine
		} else if next, _ := slices.BinarySearch(starts, s.Line+1); next < len(starts) {
			end = min(end, starts[next]-1)
		}
		out = append(out, extents.extent(s.Kind, s.Name, s.Line, end))
	}
	return out, true
}

// extentSource is a source's lines with each line's byte offset, so an
// extent's Content is a substring of the one source text: S names declared by
// one B-byte declaration retain O(B) text, not S copies (V1-3015).
type extentSource struct {
	text    string
	lines   []string
	offsets []int
}

func newExtentSource(text string) extentSource {
	lines := strings.Split(text, "\n")
	offsets := make([]int, len(lines))
	offset := 0
	for index, line := range lines {
		offsets[index] = offset
		offset += len(line) + 1
	}
	return extentSource{text: text, lines: lines, offsets: offsets}
}

// extent clamps [start, end] to the source, trims trailing blank lines and
// captures the text, which is strings.Join(lines[start-1:end], "\n").
func (source extentSource) extent(kind, name string, start, end int) SymbolExtent {
	lines := source.lines
	start = min(max(start, 1), len(lines))
	end = min(max(end, start), len(lines))
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	content := source.text[source.offsets[start-1] : source.offsets[end-1]+len(lines[end-1])]
	return SymbolExtent{Kind: kind, Name: name, Start: start, End: end, Content: content}
}

// goSymbolExtents walks the same top-level declarations goSymbols does. A
// source the grammar refuses has no extents: the scanner fallback names lines,
// not declarations.
func goSymbolExtents(text string) ([]SymbolExtent, bool) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "source.go", text, parser.SkipObjectResolution|parser.ParseComments)
	if err != nil {
		return nil, false
	}
	source := newExtentSource(text)
	line := func(p token.Pos) int { return fileSet.Position(p).Line }
	var out []SymbolExtent
	for _, declaration := range file.Decls {
		switch typed := declaration.(type) {
		case *ast.FuncDecl:
			name := typed.Name.Name
			if receiver := goReceiverName(typed); receiver != "" {
				name = receiver + "." + name
			}
			out = append(out, source.extent("func", name, line(goDocStart(typed.Doc, typed.Pos())), line(typed.End())))
		case *ast.GenDecl:
			for _, spec := range typed.Specs {
				start, end := goDocStart(typed.Doc, typed.Pos()), typed.End()
				if typed.Lparen.IsValid() {
					start, end = goDocStart(goSpecDoc(spec), spec.Pos()), spec.End()
				}
				for _, declared := range goSpecNames(spec) {
					out = append(out, source.extent(declared.kind, declared.name, line(start), line(end)))
				}
			}
		}
	}
	return out, true
}

func goDocStart(doc *ast.CommentGroup, pos token.Pos) token.Pos {
	if doc != nil {
		return doc.Pos()
	}
	return pos
}

func goSpecDoc(spec ast.Spec) *ast.CommentGroup {
	switch typed := spec.(type) {
	case *ast.TypeSpec:
		return typed.Doc
	case *ast.ValueSpec:
		return typed.Doc
	}
	return nil
}

// goReceiverName is a method's receiver base type name without pointer or
// type parameters, or "" for a function.
func goReceiverName(declaration *ast.FuncDecl) string {
	if declaration.Recv == nil || len(declaration.Recv.List) == 0 {
		return ""
	}
	expr := declaration.Recv.List[0].Type
	for {
		switch typed := expr.(type) {
		case *ast.StarExpr:
			expr = typed.X
		case *ast.ParenExpr:
			expr = typed.X
		case *ast.IndexExpr:
			expr = typed.X
		case *ast.IndexListExpr:
			expr = typed.X
		case *ast.Ident:
			return typed.Name
		default:
			return ""
		}
	}
}
