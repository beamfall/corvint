package changee2e

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/behaviorfalsify"
	"github.com/Beamfall/corvint/internal/jstestprovider"
)

// fakeSide is one side's scripted pool for the offline runner (CCE-V0-028).
type fakeSide struct {
	reason  string // a non-empty reason leaves the pool unqualified
	states  []string
	message string // the failure message of each failed repeat
	// line and file are each failed repeat's receipt failure location (PWP-V3-008); file
	// defaults to the scenario's placement and a zero line leaves the location unset.
	line int
	file string
	// teardown is a run-level reporter error's PWP-V3-009 location (`file:line`, a file relative
	// to the staged tree unless absolute); the pool is then qualified by the real qualify.
	teardown string
	// globalError is the message of an unlocated run-level reporter error; the pool is then
	// qualified by the real qualify (CCE-V0-116).
	globalError string
	// runErr is the provider run error returned with a non-empty reason (CCE-V0-116).
	runErr error
	// served are the paths the side's static server answers, given to a coverage recorder.
	served []string
}

// fakeRunner scripts the pools by application revision and records each staged tree's files.
type fakeRunner struct {
	sides          map[string]fakeSide
	controls       string // the status every control reads when the ladder reaches them
	controlReasons []string
	// controlReport, when set, is the BBF report the controls read through controlResults.
	controlReport *behaviorfalsify.Report
	staged        map[string]map[string]string
	called        int
}

func (f *fakeRunner) env(root string) *proveEnv {
	e := &proveEnv{root: root}
	e.side = func(_ context.Context, in runInput) (jstestprovider.Receipt, string, string, error) {
		f.called++
		files := map[string]string{}
		for _, p := range []string{"e2e/pages/club.page.ts", "app/components/teesheet.html", proveConfigName} {
			if data, err := os.ReadFile(filepath.Join(in.Tree, filepath.FromSlash(p))); err == nil {
				files[p] = string(data)
			}
		}
		if f.staged == nil {
			f.staged = map[string]map[string]string{}
		}
		f.staged[in.AppIdentity] = files
		s := f.sides[in.AppIdentity]
		if in.Record != nil {
			for _, p := range s.served {
				in.Record(p)
			}
		}
		var r jstestprovider.Receipt
		for _, st := range s.states {
			t := jstestprovider.TestOutcome{State: jstestprovider.ExecutionState(st)}
			if st == string(jstestprovider.StateFailed) {
				t.FailureMessage = s.message
				if s.line > 0 {
					file := filepath.Join(in.Tree, filepath.FromSlash(cmpOr(s.file, in.Placement)))
					if filepath.IsAbs(s.file) {
						file = s.file
					}
					t.AttemptDetails = []jstestprovider.AttemptDetail{{State: jstestprovider.StateFailed, FailureLocation: &jstestprovider.Anchor{File: file, Line: s.line}}}
				}
			}
			r.Tests = append(r.Tests, t)
		}
		if s.teardown != "" {
			i := strings.LastIndex(s.teardown, ":")
			line, _ := strconv.Atoi(s.teardown[i+1:])
			file := s.teardown[:i]
			if !filepath.IsAbs(file) {
				file = filepath.Join(in.Tree, filepath.FromSlash(file))
			}
			r.Infrastructure = &jstestprovider.InfrastructureFailure{Reason: jstestprovider.ReasonReporterGlobalError, Detail: "Error: worker fixture teardown",
				Location: &jstestprovider.Anchor{File: file, Line: line}}
			_, reason := qualify(r, nil, len(s.states), treeRoots(in.Tree))
			return r, "", reason, nil
		}
		if s.globalError != "" {
			r.Infrastructure = &jstestprovider.InfrastructureFailure{Reason: jstestprovider.ReasonReporterGlobalError, Detail: s.globalError}
			_, reason := qualify(r, nil, len(s.states), treeRoots(in.Tree))
			return r, "", reason, nil
		}
		if s.reason != "" {
			return r, "", s.reason, s.runErr
		}
		return r, "sha256:" + strings.Repeat("0", 64), "", nil
	}
	e.controls = func(_ context.Context, _ string, sc Scenario, _ testTree, _ jstestprovider.Receipt) []controlProof {
		if f.controlReport != nil {
			return controlResults(controlSpecs(sc), "sha256:plan", *f.controlReport)
		}
		out := notRunControls(sc, "")
		for i := range out {
			out[i].Status, out[i].Reasons = f.controls, f.controlReasons
		}
		return out
	}
	return e
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func repeat(state string) []string { return []string{state, state, state} }

// assertionMessage is the Playwright failure text of the scenario's own outcome assertion.
func assertionMessage(sc Scenario, received string) string {
	return "Error: " + sc.Marker + "\n\nexpect(locator)." + sc.Matcher + "(expected) failed\n\nLocator: getByTestId('" + sc.Locator +
		"')\nExpected: \"" + sc.Value + "\"\nReceived: " + received
}

func compiledScenario(t *testing.T, root, base, head string) Scenario {
	t.Helper()
	rep, err := Compile(bg, diffOpts(root, base, head))
	if err != nil || len(rep.Compiled()) != 1 {
		t.Fatalf("compile: %v", err)
	}
	return rep.Compiled()[0]
}

// CCE-V0-009/028 (V1-1687): every ladder verdict is produced offline, through proveScenario's
// real staging, test-tree and verdict code, with only the Playwright run and the BBF controls
// replaced by a scripted runner.
func TestCCEV0028LadderOfflineFakeRunner(t *testing.T) {
	root, base := fixtureRepo(t)
	head := commitBehaviourChange(t, root)
	sc := compiledScenario(t, root, base, head)
	mismatch := assertionMessage(sc, `"Pending"`)
	unresolved := assertionMessage(sc, "<element(s) not found>")
	offAssertion := "Error: locator.click: Timeout 5000ms exceeded.\nCall log:\n  - waiting for getByTestId('slot')"
	pass := fakeSide{states: repeat("passed")}
	cleanups := sc.CleanupLines
	if len(cleanups) == 0 {
		t.Fatal("the compiled scenario has no cleanup line")
	}
	cleanup := cleanups[0]
	for _, tc := range []struct {
		name       string
		head, base fakeSide
		controls   string
		origin     string
		want       string
		note       string
		scoring    bool
		ranControl bool
	}{
		{"unqualified head", fakeSide{reason: "provider-error", states: repeat("passed")}, pass, "killed", OriginAdmittedAtBase, VerdictInconclusive, "run-without-qualified-receipt", false, false},
		{"skipped repeat", fakeSide{states: []string{"passed", "skipped", "passed"}}, pass, "killed", OriginAdmittedAtBase, VerdictInconclusive, "repeat-incomplete", false, false},
		{"intermittent head", fakeSide{states: []string{"passed", "failed", "passed"}, message: mismatch, line: sc.AssertionLine}, pass, "killed", OriginAdmittedAtBase, VerdictUnstable, "head-pool-intermittent", false, false},
		{"regression detected", fakeSide{states: repeat("failed"), message: mismatch, line: sc.AssertionLine}, pass, "killed", OriginAdmittedAtBase, VerdictRegressionDetected, "the change broke a declared outcome", false, false},
		{"scenario broken", fakeSide{states: repeat("failed"), message: offAssertion, line: sc.AssertionLine - 1}, pass, "killed", OriginAdmittedAtBase, VerdictScenarioBroken, "head-repeat-failed at " + sc.Placement + ":" + strconv.Itoa(sc.AssertionLine-1), false, false},
		{"control survived", pass, fakeSide{states: repeat("failed"), message: mismatch, line: sc.AssertionLine}, "survived", OriginAdmittedAtBase, VerdictControlSurvived, "control-survived:control:wrong-locator", false, true},
		{"control not run", pass, fakeSide{states: repeat("failed"), message: mismatch, line: sc.AssertionLine}, "not_run", OriginAdmittedAtBase, VerdictInconclusive, "control-not-killed:control:wrong-locator", false, true},
		{"discriminates", pass, fakeSide{states: repeat("failed"), message: mismatch, line: sc.AssertionLine}, "killed", OriginAdmittedAtBase, VerdictDiscriminates, "value mismatch", true, true},
		{"new reach", pass, fakeSide{states: repeat("failed"), message: unresolved, line: sc.AssertionLine}, "killed", OriginAdmittedAtBase, VerdictNewReach, "locator did not resolve", true, true},
		{"preserves", pass, pass, "killed", OriginAdmittedAtBase, VerdictPreserves, "regression-preservation", true, true},
		{"base fails off the assertion", pass, fakeSide{states: repeat("failed"), message: offAssertion, line: sc.AssertionLine - 1}, "killed", OriginAdmittedAtBase, VerdictInconclusive, "base-mixed-or-failed-off-assertion", false, true},
		{"base fails in cleanup", pass, fakeSide{states: repeat("failed"), message: "Error: cleanup", line: cleanup}, "killed", OriginAdmittedAtBase, VerdictInconclusive, FailureCleanupFailed + " at " + sc.Placement + ":" + strconv.Itoa(cleanup), false, false},
		{"head fails in cleanup", fakeSide{states: repeat("failed"), message: "Error: cleanup", line: cleanup}, pass, "killed", OriginAdmittedAtBase, VerdictInconclusive, FailureCleanupFailed + " at " + sc.Placement + ":" + strconv.Itoa(cleanup), false, false},
		{"head teardown located in the scenario", fakeSide{states: repeat("passed"), teardown: sc.Placement + ":3"}, pass, "killed", OriginAdmittedAtBase, VerdictInconclusive, FailureTeardownFailed + " at " + sc.Placement + ":3", false, false},
		{"base teardown located in a page object", pass, fakeSide{states: repeat("passed"), teardown: "e2e/pages/club.page.ts:7"}, "killed", OriginAdmittedAtBase, VerdictInconclusive, FailureTeardownFailed + " at e2e/pages/club.page.ts:7", false, false},
		{"teardown located outside the tree is infrastructure", fakeSide{states: repeat("passed"), teardown: "/elsewhere/fixtures.js:4"}, pass, "killed", OriginAdmittedAtBase, VerdictInconclusive, "run-without-qualified-receipt", false, false},
		{"teardown located in node_modules is infrastructure", fakeSide{states: repeat("passed"), teardown: "node_modules/@playwright/test/index.js:4"}, pass, "killed", OriginAdmittedAtBase, VerdictInconclusive, "run-without-qualified-receipt", false, false},
		{"assertion message without a location is off the assertion", pass, fakeSide{states: repeat("failed"), message: mismatch}, "killed", OriginAdmittedAtBase, VerdictInconclusive, "(receipt names no failure location)", false, true},
		{"assertion message located in a page object is off the assertion", pass, fakeSide{states: repeat("failed"), message: mismatch, file: "e2e/pages/club.page.ts", line: sc.AssertionLine}, "killed", OriginAdmittedAtBase, VerdictInconclusive, "at e2e/pages/club.page.ts:" + strconv.Itoa(sc.AssertionLine), false, true},
		{"changed oracle caps discriminates", pass, fakeSide{states: repeat("failed"), message: mismatch, line: sc.AssertionLine}, "killed", OriginChangedInDiff, VerdictInconclusive, ReasonOracleSelfCertified, false, true},
		{"changed oracle caps a regression", fakeSide{states: repeat("failed"), message: mismatch, line: sc.AssertionLine}, pass, "killed", OriginChangedInDiff, VerdictInconclusive, ReasonOracleSelfCertified, false, false},
		{"new oracle keeps the verdict but does not score", pass, fakeSide{states: repeat("failed"), message: unresolved, line: sc.AssertionLine}, "killed", OriginNewInDiff, VerdictNewReach, "locator did not resolve", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeRunner{sides: map[string]fakeSide{head: tc.head, base: tc.base}, controls: tc.controls}
			s := sc
			s.OracleOrigin = tc.origin
			p := f.env(root).proveScenario(bg, base, head, s, 3)
			if p.Verdict != tc.want || !strings.Contains(p.Note, tc.note) || p.Scoring != tc.scoring || p.OracleOrigin != tc.origin {
				t.Fatalf("verdict %s (%s) scoring %v origin %s, want %s containing %q scoring %v", p.Verdict, p.Note, p.Scoring, p.OracleOrigin, tc.want, tc.note, tc.scoring)
			}
			ran := p.Controls[0].Status != string(behaviorfalsify.StatusNotRun) || tc.controls == "not_run" && p.Controls[0].Reasons == nil
			if ran != tc.ranControl {
				t.Fatalf("controls ran = %v, want %v: %+v", ran, tc.ranControl, p.Controls)
			}
			if p.Head.ApplicationRevision != head || p.Base.ApplicationRevision != base || p.Head.TestRevision != head || p.Base.TestRevision != head {
				t.Fatalf("revisions head %+v base %+v", p.Head, p.Base)
			}
			if !strings.HasPrefix(p.Head.TestTreeDigest, "sha256:") || p.Head.TestTreeDigest != p.Base.TestTreeDigest {
				t.Fatalf("test-tree digests %q and %q must be identical", p.Head.TestTreeDigest, p.Base.TestTreeDigest)
			}
		})
	}
}

// CCE-V0-017: both sides run the head test tree; the base side runs it over the base application.
// A page-object edit in the change therefore reaches the base run, and the recorded digest is the
// same on both sides.
func TestCCEV0017TestTreeHeldAtHead(t *testing.T) {
	root, base := fixtureRepo(t)
	page := strings.Replace(*overlayText(t, "e2e/pages/club.page.ts"), "async cleanup() {", "// head page object\n  async cleanup() {", 1)
	commitFiles(t, root, map[string]*string{"e2e/pages/club.page.ts": text(page)}, "page object drift")
	head := commitBehaviourChange(t, root)
	sc := compiledScenario(t, root, base, head)
	f := &fakeRunner{sides: map[string]fakeSide{head: {states: repeat("passed")}, base: {states: repeat("passed")}}, controls: "killed"}
	p := f.env(root).proveScenario(bg, base, head, sc, 3)
	if f.called != 2 {
		t.Fatalf("runner called %d times", f.called)
	}
	for _, rev := range []string{head, base} {
		if got := f.staged[rev]["e2e/pages/club.page.ts"]; got != page {
			t.Fatalf("side %s ran page object %q, want the head page object", rev, got)
		}
		if f.staged[rev][proveConfigName] != proveConfig {
			t.Fatalf("side %s lacks the generated config", rev)
		}
	}
	if strings.Contains(f.staged[base]["app/components/teesheet.html"], "Booked") || f.staged[base]["app/components/teesheet.html"] == f.staged[head]["app/components/teesheet.html"] {
		t.Fatal("the base side must run the base application tree")
	}
	if p.Head.TestTreeDigest == "" || p.Head.TestTreeDigest != p.Base.TestTreeDigest {
		t.Fatalf("digests %q / %q", p.Head.TestTreeDigest, p.Base.TestTreeDigest)
	}
}

// CCE-V0-017: an import that resolves nowhere in the test tree, and a test-tree file that
// application code imports, each read INCONCLUSIVE before any run.
func TestCCEV0017TestTreeRefusals(t *testing.T) {
	root, base := fixtureRepo(t)
	head := commitBehaviourChange(t, root)
	sc := compiledScenario(t, root, base, head)
	missing := sc
	missing.Text = []byte(strings.Replace(string(sc.Text), "import { test as base, expect }", "import { Missing } from './missing-helper';\nimport { test as base, expect }", 1))
	overlapRoot, overlapBase := fixtureRepo(t)
	commitFiles(t, overlapRoot, map[string]*string{"app/main.ts": text("import { ClubPage } from '../e2e/pages/club.page';\nexport const page = ClubPage;\n")}, "app imports a page object")
	overlapHead := commitBehaviourChange(t, overlapRoot)
	overlap := compiledScenario(t, overlapRoot, overlapBase, overlapHead)
	for _, tc := range []struct {
		name             string
		root, base, head string
		sc               Scenario
		want             string
	}{
		{"unresolved import", root, base, head, missing, ReasonTestTreeUnresolved},
		{"application imports the test tree", overlapRoot, overlapBase, overlapHead, overlap, ReasonTestTreeOverlaps},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeRunner{controls: "killed"}
			p := f.env(tc.root).proveScenario(bg, tc.base, tc.head, tc.sc, 3)
			if p.Verdict != VerdictInconclusive || p.Note != tc.want || p.Head.Reason != tc.want || p.Base.Reason != tc.want || f.called != 0 {
				t.Fatalf("verdict %s (%s), head reason %q, runner calls %d; want INCONCLUSIVE %s", p.Verdict, p.Note, p.Head.Reason, f.called, tc.want)
			}
		})
	}
}

// CCE-V0-018: omitted-event controls cover the step expected to produce the outcome and every
// fixture setup, and the perturbation comments out exactly that statement.
func TestCCEV0018OmittedEventControls(t *testing.T) {
	root, base := fixtureRepo(t)
	head := commitBehaviourChange(t, root)
	sc := compiledScenario(t, root, base, head)
	lines := strings.Split(string(sc.Text), "\n")
	specs := controlSpecs(sc)
	var ids []string
	for _, c := range specs {
		ids = append(ids, c.id)
	}
	if strings.Join(ids, ",") != "control:wrong-locator,control:wrong-expected-value,control:omitted-event:step,control:omitted-event:setup-1" {
		t.Fatalf("control IDs %v", ids)
	}
	for _, c := range specs[2:] {
		out, err := perturbLine(sc.Text, c.line, c.kind, sc.LocatorForm, sc.Locator, sc.Matcher, sc.Value, c.replacement)
		if err != nil {
			t.Fatalf("%s: %v", c.id, err)
		}
		got := strings.Split(string(out), "\n")
		if len(got) != len(lines) {
			t.Fatalf("%s changed the line count", c.id)
		}
		for i := range lines {
			if (got[i] != lines[i]) != (i == c.line-1) {
				t.Fatalf("%s changed line %d, want only %d", c.id, i+1, c.line)
			}
		}
		if !strings.Contains(got[c.line-1], "// corvint-test e2e prove: omitted-event control") {
			t.Fatalf("%s line = %q", c.id, got[c.line-1])
		}
	}
	if !strings.Contains(lines[specs[3].line-1], "seedOpenSlots()") || !strings.HasPrefix(lines[specs[2].line-1], "  await test.step(") {
		t.Fatalf("setup line %q, step line %q", lines[specs[3].line-1], lines[specs[2].line-1])
	}
	if _, err := perturbLine(sc.Text, sc.AssertionLine, behaviorfalsify.OmittedEvent, sc.LocatorForm, sc.Locator, sc.Matcher, sc.Value, omittedEvent); err == nil {
		t.Fatal("omitting the outcome assertion itself must refuse")
	}
}

// CCE-V0-026: the failure kind of a repeat at the outcome assertion is read from the message.
func TestCCEV0026FailureKind(t *testing.T) {
	sc := Scenario{Placement: "e2e/a.spec.ts", AssertionLine: 4, Marker: "corvint-e2e outcome assert:a/b", Matcher: "toHaveText", Locator: "slot-status", Value: "Booked"}
	for _, tc := range []struct {
		name, state, message string
		line                 int
		want                 string
	}{
		{"value mismatch", "failed", assertionMessage(sc, `"Pending"`), 4, FailureValueMismatch},
		{"locator unresolved", "failed", assertionMessage(sc, "<element(s) not found>"), 4, FailureLocatorUnresolved},
		{"coloured locator unresolved", "failed", assertionMessage(sc, "\x1b[31m<element(s) not found>\x1b[39m"), 4, FailureLocatorUnresolved},
		{"off the assertion", "failed", "Error: locator.click: element(s) not found", 3, ""},
		{"passed", "passed", "", 4, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := assertionFailure(tc.state, tc.message, sc.Placement, tc.line, sc); got != tc.want {
				t.Fatalf("failure kind %q, want %q", got, tc.want)
			}
		})
	}
}
