package contextindex

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func historyRecord(commit, tree, subject string, paths ...[]byte) []byte {
	var output bytes.Buffer
	output.WriteByte(0x1e)
	fmt.Fprintf(&output, "%s\x1f%s\x1f%s", commit, tree, subject)
	for _, path := range paths {
		output.WriteByte(0)
		output.Write(path)
		output.WriteByte('\n')
	}
	output.WriteByte(0)
	return output.Bytes()
}

func TestParseHistoryBuildsPythonCanonicalShape(t *testing.T) {
	commit := strings.Repeat("1", 40)
	tree := strings.Repeat("2", 40)
	entries, canonical, err := parseHistory(historyRecord(commit, tree, "roadmap: add workflow gate", []byte("AGENTS.md")), "sha1", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].subject != "roadmap: add workflow gate" || len(entries[0].paths) != 1 {
		t.Fatalf("entries=%#v", entries)
	}
	encoded, err := CanonicalJSON(canonical)
	if err != nil {
		t.Fatal(err)
	}
	want := `[["` + commit + `","` + tree + `","roadmap: add workflow gate",["AGENTS.md"]]]`
	if string(encoded) != want {
		t.Fatalf("canonical=%s want=%s", encoded, want)
	}
}

func TestDecodePythonUTF8ReplacementVectors(t *testing.T) {
	for _, test := range []struct {
		raw  []byte
		want string
	}{
		{[]byte{0xff, 0xff}, "��"},
		{[]byte{0xe1, 0x80, 'A'}, "�A"},
		{[]byte{0xf0, 0x90, 0x80, 'A'}, "�A"},
		{[]byte{0xc0, 0xaf}, "��"},
		{[]byte{0xed, 0xa0, 0x80}, "���"},
		{[]byte{0xe1, 0x80}, "�"},
	} {
		if got := decodePythonUTF8(test.raw); got != test.want {
			t.Fatalf("decodePythonUTF8(%x)=%q want=%q", test.raw, got, test.want)
		}
	}
}

func TestParseHistoryRejectsHostileRecords(t *testing.T) {
	commit := strings.Repeat("1", 40)
	tree := strings.Repeat("2", 40)
	for _, test := range []struct {
		name string
		raw  []byte
		code string
	}{
		{"malformed header", []byte("not-history"), ""},
		{"invalid path encoding", historyRecord(commit, tree, "subject", []byte{0xff}), "unsupported-query-history"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := parseHistory(test.raw, "sha1", false)
			var failure *Error
			if !errors.As(err, &failure) || failure.Code != test.code {
				t.Fatalf("error=%#v", err)
			}
		})
	}

	var oversized bytes.Buffer
	for index := 0; index <= maxHistoryCommits; index++ {
		oversized.Write(historyRecord(commit, tree, fmt.Sprintf("subject %d", index)))
	}
	if _, _, err := parseHistory(oversized.Bytes(), "sha1", false); err == nil || !strings.Contains(err.Error(), "commit limit") {
		t.Fatalf("error=%v", err)
	}
}
