//go:build darwin || linux

package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ATR-V0-126 (V1-1834, issue 1148): `dispatch salvage --attempt A` writes
// the secret-screened hand-off note of the worker holding attempt A, for a
// running attempt whose session something other than the dispatcher
// stopped and for an ended attempt the dispatcher handed off; it never
// changes the attempt, and refuses an attempt held by no worker of the
// program.
func TestATRV0126_SalvageAttemptWritesScreenedNote(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	tk := planTicket(t, root, "salvage", "P1", `["src/"]`)
	config, dir := readerCLIConfig(t, root, `printf '{}'`)
	claim := atm(t, root, nil, "claim", tk, "--holder", "w-stop", "--request-id", "claim-salvage", "--stage", "implement")
	if claim.res.Outcome != wire.OutcomeOK {
		t.Fatalf("claim %s", claim.stdout)
	}
	attempt, generation := field(claim.res.Items[0], "attemptId").Str, field(claim.res.Items[0], "generation").Str
	secret := "gh" + "p_" + strings.Repeat("a1b2c3d4", 4)
	wdir := filepath.Join(dir, "workers", "w-stop")
	if err := os.MkdirAll(filepath.Join(wdir, "evidence"), 0o700); err != nil {
		t.Fatal(err)
	}
	line, _ := json.Marshal(map[string]any{"type": "assistant", "message": map[string]any{"content": []any{map[string]any{"type": "text", "text": "left off at step 3\n" + secret}}}, "parent_tool_use_id": nil})
	if err := os.WriteFile(filepath.Join(wdir, "stdout.log"), append(line, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wdir, "evidence", "progress.md"), []byte("done: a\n"+secret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	salvage := func(want string) wire.Value {
		t.Helper()
		x := atm(t, root, nil, "dispatch", "salvage", "--program", "prog", "--config", config, "--attempt", attempt)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("salvage %s", x.stdout)
		}
		o := x.res.Items[0]
		if field(o, "worker").Str != "w-stop" || field(o, "workerState").Str != "STOPPED" || field(o, "ticket").Str != tk || field(o, "attemptPhase").Str != want ||
			field(o, "redactedLines").Str != "2" || field(o, "messages").Str != "1" || field(o, "files").Str != "1" {
			t.Fatalf("salvage result %s", x.stdout)
		}
		raw, err := os.ReadFile(field(o, "note").Str)
		if err != nil || strings.Contains(string(raw), secret) || !strings.Contains(string(raw), "left off at step 3") {
			t.Fatalf("note %s %v", raw, err)
		}
		n, err := dispatch.ReadHandoffNote(dispatch.HandoffNotePath(dir, "w-stop"))
		if err != nil || n.Worker != "w-stop" || n.Ticket != tk || n.Role != dispatch.UsageUnknown {
			t.Fatalf("note %+v %v", n, err)
		}
		return o
	}
	// The attempt still runs natively; its session is gone.
	salvage("RUNNING")
	if x := atm(t, root, nil, "attempt", "show", attempt); field(x.res.Items[0], "phase").Str != "RUNNING" {
		t.Fatalf("salvage changed the attempt: %s", x.stdout)
	}
	// An attempt the dispatcher handed off keeps its recorded holder.
	rel := atm(t, root, nil, "release", "--attempt", attempt, "--generation", generation, "--reason", wire.CodeHandoff, "--evidence", "dispatch:w-stop", "--request-id", "release-salvage")
	if rel.res.Outcome != wire.OutcomeOK {
		t.Fatalf("release %s", rel.stdout)
	}
	ended := field(atm(t, root, nil, "attempt", "show", attempt).res.Items[0], "phase").Str
	if ended == "RUNNING" {
		t.Fatal("the attempt did not end")
	}
	salvage(ended)

	// A holder that is no worker of the program, and a missing attempt.
	other := planTicket(t, root, "other", "P1", `["docs/"]`)
	c2 := atm(t, root, nil, "claim", other, "--holder", "someone", "--request-id", "claim-other")
	if c2.res.Outcome != wire.OutcomeOK {
		t.Fatalf("claim %s", c2.stdout)
	}
	if x := atm(t, root, nil, "dispatch", "salvage", "--program", "prog", "--config", config, "--attempt", field(c2.res.Items[0], "attemptId").Str); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("salvage of a foreign holder %s", x.stdout)
	}
	if x := atm(t, root, nil, "dispatch", "salvage", "--program", "prog", "--config", config); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("salvage without --attempt %s", x.stdout)
	}
}

// ERG-V0-028 (V1-2246): `dispatch salvage` names the commit the attempt's
// branch holds, and `gate packet` binds the checkpoint to it; an attempt
// whose branch does not resolve leaves the note without a commit.
func TestERGV0028_SalvagedNoteBindsPacketCheckpoint(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, _ := ergStore(t)
	tk := planTicket(t, root, "checkpointed", "P1", `["src/"]`)
	config, dir := readerCLIConfig(t, root, `printf '{}'`)
	c1, _ := packetCommit(t, root, "c1", map[string]string{"src/a.go": "package src\n"})
	git(t, root, "branch", "lane-erg", c1)
	c2, _ := packetCommit(t, root, "c2", map[string]string{"src/a.go": "package src\n\nfunc A() {}\n"})
	if err := os.MkdirAll(filepath.Join(dir, "workers", "w-lane", "evidence"), 0o700); err != nil {
		t.Fatal(err)
	}
	salvage := func(branch, holder string) *dispatch.HandoffNote {
		t.Helper()
		claim := atm(t, root, nil, "claim", tk, "--holder", holder, "--branch", branch, "--request-id", "claim-"+branch, "--stage", "implement")
		if claim.res.Outcome != wire.OutcomeOK {
			t.Fatalf("claim %s", claim.stdout)
		}
		attempt, generation := field(claim.res.Items[0], "attemptId").Str, field(claim.res.Items[0], "generation").Str
		if err := os.MkdirAll(filepath.Join(dir, "workers", holder), 0o700); err != nil {
			t.Fatal(err)
		}
		if x := atm(t, root, nil, "dispatch", "salvage", "--program", "prog", "--config", config, "--attempt", attempt); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("salvage %s", x.stdout)
		}
		if x := atm(t, root, nil, "release", "--attempt", attempt, "--generation", generation, "--reason", wire.CodeHandoff, "--evidence", "dispatch:"+holder, "--request-id", "release-"+branch); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("release %s", x.stdout)
		}
		n, err := dispatch.ReadHandoffNote(dispatch.HandoffNotePath(dir, holder))
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := salvage("no-such-branch", "w-none"); n.Commit != "" {
		t.Fatalf("an unresolved branch named commit %q", n.Commit)
	}
	n := salvage("lane-erg", "w-lane")
	if n.Commit != c1 {
		t.Fatalf("note commit %q, want %s", n.Commit, c1)
	}
	v, x := packetOf(t, root, tk, "--commit", c2, "--checkpoint", dispatch.HandoffNotePath(dir, "w-lane"))
	cp := field(v, "checkpoint")
	if field(cp, "commitBinding").Str != "OBSERVED" || field(cp, "commit").Str != c1 || field(cp, "commitRelation").Str != cli.CheckpointAncestor {
		t.Fatalf("checkpoint: %s", x.stdout)
	}
}
