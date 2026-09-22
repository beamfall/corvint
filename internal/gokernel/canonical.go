package gokernel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"unicode/utf8"
)

// CanonicalJSON matches the compact, sorted, UTF-8 JSON used by the Python
// harness protocol. It intentionally returns no terminal newline so the bytes
// are suitable for receipt hashing.
func CanonicalJSON(value any) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, fmt.Errorf("canonical JSON: %w", err)
	}
	encoded := bytes.TrimSuffix(output.Bytes(), []byte{'\n'})
	// encoding/json always quotes the two JavaScript separators. Python's
	// ensure_ascii=False does not, so restore their canonical UTF-8 spelling.
	encoded = restoreJSONSeparators(encoded)
	return encoded, nil
}

func restoreJSONSeparators(encoded []byte) []byte {
	output := make([]byte, 0, len(encoded))
	for index := 0; index < len(encoded); {
		if encoded[index] != '\\' {
			output = append(output, encoded[index])
			index++
			continue
		}
		start := index
		for index < len(encoded) && encoded[index] == '\\' {
			index++
		}
		run := index - start
		separator := []byte(nil)
		if run%2 == 1 && index+5 <= len(encoded) {
			switch string(encoded[index : index+5]) {
			case "u2028":
				separator = []byte("\u2028")
			case "u2029":
				separator = []byte("\u2029")
			}
		}
		if separator == nil {
			output = append(output, encoded[start:index]...)
			continue
		}
		output = append(output, encoded[start:index-1]...)
		output = append(output, separator...)
		index += 5
	}
	return output
}

func decodeStrictJSON(raw []byte) (map[string]any, error) {
	if !utf8.Valid(raw) {
		return nil, newError("invalid-harness-input", "harness input must be one JSON object")
	}
	if err := rejectUnpairedEscapedSurrogates(raw); err != nil {
		return nil, err
	}
	if containsInvalidJSONConstant(raw) {
		return nil, newError("invalid-harness-input", "harness input contains a non-JSON value")
	}
	if exceedsJSONNesting(raw) {
		return nil, newError("invalid-harness-input", "harness input exceeds its nesting limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decodeValue(decoder, 0)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return nil, newError("invalid-harness-input", "harness input must be one JSON object")
		}
		return nil, newError("invalid-harness-input", "harness input must be one JSON object")
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, newError("invalid-harness-input", "harness input must be one JSON object")
	}
	return object, nil
}

func exceedsJSONNesting(raw []byte) bool {
	inString := false
	escaped := false
	depth := 0
	for _, value := range raw {
		if inString {
			if escaped {
				escaped = false
			} else if value == '\\' {
				escaped = true
			} else if value == '"' {
				inString = false
			}
			continue
		}
		switch value {
		case '"':
			inString = true
		case '{', '[':
			depth++
			if depth > maxJSONDepth+1 {
				return true
			}
		case '}', ']':
			if depth > 0 {
				depth--
			}
		}
	}
	return false
}

func containsInvalidJSONConstant(raw []byte) bool {
	inString := false
	escaped := false
	for index := 0; index < len(raw); index++ {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if raw[index] == '\\' {
				escaped = true
			} else if raw[index] == '"' {
				inString = false
			}
			continue
		}
		if raw[index] == '"' {
			inString = true
			continue
		}
		for _, constant := range []string{"NaN", "Infinity", "-Infinity"} {
			end := index + len(constant)
			if end <= len(raw) && string(raw[index:end]) == constant &&
				(index == 0 || isJSONBoundary(raw[index-1])) &&
				(end == len(raw) || isJSONBoundary(raw[end])) {
				return true
			}
		}
	}
	return false
}

func isJSONBoundary(value byte) bool {
	switch value {
	case ' ', '\t', '\r', '\n', ':', ',', '[', ']', '{', '}':
		return true
	default:
		return false
	}
}

func rejectUnpairedEscapedSurrogates(raw []byte) error {
	inString := false
	for index := 0; index < len(raw); index++ {
		switch raw[index] {
		case '"':
			inString = !inString
		case '\\':
			if !inString || index+1 >= len(raw) {
				continue
			}
			if raw[index+1] != 'u' {
				index++
				continue
			}
			if index+6 > len(raw) {
				continue // the JSON decoder reports the malformed escape
			}
			value, err := strconv.ParseUint(string(raw[index+2:index+6]), 16, 16)
			if err != nil {
				continue // the JSON decoder reports the malformed escape
			}
			switch {
			case value >= 0xd800 && value <= 0xdbff:
				if index+12 > len(raw) || raw[index+6] != '\\' || raw[index+7] != 'u' {
					return newError("invalid-harness-input", "harness JSON is not canonicalizable")
				}
				low, lowErr := strconv.ParseUint(string(raw[index+8:index+12]), 16, 16)
				if lowErr != nil || low < 0xdc00 || low > 0xdfff {
					return newError("invalid-harness-input", "harness JSON is not canonicalizable")
				}
				index += 11
			case value >= 0xdc00 && value <= 0xdfff:
				return newError("invalid-harness-input", "harness JSON is not canonicalizable")
			default:
				index += 5
			}
		}
	}
	return nil
}

const maxJSONDepth = 256

func decodeValue(decoder *json.Decoder, depth int) (any, error) {
	if depth > maxJSONDepth {
		return nil, newError("invalid-harness-input", "harness input exceeds its nesting limit")
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, newError("invalid-harness-input", "harness input must be one JSON object")
	}
	delimiter, composite := token.(json.Delim)
	if !composite {
		return token, nil
	}
	switch delimiter {
	case '{':
		object := make(map[string]any)
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, newError("invalid-harness-input", "harness input must be one JSON object")
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, newError("invalid-harness-input", "harness object key must be a string")
			}
			if _, exists := object[key]; exists {
				return nil, newError("invalid-harness-input", "harness input has duplicate keys")
			}
			value, err := decodeValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return nil, newError("invalid-harness-input", "harness input must be one JSON object")
		}
		return object, nil
	case '[':
		array := make([]any, 0)
		for decoder.More() {
			value, err := decodeValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return nil, newError("invalid-harness-input", "harness input must be one JSON object")
		}
		return array, nil
	default:
		return nil, newError("invalid-harness-input", "harness input must be one JSON object")
	}
}
