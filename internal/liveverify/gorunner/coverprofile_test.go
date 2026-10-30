package gorunner

import "testing"

// TestParseCoverProfileExportsBlocks pins the parser TCQ-V0-051 reuses: the
// mode line, every block's fields, and refusal of a missing or mixed mode.
func TestParseCoverProfileExportsBlocks(t *testing.T) {
	profile := []byte("mode: count\nexample.com/m/pkg/a.go:9.41,9.72 1 3\nexample.com/m/pkg/b.go:2.1,4.2 2 0\n")
	mode, blocks, err := ParseCoverProfile(profile)
	if err != nil || mode != "count" || len(blocks) != 2 {
		t.Fatalf("mode=%q blocks=%d err=%v", mode, len(blocks), err)
	}
	want := CoverageBlock{ProfilePath: "example.com/m/pkg/a.go", StartLine: 9, StartColumn: 41, EndLine: 9, EndColumn: 72, Statements: 1, Count: 3}
	if blocks[0] != want {
		t.Fatalf("block[0] = %+v, want %+v", blocks[0], want)
	}
	if blocks[1].Count != 0 || blocks[1].EndLine != 4 {
		t.Fatalf("block[1] = %+v", blocks[1])
	}
	for name, raw := range map[string][]byte{
		"empty":      nil,
		"no-mode":    []byte("example.com/m/pkg/a.go:9.41,9.72 1 3\n"),
		"bad-mode":   []byte("mode: lines\n"),
		"mixed-mode": []byte("mode: set\nmode: count\n"),
		"bad-block":  []byte("mode: set\nexample.com/m/pkg/a.go:9.41 1 3\n"),
	} {
		if _, _, err := ParseCoverProfile(raw); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
}
