package contextindex

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// rubySpecMarkerFiles is a Go and a JS/TS source sharing a feature marker
// with an RSpec spec and a Ruby file below spec/, which only the Ruby
// reading's test-path rule classifies as tests (NGI-V0-038).
func rubySpecMarkerFiles() map[string]string {
	return map[string]string{
		"go.mod":                      "module example.test/specmarker\n\ngo 1.27\n",
		"internal/x/x.go":             "package x\n\n// feature:shared-demo\nfunc X() int { return 1 }\n",
		"web/app.js":                  "// feature:shared-demo\nexport function app() { return 1; }\n",
		"spec/example_spec.rb":        "# feature:shared-demo\nRSpec.describe 'example' do\nend\n",
		"spec/support/demo_helper.rb": "# feature:shared-demo\nmodule DemoHelper\nend\n",
		"testing/features.yaml":       "features:\n  - id: shared-demo\n    area: demo\n    summary: Shared demo.\n    status: shipped\n",
		"testing/scenarios.yaml":      "scenarios:\n  - id: shared-run\n    area: demo\n    summary: Shared run.\n    features: [shared-demo]\n    status: shipped\n",
	}
}

// rubySpecMarkerRequests are requests naming no Ruby path: default path
// impact of the Go source and syntax-profile path impact of the JS source.
func rubySpecMarkerRequests() map[string]func(*Index) (map[string]any, error) {
	return map[string]func(*Index) (map[string]any, error){
		"go-impact": func(index *Index) (map[string]any, error) {
			return Impact(index, []string{"internal/x/x.go"}, maxLimit)
		},
		"js-syntax-impact": func(index *Index) (map[string]any, error) {
			return ImpactSyntax(index, []string{"web/app.js"}, maxLimit)
		},
	}
}

// rubySpecMarkerReceipt is the request's results, coverage and unknowns,
// the members a test-path classification can move.
func rubySpecMarkerReceipt(t *testing.T, request func(*Index) (map[string]any, error)) map[string]any {
	t.Helper()
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, rubySpecMarkerFiles()))
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := request(index)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]any{"results": receipt["results"], "coverage": receipt["coverage"]}
	if unknowns, ok := receipt["unknowns"]; ok {
		out["unknowns"] = unknowns
	}
	return out
}

// TestRubySpecPathNonRubyRequestBaseBytes: NGI-V0-033, NGI-V0-038. Receipts
// for requests naming no Ruby path are byte-identical to the base commit
// a7ad984c9 (before the Ruby test-path rule), even when an RSpec path shares
// their feature marker. The golden holds the base commit's canonical bytes.
func TestRubySpecPathNonRubyRequestBaseBytes(t *testing.T) {
	t.Parallel()
	out := map[string]any{}
	for name, request := range rubySpecMarkerRequests() {
		out[name] = rubySpecMarkerReceipt(t, request)
	}
	encoded, err := CanonicalJSON(out)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "ruby-spec-marker-base", "receipts.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(append(encoded, '\n'), want) {
		t.Fatalf("non-Ruby receipts moved from the base bytes:\n got %s\nwant %s", encoded, want)
	}
}

// TestNGIV0063RubyRequestClassifiesSpecMarkersAsTests: NGI-V0-063. In a
// request naming a Ruby path, an RSpec path is a test in every row, not only
// in the autoload rows: the feature row the JS source's marker reaches cites
// the spec files' markers with test-marker authority.
func TestNGIV0063RubyRequestClassifiesSpecMarkersAsTests(t *testing.T) {
	t.Parallel()
	files := rubySpecMarkerFiles()
	files["app/models/widget.rb"] = "class Widget\nend\n"
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, files))
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := ImpactSyntax(index, []string{"web/app.js", "app/models/widget.rb"}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	authorities := map[string]string{}
	results := mapsFromAny(receipt["results"])
	for _, row := range results {
		for _, value := range mapsFromAny(row["evidence"]) {
			if key := stringValue(value["path"]); key == "spec/example_spec.rb" || key == "spec/support/demo_helper.rb" {
				authorities[key] = stringValue(value["authority"])
			}
		}
	}
	for _, key := range []string{"spec/example_spec.rb", "spec/support/demo_helper.rb"} {
		if authorities[key] != "test-marker" {
			t.Fatalf("%s cited with authority %q, want test-marker: %#v", key, authorities[key], results)
		}
	}
}

// TestNGIV0063RubyRangeClassifiesSpecMarkersAsTests: NGI-V0-063. A committed
// range admitting a Ruby path cites a changed RSpec hunk's marker with
// test-marker authority.
func TestNGIV0063RubyRangeClassifiesSpecMarkersAsTests(t *testing.T) {
	t.Parallel()
	files := rubySpecMarkerFiles()
	root := impactRepositoryWithFiles(t, files)
	base := testGit(t, root, "rev-parse", "HEAD")
	const changed = "spec/example_spec.rb"
	writeTestFile(t, root, changed, files[changed]+"# feature:shared-demo\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "changed")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	ranged, err := compileRangeImpact(context.Background(), index, base, maxLimit, false)
	if err != nil {
		t.Fatal(err)
	}
	authority := ""
	for _, row := range mapsFromAny(ranged["results"]) {
		if row["kind"] != "feature" {
			continue
		}
		for _, value := range mapsFromAny(row["evidence"]) {
			if stringValue(value["path"]) == changed {
				authority = stringValue(value["authority"])
			}
		}
	}
	if authority != "test-marker" {
		t.Fatalf("%s cited with authority %q, want test-marker: %#v", changed, authority, ranged["results"])
	}
}
