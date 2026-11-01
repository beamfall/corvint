// SPDX-License-Identifier: Apache-2.0

package ticket_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func progressEntry(phase string) ticket.Progress {
	return ticket.Progress{ProgressMark: ticket.ProgressMark{Phase: phase}, Actor: "russell", RecordedAt: "2026-10-31T12:00:00Z"}
}

// TestTLFV0880_RecordCodec: progress is omitted when empty, round-trips, and
// a malformed member refuses at /progress.
func TestTLFV0880_RecordCodec(t *testing.T) {
	rec := fixture.Ticket("AT-01")
	if bytes.Contains(rec.Encode(), []byte(`"progress"`)) {
		t.Fatal("a record without progress gained the key")
	}
	head := strings.Repeat("c", 40)
	rec.Progress = []ticket.Progress{progressEntry("IN_PROGRESS"), {ProgressMark: ticket.ProgressMark{Phase: "MERGED_AWAITING_CLOSEOUT", Head: &head}, Actor: "russell", RecordedAt: "2026-10-31T12:01:00Z"}}
	back, err := ticket.Decode(rec.Encode())
	if err != nil || !bytes.Equal(back.Encode(), rec.Encode()) {
		t.Fatalf("round trip: %v", err)
	}
	for name, edit := range map[string]func(*wire.Value){
		"empty": func(v *wire.Value) { v.Obj.Set("progress", wire.Array()) },
		"phase members": func(v *wire.Value) {
			v.Obj.Set("progress", ticket.ProgressValue([]ticket.Progress{progressEntry("BLOCKED")}))
		},
		"unknown phase": func(v *wire.Value) {
			v.Obj.Set("progress", ticket.ProgressValue([]ticket.Progress{progressEntry("DONE")}))
		},
		"extra entry key": func(v *wire.Value) {
			e := ticket.ProgressEntryValue(progressEntry("IN_PROGRESS"))
			e.Obj.Set("x", wire.Null())
			v.Obj.Set("progress", wire.Array(e))
		},
	} {
		t.Run(name, func(t *testing.T) {
			v := fixture.Ticket("AT-01").Value()
			edit(&v)
			if _, err := ticket.Decode(wire.EncodeFile(v)); err == nil || !strings.Contains(err.Error(), "progress") {
				t.Fatalf("decoded with %v", err)
			}
		})
	}
}

// TestTLFV0881_EffectiveProgress: the latest entry while live and not
// CLEARED; none once the ticket completes or archives.
func TestTLFV0881_EffectiveProgress(t *testing.T) {
	rec := fixture.Ticket("AT-01")
	if rec.EffectiveProgress() != nil || rec.EffectivePhase() != "NONE" {
		t.Fatal("a fresh ticket has effective progress")
	}
	rec.Progress = []ticket.Progress{progressEntry("IN_PROGRESS")}
	for _, s := range []string{ticket.StatusDraft, ticket.StatusOpen, ticket.StatusHeld} {
		rec.Status = s
		if rec.EffectivePhase() != "IN_PROGRESS" {
			t.Fatalf("%s: %s", s, rec.EffectivePhase())
		}
	}
	for _, s := range []string{ticket.StatusCompleted, ticket.StatusArchived} {
		rec.Status = s
		if rec.EffectiveProgress() != nil {
			t.Fatalf("%s keeps effective progress", s)
		}
	}
	rec.Status = ticket.StatusOpen
	rec.Progress = append(rec.Progress, progressEntry("CLEARED"))
	if rec.EffectiveProgress() != nil {
		t.Fatal("CLEARED keeps effective progress")
	}
}
