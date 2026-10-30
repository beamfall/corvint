package doccorpus

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

var filterSources = map[string]string{
	"src/app/cart.py":      "def total():\n    return 1\n",
	"src/app/test_cart.py": "def test_total():\n    assert True\n",
	"src/lib/util.py":      "def helper():\n    return 2\n",
	"src/readme.md":        "# Shop\n\nThe cart.\n",
}

func filteredCorpus(t *testing.T, f *DerivationFilter) (string, Manifest) {
	t.Helper()
	// The records provider claims the sources, so their declarations are derived (DCS-V1-482).
	root, m := ownedSourceCorpus(t, filterSources, []string{"src/app/cart.py", "src/app/test_cart.py", "src/lib/util.py"}, false)
	for i := range m.Providers {
		if m.Providers[i].Kind == "native" {
			m.Providers[i].Derive = f
		}
	}
	return root, m
}

func derivedPaths(a *Artifact) []string {
	var paths []string
	for _, s := range subjectsWithPrefix(a, "native:symbol:") {
		paths = append(paths, s.Evidence.Anchors[0].Path)
	}
	slices.Sort(paths)
	return paths
}

// DCS-V1-573: a derivation filter keeps only the declarations of the selected source inputs
// and names the inputs it excluded in one gap; without it, the manifest and the derivation are
// what they were.
func TestDCSV1573DerivationFilterKeepsTheSelectedDeclarations(t *testing.T) {
	t.Parallel()
	t.Run("unfiltered", func(t *testing.T) {
		t.Parallel()
		root, m := filteredCorpus(t, nil)
		data, err := Encode(m)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), `"derive"`) {
			t.Fatalf("an unfiltered manifest should encode no derive field")
		}
		parsed, err := ParseManifest(data)
		if err != nil || parsed.Providers[0].Derive != nil {
			t.Fatalf("an unfiltered manifest should parse back unfiltered: %v", err)
		}
		a := buildOwned(t, root, m)
		if got := derivedPaths(a); !slices.Equal(got, []string{"src/app/cart.py", "src/app/test_cart.py", "src/lib/util.py"}) {
			t.Fatalf("every declaration should be derived: %v", got)
		}
		for _, g := range a.Gaps {
			if strings.HasSuffix(g.Subject, ":derivation-filter") {
				t.Fatalf("no filter, no filter gap: %v", g)
			}
		}
	})
	t.Run("filtered", func(t *testing.T) {
		t.Parallel()
		root, m := filteredCorpus(t, &DerivationFilter{Paths: []string{"src/app/"}, Kinds: []string{"symbol"}})
		data, err := Encode(m)
		if err != nil {
			t.Fatal(err)
		}
		if parsed, err := ParseManifest(data); err != nil || !slices.Equal(parsed.Providers[0].Derive.Paths, []string{"src/app/"}) {
			t.Fatalf("a filtered manifest should round-trip: %v", err)
		}
		a := buildOwned(t, root, m)
		if got := derivedPaths(a); !slices.Equal(got, []string{"src/app/cart.py"}) {
			t.Fatalf("only the selected declarations should be derived: %v", got)
		}
		var gaps []Gap
		for _, g := range a.Gaps {
			if strings.HasSuffix(g.Subject, ":derivation-filter") {
				gaps = append(gaps, g)
			}
		}
		if len(gaps) != 1 || gaps[0].Subject != "native:derivation-filter" || gaps[0].Kind != "not-collected" {
			t.Fatalf("the exclusions should be one not-collected gap: %v", gaps)
		}
		for _, want := range []string{"paths [src/app/]; kinds [symbol]", "excluded 2 declarations of 2 source inputs", "src/app/test_cart.py", "src/lib/util.py"} {
			if !strings.Contains(gaps[0].Reason, want) {
				t.Fatalf("the gap should name %q: %s", want, gaps[0].Reason)
			}
		}
	})
}

// DCS-V1-573: a filter that selects nothing, names an unknown kind, holds an unsorted or
// non-canonical entry, or sits on a records provider is refused before any input is read.
func TestDCSV1573DerivationFilterRefusesMalformedFilters(t *testing.T) {
	t.Parallel()
	for name, f := range map[string]*DerivationFilter{
		"vacuous":         {},
		"unknown kind":    {Kinds: []string{"class"}},
		"unsorted kinds":  {Kinds: []string{"test", "symbol"}},
		"duplicate path":  {Paths: []string{"src/", "src/"}},
		"parent path":     {Paths: []string{"../src/"}},
		"absolute path":   {Paths: []string{"/src/"}},
		"undeclared repo": {Paths: []string{"typo:src/"}},
	} {
		m := Manifest{Schema: ManifestSchema, Providers: []Provider{{ID: "native", Kind: "native", Derive: f}}}
		if err := validDerivationFilter(m, m.Providers[0]); err == nil {
			t.Fatalf("%s: the filter should be refused", name)
		}
	}
	root, m := filteredCorpus(t, nil)
	for i := range m.Providers {
		if m.Providers[i].Kind == "records" {
			m.Providers[i].Derive = &DerivationFilter{Kinds: []string{"symbol"}}
		}
	}
	var e *Error
	if _, err := Build(context.Background(), root, m); !errors.As(err, &e) || !strings.Contains(e.Message, "records provider") {
		t.Fatalf("a filter on a records provider should be refused: %v", err)
	}
}
