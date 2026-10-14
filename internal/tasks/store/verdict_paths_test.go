package store_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// The write set and governed domain of the TCB-V0-037 tests. They match the
// Core verdict fixture (internal/taskman TestTCBV0038...), whose src/ hunk is
// anchored and whose lib/ hunk is only cited, so the two substituted arrays
// below are exactly the --touch-paths values that test evaluates.
var (
	verdictWriteSet = []string{"lib/", "src/"}
	verdictDomain   = []string{"src/"}
)

const (
	verdictWriteSetJSON = `["lib/","src/"]`
	verdictDomainJSON   = `["src/"]`
)

// verdictPathsGate prints the substituted verdict and touch placeholders.
func verdictPathsGate(id string) wire.Value {
	argv := []string{"sh", "-c", `printf '%s|%s' "$1" "$2"`, id, store.GateArgVerdictPaths, store.GateArgTouchPaths}
	return obj("gateId", str(id), "kind", str("COMMAND"), "argv", wire.Strings(argv), "cwd", str("WORKTREE"),
		"env", wire.Strings(nil), "timeoutSeconds", str("30"), "expected", obj("exitCode", str("0"), "reducer", wire.Null()),
		"evidence", wire.Strings([]string{"output"}), "inputs", wire.Strings(nil), "sharedResource", wire.Null(), "reusable", wire.Bool(false), "required", wire.Bool(true))
}

func verdictPayload(title string, touch, verdict []string, empty bool) wire.Value {
	payload := createPayload(title)
	effects, _ := payload.Obj.Get("effects")
	effects.Obj.Set("touchPaths", wire.Strings(touch))
	if verdict != nil || empty {
		effects.Obj.Set("verdictPaths", wire.Strings(verdict))
	}
	return payload
}

func setEffectsPayload(touch, verdict []string) wire.Value {
	effects := obj("coverage", str("QUALIFIED"), "externalUnbounded", wire.Bool(false), "resources", wire.Array(), "touchPaths", wire.Strings(touch))
	if verdict != nil {
		effects.Obj.Set("verdictPaths", wire.Strings(verdict))
	}
	return obj("capabilities", wire.Strings(nil), "effects", effects, "executionClass", str("AUTONOMOUS"))
}

// runVerdictGate creates a ticket with touch and verdict, claims, submits and
// runs verdictPathsGate, and returns the store, the ticket, the claim and the
// gate output.
func runVerdictGate(t *testing.T, verdict []string) (*leaseStore, string, *store.Report, string) {
	t.Helper()
	s := newLeaseStore(t, verdictPathsGate("verdict"))
	created := mutate(t, s.repo, envelope("create-verdict", mutation.OpCreate, "", "", verdictPayload("verdict", verdictWriteSet, verdict, false)))
	if created.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("create: %+v", created)
	}
	s.t0 = now(t)
	claim, _ := s.submitted(t, created.Ticket, "src", 0)
	s.passes(t, "gate-verdict", gateOf(claim, "verdict"), 2)
	g := s.results(t, claim.AttemptID)["verdict"]
	if g == nil || g.State != "PASSED" || len(g.Evidence) != 1 {
		t.Fatalf("verdict gate: %+v", g)
	}
	return s, created.Ticket, claim, string(s.evidence(t, string(g.Evidence[0].Sha256)))
}

// TestTCBV0037_VerdictPathsPlaceholder: a ticket declaring its full write set
// as touchPaths and a narrower verdictPaths gates on the narrow list through
// {claim.verdictPaths} while {claim.touchPaths} keeps the write set; without
// verdictPaths both placeholders substitute touchPaths (the TCB-V0-015
// behaviour). An edit after the claim refuses STALE_TICKET before the gate.
func TestTCBV0037_VerdictPathsPlaceholder(t *testing.T) {
	t.Parallel()
	s, id, claim, out := runVerdictGate(t, verdictDomain)
	if want := verdictDomainJSON + "|" + verdictWriteSetJSON; out != want {
		t.Fatalf("declared: output %q, want %q", out, want)
	}
	if rec := s.record(t, id); strings.Join(rec.Effects.VerdictPaths, ",") != "src/" {
		t.Fatalf("stored verdict paths: %+v", rec.Effects)
	}
	edited, err := store.Mutate(context.Background(), s.repo, operator(), envelope("edit-verdict", mutation.OpPrioritize, id, "1", obj("priority", str("P1"), "order", str("0"))), s.at(t, 3))
	if err != nil || edited.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("edit: %+v %v", edited, err)
	}
	before := storeDigest(t, s.repo)
	if report, err := s.gate(t, "gate-stale", gateOf(claim, "verdict"), 4); wire.CodeOf(err) != wire.CodeStaleTicket || report.Unretryable {
		t.Fatalf("edited ticket: %+v %v", report, err)
	}
	if storeDigest(t, s.repo) != before {
		t.Fatal("a refused substitution changed the store")
	}
	auditOK(t, s.repo)

	s, id, _, out = runVerdictGate(t, nil)
	if want := verdictWriteSetJSON + "|" + verdictWriteSetJSON; out != want {
		t.Fatalf("absent: output %q, want %q", out, want)
	}
	if rec := s.record(t, id); rec.Effects.VerdictPaths != nil {
		t.Fatalf("absent verdict paths read back: %+v", rec.Effects)
	}
	auditOK(t, s.repo)
}

// TestTCBV0036_MutationsRefuseUncoveredOrEmptyVerdictPaths: CREATE and
// SET_EFFECTS refuse, while reading the payload, a verdict path no touchPaths
// key covers (OUT_OF_SCOPE) and an empty list (MALFORMED), each naming its
// reason and writing nothing; CREATE also refuses an unsorted or duplicate
// list MALFORMED.
// SET_EFFECTS replaces the whole effects object, so omitting verdictPaths
// there returns the ticket to the touchPaths fallback. A record carrying the
// key keeps one canonical encoding through the store.
func TestTCBV0036_MutationsRefuseUncoveredOrEmptyVerdictPaths(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	refused := func(name string, env []byte, code, detail string) {
		t.Helper()
		before := storeDigest(t, s.repo)
		r, err := store.Mutate(context.Background(), s.repo, operator(), env, now(t))
		if wire.CodeOf(err) != code || !strings.Contains(err.Error(), detail) {
			t.Fatalf("%s: want %s %s, got %+v %v", name, code, detail, r, err)
		}
		if storeDigest(t, s.repo) != before {
			t.Fatalf("%s: a refusal changed the store", name)
		}
	}
	refused("create outside", envelope("c-out", mutation.OpCreate, "", "", verdictPayload("out", verdictDomain, []string{"lib/x.go"}, false)), wire.CodeOutOfScope, ticket.VerdictPathOutsideTouchPaths)
	refused("create exact key", envelope("c-exact", mutation.OpCreate, "", "", verdictPayload("exact", []string{"src"}, []string{"src/a.go"}, false)), wire.CodeOutOfScope, ticket.VerdictPathOutsideTouchPaths)
	refused("create empty", envelope("c-empty", mutation.OpCreate, "", "", verdictPayload("empty", verdictDomain, nil, true)), wire.CodeMalformed, ticket.VerdictPathsEmpty)
	refused("create unsorted", envelope("c-unsorted", mutation.OpCreate, "", "", verdictPayload("unsorted", verdictWriteSet, []string{"src/b", "src/a"}, false)), wire.CodeMalformed, "not canonical-byte sorted")
	refused("create duplicate", envelope("c-duplicate", mutation.OpCreate, "", "", verdictPayload("duplicate", verdictWriteSet, []string{"src/a", "src/a"}, false)), wire.CodeMalformed, "duplicate element")

	created := mutate(t, s.repo, envelope("c-ok", mutation.OpCreate, "", "", verdictPayload("ok", verdictWriteSet, verdictDomain, false)))
	if created.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("create: %+v", created)
	}
	rec := s.record(t, created.Ticket)
	if raw := rec.Encode(); !bytes.Contains(raw, []byte(`"touchPaths":["lib/","src/"],"verdictPaths":["src/"]}`)) {
		t.Fatalf("stored effects are not canonical: %s", raw)
	} else if back, err := ticket.Decode(raw); err != nil || !bytes.Equal(back.Encode(), raw) {
		t.Fatalf("stored record round trip: %v", err)
	}
	refused("set-effects outside", envelope("e-out", mutation.OpSetEffects, created.Ticket, "1", setEffectsPayload(verdictDomain, []string{"lib/"})), wire.CodeOutOfScope, ticket.VerdictPathOutsideTouchPaths)
	refused("set-effects empty", envelope("e-empty", mutation.OpSetEffects, created.Ticket, "1", setEffectsPayload(verdictWriteSet, []string{})), wire.CodeMalformed, ticket.VerdictPathsEmpty)
	dropped := mutate(t, s.repo, envelope("e-drop", mutation.OpSetEffects, created.Ticket, "1", setEffectsPayload(verdictWriteSet, nil)))
	if dropped.Outcome.Outcome != mutation.OutcomeCompleted || s.record(t, created.Ticket).Effects.VerdictPaths != nil {
		t.Fatalf("set-effects without verdictPaths: %+v", dropped)
	}
	auditOK(t, s.repo)
}
