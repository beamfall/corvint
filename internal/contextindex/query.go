package contextindex

import (
	"fmt"
	"sort"
	"strings"
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
	eachASCIIWord(text, func(raw string) {
		for token := range strings.SplitSeq(raw, "-") {
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
	})
}

// pythonLower lowercases as Python's str.lower does. The one unconditional
// special casing, U+0130 to "i" followed by U+0307, is applied before the
// simple mapping, so the tokenizer stays byte-compared with the oracle on
// every UTF-8 task.
func pythonLower(value string) string {
	return strings.ToLower(strings.ReplaceAll(value, "\u0130", "i\u0307"))
}

func camelSplit(value string) string {
	var output strings.Builder
	output.Grow(len(value) + len(value)/4)
	if isASCII(value) {
		// V1-2001: the same rule over bytes, without a rune copy.
		for index := 0; index < len(value); index++ {
			current := rune(value[index])
			if index > 0 {
				previous := rune(value[index-1])
				nextLower := index+1 < len(value) && asciiLower(rune(value[index+1]))
				if (asciiLower(previous) || asciiDigit(previous)) && asciiUpper(current) || asciiUpper(previous) && asciiUpper(current) && nextLower {
					output.WriteByte('-')
				}
			}
			output.WriteByte(value[index])
		}
		return output.String()
	}
	runes := []rune(value)
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

func isASCII(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] >= 0x80 {
			return false
		}
	}
	return true
}

// eachASCIIWord yields the words asciiWords returns, in order, without
// collecting them.
func eachASCIIWord(value string, yield func(string)) {
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
		yield(value[start:index])
	}
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

func asciiLetter(value rune) bool { return asciiLower(value) || asciiUpper(value) }
func asciiLower(value rune) bool  { return value >= 'a' && value <= 'z' }
func asciiUpper(value rune) bool  { return value >= 'A' && value <= 'Z' }
func asciiDigit(value rune) bool  { return value >= '0' && value <= '9' }
