package cli_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCDV0176_CompleteManualOverridesFailedIntentChecksOnlyWithAReason
// (V1-2535): complete-manual stays the close for a ticket whose intent checks
// FAILED (here TICKET_STATE of a MANUAL ticket), but a blank reason
// refuses MALFORMED at /payload/reason naming the failed checks and writes
// nothing; a nonblank reason commits and warns with each overridden check,
// and an exact replay does not repeat the warning. A ticket whose checks
// PASSED still closes with an empty reason. A note set on the COMPLETED
// ticket returns an explicit COMPLETED outcome and is readable.
func TestTCDV0176_CompleteManualOverridesFailedIntentChecksOnlyWithAReason(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %s", x.stdout)
	}
	manual := strings.Replace(strings.Replace(createPayloadJSON, `"title":"Console ticket"`, `"title":"manual"`, 1), `"executionClass":"AUTONOMOUS"`, `"executionClass":"MANUAL"`, 1)
	mx := atm(t, r.Root, nil, "ticket", "create", "--request-id", "create-manual", "--issued-at", "2026-10-08T12:00:00Z", "--payload", manual)
	if mx.res.Outcome != wire.OutcomeOK || field(mx.res.Items[0], "outcome").Str != "COMPLETED" {
		t.Fatalf("create manual: %s", mx.stdout)
	}
	incomplete := field(mx.res.Items[0], "ticketId").Str
	qualified, _ := effectsTicket(t, r.Root, "qualified", "P2", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["src/"]}`)
	checks := func(id string) string {
		t.Helper()
		return field(atm(t, r.Root, nil, "ticket", "show", id).res.Items[0], "intentChecks").Str
	}
	if checks(incomplete) != "FAILED" || checks(qualified) != "PASSED" {
		t.Fatalf("fixture intent checks: incomplete %s, qualified %s", checks(incomplete), checks(qualified))
	}
	complete := func(req, target, reason string) run {
		t.Helper()
		return atm(t, r.Root, nil, "ticket", "complete-manual", "--request-id", req, "--target", target, "--expected-revision", "1",
			"--issued-at", "2026-10-27T12:00:00Z", "--payload", `{"evidence":[],"reason":"`+reason+`"}`)
	}
	overrides := func(x run) []string {
		var out []string
		for _, w := range x.res.Warnings {
			if strings.HasPrefix(w, "COMPLETE_MANUAL overrode failed intent checks (TCD-V0-176)") {
				out = append(out, w)
			}
		}
		return out
	}

	before := fixture.TreeSnapshot(t, filepath.Join(r.Root, ".git", "taskman"))
	for _, blank := range []string{"", "  "} {
		x := complete("blank", incomplete, blank)
		if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeMalformed) || !strings.Contains(string(x.stdout), "/payload/reason") ||
			!strings.Contains(string(x.stdout), wire.CodeTicketState) {
			t.Fatalf("blank reason %q over FAILED checks: %s", blank, x.stdout)
		}
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, filepath.Join(r.Root, ".git", "taskman"))) {
		t.Fatal("a refused complete-manual changed the store")
	}

	done := complete("override", incomplete, "delivered in batch #1; effects never qualified")
	if done.res.Outcome != wire.OutcomeOK || field(done.res.Items[0], "outcome").Str != "COMPLETED" {
		t.Fatalf("override with a reason: %s", done.stdout)
	}
	if w := overrides(done); len(w) != 1 || !strings.Contains(w[0], wire.CodeTicketState+": ") {
		t.Fatalf("override warning: %q", w)
	}
	replay := complete("override", incomplete, "delivered in batch #1; effects never qualified")
	if !field(replay.res.Items[0], "replayed").Bool || len(overrides(replay)) != 0 {
		t.Fatalf("replay: %s", replay.stdout)
	}
	if x := complete("passed", qualified, ""); x.res.Outcome != wire.OutcomeOK || len(overrides(x)) != 0 {
		t.Fatalf("PASSED checks with an empty reason: %s", x.stdout)
	}

	note := atm(t, r.Root, nil, "ticket", "note", "set", incomplete, "--request-id", "note", "--issued-at", "2026-10-27T12:02:00Z", "--text", "closed over failed checks")
	if note.res.Outcome != wire.OutcomeOK || field(note.res.Items[0], "outcome").Str != "COMPLETED" {
		t.Fatalf("note set on a COMPLETED ticket: %s", note.stdout)
	}
	if shown := atm(t, r.Root, nil, "ticket", "note", "show", incomplete); field(shown.res.Items[0], "text").Str != "closed over failed checks" {
		t.Fatalf("note show: %s", shown.stdout)
	}
}

// TestTCDV0177_CompleteManualHelpStatesTheValidatorLimits (V1-2534,
// V1-2627): complete-manual verbose help (terse help points at it) names evidence as sha256 digests, what each
// should identify, and the reason bound, and both limits hold: prose evidence
// refuses MALFORMED at its index and a reason one byte over the bound refuses
// LIMIT_EXCEEDED.
func TestTCDV0177_CompleteManualHelpStatesTheValidatorLimits(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %s", x.stdout)
	}
	id, _ := effectsTicket(t, r.Root, "limits", "P2", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["src/"]}`)
	if h := atm(t, r.Root, nil, "ticket", "complete-manual", "--help"); field(h.res.Items[0], "verboseHelp").Str != "corvint-tasks ticket complete-manual --help --verbose" {
		t.Fatalf("terse help does not point at the verbose limits: %s", h.stdout)
	}
	h := atm(t, r.Root, nil, "ticket", "complete-manual", "--help", "--verbose")
	text := field(h.res.Items[0], "completeManual").Str
	for _, want := range []string{"sha256 digests", "64 lowercase hex", "suggestedEvidence", "attach-evidence digest", "0..65536 bytes", "TCD-V0-176", "TEA-V0-001"} {
		if !strings.Contains(text, want) {
			t.Fatalf("verbose help omits %q: %s", want, h.stdout)
		}
	}
	complete := func(req, payload string) run {
		t.Helper()
		return atm(t, r.Root, nil, "ticket", "complete-manual", "--request-id", req, "--target", id, "--expected-revision", "1", "--payload", payload)
	}
	if x := complete("prose", `{"evidence":["merged in batch #1"],"reason":"done"}`); !hasCode(x.res, wire.CodeMalformed) || !strings.Contains(string(x.stdout), "/payload/evidence/0") {
		t.Fatalf("prose evidence: %s", x.stdout)
	}
	if x := complete("long", `{"evidence":[],"reason":"`+strings.Repeat("x", wire.MaxProseBytes+1)+`"}`); !hasCode(x.res, wire.CodeLimitExceeded) {
		t.Fatalf("over-long reason: %s", x.stdout)
	}
	digest := string(wire.Sum([]byte("https://github.com/beamfall/corvint/actions/runs/1")))
	if x := complete("ok", `{"evidence":["`+digest+`"],"reason":"`+strings.Repeat("x", wire.MaxProseBytes)+`"}`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("a digest and a reason at the bound: %s", x.stdout)
	}
}

// TestTCDV0178_ReleaseReasonRefusalsNameTheAdmittedCodes (V1-2654): free
// text refuses MALFORMED naming every admitted code and the plain release
// without echoing the text; REVIEW_RETURNED outside review refuses
// TICKET_STATE saying how to free the attempt without a handoff; release help
// states the rule; and a non-handoff code releases.
func TestTCDV0178_ReleaseReasonRefusalsNameTheAdmittedCodes(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, claimed := leaseCLIStore(t, 1, time.Now().UTC().Add(-12*time.Minute))
	a := claimed[0]
	release := func(req, reason string) run {
		t.Helper()
		return atm(t, root, nil, "release", "--attempt", a.AttemptID, "--generation", string(a.Generation), "--request-id", req, "--reason", reason)
	}
	x := release("free-text", "lane finished")
	if !hasCode(x.res, wire.CodeMalformed) || !strings.Contains(string(x.stdout), "TCD-V0-178") || strings.Contains(string(x.stdout), "lane finished") {
		t.Fatalf("free-text reason: %s", x.stdout)
	}
	for _, code := range wire.Codes {
		if !strings.Contains(string(x.stdout), code) {
			t.Fatalf("refusal omits admitted code %s: %s", code, x.stdout)
		}
	}
	if x := release("returned", wire.CodeReviewReturned); !hasCode(x.res, wire.CodeTicketState) || !strings.Contains(string(x.stdout), "omit --reason") {
		t.Fatalf("REVIEW_RETURNED outside review: %s", x.stdout)
	}
	h := atm(t, root, nil, "release", "--help", "--verbose")
	if rule := field(h.res.Items[0], "reasonRule").Str; !strings.Contains(rule, "exactly one of reasonCodes") || !strings.Contains(rule, "Omit --reason") {
		t.Fatalf("release help reasonRule: %s", h.stdout)
	}
	if x := release("budget", wire.CodeBudgetExceeded); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("a non-handoff code should release: %s", x.stdout)
	}
}

// TestTCDV0178_TerseReleaseHelpListsReasonCodes (V1-3200): terse release
// help carries the closed reasonCodes the refusal names, keeps reasonRule
// verbose-only, and an unknown code-shaped reason refusal names --reason.
func TestTCDV0178_TerseReleaseHelpListsReasonCodes(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, claimed := leaseCLIStore(t, 1, time.Now().UTC().Add(-12*time.Minute))
	for _, flag := range []string{"--help", "-h"} {
		h := atm(t, root, nil, "release", flag)
		item := h.res.Items[0]
		var got []string
		for _, v := range field(item, "reasonCodes").Arr {
			got = append(got, v.Str)
		}
		if strings.Join(got, ",") != strings.Join(wire.Codes, ",") || field(item, "reasonRule").Str != "" {
			t.Fatalf("terse release %s: %s", flag, h.stdout)
		}
	}
	a := claimed[0]
	x := atm(t, root, nil, "release", "--attempt", a.AttemptID, "--generation", string(a.Generation), "--request-id", "unknown-code", "--reason", "NOT_A_REASON_CODE")
	if !hasCode(x.res, wire.CodeMalformed) || !strings.Contains(string(x.stdout), "--reason takes exactly one of") || strings.Contains(string(x.stdout), "NOT_A_REASON_CODE") {
		t.Fatalf("unknown code reason: %s", x.stdout)
	}
}
