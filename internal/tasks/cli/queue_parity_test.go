package cli_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCLV0742_QueueParityNamesMissingAndStaleTickets pins V1-3931: a lane
// host reading an old snapshot is compared with a newer authoritative one.
// The ticket created after the old snapshot is missing, the reprioritized
// one is stale, both trees, ages and journal seqs are named, and the result
// is refused STALE_TICKET. The same store against itself is in sync, and a
// served snapshot read names its tree and age.
func TestTCLV0742_QueueParityNamesMissingAndStaleTickets(t *testing.T) {
	r, ids := similarRepo(t)
	old := snapshotFile(t, r.Root)
	x := atm(t, r.Root, similarPayload("Parity newcomer", "created after the lane snapshot"), "ticket", "create", "--request-id", "parity-new", "--issued-at", "2026-11-01T12:00:00Z", "--payload-stdin")
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("create: %s", x.stdout)
	}
	newID := field(x.res.Items[0], "ticketId").Str
	short := func(id string) string { return id[strings.LastIndex(id, ":")+1:] }
	if x := atm(t, r.Root, nil, "ticket", "prioritize", "--request-id", "parity-p0", "--target", ids["lease"], "--expected-revision", "1", "--issued-at", "2026-11-01T12:01:00Z", "--payload", `{"order":"0","priority":"P0"}`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("prioritize: %s", x.stdout)
	}
	auth := snapshotFile(t, r.Root)

	if x := atm(t, r.Root, nil, "queue", "parity", "--against", auth); x.res.Outcome != wire.OutcomeOK || !field(x.res.Items[0], "inSync").Bool {
		t.Fatalf("the owning store against its own snapshot is not in sync: %s", x.stdout)
	}

	t.Setenv(cli.SnapshotEnv, old)
	x = atm(t, t.TempDir(), nil, "queue", "parity", "--against", auth)
	item := x.res.Items
	if x.res.Outcome != wire.OutcomeRefused || len(x.res.Codes) != 1 || x.res.Codes[0] != wire.CodeStaleTicket || len(item) != 1 || field(item[0], "inSync").Bool {
		t.Fatalf("an old snapshot was not refused STALE_TICKET: %s", x.stdout)
	}
	list := func(key string) string {
		var out []string
		for _, v := range field(item[0], key).Arr {
			out = append(out, v.Str)
		}
		return strings.Join(out, ",")
	}
	if list("missing") != short(newID) || list("stale") != short(ids["lease"]) || list("extra") != "" ||
		field(item[0], "missingCount").Str != "1" || field(item[0], "staleCount").Str != "1" {
		t.Fatalf("missing=%q stale=%q extra=%q: %s", list("missing"), list("stale"), list("extra"), x.stdout)
	}
	host, authObj := field(item[0], "host"), field(item[0], "authoritative")
	if field(authObj, "treeSha256").Str == field(host, "treeSha256").Str || field(host, "source").Str != "snapshot "+old ||
		field(authObj, "age").Str == "NOT_OBSERVED" || field(host, "age").Str == "NOT_OBSERVED" || field(authObj, "journalSeq").Str == "NOT_OBSERVED" {
		t.Fatalf("host %s / authoritative %s", wire.Encode(host), wire.Encode(authObj))
	}
	w := strings.Join(x.res.Warnings, " ")
	if !strings.Contains(w, "1 tickets missing, 1 stale, 0 extra") || !strings.Contains(w, field(authObj, "treeSha256").Str) || !strings.Contains(w, ", age ") {
		t.Fatalf("warnings: %s", w)
	}

	if x := atm(t, t.TempDir(), nil, "ticket", "show", ids["lease"]); x.res.Outcome != wire.OutcomeOK || !strings.Contains(strings.Join(x.res.Warnings, " "), ", tree "+field(host, "treeSha256").Str) {
		t.Fatalf("a snapshot read does not name its tree: %s", x.stdout)
	}
	for _, args := range [][]string{{"queue", "parity"}, {"queue", "parity", "--against", ""}} {
		if x := atm(t, t.TempDir(), nil, args...); x.res.Outcome == wire.OutcomeOK || x.res.Codes[0] != wire.CodeMalformed {
			t.Fatalf("%v was not refused MALFORMED: %s", args, x.stdout)
		}
	}
}
