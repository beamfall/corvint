package transaction

import (
	"bytes"
	"encoding/json"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"strings"
	"testing"
)

// Pure fixtures produce no archive or native execution outputs.
func untouchedContext(t *testing.T) (leaseContext, *snapshot.Attempt, *ticket.Record) {
	t.Helper()
	q, _ := intent.DecodeQueue(fixture.QueueBytes())
	v := fixture.PolicyValue()
	command := object("argv", wire.Strings([]string{"/bin/false"}), "cwd", s("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", s("3"))
	v.Obj.Set("pools", wire.Array(object("id", s("db"), "members", wire.Strings([]string{"a", "b"}), "memberConfig", object("a", object("cleanup", command)))))
	p, err := intent.DecodePolicy(wire.EncodeFile(v))
	if err != nil {
		t.Fatal(err)
	}
	rec := fixture.Ticket("AT-01")
	tickets, _ := ticket.NewInventory(q.QueueID, []*ticket.Record{rec})
	inv, _ := NewInventory(nil, nil)
	l := &LeaseRequest{Verb: LeaseClaim, TicketID: rec.TicketID.Raw, Holder: "builder", LeaseMinutes: "60", Pool: "db", Stage: "review"}
	c := leaseContext{r: admin(Lease, "claim"), l: l, seq: "2", in: Input{Inventory: inv, RecordedAt: timestamp, LeaseFacts: LeaseFacts{AttemptID: "attempt:acme:main:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", BaseCommit: strings.Repeat("a", 40)}}, st: inputState{queue: q, policy: p, tickets: tickets, head: &snapshot.Head{Generation: "0", LastSeq: "1"}, pools: &snapshot.PoolState{QueueID: q.QueueID}, reservations: &snapshot.ReservationSet{QueueID: q.QueueID}}}
	c.r.Lease = l
	a, err := c.admitted(rec, nil, &snapshot.Scope{Source: "REQUESTED", Resources: []ticket.Resource{{Class: "PATH", Key: "src"}}})
	if err != nil {
		t.Fatal(err)
	}
	posts := map[string][]byte{}
	if err := c.poolPosts(a, posts); err != nil {
		t.Fatal(err)
	}
	c.st.pools, err = snapshot.DecodePools(posts["pools.json"])
	if err != nil {
		t.Fatal(err)
	}
	c.st.attempts = map[string]*snapshot.Attempt{a.AttemptID: a}
	c.st.reservations.Entries = []snapshot.ReservationEntry{{AttemptID: a.AttemptID, Generation: a.Generation, TicketID: a.TicketID, TicketRevision: a.TicketRevision, Resources: a.Scope.Resources, Workers: "1", State: "ACTIVE", CreatedSeq: "2", Coverage: "QUALIFIED"}}
	c.seq = "3"
	c.l = &LeaseRequest{Verb: LeaseRelease, AttemptID: a.AttemptID, Generation: a.Generation, LaneUntouched: true, Evidence: "local:unused"}
	c.r.Lease = c.l
	c.r.RequestID = "release"
	return c, a, rec
}
func TestPoolLaneUntouched_Origin(t *testing.T) {
	t.Run("ordinary-supervisor-attach", func(t *testing.T) {
		c, a, _ := untouchedContext(t)
		before, err := a.Encode()
		if err != nil {
			t.Fatal(err)
		}
		f := SupervisorChange{Action: "ATTACH", ProgramID: "program", OwnerPID: 1, OwnerStarted: "start", Expected: wire.Sum(before)}
		raw, err := json.Marshal(f)
		if err != nil {
			t.Fatal(err)
		}
		p := snapshot.Programs{Profile: "taskman-programs/0", QueueID: c.r.QueueID, Entries: []snapshot.Program{{ID: "program", Profile: snapshot.SupervisedProfile, OwnerPID: 1, OwnerStarted: "start", Epoch: 1, ConfigSHA256: string(a.ConfigSha256), Phase: "ADMITTED", CurrentAttempt: a.AttemptID, CurrentGeneration: string(a.Generation)}}}
		c.in.Programs, err = p.Encode()
		if err != nil {
			t.Fatal(err)
		}
		c.in.LeaseFacts.Program = raw
		c.l = &LeaseRequest{Verb: LeaseSupervisor, AttemptID: a.AttemptID, Generation: a.Generation, Evidence: string(wire.Sum(raw))}
		c.r.Lease = c.l
		out := planSupervisor(c)
		if out.result != nil {
			t.Fatal(out.result)
		}
		next, err := snapshot.DecodeAttempt(out.posts[attemptPath(a.AttemptID)])
		if err != nil || next.Supervision == nil || next.DirectPoolAdmission == nil {
			t.Fatal("origin broke ordinary attach", err)
		}
	})

	t.Run("CAL-V0-067", func(t *testing.T) {
		c, a, rec := untouchedContext(t)
		if a.DirectPoolAdmission == nil {
			t.Fatal("direct witness absent")
		}
		c.l = &LeaseRequest{Verb: LeaseClaim, Holder: "builder", LeaseMinutes: "60", Pool: "db", Stage: "review"}
		c.r.Lease = c.l
		c.r.RequestID = "retry"
		c.st.head.Generation = "1"
		retry, err := c.admitted(rec, a, a.Scope)
		if err != nil {
			t.Fatal(err)
		}
		if retry.DirectPoolAdmission != nil || retry.LaneUntouchedAttestation != nil {
			t.Fatal("retry inherited witness")
		}
		// A prepared/health origin never receives a direct admission witness.
		en := c.st.pools.Entries[0]
		en.State, en.CommandKind, en.CommandRevision = "PREPARING", "health", strings.Repeat("d", 40)
		en.RequestSha256 = PoolClaimBinding(c.l, c.st.queue.QueueID)
		c.st.pools.Entries = []snapshot.PoolEntry{en}
		obs := snapshot.PoolObservation{AllocationID: en.AllocationID, DefinitionSha256: en.DefinitionSha256, Kind: "health", Revision: en.CommandRevision, Tree: en.CommandRevision, Class: "EXIT_ZERO", Passed: true, GroupClean: true, OutputSha256: wire.Sum(nil), EnvironmentSha256: wire.Sum(nil)}
		c.in.LeaseFacts.Pool = PoolFacts{AllocationID: en.AllocationID, Observation: obs.Encode()}
		prepared, err := c.admitted(rec, nil, a.Scope)
		if err != nil || prepared.DirectPoolAdmission != nil {
			t.Fatal("prepared origin", err)
		}
		c.in.LeaseFacts.Pool = PoolFacts{}
		c.l.Pool = ""
		plain, err := c.admitted(rec, nil, a.Scope)
		if err != nil || plain.DirectPoolAdmission != nil {
			t.Fatal("unpooled origin", err)
		}
	})
}
func TestPoolLaneUntouched_Eligibility(t *testing.T) {
	for name, change := range map[string]func(*leaseContext, *snapshot.Attempt){
		"legacy":             func(c *leaseContext, a *snapshot.Attempt) { a.DirectPoolAdmission = nil },
		"wrong-role":         func(c *leaseContext, a *snapshot.Attempt) { c.r.Actor.Role = "WORKER" },
		"acceptance":         func(c *leaseContext, a *snapshot.Attempt) { a.TicketRevision = "999" },
		"policy":             func(c *leaseContext, a *snapshot.Attempt) { a.PolicySha256 = wire.Sum(nil) },
		"config":             func(c *leaseContext, a *snapshot.Attempt) { a.ConfigSha256 = wire.Sum(nil) },
		"renewed":            func(c *leaseContext, a *snapshot.Attempt) { a.Lease.GrantedSeq = "3" },
		"expired":            func(c *leaseContext, a *snapshot.Attempt) { a.Lease.ExpiresAt = c.in.RecordedAt },
		"generation":         func(c *leaseContext, a *snapshot.Attempt) { c.l.Generation = "9" },
		"terminal":           func(c *leaseContext, a *snapshot.Attempt) { a.Phase = "CANCELLED" },
		"built":              func(c *leaseContext, a *snapshot.Attempt) { a.Phase = "BUILT" },
		"candidate":          func(c *leaseContext, a *snapshot.Attempt) { x := strings.Repeat("b", 40); a.CandidateTreeOid = &x },
		"worker":             func(c *leaseContext, a *snapshot.Attempt) { x := "worktree"; a.WorktreePath = &x },
		"spawn":              func(c *leaseContext, a *snapshot.Attempt) { a.SpawnNoExecCount = "1" },
		"supervisor":         func(c *leaseContext, a *snapshot.Attempt) { a.Supervisor = &snapshot.Supervisor{} },
		"supervision":        func(c *leaseContext, a *snapshot.Attempt) { a.Supervision = &snapshot.Supervision{} },
		"lane":               func(c *leaseContext, a *snapshot.Attempt) { a.Lane = &snapshot.Lane{} },
		"failed":             func(c *leaseContext, a *snapshot.Attempt) { a.RetryAccounting.FailedOrUnknown = true },
		"unknown-accounting": func(c *leaseContext, a *snapshot.Attempt) { a.RetryAccounting = nil },
		"gate":               func(c *leaseContext, a *snapshot.Attempt) { a.GateResults = []string{string(wire.Sum(nil))} },
		"review":             func(c *leaseContext, a *snapshot.Attempt) { a.Reviews = []string{string(wire.Sum(nil))} },
		"manifest":           func(c *leaseContext, a *snapshot.Attempt) { x := wire.Sum(nil); a.ManifestSha256 = &x },
		"pending":            func(c *leaseContext, a *snapshot.Attempt) { a.PendingEffects = []string{string(wire.Sum(nil))} },
		"runner":             func(c *leaseContext, a *snapshot.Attempt) { c.st.pools.Entries[0].RunnerPID = "1" },
		"start":              func(c *leaseContext, a *snapshot.Attempt) { c.st.pools.Entries[0].RunnerStarted = "start" },
		"command":            func(c *leaseContext, a *snapshot.Attempt) { c.st.pools.Entries[0].CommandKind = "health" },
		"revision": func(c *leaseContext, a *snapshot.Attempt) {
			c.st.pools.Entries[0].CommandRevision = strings.Repeat("c", 40)
		},
		"observation": func(c *leaseContext, a *snapshot.Attempt) {
			x := wire.Sum(nil)
			c.st.pools.Entries[0].ObservationSha256 = &x
		},
		"cleanup":           func(c *leaseContext, a *snapshot.Attempt) { c.st.pools.Entries[0].CleanupPassed = true },
		"changed":           func(c *leaseContext, a *snapshot.Attempt) { c.st.pools.Entries[0].ChangedSeq = "3" },
		"holder":            func(c *leaseContext, a *snapshot.Attempt) { c.st.pools.Entries[0].Holder = "other" },
		"stage":             func(c *leaseContext, a *snapshot.Attempt) { c.st.pools.Entries[0].Stage = "integrate" },
		"tuple":             func(c *leaseContext, a *snapshot.Attempt) { c.st.pools.Entries[0].AllocatedSeq = "1" },
		"missing-occupancy": func(c *leaseContext, a *snapshot.Attempt) { c.st.pools.Entries = nil },
	} {
		t.Run(name, func(t *testing.T) {
			c, a, _ := untouchedContext(t)
			change(&c, a)
			before, _ := c.st.pools.Encode()
			out := planRelease(c)
			after, _ := c.st.pools.Encode()
			if len(out.posts) != 0 || !bytes.Equal(before, after) {
				t.Fatal("refusal freed occupancy", out)
			}
			if out.result == nil && (out.effect == nil || out.effect.outcome == "COMPLETED") {
				t.Fatal("invalid accepted", out)
			}
		})
	}
}
func TestPoolLaneUntouched_Release(t *testing.T) {
	t.Run("only-exact-occupancy", func(t *testing.T) {
		c, _, _ := untouchedContext(t)
		other := c.st.pools.Entries[0]
		other.MemberID = "b"
		other.AllocationID = wire.Sum([]byte("other"))
		other.DefinitionSha256 = c.st.policy.MemberDefinition("db", "b")
		other.State = "QUARANTINED"
		other.Reason = "preserve"
		c.st.pools.Entries = append(c.st.pools.Entries, other)
		out := planRelease(c)
		if out.result != nil {
			t.Fatal(out.result)
		}
		got, err := snapshot.DecodePools(out.posts["pools.json"])
		if err != nil || len(got.Entries) != 1 {
			t.Fatal("wrong occupancy count", err)
		}
		want := &snapshot.PoolState{QueueID: c.st.queue.QueueID, Entries: []snapshot.PoolEntry{other}}
		raw, err := want.Encode()
		if err != nil || !bytes.Equal(out.posts["pools.json"], raw) {
			t.Fatal("unrelated occupancy changed", err)
		}
	})

	t.Run("CAL-V0-067", func(t *testing.T) {
		for _, reason := range []string{"", wire.CodeHandoff, wire.CodeReviewReturned} {
			c, a, _ := untouchedContext(t)
			c.l.Reason = reason
			out := planRelease(c)
			if out.result != nil {
				t.Fatal(out.result)
			}
			next, err := snapshot.DecodeAttempt(out.posts[attemptPath(a.AttemptID)])
			if err != nil {
				t.Fatal(err)
			}
			pools, err := snapshot.DecodePools(out.posts["pools.json"])
			if err != nil {
				t.Fatal(err)
			}
			rs, err := snapshot.DecodeReservations(out.posts["reservations.json"])
			if err != nil {
				t.Fatal(err)
			}
			if next.Phase != "CANCELLED" || next.Quiescence != "FENCED" || next.LaneUntouchedAttestation == nil || len(pools.Entries) != 0 || len(rs.Entries) != 0 {
				t.Fatal("incomplete transition")
			}
			if reason == "" && next.RetryAccounting.Disposition != "NONE" {
				t.Fatal("implicit refund")
			}
		}
		c, a, _ := untouchedContext(t)
		c.l.LaneUntouched = false
		c.l.Evidence = ""
		out := planRelease(c)
		if out.result != nil {
			t.Fatal(out.result)
		}
		p, _ := snapshot.DecodePools(out.posts["pools.json"])
		next, _ := snapshot.DecodeAttempt(out.posts[attemptPath(a.AttemptID)])
		if len(p.Entries) != 1 || p.Entries[0].State != "QUARANTINED" || next.LaneUntouchedAttestation != nil {
			t.Fatal("default changed")
		}
		q := c.st.queue.QueueID
		l := LeaseRequest{Verb: LeaseRelease, AttemptID: a.AttemptID, Generation: a.Generation}
		v, err := leaseValue(&l, q)
		want := `{"attemptId":"attempt:acme:main:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","base":null,"branch":null,"generation":"1","holder":null,"leaseMinutes":null,"reason":null,"scope":null,"ticketId":null,"verb":"RELEASE","wholeRepository":false}`
		if err != nil || string(wire.Encode(v)) != want {
			t.Fatal("legacy preimage changed", err, string(wire.Encode(v)))
		}
		l.LaneUntouched = true
		if _, err := leaseValue(&l, q); err == nil {
			t.Fatal("missing evidence")
		}
		l.Evidence = "local:unused"
		v, err = leaseValue(&l, q)
		if err != nil || !bytes.Contains(wire.Encode(v), []byte(snapshot.ProfileLaneUntouched)) {
			t.Fatal("conditional preimage", err)
		}
		for _, verb := range []string{LeaseRenew, LeaseClaim, LeaseReap} {
			l.Verb = verb
			if _, err := leaseValue(&l, q); err == nil {
				t.Fatal("flag on other verb", verb)
			}
		}
	})
}
func TestPoolLaneUntouched_Programs(t *testing.T) {
	for _, name := range []string{"absent", "unrelated", "ADMITTED", "SPAWNING", "RUNNING", "FINISHED", "owner-released", "missing-generation", "different-generation", "missing-attempt", "inventory-only", "bytes-only", "hash-mismatch", "foreign", "unknown", "duplicate", "malformed"} {
		t.Run(name, func(t *testing.T) {
			c, a, _ := untouchedContext(t)
			if name != "absent" {
				p := snapshot.Programs{Profile: "taskman-programs/0", QueueID: c.r.QueueID, Entries: []snapshot.Program{{ID: "program", Profile: snapshot.SupervisedProfile, OwnerPID: 1, OwnerStarted: "start", Epoch: 1, ConfigSHA256: string(wire.Sum(nil)), Phase: "ADMITTED", CurrentAttempt: a.AttemptID, CurrentGeneration: string(a.Generation)}}}
				x := &p.Entries[0]
				switch name {
				case "unrelated":
					x.CurrentAttempt = "attempt:acme:main:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
					x.CurrentGeneration = "2"
				case "SPAWNING", "RUNNING", "FINISHED":
					x.Phase = name
				case "owner-released":
					x.OwnerReleased = true
				case "missing-generation":
					x.CurrentGeneration = ""
				case "different-generation":
					x.CurrentGeneration = "2"
				case "missing-attempt":
					x.CurrentAttempt = ""
				case "foreign":
					p.QueueID = "queue:other:main"
				}
				raw, err := p.Encode()
				if err != nil {
					t.Fatal(err)
				}
				switch name {
				case "unknown":
					raw = bytes.Replace(raw, []byte(`"entries":`), []byte(`"unknown":true,"entries":`), 1)
				case "duplicate":
					raw = bytes.Replace(raw, []byte(`"phase":"ADMITTED"`), []byte(`"phase":"ADMITTED","phase":"ADMITTED"`), 1)
				case "malformed":
					raw = []byte("bad")
				}
				c.in.Programs = raw
				if name != "bytes-only" {
					c.in.Inventory.files["programs.json"] = bytesEntry("programs.json", raw)
				}
				if name == "inventory-only" {
					c.in.Programs = nil
				}
				if name == "hash-mismatch" {
					c.in.Programs = append(c.in.Programs, ' ')
				}
			}
			err := c.verifyLaneUntouched(a)
			if (name == "absent" || name == "unrelated") != (err == nil) {
				t.Fatal("eligibility", err)
			}
		})
	}
}
