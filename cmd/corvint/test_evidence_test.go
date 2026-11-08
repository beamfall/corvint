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
func TestRSPV0051CLIHelpAndInvalid(t *testing.T) {
	for _, args := range [][]string{{"test-evidence", "--help"}, {"help", "test-evidence"}} {
		code, out, diag := runCLI(t, args...)
		if code != 0 || diag != "" || !strings.Contains(out, "--explain") || !strings.Contains(out, "Example:") {
			t.Fatalf("%d %s %s", code, out, diag)
		}
	}
	for _, args := range [][]string{{"test-evidence"}, {"test-evidence", "--explain", "x"}, {"test-evidence", "x", "--lines", "0:0"}} {
		code, _, _ := runCLI(t, args...)
		if code != 2 {
			t.Fatalf("accepted %v", args)
		}
	}
}
