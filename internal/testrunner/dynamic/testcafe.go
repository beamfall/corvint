package dynamic

import (
	"bytes"
	"regexp"
	"strings"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// reconcileNativeOutput applies the checks that read native output beside the reports.
func reconcileNativeOutput(in tr.Input, o *tr.Observation) {
	reconcileMochaSelection(in, o)
	if in.Runner == "testcafe" {
		if line := testCafeHelperFailure(in.Stdout, in.Stderr); line != "" {
			problem(o, "browser-helper-failed", line)
		}
	}
}

// testCafeErrno is a Node system-error code as TestCafe prints it ("EEXIST: file already exists").
var testCafeErrno = regexp.MustCompile(`\bE[A-Z]{3,}:`)

// testCafeHelperFailure returns the first native output line in which TestCafe's per-HOME browser
// helper (`.testcafe-browser-tools`) failed with a Node system error, bounded to 240 bytes, or "".
// TRE-V0-114 (V1-0605): such a run never yields a complete observation, whatever its report says.
func testCafeHelperFailure(outputs ...[]byte) string {
	for _, output := range outputs {
		for _, line := range bytes.Split(output, []byte("\n")) {
			if bytes.Contains(line, []byte(".testcafe-browser-tools")) && testCafeErrno.Match(line) {
				line = bytes.TrimSpace(line)
				if len(line) > 240 {
					line = line[:240]
				}
				return strings.ToValidUTF8(string(line), "")
			}
		}
	}
	return ""
}
