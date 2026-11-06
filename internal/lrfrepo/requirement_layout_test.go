package lrfrepo

import (
	"fmt"
	"strings"
	"testing"
)

// TestScanRequirementLayoutReadsFencesHeadingsAndItems pins the shared
// layout both the OCM verifier and the dogfood coverage reader consume
// (V1-4754): fence rules, every unfenced heading, the section end and the
// requirement lines, duplicates kept, without any refusal.
func TestScanRequirementLayoutReadsFencesHeadingsAndItems(t *testing.T) {
	for _, test := range []struct {
		name, doc, want string
	}{
		{"section", "# T\n## Requirements\n- `ABC-V0-001`: a.\ntext\n- **ABC-V0-002.** b.\n## Next\n- `ABC-V0-003`: c.\n",
			"fenced ....... headings 1 end 5 items ABC-V0-001@2 ABC-V0-002@4"},
		{"no heading", "- `ABC-V0-001`: a.\n", "fenced . headings end 1 items"},
		{"two headings", "## Requirements\n- `ABC-V0-001`: a.\n## Requirements\n", "fenced ... headings 0 2 end 3 items"},
		{"fenced heading and items", "~~~\n## Requirements\n~~~\n## Requirements\n```\n- `ABC-V0-001`: a.\n## Fenced\n```\n- `ABC-V0-001`: again.\n",
			"fenced FFF.FFFF. headings 3 end 9 items ABC-V0-001@5 ABC-V0-001@8"},
		{"backtick info string opens no fence", "## Requirements\n```x`y\n## End\n", "fenced ... headings 0 end 2 items"},
		{"four-space indent opens no fence", "## Requirements\n    ~~~\n##\n", "fenced ... headings 0 end 2 items"},
		{"closing run must be as long", "## Requirements\n````\n```\n- `ABC-V0-001`: a.\n```` \t\n## End\n",
			"fenced .FFFF. headings 0 end 5 items ABC-V0-001@3"},
		{"unclosed fence", "## Requirements\n```\n## End\n", "fenced .FF headings 0 end 3 items"},
		{"deeper headings do not end it", "## Requirements\n### Sub\n##x\n- `ABC-V0-001`: a.\n", "fenced .... headings 0 end 4 items ABC-V0-001@3"},
	} {
		lines := strings.Split(strings.TrimSuffix(test.doc, "\n"), "\n")
		layout := ScanRequirementLayout(len(lines), func(number int) []byte { return []byte(lines[number]) })
		var b strings.Builder
		b.WriteString("fenced ")
		for _, fenced := range layout.Fenced {
			b.WriteString(map[bool]string{true: "F", false: "."}[fenced])
		}
		b.WriteString(" headings")
		for _, heading := range layout.Headings {
			fmt.Fprintf(&b, " %d", heading)
		}
		fmt.Fprintf(&b, " end %d items", layout.End)
		for _, item := range layout.Items {
			fmt.Fprintf(&b, " %s@%d", item.ID, item.Line)
		}
		if got := b.String(); got != test.want {
			t.Errorf("%s: layout\n got %q\nwant %q", test.name, got, test.want)
		}
	}
}

// TestRequirementLineIDReadsTheOCMLineSyntax pins the requirement-line
// grammar (OCM-V0-001 with the OCM-V0-952 ID).
func TestRequirementLineIDReadsTheOCMLineSyntax(t *testing.T) {
	for line, want := range map[string]string{
		"- `ABC-V0-001`: a":        "ABC-V0-001",
		"- `ABC-V0-1234`: a":       "ABC-V0-1234",
		"- **ABC-V0-001:** a":      "ABC-V0-001",
		"- **ABC-V0-001.** a":      "ABC-V0-001",
		"- **ABC-V0-001.** a:** b": "ABC-V0-001",
		"- `ABC-V0-001`:":          "",
		"- `ABC-V0-001` a":         "",
		"- `A-001`: a":             "",
		"- `ABC-V0-01`: a":         "",
		"- `abc-V0-001`: a":        "",
		"  - `ABC-V0-001`: a":      "",
		"* `ABC-V0-001`: a":        "",
		"- **ABC-V0-001** a":       "",
		"":                         "",
	} {
		if got := RequirementLineID([]byte(line)); got != want {
			t.Errorf("RequirementLineID(%q) = %q, want %q", line, got, want)
		}
	}
}
