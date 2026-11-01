// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// tlfUnknownTicket creates an UNKNOWN-coverage ticket with body and one
// acceptance criterion.
func tlfUnknownTicket(t *testing.T, root, title, body, criterion string) string {
	t.Helper()
	payload := strings.NewReplacer(`"title":"Console ticket"`, `"title":"`+title+`"`, `"coverage":"QUALIFIED"`, `"coverage":"UNKNOWN"`,
		`"acceptanceCriteria":["it exists"]`, `"acceptanceCriteria":["`+criterion+`"]`, `"body":null`, `"body":"`+body+`"`).Replace(createPayloadJSON)
	x := atm(t, root, nil, "ticket", "create", "--request-id", "create-"+title, "--issued-at", "2026-10-31T12:00:00Z", "--payload", payload)
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("create %s: %s", title, x.stdout)
	}
	return field(x.res.Items[0], "ticketId").Str
}

func tlfProposals(t *testing.T, root string) wire.Value {
	t.Helper()
	x := atm(t, root, nil, "ticket", "effects-proposals")
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 || !x.res.Untrusted {
		t.Fatalf("effects-proposals: %s", x.stdout)
	}
	return x.res.Items[0]
}

// TestTLFV0888_EffectsProposals: an UNKNOWN-coverage OPEN ticket gets each
// existing path its prose names, with source and span; an absent path is
// not proposed, a ticket naming none is NO_PROPOSAL, and a QUALIFIED ticket
// is not screened.
func TestTLFV0888_EffectsProposals(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, _ := ergStore(t)
	if err := os.MkdirAll(filepath.Join(root, "src", "parser"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "parser", "lex.go"), []byte("package parser\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	named := tlfUnknownTicket(t, root, "named", "The lexer in src/parser/lex.go drops tabs.", "A test under src/parser/ and docs/missing.md covers it.")
	bare := tlfUnknownTicket(t, root, "bare", "Nothing names a path here", "it works")
	tcdTicket(t, root, "qualified", "Touches src/parser/lex.go too.")
	item := tlfProposals(t, root)
	if field(item, "profile").Str != "taskman-effects-proposals/0" || field(item, "label").Str != "proposal: not authority" ||
		field(item, "mutationAuthority").Bool || field(item, "ticketsScreened").Str != "2" || field(item, "ticketsProposed").Str != "1" {
		t.Fatalf("header: %s", wire.Encode(item))
	}
	rows := map[string]wire.Value{}
	for _, r := range field(item, "tickets").Arr {
		rows[field(r, "ticketId").Str] = r
	}
	if field(rows[bare], "verdict").Str != "NO_PROPOSAL" || len(field(rows[bare], "touchPaths").Arr) != 0 {
		t.Fatalf("bare: %s", wire.Encode(rows[bare]))
	}
	got := field(rows[named], "touchPaths").Arr
	if field(rows[named], "verdict").Str != "PROPOSED" || len(got) != 2 ||
		field(got[0], "path").Str != "src/parser/" || field(got[0], "source").Str != "acceptanceCriteria[0]" ||
		field(got[1], "path").Str != "src/parser/lex.go" || field(got[1], "source").Str != "body" ||
		!strings.Contains(field(got[1], "span").Str, "src/parser/lex.go drops tabs") {
		t.Fatalf("named proposals: %s", wire.Encode(rows[named]))
	}
	help := atm(t, root, nil, "ticket", "effects-proposals", "--help", "--verbose")
	for _, want := range []string{"proposal: not authority", "set-effects", "Example:"} {
		if !strings.Contains(string(help.stdout), want) {
			t.Fatalf("help lacks %q: %s", want, help.stdout)
		}
	}
}

// TestTLFV0889_ProposalsAreNotAuthority: the read leaves the intent and state
// trees byte-identical, the ticket keeps its revision and COVERAGE_UNKNOWN
// blocker, and the output carries no request id or mutation payload.
func TestTLFV0889_ProposalsAreNotAuthority(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, _ := ergStore(t)
	id := tlfUnknownTicket(t, root, "named", "See internal/ for the code.", "it works")
	blockers := func() string {
		t.Helper()
		x := atm(t, root, nil, "ticket", "blockers", id)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("blockers: %s", x.stdout)
		}
		return string(x.stdout)
	}
	before := blockers()
	if !strings.Contains(before, "COVERAGE_UNKNOWN") {
		t.Fatalf("fixture ticket is not COVERAGE_UNKNOWN: %s", before)
	}
	intentDir, stateDir := filepath.Join(root, ".taskman"), filepath.Join(root, ".git", "taskman")
	beforeIntent, beforeState := fixture.TreeSnapshot(t, intentDir), fixture.TreeSnapshot(t, stateDir)
	item := tlfProposals(t, root)
	if !fixture.SameTree(beforeIntent, fixture.TreeSnapshot(t, intentDir)) || !fixture.SameTree(beforeState, fixture.TreeSnapshot(t, stateDir)) {
		t.Fatal("effects-proposals changed the intent or state tree")
	}
	out := string(wire.Encode(item))
	for _, key := range []string{`"requestId"`, `"payload"`, `"expectedRevision"`, `"effects"`} {
		if strings.Contains(out, key) {
			t.Fatalf("proposal output carries %s: %s", key, out)
		}
	}
	if after := blockers(); after != before {
		t.Fatalf("blockers moved:\n%s\n%s", before, after)
	}
}
