//go:build darwin || linux

package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// CRT-V0-006: an empty reply, a reply without a citations block and an
// unparseable block are three distinct states, and each cites nothing.
func TestExtractCitationsKeepsEmptyAbsentAndMalformedDistinct(t *testing.T) {
	for reply, want := range map[string]string{
		"":                                 "EMPTY",
		" \n\t":                            "EMPTY",
		"No idea.":                         "ABSENT",
		"```json\n{\"citations\":[}\n```":  "MALFORMED",
		"```json\n{\"citations\":[]}\n```": "PRESENT",
	} {
		citations, state := extractCitations(reply)
		if state != want || len(citations) != 0 {
			t.Errorf("reply %q: state %s citations %v, want %s", reply, state, citations, want)
		}
	}
}

// CRT-V0-011: a usage event with a negative or non-numeric count is not a
// token observation.
func TestNegativeTokenCountsAreNotObservations(t *testing.T) {
	for _, usage := range []string{`{"input_tokens":-1,"output_tokens":5}`, `{"input_tokens":"5"}`} {
		if tokens := parseCodexEvents([]byte(`{"type":"turn.completed","usage":` + usage + "}\n")).tokens; tokens != notObserved {
			t.Errorf("usage %s recorded as %v", usage, tokens)
		}
	}
	usage, ok := parseCodexEvents([]byte(`{"type":"turn.completed","usage":{"input_tokens":5}}` + "\n")).tokens.(map[string]any)
	if !ok || usage["input_tokens"] != 5.0 {
		t.Fatalf("a valid usage event was refused: %v", usage)
	}
}

// fakeCodex puts a codex executable first on PATH that writes replyBytes to
// its -o path and prints events followed by padding bytes of stdout.
func fakeCodex(t *testing.T, replyBytes int, events string, padding int) {
	t.Helper()
	directory := t.TempDir()
	script := "#!/bin/sh\nout=\nwhile [ $# -gt 0 ]; do if [ \"$1\" = -o ]; then out=$2; fi; shift; done\n" +
		"head -c " + strconv.Itoa(replyBytes) + " /dev/zero | tr '\\0' y > \"$out\"\n" +
		"printf '%s\\n' '" + events + "'\n" +
		"head -c " + strconv.Itoa(padding) + " /dev/zero | tr '\\0' z\n"
	if err := os.WriteFile(filepath.Join(directory, "codex"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// CRT-V0-006 and CRT-V0-011: a codex reply larger than the reply bound is read only one
// byte past it and recorded truncated; a stdout stream cut at the capture
// bound is recorded and leaves usage and tool calls NOT_OBSERVED.
func TestCodexRunBoundsTheReplyAndRecordsStdoutTruncation(t *testing.T) {
	usage := `{"type":"turn.completed","usage":{"input_tokens":5,"output_tokens":1}}`
	fakeCodex(t, maxReplyBytes+4096, usage, maxOutputBytes+1024)
	result, err := codexAgent{model: "m"}.run(context.Background(), t.TempDir(), "prompt", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.reply) != maxReplyBytes+1 || !result.stdoutTruncated || result.tokens != notObserved || result.toolCalls != notObserved {
		t.Fatalf("reply %d bytes, stdout truncated %v, tokens %v, tool calls %v", len(result.reply), result.stdoutTruncated, result.tokens, result.toolCalls)
	}
	if _, truncated := truncate(result.reply, maxReplyBytes); !truncated {
		t.Fatal("an oversized reply was not marked truncated")
	}

	fakeCodex(t, 16, usage, 0)
	result, err = codexAgent{model: "m"}.run(context.Background(), t.TempDir(), "prompt", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if result.stdoutTruncated || result.tokens == notObserved {
		t.Fatalf("an uncut stream lost its usage: truncated %v tokens %v", result.stdoutTruncated, result.tokens)
	}
}

// CRT-V0-011: a script agent whose stdout is cut at the capture bound is
// recorded as both reply- and stdout-truncated.
func TestScriptRunRecordsStdoutTruncation(t *testing.T) {
	script := filepath.Join(t.TempDir(), "agent.sh")
	body := "#!/bin/sh\nhead -c " + strconv.Itoa(maxOutputBytes+1024) + " /dev/zero | tr '\\0' z\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	result, err := scriptAgent{command: script}.run(context.Background(), t.TempDir(), "prompt", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !result.replyTruncated || !result.stdoutTruncated || len(result.reply) > maxOutputBytes || !strings.HasPrefix(result.reply, "zzz") {
		t.Fatalf("reply %d bytes, reply truncated %v, stdout truncated %v", len(result.reply), result.replyTruncated, result.stdoutTruncated)
	}
}
