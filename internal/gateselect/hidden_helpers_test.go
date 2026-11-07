// SPDX-License-Identifier: Apache-2.0
package gateselect

import "testing"

// AZU-V0-9920: the gate selector must admit the same scoped helpers as the
// affected graph and retain their source/import edges without widening the
// exception to other hidden directories or nested modules.
func TestHiddenHelperScopes_AZU_V0_9920(t *testing.T) {
	root := writeFixture(t, map[string]string{
		readScopesPath:             `{"profile":"corvint-test-read-scopes/0","packages":{".github/helper":[]}}`,
		".github/helper/h.go":      "package h\n",
		".github/helper/h_test.go": "package h\nimport \"os\"\nvar _, _ = os.Getwd()\n",
		".github/caller/c.go":      "package c\nimport _ \"example.com/fixture/.github/helper\"\n",
		".github/.hidden/h.go":     "not Go source\n",
		"outer/.github/h.go":       "not Go source\n",
		".github/nested/go.mod":    "module example.com/nested\n",
		".github/nested/n.go":      "not Go source\n",
	})
	for _, tc := range []struct{ dirty, want string }{
		{"notes/unrelated.md", "FALLBACK the selection is empty while 1 path(s) are dirty"},
		{".github/helper/h_test.go", "run example.com/fixture/.github/helper"},
		{".github/helper/h.go", "run example.com/fixture/.github/caller example.com/fixture/.github/helper"},
	} {
		t.Run(tc.dirty, func(t *testing.T) {
			var plan Receipt
			plan.Provider.Go.State = "RUNNABLE"
			plan.Plan.Dirty = []string{tc.dirty}
			if got := verdict(plan, root); got != tc.want {
				t.Fatalf("verdict %q; want %q", got, tc.want)
			}
		})
	}
}
