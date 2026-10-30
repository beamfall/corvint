package analyzerstructured

import (
	"bytes"
	"strings"
)

// parseProperties accepts a closed Java-properties UTF-8 form: one ASCII key,
// one '=', and a printable value per LF line. Escape and continuation forms are
// rejected so there is no logical-line or Unicode escape ambiguity.
func parseProperties(in decodedInput) string {
	if len(in.bytes) == 0 || in.bytes[len(in.bytes)-1] != '\n' || hasTextControl(in.bytes) || bytes.ContainsAny(in.bytes, "\r\\") {
		return "MALFORMED_INPUT"
	}
	seen := map[string]struct{}{}
	budget := &tokenBudget{}
	for _, raw := range bytes.Split(in.bytes[:len(in.bytes)-1], []byte{'\n'}) {
		line := string(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			return "UNSUPPORTED_SCHEMA"
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || !yamlAtom(key) || value == "" || len(value) > maxStringBytes {
			return "MALFORMED_INPUT"
		}
		if !budget.consume(2) {
			return "LIMIT_EXCEEDED"
		}
		for _, c := range []byte(value) {
			if c < 0x20 || c > 0x7e {
				return "MALFORMED_INPUT"
			}
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
