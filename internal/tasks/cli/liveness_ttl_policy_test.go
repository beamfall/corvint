package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// livenessTimeline drives backdated renews and heartbeats through the
// ordinary lease writer and reads back what attempt show reports.
type livenessTimeline struct {
	t      *testing.T
	root   string
	repo   *intent.Repository
	claims []*store.Report
}

func newLivenessTimeline(t *testing.T, root string, claims []*store.Report) *livenessTimeline {
	t.Helper()
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	return &livenessTimeline{t: t, root: root, repo: repo, claims: claims}
}

// write records verb (renew with leaseMinutes, or heartbeat) for holder i at.
func (l *livenessTimeline) write(verb string, i int, at time.Time, leaseMinutes string) {
	l.t.Helper()
	c := l.claims[i]
	recorded, err := wire.ParseTimestamp("recordedAt", at.Format("2006-01-02T15:04:05Z"))
	if err != nil {
		l.t.Fatal(err)
	}
	req := transaction.LeaseRequest{Verb: verb, AttemptID: c.AttemptID, Generation: c.Generation, LeaseMinutes: wire.Size(leaseMinutes)}
	id := fmt.Sprintf("%s-%d-%d", strings.ToLower(verb), i, at.Unix())
	report, err := store.Lease(context.Background(), l.repo, mutation.Binding{ID: "tester", Role: "OWNER"}, store.LeaseChoice{QueueID: fixture.QueueID, RequestID: id, Root: l.root, Lease: req, Derive: store.NoScopeDeriver}, recorded)
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		l.t.Fatalf("%s holder %d: %+v %v", verb, i, report, err)
	}
}

// recorded is the attempt as attempt show reports it: the recorded signal,
// the lease expiry and the phase, which are all holderStatus reads.
func (l *livenessTimeline) recorded(i int) *snapshot.Attempt {
	l.t.Helper()
	x := handoffCLI(l.t, l.root, "attempt", "show", l.claims[i].AttemptID)
	if x.code != 0 {
		l.t.Fatalf("attempt show: %s", x.stdout)
	}
	item := x.res.Items[0]
	last := wire.Timestamp(field(item, "lastHeartbeatAt").Str)
	return &snapshot.Attempt{Phase: field(item, "phase").Str, Lease: &snapshot.Lease{ExpiresAt: wire.Timestamp(field(field(item, "lease"), "expiresAt").Str)}, LastHeartbeatAt: &last}
}

func at(t *testing.T, s wire.Timestamp) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, string(s))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// TestCALV0277_RenewOnlyHolderNeverReadsStale: four holders share one
// timeline of ticks 540 seconds apart under the default 600-second TTL.
// Holder 0 only renews (10-minute lease, so the lease never outlasts the
// TTL); holder 1 renews and heartbeats in the same tick; holder 2 only
// renews with a 30-minute lease; holder 3 renews every other tick with a
// 30-minute lease. Classifying every second of the timeline: holder 0 never
// reads STALE_HOLDER, before or after its last renew; holder 1 records the
// same signal and reads the same status as holder 0 at every second, so
// pairing adds a write and no freshness; holder 2 never reads STALE_HOLDER
// while it keeps renewing within the TTL; holder 3, whose renews are
// further apart than the TTL, reads STALE_HOLDER inside its live lease.
// The live reads at the end agree, and both help notes say so.
func TestCALV0277_RenewOnlyHolderNeverReadsStale(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	const step = 540 * time.Second
	ticks := []time.Time{now.Add(-60*time.Second - 3*step), now.Add(-60*time.Second - 2*step), now.Add(-60*time.Second - step), now.Add(-60 * time.Second)}
	// leaseCLIStore claims holder i at base+(10+i)m with a five-minute
	// lease, so the first tick (base+14m) is inside every claim's lease.
	root, claims := leaseCLIStore(t, 4, ticks[0].Add(-14*time.Minute))
	l := newLivenessTimeline(t, root, claims)
	states := make([][]*snapshot.Attempt, len(ticks))
	for k, tick := range ticks {
		l.write(transaction.LeaseRenew, 0, tick, "10")
		l.write(transaction.LeaseRenew, 1, tick, "10")
		l.write(transaction.LeaseHeartbeat, 1, tick, "")
		l.write(transaction.LeaseRenew, 2, tick, "30")
		if k%2 == 0 {
			l.write(transaction.LeaseRenew, 3, tick, "30")
		}
		states[k] = []*snapshot.Attempt{l.recorded(0), l.recorded(1), l.recorded(2), l.recorded(3)}
		if *states[k][0].LastHeartbeatAt != *states[k][1].LastHeartbeatAt || at(t, *states[k][0].LastHeartbeatAt) != tick {
			t.Fatalf("tick %d: renew-only signal %s, paired signal %s, want %s", k, *states[k][0].LastHeartbeatAt, *states[k][1].LastHeartbeatAt, tick)
		}
	}
	var seconds, staleSparse int
	for k, from := range ticks {
		// After the last tick nobody renews again: follow every lease out.
		until := from.Add(31 * time.Minute)
		if k+1 < len(ticks) {
			until = ticks[k+1]
		}
		for now := from; now.Before(until); now = now.Add(time.Second) {
			seconds++
			s := make([]string, 4)
			for i := range s {
				s[i] = cli.HolderStatus(states[k][i], now, 600)
			}
			if s[0] == "STALE_HOLDER" {
				t.Fatalf("renew-only holder read STALE_HOLDER at %s (tick %d)", now, k)
			}
			if s[1] != s[0] {
				t.Fatalf("paired holder %s, renew-only %s at %s", s[1], s[0], now)
			}
			if s[2] == "STALE_HOLDER" && now.Before(from.Add(step)) {
				t.Fatalf("long-lease holder renewing within the TTL read STALE_HOLDER at %s", now)
			}
			if s[3] == "STALE_HOLDER" && now.Before(at(t, states[k][3].Lease.ExpiresAt)) {
				staleSparse++
			}
		}
	}
	// Holder 3 is stale for 480 s between ticks 0 and 2, and after tick 2
	// from 600 s until its 30-minute lease ends.
	if want := 480 + (1800 - 600); staleSparse != want {
		t.Fatalf("sparse holder stale for %d s inside its lease, want %d (of %d s)", staleSparse, want, seconds)
	}
	for i, want := range []string{"FRESH_HOLDER", "FRESH_HOLDER", "FRESH_HOLDER", "STALE_HOLDER"} {
		x := handoffCLI(t, root, "attempt", "show", claims[i].AttemptID)
		if got := field(x.res.Items[0], "holderStatus").Str; got != want || field(x.res.Items[0], "heartbeatTTLSeconds").Str != "600" {
			t.Fatalf("holder %d reads %s, want %s: %s", i, got, want, x.stdout)
		}
	}
	for verb, wants := range map[string][]string{
		"attempt heartbeat": {"CAL-V0-231", "need not also send a heartbeat", "same lastHeartbeatAt", "policy show", "CAL-V0-277"},
		"renew":             {"Do not pair a heartbeat with each renew", "--lease-minutes*60", "CAL-V0-277"},
	} {
		help := handoffCLI(t, root, append(strings.Fields(verb), "--help", "--verbose")...)
		note := field(help.res.Items[0], "note").Str
		for _, want := range wants {
			if !strings.Contains(note, want) {
				t.Fatalf("%s note lacks %q: %s", verb, want, note)
			}
		}
	}
}

// TestCALV0278_PolicyShowEffectiveTTL: under a policy without
// holderLiveness, policy show reports heartbeatTTLSeconds "600" beside the
// unchanged stored policy bytes and digest. A holder that heartbeats every
// 1500 seconds reads STALE_HOLDER for 900 of every 1500 seconds under the
// default TTL and never under a policy TTL of 1800; after the policy
// update, policy show, attempt show and queue status all report 1800 and
// the holder reads FRESH_HOLDER with its lease unchanged.
func TestCALV0278_PolicyShowEffectiveTTL(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	const cadence = 1500 * time.Second
	ticks := []time.Time{now.Add(-900*time.Second - 2*cadence), now.Add(-900*time.Second - cadence), now.Add(-900 * time.Second)}
	root, claims := leaseCLIStore(t, 1, ticks[0].Add(-14*time.Minute))
	l := newLivenessTimeline(t, root, claims)
	loaded, e := intent.Load(l.repo.PrimaryWorktree)
	if e != nil {
		t.Fatal(e)
	}
	show := handoffCLI(t, root, "policy", "show")
	item := show.res.Items[0]
	if _, has := field(item, "policy").Obj.Get("holderLiveness"); has || show.code != 0 {
		t.Fatalf("default policy show: %s", show.stdout)
	}
	if field(item, "heartbeatTTLSeconds").Str != "600" || field(item, "policySha256").Str != string(loaded.Policy.PolicySha256()) || !bytes.Equal(bytes.TrimSuffix(wire.EncodeFile(field(item, "policy")), []byte("\n")), bytes.TrimSuffix(loaded.Policy.Raw, []byte("\n"))) {
		t.Fatalf("default policy show changed the stored policy or omitted the TTL: %s", show.stdout)
	}
	states := make([]*snapshot.Attempt, len(ticks))
	for k, tick := range ticks {
		if k == 0 {
			l.write(transaction.LeaseRenew, 0, tick, "120")
		} else {
			l.write(transaction.LeaseHeartbeat, 0, tick, "")
		}
		states[k] = l.recorded(0)
	}
	stale := map[int64]int{}
	for k := 0; k+1 < len(ticks); k++ {
		for now := ticks[k]; now.Before(ticks[k+1]); now = now.Add(time.Second) {
			for _, ttl := range []int64{600, 1800} {
				if cli.HolderStatus(states[k], now, ttl) == "STALE_HOLDER" {
					stale[ttl]++
				}
			}
		}
	}
	if stale[600] != 2*900 || stale[1800] != 0 {
		t.Fatalf("stale seconds at a 1500 s cadence: %v, want 1800 under 600 and 0 under 1800", stale)
	}
	read := func(want, ttl string) string {
		t.Helper()
		a := handoffCLI(t, root, "attempt", "show", claims[0].AttemptID)
		q := handoffCLI(t, root, "queue", "status")
		p := handoffCLI(t, root, "policy", "show")
		live := field(q.res.Items[0], "liveAttempts").Arr
		if field(a.res.Items[0], "holderStatus").Str != want || field(a.res.Items[0], "heartbeatTTLSeconds").Str != ttl || len(live) != 1 || field(live[0], "holderStatus").Str != want || field(live[0], "heartbeatTTLSeconds").Str != ttl || field(p.res.Items[0], "heartbeatTTLSeconds").Str != ttl {
			t.Fatalf("want %s under %s:\n%s\n%s\n%s", want, ttl, a.stdout, q.stdout, p.stdout)
		}
		return field(field(a.res.Items[0], "lease"), "expiresAt").Str
	}
	expires := read("STALE_HOLDER", "600")
	holderTTLPolicy(t, root, "1800")
	if got := read("FRESH_HOLDER", "1800"); got != expires {
		t.Fatalf("the TTL change moved the lease: %s -> %s", expires, got)
	}
	p := handoffCLI(t, root, "policy", "show")
	if v := field(field(field(p.res.Items[0], "policy"), "holderLiveness"), "heartbeatTTLSeconds").Str; v != "1800" {
		t.Fatalf("policy show lacks the stored key: %s", p.stdout)
	}
	if note := field(handoffCLI(t, root, "policy", "show", "--help", "--verbose").res.Items[0], "note").Str; !strings.Contains(note, "heartbeatTTLSeconds") || !strings.Contains(note, "600") || !strings.Contains(note, "CAL-V0-278") {
		t.Fatalf("policy show help lacks the effective TTL: %s", note)
	}
}
