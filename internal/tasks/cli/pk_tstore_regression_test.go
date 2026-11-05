// SPDX-License-Identifier: Apache-2.0
package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TCL-V0-7207: split argv identifies the unexpected word and quoting repair.
func TestTCLV07207_SearchSplitTextNamesArgument(t *testing.T) {
	r := fixture.TempRepo(t)
	x := atm(t, r.Root, nil, "ticket", "search", "--text", "impact", "Makefile")
	if !hasCode(x.res, wire.CodeMalformed) || !strings.Contains(string(x.stdout), "Makefile") || !strings.Contains(string(x.stdout), "quote") {
		t.Fatalf("split text: %s", x.stdout)
	}
}

// TCL-V0-7209: unrelated root directories never become evidence for read views.
func TestTCLV07209_ReadsIgnoreExtraDirectory(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.WriteWritableState(t, r)
	a := fixture.Ticket("A")
	fixture.WriteIntent(t, r, a)
	fixture.CommitPosts(t, r, "MUTATION", "", map[string][]byte{"intent/tickets/A.json": a.Encode()})
	extra := filepath.Join(r.StateDir, "lane-private")
	if err := os.Mkdir(extra, 0700); err != nil {
		t.Fatal(err)
	}
	fixture.Write(t, filepath.Join(extra, "not-evidence"), []byte("private"))
	for _, args := range [][]string{{"queue", "status"}, {"ticket", "show", "A"}, {"receipt", "audit"}} {
		x := atm(t, r.Root, nil, args...)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s", args, x.stdout)
		}
	}
}

// TCL-V0-7210: absent archive-reason provenance is explicit.
func TestTCLV07210_ArchivedShowStatesReasonNotObserved(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.WriteWritableState(t, r)
	a := fixture.Ticket("A")
	from := a.Status
	a.Status = ticket.StatusArchived
	a.ArchivedFrom = &from
	fixture.WriteIntent(t, r, a)
	fixture.CommitPosts(t, r, "MUTATION", "", map[string][]byte{"intent/tickets/A.json": a.Encode()})
	x := atm(t, r.Root, nil, "ticket", "show", "A")
	if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "archiveReason").Str != "NOT_OBSERVED" {
		t.Fatalf("archive reason: %s", x.stdout)
	}
}

// TCL-V0-7211: labels have a documented mapping without adding wire enum values.
func TestTCLV07211_CreateTemplateMapsFixesAndTestsToChore(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.WriteIntent(t, r, fixture.Ticket("A"))
	x := atm(t, r.Root, nil, "ticket", "create", "--template")
	if x.res.Outcome != wire.OutcomeOK || !strings.Contains(string(x.stdout), "fixes and tests map to kind CHORE") {
		t.Fatalf("kind mapping: %s", x.stdout)
	}
}

// TCL-V0-7213: single-target current and implicit-revision hold settle once and replay.
func TestTCLV07213_RetriesReplayOverPendingReceipt(t *testing.T) {
	for _, mode := range []string{"current", "hold"} {
		t.Run(mode, func(t *testing.T) {
			root, _ := ergStore(t)
			setActor(t, "tester")
			a, _ := progressTicket(t, root, "alpha", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["a/"]}`)
			b, _ := progressTicket(t, root, "bravo", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["b/"]}`)
			args := []string{"ticket", "progress", "--target", a, "--expected-revision", "current", "--request-id", "retry-a", "--issued-at", "2026-11-01T11:00:00Z", "--payload", progressPayload("IN_PROGRESS", "", "", "", "")}
			if mode == "hold" {
				args = []string{"ticket", "hold", "--target", a, "--request-id", "retry-a", "--issued-at", "2026-11-01T11:00:00Z", "--payload", `{"holdId":"wait","reason":"await owner"}`}
			}
			if x := atm(t, root, nil, args...); x.res.Outcome != wire.OutcomeOK {
				t.Fatalf("first: %s", x.stdout)
			}
			seq := holdPending(t, root, func() {
				x := atm(t, root, nil, "ticket", "progress", "--target", b, "--expected-revision", "current", "--request-id", "held-b", "--issued-at", "2026-11-01T12:00:00Z", "--payload", progressPayload("IN_PROGRESS", "", "", "", ""))
				if x.res.Outcome != wire.OutcomeOK {
					t.Fatalf("held: %s", x.stdout)
				}
			})
			retry := atm(t, root, nil, args...)
			if retry.res.Outcome != wire.OutcomeOK || !field(retry.res.Items[0], "replayed").Bool {
				t.Fatalf("retry: %s", retry.stdout)
			}
			if got := auditedHeadSeq(t, root); got != seq+1 {
				t.Fatalf("replay wrote new receipt: %d want %d", got, seq+1)
			}
			if x := atm(t, root, nil, args...); x.res.Outcome != wire.OutcomeOK || !field(x.res.Items[0], "replayed").Bool {
				t.Fatalf("second retry: %s", x.stdout)
			}
		})
	}
}

// TCL-V0-7213: a corrupt request entry is named rather than treated as absent.
func TestTCLV07213_CurrentNamesRequestIndexError(t *testing.T) {
	root, _ := ergStore(t)
	setActor(t, "tester")
	a, _ := progressTicket(t, root, "alpha", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["a/"]}`)
	args := []string{"ticket", "progress", "--target", a, "--expected-revision", "current", "--request-id", "broken-request", "--issued-at", "2026-11-01T11:00:00Z", "--payload", progressPayload("IN_PROGRESS", "", "", "", "")}
	if x := atm(t, root, nil, args...); x.res.Outcome != wire.OutcomeOK {
		t.Fatal(string(x.stdout))
	}
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := snapshot.RequestPath("broken-request")
	fixture.Write(t, filepath.Join(repo.StateDir, p), []byte("broken\n"))
	x := atm(t, root, nil, args...)
	if x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), "broken-request") {
		t.Fatalf("index error: %s", x.stdout)
	}
}
