package changee2e

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/behaviorfalsify"
)

var writeGolden = flag.Bool("cce-write-golden", false, "rewrite the CCE golden compile outputs (CCE-V0-113)")

// vocabularyDeclarations extends the overlay declarations with every CCE-V0-105..112 member: a
// fixture output, a session, a typed locator and a route bound to the fixture output.
const vocabularyDeclarations = `{
  "schema": "e2e-block-declarations/0",
  "inputs": [
    {"input_id": "input:slot-note", "screen_id": "screen:admin:app.clubs.teesheets", "locator": "slot-note",
     "values": [{"value_id": "morning", "value": "Morning", "class": "valid"}, {"value_id": "blank", "value": "", "class": "invalid"}]}
  ],
  "fixtures": [
    {"fixture_id": "fixture:open-slots", "satisfies": ["club has open slots"], "outputs": ["clubId"],
     "setup": {"page_object": "file:e2e/pages/club.page.ts", "method": "seedOpenSlots"},
     "cleanup": {"page_object": "file:e2e/pages/club.page.ts", "method": "cleanup"}}
  ],
  "sessions": [
    {"session_id": "session:club-admin", "satisfies": ["signed in as club admin"], "storage_state_env": "CLUB_ADMIN_STATE"}
  ],
  "locators": [
    {"block": "outcome:rebook-slot/rebooked", "by": "role", "role": "status", "name": "Slot status"}
  ],
  "routes": [
    {"screen_id": "screen:admin:app.clubs.teesheets", "params": [{"name": "clubId", "fixture": "fixture:open-slots", "output": "clubId"}]}
  ]
}
`

// countFlow is a one-step flow on the tee sheet whose outcome is a positive element count.
const countFlow = `{"schema":"application-flow-intent/1","flow_id":"count-slots","revision":1,"kind":"ui","actor":"club admin",
"preconditions":["club has open slots"],
"steps":[{"step_id":"select-slot","action":"select an open slot"}],
"outcomes":[{"outcome_id":"three-slots","behavior":"three slots show","matcher":"toHaveCount","locator":"slot","value":"3"}],
"variations":[],"links":[],
"navigation":{"precondition_flows":[],"steps":[
{"step_id":"select-slot","state":"/clubs/{clubId}/teesheets","locator":{"test_id":"slot"},"expect":["three-slots"]}]}}
`

const vocabularyClubPage = `import { Page } from '@playwright/test';

export class ClubPage {
  constructor(private readonly page: Page) {}

  async seedOpenSlots() {
    await this.page.addInitScript(() => localStorage.setItem('open-slots', '1'));
    return { clubId: '1' };
  }

  async cleanup() {
    await this.page.evaluate(() => localStorage.removeItem('open-slots'));
  }
}
`

// unguardedRoutes is the overlay router with app.clubs unguarded, so the tee sheet's entry is
// blocked only by its clubId parameter.
func unguardedRoutes(t *testing.T) *string {
	t.Helper()
	s := *overlayText(t, "app/routes.js")
	out := strings.Replace(s, "      url: 'clubs/:clubId',\n      data: { permissions: ['club.view'] }\n", "      url: 'clubs/:clubId'\n", 1)
	if out == s {
		t.Fatal("the overlay router changed shape")
	}
	return &out
}

// vocabularyRepo commits the full-vocabulary declarations and flows at base, then the one-screen
// behaviour change at head.
func vocabularyRepo(t *testing.T, edit map[string]*string) (root, base, head string) {
	t.Helper()
	root, _ = fixtureRepo(t)
	files := map[string]*string{
		"app/routes.js":          unguardedRoutes(t),
		DeclarationsPath:         text(vocabularyDeclarations),
		"e2e/pages/club.page.ts": text(vocabularyClubPage),
		"flows/rebook-slot.json": text(rebookFlow("rebook-slot", `"club has open slots","signed in as club admin"`)),
		"flows/count-slots.json": text(countFlow),
	}
	for p, v := range edit {
		files[p] = v
	}
	base = commitFiles(t, root, files, "vocabulary")
	return root, base, commitBehaviourChange(t, root)
}

// declWith is vocabularyDeclarations with edit applied to its decoded members.
func declWith(t *testing.T, edit func(d map[string]any)) *string {
	t.Helper()
	var d map[string]any
	if err := json.Unmarshal([]byte(vocabularyDeclarations), &d); err != nil {
		t.Fatal(err)
	}
	edit(d)
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	out := string(raw) + "\n"
	return &out
}

func declMember(d map[string]any, name string, i int) map[string]any {
	return d[name].([]any)[i].(map[string]any)
}

// vocabularyCompile compiles the vocabulary repository with edit applied at base.
func vocabularyCompile(t *testing.T, edit map[string]*string) *CompileReport {
	t.Helper()
	root, base, head := vocabularyRepo(t, edit)
	r, err := Compile(bg, diffOpts(root, base, head))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return r
}

func compiledFor(t *testing.T, r *CompileReport, obligation string) Scenario {
	t.Helper()
	for _, sc := range r.Compiled() {
		if sc.Obligation == obligation {
			return sc
		}
	}
	t.Fatalf("%s did not compile:\n%s", obligation, r.Encoded())
	return Scenario{}
}

// missingReason is the reason of the entry's one missing block of kind and id, or "".
func missingReason(r *CompileReport, obligation, kind, id string) string {
	if e := entryFor(r, obligation); e != nil && e.State == Incomplete {
		for _, m := range e.Missing {
			if m.Kind == kind && m.ID == id {
				return m.Reason
			}
		}
	}
	return ""
}

// catalogRefusal is the refusal code of a catalog built over declarations decl.
func catalogRefusal(t *testing.T, decl *string) string {
	t.Helper()
	root, _ := fixtureRepo(t)
	rev := commitFiles(t, root, map[string]*string{"app/routes.js": unguardedRoutes(t), DeclarationsPath: decl,
		"e2e/pages/club.page.ts": text(vocabularyClubPage)}, "declarations")
	_, err := BuildCatalog(bg, CatalogOptions{Root: root, Rev: rev})
	return refusalCode(err)
}

func catalogUnadmitted(t *testing.T, decl *string, kind, id string) string {
	t.Helper()
	root, _ := fixtureRepo(t)
	rev := commitFiles(t, root, map[string]*string{"app/routes.js": unguardedRoutes(t), DeclarationsPath: decl,
		"e2e/pages/club.page.ts": text(vocabularyClubPage), "flows/rebook-slot.json": text(rebookFlow("rebook-slot", `"club has open slots"`))}, "declarations")
	cat, err := BuildCatalog(bg, CatalogOptions{Root: root, Rev: rev})
	if err != nil {
		t.Fatal(err)
	}
	return cat.unadmittedReason(kind, id, "")
}

const (
	obRebook = "obligation:rebook-slot/rebooked"
	obCount  = "obligation:count-slots/three-slots"
	obBooked = "obligation:book-tee-time/booked"
	teeEntry = "path:entry:screen:admin:app.clubs.teesheets"
)

// CCE-V0-105: a fixture declares the outputs its setup returns; the compiled fixture checks each
// is present before the test body runs and hands them to later blocks. An output name that is not
// a lower-camel identifier, a repeated name, or more than eight outputs refuse the declarations.
func TestCCEV0105FixtureOutputs(t *testing.T) {
	sc := compiledFor(t, vocabularyCompile(t, nil), obCount)
	src := string(sc.Text)
	for _, w := range []string{
		"const out = await clubPage.seedOpenSlots();",
		"for (const k of ['clubId']) if (out?.[k] === undefined) throw new Error('corvint-e2e fixture-output-missing ' + k);",
		"await use(out);",
		"async ({ page, po, fx1 }) =>",
		// One page-object instance per class is shared by setup, actions and cleanup.
		"await use({ clubPage: new ClubPage(page), teeSheetPage: new TeeSheetPage(page) });",
		"const { clubPage } = po;",
		"const { teeSheetPage } = po;",
	} {
		if !strings.Contains(src, w) {
			t.Fatalf("missing %q in:\n%s", w, src)
		}
	}
	if strings.Contains(src, "= new ") {
		t.Fatalf("a page object is constructed outside the shared po fixture:\n%s", src)
	}
	nine := []any{}
	for i := range 9 {
		nine = append(nine, "o"+strconv.Itoa(i))
	}
	for name, outs := range map[string][]any{"upper-case start": {"ClubId"}, "repeated": {"clubId", "clubId"}, "nine outputs": nine, "empty": {""}} {
		t.Run(name, func(t *testing.T) {
			decl := declWith(t, func(d map[string]any) { declMember(d, "fixtures", 0)["outputs"] = outs })
			if code := catalogRefusal(t, decl); code != "e2e-blocks-invalid" {
				t.Fatalf("refusal %q, want e2e-blocks-invalid", code)
			}
		})
	}
}

// CCE-V0-106: a route parameter bound to a fixture output reads that fixture's value in the entry
// URL; a scenario whose flow does not set the fixture's precondition misses the path with
// route-param-unbound, and a route naming an unadmitted fixture or an undeclared output is
// withheld from the catalog.
func TestCCEV0106OutputDataFlow(t *testing.T) {
	r := vocabularyCompile(t, map[string]*string{"flows/rebook-bare.json": text(rebookFlow("rebook-bare", `"signed in as club admin"`))})
	sc := compiledFor(t, r, obCount)
	if !strings.Contains(string(sc.Text), "page.goto('/#!/clubs/' + encodeURIComponent(String(fx1.clubId)) + '/teesheets')") {
		t.Fatalf("entry URL does not read the fixture output:\n%s", sc.Text)
	}
	if got := missingReason(r, "obligation:rebook-bare/rebooked", KindPath, teeEntry); got != "route-param-unbound:clubId" {
		t.Fatalf("rebook-bare path reason %q\n%s", got, r.Encoded())
	}
	for name, tc := range map[string]struct{ fixture, output, want string }{
		"unadmitted fixture": {"fixture:absent", "clubId", "route-fixture-unadmitted"},
		"undeclared output":  {"fixture:open-slots", "courseId", "route-output-undeclared"},
	} {
		t.Run(name, func(t *testing.T) {
			decl := declWith(t, func(d map[string]any) {
				declMember(d, "routes", 0)["params"] = []any{map[string]any{"name": "clubId", "fixture": tc.fixture, "output": tc.output}}
			})
			if got := catalogUnadmitted(t, decl, KindPath, teeEntry); got != tc.want {
				t.Fatalf("unadmitted reason %q, want %q", got, tc.want)
			}
		})
	}
}

// CCE-V0-107: a declared session satisfies a precondition no fixture does; the compiled scenario
// runs under its storage state and skips when the variable is unset; two sessions in one scenario
// miss with session-conflict; the variable reaches the run only when set; and a variable name
// that is not an upper-case identifier, is reserved, or is in the CORVINT_ namespace refuses.
func TestCCEV0107Session(t *testing.T) {
	r := vocabularyCompile(t, nil)
	sc := compiledFor(t, r, obRebook)
	for _, w := range []string{"test.use({ storageState: process.env.CLUB_ADMIN_STATE });",
		"test.skip(!process.env.CLUB_ADMIN_STATE, 'corvint-e2e session-unset CLUB_ADMIN_STATE');"} {
		if !strings.Contains(string(sc.Text), w) {
			t.Fatalf("missing %q in:\n%s", w, sc.Text)
		}
	}
	if e := entryFor(r, obRebook); sc.SessionEnv != "CLUB_ADMIN_STATE" || !slices.Contains(e.Blocks, "session:club-admin") {
		t.Fatalf("session env %q, blocks %v", sc.SessionEnv, e.Blocks)
	}
	if strings.Contains(string(compiledFor(t, r, obCount).Text), "storageState") {
		t.Fatal("a scenario with no session precondition must not use a storage state")
	}
	two := declWith(t, func(d map[string]any) {
		d["sessions"] = append(d["sessions"].([]any), map[string]any{"session_id": "session:member", "satisfies": []any{"signed in as member"}, "storage_state_env": "MEMBER_STATE"})
	})
	rc := vocabularyCompile(t, map[string]*string{DeclarationsPath: two,
		"flows/rebook-slot.json": text(rebookFlow("rebook-slot", `"club has open slots","signed in as club admin","signed in as member"`))})
	if got := missingReason(rc, obRebook, KindSession, "signed in as member"); got != "session-conflict" {
		t.Fatalf("two sessions: reason %q\n%s", got, rc.Encoded())
	}
	t.Setenv("CLUB_ADMIN_STATE", "")
	os.Unsetenv("CLUB_ADMIN_STATE")
	if slices.Contains(runEnvKeys("CLUB_ADMIN_STATE"), "CLUB_ADMIN_STATE") {
		t.Fatal("an unset session variable must not be declared to the run")
	}
	t.Setenv("CLUB_ADMIN_STATE", "/tmp/state.json")
	if !slices.Contains(runEnvKeys("CLUB_ADMIN_STATE"), "CLUB_ADMIN_STATE") || len(runEnvKeys("")) != 2 {
		t.Fatalf("run env keys %v", runEnvKeys("CLUB_ADMIN_STATE"))
	}
	for _, env := range []string{"club_state", "CORVINT_STATE", "PATH", envBaseURL, ""} {
		t.Run(env, func(t *testing.T) {
			decl := declWith(t, func(d map[string]any) { declMember(d, "sessions", 0)["storage_state_env"] = env })
			if code := catalogRefusal(t, decl); code != "e2e-blocks-invalid" {
				t.Fatalf("refusal %q", code)
			}
		})
	}
	for _, id := range []string{"fixture:open-slots", "user:club-admin", "session:"} {
		decl := declWith(t, func(d map[string]any) { declMember(d, "sessions", 0)["session_id"] = id })
		if code := catalogRefusal(t, decl); code != "e2e-blocks-invalid" {
			t.Fatalf("session ID %q: refusal %q", id, code)
		}
	}
	empty := declWith(t, func(d map[string]any) { declMember(d, "sessions", 0)["satisfies"] = []any{} })
	if code := catalogRefusal(t, empty); code != "e2e-blocks-invalid" {
		t.Fatalf("empty satisfies: refusal %q", code)
	}
}

// CCE-V0-108: a fixture compiles to a Playwright test.extend fixture whose teardown runs after
// use; a failure at the teardown call reads cleanup-failed, and only a setup with no outputs is
// an omitted-event control.
func TestCCEV0108Teardown(t *testing.T) {
	sc := compiledFor(t, vocabularyCompile(t, nil), obCount)
	lines := strings.Split(string(sc.Text), "\n")
	// Once setup succeeds, the output check and use sit in a try whose finally runs cleanup, so a
	// missing output still tears the fixture down.
	if len(sc.CleanupLines) != 1 || strings.TrimSpace(lines[sc.CleanupLines[0]-1]) != "await clubPage.cleanup();" ||
		strings.TrimSpace(lines[sc.CleanupLines[0]-2]) != "} finally {" || strings.TrimSpace(lines[sc.CleanupLines[0]-3]) != "await use(out);" ||
		!strings.HasPrefix(strings.TrimSpace(lines[sc.CleanupLines[0]-4]), "for (const k of ['clubId'])") ||
		strings.TrimSpace(lines[sc.CleanupLines[0]-5]) != "try {" || len(sc.SetupLines) != 0 {
		t.Fatalf("setup %v cleanup %v:\n%s", sc.SetupLines, sc.CleanupLines, sc.Text)
	}
	if !strings.Contains(string(sc.Text), "const test = base.extend<{ po: Record<string, any>; fx1: Record<string, any> }>({") {
		t.Fatalf("teardown is not a test.extend fixture:\n%s", sc.Text)
	}
	if got := assertionFailure("failed", "Error: x", sc.Placement, sc.CleanupLines[0], sc); got != FailureCleanupFailed {
		t.Fatalf("failure at the teardown call = %q", got)
	}
	for _, c := range controlSpecs(sc) {
		if strings.HasPrefix(c.id, "control:omitted-event:setup") {
			t.Fatalf("an output-producing setup is a control: %+v", c)
		}
	}
	root, base := fixtureRepo(t)
	plain := compiledScenario(t, root, base, commitBehaviourChange(t, root))
	plainLines := strings.Split(string(plain.Text), "\n")
	if len(plain.SetupLines) != 1 || strings.TrimSpace(plainLines[plain.SetupLines[0]-1]) != "await clubPage.seedOpenSlots();" ||
		strings.TrimSpace(plainLines[plain.SetupLines[0]]) != "await use({});" {
		t.Fatalf("plain setup %v:\n%s", plain.SetupLines, plain.Text)
	}
}

// CCE-V0-109: a declared role or label locator replaces an outcome's or input's test id; the
// wrong-locator control perturbs its accessible name; a locator for an unadmitted block is
// withheld, and a malformed or secret-shaped locator refuses.
func TestCCEV0109TypedLocators(t *testing.T) {
	sc := compiledFor(t, vocabularyCompile(t, nil), obRebook)
	want := "await expect(page.getByRole('status', { name: 'Slot status', exact: true }), 'corvint-e2e outcome assert:rebook-slot/rebooked').toHaveText('Booked');"
	lines := strings.Split(string(sc.Text), "\n")
	if strings.TrimSpace(lines[sc.AssertionLine-1]) != want || sc.LocatorForm != "role" || sc.Locator != "Slot status" {
		t.Fatalf("assertion line %q form %q locator %q", lines[sc.AssertionLine-1], sc.LocatorForm, sc.Locator)
	}
	out, err := perturbLine(sc.Text, sc.AssertionLine, behaviorfalsify.WrongLocator, sc.LocatorForm, sc.Locator, sc.Matcher, sc.Value, wrongLocator)
	if err != nil || !strings.Contains(string(out), "getByRole('status', { name: '"+wrongLocator+"', exact: true })") {
		t.Fatalf("role wrong-locator: %v\n%s", err, out)
	}
	label := declWith(t, func(d map[string]any) {
		d["locators"] = []any{map[string]any{"block": "outcome:rebook-slot/rebooked", "by": "label", "name": "Slot status"}}
	})
	lsc := compiledFor(t, vocabularyCompile(t, map[string]*string{DeclarationsPath: label}), obRebook)
	if !strings.Contains(string(lsc.Text), "expect(page.getByLabel('Slot status', { exact: true }), ") || lsc.LocatorForm != "label" {
		t.Fatalf("label locator:\n%s", lsc.Text)
	}
	if out, err := perturbLine(lsc.Text, lsc.AssertionLine, behaviorfalsify.WrongLocator, lsc.LocatorForm, lsc.Locator, lsc.Matcher, lsc.Value, wrongLocator); err != nil ||
		!strings.Contains(string(out), "getByLabel('"+wrongLocator+"', { exact: true })") {
		t.Fatalf("label wrong-locator: %v\n%s", err, out)
	}
	if _, err := perturbLine(lsc.Text, lsc.AssertionLine, behaviorfalsify.WrongLocator, "css", lsc.Locator, lsc.Matcher, lsc.Value, wrongLocator); err == nil {
		t.Fatal("an unknown locator form must refuse")
	}
	// The typed locator is part of the oracle: unchanged it stays ADMITTED_AT_BASE; adding,
	// changing or removing it in the diff reads CHANGED_IN_DIFF (CCE-V0-016).
	if sc.OracleOrigin != OriginAdmittedAtBase {
		t.Fatalf("unchanged typed locator origin %q", sc.OracleOrigin)
	}
	noTyped := declWith(t, func(d map[string]any) { delete(d, "locators") })
	for name, tc := range map[string]struct{ base, head *string }{
		"changed form": {nil, label},
		"removed":      {nil, noTyped},
		"added":        {noTyped, text(vocabularyDeclarations)},
	} {
		t.Run(name, func(t *testing.T) {
			edit := map[string]*string{}
			if tc.base != nil {
				edit[DeclarationsPath] = tc.base
			}
			root, base, _ := vocabularyRepo(t, edit)
			head := commitFiles(t, root, map[string]*string{DeclarationsPath: tc.head}, "locator change")
			r, err := Compile(bg, diffOpts(root, base, head))
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if got := compiledFor(t, r, obRebook).OracleOrigin; got != OriginChangedInDiff {
				t.Fatalf("origin %q, want %q", got, OriginChangedInDiff)
			}
		})
	}
	unadmitted := declWith(t, func(d map[string]any) { declMember(d, "locators", 0)["block"] = "outcome:absent/none" })
	if got := catalogUnadmitted(t, unadmitted, "locator", "outcome:absent/none"); got != "locator-block-unadmitted" {
		t.Fatalf("unadmitted block locator reason %q", got)
	}
	for name, tc := range map[string]struct {
		loc  map[string]any
		want string
	}{
		"role without a role": {map[string]any{"block": "outcome:rebook-slot/rebooked", "by": "role", "name": "Slot"}, "e2e-blocks-invalid"},
		"label with a role":   {map[string]any{"block": "outcome:rebook-slot/rebooked", "by": "label", "role": "status", "name": "Slot"}, "e2e-blocks-invalid"},
		"unknown form":        {map[string]any{"block": "outcome:rebook-slot/rebooked", "by": "css", "name": "Slot"}, "e2e-blocks-invalid"},
		"empty name":          {map[string]any{"block": "outcome:rebook-slot/rebooked", "by": "label", "name": ""}, "e2e-blocks-invalid"},
		"secret-shaped name":  {map[string]any{"block": "outcome:rebook-slot/rebooked", "by": "label", "name": "api_key field"}, "e2e-input-secret"},
	} {
		t.Run(name, func(t *testing.T) {
			if code := catalogRefusal(t, declWith(t, func(d map[string]any) { d["locators"] = []any{tc.loc} })); code != tc.want {
				t.Fatalf("refusal %q, want %q", code, tc.want)
			}
		})
	}
}

// CCE-V0-110: toHaveAccessibleName and a positive toHaveCount are admitted matchers; a count
// renders unquoted and its wrong-expected-value control is the next integer; a zero count is
// vacuous and a non-integer count is not compilable.
func TestCCEV0110TypedAssertions(t *testing.T) {
	sc := compiledFor(t, vocabularyCompile(t, nil), obCount)
	lines := strings.Split(string(sc.Text), "\n")
	if !strings.HasSuffix(lines[sc.AssertionLine-1], ".toHaveCount(3);") {
		t.Fatalf("count assertion %q", lines[sc.AssertionLine-1])
	}
	specs := controlSpecs(sc)
	if specs[1].kind != behaviorfalsify.WrongExpectedValue || specs[1].replacement != "4" {
		t.Fatalf("count wrong-value control %+v", specs[1])
	}
	out, err := perturbLine(sc.Text, sc.AssertionLine, specs[1].kind, sc.LocatorForm, sc.Locator, sc.Matcher, sc.Value, specs[1].replacement)
	if err != nil || !strings.HasSuffix(strings.Split(string(out), "\n")[sc.AssertionLine-1], ".toHaveCount(4);") {
		t.Fatalf("count perturbation: %v", err)
	}
	for _, tc := range []struct{ matcher, value, want string }{
		{"toHaveAccessibleName", "Booked", ""},
		{MatcherCount, "3", ""},
		{MatcherCount, "0", "outcome-vacuous"},
		{MatcherCount, "three", "outcome-not-compilable"},
		{MatcherCount, "-1", "outcome-not-compilable"},
		{"toBeVisible", "x", "outcome-not-compilable"},
	} {
		if got := outcomeAdmissionReason(tc.matcher, "slot", tc.value); got != tc.want {
			t.Fatalf("%s(%q) = %q, want %q", tc.matcher, tc.value, got, tc.want)
		}
	}
}

// CCE-V0-111: an entry blocked only by its required URL parameters is admitted when a declared
// route binds every parameter; without a route, or with a route that does not match, the path is
// missing and nothing is synthesised from an unproven edge. A constant parameter is URI-encoded.
func TestCCEV0111ParameterisedRoutes(t *testing.T) {
	bare := declWith(t, func(d map[string]any) { delete(d, "routes") })
	r := vocabularyCompile(t, map[string]*string{DeclarationsPath: bare})
	if got := missingReason(r, obRebook, KindPath, teeEntry); got != "requires-parent-params" {
		t.Fatalf("no route: reason %q\n%s", got, r.Encoded())
	}
	if e := entryFor(r, obRebook); e.ProvenPath != nil {
		t.Fatalf("a path was synthesised without a proven edge: %+v", e)
	}
	constant := declWith(t, func(d map[string]any) {
		declMember(d, "routes", 0)["params"] = []any{map[string]any{"name": "clubId", "value": "club 7"}}
	})
	sc := compiledFor(t, vocabularyCompile(t, map[string]*string{DeclarationsPath: constant}), obCount)
	if !strings.Contains(string(sc.Text), "page.goto('/#!/clubs/' + encodeURIComponent(String('club 7')) + '/teesheets')") {
		t.Fatalf("constant route:\n%s", sc.Text)
	}
	for name, tc := range map[string]struct {
		route map[string]any
		id    string
		want  string
	}{
		"deep-linkable entry": {map[string]any{"screen_id": "screen:admin:app.home", "params": []any{map[string]any{"name": "clubId", "value": "1"}}}, "path:entry:screen:admin:app.home", "route-not-parameterised"},
		"wrong name":          {map[string]any{"screen_id": "screen:admin:app.clubs.teesheets", "params": []any{map[string]any{"name": "id", "value": "1"}}}, teeEntry, "route-params-mismatch"},
		"missing name":        {map[string]any{"screen_id": "screen:admin:reports", "params": []any{map[string]any{"name": "clubId", "value": "1"}}}, "path:entry:screen:admin:reports", "route-params-mismatch"},
	} {
		t.Run(name, func(t *testing.T) {
			decl := declWith(t, func(d map[string]any) { d["routes"] = []any{tc.route} })
			if got := catalogUnadmitted(t, decl, KindPath, tc.id); got != tc.want {
				t.Fatalf("reason %q, want %q", got, tc.want)
			}
		})
	}
	// The overlay router guards app.clubs: a guarded entry stays unadmitted whatever a route binds.
	root, _ := fixtureRepo(t)
	rev := commitFiles(t, root, map[string]*string{DeclarationsPath: text(vocabularyDeclarations), "e2e/pages/club.page.ts": text(vocabularyClubPage)}, "guarded")
	cat, err := BuildCatalog(bg, CatalogOptions{Root: root, Rev: rev})
	if err != nil {
		t.Fatal(err)
	}
	if got := cat.unadmittedReason(KindPath, teeEntry, ""); got != "guarded(permission/flag)" || cat.Block(teeEntry) != nil {
		t.Fatalf("guarded entry reason %q", got)
	}
	for name, p := range map[string]map[string]any{
		"both bindings": {"name": "clubId", "value": "1", "fixture": "fixture:open-slots", "output": "clubId"},
		"neither":       {"name": "clubId"},
		"bad name":      {"name": "club-id", "value": "1"},
		"secret value":  {"name": "clubId", "value": "token-1"},
	} {
		t.Run(name, func(t *testing.T) {
			code := catalogRefusal(t, declWith(t, func(d map[string]any) { declMember(d, "routes", 0)["params"] = []any{p} }))
			want := "e2e-blocks-invalid"
			if name == "secret value" {
				want = "e2e-input-secret"
			}
			if code != want {
				t.Fatalf("refusal %q, want %q", code, want)
			}
		})
	}
}

// CCE-V0-112: every compiled action is a named test.step, the scenario retains a trace on
// failure, and the compile entry lists each step's line and block.
func TestCCEV0112StepsAndTrace(t *testing.T) {
	r := vocabularyCompile(t, nil)
	sc := compiledFor(t, r, obRebook)
	e := entryFor(r, obRebook)
	lines := strings.Split(string(sc.Text), "\n")
	var blocks []string
	for _, st := range e.Steps {
		blocks = append(blocks, st.Block)
		if !strings.HasPrefix(lines[st.Line-1], "  await test.step("+jsString(st.Block)+", () => ") {
			t.Fatalf("step %+v line %q", st, lines[st.Line-1])
		}
	}
	if !slices.Equal(blocks, []string{teeEntry, "method:e2e/pages/teesheet.page.ts#selectSlot", "method:e2e/pages/teesheet.page.ts#book"}) ||
		e.Steps[len(e.Steps)-1].Line != sc.AssertionLine-1 {
		t.Fatalf("steps %+v, assertion line %d", e.Steps, sc.AssertionLine)
	}
	if strings.Count(string(sc.Text), "test.use({ trace: 'retain-on-failure' });") != 1 {
		t.Fatalf("no trace option:\n%s", sc.Text)
	}
	if !strings.Contains(string(r.Encoded()), `"steps": [`) {
		t.Fatal("the compile report does not carry the steps")
	}
}

// headerVolatile are the compiled header lines that name the fixture's commit revisions.
var headerVolatile = regexp.MustCompile(`(?m)^// (catalog_digest|base|head|digest): .*$`)

// CCE-V0-113: the compiled text of a plain and of each full-vocabulary scenario equals its golden
// output, pinned to CompilerVersion; a golden made by another compiler fails until it is
// regenerated with -cce-write-golden.
func TestCCEV0113GoldenCompile(t *testing.T) {
	root, base := fixtureRepo(t)
	plain := compiledScenario(t, root, base, commitBehaviourChange(t, root))
	r := vocabularyCompile(t, nil)
	for name, sc := range map[string]Scenario{
		"plain": plain, "count": compiledFor(t, r, obCount), "session-route-role": compiledFor(t, r, obRebook), "fixture-output": compiledFor(t, r, obBooked),
	} {
		t.Run(name, func(t *testing.T) {
			got := headerVolatile.ReplaceAllString(string(sc.Text), "// $1: <pinned>")
			p := filepath.Join("testdata", "golden", name+".spec.ts")
			if *writeGolden {
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(p)
			if err != nil {
				t.Fatalf("golden %s: %v (write it with -cce-write-golden)", p, err)
			}
			if !strings.Contains(string(want), "\n// compiler: "+CompilerVersion+"\n") {
				t.Fatalf("golden %s is not pinned to %s: regenerate it with -cce-write-golden", p, CompilerVersion)
			}
			if got != string(want) {
				t.Fatalf("compiled %s differs from its golden %s:\n%s", name, p, got)
			}
		})
	}
}
