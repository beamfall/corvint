package dogfoodflow

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/lrfrepo"
)

// TestRequirementLayoutMatchesForBothConsumers pins, for one fixture set, the
// spans the advisory coverage reader derives and the strict OCM-V0-001
// verdict internal/lrfrepo returns (V1-4754). Both read the same section,
// fence and requirement-line layout; only their policy differs: coverage
// reads two headings as no section and keeps repeated IDs, invalid UTF-8 and
// an empty section, where the verifier refuses each. The
// multi-carriage-return fixture pins the one line-ending difference: coverage
// strips every trailing carriage return, the verifier strips one. The expected
// values were observed on the code before the shared layout existed.
func TestRequirementLayoutMatchesForBothConsumers(t *testing.T) {
	many := "## Requirements\n"
	for i := 1; i <= lrfrepo.MaxOCMObligations+1; i++ {
		many += fmt.Sprintf("- `ABC-V0-%03d`: a.\n", i)
	}
	for _, test := range []struct {
		name, blob, coverage, verifier string
	}{
		{"lf", "# T\n\n## Requirements\n\n- `ABC-V0-001`: a.\n  more\n- **ABC-V0-002:** b.\n\n## Next\n- `ABC-V0-003`: outside.\n",
			"section 2-8 items ABC-V0-001@4-6/22-48 ABC-V0-002@6-8/48-70 outside ABC-V0-003@9-10/78-103",
			"ABC-V0-001 ABC-V0-002 @5-70"},
		{"crlf", "# T\r\n\r\n## Requirements\r\n\r\n- `ABC-V0-001`: a.\r\n  more\r\n- **ABC-V0-002.** b.\r\n\r\n## Next\r\n- `ABC-V0-003`: outside.\r\n",
			"section 2-8 items ABC-V0-001@4-6/26-54 ABC-V0-002@6-8/54-78 outside ABC-V0-003@9-10/87-113",
			"ABC-V0-001 ABC-V0-002 @7-78"},
		{"utf8 without final newline", "## Requirements\n- `ABC-V0-001`: café ☕.\n- `ABC-V0-002`: b",
			"section 0-3 items ABC-V0-001@1-2/16-43 ABC-V0-002@2-3/43-60 outside",
			"ABC-V0-001 ABC-V0-002 @0-60"},
		{"fenced heading", "```\n## Requirements\n```\n## Requirements\n- `ABC-V0-001`: a.\n",
			"section 3-5 items ABC-V0-001@4-5/40-59 outside",
			"ABC-V0-001 @24-59"},
		{"duplicate headings", "## Requirements\n- `ABC-V0-001`: a.\n## Requirements\n- `ABC-V0-002`: b.\n",
			"section none items outside",
			"invalid-requirements-section"},
		{"duplicate items", "## Requirements\n- `ABC-V0-001`: a.\n- `ABC-V0-001`: again.\n",
			"section 0-3 items ABC-V0-001@1-2/16-35 ABC-V0-001@2-3/35-58 outside",
			"duplicate-requirement"},
		{"no heading", "# T\n- `ABC-V0-001`: a.\ntext\n# H\n- `ABC-V0-002`: b.\n",
			"section none items outside ABC-V0-001@1-3/4-28 ABC-V0-002@4-5/32-51",
			"invalid-requirements-section"},
		{"tilde fence closes only on a longer run", "## Requirements\n~~~~\n## Fenced\n~~~\n- `ABC-V0-001`: still fenced\n~~~~\n- `ABC-V0-002`: b.\n## End\n",
			"section 0-7 items ABC-V0-001@4-6/35-69 ABC-V0-002@6-7/69-88 outside",
			"ABC-V0-001 ABC-V0-002 @0-88"},
		{"backtick info string opens no fence", "## Requirements\n\n- `ABC-V0-001`: a.\n```x`y\n## Next\n- `ABC-V0-002`: b.\n",
			"section 0-4 items ABC-V0-001@2-4/17-43 outside ABC-V0-002@5-6/51-70",
			"ABC-V0-001 @0-43"},
		{"four-space indent opens no fence", "## Requirements\n    ```\n## Next\n- `ABC-V0-001`: x\n",
			"section 0-2 items outside ABC-V0-001@3-4/32-50",
			"missing-requirements"},
		{"unclosed fence runs to the end", "## Requirements\n- `ABC-V0-001`: a.\n```\n## Next\n- `ABC-V0-002`: fenced.\n",
			"section 0-5 items ABC-V0-001@1-4/16-47 ABC-V0-002@4-5/47-71 outside",
			"ABC-V0-001 ABC-V0-002 @0-71"},
		{"bare level-two heading ends the section", "## Requirements\n- `ABC-V0-001`: a.\n##\n- `ABC-V0-002`: b.\n",
			"section 0-2 items ABC-V0-001@1-2/16-35 outside ABC-V0-002@3-4/38-57",
			"ABC-V0-001 @0-35"},
		{"deeper and unspaced headings do not end it", "## Requirements\n### Sub\n##x\n- `ABC-V0-001`: a.\n",
			"section 0-4 items ABC-V0-001@3-4/28-47 outside",
			"ABC-V0-001 @0-47"},
		{"multiple carriage returns", "## Requirements\r\r\n- `ABC-V0-001`: a.\r\n",
			"section 0-2 items ABC-V0-001@1-2/18-38 outside",
			"invalid-requirements-section"},
		{"invalid utf-8", "## Requirements\n- `ABC-V0-001`: \xff\n",
			"section 0-2 items ABC-V0-001@1-2/16-34 outside",
			"invalid-intent"},
		{"empty", "", "section none items outside", "invalid-requirements-section"},
		{"over the obligation bound", many,
			"section 0-258 items 257 outside",
			"too-many-obligations"},
	} {
		if got := coverageLayout(parseRequirements([]byte(test.blob))); got != test.coverage {
			t.Errorf("%s: coverage layout\n got %q\nwant %q", test.name, got, test.coverage)
		}
		if got := verifierLayout([]byte(test.blob)); got != test.verifier {
			t.Errorf("%s: verifier layout\n got %q\nwant %q", test.name, got, test.verifier)
		}
	}
}

func coverageLayout(section requirementSection) string {
	items := func(list []requirementItem) string {
		if len(list) > 8 {
			return fmt.Sprintf(" %d", len(list))
		}
		var b strings.Builder
		for _, item := range list {
			fmt.Fprintf(&b, " %s@%d-%d/%d-%d", item.id, item.first, item.end, item.start, item.stop)
		}
		return b.String()
	}
	bounds := "none"
	if section.first >= 0 {
		bounds = fmt.Sprintf("%d-%d", section.first, section.end)
	}
	return "section " + bounds + " items" + items(section.items) + " outside" + items(section.outside)
}

func verifierLayout(blob []byte) string {
	if code := lrfrepo.IntentRequirementsRefusal(blob); code != "" {
		return code
	}
	ids, start, end, _, err := lrfrepo.IntentRequirements(blob)
	if err != nil {
		return "error " + err.Error()
	}
	return fmt.Sprintf("%s @%d-%d", strings.Join(ids, " "), start, end)
}
