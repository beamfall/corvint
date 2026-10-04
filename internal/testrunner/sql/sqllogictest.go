package sql

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

var sqliteSummary = regexp.MustCompile(`^(0|[1-9][0-9]*) errors out of (0|[1-9][0-9]*) tests in (.+) - (0|[1-9][0-9]*) skipped\.$`)

func parseSQLite(in tr.Input, o *tr.Observation) error {
	if len(in.Stderr) == 0 {
		problem(o, "MISSING_NATIVE_SUMMARY", "SQLite sqllogictest did not retain its aggregate summary")
		return nil
	}
	if in.Stderr[len(in.Stderr)-1] != '\n' {
		return fmt.Errorf("incomplete SQLite summary line")
	}
	lines := strings.Split(strings.TrimSuffix(string(in.Stderr), "\n"), "\n")
	m := sqliteSummary.FindStringSubmatch(lines[len(lines)-1])
	if m == nil {
		problem(o, "MISSING_NATIVE_SUMMARY", string(in.Stderr))
		return nil
	}
	for _, line := range lines[:len(lines)-1] {
		if sqliteSummary.MatchString(line) {
			return fmt.Errorf("duplicate SQLite native summary")
		}
	}
	counts := make([]int, 3)
	for i, k := range []int{1, 2, 4} {
		n, e := strconv.Atoi(m[k])
		if e != nil || n > tr.MaxTests {
			return fmt.Errorf("SQLite native count bound")
		}
		counts[i] = n
	}
	errors, executed, skipped := counts[0], counts[1], counts[2]
	if executed+skipped > tr.MaxTests {
		return fmt.Errorf("SQLite total count bound")
	}
	if m[3] != in.SourceFile {
		return fmt.Errorf("SQLite native source identity contradicts bound sourceFile")
	}
	if (errors == 0 && in.ExitCode != 0) || (errors > 0 && (in.ExitCode == 0 || in.ExitCode != errors%256)) {
		problem(o, "EXIT_CONTRADICTION", "SQLite native error count contradicts process status")
	}
	if errors == 0 && (len(in.Stdout) > 0 || len(lines) > 1) {
		problem(o, "UNCLASSIFIED_NATIVE_OUTPUT", "zero-error summary accompanied by additional native diagnostics")
	}
	halt := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(in.SourceFile) + `:[1-9][0-9]*: halt$`)
	if halt.Match(in.Stdout) {
		problem(o, "SQL_SUITE_HALTED", "native script halted before exhausting its records")
	}
	if executed+skipped == 0 {
		problem(o, "NO_TESTS", "native summary reports no executed or skipped SQL records")
		return nil
	}
	state, kind := tr.Passed, ""
	if errors > 0 {
		state, kind = tr.Failed, tr.Unknown
	} else if executed == 0 {
		state = tr.Skipped
	}
	// The native runner reports aggregate counts, not per-query identities.
	// Preserve a single suite record rather than inventing executed case names.
	o.Tests = append(o.Tests, tr.Test{ID: in.SourceFile, Name: in.SourceFile, File: in.SourceFile, Suite: in.SourceFile, Granularity: "SUITE_ONLY", ExecutedCount: &executed, SkippedCount: &skipped, State: state, FailureKind: kind, Message: string(in.Stdout) + string(in.Stderr)})
	return nil
}
