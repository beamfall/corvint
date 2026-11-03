package gateselect

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
		"walker/walker_test.go": "package walker\n\nimport \"os\"\n\nvar _, _ = os.Getwd()\n",
		"other/other_test.go":   "package other\n\nimport \"os\"\n\nvar _, _ = os.Getwd()\n",
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
			var plan Receipt
			plan.Provider.Go.State = "RUNNABLE"
			plan.Plan.Dirty = []string{tc.dirty}
			output := selectPackages(plan, fixtureModule, root, true)
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
			var plan Receipt
			plan.Provider.Go.State = "RUNNABLE"
			plan.Plan.Dirty = []string{"notes/x.md"}
			if got := verdict(plan, root); !strings.HasPrefix(got, "FALLBACK the read-scope declaration is invalid: ") {
				t.Fatalf("verdict = %q, want the invalid-declaration fallback", got)
			}
		})
	}
}

// TestSelectBoundsDeclaredPathReaders covers TSP-V0-980: a declared package
// whose test names a path is no rule (c) reader of a dirty path outside its
// read scope, while an undeclared package naming the same path still is, and
// a dirty path inside the scope still selects the declared package.
func TestSelectBoundsDeclaredPathReaders(t *testing.T) {
	files := map[string]string{
		"bounded/bounded_test.go": "package bounded\n\nvar _ = []string{\"notes/\", \"docs/\"}\n",
		"open/open_test.go":       "package open\n\nvar _ = \"notes/\"\n",
		readScopesPath:            `{"profile":"corvint-test-read-scopes/0","packages":{"bounded":["docs/"]}}`,
	}
	root := writeFixture(t, files)
	cases := []struct{ dirty, verdict, line, absent string }{
		{"notes/x.md", "run example.com/fixture/open", "reader example.com/fixture/open <- notes/x.md", "example.com/fixture/bounded"},
		{"docs/a.md", "run example.com/fixture/bounded", "example.com/fixture/bounded <- docs/a.md", ""},
	}
	for _, tc := range cases {
		t.Run(tc.dirty, func(t *testing.T) {
			var plan Receipt
			plan.Provider.Go.State = "RUNNABLE"
			plan.Plan.Dirty = []string{tc.dirty}
			output := strings.Join(selectPackages(plan, fixtureModule, root, true), "\n")
			if !strings.HasSuffix(output, "\n"+tc.verdict) {
				t.Fatalf("verdict is not %q:\n%s", tc.verdict, output)
			}
			if !strings.Contains(output+"\n", tc.line+"\n") {
				t.Fatalf("output lacks %q:\n%s", tc.line, output)
			}
			if tc.absent != "" && strings.Contains(output, tc.absent) {
				t.Fatalf("output names %q:\n%s", tc.absent, output)
			}
			// The bounds gate-ledger hashes agree with selection: a path that
			// selects no declared package is no bound of it.
			lines, err := packageBounds(fixtureModule, root, []string{tc.dirty})
			if err != nil {
				t.Fatal(err)
			}
			bounds := strings.Join(lines, "")
			if tc.absent != "" && strings.Contains(bounds, "bound "+tc.absent+" ") {
				t.Fatalf("bounds name %q:\n%s", tc.absent, bounds)
			}
			if pkg := strings.Fields(tc.verdict)[1]; !strings.Contains(bounds, "bound "+pkg+" ") {
				t.Fatalf("bounds lack %q:\n%s", pkg, bounds)
			}
		})
	}
}
