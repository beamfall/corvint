// SPDX-License-Identifier: Apache-2.0

package specindex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIndexExplicitRequirementPrefix pins the header/index consistency check
// (SRG-V0-045), including the drift found in the AFP-V0 companion spec.
func TestIndexExplicitRequirementPrefix(t *testing.T) {
	for _, tc := range []struct {
		name, declaration, want string
	}{
		{"plain match", "Requirement prefix: FIX-V0", ""},
		{"backticked match", "Requirement prefix: `FIX-V0`", ""},
		{"bold match", "- **Requirement prefix:** `FIX-V0`", ""},
		{"mismatch", "Requirement prefix: `AFP-V0`", `docs/specs/fixture-v0.md reqPrefix "FIX-V0" != header "AFP-V0"`},
		{"no explicit header", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claim := "The fixture specification keeps its index claim, Agent digest and README row in agreement."
			root := indexFixture(t, claim, claim)
			path := filepath.Join(root, "docs/specs/fixture-v0.md")
			body := string(mustRead(t, path))
			body = strings.Replace(body, "# Fixture V0\n", "# Fixture V0\n"+tc.declaration+"\n", 1)
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			var findings []string
			checkIndex(root, func(format string, args ...any) {
				findings = append(findings, fmt.Sprintf(format, args...))
			})
			if tc.want == "" {
				if len(findings) != 0 {
					t.Fatalf("unexpected findings: %q", findings)
				}
			} else if len(findings) != 1 || findings[0] != tc.want {
				t.Fatalf("findings %q, want exactly [%q]", findings, tc.want)
			}
		})
	}
}
