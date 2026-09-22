package tcq

import (
	"strings"
	"testing"
)

// TestPreflightBounds covers TCQ-V0-041: the depth and aggregate member
// ceilings are enforced by the bounded scanner BEFORE any semantic parse, so a
// hostile document never allocates a tree.
func TestPreflightBounds(t *testing.T) {
	bounds := jsonBounds{bytes: 1 << 20, depth: 4, members: 6}
	cases := []struct {
		name     string
		raw      string
		exhausts bool
	}{
		{"within-depth", `{"a":{"b":{"c":1}}}`, false},
		{"at-depth-limit", `{"a":{"b":{"c":{"d":1}}}}`, false},
		{"over-depth", `{"a":{"b":{"c":{"d":{"e":1}}}}}`, true},
		{"over-array-depth", `[[[[[1]]]]]`, true},
		{"within-members", `{"a":1,"b":2,"c":3}`, false},
		{"over-object-members", `{"a":1,"b":2,"c":3,"d":4,"e":5,"f":6,"g":7}`, true},
		{"over-array-items", `[1,2,3,4,5,6,7]`, true},
		{"braces-in-strings-do-not-nest", `{"a":"{{{{{{{{"}`, false},
		{"long-number", `{"a":` + strings.Repeat("1", maxJSONNumberBytes+1) + `}`, true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := preflightJSON([]byte(testCase.raw), bounds)
			if testCase.exhausts {
				requireCode(t, err, CodeResourceExhausted)
				return
			}
			if err != nil {
				t.Fatalf("preflightJSON: %v", err)
			}
		})
	}
}

// TestParseCanonicalRejectsNoncanonicalBytes covers TCQ-V0-040: a document that
// parses but is not in the frozen encoding is noncanonical, never normalized.
func TestParseCanonicalRejectsNoncanonicalBytes(t *testing.T) {
	cases := map[string]struct {
		raw  string
		code string
	}{
		"unsorted-keys":  {`{"b":1,"a":2}` + "\n", CodeNoncanonicalCommand},
		"space":          {`{"a": 1}` + "\n", CodeNoncanonicalCommand},
		"no-terminal-lf": {`{"a":1}`, CodeNoncanonicalCommand},
		"two-lf":         {`{"a":1}` + "\n\n", CodeNoncanonicalCommand},
		"duplicate-key":  {`{"a":1,"a":2}` + "\n", CodeInvalidCommand},
		"float":          {`{"a":1.5}` + "\n", CodeInvalidCommand},
		"not-an-object":  {`[1]` + "\n", CodeInvalidCommand},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseCanonical([]byte(testCase.raw), commandBounds, CodeInvalidCommand, CodeNoncanonicalCommand)
			requireCode(t, err, testCase.code)
		})
	}
}
