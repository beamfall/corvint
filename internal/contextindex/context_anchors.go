package contextindex

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/runtimeenv"
)

// TCP-V0-022: repository anchors are literals the term tokeniser would split
// or normalise away (a quoted error string, a URL, a scope-qualified enum
// value, a dotted configuration key, a `file.ext:line` stack frame). With
// `CORVINT_CONTEXT_ANCHORS=on` the task's anchors are matched verbatim
// against the source bodies as a fourth lexical field; the index and the
// snapshot are unchanged, so an anchor costs one Words posting walk per
// word it carries and one bounded body read per candidate.
const (
	contextAnchorMinBytes     = 4
	contextAnchorMaxBytes     = 256
	contextAnchorCap          = 16
	contextAnchorCandidateCap = 512
)

// contextAnchorClasses are tried in order; a span one class consumed is
// blanked before the next class runs, so a URL inside a quoted string is
// part of that string, not a second anchor.
var contextAnchorClasses = []struct {
	class   string
	pattern *regexp.Regexp
}{
	{"error", regexp.MustCompile(`"([^"\n]+)"`)},
	{"url", regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://[^\s"'<>()` + "`" + `]+`)},
	{"frame", regexp.MustCompile(`[A-Za-z0-9_./-]+\.[A-Za-z0-9]+:[0-9]+`)},
	{"enum", regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*(?:::[A-Za-z_][A-Za-z0-9_]*)+`)},
	{"config", regexp.MustCompile(`[a-z][a-z0-9_-]*(?:\.[a-z][a-z0-9_-]*)+`)},
}

// contextAnchorFileSuffixes keeps a bare file name (`parser.go`) out of the
// config class: the mentioned slot already answers paths.
var contextAnchorFileSuffixes = map[string]bool{
	"go": true, "py": true, "rs": true, "ts": true, "tsx": true, "js": true, "java": true, "kt": true,
	"rb": true, "cs": true, "cpp": true, "c": true, "h": true, "md": true, "json": true, "yaml": true,
	"yml": true, "toml": true, "txt": true,
}

type taskAnchor struct {
	class, literal string
}

// anchorHit is one anchor's verbatim occurrence count in one source.
type anchorHit struct {
	literal string
	count   int
}

func configureContextAnchors(compiler *taskContextCompiler) *taskContextCompiler {
	if runtimeenv.Value("CONTEXT_ANCHORS") != "on" {
		return compiler
	}
	compiler.anchors = taskAnchors(contextQueryValues(compiler.task))
	return compiler
}

// TaskHasAnchors reports whether task carries at least one TCP-V0-022
// anchor, by the extraction the compiler applies under
// CORVINT_CONTEXT_ANCHORS=on, within the compiler's task bound; the
// retrieval bench reads it to report the anchor-bearing samples as their own
// stratum.
func TaskHasAnchors(task string) bool {
	return len(task) <= maxTaskContextChars && len(taskAnchors(contextQueryValues(task))) > 0
}

// taskAnchors lists the task's anchors in text order, each class consuming
// its spans before the next, deduplicated by literal, capped at
// contextAnchorCap.
func taskAnchors(text string) []taskAnchor {
	working := []byte(text)
	found := make([]taskAnchor, 0)
	starts := make([]int, 0)
	for _, entry := range contextAnchorClasses {
		for _, span := range entry.pattern.FindAllSubmatchIndex(working, -1) {
			literal := anchorLiteral(entry.class, string(working[span[0]:span[1]]), span)
			if anchorAdmissible(entry.class, literal) {
				found = append(found, taskAnchor{class: entry.class, literal: literal})
				starts = append(starts, span[0])
			}
			blank(working, span[0], span[1])
		}
	}
	order := make([]int, len(found))
	for index := range order {
		order[index] = index
	}
	sort.SliceStable(order, func(left, right int) bool { return starts[order[left]] < starts[order[right]] })
	seen := map[string]bool{}
	anchors := make([]taskAnchor, 0, len(found))
	for _, index := range order {
		if seen[found[index].literal] || len(anchors) == contextAnchorCap {
			continue
		}
		seen[found[index].literal] = true
		anchors = append(anchors, found[index])
	}
	return anchors
}

// anchorLiteral is the matched span, without the quotes for the error class.
func anchorLiteral(class, matched string, span []int) string {
	if class == "error" {
		return matched[span[2]-span[0] : span[3]-span[0]]
	}
	return matched
}

func anchorAdmissible(class, literal string) bool {
	if len(literal) < contextAnchorMinBytes || len(literal) > contextAnchorMaxBytes {
		return false
	}
	if class == "config" && contextAnchorFileSuffixes[literal[strings.LastIndexByte(literal, '.')+1:]] {
		return false
	}
	return len(anchorWords(literal)) > 0
}

func blank(working []byte, start, end int) {
	for index := start; index < end; index++ {
		working[index] = ' '
	}
}

// anchorWords is the anchor's Words-table vocabulary: every maximal run of
// ASCII word bytes of at least three, as scanWords indexes them. Under the
// whole-anchor occurrence rule every such run of a verbatim occurrence is a
// word of the source, so the intersection of their postings is a superset of
// the sources carrying the anchor.
func anchorWords(literal string) []string {
	words := make([]string, 0)
	start := -1
	for index := 0; index <= len(literal); index++ {
		inWord := index < len(literal) && isWordByte(literal[index])
		if inWord {
			if start < 0 {
				start = index
			}
			continue
		}
		if start >= 0 && index-start >= 3 {
			words = append(words, literal[start:index])
		}
		start = -1
	}
	sort.Strings(words)
	return words
}

// anchorCandidates intersects the Words postings of the anchor's words; a
// word the table lacks empties the set. The result is ascending by source id.
func anchorCandidates(table *TermTable, words []string) []uint32 {
	var candidates []uint32
	for position, word := range words {
		low, high, ok := table.Words.find(word)
		if !ok {
			return nil
		}
		postings := table.Words.Sources[low:high]
		if position == 0 {
			candidates = append([]uint32(nil), postings...)
			continue
		}
		candidates = intersectAscending(candidates, postings)
	}
	return candidates
}

func intersectAscending(left, right []uint32) []uint32 {
	kept := left[:0]
	for len(left) > 0 && len(right) > 0 {
		switch {
		case left[0] < right[0]:
			left = left[1:]
		case left[0] > right[0]:
			right = right[1:]
		default:
			kept = append(kept, left[0])
			left, right = left[1:], right[1:]
		}
	}
	return kept
}

// countAnchor counts the whole-anchor occurrences of literal in text: an
// occurrence whose leading or trailing byte is a word byte is not extended by
// a word byte of the text, the whole-word rule the identifier field uses.
func countAnchor(text, literal string) int {
	count, offset := 0, 0
	for {
		position := strings.Index(text[offset:], literal)
		if position < 0 {
			return count
		}
		start := offset + position
		end := start + len(literal)
		if anchorBounded(text, literal, start, end) {
			count++
		}
		offset = start + 1
	}
}

func anchorBounded(text, literal string, start, end int) bool {
	if isWordByte(literal[0]) && start > 0 && isWordByte(text[start-1]) {
		return false
	}
	if isWordByte(literal[len(literal)-1]) && end < len(text) && isWordByte(text[end]) {
		return false
	}
	return true
}

// anchorOccurrences verifies one anchor over its candidates and returns the
// counted sources in ascending id order. More candidates than
// contextAnchorCandidateCap abstains: the anchor contributes nothing.
func (compiler *taskContextCompiler) anchorOccurrences(table *TermTable, literal string) ([]uint32, []int) {
	candidates := anchorCandidates(table, anchorWords(literal))
	if len(candidates) > contextAnchorCandidateCap {
		return nil, nil
	}
	sources, counts := make([]uint32, 0), make([]int, 0)
	for _, source := range candidates {
		text, ok := sourceTextBounded(compiler.index.Sources[table.Paths[source]])
		if !ok {
			continue
		}
		if count := countAnchor(text, literal); count > 0 {
			sources, counts = append(sources, source), append(counts, count)
		}
	}
	return sources, counts
}

// anchorReason is the TCP-V0-022 evidence prefix of a lexical row.
func anchorReason(hits []anchorHit) string {
	parts := make([]string, 0, len(hits))
	for _, hit := range hits {
		parts = append(parts, "`"+hit.literal+"` x"+strconv.Itoa(hit.count))
	}
	return "anchor: " + strings.Join(parts, ", ") + " verbatim; "
}
