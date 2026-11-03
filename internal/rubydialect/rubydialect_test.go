// SPDX-License-Identifier: Apache-2.0

package rubydialect

import (
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
