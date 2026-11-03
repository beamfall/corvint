// SPDX-License-Identifier: Apache-2.0

package console

import "testing"

// TestOCMV0952ChainShowsFourDigitRequirementLines pins that the console's
// clause display reads the same OCM-V0-952 requirement-line grammar.
func TestOCMV0952ChainShowsFourDigitRequirementLines(t *testing.T) {
	for line, want := range map[string]string{
		"- `ASD-V0-2230`: text":   "ASD-V0-2230",
		"- **TCP-V0-2220:** text": "TCP-V0-2220",
		"- `OCM-V0-001`: text":    "OCM-V0-001",
		"- `AB-V0-12`: text":      "",
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
