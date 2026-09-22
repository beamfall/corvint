package contextindex

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The authority trigger index answers one question about a set of changed paths:
// which project-owned authority documents already cite these paths, and is each
// citation still pinned to the content it named?
//
// The relation it reports is `cites`, never `governs`. A specification that cites
// a file may be constraining it or merely describing it, and deciding which is a
// reading of prose that no index can make (AGENTS.md invariant 2). What the index
// contributes is that the authority closure costs nothing until a change touches a
// cited path, and that when it does fire it names the citing document, its line,
// its blob, and its invariant-3 authority rank rather than a summary of them.
//
// Triggers are compiled from citations the documents already make, so there is no
// new authoring surface to keep in sync and every trigger is pinned to immutable
// content (invariant 1).

// authorityCitation matches one backticked full-path citation. Only the explicit
// `dir/file.ext:N` form is read: `check-line-citations.sh` also resolves a bare
// `:N` continuation and a bare basename, but both resolve through the surrounding
// prose, and a trigger asserted from a reading of prose is exactly the invented
// certainty invariant 2 forbids.
var authorityCitation = regexp.MustCompile("`([A-Za-z0-9_][A-Za-z0-9_./-]*/[A-Za-z0-9_.-]+\\.[a-z0-9]+):([0-9]+)((?:,[0-9]+)*)(?:-([0-9]+))?(?:@([0-9a-f]{4,64}))?`")

// The four states a citation's content anchor can be in. `unpinned` is the corpus
// default and is not a defect: DCG-V0-006 makes anchors opt-in per citation.
// `unreadable` separates "the cited bytes disagree" from "the cited bytes could
// not be read at this revision", which would otherwise both look like drift.
const (
	anchorPinned     = "pinned"
	anchorUnpinned   = "unpinned"
	anchorDrifted    = "drifted"
	anchorUnreadable = "unreadable"
)

// authorityRank and anchorRank order the trigger table. Authority comes first
// because invariant 3 is the ordering the reader cares about; anchor state breaks
// ties so that a citation still pinned to its content outranks one whose freshness
// is merely unknown, which in turn outranks one known to have drifted.
var authorityRank = map[string]int{
	"project-instructions": 0,
	"accepted-decision":    1,
	"accepted-spec":        2,
	"repository-spec":      3,
	"non-binding-decision": 4,
}

var anchorRank = map[string]int{
	anchorPinned:     0,
	anchorUnpinned:   1,
	anchorDrifted:    2,
	anchorUnreadable: 3,
}

// first..last is the written range when last is non-zero; extras is the written
// comma list. Neither is expanded until anchorState has bounded it against the
// cited target, so a citation naming an absurd range costs nothing to read.
type authorityCite struct {
	token, target, pin string
	line, first, last  int
	extras             []int
}

type authorityTriggerRow struct {
	target, document, kind, title, status string
	authority, confidence, anchor, token  string
	blobHash                              string
	line                                  int
}

// LookupAuthorityTriggers compiles the trigger table for paths and returns the
// standard lookup envelope. An empty table is reported as `NO_CANDIDATES`, which
// is the dormant case and the common one.
func LookupAuthorityTriggers(index *Index, paths []string, limit int) (map[string]any, error) {
	if err := validateLookupLimit(limit); err != nil {
		return nil, err
	}
	requested, err := authorityTriggerPaths(paths)
	if err != nil {
		return nil, err
	}
	rows, unread := authorityTriggerRows(index, requested)
	sortAuthorityTriggers(rows)
	included := rows[:minInt(limit, len(rows))]
	envelope := lookupEnvelope(index, "authority", map[string]any{"paths": stringsAny(paths)}, wireAuthorityTriggers(included), len(rows), limit)
	// Two blind spots, reported rather than absorbed into the silence: a path the
	// index does not track cannot have been scanned for citations, and a document
	// whose bytes did not load was never searched for them.
	envelope["untracked_paths"] = stringsAny(untrackedAuthorityPaths(index, paths))
	envelope["unread_documents"] = stringsAny(unread)
	return envelope, nil
}

func authorityTriggerPaths(paths []string) (map[string]struct{}, error) {
	if len(paths) == 0 {
		return nil, &Error{Message: "authority triggers require at least one path"}
	}
	requested := make(map[string]struct{}, len(paths))
	for _, candidate := range paths {
		cleaned, err := cleanImpactPath(candidate)
		if err != nil {
			return nil, err
		}
		requested[cleaned] = struct{}{}
	}
	return requested, nil
}

func untrackedAuthorityPaths(index *Index, paths []string) []string {
	untracked := make(map[string]struct{})
	for _, candidate := range paths {
		if _, tracked := index.Tracked[candidate]; !tracked {
			untracked[candidate] = struct{}{}
		}
	}
	return sortedStrings(keys(untracked))
}

// authorityTriggerRows scans every document record once and keeps the citations
// that name a requested path. It also returns the documents whose bytes it could
// not read, because a document that was never searched is a silence the caller
// must not read as an absence of authority.
func authorityTriggerRows(index *Index, requested map[string]struct{}) ([]authorityTriggerRow, []string) {
	rows := make([]authorityTriggerRow, 0)
	unread := make([]string, 0)
	for _, document := range sortedStrings(documentPaths(index)) {
		record := index.Documents[document]
		text, valid, loaded := index.Sources[record.Path].Text()
		if !valid || !loaded {
			unread = append(unread, document)
			continue
		}
		for _, cite := range documentCitations(text) {
			if _, wanted := requested[cite.target]; wanted {
				rows = append(rows, triggerRow(index, record, cite))
			}
		}
	}
	return rows, unread
}

func documentPaths(index *Index) []string {
	paths := make([]string, 0, len(index.Documents))
	for document := range index.Documents {
		paths = append(paths, document)
	}
	return paths
}

func sortedStrings(values []string) []string {
	sort.Strings(values)
	return values
}

// documentCitations returns every full-path citation the document body makes
// outside a fenced code block, in reading order.
func documentCitations(text string) []authorityCite {
	cites := make([]authorityCite, 0)
	fenced := false
	for offset, raw := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(raw), "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		for _, match := range authorityCitation.FindAllStringSubmatch(raw, -1) {
			cites = append(cites, citationFromMatch(offset+1, match))
		}
	}
	return cites
}

func citationFromMatch(line int, match []string) authorityCite {
	first, _ := strconv.Atoi(match[2])
	last, _ := strconv.Atoi(match[4])
	extras := make([]int, 0)
	for _, extra := range strings.Split(match[3], ",") {
		if number, err := strconv.Atoi(extra); err == nil {
			extras = append(extras, number)
		}
	}
	return authorityCite{
		token:  strings.Trim(match[0], "`"),
		target: match[1],
		pin:    match[5],
		line:   line,
		first:  first,
		last:   last,
		extras: extras,
	}
}

// citedNumbers is `anchored_numbers` in script/check-line-citations.sh: a range
// covers its endpoints inclusively and a comma list covers the listed lines in
// written order, with the range winning when both are written. The two must agree
// exactly, because that gate's --hash is the only supported way to mint a pin.
// It reports false, before expanding anything, when a cited line falls outside a
// target of `count` lines.
func (cite authorityCite) citedNumbers(count int) ([]int, bool) {
	inRange := func(number int) bool { return number >= 1 && number <= count }
	if cite.last != 0 {
		if cite.last >= cite.first && !(inRange(cite.first) && inRange(cite.last)) {
			return nil, false
		}
		numbers := make([]int, 0, maxInt(cite.last-cite.first+1, 0))
		for number := cite.first; number <= cite.last; number++ {
			numbers = append(numbers, number)
		}
		return numbers, true
	}
	numbers := append([]int{cite.first}, cite.extras...)
	for _, number := range numbers {
		if !inRange(number) {
			return nil, false
		}
	}
	return numbers, true
}

func triggerRow(index *Index, record Record, cite authorityCite) authorityTriggerRow {
	authority, confidence := documentAuthority(record)
	status, _ := record.Fields["status"].(string)
	return authorityTriggerRow{
		target: cite.target, document: record.Path, kind: record.Kind,
		title: stringValue(record.Fields["title"]), status: status,
		authority: authority, confidence: confidence, anchor: anchorState(index, cite),
		token: cite.token, blobHash: record.BlobHash, line: cite.line,
	}
}

// anchorState recomputes the citation's anchor over the cited path at the index
// revision. A pin therefore reports whether the cited bytes are still the bytes
// the author read at that revision, not whether an uncommitted edit changed them.
func anchorState(index *Index, cite authorityCite) string {
	if cite.pin == "" {
		return anchorUnpinned
	}
	text, valid, loaded := index.Sources[cite.target].Text()
	if !valid || !loaded {
		return anchorUnreadable
	}
	lines := strings.Split(text, "\n")
	numbers, inRange := cite.citedNumbers(len(lines))
	if !inRange {
		return anchorUnreadable
	}
	if strings.HasPrefix(anchorOf(lines, numbers), cite.pin) {
		return anchorPinned
	}
	return anchorDrifted
}

// anchorOf is `anchor_of` in script/check-line-citations.sh: each cited line
// stripped of trailing whitespace, joined by a single newline, SHA-256. Leading
// whitespace is retained because indentation is cited content.
func anchorOf(lines []string, numbers []int) string {
	cited := make([]string, 0, len(numbers))
	for _, number := range numbers {
		cited = append(cited, strings.TrimRight(lines[number-1], " \t\r\n\v\f"))
	}
	digest := sha256.Sum256([]byte(strings.Join(cited, "\n")))
	return hex.EncodeToString(digest[:])
}

func sortAuthorityTriggers(rows []authorityTriggerRow) {
	sort.Slice(rows, func(left, right int) bool {
		first, second := rows[left], rows[right]
		if first.target != second.target {
			return first.target < second.target
		}
		if authorityRank[first.authority] != authorityRank[second.authority] {
			return authorityRank[first.authority] < authorityRank[second.authority]
		}
		if anchorRank[first.anchor] != anchorRank[second.anchor] {
			return anchorRank[first.anchor] < anchorRank[second.anchor]
		}
		if first.document != second.document {
			return first.document < second.document
		}
		return first.line < second.line
	})
}

func wireAuthorityTriggers(rows []authorityTriggerRow) []map[string]any {
	wired := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		wired = append(wired, row.wire())
	}
	return wired
}

func (row authorityTriggerRow) wire() map[string]any {
	reason := "cites " + row.token + " at " + row.document + ":" + strconv.Itoa(row.line)
	return map[string]any{
		"path": row.target, "citation": row.token, "anchor": row.anchor, "relation": "cites",
		"kind": row.kind, "id": row.document, "title": row.title, "status": row.status,
		"authority": row.authority, "confidence": row.confidence,
		"evidence": []any{evidence(row.document, row.line, row.blobHash, reason, row.confidence, row.authority)},
	}
}
