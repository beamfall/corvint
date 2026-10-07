package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	codexTurn      = `{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":4,"output_tokens":3}}`
	claudeResult   = `{"type":"result","subtype":"success","is_error":false,"result":"done","session_id":"s","usage":{"input_tokens":2,"cache_read_input_tokens":3,"output_tokens":1}}`
	openCodeStart  = `{"type":"step_start","timestamp":1,"sessionID":"ses_1","part":{"type":"step-start"}}`
	openCodeFinish = `{"type":"step_finish","timestamp":2,"sessionID":"ses_1","part":{"type":"step-finish","reason":"%s","tokens":{"input":5,"output":2,"reasoning":0,"cache":{"read":1,"write":0}}}}`
)

func openCodeStep(reason string) string { return strings.Replace(openCodeFinish, "%s", reason, 1) }

// usageOf drains a worker directory whose stdout.log holds out.
func usageOf(t *testing.T, format, out string) *WorkerUsage {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "stdout.log"), []byte(out), 0o600); err != nil {
		t.Fatal(err)
	}
	u := &WorkerUsage{Format: format}
	drainUsage(dir, u)
	return u
}

func TestCALV0157_UsageVocabulariesAndStates(t *testing.T) {
	cases := []struct {
		name, format, out, state string
		input, output            uint64
	}{
		{"codex turns add", "codex", `{"type":"thread.started","thread_id":"x"}` + "\n" + codexTurn + "\n" + codexTurn + "\n", UsageKnown, 20, 6},
		{"codex failed turn", "codex", codexTurn + "\n" + `{"type":"turn.failed","error":{"message":"x"}}` + "\n", UsagePartial, 10, 3},
		{"codex usage without counters", "codex", codexTurn + "\n" + `{"type":"turn.completed","usage":{}}` + "\n", UsagePartial, 10, 3},
		{"codex torn usage line", "codex", codexTurn + "\n" + `{"type":"turn.completed","usage":{"input_` + "\n", UsagePartial, 10, 3},
		{"claude result", "claude-code", `{"type":"assistant"}` + "\n" + claudeResult + "\n", UsageKnown, 5, 1},
		{"claude failed result", "claude-code", `{"type":"result","subtype":"error_max_turns","is_error":true,"session_id":"s","usage":{"input_tokens":2,"output_tokens":1}}` + "\n", UsagePartial, 2, 1},
		{"claude two results", "claude-code", claudeResult + "\n" + claudeResult + "\n", UsagePartial, 5, 1},
		{"claude result without usage", "claude-code", `{"type":"result","subtype":"success","is_error":false,"result":"done"}` + "\n", UsageUnknown, 0, 0},
		{"opencode stopped", "opencode", openCodeStart + "\n" + openCodeStep("tool-calls") + "\n" + openCodeStart + "\n" + openCodeStep("stop") + "\n", UsageKnown, 12, 4},
		{"opencode last step not stopped", "opencode", openCodeStart + "\n" + openCodeStep("tool-calls") + "\n", UsagePartial, 6, 2},
		{"opencode step still open", "opencode", openCodeStart + "\n" + openCodeStep("stop") + "\n" + openCodeStart + "\n", UsagePartial, 6, 2},
		{"opencode malformed step", "opencode", openCodeStart + "\n" + openCodeStep("stop") + "\n" + `{"type":"step_finish","part":{}}` + "\n", UsagePartial, 6, 2},
		{"codex null counters", "codex", codexTurn + "\n" + `{"type":"turn.completed","usage":{"input_tokens":null,"output_tokens":null}}` + "\n", UsagePartial, 10, 3},
		{"codex repeated counter", "codex", codexTurn + "\n" + `{"type":"turn.completed","usage":{"input_tokens":null,"input_tokens":0,"output_tokens":0}}` + "\n", UsagePartial, 10, 3},
		{"codex escaped type repeats a counter", "codex", codexTurn + "\n" + `{"type":"turn.\u0063ompleted","usage":{"input_tokens":100,"input_tokens":0,"output_tokens":0}}` + "\n", UsagePartial, 10, 3},
		{"codex case-folded usage", "codex", codexTurn + "\n" + `{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":0},"USAGE":{"input_tokens":0,"output_tokens":0}}` + "\n", UsagePartial, 10, 3},
		{"codex escaped type unreadable", "codex", codexTurn + "\n" + `{"type":"turn.\u0063ompleted","type":0,"usage":{"input_tokens":100,"output_tokens":1}}` + "\n", UsagePartial, 10, 3},
		{"codex turn left open", "codex", `{"type":"turn.started"}` + "\n" + codexTurn + "\n" + `{"type":"turn.started"}` + "\n", UsagePartial, 10, 3},
		{"codex total overflows", "codex", codexTurn + "\n" + `{"type":"turn.completed","usage":{"input_tokens":9223372036854775808,"output_tokens":9223372036854775808}}` + "\n", UsagePartial, 10, 3},
		{"claude total overflows", "claude-code", `{"type":"result","subtype":"success","is_error":false,"result":"done","session_id":"s","usage":{"input_tokens":9223372036854775808,"output_tokens":9223372036854775808}}` + "\n", UsageUnknown, 0, 0},
		{"no usage lines", "codex", "plain text\n{\"type\":\"item.completed\"}\n", UsageUnknown, 0, 0},
		{"final line without newline", "codex", codexTurn + "\n" + codexTurn, UsageKnown, 20, 6},
	}
	for _, tc := range cases {
		u := usageOf(t, tc.format, tc.out)
		if got := u.State(true); got != tc.state || u.Input != tc.input || u.Output != tc.output {
			t.Errorf("%s: state %s %d/%d, want %s %d/%d (%+v)", tc.name, got, u.Input, u.Output, tc.state, tc.input, tc.output, u)
		}
	}
	if (*WorkerUsage)(nil).State(true) != UsageUnknown {
		t.Error("an undeclared format is not UNKNOWN")
	}
	if u := usageOf(t, "codex", codexTurn+"\n"); u.State(false) != UsagePartial {
		t.Error("a running total is not a lower bound")
	}
}

func TestCALV0157_UsageReadIsIncrementalAndBounded(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stdout.log")
	appendOut := func(s string) {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		f.WriteString(s)
		f.Close()
	}
	u := &WorkerUsage{Format: "codex"}
	readUsage(dir, u) // no file yet: nothing is lost before the end
	appendOut(codexTurn + "\n" + codexTurn[:20])
	readUsage(dir, u)
	if u.Records != 1 || u.Offset != int64(len(codexTurn)+1) {
		t.Fatalf("an incomplete line was consumed: %+v", u)
	}
	appendOut(codexTurn[20:] + "\n")
	readUsage(dir, u)
	if u.Records != 2 || u.Input != 20 {
		t.Fatalf("the completed line was not counted: %+v", u)
	}

	// An oversize line, with and without its newline yet, is skipped and
	// leaves the total PARTIAL; the following line still counts.
	long := `{"type":"turn.completed","pad":"` + strings.Repeat("x", maxUsageLine) + `"}`
	appendOut(long[:maxUsageLine+10])
	readUsage(dir, u)
	if !u.Malformed || !u.Skipping {
		t.Fatalf("an oversize partial line was not skipped: %+v", u)
	}
	appendOut(long[maxUsageLine+10:] + "\n" + codexTurn + "\n")
	readUsage(dir, u)
	if u.Records != 3 || u.Skipping || u.State(true) != UsagePartial {
		t.Fatalf("after an oversize line: %+v", u)
	}

	// One read takes at most maxUsageRead bytes; the final read that cannot
	// reach the end leaves the rest lost.
	defer func(n int64) { maxUsageRead = n }(maxUsageRead)
	maxUsageRead = int64(len(codexTurn) + 1)
	v := &WorkerUsage{Format: "codex"}
	other := t.TempDir()
	os.WriteFile(filepath.Join(other, "stdout.log"), []byte(strings.Repeat(codexTurn+"\n", 3)), 0o600)
	readUsage(other, v)
	if v.Records != 1 {
		t.Fatalf("one bounded read counted %d lines", v.Records)
	}
	drainUsage(other, v)
	if v.Records != 2 || !v.Lost || v.State(true) != UsagePartial {
		t.Fatalf("a drain short of the end is not partial: %+v", v)
	}
}

func TestCALV0157_UsageAfterRotationIsPartial(t *testing.T) {
	defer func(n int64) { workerLogSegmentBytes = n }(workerLogSegmentBytes)
	workerLogSegmentBytes = 4 * int64(len(codexTurn)+1)
	dir := t.TempDir()
	path := filepath.Join(dir, "stdout.log")
	os.WriteFile(path, []byte(strings.Repeat(codexTurn+"\n", 6)), 0o600)
	u := &WorkerUsage{Format: "codex"}
	readUsage(dir, u)
	if cut, stdout := capWorkerLogs(dir); !cut || !stdout {
		t.Fatal("stdout.log was not cut")
	}
	u.cut()
	f, _ := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	f.WriteString(codexTurn + "\n" + codexTurn + "\n")
	f.Close()
	drainUsage(dir, u)
	if u.State(true) != UsagePartial || !u.Lost || u.Records < 6 {
		t.Fatalf("a total read across a cut is not a partial lower bound: %+v", u)
	}
	if u.Input+u.Output > 8*13 {
		t.Fatalf("a cut counted lines twice: %+v", u)
	}

	// A file shorter than the read offset was cut by something else.
	v := &WorkerUsage{Format: "codex"}
	readUsage(dir, v)
	os.WriteFile(path, []byte(codexTurn+"\n"), 0o600)
	drainUsage(dir, v)
	if !v.Lost || v.State(true) != UsagePartial {
		t.Fatalf("a shrunk file is not partial: %+v", v)
	}

	// A worker whose stdout.log vanished after it was read lost its output.
	w := &WorkerUsage{Format: "codex", Offset: 10, Records: 1, Input: 1}
	drainUsage(t.TempDir(), w)
	if !w.Lost {
		t.Fatal("a missing log after a read is not lost")
	}
}

// CAL-V0-155: a scope's token sum neither wraps nor saturates while the
// window's oldest sessions are subtracted.
func TestCALV0155_TokenSumNeitherWrapsNorSaturates(t *testing.T) {
	var s tokenSum
	s.add(1 << 63)
	s.add(1 << 63)
	s.add(7)
	if !s.atLeast(1<<50) || s.value() != 1<<64-1 {
		t.Fatalf("sum wrapped: %+v", s)
	}
	s.sub(1 << 63)
	s.sub(1 << 63)
	if s.atLeast(8) || !s.atLeast(7) || s.value() != 7 {
		t.Fatalf("sum after release: %+v", s)
	}
}
