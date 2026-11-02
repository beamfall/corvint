package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCLV0739_DuplicateRecordedAsSupersedesLink pins V1-3272: the
// agent-memory duplicate guard accepts a payload whose typed supersedes link
// names its candidate, the create records that link, and `ticket search
// --supersedes` finds the ticket by it, by full or local id (TCL-V0-739).
func TestTCLV0739_DuplicateRecordedAsSupersedesLink(t *testing.T) {
	r, ids := similarRepo(t)
	dup := similarPayload("heavy offload drops untracked fixture files again", "untracked fixture files are missing on the remote", "agent-memory")
	x := atm(t, r.Root, dup, "ticket", "create", "--request-id", "dup-1", "--issued-at", "2026-10-30T11:00:00Z", "--payload-stdin")
	if x.res.Outcome != wire.OutcomeError || x.res.Codes[0] != wire.CodeDuplicateID || !strings.Contains(strings.Join(x.res.Warnings, " "), "supersedes") {
		t.Fatalf("the refusal does not offer the supersedes link: %s", x.stdout)
	}
	linked := bytes.Replace(dup, []byte(`"supersedes":null`), []byte(`"supersedes":"`+ids["stash"]+`"`), 1)
	if bytes.Equal(linked, dup) {
		t.Fatal("fixture payload has no supersedes member")
	}
	x = atm(t, r.Root, linked, "ticket", "create", "--request-id", "dup-1", "--issued-at", "2026-10-30T11:00:00Z", "--payload-stdin")
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("a create naming its candidate as supersedes was refused: %s", x.stdout)
	}
	newID := field(x.res.Items[0], "ticketId").Str
	local := ids["stash"][strings.LastIndex(ids["stash"], ":")+1:]
	for _, arg := range []string{ids["stash"], local} {
		found := atm(t, r.Root, nil, "ticket", "search", "--supersedes", arg)
		if found.res.Outcome != wire.OutcomeOK || len(found.res.Items) != 1 || field(found.res.Items[0], "ticketId").Str != newID {
			t.Fatalf("search --supersedes %s: %s", arg, found.stdout)
		}
	}
	if none := atm(t, r.Root, nil, "ticket", "search", "--supersedes", ids["lease"]); none.res.Outcome != wire.OutcomeOK || len(none.res.Items) != 0 {
		t.Fatalf("search --supersedes matched an unlinked ticket: %s", none.stdout)
	}
	if both := atm(t, r.Root, nil, "ticket", "search", "--supersedes", local, "--status", "COMPLETED"); both.res.Outcome != wire.OutcomeOK || len(both.res.Items) != 0 {
		t.Fatalf("--supersedes is not a conjunction with --status: %s", both.stdout)
	}
	// A candidate the link does not name still refuses.
	other := bytes.Replace(dup, []byte(`"supersedes":null`), []byte(`"supersedes":"`+ids["lease"]+`"`), 1)
	if x := atm(t, r.Root, other, "ticket", "create", "--request-id", "dup-3", "--issued-at", "2026-10-30T11:10:00Z", "--payload-stdin"); x.res.Outcome != wire.OutcomeError || x.res.Codes[0] != wire.CodeDuplicateID {
		t.Fatalf("a link to an unmatched ticket bypassed the guard: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, "ticket", "search", "--supersedes", ""); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("an empty --supersedes was accepted: %s", x.stdout)
	}
}
