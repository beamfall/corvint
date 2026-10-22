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
	for _, format := range []string{"", snapshotFormatValue, packFormatValue} {
		t.Run("format="+format, func(t *testing.T) {
			t.Setenv(snapshotFormatEnv, format)
			if _, err := WriteSnapshot(index); err != nil {
				t.Fatal(err)
			}
			loaded, hit, err := LoadSnapshot(context.Background(), root)
			if err != nil || !hit {
				t.Fatalf("hit=%v err=%v", hit, err)
			}
			if !reflect.DeepEqual(loaded.Fields, index.Fields) {
				t.Fatalf("loaded fields = %+v, want %+v", loaded.Fields, index.Fields)
			}
			check(t, loaded)
		})
	}
	t.Run("blob-shards", func(t *testing.T) {
		t.Setenv("CORVINT_INDEX_SHARDS", "1")
		if _, err := WriteSnapshot(index); err != nil {
			t.Fatal(err)
		}
		sharded, err := BuildForSnapshot(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(sharded.Fields, index.Fields) {
			t.Fatalf("sharded fields = %+v, want %+v", sharded.Fields, index.Fields)
		}
	})
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
