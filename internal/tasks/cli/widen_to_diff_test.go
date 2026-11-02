package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// LGL-V0-812 (V1-3827): `claim --scope-from-diff REV --widen-to-diff` on a
// ticket whose declared scope does not cover the diff adds exactly the
// uncovered paths to its touchPaths through one ticket set-effects write
// with its own receipt, then claims inside the widened DECLARED scope. A
// retry replays both and widens nothing more. Without the flag the TCD-V0-175
// refusal is unchanged, and the flag needs --scope-from-diff.
func TestLGLV0812_ClaimWidensToTheDiffWithAReceipt(t *testing.T) {
	root, base, head, _ := diffScopeRepo(t)
	narrow, _ := effectsTicket(t, root, "narrow", "P1", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["src/"]}`)
	state := filepath.Join(root, ".git", "taskman")

	// The default is the TCD-V0-175 refusal, writing nothing.
	before := fixture.TreeSnapshot(t, state)
	r := atm(t, root, nil, "claim", narrow, "--holder", "tester", "--request-id", "default", "--base", base, "--scope-from-diff", head)
	if r.res.Outcome == wire.OutcomeOK || !hasCode(r.res, wire.CodeOutOfScope) {
		t.Fatalf("default claim: %s", r.stdout)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, state)) {
		t.Fatal("the refused default claim changed the store")
	}
	if x := atm(t, root, nil, "claim", narrow, "--holder", "tester", "--request-id", "bad", "--widen-to-diff"); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeMalformed) || !strings.Contains(string(x.stdout), "LGL-V0-812") {
		t.Fatalf("--widen-to-diff without --scope-from-diff: %s", x.stdout)
	}

	claim := func() run {
		t.Helper()
		return atm(t, root, nil, "claim", narrow, "--holder", "tester", "--stage", "implement", "--request-id", "wide", "--base", base, "--scope-from-diff", head, "--widen-to-diff")
	}
	c := claim()
	if c.res.Outcome != wire.OutcomeOK {
		t.Fatalf("widening claim: %s", c.stdout)
	}
	item := c.res.Items[0]
	if field(field(item, "scope"), "source").Str != "DECLARED" {
		t.Fatalf("claim scope: %s", c.stdout)
	}
	widening := field(item, "scopeWidening")
	if field(widening, "requestId").Str != "wide/widen-to-diff" || field(widening, "receipt").Str == "" {
		t.Fatalf("no widening receipt named: %s", c.stdout)
	}
	if !strings.Contains(strings.Join(c.res.Warnings, "\n"), "SCOPE_WIDENED_TO_DIFF") {
		t.Fatalf("no widening warning: %q", c.res.Warnings)
	}
	show := atm(t, root, nil, "ticket", "show", narrow)
	var touch []string
	for _, p := range field(field(field(show.res.Items[0], "record"), "effects"), "touchPaths").Arr {
		touch = append(touch, p.Str)
	}
	if strings.Join(touch, " ") != "docs/new.md docs/old.md src/" {
		t.Fatalf("touchPaths after the widening: %v", touch)
	}
	// The widening is an ordinary set-effects write: its request replays
	// through the request index.
	if x := atm(t, root, nil, "receipt", "audit"); x.code != 0 || x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("receipt audit: %s", x.stdout)
	}

	// A retry replays the claim and widens nothing again.
	headBefore := fixture.TreeSnapshot(t, state)
	again := claim()
	if again.res.Outcome != wire.OutcomeOK || !field(again.res.Items[0], "replayed").Bool || field(again.res.Items[0], "attemptId").Str != field(item, "attemptId").Str {
		t.Fatalf("retry: %s", again.stdout)
	}
	if strings.Contains(strings.Join(again.res.Warnings, "\n"), "SCOPE_WIDENED_TO_DIFF") {
		t.Fatalf("retry widened again: %q", again.res.Warnings)
	}
	if !fixture.SameTree(headBefore, fixture.TreeSnapshot(t, state)) {
		t.Fatal("the replayed claim changed the store")
	}
}
