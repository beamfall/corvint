// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestTreeScopeSamplesAndReasons_AFP_V0_3302(t *testing.T) {

	for _, c := range []struct {
		name, entry, want string
		noReason          bool
	}{
		{name: "covered", entry: `{"reason":"pkg/p/p_test.go walks internal/","contentPredicate":"forbidden","samples":["import forbidden"]}`},
		{name: "listing", entry: `{"reason":"pkg/p/p_test.go walks internal/","listingOnly":true}`},
		{name: "uncovering", entry: `{"reason":"pkg/p/p_test.go walks internal/","contentPredicate":"other","samples":["import forbidden"]}`, want: "does not cover sample"},
		{name: "one sample missed", entry: `{"reason":"pkg/p/p_test.go walks internal/","contentPredicate":"forbidden","samples":["forbidden","missed"]}`, want: "does not cover sample"},
		{name: "empty match", entry: `{"reason":"pkg/p/p_test.go walks internal/","contentPredicate":".*","samples":["forbidden"]}`, want: "matches the empty string"},
		{name: "bad regex", entry: `{"reason":"pkg/p/p_test.go walks internal/","contentPredicate":"[","samples":["forbidden"]}`, want: "does not compile"},
		{name: "missing predicate", entry: `{"reason":"pkg/p/p_test.go walks internal/"}`, want: "contentPredicate is missing"},
		{name: "missing samples", entry: `{"reason":"pkg/p/p_test.go walks internal/","contentPredicate":"forbidden"}`, want: "needs positive sample lines"},
		{name: "ambiguous", entry: `{"reason":"pkg/p/p_test.go walks internal/","contentPredicate":"forbidden","samples":["forbidden"],"listingOnly":true}`, want: "listingOnly cannot carry"},
		{name: "predicate needs reason", entry: `{"reason":"pkg/p/p_test.go walks internal/","contentPredicate":"forbidden","samples":["forbidden"]}`, noReason: true, want: "gives no reason"},
		{name: "listing needs reason", entry: `{"reason":"pkg/p/p_test.go walks internal/","listingOnly":true}`, noReason: true, want: "gives no reason"},
	} {
		t.Run(c.name, func(t *testing.T) {
			files := map[string]string{
				"pkg/p/p_test.go":                "package p\nimport \"testing\"\nfunc TestRead(t *testing.T) {}\n",
				"internal/a/a.go":                "package a\n",
				".corvint/test-read-scopes.json": `{"profile":"corvint-test-read-scopes/0","packages":{"pkg/p":["internal/"]}}`,
			}
			if !c.noReason {
				files[treesPath] = `{"profile":"corvint-test-read-scope-trees/0","trees":{"pkg/p":{"internal/":` + c.entry + `}}}`
			}
			var stdout, stderr bytes.Buffer
			code := run(writeRepository(t, files), &stdout, &stderr)
			output := stdout.String() + stderr.String()
			if c.want == "" {
				if code != 0 {
					t.Fatalf("valid entry rejected: %d %s", code, output)
				}
				return
			}
			if code == 0 || !strings.Contains(output, c.want) || !strings.Contains(output, "pkg/p") || !strings.Contains(output, "internal/") {
				t.Fatalf("unclear refusal: %d %s; want %s", code, output, c.want)
			}
		})
	}
}
