//go:build darwin || linux

package cli

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func psrObject(pairs ...interface{}) wire.Value {
	o := wire.NewObject()
	for i := 0; i < len(pairs); i += 2 {
		o.Set(pairs[i].(string), pairs[i+1].(wire.Value))
	}
	return wire.ObjectValue(o)
}
func TestPSRForegroundSignalHelper(t *testing.T) {
	if root := os.Getenv("PSR_SIGNAL_ROOT"); root != "" {
		r := poolSweepCommand(Env{Cwd: root}, []string{"--request-id", "signal-sweep", "--timeout-seconds", "30", "--role", "OWNER"})
		if r.Outcome != wire.OutcomeOK {
			t.Fatalf("helper: %+v", r)
		}
		return
	}
}
func TestPSRForegroundSignals(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			r := fixture.TempRepo(t)
			marker := filepath.Join(fixture.TempDirOutside(t), "pids")
			v := fixture.PolicyValue()
			budgets, _ := v.Obj.Get("budgets")
			budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
			s := func(x string) wire.Value { return wire.String(x) }
			reset := psrObject("argv", wire.Strings([]string{"/bin/sh", "-c", "echo $$ > " + marker + "; /bin/sleep 60 & child=$!; echo $child >> " + marker + "; wait $child"}), "envKeys", wire.Strings(nil), "timeoutSeconds", s("20"), "maxAttempts", s("1"), "verify", psrObject("argv", wire.Strings([]string{"/bin/echo", "verified"}), "envKeys", wire.Strings(nil), "expectExit", s("0"), "expectStdout", s("verified")))
			v.Obj.Set("pools", wire.Array(psrObject("id", s("db"), "members", wire.Strings([]string{"a"}), "memberConfig", psrObject("a", psrObject("safeReuse", reset)))))
			if _, e := intent.DecodePolicy(wire.EncodeFile(v)); e != nil {
				t.Fatal(e)
			}
			fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
			fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(v))
			repo, e := intent.Resolve(r.Root)
			if e != nil {
				t.Fatal(e)
			}
			actor := mutation.Binding{ID: "tester", Role: "OWNER"}
			at, _ := wire.ParseTimestamp("now", time.Now().UTC().Truncate(time.Second).Format(time.RFC3339))
			if x, e := store.Init(context.Background(), repo, actor, "init", at); e != nil || x.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("init %+v %v", x, e)
			}
			git := func(args ...string) {
				t.Helper()
				c := exec.Command("git", args...)
				c.Dir = r.Root
				if raw, e := c.CombinedOutput(); e != nil {
					t.Fatalf("git %v %s", e, raw)
				}
			}
			git("init", "-q", "-b", "main")
			git("add", ".taskman")
			git("-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "fixture")
			payload, parseErr := wire.Parse([]byte(`{"acceptanceCriteria":["it exists"],"body":null,"capabilities":[],"dependencies":[],"dueDate":null,"effects":{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":[]},"estimateMinutes":null,"executionClass":"AUTONOMOUS","kind":"FEATURE","labels":[],"milestone":null,"order":"0","owner":null,"priority":"P2","requiredGates":[],"requirementRefs":[],"source":{"kind":"NATIVE","sourceItemId":null,"sourceQueueId":"queue:acme:main","sourceRevisionSha256":null},"supersededBy":null,"supersedes":null,"title":"Signal fixture"}` + "\n"))
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			envelope := psrObject("profile", s(mutation.Profile), "requestId", s("create"), "actor", psrObject("id", s(actor.ID), "role", s(actor.Role)), "queueId", s(fixture.QueueID), "targetId", wire.Null(), "expectedRevision", wire.Null(), "operation", s(mutation.OpCreate), "payload", payload, "issuedAt", s(string(at)))
			created, e := store.Mutate(context.Background(), repo, actor, wire.EncodeFile(envelope), at)
			if e != nil || created.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("create %+v %v", created, e)
			}
			// Intent writes are committed before the sweep's immutable-source fixture.
			git("add", ".taskman")
			git("-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "ticket")
			claim, e := store.Lease(context.Background(), repo, actor, store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "claim", Root: r.Root, Lease: transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: created.Ticket, Holder: "builder", LeaseMinutes: "5", Scope: []string{"work"}, Pool: "db"}}, at)
			if e != nil || claim.PoolAllocation == nil {
				t.Fatalf("claim %+v %v", claim, e)
			}
			rel, e := store.Lease(context.Background(), repo, actor, store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "release", Root: r.Root, Lease: transaction.LeaseRequest{Verb: transaction.LeaseRelease, AttemptID: claim.AttemptID, Generation: claim.Generation}}, at)
			if e != nil || rel.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("release %+v %v", rel, e)
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestPSRForegroundSignalHelper$", "-test.v")
			cmd.Env = append(os.Environ(), "PSR_SIGNAL_ROOT="+r.Root, "CORVINT_TASKS_ACTOR=tester", "ATM_ACTOR=tester")
			cmd.WaitDelay = time.Second
			var output bytes.Buffer
			cmd.Stdout = &output
			cmd.Stderr = &output
			if e = cmd.Start(); e != nil {
				t.Fatal(e)
			}
			joined := false
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			t.Cleanup(func() {
				if !joined {
					_ = cmd.Process.Signal(syscall.SIGTERM)
					select {
					case <-done:
						joined = true
					case <-time.After(6 * time.Second):
						_ = cmd.Process.Kill()
						select {
						case <-done:
							joined = true
						case <-time.After(2 * time.Second):
							t.Error("foreground fixture unjoined")
						}
						t.Error("foreground fixture needed hard kill; inner cleanup remains unproved")
					}
				}
			})
			deadline := time.Now().Add(4 * time.Second)
			var pids []string
			for {
				raw, _ := os.ReadFile(marker)
				pids = strings.Fields(string(raw))
				if len(pids) == 2 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("injected signal path not reached: %s", output.String())
				}
				time.Sleep(10 * time.Millisecond)
			}
			// The live child marker proves this is a pending original replay,
			// rather than a replay after terminal completion.
			beforeMarker, readErr := os.ReadFile(marker)
			if readErr != nil {
				t.Fatal(readErr)
			}
			t.Setenv("CORVINT_TASKS_ACTOR", "tester")
			t.Setenv("ATM_ACTOR", "tester")
			pending := poolSweepCommand(Env{Cwd: r.Root}, []string{"--request-id", "signal-sweep", "--timeout-seconds", "30", "--role", "OWNER"})
			if pending.Outcome != wire.OutcomeNotRun || len(pending.Items) == 0 {
				t.Fatalf("pending hidden: %+v", pending)
			}
			pendingValue, ok := pending.Items[0].Obj.Get("pending")
			if !ok || !pendingValue.Bool {
				t.Fatal("pending marker absent")
			}
			outcome, _ := pending.Items[0].Obj.Get("outcome")
			receipt, _ := pending.Items[0].Obj.Get("receiptSeq")
			if outcome.Str != "PENDING" || receipt.Str == "" {
				t.Fatal("pending receipt/state lost")
			}
			afterMarker, readErr := os.ReadFile(marker)
			if readErr != nil || !bytes.Equal(beforeMarker, afterMarker) {
				t.Fatal("pending replay started a second command", readErr)
			}
			t.Log("LIVE_PENDING_HANDLER_REPLAY_REACHED")
			if e = cmd.Process.Signal(sig); e != nil {
				t.Fatal(e)
			}
			select {
			case e = <-done:
				joined = true
				if e != nil {
					t.Fatalf("helper %v %s", e, output.String())
				}
			case <-time.After(8 * time.Second):
				t.Fatal("helper failed to join")
			}
			for _, x := range pids {
				pid, _ := strconv.Atoi(x)
				raw, e := exec.Command("/bin/ps", "-p", fmt.Sprint(pid), "-o", "stat=").Output()
				if e == nil && !strings.HasPrefix(strings.TrimSpace(string(raw)), "Z") {
					t.Fatalf("descendant alive %s %s", x, raw)
				}
			}
			raw, e := os.ReadFile(filepath.Join(repo.StateDir, "pools.json"))
			if e != nil {
				t.Fatal(e)
			}
			state, e := snapshot.DecodePools(raw)
			if e != nil || len(state.Entries) != 1 || state.Entries[0].State != "QUARANTINED" || !strings.Contains(state.Entries[0].Reason, "INTERRUPTED") {
				t.Fatalf("unsafe signal outcome %+v %v", state, e)
			}
		})
	}
}
