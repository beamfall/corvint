package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMentionBoundariesAndEvidenceSeparation(t *testing.T) {
	reqs := []requirement{{"GPK-V0-014", "spec.md"}, {"AHI-001", "spec.md"}, {"UCV0-001", "other.md"}}
	m := newMatcher(reqs)
	for _, tc := range []struct {
		input string
		want  int
	}{{"GPK-V0-014 TestGPKV0014Works", 2}, {"XGPK-V0-014 GPK-V0-0140 1GPKV0014 GPKV00140", 0}, {"TestAHI001 Works UCV0-001", 2}} {
		n := 0
		m.each([]byte(tc.input), func(string, int) { n++ })
		if n != tc.want {
			t.Fatalf("%q: %d", tc.input, n)
		}
	}
	root := t.TempDir()
	write := func(p, s string) {
		p = filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("internal/a_test.go", "func TestGPKV0014() {}")
	write("internal/a.go", "// AHI-001\nconst x = \"UCV0-001\"\n")
	write("conformance/x/fixtures/UCV0-001/file", "opaque")
	write("internal/b_test.go", "AHI-001\x00")
	if err := os.Symlink("a_test.go", filepath.Join(root, "internal/link_test.go")); err != nil {
		t.Fatal(err)
	}
	r, err := scan(root, reqs)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.mentions["GPK-V0-014"].evidence) != 1 || len(r.mentions["AHI-001"].evidence) != 0 || len(r.mentions["AHI-001"].comments) != 1 || len(r.mentions["UCV0-001"].evidence) == 0 || r.binary != 1 || r.symlinks != 1 {
		t.Fatalf("incorrect classification: %+v", r)
	}
}

// TestSRGV0017LoadRequirementsReadsBothLayouts pins SRG-V0-017 for the audit: it reads the
// committed index (no line column, SRG-V0-015) and a line-bearing one, and still refuses a
// malformed row, a bad line, a duplicate id and a foreign header.
func TestSRGV0017LoadRequirementsReadsBothLayouts(t *testing.T) {
	dir := t.TempDir()
	load := func(text string) ([]requirement, error) {
		path := filepath.Join(dir, "REQUIREMENTS.tsv")
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		return loadRequirements(path)
	}
	for _, text := range []string{
		"id\tfile\ttitle\tacceptance\tacceptedBy\nA-V0-001\ta.md\tOne\tunrecorded\t-\n",
		"id\tfile\tline\ttitle\nA-V0-001\ta.md\t3\tOne\n",
	} {
		reqs, err := load(text)
		if err != nil || len(reqs) != 1 || reqs[0] != (requirement{"A-V0-001", "a.md"}) {
			t.Fatalf("load(%q) = %v, %v", text, reqs, err)
		}
	}
	for _, text := range []string{
		"id\tfile\ttitle\tacceptance\tacceptedBy\nA-V0-001\t\tOne\tunrecorded\t-\n",
		"id\tfile\tline\ttitle\nA-V0-001\ta.md\tx\tOne\n",
		"id\tfile\ttitle\nA-V0-001\ta.md\tOne\nA-V0-001\ta.md\tAgain\n",
		"file\tid\ttitle\nA-V0-001\ta.md\tOne\n",
	} {
		if _, err := load(text); err == nil {
			t.Fatalf("load(%q) accepted an invalid index", text)
		}
	}
}
