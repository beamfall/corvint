// SPDX-License-Identifier: Apache-2.0

//go:build darwin || linux

package sessiontest

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/testduration"
)

// PTD-V1-002, PTD-V1-003 (V1-5542, per-test-durations task 7): Corvint's
// Minitest reporter writes each test's DurationMs from
// Minitest::Result#time and internal/testrunner/dynamic reads it as
// minitest-reporter (TestPTDV1002MinitestReporterDuration). The contract
// has no Minitest selection (RNC-V1-004, runner-not-implemented), so a
// `corvint test` receipt runs Minitest through the instance's full command,
// whose output it does not read: the Minitest outcome is NOT_MEASURED with
// no durationMs, never a time estimated from the launch.
func TestPTDV1003MinitestFullCommandIsNotMeasured(t *testing.T) {
	runner, m, _, dir := runnerFixture(t, "pass")
	script := filepath.Join(dir, "fake-minitest")
	if err := execfile.WriteFile(script, []byte("#!/bin/sh\n[ \"$1\" = --version ] && { echo rake 13.2.1; exit 0; }\necho '3 runs, 3 assertions, 0 failures'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner.Declared = BuiltInDeclared()
	runner.Declared.Instances = []RunnerCommand{{Key: "minitest@.", Argv: []string{script}}}
	m.Units = append(m.Units, Unit{Adapter: AdapterDeclared, Shard: 0, Slice: SliceWhole, Unit: "declared:minitest@."})
	digest, err := Digest(m)
	if err != nil {
		t.Fatal(err)
	}
	receipt := runner.Execute(context.Background(), m, digest, 0)
	if receipt.Result != ResultPass {
		t.Fatalf("receipt %s incomplete %q invocations %+v", receipt.Result, receipt.Incomplete, receipt.Invocations)
	}
	found := false
	for _, invocation := range receipt.Invocations {
		for _, outcome := range invocation.Outcomes {
			if outcome.Unit != "declared:minitest@." {
				continue
			}
			found = true
			if outcome.Outcome != OutcomePass || outcome.DurationSource != testduration.NotMeasured || outcome.DurationMs != nil || invocation.ElapsedMilliseconds < 0 {
				t.Fatalf("minitest outcome %+v in %+v", outcome, invocation)
			}
		}
	}
	if !found {
		t.Fatalf("no minitest outcome in %+v", receipt.Invocations)
	}
}
