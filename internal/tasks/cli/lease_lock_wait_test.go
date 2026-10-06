package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// lockWaitStore claims one ticket at stage integrate through the CLI and
// returns the root, repository, ticket, attempt and generation.
func lockWaitStore(t *testing.T) (string, *intent.Repository, string, string, string) {
	t.Helper()
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	id := planTicket(t, root, "lock-wait", "P1", `["src/"]`)
	x := atm(t, root, nil, "claim", id, "--holder", "lane-0", "--stage", "integrate", "--request-id", "claim-lock-wait-1")
	if x.code != 0 {
		t.Fatalf("claim: %s", x.stdout)
	}
	return root, repo, id, field(x.res.Items[0], "attemptId").Str, field(x.res.Items[0], "generation").Str
}

// retryCharge claims id again and reports the new attempt's retryCount.
func retryCharge(t *testing.T, root, id, request string) string {
	t.Helper()
	x := atm(t, root, nil, "claim", id, "--holder", "next", "--stage", "review", "--request-id", request)
	if x.code != 0 {
		t.Fatalf("reclaim: %s", x.stdout)
	}
	show := atm(t, root, nil, "attempt", "show", field(x.res.Items[0], "attemptId").Str)
	if show.code != 0 {
		t.Fatalf("attempt show: %s", show.stdout)
	}
	return field(show.res.Items[0], "retryCount").Str
}

func holdPreparation(t *testing.T, repo *intent.Repository) *authority.PreparationLock {
	t.Helper()
	held, err := authority.AcquirePreparation(context.Background(), repo, authority.LockOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return held
}

func lockTimedOut(t *testing.T, x run, elapsed time.Duration, wait time.Duration) {
	t.Helper()
	if x.code == 0 || !hasCode(x.res, wire.CodeLockTimeout) || !bytes.Contains(x.stdout, []byte(`"retryable":true`)) {
		t.Fatalf("want retryable LOCK_TIMEOUT: %s", x.stdout)
	}
	// The caller's bound, not the 30-second default, ended the wait.
	if elapsed < wait || elapsed >= authority.DefaultLockWait {
		t.Fatalf("LOCK_TIMEOUT after %v with --lock-wait %v", elapsed, wait)
	}
}

// TestCALV0109_LockWaitFlag: only release and attempt heartbeat accept
// --lock-wait, its value is whole seconds from 1 to the documented maximum,
// every refusal is MALFORMED without a write, and help lists it.
func TestCALV0109_LockWaitFlag(t *testing.T) {
	root, repo, _, attempt, generation := lockWaitStore(t)
	before := fixture.TreeSnapshot(t, repo.StateDir)
	limit := int64(authority.MaxCallerLockWait / time.Second)
	if limit != 300 {
		t.Fatalf("documented maximum is 300 seconds, constant is %d", limit)
	}
	release := []string{"release", "--attempt", attempt, "--generation", generation, "--request-id", "release-flag"}
	heartbeat := []string{"attempt", "heartbeat", "--attempt", attempt, "--generation", generation, "--request-id", "heartbeat-flag"}
	for _, base := range [][]string{release, heartbeat} {
		for _, value := range []string{"0", fmt.Sprint(limit + 1), "1000", "007", "-1", "+5", "1.5", "5s", "abc", ""} {
			x := atm(t, root, nil, append(append([]string{}, base...), "--lock-wait", value)...)
			if x.code == 0 || !hasCode(x.res, wire.CodeMalformed) {
				t.Fatalf("%v --lock-wait %q: %s", base[:2], value, x.stdout)
			}
		}
		x := atm(t, root, nil, append(append([]string{}, base...), "--lock-wait", "5", "--lock-wait", "5")...)
		if !hasCode(x.res, wire.CodeMalformed) {
			t.Fatalf("repeated --lock-wait: %s", x.stdout)
		}
	}
	for _, args := range [][]string{
		{"renew", "--attempt", attempt, "--generation", generation, "--request-id", "renew-flag", "--lock-wait", "5"},
		{"claim", "--next", "--holder", "h", "--request-id", "claim-flag", "--lock-wait", "5"},
		{"reap", "--request-id", "reap-flag", "--lock-wait", "5"},
	} {
		x := atm(t, root, nil, args...)
		if x.code == 0 || !hasCode(x.res, wire.CodeMalformed) || !strings.Contains(string(x.stdout), "--lock-wait belongs to release and attempt heartbeat") {
			t.Fatalf("%v: %s", args, x.stdout)
		}
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, repo.StateDir)) {
		t.Fatal("a refused --lock-wait wrote state")
	}
	for _, verb := range [][]string{{"release", "--help"}, {"attempt", "heartbeat", "--help"}} {
		x := atm(t, root, nil, verb...)
		usage, note := field(x.res.Items[0], "usage").Str, field(x.res.Items[0], "lockWait").Str
		if x.code != 0 || !strings.Contains(usage, "[--lock-wait SECONDS]") || !strings.Contains(note, fmt.Sprintf("1..%d", limit)) || !strings.Contains(note, "same request ID") {
			t.Fatalf("%v help: %s", verb, x.stdout)
		}
	}
}

// TestCALV0109_LockWaitBoundsContendedRelease: with --lock-wait a contended
// release or heartbeat waits for the caller's bound on preparation admission
// and on the store lock, and a holder that leaves within the bound admits it.
func TestCALV0109_LockWaitBoundsContendedRelease(t *testing.T) {
	root, repo, _, attempt, generation := lockWaitStore(t)
	before := fixture.TreeSnapshot(t, repo.StateDir)
	held := holdPreparation(t, repo)
	start := time.Now()
	x := atm(t, root, nil, "attempt", "heartbeat", "--attempt", attempt, "--generation", generation, "--request-id", "heartbeat-held", "--lock-wait", "1")
	lockTimedOut(t, x, time.Since(start), time.Second)
	if !strings.Contains(string(x.stdout), "phase=") {
		t.Fatalf("preparation timeout without its diagnostic: %s", x.stdout)
	}
	go func() {
		time.Sleep(1500 * time.Millisecond)
		_ = held.Close()
	}()
	start = time.Now()
	x = atm(t, root, nil, "attempt", "heartbeat", "--attempt", attempt, "--generation", generation, "--request-id", "heartbeat-held", "--lock-wait", "20")
	if x.code != 0 || field(x.res.Items[0], "replayed").Bool || time.Since(start) < time.Second {
		t.Fatalf("heartbeat queued behind the holder after %v: %s", time.Since(start), x.stdout)
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	if fixture.SameTree(before, fixture.TreeSnapshot(t, repo.StateDir)) {
		t.Fatal("admitted heartbeat wrote nothing")
	}

	// The store lock: preparation is free, so the release prepares and then
	// waits on taskman.lock for the caller's bound.
	before = fixture.TreeSnapshot(t, repo.StateDir)
	lock, err := authority.AcquireLock(context.Background(), repo, authority.LockOptions{})
	if err != nil {
		t.Fatal(err)
	}
	start = time.Now()
	x = atm(t, root, nil, "release", "--attempt", attempt, "--generation", generation, "--request-id", "release-locked", "--reason", "HANDOFF", "--evidence", "local:handoff", "--lock-wait", "1")
	elapsed := time.Since(start)
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	lockTimedOut(t, x, elapsed, time.Second)
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, repo.StateDir)) {
		t.Fatal("a timed-out release wrote state")
	}
}

// TestCALV0110_SameRequestHandoffReplayAfterLockTimeout: an evidence HANDOFF
// release that timed out wrote nothing; re-submitting the same request ID
// commits it once, a further re-submission (with or without --lock-wait)
// replays that receipt without a write, and the handoff charges no retry.
func TestCALV0110_SameRequestHandoffReplayAfterLockTimeout(t *testing.T) {
	root, repo, id, attempt, generation := lockWaitStore(t)
	handoff := []string{"release", "--attempt", attempt, "--generation", generation, "--request-id", "release-2671", "--reason", "HANDOFF", "--evidence", "local:review-result"}
	before := fixture.TreeSnapshot(t, repo.StateDir)
	held := holdPreparation(t, repo)
	start := time.Now()
	x := atm(t, root, nil, append(append([]string{}, handoff...), "--lock-wait", "1")...)
	lockTimedOut(t, x, time.Since(start), time.Second)
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, repo.StateDir)) {
		t.Fatal("a timed-out HANDOFF release wrote state")
	}
	first := atm(t, root, nil, append(append([]string{}, handoff...), "--lock-wait", "30")...)
	receipt := field(first.res.Items[0], "receipt").Str
	if first.code != 0 || field(first.res.Items[0], "replayed").Bool || receipt == "" {
		t.Fatalf("same-request retry did not commit: %s", first.stdout)
	}
	committed := fixture.TreeSnapshot(t, repo.StateDir)
	for _, extra := range [][]string{nil, {"--lock-wait", "5"}} {
		again := atm(t, root, nil, append(append([]string{}, handoff...), extra...)...)
		// A replay names no new receipt; the store test pins its receipt
		// sequence to the committed one.
		if again.code != 0 || !field(again.res.Items[0], "replayed").Bool || field(again.res.Items[0], "receipt").Str != "" || field(again.res.Items[0], "attemptId").Str != attempt || field(again.res.Items[0], "generation").Str != generation {
			t.Fatalf("replay %v: %s", extra, again.stdout)
		}
		if !fixture.SameTree(committed, fixture.TreeSnapshot(t, repo.StateDir)) {
			t.Fatalf("replay %v wrote state", extra)
		}
	}
	show := atm(t, root, nil, "attempt", "show", attempt)
	if field(show.res.Items[0], "handoffEvidence").Str != "local:review-result" {
		t.Fatalf("handoff not recorded: %s", show.stdout)
	}
	if got := retryCharge(t, root, id, "claim-after-handoff"); got != "0" {
		t.Fatalf("replayed HANDOFF charged a retry: retryCount %s", got)
	}
	if audit := atm(t, root, nil, "receipt", "audit"); audit.code != 0 {
		t.Fatalf("receipt audit: %s", audit.stdout)
	}
}

// TestCALV0111_PlainReleaseAfterTimedOutHandoffIsCharged: a plain release
// that follows a timed-out HANDOFF for the same attempt and generation stays
// a plain release and is charged; nothing converts it into a HANDOFF.
func TestCALV0111_PlainReleaseAfterTimedOutHandoffIsCharged(t *testing.T) {
	root, repo, id, attempt, generation := lockWaitStore(t)
	held := holdPreparation(t, repo)
	start := time.Now()
	x := atm(t, root, nil, "release", "--attempt", attempt, "--generation", generation, "--request-id", "release-handoff", "--reason", "HANDOFF", "--evidence", "local:review-result", "--lock-wait", "1")
	lockTimedOut(t, x, time.Since(start), time.Second)
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	plain := atm(t, root, nil, "release", "--attempt", attempt, "--generation", generation, "--request-id", "release-plain")
	if plain.code != 0 {
		t.Fatalf("plain release: %s", plain.stdout)
	}
	show := atm(t, root, nil, "attempt", "show", attempt)
	if field(show.res.Items[0], "handoffEvidence").Str != "" {
		t.Fatalf("plain release recorded handoff evidence: %s", show.stdout)
	}
	if got := retryCharge(t, root, id, "claim-after-plain"); got != "1" {
		t.Fatalf("plain release after a timed-out HANDOFF: retryCount %s, want 1", got)
	}
}
