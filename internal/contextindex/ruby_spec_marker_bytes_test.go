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
