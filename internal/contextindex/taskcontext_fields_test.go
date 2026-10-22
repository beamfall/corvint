package contextindex

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

// TCP-V0-162: the named fields of top-level struct types are a table of
// their own, with file and line; the declaration table is unchanged.
func TestGoStructFieldsAreIndexedWithFileAndLine_TCPV0162(t *testing.T) {
	text := "package p\n\n" +
		"type (\n" +
		"\tPlan struct {\n" +
		"\t\tExpectedTests []string\n" +
		"\t\tFirst, Second int\n" +
		"\t\t_ int\n" +
		"\t\tEmbedded\n" +
		"\t\tNested struct{ Inner int }\n" +
		"\t}\n" +
		"\tAlias = int\n" +
		")\n\n" +
		"type Embedded struct{ Own bool }\n\n" +
		"func local() { type Hidden struct{ Local int }; _ = Hidden{} }\n"
	symbols, fields, refusal := goSymbols(testSource(), text)
	if refusal != "" {
		t.Fatal(refusal)
	}
	got := make([]string, 0, len(fields))
	for _, field := range fields {
		got = append(got, fmt.Sprintf("%s %s %s:%d", field.Kind, field.Name, field.Path, field.Line))
	}
	want := []string{"field ExpectedTests sample.go:5", "field First sample.go:6", "field Second sample.go:6", "field Nested sample.go:9", "field Own sample.go:14"}
	if !slices.Equal(got, want) {
		t.Fatalf("fields = %q, want %q", got, want)
	}
	for _, symbol := range symbols {
		if symbol.Kind == "field" {
			t.Fatalf("a field leaked into the declaration table: %+v", symbol)
		}
	}
}

func fieldFixture(t *testing.T) string {
	t.Helper()
	return impactRepositoryWithFiles(t, map[string]string{
		"go.mod": "module example.test/runner\n\ngo 1.27.0\n",
		"internal/testrunner/types.go": "package testrunner\n\n// Plan is one selection.\ntype Plan struct {\n" +
			"\tSessionID string\n\tExpectedTests []string\n\tSelect string\n}\n",
		"internal/testrunner/validate.go":        "package testrunner\n\nfunc Validate(plan Plan) bool { return len(plan.ExpectedTests) > 0 }\n",
		"internal/testrunner/dynamic/browser.go": "package dynamic\n\nfunc Browse() {}\n",
		"internal/selector/select.go":            "package selector\n\nfunc Select() {}\n",
	})
}

// TCP-V0-162, V1-1015: a task naming a struct field carries the defining file
// as a definition row at the field's line, the anchor is no longer disclosed
// as unresolved, and the field table survives every snapshot encoding.
func TestTaskContextRetrievesTheStructFieldDefiner_TCPV0162(t *testing.T) {
	root := fieldFixture(t)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	task := "Select by SessionID in internal/testrunner/dynamic/browser.go and fill ExpectedTests"
	check := func(t *testing.T, index *Index) {
		t.Helper()
		packet, err := TaskContext(context.Background(), index, task, "", 10)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, row := range mapsFromAny(packet["results"]) {
			if row["kind"] == "definition" && row["id"] == "internal/testrunner/types.go" {
				found = true
			}
		}
		if !found {
			t.Fatalf("types.go is not a definition row: %v", packet["results"])
		}
		if anchors, present := packet["coverage"].(map[string]any)["unresolved_anchors"]; present {
			t.Fatalf("a field anchor is still disclosed as unresolved: %v", anchors)
		}
	}
	check(t, index)
	lines := map[string]int{}
	for _, field := range index.Fields {
		lines[field.Name] = field.Line
	}
	if lines["ExpectedTests"] != 6 || lines["SessionID"] != 5 {
		t.Fatalf("field lines = %v", lines)
	}
	if len(index.Fields) == 0 {
		t.Fatal("the fixture indexed no fields")
	}
	// Each encoding is decoded directly: LoadSnapshot would fall back to the
	// gob snapshot written beside a refused pack or sectioned file.
	identity := fixtureIdentity(index)
	for _, format := range []string{"", snapshotFormatValue, packFormatValue} {
		t.Run("format="+format, func(t *testing.T) {
			t.Setenv(snapshotFormatEnv, format)
			receipt, err := WriteSnapshot(index)
			if err != nil {
				t.Fatal(err)
			}
			for _, load := range []snapshotLoad{loadFull, loadEvent} {
				var loaded *Index
				switch format {
				case snapshotFormatValue:
					loaded, err = readSectionedSnapshot(receipt.SectionedPath, identity, receipt.Engine, load)
				case packFormatValue:
					loaded, err = readPackSnapshot(receipt.PackPath, identity, analyzerEngine(), load)
				default:
					loaded = gobDecoded(t, receipt, identity, load)
				}
				if err != nil {
					t.Fatalf("load %d: %v", load, err)
				}
				if !reflect.DeepEqual(loaded.Fields, index.Fields) {
					t.Fatalf("load %d: decoded fields = %+v, want %+v", load, loaded.Fields, index.Fields)
				}
				if load == loadFull {
					check(t, loaded)
				}
			}
		})
	}
	t.Run("blob-shards", func(t *testing.T) {
		t.Setenv(snapshotFormatEnv, "")
		t.Setenv("CORVINT_INDEX_SHARDS", "1")
		if _, err := WriteSnapshot(index); err != nil {
			t.Fatal(err)
		}
		fetched := blobBatchEntries.Load()
		sharded, err := buildWithBlobShards(context.Background(), root, nil, (*Index).compile)
		if err != nil {
			t.Fatal(err)
		}
		if got := blobBatchEntries.Load() - fetched; got != 0 || len(sharded.blobFacts) != len(index.Sources) {
			t.Fatalf("shards were not reused: fetched %d blobs, %d of %d facts", got, len(sharded.blobFacts), len(index.Sources))
		}
		if !reflect.DeepEqual(sharded.Fields, index.Fields) {
			t.Fatalf("sharded fields = %+v, want %+v", sharded.Fields, index.Fields)
		}
	})
	t.Run("snapshot-reuse", func(t *testing.T) {
		t.Setenv(snapshotFormatEnv, "")
		t.Setenv("CORVINT_INDEX_SHARDS", "")
		if _, err := WriteSnapshot(index); err != nil {
			t.Fatal(err)
		}
		reused, err := BuildForSnapshot(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		if reused.reusedSources != len(index.Sources) {
			t.Fatalf("snapshot reuse took %d of %d sources", reused.reusedSources, len(index.Sources))
		}
		if !reflect.DeepEqual(reused.Fields, index.Fields) {
			t.Fatalf("reused fields = %+v, want %+v", reused.Fields, index.Fields)
		}
	})
}

// TCP-V0-162: a parenthesized struct type is the same struct, so its fields
// are recorded rather than silently dropped.
func TestParenthesizedStructTypeFieldsAreIndexed_TCPV0162(t *testing.T) {
	text := "package p\n\ntype Plan (struct {\n\tExpectedTests []string\n})\n\ntype Deep ((struct{ Inner int }))\n"
	_, fields, refusal := goSymbols(testSource(), text)
	if refusal != "" {
		t.Fatal(refusal)
	}
	got := make([]string, 0, len(fields))
	for _, field := range fields {
		got = append(got, fmt.Sprintf("%s:%d", field.Name, field.Line))
	}
	if want := []string{"ExpectedTests:4", "Inner:7"}; !slices.Equal(got, want) {
		t.Fatalf("fields = %q, want %q", got, want)
	}
}

// TCP-V0-162: a path keeps its strongest definition match. a.go's unique
// ZuluName must not be displaced by the two-definer field AlphaFlag the task
// names first, so a.go ties b.go's unique MiddleName and cites ZuluName.
func TestDefinitionRowKeepsTheStrongestMatchPerPath_TCPV0162(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod": "module example.test/strongest\n\ngo 1.27.0\n",
		"a/a.go": "package a\n\ntype A struct{ AlphaFlag bool }\n\nfunc ZuluName() {}\n",
		"b/b.go": "package b\n\nfunc MiddleName() {}\n",
		"c/c.go": "package c\n\ntype C struct{ AlphaFlag bool }\n",
		"d/d.go": "package d\n\nfunc Other() {}\n",
		"e/e.go": "package e\n\nfunc Another() {}\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := TaskContext(context.Background(), index, "Rename `AlphaFlag`, `MiddleName` and `ZuluName`", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]string, 0)
	for _, row := range mapsFromAny(packet["results"]) {
		if row["kind"] == "definition" {
			reason := mapsFromAny(row["evidence"])[0]["reason"]
			rows = append(rows, fmt.Sprintf("%s %v", row["id"], reason))
		}
	}
	want := []string{"a/a.go defines ZuluName", "b/b.go defines MiddleName", "c/c.go defines AlphaFlag"}
	if !slices.Equal(rows, want) {
		t.Fatalf("definition rows = %q, want %q", rows, want)
	}
}

// TCP-V0-162: a field defines a name only when no declaration does, so a name
// a function already defines keeps its exact definer set.
func TestStructFieldNeverJoinsADeclaredNamesDefiners_TCPV0162(t *testing.T) {
	index, err := Build(context.Background(), fieldFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	packet, err := TaskContext(context.Background(), index, "Change `Select` in the selector", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := contextRowsByKind(t, packet)["definition"]; !slices.Equal(got, []string{"internal/selector/select.go"}) {
		t.Fatalf("definition rows = %v", got)
	}
}
