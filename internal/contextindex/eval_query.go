package contextindex

import (
	"strings"
)

// symbolsPerWorker is how many symbols have to be waiting before a second
// worker, and its own term maps and source-line cache, earns its allocation.
const symbolsPerWorker = 2_048

// EvalImpact applies the shared packet budget to the broad impact kernel
// without changing the public impact command's qualified option surface.
func EvalImpact(index *Index, paths []string, limit int, budget *int) (map[string]any, error) {
	return evalImpact(index, paths, limit, budget, false)
}

// EvalImpactSyntax preserves the versioned frontier through packet budgeting.
func EvalImpactSyntax(index *Index, paths []string, limit int, budget *int) (map[string]any, error) {
	return evalImpact(index, paths, limit, budget, true)
}

// PathImpactSyntax is the one syntax-profile path impact entry that the MCP
// corvint.impact tool and the CLI share (NGI-V0-068): a caller budget compiles
// through EvalImpactSyntax, and an omitted budget is the unbudgeted ImpactSyntax
// receipt. No surface adds an implicit budget of its own.
func PathImpactSyntax(index *Index, paths []string, limit int, budget *int) (map[string]any, error) {
	if budget != nil {
		return EvalImpactSyntax(index, paths, limit, budget)
	}
	return ImpactSyntax(index, paths, limit)
}

// SyntaxProfileSelected reports a path request that the default profile answers
// with the syntax profile, which is any request naming a Ruby path (NGI-V0-033).
func SyntaxProfileSelected(paths []string) bool {
	return rubyImpactRequested(paths)
}

func evalImpact(index *Index, paths []string, limit int, budget *int, syntaxFrontier bool) (map[string]any, error) {
	result, disclosures, view, err := impact(index, paths, limit, syntaxFrontier || rubyImpactRequested(paths)) // NGI-V0-033
	if err != nil {
		return nil, err
	}
	return compileReceipt(result, budget, view, disclosures...)
}

// evalWebPath is the oracle's web-symbol suffix set, which _build tests with
// PurePosixPath(path).suffix.lower() before it reaches _web_symbols.
func evalWebPath(value string) bool {
	switch pythonLower(evalPathExtension(value)) {
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return true
	}
	return false
}

func evalPathExtension(value string) string {
	if index := strings.LastIndexByte(value, '.'); index >= 0 {
		return value[index:]
	}
	return ""
}

// evalRecomputePathTerms restores the per-symbol construction of the symbol
// term set. Nothing in the product sets it; the receipt parity test flips it so
// one binary can emit the same receipt both ways and compare them.
var evalRecomputePathTerms = false

// evalSymbolTerms serves terms(name + " " + path), and terms(name) beside it,
// while tokenising each path once. terms distributes over a space, so that set is exactly the union of
// the two sides' sets: camelSplit inserts a hyphen only between two adjacent
// letters or digits, and a space is neither, so no boundary decision reaches
// across one; ToLower and the '_' rewrite are per character; and asciiWords
// starts a word only at an ASCII letter and ends it at the first character
// outside [letter digit _ -], so no word spans a space either. Everything
// after that -- the '-' split, the stop-word and length gates, the plural and
// -ify stems, the aliases -- is a function of one word alone.
//
// Symbols are ranked in path order, so one cached path covers every
// declaration in a file: the Beamfall pass tokenises 2,617 paths instead of
// once per symbol, and the path is the long half of the pair.
type evalSymbolTerms struct {
	path      string
	pathTerms map[string]struct{}
}

// pathTermsOf is the cached terms(sourcePath). The map is shared by every
// symbol of the file: evalPreparedSymbol.path is read only through lookups
// (evalAnyVersionedPath, intersectionCountSet), never written.
func (cache *evalSymbolTerms) pathTermsOf(sourcePath string) map[string]struct{} {
	if evalRecomputePathTerms {
		return terms(sourcePath)
	}
	if cache.pathTerms == nil || cache.path != sourcePath {
		cache.path, cache.pathTerms = sourcePath, terms(sourcePath)
	}
	return cache.pathTerms
}

// evalSourceLines serves the split source text of one path at a time. Symbols
// are ranked in path order, so a single entry covers every declaration in a
// file; splitting per symbol re-materialised and re-split the whole file once
// per declaration, which is the dominant allocation of the query path.
type evalSourceLines struct {
	index *Index
	path  string
	lines []string
	// keep is the query's term set, the keep-set of every context window.
	keep map[string]struct{}
}

func (cache *evalSourceLines) of(sourcePath string) *evalSourceLines {
	if cache.path == sourcePath {
		return cache
	}
	cache.path, cache.lines = sourcePath, nil
	if source, ok := cache.index.Sources[sourcePath]; ok {
		if text, valid, loaded := source.Text(); loaded && valid {
			cache.lines = strings.Split(text, "\n")
		}
	}
	return cache
}

// symbolContextWindow is the half-open line range a symbol's context terms are
// read from. The oracle keeps one window per extractor rather than one for the
// whole index: `_go_symbols` opens at Line-4, `_web_symbols` at Line-3, and
// `_python_symbols` at Line-3 clamped to the declaration's own end. The ranking
// scores the terms that text yields, so serving every language the Go window
// admitted a symbol the oracle's filter rejects and re-scored the one below it
// (DR-0005; decision 0007 D4 as amended 2026-08-29 on measurement). Languages
// the oracle extracts no symbols for keep the Go window: no oracle window
// exists to port for them.
func symbolContextWindow(symbol Symbol, lineCount int) (int, int) {
	end := min(lineCount, symbol.Line+20)
	if strings.HasSuffix(symbol.Path, ".py") {
		// min(len(lines), node.end_lineno, lineno+20). A symbol whose
		// extractor named no end keeps lineno+20, which is the oracle's own
		// fallback when a node carries no end_lineno.
		if symbol.EndLine > 0 {
			end = min(end, symbol.EndLine)
		}
		return max(0, symbol.Line-3), end
	}
	if evalWebPath(symbol.Path) {
		return max(0, symbol.Line-3), end
	}
	return max(0, symbol.Line-4), end
}
