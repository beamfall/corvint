package adapters

import (
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/trace"
)

// FuzzTraceRowsCountEveryRowTheRecorderWrites drives the recorder's
// normalization over arbitrary fields the way `corvint record` does (a
// validation pass, then the record built from the validated commands) and
// feeds each row it writes and its own store reader accepts to the dashboard
// trace parser. The dashboard reads the store the product writes, so it must
// accept such a row with the recorder's trace ID, paths and outcome; parsing
// arbitrary rows must refuse rather than panic.
func FuzzTraceRowsCountEveryRowTheRecorderWrites(f *testing.F) {
	f.Add("café ☃ \U0001F600 <&>", "z.go\na.go", "a.go", "go test ./...\ngit diff --check", uint8(0))
	f.Add(" edge \x7f \b\f\t end ", "", "", "", uint8(2))
	f.Add("task", "cache/cache.go", "", " z\na \na", uint8(1))
	f.Fuzz(func(t *testing.T, task, opened, changed, verification string, outcome uint8) {
		parseTraceRows([]byte(task), testRevision)
		openedPaths, changedPaths := fuzzLines(opened), fuzzLines(changed)
		tracked := slices.Concat(openedPaths, changedPaths)
		if slices.ContainsFunc(tracked, func(value string) bool { return !gitStorablePath(value) }) {
			return
		}
		validated, err := trace.NewRecord(trace.Input{
			Revision: testRevision, TreeRevision: testRevision, Task: "record validation",
			Verification: fuzzLines(verification), Outcome: "passed",
		}, tracked)
		if err != nil {
			return
		}
		record, err := trace.NewRecord(trace.Input{
			Revision: testRevision, TreeRevision: testRevision, Task: task, OpenedPaths: openedPaths,
			ChangedPaths: changedPaths, Verification: validated.Verification,
			Outcome: [3]string{"passed", "failed", "blocked"}[int(outcome)%3],
		}, tracked)
		if err != nil {
			return
		}
		row, err := trace.Encode(record)
		if err != nil {
			return
		}
		if _, err := trace.DecodeStore(row, testRevision, tracked); err != nil {
			return
		}
		rows, code, _ := parseTraceRows(row, testRevision)
		if code != "" {
			t.Fatalf("the dashboard refused recorder row %q: %s", row, code)
		}
		got := rows[0]
		if got.traceID != record.TraceID || got.outcome != record.Outcome ||
			!slices.Equal(got.openedPaths, record.OpenedPaths) || !slices.Equal(got.changedPaths, record.ChangedPaths) {
			t.Fatalf("the dashboard parsed recorder row %q as %+v", row, got)
		}
	})
}

// gitStorablePath reports whether Git can track the path at all: the recorder
// only admits tracked paths, and Git refuses a NUL or an empty, "." or ".."
// component.
func gitStorablePath(value string) bool {
	if strings.ContainsRune(value, 0) {
		return false
	}
	for part := range strings.SplitSeq(value, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

// fuzzLines splits a fuzz string into list values; the empty string is the
// empty list.
func fuzzLines(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, "\n")
}
