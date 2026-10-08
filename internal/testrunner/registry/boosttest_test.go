package registry_test

import (
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

// TestBoostTestRegistryDispatch traces TRE-V0-036 and TRE-V0-038 through the
// public registry Build/Parse/Normalize path.
func TestBoostTestRegistryDispatch(t *testing.T) {
	count := 0
	for _, r := range registry.Runners() {
		if r == "boost-test-junit" {
			count++
		}
	}
	if count != 1 {
		t.Fatal(count)
	}
	req := tr.Request{Runner: "boost-test-junit", Executable: "/test", ReportDir: "/fresh", Target: "M", ExpectedTests: []string{"M::s/ok", "M::s/off"}}
	inv, e := registry.Build(req)
	if e != nil || len(inv.Phases) != 0 || inv.Argv[0] != "--log_format=JUNIT" || inv.FailureExitCodes[0] != 201 {
		t.Fatal(inv, e)
	}
	report := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite tests="1" skipped="1" errors="0" failures="0" id="0" name="M" time="1e-05">
<testcase assertions="1" classname="s" name="ok" time="1e-05">
</testcase>
<testcase assertions="0" classname="s" name="off" time="0">
<skipped/>
</testcase>
</testsuite>
`
	in := tr.Input{Runner: req.Runner, Target: req.Target, Expected: req.ExpectedTests, Reports: map[string][]byte{"boost-junit.xml": []byte(report)}, SuccessExitCodes: inv.SuccessExitCodes, FailureExitCodes: inv.FailureExitCodes}
	o, e := registry.Parse(in)
	if e != nil || !o.Complete || len(o.Tests) != 2 || o.Tests[0].ID != "M::s/off" || o.Tests[0].State != tr.Skipped || o.Tests[1].State != tr.Passed {
		t.Fatal(o, e)
	}
	in.ExitCode = 201
	o, e = registry.Parse(in)
	if e != nil || o.Complete || o.Tests[1].State != tr.Unknown {
		t.Fatal(o, e)
	}
	in.ExitCode = 0
	in.Expected = []string{"M::s/ok"}
	o, e = registry.Parse(in)
	if e != nil || o.Complete || o.Tests[0].State != tr.Unknown {
		t.Fatal("surplus skipped case admitted", o, e)
	}
}
