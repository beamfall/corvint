package analyzerstructured

import (
	"bytes"
	"strconv"
	"strings"
)

// parseTOML is a closed TOML 1.0.0 key/value profile. Date/time literals,
// tables, arrays, inline tables and multiline values are rejected rather than
// being ambiguously interpreted by a generic TOML implementation.
func parseTOML(in decodedInput) string {
	if len(in.bytes) == 0 || in.bytes[len(in.bytes)-1] != '\n' || bytes.Contains(in.bytes, []byte{'\r'}) || hasTextControl(in.bytes) {
		return "MALFORMED_INPUT"
	}
	seen := map[string]struct{}{}
	budget := &tokenBudget{}
	for _, raw := range bytes.Split(in.bytes[:len(in.bytes)-1], []byte{'\n'}) {
		line := string(raw)
		if line == "" || strings.ContainsAny(line, "#[]{}") {
			return "UNSUPPORTED_SCHEMA"
		}
		key, value, ok := strings.Cut(line, " = ")
		if !ok || !tomlValue(value) {
			return "MALFORMED_INPUT"
		}
		if !budget.consume(2) {
			return "LIMIT_EXCEEDED"
		}
		if strings.ContainsRune(key, '.') {
			return "UNSUPPORTED_SCHEMA"
		}
		if !tomlKey(key) {
			return "MALFORMED_INPUT"
		}
		if _, duplicate := seen[key]; duplicate {
			return "DUPLICATE_VALUE"
		}
		seen[key] = struct{}{}
	}
	if len(seen) == 0 {
		return "MALFORMED_INPUT"
	}
	return ""
}
func tomlKey(value string) bool {
	if strings.ContainsRune(value, '.') {
		return false
	}
	return bareKey(value)
}

func bareKey(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, c := range []byte(value) {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}
func tomlValue(value string) bool {
	if value == "true" || value == "false" || canonicalInteger(value) {
		return true
	}
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' || len(value) > maxStringBytes || strings.ContainsAny(value[1:len(value)-1], "\\\"\n\r") {
		return false
	}
	return true
}
func canonicalInteger(value string) bool {
	if value == "0" {
		return true
	}
	if len(value) == 0 || value == "-0" {
		return false
	}
	digits := value
	if value[0] == '-' {
		digits = value[1:]
	}
	if len(digits) == 0 || digits[0] == '0' {
		return false
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return false
		}
	}
	_, err := strconv.ParseInt(value, 10, 64)
	return err == nil
}
