package cli

import (
	"strings"
	"testing"
)

// A refused --fields name is quoted with its quote, backslash and control
// characters escaped, so a hostile name cannot end the quoted span or forge a
// second diagnostic line (CodeQL alert 32).
func TestCompactFieldsRefusalEscapesHostileName(t *testing.T) {
	_, msg := parseCompactFields("x\" is ok\nfield \"y")
	want := `field "x\" is ok\nfield \"y" is not a key or key.sub name`
	if msg != want || strings.Contains(msg, "\n") {
		t.Fatalf("refusal = %q, want %q", msg, want)
	}
	if _, msg := parseCompactFields("a-b"); msg != `field "a-b" is not a key or key.sub name` {
		t.Fatalf("plain refusal changed: %q", msg)
	}
}
