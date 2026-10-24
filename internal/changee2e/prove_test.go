package changee2e

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/behaviorfalsify"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/playwrightruns"
)

// TestMain lets the test binary act as the BBF control hook in the live proof tests: BBF runs
// the hook with no arguments and CORVINT_BEHAVIOR_COMMAND_ROLE set, as the companion does.
func TestMain(m *testing.M) {
	if len(os.Args) == 2 && os.Args[1] == testServeArg {
		testServe()
		return
	}
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
		TestTitle: "obligation:a/b", AssertionID: "assert:a/b", AssertionLine: 1, Matcher: "toHaveText", Locator: "s", Value: "v", Marker: "corvint-e2e outcome assert:a/b"}
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

// pool is a qualified pool of n repeats; at places each failure on the outcome assertion with a
// value mismatch.
func pool(state string, n int, at bool) sideProof {
	kind := ""
	if at {
		kind = FailureValueMismatch
	}
	return poolKind(state, n, kind)
}

func poolKind(state string, n int, kind string) sideProof {
	s := sideProof{Qualified: true}
	for range n {
		s.Outcomes = append(s.Outcomes, repeatOutcome{State: state, AtOutcomeAssertion: kind != "", FailureKind: kind})
	}
	s.Stability = stability(s.Outcomes)
	return s
}

// controls reads the two template controls with the given statuses and the omitted-event
// controls killed.
func controls(locator, value string) []controlProof {
	return []controlProof{{ID: "control:wrong-locator", Kind: string(behaviorfalsify.WrongLocator), Status: locator},
		{ID: "control:wrong-expected-value", Kind: string(behaviorfalsify.WrongExpectedValue), Status: value},
		{ID: "control:omitted-event:step", Kind: string(behaviorfalsify.OmittedEvent), Status: "killed"},
		{ID: "control:omitted-event:setup-1", Kind: string(behaviorfalsify.OmittedEvent), Status: "killed"}}
}

// CCE-V0-009: the verdict is the first rung that holds, in the amended ten-rung order.
func TestCCEV0009VerdictOrder(t *testing.T) {
	passHead := pool("passed", 3, false)
	failBase := pool("failed", 3, true)
	unresolvedBase := poolKind("failed", 3, FailureLocatorUnresolved)
	mixedKinds := poolKind("failed", 2, FailureValueMismatch)
	mixedKinds.Outcomes = append(mixedKinds.Outcomes, repeatOutcome{State: "failed", AtOutcomeAssertion: true, FailureKind: FailureLocatorUnresolved})
	killed := controls("killed", "killed")
	setupSurvived := controls("killed", "killed")
	setupSurvived[3].Status = "survived"
	skipped := pool("passed", 2, false)
	skipped.Outcomes = append(skipped.Outcomes, repeatOutcome{State: "skipped"})
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
		{"1 empty base pool", passHead, sideProof{Qualified: true}, killed, VerdictInconclusive},
		{"1 skipped head repeat beats intermittency", skipped, failBase, killed, VerdictInconclusive},
		{"1 interrupted base repeat", passHead, pool("interrupted", 3, false), killed, VerdictInconclusive},
		{"2 intermittent head beats a head failure", mixed, failBase, killed, VerdictUnstable},
		{"2 flaky repeat is intermittent", pool("flaky", 3, false), failBase, killed, VerdictUnstable},
		{"3 head fails at the assertion and base passes", pool("failed", 3, true), pool("passed", 3, false), killed, VerdictRegressionDetected},
		{"3 regression beats survived controls", pool("failed", 3, true), pool("passed", 3, false), controls("survived", "survived"), VerdictRegressionDetected},
		{"4 head fails at the assertion but base fails too", pool("failed", 3, true), failBase, killed, VerdictScenarioBroken},
		{"4 head fails off the assertion", pool("failed", 3, false), pool("passed", 3, false), killed, VerdictScenarioBroken},
		{"4 head timeout", pool("timedOut", 3, false), failBase, killed, VerdictScenarioBroken},
		{"5 wrong-locator survived", passHead, failBase, controls("survived", "killed"), VerdictControlSurvived},
		{"5 wrong-value survived beats a not-run control", passHead, failBase, controls("not_run", "survived"), VerdictControlSurvived},
		{"5 omitted fixture setup survived", passHead, failBase, setupSurvived, VerdictControlSurvived},
		{"6 control invalid", passHead, failBase, controls("invalid_control", "killed"), VerdictInconclusive},
		{"6 control infrastructure", passHead, failBase, controls("killed", "infrastructure_failed"), VerdictInconclusive},
		{"6 control missing", passHead, failBase, nil, VerdictInconclusive},
		{"7 every base repeat fails at the assertion with a value mismatch", passHead, failBase, killed, VerdictDiscriminates},
		{"8 every base repeat fails at the assertion with the locator unresolved", passHead, unresolvedBase, killed, VerdictNewReach},
		{"9 every base repeat passes", passHead, pool("passed", 3, false), killed, VerdictPreserves},
		{"10 mixed base", passHead, mixedBase, killed, VerdictInconclusive},
		{"10 base fails off the assertion", passHead, offAssertion, killed, VerdictInconclusive},
		{"10 mixed failure kinds at the assertion", passHead, mixedKinds, killed, VerdictInconclusive},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, note := verdict(tc.head, tc.base, tc.controls)
			if got != tc.want {
				t.Fatalf("verdict = %s (%s), want %s", got, note, tc.want)
			}
			if got == VerdictPreserves && !strings.Contains(note, "regression-preservation") {
				t.Fatalf("PRESERVES note %q must say regression preservation only", note)
			}
			if got == VerdictNewReach && !strings.Contains(note, "new behaviour against its absence") {
				t.Fatalf("NEW_REACH note %q must say new behaviour against its absence", note)
			}
		})
	}
}

// CCE-V0-009 step 6 (V1-2373, V1-2374): a failure counts at the outcome assertion only when the
// receipt's PWP-V3-008 failure location is the scenario's file at the assertion line. A failure in
// the test's own cleanup is cleanup-failed; a page-object frame, a file outside the test tree, or a
// receipt with no location is off the assertion whatever the message says.
func TestCCEV0009AssertionDetector(t *testing.T) {
	tree := t.TempDir()
	text := "import { test, expect } from '@playwright/test';\ntest('t', async ({ page }) => {\n  try {\n    await page.click('#b');\n    await expect(page.getByTestId('s'), 'm').toHaveText('Booked');\n  } finally {\n    await page.close();\n  }\n});\n"
	sc := Scenario{Placement: "e2e/corvint-compiled/a.spec.ts", Text: []byte(text), AssertionLine: 5}
	at := func(file string, line int) jstestprovider.TestOutcome {
		return jstestprovider.TestOutcome{State: jstestprovider.StateFailed, AttemptDetails: []jstestprovider.AttemptDetail{
			{State: jstestprovider.StateFailed, FailureLocation: &jstestprovider.Anchor{File: file, Line: line}}}}
	}
	in := filepath.Join(tree, "e2e", "corvint-compiled", "a.spec.ts")
	for _, tc := range []struct {
		name     string
		outcome  jstestprovider.TestOutcome
		want     string
		location string
	}{
		{"at the assertion", at(in, 5), FailureValueMismatch, sc.Placement + ":5"},
		{"at the event before it", at(in, 4), "", sc.Placement + ":4"},
		{"in the test's own cleanup", at(in, 7), FailureCleanupFailed, sc.Placement + ":7"},
		{"in a page object at the same line number", at(filepath.Join(tree, "e2e", "pages", "club.page.ts"), 5), "", "e2e/pages/club.page.ts:5"},
		{"outside the test tree", at("/elsewhere/a.spec.ts", 5), "", outsideTestTree},
		{"no location", jstestprovider.TestOutcome{State: jstestprovider.StateFailed}, "", ""},
		{"timed out at the assertion", func() jstestprovider.TestOutcome { o := at(in, 5); o.State = jstestprovider.StateTimedOut; return o }(), "", sc.Placement + ":5"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file, line := receiptLocation(treeRoots(tree), tc.outcome)
			if got := assertionFailure(string(tc.outcome.State), "Error: m", file, line, sc); got != tc.want {
				t.Fatalf("assertionFailure = %q, want %q", got, tc.want)
			}
			if got := locationText(file, line); got != tc.location {
				t.Fatalf("location %q, want %q", got, tc.location)
			}
		})
	}
	if got := cleanupLines([]byte(text)); len(got) != 1 || got[0] != 7 {
		t.Fatalf("cleanup lines %v, want [7]", got)
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
		{behaviorfalsify.WrongLocator, "getByTestId('corvint-wrong-locator'), 'corvint-e2e outcome assert:book-tee-time/booked').toHaveText('Booked')"},
		{behaviorfalsify.WrongExpectedValue, "getByTestId('slot-status'), 'corvint-e2e outcome assert:book-tee-time/booked').toHaveText('corvint-wrong-value')"},
	} {
		repl := wrongLocator
		if tc.kind == behaviorfalsify.WrongExpectedValue {
			repl = wrongValue
		}
		out, err := perturbLine(sc.Text, sc.AssertionLine, tc.kind, sc.LocatorForm, sc.Locator, sc.Matcher, sc.Value, repl)
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
	if _, err := perturbLine(sc.Text, sc.AssertionLine-1, behaviorfalsify.WrongLocator, sc.LocatorForm, sc.Locator, sc.Matcher, sc.Value, wrongLocator); err == nil {
		t.Fatal("a line without the assertion must refuse the perturbation")
	}
}

// liveProve runs Prove for base..head on the fixture, or skips without a local Playwright.
func liveProve(t *testing.T, root, base, head string) scenarioProof {
	t.Helper()
	p, doc := liveProveDoc(t, root, base, head)
	if doc.Environment != environmentStatic {
		t.Fatalf("environment %q, want static", doc.Environment)
	}
	return p
}

// liveToolchain is the configured Playwright toolchain; the live tests skip without one.
func liveToolchain(t *testing.T) (modules, node string) {
	t.Helper()
	modules = os.Getenv(envModules)
	if modules == "" {
		liveUnavailable(t, "CORVINT_PLAYWRIGHT_MODULES is not set")
	}
	node = os.Getenv("CORVINT_PLAYWRIGHT_NODE")
	if node == "" {
		var err error
		if node, err = exec.LookPath("node"); err != nil {
			liveUnavailable(t, "node is not available")
		}
	}
	return modules, node
}

// envLive set to `required` makes a missing live toolchain fail the live tests instead of
// skipping them, so a lane that is meant to run them cannot pass vacuously (CCE-V0-102).
const envLive = "CORVINT_CCE_LIVE"

func liveUnavailable(t testing.TB, reason string) {
	t.Helper()
	if os.Getenv(envLive) == "required" {
		t.Fatalf("%s=required: %s", envLive, reason)
	}
	t.Skip(reason)
}

func liveProveDoc(t *testing.T, root, base, head string) (scenarioProof, proveReport) {
	t.Helper()
	modules, _ := liveToolchain(t)
	rep, err := Compile(bg, diffOpts(root, base, head))
	if err != nil || len(rep.Compiled()) != 1 {
		t.Fatalf("compile: %v", err)
	}
	return liveProveWith(t, root, modules, ProveOptions{Root: root, Base: base, Head: head, Scenarios: rep.Compiled()})
}

// liveProveWith runs Prove with the live toolchain and this test binary as the hook.
func liveProveWith(t *testing.T, root, _ string, o ProveOptions) (scenarioProof, proveReport) {
	t.Helper()
	modules, node := liveToolchain(t)
	hook, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	o.Repeats, o.NodeModules, o.NodeBin, o.HookExecutable = DefaultRepeats, modules, node, hook
	out, err := Prove(bg, o)
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
	return doc.Scenarios[0], doc
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
	var ids []string
	for _, c := range p.Controls {
		ids = append(ids, c.ID)
	}
	if strings.Join(ids, ",") != "control:wrong-locator,control:wrong-expected-value,control:omitted-event:step,control:omitted-event:setup-1" {
		t.Fatalf("controls = %+v, want the template and omitted-event controls", p.Controls)
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

// CCE-V0-034..036 live (V1-1733): an existing hosted test, selected by file and title, runs as
// given at head and base with the test tree held at head. The discriminating test fails at base on
// its own assertion and its omitted-event control is killed; the F1 static-overlay falsifier passes
// on both sides and its omitted-event control survives, so it reads CONTROL_SURVIVED.
func TestCCEV0034LiveHosted(t *testing.T) {
	liveToolchain(t)
	root, base, head := hostedRepo(t)
	p, _ := liveProveWith(t, root, "", ProveOptions{Root: root, Base: base, Head: head, Hosted: &HostedSelection{File: hostedPath, Title: "shows the booked status"}})
	requireOutcomes(t, p.Head, "passed", false)
	requireOutcomes(t, p.Base, "failed", true)
	requireVerdict(t, p, VerdictDiscriminates)
	for _, o := range p.Base.Outcomes {
		if o.FailureLocation != hostedPath+":15" {
			t.Fatalf("base failure location %q, want the receipt's %s:15", o.FailureLocation, hostedPath)
		}
	}
	f1, _ := liveProveWith(t, root, "", ProveOptions{Root: root, Base: base, Head: head, Hosted: &HostedSelection{File: hostedPath, Title: "static overlay"}})
	requireOutcomes(t, f1.Head, "passed", false)
	requireOutcomes(t, f1.Base, "passed", false)
	if f1.Verdict != VerdictControlSurvived || f1.Scoring || len(f1.Controls) != 1 || f1.Controls[0].Status != "survived" {
		t.Fatalf("F1 static overlay: %s (%s) %+v", f1.Verdict, f1.Note, f1.Controls)
	}
}

// requireOutcomes checks the observed browser outcomes, which Prove keeps even when the receipt
// does not qualify.
func requireOutcomes(t *testing.T, s sideProof, state string, atAssertion bool) {
	t.Helper()
	if len(s.Outcomes) != DefaultRepeats || s.Stability != playwrightruns.Deterministic {
		t.Fatalf("side %s: outcomes %+v stability %s (%s)", s.ApplicationRevision, s.Outcomes, s.Stability, s.Reason)
	}
	for _, o := range s.Outcomes {
		if o.State != state || o.AtOutcomeAssertion != atAssertion {
			t.Fatalf("side %s: outcome %+v, want %s at-assertion=%v", s.ApplicationRevision, o, state, atAssertion)
		}
	}
}

// requireVerdict asserts the verdict. The live tests skip only when the Playwright toolchain is
// not configured; once it is, an unqualified receipt (the PWP attempt profile binds an exact Node
// and browser tuple) fails the test rather than leaving the verdict unproved.
func requireVerdict(t testing.TB, p scenarioProof, want string) {
	t.Helper()
	if !p.Head.Qualified || !p.Base.Qualified {
		t.Fatalf("receipts not qualified (head %q, base %q, verdict %s); %s not proved%s", p.Head.Reason, p.Base.Reason, p.Verdict, want, proofLosses(p))
	}
	if p.Verdict != want {
		t.Fatalf("verdict = %s (%s), want %s%s", p.Verdict, p.Note, want, proofLosses(p))
	}
	for _, c := range p.Controls {
		if c.Status != "killed" {
			t.Fatalf("control %s = %s %v%s", c.Kind, c.Status, c.Reasons, proofLosses(p))
		}
	}
}

// proofLosses renders each unqualified side's and each lost control attempt's CCE-V0-116 loss,
// so a failed live verdict is diagnosable from the test log alone (V1-2620).
func proofLosses(p scenarioProof) string {
	var b strings.Builder
	write := func(who string, l runLoss) {
		fmt.Fprintf(&b, "\n  %s: reason=%q attempt_reasons=%q error_location=%q error_message=%q hook_process=%q",
			who, l.Reason, l.AttemptReasons, l.ErrorLocation, l.ErrorMessage, l.HookProcess)
	}
	if p.Head.Loss != nil {
		write("head side", *p.Head.Loss)
	}
	if p.Base.Loss != nil {
		write("base side", *p.Base.Loss)
	}
	for _, c := range p.Controls {
		for _, l := range c.LostAttempts {
			write(fmt.Sprintf("%s attempt %d", c.ID, l.Attempt), l)
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return "\nlost runs (CCE-V0-116):" + b.String()
}

// CCE-V0-008: the disposable tree reproduces no tracked link, so a chained link cannot carry later
// scenario writes outside it.
func TestCCEV0008UntarSkipsLinks(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, h := range []*tar.Header{
		{Name: "a", Typeflag: tar.TypeSymlink, Linkname: "."},
		{Name: "e2e", Typeflag: tar.TypeSymlink, Linkname: "a/.."},
		{Name: "index.html", Typeflag: tar.TypeReg, Mode: 0o644, Size: 2},
	} {
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte("ok")); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "tree")
	if err := untar(buf.Bytes(), dest); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "e2e"} {
		if _, err := os.Lstat(filepath.Join(dest, name)); !os.IsNotExist(err) {
			t.Fatalf("%s reproduced: %v", name, err)
		}
	}
	if data, err := os.ReadFile(filepath.Join(dest, "index.html")); err != nil || string(data) != "ok" {
		t.Fatalf("index.html = %q, %v", data, err)
	}
}

// CCE-V0-010: the proof and shadow documents carry no field that could narrow, skip, replace or
// retire a hosted test's selection; scoring is the only above-candidate signal a proof carries.
func TestCCEV0010NoSelectionFields(t *testing.T) {
	tags := func(v any) []string {
		var out []string
		rt := reflect.TypeOf(v)
		for i := range rt.NumField() {
			if tag := strings.Split(rt.Field(i).Tag.Get("json"), ",")[0]; tag != "" && tag != "-" {
				out = append(out, tag)
			}
		}
		return out
	}
	for _, v := range []any{proveReport{}, scenarioProof{}, sideProof{}, controlProof{}, shadowReport{}, entryReport{}, candidateReport{}} {
		for _, tag := range tags(v) {
			for _, word := range []string{"skip", "select", "narrow", "replace", "retire", "disable", "delete", "exclude"} {
				if strings.Contains(tag, word) {
					t.Fatalf("%T carries a selection field %q", v, tag)
				}
			}
		}
	}
}
