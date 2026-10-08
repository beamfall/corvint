package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestKHNV0018_SymbolAnchorsAndReconfirmThroughTheCLI drives the V1-0963
// slice end to end: `add --symbol` pins a declaration, an edit of another
// symbol leaves its note CURRENT while the edited one goes STALE, `reconfirm`
// re-pins the STALE note with provenance and keeps both entries in `ticket
// show`, a reconfirm of a CURRENT note is refused KNOWHOW_NOT_STALE, and an
// unresolvable symbol is refused KNOWHOW_UNRESOLVED on add and on reconfirm
// while it reads UNKNOWN.
func TestKHNV0018_SymbolAnchorsAndReconfirmThroughTheCLI(t *testing.T) {
	r := knowHowCLIRepo(t)
	commit := func(body string) {
		t.Helper()
		fixture.Write(t, filepath.Join(r.Root, "src/a.go"), []byte(body))
		git(t, r.Root, "add", "src/a.go")
		git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-m", "edit a.go")
	}
	base := "package a\n\n// F is f.\nfunc F() int { return 1 }\n\nfunc G() int { return 2 }\n"
	commit(base)
	home := planTicket(t, r.Root, "home", "P2", `["src/a.go"]`)
	c := atm(t, r.Root, nil, "claim", home, "--holder", "agent", "--request-id", "claim-home")
	if c.res.Outcome != wire.OutcomeOK {
		t.Fatalf("claim: %s", c.stdout)
	}
	attempt, gen := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	stamp := []string{"--issued-at", "2026-10-07T12:00:00Z"}
	add := func(req, rev, text string, extra ...string) []string {
		return append(append([]string{"ticket", "know-how", "add", home, "--request-id", req, "--expected-revision", rev, "--text", text}, stamp...), extra...)
	}
	reconfirmAs := func(req, rev, note, attempt, gen string) []string {
		return append([]string{"ticket", "know-how", "reconfirm", home, "--request-id", req, "--expected-revision", rev, "--note", note,
			"--attempt", attempt, "--generation", gen}, stamp...)
	}
	reconfirm := func(req, rev, note string) []string { return reconfirmAs(req, rev, note, attempt, gen) }
	refused := func(name, code string, args []string) {
		t.Helper()
		x := atm(t, r.Root, nil, args...)
		if x.res.Outcome == wire.OutcomeOK || !strings.Contains(string(x.stdout), code) {
			t.Fatalf("%s: want %s, got %s", name, code, x.stdout)
		}
	}
	for _, args := range [][]string{
		add("kh-1", "1", "F caches the tree", "--symbol", "src/a.go#F"),
		add("kh-2", "2", "G must stay pure", "--symbol", "src/a.go#G", "--anchor", "src/b.go"),
	} {
		if x := atm(t, r.Root, nil, args...); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("add: %s", x.stdout)
		}
	}
	refused("missing symbol", store.KnowHowUnresolved, add("kh-x1", "3", "t", "--symbol", "src/a.go#Nope"))
	refused("no extractor", store.KnowHowUnresolved, add("kh-x2", "3", "t", "--symbol", "docs/x.md#F"))
	refused("no symbol name", "MALFORMED", add("kh-x3", "3", "t", "--symbol", "src/a.go#"))
	refused("duplicate anchor", "MALFORMED", add("kh-x4", "3", "t", "--symbol", "src/a.go#F", "--symbol", "src/a.go#F"))

	byNote := func() map[string]wire.Value {
		t.Helper()
		x := atm(t, r.Root, nil, "ticket", "know-how", "list")
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("list: %s", x.stdout)
		}
		out := map[string]wire.Value{}
		for _, n := range knowHowNotes(t, x.res.Items[0]) {
			out[field(n, "note").Str] = n
		}
		return out
	}
	notes := byNote()
	if field(notes["1"], "freshness").Str != "CURRENT" || field(field(notes["1"], "anchors").Arr[0], "symbol").Str != "F" {
		t.Fatalf("fresh symbol note: %s", wire.Encode(notes["1"]))
	}

	// Editing G leaves the F note CURRENT and makes the G note STALE.
	commit(strings.Replace(base, "return 2", "return 3", 1))
	notes = byNote()
	if field(notes["1"], "freshness").Str != "CURRENT" || field(notes["2"], "freshness").Str != "STALE" {
		t.Fatalf("after editing G: %s %s", wire.Encode(notes["1"]), wire.Encode(notes["2"]))
	}
	refused("reconfirm CURRENT", "KNOWHOW_NOT_STALE", reconfirm("kh-r0", "3", "1"))
	refused("reconfirm unknown note", "MALFORMED", reconfirm("kh-r9", "3", "9"))
	refused("reconfirm unverified attempt", wire.CodeProvenanceUnverified, reconfirmAs("kh-r8", "3", "2", "att-unknown-khn", gen))
	if x := atm(t, r.Root, nil, reconfirm("kh-r1", "3", "2")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("reconfirm: %s", x.stdout)
	}
	notes = byNote()
	re := field(notes["2"], "reconfirmed")
	if field(notes["2"], "freshness").Str != "CURRENT" || field(re, "seq").Str != "3" || field(re, "attempt").Str != attempt ||
		field(re, "generation").Str != gen || field(field(re, "actor"), "role").Str != "OWNER" {
		t.Fatalf("reconfirmed note: %s", wire.Encode(notes["2"]))
	}
	refused("second reconfirm", "KNOWHOW_NOT_STALE", reconfirm("kh-r2", "4", "2"))

	show := atm(t, r.Root, nil, "ticket", "show", home)
	ledger := field(show.res.Items[0], "knowHow").Arr
	if len(ledger) != 3 || field(ledger[2], "operation").Str != "RECONFIRM" || field(ledger[2], "note").Str != "2" ||
		field(field(ledger[1], "anchors").Arr[0], "symbolSha256").Str == field(field(ledger[2], "anchors").Arr[0], "symbolSha256").Str {
		t.Fatalf("ticket show keeps the prior pin and the re-pin: %s", show.stdout)
	}

	// Renaming F reads UNKNOWN, never CURRENT, and cannot be re-pinned.
	commit(strings.Replace(strings.Replace(base, "return 2", "return 3", 1), "func F()", "func F2()", 1))
	if got := field(byNote()["1"], "freshness").Str; got != "UNKNOWN" {
		t.Fatalf("renamed symbol reads %s", got)
	}
	refused("reconfirm UNKNOWN", store.KnowHowUnresolved, reconfirm("kh-r3", "4", "1"))

	audit := atm(t, r.Root, nil, "receipt", "audit")
	if audit.res.Outcome != wire.OutcomeOK || field(audit.res.Items[0], "structuralConsistency").Str != "CONSISTENT" {
		t.Fatalf("receipt audit: %+v", audit.res)
	}
	help := atm(t, r.Root, nil, "ticket", "know-how", "reconfirm", "--help", "--verbose")
	for _, want := range []string{"--note", "--attempt", "KNOWHOW_NOT_STALE", "KNOWHOW_UNRESOLVED"} {
		if !strings.Contains(string(help.stdout), want) {
			t.Errorf("know-how reconfirm help lacks %q", want)
		}
	}
	if help := atm(t, r.Root, nil, "ticket", "know-how", "add", "--help"); !strings.Contains(string(help.stdout), "--symbol") {
		t.Errorf("know-how add help lacks --symbol: %s", help.stdout)
	}
}
