package cli_test

import (
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCALV0050_PolicyShowPureAndUpdateHelp(t *testing.T) {
	t.Run("CAL-V0-050 policy show purity update help", func(t *testing.T) {
		r := receiptFixture(t)
		state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
		actual, err := os.ReadFile(filepath.Join(r.IntentDir, "policy.json"))
		if err != nil {
			t.Fatal(err)
		}
		x := handoffCLI(t, r.Root, "policy", "show")
		if x.code != 0 {
			t.Fatalf("policy show: %s", x.stdout)
		}
		item := x.res.Items[0]
		if field(item, "policyVersion").Str != "1" || field(item, "policySha256").Str != string(wire.ContentID("policy", intent.ProfilePolicy, actual[:len(actual)-1])) || string(wire.EncodeFile(field(item, "policy"))) != string(actual) {
			t.Fatalf("effective policy: %s", x.stdout)
		}
		sameStore(t, r, state, intents, "policy show")
		h := handoffCLI(t, r.Root, "policy", "update", "--help", "--verbose")
		if h.code != 0 || field(h.res.Items[0], "fileFormat").Str == "" || field(h.res.Items[0], "versionRule").Str == "" {
			t.Fatalf("update help: %s", h.stdout)
		}
	})
}

func TestCALV0049_RetryReadProjections(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	id := planTicket(t, root, "retry-read", "P1", `["src/"]`)
	repo, e := intent.Resolve(root)
	if e != nil {
		t.Fatal(e)
	}
	before := fixture.TreeSnapshot(t, repo.StateDir)
	for _, args := range [][]string{{"ticket", "show", id}, {"plan", "preview"}, {"queue", "status", "--retries"}} {
		x := handoffCLI(t, root, args...)
		if x.code != 0 {
			t.Fatalf("%v: %s", args, x.stdout)
		}
		v := x.res.Items[0]
		if args[0] == "plan" {
			v = field(v, "entries").Arr[0]
		}
		if args[0] == "queue" {
			v = field(v, "retries").Arr[0]
		}
		debt := field(v, "retries")
		if debt.Obj == nil {
			t.Fatalf("missing retry projection: %s", x.stdout)
		}
		if field(debt, "charged").Str != "0" || field(debt, "limit").Str != "3" || field(debt, "remaining").Str != "3" {
			t.Fatalf("missing debt in %v: %s", args, x.stdout)
		}
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, repo.StateDir)) {
		t.Fatal("retry reads wrote state")
	}
}

func TestCALV0048_HeartbeatCLIReplayAndFence(t *testing.T) {
	t.Run("CAL-V0-048 heartbeat generation lease replay", func(t *testing.T) {
		root, claims := leaseCLIStore(t, 1, time.Now().UTC().Add(-10*time.Minute))
		c := claims[0]
		repo, e := intent.Resolve(root)
		if e != nil {
			t.Fatal(e)
		}
		read := handoffCLI(t, root, "attempt", "show", c.AttemptID)
		if read.code != 0 {
			t.Fatal(read.stdout)
		}
		expires := field(field(read.res.Items[0], "lease"), "expiresAt").Str
		args := []string{"attempt", "heartbeat", "--attempt", c.AttemptID, "--generation", string(c.Generation), "--request-id", "heartbeat-1"}
		h := handoffCLI(t, root, args...)
		if h.code != 0 {
			t.Fatalf("heartbeat: %s", h.stdout)
		}
		read = handoffCLI(t, root, "attempt", "show", c.AttemptID)
		if field(read.res.Items[0], "lastHeartbeatAt").Str == "" || field(read.res.Items[0], "holderStatus").Str != "FRESH_HOLDER" || field(field(read.res.Items[0], "lease"), "expiresAt").Str != expires {
			t.Fatalf("heartbeat read: %s", read.stdout)
		}
		before := fixture.TreeSnapshot(t, repo.StateDir)
		replay := handoffCLI(t, root, args...)
		if replay.code != 0 || !field(replay.res.Items[0], "replayed").Bool {
			t.Fatalf("heartbeat replay: %s", replay.stdout)
		}
		if !fixture.SameTree(before, fixture.TreeSnapshot(t, repo.StateDir)) {
			t.Fatal("replay refreshed heartbeat")
		}
		bad := handoffCLI(t, root, "attempt", "heartbeat", "--attempt", c.AttemptID, "--generation", "0", "--request-id", "heartbeat-stale")
		if !hasCode(bad.res, wire.CodeFenced) {
			t.Fatalf("generation fence: %s", bad.stdout)
		}
		status := handoffCLI(t, root, "queue", "status")
		if status.code != 0 || field(status.res.Items[0], "attempts").Str != "1" || field(field(status.res.Items[0], "liveAttempts").Arr[0], "holderStatus").Str != "FRESH_HOLDER" {
			t.Fatalf("status: %s", status.stdout)
		}
	})
}

func TestCALV0049_ChargedReasonsAndCleanHandoffReadback(t *testing.T) {
	t.Run("CAL-V0-049 retry debt reason handoff", func(t *testing.T) {
		root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
		id := planTicket(t, root, "charged-read", "P1", `["src/"]`)
		runOK := func(args ...string) run {
			t.Helper()
			x := handoffCLI(t, root, args...)
			if x.code != 0 {
				t.Fatalf("%v: %s", args, x.stdout)
			}
			return x
		}
		for i := 0; i <= 3; i++ {
			x := runOK("claim", id, "--holder", "qualification", "--stage", "integrate", "--request-id", fmt.Sprintf("claim-debt-%d", i))
			a, g := field(x.res.Items[0], "attemptId").Str, field(x.res.Items[0], "generation").Str
			show := runOK("ticket", "show", id)
			debt := field(show.res.Items[0], "retries")
			if field(debt, "charged").Str != fmt.Sprint(i) || field(field(debt, "byReason"), "RELEASED").Str != fmt.Sprint(i) || field(debt, "reasonHistory").Str != "COMPLETE" {
				t.Fatalf("prospective reasons: %s", show.stdout)
			}
			args := []string{"release", "--attempt", a, "--generation", g, "--request-id", fmt.Sprintf("release-debt-%d", i)}
			if i == 3 {
				args = append(args, "--reason", "HANDOFF", "--evidence", "local:review-result")
			}
			runOK(args...)
		}
		for _, args := range [][]string{{"ticket", "show", id}, {"plan", "preview"}, {"queue", "status", "--retries"}} {
			x := runOK(args...)
			v := x.res.Items[0]
			if args[0] == "plan" {
				v = field(v, "entries").Arr[0]
			}
			if args[0] == "queue" {
				v = field(v, "retries").Arr[0]
			}
			debt := field(v, "retries")
			if field(debt, "charged").Str != "3" || field(debt, "remaining").Str != "0" || field(debt, "exhausted").Bool {
				t.Fatalf("clean handoff debt: %s", x.stdout)
			}
		}
		x := runOK("claim", id, "--holder", "next-reviewer", "--stage", "review", "--request-id", "claim-clean-handoff")
		show := runOK("attempt", "show", field(x.res.Items[0], "attemptId").Str)
		if field(show.res.Items[0], "retryCount").Str != "3" || field(field(show.res.Items[0], "retryReasons"), "RELEASED").Str != "3" {
			t.Fatalf("handoff charged again: %s", show.stdout)
		}
	})
}
