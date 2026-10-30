package cli_test

import (
	"bytes"
	"strconv"
	"sync"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestERGV0005_TwoWritersOneWinner races two native writers that record
// different verdicts on the same gate under the same generation/revision
// expectation (ERG-V0-005). Exactly one completes; the other is refused,
// and once any retryable contention (lock timeout, moved snapshot) is
// retried with its own request id it is REVISION_CONFLICT, never a second
// event. The surviving reference is the winner's at generation 1, revision
// 1; history holds one event; the receipt audit replays it.
func TestERGV0005_TwoWritersOneWinner(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, tree := ergStore(t)
	runOK := func(args ...string) run {
		t.Helper()
		r := atm(t, root, nil, args...)
		if r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s", args, r.stdout)
		}
		return r
	}
	for round := 0; round < 3; round++ {
		n := strconv.Itoa(round)
		id := planTicket(t, root, "raced "+n, "P1", `["src/"]`)
		c := runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim-"+n)
		attempt, gen := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
		runOK("submit", "--attempt", attempt, "--generation", gen, "--tree", tree, "--request-id", "submit-"+n)
		subject := field(runOK("receipt", "audit").res.Items[0], "headSeq").Str
		args := func(writer, verdict string) []string {
			a := []string{"gate", "record", id, "--gate", ergGate, "--verdict", verdict, "--subject-receipt", subject,
				"--expected-generation", "0", "--expected-revision", "0", "--request-id", "race-" + n + "-" + writer,
				"--issued-at", "2026-10-05T12:00:00Z"}
			if verdict == "RETURN" {
				a = append(a, "--reason", "TESTS:missing case")
			}
			return a
		}
		writers := [][]string{args("a", "PASS"), args("b", "RETURN")}
		outs := make([]bytes.Buffer, len(writers))
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := range writers {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				var errb bytes.Buffer
				cli.Run(cli.Env{Cwd: root, Args: writers[i], Stdin: bytes.NewReader(nil), Stdout: &outs[i], Stderr: &errb})
			}(i)
		}
		close(start)
		wg.Wait()

		winner := -1
		for i := range writers {
			res, err := wire.DecodeResult(outs[i].Bytes())
			if err != nil {
				t.Fatalf("round %s writer %d: %v", n, i, err)
			}
			if res.Outcome == wire.OutcomeOK {
				if winner >= 0 {
					t.Fatalf("round %s: both writers completed", n)
				}
				winner = i
				continue
			}
			if hasCode(res, wire.CodeLockTimeout) || hasCode(res, wire.CodeSnapshotMoved) {
				res = atm(t, root, nil, writers[i]...).res
			}
			if res.Outcome == wire.OutcomeOK || len(res.Items) == 0 || field(res.Items[0], "outcome").Str != mutation.OutcomeRevisionConflict {
				t.Fatalf("round %s: the losing writer %d was not a REVISION_CONFLICT: %v %v", n, i, res.Outcome, res.Codes)
			}
		}
		if winner < 0 {
			t.Fatalf("round %s: no writer completed", n)
		}
		want := map[int]string{0: dispatch.GatePass, 1: dispatch.GateReturn}[winner]
		v, state := ergGateView(t, root, id)
		if state != want || v.Generation != "1" || v.Revision != "1" {
			t.Fatalf("round %s: surviving reference %s %+v, want the winner's %s at 1/1", n, state, v, want)
		}
		h := runOK("gate", "history", id, "--gate", ergGate)
		if len(h.res.Items) != 1 || field(h.res.Items[0], "sha256").Str != v.Head {
			t.Fatalf("round %s: history %s", n, h.stdout)
		}
		if x := atm(t, root, nil, writers[winner]...); x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "outcome").Str != mutation.OutcomeCompleted {
			t.Fatalf("round %s: the winner's retry did not replay: %s", n, x.stdout)
		}
		// Rounds share one scope, so the next claim needs this lease freed.
		runOK("release", "--attempt", attempt, "--generation", gen, "--reason", wire.CodeHandoff, "--request-id", "release-"+n)
	}
	runOK("receipt", "audit")
}
