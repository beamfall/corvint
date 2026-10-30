package contextindex

import (
	"strconv"
	"strings"
)

// Hard admission (GPK-V0-093). The broad query compiles in three stages:
// candidate generation (evalRankSymbols and the frontier paths), hard
// admission (this file), and ranking and budget (the round-robin, the
// dependency links, the limit cut and the receipt). Admission is a predicate
// on evidence, never a score: a candidate it refuses cannot be readmitted by
// any later score, and a candidate it admits still competes for rank and
// budget. Its two checks are identity and scope.
//
// Identity: on one frontier path the subjects are the declarations the task
// names (evalNamedByTask) and the declarations whose name support no
// same-path neighbour's name support strictly contains. Every other
// candidate on the path is a sibling, and a sibling is admitted only with a
// demonstrated relation to a subject:
//
//   - same declaration: one extent contains the other (a method and its
//     class);
//   - reference edge: the subject's own code, with comments and string
//     literals blanked, uses the sibling's name as an identifier, or the
//     sibling's code uses the subject's name and the sibling's name support
//     lies within the subject's (a wrapper over the same named concept);
//   - paired test: the sibling is named Test<Subject> or test_<subject>;
//   - authority link: one project-owned marker id has markers whose nearest
//     declarations are the subject and the sibling.
//
// Sharing a file, a context window or the task's vocabulary is not a
// relation. That replaces the 2026-10-01 window-word rule, which let
// AppendActiveHelp ride beside GetActiveHelpConfig whenever the sibling's
// window held one query word the subject's did not (decision 0307). A sibling
// whose source text is not loaded has no demonstrated relation and is not
// admitted (invariant 2).
//
// Scope: see evalScope.

// evalScope is the scope check on a compiled packet. The relevance floor
// (GPK-V0-039) holds a packet to two of the task's words on one result; a
// word that more than half of the indexed sources carry is evidence about the
// repository, not about the task, so it cannot carry a packet into scope on
// its own. A task word is uncommon when at most half of the sources carry it,
// and discriminating when it is uncommon and some source carries it. Where
// the task has uncommon words, one result must meet the floor with at least
// one discriminating word among the words it rests on. A task whose only
// uncommon words no source carries therefore abstains: every result rests
// wholly on repository vocabulary. "render command docs as JSON files" in
// cobra rests only on `command` and `files`, which most sources carry, while
// `render`, `docs` and `json` are present but rare: the floor alone published
// command.go:Command, and the scope check withdraws it. A task whose every
// word is common is held to the floor alone. The bar is one discriminating
// word, not the floor's two, because document frequency is coarse in a small
// repository, where a feature's own words reach half of the sources. The
// vocabulary is read only for a packet the floor already admits.
//
// A record (a project-ledger row in records) that meets the floor satisfies
// the scope check on its own: GPK-V0-093 leaves record admission unchanged,
// and a ledger row is project-owned authority (invariant 3), not repository
// vocabulary. In a repository of a few sources every word a feature carries
// can reach half of them, which would otherwise withdraw the feature the task
// names.
//
// A declaration the task names exactly satisfies the scope check the same way:
// the task writes its indexed path as one token and, outside it, its name as an
// identifier, and the result meets the floor. That is the task's own pointer into the
// repository, not shared vocabulary. "Locate Add in add.go." in a three-source
// repository, where `add` and `go` reach more than half of the sources and
// `locate` none, otherwise withdraws add.go:Add (V1-3052).
type evalScope struct {
	index          *Index
	text           string
	ordered        []string
	floor          int
	records        map[string]struct{}
	discriminating []string
	uncommon       bool
	read           bool
}

func (scope *evalScope) supported(results []map[string]any, supportIndex map[string]map[string]struct{}) bool {
	if evalStrongestSupport(results, supportIndex, scope.ordered) < scope.floor {
		return false
	}
	for _, result := range results {
		identity := evalResultIdentity(result)
		_, record := scope.records[identity]
		if (record || scope.namesExactly(result)) && evalSupportWidth(supportIndex[identity], scope.ordered) >= scope.floor {
			return true
		}
	}
	if !scope.read {
		scope.discriminating, scope.uncommon = evalDiscriminatingWords(scope.index, scope.ordered)
		scope.read = true
	}
	if !scope.uncommon {
		return true
	}
	for _, result := range results {
		support, scored := supportIndex[evalResultIdentity(result)]
		if scored && evalSupportWidth(support, scope.ordered) >= scope.floor && evalSupportWidth(support, scope.discriminating) != 0 {
			return true
		}
	}
	return false
}

// namesExactly holds for a symbol result whose indexed path the task writes as
// one token (surrounding quotes, trailing punctuation, a leading `./` and a
// `:suffix` aside) and whose name it uses as an identifier outside every
// path-like token, so neither that path's stem nor another path (`other/add.py`,
// `docs/Add.md`) names it.
func (scope *evalScope) namesExactly(result map[string]any) bool {
	if stringValue(result["kind"]) != "symbol" {
		return false
	}
	name := stringValue(result["name"])
	path, found := strings.CutSuffix(stringValue(result["id"]), ":"+name)
	if !found || path == "" {
		return false
	}
	if _, indexed := scope.index.Sources[path]; !indexed {
		return false
	}
	named := false
	words := strings.Fields(scope.text)
	for position, token := range words {
		trimmed := strings.TrimRight(strings.Trim(token, "`'\"()[]<>"), ".,;:!?")
		written, suffix, _ := strings.Cut(trimmed, ":")
		written = strings.TrimPrefix(written, "./")
		switch {
		case written == path:
			named = true
			words[position] = suffix
		case trimmed != name && evalPathLike(written):
			words[position] = ""
		}
	}
	return named && evalUsesIdentifier(strings.Join(words, " "), name)
}

// evalPathLike holds for a token written as a path: it has a separator, or it
// ends in a lowercase file extension (`add.py`, not `Server.Run`).
func evalPathLike(token string) bool {
	if strings.ContainsAny(token, "/\\") {
		return true
	}
	dot := strings.LastIndexByte(token, '.')
	if dot <= 0 || dot == len(token)-1 || len(token)-dot > 9 {
		return false
	}
	for _, value := range token[dot+1:] {
		if (value < 'a' || value > 'z') && (value < '0' || value > '9') {
			return false
		}
	}
	return true
}

// evalDiscriminatingWords keeps, in order, the task words whose source
// frequency in the index vocabulary (words and identifier terms) is nonzero
// and at most half of the indexed sources or exactly one source, and reports
// whether any task word is uncommon (so carried, or absent). It reads the term table
// without caching it on the index: the index may be shared by concurrent
// queries, so a table built here stays local to this call.
func evalDiscriminatingWords(index *Index, ordered []string) ([]string, bool) {
	table := index.Vocabulary
	if table == nil {
		table = index.buildVocabulary()
	}
	kept := make([]string, 0, len(ordered))
	uncommon := false
	for _, word := range ordered {
		frequency := 0
		for term := range terms(word) {
			words, identifiers := postingSources(&table.Words, term), postingSources(&table.Terms, term)
			frequency = max(frequency, len(unionSources(words, identifiers)))
		}
		// A word exactly one source carries identifies that source, never the
		// repository, even where that source is half or more of a tiny index.
		if frequency*2 <= len(table.Paths) || frequency == 1 {
			uncommon = true
			if frequency != 0 {
				kept = append(kept, word)
			}
		}
	}
	return kept, uncommon
}

// evalAdmitSiblings applies the identity check to one frontier path's
// candidates, keeping their order.
func evalAdmitSiblings(index *Index, candidates []evalSymbolCandidate, queryTerms map[string]struct{}, queryText string) []evalSymbolCandidate {
	if len(candidates) < 2 {
		return candidates
	}
	support := make([]map[string]struct{}, len(candidates))
	for position, candidate := range candidates {
		support[position] = evalNameSupport(candidate, queryTerms)
	}
	subject := make([]bool, len(candidates))
	allSubjects := true
	for position, candidate := range candidates {
		subject[position] = evalNamedByTask(candidate, queryTerms, queryText) || !evalStrictlyContained(position, candidates, support)
		allSubjects = allSubjects && subject[position]
	}
	if allSubjects {
		return candidates
	}
	relations := newEvalRelations(index, candidates[0].symbol.symbol.Path)
	result := make([]evalSymbolCandidate, 0, len(candidates))
	for position, candidate := range candidates {
		if subject[position] {
			result = append(result, candidate)
			continue
		}
		for other, peer := range candidates {
			if !subject[other] {
				continue
			}
			sameConcept := len(support[position]) != 0 && subsetOf(support[position], support[other])
			if relations.related(peer.symbol.symbol, candidate.symbol.symbol, sameConcept) {
				result = append(result, candidate)
				break
			}
		}
	}
	return result
}

// evalStrictlyContained reports whether another same-path candidate's name
// support strictly contains this one's.
func evalStrictlyContained(position int, candidates []evalSymbolCandidate, support []map[string]struct{}) bool {
	own := support[position]
	for other := range candidates {
		if other == position || candidates[other].symbol.symbol.Path != candidates[position].symbol.symbol.Path {
			continue
		}
		if len(own) < len(support[other]) && subsetOf(own, support[other]) {
			return true
		}
	}
	return false
}

// evalRelations answers the four demonstrated relations over one path.
type evalRelations struct {
	index   *Index
	path    string
	symbols []Symbol
	lines   []string
	code    []string
	markers map[string]map[string]struct{}
}

func newEvalRelations(index *Index, path string) *evalRelations {
	relations := &evalRelations{index: index, path: path}
	for _, symbol := range index.Symbols {
		if symbol.Path == path {
			relations.symbols = append(relations.symbols, symbol)
		}
	}
	if source, ok := index.Sources[path]; ok {
		if text, valid, loaded := source.Text(); loaded && valid {
			relations.lines = strings.Split(text, "\n")
			relations.code = strings.Split(evalBlankCommentsAndStrings(path, text), "\n")
		}
	}
	return relations
}

// related reports a demonstrated relation from subject to sibling.
// sameConcept says the sibling's name support lies within the subject's: a
// sibling whose own code calls such a subject is its wrapper (ensure_sync
// calling async_to_sync), while a caller that names another concept
// (as_view calling dispatch_request) is not admitted by the call.
func (relations *evalRelations) related(subject, sibling Symbol, sameConcept bool) bool {
	if len(relations.lines) == 0 {
		return false
	}
	subjectStart, subjectEnd := relations.extent(subject)
	siblingStart, siblingEnd := relations.extent(sibling)
	if subjectStart <= siblingStart && siblingEnd <= subjectEnd || siblingStart <= subjectStart && subjectEnd <= siblingEnd {
		return true
	}
	if evalPairedTest(subject.Name, sibling.Name) {
		return true
	}
	if evalUsesIdentifier(strings.Join(relations.code[subjectStart-1:subjectEnd], "\n"), sibling.Name) {
		return true
	}
	if sameConcept && evalUsesIdentifier(strings.Join(relations.code[siblingStart-1:siblingEnd], "\n"), subject.Name) {
		return true
	}
	return relations.markerLinked(subject, sibling)
}

// extent is the declaration's reported end when the grammar reports one, else
// the line before the path's next declaration, else the last line: the
// SymbolExtents rule, 1-based and inclusive, clamped to the source.
func (relations *evalRelations) extent(symbol Symbol) (int, int) {
	end := len(relations.lines)
	if symbol.EndLine > 0 {
		end = symbol.EndLine
	} else {
		for _, next := range relations.symbols {
			if next.Line > symbol.Line && next.Line-1 < end {
				end = next.Line - 1
			}
		}
	}
	start := min(max(symbol.Line, 1), len(relations.lines))
	return start, min(max(end, start), len(relations.lines))
}

// markerLinked holds when one marker id has markers on this path whose
// nearest declarations are the subject and the sibling.
func (relations *evalRelations) markerLinked(subject, sibling Symbol) bool {
	if relations.markers == nil {
		relations.markers = make(map[string]map[string]struct{})
		for key, markers := range relations.index.Markers {
			for _, marker := range markers {
				if marker.Path != relations.path {
					continue
				}
				if nearest, found := nearestMarkerSymbol(relations.symbols, marker); found {
					if relations.markers[key] == nil {
						relations.markers[key] = make(map[string]struct{})
					}
					relations.markers[key][evalSymbolKey(nearest)] = struct{}{}
				}
			}
		}
	}
	for _, nearest := range relations.markers {
		_, hasSubject := nearest[evalSymbolKey(subject)]
		_, hasSibling := nearest[evalSymbolKey(sibling)]
		if hasSubject && hasSibling {
			return true
		}
	}
	return false
}

func evalSymbolKey(symbol Symbol) string {
	return symbol.Name + "\x00" + strconv.Itoa(symbol.Line)
}

// evalPairedTest holds when the sibling is the subject's paired test by
// name: Test<Subject>, Test<Subject>_case, or test_<subject>.
func evalPairedTest(subject, sibling string) bool {
	if subject == "" {
		return false
	}
	if rest, ok := strings.CutPrefix(sibling, "Test"+subject); ok && (rest == "" || rest[0] == '_') {
		return true
	}
	rest, ok := strings.CutPrefix(sibling, "test_"+pythonLower(subject))
	return ok && (rest == "" || rest[0] == '_')
}

// evalUsesIdentifier holds when code uses name as a whole identifier.
func evalUsesIdentifier(code, name string) bool {
	if name == "" {
		return false
	}
	for offset := 0; ; {
		found := strings.Index(code[offset:], name)
		if found < 0 {
			return false
		}
		start, end := offset+found, offset+found+len(name)
		if (start == 0 || !evalIdentifierByte(code[start-1])) && (end == len(code) || !evalIdentifierByte(code[end])) {
			return true
		}
		offset = start + 1
	}
}

func evalIdentifierByte(value byte) bool {
	return value == '_' || value == '$' || value >= '0' && value <= '9' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= 0x80
}

// evalBlankCommentsAndStrings replaces comments and string literals with
// spaces, keeping every newline so line numbers hold. It is a lexical
// approximation, not a parser: Python reads `#` comments and single, double
// and triple quotes; every other language reads `//` and `/* */` comments,
// double quotes and backticks, and single quotes only in JavaScript and
// TypeScript sources, where a Rust lifetime or Go rune cannot be one.
func evalBlankCommentsAndStrings(path string, text string) string {
	python := strings.HasSuffix(path, ".py")
	singleQuoted := python || evalWebPath(path)
	out := []byte(text)
	blank := func(from, to int) {
		for position := from; position < to && position < len(out); position++ {
			if out[position] != '\n' {
				out[position] = ' '
			}
		}
	}
	for position := 0; position < len(text); {
		current := text[position]
		switch {
		case python && current == '#', !python && strings.HasPrefix(text[position:], "//"):
			end := strings.IndexByte(text[position:], '\n')
			if end < 0 {
				end = len(text) - position
			}
			blank(position, position+end)
			position += end
		case !python && strings.HasPrefix(text[position:], "/*"):
			end := strings.Index(text[position+2:], "*/")
			stop := len(text)
			if end >= 0 {
				stop = position + 2 + end + 2
			}
			blank(position, stop)
			position = stop
		case python && (strings.HasPrefix(text[position:], `"""`) || strings.HasPrefix(text[position:], "'''")):
			quote := text[position : position+3]
			end := strings.Index(text[position+3:], quote)
			stop := len(text)
			if end >= 0 {
				stop = position + 3 + end + 3
			}
			blank(position, stop)
			position = stop
		case current == '"' || current == '`' || current == '\'' && singleQuoted:
			stop := position + 1
			for stop < len(text) && text[stop] != current {
				if text[stop] == '\\' && current != '`' {
					stop++
				} else if text[stop] == '\n' && current != '`' {
					break
				}
				stop++
			}
			stop = min(stop+1, len(text))
			blank(position, stop)
			position = stop
		default:
			position++
		}
	}
	return string(out)
}
