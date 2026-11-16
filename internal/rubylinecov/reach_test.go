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

// reachBase opens lib/r.rb of the reach fixtures. Ruby 3.3.8 counts lines 1,
// 2, 3, 4, 5, 6, 10, 12, 13, 17, 18, 19, 21, 24, 25, 26, 28, 31 and 32: a
// one-line `when` counts only when its body runs, a lambda inside a
// multi-line literal counts only when it is called, and the literal's own
// line is its first element's.
const reachBase = `class R
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

  def x
    :method
  end
`

// guarded are the fixture's methods of one shape, appended to reachBase:
//
//	def NAME(flag)
//	  if flag
//	    OLD
//	  end
//	  x
//	end
//
// No test passes a true flag, so OLD is a counted line no test hits. The
// method returns nil when OLD declares a local x and R#x's :method when it
// does not.
var guarded = []struct{ name, old, returns string }{
	{"table", "x = {:a => 1}", "nil"},
	{"command", "puts x = {:a => 1}", "nil"},
	{"ivar", "@x = {:a => 1}", ":method"},
	{"comment", "puts(flag) # x = {flag => 1}", ":method"},
	{"remark", "puts flag # x = \"=>\"", ":method"},
	{"number", "puts 1 => x", ":method"},
	{"index", "puts [1] => x", ":method"},
	{"capture", "/(?'w'.)/ =~ 'a'", ":method"},
	{"space", "puts x = 1", "nil"},
}

// guardedStart is the line of the first guarded method's def; each takes
// seven lines with the blank line before it.
const guardedStart = 35

// guardedLine is the line of the named guarded method's OLD statement.
func guardedLine(name string) int {
	for i, g := range guarded {
		if g.name == name {
			return guardedStart + 7*i + 2
		}
	}
	panic("no guarded method " + name)
}

// reachSource is lib/r.rb of the reach fixtures.
var reachSource = func() string {
	var b strings.Builder
	b.WriteString(reachBase)
	for _, g := range guarded {
		fmt.Fprintf(&b, "\n  def %s(flag)\n    if flag\n      %s\n    end\n    x\n  end\n", g.name, g.old)
	}
	b.WriteString("end\n")
	return b.String()
}()

// reachTests is test/case_test.rb of the live reach fixture: one test a
// method.
var reachTests = func() string {
	var b strings.Builder
	b.WriteString(`require 'minitest/autorun'
require_relative '../lib/r'
class RTest < Minitest::Test
  def test_kind3; assert_equal :three, R.new.kind(3); end
  def test_keys; assert_equal [:a, :b], R.new.handlers.keys; end
  def test_local; assert_nil R.new.local(false); end
  def test_numbered; assert_equal 1, R.new.numbered; end
`)
	for _, g := range guarded {
		if g.returns == "nil" {
			fmt.Fprintf(&b, "  def test_%s; assert_nil R.new.%s(false); end\n", g.name, g.name)
		} else {
			fmt.Fprintf(&b, "  def test_%s; assert_equal %s, R.new.%s(false); end\n", g.name, g.returns, g.name)
		}
	}
	b.WriteString("end\n")
	return b.String()
}()

// reachEdits are edits of lines no test hit that still change a test's
// result: Ruby evaluates the `when` condition, the literal's key, the
// local's declaration and the block's parameter list without counting the
// line. The guarded edits each change which names a line declares while
// keeping its words: a sigil, a dot, a comment mark, a number, a blank or a
// capture's name decides whether the later x is a local or a call of R#x.
// An edit marked guard also changes the line's brackets, so the structure
// guard keeps the static plan before the reach rule reads it.
var reachEdits = []struct {
	name, with, breaks string
	line               int
	guard              bool
}{
	{"one-line when", "    when 3 then :one", "RTest#test_kind3", 4, false},
	{"lambda in a multi-line literal", "      c: -> { 2 },", "RTest#test_keys", 13, false},
	{"local renamed on an unexecuted line", "      y = 1", "RTest#test_local", 19, false},
	{"numbered block parameter on an unexecuted line", "      1 + 1", "RTest#test_numbered", 26, false},
	{"local to instance variable on a rocket line", "      @x = {:a => 1}", "RTest#test_table", guardedLine("table"), false},
	{"local to global variable on a rocket line", "      $x = {:a => 1}", "RTest#test_table", guardedLine("table"), false},
	{"command argument to attribute assignment on a rocket line", "      puts.x = {:a => 1}", "RTest#test_command", guardedLine("command"), false},
	{"instance variable to local on a rocket line", "      x = {:a => 1}", "RTest#test_ivar", guardedLine("ivar"), false},
	{"assignment moved out of a comment that holds a rocket", "      puts flag, x = {flag => 1}", "RTest#test_comment", guardedLine("comment"), true},
	{"assignment moved out of a comment that holds a rocket in a string", "      puts flag, x = \"=>\"", "RTest#test_remark", guardedLine("remark"), false},
	{"hash argument to rightward pattern by dropping a number", "      puts => x", "RTest#test_number", guardedLine("number"), false},
	{"array argument to indexed pattern by dropping a blank", "      puts[1] => x", "RTest#test_index", guardedLine("index"), false},
	{"quoted named capture renamed", "      /(?'x'.)/ =~ 'a'", "RTest#test_capture", guardedLine("capture"), false},
	{"no-break space joins the assigned name to the command", "      puts\u00a0x = 1", "RTest#test_space", guardedLine("space"), false},
}

// reachIndex is the measured coverage of reachSource under one test a
// method; TestLiveLineReach checks it line by line against the index Ruby
// collects.
func reachIndex(t *testing.T) *linecov.Index {
	t.Helper()
	kind, handlers, local, numbered, x := span{2, 2, 8, 5}, span{10, 2, 15, 5}, span{17, 2, 22, 5}, span{24, 2, 29, 5}, span{31, 2, 33, 5}
	test := func(name string, lines []int32, methods ...span) record {
		return record{pkg: "test/case_test.rb", name: name, lines: map[string][]int32{"lib/r.rb": lines}, methods: map[string][]span{"lib/r.rb": methods}}
	}
	shape := fileShape{
		Lines:   []int32{1, 2, 3, 4, 5, 6, 10, 12, 13, 17, 18, 19, 21, 24, 25, 26, 28, 31, 32},
		Methods: []span{kind, handlers, local, numbered, x},
	}
	records := []record{
		test("RTest#test_kind3", []int32{3, 5}, kind),
		test("RTest#test_keys", []int32{12}, handlers),
		test("RTest#test_local", []int32{18, 21}, local),
		test("RTest#test_numbered", []int32{25, 28}, numbered),
	}
	load := []int32{1, 2, 10, 17, 24, 31}
	for i, g := range guarded {
		def := int32(guardedStart + 7*i)
		method := span{def, 2, def + 5, 5}
		shape.Lines = append(shape.Lines, def, def+1, def+2, def+4)
		shape.Methods = append(shape.Methods, method)
		load = append(load, def)
		if g.returns == "nil" {
			records = append(records, test("RTest#test_"+g.name, []int32{def + 1, def + 4}, method))
		} else {
			records = append(records, test("RTest#test_"+g.name, []int32{32, def + 1, def + 4}, x, method))
		}
	}
	run := &Run{
		Runner:   "ruby 3.3.8 minitest 5.27.0 coverage=lines,methods",
		isolated: true,
		files:    map[string]fileShape{"lib/r.rb": shape},
		records:  records,
	}
	for _, r := range run.records {
		run.Outcomes = append(run.Outcomes, Outcome{Package: r.pkg, Name: r.name, Status: "passed", Recorded: true})
	}
	run.records = append(run.records, record{pkg: Load, name: Load, lines: map[string][]int32{"lib/r.rb": load}, methods: map[string][]span{}})
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
// of a numbered block parameter, and an edit that changes what a line
// declares by a sigil, a dot, a comment mark, a number, a blank or a
// capture's name, selects the enclosing called method's tests although no
// test hit the line.
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
		if got := selected(ix, e); c.guard && !strings.HasPrefix(got, "fallback: edit changes Ruby block structure") || !c.guard && got != c.breaks {
			t.Errorf("%s: lib/r.rb:%d selects %q, want %q (structure fallback: %v)", c.name, c.line, got, c.breaks, c.guard)
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
	root := laneProject(t, reachTests, "lib/r.rb", reachSource)
	ctx := context.Background()
	base := git(t, root, "rev-parse", "HEAD")
	argv := []string{"ruby", "test/case_test.rb"}
	_, ix := collect(t, root, base, Options{Argv: argv})
	if len(ix.Unordered) != 0 || len(ix.Tests) != 5+len(guarded) {
		t.Fatalf("index tests %d unordered %v", len(ix.Tests), ix.Unordered)
	}
	// The index the unit tests build by hand is the one Ruby measures.
	byHand := reachIndex(t)
	for line := 1; line <= strings.Count(reachSource, "\n"); line++ {
		if got, want := lineTests(ix, "lib/r.rb", line), lineTests(byHand, "lib/r.rb", line); got != want {
			t.Errorf("lib/r.rb:%d: Ruby measured %q, reachIndex records %q", line, got, want)
		}
		_, counted := lineBlock(ix, "lib/r.rb", line)
		if _, want := lineBlock(byHand, "lib/r.rb", line); counted != want {
			t.Errorf("lib/r.rb:%d: Ruby counts the line: %v, reachIndex: %v", line, counted, want)
		}
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
		"r = 1if v",              // 34 keyword glued to a number
		"r = f(v)rescue 7",       // 35 keyword after a bracket
		"r = f(v)and",            // 36 continues: the keyword needs no blank
		"  g(v)",                 // 37 continued
		"r = 1or",                // 38 continues: the keyword is glued to a number
		"  g(v)",                 // 39 continued
		"w.not",                  // 40 whole: .not is a method
		"q = 0x1f",               // 41 whole
		"order(v)",               // 42 whole: order is not the keyword or
	}, "\n")
	s := readSide(source)
	whole := map[int]bool{1: true, 2: true, 3: true, 4: true, 5: true, 6: true, 26: true, 27: true, 28: true, 29: true, 30: true, 31: true, 40: true, 41: true, 42: true}
	for n := 1; n <= 42; n++ {
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
		{"h = {a => 1}", "h = {a => 1}", true},
		{"v => w", "v => w", true},
		{"\th  =\t{a => 1}  # why", "h = {a => 1} # why", true},
		{"a <=> b", "a <=> b", true},
		{"foo(a) # x = 1", "foo(a) # x =", true},
		{"puts\u00a0x = 1", "puts\u00a0x =", true},
		{`/(?'y'.)/ =~ s`, "", false},
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

// LPCV-V0-7605: the binding signature is the line's own text, so two lines
// that declare different names never compare equal, whatever distinguishes
// them: a sigil, a dot, a bracket, a comma, a comment mark, a number, a blank
// Ruby reads as part of a name, or a capture's name. Each pair was measured
// on Ruby 3.3.8: one line of the pair declares a local the other does not.
func TestBindingSignatureKeepsLineText(t *testing.T) {
	differ := func(a, b string) bool {
		sa, oka := binding(a, rubydialectCode(a))
		sb, okb := binding(b, rubydialectCode(b))
		return !oka || !okb || sa != sb
	}
	for _, c := range [][2]string{
		{"x = {:a => 1}", "@x = {:a => 1}"},
		{"x = {:a => 1}", "@@x = {:a => 1}"},
		{"x = {:a => 1}", "$x = {:a => 1}"},
		{"x = {:a => 1}", "x[0] = {:a => 1}"},
		{"x = {:a => 1}", "X = {:a => 1}"},
		{"foo x = {:a => 1}", "foo.x = {:a => 1}"},
		{"foo x = {:a => 1}", "foo&.x = {:a => 1}"},
		{"foo x = {:a => 1}", "foo::x = {:a => 1}"},
		{"foo(a) # x = {b => c}", "foo a, x = {b => c}"},
		{"foo(a) # x = b", "foo a, x = b"},
		{"foo 1 => x", "foo => x"},
		{"foo [1] => x", "foo[1] => x"},
		{"foo :a => x", "foo => x"},
		{"foo \"a\" => x", "foo => x"},
		{"v => {a: 1}", "v => {a:}"},
		{"v => [1, b]", "v => [a, b]"},
		{"puts x = 1", "puts\u00a0x = 1"},
		{"/(?'w'.)/ =~ s", "/(?'x'.)/ =~ s"},
		{"/(?<w>.)/ =~ s", "/(?<x>.)/ =~ s"},
	} {
		if !differ(c[0], c[1]) {
			sig, _ := binding(c[0], rubydialectCode(c[0]))
			t.Errorf("%q and %q declare different names and share the signature %q", c[0], c[1], sig)
		}
	}
	// Blanks, the value after an assignment and a comment after it keep the
	// signature, so those edits stay bounded by the line's hits.
	for _, c := range [][2]string{
		{"x = 1", "x  =\t2"},
		{"      x = 1", "x = 1"},
		{"x = 1 # one", "x = 2 # two"},
		{"h = {a => 1}", "  h =  {a => 1}"},
		{"foo(1)", "bar(2) # note"},
	} {
		if differ(c[0], c[1]) {
			t.Errorf("%q and %q declare the same names and differ in signature", c[0], c[1])
		}
	}
}

// localTables asks ruby on PATH which locals each line declares when it is
// the body of a method taking flag: the method scope's local table, or "!"
// for a line that does not parse.
func localTables(t *testing.T, lines []string) []string {
	t.Helper()
	script := `STDIN.each_line(chomp: true) do |line|
  begin
    root = RubyVM::AbstractSyntaxTree.parse("def probe(flag)\n#{line}\nend\n")
    puts root.children[2].children[1].children[0].join(",")
  rescue SyntaxError
    puts "!"
  end
end
`
	cmd := exec.Command("ruby", "-W0", "-e", script)
	cmd.Stdin = strings.NewReader(strings.Join(lines, "\n") + "\n")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("ruby local tables: %v", err)
	}
	tables := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(tables) != len(lines) {
		t.Fatalf("ruby answered %d of %d lines", len(tables), len(lines))
	}
	return tables
}

// LPCV-V0-7605: measured against Ruby's own parser. Over every combination
// of an assignment target, an operator and a value below, two whole one-line
// statements that share a binding signature declare the same locals, and a
// statement with no signature declares none.
func TestLiveBindingSignatureAgreesWithRuby(t *testing.T) {
	requireRuby(t, "minitest")
	targets := []string{
		"x", "y", "@x", "@@x", "$x", "x[0]", "x.y", "self.x", "foo x", "foo  x", "foo\tx", "foo x", "foo.x", "foo&.x", "foo::x",
		"foo 1, x", "foo(1) # x", "foo [1], x", "x, y", "x, @y", "*x", "(x, y), z", "foo(x)", "foo 1", "foo [1]", "foo[1]", "foo", "foo :a", `foo "a"`,
	}
	operators := []string{" = ", "=", " =  ", " ||= ", " += ", " <<= ", " => ", "=>", " == ", " != ", " =~ ", " <=> ", " <= "}
	values := []string{
		"1", "2", "x", "y", "{:a => 1}", "{:a => 2}", "{a => b}", "{a: 1}", "{a:}", "[a, b]", "[1, b]", "foo(y = 1)", `"s"`, `"#{y = 1}"`,
		"y # z = 1", "z => y", "z = {y => 1}", "->(y) { y }", "foo { |y| y }", "y.z = 1", "@y = 1",
	}
	var lines []string
	for _, target := range targets {
		for _, operator := range operators {
			for _, value := range values {
				lines = append(lines, target+operator+value)
			}
		}
	}
	tables := localTables(t, lines)
	type group struct{ line, table string }
	groups := map[string]group{"": {"foo(1)", "flag"}}
	parsed, bounded := 0, 0
	for i, line := range lines {
		if tables[i] == "!" {
			continue
		}
		parsed++
		side := readSide(line)
		sig, ok := binding(line, side.state[0].Code)
		if !ok || side.whole(1) != "" {
			continue
		}
		bounded++
		first, seen := groups[sig]
		if !seen {
			groups[sig] = group{line, tables[i]}
			continue
		}
		if first.table != tables[i] {
			t.Errorf("%q declares %s and %q declares %s, but both have the signature %q", first.line, first.table, line, tables[i], sig)
		}
	}
	// The comparison is not vacuous: most combinations parse, and some
	// signatures are shared.
	if parsed < len(lines)/2 || bounded < parsed/2 || len(groups) >= bounded {
		t.Fatalf("%d lines, %d parse, %d bounded, %d signatures", len(lines), parsed, bounded, len(groups))
	}
	t.Logf("%d lines, %d parse, %d bounded, %d signatures", len(lines), parsed, bounded, len(groups))
}
