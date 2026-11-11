// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// headlineLines returns the AFP-V0-3642 headline lines of stderr.
func headlineLines(stderr string) []string {
	var lines []string
	for _, line := range strings.Split(stderr, "\n") {
		if strings.HasPrefix(line, affectedHeadlinePrefix) {
			lines = append(lines, line)
		}
	}
	return lines
}

// AFP-V0-3642 (V1-4321): the headline restates the plan in at most ten
// lines of at most 200 bytes each: the summary line with revision and base,
// the tier counts, the unknown reasons by count with one detail each (at most
// five, then how many more), the receipt warning, and the commands that print
// the plan. A plan with many long unknowns stays inside the bound.
func TestAffectedHeadlineIsBounded_AFPV03642(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 400)
	receipt := affectedCompactReceipt{
		Revision: strings.Repeat("a", 40),
		Range:    affectedRange{Base: strings.Repeat("b", 40)},
		Warning:  "worktree content was read unpinned " + long,
		Tiers:    &affectedTiers{Counts: [3]int{3, 44, 160}, Summary: "tier0=3 tier1=44 tier2=160"},
	}
	for index := range 9 {
		for count := 0; count <= index; count++ {
			receipt.Plan.Unknown = append(receipt.Plan.Unknown, affected.Unknown{Reason: fmt.Sprintf("REASON_%d", index), Detail: fmt.Sprintf("detail-%d-%d-%s", index, count, long)})
		}
	}
	receipt.Summary = affectedSummary{Line: "scope=UNKNOWN dirty=4 selected=207 excluded=439 unknown=45 REASON_8=9", Scope: affected.ScopeUnknown, Unknown: len(receipt.Plan.Unknown)}
	headline := renderAffectedHeadline(receipt)
	lines := strings.Split(strings.TrimSuffix(headline, "\n"), "\n")
	if len(lines) > 10 || len(headline) > 2048 || !strings.HasSuffix(headline, "\n") {
		t.Fatalf("headline is %d lines, %d bytes:\n%s", len(lines), len(headline), headline)
	}
	for _, line := range lines {
		if len(line) > 200 || !strings.HasPrefix(line, affectedHeadlinePrefix) {
			t.Fatalf("line of %d bytes or without the prefix: %q", len(line), line)
		}
	}
	want := []string{
		affectedHeadlinePrefix + "scope=UNKNOWN dirty=4 selected=207 excluded=439 unknown=45 REASON_8=9 revision=" + strings.Repeat("a", 40) + " base=" + strings.Repeat("b", 12),
		affectedHeadlinePrefix + "tiers tier0=3 tier1=44 tier2=160",
		affectedHeadlinePrefix + "unknown REASON_8 x9: detail-8-0-",
		affectedHeadlinePrefix + "unknown REASON_7 x8: detail-7-0-",
		affectedHeadlinePrefix + "unknown REASON_6 x7: detail-6-0-",
		affectedHeadlinePrefix + "unknown REASON_5 x6: detail-5-0-",
		affectedHeadlinePrefix + "unknown REASON_4 x5: detail-4-0-",
		affectedHeadlinePrefix + "unknown 4 more reasons (10 entries) in plan.unknown",
		affectedHeadlinePrefix + "warning worktree content was read unpinned ",
		affectedHeadlinePrefix + "stdout is the affected-plan/1 JSON plan; --format text prints it as lines, --summary --format text a few-line verdict",
	}
	if len(lines) != len(want) {
		t.Fatalf("headline:\n%s", headline)
	}
	for index, prefix := range want {
		if !strings.HasPrefix(lines[index], prefix) {
			t.Fatalf("line %d = %q, want prefix %q", index+1, lines[index], prefix)
		}
	}

	// A bounded plan with nothing selected is two lines: no tiers, no unknown.
	quiet := renderAffectedHeadline(affectedCompactReceipt{Revision: "c", Summary: affectedSummary{Line: "scope=BOUNDED dirty=0 selected=0 excluded=0 unknown=0", Scope: affected.ScopeBounded}})
	if got := strings.Count(quiet, "\n"); got != 2 || !strings.HasPrefix(quiet, affectedHeadlinePrefix+"scope=BOUNDED ") {
		t.Fatalf("quiet headline:\n%s", quiet)
	}
}

// AFP-V0-3642 (V1-4321): `corvint affected` writes the headline to stderr
// before the plan when stderr is its own file, and stdout stays the
// AFP-V0-398 canonical document byte for byte. When both streams are one
// file (2>&1) the headline is left out, as AFP-V0-871 leaves out the
// degradation line, so the merged output still parses; an in-process writer
// that is not a file gets no headline.
func TestAffectedHeadlineOnSeparateStderr_AFPV03642(t *testing.T) {
	root, built := selfBinaryRepository(t)
	commitSelfBinaryFiles(t, root, map[string]string{"self.go": "package self\n\nfunc Self() int { return 2 }\n"})
	arguments := []string{"--root", root, "affected", "--base", built, "--accept-stale-binary"}

	var bufferOut, bufferErr bytes.Buffer
	if code := runContext(context.Background(), arguments, strings.NewReader(""), &bufferOut, &bufferErr); code != 0 {
		t.Fatalf("in-process run: exit %d: %s", code, bufferErr.String())
	}
	if lines := headlineLines(bufferErr.String()); len(lines) != 0 {
		t.Fatalf("a writer that is not a file got a headline: %q", lines)
	}

	dir := t.TempDir()
	stdout, err := os.Create(filepath.Join(dir, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := os.Create(filepath.Join(dir, "stderr"))
	if err != nil {
		t.Fatal(err)
	}
	code := runContext(context.Background(), arguments, strings.NewReader(""), stdout, stderr)
	_ = stdout.Close()
	_ = stderr.Close()
	out, _ := os.ReadFile(stdout.Name())
	errOut, _ := os.ReadFile(stderr.Name())
	if code != 0 || !bytes.Equal(out, bufferOut.Bytes()) {
		t.Fatalf("stdout changed (exit %d):\nfile   %s\nbuffer %s", code, out, bufferOut.Bytes())
	}
	lines := headlineLines(string(errOut))
	if len(lines) < 2 || len(lines) > 10 || len(errOut) > 4096 || !strings.HasPrefix(lines[0], affectedHeadlinePrefix+"scope=BOUNDED dirty=1 selected=1 ") || !strings.HasPrefix(lines[len(lines)-1], affectedHeadlinePrefix+"stdout is the affected-plan/1 JSON plan") {
		t.Fatalf("stderr headline:\n%s", errOut)
	}

	merged, err := os.Create(filepath.Join(dir, "merged"))
	if err != nil {
		t.Fatal(err)
	}
	code = runContext(context.Background(), arguments, strings.NewReader(""), merged, merged)
	_ = merged.Close()
	both, _ := os.ReadFile(merged.Name())
	var receipt map[string]any
	if code != 0 || json.Unmarshal(both, &receipt) != nil || len(headlineLines(string(both))) != 0 {
		t.Fatalf("merged output is not one JSON document (exit %d):\n%s", code, both)
	}

	// A pipe on stderr gets no headline: a write to a closed pipe on fd 2
	// raises SIGPIPE in a Go program and would end the run before the plan.
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_ = reader.Close()
	var pipeOut bytes.Buffer
	code = runContext(context.Background(), arguments, strings.NewReader(""), &pipeOut, writer)
	_ = writer.Close()
	if code != 0 || !bytes.Equal(pipeOut.Bytes(), bufferOut.Bytes()) {
		t.Fatalf("closed-pipe stderr changed the run (exit %d):\n%s", code, pipeOut.Bytes())
	}
	reader, writer, err = os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	pipeOut.Reset()
	code = runContext(context.Background(), arguments, strings.NewReader(""), &pipeOut, writer)
	_ = writer.Close()
	piped, _ := io.ReadAll(reader)
	_ = reader.Close()
	if code != 0 || len(headlineLines(string(piped))) != 0 || !bytes.Equal(pipeOut.Bytes(), bufferOut.Bytes()) {
		t.Fatalf("pipe stderr got a headline or changed stdout (exit %d):\n%s", code, piped)
	}

	// The summary and --format text forms already are few lines: no headline.
	for _, form := range [][]string{{"--summary"}, {"--format", "text"}} {
		errFile, err := os.Create(filepath.Join(t.TempDir(), "stderr"))
		if err != nil {
			t.Fatal(err)
		}
		var formOut bytes.Buffer
		code := runContext(context.Background(), append(arguments, form...), strings.NewReader(""), &formOut, errFile)
		_ = errFile.Close()
		formErr, _ := os.ReadFile(errFile.Name())
		if code != 0 || len(headlineLines(string(formErr))) != 0 {
			t.Fatalf("%v: exit %d stderr:\n%s", form, code, formErr)
		}
	}
}
