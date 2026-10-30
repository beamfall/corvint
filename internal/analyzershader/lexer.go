package analyzershader

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"sort"
	"strings"
)

type token struct {
	text       string
	start, end int
}

const (
	maxTokens  = 32_768
	maxNesting = 1_024
)

// sanitize keeps byte offsets/newlines stable while blanking comments and
// strings. Unterminated quotes/comments are terminal instead of guessed.
func sanitize(source []byte) ([]byte, string) {
	clean := append([]byte(nil), source...)
	state := byte(0)
	for i := 0; i < len(clean); i++ {
		c := clean[i]
		if state == '/' {
			if c == '\n' {
				state = 0
				continue
			}
			clean[i] = ' '
			continue
		}
		if state == '*' {
			if c == '\n' {
				continue
			}
			clean[i] = ' '
			if c == '*' && i+1 < len(clean) && clean[i+1] == '/' {
				clean[i+1] = ' '
				i++
				state = 0
			}
			continue
		}
		if state == '"' || state == '\'' {
			clean[i] = ' '
			if c == '\n' {
				return nil, "MALFORMED_INPUT"
			}
			if c == '\\' {
				if i+1 == len(clean) {
					return nil, "MALFORMED_INPUT"
				}
				clean[i+1] = ' '
				i++
				continue
			}
			if c == state {
				state = 0
			}
			continue
		}
		if c == '/' && i+1 < len(clean) && clean[i+1] == '/' {
			clean[i], clean[i+1] = ' ', ' '
			i++
			state = '/'
			continue
		}
		if c == '/' && i+1 < len(clean) && clean[i+1] == '*' {
			clean[i], clean[i+1] = ' ', ' '
			i++
			state = '*'
			continue
		}
		if c == '"' || c == '\'' {
			clean[i] = ' '
			state = c
			continue
		}
		if c == 0 || c == '\r' || (c < 0x20 && c != '\n' && c != '\t') || c > 0x7e {
			return nil, "UNSUPPORTED_SCHEMA"
		}
	}
	if state != 0 {
		return nil, "MALFORMED_INPUT"
	}
	return clean, ""
}

// blankDirectives removes already-validated preprocessor lines from the token
// grammar without changing any source offset. Facts continue to witness the
// original bytes, while declaration recognition cannot bridge a directive.
func blankDirectives(clean []byte) {
	_ = eachLine(clean, func(start, end int) string {
		for index := start; index < end; index++ {
			if clean[index] == ' ' || clean[index] == '\t' {
				continue
			}
			if clean[index] != '#' {
				return ""
			}
			for blank := index; blank < end; blank++ {
				clean[blank] = ' '
			}
			return ""
		}
		return ""
	})
}
func tokens(clean []byte) ([]token, string) {
	capacity := len(clean) / 8
	if capacity > maxTokens {
		capacity = maxTokens
	}
	values := make([]token, 0, capacity)
	appendToken := func(value token) bool {
		if len(values) == maxTokens {
			return false
		}
		values = append(values, value)
		return true
	}
	for i := 0; i < len(clean); {
		c := clean[i]
		if asciiSpace(c) {
			i++
			continue
		}
		if identifierByte(c) {
			start := i
			for i < len(clean) && identifierByte(clean[i]) {
				i++
			}
			if !appendToken(token{string(clean[start:i]), start, i}) {
				return nil, "LIMIT_EXCEEDED"
			}
			continue
		}
		if strings.ContainsRune("#()[]{};,=", rune(c)) {
			if !appendToken(token{string(c), i, i + 1}) {
				return nil, "LIMIT_EXCEEDED"
			}
			i++
			continue
		}
		if c == '.' || c == '+' || c == '-' || c == '*' || c == '/' || c == '<' || c == '>' || c == '!' || c == '&' || c == '|' || c == ':' {
			if !appendToken(token{string(c), i, i + 1}) {
				return nil, "LIMIT_EXCEEDED"
			}
			i++
			continue
		}
		return nil, "UNSUPPORTED_SCHEMA"
	}
	if reason := balancedDelimiters(values); reason != "" {
		return nil, reason
	}
	return values, ""
}

func balancedDelimiters(values []token) string {
	stack := make([]string, 0, 64)
	for _, value := range values {
		switch value.text {
		case "(", "[", "{":
			if len(stack) == maxNesting {
				return "LIMIT_EXCEEDED"
			}
			stack = append(stack, value.text)
		case ")", "]", "}":
			if len(stack) == 0 || !closes(stack[len(stack)-1], value.text) {
				return "MALFORMED_INPUT"
			}
			stack = stack[:len(stack)-1]
		}
	}
	if len(stack) != 0 {
		return "MALFORMED_INPUT"
	}
	return ""
}

func closes(open, close string) bool {
	return open == "(" && close == ")" || open == "[" && close == "]" || open == "{" && close == "}"
}
func asciiSpace(c byte) bool { return c == ' ' || c == '\n' || c == '\t' }
func identifierByte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_'
}

type sourceIndex struct {
	source     []byte
	lineStarts []int
}

type factFactory struct {
	request Request
	input   Input
	index   sourceIndex
	binding evidenceBinding
}

func newFactFactory(request Request, input Input, source []byte) (factFactory, string) {
	binding, ok := newEvidenceBinding(request)
	if !ok {
		return factFactory{}, "ANALYZER_FAILURE"
	}
	return factFactory{request: request, input: input, index: newSourceIndex(source), binding: binding}, ""
}

func newSourceIndex(source []byte) sourceIndex {
	starts := make([]int, 1, 1+bytes.Count(source, []byte{'\n'}))
	for offset, value := range source {
		if value == '\n' {
			starts = append(starts, offset+1)
		}
	}
	return sourceIndex{source: source, lineStarts: starts}
}

func (index sourceIndex) position(offset int) Position {
	line := sort.Search(len(index.lineStarts), func(value int) bool { return index.lineStarts[value] > offset }) - 1
	if line < 0 {
		line = 0
	}
	return Position{Byte: uint32(offset), Line: uint32(line + 1), Column: uint32(offset - index.lineStarts[line] + 1)}
}

func (factory factFactory) sourceFact(start, end int, kind, subject, predicate, value, instance string) (Fact, string) {
	source := factory.index.source
	if start < 0 || end < start || end > len(source) || end-start > maxWitness {
		return Fact{}, "LIMIT_EXCEEDED"
	}
	witness := source[start:end]
	digest := sha256.Sum256(witness)
	fact := Fact{Kind: kind, InputHandle: factory.input.Handle, RelatedHandle: "-", Subject: subject, Predicate: predicate, Value: value, InstanceID: instance, Span: Span{factory.index.position(start), factory.index.position(end)}, WitnessBase64: base64.StdEncoding.EncodeToString(witness), WitnessSHA256: "sha256:" + hex.EncodeToString(digest[:])}
	fact.EvidenceSHA256 = factory.binding.evidence(fact)
	if fact.EvidenceSHA256 == "" {
		return Fact{}, "ANALYZER_FAILURE"
	}
	return fact, ""
}

// sourceFact keeps the direct unit-test seam while parser production builds one
// index per input and calls sourceIndex.sourceFact for every bounded witness.
func sourceFact(request Request, input Input, source []byte, start, end int, kind, subject, predicate, value, instance string) (Fact, string) {
	factory, reason := newFactFactory(request, input, source)
	if reason != "" {
		return Fact{}, reason
	}
	return factory.sourceFact(start, end, kind, subject, predicate, value, instance)
}
