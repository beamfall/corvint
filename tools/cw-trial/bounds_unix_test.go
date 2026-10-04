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

// CWT-V0-001: an empty reply, a reply without a claims block and an
// unparseable block are three distinct states, and each scores as an
// abstention.
func TestExtractClaimsKeepsEmptyAbsentAndMalformedDistinct(t *testing.T) {
	for reply, want := range map[string]string{
		"":                                "EMPTY",
		" \n\t":                           "EMPTY",
		"No idea.":                        "ABSENT",
		"```json\n{\"claims\":[}\n```":    "MALFORMED",
		"```json\n{\"claims\":[]}\n```\n": "PRESENT",
	} {
		if _, state := extractClaims(reply); state != want {
			t.Errorf("reply %q: state %s, want %s", reply, state, want)
		}
		record := &armRecord{Reply: reply}
		scoreArm(record, &taskRecord{Gold: fixtureGold()})
		if record.BlockState != want || record.Metrics["abstained"] != 1 {
			t.Errorf("reply %q: block %s metrics %v", reply, record.BlockState, record.Metrics)
		}
	}
}

// CWT-V0-004: a usage event with a negative or non-numeric count is not a
// token observation, and the report total refuses it.
func TestNegativeTokenCountsAreNotObservations(t *testing.T) {
	for _, usage := range []string{`{"input_tokens":-1,"output_tokens":5}`, `{"input_tokens":"5"}`} {
		events := parseCodexEvents([]byte(`{"type":"turn.completed","usage":` + usage + "}\n"))
		if events.tokens != notObserved {
			t.Errorf("usage %s recorded as %v", usage, events.tokens)
		}
	}
	sums := map[string]float64{}
	if addTokens(sums, map[string]any{"input_tokens": -3.0}) || len(sums) != 0 {
		t.Fatalf("a negative count was totalled: %v", sums)
	}
	if !addTokens(sums, map[string]any{"input_tokens": 3.0}) || sums["input_tokens"] != 3 {
		t.Fatalf("a valid count was refused: %v", sums)
	}
}

// CWT-V0-004: the reply file is read only up to one byte past the reply
// bound, so an oversized reply is never allocated whole.
func TestReadPrefixBoundsTheReplyRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reply.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("y", maxReplyBytes+4096)), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := readPrefix(path, maxReplyBytes+1)
	if err != nil || len(data) != maxReplyBytes+1 {
		t.Fatalf("read %d bytes, err %v", len(data), err)
	}
}

// fakeCodex puts a codex executable first on PATH that writes reply to its
// -o path and prints events followed by padding bytes of stdout.
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

// CWT-V0-004 and CWT-V0-014: a codex reply larger than the reply bound is
// read bounded and recorded truncated; a stdout stream cut at the capture
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
	record := &armRecord{}
	record.Reply, record.ReplyTruncated = truncate(result.reply, maxReplyBytes)
	if !record.ReplyTruncated {
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

// CWT-V0-014: a script agent whose stdout is cut at the capture bound is
// recorded as both reply- and stdout-truncated, so a lost claims block errors.
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
	if !result.replyTruncated || !result.stdoutTruncated || len(result.reply) > maxOutputBytes {
		t.Fatalf("reply %d bytes, reply truncated %v, stdout truncated %v", len(result.reply), result.replyTruncated, result.stdoutTruncated)
	}
}
