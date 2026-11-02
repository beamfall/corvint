package contextindex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestFuzzyEditDistanceOne(t *testing.T) {
	t.Run("TCP-V0-650", func(t *testing.T) {
		for _, item := range []struct {
			left, right string
			want        bool
		}{
			{"unplaned", "unplanned", true}, {"ledgr", "ledger", true}, {"ledger", "ledgr", true},
			{"leadger", "ledger", true}, {"ledegr", "ledger", true}, {"ledgar", "ledger", true},
			{"ledger", "ledger", false}, {"ledgers", "ledger", true}, {"lgdeer", "ledger", false},
			{"ledg", "ledger", false}, {"abcd", "badc", false},
		} {
			if got := editDistanceOne(item.left, item.right); got != item.want {
				t.Fatalf("editDistanceOne(%q, %q) = %v", item.left, item.right, got)
			}
		}
	})
}

func fuzzyFixtureIndex(t *testing.T) *Index {
	t.Helper()
	files := map[string]string{
		"go.mod":                          "module example.test/fuzzy\n\ngo 1.27.0\n",
		"AGENTS.md":                       "# Standing instructions\n\nRead before editing.\n",
		"internal/unplannedread/store.go": "package unplannedread\n\n// store appends unplanned read ledger entries\nfunc store() {}\n",
		"tools/scopes/scopes.go":          "package scopes\n\n// scopes are loaded here\nfunc load() {}\n",
		".corvint/scopes.json":            "{\"scopes\": [\"one\"]}\n",
		"ci/narrow.sh":                    "#!/bin/sh\n# select packages\ngit merge-base origin/main HEAD\n",
		"ci/other.sh":                     "#!/bin/sh\n# merge and base are separate words here\n",
	}
	for index := range 9 {
		files[fmt.Sprintf("pkg/p%d/p.go", index)] = fmt.Sprintf("package p%d\n\n// unplanned ledger work\nfunc f() {}\n", index)
	}
	root := impactRepositoryWithFiles(t, files)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index
}

func fuzzyRow(t *testing.T, index *Index, task, path string) (int, string) {
	t.Helper()
	packet, err := TaskContext(context.Background(), index, task, "", 20)
	if err != nil {
		t.Fatal(err)
	}
	for position, row := range mapsFromAny(packet["results"]) {
		if row["id"] == path {
			return position, mapsFromAny(row["evidence"])[0]["reason"].(string)
		}
	}
	return -1, ""
}

func TestContextFuzzyFallbackIsLabelled(t *testing.T) {
	index := fuzzyFixtureIndex(t)
	for _, item := range []struct {
		id, task, path, label string
	}{
		{"TCP-V0-650", "unplaned ledgr", "internal/unplannedread/store.go", "fuzzy `ledgr`~`ledger`, `unplaned`~`unplanned`"},
		{"TCP-V0-651", "narrowing", "ci/narrow.sh", "fuzzy `narrowing`~`narrow`; "},
		{"TCP-V0-651", "unplanned read", "internal/unplannedread/store.go", "`unplanned read`~`unplannedread`"},
		{"TCP-V0-651", "check merge-base", "ci/narrow.sh", "`merge-base` verbatim"},
		{"TCP-V0-652", "scopes config", ".corvint/scopes.json", "fuzzy `config`~`configuration file`"},
	} {
		t.Run(item.id+"/"+item.task, func(t *testing.T) {
			t.Setenv("CORVINT_CONTEXT_FUZZY", "on")
			position, reason := fuzzyRow(t, index, item.task, item.path)
			if position < 0 || !strings.Contains(reason, item.label) {
				t.Fatalf("%s at %d, reason %q, want %q", item.path, position, reason, item.label)
			}
			t.Setenv("CORVINT_CONTEXT_FUZZY", "")
			if _, reason := fuzzyRow(t, index, item.task, item.path); strings.Contains(reason, "fuzzy ") {
				t.Fatalf("flag off still labels fuzzy: %q", reason)
			}
		})
	}
	t.Run("TCP-V0-650/absent-without-flag", func(t *testing.T) {
		if position, _ := fuzzyRow(t, index, "unplaned ledgr", "internal/unplannedread/store.go"); position >= 0 {
			t.Fatalf("misspelt task matched at %d without the flag", position)
		}
	})
	t.Run("TCP-V0-652/record-data-named-by-path", func(t *testing.T) {
		t.Setenv("CORVINT_CONTEXT_FUZZY", "on")
		// Two path-field credits only: `scopes` and the configuration word.
		position, _ := fuzzyRow(t, index, "scopes config", ".corvint/scopes.json")
		code, _ := fuzzyRow(t, index, "scopes config", "tools/scopes/scopes.go")
		if position < 0 || code >= 0 && position > code {
			t.Fatalf("path-named configuration at %d after code at %d", position, code)
		}
	})
}

func TestContextFuzzyCorroborationIsLabelled(t *testing.T) {
	t.Run("TCP-V0-650", func(t *testing.T) {
		files := map[string]string{
			"go.mod":        "module example.test/corroborate\n\ngo 1.27.0\n",
			"pkg/a.go":      "package a\n\nfunc A() {}\n",
			"pkg/a_test.go": "package a\n\n// ledger\nfunc TestA() {}\n",
		}
		for index := range 8 {
			files[fmt.Sprintf("other/o%d/o.go", index)] = fmt.Sprintf("package o%d\n\n// ledger entries\nfunc f() {}\n", index)
		}
		index, err := Build(context.Background(), impactRepositoryWithFiles(t, files))
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("CORVINT_CONTEXT_FUZZY", "on")
		packet, err := TaskContext(context.Background(), index, "ledgr", "pkg/a.go", 20)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range mapsFromAny(packet["results"]) {
			if row["id"] != "pkg/a_test.go" {
				continue
			}
			summary, _ := row["summary"].(string)
			reason, _ := mapsFromAny(row["evidence"])[0]["reason"].(string)
			if !strings.Contains(summary, "lexical") || !strings.HasPrefix(reason, "fuzzy `ledgr`~`ledger`; ") {
				t.Fatalf("fuzzy corroboration undisclosed: summary %q, reason %q", summary, reason)
			}
			return
		}
		t.Fatal("counterpart test missing from the packet")
	})
}

func TestContextFuzzyVerbatimKeepsCase(t *testing.T) {
	t.Run("TCP-V0-651", func(t *testing.T) {
		index, err := Build(context.Background(), impactRepositoryWithFiles(t, map[string]string{
			"go.mod":      "module example.test/verbatim\n\ngo 1.27.0\n",
			"ci/upper.sh": "#!/bin/sh\ncurl -H X-Request-ID:1 example.test\n",
			"ci/lower.sh": "#!/bin/sh\necho x-request-id\n",
			"pkg/p/p.go":  "package p\n\nfunc f() {}\n",
		}))
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("CORVINT_CONTEXT_FUZZY", "on")
		if _, reason := fuzzyRow(t, index, "X-Request-ID", "ci/upper.sh"); !strings.Contains(reason, "`X-Request-ID`") {
			t.Fatalf("verbatim literal missed its case-exact source: %q", reason)
		}
		if _, reason := fuzzyRow(t, index, "X-Request-ID", "ci/lower.sh"); strings.Contains(reason, "verbatim") {
			t.Fatalf("lowercase source took the verbatim credit: %q", reason)
		}
	})
}

func TestContextFuzzyDefaultBytes(t *testing.T) {
	t.Run("TCP-V0-650", func(t *testing.T) {
		index := recipeFixtureIndex(t)
		golden, err := os.ReadFile("testdata/context-recipe-default-golden.json")
		if err != nil {
			t.Fatal(err)
		}
		for _, flag := range []string{"", "off", "1"} {
			t.Setenv("CORVINT_CONTEXT_FUZZY", flag)
			if compiler := newTaskContextCompiler(index, "ledger", ""); compiler.fuzzy {
				t.Fatalf("flag %q enabled the fuzzy fallback", flag)
			}
			packet := recipePacket(t, index, "", recipeFixtureTask)
			encoded, err := json.MarshalIndent(packet, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(append(encoded, '\n'), golden) {
				t.Fatalf("flag %q changed golden packet bytes", flag)
			}
		}
		t.Setenv("CORVINT_CONTEXT_FUZZY", "on")
		t.Setenv("CORVINT_CONTEXT_TERMS", "ident")
		if compiler := newTaskContextCompiler(index, "ledger", ""); compiler.fuzzy {
			t.Fatal("identifier term selection kept the fuzzy fallback on")
		}
	})
}
