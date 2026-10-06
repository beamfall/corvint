package store_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// implementOn claims ticket id at the implement stage on member, releases
// it and confirms the member safe, so it is FREE again for a later stage.
func (s *leaseStore) implementOn(t *testing.T, id, member string, exclude ...string) *store.Report {
	t.Helper()
	c := claimOf(id, id)
	c.Pool, c.Stage, c.ExcludeMembers = "db", "implement", exclude
	r := s.lease(t, id+"-implement", c, 0, nil)
	if r.PoolAllocation == nil || r.PoolAllocation.MemberID != member {
		t.Fatalf("implement %+v", r)
	}
	s.freeAgain(t, id+"-implement", r)
	return r
}

func (s *leaseStore) freeAgain(t *testing.T, prefix string, r *store.Report) {
	t.Helper()
	// A health-backed claim writes receipts on the real clock.
	s.t0 = now(t)
	s.lease(t, prefix+"-release", transaction.LeaseRequest{Verb: transaction.LeaseRelease, AttemptID: r.AttemptID, Generation: r.Generation}, 0, nil)
	s.lease(t, prefix+"-safe", transaction.LeaseRequest{Verb: transaction.LeasePoolSafe, Member: r.PoolAllocation.MemberID, Allocation: string(r.PoolAllocation.AllocationID), Evidence: "local-reset", Reason: "operator confirmed reset"}, 0, nil)
}

func reviewOf(id, mode string, exclude ...string) transaction.LeaseRequest {
	c := claimOf(id, id)
	c.Pool, c.Stage, c.ExcludeAuthors, c.ExcludeMembers = "db", "review", mode, exclude
	return c
}

// CAL-V0-098: a review claim excludes the implement author's member, unions
// it with explicit exclusions, reports exhaustion with the author named,
// binds the mode into replay, and refuses unverifiable history.
func TestCALV0098_ClaimExcludesImplementAuthor(t *testing.T) {
	s := newLeaseStore(t)
	exclusionPolicy(t, s, wire.Null())
	id := s.ticket(t, "authored")
	s.implementOn(t, id, "b", "a", "review")

	review := s.lease(t, "review-1", reviewOf(id, transaction.ExcludeAuthorsLatest, "review"), 0, nil)
	if review.PoolAllocation == nil || review.PoolAllocation.MemberID != "a" {
		t.Fatalf("review %+v", review)
	}
	replay := s.lease(t, "review-1", reviewOf(id, transaction.ExcludeAuthorsLatest, "review"), 0, nil)
	if replay.Kind != "Replay" || replay.Generation != review.Generation {
		t.Fatalf("replay %+v", replay)
	}
	if c := s.lease(t, "review-1", reviewOf(id, transaction.ExcludeAuthorsAll, "review"), 0, nil); !c.Outcome.HasCode(wire.CodeRequestIDConflict) {
		t.Fatalf("changed mode replayed %+v", c)
	}
	s.freeAgain(t, "review-1", review)

	// The review generation is skipped; the implement author b stays excluded.
	exhausted := s.lease(t, "review-2", reviewOf(id, transaction.ExcludeAuthorsLatest, "a", "review"), 0, nil)
	if !exhausted.Outcome.HasCode(wire.CodeResourceCollision) || !strings.Contains(exhausted.Detail, "excluded implement authors: member b of pool db") {
		t.Fatalf("exhausted %+v", exhausted)
	}
	integrate := reviewOf(id, transaction.ExcludeAuthorsAll, "review")
	integrate.Stage = "integrate"
	if r := s.lease(t, "integrate-1", integrate, 0, nil); r.PoolAllocation == nil || r.PoolAllocation.MemberID != "a" {
		t.Fatalf("integrate %+v", r)
	}

	implement := reviewOf(id, transaction.ExcludeAuthorsLatest)
	implement.Stage = "implement"
	if r, err := store.Lease(context.Background(), s.repo, operator(), store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "implement-authors", Root: s.root, Lease: implement}, s.at(t, 0)); err == nil && r.Outcome.Outcome == mutation.OutcomeCompleted {
		t.Fatalf("implement stage accepted %+v", r)
	}

	plain := s.ticket(t, "unpooled")
	one := s.claim(t, "plain-1", plain, 0, "plain")
	s.lease(t, "plain-release", releaseOf(one), 0, nil)
	if r := s.lease(t, "plain-review", reviewOf(plain, transaction.ExcludeAuthorsLatest), 0, nil); !r.Outcome.HasCode(wire.CodeIndependenceUnverified) || !strings.Contains(r.Detail, "recorded no stage") {
		t.Fatalf("unpooled implement %+v", r)
	}
	// CAL-V0-107: explicit members cover the generation that recorded no
	// member, and the admitted claim reports it.
	covered := s.lease(t, "plain-review-covered", reviewOf(plain, transaction.ExcludeAuthorsLatest, "review"), 0, nil)
	if covered.PoolAllocation == nil || covered.PoolAllocation.MemberID == "review" || covered.AuthorExclusion == nil || len(covered.AuthorExclusion.Covered) != 1 || len(covered.AuthorExclusion.Notes()) != 2 {
		t.Fatalf("covered unpooled implement %+v", covered)
	}
	// CAL-V0-107: a ticket with no implement generation has nothing to
	// exclude, so a review claim is admitted, not refused.
	fresh := s.ticket(t, "never-implemented")
	if r := s.lease(t, "fresh-review", reviewOf(fresh, transaction.ExcludeAuthorsLatest), 0, nil); r.Outcome.Outcome != mutation.OutcomeCompleted || r.PoolAllocation == nil || r.AuthorExclusion == nil || len(r.AuthorExclusion.Authors) != 0 {
		t.Fatalf("unimplemented ticket %+v", r)
	}
	auditOK(t, s.repo)
}

// CAL-V0-098: CLAIM_NEXT skips a ticket whose authors are unverifiable and
// claims the next one on a member its author did not hold.
func TestCALV0098_ClaimNextExcludesAuthors(t *testing.T) {
	s := newLeaseStore(t)
	exclusionPolicy(t, s, wire.Null())
	// A pooled claim without a stage recorded its member but no stage: no
	// explicit member covers it (CAL-V0-107), so CLAIM_NEXT skips it.
	unverified := s.ticket(t, "unverified")
	stageless := claimOf(unverified, unverified)
	stageless.Pool, stageless.ExcludeMembers = "db", []string{"a", "review"}
	if r := s.lease(t, "stageless", stageless, 0, nil); r.PoolAllocation == nil || r.PoolAllocation.MemberID != "b" {
		t.Fatalf("stage-less %+v", r)
	} else {
		s.freeAgain(t, "stageless", r)
	}
	want := s.ticket(t, "implemented")
	s.implementOn(t, want, "b", "a", "review")
	derive := func(_ context.Context, _, _, title, _ string) ([]string, string, bool) {
		return []string{title}, string(wire.Sum([]byte(title))), true
	}
	next := transaction.LeaseRequest{Verb: transaction.LeaseClaimNext, Holder: "reviewer", LeaseMinutes: "60", Pool: "db", Stage: "review", ExcludeAuthors: transaction.ExcludeAuthorsLatest, ExcludeMembers: []string{"review"}}
	r := s.lease(t, "next-review", next, 0, derive)
	if r.Ticket != want || r.PoolAllocation == nil || r.PoolAllocation.MemberID != "a" {
		t.Fatalf("next %+v", r)
	}
	s.lease(t, "next-review-release", releaseOf(r), 0, nil)
	blocked := s.lease(t, "next-none", next, 0, derive)
	if blocked.PoolAllocation != nil || blocked.Outcome.Outcome == mutation.OutcomeCompleted || !blocked.Outcome.HasCode(wire.CodeIndependenceUnverified) || !strings.Contains(blocked.Detail, "recorded no stage") {
		t.Fatalf("next without eligible ticket %+v", blocked)
	}
	auditOK(t, s.repo)
}

// CAL-V0-098: health preparation never probes an excluded author.
func TestCALV0098_HealthSkipsAuthor(t *testing.T) {
	s := newLeaseStore(t)
	markers := t.TempDir()
	touch := func(name string) wire.Value {
		return obj("health", obj("argv", wire.Strings([]string{"/usr/bin/touch", filepath.Join(markers, name)}), "cwd", str("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", str("3")))
	}
	exclusionPolicy(t, s, obj("a", touch("a"), "b", touch("b"), "review", touch("review")))
	id := s.ticket(t, "health-author")
	s.implementOn(t, id, "b", "a", "review")
	for _, m := range []string{"a", "b", "review"} {
		_ = os.Remove(filepath.Join(markers, m))
	}
	r := s.lease(t, "health-review", reviewOf(id, transaction.ExcludeAuthorsLatest, "review"), 0, nil)
	if r.PoolAllocation == nil || r.PoolAllocation.MemberID != "a" {
		t.Fatalf("health review %+v", r)
	}
	if _, err := os.Stat(filepath.Join(markers, "b")); !os.IsNotExist(err) {
		t.Fatalf("author health ran: %v", err)
	}
	if _, err := os.Stat(filepath.Join(markers, "a")); err != nil {
		t.Fatalf("eligible health did not run: %v", err)
	}
	s.t0 = now(t)
	unverified := s.ticket(t, "health-unverified")
	s.lease(t, "health-plain-release", releaseOf(s.claim(t, "health-plain", unverified, 0, "health-plain")), 0, nil)
	before, _ := os.ReadDir(markers)
	if r := s.lease(t, "health-unverified", reviewOf(unverified, transaction.ExcludeAuthorsLatest), 0, nil); !r.Outcome.HasCode(wire.CodeIndependenceUnverified) {
		t.Fatalf("unverified health %+v", r)
	}
	if after, _ := os.ReadDir(markers); len(after) != len(before) {
		t.Fatal("unverified claim probed a member")
	}
	// CAL-V0-107: with an explicit member the unrecorded generation is
	// covered, and health preparation (which carries no explicit members)
	// covers it too instead of refusing the prepared claim.
	s.t0 = now(t)
	covered := s.lease(t, "health-covered", reviewOf(unverified, transaction.ExcludeAuthorsLatest, "review"), 0, nil)
	if covered.PoolAllocation == nil || covered.PoolAllocation.MemberID != "b" || covered.AuthorExclusion == nil || len(covered.AuthorExclusion.Covered) != 1 {
		t.Fatalf("covered health %+v", covered)
	}
	if _, err := os.Stat(filepath.Join(markers, "b")); err != nil {
		t.Fatalf("covered claim health did not run: %v", err)
	}
}

// CAL-V0-098: when the ticket's implement author changes between the claim's
// refusal and health preparation, preparation rederives the exclusion and
// the newly excluded member's health command never runs.
func TestCALV0098_HealthPrepareRederivesAuthors(t *testing.T) {
	s := newLeaseStore(t)
	markers := t.TempDir()
	touch := func(name string) wire.Value {
		return obj("health", obj("argv", wire.Strings([]string{"/usr/bin/touch", filepath.Join(markers, name)}), "cwd", str("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", str("3")))
	}
	exclusionPolicy(t, s, obj("a", touch("a"), "b", touch("b"), "review", touch("review")))
	id := s.ticket(t, "drifting-author")
	s.implementOn(t, id, "a", "b", "review")
	fired := false
	restore := store.SetHealthPrepareHookForTest(func(member string) {
		if fired || member != "b" {
			return
		}
		fired = true
		// Another caller implements the ticket on b and frees it again.
		s.t0 = now(t)
		c := claimOf(id, id)
		c.Pool, c.Stage, c.ExcludeMembers = "db", "implement", []string{"a", "review"}
		r := s.lease(t, "drift-implement", c, 0, nil)
		if r.PoolAllocation == nil || r.PoolAllocation.MemberID != "b" {
			t.Fatalf("drift implement %+v", r)
		}
		s.freeAgain(t, "drift-implement", r)
		_ = os.Remove(filepath.Join(markers, "b"))
	})
	defer restore()
	for _, m := range []string{"a", "b", "review"} {
		_ = os.Remove(filepath.Join(markers, m))
	}
	r := s.lease(t, "drift-review", reviewOf(id, transaction.ExcludeAuthorsLatest, "review"), 0, nil)
	if !fired {
		t.Fatal("preparation of b was never attempted")
	}
	if _, err := os.Stat(filepath.Join(markers, "b")); !os.IsNotExist(err) {
		t.Fatalf("new author's health ran: %v", err)
	}
	if r.PoolAllocation == nil || r.PoolAllocation.MemberID != "a" {
		t.Fatalf("drift review %+v", r)
	}
	if _, err := os.Stat(filepath.Join(markers, "a")); err != nil {
		t.Fatalf("eligible health did not run: %v", err)
	}
	auditOK(t, s.repo)
}

// CAL-V0-107: health preparation covers an unrecorded generation only when
// the claim's caller supplied explicit members. A generation that races in
// before preparation of a claim without --exclude-member refuses before any
// health command runs, so no member is probed or quarantined.
func TestCALV0107_HealthPrepareNeverImpliesCover(t *testing.T) {
	s := newLeaseStore(t)
	markers := t.TempDir()
	touch := func(name string) wire.Value {
		return obj("health", obj("argv", wire.Strings([]string{"/usr/bin/touch", filepath.Join(markers, name)}), "cwd", str("REPOSITORY"), "env", wire.Array(), "timeoutSeconds", str("3")))
	}
	exclusionPolicy(t, s, obj("a", touch("a"), "b", touch("b"), "review", touch("review")))
	id := s.ticket(t, "raced-unrecorded")
	s.implementOn(t, id, "b", "a", "review")
	fired := false
	restore := store.SetHealthPrepareHookForTest(func(string) {
		if fired {
			return
		}
		fired = true
		// Another caller claims and releases the ticket without a pool, so
		// its newest generation records no stage or member.
		s.t0 = now(t)
		s.lease(t, "race-plain-release", releaseOf(s.claim(t, "race-plain", id, 0, "race-plain")), 0, nil)
	})
	defer restore()
	for _, m := range []string{"a", "b", "review"} {
		_ = os.Remove(filepath.Join(markers, m))
	}
	r := s.lease(t, "race-review", reviewOf(id, transaction.ExcludeAuthorsLatest), 0, nil)
	if !fired {
		t.Fatal("preparation was never attempted")
	}
	if r.PoolAllocation != nil || !r.Outcome.HasCode(wire.CodeIndependenceUnverified) {
		t.Fatalf("raced review %+v", r)
	}
	if ran, _ := os.ReadDir(markers); len(ran) != 0 {
		t.Fatalf("health ran without an explicit cover: %v", ran)
	}
	for _, en := range untouchedPoolState(t, s).Entries {
		t.Fatalf("pool mutated: %+v", en)
	}
	auditOK(t, s.repo)
}
