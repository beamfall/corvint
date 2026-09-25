package contextindex

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// TaskContext compiles the task-context packet (task-context-packet-v0): the
// files an agent must read to act on one task, drawn from sources a lexical
// listing cannot express, with the task's own subject path kept out of the
// results. Every row carries one evidence line naming the relation that
// admitted it. The packet is read-only and Go-only; no oracle speaks it.
func TaskContext(ctx context.Context, index *Index, task, subject string, limit int) (map[string]any, error) {
	return taskContext(ctx, index, task, subject, limit, nil)
}

func taskContext(ctx context.Context, index *Index, task, subject string, limit int, weights SlotWeights) (map[string]any, error) {
	if limit < 1 || limit > maxLimit {
		return nil, &Error{Message: fmt.Sprintf("limit must be an integer from 1 to %d", maxLimit)}
	}
	task = strings.TrimSpace(task)
	if task == "" {
		return nil, &Error{Message: "task text must be non-empty"}
	}
	if len(task) > maxTaskContextChars {
		return nil, &Error{Message: fmt.Sprintf("task text exceeds %d characters", maxTaskContextChars)}
	}
	if subject != "" {
		cleaned, err := cleanImpactPath(subject)
		if err != nil {
			return nil, err
		}
		subject = cleaned
		if !trackedPath(index, subject) {
			return nil, &Error{Message: fmt.Sprintf("subject path is not tracked at revision %s: %s", index.Revision, subject)}
		}
	}
	compiler := newTaskContextCompiler(index, task, subject)
	compiler.slotWeights = weights
	compiler.recency = startContextRecency(ctx, index)
	if subject != "" {
		compiler.startHistory(ctx)
	}
	rows := compiler.compile(limit)
	if compiler.historyErr != nil {
		return nil, compiler.historyErr
	}
	compiler.answerability = compiler.answer()
	compiler.answerability.relations = relationRows(rows)
	if compiler.answerability.unsupported() {
		compiler.answerability.nearest = compiler.withheldClaims(rows)
		rows = compiler.reservedOnly(rows)
	}
	packet := compiler.packet(rows, limit)
	compiler.attachSpans(packet, rows)
	if err := index.SnapshotRefusal(); err != nil {
		return nil, err
	}
	return packet, nil
}

// contextRow is one admitted file with the relation that admitted it.
type contextRow struct {
	kind, path, summary, reason, confidence, authority string
	score, line                                        int
}

type taskIdentifier struct {
	name   string
	weight int
}

type taskContextCompiler struct {
	index        *Index
	task         string
	subject      string
	identifiers  []taskIdentifier
	terms        []string
	chosen       map[string]struct{}
	support      map[string][]string
	history      []historyEntry
	historyErr   error
	historyReady chan struct{}
	evidence     []int
	admitted     int
	// candidates records, per relation, every path that relation's generator
	// materialised: the denominator TCP-V0-011's `withheld` is measured from.
	// A relation absent here never ran.
	candidates map[string][]string
	// relationState is TCP-V0-011's per-relation `state`.
	relationState map[string]string
	// reserved is the governing and spec-mentioned rows (TCP-V0-008/009) in
	// their fixed order, before the final truncation.
	reserved []contextRow
	// promoted maps a path a reserved row took over to the relation of the
	// ordinary row it removed (TCP-V0-003's promote-in-place rule).
	promoted map[string]string
	// answerability is TCP-V0-016's conjunction verdict, computed after the
	// slots so it can withhold every row they admitted.
	answerability               answerability
	slotOmitted, truncated      bool
	historyFull, cochangeCapped bool
	// lexical caches the scored posting walk (TCP-V0-014) across the test
	// slot and the lexical fill.
	lexical       []lexicalHit
	selectedTerms *contextTermSelection
	// anchors is TCP-V0-022's verbatim literal field, empty unless
	// `CORVINT_CONTEXT_ANCHORS=on`.
	anchors []taskAnchor
	// slotWeights is an admitted learned trace's relation order (LTA-V0-011).
	slotWeights SlotWeights
	// recency is TCP-V0-035..038's history reading, nil unless
	// `CORVINT_CONTEXT_RECENCY=on`.
	recency *contextRecency
	// roles is TCP-V0-040's role-line field, off unless
	// `CORVINT_CONTEXT_ROLES=on`.
	roles bool
	// graphRanking is TCP-V0-034's opt-in, set by `CORVINT_CONTEXT_GRAPH=on`.
	graphRanking bool
}

// startHistory reads the co-change history beside the slots that do not need
// it (pair, mentioned, definition, reverse-import); awaitHistory joins it
// before the cochange slot. The Git call costs about 25 ms that would
// otherwise sit in series with the whole compile.
func (compiler *taskContextCompiler) startHistory(ctx context.Context) {
	compiler.historyReady = make(chan struct{})
	go func() {
		defer close(compiler.historyReady)
		compiler.history, compiler.historyErr = readCoChangeHistory(ctx, compiler.index)
	}()
}

func (compiler *taskContextCompiler) awaitHistory() {
	if compiler.historyReady != nil {
		<-compiler.historyReady
	}
}

const (
	// maxTaskContextChars bounds the task: a review comment with its diff hunk
	// is the intended input and can run past the query bound (TCP-V0-002).
	maxTaskContextChars = 32_000
	contextPairCap      = 3
	contextMentionCap   = 3
	contextSymbolCap    = 3
	contextImporterCap  = 3
	contextReferenceCap = 3
	// A reference row's score is its summed rarity above 600, ordered before
	// the cap and clamped below the definition slot's 800 for display.
	contextReferenceMaxScore = 799
	contextCochangeCap       = 5
	contextSiblingCap        = 3
	// contextTestCap reserves one slot for a test↔code row (TCP-V0-015).
	contextTestCap = 1
	// contextTestCorroborated bounds the per-anchor candidates whose text or
	// import edge is read to corroborate the cheaper signals.
	contextTestCorroborated = 10
	// contextTestMentionPostings bounds the identifier postings the mention
	// signal walks: wider than contextMaxDefiners because a test names its
	// package's common types too, and rarity still orders the credit.
	contextTestMentionPostings = 500
	// The co-change slot ignores commits touching more paths than a cap that
	// tightens with repository age: a young history (up to
	// contextCochangeYoungCommits non-merge commits) keeps commits up to
	// contextCochangeYoungCap paths, a history filling the read window is held
	// to contextCochangeMatureCap, and the cap is linear between (decision 0025).
	contextCochangeYoungCommits = 50
	contextCochangeYoungCap     = 50
	contextCochangeMatureCap    = 8
	contextMaxDefiners          = 50
	contextMaxBytes             = 1 << 20
	contextDocumentationHead    = 5
	contextDocumentationQuota   = 2
	// contextSpecMentionCap bounds the reserved spec rows (TCP-V0-009).
	contextSpecMentionCap = 3
	// contextRoutedCap bounds the reserved instruction-routed rows, and a
	// passage of the governing file routes only when it shares at least
	// contextRoutedMinTerms distinct task terms whose body idf reaches
	// contextRoutedMinIDF, so words most sources use never count (TCP-V0-047).
	contextRoutedCap      = 2
	contextRoutedMinTerms = 2
	contextRoutedMinIDF   = 2.0
)

// The two reserved relations (TCP-V0-008/009) and the fixed relation order
// TCP-V0-011's receipt members are sorted in: TCP-V0-004's slot order with the
// reserved relations first.
const (
	governingRelation         = "governing"
	specMentionedRelation     = "spec-mentioned"
	instructionRoutedRelation = "instruction-routed"
)

var contextRelationOrder = []string{
	governingRelation, specMentionedRelation, instructionRoutedRelation, "pair", "mentioned", "definition",
	"reverse-import", "reference", "cochange", "sibling", "test", "lexical", "documentation",
}

var (
	contextBacktick   = regexp.MustCompile("`([A-Za-z_][A-Za-z0-9_.]*)`")
	contextIdentifier = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]{2,}`)
	contextToken      = regexp.MustCompile(`[A-Za-z0-9]+`)
	contextCamel      = regexp.MustCompile(`([a-z])([A-Z])`)
	contextEscape     = regexp.MustCompile(`\\+[ntr"\\]`)
	contextBackquoted = regexp.MustCompile("`([^`\n]+)`")
	contextListItem   = regexp.MustCompile(`^[ \t]*(?:[-*+]|[0-9]+\.)[ \t]`)
	contextFence      = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})(.*)$")
	contextJSONKey    = regexp.MustCompile(`"[A-Za-z0-9_]+":`)
)

func newTaskContextCompiler(index *Index, task, subject string) *taskContextCompiler {
	return configureContextGraph(configureContextRoles(configureContextAnchors(configureContextTerms(&taskContextCompiler{
		index:         index,
		task:          task,
		subject:       subject,
		identifiers:   taskIdentifiers(task),
		terms:         taskLexicalTerms(task),
		chosen:        map[string]struct{}{},
		support:       map[string][]string{},
		candidates:    map[string][]string{},
		relationState: map[string]string{},
		promoted:      map[string]string{},
	}))))
}

// compile runs the slots in evidence order and fills the remainder lexically.
// A subject enables the change-shape slots; without one the packet is the
// retrieval shape.
func (compiler *taskContextCompiler) compile(limit int) []contextRow {
	rows := make([]contextRow, 0, limit)
	compiler.markRan("mentioned", "definition", "lexical", "documentation")
	if compiler.subject != "" {
		compiler.markRan("pair", "reverse-import", "reference", "cochange", "sibling")
		compiler.markSubjectSymbols()
		rows = compiler.takeSlot(rows, compiler.pairRows(compiler.subject), contextPairCap)
	} else {
		compiler.markState("subject-absent", "pair", "reverse-import", "reference", "cochange", "sibling")
	}
	frames, frameActive := compiler.frameRelationRows()
	if frameActive {
		rows = compiler.takeSlot(rows, frames, contextMentionCap)
	}
	rows = compiler.takeSlot(rows, compiler.mentionRows(), contextMentionCap)
	rows = compiler.takeSlot(rows, compiler.symbolRows(), contextSymbolCap)
	if compiler.subject != "" {
		rows = compiler.takeSlot(rows, compiler.importerRows(), contextImporterCap)
		rows = compiler.takeSlot(rows, compiler.referenceRows(), contextReferenceCap)
		compiler.awaitHistory()
		if compiler.historyErr != nil {
			return nil
		}
		compiler.historyFull = len(compiler.history) >= maxHistoryCommits
		if len(compiler.history) == 0 {
			compiler.markState("empty-history", "cochange")
		}
		rows = compiler.takeSlot(rows, compiler.recencyCochange(compiler.cochangeRows()), contextCochangeCap)
		rows = compiler.takeSlot(rows, compiler.siblingRows(), contextSiblingCap)
	}
	if compiler.subject == "" && !frameActive {
		compiler.markRan("test")
		rows = compiler.takeSlot(rows, compiler.testRows(compiler.testAnchors(rows, limit)), contextTestCap)
	}
	rows = compiler.takeSlot(rows, compiler.recencyLexical(compiler.lexicalRows(len(rows))), limit)
	rows = orderBySlotWeight(rows, compiler.slotWeights)
	rows = compiler.corroborate(rows)
	rows = compiler.reserve(rows)
	rows = compiler.placeGraphRows(rows, limit)
	// `candidates` is the distinct paths the slots admitted (TCP-V0-006); a
	// row a slot cap held back is `withheld`, not a candidate.
	compiler.admitted = len(rows)
	if len(rows) > limit {
		rows, compiler.truncated = rows[:limit], true
	}
	return rows
}

// markRan records that a relation's generator ran with nothing withheld yet;
// markState pins a TCP-V0-011 state the generator itself cannot report.
func (compiler *taskContextCompiler) markRan(relations ...string) {
	for _, relation := range relations {
		compiler.relationState[relation] = "examined"
		if compiler.candidates[relation] == nil {
			compiler.candidates[relation] = []string{}
		}
	}
}

func (compiler *taskContextCompiler) markState(state string, relations ...string) {
	for _, relation := range relations {
		compiler.relationState[relation] = state
	}
}

// markSubjectSymbols discloses that the `reference` slot read an incomplete
// symbol table for the subject (TCP-V0-011): a subject whose symbols were
// dropped otherwise reports `examined` with nothing withheld, a silent loss
// (AGENTS.md invariant 2).
func (compiler *taskContextCompiler) markSubjectSymbols() {
	if compiler.subjectSymbolsIncomplete() {
		compiler.markState("subject-symbols-incomplete", "reference")
	}
}

// subjectSymbolsIncomplete reports whether the index dropped some or all of
// the subject's symbols: an `Unparsed` entry whose facts are not only its
// imports, or an `ExtractionNotes` entry for a symbol walk that did not finish.
func (compiler *taskContextCompiler) subjectSymbolsIncomplete() bool {
	noted := slices.ContainsFunc(compiler.index.ExtractionNotes, func(note ExtractionNote) bool { return note.Path == compiler.subject })
	refused := slices.ContainsFunc(compiler.index.Unparsed, func(item Unparsed) bool {
		return item.Path == compiler.subject && item.Facts != "imports"
	})
	return noted || refused
}

// takeSlot records what a slot's generator materialised before admitting from
// it. The record is per candidate row kind rather than per slot, because the
// `mentioned` slot also yields `pair` rows for a named path's counterpart.
func (compiler *taskContextCompiler) takeSlot(rows, candidates []contextRow, cap int) []contextRow {
	for _, candidate := range candidates {
		compiler.candidates[candidate.kind] = append(compiler.candidates[candidate.kind], candidate.path)
	}
	return compiler.take(rows, candidates, cap)
}

// corroborate ranks a row that a later slot would also have admitted above
// rows with one relation only, keeping slot order among equals; the row's
// summary names the corroborating relations and its score rises by 50 each
// (decision 0027). Rows keep their one evidence row: the relation that
// admitted them.
func (compiler *taskContextCompiler) corroborate(rows []contextRow) []contextRow {
	for index := range rows {
		others := compiler.support[rows[index].path]
		if len(others) == 0 {
			continue
		}
		rows[index].score += 50 * len(others)
		rows[index].summary += "; also " + strings.Join(others, ", ")
	}
	sort.SliceStable(rows, func(left, right int) bool {
		return len(compiler.support[rows[left].path]) > len(compiler.support[rows[right].path])
	})
	return rows
}

// take appends candidates not yet chosen and never the subject, up to cap.
func (compiler *taskContextCompiler) take(rows, candidates []contextRow, cap int) []contextRow {
	taken := 0
	for position, candidate := range candidates {
		if taken == cap {
			compiler.slotOmitted = compiler.slotOmitted || compiler.anyEligible(candidates[position:])
			break
		}
		if candidate.path == compiler.subject {
			continue
		}
		if _, seen := compiler.chosen[candidate.path]; seen {
			compiler.support[candidate.path] = appendRelation(compiler.support[candidate.path], candidate.kind)
			continue
		}
		compiler.chosen[candidate.path] = struct{}{}
		rows = append(rows, candidate)
		taken++
	}
	return rows
}

// anyEligible reports whether a slot that has hit its cap still holds a
// candidate the packet could have carried: the slot-cap half of
// TCP-V0-011's `budget_shortage` `slots`.
func (compiler *taskContextCompiler) anyEligible(candidates []contextRow) bool {
	for _, candidate := range candidates {
		if candidate.path == compiler.subject {
			continue
		}
		if _, seen := compiler.chosen[candidate.path]; !seen {
			return true
		}
	}
	return false
}

func appendRelation(relations []string, kind string) []string {
	if slices.Contains(relations, kind) {
		return relations
	}
	return append(relations, kind)
}

// pairRows: the test or source counterpart of a path by naming convention,
// its module directory, and the mirrored test/source directory.
func (compiler *taskContextCompiler) pairRows(anchor string) []contextRow {
	compiler.markRan("pair")
	stem := contextStem(anchor)
	anchorIsTest := contextIsTest(anchor)
	candidates := make([]contextRow, 0)
	for candidate := range compiler.trackedPaths() {
		if candidate == anchor {
			continue
		}
		relation := pairRelation(anchor, candidate, stem, anchorIsTest)
		if relation == "" {
			continue
		}
		candidates = append(candidates, contextRow{
			kind: "pair", path: candidate, score: 900, line: 1,
			summary: relation + " of " + anchor, reason: relation + " of " + anchor,
			confidence: pairConfidence(relation), authority: "test-convention",
		})
	}
	compiler.orderByEvidence(candidates)
	return candidates
}

func pairRelation(anchor, candidate, stem string, anchorIsTest bool) string {
	candidateIsTest := contextIsTest(candidate)
	sameStem := contextStem(candidate) == stem
	switch {
	case sameStem && anchorIsTest != candidateIsTest && path.Dir(candidate) == path.Dir(anchor):
		return pairName(candidateIsTest)
	case sameStem && anchorIsTest != candidateIsTest && mirroredDirectory(path.Dir(anchor), path.Dir(candidate)):
		return pairName(candidateIsTest) + " in the mirrored directory"
	case sameStem && anchorIsTest != candidateIsTest:
		return pairName(candidateIsTest) + " elsewhere in the tree"
	case strings.HasPrefix(candidate, strings.TrimSuffix(anchor, path.Ext(anchor))+"/"):
		return "module directory member"
	}
	return ""
}

// pairConfidence: a counterpart found anywhere in the tree by stem alone is
// medium (decision 0026: 4 gold in 35 such rows, 3 of 5 wrong `certain`
// claims); the same-directory, mirrored-directory, and module rows stay high.
func pairConfidence(relation string) string {
	if strings.HasSuffix(relation, "elsewhere in the tree") {
		return "medium"
	}
	return "high"
}

func pairName(candidateIsTest bool) string {
	if candidateIsTest {
		return "test counterpart"
	}
	return "source counterpart"
}

// mirroredDirectory holds when one directory is the other with a test segment
// swapped for a source segment (`__tests__`/`src`, `tests`/`src`, `test`/`src`).
func mirroredDirectory(left, right string) bool {
	if left == right {
		return false
	}
	return mirrorSegments(left) == mirrorSegments(right)
}

func mirrorSegments(directory string) string {
	parts := strings.Split(directory, "/")
	for position, part := range parts {
		switch part {
		case "__tests__", "tests", "test", "src", "lib":
			parts[position] = "*"
		}
	}
	return strings.Join(parts, "/")
}

// contextIsTest extends impact's test rule with the Java and JVM convention
// of a `Test`/`Tests` suffix on the class name.
func contextIsTest(value string) bool {
	if isTestPath(value) {
		return true
	}
	base := strings.TrimSuffix(path.Base(value), path.Ext(value))
	return strings.HasSuffix(base, "Test") || strings.HasSuffix(base, "Tests")
}

func contextStem(value string) string {
	base := path.Base(value)
	base = strings.TrimSuffix(base, path.Ext(base))
	for _, infix := range []string{".test", ".spec", "_test", "_spec", "Tests", "Test"} {
		base = strings.TrimSuffix(base, infix)
	}
	base = strings.TrimPrefix(base, "test_")
	return strings.ToLower(base)
}

// mentionRows: every tracked path the task names, by full path or by an
// unambiguous basename, plus that path's counterpart.
func (compiler *taskContextCompiler) mentionRows() []contextRow {
	rows := make([]contextRow, 0)
	for _, item := range compiler.mentionedPaths() {
		rows = append(rows, contextRow{
			kind: "mentioned", path: item, score: 850, line: 1,
			summary: "named in the task", reason: "the task names " + item,
			confidence: "high", authority: "task-text",
		})
		for _, partner := range compiler.pairRows(item) {
			partner.summary, partner.score = partner.reason+", which the task names", 840
			rows = append(rows, partner)
		}
	}
	return rows
}

// mentionedPaths is the sorted tracked paths the task names by full path or
// by an unambiguous basename.
func (compiler *taskContextCompiler) mentionedPaths() []string {
	byBase := map[string][]string{}
	for candidate := range compiler.trackedPaths() {
		byBase[path.Base(candidate)] = append(byBase[path.Base(candidate)], candidate)
	}
	mentioned := make([]string, 0)
	for _, token := range contextPathTokens(compiler.task) {
		if _, tracked := compiler.trackedPaths()[token]; tracked {
			mentioned = append(mentioned, token)
			continue
		}
		if owners := byBase[token]; len(owners) == 1 {
			mentioned = append(mentioned, owners[0])
		}
	}
	sort.Strings(mentioned)
	return mentioned
}

func contextPathTokens(task string) []string {
	seen := map[string]struct{}{}
	tokens := make([]string, 0)
	for _, field := range strings.FieldsFunc(task, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\t' || r == '`' || r == '"' || r == '\'' || r == '(' || r == ')' || r == ',' || r == ':'
	}) {
		token := strings.TrimRight(strings.TrimPrefix(field, "./"), ".;")
		if !strings.Contains(token, ".") || strings.HasPrefix(token, ".") {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		tokens = append(tokens, token)
	}
	return tokens
}

// symbolRows: definitions of identifiers the task names, rarer definers first.
func (compiler *taskContextCompiler) symbolRows() []contextRow {
	definers := compiler.eligibleDefiners()
	type scored struct {
		row   contextRow
		score float64
	}
	found := make([]scored, 0)
	seen := map[string]struct{}{}
	for _, identifier := range compiler.identifiers {
		if !definitionEligible(identifier) {
			continue
		}
		symbols := definers[identifier.name]
		if len(symbols) == 0 || len(symbols) > contextMaxDefiners {
			continue
		}
		rarity := math.Log(float64(len(compiler.index.Sources)+1) / float64(len(symbols)))
		for _, symbol := range symbols {
			if _, ok := seen[symbol.Path]; ok {
				continue
			}
			seen[symbol.Path] = struct{}{}
			found = append(found, scored{contextRow{
				kind: "definition", path: symbol.Path, score: 800, line: symbol.Line,
				summary: "defines " + symbol.Name + ", which the task names",
				reason:  "defines " + symbol.Name, confidence: "high", authority: SyntaxAuthority,
			}, float64(identifier.weight) * rarity})
		}
	}
	sort.SliceStable(found, func(left, right int) bool {
		if found[left].score != found[right].score {
			return found[left].score > found[right].score
		}
		return found[left].row.path < found[right].row.path
	})
	rows := make([]contextRow, 0, len(found))
	for _, item := range found {
		rows = append(rows, item.row)
	}
	return rows
}

// eligibleDefiners: every symbol of each definitionEligible task identifier,
// keyed by name in the index's symbol order; the symbol walk is skipped when
// no identifier is eligible. Only the lookup keys are narrowed, never a name's
// symbols, so each definer count, and with it the contextMaxDefiners cut and
// the rarity weight, is exact.
func (compiler *taskContextCompiler) eligibleDefiners() map[string][]Symbol {
	wanted := map[string]struct{}{}
	for _, identifier := range compiler.identifiers {
		if definitionEligible(identifier) {
			wanted[identifier.name] = struct{}{}
		}
	}
	definers := map[string][]Symbol{}
	if len(wanted) == 0 {
		return definers
	}
	for _, symbol := range compiler.index.Symbols {
		if _, ok := wanted[symbol.Name]; ok {
			definers[symbol.Name] = append(definers[symbol.Name], symbol)
		}
	}
	return definers
}

// definitionEligible narrows the identifiers the `definition` slot may look up
// to those shaped like code the task actually names (TCP-V0-010): backticked
// unless the word is on the English stop list (TCP-V0-015: `all` in backticks
// is prose, and its three definers are noise), camelCase, snake_case, or
// otherwise carrying a digit or an underscore. It
// filters this generator's input only; the shared taskIdentifiers set, and so
// the pair ordering and the sibling slot's identifier evidence, is unchanged.
func definitionEligible(identifier taskIdentifier) bool {
	if identifier.weight >= 3 {
		_, prose := taskStopWords[identifier.name]
		return !prose
	}
	for position := 0; position < len(identifier.name); position++ {
		letter := identifier.name[position]
		if letter == '_' || (letter >= '0' && letter <= '9') {
			return true
		}
		if position > 0 && letter >= 'A' && letter <= 'Z' {
			return true
		}
	}
	return false
}

// importerRows: the subject's reverse importers from the index's import graph,
// plus the Swift module importers no `impact` rule names (decision 0030).
func (compiler *taskContextCompiler) importerRows() []contextRow {
	rows := make([]contextRow, 0)
	for _, edge := range reverseImporters(compiler.index, compiler.subject) {
		line, loaded := importEvidenceLine(compiler.index.Sources[edge.path], edge.imported)
		if !loaded {
			continue
		}
		rows = append(rows, contextRow{
			kind: "reverse-import", path: edge.path, score: 700,
			line:    line,
			summary: "imports " + compiler.subject, reason: "imports " + edge.imported,
			confidence: "high", authority: SyntaxAuthority,
		})
	}
	rows = append(rows, compiler.swiftImporterRows()...)
	sort.Slice(rows, func(left, right int) bool { return rows[left].path < rows[right].path })
	return rows
}

// referenceRows: the files bound to the subject by name rather than by an
// import edge (decision 0035): a source naming, as a whole word, a symbol the
// subject defines, weighted by the symbol's rarity across sources. A name
// more than contextMaxDefiners sources use is a common word and admits
// nothing. Every lookup is a word-table posting walk. Naming the subject's
// stem, in either direction, was probed and added nothing on any set.
func (compiler *taskContextCompiler) referenceRows() []contextRow {
	table := compiler.index.vocabulary()
	subjectID, ok := table.sourceID(compiler.subject)
	if !ok {
		return nil
	}
	weight := map[int]float64{}
	reason := map[int]string{}
	credit := func(source int, value float64, why string) {
		if source == subjectID {
			return
		}
		weight[source] += value
		if reason[source] == "" {
			reason[source] = why
		}
	}
	for _, symbol := range compiler.subjectSymbols() {
		low, high, found := table.Words.find(symbol)
		if !found || high-low > contextMaxDefiners {
			continue
		}
		rarity := math.Log(float64(len(table.Paths)+1) / float64(high-low))
		for index := low; index < high; index++ {
			credit(int(table.Words.Sources[index]), rarity, "names "+symbol+", which the subject defines")
		}
	}
	rows := make([]contextRow, 0, len(weight))
	for source, value := range weight {
		rows = append(rows, contextRow{
			kind: "reference", path: table.Paths[source], score: 600 + int(math.Round(10*value)), line: 1,
			summary: reason[source], reason: reason[source], confidence: "medium", authority: SyntaxAuthority,
		})
	}
	sort.Slice(rows, func(left, right int) bool {
		if rows[left].score != rows[right].score {
			return rows[left].score > rows[right].score
		}
		return rows[left].path < rows[right].path
	})
	for index := range rows {
		rows[index].score = min(rows[index].score, contextReferenceMaxScore)
	}
	return rows
}

// subjectSymbols is the sorted set of names the subject defines, at least
// four bytes long so a one-letter receiver or a two-letter accessor is not a
// reference.
func (compiler *taskContextCompiler) subjectSymbols() []string {
	names := map[string]struct{}{}
	for _, symbol := range compiler.index.Symbols {
		if symbol.Path == compiler.subject && len(symbol.Name) >= 4 {
			names[symbol.Name] = struct{}{}
		}
	}
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

var swiftImportLine = regexp.MustCompile(`(?m)^[ \t]*(?:@testable[ \t]+)?import[ \t]+([A-Za-z_][A-Za-z0-9_]*)`)

// swiftImporterRows: a Swift source in another SwiftPM module that imports the
// subject's module and names a symbol the subject defines. The module edge alone
// admits every file of every client (probe: 22 gold in 351 rows); the symbol
// corroboration holds precision at the co-change slot's level (17 in 127). The
// row is medium because the import names the module, not the file.
func (compiler *taskContextCompiler) swiftImporterRows() []contextRow {
	module := swiftModule(compiler.subject)
	if module == "" {
		return nil
	}
	patterns := wordPatterns(compiler.swiftSubjectSymbols())
	if len(patterns) == 0 {
		return nil
	}
	rows := make([]contextRow, 0)
	for candidate, source := range compiler.index.Sources {
		if !strings.HasSuffix(candidate, ".swift") || swiftModule(candidate) == module {
			continue
		}
		text, valid, loaded := source.Text()
		if !loaded || !valid {
			continue
		}
		line := swiftImportsModule(text, module)
		if line == 0 {
			continue
		}
		name := firstNamed(text, patterns)
		if name == "" {
			continue
		}
		rows = append(rows, contextRow{
			kind: "reverse-import", path: candidate, score: 700, line: line,
			summary: "imports module " + module + " and names " + name, reason: "imports " + module + "; names " + name,
			confidence: "medium", authority: SyntaxAuthority,
		})
	}
	return rows
}

// swiftModule is the SwiftPM module a path belongs to: the segment after
// `Sources/` or `Tests/`, or empty for a path outside either layout.
func swiftModule(value string) string {
	parts := strings.Split(value, "/")
	for position, part := range parts[:len(parts)-1] {
		if (part == "Sources" || part == "Tests") && position+1 < len(parts)-1 {
			return parts[position+1]
		}
	}
	return ""
}

func (compiler *taskContextCompiler) swiftSubjectSymbols() []string {
	names := make([]string, 0)
	for _, symbol := range compiler.index.Symbols {
		if symbol.Path == compiler.subject {
			names = append(names, symbol.Name)
		}
	}
	return names
}

// swiftImportsModule reports the 1-based line of `import module`, or 0.
func swiftImportsModule(text, module string) int {
	for _, match := range swiftImportLine.FindAllStringSubmatchIndex(text, -1) {
		if text[match[2]:match[3]] == module {
			return strings.Count(text[:match[0]], "\n") + 1
		}
	}
	return 0
}

type namePattern struct {
	name    string
	pattern *regexp.Regexp
}

func wordPatterns(names []string) []namePattern {
	patterns := make([]namePattern, 0, len(names))
	for _, name := range names {
		patterns = append(patterns, namePattern{name, regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)})
	}
	return patterns
}

// firstNamed returns the first symbol name that text mentions as a whole word.
func firstNamed(text string, patterns []namePattern) string {
	for _, item := range patterns {
		if item.pattern.MatchString(text) {
			return item.name
		}
	}
	return ""
}

// readCoChangeHistory reads the recent non-merge commits behind the revision
// with their paths: the bounded, local history the co-change slot counts over.
func readCoChangeHistory(ctx context.Context, index *Index) ([]historyEntry, error) {
	if history, ok := packHistory(index); ok {
		return history, nil
	}
	// `%D` limited to no refs prints only `grafted`, on a shallow boundary or a
	// grafts entry, whose path list is a diff against a parent Git does not have.
	raw, err := git(ctx, index.Root, maxHistoryBytes, nil, "log", "-z", fmt.Sprintf("-%d", maxHistoryCommits), "--no-merges", "--no-renames", "--decorate-refs=refs/nothing", "--format=%x1e%D%x1d%H%x1f%T%x1f%s", "--name-only", "HEAD")
	if err != nil {
		return nil, err
	}
	entries, _, err := parseHistory(dropGraftedCommits(raw), index.ObjectFormat)
	return entries, err
}

// dropGraftedCommits removes each record decorated `grafted` and strips the
// decoration field from the rest, leaving the format parseHistory reads.
func dropGraftedCommits(raw []byte) []byte {
	kept := make([]byte, 0, len(raw))
	for _, record := range bytes.Split(raw, []byte{0x1e}) {
		// A record without the field reaches parseHistory whole, which refuses it.
		decoration, rest, found := bytes.Cut(record, []byte{0x1d})
		if !found {
			decoration, rest = nil, record
		}
		if slices.Contains(strings.Split(string(bytes.TrimSpace(decoration)), ", "), "grafted") {
			continue
		}
		kept = append(append(kept, 0x1e), rest...)
	}
	return kept
}

// cochangeCommitCap is the largest commit (in paths) the co-change slot counts
// for a history of the given length.
func cochangeCommitCap(commits int) int {
	span := maxHistoryCommits - contextCochangeYoungCommits
	cap := contextCochangeYoungCap - (commits-contextCochangeYoungCommits)*(contextCochangeYoungCap-contextCochangeMatureCap)/span
	return maxInt(contextCochangeMatureCap, minInt(contextCochangeYoungCap, cap))
}

// cochangeRows: tracked paths that changed in the same commits as the subject,
// most co-changes first; a commit over the age-dependent cap is ignored.
func (compiler *taskContextCompiler) cochangeRows() []contextRow {
	tracked := compiler.trackedPaths()
	counts := map[string]int{}
	commits := 0
	commitCap := cochangeCommitCap(len(compiler.history))
	for _, entry := range compiler.history {
		if !slices.Contains(entry.paths, compiler.subject) {
			continue
		}
		if len(entry.paths) > commitCap {
			compiler.cochangeCapped = true
			continue
		}
		commits++
		for _, candidate := range entry.paths {
			if _, ok := tracked[candidate]; ok && candidate != compiler.subject {
				counts[candidate]++
			}
		}
	}
	rows := make([]contextRow, 0, len(counts))
	for candidate, count := range counts {
		// History is a reason to read a file, never proof that it changes with
		// this edit (invariant 3): every co-change row is medium, so the agent
		// claims it at most `likely` (decision 0025, amended on the first
		// history-backed reading).
		confidence := "medium"
		relation := fmt.Sprintf("changed with %s in %d of %d recent commits (commits over %d paths ignored)", compiler.subject, count, commits, commitCap)
		rows = append(rows, contextRow{
			kind: "cochange", path: candidate, score: 600 + count, line: 1,
			summary: relation, reason: relation,
			confidence: confidence, authority: "git-history",
		})
	}
	sort.Slice(rows, func(left, right int) bool {
		if rows[left].score != rows[right].score {
			return rows[left].score > rows[right].score
		}
		return rows[left].path < rows[right].path
	})
	return rows
}

// siblingRows: the subject's directory, then its parent subtree where the
// task's identifiers appear, ordered by that identifier evidence.
func (compiler *taskContextCompiler) siblingRows() []contextRow {
	directory := path.Dir(compiler.subject)
	parent := path.Dir(directory)
	rows := make([]contextRow, 0)
	for candidate := range compiler.trackedPaths() {
		candidateDirectory := path.Dir(candidate)
		if candidateDirectory == directory {
			rows = append(rows, compiler.siblingRow(candidate, "same directory as", 500))
			continue
		}
		if directory != "." && (candidateDirectory == parent || strings.HasPrefix(candidateDirectory, parent+"/")) && compiler.identifierEvidence(candidate) > 0 {
			rows = append(rows, compiler.siblingRow(candidate, "parent subtree of", 450))
		}
	}
	compiler.orderByEvidence(rows)
	compiler.orderByNameOverlap(rows)
	return rows
}

// orderByNameOverlap puts siblings whose basename shares a term with the
// subject's basename first (`PlaybackPlanning.swift` beside
// `PlaybackPlanningTests.swift`, `PlaybackState.swift`), keeping the evidence
// order within each group (decision 0031: +10 gold rows in the top 20 on the
// 63-task corvint v2 set, +2 on beamfall-apple, 0 on beamfall, offline).
func (compiler *taskContextCompiler) orderByNameOverlap(rows []contextRow) {
	subject := stringSetOf(lexicalTerms(strings.TrimSuffix(path.Base(compiler.subject), path.Ext(compiler.subject))))
	overlap := make(map[string]int, len(rows))
	for _, row := range rows {
		for _, term := range lexicalTerms(strings.TrimSuffix(path.Base(row.path), path.Ext(row.path))) {
			if _, ok := subject[term]; ok {
				overlap[row.path]++
			}
		}
	}
	sort.SliceStable(rows, func(left, right int) bool {
		return overlap[rows[left].path] > overlap[rows[right].path]
	})
}

func (compiler *taskContextCompiler) siblingRow(candidate, relation string, score int) contextRow {
	return contextRow{
		kind: "sibling", path: candidate, score: score, line: 1,
		summary: relation + " " + compiler.subject, reason: relation + " " + compiler.subject,
		confidence: "medium", authority: "directory",
	}
}

// lexicalRows is the listing a term search would give: distinct task terms
// present in the file or its path, then occurrences, then path.
type lexicalHit struct {
	path                  string
	source                uint32
	distinct, occurrences int
	score, rarestIDF      float64
	rarest                string
	documentation         bool
	anchors               []anchorHit
	role                  *roleHit
}

// lexicalHits is the scored posting walk, run once per compile: the test slot
// reads it for its anchors before the lexical fill orders it.
func (compiler *taskContextCompiler) lexicalHits() []lexicalHit {
	if compiler.lexical != nil {
		return compiler.lexical
	}
	// One posting walk per task term over the term table (decision 0033),
	// scored with BM25 (k1 1.2, b 0.3): the inverse document frequency of a
	// term comes from its posting length, the term frequency from the counted
	// body postings, and the length normalisation from the source's body
	// token count. A path term is a second field scored at tf 1 without
	// length normalisation. `distinct` and `occurrences` are kept for the
	// evidence line; the order is the BM25 score.
	const k1, b, pathGain = 1.2, 0.3, 1.0
	table := compiler.index.vocabulary()
	lengths, average := table.documentLengths()
	corpus := float64(len(table.Paths))
	if average == 0 {
		average = 1
	}
	idfOf := func(postings int) float64 {
		return math.Log(1 + (corpus-float64(postings)+0.5)/(float64(postings)+0.5))
	}
	distinct := make([]int, len(table.Paths))
	occurrences := make([]int, len(table.Paths))
	scores := make([]float64, len(table.Paths))
	rarestIDF := make([]float64, len(table.Paths))
	rarest := make([]string, len(table.Paths))
	credit := func(field int, source uint32, term string, idf, gain float64) {
		gain = compiler.queryTermGain(field, term, gain)
		scores[source] += gain
		distinct[source]++
		if idf > rarestIDF[source] {
			rarestIDF[source], rarest[source] = idf, term
		}
	}
	for _, term := range compiler.terms {
		if low, high, ok := table.Terms.find(term); ok {
			idf := idfOf(high - low)
			for index := low; index < high; index++ {
				source := table.Terms.Sources[index]
				tf := float64(table.Terms.Counts[index])
				norm := k1 * (1 - b + b*float64(lengths[source])/average)
				occurrences[source] += int(table.Terms.Counts[index])
				credit(0, source, term, idf, idf*tf*(k1+1)/(tf+norm))
			}
		}
		if low, high, ok := table.PathTerms.find(term); ok {
			idf := idfOf(high - low)
			for index := low; index < high; index++ {
				credit(1, table.PathTerms.Sources[index], term, idf, idf*(k1+1)/(1+k1)*pathGain)
			}
		}
	}
	// A whole identifier the task names (`stripFinalNewline`) is a third
	// field over the as-written identifier vocabulary: the body postings
	// split camel case, so only Words can answer the exact name, and its
	// posting length is the rarity the task's own wording carries.
	for _, identifier := range compiler.lexicalIdentifiers() {
		if compiler.selectedTerms == nil && (len(identifier.name) < 3 || !hasInnerCamelBoundary(identifier.name) && !strings.Contains(identifier.name, "_")) {
			continue
		}
		if low, high, ok := table.Words.find(identifier.name); ok {
			idf := idfOf(high - low)
			for index := low; index < high; index++ {
				credit(2, table.Words.Sources[index], identifier.name, idf, idf*(k1+1)/(1+k1))
			}
		}
	}
	// TCP-V0-022: an anchor is a fourth field, verified verbatim over the
	// sources its words' postings share; its idf is the verified posting
	// length and its tf the whole-anchor occurrence count, scored like a body
	// term. The anchor field is a credit inside the lexical slot, so an anchor
	// row keeps the slot's score and never outranks a reserved authority row.
	var anchorHits map[uint32][]anchorHit
	for _, anchor := range compiler.anchors {
		if anchorHits == nil {
			anchorHits = map[uint32][]anchorHit{}
		}
		sources, counts := compiler.anchorOccurrences(table, anchor.literal)
		idf := idfOf(len(sources))
		for index, source := range sources {
			tf := float64(counts[index])
			norm := k1 * (1 - b + b*float64(lengths[source])/average)
			occurrences[source] += counts[index]
			anchorHits[source] = append(anchorHits[source], anchorHit{literal: anchor.literal, count: counts[index]})
			credit(3, source, anchor.literal, idf, idf*tf*(k1+1)/(tf+norm))
		}
	}
	// TCP-V0-040: a role line is a fifth field over the highest-scoring
	// sources, credited as a path term is: tf 1, no length normalisation.
	roles := map[uint32]*roleHit{}
	for _, role := range compiler.roleHits(table, scores) {
		for _, term := range role.terms {
			if low, high, ok := table.Terms.find(term); ok {
				idf := idfOf(high - low)
				credit(1, role.source, term, idf, idf*roleGain)
			}
		}
		roles[role.source] = &role
	}
	hits := make([]lexicalHit, 0)
	for source, count := range distinct {
		if count > 0 {
			hits = append(hits, lexicalHit{
				path: table.Paths[source], source: uint32(source), distinct: count, occurrences: occurrences[source],
				score: scores[source], rarestIDF: rarestIDF[source], rarest: rarest[source],
				documentation: isDocumentationSuffix(table.Paths[source]), anchors: anchorHits[uint32(source)],
				role: roles[uint32(source)],
			})
		}
	}
	sort.Slice(hits, func(left, right int) bool {
		if hits[left].score != hits[right].score {
			return hits[left].score > hits[right].score
		}
		if hits[left].distinct != hits[right].distinct {
			return hits[left].distinct > hits[right].distinct
		}
		return hits[left].path < hits[right].path
	})
	compiler.lexical = hits
	return hits
}

// lexicalRows orders the hits: TCP-V0-013 places documentation after the
// fifth code row of the packet.
func (compiler *taskContextCompiler) lexicalRows(taken int) []contextRow {
	hits := compiler.lexicalHits()
	code, documentation := make([]lexicalHit, 0, len(hits)), make([]lexicalHit, 0, len(hits))
	for _, item := range hits {
		if item.documentation {
			documentation = append(documentation, item)
			continue
		}
		code = append(code, item)
	}
	// TCP-V0-013 counts the five code rows over the whole packet, so rows the
	// earlier slots took shorten the head the documentation quota waits for.
	head := min(max(contextDocumentationHead-taken, 0), len(code))
	quota := min(contextDocumentationQuota, len(documentation))
	ordered := make([]lexicalHit, 0, len(hits))
	ordered = append(ordered, code[:head]...)
	ordered = append(ordered, documentation[:quota]...)
	ordered = append(ordered, code[head:]...)
	ordered = append(ordered, documentation[quota:]...)
	rows := make([]contextRow, 0, len(ordered))
	for _, item := range ordered {
		kind := "lexical"
		reason := fmt.Sprintf("%d distinct task terms, %d occurrences; rarest `%s` (idf %.2f); bm25 %.2f",
			item.distinct, item.occurrences, item.rarest, item.rarestIDF, item.score)
		if len(item.anchors) > 0 {
			reason = anchorReason(item.anchors) + reason
		}
		if item.role != nil {
			reason = roleReason(item.role) + reason
		}
		if item.documentation {
			kind = "documentation"
			reason = "documentation: " + reason
		}
		rows = append(rows, contextRow{
			kind: kind, path: item.path, score: 300, line: 1,
			summary:    fmt.Sprintf("%d task terms match", item.distinct),
			reason:     reason,
			confidence: "low", authority: "vocabulary",
		})
	}
	return rows
}

func isDocumentationSuffix(candidate string) bool {
	switch strings.ToLower(path.Ext(candidate)) {
	case ".md", ".mdx", ".rst", ".txt":
		return true
	}
	return false
}

// identifierEvidence is the weight of the task's identifiers present in a
// source as whole words, summed once for every source from the word table on
// the first call (decision 0033); a path the table does not carry scores 0.
func (compiler *taskContextCompiler) identifierEvidence(candidate string) int {
	table := compiler.index.vocabulary()
	if compiler.evidence == nil {
		compiler.evidence = make([]int, len(table.Paths))
		for _, identifier := range compiler.identifiers {
			low, high, ok := table.Words.find(identifier.name)
			if !ok {
				continue
			}
			for index := low; index < high; index++ {
				compiler.evidence[table.Words.Sources[index]] += identifier.weight
			}
		}
	}
	source, ok := table.sourceID(candidate)
	if !ok {
		return 0
	}
	return compiler.evidence[source]
}

func (compiler *taskContextCompiler) orderByEvidence(rows []contextRow) {
	evidence := make(map[string]int, len(rows))
	for _, row := range rows {
		evidence[row.path] = compiler.identifierEvidence(row.path)
	}
	sort.SliceStable(rows, func(left, right int) bool {
		if evidence[rows[left].path] != evidence[rows[right].path] {
			return evidence[rows[left].path] > evidence[rows[right].path]
		}
		return rows[left].path < rows[right].path
	})
}

// trackedPaths is the candidate universe for the path-shaped slots: every
// tree path when the index recorded them, else the indexed sources.
func (compiler *taskContextCompiler) trackedPaths() map[string]struct{} {
	if len(compiler.index.Tracked) != 0 {
		return compiler.index.Tracked
	}
	paths := make(map[string]struct{}, len(compiler.index.Sources))
	for candidate := range compiler.index.Sources {
		paths[candidate] = struct{}{}
	}
	return paths
}

// contextInstructionPrecedence is TCP-V0-008's literal file precedence, ahead
// of `.github/instructions/*.instructions.md`; no other nested instruction
// path is eligible.
var contextInstructionPrecedence = []string{
	"AGENTS.md", "CLAUDE.md", "GEMINI.md", "copilot-instructions.md", ".github/copilot-instructions.md",
}

// reserve prepends the governing and spec-mentioned rows (TCP-V0-008/009).
// It runs after TCP-V0-004's corroboration sort and before the final
// truncation, so neither can reorder or drop a reservation. An ordinary row
// for the same path is promoted in place: the reservation keeps its fixed
// fields and its sole evidence row, and names the removed row's relations in
// its summary (TCP-V0-003).
func (compiler *taskContextCompiler) reserve(rows []contextRow) []contextRow {
	compiler.reserved = compiler.reservedRows()
	if len(compiler.reserved) == 0 {
		return rows
	}
	position := make(map[string]int, len(compiler.reserved))
	for index, row := range compiler.reserved {
		position[row.path] = index
	}
	kept := make([]contextRow, 0, len(rows)+len(compiler.reserved))
	for _, row := range rows {
		index, promoted := position[row.path]
		if !promoted {
			kept = append(kept, row)
			continue
		}
		compiler.promoted[row.path] = row.kind
		compiler.reserved[index].summary += "; also " + strings.Join(
			orderRelations(append([]string{row.kind}, compiler.support[row.path]...)), ", ")
	}
	return append(compiler.reserved, kept...)
}

// orderRelations sorts relation names into TCP-V0-011's fixed slot order and
// drops duplicates.
func orderRelations(relations []string) []string {
	present := stringSetOf(relations)
	ordered := make([]string, 0, len(present))
	for _, relation := range contextRelationOrder {
		if _, ok := present[relation]; ok {
			ordered = append(ordered, relation)
		}
	}
	return ordered
}

func (compiler *taskContextCompiler) reservedRows() []contextRow {
	rows := make([]contextRow, 0, 1+contextSpecMentionCap+contextRoutedCap)
	if row, ok := compiler.governingRow(); ok {
		rows = append(rows, row)
	}
	rows = append(rows, compiler.specMentionedRows(rows)...)
	return append(rows, compiler.instructionRoutedRows(rows)...)
}

// governingRow reserves the repository's standing instructions (TCP-V0-008).
// `documentKind` classifies an instruction file without establishing which one
// governs, so precedence is decided here. A candidate the index did not read --
// over `maxSourceBytes`, or excluded -- reserves nothing and no lower-precedence
// file substitutes for it; the packet says so through `unexamined`.
func (compiler *taskContextCompiler) governingRow() (contextRow, bool) {
	candidates := compiler.instructionCandidates()
	compiler.candidates[governingRelation] = candidates
	compiler.relationState[governingRelation] = "examined"
	if len(candidates) == 0 {
		return contextRow{}, false
	}
	governing := candidates[0]
	if !compiler.readable(governing) {
		compiler.relationState[governingRelation] = "capped"
		return contextRow{}, false
	}
	return contextRow{
		kind: governingRelation, path: governing, score: 1000, line: 1,
		summary: "this project's standing instructions", reason: "the highest-precedence tracked instruction file",
		confidence: "high", authority: "project-instructions",
	}, true
}

// instructionPassage is one paragraph or list item of the governing file: a
// run of non-blank lines, where a Markdown list item opens a new passage. A
// fenced code block is literal text, not routing prose: its lines, fences
// included, end a passage and belong to none. A fence closes only as
// CommonMark closes it (see nextFence).
type instructionPassage struct {
	line  int
	lines []string
}

// routedPassage is a passage that shares enough task terms to route, with the
// shared terms and their summed idf as its order key.
type routedPassage struct {
	instructionPassage
	shared []string
	weight float64
}

func instructionPassages(text string) []instructionPassage {
	passages := make([]instructionPassage, 0)
	open, fence := false, ""
	for index, line := range strings.Split(text, "\n") {
		inside := fence != ""
		fence = nextFence(fence, line)
		prose := !inside && fence == "" && strings.TrimSpace(line) != ""
		if prose && (!open || contextListItem.MatchString(line)) {
			passages = append(passages, instructionPassage{line: index + 1})
		}
		if prose {
			passages[len(passages)-1].lines = append(passages[len(passages)-1].lines, line)
		}
		open = prose
	}
	return passages
}

// nextFence returns the fence open after line, "" when none is: a fence line
// opens one when none is open, and closes the open one only with the same
// character, at least as long, and no info string, so a fence of the other
// kind or a shorter one inside a block is literal text.
func nextFence(open, line string) string {
	match := contextFence.FindStringSubmatch(line)
	if match == nil {
		return open
	}
	if open == "" {
		return match[1]
	}
	closes := match[1][0] == open[0] && len(match[1]) >= len(open) && strings.TrimSpace(match[2]) == ""
	if !closes {
		return open
	}
	return ""
}

// instructionRoutedRows reserves the tracked paths the governing instructions
// name, in backticks, inside a passage that routedPassages keeps
// (TCP-V0-047). The project's own
// routing outranks lexical placement (invariant 3), but only where its words
// meet the task's, so a path named in an unrelated passage reserves nothing.
func (compiler *taskContextCompiler) instructionRoutedRows(taken []contextRow) []contextRow {
	if len(taken) == 0 || taken[0].kind != governingRelation {
		compiler.relationState[instructionRoutedRelation] = "not-applicable"
		return nil
	}
	governing := taken[0].path
	compiler.relationState[instructionRoutedRelation] = "examined"
	text, _ := sourceTextBounded(compiler.index.Sources[governing])
	seen := map[string]struct{}{}
	for _, row := range taken {
		seen[row.path] = struct{}{}
	}
	materialised := make([]string, 0)
	rows := make([]contextRow, 0, contextRoutedCap)
	for _, passage := range compiler.routedPassages(text) {
		for offset, line := range passage.lines {
			for _, match := range contextBackquoted.FindAllStringSubmatch(line, -1) {
				candidate := match[1]
				if _, tracked := compiler.index.Sources[candidate]; !tracked {
					continue
				}
				if _, done := seen[candidate]; done || candidate == compiler.subject {
					continue
				}
				seen[candidate] = struct{}{}
				materialised = append(materialised, candidate)
				if len(rows) == contextRoutedCap {
					compiler.slotOmitted = true
					continue
				}
				reason := fmt.Sprintf("named by the governing instructions for this task: %s:%d shares `%s`",
					governing, passage.line+offset, strings.Join(passage.shared, "`, `"))
				rows = append(rows, contextRow{
					kind: instructionRoutedRelation, path: candidate, score: 850, line: 1,
					summary: "named by the governing instructions for this task", reason: reason,
					confidence: "medium", authority: "instruction-reference",
				})
			}
		}
	}
	compiler.candidates[instructionRoutedRelation] = materialised
	return rows
}

// routedPassages keeps the passages that share at least contextRoutedMinTerms
// distinct task terms at or above contextRoutedMinIDF, strongest first: summed
// body idf of those terms, then file order. A term the body term table does
// not hold, such as an unsplit camelCase compound, has no document frequency
// and never counts.
func (compiler *taskContextCompiler) routedPassages(text string) []routedPassage {
	table := compiler.index.vocabulary()
	corpus := float64(len(table.Paths))
	task := stringSetOf(compiler.terms)
	routed := make([]routedPassage, 0)
	for _, passage := range instructionPassages(text) {
		shared := make([]string, 0)
		weight := 0.0
		for _, term := range taskLexicalTerms(strings.Join(passage.lines, "\n")) {
			if _, ok := task[term]; !ok {
				continue
			}
			low, high, held := table.Terms.find(term)
			if !held {
				continue
			}
			postings := float64(high - low)
			idf := math.Log(1 + (corpus-postings+0.5)/(postings+0.5))
			if idf < contextRoutedMinIDF {
				continue
			}
			shared = append(shared, term)
			weight += idf
		}
		if len(shared) < contextRoutedMinTerms {
			continue
		}
		routed = append(routed, routedPassage{instructionPassage: passage, shared: shared, weight: weight})
	}
	sort.SliceStable(routed, func(left, right int) bool {
		return routed[left].weight > routed[right].weight
	})
	return routed
}

func (compiler *taskContextCompiler) instructionCandidates() []string {
	files := make([]string, 0)
	for candidate := range compiler.trackedPaths() {
		if candidate == compiler.subject || documentKind(candidate) != "instructions" {
			continue
		}
		if _, eligible := instructionRank(candidate); !eligible {
			continue
		}
		files = append(files, candidate)
	}
	sort.Slice(files, func(left, right int) bool {
		leftRank, _ := instructionRank(files[left])
		rightRank, _ := instructionRank(files[right])
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return files[left] < files[right]
	})
	return files
}

func instructionRank(candidate string) (int, bool) {
	if index := slices.Index(contextInstructionPrecedence, candidate); index >= 0 {
		return index, true
	}
	if path.Dir(candidate) == ".github/instructions" {
		return len(contextInstructionPrecedence), true
	}
	return 0, false
}

// readable reports whether the index took a tracked path in and can hand back
// its text within the source bound.
func (compiler *taskContextCompiler) readable(candidate string) bool {
	if slices.ContainsFunc(compiler.index.Exclusions, func(item Exclusion) bool { return item.Path == candidate }) {
		return false
	}
	source, ok := compiler.index.Sources[candidate]
	if !ok {
		return false
	}
	_, loaded := sourceTextBounded(source)
	return loaded
}

// evidenceGapReason explains why the index carries no `blob_hash` for a
// tracked path (TCP-V0-003, TCP-V0-005, TCP-V0-016): either recorded as an
// `Exclusion` (a forbidden path, a size bound, or a pin-time refusal such as
// generated or LFS content), or -- the one case `admittedEntries` (index.go)
// does not record anywhere -- filtered out for an unadmitted suffix before
// pinning ran. This reads that existing, per-path evidence; it adds nothing
// to `index.Exclusions` and changes no corpus-wide count.
func evidenceGapReason(index *Index, candidate string) string {
	for _, exclusion := range index.Exclusions {
		if exclusion.Path == candidate {
			return exclusion.Reason
		}
	}
	if path.Ext(candidate) != "" && !ImpactPathAdmitted(candidate) {
		return "source suffix is not admitted for indexing"
	}
	return "tracked but not indexed; no exclusion was recorded"
}

// subjectEvidenceGap explains why a tracked subject carries no `blob_hash`
// (TCP-V0-005). TaskContext already refuses an untracked subject, so a
// subject reaching here that is absent from `index.Sources` was screened out
// while the index was built.
func (compiler *taskContextCompiler) subjectEvidenceGap() string {
	return evidenceGapReason(compiler.index, compiler.subject)
}

// contextClauseLine is the definition grammar `script/check-requirement-definitions.sh`
// uses: a list-leading, optionally bolded or backticked requirement id followed
// by a colon or a period.
var contextClauseLine = regexp.MustCompile("^[ \t]*(?:[-*][ \t]+)?(?:\\*\\*)?`?([A-Z][A-Z0-9-]*-[0-9]{3})`?(?:\\*\\*)?[ \t]*[:.]")

// contextRequirementID is the id token shape; its boundaries are checked
// separately because RE2 has no lookaround.
var contextRequirementID = regexp.MustCompile(`[A-Z][A-Z0-9-]*-[0-9]{3}`)

type specDefinition struct {
	path string
	line int
}

// specMentionedRows reserves the specs that define a requirement id the task
// names, and the tracked spec paths it names outright (TCP-V0-009). Admission
// needs a defining clause, so prose such as `ISO-123` admits nothing, and
// title-term overlap admits nothing at all: discovery relevance is not
// governing authority.
func (compiler *taskContextCompiler) specMentionedRows(taken []contextRow) []contextRow {
	specs := compiler.specPaths()
	ids := compiler.requirementIDs()
	named := compiler.namedSpecPaths(specs, compiler.task)
	// Activation reads the task or the subject; path admission reads the task
	// alone, so a subject naming only a spec path runs the pass and admits
	// nothing through it (TCP-V0-009).
	mentioned := len(named) + len(compiler.namedSpecPaths(specs, compiler.subjectText()))
	if len(ids) == 0 && mentioned == 0 {
		compiler.relationState[specMentionedRelation] = "not-applicable"
		return nil
	}
	definers, capped := compiler.specDefinitions(specs)
	compiler.relationState[specMentionedRelation] = "examined"
	if capped {
		compiler.relationState[specMentionedRelation] = "capped"
	}
	materialised := make([]string, 0)
	rows := make([]contextRow, 0, contextSpecMentionCap)
	seen := map[string]struct{}{}
	for _, row := range taken {
		seen[row.path] = struct{}{}
	}
	admit := func(candidate string, line int, reason string) bool {
		// Deduplication precedes the cap: a path the packet already carries is
		// not a candidate the budget turned away, so it cannot report a
		// shortage (TCP-V0-009/011).
		if _, done := seen[candidate]; done || candidate == compiler.subject {
			return false
		}
		if len(rows) == contextSpecMentionCap {
			compiler.slotOmitted = true
			return false
		}
		seen[candidate] = struct{}{}
		rows = append(rows, contextRow{
			kind: specMentionedRelation, path: candidate, score: 900, line: line,
			summary: reason, reason: reason, confidence: "high", authority: "repository-spec",
		})
		return true
	}
	for _, id := range ids {
		owners := definers[id]
		if len(owners) == 0 {
			continue
		}
		reason := "defines " + id
		if len(owners) > 1 {
			reason = fmt.Sprintf("defines %s; ambiguous-definition (%d specs)", id, len(owners))
		}
		// Every definer is a candidate in ascending path order; the first that
		// fits the cap is admitted and the rest are held back.
		admitted := false
		for _, owner := range owners {
			materialised = append(materialised, owner.path)
			if admitted {
				continue
			}
			admitted = admit(owner.path, owner.line, reason)
		}
	}
	for _, candidate := range named {
		materialised = append(materialised, candidate)
		admit(candidate, 1, "the task names "+candidate)
	}
	compiler.candidates[specMentionedRelation] = materialised
	return rows
}

// specPaths is the tracked, non-recursive `docs/specs/*.md` set the resolver
// reads, minus its README.
func (compiler *taskContextCompiler) specPaths() []string {
	found := make([]string, 0)
	for candidate := range compiler.trackedPaths() {
		if path.Dir(candidate) != "docs/specs" || path.Ext(candidate) != ".md" || path.Base(candidate) == "README.md" {
			continue
		}
		found = append(found, candidate)
	}
	sort.Strings(found)
	return found
}

// specDefinitions maps each requirement id to the specs whose body carries its
// clause line, in ascending path order. A spec the bounded reader cannot hand
// back defines nothing and makes the relation's state `capped`.
func (compiler *taskContextCompiler) specDefinitions(specs []string) (map[string][]specDefinition, bool) {
	definitions := map[string][]specDefinition{}
	carried := map[string]struct{}{}
	capped := false
	for _, candidate := range specs {
		source, present := compiler.index.Sources[candidate]
		text, loaded := sourceTextBounded(source)
		if !present || !loaded {
			capped = true
			continue
		}
		for number, line := range strings.Split(text, "\n") {
			match := contextClauseLine.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			// One owner per path: a spec repeating an id's clause defines it
			// once, so `ambiguous-definition (N specs)` counts specs.
			if _, done := carried[match[1]+"\x00"+candidate]; done {
				continue
			}
			carried[match[1]+"\x00"+candidate] = struct{}{}
			definitions[match[1]] = append(definitions[match[1]], specDefinition{candidate, number + 1})
		}
	}
	return definitions, capped
}

// requirementIDs is the sorted set of id tokens the task names, plus those the
// subject's own body names when the index read it.
func (compiler *taskContextCompiler) requirementIDs() []string {
	found := append(requirementIDTokens(compiler.task), requirementIDTokens(compiler.subjectText())...)
	sort.Strings(found)
	return slices.Compact(found)
}

// subjectText is the subject's own body, and empty unless the subject is
// tracked, read, and under `maxSourceBytes`.
func (compiler *taskContextCompiler) subjectText() string {
	if compiler.subject == "" {
		return ""
	}
	source, ok := compiler.index.Sources[compiler.subject]
	if !ok {
		return ""
	}
	text, loaded := sourceTextBounded(source)
	if !loaded {
		return ""
	}
	return text
}

func requirementIDTokens(text string) []string {
	found := make([]string, 0)
	for _, span := range contextRequirementID.FindAllStringIndex(text, -1) {
		if idBoundary(text, span[0]-1) && idBoundary(text, span[1]) {
			found = append(found, text[span[0]:span[1]])
		}
	}
	return found
}

// idBoundary holds where an id token ends: outside `[A-Za-z0-9_-]`, or at the
// text boundary.
func idBoundary(text string, at int) bool {
	if at < 0 || at >= len(text) {
		return true
	}
	letter := text[at]
	switch {
	case letter == '_' || letter == '-':
		return false
	case letter >= '0' && letter <= '9':
		return false
	case letter >= 'a' && letter <= 'z':
		return false
	case letter >= 'A' && letter <= 'Z':
		return false
	}
	return true
}

func (compiler *taskContextCompiler) namedSpecPaths(specs []string, text string) []string {
	tracked := stringSetOf(specs)
	found := make([]string, 0)
	for _, token := range contextPathTokens(text) {
		if _, ok := tracked[token]; ok {
			found = append(found, token)
		}
	}
	sort.Strings(found)
	return slices.Compact(found)
}

// governance names what authority the packet found for this task: an
// instruction file, else a defining spec, else neither -- which is not a claim
// that the repository has none (invariant 2).
func (compiler *taskContextCompiler) governance() string {
	rows := compiler.governanceRows()
	for _, row := range rows {
		if row.kind == governingRelation {
			return "reserved"
		}
	}
	if len(rows) != 0 {
		return specMentionedRelation
	}
	return "unresolved"
}

// unexamined reports, per relation in the fixed order, how far the packet
// looked and how many candidates its generator held back (TCP-V0-011). It is
// measured from what the generators already produced and widens nothing.
func (compiler *taskContextCompiler) unexamined() []any {
	report := make([]any, 0, len(contextRelationOrder)+1)
	for _, relation := range compiler.contextRelations() {
		state := compiler.relationState[relation]
		if state == "" {
			state = "not-applicable"
		}
		report = append(report, map[string]any{
			"relation": relation, "state": state, "withheld": compiler.withheld(relation, state),
		})
	}
	return report
}

func (compiler *taskContextCompiler) withheld(relation, state string) any {
	if state == "subject-absent" || state == "not-applicable" {
		return nil
	}
	reserved := make(map[string]struct{}, len(compiler.reserved))
	for _, row := range compiler.reserved {
		reserved[row.path] = struct{}{}
	}
	counted := map[string]struct{}{}
	count := 0
	for _, candidate := range compiler.candidates[relation] {
		if candidate == compiler.subject {
			continue
		}
		if _, done := counted[candidate]; done {
			continue
		}
		counted[candidate] = struct{}{}
		if dropped, promoted := compiler.promoted[candidate]; promoted && dropped == relation {
			count++
			continue
		}
		if _, admitted := compiler.chosen[candidate]; admitted {
			continue
		}
		if _, admitted := reserved[candidate]; admitted {
			continue
		}
		count++
	}
	return count
}

// criticalSelectors splits the reserved set into what the packet carries and
// what the limit left out, each exhaustive over that bounded set rather than a
// sample (TCP-V0-006).
func (compiler *taskContextCompiler) criticalSelectors(rows []contextRow) ([]any, []any) {
	included := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		included[row.path] = struct{}{}
	}
	carried, missing := make([]any, 0), make([]any, 0)
	ordered := compiler.governanceRows()
	sort.SliceStable(ordered, func(left, right int) bool {
		leftRank := slices.Index(contextRelationOrder, ordered[left].kind)
		rightRank := slices.Index(contextRelationOrder, ordered[right].kind)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return ordered[left].path < ordered[right].path
	})
	for _, row := range ordered {
		selector := map[string]any{"relation": row.kind, "path": row.path}
		if _, ok := included[row.path]; ok {
			carried = append(carried, selector)
			continue
		}
		missing = append(missing, selector)
	}
	return carried, missing
}

// budgetShortage says what bounded the packet: a slot cap or the limit first,
// else the work the history window left undone, else nothing. `work` reports
// unexamined scope without asserting that more candidates exist.
func (compiler *taskContextCompiler) budgetShortage() string {
	switch {
	case compiler.slotOmitted || compiler.truncated:
		return "slots"
	case compiler.historyFull || compiler.cochangeCapped:
		return "work"
	}
	return "none"
}

func sourceTextBounded(source Source) (string, bool) {
	if max(len(source.Data), int(source.body.length)) > contextMaxBytes {
		return "", false
	}
	text, valid, loaded := source.Text()
	return text, loaded && valid
}

// taskIdentifiers: backticked names weigh three, other identifiers one;
// duplicates keep their highest weight.
func taskIdentifiers(task string) []taskIdentifier {
	weights := map[string]int{}
	for _, match := range contextBacktick.FindAllStringSubmatch(task, -1) {
		for _, part := range strings.Split(match[1], ".") {
			if len(part) >= 3 {
				weights[part] = 3
			}
		}
	}
	for _, name := range contextIdentifier.FindAllString(task, -1) {
		if _, ok := weights[name]; !ok {
			weights[name] = 1
		}
	}
	result := make([]taskIdentifier, 0, len(weights))
	for name, weight := range weights {
		result = append(result, taskIdentifier{name, weight})
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].weight != result[right].weight {
			return result[left].weight > result[right].weight
		}
		return result[left].name < result[right].name
	})
	return result
}

// taskLexicalTerms is lexicalTerms plus every whole identifier the camel-case
// split would otherwise lose, and minus a fixed list of English function and
// retrieval-phrasing words. The body and path term tables split camel case
// too (termCounter.count, lexicalTerms), so `stripFinalNewline` in a body or
// path is `strip`, `final` and `newline`; the lowered compound
// "stripfinalnewline" matches only a body or path that writes it as one
// unsplit run, such as `stripfinalnewline` or `STRIPFINALNEWLINE`. The
// identifier as written is answered by the Words field through
// lexicalIdentifiers, not by this term. The stop list
// has no per-repository parameter; it removes words whose rarity in code is
// an artefact of question phrasing ("how", "retrieve"), not evidence.
func taskLexicalTerms(text string) []string {
	seen := stringSetOf(lexicalTerms(text))
	for _, token := range contextToken.FindAllString(text, -1) {
		if len(token) < 2 || !hasInnerCamelBoundary(token) {
			continue
		}
		seen[strings.ToLower(token)] = struct{}{}
	}
	result := make([]string, 0, len(seen))
	for term := range seen {
		if _, stop := taskStopWords[term]; stop {
			continue
		}
		result = append(result, term)
	}
	sort.Strings(result)
	return result
}

func hasInnerCamelBoundary(token string) bool {
	return contextCamel.MatchString(token)
}

var taskStopWords = stringSetOf([]string{
	"a", "all", "an", "and", "are", "as", "at", "be", "by", "can", "do", "does", "for", "from", "how",
	"if", "in", "is", "it", "its", "of", "on", "or", "our", "retrieve", "show", "that", "the",
	"their", "this", "to", "what", "when", "where", "which", "who", "why", "with", "would", "you",
})

func lexicalTerms(text string) []string {
	spaced := contextCamel.ReplaceAllString(text, "$1 $2")
	seen := map[string]struct{}{}
	result := make([]string, 0)
	for _, token := range contextToken.FindAllString(spaced, -1) {
		token = strings.ToLower(token)
		if len(token) < 2 {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		result = append(result, token)
	}
	sort.Strings(result)
	return result
}

func stringSetOf(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

// packet renders the rows. The subject is carried beside the results, never
// among them; a packet with no candidate says so in its state. The subject
// carries exactly one of `blob_hash` (the index pinned it) or `evidence_gap`
// (it did not, and why) -- never neither, so a subject the index could not
// read never looks indistinguishable from one it could (AGENTS.md
// invariant 2).
func (compiler *taskContextCompiler) packet(rows []contextRow, limit int) map[string]any {
	results := make([]any, 0, len(rows))
	for _, row := range rows {
		source, pinned := compiler.index.Sources[row.path]
		entry := evidence(row.path, row.line, source.BlobHash, row.reason, row.confidence, row.authority)
		entry["trust"] = TrustClass(row.authority)
		if !pinned {
			// An empty blob_hash alone does not disclose that the row is
			// unpinned (AGENTS.md invariant 2): downgrade the confidence the
			// relation would otherwise claim and name why, the same
			// disclosure TCP-V0-005 already makes for the subject.
			entry["confidence"] = "low"
			entry["evidence_gap"] = evidenceGapReason(compiler.index, row.path)
		}
		results = append(results, map[string]any{
			"kind": row.kind, "id": row.path, "score": row.score, "summary": row.summary, "action": rowAction(row),
			"evidence": []any{entry},
		})
	}
	critical, missing := compiler.criticalSelectors(rows)
	state := "READY"
	if len(rows) == 0 {
		state = "NO_CANDIDATES"
	}
	var subject any
	if compiler.subject != "" {
		subject = map[string]any{
			"path": compiler.subject,
			"role": "the task's own path: the subject of the question, never one of its answers",
		}
		if source, ok := compiler.index.Sources[compiler.subject]; ok {
			subject.(map[string]any)["blob_hash"] = source.BlobHash
		} else {
			subject.(map[string]any)["evidence_gap"] = compiler.subjectEvidenceGap()
		}
	}
	packet := map[string]any{
		"tool": "context", "ok": true, "mutates": false, "schema_version": 1,
		"revision": compiler.index.Revision, "state": state, "subject": subject,
		"request": map[string]any{"limit": limit, "task_chars": len(compiler.task)},
		"coverage": map[string]any{
			"candidates": compiler.admitted, "included_results": len(rows),
			"omitted_results": maxInt(compiler.admitted-len(rows), 0),
			"governance":      compiler.governance(), "critical": critical, "critical_missing": missing,
			"unexamined": compiler.unexamined(), "budget_shortage": compiler.budgetShortage(),
			"answerability": compiler.answerability.packet(compiler.index), "governance_refused": compiler.governanceRefused(),
		},
		"results": results,
	}
	compiler.recencyCoverage(packet["coverage"].(map[string]any), rows)
	return packet
}

// rowAction says what to do with the file for this task, one sentence per
// relation, claiming only what the relation establishes (decision 0028).
func rowAction(row contextRow) string {
	switch row.kind {
	case governingRelation:
		return "Read this project's standing instructions before changing anything."
	case specMentionedRelation:
		return "Read the requirement clause this task names before changing its behavior."
	case instructionRoutedRelation:
		return "Read this file: the governing instructions name it in a passage that shares this task's terms, so the project routes work like this through it."
	case "pair":
		if contextIsTest(row.path) {
			return "Update this test: it is the subject's test counterpart, so a behaviour change in the subject changes what it must assert."
		}
		return "Read this source: the subject is its test, so a change in the test exercises what is defined here."
	case "mentioned":
		return "Open this file: the task names it or its counterpart, so act on it as the task says."
	case "definition":
		name := strings.TrimPrefix(row.reason, "defines ")
		return "Open this definition of `" + name + "`, which the task names: a change to how `" + name + "` is defined or used must be reconciled here."
	case "reverse-import":
		return "Check this importer of the subject: an exported change in the subject changes what it compiles or runs against."
	case "reference":
		return "Open this file: it " + row.reason + ", so a renamed or reshaped definition in the subject reaches it by name."
	case "cochange":
		return "Claim this file at `likely`: it " + row.reason + ", so this change most likely touches it too; open it to confirm."
	case "sibling":
		return "Scan this file, which is " + row.reason + ", for uses of the identifiers the task names; a change in the subject's package often lands here too."
	case "test":
		if contextIsTest(row.path) {
			return "Update this test: it " + row.reason + ", so a behaviour change in that file changes what it must assert."
		}
		return "Read this source: it " + row.reason + ", so the admitted test exercises what is defined here."
	case contextGraphRelation:
		return graphAction(row)
	default:
		return "Read this file only if the task terms it matches (" + row.reason + ") are load-bearing; a term match is not a relation."
	}
}

// trackedPath admits any path in the tree, indexed or not: an unread subject
// still has counterparts, siblings, and importers even without indexed text.
func trackedPath(index *Index, value string) bool {
	if _, ok := index.Sources[value]; ok {
		return true
	}
	_, ok := index.Tracked[value]
	return ok
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

// The `test` relation (TCP-V0-015): the test counterpart of a code file the
// packet admits, and the source an admitted test exercises, from four
// deterministic signals read at query time from the existing tables: the
// mirrored path/stem (`pairRelation`), an import edge from the test to the
// source (`Imports`), a whole-word mention in the test of a name the source
// declares (`Words`, rarity-weighted as `referenceRows` weights), and a test
// name (`Test<Name>`, `test_<name>`, `describe('<name>')`) whose camel-split
// tokens start with a declared name. One slot is reserved for the best row.

var contextDescribe = regexp.MustCompile("describe\\(\\s*['\"`]([^'\"`]+)")

type testCandidate struct {
	path, anchor, mirrored, named string
	imports                       bool
	mentions                      int
	rarest                        string
	rarestIDF, weight             float64
	position                      int
}

func (candidate *testCandidate) signals() int {
	count := 0
	for _, fired := range []bool{candidate.mirrored != "", candidate.imports, candidate.mentions > 0, candidate.named != ""} {
		if fired {
			count++
		}
	}
	return count
}

// outranks orders candidates by the anchor's packet position (the packet's
// own evidence order: a mentioned or defined anchor before a lexical one),
// then summed signal weight (a rarity-weighted mention outweighs a stem match
// elsewhere in the tree), then fired signals, then path.
func (candidate *testCandidate) outranks(other *testCandidate) bool {
	if candidate.position != other.position {
		return candidate.position < other.position
	}
	if candidate.weight != other.weight {
		return candidate.weight > other.weight
	}
	if candidate.signals() != other.signals() {
		return candidate.signals() > other.signals()
	}
	return candidate.path < other.path
}

func (candidate *testCandidate) reason() string {
	labels := make([]string, 0, 4)
	if candidate.mirrored != "" {
		labels = append(labels, "mirrored stem ("+candidate.mirrored+")")
	}
	if candidate.imports {
		labels = append(labels, "import edge")
	}
	switch {
	case candidate.mentions == 1:
		labels = append(labels, fmt.Sprintf("names %s (idf %.2f)", candidate.rarest, candidate.rarestIDF))
	case candidate.mentions > 1:
		labels = append(labels, fmt.Sprintf("names %d declared identifiers, rarest %s (idf %.2f)", candidate.mentions, candidate.rarest, candidate.rarestIDF))
	}
	if candidate.named != "" {
		labels = append(labels, "test name "+candidate.named)
	}
	direction := "tests "
	if contextIsTest(candidate.anchor) {
		direction = "is tested by "
	}
	return direction + candidate.anchor + ": " + strings.Join(labels, ", ")
}

// testAnchors is every admitted code path plus the lexical hits that would
// fill the packet to one row short of the limit: the rows the packet will
// carry, so a test row always sits beside its anchor.
func (compiler *taskContextCompiler) testAnchors(rows []contextRow, limit int) []string {
	anchors := make([]string, 0, limit)
	for _, row := range rows {
		anchors = append(anchors, row.path)
	}
	for _, hit := range compiler.lexicalHits() {
		if len(anchors) >= limit-1 {
			break
		}
		if _, taken := compiler.chosen[hit.path]; taken || hit.documentation {
			continue
		}
		anchors = append(anchors, hit.path)
	}
	return anchors
}

func (compiler *taskContextCompiler) testRows(anchors []string) []contextRow {
	linker := compiler.newTestLinker()
	best := map[string]*testCandidate{}
	for position, anchor := range anchors {
		for _, candidate := range linker.candidates(anchor) {
			candidate.position = position
			if current, ok := best[candidate.path]; ok && !candidate.outranks(current) {
				continue
			}
			best[candidate.path] = candidate
		}
	}
	ordered := make([]*testCandidate, 0, len(best))
	for _, candidate := range best {
		ordered = append(ordered, candidate)
	}
	sort.Slice(ordered, func(left, right int) bool { return ordered[left].outranks(ordered[right]) })
	rows := make([]contextRow, 0, len(ordered))
	for _, candidate := range ordered {
		confidence := "medium"
		if candidate.signals() >= 2 {
			confidence = "high"
		}
		rows = append(rows, contextRow{
			kind: "test", path: candidate.path, score: 650, line: 1,
			summary: candidate.reason(), reason: candidate.reason(),
			confidence: confidence, authority: "test-convention",
		})
	}
	return rows
}

type testName struct {
	path, name string
	tokens     []string
}

type testLinker struct {
	compiler *taskContextCompiler
	table    *TermTable
	// declared maps a declared name (at least four bytes, at most
	// contextMaxDefiners definers) to its definers; declaredBy is the inverse;
	// declaredByToken indexes the names by their first camel-split token.
	declared, declaredBy, declaredByToken map[string][]string
	// declaredTokens is nameTokens of each retained declared name, split once.
	declaredTokens map[string][]string
	// testNames indexes the test-function names of the test paths by their
	// first camel-split token after the `Test`/`test_` prefix.
	testNames map[string][]testName
	// tracked is the compiler's candidate universe, read once; counterparts
	// indexes it by contextStem with each path's contextIsTest role, so
	// creditMirrored consults the exact-stem paths only: every pairRelation
	// other than the module-directory one requires the same stem and the
	// opposite role.
	tracked      map[string]struct{}
	counterparts map[string][]counterpart
}

type counterpart struct {
	path   string
	isTest bool
}

func (compiler *taskContextCompiler) newTestLinker() *testLinker {
	linker := &testLinker{
		compiler: compiler, table: compiler.index.vocabulary(),
		declared: map[string][]string{}, declaredBy: map[string][]string{},
		declaredByToken: map[string][]string{}, declaredTokens: map[string][]string{},
		testNames: map[string][]testName{},
		tracked:   compiler.trackedPaths(), counterparts: map[string][]counterpart{},
	}
	for candidate := range linker.tracked {
		stem := contextStem(candidate)
		linker.counterparts[stem] = append(linker.counterparts[stem], counterpart{candidate, contextIsTest(candidate)})
	}
	for _, symbol := range compiler.index.Symbols {
		if len(symbol.Name) >= 4 {
			linker.declared[symbol.Name] = appendRelation(linker.declared[symbol.Name], symbol.Path)
		}
		remainder, isTestName := testNameRemainder(symbol.Name)
		if isTestName && contextIsTest(symbol.Path) {
			linker.addTestName(symbol.Path, symbol.Name, remainder)
		}
	}
	for name, definers := range linker.declared {
		if len(definers) > contextMaxDefiners {
			delete(linker.declared, name)
			continue
		}
		for _, definer := range definers {
			linker.declaredBy[definer] = append(linker.declaredBy[definer], name)
		}
		tokens := nameTokens(name)
		linker.declaredTokens[name] = tokens
		if len(tokens) > 0 {
			linker.declaredByToken[tokens[0]] = append(linker.declaredByToken[tokens[0]], name)
		}
	}
	for _, names := range linker.declaredBy {
		sort.Strings(names)
	}
	return linker
}

func (linker *testLinker) addTestName(path, name, remainder string) {
	tokens := nameTokens(remainder)
	if len(tokens) == 0 {
		return
	}
	linker.testNames[tokens[0]] = append(linker.testNames[tokens[0]], testName{path, name, tokens})
}

// testNameRemainder strips the `Test`/`test_` prefix of a test-function name.
func testNameRemainder(name string) (string, bool) {
	switch {
	case strings.HasPrefix(name, "test_"):
		return name[5:], len(name) > 5
	case strings.HasPrefix(name, "Test") && len(name) > 4 && name[4] >= 'A' && name[4] <= 'Z':
		return name[4:], true
	}
	return "", false
}

// nameTokens is the lowercase camel-split token list of a name: each token a
// maximal run of ASCII letters and digits, split further where a lowercase
// letter meets an uppercase one; every other byte, ASCII or not, separates.
// A name with no token yields nil. This is the exact ASCII semantics of the
// regex form (contextCamel `$1 $2`, then contextToken) it replaced.
func nameTokens(name string) []string {
	var tokens []string
	start := -1
	for position := 0; position < len(name); position++ {
		if !isASCIIAlnum(name[position]) {
			tokens, start = appendNameToken(tokens, name, start, position), -1
			continue
		}
		if start >= 0 && isASCIILower(name[position-1]) && isASCIIUpper(name[position]) {
			tokens, start = appendNameToken(tokens, name, start, position), position
			continue
		}
		if start < 0 {
			start = position
		}
	}
	return appendNameToken(tokens, name, start, len(name))
}

func appendNameToken(tokens []string, name string, start, end int) []string {
	if start < 0 {
		return tokens
	}
	return append(tokens, strings.ToLower(name[start:end]))
}

func tokensStartWith(tokens, prefix []string) bool {
	if len(prefix) == 0 || len(prefix) > len(tokens) {
		return false
	}
	return slices.Equal(tokens[:len(prefix)], prefix)
}

// candidates is the ordered counterpart list of one anchor; a test anchor
// yields sources and a code anchor yields tests.
func (linker *testLinker) candidates(anchor string) []*testCandidate {
	anchorIsTest := contextIsTest(anchor)
	found := map[string]*testCandidate{}
	credit := func(path string) *testCandidate {
		if path == anchor || contextIsTest(path) == anchorIsTest {
			return nil
		}
		if _, tracked := linker.tracked[path]; !tracked {
			return nil
		}
		if found[path] == nil {
			found[path] = &testCandidate{path: path, anchor: anchor}
		}
		return found[path]
	}
	linker.creditMirrored(anchor, anchorIsTest, credit)
	if anchorIsTest {
		linker.creditSourcesOfTest(anchor, credit)
	} else {
		linker.creditTestsOfSource(anchor, credit)
	}
	ordered := make([]*testCandidate, 0, len(found))
	for _, candidate := range found {
		ordered = append(ordered, candidate)
	}
	sort.Slice(ordered, func(left, right int) bool { return ordered[left].outranks(ordered[right]) })
	for _, candidate := range ordered[:min(len(ordered), contextTestCorroborated)] {
		linker.corroborate(candidate, anchorIsTest)
	}
	sort.Slice(ordered, func(left, right int) bool { return ordered[left].outranks(ordered[right]) })
	return ordered
}

// creditMirrored is signal one: the stem and directory convention `pair`
// reads, less the module-directory relation. What remains needs the anchor's
// stem and the opposite role, so only the indexed exact-stem paths are read.
func (linker *testLinker) creditMirrored(anchor string, anchorIsTest bool, credit func(string) *testCandidate) {
	stem := contextStem(anchor)
	for _, candidate := range linker.counterparts[stem] {
		if candidate.isTest == anchorIsTest {
			continue
		}
		relation := pairRelation(anchor, candidate.path, stem, anchorIsTest)
		if entry := credit(candidate.path); entry != nil {
			entry.mirrored = relation
			entry.weight += mirroredWeight(relation)
		}
	}
}

func mirroredWeight(relation string) float64 {
	if pairConfidence(relation) == "high" {
		return 1
	}
	return 0.5
}

// creditTestsOfSource runs signals two (import edge), three (mention) and
// four (test name) from a code anchor toward the test paths.
func (linker *testLinker) creditTestsOfSource(anchor string, credit func(string) *testCandidate) {
	for _, edge := range reverseImporters(linker.compiler.index, anchor) {
		if entry := credit(edge.path); entry != nil {
			entry.imports = true
			entry.weight++
		}
	}
	for _, name := range linker.declaredBy[anchor] {
		low, high, found := linker.table.Words.find(name)
		if !found || high-low > contextTestMentionPostings {
			continue
		}
		idf := math.Log(float64(len(linker.table.Paths)+1) / float64(high-low))
		for index := low; index < high; index++ {
			if entry := credit(linker.table.Paths[linker.table.Words.Sources[index]]); entry != nil {
				entry.mention(name, idf)
			}
		}
		tokens := linker.declaredTokens[name]
		if len(tokens) == 0 {
			continue
		}
		for _, test := range linker.testNames[tokens[0]] {
			if !tokensStartWith(test.tokens, tokens) {
				continue
			}
			if entry := credit(test.path); entry != nil {
				entry.name(test.name)
			}
		}
	}
}

// creditSourcesOfTest runs signals three and four from a test anchor toward
// the definers of the names its text and test functions carry; the import
// edge is corroborated on the ordered head.
func (linker *testLinker) creditSourcesOfTest(anchor string, credit func(string) *testCandidate) {
	text, loaded := sourceTextBounded(linker.compiler.index.Sources[anchor])
	if !loaded {
		return
	}
	for _, word := range keys(scanWords(text)) {
		definers := linker.declared[word]
		if len(definers) == 0 {
			continue
		}
		low, high, found := linker.table.Words.find(word)
		if !found || high-low > contextTestMentionPostings {
			continue
		}
		idf := math.Log(float64(len(linker.table.Paths)+1) / float64(high-low))
		for _, definer := range definers {
			if entry := credit(definer); entry != nil {
				entry.mention(word, idf)
			}
		}
	}
	for _, named := range linker.namedByTest(anchor, text) {
		for _, name := range linker.declaredByToken[named.tokens[0]] {
			if !tokensStartWith(named.tokens, linker.declaredTokens[name]) {
				continue
			}
			for _, definer := range linker.declared[name] {
				if entry := credit(definer); entry != nil {
					entry.name(named.name)
				}
			}
		}
	}
}

// namedByTest is the test-name vocabulary of one test path: its
// `Test<Name>`/`test_<name>` symbols and the `describe('<name>')` blocks of
// its text.
func (linker *testLinker) namedByTest(anchor, text string) []testName {
	names := make([]testName, 0)
	for _, entries := range linker.testNames {
		for _, entry := range entries {
			if entry.path == anchor {
				names = append(names, entry)
			}
		}
	}
	for _, match := range contextDescribe.FindAllStringSubmatch(text, -1) {
		if tokens := nameTokens(match[1]); len(tokens) > 0 {
			names = append(names, testName{anchor, "describe '" + match[1] + "'", tokens})
		}
	}
	sort.Slice(names, func(left, right int) bool { return names[left].name < names[right].name })
	return names
}

// corroborate reads the signals the cheap pass cannot: the import edge from a
// candidate test to a test anchor's source, and `describe('<name>')` blocks in
// a candidate test's text for a code anchor.
func (linker *testLinker) corroborate(candidate *testCandidate, anchorIsTest bool) {
	if anchorIsTest {
		for _, edge := range reverseImporters(linker.compiler.index, candidate.path) {
			if edge.path == candidate.anchor && !candidate.imports {
				candidate.imports = true
				candidate.weight++
			}
		}
		return
	}
	if candidate.named != "" {
		return
	}
	text, loaded := sourceTextBounded(linker.compiler.index.Sources[candidate.path])
	if !loaded {
		return
	}
	for _, match := range contextDescribe.FindAllStringSubmatch(text, -1) {
		tokens := nameTokens(match[1])
		for _, name := range linker.declaredBy[candidate.anchor] {
			if tokensStartWith(tokens, linker.declaredTokens[name]) {
				candidate.name("describe '" + match[1] + "'")
				return
			}
		}
	}
}

func (candidate *testCandidate) mention(name string, idf float64) {
	candidate.mentions++
	candidate.weight += idf
	if idf > candidate.rarestIDF {
		candidate.rarest, candidate.rarestIDF = name, idf
	}
}

func (candidate *testCandidate) name(label string) {
	if candidate.named != "" {
		return
	}
	candidate.named = label
	candidate.weight++
}

// Answerability (TCP-V0-016). The task's specific terms are the names it
// writes: a backticked token, or an identifier with an inner camel-case
// boundary or an underscore, that fewer sources name than the cut (one
// sixteenth of the indexed sources, never below two). Identifier vocabulary
// is closed, so a name no source writes is a claim the index refutes; prose
// vocabulary is open and abundant, so plain words, rare or not, are not
// terms of the conjunction. The conjunction is over the known names (those
// at least one source names); a source supports it by naming them together,
// and the required support is all of them, or all but one from three; both
// are reported. The packet withholds the slot rows in one case only: the
// task writes two or more names, no indexed source and no tracked path
// carries any of them, and no relation (a mentioned path, its pair, a
// definer) admitted a row: zero support anywhere over the closed identifier
// vocabulary, which a relation row would overrule because a source the index
// does not read (a `.java` file) still answers by path (invariant 2). It is
// a proof of absence rather than a threshold. (Withholding on a refuted name beside three or
// more known names no source carries together was measured and withdrawn,
// decision 0068: the joining source is often one over the index's size bound
// and so has no postings.) The withheld rows lead the nearest claims, each
// with what it lacks. A task that writes one unknown name (a feature it asks
// for) is answered. The reserved rows stay: project authority is not an
// answer to the task (TCP-V0-008).
const (
	specificTermFraction = 16
	nearestClaimCap      = 3
)

type specificTerm struct {
	term    string
	sources []uint32
	// tracked is the count of tracked paths carrying the name, indexed or
	// not: a `.java` source the index does not read still answers by name.
	tracked int
}

func (term specificTerm) known() bool { return len(term.sources) > 0 || term.tracked > 0 }

type nearestClaim struct {
	path            string
	supports, lacks []string
}

type answerability struct {
	terms []specificTerm
	known int
	// relations counts the rows one of TCP-V0-016(c)'s seven rescue relations
	// admitted (relationRows): mentioned, pair, definition, reverse-import,
	// reference, cochange, sibling. Those are evidence of a locus that a
	// vocabulary absence cannot overrule (invariant 2); lexical, documentation,
	// test and reserved rows are not counted.
	relations         int
	required, support int
	supporting        int
	nearest           []nearestClaim
}

// reservedOnly keeps the governing, spec-mentioned and instruction-routed rows
// an unsupported conjunction does not withdraw.
func (compiler *taskContextCompiler) reservedOnly(rows []contextRow) []contextRow {
	kept := make([]contextRow, 0)
	for _, row := range rows {
		if reservedRelation(row.kind) {
			kept = append(kept, row)
		}
	}
	return kept
}

func reservedRelation(kind string) bool {
	return kind == governingRelation || kind == specMentionedRelation || kind == instructionRoutedRelation
}

// withheldClaims are the first slot rows the verdict withholds, as the
// nearest claims: what the packet would have said, and every name each lacks.
func (compiler *taskContextCompiler) withheldClaims(rows []contextRow) []nearestClaim {
	claims := make([]nearestClaim, 0, nearestClaimCap)
	for _, row := range rows {
		if reservedRelation(row.kind) || len(claims) == nearestClaimCap {
			continue
		}
		lacks := make([]string, 0, len(compiler.answerability.terms))
		for _, term := range compiler.answerability.terms {
			lacks = append(lacks, term.term)
		}
		claims = append(claims, nearestClaim{path: row.path, supports: []string{}, lacks: lacks})
	}
	return claims
}

func (verdict answerability) unsupported() bool {
	return verdict.namesUnsupported() && verdict.relations == 0
}

func (verdict answerability) namesUnsupported() bool {
	return len(verdict.terms) >= 2 && verdict.known == 0
}

// relationRows counts only the rescue relations named by TCP-V0-016.
// Test rows may derive solely from lexical anchors, so neither they nor
// unrecognized future row kinds establish independent support for a task.
func relationRows(rows []contextRow) int {
	count := 0
	for _, row := range rows {
		switch row.kind {
		case "mentioned", "pair", "definition", "reverse-import", "reference", "cochange", "sibling":
			count++
		}
	}
	return count
}

// unnamed is the specific terms no source names, backticked for a reason.
func (verdict answerability) unnamed() []string {
	result := make([]string, 0)
	for _, term := range verdict.terms {
		if !term.known() {
			result = append(result, "`"+term.term+"`")
		}
	}
	return result
}

// taskNames is the task's names as written. A task may be a JSON document:
// its escape sequences are whitespace and its object keys are not task text,
// so a `\n` before a capitalised word does not forge a camel-case name and a
// key such as `failure_excerpt` is not a refuted claim.
func taskNames(task string) []string {
	text := contextEscape.ReplaceAllString(contextJSONKey.ReplaceAllString(task, " "), " ")
	seen := map[string]struct{}{}
	names := make([]string, 0)
	add := func(token string) {
		if _, ok := seen[token]; ok {
			return
		}
		seen[token] = struct{}{}
		names = append(names, token)
	}
	for _, match := range contextBacktick.FindAllStringSubmatch(text, -1) {
		for _, part := range strings.Split(match[1], ".") {
			if len(part) >= 3 {
				add(part)
			}
		}
	}
	for _, token := range contextIdentifier.FindAllString(text, -1) {
		if nameShaped(token) {
			add(token)
		}
	}
	return names
}

// nameShaped is an identifier with an inner camel-case boundary or an inner
// underscore once its outer underscores are trimmed: `_PyObject_Malloc` is a
// name, the markdown italics `_No response_` and a rule `_______` are not.
func nameShaped(token string) bool {
	inner := strings.Trim(token, "_")
	return hasInnerCamelBoundary(inner) || strings.Contains(inner, "_")
}

// specificTerms resolves each name's sources: the sources writing it as an
// identifier, the sources carrying it lowercased as one body term, and the
// sources whose path contains it (`listeners_test` is a name only the path
// `listeners_test.go` carries). Names at or above the cut are not specific.
func (compiler *taskContextCompiler) specificTerms() []specificTerm {
	table := compiler.index.vocabulary()
	cut := max(len(table.Paths)/specificTermFraction, 2)
	names := taskNames(compiler.task)
	terms := make([]specificTerm, 0, len(names))
	tracked := compiler.trackedPaths()
	for _, name := range names {
		sources := unionSources(postingSources(&table.Words, name), postingSources(&table.Terms, strings.ToLower(name)))
		sources = unionSources(sources, pathSources(table, name))
		if len(sources) < cut {
			terms = append(terms, specificTerm{term: name, sources: sources, tracked: trackedNaming(tracked, name)})
		}
	}
	sort.Slice(terms, func(left, right int) bool { return terms[left].term < terms[right].term })
	return terms
}

func postingSources(postings *termPostings, key string) []uint32 {
	low, high, ok := postings.find(key)
	if !ok {
		return nil
	}
	return postings.Sources[low:high]
}

func pathSources(table *TermTable, name string) []uint32 {
	lower := strings.ToLower(name)
	sources := make([]uint32, 0)
	for source, candidate := range table.Paths {
		if strings.Contains(strings.ToLower(candidate), lower) {
			sources = append(sources, uint32(source))
		}
	}
	return sources
}

func trackedNaming(tracked map[string]struct{}, name string) int {
	lower := strings.ToLower(name)
	count := 0
	for candidate := range tracked {
		if strings.Contains(strings.ToLower(candidate), lower) {
			count++
		}
	}
	return count
}

func unionSources(left, right []uint32) []uint32 {
	merged := make([]uint32, 0, len(left)+len(right))
	merged = append(merged, left...)
	merged = append(merged, right...)
	slices.Sort(merged)
	return slices.Compact(merged)
}

// answer counts, per source, the specific terms it names and keeps the best
// support beside the sources that reach it.
func (compiler *taskContextCompiler) answer() answerability {
	table := compiler.index.vocabulary()
	terms := compiler.specificTerms()
	verdict := answerability{terms: terms}
	counts := make([]uint16, len(table.Paths))
	for _, term := range terms {
		if term.known() {
			verdict.known++
		}
		for _, source := range term.sources {
			counts[source]++
		}
	}
	verdict.required = min(verdict.known, 2)
	for _, count := range counts {
		verdict.support = max(verdict.support, int(count))
	}
	for _, count := range counts {
		if int(count) >= verdict.required && count > 0 {
			verdict.supporting++
		}
	}
	if verdict.support > 0 {
		verdict.nearest = nearestClaims(table, terms, counts, verdict.support)
	}
	return verdict
}

// nearestClaims are the sources with the best support, by path, each with
// the specific terms it names and the ones it lacks.
func nearestClaims(table *TermTable, terms []specificTerm, counts []uint16, best int) []nearestClaim {
	claims := make([]nearestClaim, 0, nearestClaimCap)
	for source, count := range counts {
		if int(count) != best {
			continue
		}
		claims = append(claims, nearestClaim{path: table.Paths[source]})
	}
	sort.Slice(claims, func(left, right int) bool { return claims[left].path < claims[right].path })
	claims = claims[:min(len(claims), nearestClaimCap)]
	for index := range claims {
		id, _ := table.sourceID(claims[index].path)
		for _, term := range terms {
			if _, named := slices.BinarySearch(term.sources, uint32(id)); named {
				claims[index].supports = append(claims[index].supports, term.term)
				continue
			}
			claims[index].lacks = append(claims[index].lacks, term.term)
		}
	}
	return claims
}

// packet renders the verdict: every specific term with its posting length,
// the support required and found, a reason a reader can check against the
// index, and the claims made or withheld by the support verdict.
func (verdict answerability) packet(index *Index) map[string]any {
	specific := make([]any, 0, len(verdict.terms))
	for _, term := range verdict.terms {
		specific = append(specific, map[string]any{"term": term.term, "sources": len(term.sources), "tracked_paths": term.tracked})
	}
	result := map[string]any{
		"specific_terms": specific, "known": verdict.known,
		"required": verdict.required, "support": verdict.support, "supporting_sources": verdict.supporting,
		"reason": verdict.reason(),
	}
	if len(verdict.terms) == 0 {
		result["verdict"] = "no-specific-terms"
		return result
	}
	if verdict.namesUnsupported() && !verdict.unsupported() {
		result["verdict"] = "relations-answer"
		return result
	}
	if verdict.unsupported() {
		result["verdict"] = "unsupported-conjunction"
		result["nearest_claims"] = verdict.claimPackets(index)
		return result
	}
	if verdict.supporting == 0 {
		result["verdict"] = "not-withheld"
		result["nearest_claims"] = []any{}
		return result
	}
	result["verdict"] = "supported"
	result["nearest_claims"] = verdict.claimPackets(index)
	return result
}

func (verdict answerability) claimPackets(index *Index) []any {
	nearest := make([]any, 0, len(verdict.nearest))
	for _, claim := range verdict.nearest {
		source, pinned := index.Sources[claim.path]
		entry := map[string]any{
			"path": claim.path, "blob_hash": source.BlobHash,
			"supports": stringsToAny(claim.supports), "lacks": stringsToAny(claim.lacks),
		}
		if !pinned {
			// Same disclosure as a result row (TCP-V0-003): an empty
			// blob_hash here must not pass without saying why either
			// (AGENTS.md invariant 2).
			entry["evidence_gap"] = evidenceGapReason(index, claim.path)
		}
		nearest = append(nearest, entry)
	}
	return nearest
}

func (verdict answerability) reason() string {
	unnamed := verdict.unnamed()
	switch {
	case len(verdict.terms) == 0:
		return "the task writes no name fewer than one source in sixteen carries"
	case verdict.known == 0 && len(verdict.terms) == 1:
		return "one unknown name is a feature as often as an absence; no conjunction to test"
	case verdict.known == 0 && verdict.relations > 0:
		return fmt.Sprintf("no source names any specific term (%s), but %d rows are admitted by a relation the task names, which a vocabulary absence cannot overrule",
			strings.Join(unnamed, ", "), verdict.relations)
	case verdict.known == 0:
		return "no source names any specific term: " + strings.Join(unnamed, ", ")
	case len(unnamed) > 0:
		return fmt.Sprintf("%d sources name at least %d of the %d known specific terms together; no source names %s",
			verdict.supporting, verdict.required, verdict.known, strings.Join(unnamed, ", "))
	}
	return fmt.Sprintf("%d sources name at least %d of the %d known specific terms together",
		verdict.supporting, verdict.required, verdict.known)
}
