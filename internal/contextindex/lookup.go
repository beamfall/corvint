package contextindex

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// lookup.go is the structural query surface of the `context` verb
// (TCP-V0-017, proposed): three read-only sub-modes over the compiled index
// so an agent iterates against the snapshot instead of grep. Every mode reads
// only the tables the packet reads (Symbols, the term table, the import
// graph, Sources), so an excluded path can never appear: it is absent from
// all of them.

const (
	// LookupDefaultLimit is the result cap when the invocation names none.
	LookupDefaultLimit = 20
	// maxLookupIdentifierBytes bounds one identifier or grep term.
	maxLookupIdentifierBytes = 256
	// lookupMatchingLinesCap is the most matching lines `grep` quotes per file.
	lookupMatchingLinesCap = 5
	// lookupLineBytesCap truncates a quoted line.
	lookupLineBytesCap = 240
	// LookupIdentifierErrorCode is the typed refusal for an identifier or term
	// that is empty or over maxLookupIdentifierBytes.
	LookupIdentifierErrorCode = "unsupported-context-lookup-identifier"
)

// validateLookupIdentifier refuses an empty or over-long identifier or term.
func validateLookupIdentifier(identifier string) error {
	if identifier == "" {
		return &Error{Code: LookupIdentifierErrorCode, Message: "context lookup identifier must be non-empty"}
	}
	if len(identifier) > maxLookupIdentifierBytes {
		return &Error{Code: LookupIdentifierErrorCode, Message: fmt.Sprintf("context lookup identifier exceeds %d bytes", maxLookupIdentifierBytes)}
	}
	return nil
}

func validateLookupLimit(limit int) error {
	if limit < 1 || limit > maxLimit {
		return &Error{Message: fmt.Sprintf("limit must be an integer from 1 to %d", maxLimit)}
	}
	return nil
}

// lookupEnvelope is the shared output shape: the packet's identity fields,
// the request echoed, and the same candidate accounting as `coverage`.
func lookupEnvelope(index *Index, mode string, request map[string]any, results []map[string]any, candidates, limit int) map[string]any {
	state := "READY"
	if len(results) == 0 {
		state = "NO_CANDIDATES"
	}
	rows := make([]any, 0, len(results))
	for _, result := range results {
		rows = append(rows, result)
	}
	request["limit"] = limit
	return map[string]any{
		"tool": "context", "mode": mode, "ok": true, "mutates": false, "schema_version": 1,
		"revision": index.Revision, "state": state, "request": request,
		"coverage": map[string]any{
			"candidates": candidates, "included_results": len(results),
			"omitted_results": maxInt(candidates-len(results), 0),
		},
		"results": rows,
	}
}

// LookupDefinitions lists the symbols defining identifier: exact-name matches
// first, then case-insensitive ones, each class ordered by rarity (fewest
// definers of that name first), then name, path, line.
func LookupDefinitions(index *Index, identifier string, limit int) (map[string]any, error) {
	if err := validateLookupLimit(limit); err != nil {
		return nil, err
	}
	if err := validateLookupIdentifier(identifier); err != nil {
		return nil, err
	}
	matched := make([]Symbol, 0)
	for _, symbol := range index.Symbols {
		if strings.EqualFold(symbol.Name, identifier) {
			matched = append(matched, symbol)
		}
	}
	definers := map[string]int{}
	for _, symbol := range matched {
		definers[symbol.Name]++
	}
	sort.SliceStable(matched, func(left, right int) bool {
		leftExact, rightExact := matched[left].Name == identifier, matched[right].Name == identifier
		if leftExact != rightExact {
			return leftExact
		}
		if definers[matched[left].Name] != definers[matched[right].Name] {
			return definers[matched[left].Name] < definers[matched[right].Name]
		}
		if matched[left].Name != matched[right].Name {
			return matched[left].Name < matched[right].Name
		}
		if matched[left].Path != matched[right].Path {
			return matched[left].Path < matched[right].Path
		}
		return matched[left].Line < matched[right].Line
	})
	results := make([]map[string]any, 0, minInt(limit, len(matched)))
	for _, symbol := range matched[:minInt(limit, len(matched))] {
		match := "exact"
		if symbol.Name != identifier {
			match = "case-insensitive"
		}
		extent := fmt.Sprintf("lines %d-%d", symbol.Line, symbol.EndLine)
		if symbol.EndLine == 0 {
			extent = fmt.Sprintf("line %d, extent not reported", symbol.Line)
		}
		reason := fmt.Sprintf("defines `%s` (%s) at %s; %s match; %d definers", symbol.Name, symbol.Kind, extent, match, definers[symbol.Name])
		results = append(results, map[string]any{
			"kind": "definition", "id": symbol.Path, "name": symbol.Name, "symbol_kind": symbol.Kind,
			"line": symbol.Line, "end_line": symbol.EndLine, "match": match,
			"evidence": []any{evidence(symbol.Path, symbol.Line, index.Sources[symbol.Path].BlobHash, reason, "high", SyntaxAuthority)},
		})
	}
	return lookupEnvelope(index, "defs", map[string]any{"identifier": identifier}, results, len(matched), limit), nil
}

// referenceHit is one file naming or importing the identifier's definer.
type referenceHit struct {
	path, imported string
	count, line    int
}

// referenceTier orders a file that both names the identifier and imports its
// definer before one that only names it, before one that only imports.
func (hit referenceHit) tier() int {
	switch {
	case hit.count > 0 && hit.imported != "":
		return 0
	case hit.count > 0:
		return 1
	}
	return 2
}

// LookupReferences lists the files that name identifier as a whole word
// (the identifier vocabulary, as written) or import a file defining it (the
// reverse-import rules of `impact`), with whole-word counts, excluding the
// defining files themselves.
func LookupReferences(index *Index, identifier string, limit int) (map[string]any, error) {
	if err := validateLookupLimit(limit); err != nil {
		return nil, err
	}
	if err := validateLookupIdentifier(identifier); err != nil {
		return nil, err
	}
	definers := map[string]struct{}{}
	for _, symbol := range index.Symbols {
		if symbol.Name == identifier {
			definers[symbol.Path] = struct{}{}
		}
	}
	definerPaths := make([]string, 0, len(definers))
	for path := range definers {
		definerPaths = append(definerPaths, path)
	}
	sort.Strings(definerPaths)
	hits := map[string]*referenceHit{}
	hitOf := func(path string) *referenceHit {
		if hit, ok := hits[path]; ok {
			return hit
		}
		hit := &referenceHit{path: path}
		hits[path] = hit
		return hit
	}
	table := index.vocabulary()
	if low, high, ok := table.Words.find(identifier); ok {
		for position := low; position < high; position++ {
			path := table.Paths[table.Words.Sources[position]]
			if _, defining := definers[path]; defining {
				continue
			}
			text, _, _ := index.Sources[path].Text()
			hit := hitOf(path)
			hit.count, hit.line = countWholeWord(text, identifier)
		}
	}
	for _, definer := range definerPaths {
		for _, edge := range reverseImporters(index, definer) {
			if _, defining := definers[edge.path]; defining {
				continue
			}
			hit := hitOf(edge.path)
			if hit.imported == "" {
				hit.imported = edge.imported
			}
		}
	}
	ordered := make([]*referenceHit, 0, len(hits))
	for _, hit := range hits {
		ordered = append(ordered, hit)
	}
	sort.Slice(ordered, func(left, right int) bool {
		if ordered[left].tier() != ordered[right].tier() {
			return ordered[left].tier() < ordered[right].tier()
		}
		if ordered[left].count != ordered[right].count {
			return ordered[left].count > ordered[right].count
		}
		return ordered[left].path < ordered[right].path
	})
	results := make([]map[string]any, 0, minInt(limit, len(ordered)))
	for _, hit := range ordered[:minInt(limit, len(ordered))] {
		results = append(results, referenceResult(index, identifier, hit))
	}
	request := map[string]any{"identifier": identifier, "definers": stringsAny(definerPaths)}
	return lookupEnvelope(index, "refs", request, results, len(ordered), limit), nil
}

func referenceResult(index *Index, identifier string, hit *referenceHit) map[string]any {
	parts := make([]string, 0, 2)
	line := hit.line
	if hit.count > 0 {
		parts = append(parts, fmt.Sprintf("names `%s` as a whole word %d times, first at line %d", identifier, hit.count, hit.line))
	}
	if hit.imported != "" {
		parts = append(parts, fmt.Sprintf("imports %s, which defines it", hit.imported))
		if line == 0 {
			// An unresolved import line stays 0: the row then claims no line.
			line, _ = importEvidenceLine(index.Sources[hit.path], hit.imported)
		}
	}
	confidence := "medium"
	if hit.tier() == 0 {
		confidence = "high"
	}
	// No line in the source supports the row, so it makes no line claim and
	// cannot carry the confidence of one that does.
	if line == 0 {
		confidence = "low"
	}
	return map[string]any{
		"kind": "reference", "id": hit.path, "count": hit.count, "line": line,
		"imports_definer": hit.imported != "",
		"evidence":        []any{evidence(hit.path, line, index.Sources[hit.path].BlobHash, strings.Join(parts, "; "), confidence, SyntaxAuthority)},
	}
}

// countWholeWord counts the occurrences of word in text bounded by non-word
// bytes on both sides, and reports the 1-based line of the first.
func countWholeWord(text, word string) (int, int) {
	count, firstLine, line, from := 0, 0, 1, 0
	for {
		at := strings.Index(text[from:], word)
		if at < 0 {
			return count, firstLine
		}
		start := from + at
		end := start + len(word)
		line += strings.Count(text[from:start], "\n")
		from = start + 1
		if start > 0 && isWordByte(text[start-1]) {
			continue
		}
		if end < len(text) && isWordByte(text[end]) {
			continue
		}
		count++
		if firstLine == 0 {
			firstLine = line
		}
	}
}

// grepHit is one source scored against the grep terms.
type grepHit struct {
	source                int
	distinct, occurrences int
	score                 float64
	// lastToken is the 1-based position of the last token credited, so a
	// token matched in both the body and the path counts once in distinct.
	lastToken int
}

// LookupGrep ranks the sources holding the given terms by BM25 over the body
// and path term fields (TCP-V0-014's formula without the whole-identifier
// field or the stop list: the caller chose the terms) and quotes the matching
// lines of each listed source. Terms are matched as whole tokens of the term
// table; substring and regular-expression matching is the n-gram lane's scope.
func LookupGrep(index *Index, terms []string, limit int) (map[string]any, error) {
	if err := validateLookupLimit(limit); err != nil {
		return nil, err
	}
	if len(terms) == 0 {
		return nil, validateLookupIdentifier("")
	}
	for _, term := range terms {
		if err := validateLookupIdentifier(term); err != nil {
			return nil, err
		}
	}
	tokens := lexicalTerms(strings.Join(terms, " "))
	hits := grepScores(index.vocabulary(), tokens)
	table := index.vocabulary()
	sort.Slice(hits, func(left, right int) bool {
		if hits[left].score != hits[right].score {
			return hits[left].score > hits[right].score
		}
		if hits[left].distinct != hits[right].distinct {
			return hits[left].distinct > hits[right].distinct
		}
		return table.Paths[hits[left].source] < table.Paths[hits[right].source]
	})
	results := make([]map[string]any, 0, minInt(limit, len(hits)))
	for _, hit := range hits[:minInt(limit, len(hits))] {
		results = append(results, grepResult(index, table.Paths[hit.source], tokens, hit))
	}
	request := map[string]any{"terms": stringsAny(terms), "tokens": stringsAny(tokens)}
	return lookupEnvelope(index, "grep", request, results, len(hits), limit), nil
}

// grepScores is one posting walk per token over the body and path fields,
// scored as lexicalRows scores them.
func grepScores(table *TermTable, tokens []string) []grepHit {
	const k1, b = 1.2, 0.3
	lengths, average := table.documentLengths()
	corpus := float64(len(table.Paths))
	if average == 0 {
		average = 1
	}
	idfOf := func(postings int) float64 {
		return math.Log(1 + (corpus-float64(postings)+0.5)/(float64(postings)+0.5))
	}
	scored := map[uint32]*grepHit{}
	credit := func(token int, source uint32, occurrences int, gain float64) {
		hit, ok := scored[source]
		if !ok {
			hit = &grepHit{source: int(source)}
			scored[source] = hit
		}
		if hit.lastToken != token {
			hit.distinct++
			hit.lastToken = token
		}
		hit.occurrences += occurrences
		hit.score += gain
	}
	for tokenIndex, token := range tokens {
		if low, high, ok := table.Terms.find(token); ok {
			idf := idfOf(high - low)
			for position := low; position < high; position++ {
				source := table.Terms.Sources[position]
				tf := float64(table.Terms.Counts[position])
				norm := k1 * (1 - b + b*float64(lengths[source])/average)
				credit(tokenIndex+1, source, int(table.Terms.Counts[position]), idf*tf*(k1+1)/(tf+norm))
			}
		}
		if low, high, ok := table.PathTerms.find(token); ok {
			idf := idfOf(high - low)
			for position := low; position < high; position++ {
				credit(tokenIndex+1, table.PathTerms.Sources[position], 0, idf*(k1+1)/(1+k1))
			}
		}
	}
	hits := make([]grepHit, 0, len(scored))
	for _, hit := range scored {
		hits = append(hits, *hit)
	}
	return hits
}

func grepResult(index *Index, path string, tokens []string, hit grepHit) map[string]any {
	text, _, _ := index.Sources[path].Text()
	lines := matchingLines(text, tokens)
	// A hit no quoted line backs claims no line (TCP-V0-017(c)): line 0.
	line := 0
	if len(lines) > 0 {
		line = lines[0]["line"].(int)
	}
	rows := make([]any, 0, len(lines))
	for _, item := range lines {
		rows = append(rows, item)
	}
	score := math.Round(hit.score*100) / 100
	reason := fmt.Sprintf("%d of %d terms, %d occurrences; bm25 %.2f", hit.distinct, len(tokens), hit.occurrences, score)
	if hit.occurrences == 0 {
		reason += "; path-only match, no line"
	}
	return map[string]any{
		"kind": "lexical", "id": path, "distinct": hit.distinct, "occurrences": hit.occurrences,
		"bm25": score, "lines": rows,
		"evidence": []any{evidence(path, line, index.Sources[path].BlobHash, reason, "low", "vocabulary")},
	}
}

// matchingLines quotes the first lookupMatchingLinesCap lines whose tokens
// (countTerms' rule, the one the body postings were built with) include a
// grep token.
func matchingLines(text string, tokens []string) []map[string]any {
	wanted := stringSetOf(tokens)
	lines := make([]map[string]any, 0, lookupMatchingLinesCap)
	for number, line := range strings.SplitAfter(text, "\n") {
		if len(lines) == lookupMatchingLinesCap {
			break
		}
		matched := lineTokens(line, wanted)
		if len(matched) == 0 {
			continue
		}
		quoted := strings.TrimRight(line, "\r\n")
		if len(quoted) > lookupLineBytesCap {
			quoted = quoted[:lookupLineBytesCap]
		}
		lines = append(lines, map[string]any{"line": number + 1, "text": quoted, "terms": stringsAny(matched)})
	}
	return lines
}

func lineTokens(line string, wanted map[string]struct{}) []string {
	matched := make([]string, 0)
	for term := range countTerms(line) {
		if _, ok := wanted[term]; ok {
			matched = append(matched, term)
		}
	}
	sort.Strings(matched)
	return matched
}

func stringsAny(values []string) []any {
	result := make([]any, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	return result
}
