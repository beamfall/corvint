package contextindex

import (
	"fmt"
	"path"
	"slices"
	"sort"
	"strings"
)

// The `spec-cited` relation (TCP-V0-990..994, V1-1269) reserves the code a
// spec this task routes to names as its own surface: a spec-routed
// implementation task edits or reuses what its spec cites, which shares few
// terms with the task text and so loses every lexical position.
const (
	specCitedRelation = "spec-cited"
	// contextSpecCitedCap bounds the reserved spec-cited rows.
	contextSpecCitedCap = 3
	// contextSpecCitedSamples bounds the `omitted_spec_cited` samples: the
	// most rows any packet may carry, so a reader who wants one more file
	// finds it by path.
	contextSpecCitedSamples = maxLimit
)

// contextRankOrder is TCP-V0-011's relation order with `spec-cited` after the
// other reserved relations; it orders rows and selectors. The receipt lists
// `spec-cited` only when the relation ran (contextRelations).
var contextRankOrder = slices.Insert(slices.Clone(contextRelationOrder),
	slices.Index(contextRelationOrder, instructionRoutedRelation)+1, specCitedRelation)

func relationRank(kind string) int {
	return slices.Index(contextRankOrder, kind)
}

// specCitation is one tracked path a routed spec cites: the first citing spec
// and its spelling, and how often the routed specs cite it.
type specCitation struct {
	path, spec, cited string
	count, first      int
}

// specCitedRows reserves the tracked non-documentation paths the clean
// spec-mentioned rows' bodies cite in backticks, most-cited first, at most
// contextSpecCitedCap of them; every other candidate is named in
// `omitted_spec_cited`. Without a candidate the relation does not run and the
// packet keeps its bytes.
func (compiler *taskContextCompiler) specCitedRows(taken []contextRow) []contextRow {
	seen := make(map[string]struct{}, len(taken))
	specs := make([]string, 0, contextSpecMentionCap)
	for _, row := range taken {
		seen[row.path] = struct{}{}
		if row.kind == specMentionedRelation && row.downgrade == "" {
			specs = append(specs, row.path)
		}
	}
	citations, capped := compiler.specCitations(specs, seen)
	if len(citations) == 0 {
		return nil
	}
	compiler.relationState[specCitedRelation] = "examined"
	if capped {
		compiler.relationState[specCitedRelation] = "capped"
	}
	materialised := make([]string, 0, len(citations))
	rows := make([]contextRow, 0, min(len(citations), contextSpecCitedCap))
	for _, citation := range citations {
		materialised = append(materialised, citation.path)
		if len(rows) == contextSpecCitedCap {
			compiler.slotOmitted = true
			compiler.omittedSpecCited = append(compiler.omittedSpecCited, citation)
			continue
		}
		rows = append(rows, contextRow{
			kind: specCitedRelation, path: citation.path, score: 825, line: 1,
			summary: "cited by " + citation.spec + ", the spec this task routes to",
			reason:  specCitationReason(citation), confidence: "medium", authority: "document-reference",
		})
	}
	compiler.candidates[specCitedRelation] = materialised
	return rows
}

func specCitationReason(citation specCitation) string {
	return fmt.Sprintf("%s cites `%s` (%d citations)", citation.spec, citation.cited, citation.count)
}

// specCitations resolves every backquoted span of the specs, in spec order,
// to at most one tracked path each (specCitedPath), and orders the distinct
// paths by citation count, then first citation. A spec the bounded reader
// cannot hand back cites nothing and reports `capped`.
func (compiler *taskContextCompiler) specCitations(specs []string, seen map[string]struct{}) ([]specCitation, bool) {
	byPath := map[string]int{}
	citations := make([]specCitation, 0)
	capped := false
	position := 0
	for _, spec := range specs {
		text, loaded := sourceTextBounded(compiler.index.Sources[spec])
		if !loaded {
			capped = true
			continue
		}
		for _, match := range contextBackquoted.FindAllStringSubmatch(text, -1) {
			position++
			candidate, ok := compiler.specCitedPath(match[1])
			if !ok || candidate == compiler.subject || isDocumentationSuffix(candidate) {
				continue
			}
			if _, done := seen[candidate]; done {
				continue
			}
			if at, known := byPath[candidate]; known {
				citations[at].count++
				continue
			}
			byPath[candidate] = len(citations)
			citations = append(citations, specCitation{
				path: candidate, spec: spec, cited: strings.TrimSpace(match[1]), count: 1, first: position,
			})
		}
	}
	sort.SliceStable(citations, func(left, right int) bool {
		if citations[left].count != citations[right].count {
			return citations[left].count > citations[right].count
		}
		return citations[left].first < citations[right].first
	})
	return citations, capped
}

// specCitedPath resolves one backquoted span to an indexed path: its first
// word as a file (a trailing `:LINE` dropped), or as a directory through its
// entry file `main.go` or `<base>.go`; `make TARGET` as the root Makefile;
// and `NAME VERB` as `cmd/NAME/VERB.go`, the Go command layout.
func (compiler *taskContextCompiler) specCitedPath(span string) (string, bool) {
	words := strings.Fields(span)
	if len(words) == 0 {
		return "", false
	}
	first := strings.TrimPrefix(words[0], "./")
	if colon := strings.IndexByte(first, ':'); colon > 0 {
		first = first[:colon]
	}
	first = strings.TrimSuffix(first, "/")
	if compiler.indexed(first) {
		return first, true
	}
	if first != "" && first != "." {
		for _, entry := range []string{"main.go", path.Base(first) + ".go"} {
			if candidate := first + "/" + entry; compiler.indexed(candidate) {
				return candidate, true
			}
		}
	}
	if len(words) < 2 {
		return "", false
	}
	if words[0] == "make" && compiler.indexed("Makefile") {
		return "Makefile", true
	}
	if !strings.ContainsAny(words[0]+words[1], "/.") {
		if candidate := "cmd/" + words[0] + "/" + words[1] + ".go"; compiler.indexed(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func (compiler *taskContextCompiler) indexed(candidate string) bool {
	if candidate == "" {
		return false
	}
	_, ok := compiler.index.Sources[candidate]
	return ok
}

// omittedSpecCitedCoverage is TCP-V0-992: every spec-cited candidate the cap
// held back that no carried row names is listed with its reason. The member
// is present only when there is one and changes no ranking.
func (compiler *taskContextCompiler) omittedSpecCitedCoverage(coverage map[string]any, rows []contextRow) {
	carried := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		carried[row.path] = struct{}{}
	}
	omitted := make([]specCitation, 0, len(compiler.omittedSpecCited))
	for _, citation := range compiler.omittedSpecCited {
		if _, ok := carried[citation.path]; !ok {
			omitted = append(omitted, citation)
		}
	}
	if len(omitted) == 0 {
		return
	}
	shown := omitted[:min(len(omitted), contextSpecCitedSamples)]
	samples := make([]any, 0, len(shown))
	for _, citation := range shown {
		samples = append(samples, map[string]any{
			"path":   citation.path,
			"reason": fmt.Sprintf("%s; omitted: spec-cited cap %d", specCitationReason(citation), contextSpecCitedCap),
		})
	}
	coverage["omitted_spec_cited"] = map[string]any{"count": len(omitted), "samples": samples}
}
