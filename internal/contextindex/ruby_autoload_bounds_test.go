package contextindex

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// rubyBoundsImpact builds files and returns the loaded autoload index and the
// path impact receipt for changed.
func rubyBoundsImpact(t *testing.T, files map[string]string, changed string) (*rubyAutoload, map[string]any) {
	t.Helper()
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, files))
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := Impact(index, []string{changed}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	return loadRubyAutoload(index), receipt
}

// rubyCraftedNamespaces is an unbalanced file (a stray leading `end`) that
// opens count namespaces, each naming X, and the mapped Ns<i>::X each
// fallback candidate resolves to.
func rubyCraftedNamespaces(count int) map[string]string {
	files := map[string]string{}
	var crafted strings.Builder
	crafted.WriteString("end\n")
	for i := range count {
		files[fmt.Sprintf("app/models/ns%d/x.rb", i)] = fmt.Sprintf("module Ns%d\n  class X\n  end\nend\n", i)
		fmt.Fprintf(&crafted, "module Ns%d\n  X.new\nend\n", i)
	}
	files["app/models/crafted.rb"] = crafted.String()
	return files
}

// TestRubyAutoloadResolutionBound: NGI-V0-040. An unbalanced file opening
// more namespaces than rubyResolutionBound.limit resolves each reference against at
// most that many, and a plan counts in the ruby-reference-index-bounded
// unknown every cut reference that could name its constant (NGI-V0-064).
func TestRubyAutoloadResolutionBound(t *testing.T) {
	t.Parallel()
	const count = 80
	autoload, receipt := rubyBoundsImpact(t, rubyCraftedNamespaces(count), "app/models/ns0/x.rb")
	source := autoload.sources["app/models/crafted.rb"]
	if source == nil || source.balanced || len(source.namespaces) != count {
		t.Fatalf("crafted source must be unbalanced with %d namespaces: %#v", count, source)
	}
	for _, reference := range source.references {
		resolved, complete := autoload.resolutions(source, reference)
		if complete || len(resolved) > rubyResolutionBound.limit+2 {
			t.Fatalf("reference %q: %d candidates, complete=%v", reference.name, len(resolved), complete)
		}
	}
	if autoload.bounded != len(source.references) {
		t.Fatalf("bounded = %d, want every crafted reference (%d)", autoload.bounded, len(source.references))
	}
	// NGI-V0-064: Ns0::X lies within the namespaces every cut reference
	// tried, so no cut could have hidden a referrer of it.
	if got := rubyUnknownCount(receipt, "ruby-reference-index-bounded"); got != 0 {
		t.Fatalf("ruby-reference-index-bounded = %d for a constant every cut resolved, want 0: %#v", got, receipt["unknowns"])
	}
	// The first rubyResolutionBound.limit namespaces still resolve: Ns0::X is
	// reached from the crafted file.
	if rubyResultRows(receipt)["app/models/crafted.rb"] == nil {
		t.Fatalf("crafted.rb must still reference Ns0::X within the bound: %#v", receipt["results"])
	}
	// Ns70::X lies past the bound: every crafted reference could name it, so
	// every one is counted on its plan.
	autoload, receipt = rubyBoundsImpact(t, rubyCraftedNamespaces(count), "app/models/ns70/x.rb")
	named := 0
	for _, reference := range source.references {
		if reference.name == "X" {
			named++
		}
	}
	if got := rubyUnknownCount(receipt, "ruby-reference-index-bounded"); got != named || named != count {
		t.Fatalf("ruby-reference-index-bounded = %d, want every crafted X reference (%d of %d): %#v", got, named, count, receipt["unknowns"])
	}
}

// TestRubyAutoloadSiteBound: NGI-V0-040, NGI-V0-064. The index retains at
// most rubySiteBound.limit sites; every cut reference that could name a
// constant the plan traverses is counted, and below the bound nothing is.
func TestRubyAutoloadSiteBound(t *testing.T) {
	files := map[string]string{
		"app/models/a.rb": "class A\nend\n",
		"app/models/b.rb": "class B\n  def x = [A, A, A]\nend\n",
		"app/models/c.rb": "class C\n  def x = [A, B]\nend\n",
	}
	autoload, receipt := rubyBoundsImpact(t, files, "app/models/a.rb")
	if autoload.bounded != 0 || rubyUnknownCount(receipt, "ruby-reference-index-bounded") != 0 {
		t.Fatalf("default bound must not cut a small repository: %d %#v", autoload.bounded, receipt["unknowns"])
	}
	unbounded := autoload.retained
	saved := rubySiteBound.limit
	rubySiteBound.limit = 4
	defer func() { rubySiteBound.limit = saved }()
	autoload, receipt = rubyBoundsImpact(t, files, "app/models/a.rb")
	if unbounded <= rubySiteBound.limit || autoload.retained != rubySiteBound.limit {
		t.Fatalf("retained = %d of %d, want the bound %d", autoload.retained, unbounded, rubySiteBound.limit)
	}
	total := 0
	for _, sites := range autoload.sites {
		total += len(sites)
	}
	if total != rubySiteBound.limit || autoload.bounded == 0 {
		t.Fatalf("sites = %d, bounded = %d", total, autoload.bounded)
	}
	// A cut site inside the changed file hides no referrer beyond its own path
	// row, and a cut naming only a constant the plan never traverses hides
	// none of its referrers, so at most the cuts in other files are counted
	// (NGI-V0-064).
	outside := 0
	for _, cut := range autoload.cuts {
		if cut.path != "app/models/a.rb" {
			outside++
		}
	}
	if got := rubyUnknownCount(receipt, "ruby-reference-index-bounded"); got == 0 || got > outside {
		t.Fatalf("ruby-reference-index-bounded = %d, want between 1 and the %d cuts outside a.rb: %+v", got, outside, autoload.cuts)
	}
	// NGI-V0-064: nothing references C, so no cut reference could name it and
	// its plan carries no bounded unknown even though the index was cut.
	autoload, receipt = rubyBoundsImpact(t, files, "app/models/c.rb")
	if autoload.bounded == 0 {
		t.Fatal("the site bound must still cut the index")
	}
	if got := rubyUnknownCount(receipt, "ruby-reference-index-bounded"); got != 0 {
		t.Fatalf("ruby-reference-index-bounded = %d for an unreferenced constant, want 0: %#v", got, receipt["unknowns"])
	}
}

// TestRubyAutoloadSecondHopConstantOnce: NGI-V0-034. Paths mapping to one
// constant (foo12.rb, foo_12.rb and foo1_2.rb all name Foo12) are one
// second-hop traversal, and the referrer is reported once.
func TestRubyAutoloadSecondHopConstantOnce(t *testing.T) {
	files := map[string]string{
		"app/models/a.rb":      "class A\nend\n",
		"app/models/foo12.rb":  "class Foo12\n  def x = A.new\nend\n",
		"app/models/foo_12.rb": "class Foo12\n  def y = A.new\nend\n",
		"app/models/foo1_2.rb": "class Foo12\n  def z = A.new\nend\n",
		"app/models/z.rb":      "class Z\n  def x = Foo12.new\nend\n",
	}
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, files))
	if err != nil {
		t.Fatal(err)
	}
	autoload := loadRubyAutoload(index)
	for _, name := range []string{"app/models/foo12.rb", "app/models/foo_12.rb", "app/models/foo1_2.rb"} {
		if constant, ok := autoload.mapper.Constant(name); !ok || constant != "Foo12" {
			t.Fatalf("%s maps to %q, %v", name, constant, ok)
		}
	}
	ranked, _ := autoload.impact(index, "app/models/a.rb")
	if autoload.lookups != 2 {
		t.Fatalf("referrer lookups = %d, want 2 (A, then Foo12 once)", autoload.lookups)
	}
	seen := map[string]int{}
	for _, item := range ranked {
		seen[item.key]++
	}
	if seen["app/models/z.rb"] != 1 {
		t.Fatalf("z.rb reported %d times: %#v", seen["app/models/z.rb"], seen)
	}
}
