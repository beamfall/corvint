package ruby

import (
	"context"
	"strings"
	"testing"
)

// TestHeredocAndLexicalNestingBoundConstantEdges_RUBYAFFECTEDV0043: a
// constant named only in heredoc text adds no edge, `Bar` inside `module Foo`
// still reaches Foo::Bar, and a `Bar` after Foo's `end` no longer does.
func TestHeredocAndLexicalNestingBoundConstantEdges_RUBYAFFECTEDV0043(t *testing.T) {
	root := writeIssue1130(t, true)
	write(t, root, "app/models/widget.rb", "class Widget\nend\n")
	write(t, root, "app/models/bar.rb", "class Bar\nend\n")
	write(t, root, "app/models/foo/bar.rb", "module Foo\n  class Bar\n  end\nend\n")
	write(t, root, "app/services/foo/report.rb", strings.Join([]string{
		"module Foo",
		"  class Report",
		"    def run",
		"      sql = <<~SQL",
		"        SELECT * FROM Widget -- don't",
		"      SQL",
		"      return if sql.empty?",
		"      Bar.new(sql)",
		"    end",
		"  end",
		"end",
		"",
	}, "\n"))
	write(t, root, "app/services/after.rb", "module Foo\n  def self.ready? = true\nend\n\nclass After\n  def run\n    Bar.new\n  end\nend\n")
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	report := unitByID(t, result, "ruby:source:app/services/foo/report")
	if !equal(report.AdvisoryImports, []string{"ruby:source:app/models/bar", "ruby:source:app/models/foo/bar"}) {
		t.Fatalf("report advisory edges=%v", report.AdvisoryImports)
	}
	after := unitByID(t, result, "ruby:source:app/services/after")
	if !equal(after.AdvisoryImports, []string{"ruby:source:app/models/bar"}) {
		t.Fatalf("after advisory edges=%v", after.AdvisoryImports)
	}
}

// TestHeredocMaskKeepsEvaluatedAndUnterminatedBodies_RUBYAFFECTEDV0043 keeps
// text the mask cannot prove is data.
func TestHeredocMaskKeepsEvaluatedAndUnterminatedBodies_RUBYAFFECTEDV0043(t *testing.T) {
	cases := map[string]bool{
		"x = <<~SQL\n  Widget\nSQL\nBar\n":                       false,
		"x = <<-'SQL'\n  Widget\n  SQL\nBar\n":                   false,
		"f(<<~A, <<~B)\n  Widget\nA\n  Widget\nB\nBar\n":         false,
		"class_eval <<~RUBY\n  def go = Widget.new\nRUBY\nBar\n": true,
		"x = <<~SQL\n  Widget\nBar\n":                            true,
		"x = \"<<~SQL\"\nWidget\nSQL\n":                          true,
		"items << Widget\n":                                      true,
	}
	for source, kept := range cases {
		masked := maskHeredocBodies(source)
		if strings.Count(masked, "\n") != strings.Count(source, "\n") {
			t.Fatalf("%q: line count changed: %q", source, masked)
		}
		if strings.Contains(masked, "Widget") != kept {
			t.Fatalf("%q: masked=%q, want Widget kept=%v", source, masked, kept)
		}
	}
}

// TestLexicalScopesBalanceModifiersAndFallBack_RUBYAFFECTEDV0043 balances
// modifier forms, endless and setter defs, loop `do` and keyword-named
// methods and labels, and falls back to every opened namespace when it
// cannot balance the blocks.
func TestLexicalScopesBalanceModifiersAndFallBack_RUBYAFFECTEDV0043(t *testing.T) {
	source := strings.Join([]string{
		"module Foo",
		"  class Bar < Base",
		"    def name=(value)",
		"      @name = value unless value.nil?",
		"      return if value.empty?",
		"      self.class",
		"      opts = { if: 1, end: 2 }",
		"      x = if value then 1 else 2 end",
		"      raise \"no\" if x",
		"      while x do x -= 1 end",
		"      save! if valid?",
		"      [1].each do |y| y end",
		"    end",
		"    def ready? = true",
		"  end",
		"end",
		"Top",
	}, "\n")
	masked := maskQuotedLiteralsAndInlineComments(source)
	scopes := scanLexicalScopes(source, masked)
	if scopes.fallback != nil {
		t.Fatalf("balanced source fell back: %v", scopes.fallback)
	}
	if got := scopes.at(strings.Index(source, "Top")); len(got) != 0 {
		t.Fatalf("top-level namespaces=%v", got)
	}
	if got := scopes.at(strings.Index(source, "@name")); !equal(got, []string{"Bar", "Foo", "Foo::Bar"}) {
		t.Fatalf("method namespaces=%v", got)
	}
	for _, unbalanced := range []string{"module Foo\nend\nend\nTop\n", "module Foo\n  x = 1 if\nTop\n", "module Foo\nend\n=begin\nend\n=end\nTop\n"} {
		masked := maskQuotedLiteralsAndInlineComments(unbalanced)
		scopes := scanLexicalScopes(unbalanced, masked)
		if !equal(scopes.at(strings.Index(unbalanced, "Top")), []string{"Foo"}) {
			t.Fatalf("%q: want whole-file fallback, got frames=%v fallback=%v", unbalanced, scopes.frames, scopes.fallback)
		}
	}
}

// TestHeredocMaskRejectsShiftsAndKeepsInterpolation_RUBYAFFECTEDV0043: a
// shift such as `items<<Widget` opens no heredoc, interpolated code in a
// heredoc stays, a single-quoted heredoc interpolates nothing, and an
// interpolation spanning lines keeps the rest of the body.
func TestHeredocMaskRejectsShiftsAndKeepsInterpolation_RUBYAFFECTEDV0043(t *testing.T) {
	for source, want := range map[string]map[string]bool{
		"items<<Widget\nBar.new\nWidget\n":                  {"Bar": true},
		"items <<Widget\nBar.new\nWidget\n":                 {"Bar": true},
		"list.push(x)<<Item\nBar.new\nItem\n":               {"Bar": true},
		"x = <<~SQL\n  #{Widget.table} don't\nSQL\nBar\n":   {"Widget": true, "don't": false, "Bar": true},
		"x = <<~'SQL'\n  #{Widget.table}\nSQL\nBar\n":       {"Widget": false, "Bar": true},
		"x = <<~SQL\n  #{Widget.call(\n  Baz)}\nSQL\nBar\n": {"Widget": true, "Baz": true, "Bar": true},
		"x = <<~SQL\n  Widget\nSQL\nBar\n":                  {"Widget": false, "Bar": true},
	} {
		masked := maskHeredocBodies(source)
		if strings.Count(masked, "\n") != strings.Count(source, "\n") {
			t.Fatalf("%q: line count changed: %q", source, masked)
		}
		for word, kept := range want {
			if strings.Contains(masked, word) != kept {
				t.Fatalf("%q: masked=%q, want %q kept=%v", source, masked, word, kept)
			}
		}
	}
}

// TestRegexKeywordFallsBack_RUBYAFFECTEDV0043: a regex literal holding a block
// keyword could balance the stack wrongly, so the scan falls back to every
// namespace the file opens.
func TestRegexKeywordFallsBack_RUBYAFFECTEDV0043(t *testing.T) {
	source := "X = /begin/\nmodule Foo\n  Y = /end/\n  Bar\nend\n"
	masked := maskQuotedLiteralsAndInlineComments(source)
	scopes := scanLexicalScopes(source, masked)
	if !equal(scopes.at(strings.Index(source, "Bar")), []string{"Foo"}) {
		t.Fatalf("want whole-file fallback, got frames=%v fallback=%v", scopes.frames, scopes.fallback)
	}
}

// TestRegexLiteralMaskHidesQuotesNotDivision_RUBYAFFECTEDV0044: a quote inside
// a /.../ or %r regex literal in an operand position opens no string, so the
// constants after it stay visible, while division masks nothing.
func TestRegexLiteralMaskHidesQuotesNotDivision_RUBYAFFECTEDV0044(t *testing.T) {
	for source, want := range map[string]map[string]bool{
		"x =~ /'/\nFoo::Bar.call\n":                  {"Foo::Bar": true},
		"if x =~ /[/\"]/ then Baz end\nQux\n":        {"Baz": true, "Qux": true},
		"case x\nwhen /\"/ then Widget\nend\n":       {"Widget": true},
		"y = x.match(%r{'}) && Gadget\n":             {"Gadget": true},
		"y = x.match(%r(\")) && Gadget\n":            {"Gadget": true},
		"a = b / c; Widget\nd = e/f / g + Gadget\n":  {"Widget": true, "Gadget": true},
		"total = sum(x) / 2 + Widget.count / Rate\n": {"Widget": true, "Rate": true},
		"x = /Hidden/\n":                             {"Hidden": false},
		"x = a / Kept\n":                             {"Kept": true},
		"x.split /,/\nFoo\n":                         {"Foo": true},
	} {
		masked := maskQuotedLiteralsAndInlineComments(source)
		if len(masked) != len(source) {
			t.Fatalf("%q: length changed: %q", source, masked)
		}
		for word, kept := range want {
			if strings.Contains(masked, word) != kept {
				t.Fatalf("%q: masked=%q, want %q kept=%v", source, masked, word, kept)
			}
		}
	}
}
