// SPDX-License-Identifier: Apache-2.0

package platform

import (
	"bytes"
	"sort"
	"unicode/utf8"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// SwiftTerminalFailures retains only validated failed terminals from the captured
// prefix. Completeness, process errors and successful results remain the concern
// of Parse; its frozen observation and error behavior are unchanged. A later
// malformed record cannot undo a failure already reported (RSP-V0-003).
func SwiftTerminalFailures(in tr.Input) []string {
	if len(in.Stdout) > tr.MaxReportBytes || len(in.Stderr) > tr.MaxReportBytes || len(in.Reports) > tr.MaxReports {
		return nil
	}
	for _, raw := range in.Reports {
		if len(raw) > tr.MaxReportBytes {
			return nil
		}
	}
	prefix := func(raw []byte) []byte {
		raw = raw[:bytes.LastIndexByte(raw, '\n')+1]
		// Validate one record at a time so corrupt bytes in a later record do not
		// suppress terminals already validated in the captured prefix.
		end := 0
		for _, line := range bytes.SplitAfter(raw, []byte("\n")) {
			if !utf8.Valid(line) {
				break
			}
			end += len(line)
		}
		return raw[:end]
	}
	failed := map[string]bool{}
	retain := func(id string) { failed[id] = true }
	var observation tr.Observation
	switch in.Runner {
	case "swift-xctest":
		in.Stdout = prefix(in.Stdout)
		_ = parseXCTestFailures(in, &observation, retain)
	case "swift-testing":
		if len(in.Reports) != 1 {
			return nil
		}
		in.Reports = map[string][]byte{"swift-events.jsonl": prefix(in.Reports["swift-events.jsonl"])}
		_ = parseSwiftFailures(in, &observation, retain)
	}
	ids := make([]string, 0, len(failed))
	for id := range failed {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
