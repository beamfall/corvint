package contextindex

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func lookupFixture(t *testing.T) *Index {
	t.Helper()
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":                "module example.test/fixture\n\ngo 1.27.0\n",
		"cache/demux.go":        "package cache\n\nfunc Split(key string) string { return key }\n",
		"cache/demux_test.go":   "package cache\n\nfunc TestSplit() {\n\t_ = Split(\"k\")\n\t_ = Split(\"key\")\n}\n",
		"server/server.go":      "package server\n\nimport \"example.test/fixture/cache\"\n\nfunc Run() string { return cache.Split(\"key\") }\n",
		"other/a.go":            "package other\n\nfunc split() {}\n",
		"other/b.go":            "package other\n\nfunc split() {}\n",
		"other/c.go":            "package other\n\nconst SPLIT = 1\n",
		"vendor/example/bad.go": "package bad\n\nfunc Split(key string) string { return key + \"split\" }\n",
		"testing/features.yaml": "features: []\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index
}

func lookupIDs(result map[string]any) []string {
	ids := make([]string, 0)
	for _, row := range result["results"].([]any) {
		ids = append(ids, row.(map[string]any)["id"].(string))
	}
	return ids
}

func TestLookupDefinitionsOrdersExactThenRarestCaseInsensitive(t *testing.T) {
	index := lookupFixture(t)
	result, err := LookupDefinitions(index, "Split", 20)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"cache/demux.go", "other/c.go", "other/a.go", "other/b.go"}
	if got := lookupIDs(result); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("defs order = %v, want %v", got, want)
	}
	first := result["results"].([]any)[0].(map[string]any)
	reason := first["evidence"].([]any)[0].(map[string]any)["reason"].(string)
	if first["match"] != "exact" || first["line"] != 3 || !strings.Contains(reason, "defines `Split` (func)") || result["revision"] != index.Revision {
		t.Fatalf("first definition = %v", first)
	}
	if limited, _ := LookupDefinitions(index, "Split", 2); len(lookupIDs(limited)) != 2 || limited["coverage"].(map[string]any)["omitted_results"] != 2 {
		t.Fatalf("limit 2 = %v", limited)
	}
}

func TestLookupReferencesExcludesDefinersAndRanksImportersFirst(t *testing.T) {
	index := lookupFixture(t)
	result, err := LookupReferences(index, "Split", 20)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"server/server.go", "cache/demux_test.go"}
	if got := lookupIDs(result); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("refs order = %v, want %v", got, want)
	}
	rows := result["results"].([]any)
	importer, test := rows[0].(map[string]any), rows[1].(map[string]any)
	if importer["count"] != 1 || importer["imports_definer"] != true || test["count"] != 2 || test["line"] != 4 {
		t.Fatalf("refs rows = %v", rows)
	}
	if definers := result["request"].(map[string]any)["definers"].([]any); len(definers) != 1 || definers[0] != "cache/demux.go" {
		t.Fatalf("definers = %v", definers)
	}
}

func TestLookupGrepRanksByBM25AndQuotesMatchingLines(t *testing.T) {
	index := lookupFixture(t)
	result, err := LookupGrep(index, []string{"split", "key"}, 20)
	if err != nil {
		t.Fatal(err)
	}
	ids := lookupIDs(result)
	if len(ids) < 3 || ids[0] != "cache/demux.go" || ids[1] != "cache/demux_test.go" {
		t.Fatalf("grep order = %v", ids)
	}
	first := result["results"].([]any)[0].(map[string]any)
	lines := first["lines"].([]any)
	if first["distinct"] != 2 || len(lines) != 1 || lines[0].(map[string]any)["line"] != 3 || lines[0].(map[string]any)["text"] != "func Split(key string) string { return key }" {
		t.Fatalf("grep first = %v", first)
	}
	if quoted := result["results"].([]any)[1].(map[string]any)["lines"].([]any); len(quoted) != 3 || quoted[1].(map[string]any)["text"] != "\t_ = Split(\"k\")" {
		t.Fatalf("grep test lines = %v", quoted)
	}
	if empty, err := LookupGrep(index, []string{"zzzz"}, 20); err != nil || empty["state"] != "NO_CANDIDATES" {
		t.Fatalf("unknown term: %v %v", empty, err)
	}
}

// TestLookupGrepCountsATokenInBodyAndPathOnce: TCP-V0-017 orders by distinct
// tokens, so a token found in both fields of one file is one token, not two.
func TestLookupGrepCountsATokenInBodyAndPathOnce(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":           "module example.test/fixture\n\ngo 1.27.0\n",
		"widget/widget.go": "package widget\n\n// widget renders.\nfunc Render() {}\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	result, err := LookupGrep(index, []string{"widget"}, 20)
	if err != nil {
		t.Fatal(err)
	}
	first := result["results"].([]any)[0].(map[string]any)
	reason := first["evidence"].([]any)[0].(map[string]any)["reason"].(string)
	if first["distinct"] != 1 || !strings.HasPrefix(reason, "1 of 1 terms") {
		t.Fatalf("grep widget = %v", first)
	}
}

// TestLookupGrepPathOnlyHitClaimsNoLine: TCP-V0-017(c) gives a file whose
// token occurs only in its path line 0 and a path-only reason, never line 1.
func TestLookupGrepPathOnlyHitClaimsNoLine(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":            "module example.test/fixture\n\ngo 1.27.0\n",
		"other/splitter.go": "package other\n\nfunc Run() {}\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	result, err := LookupGrep(index, []string{"splitter"}, 20)
	if err != nil {
		t.Fatal(err)
	}
	first := result["results"].([]any)[0].(map[string]any)
	row := first["evidence"].([]any)[0].(map[string]any)
	if first["id"] != "other/splitter.go" || len(first["lines"].([]any)) != 0 || row["line"] != 0 ||
		!strings.HasSuffix(row["reason"].(string), "; path-only match, no line") {
		t.Fatalf("path-only grep hit = %v", first)
	}
}

func TestLookupNeverListsExcludedPathsAndRefusesBadIdentifiers(t *testing.T) {
	index := lookupFixture(t)
	if _, excluded := index.Sources["vendor/example/bad.go"]; excluded {
		t.Fatal("vendor path was indexed")
	}
	defs, _ := LookupDefinitions(index, "Split", 50)
	refs, _ := LookupReferences(index, "Split", 50)
	grep, _ := LookupGrep(index, []string{"split", "bad"}, 50)
	for mode, result := range map[string]map[string]any{"defs": defs, "refs": refs, "grep": grep} {
		for _, id := range lookupIDs(result) {
			if strings.HasPrefix(id, "vendor/") {
				t.Fatalf("%s listed excluded path %s", mode, id)
			}
		}
	}
	for _, identifier := range []string{"", strings.Repeat("a", 257)} {
		var contextError *Error
		if _, err := LookupDefinitions(index, identifier, 20); !errors.As(err, &contextError) || contextError.Code != LookupIdentifierErrorCode {
			t.Fatalf("defs %q: %v", identifier, err)
		}
		if _, err := LookupReferences(index, identifier, 20); !errors.As(err, &contextError) || contextError.Code != LookupIdentifierErrorCode {
			t.Fatalf("refs %q: %v", identifier, err)
		}
		if _, err := LookupGrep(index, []string{"ok", identifier}, 20); !errors.As(err, &contextError) || contextError.Code != LookupIdentifierErrorCode {
			t.Fatalf("grep %q: %v", identifier, err)
		}
	}
	if _, err := LookupDefinitions(index, "Split", 0); err == nil {
		t.Fatal("limit 0 accepted")
	}
}
