package lrf

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

var requirementIDAtStart = regexp.MustCompile(`^[A-Z][A-Z0-9-]{2,31}-[0-9]{3}`)

var stopTerms = func() map[string]struct{} {
	words := strings.Fields(`
also been build change changed changes claim dist docs document documentation error evidence
false from generated have hunk include into must only package packages print report reports
requirement requirements return self shall should sidecar source sources string supported test
testing tests that their then there these they this those true unknown value vendor vendored when
where which while will with`)
	result := make(map[string]struct{}, len(words))
	for _, word := range words {
		result[word] = struct{}{}
	}
	return result
}()

var knownSidecars = map[string]struct{}{
	".corvint/change.cem.json": {},
	".corvint/change.ocm.json": {},
	".corvint/cem-review.md":   {},
	".corvint/ocm-review.md":   {},
}

func identifierTerms(data []byte, limit int) (map[string]struct{}, bool) {
	data = redactRequirementIDs(redactContentIDs(data))
	terms := make(map[string]struct{})
	for index := 0; index < len(data); {
		if !asciiLetter(data[index]) {
			index++
			continue
		}
		end := index + 1
		for end < len(data) && asciiIdentifier(data[end]) {
			end++
		}
		for _, segment := range splitIdentifier(data[index:end]) {
			lowered := asciiLower(segment)
			if !qualifyingSegment(lowered) {
				continue
			}
			term := string(lowered)
			if _, stopped := stopTerms[term]; stopped {
				continue
			}
			terms[term] = struct{}{}
			if len(terms) > limit {
				return nil, true
			}
		}
		index = end
	}
	return terms, false
}

func redactRequirementIDs(data []byte) []byte {
	var out []byte
	for index := 0; index < len(data); {
		if index > 0 && asciiIdentifier(data[index-1]) {
			out = append(out, data[index])
			index++
			continue
		}
		matched := requirementIDAtStart.FindIndex(data[index:])
		if matched == nil || matched[0] != 0 {
			out = append(out, data[index])
			index++
			continue
		}
		end := index + matched[1]
		if end < len(data) && asciiIdentifier(data[end]) {
			out = append(out, data[index])
			index++
			continue
		}
		out = append(out, ' ')
		index = end
	}
	return out
}

func redactContentIDs(data []byte) []byte {
	prefixes := [][]byte{[]byte("hunk:sha256:"), []byte("evidence:sha256:"), []byte("claim:sha256:")}
	var out []byte
	for index := 0; index < len(data); {
		matched := 0
		for _, prefix := range prefixes {
			end := index + len(prefix) + 64
			if end > len(data) || !bytes.Equal(data[index:index+len(prefix)], prefix) {
				continue
			}
			if index > 0 && !asciiNonIdentifier(data[index-1]) {
				continue
			}
			if end < len(data) && !asciiNonIdentifier(data[end]) {
				continue
			}
			if !lowerHex(data[index+len(prefix) : end]) {
				continue
			}
			matched = end - index
			break
		}
		if matched == 0 {
			out = append(out, data[index])
			index++
			continue
		}
		out = append(out, ' ')
		index += matched
	}
	return out
}

func splitIdentifier(run []byte) [][]byte {
	pieces := make([][]byte, 0, 4)
	start := 0
	for index := 1; index < len(run); index++ {
		previous := run[index-1]
		current := run[index]
		var following byte
		if index+1 < len(run) {
			following = run[index+1]
		}
		boundary := current == '_' || previous == '_'
		boundary = boundary || asciiLetter(previous) && asciiDigit(current)
		boundary = boundary || asciiDigit(previous) && asciiLetter(current)
		boundary = boundary || asciiLowerLetter(previous) && asciiUpperLetter(current)
		boundary = boundary || asciiUpperLetter(previous) && asciiUpperLetter(current) && asciiLowerLetter(following)
		if !boundary {
			continue
		}
		if start < index {
			piece := bytes.Trim(run[start:index], "_")
			if len(piece) > 0 {
				pieces = append(pieces, piece)
			}
		}
		if current == '_' {
			start = index + 1
		} else {
			start = index
		}
	}
	if start < len(run) {
		piece := bytes.Trim(run[start:], "_")
		if len(piece) > 0 {
			pieces = append(pieces, piece)
		}
	}
	return pieces
}

func asciiLower(data []byte) []byte {
	result := append([]byte(nil), data...)
	for index, value := range result {
		if asciiUpperLetter(value) {
			result[index] += 'a' - 'A'
		}
	}
	return result
}

func qualifyingSegment(segment []byte) bool {
	if len(segment) < 4 {
		return false
	}
	hasLetter := false
	hexOnly := len(segment) >= 8
	for _, value := range segment {
		hasLetter = hasLetter || asciiLowerLetter(value)
		hexOnly = hexOnly && ((value >= '0' && value <= '9') || (value >= 'a' && value <= 'f'))
	}
	return hasLetter && !hexOnly
}

func basenameStem(path string) []byte {
	if _, excluded := knownSidecars[path]; excluded {
		return nil
	}
	basename := path
	if slash := strings.LastIndexByte(path, '/'); slash >= 0 {
		basename = path[slash+1:]
	}
	if dot := strings.LastIndexByte(basename, '.'); dot >= 0 {
		basename = basename[:dot]
	}
	return []byte(basename)
}

func subjectTerms(body []byte, path string, limit int) (map[string]struct{}, bool) {
	bodyTerms, overflow := identifierTerms(body, limit)
	if overflow {
		return nil, true
	}
	pathTerms, overflow := identifierTerms(basenameStem(path), limit)
	if overflow {
		return nil, true
	}
	for term := range pathTerms {
		bodyTerms[term] = struct{}{}
		if len(bodyTerms) > limit {
			return nil, true
		}
	}
	return bodyTerms, false
}

func requirementBody(statement []byte, obligationID string) ([]byte, error) {
	prefix := []byte("- `" + obligationID + "`: ")
	if !bytes.HasPrefix(statement, prefix) {
		return nil, &Error{Code: "invalid-requirement-prefix", Message: "requirement line prefix is invalid"}
	}
	body := statement[len(prefix):]
	if bytes.HasSuffix(body, []byte("\r\n")) {
		body = body[:len(body)-2]
	} else if bytes.HasSuffix(body, []byte("\n")) {
		body = body[:len(body)-1]
	}
	if bytes.ContainsAny(body, "\r\n") {
		return nil, &Error{Code: "invalid-requirement-prefix", Message: "requirement must occupy one logical line"}
	}
	return body, nil
}

func intersects(left, right map[string]struct{}) bool {
	if len(left) > len(right) {
		left, right = right, left
	}
	for term := range left {
		if _, found := right[term]; found {
			return true
		}
	}
	return false
}

func asciiLetter(value byte) bool      { return asciiLowerLetter(value) || asciiUpperLetter(value) }
func asciiLowerLetter(value byte) bool { return value >= 'a' && value <= 'z' }
func asciiUpperLetter(value byte) bool { return value >= 'A' && value <= 'Z' }
func asciiDigit(value byte) bool       { return value >= '0' && value <= '9' }
func asciiIdentifier(value byte) bool  { return asciiLetter(value) || asciiDigit(value) || value == '_' }

// asciiNonIdentifier is a content-ID boundary byte: ASCII and outside [A-Za-z0-9_].
func asciiNonIdentifier(value byte) bool { return value < 0x80 && !asciiIdentifier(value) }

func lowerHex(data []byte) bool {
	for _, value := range data {
		if !asciiDigit(value) && (value < 'a' || value > 'f') {
			return false
		}
	}
	return len(data) > 0
}

func validPath(path string) bool { return validUTF8(path) && wire.ValidatePath(path) == nil }
