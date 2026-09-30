package contextindex

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func unknownCodes(receipt map[string]any, name string) map[string]bool {
	codes := map[string]bool{}
	for _, row := range mapsFromAny(receipt["unknowns"]) {
		if row["path"] == name {
			codes[stringValue(row["code"])] = true
		}
	}
	return codes
}

// NGI-V0-002 NGI-V0-004 NGI-V0-006: every new suffix works at the repository
// root without a Go module, carries real committed hunks, and repeats exactly.
func TestNonGoRangeImpact(t *testing.T) {
	for _, suffix := range []string{".rb", ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx"} {
		t.Run(suffix, func(t *testing.T) {
			name := "value" + suffix
			before, after := "// feature:changed\nfunction value() { return 1; }\n", "// feature:changed\nfunction value() { return 2; }\n"
			if suffix == ".rb" {
				before, after = "# feature:changed\ndef value\n  1\nend\n", "# feature:changed\ndef value\n  2\nend\n"
			}
			root := impactRepositoryWithFiles(t, map[string]string{name: before})
			base := testGit(t, root, "rev-parse", "HEAD")
			writeTestFile(t, root, name, after)
			testGit(t, root, "add", ".")
			testGit(t, root, "commit", "-qm", "changed")
			index, err := Build(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			for _, expanded := range []bool{false, true} {
				a, err := compileRangeImpact(context.Background(), index, base, 10, expanded)
				if err != nil {
					t.Fatal(err)
				}
				b, err := compileRangeImpact(context.Background(), index, base, 10, expanded)
				if err != nil {
					t.Fatal(err)
				}
				ab, _ := CanonicalJSON(a)
				bb, _ := CanonicalJSON(b)
				if !bytes.Equal(ab, bb) {
					t.Fatal("repeat changed receipt bytes")
				}
				if !stringIn(resultKeys(a), "path:"+name) || a["omissions"].(map[string]any)["count"] != 0 {
					t.Fatalf("member missing: %v", a)
				}
				binding := a["range"].(map[string]any)
				if binding["headTree"] != index.Revision || binding["changedGoPathCount"] != 0 || binding["hunkCount"] != 1 {
					t.Fatalf("binding=%v", binding)
				}
				codes := unknownCodes(a, name)
				if !codes["dynamic-dispatch-unresolved"] {
					t.Fatalf("unknowns=%v", a["unknowns"])
				}
				for _, row := range mapsFromAny(a["results"]) {
					if row["kind"] == "path" {
						e := mapsFromAny(row["evidence"])[0]
						if e["authority"] != "git-diff-hunk" || e["blob_hash"] != index.Sources[name].BlobHash {
							t.Fatalf("evidence=%v", e)
						}
					}
				}
				for _, check := range anySlice(a["verification"]) {
					if strings.HasPrefix(stringValue(check), "go test ./.") {
						t.Fatalf("invented Go test: %v", check)
					}
				}
			}
		})
	}
}

// NGI-V0-003: lexical probes ignore comments and literal bodies. The baseline
// uncertainty stays even when a negative control has no observed dynamic call.
func TestNonGoImpactDynamicUnknowns(t *testing.T) {
	fixtures := []struct{ name, body, observed string }{
		{"app/job.rb", "def perform(key)\n public_send(key)\nend\n", "observed-reflective-dispatch"},
		{"app/twin.rb", "# public_send(key)\ndef perform\n 'define_method(key)'\nend\n", ""},
		{"src/a.js", "function value(obj,key) { return obj[key](); }\n", "observed-computed-dispatch"},
		{"src/b.ts", "function value(key: string) { return import(key); }\n", "dynamic-load-unresolved"},
		{"src/twin.tsx", "// obj[key]()\nconst text = 'import(key)';\n", ""},
		{"src/literal.mjs", "export const value = import('./literal.js');\n", ""},
		{"app/require.rb", "require path\nload path\n", "dynamic-load-unresolved"},
		{"app/interpolation.rb", "require(\"#{name}\")\n", "dynamic-load-unresolved"},
		{"app/literal.rb", "require 'literal'\nload(\"literal.rb\")\n", ""},
		{"app/load_twin.rb", "# require path\ntext = 'load path'\n", ""},
		{"src/concatenation.js", "const value = import('./' + name);\n", "dynamic-load-unresolved"},
		{"src/load_twin.js", "// import('./' + name)\nconst text = \"require(name)\";\n", ""},
	}
	files := map[string]string{}
	for _, fixture := range fixtures {
		files[fixture.name] = fixture.body
	}
	root := impactRepositoryWithFiles(t, files)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		receipt, err := Impact(index, []string{f.name}, 1)
		if err != nil {
			t.Fatal(err)
		}
		codes := unknownCodes(receipt, f.name)
		if !codes["dynamic-dispatch-unresolved"] {
			t.Fatal("baseline frontier absent")
		}
		if f.observed != "" && !codes[f.observed] {
			t.Fatalf("%s codes=%v", f.name, codes)
		}
		if f.observed == "" && (codes["observed-reflective-dispatch"] || codes["observed-computed-dispatch"] || codes["dynamic-load-unresolved"]) {
			t.Fatalf("literal/comment leaked: %s %v", f.name, codes)
		}
	}
}

// NGI-V0-003 NGI-V0-006: the result limit never hides another path's frontier.
func TestNonGoImpactFrontierBeyondLimit(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{"a.rb": "def a; end\n", "b.ts": "export function b() {}\n"})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := Impact(index, []string{"b.ts", "a.rb"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.rb", "b.ts"} {
		if !unknownCodes(receipt, name)["dynamic-dispatch-unresolved"] {
			t.Fatalf("lost frontier for %s", name)
		}
	}
	budget := 8192
	compiled, err := EvalImpact(index, []string{"a.rb", "b.ts"}, 1, &budget)
	if err != nil {
		t.Fatal(err)
	}
	if !unknownCodes(compiled, "b.ts")["dynamic-dispatch-unresolved"] {
		t.Fatal("budget projection lost frontier")
	}
}

// NGI-V0-003 NGI-V0-006: excluded syntax cannot silently lose its frontier
// through the path compiler's early OUT_OF_SCOPE return.
func TestNonGoImpactExcludedFrontier(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{"vendor/value.rb": "def value; end\n"})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := Impact(index, []string{"vendor/value.rb"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	codes := unknownCodes(receipt, "vendor/value.rb")
	if !codes["dynamic-dispatch-unresolved"] || !codes["source-analysis-unavailable"] {
		t.Fatalf("excluded frontier=%v", receipt["unknowns"])
	}
	if len(resultKeys(receipt)) != 0 {
		t.Fatal("excluded source produced ranked evidence")
	}
}

// NGI-V0-004: Go-only input keeps the existing shape and module constraint.
func TestNonGoRangeImpactLegacyGoBytes(t *testing.T) {
	root := impactRepository(t, "")
	base := commitRangeFixture(t, root)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	a, err := RangeImpact(context.Background(), index, base, 50)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := a["unknowns"]; exists {
		t.Fatal("Go-only receipt changed by extension")
	}
	if a["range"].(map[string]any)["changedGoPathCount"] != 2 {
		t.Fatal("Go count changed")
	}
	b, err := RangeImpact(context.Background(), index, base, 50)
	if err != nil {
		t.Fatal(err)
	}
	ab, _ := CanonicalJSON(a)
	bb, _ := CanonicalJSON(b)
	if !bytes.Equal(ab, bb) {
		t.Fatal("Go receipt unstable")
	}
}

// NGI-V0-002 NGI-V0-004 NGI-V0-006 ERI-V0-002 ERI-V0-004: a mixed
// range keeps Go counting/test selectors distinct from syntax-only members.
func TestNonGoRangeImpactMixed(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":                  "module example.test/mixed\n\ngo 1.27\n",
		"internal/value/value.go": "package value\nfunc Value() int { return 1 }\n",
		"value.ts":                "export function value() { return 1; }\n",
	})
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTestFile(t, root, "internal/value/value.go", "package value\nfunc Value() int { return 2 }\n")
	writeTestFile(t, root, "value.ts", "export function value() { return 2; }\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "mixed range")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, expanded := range []bool{false, true} {
		receipt, err := compileRangeImpact(context.Background(), index, base, 10, expanded)
		if err != nil {
			t.Fatal(err)
		}
		if receipt["range"].(map[string]any)["changedGoPathCount"] != 1 || len(resultKeys(receipt)) != 2 {
			t.Fatalf("mixed membership=%v", receipt)
		}
		if !unknownCodes(receipt, "value.ts")["dynamic-dispatch-unresolved"] || len(unknownCodes(receipt, "internal/value/value.go")) != 0 {
			t.Fatalf("mixed frontier=%v", receipt["unknowns"])
		}
	}
}

// NGI-V0-006 ERI-V0-002: new-language members retain fail-closed snapshot,
// binary, excluded-source and target-blob checks in both capacity profiles.
func TestNonGoRangeImpactRefusals(t *testing.T) {
	for _, suffix := range []string{".rb", ".js", ".ts"} {
		for _, failure := range []string{"dirty", "binary", "excluded", "drift", "snapshot"} {
			t.Run(suffix+"/"+failure, func(t *testing.T) {
				name := "value" + suffix
				root := impactRepositoryWithFiles(t, map[string]string{name: "value = 1\n"})
				base := testGit(t, root, "rev-parse", "HEAD")
				writeTestFile(t, root, name, "value = 2\n")
				if failure == "binary" {
					writeTestFile(t, root, name, "value\x00binary\n")
				}
				if failure == "excluded" {
					writeTestFile(t, root, "vendor/excluded"+suffix, "value = 2\n")
				}
				testGit(t, root, "add", ".")
				testGit(t, root, "commit", "-qm", failure)
				if failure == "dirty" {
					writeTestFile(t, root, name, "value = 3\n")
				}
				index, err := Build(context.Background(), root)
				if err != nil {
					t.Fatal(err)
				}
				code := "unsupported-impact-range"
				if failure == "dirty" {
					code = "unsupported-impact-worktree"
				}
				if failure == "snapshot" {
					writeTestFile(t, root, name, "value = 3\n")
					code = "impact-range-drift"
				}
				if failure == "drift" {
					source := index.Sources[name]
					source.BlobHash = strings.Repeat("0", 40)
					index.Sources[name] = source
					code = "impact-range-drift"
				}
				for _, expanded := range []bool{false, true} {
					_, err = compileRangeImpact(context.Background(), index, base, 10, expanded)
					assertRangeErrorCode(t, err, code)
				}
			})
		}
	}
}
