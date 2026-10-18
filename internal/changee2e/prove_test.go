package changee2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/behaviorfalsify"
	"github.com/Beamfall/corvint/internal/playwrightruns"
)

// TestMain lets the test binary act as the BBF control hook in the live proof tests: BBF runs
// the hook with no arguments and CORVINT_BEHAVIOR_COMMAND_ROLE set, as the companion does.
func TestMain(m *testing.M) {
	if role := os.Getenv("CORVINT_BEHAVIOR_COMMAND_ROLE"); role != "" && len(os.Args) == 1 {
		if err := RunHook(context.Background(), role, os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// CCE-V0-008 and the limits section: repeats outside 1..100 and more than 16 scenarios refuse with
// named codes instead of sampling; retries and workers are not options at all.
func TestCCEV0008ProveLimitsRefuse(t *testing.T) {
	sc := Scenario{Obligation: "obligation:a/b", Placement: "e2e/corvint-compiled/a.spec.ts", Text: []byte("x\n"),
		TestTitle: "obligation:a/b", AssertionID: "assert:a/b", AssertionLine: 1, Matcher: "toHaveText", Locator: "s", Value: "v"}
	many := make([]Scenario, MaxScenarios+1)
	for i := range many {
		many[i] = sc
		many[i].Placement = fmt.Sprintf("e2e/corvint-compiled/a%d.spec.ts", i)
	}
	for _, tc := range []struct {
		name      string
		repeats   int
		scenarios []Scenario
		code      string
	}{
		{"negative repeats", -1, []Scenario{sc}, "e2e-repeats-over-limit"},
		{"repeats over limit", MaxRepeats + 1, []Scenario{sc}, "e2e-repeats-over-limit"},
		{"scenarios over limit", 1, many, "e2e-scenarios-over-limit"},
		{"no scenarios", 1, nil, "e2e-scenarios-required"},
		{"escaping placement", 1, []Scenario{{Obligation: "o", Placement: "../x.spec.ts", Text: []byte("x"), TestTitle: "o", AssertionID: "a", AssertionLine: 1, Matcher: "m", Locator: "l"}}, "e2e-scenario-invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Prove(bg, ProveOptions{Root: t.TempDir(), Base: "HEAD", Head: "HEAD", Repeats: tc.repeats, Scenarios: tc.scenarios})
			if got := refusalCode(err); got != tc.code {
				t.Fatalf("refusal = %q (%v), want %q", got, err, tc.code)
			}
		})
	}
}

func pool(state string, n int, at bool) sideProof {
	s := sideProof{Qualified: true}
	for range n {
		s.Outcomes = append(s.Outcomes, repeatOutcome{State: state, AtOutcomeAssertion: at})
	}
	s.Stability = stability(s.Outcomes)
	return s
}

func controls(locator, value string) []controlProof {
	return []controlProof{{Kind: string(behaviorfalsify.WrongLocator), Status: locator}, {Kind: string(behaviorfalsify.WrongExpectedValue), Status: value}}
}

// CCE-V0-009: the verdict is the first rule that holds, in the specified order.
func TestCCEV0009VerdictOrder(t *testing.T) {
	passHead := pool("passed", 3, false)
	failBase := pool("failed", 3, true)
	killed := controls("killed", "killed")
	unqualified := passHead
	unqualified.Qualified = false
	mixed := pool("passed", 2, false)
	mixed.Outcomes = append(mixed.Outcomes, repeatOutcome{State: "failed"})
	mixed.Stability = stability(mixed.Outcomes)
	offAssertion := pool("failed", 3, false)
	mixedBase := pool("failed", 2, true)
	mixedBase.Outcomes = append(mixedBase.Outcomes, repeatOutcome{State: "passed"})
	for _, tc := range []struct {
		name       string
		head, base sideProof
		controls   []controlProof
		want       string
	}{
		{"1 head unqualified beats everything", unqualified, failBase, controls("survived", "survived"), VerdictInconclusive},
		{"1 base unqualified", passHead, sideProof{Qualified: false}, killed, VerdictInconclusive},
		{"2 intermittent head beats a head failure", mixed, failBase, killed, VerdictUnstable},
		{"2 flaky repeat is intermittent", pool("flaky", 3, false), failBase, killed, VerdictUnstable},
		{"3 deterministic head failure", pool("failed", 3, true), failBase, killed, VerdictFailsAtHead},
		{"3 head timeout", pool("timedOut", 3, false), failBase, killed, VerdictFailsAtHead},
		{"4 wrong-locator survived", passHead, failBase, controls("survived", "killed"), VerdictControlSurvived},
		{"4 wrong-value survived beats a not-run control", passHead, failBase, controls("not_run", "survived"), VerdictControlSurvived},
		{"5 control invalid", passHead, failBase, controls("invalid_control", "killed"), VerdictInconclusive},
		{"5 control infrastructure", passHead, failBase, controls("killed", "infrastructure_failed"), VerdictInconclusive},
		{"5 control missing", passHead, failBase, nil, VerdictInconclusive},
		{"6 every base repeat fails at the assertion", passHead, failBase, killed, VerdictDiscriminates},
		{"7 every base repeat passes", passHead, pool("passed", 3, false), killed, VerdictPreserves},
		{"8 mixed base", passHead, mixedBase, killed, VerdictInconclusive},
		{"8 base fails off the assertion", passHead, offAssertion, killed, VerdictInconclusive},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, note := verdict(tc.head, tc.base, tc.controls)
			if got != tc.want {
				t.Fatalf("verdict = %s (%s), want %s", got, note, tc.want)
			}
			if got == VerdictPreserves && !strings.Contains(note, "regression-preservation") {
				t.Fatalf("PRESERVES note %q must say regression preservation only", note)
			}
		})
	}
}

// CCE-V0-009 step 6: a base failure counts only when it is at the outcome assertion, matched from
// the Playwright failure message with its colour codes stripped.
func TestCCEV0009AssertionDetector(t *testing.T) {
	coloured := "Error: \x1b[2mexpect(\x1b[22m\x1b[31mlocator\x1b[39m\x1b[2m).\x1b[22mtoHaveText\x1b[2m(\x1b[22m\x1b[32mexpected\x1b[39m\x1b[2m)\x1b[22m failed\n\nLocator: getByTestId('slot-status')\nExpected: \x1b[32m\"Booked\"\x1b[39m\nReceived: \x1b[31m\"Pending\"\x1b[39m"
	for _, tc := range []struct {
		name, state, message, locator string
		want                          bool
	}{
		{"coloured outcome failure", "failed", coloured, "slot-status", true},
		{"double-quoted locator", "failed", "expect(locator).toHaveText(expected) failed\nLocator: getByTestId(\"slot-status\")", "slot-status", true},
		{"other locator", "failed", coloured, "slot", false},
		{"other matcher", "failed", "expect(locator).toBeVisible() failed\nLocator: getByTestId('slot-status')", "slot-status", false},
		{"navigation failure", "failed", "page.goto: net::ERR_CONNECTION_REFUSED", "slot-status", false},
		{"timed out is not at the assertion", "timedOut", coloured, "slot-status", false},
		{"passed", "passed", "", "slot-status", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := failsAtAssertion(tc.state, tc.message, "toHaveText", tc.locator); got != tc.want {
				t.Fatalf("failsAtAssertion = %v, want %v", got, tc.want)
			}
		})
	}
}

// CCE-V0-009 step 2 (PWR-V0-003 vocabulary): differing outcomes or any flaky repeat is
// INTERMITTENT; an empty pool is INSUFFICIENT.
func TestCCEV0009HeadIntermittency(t *testing.T) {
	o := func(states ...string) []repeatOutcome {
		out := []repeatOutcome{}
		for _, s := range states {
			out = append(out, repeatOutcome{State: s})
		}
		return out
	}
	for _, tc := range []struct {
		name string
		outs []repeatOutcome
		want string
	}{
		{"all passed", o("passed", "passed", "passed"), playwrightruns.Deterministic},
		{"all failed", o("failed", "failed"), playwrightruns.Deterministic},
		{"one differs", o("passed", "failed", "passed"), playwrightruns.Intermittent},
		{"flaky", o("flaky"), playwrightruns.Intermittent},
		{"empty", o(), playwrightruns.Insufficient},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := stability(tc.outs); got != tc.want {
				t.Fatalf("stability = %s, want %s", got, tc.want)
			}
		})
	}
}

// CCE-V0-009 steps 4-5: the controls perturb exactly the outcome assertion line of the real
// compiled scenario.
func TestCCEV0009ControlPerturbation(t *testing.T) {
	root, base := fixtureRepo(t)
	head := commitBehaviourChange(t, root)
	rep, err := Compile(bg, diffOpts(root, base, head))
	if err != nil || len(rep.Compiled()) == 0 {
		t.Fatalf("compile: %v", err)
	}
	sc := rep.Compiled()[0]
	lines := strings.Split(string(sc.Text), "\n")
	for _, tc := range []struct {
		kind behaviorfalsify.ControlKind
		want string
	}{
		{behaviorfalsify.WrongLocator, "getByTestId('corvint-wrong-locator')).toHaveText('Booked')"},
		{behaviorfalsify.WrongExpectedValue, "getByTestId('slot-status')).toHaveText('corvint-wrong-value')"},
	} {
		repl := wrongLocator
		if tc.kind == behaviorfalsify.WrongExpectedValue {
			repl = wrongValue
		}
		out, err := perturbLine(sc.Text, sc.AssertionLine, tc.kind, sc.Locator, sc.Matcher, sc.Value, repl)
		if err != nil {
			t.Fatalf("%s: %v", tc.kind, err)
		}
		got := strings.Split(string(out), "\n")
		for i := range lines {
			changed := got[i] != lines[i]
			if changed != (i == sc.AssertionLine-1) {
				t.Fatalf("%s changed line %d, want only %d", tc.kind, i+1, sc.AssertionLine)
			}
		}
		if !strings.Contains(got[sc.AssertionLine-1], tc.want) {
			t.Fatalf("%s line = %q", tc.kind, got[sc.AssertionLine-1])
		}
	}
	if _, err := perturbLine(sc.Text, sc.AssertionLine-1, behaviorfalsify.WrongLocator, sc.Locator, sc.Matcher, sc.Value, wrongLocator); err == nil {
		t.Fatal("a line without the assertion must refuse the perturbation")
	}
}

// liveProve runs Prove for base..head on the fixture, or skips without a local Playwright.
func liveProve(t *testing.T, root, base, head string) scenarioProof {
	t.Helper()
	modules := os.Getenv(envModules)
	if modules == "" {
		t.Skip("CORVINT_PLAYWRIGHT_MODULES is not set")
	}
	node := os.Getenv("CORVINT_PLAYWRIGHT_NODE")
	if node == "" {
		var err error
		if node, err = exec.LookPath("node"); err != nil {
			t.Skip("node is not available")
		}
	}
	rep, err := Compile(bg, diffOpts(root, base, head))
	if err != nil || len(rep.Compiled()) != 1 {
		t.Fatalf("compile: %v", err)
	}
	hook, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	out, err := Prove(bg, ProveOptions{Root: root, Base: base, Head: head, Repeats: DefaultRepeats, NodeModules: modules,
		NodeBin: node, Scenarios: rep.Compiled(), HookExecutable: hook})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), root) || strings.Contains(string(out), filepath.Dir(modules)) {
		t.Fatal("proof document carries an absolute path")
	}
	var doc proveReport
	if err := json.Unmarshal(out, &doc); err != nil || doc.Schema != ProveSchema || len(doc.Scenarios) != 1 || doc.Workers != 1 || doc.Retries != 0 {
		t.Fatalf("proof document: %v\n%s", err, out)
	}
	t.Logf("%s", out)
	return doc.Scenarios[0]
}

// CCE-V0-008/009 live: the one-screen behaviour change passes at head, fails at base on the
// outcome assertion, and both controls are killed.
func TestCCEV0009LiveDiscriminates(t *testing.T) {
	root, base := fixtureRepo(t)
	head := commitBehaviourChange(t, root)
	p := liveProve(t, root, base, head)
	requireOutcomes(t, p.Head, "passed", false)
	requireOutcomes(t, p.Base, "failed", true)
	requireVerdict(t, p, VerdictDiscriminates)
	if len(p.Controls) != 2 {
		t.Fatalf("controls = %+v, want both BBF controls", p.Controls)
	}
}

// CCE-V0-009 live: a change that keeps the outcome reads PRESERVES.
func TestCCEV0009LivePreserves(t *testing.T) {
	root, _ := fixtureRepo(t)
	base := commitBehaviourChange(t, root)
	booked := strings.ReplaceAll(*overlayText(t, "app/components/teesheet.head.html"), `class="slot open"`, `class="slot available"`)
	head := commitFiles(t, root, map[string]*string{"app/components/teesheet.html": text(booked)}, "rename slot class")
	p := liveProve(t, root, base, head)
	requireOutcomes(t, p.Head, "passed", false)
	requireOutcomes(t, p.Base, "passed", false)
	requireVerdict(t, p, VerdictPreserves)
}

// requireOutcomes checks the observed browser outcomes, which Prove keeps even when the receipt
// does not qualify.
func requireOutcomes(t *testing.T, s sideProof, state string, atAssertion bool) {
	t.Helper()
	if len(s.Outcomes) != DefaultRepeats || s.Stability != playwrightruns.Deterministic {
		t.Fatalf("side %s: outcomes %+v stability %s (%s)", s.Revision, s.Outcomes, s.Stability, s.Reason)
	}
	for _, o := range s.Outcomes {
		if o.State != state || o.AtOutcomeAssertion != atAssertion {
			t.Fatalf("side %s: outcome %+v, want %s at-assertion=%v", s.Revision, o, state, atAssertion)
		}
	}
}

// requireVerdict asserts the verdict when both runs qualify. Without a qualified receipt (the
// PWP attempt profile binds an exact Node and browser tuple) the verdict must fail closed to
// INCONCLUSIVE and the test skips, naming the reason.
func requireVerdict(t *testing.T, p scenarioProof, want string) {
	t.Helper()
	if !p.Head.Qualified || !p.Base.Qualified {
		if p.Verdict != VerdictInconclusive {
			t.Fatalf("unqualified run gave verdict %s", p.Verdict)
		}
		t.Skipf("receipts not qualified on this host (head %q, base %q); %s not asserted", p.Head.Reason, p.Base.Reason, want)
	}
	if p.Verdict != want {
		t.Fatalf("verdict = %s (%s), want %s", p.Verdict, p.Note, want)
	}
	for _, c := range p.Controls {
		if c.Status != "killed" {
			t.Fatalf("control %s = %s %v", c.Kind, c.Status, c.Reasons)
		}
	}
}
