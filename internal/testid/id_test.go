// SPDX-License-Identifier: Apache-2.0
package testid

import (
	"errors"
	"reflect"
	"testing"
)

// CTI-V1-001: every runner's ID formats as `<runner>:<file>::<segment>...`,
// a file-level ID has no segments, and each parses back to the same parts
// and the same bytes.
func TestCTIV1001FormatParseRoundTrip(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		id   ID
		want string
	}{
		{ID{Jest, "web/sum.test.js", []string{"sum", "adds"}}, "jest:web/sum.test.js::sum::adds"},
		{ID{Go, "internal/calc", []string{"TestAdd", "negative"}}, "go:internal/calc::TestAdd::negative"},
		{ID{Go, "pkg/p", []string{"TestParse", "empty", "utf8"}}, "go:pkg/p::TestParse::empty::utf8"},
		{ID{Go, ".", []string{"TestRoot"}}, "go:.::TestRoot"},
		{ID{Vitest, "src/a.test.ts", []string{"case 1"}}, "vitest:src/a.test.ts::case 1"},
		{ID{Playwright, "e2e/login.spec.ts", []string{"chromium", "signs in"}}, "playwright:e2e/login.spec.ts::chromium::signs in"},
		{ID{Playwright, "e2e/login.spec.ts", []string{"", "signs in"}}, "playwright:e2e/login.spec.ts::::signs in"},
		{ID{Pytest, "tests/test_a.py", []string{"TestA", "test_b[x::y]"}}, "pytest:tests/test_a.py::TestA::test_b[x%3A%3Ay]"},
		{ID{RSpec, "spec/user_spec.rb", []string{"User", "saves"}}, "rspec:spec/user_spec.rb::User::saves"},
		{ID{Minitest, "test/calc_test.rb", []string{"CalcTest", "test_add"}}, "minitest:test/calc_test.rb::CalcTest::test_add"},
		{ID{Jest, "web/sum.test.js", nil}, "jest:web/sum.test.js"},
		{ID{Go, "internal/calc", nil}, "go:internal/calc"},
	} {
		got := c.id.String()
		if got != c.want {
			t.Errorf("%+v formats %q, want %q", c.id, got, c.want)
			continue
		}
		parsed, err := Parse(got)
		if err != nil {
			t.Errorf("Parse(%q): %v", got, err)
			continue
		}
		if parsed.String() != got || parsed.Runner != c.id.Runner || parsed.File != c.id.File || len(parsed.Segments) != len(c.id.Segments) {
			t.Errorf("Parse(%q) = %+v", got, parsed)
		}
		for index := range c.id.Segments {
			if parsed.Segments[index] != c.id.Segments[index] {
				t.Errorf("Parse(%q) segment %d = %q, want %q", got, index, parsed.Segments[index], c.id.Segments[index])
			}
		}
		if parsed.IsFileLevel() != (len(c.id.Segments) == 0) || parsed.FileLevel().String() != c.id.Runner+":"+Escape(c.id.File) {
			t.Errorf("%q file level: %t %q", got, parsed.IsFileLevel(), parsed.FileLevel())
		}
	}
	for id, want := range map[string]Granularity{"go:a::T": GranularityTest, "go:a": GranularityPackage, "jest:a.js": GranularityFile, "jest:a.js::x": GranularityTest} {
		parsed, err := Parse(id)
		if err != nil || parsed.Granularity() != want {
			t.Errorf("%q granularity %q (%v), want %q", id, parsed.Granularity(), err, want)
		}
	}
}

// CTI-V1-002: `%`, `:`, control characters and invalid UTF-8 bytes are
// escaped as upper-case %XX and nothing else is; every case round-trips
// through Escape, Unescape, String and Parse byte for byte.
func TestCTIV1002EscapeRoundTrip(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ raw, escaped string }{
		{"GET /a:b", "GET /a%3Ab"},
		{"100%", "100%25"},
		{"%3A", "%253A"},
		{"a::b", "a%3A%3Ab"},
		{"tab\there", "tab%09here"},
		{"new\nline", "new%0Aline"},
		{"nul\x00", "nul%00"},
		{"del\x7f", "del%7F"},
		{"c1\u0085", "c1%C2%85"},
		{"bad\xffbyte", "bad%FFbyte"},
		{"trunc\xe2\x82", "trunc%E2%82"},
		{"émoji 🚀 [x] #2 / \\ ?*", "émoji 🚀 [x] #2 / \\ ?*"},
		{"�", "�"},
		{"", ""},
	} {
		if got := Escape(c.raw); got != c.escaped {
			t.Errorf("Escape(%q) = %q, want %q", c.raw, got, c.escaped)
		}
		back, err := Unescape(c.escaped)
		if err != nil || back != c.raw {
			t.Errorf("Unescape(%q) = %q, %v; want %q", c.escaped, back, err, c.raw)
		}
		id := ID{Runner: Vitest, File: "src/a.test.ts", Segments: []string{c.raw}}
		parsed, err := Parse(id.String())
		if err != nil || parsed.Segments[0] != c.raw || parsed.String() != id.String() {
			t.Errorf("segment %q: Parse(%q) = %+v, %v", c.raw, id.String(), parsed, err)
		}
	}
	colonFile := ID{Runner: Pytest, File: "tests/a:b%.py", Segments: []string{"test"}}
	if got := colonFile.String(); got != "pytest:tests/a%3Ab%25.py::test" {
		t.Fatalf("file escape: %q", got)
	}
	if parsed, err := Parse(colonFile.String()); err != nil || parsed.File != "tests/a:b%.py" {
		t.Fatalf("file round trip: %+v %v", parsed, err)
	}
	for _, bad := range []string{"%3a", "%3", "%", "%G0", "%41", "a:b", "x\ty", "\xff", "%25%"} {
		if _, err := Unescape(bad); !errors.Is(err, ErrMalformed) {
			t.Errorf("Unescape(%q) err %v, want ErrMalformed", bad, err)
		}
	}
}

// CTI-V1-001, CTI-V1-011: malformed IDs never parse: an unknown or missing
// runner token, a file that is not clean and repository-relative, a stray
// colon and a non-canonical escape.
func TestCTIV1001ParseRefusesMalformed(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{
		"", "jest", "cypress:a.cy.ts::x", "typescript:vitest:src/a.test.ts", "JEST:a.js",
		"jest:", "jest:/abs/a.js", "jest:../a.js", "jest:a/../b.js", "jest:./a.js", "jest:a//b.js", "jest:a/",
		"jest:a.js:x", "jest:a.js:::x", "jest:a%3a.js", "jest:a.js::%41", "jest:a.js::x\ty",
	} {
		if parsed, err := Parse(bad); !errors.Is(err, ErrMalformed) {
			t.Errorf("Parse(%q) = %+v, %v; want ErrMalformed", bad, parsed, err)
		}
	}
	if _, err := New("cypress", "a.js"); !errors.Is(err, ErrMalformed) {
		t.Errorf("New with an unknown runner: %v", err)
	}
	if _, err := New(Jest, "/a.js"); !errors.Is(err, ErrMalformed) {
		t.Errorf("New with an absolute file: %v", err)
	}
	if id, err := New(Jest, "a.js", "x"); err != nil || id.String() != "jest:a.js::x" {
		t.Errorf("New: %+v %v", id, err)
	}
	if child := (ID{Runner: Go, File: "p"}).Child("TestA").Child("sub"); child.String() != "go:p::TestA::sub" {
		t.Errorf("Child: %q", child)
	}
	if !reflect.DeepEqual(Runners, []string{"go", "vitest", "jest", "playwright", "pytest", "rspec", "minitest"}) {
		t.Errorf("Runners = %v", Runners)
	}
}

// CTI-V1-008: a file-level selection matches every ID under its file and
// nothing beside it; a test-level selection matches the test and its
// subtests; a runner instance matches every ID of the runner under its
// root; the comparison is on escaped forms and an unparsable side never
// matches.
func TestCTIV1008FileLevelMatch(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		selection, id string
		want          bool
	}{
		{"rspec:spec/user_spec.rb", "rspec:spec/user_spec.rb::User::saves", true},
		{"rspec:spec/user_spec.rb", "rspec:spec/user_spec.rb::User::rejects", true},
		{"rspec:spec/user_spec.rb", "rspec:spec/user_spec.rb::User::later", true},
		{"rspec:spec/user_spec.rb", "rspec:spec/user_spec.rb", true},
		{"rspec:spec/user_spec.rb", "rspec:spec/user_spec.rb2::x", false},
		{"rspec:spec/user_spec.rb", "minitest:spec/user_spec.rb::X::y", false},
		{"go:internal/calc", "go:internal/calc::TestAdd::negative", true},
		{"go:internal/calc", "go:internal/calc/sub::TestAdd", false},
		{"go:internal/calc::TestAdd", "go:internal/calc::TestAdd::negative", true},
		{"go:internal/calc::TestAdd", "go:internal/calc::TestAddMore", false},
		{"vitest:src/a%3Ab.test.ts", "vitest:src/a%3Ab.test.ts::case 1", true},
		{"vitest:src/a", "vitest:src/a%3Ab.test.ts::case 1", false},
		{"jest:a.js", "jest:a.js:x", false},
		{"cypress:a.js", "cypress:a.js::x", false},
		{"jest:a.js:", "jest:a.js::x", false},
	} {
		if got := Matches(c.selection, c.id); got != c.want {
			t.Errorf("Matches(%q, %q) = %t, want %t", c.selection, c.id, got, c.want)
		}
	}
	for _, c := range []struct {
		runner, root, id string
		want             bool
	}{
		{Vitest, "web", "vitest:web/src/a.test.ts::x", true},
		{Vitest, "web", "vitest:web::x", true},
		{Vitest, "web", "vitest:webapp/a.test.ts::x", false},
		{Vitest, "web", "jest:web/a.test.ts::x", false},
		{Vitest, ".", "vitest:a.test.ts::x", true},
		{Vitest, "../web", "vitest:web/a.test.ts::x", false},
		{Go, "tools/x", "go:tools/x/y::TestA", true},
		{Go, "tools/x", "not an id", false},
	} {
		if got := MatchesInstance(c.runner, c.root, c.id); got != c.want {
			t.Errorf("MatchesInstance(%q, %q, %q) = %t, want %t", c.runner, c.root, c.id, got, c.want)
		}
	}
}
