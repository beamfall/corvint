// SPDX-License-Identifier: Apache-2.0

package platform

import (
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

func TestRSPV0003ShellPartialResultsBoundary(t *testing.T) {
	for _, in := range []tr.Input{
		{Runner: "bats", Stdout: []byte("1..2\nnot ok 1 first\n"), ExitCode: 1},
		{Runner: "bats", Stdout: []byte("1..2\nnot ok 1 first\nfuture-result 2 second\n"), ExitCode: 1},
		{Runner: "shellspec", Stdout: []byte(`<testsuites name="shellspec" tests="2" errors="0" failures="1"><testsuite name="spec.sh" tests="2" errors="0" failures="1" skipped="0"><testcase classname="spec.sh" name="first"><failure message="failed"/></testcase></testsuite></testsuites>`), ExitCode: 101},
	} {
		legacy, err := Parse(in)
		if err == nil || legacy.Complete || len(legacy.Tests) != 1 || legacy.Tests[0].State != tr.Unknown {
			t.Fatalf("legacy observation changed: %+v %v", legacy, err)
		}
		partial, err := ParseShellResults(in)
		if err == nil || partial.Complete || len(partial.Tests) != 1 || partial.Tests[0].State != tr.Failed || len(partial.Problems) == 0 {
			t.Fatalf("partial terminal result lost: %+v %v", partial, err)
		}
	}
	if _, err := ParseShellResults(tr.Input{Runner: "future-shell"}); err == nil {
		t.Fatal("unknown runner admitted")
	}
}
