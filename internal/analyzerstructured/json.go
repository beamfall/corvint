package analyzerstructured

import (
	"bytes"
	"encoding/json"
	"io"
	"unicode/utf8"
)

type tokenBudget struct{ used int }

func (budget *tokenBudget) consume(count int) bool {
	if count < 0 || budget.used > maxTokenCount-count {
		return false
	}
	budget.used += count
	return true
}

// jsonUnique validates RFC 8259 structure while rejecting duplicate object keys
// before the standard decoder can overwrite one of them.
func jsonUnique(raw []byte, budget *tokenBudget) bool {
	return jsonUniqueBounded(raw, 0, budget, maxStringBytes)
}

// jsonEnvelope keeps the ordinary document-string limit separate from the
// content_base64 transport field. Envelope fields are constrained again by
// validateEnvelope before they can be reflected in a bound rejection.
func jsonEnvelope(raw []byte) bool {
	return jsonUniqueBounded(raw, 0, &tokenBudget{}, maxFrameBytes)
}

func jsonUniqueBounded(raw []byte, depth int, budget *tokenBudget, stringLimit int) bool {
	if !utf8.Valid(raw) || jsonHasLoneSurrogate(raw) {
		return false
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if !jsonValue(d, depth, budget, stringLimit) {
		return false
	}
	return d.Decode(new(any)) == io.EOF
}
func jsonValue(d *json.Decoder, depth int, budget *tokenBudget, stringLimit int) bool {
	if !budget.consume(1) {
		return false
	}
	tok, err := d.Token()
	if err != nil {
		return false
	}
	delim, isDelim := tok.(json.Delim)
	if !isDelim {
		if s, ok := tok.(string); ok && len(s) > stringLimit {
			return false
		}
		return true
	}
	switch delim {
	case '{':
		if depth == maxDepth {
			return false
		}
		seen := map[string]struct{}{}
		for d.More() {
			if !budget.consume(1) {
				return false
			}
			key, err := d.Token()
			if err != nil {
				return false
			}
			value, ok := key.(string)
			if !ok || len(value) > maxStringBytes {
				return false
			}
			if _, duplicate := seen[value]; duplicate {
				return false
			}
			seen[value] = struct{}{}
			if !jsonValue(d, depth+1, budget, stringLimit) {
				return false
			}
		}
		if !budget.consume(1) {
			return false
		}
		_, err := d.Token()
		return err == nil
	case '[':
		if depth == maxDepth {
			return false
		}
		for d.More() {
			if !jsonValue(d, depth+1, budget, stringLimit) {
				return false
			}
		}
		if !budget.consume(1) {
			return false
		}
		_, err := d.Token()
		return err == nil
	}
	return false
}

func jsonHasLoneSurrogate(raw []byte) bool {
	for index := 0; index < len(raw); {
		if raw[index] != '"' {
			index++
			continue
		}
		index++
		for index < len(raw) {
			if raw[index] == '"' {
				index++
				break
			}
			if raw[index] != '\\' {
				index++
				continue
			}
			if index+1 >= len(raw) {
				return false
			}
			if raw[index+1] != 'u' {
				index += 2
				continue
			}
			value, ok := jsonEscapeCodeUnit(raw[index:])
			if !ok {
				return false
			}
			if value >= 0xd800 && value <= 0xdbff {
				next, paired := jsonEscapeCodeUnit(raw[index+6:])
				if !paired || next < 0xdc00 || next > 0xdfff {
					return true
				}
				index += 12
				continue
			}
			if value >= 0xdc00 && value <= 0xdfff {
				return true
			}
			index += 6
		}
	}
	return false
}

func jsonEscapeCodeUnit(raw []byte) (uint16, bool) {
	if len(raw) < 6 || raw[0] != '\\' || raw[1] != 'u' {
		return 0, false
	}
	var value uint16
	for _, digit := range raw[2:6] {
		value <<= 4
		switch {
		case digit >= '0' && digit <= '9':
			value += uint16(digit - '0')
		case digit >= 'a' && digit <= 'f':
			value += uint16(digit-'a') + 10
		case digit >= 'A' && digit <= 'F':
			value += uint16(digit-'A') + 10
		default:
			return 0, false
		}
	}
	return value, true
}

func parseJSON(in decodedInput) string {
	if !jsonUnique(in.bytes, &tokenBudget{}) {
		return "MALFORMED_INPUT"
	}
	return ""
}
func parseJSONL(in decodedInput) string {
	if len(in.bytes) == 0 || in.bytes[len(in.bytes)-1] != '\n' || bytes.Contains(in.bytes, []byte{'\r'}) {
		return "MALFORMED_INPUT"
	}
	lines := bytes.Split(in.bytes[:len(in.bytes)-1], []byte{'\n'})
	if len(lines) == 0 || len(lines) > maxTokenCount {
		return "LIMIT_EXCEEDED"
	}
	budget := &tokenBudget{}
	for _, line := range lines {
		if len(line) == 0 || !jsonObject(line, budget) {
			return "MALFORMED_INPUT"
		}
	}
	return ""
}

func jsonObject(raw []byte, budget *tokenBudget) bool {
	trimmed := bytes.TrimLeft(raw, " \t")
	return len(trimmed) > 0 && trimmed[0] == '{' && jsonUnique(trimmed, budget)
}
