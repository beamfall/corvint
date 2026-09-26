package cli_test

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func TestTMV0008_AS11_ActorEnvironmentCompatibility(t *testing.T) {
	tests := []struct {
		name, primary, legacy, want string
	}{
		{"new only", "new-actor", "", "new-actor"},
		{"legacy only", "", "legacy-actor", "legacy-actor"},
		{"equal", "same-actor", "same-actor", "same-actor"},
		{"OS user fallback", "", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CORVINT_TASKS_ACTOR", tc.primary)
			t.Setenv("ATM_ACTOR", tc.legacy)
			r := fixture.TempRepo(t)
			fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
			fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
			x := atm(t, r.Root, nil, "init")
			if x.res.Outcome != wire.OutcomeOK {
				t.Fatalf("init: %+v", x.res)
			}
			receipt, err := os.ReadFile(filepath.Join(r.StateDir, "receipts", "000000000001.json"))
			if err != nil {
				t.Fatal(err)
			}
			want := tc.want
			if want == "" {
				u, err := user.Current()
				if err != nil {
					t.Fatalf("OS-user fallback unavailable: %v", err)
				}
				want = u.Username
			}
			if !strings.Contains(string(receipt), `"id":"`+want+`"`) {
				t.Fatalf("receipt does not record actor %q: %s", want, receipt)
			}
		})
	}
}

func TestTMV0008_AS11_ConflictingActorEnvironmentLeavesFixtureUntouched(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "new-actor")
	t.Setenv("ATM_ACTOR", "legacy-actor")
	r := fixture.TempRepo(t)
	fixture.WriteState(t, r)
	fixture.WriteIntent(t, r, fixture.Ticket("A"))
	stateBefore := fixture.TreeSnapshot(t, r.StateDir)
	intentBefore := fixture.TreeSnapshot(t, r.IntentDir)

	x := atm(t, r.Root, nil, "ticket", "create", "--request-id", "conflict", "--payload-stdin")
	if x.res.Outcome != wire.OutcomeError || !hasCode(x.res, wire.CodeMalformed) {
		t.Fatalf("conflict: %+v", x.res)
	}
	if len(x.res.Warnings) != 1 || !strings.Contains(x.res.Warnings[0], "disagree") {
		t.Fatalf("conflict warning: %v", x.res.Warnings)
	}
	fixture.AssertUntouched(t, r, stateBefore, intentBefore, "conflicting actor environment")
}

func TestTMV0008_AS07_ConflictingActorEnvironmentDoesNotGateReads(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "new-actor")
	t.Setenv("ATM_ACTOR", "legacy-actor")
	r := fixture.TempRepo(t)
	fixture.WriteState(t, r)
	fixture.WriteIntent(t, r, fixture.Ticket("A"))
	stateBefore := fixture.TreeSnapshot(t, r.StateDir)
	intentBefore := fixture.TreeSnapshot(t, r.IntentDir)

	x := atm(t, r.Root, nil, "ticket", "list")
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("read with conflicting actor environment: %+v", x.res)
	}
	fixture.AssertUntouched(t, r, stateBefore, intentBefore, "read with conflicting actor environment")
}

// TestTMV0008_AS11_InitMakesTheStoreReadable is the end of the
// UNINITIALIZED refusal: after `atm init` the read verbs answer OK against a
// real head instead of refusing, and a second init refuses without writing.
func TestTMV0008_AS11_InitMakesTheStoreReadable(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())

	// Before init every read refuses: that is the state this slice ends.
	before := atm(t, r.Root, nil, "queue", "status")
	if before.res.Outcome != wire.OutcomeRefused {
		t.Fatalf("an uninitialized store must refuse reads: %+v", before.res)
	}

	x := atm(t, r.Root, nil, "init")
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	if got := field(x.res.Items[0], "receipt").Str; got != "000000000001.json" {
		t.Errorf("receipt = %q, want the genesis receipt", got)
	}
	// Decision 0003: the binding is recorded, never claimed as authenticated.
	if got := field(x.res.Items[0], "actorAuthentication").Str; got != "NOT_OBSERVED" {
		t.Errorf("actorAuthentication = %q, want NOT_OBSERVED", got)
	}

	after := atm(t, r.Root, nil, "queue", "status")
	if after.res.Outcome != wire.OutcomeOK {
		t.Fatalf("queue status after init: %+v", after.res)
	}
	if after.res.Snapshot == nil {
		t.Fatal("a read of an initialized store must carry a snapshot")
	}

	again := atm(t, r.Root, nil, "init")
	if again.res.Outcome != wire.OutcomeRefused {
		t.Errorf("a second init must refuse, got %+v", again.res)
	}
}

func TestTMV0009_AS11_InvalidInitRoleThenRetry(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	bad := atm(t, r.Root, nil, "init", "--role", "INVALID")
	if bad.res.Outcome == wire.OutcomeOK {
		t.Fatal("invalid role succeeded")
	}
	if _, err := os.Lstat(r.StateDir); !os.IsNotExist(err) {
		t.Fatalf("invalid role stranded state: %v", err)
	}
	good := atm(t, r.Root, nil, "init", "--role", "OWNER")
	if good.res.Outcome != wire.OutcomeOK {
		t.Fatalf("corrected retry: %+v", good.res)
	}
}

// TestCTSV0001_InitRefusesOverCommittedTickets pins V1-0323 end to end: init
// over committed tickets refuses with a code and writes nothing, and the same
// init succeeds once the intent store holds no records.
func TestCTSV0001_InitRefusesOverCommittedTickets(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.WriteIntent(t, r, fixture.Ticket("A"))

	refused := atm(t, r.Root, nil, "init")
	if refused.res.Outcome != wire.OutcomeRefused || !hasCode(refused.res, wire.CodeIntentDiverged) {
		t.Fatalf("init over a committed ticket: %+v", refused.res)
	}
	if !strings.Contains(strings.Join(refused.res.Warnings, "\n"), "tickets/A.json") {
		t.Errorf("refusal does not name the record: %v", refused.res.Warnings)
	}
	if _, err := os.Lstat(r.StateDir); !os.IsNotExist(err) {
		t.Fatalf("a refused init created the state dir: %v", err)
	}

	if err := os.Remove(filepath.Join(r.IntentDir, "tickets", "A.json")); err != nil {
		t.Fatal(err)
	}
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init over an empty intent store: %+v", x.res)
	}
}
