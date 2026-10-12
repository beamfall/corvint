package main

import (
	"strings"
	"testing"
)

func TestParseJSONRefusesDuplicateKeys(t *testing.T) {
	if _, err := parseJSON([]byte(`{"a":1,"b":{"c":1,"c":2}}`)); err == nil || !strings.Contains(err.Error(), `duplicate key "c"`) {
		t.Fatalf("nested duplicate: %v", err)
	}
	if _, err := parseJSON([]byte(`[{"a":1},{"a":2}]`)); err != nil {
		t.Fatalf("same key in different objects: %v", err)
	}
}

func TestInsertMemberCompact(t *testing.T) {
	src := []byte(`{"a":1,"b":2}`)
	root, err := parseJSON(src)
	if err != nil {
		t.Fatal(err)
	}
	edit, err := insertMember(src, root, "a", "n", func(indent string) string { return indent + "3" })
	if err != nil {
		t.Fatal(err)
	}
	out, err := applySplices(src, []splice{edit})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out); got != "{\"a\":1,\n\"n\": 3,\"b\":2}" {
		t.Fatalf("compact insert: %q", got)
	}
}
