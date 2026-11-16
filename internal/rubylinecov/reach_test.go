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
	"github.com/Beamfall/corvint/internal/rubydialect"
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

// percentGuarded are methods of the guarded shape whose OLD holds a percent
// literal delimited by a blank. They live in lib/q.rb, so that lib/r.rb holds
// no such literal and its edits are compared in the squeezed mode.
var percentGuarded = []struct{ name, old, returns string }{
	{"percent", "%  =>x ", "nil"},
	{"tabpercent", "[%\t\t,x=1\t]", "nil"},
	{"qpercent", "[%q  ,x=1 ]", "nil"},
	{"tabqpercent", "%Q\t\t=>x\t", "nil"},
}

// percentStart is the line of the first percentGuarded method's def in
// lib/q.rb, after Q#x and Q#y.
const percentStart = 10

// percentLine is the line of the named percentGuarded method's OLD statement.
func percentLine(name string) int {
	for i, g := range percentGuarded {
		if g.name == name {
			return percentStart + 7*i + 2
		}
	}
	panic("no percent method " + name)
}

// closers are the two methods at the end of lib/q.rb, eight lines each with
// the blank line before them:
//
//	def NAME(flag)
//	  if flag
//	    OLD
//	\t\ty=1 ; x = 2
//	  end
//	  y
//	end
//
// OLD ends in the blank that closes its space-delimited literal. Without that
// blank the literal runs onto the next line and swallows `y=1`, so y is no
// longer a local and the method returns Q#y's :method instead of nil.
var closers = []struct{ name, old string }{
	{"trailing", "p 1, % a "},
	{"assigned", "z = % a "},
}

// closerLine is the line of the named closer's OLD statement.
func closerLine(name string) int {
	for i, c := range closers {
		if c.name == name {
			return percentStart + 7*len(percentGuarded) + 8*i + 2
		}
	}
	panic("no closer " + name)
}

// percentSource is lib/q.rb of the reach fixtures.
var percentSource = func() string {
	var b strings.Builder
	b.WriteString("class Q\n  def x\n    :method\n  end\n\n  def y\n    :method\n  end\n")
	for _, g := range percentGuarded {
		b.WriteString("\n  def " + g.name + "(flag)\n    if flag\n      " + g.old + "\n    end\n    x\n  end\n")
	}
	for _, c := range closers {
		b.WriteString("\n  def " + c.name + "(flag)\n    if flag\n      " + c.old + "\n\t\ty=1 ; x = 2\n    end\n    y\n  end\n")
	}
	b.WriteString("end\n")
	return b.String()
}()

// reachSources are the fixture's library files by path.
var reachSources = map[string]string{"lib/r.rb": reachSource, "lib/q.rb": percentSource}

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
require_relative '../lib/q'
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
	for _, g := range percentGuarded {
		fmt.Fprintf(&b, "  def test_%s; assert_nil Q.new.%s(false); end\n", g.name, g.name)
	}
	for _, c := range closers {
		fmt.Fprintf(&b, "  def test_%s; assert_nil Q.new.%s(false); end\n", c.name, c.name)
	}
	b.WriteString("end\n")
	return b.String()
}()

// reachEdits are edits of lines no test hit that still change a test's
// result: Ruby evaluates the `when` condition, the literal's key, the
// local's declaration and the block's parameter list without counting the
// line. The guarded edits each change which names a line declares while
// keeping its words: a sigil, a dot, a comment mark, a number, a blank, the
// length of a blank run that delimits a percent literal or a capture's name
// decides whether the later x is a local or a call of R#x.
// An edit marked guard also changes the line's brackets, so the structure
// guard keeps the static plan before the reach rule reads it.
var reachEdits = []struct {
	name, with, breaks string
	line               int
	guard              bool
	path               string
}{
	{"one-line when", "    when 3 then :one", "RTest#test_kind3", 4, false, "lib/r.rb"},
	{"lambda in a multi-line literal", "      c: -> { 2 },", "RTest#test_keys", 13, false, "lib/r.rb"},
	{"local renamed on an unexecuted line", "      y = 1", "RTest#test_local", 19, false, "lib/r.rb"},
	{"numbered block parameter on an unexecuted line", "      1 + 1", "RTest#test_numbered", 26, false, "lib/r.rb"},
	{"local to instance variable on a rocket line", "      @x = {:a => 1}", "RTest#test_table", guardedLine("table"), false, "lib/r.rb"},
	{"local to global variable on a rocket line", "      $x = {:a => 1}", "RTest#test_table", guardedLine("table"), false, "lib/r.rb"},
	{"command argument to attribute assignment on a rocket line", "      puts.x = {:a => 1}", "RTest#test_command", guardedLine("command"), false, "lib/r.rb"},
	{"instance variable to local on a rocket line", "      x = {:a => 1}", "RTest#test_ivar", guardedLine("ivar"), false, "lib/r.rb"},
	{"assignment moved out of a comment that holds a rocket", "      puts flag, x = {flag => 1}", "RTest#test_comment", guardedLine("comment"), true, "lib/r.rb"},
	{"assignment moved out of a comment that holds a rocket in a string", "      puts flag, x = \"=>\"", "RTest#test_remark", guardedLine("remark"), false, "lib/r.rb"},
	{"hash argument to rightward pattern by dropping a number", "      puts => x", "RTest#test_number", guardedLine("number"), false, "lib/r.rb"},
	{"array argument to indexed pattern by dropping a blank", "      puts[1] => x", "RTest#test_index", guardedLine("index"), false, "lib/r.rb"},
	{"quoted named capture renamed", "      /(?'x'.)/ =~ 'a'", "RTest#test_capture", guardedLine("capture"), false, "lib/r.rb"},
	{"no-break space joins the assigned name to the command", "      puts\u00a0x = 1", "RTest#test_space", guardedLine("space"), false, "lib/r.rb"},
	// A percent literal delimited by a blank closes at the next blank, so the
	// length of a blank run decides where the literal ends and what follows.
	{"space-delimited percent literal swallows a rightward pattern", "      % =>x ", "RTest#test_percent", percentLine("percent"), false, "lib/q.rb"},
	{"tab-delimited percent literal swallows an assignment", "      [%\t,x=1\t]", "RTest#test_tabpercent", percentLine("tabpercent"), false, "lib/q.rb"},
	{"space-delimited %q literal swallows an assignment", "      [%q ,x=1 ]", "RTest#test_qpercent", percentLine("qpercent"), false, "lib/q.rb"},
	{"tab-delimited %Q literal swallows a rightward pattern", "      %Q\t=>x\t", "RTest#test_tabqpercent", percentLine("tabqpercent"), false, "lib/q.rb"},
	// The blank that closes a space-delimited literal sits after the line's
	// last operator, or on a line with none: without it the literal runs onto
	// the next line and swallows that line's assignment.
	{"closing blank of a percent literal removed after a command", "      p 1, % a", "RTest#test_trailing", closerLine("trailing"), false, "lib/q.rb"},
	{"closing blank of a percent literal removed after an assignment", "      z = % a", "RTest#test_assigned", closerLine("assigned"), false, "lib/q.rb"},
}

// reachIndex is the measured coverage of reachSource and percentSource under
// one test a method; TestLiveLineReach checks it line by line against the
// index Ruby collects.
func reachIndex(t *testing.T) *linecov.Index {
	t.Helper()
	kind, handlers, local, numbered, x := span{2, 2, 8, 5}, span{10, 2, 15, 5}, span{17, 2, 22, 5}, span{24, 2, 29, 5}, span{31, 2, 33, 5}
	test := func(name, path string, lines []int32, methods ...span) record {
		return record{pkg: "test/case_test.rb", name: name, lines: map[string][]int32{path: lines}, methods: map[string][]span{path: methods}}
	}
	shape := fileShape{
		Lines:   []int32{1, 2, 3, 4, 5, 6, 10, 12, 13, 17, 18, 19, 21, 24, 25, 26, 28, 31, 32},
		Methods: []span{kind, handlers, local, numbered, x},
	}
	records := []record{
		test("RTest#test_kind3", "lib/r.rb", []int32{3, 5}, kind),
		test("RTest#test_keys", "lib/r.rb", []int32{12}, handlers),
		test("RTest#test_local", "lib/r.rb", []int32{18, 21}, local),
		test("RTest#test_numbered", "lib/r.rb", []int32{25, 28}, numbered),
	}
	load := []int32{1, 2, 10, 17, 24, 31}
	for i, g := range guarded {
		def := int32(guardedStart + 7*i)
		method := span{def, 2, def + 5, 5}
		shape.Lines = append(shape.Lines, def, def+1, def+2, def+4)
		shape.Methods = append(shape.Methods, method)
		load = append(load, def)
		if g.returns == "nil" {
			records = append(records, test("RTest#test_"+g.name, "lib/r.rb", []int32{def + 1, def + 4}, method))
		} else {
			records = append(records, test("RTest#test_"+g.name, "lib/r.rb", []int32{32, def + 1, def + 4}, x, method))
		}
	}
	// lib/q.rb: Q#x and Q#y, which no test calls, then the percent methods and
	// the closers. Ruby counts a closer's second body line too.
	percent := fileShape{Lines: []int32{1, 2, 3, 6, 7}, Methods: []span{{2, 2, 4, 5}, {6, 2, 8, 5}}}
	percentLoad := []int32{1, 2, 6}
	for i, g := range percentGuarded {
		def := int32(percentStart + 7*i)
		method := span{def, 2, def + 5, 5}
		percent.Lines = append(percent.Lines, def, def+1, def+2, def+4)
		percent.Methods = append(percent.Methods, method)
		percentLoad = append(percentLoad, def)
		records = append(records, test("RTest#test_"+g.name, "lib/q.rb", []int32{def + 1, def + 4}, method))
	}
	for i, c := range closers {
		def := int32(percentStart + 7*len(percentGuarded) + 8*i)
		method := span{def, 2, def + 6, 5}
		percent.Lines = append(percent.Lines, def, def+1, def+2, def+3, def+5)
		percent.Methods = append(percent.Methods, method)
		percentLoad = append(percentLoad, def)
		records = append(records, test("RTest#test_"+c.name, "lib/q.rb", []int32{def + 1, def + 5}, method))
	}
	run := &Run{
		Runner:   "ruby 3.3.8 minitest 5.27.0 coverage=lines,methods",
		isolated: true,
		files:    map[string]fileShape{"lib/r.rb": shape, "lib/q.rb": percent},
		records:  records,
	}
	for _, r := range run.records {
		run.Outcomes = append(run.Outcomes, Outcome{Package: r.pkg, Name: r.name, Status: "passed", Recorded: true})
	}
	run.records = append(run.records, record{pkg: Load, name: Load, lines: map[string][]int32{"lib/r.rb": load, "lib/q.rb": percentLoad}, methods: map[string][]span{}})
	ix, err := Index(".", "c", run, map[string]string{"lib/r.rb": "old", "lib/q.rb": "old", "test/case_test.rb": "t"})
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
// test hit the line. So does every edit of lib/q.rb, which may hold a
// percent literal a blank delimits: the rule that holds without Ruby's own
// reading of the file.
func TestAnalyzeLinesUnboundedReach(t *testing.T) {
	ix := reachIndex(t)
	for _, c := range reachEdits {
		e := AnalyzeLines(ix, []linecov.FileDiff{lineDiff(c.path, c.line)}, reachEdit(c.path, c.line, c.with))
		if got := selected(ix, e); c.guard && !strings.HasPrefix(got, "fallback: edit changes Ruby block structure") || !c.guard && got != c.breaks {
			t.Errorf("%s: %s:%d selects %q, want %q (structure fallback: %v)", c.name, c.path, c.line, got, c.breaks, c.guard)
		}
		p, _ := Plan(ix, []string{"test/case_test.rb"}, linecov.Effect{}, e)
		if !kept(p["test/case_test.rb"], c.breaks) {
			t.Errorf("%s: the plan %+v drops %s", c.name, p["test/case_test.rb"], c.breaks)
		}
	}
}

// lineDiff is the diff of one replaced line of path.
func lineDiff(path string, line int) linecov.FileDiff {
	return linecov.FileDiff{Path: path, OldOID: "old", NewOID: "new", Hunks: []linecov.Hunk{{Start: line, End: line}}, NewHunks: []linecov.Hunk{{Start: line, End: line}}}
}

// reachEdit is both sides of a reach fixture file with one line replaced,
// read without Ruby.
func reachEdit(path string, line int, with string) Sources {
	newSource := replaceLine(reachSources[path], line, with)
	return Sources{
		Old:   func(string) ([]byte, error) { return []byte(reachSources[path]), nil },
		New:   func(string) ([]byte, error) { return []byte(newSource), nil },
		Check: func(string, []byte) error { return nil },
		Lex:   lexerAgrees,
	}
}

// lexerAgrees stands in for Ruby's lexer in the tests that run without Ruby:
// it reads a source as the lexer does, so every rule above the lexer is
// exercised and none is hidden behind a disagreement.
func lexerAgrees(_ string, src []byte) ([]rubydialect.LineState, error) {
	return rubydialect.LineStates(string(src)), nil
}

// liveSources reads both sides from Git and asks ruby on PATH for the syntax
// check and for its own reading.
func liveSources(ctx context.Context, root, base, head string) Sources {
	return Sources{Old: linecov.Reader(ctx, root, base), New: linecov.Reader(ctx, root, head), Check: rubyCheck, Lex: RubyLexer(ctx, "ruby", nil)}
}

// misreadFallback starts the fallback of a file whose side Ruby reads
// differently from the lexer.
const misreadFallback = "fallback: Ruby reads the old side of "

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
// fails a test, and the line-level plan keeps every test that fails. Ruby
// reads lib/r.rb as the lexer does, so its edits reach the rules above the
// lexer and select what the tests without Ruby select; Ruby reads the
// blank-delimited literals of lib/q.rb, which the lexer does not, so every
// edit there keeps the static plan.
func TestLiveLineReach(t *testing.T) {
	requireRuby(t, "minitest")
	root := laneProject(t, reachTests, "lib/r.rb", reachSource, "lib/q.rb", percentSource)
	ctx := context.Background()
	base := git(t, root, "rev-parse", "HEAD")
	argv := []string{"ruby", "test/case_test.rb"}
	_, ix := collect(t, root, base, Options{Argv: argv})
	if len(ix.Unordered) != 0 || len(ix.Tests) != 5+len(guarded)+len(percentGuarded)+len(closers) {
		t.Fatalf("index tests %d unordered %v", len(ix.Tests), ix.Unordered)
	}
	// The index the unit tests build by hand is the one Ruby measures.
	byHand := reachIndex(t)
	for path, source := range reachSources {
		for line := 1; line <= strings.Count(source, "\n"); line++ {
			if got, want := lineTests(ix, path, line), lineTests(byHand, path, line); got != want {
				t.Errorf("%s:%d: Ruby measured %q, reachIndex records %q", path, line, got, want)
			}
			_, counted := lineBlock(ix, path, line)
			if _, want := lineBlock(byHand, path, line); counted != want {
				t.Errorf("%s:%d: Ruby counts the line: %v, reachIndex: %v", path, line, counted, want)
			}
		}
	}
	// The premise of each edit: no test hit the edited line.
	for _, c := range reachEdits {
		if got := lineTests(ix, c.path, c.line); got != "" {
			t.Fatalf("%s: %s:%d was hit by %q", c.name, c.path, c.line, got)
		}
	}
	for _, c := range reachEdits {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(c.path)), []byte(replaceLine(reachSources[c.path], c.line, c.with)), 0o600); err != nil {
			t.Fatal(err)
		}
		git(t, root, "commit", "-qam", c.name)
		head := git(t, root, "rev-parse", "HEAD")
		diffs, err := linecov.Diff(ctx, root, base, head)
		if err != nil {
			t.Fatal(err)
		}
		e := AnalyzeLines(ix, diffs, liveSources(ctx, root, base, head))
		switch got := selected(ix, e); {
		case c.path == "lib/q.rb":
			if !strings.HasPrefix(got, misreadFallback+"lib/q.rb differently from the lexer: line ") {
				t.Errorf("%s: %s:%d selects %q, want the fallback of a file Ruby reads differently", c.name, c.path, c.line, got)
			}
		case c.guard:
			if !strings.HasPrefix(got, "fallback: edit changes Ruby block structure") {
				t.Errorf("%s: %s:%d selects %q, want the structure fallback", c.name, c.path, c.line, got)
			}
		case got != c.breaks:
			t.Errorf("%s: %s:%d selects %q, want %q", c.name, c.path, c.line, got, c.breaks)
		}
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

// bodySource is lib/s.rb of the string-body fixture: a run of blanks inside
// a string, as an assigned value and as a pattern, on lines a test hits and
// on a line no test hits.
const bodySource = `class S
  def text
    s = "a  b"
    s
  end

  def pair
    ["a  b", 1] => ["a  b", y]
    y
  end

  def unused(flag)
    if flag
      t = "a  b"
    end
    t
  end
end
`

// LPCV-V0-7605: measured end to end. Squeezing a run of blanks inside a
// string body changes a value, never a binding, so an edit of the run is
// harmless on a line no test hits and selected by its hits on a line a test
// hits: the suite at the edited head fails exactly the tests the plan keeps.
func TestLiveBlankRunInStringBody(t *testing.T) {
	requireRuby(t, "minitest")
	tests := `require 'minitest/autorun'
require_relative '../lib/s'
class STest < Minitest::Test
  def test_text; assert_equal "a  b", S.new.text; end
  def test_pair; assert_equal 1, S.new.pair; end
  def test_unused; assert_nil S.new.unused(false); end
end
`
	root := laneProject(t, tests, "lib/s.rb", bodySource)
	ctx := context.Background()
	base := git(t, root, "rev-parse", "HEAD")
	argv := []string{"ruby", "test/case_test.rb"}
	_, ix := collect(t, root, base, Options{Argv: argv})
	for _, c := range []struct {
		name, with, hits, breaks string
		line                     int
	}{
		{"assigned string on a hit line", `    s = "a b"`, "STest#test_text", "STest#test_text", 3},
		{"pattern string on a hit line", `    ["a  b", 1] => ["a b", y]`, "STest#test_pair", "STest#test_pair", 8},
		{"assigned string on a line no test hits", `      t = "a b"`, "", "", 14},
	} {
		if got := lineTests(ix, "lib/s.rb", c.line); got != c.hits {
			t.Fatalf("%s: lib/s.rb:%d was hit by %q, want %q", c.name, c.line, got, c.hits)
		}
		if err := os.WriteFile(filepath.Join(root, "lib", "s.rb"), []byte(replaceLine(bodySource, c.line, c.with)), 0o600); err != nil {
			t.Fatal(err)
		}
		git(t, root, "commit", "-qam", c.name)
		head := git(t, root, "rev-parse", "HEAD")
		diffs, err := linecov.Diff(ctx, root, base, head)
		if err != nil {
			t.Fatal(err)
		}
		e := AnalyzeLines(ix, diffs, liveSources(ctx, root, base, head))
		if got := selected(ix, e); got != c.breaks {
			t.Errorf("%s: lib/s.rb:%d selects %q, want %q", c.name, c.line, got, c.breaks)
		}
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
			t.Errorf("%s: the edit fails %v, want %q", c.name, failing, c.breaks)
		}
		git(t, root, "reset", "-q", "--hard", base)
	}
}

// beyond are library files in which one edited line of method a changes a
// method the edit is not in, through a literal the lexer does not read. Each
// was measured on Ruby 3.3.8: both sides parse, and the named test passes at
// the base and fails at the edited head.
var beyond = []struct {
	name, path, source, with, breaks string
	line                             int
}{
	// Without its closing blank the literal runs to the next space, in the
	// comment four lines on: it swallows the end of a and the second b.
	{"closing blank removed, literal swallows a later method", "lib/c1.rb",
		"class C1\n\tdef b;2;end\n\tdef a\n\t\tz = % a \n\tend\n\tdef\tb;1;end\n\t# ;end\nend\n",
		"\t\tz = % a", "CTest#test_b1", 4},
	// With a second blank the literal closes empty, and the quote it held
	// opens a string that runs to the comment and swallows the second b.
	{"blank added, a quote leaves the literal and opens a string", "lib/c2.rb",
		"class C2\n  def b; 2; end\n  def a\n    z = % \" \n  end\n  def b; 1; end\n  # \"; end\nend\n",
		"    z = %  \" ", "CTest#test_b2", 4},
	// A percent sign at the end of a line opens a literal the newline
	// delimits: the next line is its body. With a left operand it is the
	// modulo operator, and the next line is code that redefines y.
	{"newline-delimited literal becomes an operator", "lib/c3.rb",
		"class C3\n  def y ; 1 ; end\n  def a\n    z = %\n    y ; end ; def y ; 2\n  end\nend\n",
		"    z = 1 %", "CTest#test_y3", 4},
}

// LPCV-V0-7605: measured end to end. An edit whose reach the lexer cannot
// see, because Ruby reads a literal where the lexer reads code, keeps the
// static plan and so every test that fails at the edited head: Ruby's own
// reading of the file differs from the lexer's. Selecting the tests of the
// enclosing called method is not enough here, since the failing test calls
// another method.
func TestLiveEditBeyondTheLexer(t *testing.T) {
	requireRuby(t, "minitest")
	tests := `require 'minitest/autorun'
require_relative '../lib/c1'
require_relative '../lib/c2'
require_relative '../lib/c3'
class CTest < Minitest::Test
  def test_a1; C1.new.a; assert true; end
  def test_b1; assert_equal 1, C1.new.b; end
  def test_a2; C2.new.a; assert true; end
  def test_b2; assert_equal 1, C2.new.b; end
  def test_a3; C3.new.a; assert true; end
  def test_y3; assert_equal 1, C3.new.y; end
end
`
	var files []string
	for _, c := range beyond {
		files = append(files, c.path, c.source)
	}
	root := laneProject(t, tests, files...)
	ctx := context.Background()
	base := git(t, root, "rev-parse", "HEAD")
	argv := []string{"ruby", "test/case_test.rb"}
	run, ix := collect(t, root, base, Options{Argv: argv})
	if run.ExitCode() != 0 || len(ix.Tests) != 7 {
		t.Fatalf("base outcomes %+v", run.Outcomes)
	}
	for _, c := range beyond {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(c.path)), []byte(replaceLine(c.source, c.line, c.with)), 0o600); err != nil {
			t.Fatal(err)
		}
		git(t, root, "commit", "-qam", c.name)
		head := git(t, root, "rev-parse", "HEAD")
		diffs, err := linecov.Diff(ctx, root, base, head)
		if err != nil {
			t.Fatal(err)
		}
		e := AnalyzeLines(ix, diffs, liveSources(ctx, root, base, head))
		if got := selected(ix, e); !strings.HasPrefix(got, misreadFallback+c.path+" differently from the lexer: line ") {
			t.Errorf("%s: %s:%d selects %q, want the fallback of a file Ruby reads differently", c.name, c.path, c.line, got)
		}
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

// LPCV-V0-7605: without Ruby's reading the same three edits are held by the
// rules above the lexer alone, and those rules are not enough: each selects
// only the tests of the method the edit is in. This is what the lexer
// comparison is for, and why a missing lexer forbids narrowing.
func TestEditBeyondTheLexerNeedsRuby(t *testing.T) {
	for _, c := range beyond {
		newSource := replaceLine(c.source, c.line, c.with)
		src := Sources{
			Old:   func(string) ([]byte, error) { return []byte(c.source), nil },
			New:   func(string) ([]byte, error) { return []byte(newSource), nil },
			Check: func(string, []byte) error { return nil },
		}
		ix := beyondIndex(t, c.path)
		d := lineDiff(c.path, c.line)
		if got := selected(ix, AnalyzeLines(ix, []linecov.FileDiff{d}, src)); got != "fallback: no Ruby lexer for "+c.path {
			t.Errorf("%s: without a lexer the edit selects %q", c.name, got)
		}
		src.Lex = lexerAgrees
		if got := selected(ix, AnalyzeLines(ix, []linecov.FileDiff{d}, src)); got != "CTest#test_a" {
			t.Errorf("%s: with a lexer that agrees the edit selects %q, want only CTest#test_a", c.name, got)
		}
		// A reading that differs on any line, in its code or in the state the
		// line begins or ends in, or that cannot be had, keeps the static plan.
		for name, lex := range map[string]func(string, []byte) ([]rubydialect.LineState, error){
			"code": func(_ string, src []byte) ([]rubydialect.LineState, error) {
				states := rubydialect.LineStates(string(src))
				states[c.line-1].Code = ""
				return states, nil
			},
			"inside": func(_ string, src []byte) ([]rubydialect.LineState, error) {
				states := rubydialect.LineStates(string(src))
				states[c.line].Inside = !states[c.line].Inside
				return states, nil
			},
			"open": func(_ string, src []byte) ([]rubydialect.LineState, error) {
				states := rubydialect.LineStates(string(src))
				states[c.line-1].Open = !states[c.line-1].Open
				return states, nil
			},
			"length": func(_ string, src []byte) ([]rubydialect.LineState, error) {
				return rubydialect.LineStates(string(src))[1:], nil
			},
			"error": func(string, []byte) ([]rubydialect.LineState, error) { return nil, fmt.Errorf("no ruby") },
		} {
			src.Lex = lex
			if got := selected(ix, AnalyzeLines(ix, []linecov.FileDiff{d}, src)); !strings.HasPrefix(got, misreadFallback+c.path+" differently from the lexer: ") {
				t.Errorf("%s: a reading that differs in %s selects %q", c.name, name, got)
			}
		}
		// Trailing blanks are not part of a reading.
		src.Lex = func(_ string, src []byte) ([]rubydialect.LineState, error) {
			states := rubydialect.LineStates(string(src))
			for i := range states {
				states[i].Code += " \t"
			}
			return states, nil
		}
		if got := selected(ix, AnalyzeLines(ix, []linecov.FileDiff{d}, src)); got != "CTest#test_a" {
			t.Errorf("%s: a reading that differs in trailing blanks selects %q", c.name, got)
		}
	}
}

// beyondIndex is the coverage of one beyond file, whose method a spans lines
// 3 to 5 or 6 and is called by one test, with line 4 counted and hit.
func beyondIndex(t *testing.T, path string) *linecov.Index {
	t.Helper()
	a := span{3, 2, 6, 5}
	run := &Run{
		Runner:   "ruby 3.3.8 minitest 5.27.0 coverage=lines,methods",
		isolated: true,
		files:    map[string]fileShape{path: {Lines: []int32{1, 2, 3, 4}, Methods: []span{a}}},
		records: []record{
			{pkg: "test/case_test.rb", name: "CTest#test_a", lines: map[string][]int32{path: {4}}, methods: map[string][]span{path: {a}}},
			{pkg: "test/case_test.rb", name: "CTest#test_b", lines: map[string][]int32{path: {2}}, methods: map[string][]span{}},
		},
	}
	for _, r := range run.records {
		run.Outcomes = append(run.Outcomes, Outcome{Package: r.pkg, Name: r.name, Status: "passed", Recorded: true})
	}
	run.records = append(run.records, record{pkg: Load, name: Load, lines: map[string][]int32{path: {1, 3}}, methods: map[string][]span{}})
	ix, err := Index(".", "c", run, map[string]string{path: "old", "test/case_test.rb": "t"})
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

// LPCV-V0-7605: an edit that keeps a whole statement's bindings narrows to
// the line's hits, and to none on a line no test hit, in a file with no
// percent literal a blank may delimit (lib/r.rb). In a file that may hold one
// (lib/q.rb) no replaced hunk is bounded by its line's text or hits: it
// selects the enclosing called method's tests.
func TestAnalyzeLinesBoundedReach(t *testing.T) {
	ix := reachIndex(t)
	for _, c := range []struct {
		name, with, want string
		line             int
		path             string
	}{
		{"same local, other value, unexecuted", "      x = 2", "", 19, "lib/r.rb"},
		{"executed read", "    x.to_s", "RTest#test_local", 21, "lib/r.rb"},
		{"executed call", "    f.arity + 0", "RTest#test_numbered", 28, "lib/r.rb"},
		// A method name's ! suffix and a comma before a string literal are not
		// continuations, so neither edit changes the block structure.
		{"statement ending in a method suffix", "    fail!", "RTest#test_local", 21, "lib/r.rb"},
		{"statement ending in a literal", "    raise E, \"no\"", "RTest#test_local", 21, "lib/r.rb"},
		{"statement continued onto the next line", "    x +", "fallback: edit changes Ruby block structure lib/r.rb: keywords, brackets or continuations differ at line 21", 21, "lib/r.rb"},
		{"when body moved off its condition stays with the method", "    when 1 then :uno", "RTest#test_kind3", 4, "lib/r.rb"},
		// The squeezed comparison: indentation and blank runs keep the binding.
		{"re-indented local, unexecuted", "    x = 1", "", 19, "lib/r.rb"},
		{"blank runs around the operator of an unexecuted local", "\t\tx  =\t1", "", 19, "lib/r.rb"},
		// lib/q.rb may hold a percent literal delimited by a blank.
		{"re-indented assignment in a file with a blank-delimited literal", "    z = % a ", "RTest#test_assigned", closerLine("assigned"), "lib/q.rb"},
		{"other argument, same bindings, in a file with a blank-delimited literal", "      p 2, % a ", "RTest#test_trailing", closerLine("trailing"), "lib/q.rb"},
		{"other value, same binding, in a file with a blank-delimited literal", "      z = % b ", "RTest#test_assigned", closerLine("assigned"), "lib/q.rb"},
	} {
		if got := selected(ix, AnalyzeLines(ix, []linecov.FileDiff{lineDiff(c.path, c.line)}, reachEdit(c.path, c.line, c.with))); got != c.want {
			t.Errorf("%s: %s:%d selects %q, want %q", c.name, c.path, c.line, got, c.want)
		}
	}
}

// LPCV-V0-7605: in a file that may hold a percent literal a blank delimits,
// a deleted line and an inserted line select the enclosing called method's
// tests as a replaced one does, although no test hit the lines around them.
func TestAnalyzeLinesBlankDelimitedFileHunkKinds(t *testing.T) {
	ix := reachIndex(t)
	line := closerLine("trailing")
	lines := strings.Split(percentSource, "\n")
	deleted := strings.Join(append(append([]string{}, lines[:line-1]...), lines[line:]...), "\n")
	inserted := strings.Join(append(append(append([]string{}, lines[:line]...), "      p 2"), lines[line:]...), "\n")
	for _, c := range []struct {
		name, source string
		old, new     linecov.Hunk
	}{
		{"deleted line", deleted, linecov.Hunk{Start: line, End: line}, linecov.Hunk{Start: line - 1, End: line - 2}},
		{"inserted line", inserted, linecov.Hunk{Start: line, End: line - 1}, linecov.Hunk{Start: line + 1, End: line + 1}},
	} {
		src := reachEdit("lib/q.rb", line, "")
		src.New = func(string) ([]byte, error) { return []byte(c.source), nil }
		d := linecov.FileDiff{Path: "lib/q.rb", OldOID: "old", NewOID: "new", Hunks: []linecov.Hunk{c.old}, NewHunks: []linecov.Hunk{c.new}}
		if got := selected(ix, AnalyzeLines(ix, []linecov.FileDiff{d}, src)); got != "RTest#test_trailing" {
			t.Errorf("%s at lib/q.rb:%d selects %q, want RTest#test_trailing", c.name, line, got)
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
		"q = 0xdef",              // 43 whole: the hex digits d, e and f are not the keyword def
		"q = 0xabcdef",           // 44 whole
		"r = 0x1fif v",           // 45 keyword: Ruby ends the hex number at the i
		"r = 0b1_0if v",          // 46 keyword after a binary number
		"q = 0xdeend",            // 47 whole: the hex number takes the second e, so nd is a name
		"r = 0o17if v",           // 48 keyword after an octal number
		"r = 0d9if v",            // 49 keyword after a decimal-prefixed number
	}, "\n")
	s := readSide(source)
	whole := map[int]bool{1: true, 2: true, 3: true, 4: true, 5: true, 6: true, 26: true, 27: true, 28: true, 29: true, 30: true, 31: true, 40: true, 41: true, 42: true, 43: true, 44: true, 47: true}
	for n := 1; n <= len(s.raw); n++ {
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
		// A line that may hold a blank-delimited percent literal is never
		// compared: its file bounds no hunk.
		if holdsBlankDelimited(a) || holdsBlankDelimited(b) {
			return true
		}
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
		// The length of a blank run that delimits a percent literal, and the
		// blank that closes one.
		{"p 1, % a ", "p 1, % a"},
		{"z = % a ", "z = % a"},
		{"%  =>x ", "% =>x "},
		{"%\t\t=>x\t", "%\t=>x\t"},
		{"%q  =>x ", "%q =>x "},
		{"%Q\t\t=>x\t", "%Q\t=>x\t"},
		{"[%  ,x=1 ]", "[% ,x=1 ]"},
		{"[%\t\t,x=1\t]", "[%\t,x=1\t]"},
		{"[%q  ,x=1 ]", "[%q ,x=1 ]"},
		{"puts %s\t\t,x=1\t", "puts %s\t,x=1\t"},
		{"puts %q  ,x=1 ", "puts %q ,x=1 "},
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

// LPCV-V0-7605: a file that may hold a percent literal a blank delimits
// bounds no replaced hunk by its line's text, on either side and whatever the
// line says. Measured on Ruby 3.3.8: after `p 1, % a`, the line
// `    ,y=1 , x = 2` declares y and x, because the first blank of its
// indentation closes the literal, and `\t\t,y=1 , x = 2` declares only x,
// because the literal runs to the blank after `y=1`; and `p 1, % a ` closes
// the literal at its last blank, where `p 1, % a` runs it onto the next line.
func TestBlankDelimitedFileBoundsNoHunk(t *testing.T) {
	for _, c := range []struct {
		src  string
		want bool
	}{
		{"% =>x ", true}, {"%\t=>x", true}, {"%q x ", true}, {"%Q\tx", true}, {"%s x ", true}, {"x = %w[a]", false},
		{"a % b", true}, {"\"%d items\" % n", true}, {"x %= 2", false}, {"%(a b)", false}, {"100%", false}, {"", false},
	} {
		if got := holdsBlankDelimited(c.src); got != c.want {
			t.Errorf("holdsBlankDelimited(%q) = %v, want %v", c.src, got, c.want)
		}
	}
	reach := func(oldSrc, newSrc string, line int) string {
		why, _ := unbounded(readSide(oldSrc), readSide(newSrc), linecov.Hunk{Start: line, End: line}, linecov.Hunk{Start: line, End: line})
		return why
	}
	const matching = "a line of a file that may hold a blank-delimited percent literal"
	// method is a method whose fourth line is body after the third, call.
	method := func(call, body string) string {
		return "def m(flag)\n  if flag\n    " + call + "\n" + body + "\n  end\nend\n"
	}
	if why := reach(method("p 1, % a", "    ,y=1 , x = 2"), method("p 1, % a", "\t\t,y=1 , x = 2"), 4); why != matching {
		t.Errorf("an indentation edit after an open blank-delimited literal is bounded: %q", why)
	}
	// The blank that closes a literal sits after the line's last operator, or
	// on a line with none: removing it lets the literal swallow the next line.
	for _, call := range []string{"p 1, % a", "z = % a"} {
		if why := reach(method(call+" ", "\t\ty=1 ; x = 2"), method(call, "\t\ty=1 ; x = 2"), 3); why != matching {
			t.Errorf("removing the blank that closes the literal of %q is bounded: %q", call, why)
		}
	}
	// An edit that keeps every byte of the line through its last operator, and
	// one of a line that holds no literal itself, are unbounded too.
	if why := reach(method("z = % a ", "    w = 1"), method("z = % b ", "    w = 1"), 3); why != matching {
		t.Errorf("another value in a blank-delimited literal is bounded: %q", why)
	}
	if why := reach(method("z = % a ", "    w = 1"), method("z = % a ", "    w = 2"), 4); why != matching {
		t.Errorf("an edit beside a blank-delimited literal is bounded: %q", why)
	}
	// Without such a literal anywhere, indentation and blank runs keep the
	// signature, so the edit stays bounded by the line's hits.
	if why := reach(method("p 1, 2", "    x  = 2"), method("p 1, 2", "\t\tx = 2"), 4); why != "" {
		t.Errorf("a blank-only edit in a file without a blank-delimited literal is unbounded: %q", why)
	}
	// The literal on either side is enough.
	if why := reach(method("p 1, 2", "    x  = 2"), method("p 1, % a", "    x = 2"), 4); why != matching {
		t.Errorf("a blank-only edit beside a new blank-delimited literal is bounded: %q", why)
	}
	if why := reach(method("p 1, % a", "    x  = 2"), method("p 1, 2", "    x = 2"), 4); why != matching {
		t.Errorf("a blank-only edit beside a removed blank-delimited literal is bounded: %q", why)
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
		// Percent literals delimited by a blank: the run's length decides
		// whether y is assigned.
		"[%  ,y=1 ]", "[% ,y=1 ]", "[%\t\t,y=1\t]", "[%\t,y=1\t]", "[%q  ,y=1 ]", "[%q ,y=1 ]",
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
	// A file that may hold a percent literal a blank delimits bounds no hunk,
	// so only the lines a file without one can hold are grouped; the percent
	// values prove that such a line is left out, not compared.
	type group struct{ line, table string }
	groups := map[string]group{"": {"foo(1)", "flag"}}
	parsed, bounded, matching := 0, 0, 0
	for i, line := range lines {
		if tables[i] == "!" {
			continue
		}
		parsed++
		side := readSide(line)
		if side.blankDelimited {
			matching++
			if why, _ := unbounded(side, side, linecov.Hunk{Start: 1, End: 1}, linecov.Hunk{Start: 1, End: 1}); why == "" {
				t.Errorf("%q may hold a blank-delimited percent literal and is bounded", line)
			}
			continue
		}
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
	// The comparison is not vacuous: most combinations parse, some signatures
	// are shared, and some lines hold a blank-delimited literal.
	if parsed < len(lines)/2 || bounded < parsed/2 || len(groups) >= bounded || matching == 0 {
		t.Fatalf("%d lines, %d parse, %d bounded, %d signatures, %d with a blank-delimited literal", len(lines), parsed, bounded, len(groups), matching)
	}
	t.Logf("%d lines, %d parse, %d bounded, %d signatures, %d with a blank-delimited literal", len(lines), parsed, bounded, len(groups), matching)
}

// LPCV-V0-7605: measured against Ruby's own lexer. RubyLexer reads a source
// as rubydialect.LineStates does wherever the lexer is right, on every kind
// of literal and comment, and differs on the line where the lexer misreads
// Ruby: a percent literal delimited by a blank, by a newline or by `=`, two
// divisions on one line, a heredoc with a lower-case tag, a quote in a
// special global, an operator method's name beside its symbol and the text
// after __END__. A source Ruby does not parse has no reading.
func TestLiveRubyLexerAgainstTheLexer(t *testing.T) {
	requireRuby(t, "minitest")
	lex := RubyLexer(context.Background(), "ruby", nil)
	misreadLine := func(src string) string {
		return readSide(src).misread(lex, "lib/z.rb", []byte(src))
	}
	agreed := strings.Join([]string{
		"# a comment with a \" quote",
		"class Z",
		"  A = \"a #{1 + \"b\".size} c\" # after",
		"  B = 'it''s'",
		"  C = %w[a b] + %i[c d] + [%q(e (f) g), %Q{h #{1} i}]",
		"  D = [/re\\/x/i, %r{a/b}x, ?a, ?\\n, :sym, :\"quoted sym\", {\"k\": 1, 'l': 2}]",
		"  def m(v)",
		"    text = <<~TEXT + <<-'RAW'",
		"      body #{v} with def and end",
		"    TEXT",
		"      raw #{not} interpolated",
		"      RAW",
		"    v % 2 == 0 ? `echo #{v}` : (v / 2)",
		"    puts %(a) unless v =~ /a\"b/",
		"  end",
		"=begin",
		"def not_code",
		"=end",
		"  E = \"two",
		"  lines\" + 'x'",
		"  F = %w[",
		"    a b",
		"  ]",
		"end",
		"",
	}, "\n")
	if why := misreadLine(agreed); why != "" {
		t.Errorf("Ruby and the lexer differ on a source the lexer reads right: %s", why)
	}
	states, err := lex("lib/z.rb", []byte(agreed))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		line         int
		code         string
		inside, open bool
	}{
		{3, "  A =", false, false},
		{8, "    text =         +", false, true},
		{9, "", true, true},
		{10, "", true, true},
		{12, "", true, false},
		{13, "    v % 2 == 0 ? " + strings.Repeat(" ", len("`echo #{v}`")) + " : (v / 2)", false, false},
		{14, "    puts      unless v =~", false, false},
		{16, "", false, true},
		{18, "", true, false},
		{19, "  E =", false, true},
		{20, "         +", true, false},
	} {
		got := states[c.line-1]
		if strings.TrimRight(got.Code, " ") != c.code || got.Inside != c.inside || got.Open != c.open {
			t.Errorf("line %d: Ruby reads %q inside=%v open=%v, want %q %v %v", c.line, got.Code, got.Inside, got.Open, c.code, c.inside, c.open)
		}
	}
	for _, c := range []struct{ name, src, want string }{
		{"space-delimited percent literal", "def m\n  z = % a \nend\n", "line 2"},
		{"tab-delimited %q literal", "def m\n  z = %q\ta\t\nend\n", "line 2"},
		{"newline-delimited percent literal", "def m\n  z = %\n  a\nend\n", "line 2"},
		{"percent literal delimited by an equals sign", "def m\n  z = %=a=\nend\n", "line 2"},
		{"two divisions on one line", "def m(a, b)\n  a / b + b / a\nend\n", "line 2"},
		{"heredoc with a lower-case tag", "def m\n  z = <<eos\n  def\neos\nend\n", "line 2"},
		{"quote in a special global", "def m\n  z = $'\nend\n", "line 2"},
		{"operator method name beside its symbol", "class Z\n  def /(x); x.send(:/, 1); end\nend\n", "line 2"},
		{"text after __END__", "def m\nend\n__END__\ndef\n", "line 4"},
	} {
		if why := misreadLine(c.src); why != c.want {
			t.Errorf("%s: Ruby and the lexer differ at %q, want %q", c.name, why, c.want)
		}
	}
	if why := misreadLine("def m\n  (\nend\n"); !strings.HasPrefix(why, "Ruby does not lex lib/z.rb") {
		t.Errorf("a source that does not parse is read: %q", why)
	}
}
