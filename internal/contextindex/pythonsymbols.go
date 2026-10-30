package contextindex

import (
	"path"
	"strings"

	"github.com/Beamfall/corvint/internal/pythongrammar"
	"github.com/Beamfall/corvint/internal/pythonsyntax"
)

// pythonDefinitions turns the grammar's definition facts into index symbols.
// The grammar names the construct it parsed; the identity of the file it came
// from belongs to this collector, which closes over its source.
type pythonDefinitions struct {
	source  Source
	symbols []Symbol
	// byLine finds a definition again when the grammar reports where its suite
	// ended. A definition's suite consumes the rest of its line, so no two
	// definitions in one source start on the same line and the start line is
	// an identity here.
	byLine map[int]int
}

// pythonSymbolKinds maps the grammar's definition kinds onto the index
// vocabulary, which does not distinguish a coroutine from a plain function.
var pythonSymbolKinds = map[string]string{
	"function":       "func",
	"async-function": "func",
	"class":          "class",
}

func (collector *pythonDefinitions) Add(kind, _, _, value string, line, _ int) pythongrammar.Failure {
	if kind != "python.definition" {
		return ""
	}
	separator := strings.IndexByte(value, ':')
	if separator < 0 {
		return ""
	}
	symbolKind, known := pythonSymbolKinds[value[:separator]]
	if !known {
		return ""
	}
	name := value[separator+1:]
	if collector.byLine == nil {
		collector.byLine = map[int]int{}
	}
	collector.byLine[line] = len(collector.symbols)
	collector.symbols = append(collector.symbols, Symbol{Kind: symbolKind, Name: name, Path: collector.source.Path, BlobHash: collector.source.BlobHash, Line: line})
	return ""
}

// DefinitionEnd records the last line of a definition's suite, which is the
// oracle's ast end_lineno for the same node and the bound its Python context
// window is clamped to. It is the optional half of the grammar's sink: the
// definition fact itself carries only the line the declaration opens on.
func (collector *pythonDefinitions) DefinitionEnd(startLine, endLine int) {
	index, known := collector.byLine[startLine]
	if !known {
		return
	}
	collector.symbols[index].EndLine = endLine
}

// pythonSymbols extracts definitions from one Python source through the closed
// 3.12 grammar. It reports nothing for a source the grammar rejects, because a
// partial walk would name a subset of the file's definitions as if it were all
// of them.
func pythonSymbols(source Source, text string) []Symbol {
	// The grammar requires a newline-terminated final line where the oracle's
	// parser does not, and appending one cannot change which definitions a
	// Python source contains.
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	collector := &pythonDefinitions{source: source}
	if pythonsyntax.ParseValidated(source.Path, []byte(text), collector) != "" {
		return nil
	}
	return collector.symbols
}

// pythonImports collects the grammar's static import facts. The grammar emits
// the module exactly as written, so a relative import arrives with its leading
// dots intact ("." , ".sibling", "..pkg.mod") where CPython's ast splits them
// into node.level and node.module. Recovering the level is therefore a matter
// of counting the leading dots.
type pythonImports struct {
	sourcePath string
	modules    map[string]struct{}
}

func (collector *pythonImports) Add(kind, _, _, value string, _, _ int) pythongrammar.Failure {
	if kind != "python.import.static" {
		return ""
	}
	module := resolvePythonModule(collector.sourcePath, value)
	if module != "" {
		collector.modules[module] = struct{}{}
	}
	return ""
}

// pythonImportLine returns the line of the first static import statement whose
// resolved module is imported, or 0 when none is (including a source the
// grammar refuses, which the import set also reads as importing nothing).
func pythonImportLine(sourcePath, text, imported string) int {
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	collector := &pythonImportLines{sourcePath: sourcePath, imported: imported}
	if refusal := pythonsyntax.ParseValidated(sourcePath, []byte(text), collector); refusal != "" {
		return 0
	}
	return collector.line
}

type pythonImportLines struct {
	sourcePath, imported string
	line                 int
}

func (collector *pythonImportLines) Add(kind, _, _, value string, line, _ int) pythongrammar.Failure {
	if kind != "python.import.static" {
		return ""
	}
	if collector.line != 0 {
		return ""
	}
	if resolvePythonModule(collector.sourcePath, value) == collector.imported {
		collector.line = line
	}
	return ""
}

// pythonFacts fans one grammar walk out to both collectors. Definitions and
// imports were previously gathered by two ParsePython312Subset calls over the
// same bytes, which parsed every Python source in the repository twice.
type pythonFacts struct {
	definitions pythonDefinitions
	imports     pythonImports
}

func (collector *pythonFacts) Add(kind, subject, relation, value string, line, column int) pythongrammar.Failure {
	collector.definitions.Add(kind, subject, relation, value, line, column)
	collector.imports.Add(kind, subject, relation, value, line, column)
	return ""
}

// DefinitionEnd forwards the span to the only collector that holds symbols.
// The fan-out sink implements it so the assertion the grammar makes against
// the sink it was handed succeeds for the two-collector walk as well.
func (collector *pythonFacts) DefinitionEnd(startLine, endLine int) {
	collector.definitions.DefinitionEnd(startLine, endLine)
}

// pythonSourceFacts extracts one Python source's definitions and imports in a
// single parse. A source the grammar rejects yields neither, matching both the
// definition rule above and the oracle's `except SyntaxError: return set()`.
//
// The third result names the refusal when there was one. Dropping it made the
// rejection unobservable: the caller appended an empty symbol slice for a file
// that stayed counted as indexed, so a file the grammar could not read and a
// file that defines nothing produced identical evidence.
func pythonSourceFacts(source Source, text string) ([]Symbol, map[string]struct{}, string) {
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	collector := &pythonFacts{
		definitions: pythonDefinitions{source: source},
		imports:     pythonImports{sourcePath: source.Path, modules: map[string]struct{}{}},
	}
	if refusal := pythonsyntax.ParseValidated(source.Path, []byte(text), collector); refusal != "" {
		return nil, nil, string(refusal)
	}
	return collector.definitions.symbols, collector.imports.modules, ""
}

// resolvePythonModule reproduces the oracle's relative-import resolution in
// src/context_corvint_index.py:1197-1211. An absolute import is returned
// unchanged; `import a.b.c` and `from a.b import c` both yield the module
// string as written, which is why both forms share this path.
func resolvePythonModule(sourcePath, written string) string {
	level := 0
	for level < len(written) && written[level] == '.' {
		level++
	}
	module := written[level:]
	if level == 0 {
		return module
	}

	parts := pythonPathParts(sourcePath)
	if len(parts) > 0 && (parts[0] == "src" || parts[0] == "lib") {
		parts = parts[1:]
	}
	if len(parts) > 0 {
		// Drop the module's own name to reach its package directory. The
		// oracle pops __init__ and a plain module name alike.
		parts = parts[:len(parts)-1]
	}
	keep := max(0, len(parts)-level+1)
	resolved := append(append([]string{}, parts[:keep]...), strings.Split(module, ".")...)

	kept := resolved[:0]
	for _, part := range resolved {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, ".")
}

// pythonPathParts is PurePosixPath(path).with_suffix("").parts: the repository
// path with its final extension removed, split on the separator.
func pythonPathParts(sourcePath string) []string {
	stem := strings.TrimSuffix(sourcePath, path.Ext(sourcePath))
	if stem == "" {
		return nil
	}
	return strings.Split(stem, "/")
}

// pythonImportSet returns the modules a Python source imports, matching the
// oracle's ast-based extraction. A file the grammar cannot parse yields no
// imports, exactly as the oracle returns an empty set on SyntaxError.
func pythonImportSet(sourcePath, text string) map[string]struct{} {
	_, modules, _ := pythonSourceFacts(Source{Path: sourcePath}, text)
	if modules == nil {
		return map[string]struct{}{}
	}
	return modules
}

// PythonImportCandidates is the set of module spellings by which other Python
// sources may import sourcePath, reproducing src/context_corvint.py:723-731.
// Exported so other packages that resolve raw import specifiers to source
// paths (e.g. internal/disagree's structural channel) can reuse the same
// oracle-matching candidate set instead of re-deriving it.
func PythonImportCandidates(sourcePath string) map[string]struct{} {
	parts := pythonPathParts(sourcePath)
	candidates := map[string]struct{}{strings.Join(parts, "."): {}}
	if len(parts) > 0 && (parts[0] == "src" || parts[0] == "lib") {
		candidates[strings.Join(parts[1:], ".")] = struct{}{}
	}
	if len(parts) > 0 {
		candidates[parts[len(parts)-1]] = struct{}{}
	}
	if len(parts) > 0 && parts[len(parts)-1] == "__init__" {
		candidates[strings.Join(parts[:len(parts)-1], ".")] = struct{}{}
		if parts[0] == "src" || parts[0] == "lib" {
			candidates[strings.Join(parts[1:len(parts)-1], ".")] = struct{}{}
		}
	}
	delete(candidates, "")
	return candidates
}
