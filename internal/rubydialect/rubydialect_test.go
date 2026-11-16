// SPDX-License-Identifier: Apache-2.0

package rubydialect

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/langplugin/decl"
	"github.com/Beamfall/corvint/internal/langplugin/dialect"
)

// TestRegistersDeclaredDialects_LGPV0025 checks the plugin registers a hook
// for every dialect the Ruby declaration names.
func TestRegistersDeclaredDialects_LGPV0025(t *testing.T) {
	for _, d := range decl.Of(plugin).Dialects {
		var ok bool
		switch d.Kind {
		case decl.ImpactDialect:
			_, ok = dialect.LookupImpact(d.Name)
		case decl.RegionDialect:
			_, ok = dialect.LookupRegion(d.Name)
		default:
			continue
		}
		if !ok {
			t.Errorf("%s dialect %q not registered", d.Kind, d.Name)
		}
	}
}

func TestStaticLoad_LGPV0025(t *testing.T) {
	for tail, want := range map[string]bool{
		` "foo"`:          true,
		`("foo/bar")`:     true,
		` 'foo' # note`:   true,
		` "foo#{name}"`:   false,
		` 'foo#{name}'`:   true,
		` name`:           false,
		` "foo" + suffix`: false,
		`("foo"`:          false,
	} {
		if got := StaticLoad(tail); got != want {
			t.Errorf("StaticLoad(%q) = %v, want %v", tail, got, want)
		}
	}
}

func TestSymbolsSkipsLiteralsAndReportsCaps_LGPV0025(t *testing.T) {
	text := "class Foo\n  SQL = <<~Q\n    def not_a_method\n  Q\n  def bar; end\nend\n"
	got := Symbols(text, 100, 100)
	if got.Stop != "" || got.UnterminatedComment || got.UnterminatedString {
		t.Fatalf("unexpected incompleteness: %+v", got)
	}
	names := map[string]bool{}
	for _, d := range got.Declarations {
		names[d.Name] = true
	}
	if !names["Foo"] || !names["bar"] || names["not_a_method"] {
		t.Fatalf("declarations = %+v", got.Declarations)
	}
	if capped := Symbols(text, 2, 100); capped.Stop != dialect.LineCap {
		t.Fatalf("line cap stop = %q", capped.Stop)
	}
	if capped := Symbols(text, 100, 1); capped.Stop != dialect.SymbolCap {
		t.Fatalf("symbol cap stop = %q", capped.Stop)
	}
	if open := Symbols("x = \"open\n", 100, 100); !open.UnterminatedString {
		t.Fatalf("unterminated string not reported: %+v", open)
	}
}

// LineStates reports, beside each line's code, whether the line begins or
// ends inside a literal, a heredoc body or a block comment (LPCV-V0-7605).
func TestLineStates(t *testing.T) {
	source := strings.Join([]string{
		`a = "one`,     // 1 opens a string
		`two" + b`,     // 2 closes it
		`c = <<~T + d`, // 3 owes a heredoc body
		`  body #{e}`,  // 4 heredoc body
		`T`,            // 5 terminator
		`=begin`,       // 6 block comment
		`f`,            // 7 inside it
		`=end`,         // 8 ends it
		`g = %w[`,      // 9 opens a percent literal
		`  h ]`,        // 10 closes it
		`i # note`,     // 11 plain code
	}, "\n")
	want := []struct {
		code         string
		inside, open bool
	}{
		{"a =", false, true}, {"+ b", true, false}, {"c =  + d", false, true}, {"", true, true}, {"", true, false},
		{"", false, true}, {"", true, true}, {"", true, false}, {"g =", false, true}, {"", true, false}, {"i", false, false},
	}
	states := LineStates(source)
	if len(states) != len(want) {
		t.Fatalf("%d states for %d lines", len(states), len(want))
	}
	for i, w := range want {
		got := states[i]
		if strings.Join(strings.Fields(got.Code), " ") != strings.Join(strings.Fields(w.code), " ") || got.Inside != w.inside || got.Open != w.open {
			t.Errorf("line %d: code %q inside %v open %v, want %q %v %v", i+1, got.Code, got.Inside, got.Open, w.code, w.inside, w.open)
		}
	} // A blanked literal at either end of a line's code is a value.
	for _, c := range []struct {
		line         string
		starts, ends bool
	}{
		{`raise ArgumentError, "no blocks"`, false, true},
		{`raise ArgumentError, "no blocks" # note`, false, true},
		{`"abc".upcase`, true, false},
		{`  'a' + 'b'  `, true, true},
		{`s =~ /re/`, false, true},
		{`foo(1) # "not code"`, false, false},
		{`x = a +`, false, false},
		{``, false, false},
	} {
		got := LineStates(c.line)[0]
		if got.StartsLiteral != c.starts || got.EndsLiteral != c.ends {
			t.Errorf("%q: starts %v ends %v, want %v %v", c.line, got.StartsLiteral, got.EndsLiteral, c.starts, c.ends)
		}
	}
}
