//go:build darwin || linux

package dispatch

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

func testResolveCommit(dir, rev string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--verify", rev+"^{commit}").Output()
	return strings.TrimSpace(string(out)), err
}

func testListCommits(dir, base string, max int) ([]string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-list", "--max-count="+strconv.Itoa(max), "HEAD", "^"+base).Output()
	return strings.Fields(string(out)), err
}

func finishedEvents(t *testing.T, d *Dispatcher) []Event {
	t.Helper()
	events, err := ReadEvents(d.dir, 1000)
	if err != nil {
		t.Fatal(err)
	}
	var out []Event
	for _, e := range events {
		if e.Kind == "finished" {
			out = append(out, e)
		}
	}
	return out
}

// ATR-V0-350..353 (issue 1799): a worker stopped by a signal after it made a
// commit is reported with its structured outcome and the commit, is not
// "no progress", names the signal, and carries its last structured summary
// rather than the interrupted tool-output write the stop left behind.
func TestATRV0350_SignalStopWithCommitFinishedOutcome(t *testing.T) {
	const fragment = `{"type":"user","message":{"content":[{"type":"tool_result","content":"map(([key, customer]) => ...\n120:   ) as ...\n[Output truncated. Continue reading with offse`
	for name, tc := range map[string]struct {
		tail, code, signal, label, summary string
		commit                             bool
	}{
		"killed by SIGTERM after a commit": {tail: `kill -TERM $$`, code: "-1", signal: "SIGTERM", commit: true,
			label: "stopped by SIGTERM (operator-stop); 1 commit, store state unchanged", summary: "committed the fix"},
		"exit 130 before any summary": {tail: `exit 130`, code: "130", signal: "SIGINT",
			label: "stopped by SIGINT (operator-stop) before recording progress", summary: StoppedBeforeSummary},
	} {
		t.Run(name, func(t *testing.T) {
			script := ""
			if tc.commit {
				script = `echo '{"type":"assistant","message":{"content":[{"type":"text","text":"committed the fix"}]}}'` + "\n" +
					`git -c user.name=t -c user.email=t@example.invalid -c commit.gpgsign=false commit -q --allow-empty -m work >/dev/null` + "\n"
			}
			script += "printf '%s' '" + fragment + "'\n" + tc.tail
			c := testConfig(t, script)
			gitIn(t, c.WorkRoot, "init", "-q")
			gitIn(t, c.WorkRoot, "commit", "-q", "--allow-empty", "-m", "base")
			base := gitIn(t, c.WorkRoot, "rev-parse", "HEAD")
			q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
			d, err := Open("prog", c, q, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			d.ResolveCommit, d.ListCommits = testResolveCommit, testListCommits
			ctx := context.Background()
			if err := d.Tick(ctx); err != nil || d.Running() != 1 {
				t.Fatalf("tick: %v running %d", err, d.Running())
			}
			waitEnded(t, d)
			if err := d.Tick(ctx); err != nil {
				t.Fatal(err)
			}
			fin := finishedEvents(t, d)
			if len(fin) == 0 {
				t.Fatal("no finished event")
			}
			e := fin[0]
			want := map[string]string{"exitCode": tc.code, "exitSignal": tc.signal, "stopReason": StopOperator, "progress": "false", "session": SessionNoProgress, "progressLabel": tc.label, "attemptRelease": AttemptNone, "handoffNote": "false", "commitCount": "0", "commits": ""}
			if tc.commit {
				// The first commit after base is the stopped worker's; a
				// relaunch may have committed since.
				made := strings.Fields(gitIn(t, c.WorkRoot, "rev-list", "--reverse", base+"..HEAD"))
				if len(made) == 0 {
					t.Fatal("the worker made no commit")
				}
				head := made[0]
				want["commitCount"], want["commits"] = "1", head
			}
			for k, v := range want {
				if got, ok := e.Detail[k]; !ok || got != v {
					t.Errorf("detail %s = %q (present %v), want %q", k, got, ok, v)
				}
			}
			if strings.Contains(e.Message, "no progress") || !strings.Contains(e.Message, "(exit "+tc.code+", "+tc.label+")") {
				t.Errorf("message %q lacks the derived label %q", e.Message, tc.label)
			}
			if !strings.HasSuffix(e.Message, ": "+tc.summary) || strings.Contains(e.Message, "Output truncated") {
				t.Errorf("message %q does not end with summary %q", e.Message, tc.summary)
			}
		})
	}
}

// ATR-V0-350: the stop reason prefers the dispatcher's own stop, then the
// observed exit; the exit signal comes from the wait status or 128+N.
func TestATRV0350_StopReasonAndExitSignal(t *testing.T) {
	for _, tc := range []struct {
		kill           string
		st             exitStatus
		seen           bool
		reason, signal string
	}{
		{"", exitStatus{code: 0}, true, StopCompleted, "NONE"},
		{"", exitStatus{code: 1}, true, StopCrash, "NONE"},
		{"", exitStatus{code: 130}, true, StopOperator, "SIGINT"},
		{"", exitStatus{code: 143}, true, StopOperator, "SIGTERM"},
		{"", exitStatus{code: -1, signal: 9}, true, StopOperator, "SIGKILL"},
		{"", exitStatus{code: -1, signal: 11}, true, StopCrash, "SIGSEGV"},
		{"", exitStatus{code: 134}, true, StopCrash, "SIGABRT"},
		{"", exitStatus{code: -1, signal: 30}, true, StopCrash, "SIG30"},
		{"ORPHANED", exitStatus{code: 0}, true, StopCompleted, "NONE"},
		{"CONTEXT", exitStatus{code: -1, signal: 15}, true, StopContextLimit, "SIGTERM"},
		{"LEASE_EXPIRED", exitStatus{code: 143}, true, StopLeaseExpired, "SIGTERM"},
		{"IDLE", exitStatus{}, false, StopIdleTimeout, "NOT_OBSERVED"},
		{"WALL", exitStatus{code: -1, signal: 9}, true, StopWallLimit, "SIGKILL"},
		{"", exitStatus{}, false, StopUnknown, "NOT_OBSERVED"},
	} {
		if got := stopReason(tc.kill, tc.st, tc.seen); got != tc.reason {
			t.Errorf("stopReason(%q, %+v, %v) = %q, want %q", tc.kill, tc.st, tc.seen, got, tc.reason)
		}
		if got := exitSignalText(tc.st, tc.seen); got != tc.signal {
			t.Errorf("exitSignalText(%+v, %v) = %q, want %q", tc.st, tc.seen, got, tc.signal)
		}
	}
}

// ATR-V0-352: the label comes from the accounting class and the observed
// facts; a stop is named and a committed session is not "no progress".
func TestATRV0352_ProgressLabelFromFacts(t *testing.T) {
	for _, tc := range []struct {
		class, reason, signal string
		commits               int
		seen                  bool
		want                  string
	}{
		{SessionNoProgress, StopCompleted, "NONE", 0, true, "no progress"},
		{SessionNoProgress, StopCompleted, "NONE", 0, false, "no progress"},
		{SessionNoProgress, StopCompleted, "NONE", 2, true, "2 commits, store state unchanged"},
		{SessionProgress, StopCompleted, "NONE", 1, true, "progress recorded"},
		{SessionProgress, StopOperator, "SIGINT", 0, true, "stopped by SIGINT (operator-stop); progress recorded"},
		{SessionNoProgress, StopContextLimit, "SIGTERM", 1, true, "stopped by SIGTERM (context-limit); 1 commit, store state unchanged"},
		{SessionNoProgress, StopCrash, "NONE", 0, true, "failed (crash) before recording progress"},
		{SessionNoProgress, StopLeaseExpired, "NOT_OBSERVED", 0, false, "stopped (lease-expired) before recording progress"},
		{SessionNoProgress, StopUnknown, "NOT_OBSERVED", 0, false, "no progress"},
		{SessionInfrastructure, StopOperator, "SIGTERM", 0, true, "stopped by SIGTERM (operator-stop); infrastructure session"},
		{SessionHeld, StopCompleted, "NONE", 0, true, "held by a typed escalation"},
	} {
		if got := progressLabel(tc.class, tc.reason, tc.signal, tc.commits, tc.seen); got != tc.want {
			t.Errorf("progressLabel(%+v) = %q, want %q", tc, got, tc.want)
		}
	}
}

// ATR-V0-351: what became of the worker's attempt, from the final
// observation and this dispatcher's own hand-offs.
func TestATRV0351_AttemptRelease(t *testing.T) {
	d := &Dispatcher{ledger: &Ledger{Workers: []*Worker{{ID: "w1"}, {ID: "w2"}, {ID: "w3"}, {ID: "w4"}, {ID: "w5"}}}}
	d.noteHeld(&Observation{Attempts: []Attempt{{ID: "a1", Holder: "w1", Live: true}, {ID: "a2", Holder: "w2", Live: true}, {ID: "a3", Holder: "w3", Live: true}, {ID: "a5", Holder: "w5", Live: true}}})
	d.noteHandedOff("w1")
	d.noteHandedOff("gone") // a worker no longer recorded leaves no mark
	final := &Observation{Attempts: []Attempt{{ID: "a1", Holder: "", Live: false}, {ID: "a3", Holder: "w3", Live: true}}}
	for id, want := range map[string]string{"w1": AttemptHandedOff, "w2": AttemptReleased, "w3": AttemptHeld, "w4": AttemptNone, "w5": AttemptReaped} {
		w := &Worker{ID: id}
		if id == "w5" {
			w.KillReason = "LEASE_EXPIRED"
		}
		if got := d.attemptRelease(final, w); got != want {
			t.Errorf("attemptRelease(%s) = %q, want %q", id, got, want)
		}
	}
	if d.handedOff["gone"] {
		t.Error("an unrecorded worker was marked handed off")
	}
	d.remove("w1")
	if d.handedOff["w1"] || d.everHeld["w1"] {
		t.Error("remove kept the worker's release marks")
	}
}

// ATR-V0-353: an event stream's interrupted last write is skipped, and a
// worker that did not complete never carries a raw tail.
func TestATRV0353_SummaryNeverAStopFragment(t *testing.T) {
	gemini := `{"type":"message","role":"assistant","delta":true,"content":"first "}` + "\n" + `{"type":"message","role":"assistant","delta":true,"content":"reply"}` + "\n" + `{"type":"tool_use","tool_name":"x"}` + "\n"
	stream := `{"type":"assistant","message":{"content":[{"type":"text","text":"last words"}]}}` + "\n" + `{"type":"user","message":{"content":[{"type":"tool_result","content":"partial`
	for name, tc := range map[string]struct {
		stdout, reason, want string
	}{
		"interrupted stream, stopped":   {stream, StopOperator, "last words"},
		"interrupted stream, completed": {stream, StopCompleted, "last words"},
		"plain text, stopped":           {"working\nhalf a line", StopOperator, StoppedBeforeSummary},
		"plain text, completed":         {"working\nall done\n", StopCompleted, "working\nall done"},
		"fragment only, stopped":        {`{"type":"user","message":{"content":"partial`, StopContextLimit, StoppedBeforeSummary},
		"nothing, exit unobserved":      {"", StopUnknown, StoppedBeforeSummary},
		"mixed text, crash":             {`{"type":"result","result":"x"}` + "\nplain\n", StopCrash, StoppedBeforeSummary},
		"cut-off delta reply, stopped":  {gemini + `{"type":"message","role":"assistant","delta":true,"content":"Implemented the"}` + "\n", StopOperator, "first reply"},
		"cut-off delta reply, only":     {`{"type":"message","role":"assistant","delta":true,"content":"Implemented the"}` + "\n", StopOperator, StoppedBeforeSummary},
		"ended delta reply, completed":  {gemini + `{"type":"message","role":"assistant","delta":true,"content":"Implemented the fix"}` + "\n", StopCompleted, "Implemented the fix"},
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "stdout.log"), []byte(tc.stdout), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := finishSummary(dir, tc.reason); got != tc.want {
			t.Errorf("%s: summary %q, want %q", name, got, tc.want)
		}
	}
}
