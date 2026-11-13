// SPDX-License-Identifier: Apache-2.0

//go:build darwin || linux

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/sessiontest"
	"github.com/Beamfall/corvint/internal/testduration"
)

// ptdTimedGo is the session fixture's recording `go` with a slow launch: it sleeps 400 ms, as a
// build would, then reports each named package passing with a test that took 1 ms and a package
// Elapsed of 2 ms.
const ptdTimedGo = `#!/bin/sh
echo "go $*" >> "$SESSION_FAKE_LOG"
case "$1" in
env) echo go1.27.1; exit 0;;
list) echo example.test/m/a; echo example.test/m/b; exit 0;;
esac
sleep 0.4
for arg in "$@"; do
  case "$arg" in example.test/*)
    echo "{\"Action\":\"pass\",\"Package\":\"$arg\",\"Test\":\"TestA\",\"Elapsed\":0.001}"
    echo "{\"Action\":\"pass\",\"Package\":\"$arg\",\"Elapsed\":0.002}";;
  esac
done
`

// PTD-V1-002, PTD-V1-004, PTD-V1-005, PTD-V1-008: a `corvint test` Go package unit records the
// package Elapsed go test -json reports (2 ms, go-test2json, granularity file), never a share of
// the launch time, which still includes the 400 ms the launch spent; the run evidence row the run
// appends copies the duration, its source and its granularity.
func TestPTDV1005GoReceiptDurationAndInvocationTime(t *testing.T) {
	root, base, _ := sessionFixture(t, "pass")
	bin := t.TempDir()
	if err := execfile.WriteFile(filepath.Join(bin, "go"), []byte(ptdTimedGo), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if code, _, stderr := runCLI(t, "--root", root, "test", "--base", base); code != 0 {
		t.Fatalf("corvint test %d %s", code, stderr)
	}
	receipt := readSessionReceipt(t, filepath.Join(root, ".git", "corvint", "test-run.json"))
	if len(receipt.Invocations) != 1 || len(receipt.Invocations[0].Outcomes) != 1 {
		t.Fatalf("receipt %+v", receipt)
	}
	invocation, outcome := receipt.Invocations[0], receipt.Invocations[0].Outcomes[0]
	if outcome.DurationMs == nil || *outcome.DurationMs != 2 || outcome.DurationSource != testduration.SourceGo ||
		outcome.DurationGranularity != testduration.GranularityFile || outcome.Outcome != sessiontest.OutcomePass || len(receipt.DurationInvalid) != 0 {
		t.Fatalf("outcome %+v durationInvalid %v", outcome, receipt.DurationInvalid)
	}
	if invocation.ElapsedMilliseconds < 400 {
		t.Fatalf("the invocation time %d ms lost the launch", invocation.ElapsedMilliseconds)
	}
	snapshot, err := rvdStore(root).Read()
	if err != nil || len(snapshot.Records) != 1 || len(snapshot.Records[0].Rows) != 1 {
		t.Fatalf("store %+v %v", snapshot, err)
	}
	row := snapshot.Records[0].Rows[0]
	if row.DurationMs == nil || *row.DurationMs != 2 || row.DurationSource != testduration.SourceGo || row.DurationGranularity != testduration.GranularityFile {
		t.Fatalf("run evidence row %+v", row)
	}
}
