// SPDX-License-Identifier: Apache-2.0
package main

import (
	"strings"
	"testing"
)

func TestRMVV1008RetiredCommandsRefuse(t *testing.T) {
	for _, command := range []string{"test-plan", "test-validity", "runs"} {
		code, out, err := runCLI(t, command)
		if code != 2 || out != "" || !strings.Contains(err, "invalid choice:") {
			t.Fatalf("%s: code=%d out=%q err=%q", command, code, out, err)
		}
	}
	root := cliRepository(t)
	for _, command := range []string{"lint", "duplicates", "validity"} {
		code, out, err := runCLI(t, "--root", root, "tests", command)
		if code != 2 || out != "" || !strings.Contains(err, "tests-invalid-arguments") {
			t.Fatalf("tests %s: code=%d out=%q err=%q", command, code, out, err)
		}
	}
}
