package tcq

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Producers of OCM extractor `corvint-test-claim/1`. The re-derived anchor profile
// is exactly one producer plus `/1` (TCQ-V0-004/005).
const (
	producerPythonTestName  = "python-test-name"
	producerPythonDocstring = "python-docstring"
	producerGoTestName      = "go-test-name"
	producerGoTableCase     = "go-table-case"
)

var (
	goFunctionPattern = regexp.MustCompile(`\bfunc\s+(Test[A-Z][A-Za-z0-9_]*)\s*\(`)
	goCasePattern2    = regexp.MustCompile(`\b(?:(?:name|testName|test_name)\s*:|[A-Za-z_][A-Za-z0-9_]*\.Run\()\s*"((?:[^"\\]|\\.)*)"`)
)

// anchorCandidate is one re-extraction result: the producer that would have
// proposed this exact (selector, span) from the target blob.
type anchorCandidate struct {
	producer string
	selector string
	start    int
	end      int
}

// anchorProfile re-derives the anchor profile for one selected claim edge. Per
// TCQ-V0-004 it re-runs the extractor against the target blob and selector and
// trusts no caller-supplied language, anchor kind, span, source, runtime name,
// or profile label. Two producers agreeing on one span is not a profile.
func anchorProfile(candidates []anchorCandidate, selector string, start, end int64) (string, bool) {
	producers := map[string]bool{}
	for _, candidate := range candidates {
		if candidate.selector == selector && int64(candidate.start) == start && int64(candidate.end) == end {
			producers[candidate.producer] = true
		}
	}
	if len(producers) != 1 {
		return "", false
	}
	for producer := range producers {
		return producer + "/1", true
	}
	return "", false
}

// pythonCandidates re-extracts the Python claim anchors of one blob. It includes
// nested local functions because the extractor proposes them; TCQ-V0-013 then
// declines to give them a unit, which is what makes such an edge an association
// abstention rather than a silent match.
func pythonCandidates(analysis pythonAnalysis) []anchorCandidate {
	var candidates []anchorCandidate
	for _, function := range analysis.functions {
		selector := "test:" + function.name
		candidates = append(candidates, anchorCandidate{
			producer: producerPythonTestName, selector: selector,
			start: function.nameStart, end: function.nameEnd,
		})
		if function.hasDoc {
			candidates = append(candidates, anchorCandidate{
				producer: producerPythonDocstring, selector: selector + "#doc",
				start: function.docStart, end: function.docEnd,
			})
		}
	}
	return candidates
}

// goCandidates re-extracts the Go claim anchors of one blob. The extractor works
// over comment-masked bytes and a regex, independently of the `go-lexical/1`
// unit scan — so a comment lookalike proposes nothing and a case whose parent
// the scanner rejects still yields a profile with no unit.
func goCandidates(data []byte) []anchorCandidate {
	masked := maskGoComments(data)
	functions := goFunctionPattern.FindAllSubmatchIndex(masked, -1)
	var candidates []anchorCandidate
	for _, match := range functions {
		candidates = append(candidates, anchorCandidate{
			producer: producerGoTestName,
			selector: "test:" + string(masked[match[2]:match[3]]),
			start:    match[2], end: match[3],
		})
	}
	for _, match := range goCasePattern2.FindAllSubmatchIndex(masked, -1) {
		parent := enclosingGoTest(functions, masked, match[0])
		if parent == "" {
			continue
		}
		var value string
		if err := json.Unmarshal([]byte(`"`+string(masked[match[2]:match[3]])+`"`), &value); err != nil {
			continue
		}
		candidates = append(candidates, anchorCandidate{
			producer: producerGoTableCase,
			selector: "test:" + parent + "/case:" + selectorFragment(value),
			start:    match[2], end: match[3],
		})
	}
	return candidates
}

func enclosingGoTest(functions [][]int, masked []byte, position int) string {
	name := ""
	for _, match := range functions {
		if match[0] >= position {
			break
		}
		name = string(masked[match[2]:match[3]])
	}
	return name
}

// maskGoComments blanks comment bytes without mistaking a comment marker inside
// a string or rune literal, preserving every byte offset.
func maskGoComments(data []byte) []byte {
	masked := append([]byte(nil), data...)
	index := 0
	for index < len(data) {
		switch {
		case hasAt(data, index, "//"):
			index = blankUntilNewline(masked, index)
		case hasAt(data, index, "/*"):
			index = blankBlockComment(masked, data, index)
		case data[index] == '"' || data[index] == '\'' || data[index] == '`':
			index = skipGoLiteral(data, index)
		default:
			index++
		}
	}
	return masked
}

func hasAt(data []byte, index int, prefix string) bool {
	return strings.HasPrefix(string(data[index:min(index+len(prefix), len(data))]), prefix)
}

func blankUntilNewline(masked []byte, index int) int {
	for index < len(masked) && masked[index] != '\n' {
		masked[index] = ' '
		index++
	}
	return index
}

func blankBlockComment(masked, data []byte, index int) int {
	end := strings.Index(string(data[index:]), "*/")
	stop := len(data)
	if end >= 0 {
		stop = index + end + 2
	}
	for ; index < stop; index++ {
		if masked[index] != '\n' && masked[index] != '\r' {
			masked[index] = ' '
		}
	}
	return stop
}

func skipGoLiteral(data []byte, index int) int {
	quote := data[index]
	index++
	for index < len(data) {
		if quote != '`' && data[index] == '\\' {
			index += 2
			continue
		}
		if data[index] == quote {
			return index + 1
		}
		index++
	}
	return index
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}
