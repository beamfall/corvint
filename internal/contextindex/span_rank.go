package contextindex

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/runtimeenv"
)

// TCP-V0-025..027: with `CORVINT_CONTEXT_SPANS=on` the packet carries span
// rows beside its file rows: for each result, in rank order, the core span
// the task's names locate (a definition of a name the task writes, else the
// declaration enclosing the first line naming one, else the line carrying the
// most task terms), then call sites of each core span's symbol, all bounded by
// a declared line budget. Spans read the index's symbols, its `Words`
// postings and its reverse-import rules; the index, snapshot and pack are
// unchanged and the `results` array is untouched.
const (
	contextSpanLineBudget = 240
	contextSpanCoreCap    = 8
	contextSpanRowCap     = 3
	contextSpanCallCap    = 2
	contextSpanMaxLines   = 80
	contextSpanCallRadius = 2
	contextSpanLineRadius = 5
	contextSpanMinSymbol  = 4
)

type contextSpan struct {
	role, path, symbol, reason, confidence, authority string
	start, end                                        int
}

func (span contextSpan) lines() int { return span.end - span.start + 1 }

func (span contextSpan) overlaps(other contextSpan) bool {
	return span.path == other.path && span.start <= other.end && other.start <= span.end
}

func contextSpansEnabled() bool { return runtimeenv.Value("CONTEXT_SPANS") == "on" }

// attachSpans is the one hook the packet calls: it adds the `spans` member
// and `coverage.sufficiency` (TCP-V0-028) when the flag is on and leaves the
// packet's bytes alone otherwise.
func (compiler *taskContextCompiler) attachSpans(packet map[string]any, rows []contextRow) {
	if !contextSpansEnabled() {
		return
	}
	ranker := newSpanRanker(compiler, rows)
	spans, omitted := ranker.rank(contextSpanLineBudget)
	packet["spans"] = ranker.packet(spans, omitted, contextSpanLineBudget)
	packet["coverage"].(map[string]any)["sufficiency"] = compiler.sufficiency(spans, ranker.lines).packet()
}

type spanName struct {
	name   string
	weight int
}

type spanRanker struct {
	compiler *taskContextCompiler
	rows     []contextRow
	// names are the task's code-shaped names: definitionEligible identifiers
	// with their weight, then TCP-V0-016's names at weight one.
	names   []spanName
	weights map[string]int
	terms   []string
	symbols map[string][]Symbol
	text    map[string]string
	split   map[string][]string
}

func newSpanRanker(compiler *taskContextCompiler, rows []contextRow) *spanRanker {
	ranker := &spanRanker{
		compiler: compiler, rows: rows, weights: map[string]int{},
		terms: compiler.terms, symbols: map[string][]Symbol{}, text: map[string]string{}, split: map[string][]string{},
	}
	for _, identifier := range compiler.identifiers {
		if definitionEligible(identifier) {
			ranker.addName(identifier.name, identifier.weight)
		}
	}
	for _, name := range taskNames(compiler.task) {
		ranker.addName(name, 1)
	}
	sort.SliceStable(ranker.names, func(left, right int) bool { return ranker.names[left].weight > ranker.names[right].weight })
	ranker.loadSymbols()
	return ranker
}

func (ranker *spanRanker) addName(name string, weight int) {
	if _, seen := ranker.weights[name]; seen {
		return
	}
	ranker.weights[name] = weight
	ranker.names = append(ranker.names, spanName{name, weight})
}

// loadSymbols keeps the symbols of the result paths, in line order.
func (ranker *spanRanker) loadSymbols() {
	wanted := map[string]struct{}{}
	for _, row := range ranker.rows {
		wanted[row.path] = struct{}{}
	}
	for _, symbol := range ranker.compiler.index.Symbols {
		if _, ok := wanted[symbol.Path]; ok {
			ranker.symbols[symbol.Path] = append(ranker.symbols[symbol.Path], symbol)
		}
	}
	for path := range ranker.symbols {
		sort.SliceStable(ranker.symbols[path], func(left, right int) bool {
			return ranker.symbols[path][left].Line < ranker.symbols[path][right].Line
		})
	}
}

// lines is a pinned source's text split into lines; an unindexed, oversized,
// unloadable or non-text source has none, so it yields no span.
func (ranker *spanRanker) lines(path string) ([]string, bool) {
	if cached, ok := ranker.split[path]; ok {
		return cached, cached != nil
	}
	source, pinned := ranker.compiler.index.Sources[path]
	text, readable := sourceTextBounded(source)
	if !pinned || !readable {
		ranker.split[path] = nil
		return nil, false
	}
	text = strings.TrimSuffix(text, "\n")
	ranker.text[path] = text
	ranker.split[path] = strings.Split(text, "\n")
	return ranker.split[path], true
}

// rank selects core spans in result order, then call sites in core order,
// skipping any span that would overrun the remaining budget.
func (ranker *spanRanker) rank(budget int) ([]contextSpan, int) {
	cores := ranker.coreSpans()
	candidates := append(append([]contextSpan{}, cores...), ranker.callSites(cores)...)
	selected := make([]contextSpan, 0, len(candidates))
	used, omitted := 0, 0
	for _, span := range candidates {
		if coveredBy(selected, span) {
			continue
		}
		if used+span.lines() > budget {
			omitted++
			continue
		}
		selected = append(selected, span)
		used += span.lines()
	}
	return selected, omitted
}

func coveredBy(selected []contextSpan, span contextSpan) bool {
	for _, other := range selected {
		if other.overlaps(span) {
			return true
		}
	}
	return false
}

// coreSpans takes, per result row in rank order and reserved rows excluded
// (project authority is not the task's code), the definitions of task names
// the file declares, at most contextSpanRowCap of them, else one span located
// by a task name or task terms; at most contextSpanCoreCap in all.
func (ranker *spanRanker) coreSpans() []contextSpan {
	cores := make([]contextSpan, 0, contextSpanCoreCap)
	for _, row := range ranker.rows {
		if row.kind == governingRelation || row.kind == specMentionedRelation {
			continue
		}
		cores = append(cores, ranker.rowSpans(row.path)...)
	}
	return cores[:min(len(cores), contextSpanCoreCap)]
}

func (ranker *spanRanker) rowSpans(path string) []contextSpan {
	lines, ok := ranker.lines(path)
	if !ok {
		return nil
	}
	if spans := ranker.definitionSpans(path, lines); len(spans) > 0 {
		return spans
	}
	if span, found := ranker.namedLineSpan(path, lines); found {
		return []contextSpan{span}
	}
	if span, found := ranker.termLineSpan(path, lines); found {
		return []contextSpan{span}
	}
	return nil
}

// definitionSpans: the declarations of the task names the file defines,
// heaviest name first, then by line, one per name.
func (ranker *spanRanker) definitionSpans(path string, lines []string) []contextSpan {
	defined := make([]Symbol, 0)
	seen := map[string]struct{}{}
	for _, symbol := range ranker.symbols[path] {
		_, repeated := seen[symbol.Name]
		if ranker.weights[symbol.Name] == 0 || repeated || symbol.Line < 1 || symbol.Line > len(lines) {
			continue
		}
		seen[symbol.Name] = struct{}{}
		defined = append(defined, symbol)
	}
	sort.SliceStable(defined, func(left, right int) bool {
		return ranker.weights[defined[left].Name] > ranker.weights[defined[right].Name]
	})
	spans := make([]contextSpan, 0, min(len(defined), contextSpanRowCap))
	for _, symbol := range defined[:min(len(defined), contextSpanRowCap)] {
		start, end := ranker.extent(path, symbol, len(lines))
		spans = append(spans, contextSpan{
			role: "core", path: path, symbol: symbol.Name, start: start, end: end, confidence: "high",
			authority: SyntaxAuthority, reason: "defines `" + symbol.Name + "`, which the task names",
		})
	}
	return spans
}

// namedLineSpan: the first line naming the heaviest task name the file names
// as a whole word, widened to its enclosing declaration.
func (ranker *spanRanker) namedLineSpan(path string, lines []string) (contextSpan, bool) {
	for _, name := range ranker.names {
		if _, line := countWholeWord(ranker.text[path], name.name); line > 0 {
			span := ranker.enclosing(path, line, len(lines))
			span.reason = fmt.Sprintf("names `%s` at line %d%s", name.name, line, insideReason(span.symbol))
			span.confidence, span.authority = "medium", SyntaxAuthority
			return span, true
		}
	}
	return contextSpan{}, false
}

// termLineSpan: the earliest line carrying the most distinct task terms.
func (ranker *spanRanker) termLineSpan(path string, lines []string) (contextSpan, bool) {
	wanted := stringSetOf(ranker.terms)
	bestLine, bestCount := 0, 0
	for index, line := range lines {
		count := 0
		for _, term := range lexicalTerms(line) {
			if _, ok := wanted[term]; ok {
				count++
			}
		}
		if count > bestCount {
			bestLine, bestCount = index+1, count
		}
	}
	if bestCount == 0 {
		return contextSpan{}, false
	}
	span := ranker.enclosing(path, bestLine, len(lines))
	span.reason = fmt.Sprintf("carries %d task terms at line %d%s", bestCount, bestLine, insideReason(span.symbol))
	span.confidence, span.authority = "low", "vocabulary"
	return span, true
}

func insideReason(symbol string) string {
	if symbol == "" {
		return ""
	}
	return " inside `" + symbol + "`"
}

// extent is a symbol's declared end when the grammar reports one, else the
// line before the file's next symbol, else the file's end; trailing blank
// lines are trimmed and the span is clipped to contextSpanMaxLines.
func (ranker *spanRanker) extent(path string, symbol Symbol, count int) (int, int) {
	end := count
	if symbol.EndLine > 0 {
		end = symbol.EndLine
	} else if next, ok := ranker.nextSymbolLine(path, symbol.Line); ok {
		end = next - 1
	}
	lines, _ := ranker.lines(path)
	end = min(max(end, symbol.Line), count)
	for end > symbol.Line && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return symbol.Line, min(end, symbol.Line+contextSpanMaxLines-1)
}

func (ranker *spanRanker) nextSymbolLine(path string, line int) (int, bool) {
	for _, symbol := range ranker.symbols[path] {
		if symbol.Line > line {
			return symbol.Line, true
		}
	}
	return 0, false
}

// enclosing is the last declaration starting at or before line whose extent
// reaches it, else a window of contextSpanLineRadius lines either side.
func (ranker *spanRanker) enclosing(path string, line, count int) contextSpan {
	var owner *Symbol
	for index := range ranker.symbols[path] {
		if symbol := ranker.symbols[path][index]; symbol.Line >= 1 && symbol.Line <= line {
			owner = &ranker.symbols[path][index]
		}
	}
	if owner != nil {
		if start, end := ranker.extent(path, *owner, count); end >= line {
			return contextSpan{role: "core", path: path, symbol: owner.Name, start: start, end: end}
		}
	}
	return contextSpan{role: "core", path: path, start: max(line-contextSpanLineRadius, 1), end: min(line+contextSpanLineRadius, count)}
}

// callSites: for each core span with a symbol of at least four bytes, the
// sources naming it as a whole word (at most contextMaxDefiners besides the
// definer, which counts too), packet rows first in rank order, then the
// definer's reverse importers, then by path; each yields a window around its
// first naming line that no core span overlaps.
func (ranker *spanRanker) callSites(cores []contextSpan) []contextSpan {
	sites := make([]contextSpan, 0)
	for _, core := range cores {
		if len(core.symbol) < contextSpanMinSymbol {
			continue
		}
		sites = append(sites, ranker.coreCallSites(core, cores)...)
	}
	return sites
}

func (ranker *spanRanker) coreCallSites(core contextSpan, cores []contextSpan) []contextSpan {
	table := ranker.compiler.index.vocabulary()
	low, high, found := table.Words.find(core.symbol)
	if !found || high-low > contextMaxDefiners+1 {
		return nil
	}
	candidates := make([]string, 0, high-low)
	for index := low; index < high; index++ {
		candidates = append(candidates, table.Paths[table.Words.Sources[index]])
	}
	ranker.orderCallers(core.path, candidates)
	sites := make([]contextSpan, 0, contextSpanCallCap)
	for _, candidate := range candidates {
		if len(sites) == contextSpanCallCap {
			break
		}
		if site, ok := ranker.callSite(core, candidate, cores); ok {
			sites = append(sites, site)
		}
	}
	return sites
}

func (ranker *spanRanker) orderCallers(definer string, candidates []string) {
	rank := map[string]int{}
	for position, row := range ranker.rows {
		rank[row.path] = position + 1
	}
	importers := map[string]struct{}{}
	for _, edge := range reverseImporters(ranker.compiler.index, definer) {
		importers[edge.path] = struct{}{}
	}
	sort.SliceStable(candidates, func(left, right int) bool {
		return callerKey(candidates[left], rank, importers) < callerKey(candidates[right], rank, importers)
	})
}

// callerKey orders a packet row by its rank, an importer after every row, and
// any other naming source last, each tier by path.
func callerKey(candidate string, rank map[string]int, importers map[string]struct{}) string {
	if position, ok := rank[candidate]; ok {
		return fmt.Sprintf("0 %06d", position)
	}
	if _, ok := importers[candidate]; ok {
		return "1 " + candidate
	}
	return "2 " + candidate
}

func (ranker *spanRanker) callSite(core contextSpan, candidate string, cores []contextSpan) (contextSpan, bool) {
	lines, ok := ranker.lines(candidate)
	if !ok {
		return contextSpan{}, false
	}
	for index, text := range lines {
		if count, _ := countWholeWord(text, core.symbol); count == 0 {
			continue
		}
		line := index + 1
		site := contextSpan{
			role: "call-site", path: candidate, symbol: core.symbol, confidence: "medium", authority: SyntaxAuthority,
			start: max(line-contextSpanCallRadius, 1), end: min(line+contextSpanCallRadius, len(lines)),
			reason: fmt.Sprintf("names `%s` at line %d, the declaration of the core span %s:%d-%d", core.symbol, line, core.path, core.start, core.end),
		}
		if !coveredBy(cores, site) {
			return site, true
		}
	}
	return contextSpan{}, false
}

func (ranker *spanRanker) packet(spans []contextSpan, omitted, budget int) map[string]any {
	rows := make([]any, 0, len(spans))
	used := 0
	for _, span := range spans {
		used += span.lines()
		rows = append(rows, map[string]any{
			"role": span.role, "path": span.path, "start_line": span.start, "end_line": span.end,
			"symbol": span.symbol, "reason": span.reason, "confidence": span.confidence,
			"blob_hash": ranker.compiler.index.Sources[span.path].BlobHash,
			"authority": span.authority, "trust": TrustClass(span.authority),
		})
	}
	return map[string]any{"line_budget": budget, "lines_used": used, "omitted": omitted, "rows": rows}
}
