package analyzerstructured

import (
	"bytes"
	"strings"
)

// parseHCL recognizes a static attribute-only HCL 2.0 subset. Blocks,
// traversals, templates, function calls and all expression operators are
// rejected; this profile does not claim to evaluate HCL.
func parseHCL(in decodedInput) string {
	if len(in.bytes) == 0 || in.bytes[len(in.bytes)-1] != '\n' || hasTextControl(in.bytes) {
		return "MALFORMED_INPUT"
	}
	if bytes.ContainsAny(in.bytes, "\r#{}()[]$") || bytes.Contains(in.bytes, []byte("//")) || bytes.Contains(in.bytes, []byte("/*")) || bytes.Contains(in.bytes, []byte("*/")) {
		return "UNSUPPORTED_SCHEMA"
	}
	seen := map[string]struct{}{}
	budget := &tokenBudget{}
	for _, raw := range bytes.Split(in.bytes[:len(in.bytes)-1], []byte{'\n'}) {
		line := string(raw)
		if line == "" || strings.HasPrefix(line, "//") {
			return "UNSUPPORTED_SCHEMA"
		}
		key, value, ok := strings.Cut(line, " = ")
		if !ok || !hclIdentifier(key) || !hclValue(value) {
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
func hclIdentifier(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for i, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || (i > 0 && c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}
func hclValue(value string) bool {
	if value == "true" || value == "false" || canonicalInteger(value) {
		return true
	}
	return len(value) >= 2 && len(value) <= maxStringBytes && value[0] == '"' && value[len(value)-1] == '"' && !strings.ContainsAny(value[1:len(value)-1], "\\\"")
}
