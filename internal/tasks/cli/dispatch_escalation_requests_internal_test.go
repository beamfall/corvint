//go:build darwin || linux

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ESC-V0-009: dispatch status shows each current OPEN request's kind,
// original OPEN time, nonnegative age and whether it holds, sorted by ticket
// then request and apart from parked keys, infrastructure retry state and
// the 499 tier ladder. A local clock behind the OPEN time shows age 0 and
// clockUncertain, never a negative age, and a ticket whose material could
// not be validated shows UNKNOWN material and no request.
func TestIssue502_DispatchStatusShowsEscalationRequests(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	l := &dispatch.Ledger{Program: "prog", Backoff: map[string]*dispatch.BackoffState{"ticket:a:q:t9": {Parked: true}},
		InfraRetry: map[string]*dispatch.InfraEpisode{"ticket:a:q:t1": {AcceptanceRevision: "2", State: dispatch.InfraWaiting, Limit: 3, CooldownUntil: now.Add(time.Minute)}}}
	if _, ok := statusField(t, dispatchStatusValue(&dispatch.Config{}, t.TempDir(), l, nil, now), "escalationRequests"); ok {
		t.Fatal("a ledger with no observation shows escalation requests")
	}
	l.Seen = &dispatch.Seen{
		Escalations: map[string][]string{"ticket:a:q:t1": {"q-a"}},
		Requests: map[string][]dispatch.OpenRequest{
			"ticket:a:q:t2": {{RequestID: "q-f", Kind: "scope", RecordedAt: "2026-10-05T12:00:30Z"}},
			"ticket:a:q:t1": {{RequestID: "q-a", Kind: "decision", RecordedAt: "2026-10-05T11:00:00Z"}, {RequestID: "q-i", Kind: "infrastructure", RecordedAt: "2026-10-05T11:59:59Z"}},
		},
		RequestsUnknown: []string{"ticket:a:q:t3"},
	}
	v := dispatchStatusValue(&dispatch.Config{}, t.TempDir(), l, nil, now)
	open, ok := statusField(t, v, "escalationRequests")
	want := `[{"ageSeconds":"3600","clockUncertain":false,"holds":true,"kind":"decision","recordedAt":"2026-10-05T11:00:00Z","requestId":"q-a","ticket":"ticket:a:q:t1"},` +
		`{"ageSeconds":"1","clockUncertain":false,"holds":false,"kind":"infrastructure","recordedAt":"2026-10-05T11:59:59Z","requestId":"q-i","ticket":"ticket:a:q:t1"},` +
		`{"ageSeconds":"0","clockUncertain":true,"holds":false,"kind":"scope","recordedAt":"2026-10-05T12:00:30Z","requestId":"q-f","ticket":"ticket:a:q:t2"},` +
		`{"material":"UNKNOWN","ticket":"ticket:a:q:t3"}]`
	if !ok || string(wire.Encode(open)) != want {
		t.Fatalf("escalationRequests = %s", wire.Encode(open))
	}
	parked, _ := statusField(t, v, "parked")
	if len(parked.Arr) != 1 || parked.Arr[0].Str != "ticket:a:q:t9" {
		t.Fatalf("parked = %+v", parked)
	}
	if _, ok := statusField(t, v, "infrastructureRetry"); !ok {
		t.Fatal("retry state is no longer shown beside the requests")
	}
	if _, ok := statusField(t, v, "escalation"); ok {
		t.Fatal("a configuration without a ladder shows 499 tiers")
	}
}

// ESC-V0-009: the dispatcher records the native observation's open requests
// and unknown material in its own ledger, status shows them from that
// ledger alone, and the status read leaves the ledger bytes unchanged.
func TestIssue502_DispatchStatusRequestsFromLedgerWithoutWrite(t *testing.T) {
	ticketOf := func(local string) dispatch.Ticket {
		return dispatch.Ticket{ID: "ticket:a:q:" + local, Local: local, Status: "OPEN", Revision: "1", Order: 1, Plan: "BLOCKED", PlanReason: wire.CodeEscalationPending, NextStage: dispatch.StateNone}
	}
	held, unknown := ticketOf("t1"), ticketOf("t2")
	held.EscalationPending = []string{"q-a"}
	held.OpenRequests = []dispatch.OpenRequest{{RequestID: "q-a", Kind: "blocked", RecordedAt: "2026-10-05T11:00:00Z"}, {RequestID: "q-i", Kind: "infrastructure", RecordedAt: "2026-10-05T11:30:00Z"}}
	unknown.EscalationUnknown = true
	c := issue502DispatchConfig(t)
	d, err := dispatch.Open("prog", c, &issue502Queue{obs: dispatch.Observation{Tickets: []dispatch.Ticket{unknown, held}}}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	dir := dispatch.ProgramDir(c, "prog")
	before, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	l, err := dispatch.LoadLedger(dir, "prog")
	if err != nil {
		t.Fatal(err)
	}
	v, ok := statusField(t, dispatchStatusValue(c, dir, l, nil, time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)), "escalationRequests")
	want := `[{"ageSeconds":"3600","clockUncertain":false,"holds":true,"kind":"blocked","recordedAt":"2026-10-05T11:00:00Z","requestId":"q-a","ticket":"ticket:a:q:t1"},` +
		`{"ageSeconds":"1800","clockUncertain":false,"holds":false,"kind":"infrastructure","recordedAt":"2026-10-05T11:30:00Z","requestId":"q-i","ticket":"ticket:a:q:t1"},` +
		`{"material":"UNKNOWN","ticket":"ticket:a:q:t2"}]`
	if !ok || string(wire.Encode(v)) != want {
		t.Fatalf("escalationRequests = %s", wire.Encode(v))
	}
	after, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("the status read changed the dispatcher ledger")
	}
}

// ESC-V0-009: a ledger's recorded requests load beside the strict members
// only in the shape diff writes; anything else refuses the ledger instead of
// showing an invented kind or age.
func TestIssue502_DispatchLedgerRequestsValidated(t *testing.T) {
	digest := strings.Repeat("a", 64)
	good := func() *dispatch.Seen {
		return &dispatch.Seen{Tickets: map[string]string{}, Claims: map[string]string{}, Lanes: map[string]string{},
			Requests:        map[string][]dispatch.OpenRequest{"ticket:a:q:t": {{RequestID: "q-a", Kind: "decision", RecordedAt: "2026-10-05T11:00:00Z"}, {RequestID: "q-b", Kind: "infrastructure", RecordedAt: "2026-10-05T11:00:00Z"}}},
			RequestsUnknown: []string{"ticket:a:q:u", "ticket:a:q:v"}}
	}
	write := func(t *testing.T, seen *dispatch.Seen, progress bool, patch func(string) string) string {
		t.Helper()
		dir := t.TempDir()
		l := &dispatch.Ledger{Profile: dispatch.StateProfile, Program: "prog", Workers: []*dispatch.Worker{}, Backoff: map[string]*dispatch.BackoffState{}, Seen: seen}
		if progress {
			l.Progress = map[string]*dispatch.ProgressHistory{"ticket:a:q:t": {Current: digest, Seen: []string{digest}}}
		}
		raw, err := json.Marshal(l)
		if err != nil {
			t.Fatal(err)
		}
		if patch != nil {
			raw = []byte(patch(string(raw)))
		}
		if err := os.WriteFile(filepath.Join(dir, "state.json"), raw, 0o600); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	for _, progress := range []bool{false, true} {
		if _, err := dispatch.LoadLedger(write(t, good(), progress, nil), "prog"); err != nil {
			t.Fatalf("progress=%v: the written shape refused: %v", progress, err)
		}
	}
	for name, bad := range map[string]func(*dispatch.Seen){
		"unsorted":       func(s *dispatch.Seen) { r := s.Requests["ticket:a:q:t"]; r[0], r[1] = r[1], r[0] },
		"duplicate":      func(s *dispatch.Seen) { r := s.Requests["ticket:a:q:t"]; r[1].RequestID = r[0].RequestID },
		"kind":           func(s *dispatch.Seen) { s.Requests["ticket:a:q:t"][0].Kind = "urgent" },
		"time":           func(s *dispatch.Seen) { s.Requests["ticket:a:q:t"][0].RecordedAt = "2026-10-05 11:00:00" },
		"empty":          func(s *dispatch.Seen) { s.Requests["ticket:a:q:t"] = []dispatch.OpenRequest{} },
		"ticket key":     func(s *dispatch.Seen) { s.Requests["t"] = s.Requests["ticket:a:q:t"] },
		"unknown order":  func(s *dispatch.Seen) { s.RequestsUnknown = []string{"ticket:a:q:v", "ticket:a:q:u"} },
		"unknown listed": func(s *dispatch.Seen) { s.RequestsUnknown = []string{"ticket:a:q:t"} },
	} {
		seen := good()
		bad(seen)
		if _, err := dispatch.LoadLedger(write(t, seen, false, nil), "prog"); err == nil {
			t.Errorf("%s: loaded", name)
		}
	}
	extra := func(s string) string {
		return strings.Replace(s, `"kind":"decision"`, `"kind":"decision","age":"5"`, 1)
	}
	if _, err := dispatch.LoadLedger(write(t, good(), true, extra), "prog"); err == nil {
		t.Error("strict reader admitted an unknown request member")
	}
	// The request members alone engage the strict reader: with no progress,
	// pool sweep, retry or loop member, a duplicate or case-aliased member
	// still refuses the ledger instead of showing the last value.
	for name, patch := range map[string]func(string) string{
		"duplicate kind": func(s string) string {
			return strings.Replace(s, `"kind":"decision"`, `"kind":"decision","kind":"scope"`, 1)
		},
		"kind alias":     func(s string) string { return strings.Replace(s, `"kind":"decision"`, `"Kind":"decision"`, 1) },
		"requests alias": func(s string) string { return strings.Replace(s, `"requests":`, `"Requests":`, 1) },
		"duplicate unknown": func(s string) string {
			return strings.Replace(s, `"requestsUnknown":`, `"requestsUnknown":[],"requestsUnknown":`, 1)
		},
		"null request": func(s string) string { return strings.Replace(s, `{"requestId":"q-b"`, `null,{"requestId":"q-b"`, 1) },
		"null requests": func(s string) string {
			return strings.Replace(s, `{"ticket:a:q:t":[{"requestId":"q-a","kind":"decision","recordedAt":"2026-10-05T11:00:00Z"},{"requestId":"q-b","kind":"infrastructure","recordedAt":"2026-10-05T11:00:00Z"}]}`, `null`, 1)
		},
		"trailing": func(s string) string { return s + `{}` },
	} {
		for _, progress := range []bool{false, true} {
			if _, err := dispatch.LoadLedger(write(t, good(), progress, patch), "prog"); err == nil {
				t.Errorf("%s progress=%v: loaded", name, progress)
			}
		}
	}
}
