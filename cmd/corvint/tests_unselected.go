package main

import (
	"io"

	"github.com/Beamfall/corvint/internal/testsource"
)

// runTestsUnselected prints the corvint-test-file-selection/0 report (TSA-V0-145): exit 1 when a
// test-looking file is TEST-FILE-UNSELECTED, 0 otherwise, 2 on refusal.
func runTestsUnselected(repo testsource.Repo, args []string, out, diagnostic io.Writer) int {
	f, once := testsFlags("tests unselected")
	options := testsource.UnselectedOptions{Revision: "HEAD"}
	once("rev", &options.Revision)
	if err := f.Parse(args); err != nil || f.NArg() != 0 {
		return testsRefuse(diagnostic, "tests-invalid-arguments", "usage: corvint tests unselected [--rev REV]")
	}
	report, err := testsource.UnselectedFiles(repo, options)
	if err != nil {
		return testsFailure(diagnostic, err)
	}
	if code := testsWrite(out, diagnostic, report); code != 0 {
		return code
	}
	if len(report.Findings) > 0 {
		return 1
	}
	return 0
}
