// SPDX-License-Identifier: Apache-2.0
package cli_test

import (
	"strings"
	"testing"
)

// TCL-V0-7405: help is usable without a source checkout and required flags
// are diagnosed by name, with the same runnable example as the help.
func TestTCLV07405ManualHelpAndMissingFlags(t *testing.T) {
	root := papercutStore(t).Root
	for _, args := range [][]string{{"help"}, {"help", "--verbose"}, {"ticket", "create", "--help", "--verbose"}} {
		x := atm(t, root, nil, args...)
		if strings.Contains(string(x.stdout), "docs/") {
			t.Fatalf("unshipped reference: %s", x.stdout)
		}
	}
	h := atm(t, root, nil, "ticket", "complete-manual", "--help")
	if !strings.Contains(string(h.stdout), "--target") || !strings.Contains(string(h.stdout), "--expected-revision") || !strings.Contains(string(h.stdout), "example") {
		t.Fatal(h.stdout)
	}
	for _, missing := range []string{"--target", "--expected-revision"} {
		args := []string{"ticket", "complete-manual", "--request-id", "manual-missing", "--payload", `{"evidence":[],"reason":"done"}`}
		if missing == "--target" {
			args = append(args, "--expected-revision", "1")
		} else {
			args = append(args, "--target", "V1-1234")
		}
		x := atm(t, root, nil, args...)
		if x.code == 0 || !strings.Contains(string(x.stdout), missing) || !strings.Contains(string(x.stdout), "example: corvint-tasks ticket complete-manual") {
			t.Fatal(x.stdout)
		}
	}
}

// TCL-V0-7405: the warning reports the effective coverage, never a constant.
func TestTCLV07405UnboundedActualCoverage(t *testing.T) {
	root := serialFallbackStore(t)
	for _, coverage := range []string{"UNKNOWN", "INCOMPLETE"} {
		_, x := effectsTicket(t, root, strings.ToLower(coverage), "P1", `{"coverage":"`+coverage+`","externalUnbounded":false,"resources":[],"touchPaths":[]}`)
		warnings := effectsUnbounded(x)
		if len(warnings) != 1 || !strings.Contains(warnings[0], "(coverage "+coverage+", 0 touchPaths, 0 resources)") || strings.Contains(warnings[0], "QUALIFIED") {
			t.Fatalf("%s: %v", coverage, warnings)
		}
	}
}
