// SPDX-License-Identifier: Apache-2.0
package main

import (
	"strings"
	"testing"
)

func TestRSPV0051CLIReadAndNoWrite(t *testing.T) {
	root := cliRepository(t)
	for _, args := range [][]string{{"file.go"}, {"file.go", "--lines", "1:2"}, {"--test", "suite/test"}, {"--explain", "suite/test", "--base", "HEAD"}, {"file.go", "--format", "text"}, {"file.go", "--revision", "missing"}} {
		before := staticSnapshot(t, root) + treeDigest(t, root)
		code, out, diag := runCLI(t, append([]string{"--root", root, "test-evidence"}, args...)...)
		if code != 0 || diag != "" || !strings.Contains(out, "UNKNOWN") {
			t.Fatalf("%v: %d %s %s", args, code, out, diag)
		}
		if after := staticSnapshot(t, root) + treeDigest(t, root); after != before {
			t.Fatal("read mutated repository")
		}
	}
}
