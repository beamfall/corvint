package platform

import (
	"bytes"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"strings"
	"testing"
)

func TestSwiftPMNativeXCTestRetainsSkipAndFailureEvents(t *testing.T) {
	in := tr.Input{Runner: "swift-xctest", Stdout: fixture(t, "swift-xctest-matrix.txt"), ExitCode: 1}
	o, e := Parse(in)
	if e != nil || !o.Complete || len(o.Tests) != 4 {
		t.Fatalf("%+v %v", o, e)
	}
	states, kinds := map[string]int{}, map[string]int{}
	for _, row := range o.Tests {
		states[row.State]++
		kinds[row.FailureKind]++
		if len(row.Attempts) != 0 {
			t.Fatal("invented retry attempt")
		}
		if row.ID == "ProofTests.Proof/testFail" && !strings.Contains(row.Message, "CEM_SECOND") {
			t.Fatal("second assertion lost")
		}
	}
	if states[tr.Passed] != 1 || states[tr.Failed] != 2 || states[tr.Skipped] != 1 || kinds[tr.Assertion] != 1 || kinds[tr.Infrastructure] != 1 {
		t.Fatalf("%v %v", states, kinds)
	}
	for _, bad := range [][]byte{in.Stdout[:len(in.Stdout)/2], bytes.Replace(in.Stdout, []byte("3 failures (1 unexpected)"), []byte("2 failures (1 unexpected)"), 1), bytes.Replace(in.Stdout, []byte("testSkip]' skipped"), []byte("testSkip]' passed"), 1), bytes.Replace(in.Stdout, []byte("Test Suite 'Proof' failed"), []byte("Test Suite 'Proof' passed"), 1)} {
		in.Stdout = bad
		o, e = Parse(in)
		if e == nil && o.Complete {
			t.Fatal("contradictory XCTest protocol admitted")
		}
	}
}
func TestSwiftPMNativeXCTestPassAndZero(t *testing.T) {
	o, e := Parse(tr.Input{Runner: "swift-xctest", Stdout: fixture(t, "swift-xctest-pass.txt"), ExitCode: 0})
	if e != nil || !o.Complete || len(o.Tests) != 1 || o.Tests[0].ID != "ProofTests.Proof/testPass" {
		t.Fatalf("%+v %v", o, e)
	}
	o, e = Parse(tr.Input{Runner: "swift-xctest", Stdout: fixture(t, "swift-xctest-zero.txt"), ExitCode: 0})
	if e == nil && o.Complete {
		t.Fatal("native no-selected-tests output became pass")
	}
}
func TestSwiftPMXCTestBuildPinsPackageAndExactSelector(t *testing.T) {
	r := tr.Request{Runner: "swift-xctest", Root: "/source", Project: "pkg", Config: "/source/pkg/Package.swift", ConfigSha256: strings.Repeat("a", 64), Executable: "/tools/swift", ExecutableSha256: strings.Repeat("b", 64), ReportDir: "/fresh", Selectors: []string{"ProofTests.Proof/testPass"}}
	v, e := Build(r)
	if e != nil || !strings.Contains(strings.Join(v.Argv, " "), "--disable-swift-testing --no-parallel --disable-automatic-resolution --skip-update") {
		t.Fatalf("%+v %v", v, e)
	}
	r.Selectors = []string{"Proof/testPass"}
	if _, e = Build(r); e == nil {
		t.Fatal("short selector admitted")
	}
}

func TestSwiftPMSetupAndBuildFailures(t *testing.T) {
	o, e := Parse(tr.Input{Runner: "swift-xctest", Stdout: fixture(t, "swift-xctest-setup.txt"), ExitCode: 1})
	if e != nil || !o.Complete || len(o.Tests) != 1 || o.Tests[0].State != tr.Failed || o.Tests[0].FailureKind != tr.Infrastructure {
		t.Fatalf("%+v %v", o, e)
	}
	o, e = Parse(tr.Input{Runner: "swift-xctest", Stdout: fixture(t, "swift-xctest-build-error.txt"), ExitCode: 1})
	if e == nil && o.Complete {
		t.Fatal("compiler failure became a complete test observation")
	}
}
