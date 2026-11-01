package changee2e

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/behaviorfalsify"
	"github.com/Beamfall/corvint/internal/jstestprovider"
)

// hostedSpec is a hosted test file with a discriminating test and an F1 static-overlay
// falsifier: its assertion holds whether or not the booking happened.
const hostedSpec = `import { test, expect } from '@playwright/test';
import { ClubPage } from '../pages/club.page';
import { HomePage } from '../pages/home.page';
import { TeeSheetPage } from '../pages/teesheet.page';

test('shows the booked status', async ({ page }) => {
  const club = new ClubPage(page);
  const home = new HomePage(page);
  const sheet = new TeeSheetPage(page);
  await club.seedOpenSlots();
  await page.goto('/#!/home');
  await home.goToTeeSheets();
  await sheet.selectSlot();
  await sheet.book();
  await expect(page.getByTestId('slot-status')).toHaveText('Booked');
});

test("static overlay", async ({ page }) => {
  const club = new ClubPage(page);
  const home = new HomePage(page);
  const sheet = new TeeSheetPage(page);
  await club.seedOpenSlots();
  await page.goto('/#!/home');
  await home.goToTeeSheets();
  await sheet.selectSlot();
  await sheet.book();
  await expect(page.getByTestId('slot').first()).toBeVisible();
});
`

const hostedPath = "e2e/specs/hosted.spec.ts"

// hostedRepo commits the hosted spec at base and the behaviour change at head.
func hostedRepo(t *testing.T) (root, base, head string) {
	t.Helper()
	root, _ = fixtureRepo(t)
	base = commitFiles(t, root, map[string]*string{hostedPath: text(hostedSpec)}, "hosted test")
	return root, base, commitBehaviourChange(t, root)
}

// CCE-V0-034/035/036: a hosted test is read at head by title into its assertion lines, the
// awaited action before each assertion, and the oracle_origin join of the file at base and head.
func TestCCEV0034HostedScenario(t *testing.T) {
	root, base, head := hostedRepo(t)
	sc, err := hostedScenario(bg, root, base, head, HostedSelection{File: hostedPath, Title: "shows the booked status"})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(hostedSpec, "\n")
	if !sc.Hosted || sc.Placement != hostedPath || sc.TestTitle != "shows the booked status" || sc.DeclLine != 6 || sc.OracleOrigin != OriginAdmittedAtBase {
		t.Fatalf("scenario %+v", sc)
	}
	if !slices.Equal(sc.AssertionLines, []int{15}) || !slices.Equal(sc.EventLines, []int{14}) || sc.AssertionLine != 15 ||
		!strings.Contains(lines[14], "toHaveText('Booked')") || !strings.Contains(lines[13], "sheet.book()") {
		t.Fatalf("assertions %v events %v", sc.AssertionLines, sc.EventLines)
	}
	if err := validateScenarios([]Scenario{sc}); err != nil {
		t.Fatalf("a hosted scenario must validate without a compiled binding: %v", err)
	}
	f1, err := hostedScenario(bg, root, base, head, HostedSelection{File: hostedPath, Title: "static overlay"})
	if err != nil || f1.DeclLine != 18 || !slices.Equal(f1.AssertionLines, []int{27}) {
		t.Fatalf("double-quoted title: %+v %v", f1, err)
	}
	changed := commitFiles(t, root, map[string]*string{hostedPath: text(strings.Replace(hostedSpec, "'Booked'", "'Reserved'", 1))}, "edit oracle")
	if sc, err := hostedScenario(bg, root, base, changed, HostedSelection{File: hostedPath, Title: "shows the booked status"}); err != nil || sc.OracleOrigin != OriginChangedInDiff {
		t.Fatalf("edited test file: %s %v", sc.OracleOrigin, err)
	}
	newFile := commitFiles(t, root, map[string]*string{"e2e/specs/new.spec.ts": text("import { test, expect } from '@playwright/test';\ntest('only', async ({ page }) => {\n  await page.goto('/');\n  await expect(page).toHaveTitle('x');\n});\n")}, "new test")
	if sc, err := hostedScenario(bg, root, base, newFile, HostedSelection{File: "e2e/specs/new.spec.ts"}); err != nil || sc.OracleOrigin != OriginNewInDiff || sc.TestTitle != "only" {
		t.Fatalf("new test file without a title: %+v %v", sc, err)
	}
	noEvent := commitFiles(t, root, map[string]*string{"e2e/specs/bare.spec.ts": text("import { test, expect } from '@playwright/test';\ntest('bare', async () => {\n  expect(1).toBe(1);\n});\n")}, "bare")
	for _, tc := range []struct {
		name, head string
		sel        HostedSelection
		code       string
	}{
		{"two tests without a title", head, HostedSelection{File: hostedPath}, "e2e-hosted-test-ambiguous"},
		{"unknown title", head, HostedSelection{File: hostedPath, Title: "nope"}, "e2e-hosted-test-invalid"},
		{"not a test file", head, HostedSelection{File: "app/components/teesheet.html"}, "e2e-hosted-test-invalid"},
		{"escaping path", head, HostedSelection{File: "../x.spec.ts"}, "e2e-hosted-test-invalid"},
		{"absent at head", head, HostedSelection{File: "e2e/specs/none.spec.ts"}, "e2e-hosted-test-invalid"},
		{"assertion without an awaited action", noEvent, HostedSelection{File: "e2e/specs/bare.spec.ts"}, "e2e-hosted-test-invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var r *Refusal
			if _, err := hostedScenario(bg, root, base, tc.head, tc.sel); !errors.As(err, &r) || r.Code != tc.code {
				t.Fatalf("%v, want %s", err, tc.code)
			}
		})
	}
	var r *Refusal
	if _, err := Prove(bg, ProveOptions{Root: root, Base: base, Head: head, Hosted: &HostedSelection{File: hostedPath}, Scenarios: []Scenario{sc}}); !errors.As(err, &r) || r.Code != "e2e-hosted-test-invalid" {
		t.Fatalf("hosted with compiled scenarios: %v", err)
	}
}

// CCE-V0-036: a hosted test's controls are one omitted-event per awaited action before an
// assertion; the BBF definition selects the test by title and carries no compiled binding, and
// the hook accepts it.
func TestCCEV0036HostedControls(t *testing.T) {
	root, base, head := hostedRepo(t)
	sc, err := hostedScenario(bg, root, base, head, HostedSelection{File: hostedPath, Title: "shows the booked status"})
	if err != nil {
		t.Fatal(err)
	}
	specs := controlSpecs(sc)
	if len(specs) != 1 || specs[0].id != "control:omitted-event:step-1" || specs[0].kind != behaviorfalsify.OmittedEvent || specs[0].line != 14 {
		t.Fatalf("controls %+v", specs)
	}
	out, err := perturbLine(sc.Text, specs[0].line, specs[0].kind, "", "", "", "", specs[0].replacement)
	if err != nil || strings.Contains(string(out), "await sheet.book();\n  await expect(page.getByTestId('slot-status'))") {
		t.Fatalf("perturb: %v", err)
	}
	e := &proveEnv{root: root, runnerVersion: "1.0.0"}
	req := e.controlRequest(sc, controlSpecs(sc), head, root, t.TempDir(), jstestprovider.Receipt{})
	def := req.Controls[0].Definition
	if def["grep"] != grepTitle(sc.TestTitle) || def["file"] != hostedPath || def["line"] != "14" {
		t.Fatalf("definition %v", def)
	}
	for _, k := range []string{"locator", "matcher", "marker", "value"} {
		if _, ok := def[k]; ok {
			t.Fatalf("definition carries an empty %s", k)
		}
	}
	if req.Target.TestLine != sc.DeclLine || req.Target.TestTitle != sc.TestTitle {
		t.Fatalf("target %+v", req.Target)
	}
	argv := testArgv(runInput{Placement: hostedPath, Repeats: 3, Grep: def["grep"]})
	if argv[len(argv)-1] != "--grep="+grepTitle("shows the booked status") {
		t.Fatalf("argv %v", argv)
	}
	if got := grepTitle("a (b) c.d"); got != `a \(b\) c\.d` {
		t.Fatalf("grep %q", got)
	}
	// The hook validates an omitted-event definition without a locator, matcher or marker and
	// stops later, at the toolchain; a template control still needs them.
	t.Setenv(envNodeBin, "")
	inv := behaviorfalsify.Invocation{Control: behaviorfalsify.PlannedControl{Kind: behaviorfalsify.OmittedEvent, Definition: def}}
	if _, err := runControl(bg, inv); err == nil || err.Error() != "e2e-hook-toolchain-unavailable" {
		t.Fatalf("omitted-event hook: %v", err)
	}
	inv.Control.Kind = behaviorfalsify.WrongLocator
	if _, err := runControl(bg, inv); err == nil || err.Error() != "e2e-hook-definition-invalid" {
		t.Fatalf("wrong-locator hook without a locator: %v", err)
	}
}

// CCE-V0-035/036 (V1-1733): the hosted ladder runs offline. The F1 static-overlay falsifier passes
// on both sides and its omitted-event control survives, so it reads CONTROL_SURVIVED and does not
// score; a discriminating hosted test reads DISCRIMINATES; a base failure located off the
// assertion lines (setup) reads INCONCLUSIVE; a no-op diff reads PRESERVES only.
func TestCCEV0035HostedLadderOffline(t *testing.T) {
	root, base, head := hostedRepo(t)
	booked, err := hostedScenario(bg, root, base, head, HostedSelection{File: hostedPath, Title: "shows the booked status"})
	if err != nil {
		t.Fatal(err)
	}
	f1, err := hostedScenario(bg, root, base, head, HostedSelection{File: hostedPath, Title: "static overlay"})
	if err != nil {
		t.Fatal(err)
	}
	pass := fakeSide{states: repeat("passed")}
	mismatch := "Error: expect(locator).toHaveText(expected) failed\n\nExpected: \"Booked\"\nReceived: \"Pending\""
	for _, tc := range []struct {
		name       string
		sc         Scenario
		head, base fakeSide
		controls   string
		want, note string
		scoring    bool
	}{
		{"F1 static overlay", f1, pass, pass, "survived", VerdictControlSurvived, "control-survived:control:omitted-event:step-1", false},
		{"discriminating hosted test", booked, pass, fakeSide{states: repeat("failed"), message: mismatch, line: 15}, "killed", VerdictDiscriminates, "value mismatch", true},
		{"base setup failure", booked, pass, fakeSide{states: repeat("failed"), message: "Error: page.goto: net::ERR_CONNECTION_REFUSED", line: 11}, "killed", VerdictInconclusive, "at " + hostedPath + ":11", false},
		{"no-op diff", booked, pass, pass, "killed", VerdictPreserves, "regression-preservation", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeRunner{sides: map[string]fakeSide{head: tc.head, base: tc.base}, controls: tc.controls}
			p := f.env(root).proveScenario(bg, base, head, tc.sc, 3)
			if p.Verdict != tc.want || !strings.Contains(p.Note, tc.note) || p.Scoring != tc.scoring || !p.Hosted {
				t.Fatalf("verdict %s (%s) scoring %v hosted %v, want %s containing %q scoring %v", p.Verdict, p.Note, p.Scoring, p.Hosted, tc.want, tc.note, tc.scoring)
			}
			if f.staged[head][proveConfigName] != proveConfig || f.called != 2 {
				t.Fatalf("hosted sides must run the head test tree under the prove config (%d runs)", f.called)
			}
		})
	}
}
