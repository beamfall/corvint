// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"slices"
	"testing"
)

// AZU-V0-005 and AFP-V0-023: explicitly addressed hidden Go packages
// participate in attribution and read scopes without admitting metadata,
// fixtures or nested modules into the root module's index.
func TestAZUV0005HiddenPackageSelection(t *testing.T) {
	root := writeFixture(t, map[string]string{
		".github/hidden/hidden.go":      "package hidden\n",
		"_private/private.go":           "package private\n",
		"client/client.go":              "package client\nimport _ \"example.com/fixture/.github/hidden\"\n",
		".git/fake.go":                  "invalid metadata\n",
		".hg/fake.go":                   "invalid metadata\n",
		".svn/fake.go":                  "invalid metadata\n",
		"testdata/.hidden/fake.go":      "invalid fixture\n",
		".hidden/nested/go.mod":         "module example.com/nested\n",
		".hidden/nested/fake.go":        "invalid nested source\n",
		".github/hidden/hidden_test.go": "package hidden\n",
		readScopesPath:                  `{"profile":"corvint-test-read-scopes/0","packages":{".github/hidden":["docs/"],"_private":[]}}`,
	})
	index, err := indexRepository(root, fixtureModule)
	if err != nil {
		t.Fatal(err)
	}
	if got := sortedKeys(index.packages); !slices.Equal(got, []string{".github/hidden", "_private", "client"}) {
		t.Fatalf("indexed packages = %v", got)
	}
	for _, tc := range []struct{ dirty, want string }{
		{".github/hidden/hidden.go", "run example.com/fixture/.github/hidden example.com/fixture/client"},
		{".github/hidden/hidden_test.go", "run example.com/fixture/.github/hidden"},
		{"_private/private.go", "run example.com/fixture/_private"},
		{"docs/guide.md", "run example.com/fixture/.github/hidden"},
	} {
		var plan Receipt
		plan.Provider.Go.State = "RUNNABLE"
		plan.Plan.Dirty = []string{tc.dirty}
		if got := verdict(plan, root); got != tc.want {
			t.Fatalf("%s: verdict = %q, want %q", tc.dirty, got, tc.want)
		}
	}
}
