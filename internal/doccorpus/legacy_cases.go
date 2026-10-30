package doccorpus

import (
	"bytes"
	"encoding/csv"
	json "encoding/json/v2"
	"html"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Test-management export import (DCP-V1-070..079). A caller-exported TestRail
// JSON or CSV document becomes legacy cases in the BehaviorLegacyCase
// inventory shape with state `manual`: they are never executable, never a
// runtime baseline and never count as runtime parity. Corvint reads the file;
// it never contacts the test-management service.
const (
	LegacyCasesSchema      = "corvint-legacy-cases/1"
	LegacyCaseSearchSchema = "corvint-legacy-case-search/1"

	LegacyFormatTestRailJSON = "testrail-json"
	LegacyFormatCSV          = "csv"

	LegacyContentIDsPathsTitles = "ids-paths-titles"
	LegacyContentFull           = "full"

	// LegacyCasesMaxBytes bounds one export file; MaxRecords bounds its cases.
	LegacyCasesMaxBytes = 32 << 20

	legacyTitleBytes    = 1024
	legacyPathElemBytes = 512
	legacyPathDepth     = 32
	legacyPrecondBytes  = 8 << 10
	legacyStepBytes     = 4 << 10
	legacySteps         = 64
	legacyQueryBytes    = 1024
	legacyQueryTokens   = 32
)

// legacySearchFields are the fields a case search reads (DCP-V1-075).
var legacySearchFields = []string{"title", "path", "preconditions", "steps"}

// LegacyCaseSource identifies the one export a record or search read.
type LegacyCaseSource struct {
	ID     string `json:"id"`
	Format string `json:"format"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Cases  int    `json:"cases"`
}

// LegacyCaseStep is one step of a case; only `full` content emits it.
type LegacyCaseStep struct {
	Content  string `json:"content"`
	Expected string `json:"expected"`
}

// LegacyCaseContent is the step text `full` content emits. It is absent under
// ids-paths-titles.
type LegacyCaseContent struct {
	Preconditions string           `json:"preconditions"`
	Steps         []LegacyCaseStep `json:"steps"`
}

// LegacyCaseBound records one field the importer bounded: its stripped size
// and the bytes kept. A bounded field is never silently truncated.
type LegacyCaseBound struct {
	Field string `json:"field"`
	Bytes int    `json:"bytes"`
	Kept  int    `json:"kept"`
}

// ImportedLegacyCase is one imported case. Legacy holds the existing inventory
// shape keyed by the case ID; Digest is the sha256 of the case's full
// stripped and bounded content, so a step edit changes it under either
// content policy without emitting the step text.
type ImportedLegacyCase struct {
	Legacy  BehaviorLegacyCase `json:"legacy"`
	Path    []string           `json:"path"`
	Digest  string             `json:"digest"`
	Bounded []LegacyCaseBound  `json:"bounded"`
	Content *LegacyCaseContent `json:"content,omitempty"`
}

// LegacyCasesRecord is the import-cases output.
type LegacyCasesRecord struct {
	Schema        string               `json:"schema"`
	Content       string               `json:"content"`
	LocalOnly     bool                 `json:"local_only"`
	RuntimeParity bool                 `json:"runtime_parity"`
	Source        LegacyCaseSource     `json:"source"`
	Limitations   []string             `json:"limitations"`
	Cases         []ImportedLegacyCase `json:"cases"`
}

// LegacyCaseHit is one search result: case ID, path and title only.
type LegacyCaseHit struct {
	ID      string   `json:"id"`
	Path    []string `json:"path"`
	Title   string   `json:"title"`
	Matched []string `json:"matched_fields"`
	// Score is the ranked-mode score (DCP-V1-166); it is omitted under the
	// default all-token mode so that receipt stays byte-identical.
	Score int `json:"score,omitzero"`
}

// LegacyCaseBoundedHit names a searched case whose text was bounded, so a
// "none found" states which text was not read.
type LegacyCaseBoundedHit struct {
	ID     string   `json:"id"`
	Fields []string `json:"fields"`
}

// LegacyCaseSearch is the auditable search receipt (DCP-V1-075..077).
type LegacyCaseSearch struct {
	Schema      string                 `json:"schema"`
	Query       string                 `json:"query"`
	ID          string                 `json:"id,omitzero"`
	Mode        string                 `json:"mode,omitzero"`
	Tokens      []string               `json:"tokens"`
	Dropped     []string               `json:"stop_words_dropped,omitzero"`
	Match       string                 `json:"match"`
	Fields      []string               `json:"fields"`
	Searched    []LegacyCaseSource     `json:"searched"`
	State       string                 `json:"state"`
	Total       int                    `json:"total"`
	Limit       int                    `json:"limit"`
	Results     []LegacyCaseHit        `json:"results"`
	Bounded     []LegacyCaseBoundedHit `json:"bounded"`
	Limitations []string               `json:"limitations"`
}

// legacyCase is one normalized case before the content policy applies.
type legacyCase struct {
	id, title     string
	path          []string
	preconditions string
	steps         []LegacyCaseStep
	bounded       []LegacyCaseBound
}

// ReadLegacyCasesFile reads one bounded export file under root.
func ReadLegacyCasesFile(root, path string) ([]byte, error) {
	return readFileBounded(root, path, LegacyCasesMaxBytes)
}

// ImportLegacyCases converts one export into a legacy-cases record
// (DCP-V1-070..074). path is the export's root-relative path, recorded with
// its digest.
func ImportLegacyCases(format, content, path string, data []byte) ([]byte, error) {
	if content == "" {
		content = LegacyContentIDsPathsTitles
	}
	if content != LegacyContentIDsPathsTitles && content != LegacyContentFull {
		return nil, fail("legacy case content policy must be ids-paths-titles or full")
	}
	cases, source, err := parseLegacyCases(format, path, data)
	if err != nil {
		return nil, err
	}
	record := LegacyCasesRecord{Schema: LegacyCasesSchema, Content: content, LocalOnly: content == LegacyContentFull, Source: source, Cases: []ImportedLegacyCase{},
		Limitations: []string{
			"manual legacy cases are not executable and never count as runtime parity",
			"legacy evidence anchors name the export file and digest, not a repository revision; bind them to a committed revision before a behavior registry admits them",
			"case HTML is stripped by a bounded tag scanner, not an HTML parser",
		}}
	if content == LegacyContentFull {
		record.Limitations = append(record.Limitations, "full content retains preconditions, steps and expected results; keep this record local and do not commit or share it")
	} else {
		record.Limitations = append(record.Limitations, "ids-paths-titles omits preconditions, steps and expected results; the digest still covers them")
	}
	for _, c := range cases {
		digest, err := c.digest()
		if err != nil {
			return nil, err
		}
		item := ImportedLegacyCase{Path: c.path, Digest: digest, Bounded: c.bounded,
			Legacy: BehaviorLegacyCase{ID: c.id, Suite: strings.Join(c.path, " > "), Case: c.title, State: "manual", Fixtures: []string{}, Roles: []string{}, Criteria: []BehaviorLegacyCriterion{},
				Evidence: Anchor{Path: source.Path, SHA256: source.SHA256, SpanSHA256: digest, Region: "case:" + c.id, Authority: "external-provider", Kind: "imported", Reason: "test-management export case"}}}
		if content == LegacyContentFull {
			item.Content = &LegacyCaseContent{Preconditions: c.preconditions, Steps: c.steps}
		}
		record.Cases = append(record.Cases, item)
	}
	return Encode(record)
}

// Legacy case search modes (DCP-V1-165..168). MatchAll is the default and
// keeps the original receipt bytes; MatchAny ranks; the id mode is set by an
// ID lookup.
const (
	LegacyMatchAll = "all"
	LegacyMatchAny = "any"
	legacyMatchID  = "id"
)

// LegacyCaseQuery is one case search request. Exactly one of Query and ID is
// set; Match applies to Query only and defaults to all.
type LegacyCaseQuery struct {
	Query string
	ID    string
	Match string
	Limit int
}

// legacyFieldWeights rank a matched token by the best field it matched in
// (title > path > preconditions = steps) under MatchAny (DCP-V1-166).
var legacyFieldWeights = map[string]int{"title": 4, "path": 2, "preconditions": 1, "steps": 1}

// legacyStopWords is the fixed English function-word list MatchAny drops from
// the query (DCP-V1-166). MatchAll never drops a word.
var legacyStopWords = map[string]bool{}

func init() {
	for _, word := range strings.Fields("a an and are as at be by can do does for from how i in is it my of on or should that the this to was we what when where which who why will with") {
		legacyStopWords[word] = true
	}
}

// SearchLegacyCases searches one export's title, suite/section path,
// preconditions and steps (DCP-V1-075..077). Every query token must be a
// case-insensitive prefix of a word in a searched field. The receipt names
// the query, its tokens, the searched fields and the searched source, and
// returns only case ID, path and title.
func SearchLegacyCases(format, path string, data []byte, query string, limit int) ([]byte, error) {
	return SearchLegacyCasesQuery(format, path, data, LegacyCaseQuery{Query: query, Limit: limit})
}

// SearchLegacyCasesQuery runs one case search with an explicit mode
// (DCP-V1-165..168). An ID lookup returns exactly that case. A query that is
// exactly one `C<digits>` word naming a case in the export is that ID lookup;
// when no case has that ID the query searches as before, so no other query's
// result changes. MatchAny ranks cases by score, then export order.
func SearchLegacyCasesQuery(format, path string, data []byte, q LegacyCaseQuery) ([]byte, error) {
	if q.ID != "" && (q.Query != "" || q.Match != "") {
		return nil, fail("legacy case search takes a query or an id, not both, and an id lookup takes no match mode")
	}
	if q.Match == "" {
		q.Match = LegacyMatchAll
	}
	if q.Match != LegacyMatchAll && q.Match != LegacyMatchAny {
		return nil, fail("legacy case match must be all or any")
	}
	if q.Limit < 1 || q.Limit > MaxResults {
		return nil, fail("legacy case search limit out of bounds")
	}
	if q.ID != "" {
		id, ok := legacyCaseID(q.ID)
		if !ok || strings.TrimSpace(q.ID) != q.ID {
			return nil, fail("legacy case id must be C or a bare number of 1..10 digits without a leading zero")
		}
		cases, source, err := parseLegacyCases(format, path, data)
		if err != nil {
			return nil, err
		}
		return legacyIDReceipt(q.ID, "", id, cases, source, q.Limit)
	}
	if len(q.Query) > legacyQueryBytes || !utf8.ValidString(q.Query) {
		return nil, fail("legacy case query must be valid UTF-8 of at most 1024 bytes")
	}
	tokens := legacyTokens(q.Query)
	if len(tokens) == 0 || len(tokens) > legacyQueryTokens {
		return nil, fail("legacy case query needs 1..32 letter or digit words")
	}
	scored, dropped := tokens, []string(nil)
	if q.Match == LegacyMatchAny {
		scored = []string{}
		for _, token := range tokens {
			if legacyStopWords[token] {
				dropped = append(dropped, token)
			} else {
				scored = append(scored, token)
			}
		}
		if len(scored) == 0 {
			return nil, fail("legacy case query has only stop words under match any")
		}
	}
	cases, source, err := parseLegacyCases(format, path, data)
	if err != nil {
		return nil, err
	}
	if id, ok := legacyQueryID(q.Query); ok {
		for _, c := range cases {
			if c.id == id {
				return legacyIDReceipt("", q.Query, id, cases, source, q.Limit)
			}
		}
	}
	receipt := LegacyCaseSearch{Schema: LegacyCaseSearchSchema, Query: q.Query, Tokens: scored, Dropped: dropped,
		Match:  "every query token is a case-insensitive prefix of a letter-or-digit word in one of the fields",
		Fields: legacySearchFields, Searched: []LegacyCaseSource{source}, Limit: q.Limit, Results: []LegacyCaseHit{}, Bounded: []LegacyCaseBoundedHit{},
		Limitations: []string{
			"results carry case ID, path and title only; precondition and step text is searched but never returned",
			"expected results are not searched",
			"a none-found covers only the searched source and fields listed here, and the bounded text of the cases listed in bounded",
		}}
	if q.Match == LegacyMatchAny {
		receipt.Mode = LegacyMatchAny
		receipt.Match = "a case matches when at least one non-stop-word query token is a case-insensitive prefix of a letter-or-digit word in one of the fields; each matched token scores its best field weight (title 4, path 2, preconditions 1, steps 1); results are ordered by score descending, then export order"
		receipt.Limitations = append(receipt.Limitations, "match any drops a fixed English stop-word list from the query; a dropped word is listed in stop_words_dropped and never scores")
	}
	hits := []LegacyCaseHit{}
	for _, c := range cases {
		fields := map[string][]string{"title": legacyTokens(c.title), "path": legacyTokens(strings.Join(c.path, " ")), "preconditions": legacyTokens(c.preconditions)}
		steps := []string{}
		for _, s := range c.steps {
			steps = append(steps, s.Content)
		}
		fields["steps"] = legacyTokens(strings.Join(steps, " "))
		if bounded := c.searchedBounds(); len(bounded) > 0 {
			receipt.Bounded = append(receipt.Bounded, LegacyCaseBoundedHit{ID: c.id, Fields: bounded})
		}
		if q.Match == LegacyMatchAny {
			if matched, score := legacyScore(scored, fields); score > 0 {
				hits = append(hits, LegacyCaseHit{ID: c.id, Path: c.path, Title: c.title, Matched: matched, Score: score})
			}
			continue
		}
		matched, ok := legacyMatch(tokens, fields)
		if !ok {
			continue
		}
		receipt.Total++
		if len(receipt.Results) < q.Limit {
			receipt.Results = append(receipt.Results, LegacyCaseHit{ID: c.id, Path: c.path, Title: c.title, Matched: matched})
		}
	}
	if q.Match == LegacyMatchAny {
		sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
		receipt.Total = len(hits)
		receipt.Results = hits[:min(len(hits), q.Limit)]
	}
	receipt.State = "found"
	if receipt.Total == 0 {
		receipt.State = "none-found"
	}
	return Encode(receipt)
}

// legacyQueryID reports whether the query is exactly one `C<digits>` word
// (no surrounding space or other text), naming a case ID (DCP-V1-167).
func legacyQueryID(query string) (string, bool) {
	if !strings.HasPrefix(query, "C") {
		return "", false
	}
	id, ok := legacyCaseID(query)
	return id, ok && id == query
}

// legacyIDReceipt is the receipt of an ID lookup: exactly the named case, or
// none-found when the export has no case with that ID (DCP-V1-165).
func legacyIDReceipt(rawID, query, id string, cases []legacyCase, source LegacyCaseSource, limit int) ([]byte, error) {
	receipt := LegacyCaseSearch{Schema: LegacyCaseSearchSchema, Query: query, ID: rawID, Mode: legacyMatchID, Tokens: []string{},
		Match:  "the case ID equals " + id,
		Fields: []string{"id"}, Searched: []LegacyCaseSource{source}, State: "none-found", Limit: limit, Results: []LegacyCaseHit{}, Bounded: []LegacyCaseBoundedHit{},
		Limitations: []string{
			"results carry case ID, path and title only",
			"an id lookup reads only case IDs; a none-found covers only the searched source listed here",
		}}
	for _, c := range cases {
		if c.id == id {
			receipt.State, receipt.Total = "found", 1
			receipt.Results = append(receipt.Results, LegacyCaseHit{ID: c.id, Path: c.path, Title: c.title, Matched: []string{"id"}})
		}
	}
	return Encode(receipt)
}

// legacyScore sums, over the tokens that prefix a word of some field, the
// best weight among the fields each token matched, and names the fields that
// held a match in search-field order.
func legacyScore(tokens []string, fields map[string][]string) ([]string, int) {
	hit, score := map[string]bool{}, 0
	for _, token := range tokens {
		best := 0
		for _, field := range legacySearchFields {
			for _, word := range fields[field] {
				if strings.HasPrefix(word, token) {
					hit[field] = true
					best = max(best, legacyFieldWeights[field])
					break
				}
			}
		}
		score += best
	}
	matched := []string{}
	for _, field := range legacySearchFields {
		if hit[field] {
			matched = append(matched, field)
		}
	}
	return matched, score
}

// legacyMatch reports whether every token prefixes a word of some field, and
// which fields held a match, in search-field order.
func legacyMatch(tokens []string, fields map[string][]string) ([]string, bool) {
	hit := map[string]bool{}
	for _, token := range tokens {
		found := false
		for _, field := range legacySearchFields {
			for _, word := range fields[field] {
				if strings.HasPrefix(word, token) {
					hit[field], found = true, true
					break
				}
			}
		}
		if !found {
			return nil, false
		}
	}
	matched := []string{}
	for _, field := range legacySearchFields {
		if hit[field] {
			matched = append(matched, field)
		}
	}
	return matched, true
}

// legacyTokens lowercases text and splits it into unique letter-or-digit
// words in first-seen order.
func legacyTokens(text string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, word := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if !seen[word] {
			seen[word] = true
			out = append(out, word)
		}
	}
	return out
}

// searchedBounds names the bounded fields a search reads.
func (c legacyCase) searchedBounds() []string {
	out := []string{}
	for _, b := range c.bounded {
		if !strings.HasSuffix(b.Field, ".expected") {
			out = append(out, b.Field)
		}
	}
	return out
}

func (c legacyCase) digest() (string, error) {
	return hashValue(struct {
		ID            string           `json:"id"`
		Path          []string         `json:"path"`
		Title         string           `json:"title"`
		Preconditions string           `json:"preconditions"`
		Steps         []LegacyCaseStep `json:"steps"`
	}{c.id, c.path, c.title, c.preconditions, c.steps})
}

func parseLegacyCases(format, path string, data []byte) ([]legacyCase, LegacyCaseSource, error) {
	digest := Digest(data)
	source := LegacyCaseSource{ID: format + ":sha256:" + digest, Format: format, Path: path, SHA256: digest}
	if len(data) > LegacyCasesMaxBytes {
		return nil, source, fail("legacy case export exceeds its byte bound")
	}
	if !utf8.Valid(data) {
		return nil, source, fail("legacy case export is not UTF-8")
	}
	var cases []legacyCase
	var err error
	switch format {
	case LegacyFormatTestRailJSON:
		cases, err = parseTestRailJSON(data)
	case LegacyFormatCSV:
		cases, err = parseLegacyCSV(data)
	default:
		return nil, source, fail("legacy case format must be testrail-json or csv")
	}
	if err != nil {
		return nil, source, err
	}
	if len(cases) == 0 || len(cases) > MaxRecords {
		return nil, source, fail("legacy case export has no cases or exceeds 4096 cases")
	}
	seen := map[string]bool{}
	for _, c := range cases {
		if seen[c.id] {
			return nil, source, fail("legacy case export repeats case " + c.id)
		}
		seen[c.id] = true
	}
	sort.Slice(cases, func(i, j int) bool {
		if len(cases[i].id) != len(cases[j].id) {
			return len(cases[i].id) < len(cases[j].id)
		}
		return cases[i].id < cases[j].id
	})
	source.Cases = len(cases)
	return cases, source, nil
}

// legacyCaseID normalizes a TestRail case ID: `C` plus 1..10 digits without
// a leading zero; a bare number gains the `C`.
func legacyCaseID(raw string) (string, bool) {
	digits := strings.TrimPrefix(strings.TrimSpace(raw), "C")
	if digits == "" || len(digits) > 10 || digits[0] == '0' {
		return "", false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	return "C" + digits, true
}

type testRailExport struct {
	Suites []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"suites"`
	Sections []struct {
		ID       int64  `json:"id"`
		SuiteID  int64  `json:"suite_id"`
		ParentID *int64 `json:"parent_id"`
		Name     string `json:"name"`
	} `json:"sections"`
	Cases []struct {
		ID            int64  `json:"id"`
		Title         string `json:"title"`
		SectionID     *int64 `json:"section_id"`
		SuiteID       *int64 `json:"suite_id"`
		Preconditions string `json:"custom_preconds"`
		Steps         string `json:"custom_steps"`
		Expected      string `json:"custom_expected"`
		Separated     []struct {
			Content  string `json:"content"`
			Expected string `json:"expected"`
		} `json:"custom_steps_separated"`
	} `json:"cases"`
}

// parseTestRailJSON reads {suites, sections, cases} as the TestRail API
// returns them. Members Corvint does not read are ignored, because exports
// carry many tool-specific fields; duplicate members are refused.
func parseTestRailJSON(data []byte) ([]legacyCase, error) {
	var export testRailExport
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, fail("legacy case export is not a TestRail {suites, sections, cases} JSON document")
	}
	if len(export.Suites) > MaxRecords || len(export.Sections) > MaxRecords*4 {
		return nil, fail("legacy case export exceeds its suite or section bound")
	}
	suites := map[int64]string{}
	for _, s := range export.Suites {
		if _, dup := suites[s.ID]; dup {
			return nil, fail("legacy case export repeats suite " + strconv.FormatInt(s.ID, 10))
		}
		suites[s.ID] = s.Name
	}
	type section struct {
		suite  int64
		parent *int64
		name   string
	}
	sections := map[int64]section{}
	for _, s := range export.Sections {
		if _, dup := sections[s.ID]; dup {
			return nil, fail("legacy case export repeats section " + strconv.FormatInt(s.ID, 10))
		}
		sections[s.ID] = section{s.SuiteID, s.ParentID, s.Name}
	}
	out := []legacyCase{}
	for _, raw := range export.Cases {
		id, ok := legacyCaseID(strconv.FormatInt(raw.ID, 10))
		if !ok {
			return nil, fail("legacy case export has a case without a positive id")
		}
		names := []string{}
		suiteID := int64(0)
		hasSuite := false
		if raw.SuiteID != nil {
			suiteID, hasSuite = *raw.SuiteID, true
		}
		if raw.SectionID != nil {
			current, depth := *raw.SectionID, 0
			for {
				s, ok := sections[current]
				if !ok {
					return nil, fail("legacy case " + id + " names an unknown section")
				}
				if depth++; depth > legacyPathDepth {
					return nil, fail("legacy case " + id + " has a section cycle or a path deeper than 32")
				}
				if _, ok := suites[s.suite]; !ok {
					return nil, fail("legacy case " + id + " names a section in an unknown suite")
				}
				names = append([]string{s.name}, names...)
				if !hasSuite {
					suiteID, hasSuite = s.suite, true
				}
				if s.parent == nil {
					break
				}
				current = *s.parent
			}
		}
		if hasSuite {
			name, ok := suites[suiteID]
			if !ok {
				return nil, fail("legacy case " + id + " names an unknown suite")
			}
			names = append([]string{name}, names...)
		}
		steps := []LegacyCaseStep{}
		if raw.Steps != "" || raw.Expected != "" {
			steps = append(steps, LegacyCaseStep{Content: raw.Steps, Expected: raw.Expected})
		}
		for _, s := range raw.Separated {
			steps = append(steps, LegacyCaseStep{Content: s.Content, Expected: s.Expected})
		}
		c, err := normalizeLegacyCase(id, raw.Title, names, raw.Preconditions, steps)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// parseLegacyCSV reads a TestRail-style CSV export with a header row. ID and
// Title are required; Suite, Section, Section Hierarchy (" > " separated),
// Preconditions, Steps, Expected Result, Steps (Step) and Steps (Expected
// Result) are optional. A row with an empty ID continues the previous case's
// separated steps.
func parseLegacyCSV(data []byte) ([]legacyCase, error) {
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
	reader.FieldsPerRecord = 0
	header, err := reader.Read()
	if err != nil {
		return nil, fail("legacy case CSV has no header row")
	}
	columns := map[string]int{}
	for i, name := range header {
		key := strings.ToLower(strings.TrimSpace(name))
		if _, dup := columns[key]; dup {
			return nil, fail("legacy case CSV repeats column " + strings.TrimSpace(name))
		}
		columns[key] = i
	}
	if _, ok := columns["id"]; !ok {
		return nil, fail("legacy case CSV needs ID and Title columns")
	}
	if _, ok := columns["title"]; !ok {
		return nil, fail("legacy case CSV needs ID and Title columns")
	}
	type pending struct {
		id, title, preconditions string
		path                     []string
		steps                    []LegacyCaseStep
	}
	var rows []*pending
	for line := 2; ; line++ {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fail("legacy case CSV is malformed at record " + strconv.Itoa(line))
		}
		if len(rows) > MaxRecords {
			return nil, fail("legacy case export has no cases or exceeds 4096 cases")
		}
		cell := func(name string) string {
			if i, ok := columns[name]; ok {
				return row[i]
			}
			return ""
		}
		separated := LegacyCaseStep{Content: cell("steps (step)"), Expected: cell("steps (expected result)")}
		if strings.TrimSpace(cell("id")) == "" {
			if len(rows) == 0 || separated == (LegacyCaseStep{}) {
				return nil, fail("legacy case CSV record " + strconv.Itoa(line) + " has no case ID")
			}
			last := rows[len(rows)-1]
			last.steps = append(last.steps, separated)
			continue
		}
		id, ok := legacyCaseID(cell("id"))
		if !ok {
			return nil, fail("legacy case CSV record " + strconv.Itoa(line) + " has an invalid case ID")
		}
		path := []string{}
		if suite := strings.TrimSpace(cell("suite")); suite != "" {
			path = append(path, suite)
		}
		if hierarchy := cell("section hierarchy"); strings.TrimSpace(hierarchy) != "" {
			for _, part := range strings.Split(hierarchy, ">") {
				if part = strings.TrimSpace(part); part != "" {
					path = append(path, part)
				}
			}
		} else if section := strings.TrimSpace(cell("section")); section != "" {
			path = append(path, section)
		}
		p := &pending{id: id, title: cell("title"), preconditions: cell("preconditions"), path: path}
		if steps, expected := cell("steps"), cell("expected result"); steps != "" || expected != "" {
			p.steps = append(p.steps, LegacyCaseStep{Content: steps, Expected: expected})
		}
		if separated != (LegacyCaseStep{}) {
			p.steps = append(p.steps, separated)
		}
		rows = append(rows, p)
	}
	out := []legacyCase{}
	for _, p := range rows {
		c, err := normalizeLegacyCase(p.id, p.title, p.path, p.preconditions, p.steps)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// normalizeLegacyCase strips HTML from preconditions and steps, collapses
// whitespace and bounds every field, recording each bound (DCP-V1-072/073).
func normalizeLegacyCase(id, title string, path []string, preconditions string, steps []LegacyCaseStep) (legacyCase, error) {
	c := legacyCase{id: id, bounded: []LegacyCaseBound{}}
	bound := func(field, value string, limit int) string {
		if len(value) <= limit {
			return value
		}
		kept := limit
		for kept > 0 && !utf8.RuneStart(value[kept]) {
			kept--
		}
		c.bounded = append(c.bounded, LegacyCaseBound{Field: field, Bytes: len(value), Kept: kept})
		return value[:kept]
	}
	c.title = bound("title", collapseSpace(html.UnescapeString(title)), legacyTitleBytes)
	if strings.TrimSpace(c.title) == "" {
		return c, fail("legacy case " + id + " has no title")
	}
	if len(path) > legacyPathDepth {
		return c, fail("legacy case " + id + " has a path deeper than 32")
	}
	c.path = []string{}
	for i, elem := range path {
		if elem = collapseSpace(html.UnescapeString(elem)); elem != "" {
			c.path = append(c.path, bound("path["+strconv.Itoa(i)+"]", elem, legacyPathElemBytes))
		}
	}
	c.preconditions = bound("preconditions", stripLegacyHTML(preconditions), legacyPrecondBytes)
	c.steps = []LegacyCaseStep{}
	for i, s := range steps {
		if i == legacySteps {
			c.bounded = append(c.bounded, LegacyCaseBound{Field: "steps", Bytes: len(steps), Kept: legacySteps})
			break
		}
		at := "steps[" + strconv.Itoa(i) + "]"
		c.steps = append(c.steps, LegacyCaseStep{Content: bound(at+".content", stripLegacyHTML(s.Content), legacyStepBytes), Expected: bound(at+".expected", stripLegacyHTML(s.Expected), legacyStepBytes)})
	}
	return c, nil
}

// stripLegacyHTML removes tags, comments and script/style bodies, decodes
// entities and collapses whitespace. A `<` that does not open a tag is text.
// A tag ends at the first `>` outside a quoted attribute value; an
// unterminated tag, comment or script/style body drops the rest of the text.
func stripLegacyHTML(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '<' || i+1 >= len(s) || !(s[i+1] == '/' || s[i+1] == '!' || s[i+1] == '?' || s[i+1] < utf8.RuneSelf && unicode.IsLetter(rune(s[i+1]))) {
			b.WriteByte(s[i])
			i++
			continue
		}
		if strings.HasPrefix(s[i:], "<!--") {
			end := strings.Index(s[i+4:], "-->")
			if end < 0 {
				break
			}
			i += 4 + end + 3
			b.WriteByte(' ')
			continue
		}
		end := tagEnd(s[i:])
		if end < 0 {
			break
		}
		for _, raw := range []string{"script", "style"} {
			if hasTagName(s[i+1:], raw) {
				close := indexCloseTag(s[i+end:], raw)
				if close < 0 {
					return collapseSpace(html.UnescapeString(b.String()))
				}
				stop := tagEnd(s[i+end+close:])
				if stop < 0 {
					return collapseSpace(html.UnescapeString(b.String()))
				}
				end += close + stop
			}
		}
		i += end + 1
		b.WriteByte(' ')
	}
	return collapseSpace(html.UnescapeString(b.String()))
}

// hasASCIIPrefixFold reports whether s starts with the lower-case ASCII
// prefix p, ignoring ASCII case only, so byte offsets stay those of s.
func hasASCIIPrefixFold(s, p string) bool {
	if len(s) < len(p) {
		return false
	}
	for i := 0; i < len(p); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != p[i] {
			return false
		}
	}
	return true
}

// hasTagName reports whether s starts with the complete tag name, so
// `script` matches `script>` or `script type=...` but not `scripture`.
func hasTagName(s, name string) bool {
	if !hasASCIIPrefixFold(s, name) {
		return false
	}
	if len(s) == len(name) {
		return true
	}
	switch s[len(name)] {
	case '>', '/', ' ', '\t', '\n', '\r', '\f':
		return true
	}
	return false
}

// indexCloseTag is the byte offset in s of the first `</name` closing tag
// with a complete name, or -1.
func indexCloseTag(s, name string) int {
	for i := 0; i+2 <= len(s); i++ {
		if s[i] == '<' && s[i+1] == '/' && hasTagName(s[i+2:], name) {
			return i
		}
	}
	return -1
}

// tagEnd is the offset of the `>` that closes the tag opening s, skipping
// quoted attribute values, or -1 when the tag is unterminated.
func tagEnd(s string) int {
	var quote byte
	for i := 1; i < len(s); i++ {
		switch c := s[i]; {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '>':
			return i
		}
	}
	return -1
}

func collapseSpace(s string) string { return strings.Join(strings.Fields(s), " ") }
