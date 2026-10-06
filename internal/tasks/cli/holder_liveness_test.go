package cli_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// holderTTLPolicy sets policy holderLiveness.heartbeatTTLSeconds through
// policy update.
func holderTTLPolicy(t *testing.T, root, ttl string) {
	t.Helper()
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	loaded, e := intent.Load(repo.PrimaryWorktree)
	if e != nil {
		t.Fatal(e)
	}
	v, err := wire.Parse(loaded.Policy.Raw)
	if err != nil {
		t.Fatal(err)
	}
	v.Obj.Set("policyVersion", wire.String(fmt.Sprint(loaded.Policy.PolicyVersion.Uint64()+1)))
	v.Obj.Set("holderLiveness", wire.ObjectValue(wire.NewObject().Set("heartbeatTTLSeconds", wire.String(ttl))))
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "policy.json")
	fixture.Write(t, path, wire.EncodeFile(v))
	if x := handoffCLI(t, root, "policy", "update", "--request-id", "holder-ttl-"+ttl, "--expected-policy-version", string(loaded.Policy.PolicyVersion), "--file", path); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("policy update: %s", x.stdout)
	}
}

// TestCALV0115_PolicyTTLDrivesHolderReads: a claim-initialized signal about
// six minutes old under a renewed (live) work lease reads FRESH_HOLDER under
// the 600-second default and STALE_HOLDER under a 300-second policy TTL.
// The stale observation is derived on read: reads write nothing, the
// attempt keeps its lease, phase and reservation, and another holder still
// cannot claim the ticket. A current-generation heartbeat makes it fresh
// again without moving the lease; an older generation is fenced.
func TestCALV0115_PolicyTTLDrivesHolderReads(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	// leaseCLIStore claims at base+10m with a five-minute lease.
	root, claims := leaseCLIStore(t, 1, now.Add(-16*time.Minute))
	c := claims[0]
	repo, e := intent.Resolve(root)
	if e != nil {
		t.Fatal(e)
	}
	renewAt, err := wire.ParseTimestamp("recordedAt", now.Add(-3*time.Minute).Format("2006-01-02T15:04:05Z"))
	if err != nil {
		t.Fatal(err)
	}
	renew := transaction.LeaseRequest{Verb: transaction.LeaseRenew, AttemptID: c.AttemptID, Generation: c.Generation, LeaseMinutes: "30"}
	if report, err := store.Lease(context.Background(), repo, mutation.Binding{ID: "tester", Role: "OWNER"}, store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "renew-ttl", Root: root, Lease: renew, Derive: store.NoScopeDeriver}, renewAt); err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("renew: %+v %v", report, err)
	}
	show := func(want, ttl string) (string, string) {
		t.Helper()
		before := fixture.TreeSnapshot(t, repo.StateDir)
		read := handoffCLI(t, root, "attempt", "show", c.AttemptID)
		status := handoffCLI(t, root, "queue", "status")
		if !fixture.SameTree(before, fixture.TreeSnapshot(t, repo.StateDir)) {
			t.Fatalf("holder reads wrote state under ttl %s", ttl)
		}
		if read.code != 0 || status.code != 0 {
			t.Fatalf("reads: %s %s", read.stdout, status.stdout)
		}
		item := read.res.Items[0]
		live := field(status.res.Items[0], "liveAttempts").Arr
		if field(item, "holderStatus").Str != want || field(item, "heartbeatTTLSeconds").Str != ttl || field(status.res.Items[0], "attempts").Str != "1" || len(live) != 1 || field(live[0], "holderStatus").Str != want || field(live[0], "heartbeatTTLSeconds").Str != ttl {
			t.Fatalf("want %s ttl %s: %s\n%s", want, ttl, read.stdout, status.stdout)
		}
		return field(item, "phase").Str, field(field(item, "lease"), "expiresAt").Str
	}
	phase, expires := show("FRESH_HOLDER", "600")
	holderTTLPolicy(t, root, "300")
	if p, x := show("STALE_HOLDER", "300"); p != phase || x != expires || phase != "RUNNING" {
		t.Fatalf("stale changed the attempt: %s %s -> %s %s", phase, expires, p, x)
	}
	if other := handoffCLI(t, root, "claim", c.Ticket, "--holder", "coordinator", "--request-id", "steal"); other.res.Outcome == wire.OutcomeOK || !hasCode(other.res, wire.CodeAttemptLive) {
		t.Fatalf("stale holder freed the ticket: %s", other.stdout)
	}
	bad := handoffCLI(t, root, "attempt", "heartbeat", "--attempt", c.AttemptID, "--generation", "0", "--request-id", "heartbeat-old")
	if !hasCode(bad.res, wire.CodeFenced) {
		t.Fatalf("older generation heartbeat: %s", bad.stdout)
	}
	show("STALE_HOLDER", "300")
	if h := handoffCLI(t, root, "attempt", "heartbeat", "--attempt", c.AttemptID, "--generation", string(c.Generation), "--request-id", "heartbeat-now"); h.code != 0 {
		t.Fatalf("heartbeat: %s", h.stdout)
	}
	if p, x := show("FRESH_HOLDER", "300"); p != phase || x != expires {
		t.Fatalf("heartbeat moved the lease: %s %s -> %s %s", phase, expires, p, x)
	}
}

// TestCALV0116_StaleHolderCoordinatorHandoff: under a steady policy TTL a
// stale holder stays RUNNING with its reservation until a coordinator
// chooses an evidence HANDOFF release through the ordinary fenced writer;
// that release charges no retry. release --help names STALE_HOLDER as
// advisory input, not authority.
func TestCALV0116_StaleHolderCoordinatorHandoff(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	root, claims := leaseCLIStoreWith(t, 1, now.Add(-16*time.Minute), func(p *wire.Object) {
		p.Set("holderLiveness", wire.ObjectValue(wire.NewObject().Set("heartbeatTTLSeconds", wire.String("300"))))
	}, "implement")
	c := claims[0]
	repo, e := intent.Resolve(root)
	if e != nil {
		t.Fatal(e)
	}
	renewAt, err := wire.ParseTimestamp("recordedAt", now.Add(-3*time.Minute).Format("2006-01-02T15:04:05Z"))
	if err != nil {
		t.Fatal(err)
	}
	renew := transaction.LeaseRequest{Verb: transaction.LeaseRenew, AttemptID: c.AttemptID, Generation: c.Generation, LeaseMinutes: "30"}
	if report, err := store.Lease(context.Background(), repo, mutation.Binding{ID: "tester", Role: "OWNER"}, store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "renew-ttl", Root: root, Lease: renew, Derive: store.NoScopeDeriver}, renewAt); err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("renew: %+v %v", report, err)
	}
	before := fixture.TreeSnapshot(t, repo.StateDir)
	read := handoffCLI(t, root, "attempt", "show", c.AttemptID)
	if field(read.res.Items[0], "holderStatus").Str != "STALE_HOLDER" || field(read.res.Items[0], "heartbeatTTLSeconds").Str != "300" || field(read.res.Items[0], "phase").Str != "RUNNING" {
		t.Fatalf("stale read: %s", read.stdout)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, repo.StateDir)) {
		t.Fatal("stale read wrote state")
	}
	rel := handoffCLI(t, root, "release", "--attempt", c.AttemptID, "--generation", string(c.Generation), "--reason", wire.CodeHandoff, "--evidence", "coordinator-stale-holder", "--request-id", "coordinator-handoff")
	if rel.res.Outcome != wire.OutcomeOK {
		t.Fatalf("coordinator handoff: %s", rel.stdout)
	}
	after := handoffCLI(t, root, "attempt", "show", c.AttemptID)
	if field(after.res.Items[0], "holderStatus").Str != "TERMINAL" {
		t.Fatalf("released read: %s", after.stdout)
	}
	next := handoffCLI(t, root, "claim", c.Ticket, "--holder", "successor", "--stage", "implement", "--request-id", "successor-claim")
	if next.res.Outcome != wire.OutcomeOK {
		t.Fatalf("successor claim: %s", next.stdout)
	}
	show := handoffCLI(t, root, "ticket", "show", c.Ticket)
	if field(field(show.res.Items[0], "retries"), "charged").Str != "0" {
		t.Fatalf("handoff charged a retry: %s", show.stdout)
	}
	help := handoffCLI(t, root, "release", "--help")
	found := false
	for _, line := range field(help.res.Items[0], "handoffPreconditions").Arr {
		if strings.Contains(line.Str, "STALE_HOLDER") && strings.Contains(line.Str, "not release authority") {
			found = true
		}
	}
	if !found {
		t.Fatalf("release help: %s", help.stdout)
	}
}
