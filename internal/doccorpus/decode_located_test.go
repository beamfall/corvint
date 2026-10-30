package doccorpus

import (
	"errors"
	"strings"
	"testing"
)

// TestDCXV1470LocatedDecodeRefusal pins that a caller input's closed-decode refusal names the
// input, the JSON pointer, the record id, the reason and the offset, while an artifact keeps the
// bare message its streamed decoder must match (DCX-V1-470).
func TestDCXV1470LocatedDecodeRefusal(t *testing.T) {
	shard := []byte(`{"subjects":[{"id":"subject:a","evidence":{"anchors":[]}},` +
		`{"id":"subject:b","evidence":{"anchors":[false]}}]}`)
	cases := []struct {
		name, data string
		want       []string
	}{
		{"wrong kind", string(shard), []string{
			"invalid closed JSON input: corpus-provider/shard-001.json: /subjects/1/evidence/anchors/0: record subject:b: expected object, got false (offset ",
		}},
		{"unknown member", `{"subjects":[{"id":"s:1","bogus":1}]}`, []string{
			"corpus-provider/shard-001.json: /subjects/0/bogus: record s:1: unknown member (offset ",
		}},
		{"syntax", `{"subjects":[{"id":"s:1",]}`, []string{
			"corpus-provider/shard-001.json: /subjects/0: invalid character ',' at start of value (offset 24)",
		}},
		{"no printable id", `{"subjects":[{"id":"s\u0007","kind":1}]}`, []string{
			"corpus-provider/shard-001.json: /subjects/0/kind: expected string, got number (offset ",
		}},
	}
	for _, tc := range cases {
		var record ProviderRecord
		err := decodeNamed("corpus-provider/shard-001.json", []byte(tc.data), &record, MaxProviderBytes)
		var refusal *Error
		if !errors.As(err, &refusal) || refusal.Code != "corpus-refused" {
			t.Fatalf("%s: err = %v, want corpus-refused", tc.name, err)
		}
		for _, want := range tc.want {
			if !strings.Contains(refusal.Message, want) {
				t.Fatalf("%s: message %q lacks %q", tc.name, refusal.Message, want)
			}
		}
		if tc.name == "no printable id" && strings.Contains(refusal.Message, "record ") {
			t.Fatalf("unprintable id reached the message: %q", refusal.Message)
		}
	}
	long := `{"` + strings.Repeat("x", 4096) + `":1}`
	var record ProviderRecord
	err := decodeNamed(strings.Repeat("p", 4096), []byte(long), &record, MaxProviderBytes)
	if err == nil || len(err.Error()) > 2048 {
		t.Fatalf("unbounded message: %v", err)
	}
	var artifact Artifact
	if err := decodeBounded(shard, &artifact, MaxCorpusBytes); err == nil || err.Error() != fail("invalid closed JSON input").Error() {
		t.Fatalf("artifact decode message changed: %v", err)
	}
	if _, err := ParseManifest([]byte(`{"schema":1}`)); err == nil || !strings.Contains(err.Error(), "manifest: /schema: expected string, got number") {
		t.Fatalf("manifest refusal not located: %v", err)
	}
}
