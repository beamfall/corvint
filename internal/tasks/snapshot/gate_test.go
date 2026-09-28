package snapshot

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const testAttempt = "attempt:acme:main:57ddbdeca215924fd0ea543f91045dad"

func passedResult() GateResult {
	tree, cwd, clean, exit := strings.Repeat("a", 40), "WORKTREE", true, wire.CountOf(0)
	d := wire.Sum([]byte("x"))
	return GateResult{GateID: "verify", AttemptID: testAttempt, Generation: "1", TicketRevision: wire.CountOf(1), DefinitionSha256: d,
		CandidateTreeOid: tree, ExecutedTreeOid: &tree, ExecutedCwd: &cwd, PorcelainClean: &clean, InputsSha256: d, EnvironmentSha256: d,
		PolicySha256: d, ConfigSha256: d, StartedAt: "2026-09-27T00:00:00Z", EndedAt: "2026-09-27T00:01:00Z", State: "PASSED",
		OutcomeClass: "EXIT", ExitCode: &exit, Evidence: []GateEvidence{{Label: "output", Sha256: wire.Sum(nil), Bytes: "0"}}}
}

// TestCALV0016_GateResultRoundTrips: an encoded result decodes to itself
// and re-encodes to the same bytes.
func TestCALV0016_GateResultRoundTrips(t *testing.T) {
	g := passedResult()
	raw, err := g.Encode()
	if err != nil {
		t.Fatal(err)
	}
	back, err := DecodeGateResult(raw)
	if err != nil {
		t.Fatal(err)
	}
	again, err := back.Encode()
	if err != nil || !bytes.Equal(raw, again) {
		t.Fatalf("re-encode: %v\n%s\n%s", err, raw, again)
	}
}

// TestCALV0016_PassedNeedsACleanExitAtTheCandidate: PASSED is refused
// for a dirty worktree, another tree, or an outcome other than EXIT.
func TestCALV0016_PassedNeedsACleanExitAtTheCandidate(t *testing.T) {
	other, dirty := strings.Repeat("b", 40), false
	for name, spoil := range map[string]func(*GateResult){
		"dirty":   func(g *GateResult) { g.PorcelainClean = &dirty },
		"moved":   func(g *GateResult) { g.ExecutedTreeOid = &other },
		"timeout": func(g *GateResult) { g.OutcomeClass, g.ExitCode = "TIMEOUT", nil },
	} {
		g := passedResult()
		spoil(&g)
		if _, err := g.Encode(); wire.CodeOf(err) != wire.CodeMalformed {
			t.Errorf("%s: %v", name, err)
		}
		g.State = "FAILED"
		if _, err := g.Encode(); err != nil {
			t.Errorf("%s as FAILED: %v", name, err)
		}
	}
}

// TestCALV0017_ManifestRoundTrips.
func TestCALV0017_ManifestRoundTrips(t *testing.T) {
	id, err := wire.ParseTicketID("ticketId", "ticket:acme:main:AT-0001")
	if err != nil {
		t.Fatal(err)
	}
	d := wire.Sum([]byte("x"))
	budget := map[string]BudgetField{}
	for _, name := range intent.LaneBudgetNames {
		budget[name] = BudgetField{State: "NOT_OBSERVED"}
	}
	m := Manifest{AttemptID: testAttempt, Generation: "1", TicketID: id, TicketRevision: wire.CountOf(1), TicketRecordSha256: d, PolicySha256: d,
		ConfigSha256: d, BaseCommit: strings.Repeat("c", 40), CandidateTreeOid: strings.Repeat("a", 40), GateResults: []string{string(d)},
		Budget: budget, ScopeCheck: "WITHIN", Mode: "DEVELOPMENT"}
	raw, err := m.Encode()
	if err != nil {
		t.Fatal(err)
	}
	back, err := DecodeManifest(raw)
	if err != nil || back.CandidateTreeOid != m.CandidateTreeOid || len(back.GateResults) != 1 || back.TicketID.Raw != id.Raw {
		t.Fatalf("decode: %+v %v", back, err)
	}
}
