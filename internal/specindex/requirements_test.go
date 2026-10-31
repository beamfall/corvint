package specindex

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestSRGV0015IndexOrderedByDefinitionPosition pins SRG-V0-015 on the live index: no line column,
// and rows ordered by file, then by the position of each definition in that file, so an edit that
// shifts a spec's lines rewrites no row.
func TestSRGV0015IndexOrderedByDefinitionPosition(t *testing.T) {
	text := string(mustRead(t, "../../docs/specs/REQUIREMENTS.tsv"))
	if header, _, _ := strings.Cut(text, "\n"); strings.Contains("\t"+header+"\t", "\tline\t") {
		t.Fatalf("REQUIREMENTS.tsv header %q carries a line column", header)
	}
	specs := map[string]string{}
	previousFile, previousLine := "", 0
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n")[1:] {
		fields := strings.Split(line, "\t")
		id, file := fields[0], fields[1]
		if _, ok := specs[file]; !ok {
			specs[file] = string(mustRead(t, filepath.Join("../..", filepath.FromSlash(file))))
		}
		at := DefinitionLine(specs[file], id)
		switch {
		case file < previousFile:
			t.Fatalf("%s in %s follows a row of %s: rows are not ordered by file", id, file, previousFile)
		case file == previousFile && at < previousLine:
			t.Fatalf("%s (%s:%d) follows a definition at line %d: rows are not in definition order", id, file, at, previousLine)
		}
		previousFile, previousLine = file, at
	}
}

// TestSRGV0017DefinitionLineLocatesEveryRow pins SRG-V0-017 on the live corpus: every committed
// row resolves to a definition line in its spec, and that line holds the row's title.
func TestSRGV0017DefinitionLineLocatesEveryRow(t *testing.T) {
	rows, err := ParseIndex(string(mustRead(t, "../../docs/specs/REQUIREMENTS.tsv")))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 1000 {
		t.Fatalf("parsed %d rows from the live index, want the whole corpus", len(rows))
	}
	specs := map[string][]string{}
	for id, row := range rows {
		lines, ok := specs[row.File]
		if !ok {
			lines = strings.Split(string(mustRead(t, filepath.Join("../..", filepath.FromSlash(row.File)))), "\n")
			specs[row.File] = lines
		}
		at := DefinitionLine(strings.Join(lines, "\n"), id)
		if at < 1 || at > len(lines) {
			t.Errorf("%s: no definition line in %s", id, row.File)
			continue
		}
		collapsed := strings.Join(strings.FieldsFunc(lines[at-1], func(r rune) bool { return r == '\t' || r == '\r' }), " ")
		if !strings.Contains(collapsed, row.Title) {
			t.Errorf("%s: %s:%d %q does not hold the title %q", id, row.File, at, lines[at-1], row.Title)
		}
	}
}

// TestSRGV0017ParseIndexReadsBothLayouts pins SRG-V0-017: a reader parses the committed layout and
// the line-bearing one (the `--lines` view, or an index pinned before SRG-V0-015), and refuses
// text that is not an index.
func TestSRGV0017ParseIndexReadsBothLayouts(t *testing.T) {
	committed, err := ParseIndex("id\tfile\ttitle\tacceptance\tacceptedBy\nA-V0-001\tdocs/specs/a.md\tOne\tstanding\tV1-0001\n\tdocs/specs/a.md\tNo id\tunrecorded\t-\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := (Row{ID: "A-V0-001", File: "docs/specs/a.md", Title: "One", Acceptance: "standing", AcceptedBy: "V1-0001"}); len(committed) != 1 || committed["A-V0-001"] != want {
		t.Fatalf("committed layout parsed %+v, want only %+v", committed, want)
	}
	legacy, err := ParseIndex("id\tfile\tline\ttitle\nA-V0-001\tdocs/specs/a.md\t7\tOne\nB-V0-001\tdocs/specs/b.md\tx\tBad line\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := (Row{ID: "A-V0-001", File: "docs/specs/a.md", Line: 7, Title: "One"}); len(legacy) != 1 || legacy["A-V0-001"] != want {
		t.Fatalf("line-bearing layout parsed %+v, want only %+v", legacy, want)
	}
	for _, text := range []string{"", "id\ttitle\n", "file\tid\ttitle\n", "id\tfile\tline\n"} {
		if _, err := ParseIndex(text); err != ErrHeader {
			t.Errorf("ParseIndex(%q) error %v, want ErrHeader", text, err)
		}
	}
}

// TestSRGV0017DefinitionLineFollowsGeneratorRanks pins SRG-V0-017 against SRG-V0-003/004: a
// definition beats an earlier table row, a table row is the fallback, a wrapped prose continuation
// and a passing mention never count, and a longer id sharing the prefix is a different id. A marked
// line that wraps prose ranks after every unwrapped definition and is only a fallback (SRG-V0-548).
func TestSRGV0017DefinitionLineFollowsGeneratorRanks(t *testing.T) {
	spec := strings.Join([]string{
		"# Spec",                             // 1
		"",                                   // 2
		"| X-V0-001 | a table row first |",   // 3
		"| `T-V0-001` | only in a table |",   // 4
		"",                                   // 5
		"Prose that wraps and the next line", // 6
		"X-V0-002: is a continuation, not a definition.", // 7
		"",                                     // 8
		"- `X-V0-001`: the marked definition.", // 9
		"**X-V0-003.** a bold period definition.", // 10
		"X-V0-004-005: a longer id.",              // 11
		"",                                        // 12
		"X-V0-002: a paragraph-start definition.",                 // 13
		"See X-V0-005 in passing.",                                // 14
		"`X-V0-006`. a marked but wrapped citation (SRG-V0-548).", // 15
		"`X-V0-007`: only ever a wrapped citation.",               // 16
		"",                                    // 17
		"X-V0-006. the unwrapped definition.", // 18
	}, "\n")
	for id, want := range map[string]int{"X-V0-001": 9, "T-V0-001": 4, "X-V0-002": 13, "X-V0-003": 10, "X-V0-004": 0, "X-V0-005": 0, "X-V0-006": 18, "X-V0-007": 16} {
		if got := DefinitionLine(spec, id); got != want {
			t.Errorf("DefinitionLine(%s) = %d, want %d", id, got, want)
		}
	}
}
