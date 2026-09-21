package contextindex

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxQueryChars is the task bound. Decision 0023 raised it from the oracle's
// 2,000 to 8,000 runes: a failure excerpt is a task, and the tokenisers are
// linear in it (DR-0016).
const maxQueryChars = 8_000

var (
	// stopWords are the words a task is phrased WITH rather than about: they
	// occur in every question and so can never be evidence that this repository
	// answers this one. The list already carried the interrogatives "how" and
	// "where"; the interrogative pronouns and determiners belong to the same
	// class and are here for the same reason, because leaving them in let a
	// question's own framing ("what is the best way to ...") supply the term
	// overlap that admits a result -- support the repository never gave.
	// "when" and "while" are deliberately absent: they are also subordinating
	// conjunctions that occur in scenario prose and comments, so unlike the
	// pronouns they do carry repository meaning.
	stopWords   = stringSet("a an and are as at be by code for from how in is it of on or that the this to what where which who whom whose why with feature find change")
	termAliases = map[string][]string{
		"argument": {"parameter"}, "arguments": {"parameter"},
		"generate": {"gen"}, "generated": {"gen"}, "generator": {"gen"},
		"maximum": {"max"}, "minimum": {"min"},
		"parameter": {"argument"}, "parameters": {"argument"},
		"sync": {"synchronous"}, "synchronous": {"sync"},
	}
	agentToolingTerms     = stringSet("agent agents corvint budget codex context embedding embeddings evaluator federation memory packet precision retrieval roadmap semantic task tasks token tokens tooling trace traces")
	agentToolingAnchors   = stringSet("agent agents corvint codex embedding embeddings evaluator federation tooling")
	projectOperationTerms = stringSet("contribute contributor gate gates instruction instructions orient orientation roadmap ticket tickets workflow workflows")
)

type instructionCandidate struct {
	score  int
	record Record
	result map[string]any
}

type instructionMatch struct {
	count, line int
	text        string
	overlap     []string
}

// QueryAuthorityStart compiles the deliberately narrow GPK-V0-028 task-start profile.
func QueryAuthorityStart(ctx context.Context, index *Index, text string, limit int) (map[string]any, error) {
	return QueryAuthorityStartBudget(ctx, index, text, limit, nil)
}

// QueryAuthorityStartBudget adds packet selection to the authority-start
// profile. The packet is the Python oracle's project-operations answer: the
// uniquely highest instruction result, a second ranked instruction document
// when the limit admits one, then at most three advisory learned-path
// candidates when the limit leaves room for them.
func QueryAuthorityStartBudget(ctx context.Context, index *Index, text string, limit int, budget *int) (map[string]any, error) {
	if err := ValidateQueryAuthorityStart(text, limit); err != nil {
		return nil, err
	}
	trimmed := TrimPythonSpace(text)
	queryText := pythonLower(trimmed)
	queryTerms := terms(queryText)
	intent := evalInferIntent(trimmed)
	candidates, err := selectQueryAuthority(index, queryText, queryTerms)
	if err != nil {
		return nil, err
	}
	learned, learning, err := evalLearnedCandidates(ctx, index, startHistoryObservation(ctx, index), trimmed, limit, 2, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	results := make([]map[string]any, 0, 5)
	for _, candidate := range candidates[:min(2, limit, len(candidates))] {
		results = append(results, candidate.result)
	}
	results = append(results, allowedLearned(learned, intent, min(3, limit-len(results)))...)
	contextReceipt, err := receipt(index, "query", map[string]any{"text": trimmed, "limit": limit}, results, limit, "")
	if err != nil {
		return nil, err
	}
	contextReceipt["learning"] = learning
	contextReceipt["intent"] = map[string]any{
		"id": intent.id, "confidence": intent.confidence, "matched_terms": stringsToAny(intent.matched),
	}
	contextReceipt["abstention"] = map[string]any{"active": false, "reason": "none"}
	return compileReceipt(contextReceipt, budget, index)
}

// allowedLearned keeps the learned-path candidates the intent admits, at most
// maximum of them, in the oracle's order.
func allowedLearned(learned []map[string]any, intent evalIntent, maximum int) []map[string]any {
	kept := make([]map[string]any, 0, len(learned))
	for _, candidate := range learned {
		if evalResultAllowed(candidate, intent) {
			kept = append(kept, candidate)
		}
	}
	return kept[:min(len(kept), max(0, maximum))]
}

// selectQueryAuthority returns the ranked instruction candidates once the
// uniquely highest one is the root AGENTS.md; any other authority is refused.
func selectQueryAuthority(index *Index, queryText string, queryTerms map[string]struct{}) ([]instructionCandidate, error) {
	candidates, err := instructionCandidates(index, queryText, queryTerms)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 || candidates[0].record.ID != "AGENTS.md" || (len(candidates) > 1 && candidates[0].score == candidates[1].score) {
		return nil, &Error{Code: "unsupported-query-authority", Message: "native Go authority-start query requires one uniquely highest-ranked root AGENTS.md"}
	}
	return candidates, nil
}

// ValidateQueryAuthorityStart rejects query profiles that need no repository
// evidence before an index or Git operation is started.
func ValidateQueryAuthorityStart(text string, limit int) error {
	trimmed, err := validateQueryTask(text)
	if err != nil {
		return err
	}
	if err := validateQueryLimit(limit); err != nil {
		return err
	}
	if intent, _ := normalizedQueryIntent(trimmed); intent != "project-operations" {
		return &Error{Code: "unsupported-query-intent", Message: "native Go query currently supports only project-operations task orientation"}
	}
	return nil
}

// ValidateQueryCommand classifies and validates the standalone query profiles
// before an adapter opens the repository. Project operations take the
// authority-start validator; repository and agent-tooling tasks take the
// EvalQuery limit. Every UTF-8 task the oracle accepts is accepted.
func ValidateQueryCommand(text string, limit int) (string, error) {
	trimmed, err := validateQueryTask(text)
	if err != nil {
		return "", err
	}
	intent, _ := normalizedQueryIntent(trimmed)
	if intent == "project-operations" {
		if err := ValidateQueryAuthorityStart(text, limit); err != nil {
			return "", err
		}
		return intent, nil
	}
	if err := validateQueryLimit(limit); err != nil {
		return "", err
	}
	return intent, nil
}

func validateQueryLimit(limit int) error {
	if limit < 1 || limit > maxLimit {
		return &Error{Message: fmt.Sprintf("limit must be an integer from 1 to %d", maxLimit)}
	}
	return nil
}

func validateQueryTask(text string) (string, error) {
	trimmed := TrimPythonSpace(text)
	if text == "" || trimmed == "" {
		return "", &Error{Message: "query text must be non-empty"}
	}
	if !utf8.ValidString(text) {
		return "", &Error{Code: "unsupported-query-task", Message: "native Go query requires a UTF-8 task"}
	}
	if utf8.RuneCountInString(text) > maxQueryChars {
		return "", &Error{Message: fmt.Sprintf("query text exceeds %d characters", maxQueryChars)}
	}
	return trimmed, nil
}

func queryIntent(text string, values map[string]struct{}) (string, []string) {
	matchedTooling := intersection(values, agentToolingTerms)
	anchors := make(map[string]struct{})
	for term := range values {
		if _, ok := agentToolingAnchors[term]; !ok {
			continue
		}
		if term == "agents" {
			term = "agent"
		} else if term == "embeddings" {
			term = "embedding"
		}
		anchors[term] = struct{}{}
	}
	if len(anchors) >= 2 || (len(anchors) == 1 && len(matchedTooling) >= 3) {
		return "agent-tooling", matchedTooling
	}
	operations := intersection(values, projectOperationTerms)
	if len(operations) >= 2 || (strings.Contains(text, "work queue") && len(operations) != 0) {
		return "project-operations", operations
	}
	return "repository", nil
}

func normalizedQueryIntent(text string) (string, []string) {
	normalized := pythonLower(TrimPythonSpace(text))
	return queryIntent(normalized, terms(normalized))
}

// QueryIntent exposes the existing classifier for bounded diagnostic metadata;
// it never retains the supplied task text.
func QueryIntent(text string) string {
	intent, _ := normalizedQueryIntent(text)
	return intent
}

func instructionCandidates(index *Index, queryText string, queryTerms map[string]struct{}) ([]instructionCandidate, error) {
	compactQuery := compactText(queryText)
	candidates := make([]instructionCandidate, 0)
	for _, record := range index.Documents {
		if record.Kind != "instructions" || !projectOperationPath(record.Path) {
			continue
		}
		source, ok := index.Sources[record.Path]
		if !ok {
			continue
		}
		if source.Mode != "100644" && source.Mode != "100755" {
			continue
		}
		body, valid, loaded := source.Text()
		if !loaded || !valid || strings.ContainsRune(body, '\r') {
			return nil, &Error{Code: "unsupported-query-authority", Message: "native Go authority-start query requires UTF-8 LF-only regular instruction authority"}
		}
		searchable := record.Path + " " + stringValue(record.Fields["title"]) + " " +
			stringValue(record.Fields["summary"]) + " " + stringValue(record.Fields["headings"]) + " " + body
		overlap := termOverlap(queryTerms, terms(searchable))
		compactTitle := compactText(stringValue(record.Fields["title"]))
		exactTitle := len(compactTitle) >= 4 && strings.Contains(compactQuery, compactTitle)
		if overlap < 1 && !exactTitle {
			continue
		}
		score := overlap*18 + 240
		if exactTitle {
			score += 80
		}
		status := pythonLower(stringValue(record.Fields["status"]))
		if status == "accepted" || status == "approved" || status == "current" || status == "active" || strings.HasPrefix(status, "partially-superseded-by:") {
			score += 20
		}
		result, err := instructionResult(index, record, source, score, queryTerms)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, instructionCandidate{score: score, record: record, result: result})
	}
	sort.Slice(candidates, func(left, right int) bool {
		if candidates[left].score != candidates[right].score {
			return candidates[left].score > candidates[right].score
		}
		return candidates[left].record.ID < candidates[right].record.ID
	})
	return candidates, nil
}

func instructionResult(index *Index, record Record, source Source, score int, queryTerms map[string]struct{}) (map[string]any, error) {
	matches := make([]instructionMatch, 0)
	body, _, _ := source.Text()
	for offset, line := range strings.Split(body, "\n") {
		number := offset + 1
		stripped := strings.TrimSpace(line)
		if stripped == "" || strings.HasPrefix(stripped, "```") || number == record.Line {
			continue
		}
		overlap := intersection(queryTerms, terms(stripped))
		if len(overlap) != 0 {
			matches = append(matches, instructionMatch{
				count: len(overlap), line: number, text: truncateRunes(stripped, 300), overlap: overlap,
			})
		}
	}
	selected := make([]instructionMatch, 0, 3)
	uncovered := cloneSet(queryTerms)
	for len(matches) != 0 && len(selected) < 3 {
		best := 0
		for index := 1; index < len(matches); index++ {
			if instructionMatchBetter(matches[index], matches[best], uncovered) {
				best = index
			}
		}
		selected = append(selected, matches[best])
		for _, term := range matches[best].overlap {
			delete(uncovered, term)
		}
		matches = append(matches[:best], matches[best+1:]...)
	}
	evidenceItems := make([]any, 0, maxEvidence)
	for _, item := range selected {
		evidenceItems = append(evidenceItems, evidence(record.Path, item.line, record.BlobHash,
			"project instruction matches: "+strings.Join(item.overlap, ", "), "authoritative", "project-instructions"))
	}
	selectedTexts := make([]string, len(selected))
	for index, item := range selected {
		selectedTexts[index] = item.text
	}
	selectedText := pythonLower(strings.Join(selectedTexts, "\n"))
	references := stringsField(record.Fields["references"])
	sort.Slice(references, func(left, right int) bool {
		leftMentioned := strings.Contains(selectedText, pythonLower(references[left]))
		rightMentioned := strings.Contains(selectedText, pythonLower(references[right]))
		if leftMentioned != rightMentioned {
			return leftMentioned
		}
		leftOverlap := termOverlap(queryTerms, terms(references[left]))
		rightOverlap := termOverlap(queryTerms, terms(references[right]))
		if leftOverlap != rightOverlap {
			return leftOverlap > rightOverlap
		}
		leftScript, rightScript := strings.HasPrefix(references[left], "script/"), strings.HasPrefix(references[right], "script/")
		if leftScript != rightScript {
			return leftScript
		}
		return references[left] < references[right]
	})
	includedReferences := make([]string, 0, maxEvidence-len(evidenceItems))
	for _, reference := range references {
		if len(evidenceItems) >= maxEvidence {
			break
		}
		referenced, ok := index.Sources[reference]
		if !ok {
			return nil, &Error{Code: "unsupported-query-authority", Message: "native Go authority-start query found an unavailable instruction reference"}
		}
		evidenceItems = append(evidenceItems, evidence(reference, 1, referenced.BlobHash,
			"project instruction references "+reference, "high", "instruction-reference"))
		includedReferences = append(includedReferences, reference)
	}
	return map[string]any{
		"kind": "instructions", "id": record.ID, "score": score,
		"title":   stringValue(record.Fields["title"]),
		"summary": truncateRunes(strings.Join(selectedTexts, " | "), 500),
		"status":  "binding", "references": includedReferences, "evidence": evidenceItems,
	}, nil
}

func instructionMatchBetter(left, right instructionMatch, uncovered map[string]struct{}) bool {
	leftNew, rightNew := 0, 0
	for _, term := range left.overlap {
		if _, ok := uncovered[term]; ok {
			leftNew++
		}
	}
	for _, term := range right.overlap {
		if _, ok := uncovered[term]; ok {
			rightNew++
		}
	}
	if leftNew != rightNew {
		return leftNew > rightNew
	}
	if left.count != right.count {
		return left.count > right.count
	}
	return left.line < right.line
}

func projectOperationPath(value string) bool {
	admitted := value == "AGENTS.md" || value == "CLAUDE.md" || value == "GEMINI.md" || value == "Makefile" ||
		strings.HasPrefix(value, ".github/workflows/") || strings.HasPrefix(value, "docs/agent-workflows") ||
		strings.HasPrefix(value, "docs/plans/") || strings.HasPrefix(value, "script/")
	if !admitted || documentKind(value) != "instructions" {
		return admitted
	}
	_, eligible := instructionRank(value)
	return eligible
}

func terms(text string) map[string]struct{} {
	result := make(map[string]struct{})
	collectTerms(text, result)
	return result
}

// collectTerms is the body of terms against a caller-owned map, so a caller
// that already holds part of the answer can add to it instead of building a
// second map and merging. terms is byte-compared against the frozen Python
// oracle; this split moves no line of that pipeline.
func collectTerms(text string, result map[string]struct{}) {
	text = pythonLower(camelSplit(text))
	text = strings.ReplaceAll(text, "_", "-")
	for _, raw := range asciiWords(text) {
		for _, token := range strings.Split(raw, "-") {
			if len(token) <= 1 || contains(stopWords, token) {
				continue
			}
			result[token] = struct{}{}
			if len(token) > 4 && strings.HasSuffix(token, "s") &&
				!strings.HasSuffix(token, "ss") && !strings.HasSuffix(token, "as") &&
				!strings.HasSuffix(token, "is") && !strings.HasSuffix(token, "us") {
				result[token[:len(token)-1]] = struct{}{}
			}
			for _, alias := range termAliases[token] {
				result[alias] = struct{}{}
			}
			if len(token) > 5 && strings.HasSuffix(token, "ify") {
				result[token[:len(token)-3]] = struct{}{}
			}
		}
		if len(raw) > 2 && !contains(stopWords, raw) {
			result[raw] = struct{}{}
		}
	}
}

// pythonLower lowercases as Python's str.lower does. The one unconditional
// special casing, U+0130 to "i" followed by U+0307, is applied before the
// simple mapping, so the tokenizer stays byte-compared with the oracle on
// every UTF-8 task.
func pythonLower(value string) string {
	return strings.ToLower(strings.ReplaceAll(value, "\u0130", "i\u0307"))
}

func camelSplit(value string) string {
	runes := []rune(value)
	var output strings.Builder
	for index, current := range runes {
		if index > 0 {
			previous := runes[index-1]
			nextLower := index+1 < len(runes) && asciiLower(runes[index+1])
			if (asciiLower(previous) || asciiDigit(previous)) && asciiUpper(current) || asciiUpper(previous) && asciiUpper(current) && nextLower {
				output.WriteByte('-')
			}
		}
		output.WriteRune(current)
	}
	return output.String()
}

func asciiWords(value string) []string {
	result := make([]string, 0)
	for index := 0; index < len(value); {
		if !asciiLetter(rune(value[index])) {
			index++
			continue
		}
		start := index
		index++
		for index < len(value) {
			character := rune(value[index])
			if !asciiLetter(character) && !asciiDigit(character) && character != '_' && character != '-' {
				break
			}
			index++
		}
		result = append(result, value[start:index])
	}
	return result
}

func compactText(value string) string {
	var output strings.Builder
	for _, character := range pythonLower(value) {
		if asciiLower(character) || asciiDigit(character) {
			output.WriteRune(character)
		}
	}
	return output.String()
}

func stringSet(value string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, item := range strings.Fields(value) {
		result[item] = struct{}{}
	}
	return result
}

func cloneSet(values map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for value := range values {
		result[value] = struct{}{}
	}
	return result
}

func intersection(left, right map[string]struct{}) []string {
	result := make([]string, 0)
	for value := range left {
		if _, ok := right[value]; ok {
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func termOverlap(left, right map[string]struct{}) int { return len(intersection(left, right)) }

func stringsToAny(values []string) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = value
	}
	return result
}

func asciiLetter(value rune) bool { return asciiLower(value) || asciiUpper(value) }
func asciiLower(value rune) bool  { return value >= 'a' && value <= 'z' }
func asciiUpper(value rune) bool  { return value >= 'A' && value <= 'Z' }
func asciiDigit(value rune) bool  { return value >= '0' && value <= '9' }

// termsMatching returns terms(text) restricted to keep, without ever
// materialising the terms text would have produced outside it. The one caller
// that needs it -- the 2000-character context window each ranked symbol carries
// (evalSymbolChunk.collect) -- consumes the set only as
// intersection(queryTerms, candidate.context) in evalRankSymbols, so a term the
// query never mentions cannot survive the intersection and need never be built.
//
// The streaming path walks the exact pipeline terms() materialises -- camelSplit's
// inserted hyphens, ToLower, '_' rewritten to '-', asciiWords, split on '-' --
// out of one stack word buffer, so it allocates only the surviving keys. It
// reads bytes where terms() reads runes, which is sound because every byte of a
// non-ASCII rune is itself non-ASCII: such a rune is a word separator before and
// after ToLower, and it can neither trigger nor satisfy a camelSplit boundary.
// The one case that breaks -- a rune that lowercases INTO ASCII and so joins two
// tokens the byte scan reads as separated -- routes to terms() itself.
//
// terms() itself is unchanged. It is byte-compared against the frozen Python
// oracle, so this is a second reader of the same grammar, never a replacement.
func termsMatching(text string, keep map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{})
	if len(keep) == 0 {
		return result
	}
	if !lowersOutsideASCII(text) {
		for _, term := range intersection(keep, terms(text)) {
			result[term] = struct{}{}
		}
		return result
	}
	var storage [256]byte
	for index := 0; index < len(text); {
		if !asciiLetter(rune(text[index])) {
			index++
			continue
		}
		start := index
		for index++; index < len(text) && wordByte(text[index]); index++ {
		}
		collectMatchingWord(result, keep, appendSplitWord(storage[:0], text, start, index))
	}
	return result
}

// lowersOutsideASCII reports whether every non-ASCII rune in text stays
// non-ASCII under ToLower, which is what lets the streaming path treat those
// bytes as separators. Unicode 16 has exactly two runes that do not -- U+0130
// lowers to 'i' and U+212A lowers to 'k' -- but the predicate asks the tables
// rather than naming them, so a future revision cannot silently widen the set.
// The scan decodes only where it must: ASCII bytes are advanced one at a time.
func lowersOutsideASCII(text string) bool {
	for index := 0; index < len(text); {
		if text[index] < utf8.RuneSelf {
			index++
			continue
		}
		character, size := utf8.DecodeRuneInString(text[index:])
		if unicode.ToLower(character) < utf8.RuneSelf {
			return false
		}
		index += size
	}
	return true
}

// wordByte reports the continuation class asciiWords consumes a word through.
func wordByte(value byte) bool {
	character := rune(value)
	return asciiLetter(character) || asciiDigit(character) || value == '_' || value == '-'
}

// appendSplitWord writes text[start:end) as asciiWords would have read it out of
// camelSplit's output: hyphens inserted at case boundaries, '_' rewritten, ASCII
// letters lowered. A boundary at start itself is dropped because the hyphen it
// inserts lands before the word and asciiWords skips it.
func appendSplitWord(buffer []byte, text string, start, end int) []byte {
	for index := start; index < end; index++ {
		if index > start && camelBoundary(text, index) {
			buffer = append(buffer, '-')
		}
		buffer = append(buffer, lowerWordByte(text[index]))
	}
	return buffer
}

func camelBoundary(text string, index int) bool {
	previous, current := rune(text[index-1]), rune(text[index])
	if !asciiUpper(current) {
		return false
	}
	if asciiLower(previous) || asciiDigit(previous) {
		return true
	}
	return asciiUpper(previous) && index+1 < len(text) && asciiLower(rune(text[index+1]))
}

func lowerWordByte(value byte) byte {
	if value == '_' {
		return '-'
	}
	if value >= 'A' && value <= 'Z' {
		return value + 'a' - 'A'
	}
	return value
}

// collectMatchingWord walks one asciiWords word the way terms() does: every
// hyphen-separated token and its derivations, then the whole word.
func collectMatchingWord(result, keep map[string]struct{}, word []byte) {
	for offset := 0; offset <= len(word); {
		end := offset
		for end < len(word) && word[end] != '-' {
			end++
		}
		collectMatchingToken(result, keep, word[offset:end])
		offset = end + 1
	}
	if _, stop := stopWords[string(word)]; len(word) > 2 && !stop {
		keepTerm(result, keep, word)
	}
}

func collectMatchingToken(result, keep map[string]struct{}, token []byte) {
	if len(token) <= 1 {
		return
	}
	if _, stop := stopWords[string(token)]; stop {
		return
	}
	keepTerm(result, keep, token)
	if pluralStem(token) {
		keepTerm(result, keep, token[:len(token)-1])
	}
	for _, alias := range termAliases[string(token)] {
		if _, ok := keep[alias]; ok {
			result[alias] = struct{}{}
		}
	}
	if len(token) > 5 && string(token[len(token)-3:]) == "ify" {
		keepTerm(result, keep, token[:len(token)-3])
	}
}

// pluralStem mirrors terms()'s singular rule: a token longer than four
// characters ending in a lone "s" that is not "ss", "as", "is" or "us".
func pluralStem(token []byte) bool {
	if len(token) <= 4 || token[len(token)-1] != 's' {
		return false
	}
	switch token[len(token)-2] {
	case 's', 'a', 'i', 'u':
		return false
	}
	return true
}

// keepTerm admits one derived term. Both guards index by string(value), which
// the compiler serves without allocating, so a key is only ever materialised
// once and only when the query asked for it.
func keepTerm(result, keep map[string]struct{}, value []byte) {
	if _, wanted := keep[string(value)]; !wanted {
		return
	}
	if _, seen := result[string(value)]; seen {
		return
	}
	result[string(value)] = struct{}{}
}

// QuerySnapshotAuthority selects immutable project authority only. Mutable
// traces and checkout-bracketed history are outside the explicit snapshot.
func QuerySnapshotAuthority(index *Index, text string) (map[string]any, error) {
	if err := ValidateQueryAuthorityStart(text, 1); err != nil {
		return nil, err
	}
	trimmed := TrimPythonSpace(text)
	candidates, err := selectQueryAuthority(index, pythonLower(trimmed), terms(pythonLower(trimmed)))
	if err != nil {
		return nil, err
	}
	results := []map[string]any{candidates[0].result}
	packet, err := receipt(index, "query", map[string]any{"text": trimmed, "limit": 1}, results, 1, "", "history and local traces are outside the immutable planning snapshot")
	if err != nil {
		return nil, err
	}
	packet["abstention"] = map[string]any{"active": false, "reason": "none"}
	return compileReceipt(packet, nil, index)
}
