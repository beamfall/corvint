// SPDX-License-Identifier: Apache-2.0
package mergedrivers

import (
	"context"
	"strings"
	"testing"
)

func TestAppendDWCV07700(t *testing.T) {
	for _, tc := range []struct {
		name, base, ours, theirs, want string
		clean                          bool
	}{
		{"headings", "anchor\n", "anchor\n## Ours\nA\n", "anchor\n## Theirs\nB\n", "anchor\n## Ours\nA\n\n## Theirs\nB\n", true},
		{"rows", "anchor\n", "anchor\n- ours\n", "anchor\n- theirs\n", "anchor\n- ours\n- theirs\n", true},
		{"no-final-lf", "anchor\n", "anchor\nours", "anchor\ntheirs", "anchor\nours\ntheirs\n", true},
		{"blank-base", "anchor\n \n", "anchor\nours\n", "anchor\n# theirs\n", "anchor\nours\n\n# theirs\n", true},
		{"blank-ours-tail", "anchor\n", "anchor\nours\n\n", "anchor\n# theirs\n", "anchor\nours\n\n# theirs\n", true},
		{"blank-theirs-head", "anchor\n", "anchor\nours\n", "anchor\n\n# theirs\n", "anchor\nours\n\n# theirs\n", true},
		{"real-edit", "anchor\nold\n", "anchor\nours\n", "anchor\ntheirs\n", "", false},
		{"crlf", "anchor\r\n", "anchor\r\nours\r\n", "anchor\r\n# theirs\r\n", "anchor\r\nours\r\n\n# theirs\r\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, clean, err := Append(context.Background(), t.TempDir(), []byte(tc.base), []byte(tc.ours), []byte(tc.theirs), 11)
			if err != nil || clean != tc.clean {
				t.Fatalf("clean=%v err=%v result=%q", clean, err, result)
			}
			if clean && string(result) != tc.want {
				t.Fatalf("got %q, want %q", result, tc.want)
			}
			if !clean && (!strings.Contains(string(result), "||||||||||| base\nold\n") || !strings.Contains(string(result), "<<<<<<<<<<< ours\nours\n")) {
				t.Fatalf("edit conflict was not retained verbatim: %q", result)
			}
		})
	}
	malformed := []byte("<<<<<<< ours\naddition\n||||||| base\n=======\nother\n")
	if got, clean := resolveAppend(malformed, 7); clean || string(got) != string(malformed) {
		t.Fatalf("malformed conflict changed: %q clean=%v", got, clean)
	}
}

func TestJSONDWCV07702(t *testing.T) {
	for _, tc := range []struct{ name, base, ours, theirs, want string }{
		{"root-union", `{}`, `{"a":1}`, `{"b":2}`, "{\n  \"a\": 1,\n  \"b\": 2\n}\n"},
		{"add-add-empty-base", ``, `{"a":1}`, `{"b":2}`, "{\n  \"a\": 1,\n  \"b\": 2\n}\n"},
		{"empty-ours", `{}`, ``, `{}`, ""},
		{"registry-union", `{"profile":"p","packages":{}}`, `{"profile":"p","packages":{"a":["<a>"]}}`, `{"profile":"p","packages":{"b":[]}}`, "{\n  \"packages\": {\n    \"a\": [\n      \"<a>\"\n    ],\n    \"b\": []\n  },\n  \"profile\": \"p\"\n}\n"},
		{"divergence", `{"a":0}`, `{"a":1}`, `{"a":2}`, ""},
		{"entry-atomic", `{"scripts":{"a":{"x":0,"y":0}}}`, `{"scripts":{"a":{"x":1,"y":0}}}`, `{"scripts":{"a":{"x":0,"y":1}}}`, ""},
		{"delete-edit", `{"a":0}`, `{}`, `{"a":2}`, ""},
		{"delete-unchanged", `{"a":0}`, `{}`, `{"a":0}`, "{}\n"},
		{"equal", `{}`, `{"a":1}`, `{"a":1}`, "{\n  \"a\": 1\n}\n"},
		{"precision", `{}`, `{"a":9007199254740993}`, `{"b":null}`, "{\n  \"a\": 9007199254740993,\n  \"b\": null\n}\n"},
		{"duplicate", `{}`, `{"a":1,"a":2}`, `{}`, ""},
		{"nested-duplicate", `{}`, `{"packages":{"a":1,"a":2}}`, `{}`, ""},
		{"array-root", `{}`, `[]`, `{}`, ""},
		{"null-root", `{}`, `null`, `{}`, ""},
		{"trailing", `{}`, `{} {}`, `{}`, ""},
		{"malformed", `{}`, `{"a":`, `{}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := JSON([]byte(tc.base), []byte(tc.ours), []byte(tc.theirs))
			if tc.want == "" {
				if err == nil || got != nil {
					t.Fatalf("conflict accepted: %s %v", got, err)
				}
			} else if err != nil || string(got) != tc.want {
				t.Fatalf("got %q err=%v, want %q", got, err, tc.want)
			}
		})
	}
}
