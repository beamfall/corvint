//go:build darwin || linux

package cli

import (
	"bytes"
	"context"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
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

func psrCLIReady(t *testing.T, resetScript string) (*intent.Repository, string) {
	t.Helper()
	r := fixture.TempRepo(t)

	v := fixture.PolicyValue()
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	s := func(x string) wire.Value { return wire.String(x) }
	reset := psrObject("argv", wire.Strings([]string{"/bin/sh", "-c", resetScript}), "envKeys", wire.Strings(nil), "timeoutSeconds", s("20"), "maxAttempts", s("1"), "verify", psrObject("argv", wire.Strings([]string{"/bin/echo", "verified"}), "envKeys", wire.Strings(nil), "expectExit", s("0"), "expectStdout", s("verified")))
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
	return repo, r.Root
}
func psrBinary(t *testing.T) string {
	t.Helper()
	bin := os.Getenv("PSR_TEST_BINARY")
	if bin == "" {
		t.Skip("compiled public CLI qualification requires PSR_TEST_BINARY")
	}
	return bin
}
func psrBinaryCall(t *testing.T, root string, args ...string) *wire.Result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, psrBinary(t), args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CORVINT_TASKS_ACTOR=tester", "ATM_ACTOR=tester")
	cmd.WaitDelay = time.Second
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	result, decode := wire.DecodeResult(out.Bytes())
	if decode != nil {
		t.Fatalf("binary %v %v %s %s", err, decode, out.Bytes(), stderr.Bytes())
	}
	return result
}
func TestPSRCompiledPoolSweep(t *testing.T) {
	_ = psrBinary(t)
	repo, root := psrCLIReady(t, "printf reset")
	h := psrBinaryCall(t, root, "pool", "sweep", "--help")
	if h.Outcome != wire.OutcomeOK {
		t.Fatal(h)
	}
	args := []string{"pool", "sweep", "--request-id", "compiled-sweep", "--timeout-seconds", "15", "--role", "OWNER"}
	out := psrBinaryCall(t, root, args...)
	if out.Outcome != wire.OutcomeOK {
		t.Fatal(out)
	}
	raw, err := os.ReadFile(filepath.Join(repo.StateDir, "pools.json"))
	if err != nil {
		t.Fatal(err)
	}
	pools, err := snapshot.DecodePools(raw)
	if err != nil || len(pools.Entries) != 0 {
		t.Fatal(pools, err)
	}
	replay := psrBinaryCall(t, root, args...)
	if replay.Outcome != wire.OutcomeOK || len(replay.Items) != len(out.Items) || !bytes.Equal(wire.EncodeFile(replay.Items[len(replay.Items)-1]), wire.EncodeFile(out.Items[len(out.Items)-1])) {
		t.Fatal("aggregate replay changed", replay)
	}
	audit := psrBinaryCall(t, root, "receipt", "audit")
	if audit.Outcome != wire.OutcomeOK {
		t.Fatal(audit)
	}
}

func TestPSRCompiledSignals(t *testing.T) {
	_ = psrBinary(t)
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			scratch := t.TempDir()
			marker := filepath.Join(scratch, "children")
			repo, root := psrCLIReady(t, "sleep 60 & child=$!; echo $$ $child > '"+marker+"'; wait")
			cmd := exec.Command(psrBinary(t), "pool", "sweep", "--request-id", "signal", "--timeout-seconds", "15", "--role", "OWNER")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "CORVINT_TASKS_ACTOR=tester", "ATM_ACTOR=tester")
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			cmd.WaitDelay = time.Second
			var out, stderr bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &stderr
			if e := cmd.Start(); e != nil {
				t.Fatal(e)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			joined := false
			defer func() {
				if !joined {
					_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
					<-done
				}
			}()
			psrWaitFile(t, marker)
			raw, e := os.ReadFile(marker)
			if e != nil {
				t.Fatal(e)
			}
			children := []int{}
			for _, v := range strings.Fields(string(raw)) {
				pid, e := strconv.Atoi(v)
				if e != nil {
					t.Fatal(e)
				}
				children = append(children, pid)
			}
			if len(children) != 2 {
				t.Fatal("fixture child initialization", string(raw))
			}
			if e = cmd.Process.Signal(sig); e != nil {
				t.Fatal(e)
			}
			select {
			case <-done:
				joined = true
			case <-time.After(38 * time.Second):
				t.Fatal("compiled signal did not join")
			}
			if _, e = wire.DecodeResult(out.Bytes()); e != nil {
				t.Fatalf("missing bounded result: %v %s %s", e, out.Bytes(), stderr.Bytes())
			}
			for _, pid := range children {
				end := time.Now().Add(2 * time.Second)
				for time.Now().Before(end) {
					id, _ := supervisor.ProcessIdentity(pid)
					if id == "" {
						break
					}
					time.Sleep(10 * time.Millisecond)
				}
				if id, _ := supervisor.ProcessIdentity(pid); id != "" {
					t.Fatalf("owned child survived %d", pid)
				}
			}
			raw, e = os.ReadFile(filepath.Join(repo.StateDir, "pools.json"))
			if e != nil {
				t.Fatal(e)
			}
			pools, e := snapshot.DecodePools(raw)
			if e != nil || len(pools.Entries) != 1 || pools.Entries[0].State == "FREE" {
				t.Fatal("signal freed member", pools, e)
			}
		})
	}
}
