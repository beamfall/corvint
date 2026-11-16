// SPDX-License-Identifier: Apache-2.0
package rubylinecov

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/linecov"
)

// reachSource is lib/r.rb of the reach fixtures. Ruby 3.3.8 counts lines 1,
// 2, 3, 4, 5, 6, 10, 12, 13, 17, 18, 19, 21, 24, 25, 26 and 28: a one-line
// `when` counts only when its body runs, a lambda inside a multi-line literal
// counts only when it is called, and the literal's own line is its first
// element's.
const reachSource = `class R
  def kind(v)
    case v
    when 1 then :one
    when 3 then :three
    else :other
    end
  end

  def handlers
    {
      a: -> { 1 },
      b: -> { 2 },
    }
  end

  def local(flag)
    if flag
      x = 1
    end
    x
  end

  def numbered
    f = lambda do
      _1 + 1
    end
    f.arity
  end
end
`

// reachEdits are edits of lines no test hit that still change a test's
// result: Ruby evaluates the `when` condition, the literal's key, the
// local's declaration and the block's parameter list without counting the
// line.
var reachEdits = []struct {
	name, with, breaks string
	line               int
}{
	{"one-line when", "    when 3 then :one", "RTest#test_kind3", 4},
	{"lambda in a multi-line literal", "      c: -> { 2 },", "RTest#test_keys", 13},
	{"local renamed on an unexecuted line", "      y = 1", "RTest#test_local", 19},
	{"numbered block parameter on an unexecuted line", "      1 + 1", "RTest#test_numbered", 26},
}

// reachIndex is the measured coverage of reachSource under four tests, each
// calling one method.
func reachIndex(t *testing.T) *linecov.Index {
	t.Helper()
	kind, handlers, local, numbered := span{2, 2, 8, 5}, span{10, 2, 15, 5}, span{17, 2, 22, 5}, span{24, 2, 29, 5}
	test := func(name string, method span, lines ...int32) record {
		return record{pkg: "test/case_test.rb", name: name, lines: map[string][]int32{"lib/r.rb": lines}, methods: map[string][]span{"lib/r.rb": {method}}}
	}
	run := &Run{
		Runner:   "ruby 3.3.8 minitest 5.27.0 coverage=lines,methods",
		isolated: true,
		files: map[string]fileShape{"lib/r.rb": {
			Lines:   []int32{1, 2, 3, 4, 5, 6, 10, 12, 13, 17, 18, 19, 21, 24, 25, 26, 28},
			Methods: []span{kind, handlers, local, numbered},
		}},
		records: []record{
			test("RTest#test_kind3", kind, 3, 5),
			test("RTest#test_keys", handlers, 12),
			test("RTest#test_local", local, 18, 21),
			test("RTest#test_numbered", numbered, 25, 28),
			{pkg: Load, name: Load, lines: map[string][]int32{"lib/r.rb": {1, 2, 10, 17, 24}}, methods: map[string][]span{}},
		},
	}
	for _, r := range run.records[:4] {
		run.Outcomes = append(run.Outcomes, Outcome{Package: r.pkg, Name: r.name, Status: "passed", Recorded: true})
	}
	ix, err := Index(".", "c", run, map[string]string{"lib/r.rb": "old", "test/case_test.rb": "t"})
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

// replaceLine returns source with its 1-based line replaced.
func replaceLine(source string, line int, with string) string {
	lines := strings.Split(source, "\n")
	lines[line-1] = with
	return strings.Join(lines, "\n")
}

// LPCV-V0-7605: a counted line's hits bound its reach only when the line is
// one whole statement whose bindings the edit keeps. An edit of a one-line
// `when`, of a line inside a multi-line literal, of a local's declaration or
// of a numbered block parameter selects the enclosing called method's tests
// although no test hit the line.
func TestAnalyzeLinesUnboundedReach(t *testing.T) {
	ix := reachIndex(t)
	for _, c := range reachEdits {
		newSource := replaceLine(reachSource, c.line, c.with)
		d := linecov.FileDiff{Path: "lib/r.rb", OldOID: "old", NewOID: "new", Hunks: []linecov.Hunk{{Start: c.line, End: c.line}}, NewHunks: []linecov.Hunk{{Start: c.line, End: c.line}}}
		src := Sources{
			Old:   func(string) ([]byte, error) { return []byte(reachSource), nil },
			New:   func(string) ([]byte, error) { return []byte(newSource), nil },
			Check: func(string, []byte) error { return nil },
		}
		e := AnalyzeLines(ix, []linecov.FileDiff{d}, src)
		if got := selected(ix, e); got != c.breaks {
			t.Errorf("%s: lib/r.rb:%d selects %q, want %q", c.name, c.line, got, c.breaks)
		}
		p, _ := Plan(ix, []string{"test/case_test.rb"}, linecov.Effect{}, e)
		if !kept(p["test/case_test.rb"], c.breaks) {
			t.Errorf("%s: the plan %+v drops %s", c.name, p["test/case_test.rb"], c.breaks)
		}
	}
}

// kept reports whether a file's plan runs the named test.
func kept(p linecov.PackagePlan, name string) bool {
	if p.Mode == linecov.Whole {
		return true
	}
	if p.Mode != linecov.Tests {
		return false
	}
	for _, n := range p.Tests {
		if n == name {
			return true
		}
	}
	return false
}

// rubyCheck is `ruby -c` on PATH.
func rubyCheck(_ string, src []byte) error {
	cmd := exec.Command("ruby", "-c", "-")
	cmd.Stdin = bytes.NewReader(src)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s", out)
	}
	return nil
}

// LPCV-V0-7605: measured end to end. Each reach edit is committed on a real
// Minitest project whose index Ruby collected; the suite at the edited head
// fails a test, and the line-level plan keeps every test that fails.
func TestLiveLineReach(t *testing.T) {
	requireRuby(t, "minitest")
	tests := `require 'minitest/autorun'
require_relative '../lib/r'
class RTest < Minitest::Test
  def test_kind3; assert_equal :three, R.new.kind(3); end
  def test_keys; assert_equal [:a, :b], R.new.handlers.keys; end
  def test_local; assert_nil R.new.local(false); end
  def test_numbered; assert_equal 1, R.new.numbered; end
end
`
	root := laneProject(t, tests, "lib/r.rb", reachSource)
	ctx := context.Background()
	base := git(t, root, "rev-parse", "HEAD")
	argv := []string{"ruby", "test/case_test.rb"}
	_, ix := collect(t, root, base, Options{Argv: argv})
	if len(ix.Unordered) != 0 || len(ix.Tests) != 5 {
		t.Fatalf("index tests %d unordered %v", len(ix.Tests), ix.Unordered)
	}
	// The premise of each edit: no test hit the edited line.
	for _, c := range reachEdits {
		if got := lineTests(ix, "lib/r.rb", c.line); got != "" {
			t.Fatalf("%s: lib/r.rb:%d was hit by %q", c.name, c.line, got)
		}
	}
	for _, c := range reachEdits {
		if err := os.WriteFile(filepath.Join(root, "lib", "r.rb"), []byte(replaceLine(reachSource, c.line, c.with)), 0o600); err != nil {
			t.Fatal(err)
		}
		git(t, root, "commit", "-qam", c.name)
		head := git(t, root, "rev-parse", "HEAD")
		diffs, err := linecov.Diff(ctx, root, base, head)
		if err != nil {
			t.Fatal(err)
		}
		e := AnalyzeLines(ix, diffs, Sources{Old: linecov.Reader(ctx, root, base), New: linecov.Reader(ctx, root, head), Check: rubyCheck})
		p, _ := Plan(ix, []string{"test/case_test.rb"}, linecov.Effect{}, e)
		run, err := Collect(ctx, Options{Root: root, Work: t.TempDir(), Argv: argv, Timeout: 2 * time.Minute})
		if err != nil {
			t.Fatal(err)
		}
		var failing []string
		for _, o := range run.Outcomes {
			if o.Status == "failed" {
				failing = append(failing, o.Name)
			}
		}
		if strings.Join(failing, " ") != c.breaks {
			t.Fatalf("%s: the edit fails %v, want %s", c.name, failing, c.breaks)
		}
		for _, name := range failing {
			if !kept(p["test/case_test.rb"], name) {
				t.Errorf("%s: %s fails at the edited head but the plan %+v (%s) drops it", c.name, name, p["test/case_test.rb"], selected(ix, e))
			}
		}
		git(t, root, "reset", "-q", "--hard", base)
	}
}

// LPCV-V0-7605: an edit that keeps a whole statement's bindings narrows to
// the line's hits, and to none on a line no test hit.
func TestAnalyzeLinesBoundedReach(t *testing.T) {
	ix := reachIndex(t)
	for _, c := range []struct {
		name, with, want string
		line             int
	}{
		{"same local, other value, unexecuted", "      x = 2", "", 19},
		{"executed read", "    x.to_s", "RTest#test_local", 21},
		{"executed call", "    f.arity + 0", "RTest#test_numbered", 28},
		// A method name's ! suffix and a comma before a string literal are not
		// continuations, so neither edit changes the block structure.
		{"statement ending in a method suffix", "    fail!", "RTest#test_local", 21},
		{"statement ending in a literal", "    raise E, \"no\"", "RTest#test_local", 21},
		{"statement continued onto the next line", "    x +", "fallback: edit changes Ruby block structure lib/r.rb: keywords, brackets or continuations differ at line 21", 21},
		{"when body moved off its condition stays with the method", "    when 1 then :uno", "RTest#test_kind3", 4},
	} {
		newSource := replaceLine(reachSource, c.line, c.with)
		d := linecov.FileDiff{Path: "lib/r.rb", OldOID: "old", NewOID: "new", Hunks: []linecov.Hunk{{Start: c.line, End: c.line}}, NewHunks: []linecov.Hunk{{Start: c.line, End: c.line}}}
		src := Sources{
			Old:   func(string) ([]byte, error) { return []byte(reachSource), nil },
			New:   func(string) ([]byte, error) { return []byte(newSource), nil },
			Check: func(string, []byte) error { return nil },
		}
		if got := selected(ix, AnalyzeLines(ix, []linecov.FileDiff{d}, src)); got != c.want {
			t.Errorf("%s: lib/r.rb:%d selects %q, want %q", c.name, c.line, got, c.want)
		}
	}
}

// LPCV-V0-7605: the whole-statement and binding rules, line by line.
func TestLineReachRules(t *testing.T) {
	source := strings.Join([]string{
		"foo(1)",                 // 1 whole
		"x = a + b",              // 2 whole
		"reset!",                 // 3 whole: ! is the method's suffix
		"v.empty?",               // 4 whole
		"self.class.new(1)",      // 5 whole: .class is a method
		"xs.each { |i| p i }",    // 6 whole
		"when 1 then :one",       // 7 keyword
		"foo if bar",             // 8 keyword
		"a +",                    // 9 continues
		"  b",                    // 10 continued
		"v.to_s",                 // 11 continued by the next line
		"  .upcase",              // 12 leads
		"list = [",               // 13 opens a bracket
		"  1,",                   // 14 inside a bracket
		"]",                      // 15 closes a bracket
		"text = <<~T",            // 16 opens a heredoc
		"  body",                 // 17 heredoc body
		"T",                      // 18 heredoc terminator
		"a; b",                   // 19 two statements
		"# note",                 // 20 comment
		"",                       // 21 blank
		"foo unless",             // 22 keyword
		"  bar",                  // 23 continued
		"cond ?",                 // 24 continues
		"  1 : 2",                // 25 continued
		"puts \"a #{b}\"",        // 26 whole
		"x.nil? ? 1 : 2",         // 27 whole
		"p :done",                // 28 whole
		"raise E, \"no blocks\"", // 29 whole: the trailing comma precedes a literal
		"\"abc\".upcase",         // 30 whole: the leading dot follows a literal
		"s =~ /re/",              // 31 whole
		"t = \"a\" +",            // 32 continues
		"  \"b\"",                // 33 continued
	}, "\n")
	s := readSide(source)
	whole := map[int]bool{1: true, 2: true, 3: true, 4: true, 5: true, 6: true, 26: true, 27: true, 28: true, 29: true, 30: true, 31: true}
	for n := 1; n <= 33; n++ {
		if why := s.whole(n); (why == "") != whole[n] {
			t.Errorf("line %d %q: whole says %q, want whole=%v", n, s.raw[n-1], why, whole[n])
		}
	}
	if unbalanced := readSide("foo(\n"); unbalanced.whole(1) == "" {
		t.Errorf("a line of an unbalanced file is whole")
	}
	for _, c := range []struct {
		raw, sig string
		ok       bool
	}{
		{"x = 1", "x =", true},
		{"x  =  2", "x =", true},
		{"y = 1", "y =", true},
		{"a, b = c", "a, b =", true},
		{"x ||= 1", "x ||=", true},
		{"x <<= 1", "x <<=", true},
		{"x == 1", "", true},
		{"a <= b", "", true},
		{"a != b", "", true},
		{"s =~ /re/", "", true},
		{"foo(1)", "", true},
		{"h = {a => 1}", "h = a =>", true},
		{"v => w", "v => w", true},
		{`puts "#{y = 1}"`, `puts "#{y =`, true},
		{`raise "use it"`, "", true},
		{`/(?<y>.)/ =~ s`, "", false},
		{"puts _1", "", false},
		{"puts it", "", false},
		{`puts "#{it}"`, "", false},
	} {
		code := rubydialectCode(c.raw)
		if sig, ok := binding(c.raw, code); sig != c.sig || ok != c.ok {
			t.Errorf("binding(%q) = %q, %v; want %q, %v", c.raw, sig, ok, c.sig, c.ok)
		}
	}
}

func rubydialectCode(raw string) string { return readSide(raw).state[0].Code }
