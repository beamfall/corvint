// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// affectedHeadlinePrefix starts every line of the AFP-V0-3642 headline, so a
// reader of a captured stderr can tell it from the other stderr lines.
const affectedHeadlinePrefix = "corvint affected: "

const (
	// affectedHeadlineMaxLineBytes bounds each headline line (AFP-V0-3642).
	affectedHeadlineMaxLineBytes = 200
	// affectedHeadlineUnknownReasons is how many unknown reasons the
	// headline names before it counts the rest.
	affectedHeadlineUnknownReasons = 5
)

// affectedHeadlineDetail tells a reader where the plan is and how to read it
// without a JSON tool.
const affectedHeadlineDetail = "stdout is the affected-plan/1 JSON plan; --format text prints it as lines, --summary --format text a few-line verdict"

// renderAffectedHeadline is the AFP-V0-3642 headline of a compact receipt: at
// most ten lines of at most affectedHeadlineMaxLineBytes bytes, each with
// affectedHeadlinePrefix. It restates the receipt and never changes the plan.
func renderAffectedHeadline(receipt affectedCompactReceipt) string {
	var lines []string
	header := receipt.Summary.Line + " revision=" + receipt.Revision
	if receipt.Range.Base != "" {
		header += " base=" + receipt.Range.Base
	}
	lines = append(lines, header)
	if receipt.Tiers != nil {
		lines = append(lines, "tiers "+receipt.Tiers.Summary)
	}
	type reason struct {
		name   string
		count  int
		detail string
	}
	byName := map[string]*reason{}
	var reasons []*reason
	for _, unknown := range receipt.Plan.Unknown {
		entry := byName[unknown.Reason]
		if entry == nil {
			entry = &reason{name: unknown.Reason, detail: unknown.Detail}
			byName[unknown.Reason] = entry
			reasons = append(reasons, entry)
		}
		entry.count++
	}
	sort.SliceStable(reasons, func(i, j int) bool {
		if reasons[i].count != reasons[j].count {
			return reasons[i].count > reasons[j].count
		}
		return reasons[i].name < reasons[j].name
	})
	for index, entry := range reasons {
		if index == affectedHeadlineUnknownReasons {
			rest := 0
			for _, left := range reasons[index:] {
				rest += left.count
			}
			lines = append(lines, fmt.Sprintf("unknown %d more reasons (%d entries) in plan.unknown", len(reasons)-index, rest))
			break
		}
		lines = append(lines, fmt.Sprintf("unknown %s x%d: %s", entry.name, entry.count, entry.detail))
	}
	if receipt.Warning != "" {
		lines = append(lines, "warning "+receipt.Warning)
	}
	lines = append(lines, affectedHeadlineDetail)
	var out strings.Builder
	for _, line := range lines {
		out.WriteString(boundHeadlineLine(affectedHeadlinePrefix + textWord(line)))
		out.WriteString("\n")
	}
	return out.String()
}

// boundHeadlineLine cuts line to affectedHeadlineMaxLineBytes bytes at a rune
// boundary, ending a cut line with "...".
func boundHeadlineLine(line string) string {
	if len(line) <= affectedHeadlineMaxLineBytes {
		return line
	}
	cut := affectedHeadlineMaxLineBytes - len("...")
	for cut > 0 && line[cut]&0xC0 == 0x80 {
		cut--
	}
	return line[:cut] + "..."
}

// writeAffectedHeadline writes the AFP-V0-3642 headline of a compact receipt
// to stderr before the plan is written to stdout. It writes nothing for any
// other receipt, for an stderr that is not a file (an in-process caller), or
// when stdout and stderr are one pipe or file, as after 2>&1, so stdout stays
// one parseable document there (AFP-V0-871). It also writes nothing when
// stderr is a pipe or socket: Go turns a write to a broken fd 2 pipe into
// SIGPIPE, which would end the run before the plan reaches stdout. Only a
// terminal or a regular file receives the headline, and a write failure there
// is ignored: the plan on stdout is the result.
func writeAffectedHeadline(receipt any, stdout, stderr io.Writer) {
	compact, ok := receipt.(affectedCompactReceipt)
	if !ok {
		return
	}
	file, isFile := stderr.(*os.File)
	if !isFile || sharedOutputStream(stdout, stderr) || !headlineSafeStream(file) {
		return
	}
	_, _ = io.WriteString(stderr, renderAffectedHeadline(compact))
}

// headlineSafeStream reports whether file is a character device (a terminal
// or /dev/null) or a regular file, the streams where a write cannot raise
// SIGPIPE.
func headlineSafeStream(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	mode := info.Mode()
	return mode.IsRegular() || mode&os.ModeCharDevice != 0
}
