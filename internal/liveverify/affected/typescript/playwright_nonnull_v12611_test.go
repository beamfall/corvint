package typescript

import (
	"strings"
	"testing"
)

// TJAA-V0-810 (V1-2611): a computed member read or write after a TypeScript non-null assertion
// (`x![k]`, `x()![k]`, `x[0]![k]`) is a member access, so it stops the tag closure at its line;
// a logical not before an array literal (`![a].length`) stays exact.
func TestPlaywrightTagClosureNonNullComputedMember_V12611(t *testing.T) {
	const stop = "e2e/plain.spec.ts holds a tag or construct that is not one literal @\\S+ string (line 3"
	const head = "const details: any = { tag: ['@base'] };\nconst key = 't' + 'ag';\n"
	for _, row := range []struct {
		label, line string
		stops       bool
	}{
		{"write after identifier assertion", "details![key].push('@smoke');\n", true},
		{"read after identifier assertion", "const tags = details![key];\n", true},
		{"after call assertion", "(() => details)()![key].push('@smoke');\n", true},
		{"after index assertion", "[details][0]![key].push('@smoke');\n", true},
		{"spaced and doubled assertion", "details ! ! [key].push('@smoke');\n", true},
		{"logical not of an array literal", "const empty = ![key].length;\n", false},
	} {
		t.Run(row.label, func(t *testing.T) {
			root := grepFixture(t, grepRepro)
			write(t, root, "e2e/plain.spec.ts", head+row.line+"test('plain', async () => {});\n")
			unverified, omissions := gh1647Omission(t, root, grepReproCases())
			if !row.stops {
				if len(unverified) != 0 || len(omissions) != 0 {
					t.Fatalf("not verified: unverified=%+v omissions=%+v", unverified, omissions)
				}
				return
			}
			if len(omissions) != 1 || !strings.Contains(omissions[0].Detail, stop) {
				t.Fatalf("omissions=%+v unverified=%+v, want a stop naming %q", omissions, unverified, stop)
			}
		})
	}
}
