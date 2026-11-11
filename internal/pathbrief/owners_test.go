// SPDX-License-Identifier: Apache-2.0

package pathbrief

import (
	"slices"
	"testing"
)

// TBC-V1-010: CODEOWNERS pattern translation.
func TestOwnerPattern(t *testing.T) {
	cases := []struct {
		pattern string
		match   []string
		miss    []string
	}{
		{"*", []string{"a.go", "a/b.go"}, nil},
		{"*.go", []string{"a.go", "x/a.go", "x/y/a.go"}, []string{"a.txt", "ago"}},
		{"/docs/", []string{"docs/a.md", "docs/x/a.md"}, []string{"x/docs/a.md", "docs", "mydocs/a.md"}},
		{"docs/", []string{"docs/a.md", "x/docs/a.md"}, []string{"mydocs/a.md", "docs"}},
		{"/a/b.go", []string{"a/b.go"}, []string{"x/a/b.go", "a/b.gox"}},
		{"**/x", []string{"x", "a/x", "a/b/x", "a/x/y"}, []string{"ax", "a/xy"}},
		{"a/**/b", []string{"a/b", "a/q/b", "a/q/r/b"}, []string{"b", "x/a/b"}},
		{"a/**", []string{"a/f", "a/f/g"}, []string{"b/a/f", "ab/f"}},
		{"/a**go", []string{"ago", "axgo"}, []string{"a/x/b.go", "a/go"}},
		{"a**/b", []string{"a/b", "ax/b"}, []string{"a/q/b", "x/a/b"}},
		{"a/*", []string{"a/f"}, []string{"a/f/g", "b/a/f"}},
		{"/*", []string{"a.go", "Makefile"}, []string{"src/main.go", "a/b/c.go"}},
		{"?.go", []string{"a.go", "x/a.go"}, []string{"ab.go"}},
		{"!neg", nil, []string{"neg"}},
		{"[ab].go", nil, []string{"a.go"}},
	}
	for _, c := range cases {
		re := ownerPattern(c.pattern)
		if re == nil {
			if len(c.match) != 0 {
				t.Errorf("%q has no matcher", c.pattern)
			}
			continue
		}
		for _, name := range c.match {
			if !re.MatchString(name) {
				t.Errorf("%q should match %q", c.pattern, name)
			}
		}
		for _, name := range c.miss {
			if re.MatchString(name) {
				t.Errorf("%q should not match %q", c.pattern, name)
			}
		}
	}
	if ownerPattern("[ab].go") != nil || ownerPattern("!neg") != nil || ownerPattern("/") != nil {
		t.Error("negation, ranges and an empty body must match nothing")
	}
}

// TBC-V1-010: parsing keeps true line numbers, ignores comments and blanks, and the last match wins.
func TestParseOwnersAndOwnerOf(t *testing.T) {
	rules := parseOwners("# comment\n\n*.go @a @b # trailing\n  /x/  \nlonely\n!neg @z\n\t\n")
	if len(rules) != 4 {
		t.Fatalf("rules = %d", len(rules))
	}
	wantLines := []int{3, 4, 5, 6}
	for i, r := range rules {
		if r.line != wantLines[i] {
			t.Errorf("rule %d line %d, want %d", i, r.line, wantLines[i])
		}
	}
	if !slices.Equal(rules[0].owners, []string{"@a", "@b"}) {
		t.Errorf("owners %v (a trailing comment is not an owner)", rules[0].owners)
	}
	if rules[1].owners == nil || len(rules[1].owners) != 0 || rules[1].pattern != "/x/" {
		t.Errorf("a line with no owners keeps its pattern: %+v", rules[1])
	}
	for name, want := range map[string]int{"y.go": 0, "x/y.go": 1, "x/": 1, "lonely": 2, "d/lonely": 2, "none.txt": -1, "neg": -1} {
		if got := ownerOf(rules, name); got != want {
			t.Errorf("ownerOf(%q) = %d, want %d", name, got, want)
		}
	}
	if ownerOf(nil, "a") != -1 {
		t.Error("no rules")
	}
	// An escaped space is part of the pattern, not the end of it.
	spaced := parseOwners("* @all\ndocs/my\\ file.md @docs @more\nold\\ dir/\t@dir\n")
	if len(spaced) != 3 || spaced[1].pattern != "docs/my file.md" || !slices.Equal(spaced[1].owners, []string{"@docs", "@more"}) || !slices.Equal(spaced[2].owners, []string{"@dir"}) {
		t.Fatalf("escaped spaces: %+v", spaced)
	}
	for name, want := range map[string]int{"docs/my file.md": 1, "docs/my": 0, "old dir/x.go": 2, "x/old dir/y": 2, "other.md": 0} {
		if got := ownerOf(spaced, name); got != want {
			t.Errorf("ownerOf(%q) = %d, want %d", name, got, want)
		}
	}
}
