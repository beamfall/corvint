package main

import (
	"strings"
	"testing"
)

// TestSelectPackagesHonorsDeclaredReadScopes covers AFP-V0-023: a declared
// root locator leaves the unresolved floor and is selected, on a `declared`
// line, only when a dirty path is in its scope; its undeclared dependent stays
// on the floor; any defect in the declaration falls back.
func TestSelectPackagesHonorsDeclaredReadScopes(t *testing.T) {
	files := map[string]string{
		"walker/walker_test.go": "package walker\n\nimport \"runtime\"\n\nvar _, file, _, _ = runtime.Caller(0)\n",
		"other/other_test.go":   "package other\n\nimport \"runtime\"\n\nvar _, file, _, _ = runtime.Caller(0)\n",
		"plain/plain.go":        "package plain\n",
		readScopesPath:          `{"profile":"corvint-test-read-scopes/0","packages":{"walker":["docs/","go.mod"]}}`,
	}
	root := writeFixture(t, files)
	cases := []struct{ dirty, verdict, line string }{
		{"notes/x.md", "run example.com/fixture/other", ""},
		{"docs/a/b.md", "run example.com/fixture/other example.com/fixture/walker", "declared example.com/fixture/walker <- docs/a/b.md"},
		{"docs", "run example.com/fixture/other example.com/fixture/walker", "declared example.com/fixture/walker <- docs"},
		{"go.mod.orig", "run example.com/fixture/other", ""},
		{readScopesPath, "run example.com/fixture/other example.com/fixture/walker", "declared example.com/fixture/walker <- " + readScopesPath},
		{"walker/testdata/x", "run example.com/fixture/other example.com/fixture/walker", "reader example.com/fixture/walker <- walker/testdata/x"},
	}
	for _, tc := range cases {
		t.Run(tc.dirty, func(t *testing.T) {
			var plan receipt
			plan.Provider.Go.State = "RUNNABLE"
			plan.Plan.Dirty = []string{tc.dirty}
			output := selectPackages(plan, fixtureModule, root)
			if got := output[len(output)-1]; got != tc.verdict {
				t.Fatalf("verdict = %q, want %q\n%s", got, tc.verdict, strings.Join(output, "\n"))
			}
			if tc.line != "" && !strings.Contains(strings.Join(output, "\n")+"\n", tc.line+"\n") {
				t.Fatalf("output lacks %q:\n%s", tc.line, strings.Join(output, "\n"))
			}
		})
	}
	for name, declaration := range map[string]string{
		"unknown package": `{"profile":"corvint-test-read-scopes/0","packages":{"nowhere":[]}}`,
		"unknown member":  `{"profile":"corvint-test-read-scopes/0","packages":{},"x":1}`,
		"unsorted":        `{"profile":"corvint-test-read-scopes/0","packages":{"walker":["z","a"]}}`,
		"git entry":       `{"profile":"corvint-test-read-scopes/0","packages":{"walker":[".git/"]}}`,
		"root package":    `{"profile":"corvint-test-read-scopes/0","packages":{".":[]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			files[readScopesPath] = declaration
			root := writeFixture(t, files)
			var plan receipt
			plan.Provider.Go.State = "RUNNABLE"
			plan.Plan.Dirty = []string{"notes/x.md"}
			if got := verdict(plan, root); !strings.HasPrefix(got, "FALLBACK the read-scope declaration is invalid: ") {
				t.Fatalf("verdict = %q, want the invalid-declaration fallback", got)
			}
		})
	}
}
