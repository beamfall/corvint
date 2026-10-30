package mobile

import (
	"encoding/json"
	"strings"
	"testing"
)

// js renders each value spliced into the generated WDIO configuration as a
// complete JSON (so JavaScript) string literal: a hostile quote, backslash,
// line terminator or closing script tag cannot end the literal or start a new
// statement (CodeQL alerts 33, 40 and 41 are false positives on this sink).
func TestJSLiteralEscapesHostileStrings(t *testing.T) {
	for _, hostile := range []string{`a"b`, `a'b`, `a\b`, "a\nb\r", "a b ", "</script><!--", "a`${x}`"} {
		literal := js(hostile)
		var back string
		if err := json.Unmarshal([]byte(literal), &back); err != nil || back != hostile {
			t.Fatalf("js(%q) = %s does not round-trip: %v", hostile, literal, err)
		}
		if !strings.HasPrefix(literal, `"`) || !strings.HasSuffix(literal, `"`) || strings.Count(literal, `"`)-strings.Count(literal, `\"`) != 2 {
			t.Fatalf("js(%q) = %s is not one closed string literal", hostile, literal)
		}
		if strings.ContainsAny(literal, "\n\r  <>") {
			t.Fatalf("js(%q) = %s carries a raw line terminator or tag character", hostile, literal)
		}
	}
}
