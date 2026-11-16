// SPDX-License-Identifier: Apache-2.0
package rubylinecov

import (
	"errors"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/linecov"
)

// linesSource is lib/a.rb of the synthetic index: Ruby counts lines 1, 2, 3,
// 5, 7, 11 and 12; when, else and the end lines are uncounted.
const linesSource = `class A
  def x(v)
    case v
    when 1
      :one
    else
      :other
    end
  end

  def unused
    :never
  end
end
`

// linesIndex records two tests of the called method x (t1 takes the else
// branch, t2 the when branch) and the load bucket's class and def lines.
func linesIndex(t *testing.T) *linecov.Index {
	t.Helper()
	x, unused := span{2, 2, 9, 5}, span{11, 2, 13, 5}
	run := &Run{
		Runner:   "ruby 3.3.8 minitest 5.27.0 coverage=lines,methods",
		isolated: true,
		files:    map[string]fileShape{"lib/a.rb": {Lines: []int32{1, 2, 3, 5, 7, 11, 12}, Methods: []span{x, unused}}},
		Outcomes: []Outcome{{Package: "test/a_test.rb", Name: "A#t1", Status: "passed", Recorded: true}, {Package: "test/a_test.rb", Name: "A#t2", Status: "passed", Recorded: true}},
		records: []record{
			{pkg: "test/a_test.rb", name: "A#t1", lines: map[string][]int32{"lib/a.rb": {3, 7}}, methods: map[string][]span{"lib/a.rb": {x}}},
			{pkg: "test/a_test.rb", name: "A#t2", lines: map[string][]int32{"lib/a.rb": {3, 5}}, methods: map[string][]span{"lib/a.rb": {x}}},
			{pkg: Load, name: Load, lines: map[string][]int32{"lib/a.rb": {1, 2, 11}}, methods: map[string][]span{}},
		},
	}
	ix, err := Index(".", "c", run, map[string]string{"lib/a.rb": "old", "test/a_test.rb": "t"})
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

// linesEdit replaces old line `line` with the given lines (none deletes it)
// or, with insert, inserts them after it; it returns the diff and new source.
func linesEdit(line int, insert bool, with ...string) (linecov.FileDiff, string) {
	lines := strings.Split(linesSource, "\n")
	d := linecov.FileDiff{Path: "lib/a.rb", OldOID: "old", NewOID: "new"}
	var out []string
	if insert {
		out = append(append(append(out, lines[:line]...), with...), lines[line:]...)
		d.Hunks = []linecov.Hunk{{Start: line, End: line - 1}}
		d.NewHunks = []linecov.Hunk{{Start: line + 1, End: line + len(with)}}
	} else {
		out = append(append(append(out, lines[:line-1]...), with...), lines[line:]...)
		d.Hunks = []linecov.Hunk{{Start: line, End: line}}
		d.NewHunks = []linecov.Hunk{{Start: line, End: line + len(with) - 1}}
		if len(with) == 0 {
			d.NewHunks = []linecov.Hunk{{Start: line - 1, End: line - 2}}
		}
	}
	return d, strings.Join(out, "\n")
}

func linesSources(newSource string) Sources {
	return Sources{
		Old: func(string) ([]byte, error) { return []byte(linesSource), nil },
		New: func(string) ([]byte, error) { return []byte(newSource), nil },
		Check: func(_ string, src []byte) error {
			if strings.Contains(string(src), "BAD") {
				return errors.New("-:5: syntax error")
			}
			return nil
		},
		Lex: lexerAgrees,
	}
}

// LPCV-V0-7605 (decision 0601): a counted line selects the tests that hit
// it, a zero-hit line selects none, an uncounted line or an insertion selects
// the enclosing called method's tests, and the rest falls back.
func TestAnalyzeLines(t *testing.T) {
	ix := linesIndex(t)
	for _, c := range []struct {
		name   string
		line   int
		insert bool
		with   []string
		want   string
	}{
		{"when branch", 5, false, []string{"      :uno"}, "A#t2"},
		{"else branch", 7, false, []string{"      :otro"}, "A#t1"},
		{"case head", 3, false, []string{"    case v.to_i"}, "A#t1 A#t2"},
		{"zero-hit line of an uncalled method", 12, false, []string{"    :other"}, ""},
		// The new side declares a local, which changes how other lines parse
		// whether or not this line runs: its zero hits bound nothing.
		{"zero-hit line replaced by a new local", 12, false, []string{"    y = 1", "    :other"}, "fallback: edit of a line whose bindings change outside a called method body lib/a.rb:12"},
		{"zero-hit line replaced by two statements", 12, false, []string{"    :first", "    :other"}, ""},
		{"zero-hit line deleted", 12, false, nil, ""},
		{"uncounted when line", 4, false, []string{"    when 2"}, "A#t1 A#t2"},
		{"uncounted else line", 6, false, []string{"    else # other"}, "A#t1 A#t2"},
		{"insertion in a called method", 5, true, []string{"      puts 1"}, "A#t1 A#t2"},
		{"insertion in an uncalled method", 12, true, []string{"    puts 1"}, "fallback: insertion outside a called method body lib/a.rb:12"},
		{"def line", 2, false, []string{"  def x(v = 1)"}, "fallback: the change reaches load-time code"},
		{"blank line between methods", 10, false, []string{"  # note"}, "fallback: edit of an uncounted line outside a called method body lib/a.rb:10"},
		{"end of an uncalled method", 13, false, []string{"  end # unused"}, "fallback: edit of an uncounted line outside a called method body lib/a.rb:13"},
	} {
		d, newSource := linesEdit(c.line, c.insert, c.with...)
		if got := selected(ix, AnalyzeLines(ix, []linecov.FileDiff{d}, linesSources(newSource))); got != c.want {
			t.Errorf("%s: selects %q, want %q", c.name, got, c.want)
		}
	}
}

// LPCV-V0-7605 (decision 0601): a block-structure change, a lexical boundary
// that moves past the hunk, a new side that does not parse or cannot be
// checked, missing blobs and every earlier fallback keep the static plan.
func TestAnalyzeLinesStructureAndSyntax(t *testing.T) {
	ix := linesIndex(t)
	for _, c := range []struct {
		name string
		line int
		with []string
		want string
	}{
		{"closes the method early", 12, []string{"    end; def other; :never"}, "edit changes Ruby block structure"},
		{"adds a modifier", 5, []string{"      :one if v"}, "edit changes Ruby block structure"},
		{"continues onto the next line", 12, []string{"    :never +"}, "edit changes Ruby block structure"},
		{"opens a heredoc past the hunk", 12, []string{"    <<~TEXT"}, "edit changes Ruby block structure"},
		{"does not parse", 12, []string{"    BAD"}, "does not parse"},
	} {
		d, newSource := linesEdit(c.line, false, c.with...)
		if got := selected(ix, AnalyzeLines(ix, []linecov.FileDiff{d}, linesSources(newSource))); !strings.Contains(got, "fallback: ") || !strings.Contains(got, c.want) {
			t.Errorf("%s: selects %q, want a fallback naming %q", c.name, got, c.want)
		}
	}
	d, newSource := linesEdit(12, false, "    :other")
	src := linesSources(newSource)
	noCheck := src
	noCheck.Check = nil
	noOld := src
	noOld.Old = nil
	noLexer := src
	noLexer.Lex = nil
	for name, c := range map[string]struct {
		diffs []linecov.FileDiff
		src   Sources
		want  string
	}{
		"no syntax check":   {[]linecov.FileDiff{d}, noCheck, "fallback: no Ruby syntax check for lib/a.rb"},
		"no blobs":          {[]linecov.FileDiff{d}, noOld, "fallback: source blobs unavailable for lib/a.rb"},
		"no Ruby lexer":     {[]linecov.FileDiff{d}, noLexer, "fallback: no Ruby lexer for lib/a.rb"},
		"stale index blob":  {[]linecov.FileDiff{{Path: "lib/a.rb", OldOID: "other", NewOID: "new", Hunks: d.Hunks, NewHunks: d.NewHunks}}, src, "fallback: index records lib/a.rb at another blob"},
		"absent file":       {[]linecov.FileDiff{{Path: "lib/b.rb", OldOID: "b", NewOID: "c", Hunks: d.Hunks, NewHunks: d.NewHunks}}, src, "fallback: uninstrumented file lib/b.rb"},
		"non-Ruby input":    {[]linecov.FileDiff{{Path: "lib/data.yml", OldOID: "a", NewOID: "b"}}, src, "fallback: non-Ruby input lib/data.yml"},
		"added file":        {[]linecov.FileDiff{{Path: "lib/c.rb", NewOID: "b"}}, src, "fallback: added or deleted file lib/c.rb"},
		"test helper":       {[]linecov.FileDiff{{Path: "test/helper.rb", OldOID: "a", NewOID: "b"}}, src, "fallback: test helper test/helper.rb"},
		"changed test file": {[]linecov.FileDiff{{Path: "test/a_test.rb", OldOID: "t", NewOID: "u"}}, src, ""},
	} {
		e := AnalyzeLines(ix, c.diffs, c.src)
		if got := selected(ix, e); got != c.want {
			t.Errorf("%s: selects %q, want %q", name, got, c.want)
		}
		if name == "changed test file" && !e.Whole["test/a_test.rb"] {
			t.Errorf("a changed test file does not run whole")
		}
	}
	// The plan keeps the whole file for a fallback and excludes it for a
	// zero-hit edit.
	p, _ := Plan(ix, []string{"test/a_test.rb"}, linecov.Effect{}, AnalyzeLines(ix, []linecov.FileDiff{d}, src))
	if p["test/a_test.rb"].Mode != linecov.Exclude {
		t.Fatalf("zero-hit edit plan %+v", p)
	}
	bad, badSource := linesEdit(12, false, "    BAD")
	p, _ = Plan(ix, []string{"test/a_test.rb"}, linecov.Effect{}, AnalyzeLines(ix, []linecov.FileDiff{bad}, linesSources(badSource)))
	if p["test/a_test.rb"].Mode != linecov.Whole {
		t.Fatalf("fallback plan %+v", p)
	}
}
