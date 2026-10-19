package mutation_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const sccMutationCommit = "0123456789abcdef0123456789abcdef01234567"

// TestSCCV0010_HoldConditionRoundTrip: a HOLD may carry one typed condition
// of each kind; it is stored on the hold, survives the record codec and the
// payload renderer byte for byte, and a hold without one encodes exactly as
// before the condition existed.
func TestSCCV0010_HoldConditionRoundTrip(t *testing.T) {
	conds := map[string]wire.Value{
		ticket.HoldConditionCommitContained: obj("kind", str(ticket.HoldConditionCommitContained), "ref", str("refs/heads/main"), "commit", str(sccMutationCommit)),
		ticket.HoldConditionPathExists:      obj("kind", str(ticket.HoldConditionPathExists), "ref", str("refs/remotes/origin/main"), "path", str("internal/tasks/")),
		ticket.HoldConditionSymbolDefined:   obj("kind", str(ticket.HoldConditionSymbolDefined), "ref", str("refs/heads/main"), "path", str("internal/tasks/cli/cli.go"), "symbol", str("Run")),
	}
	for kind, cond := range conds {
		t.Run(kind, func(t *testing.T) {
			payload := obj("holdId", str("h"), "reason", str("wait for upstream"), "condition", cond)
			if _, err := mutation.CanonicalPayload(mutation.OpHold, payload); err != nil {
				t.Fatalf("payload refused: %v", err)
			}
			rec := step(t, owner, nil, fixture.Ticket("AT-01"), mutation.OpHold, payload, "1")
			if rec.Status != ticket.StatusHeld || len(rec.Holds) != 1 || rec.Holds[0].Condition == nil || rec.Holds[0].Condition.Kind != kind {
				t.Fatalf("hold not stored with its condition: %s %+v", rec.Status, rec.Holds)
			}
			if !wire.Equal(rec.Holds[0].Condition.Value(), cond) {
				t.Fatalf("condition value %s; want %s", wire.EncodeFile(rec.Holds[0].Condition.Value()), wire.EncodeFile(cond))
			}
			raw := rec.Encode()
			back, err := ticket.Decode(raw)
			if err != nil {
				t.Fatalf("decode: %v\n%s", err, raw)
			}
			if !bytes.Equal(back.Encode(), raw) || back.Holds[0].Condition == nil {
				t.Fatalf("record round trip changed bytes or dropped the condition")
			}
			rendered := mutation.PayloadValue(&mutation.HoldPayload{HoldID: "h", Reason: "wait for upstream", Condition: back.Holds[0].Condition})
			if !wire.Equal(rendered, payload) {
				t.Fatalf("rendered payload %s; want %s", wire.EncodeFile(rendered), wire.EncodeFile(payload))
			}
		})
	}

	plain := step(t, owner, nil, fixture.Ticket("AT-01"), mutation.OpHold, obj("holdId", str("h"), "reason", str("wait")), "1")
	if plain.Holds[0].Condition != nil || bytes.Contains(plain.Encode(), []byte(`"condition"`)) {
		t.Fatalf("a hold without a condition must encode without the key")
	}
	if bytes.Contains(wire.EncodeFile(mutation.PayloadValue(&mutation.HoldPayload{HoldID: "h", Reason: "wait"})), []byte(`"condition"`)) {
		t.Fatalf("a payload without a condition must render without the key")
	}
}

// TestSCCV0010_HoldConditionRefusals: the condition is closed per kind, its
// ref is a full refname under refs/, its symbol is identifier-shaped and
// bounded, and a SYMBOL_DEFINED path names a file.
func TestSCCV0010_HoldConditionRefusals(t *testing.T) {
	cc := func(kv ...interface{}) wire.Value { return obj(kv...) }
	k := func(s string) wire.Value { return str(s) }
	cases := map[string]wire.Value{
		"short ref":        cc("kind", k("PATH_EXISTS"), "ref", k("main"), "path", k("a/b")),
		"range ref":        cc("kind", k("PATH_EXISTS"), "ref", k("refs/heads/x..y"), "path", k("a/b")),
		"lock ref":         cc("kind", k("PATH_EXISTS"), "ref", k("refs/heads/x.lock"), "path", k("a/b")),
		"inner lock ref":   cc("kind", k("PATH_EXISTS"), "ref", k("refs/heads/x.lock/y"), "path", k("a/b")),
		"reflog ref":       cc("kind", k("PATH_EXISTS"), "ref", k("refs/heads/x@{1}"), "path", k("a/b")),
		"revision ref":     cc("kind", k("PATH_EXISTS"), "ref", k("refs/heads/x~1"), "path", k("a/b")),
		"dash symbol":      cc("kind", k("SYMBOL_DEFINED"), "ref", k("refs/heads/main"), "path", k("a.go"), "symbol", k("a-b")),
		"dollar symbol":    cc("kind", k("SYMBOL_DEFINED"), "ref", k("refs/heads/main"), "path", k("a.go"), "symbol", k("$x")),
		"long symbol":      cc("kind", k("SYMBOL_DEFINED"), "ref", k("refs/heads/main"), "path", k("a.go"), "symbol", k(strings.Repeat("s", ticket.HoldConditionMaxSymbolBytes+1))),
		"digit symbol":     cc("kind", k("SYMBOL_DEFINED"), "ref", k("refs/heads/main"), "path", k("a.go"), "symbol", k("9a")),
		"directory symbol": cc("kind", k("SYMBOL_DEFINED"), "ref", k("refs/heads/main"), "path", k("a/"), "symbol", k("A")),
		"extra key":        cc("kind", k("PATH_EXISTS"), "ref", k("refs/heads/main"), "path", k("a/b"), "commit", k(sccMutationCommit)),
		"missing key":      cc("kind", k("COMMIT_CONTAINED"), "ref", k("refs/heads/main")),
		"short commit":     cc("kind", k("COMMIT_CONTAINED"), "ref", k("refs/heads/main"), "commit", k("0123456")),
		"unknown kind":     cc("kind", k("TEST_PASSES"), "ref", k("refs/heads/main"), "path", k("a/b")),
		"not an object":    k("refs/heads/main"),
	}
	for name, cond := range cases {
		t.Run(name, func(t *testing.T) {
			payload := obj("holdId", str("h"), "reason", str("wait"), "condition", cond)
			if _, err := mutation.CanonicalPayload(mutation.OpHold, payload); err == nil {
				t.Fatalf("payload accepted")
			}
			if _, err := mutation.Decode(envelope("p", owner, "AT-01", "1", mutation.OpHold, payload)); err == nil {
				t.Fatalf("envelope accepted")
			}
		})
	}
	if !ticket.HoldConditionRefValid("refs/heads/release/v1.0") || ticket.HoldConditionRefValid("refs/heads/") ||
		ticket.HoldConditionRefValid("refs/heads/foo.lock/bar") || ticket.HoldConditionRefValid("refs/heads/.x/y") {
		t.Fatalf("refname rule misjudges a nested branch, a trailing slash, an inner .lock or a dot component")
	}
}

// TestSCCV0010_AdoptCarriesHoldCondition: adopting a file hold composes a
// HOLD that keeps its condition, and a file that only adds or changes the
// condition of an existing hold is not covered by the composition rule.
func TestSCCV0010_AdoptCarriesHoldCondition(t *testing.T) {
	cond := &ticket.HoldCondition{Kind: ticket.HoldConditionCommitContained, Ref: "refs/heads/main", Commit: sccMutationCommit}
	canonical := fixture.Ticket("AT-01")
	file := fileOf(t, canonical, func(r *ticket.Record) {
		r.Status = ticket.StatusHeld
		r.Holds = []ticket.Hold{{HoldID: "h", Actor: "someone", Reason: "wait", PlacedAt: now, Condition: cond}}
	})
	plan := adopt(t, newCtx(t, operator, nil, canonical), canonical, file)
	want(t, plan, mutation.OutcomeCompleted, "")
	if h := plan.Post.Holds; len(h) != 1 || h[0].Condition == nil || !wire.Equal(h[0].Condition.Value(), cond.Value()) {
		t.Fatalf("adopted hold lost its condition: %+v", h)
	}

	held := step(t, owner, nil, fixture.Ticket("AT-01"), mutation.OpHold, obj("holdId", str("h"), "reason", str("wait")), "1")
	edited := fileOf(t, held, func(r *ticket.Record) { r.Holds[0].Condition = cond })
	plan = adopt(t, newCtx(t, operator, nil, held), held, edited)
	if plan.Outcome.Outcome != mutation.OutcomeValidationFailed || !plan.Outcome.HasCode(wire.CodeAdoptUnsupportedField) {
		t.Fatalf("condition-only edit: %s %v (%s)", plan.Outcome.Outcome, plan.Outcome.Codes, plan.Detail)
	}
}
