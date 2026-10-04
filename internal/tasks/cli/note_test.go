package cli_test

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestONV0008_TicketNoteSetShowClearThroughTheCLI drives `ticket note
// set|show|clear` and the `ticket show` operatorNote view against a native
// store: NONE before the first note, CURRENT with the original text, an
// identical retry replays, a stale supersedes refuses, CLEAR keeps the
// reference as CLEARED, and a missing event is an explicit evidence failure,
// never NONE.
func TestONV0008_TicketNoteSetShowClearThroughTheCLI(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	atm(t, r.Root, nil, "init")
	created := atm(t, r.Root, nil, "ticket", "create",
		"--request-id", "req-1", "--issued-at", "2026-09-07T12:00:00Z", "--payload", createPayloadJSON)
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("ticket create: %+v", created.res)
	}
	id := field(created.res.Items[0], "ticketId").Str
	issued := "2026-10-04T12:00:00Z"

	note := func(args ...string) wire.Value {
		t.Helper()
		x := atm(t, r.Root, nil, append([]string{"ticket", "note", "show"}, args...)...)
		if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 || !x.res.Untrusted {
			t.Fatalf("ticket note show: %+v", x.res)
		}
		return x.res.Items[0]
	}
	if got := note(id); field(got, "state").Str != "NONE" || field(got, "revision").Str != "0" {
		t.Fatalf("before any note: %s", wire.Encode(got))
	}

	setArgs := []string{"ticket", "note", "set", id, "--request-id", "note-1", "--issued-at", issued,
		"--text", "flaky gate: rerun once before triage", "--supersedes", "0"}
	set := atm(t, r.Root, nil, setArgs...)
	if set.res.Outcome != wire.OutcomeOK || field(set.res.Items[0], "resultingRevision").Str != "2" ||
		field(set.res.Items[0], "resultingAcceptanceRevision").Str != "1" {
		t.Fatalf("note set: %+v", set.res)
	}
	current := note(id)
	if field(current, "state").Str != "CURRENT" || field(current, "text").Str != "flaky gate: rerun once before triage" ||
		field(current, "revision").Str != "1" || field(field(current, "actor"), "role").Str != "OWNER" {
		t.Fatalf("current note: %s", wire.Encode(current))
	}
	shown := atm(t, r.Root, nil, "ticket", "show", id)
	if shown.res.Outcome != wire.OutcomeOK || field(field(shown.res.Items[0], "operatorNote"), "state").Str != "CURRENT" {
		t.Fatalf("ticket show operatorNote: %+v", shown.res)
	}

	again := atm(t, r.Root, nil, setArgs...)
	if again.res.Outcome != wire.OutcomeOK || !field(again.res.Items[0], "replayed").Bool || field(again.res.Items[0], "receipt").Str != "" {
		t.Fatalf("identical retry did not replay: %+v", again.res)
	}
	stale := atm(t, r.Root, nil, "ticket", "note", "set", id, "--request-id", "note-2", "--issued-at", issued,
		"--text", "a second writer", "--supersedes", "0")
	if stale.res.Outcome == wire.OutcomeOK || len(stale.res.Items) != 1 || field(stale.res.Items[0], "outcome").Str != mutation.OutcomeRevisionConflict {
		t.Fatalf("a stale supersedes was accepted: %+v", stale.res)
	}

	cleared := atm(t, r.Root, nil, "ticket", "note", "clear", id, "--request-id", "note-3", "--issued-at", issued, "--supersedes", "1")
	if cleared.res.Outcome != wire.OutcomeOK || field(cleared.res.Items[0], "resultingRevision").Str != "3" {
		t.Fatalf("note clear: %+v", cleared.res)
	}
	gone := note(id)
	if field(gone, "state").Str != "CLEARED" || field(gone, "revision").Str != "2" || field(gone, "text").Kind != wire.KindNull {
		t.Fatalf("cleared note: %s", wire.Encode(gone))
	}

	if err := os.Remove(filepath.Join(r.StateDir, "evidence", field(gone, "head").Str)); err != nil {
		t.Fatal(err)
	}
	missing := atm(t, r.Root, nil, "ticket", "note", "show", id)
	if missing.res.Outcome == wire.OutcomeOK || len(missing.res.Codes) == 0 || missing.res.Codes[0] != wire.CodeMissingEvidence {
		t.Fatalf("missing event was not an evidence failure: %+v", missing.res)
	}
	// `ticket show` also audits the journal, which already refuses the
	// absent retained event as a fork rather than rendering any note state.
	degraded := atm(t, r.Root, nil, "ticket", "show", id)
	if degraded.res.Outcome == wire.OutcomeOK || len(degraded.res.Codes) == 0 || degraded.res.Codes[0] != wire.CodeJournalForked {
		t.Fatalf("ticket show hid a missing event: %+v", degraded.res)
	}

	if x := atm(t, r.Root, nil, "ticket", "note", "set", id, "--request-id", "note-4"); x.res.Outcome == wire.OutcomeOK {
		t.Fatal("note set without text succeeded")
	}
	if x := atm(t, r.Root, nil, "ticket", "note", "clear", id, "--request-id", "note-5", "--text", "x"); x.res.Outcome == wire.OutcomeOK {
		t.Fatal("note clear accepted --text")
	}
}

// TestONV0004_NotePolicyNeedsAnExplicitOperatorRow checks ON-V0-004 through
// the CLI writer: OPERATOR may write a note only through an explicit
// policy.roles.OPERATOR row that names the note verb, OWNER's default row
// includes it, an explicit OWNER row narrows it, and no other role's row may
// name it at all.
func TestONV0004_NotePolicyNeedsAnExplicitOperatorRow(t *testing.T) {
	operatorDefault := []string{"ARCHIVE", "COMPLETE_MANUAL", "CREATE", "GRANT_APPROVAL", "HOLD", "PRIORITIZE", "REFINE",
		"RELEASE_CANDIDATE", "RELEASE_CREATE", "RELEASE_EXTERNAL_ATTEST", "RELEASE_HOLD", "RELEASE_UPDATE", "REOPEN",
		"RESTORE", "REVOKE_APPROVAL", "SET_DEPENDENCIES", "SET_EFFECTS", "SET_GATES"}
	withNote := append(append([]string{}, operatorDefault...), "NOTE_SET")
	slices.Sort(withNote)
	cases := []struct {
		name  string
		roles map[string][]string
		role  string
		ok    bool
	}{
		{"operator without a row", nil, "OPERATOR", false},
		{"operator default-only row", map[string][]string{"OPERATOR": operatorDefault}, "OPERATOR", false},
		{"operator explicit grant", map[string][]string{"OPERATOR": withNote}, "OPERATOR", true},
		{"owner default", nil, "OWNER", true},
		{"owner narrowed", map[string][]string{"OWNER": {"CREATE"}}, "OWNER", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := fixture.TempRepo(t)
			policy := fixture.PolicyValue()
			roles := wire.NewObject()
			for _, role := range slices.Sorted(maps.Keys(c.roles)) {
				roles.Set(role, wire.Strings(c.roles[role]))
			}
			policy.Obj.Set("roles", wire.ObjectValue(roles))
			fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
			fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(policy))
			if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
				t.Fatalf("init: %+v", x.res)
			}
			created := atm(t, r.Root, nil, "ticket", "create", "--request-id", "req-1", "--issued-at", "2026-09-07T12:00:00Z", "--payload", createPayloadJSON)
			if created.res.Outcome != wire.OutcomeOK {
				t.Fatalf("ticket create: %+v", created.res)
			}
			id := field(created.res.Items[0], "ticketId").Str
			x := atm(t, r.Root, nil, "ticket", "note", "set", id, "--request-id", "note-1", "--issued-at", "2026-10-04T12:00:00Z",
				"--text", "operator context", "--role", c.role)
			if (x.res.Outcome == wire.OutcomeOK) != c.ok || (!c.ok && (len(x.res.Items) != 1 || field(x.res.Items[0], "outcome").Str != mutation.OutcomeUnauthorized)) {
				t.Fatalf("note set as %s: %+v", c.role, x.res)
			}
		})
	}
	_, err := intent.DecodePolicy(wire.EncodeFile(func() wire.Value {
		p := fixture.PolicyValue()
		p.Obj.Set("roles", wire.ObjectValue(wire.NewObject().Set("WORKER", wire.Strings([]string{"NOTE_SET", "REFINE"}))))
		return p
	}()))
	if err == nil {
		t.Fatal("a WORKER row naming NOTE_SET decoded")
	}
}
