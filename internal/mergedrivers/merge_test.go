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

func TestAppendDWCV07700DefaultMarkerMarkdown(t *testing.T) {
	for _, tc := range []struct {
		name, base, ours, theirs, want string
		clean                          bool
	}{
		{"ours-heading-edit", "old\n", "Ours\n=======\n", "Theirs\n", "<<<<<<<< ours\nOurs\n=======\n|||||||| base\nold\n========\nTheirs\n>>>>>>>> theirs\n", false},
		{"theirs-heading-edit", "old\n", "Ours\n", "Theirs\n=======\n", "<<<<<<<< ours\nOurs\n|||||||| base\nold\n========\nTheirs\n=======\n>>>>>>>> theirs\n", false},
		{"ours-heading-addition", "", "Ours\n=======\n", "Theirs\n", "Ours\n=======\nTheirs\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, clean, err := Append(context.Background(), t.TempDir(), []byte(tc.base), []byte(tc.ours), []byte(tc.theirs), 7)
			if err != nil || clean != tc.clean || string(got) != tc.want {
				t.Fatalf("got %q clean=%v err=%v, want %q clean=%v", got, clean, err, tc.want, tc.clean)
			}
		})
	}
}

func TestAppendDWCV07700MalformedDefaultMarkers(t *testing.T) {
	for _, data := range []string{
		"<<<<<<< ours\nOurs\n=======\nTheirs\n>>>>>>> theirs\n",
		"<<<<<<< ours\nOurs\n||||||| base\n>>>>>>> theirs\n",
		"<<<<<<< ours\nOurs\n||||||| base\n=======\nTheirs\n",
	} {
		got, clean := resolveAppend([]byte(data), 7)
		if clean || string(got) != data {
			t.Errorf("malformed conflict changed: got %q clean=%v, want %q clean=false", got, clean, data)
		}
	}
}

func TestAppendDWCV07700LiteralDelimiters(t *testing.T) {
	for _, size := range []int{7, 11} {
		lt, bar, eq, gt := strings.Repeat("<", size), strings.Repeat("|", size), strings.Repeat("=", size), strings.Repeat(">", size)
		for _, literal := range []string{lt + " ours", bar + " example", bar + " base", gt + " example", gt + " theirs", eq} {
			for _, side := range []string{"base", "ours", "theirs"} {
				t.Run(literal+"/"+side, func(t *testing.T) {
					base, ours, theirs := "old\n", "Ours\n", "Theirs\n"
					switch side {
					case "base":
						base = literal + "\nold\n"
					case "ours":
						ours = literal + "\n" + eq + "\nOurs\n"
					case "theirs":
						theirs = "Theirs\n" + literal + "\n"
					}
					want := lt + "< ours\n" + ours + bar + "| base\n" + base + eq + "=\n" + theirs + gt + "> theirs\n"
					got, clean, err := Append(context.Background(), t.TempDir(), []byte(base), []byte(ours), []byte(theirs), size)
					if err != nil || clean || string(got) != want {
						t.Fatalf("got %q clean=%v err=%v, want verbatim conflict %q", got, clean, err, want)
					}
				})
			}
		}
		literal := lt + " ours\n" + bar + " base\n" + eq + "\n" + gt + " theirs\n"
		for _, tc := range []struct{ name, base, ours, theirs, want string }{
			{"identical", literal, literal, literal, literal},
			{"one-sided", "old\n", literal, "old\n", literal},
			{"disjoint", "before\n" + literal + "after\n", "Ours\n" + literal + "after\n", "before\n" + literal + "Theirs\n", "Ours\n" + literal + "Theirs\n"},
		} {
			t.Run(tc.name+"/"+lt, func(t *testing.T) {
				got, clean, err := Append(context.Background(), t.TempDir(), []byte(tc.base), []byte(tc.ours), []byte(tc.theirs), size)
				if err != nil || !clean || string(got) != tc.want {
					t.Fatalf("got %q clean=%v err=%v, want clean text %q", got, clean, err, tc.want)
				}
			})
		}
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
