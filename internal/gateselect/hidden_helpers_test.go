// SPDX-License-Identifier: Apache-2.0
package gateselect

import "testing"

// AZU-V0-9920: the gate selector must admit the same scoped helpers as the
// affected graph and retain their source/import edges. Other hidden
// directories are indexed Go source too (AZU-V0-005), so they must parse;
// a nested module stays outside the root module's index.
func TestHiddenHelperScopes_AZU_V0_9920(t *testing.T) {
	root := writeFixture(t, map[string]string{
		readScopesPath:             `{"profile":"corvint-test-read-scopes/0","packages":{".github/helper":[]}}`,
		".github/helper/h.go":      "package h\n",
		".github/helper/h_test.go": "package h\nimport \"os\"\nvar _, _ = os.Getwd()\n",
		".github/caller/c.go":      "package c\nimport _ \"example.com/fixture/.github/helper\"\n",
		".github/.hidden/h.go":     "package hidden\n",
		"outer/.github/h.go":       "package outer\n",
		".github/nested/go.mod":    "module example.com/nested\n",
		".github/nested/n.go":      "not Go source\n",
		// Declared output in a hidden directory is not a package even with a
		// Go suffix (AZU-V0-9921), so a retained dump cannot fail indexing.
		nonSourcePath:            `{"profile":"corvint-non-source-paths/0","entries":[{"path":".agent-evidence/","reason":"retained evidence"}]}`,
		".agent-evidence/run.go": "retained output\n",
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
