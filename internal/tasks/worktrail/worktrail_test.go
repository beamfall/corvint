package worktrail_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/worktrail"
)

// TestTAWV0002_TrailerRoundTrip: every valid transition formats and parses
// back to itself, and malformed blocks refuse with a named tag.
func TestTAWV0002_TrailerRoundTrip(t *testing.T) {
	valid := []worktrail.Transition{
		{Tickets: []string{"V1-3975"}, State: worktrail.StateStarted},
		{Tickets: []string{"V1-3975", "V1-0001"}, State: worktrail.StateDelivered, Lane: "taw-w15"},
		{Tickets: []string{"V1-3975"}, State: worktrail.StateBlocked, Reason: "needs owner decision"},
	}
	for _, tr := range valid {
		if err := tr.Validate(); err != nil {
			t.Fatalf("%v: %v", tr, err)
		}
		got, ok, err := worktrail.Parse(append([]string{"Signed-off-by: x"}, tr.Lines()...))
		if err != nil || !ok {
			t.Fatalf("parse %v: ok=%v err=%v", tr, ok, err)
		}
		if !slices.Equal(got.Tickets, tr.Tickets) || got.State != tr.State || got.Reason != tr.Reason || got.Lane != tr.Lane {
			t.Fatalf("round trip %v -> %v", tr, got)
		}
		args := tr.GitArgs()
		if len(args) != 2*len(tr.Lines()) || args[0] != "--trailer" {
			t.Fatalf("git args %v", args)
		}
	}
	if _, ok, err := worktrail.Parse([]string{"Reviewed-by: someone"}); ok || err != nil {
		t.Fatalf("no transition: ok=%v err=%v", ok, err)
	}
	refusals := map[string][]string{
		worktrail.TagState:          {"Corvint-Ticket: V1-1", "Corvint-Ticket-State: merged"},
		worktrail.TagReasonRequired: {"Corvint-Ticket: V1-1", "Corvint-Ticket-State: blocked"},
		worktrail.TagTicketID:       {"Corvint-Ticket: V1 1", "Corvint-Ticket-State: started"},
		worktrail.TagMalformed:      {"Corvint-Ticket: V1-1", "Corvint-Ticket-State: started", "Corvint-Ticket-State: delivered"},
	}
	for tag, lines := range refusals {
		_, ok, err := worktrail.Parse(lines)
		if !ok || err == nil || !strings.Contains(err.Error(), tag) {
			t.Fatalf("%s: ok=%v err=%v", tag, ok, err)
		}
	}
	if _, _, err := worktrail.Parse([]string{"Corvint-Ticket-State: started"}); err == nil {
		t.Fatal("state without ticket accepted")
	}
}
