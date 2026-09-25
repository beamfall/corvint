package contextindex

import (
	"context"
	"fmt"
	"testing"
)

// TestMarkersComeOnlyFromCommentSpans is the frozen negative case for panel
// blocker B4: a marker inside a string literal or a data file is never
// project-authority evidence, while a comment marker keeps its line and column.
func TestMarkersComeOnlyFromCommentSpans(t *testing.T) {
	t.Run("GPK-V0-068", func(t *testing.T) {
		root := testRepository(t)
		writeTestFile(t, root, "internal/token/token_test.go", "package token\n\n"+
			"// feature:real-marker\n"+
			"var fixture = \"// feature:string-fixture\"\n"+
			"var raw = `\nscenario:raw-fixture\n`\n"+
			"var r = '\"' /* scenario:block-marker */\n")
		writeTestFile(t, root, "web/flow.mjs", "const s = \"feature:js-string\"; // feature:js-comment\nconst t = `\nscenario:js-template\n`;\n")
		writeTestFile(t, root, "script/tool.py", "\"\"\"\nfeature:py-docstring\n\"\"\"\nx = 'feature:py-string'  # feature:py-comment\n")
		writeTestFile(t, root, "testing/goldens.json", "{\"critical\": [\"feature:json-string\"]}\n")
		testGit(t, root, "add", ".")
		testGit(t, root, "commit", "-qm", "marker fixtures")

		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		want := map[string]string{
			"feature:real-marker":   "internal/token/token_test.go:3:3",
			"scenario:block-marker": "internal/token/token_test.go:8:15",
			"feature:js-comment":    "web/flow.mjs:1:34",
			"feature:py-comment":    "script/tool.py:4:27",
		}
		got := make(map[string]string)
		for key, markers := range index.Markers {
			for _, marker := range markers {
				got[key] = fmt.Sprintf("%s:%d:%d", marker.Path, marker.Line, marker.Column)
			}
		}
		for key, location := range want {
			if got[key] != location {
				t.Errorf("marker %s at %q, want %q", key, got[key], location)
			}
		}
		for key := range got {
			if _, ok := want[key]; !ok {
				t.Errorf("marker %s from a non-comment span at %s", key, got[key])
			}
		}
	})
}

// TestImpactCreditsOnlyRelatedMarkedTestsAndKeepsCallers pins the other half
// of B4: a same-package test whose marker does not relate to the change is not
// a 850 marked-test row, and a cross-package caller stays inside the limit
// when same-package references would otherwise fill it.
func TestImpactCreditsOnlyRelatedMarkedTestsAndKeepsCallers(t *testing.T) {
	t.Run("GPK-V0-068", func(t *testing.T) {
		root := testRepository(t)
		writeTestFile(t, root, "internal/token/other_test.go", "package token\n\n// feature:unrelated-flow\nfunc TestOther() {}\n")
		writeTestFile(t, root, "internal/token/mint_test.go", "package token\n\n// feature:minting\nfunc TestMint() { _ = MintToken() }\n")
		for number := range 10 {
			writeTestFile(t, root, fmt.Sprintf("internal/token/use%02d.go", number),
				fmt.Sprintf("package token\n\nfunc Use%02d() string { return MintToken() }\n", number))
		}
		writeTestFile(t, root, "cmd/app/main.go", "package main\n\nimport \"example.test/fixture/internal/token\"\n\nfunc main() { _ = token.MintToken() }\n")
		testGit(t, root, "add", ".")
		testGit(t, root, "commit", "-qm", "impact fixtures")

		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		receipt, err := Impact(index, []string{"internal/token/token.go"}, 10)
		if err != nil {
			t.Fatalf("Impact: %v", err)
		}
		scores := make(map[string]any)
		for _, raw := range anySlice(receipt["results"]) {
			result := raw.(map[string]any)
			scores[fmt.Sprintf("%s:%s", result["kind"], result["id"])] = result["score"]
		}
		if score, ok := scores["test:internal/token/other_test.go"]; ok && score == 850 {
			t.Errorf("unrelated marked test credited at 850: %v", scores)
		}
		if scores["test:internal/token/mint_test.go"] != 850 {
			t.Errorf("related marked test score = %v, want 850: %v", scores["test:internal/token/mint_test.go"], scores)
		}
		if _, ok := scores["reverse-import:cmd/app/main.go"]; !ok {
			t.Errorf("cross-package caller omitted at limit 10: %v", scores)
		}
	})
}
