// SPDX-License-Identifier: Apache-2.0

package dogfoodflow

import "testing"

// TestOCMV0952CoverageReadsFourDigitRequirementLines pins that the coverage
// reader uses the OCM-V0-952 requirement-line grammar of the intent parser.
func TestOCMV0952CoverageReadsFourDigitRequirementLines(t *testing.T) {
	for line, want := range map[string]string{
		"- `ASD-V0-2230`: text":   "ASD-V0-2230",
		"- **TCP-V0-2220:** text": "TCP-V0-2220",
		"- **TCP-V0-2221.** text": "TCP-V0-2221",
		"- `OCM-V0-001`: text":    "OCM-V0-001",
		"- `AB-V0-12`: text":      "",
		"- `ASD-V0-2230` text":    "",
	} {
		got := ""
		if match := requirementLine.FindStringSubmatch(line); match != nil {
			got = match[1] + match[2]
		}
		if got != want {
			t.Errorf("requirementLine(%q) = %q, want %q", line, got, want)
		}
	}
}
