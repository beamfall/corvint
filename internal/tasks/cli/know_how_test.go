package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// knowHowCLIRepo is a claimable store over a Git checkout whose base commit
// holds src/a.go, src/b.go and docs/x.md.
func knowHowCLIRepo(t *testing.T) *fixture.Repo {
	t.Helper()
	r := exclusionCLIRepo(t)
	for p, body := range map[string]string{"src/a.go": "package a\n", "src/b.go": "package b\n", "docs/x.md": "# x\n"} {
		fixture.Write(t, filepath.Join(r.Root, p), []byte(body))
	}
	git(t, r.Root, "add", "src", "docs")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-m", "files")
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	return r
}

func knowHowNotes(t *testing.T, v wire.Value) []wire.Value {
	t.Helper()
	if field(v, "trust").Str != "UNTRUSTED_AGENT_AUTHORED_DATA" {
		t.Fatalf("projection is not labelled untrusted: %s", wire.Encode(v))
	}
	return field(v, "notes").Arr
}

// TestKHNV0006_KnowHowThroughTheCLI drives the slice end to end: `ticket
// know-how add` pins each anchor's blob at HEAD through a receipt-backed
// mutation, an identical retry replays, `list` is a labelled read that
// writes nothing, a later commit makes a note STALE and a deleted file
// UNKNOWN, supersede and retract keep history in `ticket show`, a secret is
// refused, and claim delivers the intersecting notes under the byte cap.
func TestKHNV0006_KnowHowThroughTheCLI(t *testing.T) {
	r := knowHowCLIRepo(t)
	home := planTicket(t, r.Root, "home", "P2", `["src/a.go"]`)
	addArgs := func(req, rev, text string, extra ...string) []string {
		return append([]string{"ticket", "know-how", "add", home, "--request-id", req, "--expected-revision", rev,
			"--issued-at", "2026-10-07T12:00:00Z", "--text", text}, extra...)
	}
	first := addArgs("kh-1", "1", "run make gen before go test in src", "--anchor", "src/b.go", "--anchor", "src/a.go", "--route", "flow-build")
	x := atm(t, r.Root, nil, first...)
	if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "outcome").Str != mutation.OutcomeCompleted || field(x.res.Items[0], "resultingAcceptanceRevision").Str != "1" {
		t.Fatalf("add: %s", x.stdout)
	}
	if again := atm(t, r.Root, nil, first...); again.res.Outcome != wire.OutcomeOK || !field(again.res.Items[0], "replayed").Bool {
		t.Fatalf("identical retry did not replay: %s", again.stdout)
	}
	if x := atm(t, r.Root, nil, addArgs("kh-2", "2", "docs note", "--anchor", "docs/x.md")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("second add: %s", x.stdout)
	}
	for name, args := range map[string][]string{
		"missing file": addArgs("kh-x1", "3", "t", "--anchor", "src/gone.go"),
		"absolute":     addArgs("kh-x2", "3", "t", "--anchor", "/etc/passwd"),
		"dotdot":       addArgs("kh-x3", "3", "t", "--anchor", "../x.go"),
		"no anchor":    addArgs("kh-x4", "3", "t"),
		"secret":       addArgs("kh-x5", "3", "token AKIAABCDEFGHIJKLMNOP", "--anchor", "src/a.go"),
		// Screened before the pin, whose missing-file error names the path.
		"secret path":  addArgs("kh-x6", "3", "t", "--anchor", "src/AKIAABCDEFGHIJKLMNOP.go"),
		"secret route": addArgs("kh-x7", "3", "t", "--anchor", "src/a.go", "--route", "AKIAABCDEFGHIJKLMNOP"),
	} {
		if x := atm(t, r.Root, nil, args...); x.res.Outcome == wire.OutcomeOK {
			t.Fatalf("%s accepted: %s", name, x.stdout)
		} else if strings.HasPrefix(name, "secret") && (!strings.Contains(string(x.stdout), mutation.KnowHowSecretDetail) || strings.Contains(string(x.stdout), "AKIAABCDEFGHIJKLMNOP") ||
			!strings.Contains(string(x.stdout), `"`+wire.CodeSecretDetected+`"`) || strings.Contains(string(x.stdout), `"`+wire.CodeMalformed+`"`)) {
			t.Fatalf("secret refusal: %s", x.stdout)
		}
	}

	list := func(args ...string) []wire.Value {
		t.Helper()
		state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
		x := atm(t, r.Root, nil, append([]string{"ticket", "know-how", "list"}, args...)...)
		if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 || !x.res.Untrusted {
			t.Fatalf("list: %s", x.stdout)
		}
		if !fixture.SameTree(state, fixture.TreeSnapshot(t, r.StateDir)) || !fixture.SameTree(intents, fixture.TreeSnapshot(t, r.IntentDir)) {
			t.Fatal("know-how list wrote state")
		}
		return knowHowNotes(t, x.res.Items[0])
	}
	all := list()
	if len(all) != 2 || field(all[0], "freshness").Str != "CURRENT" || field(all[1], "freshness").Str != "CURRENT" {
		t.Fatalf("fresh list: %s", wire.Encode(wire.Array(all...)))
	}
	for _, n := range all {
		for _, a := range field(n, "anchors").Arr {
			if field(a, "blob").Str == "" {
				t.Fatalf("anchor not pinned: %s", wire.Encode(n))
			}
		}
	}
	if got := list("--path", "src/"); len(got) != 1 || field(got[0], "note").Str != "1" {
		t.Fatalf("prefix filter: %s", wire.Encode(wire.Array(got...)))
	}
	if got := list("--path", "src"); len(got) != 0 {
		t.Fatal("a path without a trailing slash matched as a prefix")
	}
	if got := list("--ticket", home, "--path", "docs/x.md"); len(got) != 1 || field(got[0], "note").Str != "2" {
		t.Fatalf("ticket and exact filter: %s", wire.Encode(wire.Array(got...)))
	}

	// A committed change makes note 1 STALE; deleting docs/x.md makes note 2
	// UNKNOWN; CURRENT and UNKNOWN sort before STALE.
	fixture.Write(t, filepath.Join(r.Root, "src/b.go"), []byte("package b // changed\n"))
	if err := os.Remove(filepath.Join(r.Root, "docs/x.md")); err != nil {
		t.Fatal(err)
	}
	git(t, r.Root, "add", "-A", "src", "docs")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-m", "change")
	later := list()
	if len(later) != 2 || field(later[0], "note").Str != "2" || field(later[0], "freshness").Str != "UNKNOWN" ||
		field(later[1], "freshness").Str != "STALE" {
		t.Fatalf("later list: %s", wire.Encode(wire.Array(later...)))
	}

	// Claim of a ticket touching src/ delivers note 1 only, compact and
	// labelled; a ticket with no touchPaths gets an empty delivery.
	work := planTicket(t, r.Root, "work", "P2", `["src/"]`)
	c := atm(t, r.Root, nil, "claim", work, "--holder", "agent", "--request-id", "claim-work")
	if c.res.Outcome != wire.OutcomeOK {
		t.Fatalf("claim: %s", c.stdout)
	}
	kh := field(c.res.Items[0], "knowHow")
	got := knowHowNotes(t, kh)
	if field(kh, "state").Str != "DELIVERED" || field(kh, "omitted").Str != "0" || len(got) != 1 || field(got[0], "freshness").Str != "STALE" {
		t.Fatalf("claim delivery: %s", wire.Encode(kh))
	}
	if strings.Contains(string(wire.Encode(kh)), `"blob"`) || len(wire.Encode(field(kh, "notes"))) > 2048 {
		t.Fatalf("claim delivery is not the compact projection: %s", wire.Encode(kh))
	}

	// Supersede and retract keep every entry in ticket show.
	if x := atm(t, r.Root, nil, addArgs("kh-3", "3", "src/b.go changed; run make gen-all", "--anchor", "src/b.go", "--supersedes", "1", "--reason", "command renamed")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("supersede: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, "ticket", "know-how", "retract", home, "--request-id", "kh-4", "--expected-revision", "4",
		"--issued-at", "2026-10-07T12:00:00Z", "--note", "2", "--reason", "docs/x.md was removed"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("retract: %s", x.stdout)
	}
	show := atm(t, r.Root, nil, "ticket", "show", home)
	ledger := field(show.res.Items[0], "knowHow").Arr
	if len(ledger) != 4 || field(ledger[2], "supersedes").Str != "1" || field(ledger[3], "operation").Str != "RETRACT" {
		t.Fatalf("ticket show knowHow: %s", show.stdout)
	}
	if got := list(); len(got) != 1 || field(got[0], "note").Str != "3" || field(got[0], "freshness").Str != "CURRENT" {
		t.Fatalf("active after supersede and retract: %s", wire.Encode(wire.Array(got...)))
	}

	audit := atm(t, r.Root, nil, "receipt", "audit")
	if audit.res.Outcome != wire.OutcomeOK || field(audit.res.Items[0], "structuralConsistency").Str != "CONSISTENT" {
		t.Fatalf("receipt audit: %+v", audit.res)
	}
	help := atm(t, r.Root, nil, "ticket", "know-how", "add", "--help", "--verbose")
	for _, want := range []string{"--anchor", "--commit", "untrusted", "secret-screened", "STALE"} {
		if !strings.Contains(string(help.stdout), want) {
			t.Errorf("know-how add help lacks %q", want)
		}
	}
}

// TestKHNV0006_ClaimDeliveryIsCapped: many intersecting notes deliver the
// ordered prefix that fits 2 KiB, with the omitted count and the read hint.
func TestKHNV0006_ClaimDeliveryIsCapped(t *testing.T) {
	r := knowHowCLIRepo(t)
	home := planTicket(t, r.Root, "home", "P2", `[]`)
	text := strings.Repeat("long note ", 90)
	for i := 1; i <= 6; i++ {
		n := string(rune('0' + i))
		x := atm(t, r.Root, nil, "ticket", "know-how", "add", home, "--request-id", "kh-"+n, "--expected-revision", n,
			"--issued-at", "2026-10-07T12:00:0"+n+"Z", "--text", text+n, "--anchor", "src/a.go")
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("add %s: %s", n, x.stdout)
		}
	}
	work := planTicket(t, r.Root, "work", "P1", `["src/a.go"]`)
	c := atm(t, r.Root, nil, "claim", "--next", "--holder", "agent", "--request-id", "claim-next")
	if c.res.Outcome != wire.OutcomeOK || field(c.res.Items[0], "ticketId").Str != work {
		t.Fatalf("claim --next: %s", c.stdout)
	}
	kh := field(c.res.Items[0], "knowHow")
	notes := knowHowNotes(t, kh)
	if len(notes) == 0 || len(notes) >= 6 || field(kh, "matched").Str != "6" || field(kh, "omitted").Str != string(rune('0'+6-len(notes))) ||
		field(kh, "hint").Str == "" || len(wire.Encode(kh)) > store.KnowHowDeliveryMaxBytes {
		t.Fatalf("capped delivery: %s", wire.Encode(kh))
	}
	if field(notes[0], "note").Str != "6" {
		t.Fatalf("newest note is not first: %s", wire.Encode(notes[0]))
	}
}
