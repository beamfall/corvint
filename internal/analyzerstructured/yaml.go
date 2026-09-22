package analyzerstructured

import (
	"bytes"
	"strings"
)

// parseYAML intentionally recognizes one canonical YAML 1.2 core mapping
// subset. Tags, anchors, aliases, flow syntax, directives, scalars that can
// invoke implicit typing, and indentation-based nesting are not this profile.
func parseYAML(in decodedInput) string {
	if len(in.bytes) == 0 || in.bytes[len(in.bytes)-1] != '\n' || bytes.Contains(in.bytes, []byte{'\r'}) || hasTextControl(in.bytes) || bytes.HasPrefix(in.bytes, []byte{0xef, 0xbb, 0xbf}) {
		return "MALFORMED_INPUT"
	}
	seen := map[string]struct{}{}
	budget := &tokenBudget{}
	for _, raw := range bytes.Split(in.bytes[:len(in.bytes)-1], []byte{'\n'}) {
		line := string(raw)
		if line == "" || strings.HasPrefix(line, " ") || strings.ContainsAny(line, "#&*!|>{}[],@`\\") {
			return "UNSUPPORTED_SCHEMA"
		}
		key, value, ok := strings.Cut(line, ": ")
		if !ok || !yamlAtom(key) || !yamlScalar(key) || !yamlScalar(value) {
			return "MALFORMED_INPUT"
		}
		if !budget.consume(2) {
			return "LIMIT_EXCEEDED"
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
func yamlAtom(value string) bool {
	if len(value) == 0 || len(value) > maxStringBytes {
		return false
	}
	for _, c := range []byte(value) {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-' || c == '.') {
			return false
		}
	}
	return true
}
func yamlScalar(value string) bool {
	if len(value) == 0 || len(value) > maxStringBytes || strings.TrimSpace(value) != value {
		return false
	}
	if value == "-" || value == "?" || value == ":" {
		return false
	}
	lower := strings.ToLower(value)
	if lower == "true" || lower == "false" || lower == "null" {
		return value == lower
	}
	unsigned := strings.TrimPrefix(value, "-")
	if strings.HasPrefix(value, "+") || strings.HasPrefix(unsigned, ".") {
		return false
	}
	if lower == ".inf" || lower == ".nan" {
		return false
	}
	if canonicalInteger(value) {
		return true
	}
	if yamlAtom(value) && (unsigned[0] < '0' || unsigned[0] > '9') {
		return true
	}
	return strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") && len(value) >= 2 && !strings.ContainsAny(value[1:len(value)-1], "\\\"")
}
