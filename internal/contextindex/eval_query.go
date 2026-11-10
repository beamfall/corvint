package contextindex

import (
	"fmt"
	"path"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

// symbolsPerWorker is how many symbols have to be waiting before a second
// worker, and its own term maps and source-line cache, earns its allocation.
const symbolsPerWorker = 2_048

// evalConfidentCeiling is the largest `limit` evalConfidentSymbols can still
// narrow by: its own caps are 6 before dependency linking, 8 after it and 10
// after the fully-named fill, so at 10 every caller-limit cut has saturated and
// what remains is the fallback's intrinsic admission.
const evalConfidentCeiling = 10

var (
	evalGenericSymbols = stringSet("client config create delete device generate handle manager media process profile render run server session token update")
	evalGenericRouting = stringSet("browser client device manager media profile server session settings")
	evalToolingTerms   = stringSet("agent agents corvint budget codex context embedding embeddings evaluator federation memory packet precision retrieval roadmap semantic task tasks token tokens tooling trace traces")
	evalToolingPaths   = []string{"script/", "tools/", "docs/agent-", "docs/architecture/AGENT-TOOLING.md", "testing/context-retrieval"}
)

type evalIntent struct {
	id, confidence string
	matched        []string
}

// QueryTrace is the bounded trace-reader output EvalQuery is permitted to
// rank. Callers cannot use it to bypass trace-store validation: snapshots are
// accepted only through NewQueryTraceSnapshot.
// Revision is the commit the trace was recorded against. A candidate path's
// evidence always pins to the index's current blob for that path, which may
// postdate this revision; callers that build evidence from a trace MUST
// disclose Revision alongside the pin rather than let the current blob stand
// in for what the trace actually observed.
type QueryTrace struct {
	TraceID, Task, Outcome, Revision string
	OpenedPaths                      []string
	ChangedPaths                     []string
}

// QueryTraceSnapshot is an immutable, state-validating view of one completed
// tracerecordrepo.Read call.
type QueryTraceSnapshot struct {
	state   string
	records []QueryTrace
	valid   bool
}

type evalRecordCandidate struct {
	score, order int
	id           string
	record       Record
	result       map[string]any
	// support names the query terms that actually contributed to this
	// candidate's score. It is the evidence behind the packet's claim of
	// relevance, and evalQuerySupported reads it, never the score.
	support map[string]struct{}
}

type evalDocumentCandidate struct {
	score   int
	id      string
	result  map[string]any
	support map[string]struct{}
}

type evalPreparedSymbol struct {
	symbol                     Symbol
	terms, context, name, path map[string]struct{}
	nameParts                  []string
	compactName                string
}

type evalSymbolCandidate struct {
	score   int
	id      string
	symbol  evalPreparedSymbol
	result  map[string]any
	support map[string]struct{}
}

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

func evalInferIntent(text string) evalIntent {
	id, matched := normalizedQueryIntent(text)
	confidence := "default"
	if id != "repository" {
		confidence = "high"
	}
	return evalIntent{id: id, confidence: confidence, matched: matched}
}

func evalRelevanceTerms(values map[string]struct{}, intent evalIntent) map[string]struct{} {
	result := cloneSet(values)
	if intent.id == "agent-tooling" {
		for term := range evalToolingTerms {
			delete(result, term)
		}
	}
	return result
}

func evalOrderedTerms(text string) []string {
	text = pythonLower(camelSplit(text))
	text = strings.ReplaceAll(text, "_", "-")
	result := make([]string, 0)
	seen := make(map[string]struct{})
	for _, raw := range asciiWords(text) {
		for _, token := range strings.Split(raw, "-") {
			if len(token) <= 1 || contains(stopWords, token) {
				continue
			}
			if _, exists := seen[token]; !exists {
				seen[token] = struct{}{}
				result = append(result, token)
			}
		}
	}
	return result
}

func evalRankRecords(index *Index, queryText string, queryTerms map[string]struct{}, ordered []string, intent evalIntent) ([]evalRecordCandidate, map[string]struct{}) {
	values := append(sortedRecords(index.Features), sortedRecords(index.Scenarios)...)
	result := make([]evalRecordCandidate, 0)
	crossField := make(map[string]struct{})
	for _, record := range values {
		if intent.id != "repository" && (record.Kind == "feature" || record.Kind == "scenario") {
			continue
		}
		rid := pythonLower(record.ID)
		area := pythonLower(pythonString(valueOr(record.Fields["area"], "")))
		summary := pythonLower(pythonString(valueOr(record.Fields["summary"], "")))
		haystack := terms(rid + " " + area + " " + summary)
		overlap := intersection(queryTerms, haystack)
		idOverlap := intersection(queryTerms, terms(rid))
		specific, generic := differenceCount(idOverlap, evalGenericRouting), intersectionCount(idOverlap, evalGenericRouting)
		score := len(overlap)*20 + specific*map[string]int{"feature": 100, "scenario": 50}[record.Kind] + generic*map[string]int{"feature": 15, "scenario": 10}[record.Kind]
		independent := len(difference(difference(intersectionSet(overlap), intersectionSet(idOverlap)), evalGenericRouting))
		if len(idOverlap) != 0 && independent != 0 {
			score += 80
			crossField[record.Kind+":"+record.ID] = struct{}{}
		}
		if record.Kind == "feature" {
			for _, token := range idOverlap {
				if contains(evalGenericRouting, token) {
					continue
				}
				if position := stringIndex(ordered, token); position >= 0 {
					score += max(0, (len(ordered)-position)*30)
				}
			}
		}
		if queryText == rid {
			score += 1000
		} else if strings.Contains(queryText, rid) {
			score += 300
		}
		if area != "" && contains(queryTerms, area) {
			score += 15
		}
		if score != 0 {
			result = append(result, evalRecordCandidate{score: score, order: map[string]int{"feature": 0, "scenario": 1}[record.Kind], id: record.ID, record: record, result: recordResult(index, record, score, "canonical metadata matches query"), support: intersectionSet(overlap)})
		}
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].score != result[right].score {
			return result[left].score > result[right].score
		}
		if result[left].order != result[right].order {
			return result[left].order < result[right].order
		}
		return result[left].id < result[right].id
	})
	return result, crossField
}

func evalRankDocuments(index *Index, queryText string, queryTerms map[string]struct{}, intent evalIntent) ([]evalDocumentCandidate, error) {
	compactQuery := compactText(queryText)
	named := evalNamedPaths(index, queryText)
	result := make([]evalDocumentCandidate, 0)
	for _, record := range sortedRecords(index.Documents) {
		if record.Kind == "instructions" && intent.id == "repository" {
			continue
		}
		if intent.id == "project-operations" && record.Kind != "instructions" {
			continue
		}
		if !evalPathAllowed(record.Path, intent) {
			continue
		}
		searchable := record.Path + " " + pythonString(valueOr(record.Fields["title"], "")) + " " + pythonString(valueOr(record.Fields["summary"], "")) + " " + pythonString(valueOr(record.Fields["headings"], ""))
		source := index.Sources[record.Path]
		if record.Kind == "instructions" {
			body, valid, loaded := source.Text()
			if !loaded || !valid {
				continue
			}
			searchable += " " + body
		}
		overlap := intersection(queryTerms, terms(searchable))
		title := pythonString(valueOr(record.Fields["title"], ""))
		compactTitle := compactText(title)
		exactTitle := len(compactTitle) >= 4 && strings.Contains(compactQuery, compactTitle)
		minimum := 2
		if record.Kind == "instructions" && intent.id == "project-operations" {
			minimum = 1
		}
		cited := 0
		if record.Kind != "instructions" {
			cited = evalCitedNamedPaths(record, named)
		}
		if len(overlap) < minimum && !exactTitle && cited == 0 {
			continue
		}
		score := len(overlap)*18 + cited*evalNamedPathCredit
		if cited >= 2 && cited == len(named) {
			score += evalNamedPathCredit
		}
		if exactTitle {
			score += 80
		}
		if record.Kind == "instructions" && intent.id == "project-operations" {
			score += 240
		}
		status := pythonLower(pythonString(valueOr(record.Fields["status"], "")))
		if status == "accepted" || status == "approved" || status == "current" || status == "active" || strings.HasPrefix(status, "partially-superseded-by:") {
			score += 20
		}
		var candidate map[string]any
		var err error
		if record.Kind == "instructions" {
			candidate, err = instructionResult(index, record, source, score, queryTerms)
		} else {
			candidate = documentResult(index, record, score, "specification/decision matches query")
		}
		if err != nil {
			return nil, err
		}
		support := intersectionSet(overlap)
		if exactTitle {
			for _, term := range intersection(queryTerms, terms(title)) {
				support[term] = struct{}{}
			}
		}
		// Decision 0014: under project-operations intent the instructions
		// result answers the words that classified the task, so the floor
		// counts them as its support (GPK-V0-039).
		if record.Kind == "instructions" && intent.id == "project-operations" {
			for _, term := range intent.matched {
				support[term] = struct{}{}
			}
		}
		result = append(result, evalDocumentCandidate{score: score, id: record.ID, result: candidate, support: support})
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].score != result[right].score {
			return result[left].score > result[right].score
		}
		return result[left].id < result[right].id
	})
	return result, nil
}

// evalNamedPathCredit is what a document earns for each indexed path the task
// names that its own references cite, and once more when it cites every one of
// two or more (GPK-V0-975): a little more than two shared words each, so the
// document that governs the named files outranks one that only repeats their
// path tokens or cites one of them among many.
const evalNamedPathCredit = 40

// evalNamedPaths is the sorted indexed paths the task names by full path or by
// an unambiguous basename carrying a dot after its first byte (GPK-V0-975),
// the reading TCP-V0-003's `mentioned` rule gives a task. The task is read
// lowercased, so a full path names every indexed path it case-folds to
// (`src/Foo.go` and `src/foo.go` alike), never one picked by map order.
func evalNamedPaths(index *Index, queryText string) []string {
	byBase := map[string][]string{}
	lowered := map[string][]string{}
	for candidate := range index.Sources {
		folded := pythonLower(candidate)
		lowered[folded] = append(lowered[folded], candidate)
		base := pythonLower(path.Base(candidate))
		byBase[base] = append(byBase[base], candidate)
	}
	named := make([]string, 0)
	for _, token := range contextPathTokens(pythonLower(queryText)) {
		if candidates, ok := lowered[token]; ok {
			named = append(named, candidates...)
			continue
		}
		if !strings.Contains(token, ".") || strings.HasPrefix(token, ".") {
			continue
		}
		if owners := byBase[token]; len(owners) == 1 {
			named = append(named, owners[0])
		}
	}
	sort.Strings(named)
	return slices.Compact(named)
}

// evalCitedNamedPaths counts the named paths a document's references cite; a
// document never cites itself, so the named file earns nothing from its own name.
func evalCitedNamedPaths(record Record, named []string) int {
	if len(named) == 0 {
		return 0
	}
	references := stringsField(record.Fields["references"])
	cited := 0
	for _, candidate := range named {
		if candidate != record.Path && stringIn(references, candidate) {
			cited++
		}
	}
	return cited
}

// evalRankSymbols returns the ranked candidates and the index-wide symbol term
// frequency they were scored against. The table is returned rather than
// recomputed downstream because it counts every indexed symbol, including the
// ones the ranking loop drops: it is the oracle's `corvint.symbol_term_frequency`,
// and evalConfidentSymbols picks its anchor term out of the same counts.
func evalRankSymbols(index *Index, queryText string, queryTerms map[string]struct{}, intent evalIntent) ([]evalSymbolCandidate, map[string]int) {
	prepared, nameFrequency, termFrequency, anyVersioned := evalPrepareRanking(index, queryText, queryTerms, intent)
	compactQuery := compactText(queryText)
	// Decision 0017: a `vN` token narrows the universe only when some path in it
	// carries a version term to narrow by; where none does, the tokens are
	// ignored rather than emptying the universe (GPK-V0-043, DR-0015).
	versionTerms := evalVersionTerms(queryTerms)
	if len(versionTerms) != 0 && !anyVersioned() {
		versionTerms = nil
	}
	result := make([]evalSymbolCandidate, 0)
	for _, candidate := range prepared {
		overlap := intersection(queryTerms, candidate.terms)
		contextOverlap := intersection(queryTerms, candidate.context)
		nameOverlap := intersection(queryTerms, candidate.name)
		if len(versionTerms) != 0 && intersectionCountSet(versionTerms, candidate.path) == 0 {
			continue
		}
		frequency := nameFrequency[candidate.compactName]
		exactName := queryText == pythonLower(candidate.symbol.Name) || len(candidate.compactName) >= 5 && frequency == 1 && strings.Contains(compactQuery, candidate.compactName)
		if (contains(evalGenericSymbols, candidate.compactName) || frequency > 2) && len(overlap) < 2 {
			exactName = false
		}
		if contains(evalGenericSymbols, candidate.compactName) {
			support := difference(intersectionSet(contextOverlap), candidate.name)
			if len(support) < 2 {
				continue
			}
		}
		if !exactName && len(overlap) < 2 && len(contextOverlap) < 3 {
			continue
		}
		score := 0
		for _, term := range overlap {
			score += max(6, 24-min(18, termFrequency[term]-1))
		}
		score += len(contextOverlap)*6 + len(nameOverlap)*8
		if len(queryTerms) != 0 {
			score += 120 * len(contextOverlap) / len(queryTerms)
		}
		matchedParts := 0
		for _, part := range candidate.nameParts {
			if contains(queryTerms, part) {
				matchedParts++
			}
		}
		if len(candidate.nameParts) != 0 {
			score += matchedParts*50 + 20*matchedParts/len(candidate.nameParts)
			if len(candidate.nameParts) == 1 && contains(queryTerms, candidate.nameParts[0]) {
				score += 50
			}
			specificName := !contains(evalGenericSymbols, candidate.compactName) && (len(candidate.nameParts) > 1 || frequency <= 2)
			if matchedParts == len(candidate.nameParts) && candidate.symbol.Kind == "func" && specificName {
				score += 60
			}
		}
		if queryText == pythonLower(candidate.symbol.Name) {
			score += 800
		} else if exactName {
			score += 30
		}
		if score == 0 {
			continue
		}
		id := fmt.Sprintf("%s:%d:%s", candidate.symbol.Path, candidate.symbol.Line, candidate.symbol.Name)
		result = append(result, evalSymbolCandidate{score: score, id: id, symbol: candidate, result: featureSymbolResult(candidate.symbol, score, "declaration/path strongly matches query vocabulary"), support: intersectionSet(append(append([]string{}, overlap...), contextOverlap...))})
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].score != result[right].score {
			return result[left].score > result[right].score
		}
		return result[left].id < result[right].id
	})
	return result, termFrequency
}

// evalPrepareSymbols prepares the ranking input for every indexed symbol and
// counts the two frequency tables the scoring reads. Symbols are independent,
// so the pass is chunked across the CPUs: each chunk keeps its own counts and
// its own candidate list, and the merge walks the chunks in order, so the
// prepared slice and both counts are exactly what one sequential pass produced.
var evalVersionTerm = regexp.MustCompile(`^v[0-9]+$`)

// evalVersionTerms returns the query's `v[0-9]+` tokens, the oracle's
// version-narrowing vocabulary.
func evalVersionTerms(queryTerms map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{})
	for term := range queryTerms {
		if evalVersionTerm.MatchString(term) {
			result[term] = struct{}{}
		}
	}
	return result
}

// evalAnyVersionedPath reports whether any symbol in the ranking universe has a
// path term a version token could narrow by.
func evalAnyVersionedPath(prepared []evalPreparedSymbol) bool {
	for _, candidate := range prepared {
		for term := range candidate.path {
			if evalVersionTerm.MatchString(term) {
				return true
			}
		}
	}
	return false
}

// evalPrepareRanking prepares the ranking input through the snapshot's symbol
// term table when the index carries one for its symbols (IDX-SNAP-V0-807),
// and through the full pass otherwise. anyVersioned is evalAnyVersionedPath
// over the full pass's prepared set either way, evaluated only when called.
func evalPrepareRanking(index *Index, queryText string, queryTerms map[string]struct{}, intent evalIntent) ([]evalPreparedSymbol, map[string]int, map[string]int, func() bool) {
	windows := index.symbolWindowTerms(queryTerms)
	if index.symbolTerms.usable(index, windows) {
		prepared, nameFrequency, termFrequency, anyVersioned := evalPreparePrecomputed(index, index.symbolTerms, queryText, queryTerms, intent, windows)
		return prepared, nameFrequency, termFrequency, func() bool { return anyVersioned }
	}
	prepared, nameFrequency, termFrequency := evalPrepareSymbolsWith(index, queryTerms, intent, windows)
	return prepared, nameFrequency, termFrequency, func() bool { return evalAnyVersionedPath(prepared) }
}

func evalPrepareSymbolsWith(index *Index, queryTerms map[string]struct{}, intent evalIntent, windows []map[string]struct{}) ([]evalPreparedSymbol, map[string]int, map[string]int) {
	workers := min(runtime.NumCPU(), max(len(index.Symbols)/symbolsPerWorker, 1))
	chunks := make([]evalSymbolChunk, workers)
	var pending sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		pending.Add(1)
		// Contiguous chunks, never a stride: symbols arrive in path order, so a
		// worker that owns a run of them reads each file's text once.
		low := worker * len(index.Symbols) / workers
		high := (worker + 1) * len(index.Symbols) / workers
		go func(chunk *evalSymbolChunk, low, high int) {
			defer pending.Done()
			chunk.collect(index, queryTerms, intent, windows, low, high)
		}(&chunks[worker], low, high)
	}
	pending.Wait()

	prepared := make([]evalPreparedSymbol, 0, len(index.Symbols))
	nameFrequency := make(map[string]int)
	termFrequency := make(map[string]int)
	for _, chunk := range chunks {
		prepared = append(prepared, chunk.prepared...)
		for name, count := range chunk.nameFrequency {
			nameFrequency[name] += count
		}
		for term, count := range chunk.termFrequency {
			termFrequency[term] += count
		}
	}
	return prepared, nameFrequency, termFrequency
}

type evalSymbolChunk struct {
	prepared                     []evalPreparedSymbol
	nameFrequency, termFrequency map[string]int
}

func (chunk *evalSymbolChunk) collect(index *Index, queryTerms map[string]struct{}, intent evalIntent, windows []map[string]struct{}, low, high int) {
	chunk.nameFrequency = make(map[string]int)
	chunk.termFrequency = make(map[string]int)
	sources := evalSourceLines{index: index, keep: queryTerms}
	symbolTermsOf := evalSymbolTerms{}
	for offset, symbol := range index.Symbols[low:high] {
		// One term set, not two: the frequency table and the candidate's own
		// terms are the same value over the same text.
		symbolTerms, nameTerms := symbolTermsOf.of(symbol.Name, symbol.Path)
		compactName := compactText(symbol.Name)
		chunk.nameFrequency[compactName]++
		for term := range symbolTerms {
			chunk.termFrequency[term]++
		}
		// The ranking loop drops these three classes before it reads any prepared
		// field, and all three predicates are pure functions of the path and the
		// intent, so preparing them produces nothing any output depends on. Both
		// frequency tables above still count every symbol, exactly as before.
		if isTestPath(symbol.Path) || !evalPathAllowed(symbol.Path, intent) || len(queryTerms) == 0 {
			continue
		}
		context := map[string]struct{}(nil)
		if windows != nil {
			context = windows[low+offset]
		} else {
			context = sources.of(symbol.Path).contextTerms(symbol)
		}
		chunk.prepared = append(chunk.prepared, evalPreparedSymbol{
			symbol: symbol, terms: symbolTerms, context: context, name: nameTerms,
			path: symbolTermsOf.pathTermsOf(symbol.Path), nameParts: evalOrderedTerms(symbol.Name), compactName: compactName,
		})
	}
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

func (cache *evalSymbolTerms) of(name, sourcePath string) (symbolTerms, nameTerms map[string]struct{}) {
	nameTerms = terms(name)
	if evalRecomputePathTerms {
		return terms(name + " " + sourcePath), nameTerms
	}
	pathTerms := cache.pathTermsOf(sourcePath)
	result := make(map[string]struct{}, len(pathTerms)+len(nameTerms))
	for term := range pathTerms {
		result[term] = struct{}{}
	}
	for term := range nameTerms {
		result[term] = struct{}{}
	}
	return result, nameTerms
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

// evalUnfilteredKeepSetTerms restores the pre-filter construction at every
// keep-set call site. Nothing in the product sets it; the receipt parity test
// flips it so one binary can emit the same receipt both ways and compare them.
var evalUnfilteredKeepSetTerms = false

// keepSetTerms is termsMatching under the parity switch. Every caller has
// already proved its consumer reads the set only through an intersection with
// keep, so the two branches are the same value at different cost.
func keepSetTerms(text string, keep map[string]struct{}) map[string]struct{} {
	if evalUnfilteredKeepSetTerms {
		return terms(text)
	}
	return termsMatching(text, keep)
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

// contextTerms is the keep-set of one symbol's context window over this
// source. The context window is the widest text the query path tokenises --
// 2000 characters per symbol -- and evalRankSymbols reads the set it yields
// only through intersection(queryTerms, candidate.context), so building the
// terms the query never asked for is work whose every result is discarded.
// The other three sets stay unfiltered: terms feeds chunk.termFrequency, and
// name and path tokenise one name and one path, so there is no growth to
// remove.
func (cache *evalSourceLines) contextTerms(symbol Symbol) map[string]struct{} {
	return keepSetTerms(symbolContextText(symbol, cache.lines), cache.keep)
}

func evalConfidentSymbols(index *Index, ranked []evalSymbolCandidate, termFrequency map[string]int, queryText string, queryTerms map[string]struct{}, limit int) []evalSymbolCandidate {
	if len(ranked) == 0 {
		return nil
	}
	top := ranked[0].symbol.symbol
	direction := ""
	for _, prefix := range []string{"from", "to"} {
		if strings.HasPrefix(pythonLower(top.Name), prefix) {
			direction = prefix
			break
		}
	}
	directTerms := intersectionSet(intersection(evalOrderedTermSet(queryText), terms(top.Name+" "+top.Path)))
	anchor := ""
	for term := range directTerms {
		if anchor == "" || termFrequency[term] < termFrequency[anchor] || termFrequency[term] == termFrequency[anchor] && term < anchor {
			anchor = term
		}
	}
	frontierPaths := make([]string, 0, 3)
	pathTopScores := make(map[string]int)
	for _, candidate := range ranked {
		candidatePath := candidate.symbol.symbol.Path
		if _, exists := pathTopScores[candidatePath]; exists {
			continue
		}
		if candidate.score*4 < ranked[0].score*3 {
			continue
		}
		if candidatePath != top.Path && direction != "" && !strings.HasPrefix(pythonLower(candidate.symbol.symbol.Name), direction) {
			continue
		}
		if anchor != "" && !contains(terms(candidate.symbol.symbol.Name+" "+candidatePath), anchor) {
			continue
		}
		pathTopScores[candidatePath] = candidate.score
		frontierPaths = append(frontierPaths, candidatePath)
		if len(frontierPaths) == 3 {
			break
		}
	}
	byPath := make([][]evalSymbolCandidate, len(frontierPaths))
	for position, candidatePath := range frontierPaths {
		for _, candidate := range ranked {
			if candidate.symbol.symbol.Path == candidatePath && candidate.score*5 >= pathTopScores[candidatePath]*3 {
				byPath[position] = append(byPath[position], candidate)
			}
		}
		byPath[position] = evalPruneAuxiliarySymbols(byPath[position], queryText)
		byPath[position] = evalAdmitSiblings(index, byPath[position], queryTerms, queryText)
	}
	confident := make([]evalSymbolCandidate, 0, min(limit, 10))
	for offset := 0; len(confident) < min(limit, 6); offset++ {
		advanced := false
		for _, candidates := range byPath {
			if offset >= len(candidates) {
				continue
			}
			advanced = true
			confident = append(confident, candidates[offset])
			if len(confident) == min(limit, 6) {
				break
			}
		}
		if !advanced {
			break
		}
	}
	if len(confident) != 0 {
		confident = evalLinkedSymbols(index, ranked, confident, queryText, limit)
	}
	if len(confident) == 0 || len(confident) >= limit {
		return confident
	}
	selected := make(map[string]struct{}, len(confident))
	for _, candidate := range confident {
		selected[candidate.symbol.symbol.Path+":"+candidate.symbol.symbol.Name] = struct{}{}
	}
	fullyNamed := make([]evalSymbolCandidate, 0)
	for _, candidate := range ranked {
		kind := candidate.symbol.symbol.Kind
		if kind != "func" && kind != "class" && kind != "interface" && kind != "type" || len(candidate.symbol.nameParts) < 2 {
			continue
		}
		allMatched := true
		for _, part := range candidate.symbol.nameParts {
			if intersectionCountSet(terms(part), queryTerms) == 0 {
				allMatched = false
				break
			}
		}
		if allMatched {
			fullyNamed = append(fullyNamed, candidate)
		}
	}
	sort.Slice(fullyNamed, func(left, right int) bool {
		if len(fullyNamed[left].symbol.nameParts) != len(fullyNamed[right].symbol.nameParts) {
			return len(fullyNamed[left].symbol.nameParts) > len(fullyNamed[right].symbol.nameParts)
		}
		if fullyNamed[left].score != fullyNamed[right].score {
			return fullyNamed[left].score > fullyNamed[right].score
		}
		return fullyNamed[left].id < fullyNamed[right].id
	})
	for _, candidate := range fullyNamed {
		id := candidate.symbol.symbol.Path + ":" + candidate.symbol.symbol.Name
		if _, exists := selected[id]; exists {
			continue
		}
		confident = append(confident, candidate)
		selected[id] = struct{}{}
		if len(confident) == min(limit, 10) {
			break
		}
	}
	return confident
}

// evalNameSupport is the query words a declaration's name parts carry, as
// written or by a prefix of four bytes or more in either direction.
func evalNameSupport(candidate evalSymbolCandidate, queryTerms map[string]struct{}) map[string]struct{} {
	support := make(map[string]struct{})
	for term := range queryTerms {
		if _, carried := candidate.symbol.name[term]; carried {
			support[term] = struct{}{}
			continue
		}
		for _, part := range candidate.symbol.nameParts {
			if len(part) >= 4 && len(term) >= 4 && (strings.HasPrefix(term, part) || strings.HasPrefix(part, term)) {
				support[term] = struct{}{}
				break
			}
		}
	}
	return support
}

func subsetOf(inner, outer map[string]struct{}) bool {
	for term := range inner {
		if _, carried := outer[term]; !carried {
			return false
		}
	}
	return true
}

// evalNamedByTask holds when the task writes the declaration's compact name
// (five bytes or more, so a short common word is not a name) or every one of a
// multi-part name's parts.
func evalNamedByTask(candidate evalSymbolCandidate, queryTerms map[string]struct{}, queryText string) bool {
	compactName := candidate.symbol.compactName
	if len(compactName) >= 5 && strings.Contains(compactText(queryText), compactName) {
		return true
	}
	if len(candidate.symbol.nameParts) < 2 {
		return false
	}
	for _, part := range candidate.symbol.nameParts {
		if intersectionCountSet(terms(part), queryTerms) == 0 {
			return false
		}
	}
	return true
}

// evalPruneAuxiliarySymbols removes same-path values that only inherit a
// callable or type's context vocabulary. A value named explicitly by the task
// remains eligible; otherwise the declaration that performs or defines the
// behavior is the smaller proof when both kinds occupy the same frontier.
func evalPruneAuxiliarySymbols(candidates []evalSymbolCandidate, queryText string) []evalSymbolCandidate {
	if len(candidates) < 2 {
		return candidates
	}
	hasBehavior := false
	for _, candidate := range candidates {
		switch candidate.symbol.symbol.Kind {
		case "class", "func", "interface", "method", "type":
			hasBehavior = true
		}
	}
	if !hasBehavior {
		return candidates
	}
	compactQuery := compactText(queryText)
	queryTerms := terms(queryText)
	result := make([]evalSymbolCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		kind := candidate.symbol.symbol.Kind
		behavior := kind == "class" || kind == "func" || kind == "interface" || kind == "method" || kind == "type"
		explicit := candidate.symbol.compactName != "" && strings.Contains(compactQuery, candidate.symbol.compactName)
		if !explicit && len(candidate.symbol.nameParts) != 0 {
			explicit = true
			for _, part := range candidate.symbol.nameParts {
				if intersectionCountSet(terms(part), queryTerms) == 0 {
					explicit = false
					break
				}
			}
		}
		if behavior || explicit {
			result = append(result, candidate)
		}
	}
	return result
}

func evalPathAllowed(candidatePath string, intent evalIntent) bool {
	if intent.id == "repository" {
		return true
	}
	if intent.id == "agent-tooling" {
		if candidatePath == "AGENTS.md" {
			return true
		}
		for _, prefix := range evalToolingPaths {
			if strings.HasPrefix(candidatePath, prefix) {
				return true
			}
		}
		return false
	}
	return projectOperationPath(candidatePath)
}

func difference(values, removed map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{})
	for value := range values {
		if !contains(removed, value) {
			result[value] = struct{}{}
		}
	}
	return result
}
func differenceCount(values []string, removed map[string]struct{}) int {
	count := 0
	for _, value := range values {
		if !contains(removed, value) {
			count++
		}
	}
	return count
}
func intersectionCount(values []string, right map[string]struct{}) int {
	count := 0
	for _, value := range values {
		if contains(right, value) {
			count++
		}
	}
	return count
}
func intersectionCountSet(left, right map[string]struct{}) int {
	count := 0
	for value := range left {
		if contains(right, value) {
			count++
		}
	}
	return count
}
func intersectionSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}
func stringIndex(values []string, wanted string) int {
	for index, value := range values {
		if value == wanted {
			return index
		}
	}
	return -1
}
func evalOrderedTermSet(value string) map[string]struct{} {
	return intersectionSet(evalOrderedTerms(value))
}

// evalQuerySupportIndex maps a rendered result identity to the query terms that
// earned it. Only candidates a ranker scored against the query appear here;
// results carried in behind another result -- a matched feature's
// implementation symbols -- are deliberately absent, because they are evidence
// about that feature and not about the query. Learned paths are also absent:
// advisory learning cannot decide whether the primary packet clears the floor.
func evalQuerySupportIndex(records []evalRecordCandidate, documents []evalDocumentCandidate, symbols []evalSymbolCandidate) map[string]map[string]struct{} {
	index := make(map[string]map[string]struct{}, len(records)+len(documents)+len(symbols))
	for _, candidate := range records {
		index[evalResultIdentity(candidate.result)] = candidate.support
	}
	for _, candidate := range documents {
		index[evalResultIdentity(candidate.result)] = candidate.support
	}
	for _, candidate := range symbols {
		index[evalResultIdentity(candidate.result)] = candidate.support
	}
	return index
}

func evalResultIdentity(result map[string]any) string {
	return stringValue(result["kind"]) + ":" + stringValue(result["id"])
}

// evalStrongestSupport is the widest support any single emitted result earned,
// counted in words of the query as written rather than in the derived terms
// terms() expands them into. The derived set carries plural stems and aliases,
// so counting it would let one query word ("leaves" -> "leaves", "leave") look
// like two independent matches.
//
// Basis for the floor this feeds. Every result Corvint emits is a lexical match,
// so a packet asserts "this repository answers your task" only as far as the
// task's own words were answered. The evidence for that assertion is the words
// a result matched -- never its score, which is an unnormalised sum of term
// counts, id bonuses and position bonuses and so is not comparable between two
// different queries. Two properties of the index make an unsupported packet
// indistinguishable from a supported one by score alone:
//
//   - Canonical ids are hyphenated English and terms() splits them, so one
//     ordinary word selects a specific feature: "how do I bake sourdough bread
//     at high altitude" matches feature:a11y-high-contrast on "high", and on
//     nothing else in the repository.
//   - featureImplementationCandidates then carries that feature's implementation
//     symbols into the packet, and those symbols outscore the feature that
//     admitted them. A one-word accident renders as a confident multi-result
//     packet with the highest scores in it belonging to results that matched no
//     query word at all.
//
// So the floor is per result, not over the packet's union: one word matching
// result A and a different word matching result B is two unrelated accidents,
// not one relevant answer. A result carried in behind another (a matched
// feature's implementations) has no entry in the support index and contributes
// nothing, which is the point -- it is evidence about that feature, not about
// this query.
//
// The bar is two words, and it is a floor rather than a filter: the packet is
// withdrawn only when NO result clears it. A genuinely relevant low-score
// result is never suppressed, because one cleared result keeps every other
// result in the packet. A query offering fewer than two words to match is held
// to what it offers, so a bare "hls" lookup still answers.
func evalStrongestSupport(results []map[string]any, index map[string]map[string]struct{}, ordered []string) int {
	widest := 0
	for _, result := range results {
		support, scored := index[evalResultIdentity(result)]
		if !scored {
			continue
		}
		widest = max(widest, evalSupportWidth(support, ordered))
	}
	return widest
}

// evalSupportWidth counts the query words, as written, that a result's support
// terms rest on: the measure GPK-V0-039 holds a packet to and GPK-V0-083 orders
// the primary packet by.
func evalSupportWidth(support map[string]struct{}, ordered []string) int {
	matched := 0
	for _, word := range ordered {
		if intersectionCountSet(terms(word), support) != 0 {
			matched++
		}
	}
	return matched
}

// evalRanked is the ranking pass of one query: every candidate class ranked
// and the packet it admits before trace learning and the relevance floor.
// EvalQuery and QueryRelevanceAbstention share it, so the two can only differ
// by what learning adds (TCP-V0-960).
type evalRanked struct {
	text, trimmed, queryText       string
	intent                         evalIntent
	queryTerms                     map[string]struct{}
	ordered                        []string
	records, competitive, admitted []evalRecordCandidate
	documents                      []evalDocumentCandidate
	confident, admittedConfident   []evalSymbolCandidate
	widerSymbols                   []evalSymbolCandidate
	results                        []map[string]any
}

func evalRank(index *Index, text string, limit int) (*evalRanked, error) {
	trimmed := TrimPythonSpace(text)
	queryText := pythonLower(trimmed)
	intent := evalInferIntent(queryText)
	queryTerms := evalRelevanceTerms(terms(trimmed), intent)
	ordered := evalOrderedTerms(trimmed)
	records, crossField := evalRankRecords(index, queryText, queryTerms, ordered, intent)
	documents, err := evalRankDocuments(index, queryText, queryTerms, intent)
	if err != nil {
		return nil, err
	}
	symbols, symbolTermFrequency := evalRankSymbols(index, queryText, queryTerms, intent)
	confident := evalConfidentSymbols(index, symbols, symbolTermFrequency, queryText, queryTerms, limit)
	// GPK-V0-040. Every `limit` cut inside evalConfidentSymbols is a prefix cut
	// of one sequence, and its own caps saturate at evalConfidentCeiling, so
	// re-selecting there yields the universe the fallback admitted and its
	// first `limit` entries are byte-identical to `confident`. `confident`
	// stays the narrowed list the support index and the relevance floor were
	// written against; only `results`, which the receipt denominates, reads the
	// wider one. A caller already at or above the ceiling gets neither a second
	// selection nor a different list.
	admittedConfident := confident
	if limit < evalConfidentCeiling {
		admittedConfident = evalConfidentSymbols(index, symbols, symbolTermFrequency, queryText, queryTerms, evalConfidentCeiling)
	}

	topRecordScore := 0
	if len(records) != 0 {
		topRecordScore = records[0].score
	}
	competitive := make([]evalRecordCandidate, 0, len(records))
	for _, candidate := range records {
		_, supported := crossField[candidate.record.Kind+":"+candidate.record.ID]
		if candidate.score*5 >= topRecordScore*3 || candidate.score >= 100 && supported {
			competitive = append(competitive, candidate)
		}
	}
	// GPK-V0-040. The ranking ceiling is the receipt's to apply, not the
	// selection's: applying it here and then handing `receipt` a list that can
	// no longer exceed `limit` makes `omitted_results` structurally zero and
	// lets the packet assert a completeness it never measured. `competitive`
	// stays the ceiling-narrowed list every stage below was written against --
	// featureLists, the negative-claim scan and the support index all read it,
	// and none of their behavior changes -- while `results` accumulates the
	// whole admitted universe and `receipt` truncates it. The first `limit`
	// entries are byte-identical to what this built before, because every
	// removed stop was a prefix cut of the same sequence.
	admitted := competitive
	if len(competitive) > limit {
		competitive = competitive[:limit]
	}

	// GPK-V0-083 (proposed, pending owner acceptance (V1-0266)). Records and
	// documents precede confident symbols only as answers to the task, and
	// GPK-V0-039 measures an answer by the
	// query words a result rests on, never by its score. A confident symbol
	// resting on more of those words than every record and document in the
	// packet leads it once the packet has stood on its own (see the floor
	// below); a symbol wider than none stays out, as before, and the records
	// and documents keep their own class order behind the placed symbols. An
	// empty record packet places no symbol: the confident selection below
	// answers instead.
	results := make([]map[string]any, 0, len(admitted)+2)
	widest := len(ordered)
	if len(admitted) != 0 || len(documents) != 0 {
		widest = 0
	}
	for _, candidate := range admitted {
		results = append(results, candidate.result)
		widest = max(widest, evalSupportWidth(candidate.support, ordered))
	}
	// The document slot cap was min(2, limit-len(results)), which is the class's
	// own cap of 2 narrowed by whatever the ceiling had already spent. Counting
	// the admitted universe at the narrowed cap would report a universe that
	// shrinks as `limit` shrinks, so the class cap alone is what admits here.
	for _, candidate := range documents[:min(len(documents), 2)] {
		results = append(results, candidate.result)
		widest = max(widest, evalSupportWidth(candidate.support, ordered))
	}
	widerSymbols := make([]evalSymbolCandidate, 0, len(admittedConfident))
	for _, candidate := range admittedConfident {
		if evalSupportWidth(candidate.support, ordered) > widest {
			widerSymbols = append(widerSymbols, candidate)
		}
	}
	seenSymbols := make(map[string]struct{})
	featureLists := make([][]map[string]any, 0, 2)
	for _, candidate := range competitive {
		if candidate.record.Kind != "feature" || candidate.score*4 < topRecordScore*3 {
			continue
		}
		// Three per feature is this class's own cap (GPK-V0-040, decision 0157);
		// min(limit, cap) would narrow it by the ceiling, and
		// featureImplementationCandidates truncates only after ranking, so the
		// wider call's prefix is the narrower call's whole answer. Admitting at
		// the class cap keeps the universe the same size whatever `limit` the
		// caller asked for.
		featureLists = append(featureLists, featureImplementationCandidates(index, candidate.record, 3))
		if len(featureLists) == 2 {
			break
		}
	}
	for offset := 0; ; offset++ {
		advanced := false
		for _, candidates := range featureLists {
			if offset >= len(candidates) {
				continue
			}
			advanced = true
			candidate := candidates[offset]
			id := stringValue(candidate["id"])
			if _, exists := seenSymbols[id]; !exists {
				results = append(results, candidate)
				seenSymbols[id] = struct{}{}
			}
		}
		if !advanced {
			break
		}
	}
	if len(results) == 0 && len(confident) != 0 {
		for _, candidate := range admittedConfident {
			id := stringValue(candidate.result["id"])
			if _, exists := seenSymbols[id]; exists {
				continue
			}
			results = append(results, candidate.result)
			seenSymbols[id] = struct{}{}
		}
	} else if len(documents) != 0 && len(competitive) == 0 {
		for _, candidate := range admittedConfident[:min(len(admittedConfident), 2)] {
			id := stringValue(candidate.result["id"])
			if _, exists := seenSymbols[id]; !exists {
				results = append(results, candidate.result)
				seenSymbols[id] = struct{}{}
			}
		}
	}

	return &evalRanked{
		text: text, trimmed: trimmed, queryText: queryText, intent: intent, queryTerms: queryTerms, ordered: ordered,
		records: records, competitive: competitive, admitted: admitted, documents: documents,
		confident: confident, admittedConfident: admittedConfident, widerSymbols: widerSymbols, results: results,
	}, nil
}

// relevanceFloor is GPK-V0-066/GPK-V0-093's floor over results, the packet
// as compiled so far: whether it fails the floor, and whether the confident
// symbols alone (fallback) pass it in its place.
func (ranked *evalRanked) relevanceFloor(index *Index, results []map[string]any, limit int) (map[string]map[string]struct{}, bool, bool, []map[string]any) {
	ordered, competitive, documents, confident := ranked.ordered, ranked.competitive, ranked.documents, ranked.confident
	text, admittedConfident := ranked.text, ranked.admittedConfident
	emitted := results[:min(len(results), limit)]
	floor := min(2, len(ordered))
	supportIndex := evalQuerySupportIndex(competitive, documents, confident)
	// GPK-V0-083 judges the floor over the packet as compiled without the
	// placed symbols: a symbol placed ahead of records that answer nothing
	// does not carry them past GPK-V0-066.
	// GPK-V0-093 scope admission: the floor's words must include one of the task's
	// discriminating words where the task has any (evalScope).
	scope := evalScope{index: index, text: text, ordered: ordered, floor: floor, records: make(map[string]struct{}, len(competitive))}
	for _, candidate := range competitive {
		scope.records[evalResultIdentity(candidate.result)] = struct{}{}
	}
	belowFloor := len(results) != 0 && !scope.supported(emitted, supportIndex)
	// GPK-V0-066. Records and documents take precedence over symbols only as
	// answers to the task. When the packet they built fails the floor, none of
	// them is one, so the packet is compiled as though no record or document
	// matched: the confident symbols alone, judged by the same floor. The
	// withdrawn records leave no tie state or nearest claim behind.
	fallback := make([]map[string]any, 0, len(admittedConfident)+3)
	for _, candidate := range admittedConfident {
		fallback = append(fallback, candidate.result)
	}
	substituted := belowFloor && scope.supported(fallback[:min(len(fallback), limit)], supportIndex)
	return supportIndex, belowFloor, substituted, fallback
}

// QueryRelevanceAbstention reports the abstention `corvint query` reaches on
// text through its ranking pass and relevance floor alone (TCP-V0-960):
// `no-relevant-candidates` when nothing ranks, `below-relevance-floor` when
// the ranked packet and its confident-symbol substitute both fail the floor,
// and "" when the floor admits a packet. It reads only the index: no trace
// learning, no Git observation, no stored state.
func QueryRelevanceAbstention(index *Index, text string, limit int) (string, error) {
	if limit < 1 || limit > maxLimit {
		return "", &Error{Message: fmt.Sprintf("limit must be an integer from 1 to %d", maxLimit)}
	}
	if text == "" || TrimPythonSpace(text) == "" || utf8.RuneCountInString(text) > maxQueryChars {
		return "", &Error{Message: "query text must be non-empty and within the query limit"}
	}
	ranked, err := evalRank(index, text, limit)
	if err != nil {
		return "", err
	}
	if len(ranked.results) == 0 {
		return "no-relevant-candidates", nil
	}
	if _, belowFloor, substituted, _ := ranked.relevanceFloor(index, ranked.results, limit); belowFloor && !substituted {
		return "below-relevance-floor", nil
	}
	return "", nil
}
