package funclink

import (
	"errors"
	"math/rand"
	"reflect"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/appmap"
)

const (
	poLogin   = "page-object:e2e/pages/login.ts#LoginPage"
	mSignIn   = poLogin + ".signIn"
	mOpen     = poLogin + ".open"
	mUnused   = poLogin + ".unused"
	poHelpers = "page-object:e2e/helpers/nav.ts"
	mGoHome   = poHelpers + ".goHome"
	poCart    = "page-object:e2e/pages/cart.ts#CartPage"
	mAdd      = poCart + ".add"
	screenLog = "screen:app:login"
	screenHom = "screen:app:home"
)

func anchor(path string, start, end int) appmap.Anchor {
	return appmap.Anchor{Path: path, Start: start, End: end, Blob: "b"}
}

var (
	selEmail  = appmap.Selector{ID: "sel-email", Kind: "label", Value: "Email"}
	selSubmit = appmap.Selector{ID: "sel-submit", Kind: "role", Value: "button:Sign in"}
)

func step(kind string, loc *appmap.Selector, via ...string) appmap.SKStep {
	return appmap.SKStep{Kind: kind, Locator: loc, Via: via}
}

// fixtureModel is a small SKH-V0 model: LoginPage.signIn calls open, so a test calling signIn
// also uses open; the nav helper is a module step helper; CartPage uses the same email selector
// so the binding rule must not borrow its tests.
func fixtureModel() *appmap.SKModel {
	return &appmap.SKModel{
		PageObjects: []appmap.SKPageObject{
			{ID: poLogin, Name: "LoginPage", Form: appmap.FormPlaywrightClass, Path: "e2e/pages/login.ts",
				Screen: screenLog, Join: appmap.StatusResolved, Anchor: anchor("e2e/pages/login.ts", 1, 40),
				Bindings: []appmap.SKBinding{
					{Name: "email", Locator: selEmail, Anchor: anchor("e2e/pages/login.ts", 3, 3)},
					{Name: "submit", Locator: selSubmit, Anchor: anchor("e2e/pages/login.ts", 4, 4)},
				},
				Methods: []appmap.SKMethod{
					{ID: mOpen, Name: "open", Anchor: anchor("e2e/pages/login.ts", 10, 14)},
					{ID: mSignIn, Name: "signIn", Anchor: anchor("e2e/pages/login.ts", 16, 24)},
					{ID: mUnused, Name: "unused", Anchor: anchor("e2e/pages/login.ts", 26, 30)},
				}},
			{ID: poHelpers, Name: "nav", Form: appmap.FormPlaywrightModule, Path: "e2e/helpers/nav.ts",
				Join: appmap.StatusUnknown, Anchor: anchor("e2e/helpers/nav.ts", 0, 0),
				Methods: []appmap.SKMethod{{ID: mGoHome, Name: "goHome", Anchor: anchor("e2e/helpers/nav.ts", 2, 6)}}},
			{ID: poCart, Name: "CartPage", Form: appmap.FormPlaywrightClass, Path: "e2e/pages/cart.ts",
				Join: appmap.StatusUnknown, Anchor: anchor("e2e/pages/cart.ts", 1, 20),
				Bindings: []appmap.SKBinding{{Name: "email", Locator: selEmail, Anchor: anchor("e2e/pages/cart.ts", 2, 2)}},
				Methods:  []appmap.SKMethod{{ID: mAdd, Name: "add", Anchor: anchor("e2e/pages/cart.ts", 5, 9)}}},
		},
		Tests: []appmap.SKTest{
			{ID: "test:e2e/login.spec.ts#a", Path: "e2e/login.spec.ts", Complete: true, Steps: []appmap.SKStep{
				step(appmap.StepVisit, nil, mSignIn, mOpen),
				step(appmap.StepFill, &selEmail, mSignIn),
				step(appmap.StepClick, &selSubmit, mSignIn),
			}},
			{ID: "test:e2e/login.spec.ts#b", Path: "e2e/login.spec.ts", Complete: true, Steps: []appmap.SKStep{
				step(appmap.StepVisit, nil, mOpen),
			}},
			{ID: "test:e2e/home.spec.ts#c", Path: "e2e/home.spec.ts", Complete: true, Steps: []appmap.SKStep{
				step(appmap.StepVisit, nil, mGoHome),
				step(appmap.StepAssert, nil),
			}},
			{ID: "test:e2e/cart.spec.ts#d", Path: "e2e/cart.spec.ts", Complete: true, Steps: []appmap.SKStep{
				step(appmap.StepFill, &selEmail, mAdd),
			}},
		},
		Records: []appmap.SKRecord{
			{ID: "r1", Source: mGoHome, Tests: []string{"test:e2e/home.spec.ts#c"}},
			{ID: "r2", Source: "test-body", Tests: []string{"test:e2e/cart.spec.ts#d"}},
		},
	}
}

func fixtureMap() *appmap.Map {
	return &appmap.Map{
		Screens: []appmap.Screen{
			{ID: screenLog, Anchor: anchor("web/src/routes.tsx", 10, 12)},
			{ID: screenHom, Anchor: anchor("web/src/routes.tsx", 14, 16),
				Specs: []appmap.Attribution{{File: "e2e/home.spec.ts", Basis: "goto"}}},
		},
		Flows: []appmap.Flow{{ID: "flow:checkout", FlowID: "checkout", Anchor: anchor("flows/checkout.json", 0, 0),
			Steps: []appmap.Step{{ID: "step:checkout/pay", Tests: []string{"e2e/cart.spec.ts::pays"}}}}},
	}
}

func fixtureIntents() appflows.IntentSet {
	return appflows.IntentSet{Dir: "flows", Flows: []appflows.FlowIntent{{
		FlowID: "checkout",
		Steps:  []appflows.FlowStep{{StepID: "pay", Action: "pay"}},
		Links: []appflows.FlowLink{
			{From: "pay", Basis: "declared", Target: appflows.LinkTarget{Type: "source", Path: "web/src/pay.ts", StartLine: 5, EndLine: 30}},
			{From: "pay", Basis: "declared", Target: appflows.LinkTarget{Type: "test", TestKey: "e2e/cart.spec.ts::pays"}},
			{From: "pay", Basis: "inferred", Target: appflows.LinkTarget{Type: "test", TestKey: "e2e/inferred.spec.ts::x"}},
		},
	}}}
}

type fakeObserved struct {
	links []ObservedLink
	err   error
}

func (f fakeObserved) ObservedLinks() ([]ObservedLink, error) { return f.links, f.err }

func fixtureLinks(t *testing.T, obs ObservedSource) *Links {
	t.Helper()
	b := NewBuilder()
	b.FromIntents(fixtureIntents())
	b.FromMap(fixtureMap())
	b.FromImpact(&appmap.ImpactReport{Screens: []appmap.ImpactScreen{{ID: screenLog, Paths: []string{"web/src/Login.tsx"}}}})
	b.FromSiteKnowledge(fixtureModel(), fixtureMap())
	b.FromSetup([]appmap.SKSetup{
		{ID: "setup:auth", Path: "e2e/fixtures.ts", Anchor: anchor("e2e/fixtures.ts", 3, 9)},
		{ID: "setup:orphan", Path: "e2e/fixtures.ts", Anchor: anchor("e2e/fixtures.ts", 20, 22)},
	}, []appmap.SKSetupUse{{Test: "test:e2e/login.spec.ts#a", Uses: []string{"setup:auth"}}})
	if err := b.FromObserved(obs); err != nil {
		t.Fatal(err)
	}
	return b.Build()
}

func testsOf(u *Unit) []string {
	out := []string{}
	for _, t := range u.Tests {
		out = append(out, t.Test)
	}
	return out
}

func TestMCJV0_020_UnitsCarryCodeAndTestSetsFromEverySource(t *testing.T) {
	l := fixtureLinks(t, fakeObserved{links: []ObservedLink{{Unit: Ref{KindScreen, screenHom},
		Code: []Span{{Path: "web/src/Home.tsx", Start: 1, End: 50}}, Tests: []string{"go:web.TestHome"}}}})

	flow := l.Unit(Ref{KindFlow, "flow:checkout"})
	if flow == nil || len(flow.Code) != 1 || flow.Code[0].Span != (Span{"web/src/pay.ts", 5, 30}) || flow.Code[0].Authority != Declared {
		t.Fatalf("flow code = %+v", flow)
	}
	if got := testsOf(flow); !reflect.DeepEqual(got, []string{"e2e/cart.spec.ts::pays"}) {
		t.Fatalf("flow tests = %v (an inferred AFU link must not be read as declared)", got)
	}
	if st := l.Unit(Ref{KindStep, "step:checkout/pay"}); st == nil || len(st.Code) != 1 || len(st.Tests) != 1 {
		t.Fatalf("step unit = %+v", st)
	}

	login := l.Unit(Ref{KindScreen, screenLog})
	wantCode := []Span{{"web/src/Login.tsx", 0, 0}, {"web/src/routes.tsx", 10, 12}}
	var gotCode []Span
	for _, c := range login.Code {
		gotCode = append(gotCode, c.Span)
		if c.Authority != Static {
			t.Fatalf("screen code authority = %v", c.Authority)
		}
	}
	if !reflect.DeepEqual(gotCode, wantCode) {
		t.Fatalf("screen code = %v, want %v", gotCode, wantCode)
	}
	if got := testsOf(login); !reflect.DeepEqual(got, []string{"test:e2e/login.spec.ts#a", "test:e2e/login.spec.ts#b"}) {
		t.Fatalf("login screen tests = %v", got)
	}
	home := l.Unit(Ref{KindScreen, screenHom})
	if got := testsOf(home); !reflect.DeepEqual(got, []string{"go:web.TestHome", "test:e2e/home.spec.ts#c"}) {
		t.Fatalf("home screen tests = %v", got)
	}

	if got := testsOf(l.Unit(Ref{KindMethod, mOpen})); !reflect.DeepEqual(got, []string{"test:e2e/login.spec.ts#a", "test:e2e/login.spec.ts#b"}) {
		t.Fatalf("open callers = %v (a nested call through signIn uses open)", got)
	}
	if got := testsOf(l.Unit(Ref{KindMethod, mSignIn})); !reflect.DeepEqual(got, []string{"test:e2e/login.spec.ts#a"}) {
		t.Fatalf("signIn callers = %v", got)
	}
	if got := testsOf(l.Unit(Ref{KindSetup, "setup:auth"})); !reflect.DeepEqual(got, []string{"test:e2e/login.spec.ts#a"}) {
		t.Fatalf("setup users = %v", got)
	}
	if u := l.Unit(Ref{KindMethod, mUnused}); u == nil || len(u.Tests) != 0 {
		t.Fatalf("unused method = %+v", u)
	}
}

func TestMCJV0_020_LowerAuthorityNeverOverwritesHigher(t *testing.T) {
	r := Ref{KindFlow, "flow:x"}
	span := Span{"a.go", 1, 9}
	offers := []Authority{Observed, Declared, Static}
	var first *Links
	// Every offer order yields the same entry: the highest authority, every offer kept.
	for _, perm := range [][]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}} {
		b := NewBuilder()
		for _, i := range perm {
			b.AddCode(r, span, offers[i])
			b.AddTest(r, "T", offers[i])
		}
		b.AddTest(r, "onlyDeclared", Declared)
		b.AddTest(r, "onlyObserved", Observed)
		b.AddCode(r, Span{"b.go", 3, 4}, Static)
		l := b.Build()
		u := l.Unit(r)
		if u.Code[0].Authority != Declared || u.Tests[0].Authority != Declared ||
			!reflect.DeepEqual(u.Tests[0].Offered, []Authority{Declared, Static, Observed}) {
			t.Fatalf("perm %v: unit = %+v", perm, u)
		}
		if first == nil {
			first = l
		} else if !reflect.DeepEqual(first, l) {
			t.Fatalf("perm %v changed the build", perm)
		}
	}
	got := map[string]bool{}
	for _, d := range first.Disagreements {
		got[d.Set+" "+d.Item+" "+d.Lower.String()+" "+d.Only] = true
	}
	for _, want := range []string{
		"tests onlyObserved observed lower",  // kept at observed and reported
		"tests onlyDeclared observed higher", // declared, never observed
		"tests onlyDeclared static higher",
		"code b.go:3-4 static lower",
	} {
		if !got[want] {
			t.Errorf("missing disagreement %q in %v", want, got)
		}
	}
	u := first.Unit(r)
	for _, e := range u.Tests {
		if e.Test == "onlyObserved" && e.Authority != Observed {
			t.Fatalf("observed-only entry authority = %v", e.Authority)
		}
	}
}

func TestMCJV0_020_ObservedSourceErrorAddsNothing(t *testing.T) {
	b := NewBuilder()
	err := b.FromObserved(fakeObserved{links: []ObservedLink{{Unit: Ref{KindScreen, "s"}, Tests: []string{"T"}}}, err: errors.New("index refused")})
	if err == nil || len(b.Build().Units) != 0 {
		t.Fatalf("err=%v units=%v", err, b.Build().Units)
	}
	if NewBuilder().FromObserved(nil) != nil {
		t.Fatal("nil observed source must offer nothing")
	}
}

func TestMCJV0_021_AddsLinkedTestsForUncoveredBlocksOnly(t *testing.T) {
	l := fixtureLinks(t, nil)
	keep := []string{"go:kept.TestX"}
	changed := []Span{{"web/src/pay.ts", 10, 12}, {"web/src/Login.tsx", 7, 7}, {"internal/other.go", 1, 2}}
	fresh := []Span{{"web/src/Login.tsx", 1, 20}}
	sel := l.AddUncovered(keep, changed, fresh)

	if !reflect.DeepEqual(sel.Tests, []string{"e2e/cart.spec.ts::pays", "go:kept.TestX"}) {
		t.Fatalf("tests = %v", sel.Tests)
	}
	if len(sel.Added) != 1 {
		t.Fatalf("added = %+v", sel.Added)
	}
	units := map[Ref]Authority{}
	for _, r := range sel.Added[0].Reasons {
		if r.Rule != ReasonFunctionalityLink || r.Block != changed[0] {
			t.Fatalf("reason = %+v", r)
		}
		units[r.Unit] = r.Authority
	}
	if units[Ref{KindFlow, "flow:checkout"}] != Declared || units[Ref{KindStep, "step:checkout/pay"}] != Declared {
		t.Fatalf("reason units = %v", units)
	}
	if !reflect.DeepEqual(sel.Uncovered, []Span{changed[0], changed[2]}) || !reflect.DeepEqual(sel.Unlinked, []Span{changed[2]}) {
		t.Fatalf("uncovered=%v unlinked=%v", sel.Uncovered, sel.Unlinked)
	}

	// A block a fresh record only partly covers is uncovered: the screen's tests are added.
	sel = l.AddUncovered(nil, []Span{{"web/src/Login.tsx", 18, 25}}, fresh)
	if !reflect.DeepEqual(sel.Tests, []string{"test:e2e/login.spec.ts#a", "test:e2e/login.spec.ts#b"}) {
		t.Fatalf("partly covered block tests = %v", sel.Tests)
	}
}

// randomLinks builds links from random offers over a small universe.
func randomLinks(rng *rand.Rand) *Links {
	b := NewBuilder()
	auths := []Authority{Declared, Static, Observed}
	for i := 0; i < 1+rng.Intn(12); i++ {
		r := Ref{Kind([]Kind{KindScreen, KindFlow, KindMethod, KindSetup}[rng.Intn(4)]), "u" + string(rune('a'+rng.Intn(6)))}
		for j := 0; j < rng.Intn(4); j++ {
			s := 1 + rng.Intn(40)
			b.AddCode(r, Span{"f" + string(rune('0'+rng.Intn(3))), s, s + rng.Intn(6)}, auths[rng.Intn(3)])
		}
		for j := 0; j < rng.Intn(5); j++ {
			b.AddTest(r, "t"+string(rune('a'+rng.Intn(10))), auths[rng.Intn(3)])
		}
		if rng.Intn(2) == 0 {
			s := 1 + rng.Intn(40)
			b.AddSurface(SurfaceMethod, r.ID, r, Span{"p" + string(rune('0'+rng.Intn(2))), s, s + rng.Intn(5)}, []string{"t" + string(rune('a'+rng.Intn(10)))}, Static)
		}
	}
	return b.Build()
}

func randomBlocks(rng *rand.Rand, prefix string) []Span {
	var out []Span
	for i := 0; i < rng.Intn(6); i++ {
		s := 1 + rng.Intn(45)
		out = append(out, Span{prefix + string(rune('0'+rng.Intn(3))), s, s + rng.Intn(4)})
	}
	return out
}

func TestMCJV0_021_023_FaultInjectionNeverDropsATest(t *testing.T) {
	rng := rand.New(rand.NewSource(20261103))
	for i := 0; i < 2000; i++ {
		l := randomLinks(rng)
		var keep []string
		for j := 0; j < rng.Intn(6); j++ {
			keep = append(keep, "t"+string(rune('a'+rng.Intn(12))))
		}
		changed := append(randomBlocks(rng, "f"), randomBlocks(rng, "p")...)
		fresh := randomBlocks(rng, "f")
		for name, sel := range map[string]Selection{
			"021": l.AddUncovered(keep, changed, fresh),
			"023": l.SelectSurfaces(keep, changed),
		} {
			for _, k := range keep {
				if !slices.Contains(sel.Tests, k) {
					t.Fatalf("case %d rule %s dropped kept test %s: %v", i, name, k, sel.Tests)
				}
			}
			for _, a := range sel.Added {
				if !slices.Contains(sel.Tests, a.Test) || len(a.Reasons) == 0 {
					t.Fatalf("case %d rule %s: added %+v not selected with a reason", i, name, a)
				}
			}
		}
		// Every test of a unit whose code an uncovered block reaches is selected.
		sel := l.AddUncovered(keep, changed, fresh)
		for _, b := range sel.Uncovered {
			for _, u := range l.Units {
				if codeAuthority(u, b) == 0 {
					continue
				}
				for _, te := range u.Tests {
					if !slices.Contains(sel.Tests, te.Test) {
						t.Fatalf("case %d: unit %s reached by %v lost test %s", i, u.ID, b, te.Test)
					}
				}
			}
		}
		// A covered block adds nothing: covering every changed block leaves exactly keep.
		if got := l.AddUncovered(keep, changed, changed); len(got.Added) != 0 {
			t.Fatalf("case %d: covered blocks added %+v", i, got.Added)
		}
	}
}

func surfaceTests(l *Links, changed ...Span) []string {
	return l.SelectSurfaces(nil, changed).Tests
}

func TestMCJV0_023_PageObjectMethodChangeSelectsExactlyItsCallers(t *testing.T) {
	model := fixtureModel()
	l := fixtureLinks(t, nil)
	// Fault injection: change every line of every method; the selection must equal the callers
	// read independently from the step call chains, never more and never fewer.
	for _, po := range model.PageObjects {
		for _, m := range po.Methods {
			want := map[string]bool{}
			for _, tc := range model.Tests {
				for _, s := range tc.Steps {
					if slices.Contains(s.Via, m.ID) {
						want[tc.ID] = true
					}
				}
			}
			for line := m.Anchor.Start; line <= m.Anchor.End; line++ {
				sel := l.SelectSurfaces(nil, []Span{{m.Anchor.Path, line, line}})
				if !reflect.DeepEqual(sel.Tests, sortedSet(want)) {
					t.Fatalf("%s line %d selected %v, want %v", m.ID, line, sel.Tests, sortedSet(want))
				}
				if len(want) == 0 && (len(sel.Unresolved) != 1 || sel.Unresolved[0].ID != m.ID) {
					t.Fatalf("%s has no callers but is not unresolved: %+v", m.ID, sel.Unresolved)
				}
			}
		}
	}
	// A binding selects the tests with a step on its locator through its own page object only.
	if got := surfaceTests(l, Span{"e2e/pages/login.ts", 3, 3}); !reflect.DeepEqual(got, []string{"test:e2e/login.spec.ts#a"}) {
		t.Fatalf("email binding selected %v", got)
	}
	if got := surfaceTests(l, Span{"e2e/pages/cart.ts", 2, 2}); !reflect.DeepEqual(got, []string{"test:e2e/cart.spec.ts#d"}) {
		t.Fatalf("cart binding selected %v", got)
	}
	// Class body outside any method or binding, and a block reaching past a method, select every
	// test of the page object.
	all := []string{"test:e2e/login.spec.ts#a", "test:e2e/login.spec.ts#b"}
	if got := surfaceTests(l, Span{"e2e/pages/login.ts", 7, 7}); !reflect.DeepEqual(got, all) {
		t.Fatalf("class body selected %v", got)
	}
	if got := surfaceTests(l, Span{"e2e/pages/login.ts", 14, 16}); !reflect.DeepEqual(got, all) {
		t.Fatalf("straddling block selected %v", got)
	}
	// A module step helper, a setup record and a flow declaration select their users.
	if got := surfaceTests(l, Span{"e2e/helpers/nav.ts", 4, 4}); !reflect.DeepEqual(got, []string{"test:e2e/home.spec.ts#c"}) {
		t.Fatalf("step helper selected %v", got)
	}
	if got := surfaceTests(l, Span{"e2e/fixtures.ts", 5, 5}); !reflect.DeepEqual(got, []string{"test:e2e/login.spec.ts#a"}) {
		t.Fatalf("setup selected %v", got)
	}
	if got := surfaceTests(l, Span{"flows/checkout.json", 2, 2}); !reflect.DeepEqual(got, []string{"e2e/cart.spec.ts::pays"}) {
		t.Fatalf("flow declaration selected %v", got)
	}
	sel := l.SelectSurfaces(nil, []Span{{"e2e/fixtures.ts", 21, 21}})
	if len(sel.Tests) != 0 || len(sel.Unresolved) != 1 || sel.Unresolved[0].ID != "setup:orphan" {
		t.Fatalf("orphan setup = %+v", sel)
	}
	for _, a := range l.SelectSurfaces(nil, []Span{{"e2e/pages/login.ts", 20, 20}}).Added {
		if a.Reasons[0].Rule != ReasonTestSurface || a.Reasons[0].Surface != string(SurfaceMethod)+" "+mSignIn {
			t.Fatalf("reason = %+v", a.Reasons)
		}
	}
}

func TestMCJV0_022_ShadowReplacementRecordsTheFallbackMissCheck(t *testing.T) {
	obs := fakeObserved{links: []ObservedLink{{Unit: Ref{KindScreen, "screen:app:observed"},
		Code: []Span{{Path: "web/src/Obs.tsx"}}, Tests: []string{"go:web.TestObs"}}}}
	l := fixtureLinks(t, obs)
	fallback := []string{"e2e/cart.spec.ts::pays", "test:e2e/login.spec.ts#a", "test:e2e/home.spec.ts#c"}

	r := l.ShadowReplace([]Span{{"web/src/pay.ts", 6, 6}}, fallback)
	if r.Mode != "shadow" || !r.Eligible || !reflect.DeepEqual(r.Proposed, []string{"e2e/cart.spec.ts::pays"}) ||
		!reflect.DeepEqual(r.Omitted, []string{"test:e2e/home.spec.ts#c", "test:e2e/login.spec.ts#a"}) ||
		!reflect.DeepEqual(r.Fallback, sortedUnique(fallback)) {
		t.Fatalf("report = %+v", r)
	}
	if got := FallbackMisses(r, []string{"test:e2e/login.spec.ts#a", "e2e/cart.spec.ts::pays"}); !reflect.DeepEqual(got, []string{"test:e2e/login.spec.ts#a"}) {
		t.Fatalf("fallback misses = %v", got)
	}

	// A unit whose code set is only observed never replaces a fallback.
	r = l.ShadowReplace([]Span{{"web/src/Obs.tsx", 3, 3}}, fallback)
	if r.Eligible || len(r.Ineligible) != 1 || r.Ineligible[0].Reason != "observed-only" || len(r.Proposed) != 0 {
		t.Fatalf("observed-only report = %+v", r)
	}
	r = l.ShadowReplace([]Span{{"web/src/pay.ts", 6, 6}, {"cmd/x/main.go", 1, 1}}, fallback)
	if r.Eligible || r.Ineligible[0].Reason != "unlinked" || len(r.Proposed) != 1 {
		t.Fatalf("unlinked report = %+v", r)
	}
	if r := l.ShadowReplace(nil, fallback); r.Eligible {
		t.Fatal("an empty change is never eligible")
	}
}

func TestMCJV0_024_GapReport(t *testing.T) {
	b := NewBuilder()
	b.FromMap(fixtureMap())
	b.FromSiteKnowledge(fixtureModel(), fixtureMap())
	l := b.Build()
	g := l.Gaps([]string{"test:e2e/login.spec.ts#a", "test:e2e/orphan.spec.ts#z"}, []Span{{"web/src/routes.tsx", 11, 11}, {"README.md", 1, 1}})
	if !reflect.DeepEqual(g.Untested, []Ref{}) {
		// The login screen has tests via its page object; the home screen via its spec file.
		t.Fatalf("untested = %v", g.Untested)
	}
	if !reflect.DeepEqual(g.Unmapped, []string{"test:e2e/orphan.spec.ts#z"}) || !reflect.DeepEqual(g.Unlinked, []Span{{"README.md", 1, 1}}) {
		t.Fatalf("gaps = %+v", g)
	}
	b.AddCode(Ref{KindScreen, "screen:app:admin"}, Span{"web/src/routes.tsx", 30, 31}, Static)
	if g := b.Build().Gaps(nil, nil); !reflect.DeepEqual(g.Untested, []Ref{{KindScreen, "screen:app:admin"}}) {
		t.Fatalf("untested = %v", g.Untested)
	}
}

// A direct binding access such as `login.email.fill(...)` has a locator but no page-object method
// in its call chain. It must still select the test when the binding changes, even when another
// test reaches the same binding through a method; with two page objects on that locator, both
// bindings take it, since the step cannot say which one it used.
func TestMCJV0_023_DirectBindingAccessSelectsItsTest(t *testing.T) {
	model := fixtureModel()
	model.Tests = append(model.Tests,
		appmap.SKTest{ID: "test:e2e/direct.spec.ts#e", Path: "e2e/direct.spec.ts", Complete: true,
			Steps: []appmap.SKStep{step(appmap.StepClick, &selSubmit)}},
		appmap.SKTest{ID: "test:e2e/direct.spec.ts#f", Path: "e2e/direct.spec.ts", Complete: true,
			Steps: []appmap.SKStep{step(appmap.StepFill, &selEmail, "helper:unrelated")}})
	b := NewBuilder()
	b.FromSiteKnowledge(model, nil)
	l := b.Build()
	if got, want := surfaceTests(l, Span{"e2e/pages/login.ts", 4, 4}), []string{"test:e2e/direct.spec.ts#e", "test:e2e/login.spec.ts#a"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("submit binding selected %v, want %v", got, want)
	}
	if got, want := surfaceTests(l, Span{"e2e/pages/login.ts", 3, 3}), []string{"test:e2e/direct.spec.ts#f", "test:e2e/login.spec.ts#a"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("login email binding selected %v, want %v", got, want)
	}
	if got, want := surfaceTests(l, Span{"e2e/pages/cart.ts", 2, 2}), []string{"test:e2e/cart.spec.ts#d", "test:e2e/direct.spec.ts#f"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("cart email binding selected %v, want %v", got, want)
	}
	// The page object's own span selects its direct users too.
	if got := surfaceTests(l, Span{"e2e/pages/login.ts", 7, 7}); !slices.Contains(got, "test:e2e/direct.spec.ts#e") {
		t.Fatalf("class body selected %v", got)
	}
}

// A changed block that a code set holds only in part is not fully linked: its tests are still
// added, but replacement is ineligible and the unheld lines are reported as unlinked.
func TestMCJV0_022_024_PartialLinkKeepsTheRemainderUnlinked(t *testing.T) {
	l := fixtureLinks(t, nil)
	// web/src/pay.ts:5-30 is declared for the checkout flow.
	changed := []Span{{"web/src/pay.ts", 25, 45}}
	r := l.ShadowReplace(changed, []string{"e2e/cart.spec.ts::pays", "test:e2e/home.spec.ts#c"})
	if r.Eligible || !reflect.DeepEqual(r.Ineligible, []Ineligible{{Block: Span{"web/src/pay.ts", 31, 45}, Reason: "partially-linked"}}) ||
		!reflect.DeepEqual(r.Proposed, []string{"e2e/cart.spec.ts::pays"}) {
		t.Fatalf("partial report = %+v", r)
	}
	if g := l.Gaps(nil, changed); !reflect.DeepEqual(g.Unlinked, []Span{{"web/src/pay.ts", 31, 45}}) {
		t.Fatalf("gaps unlinked = %v", g.Unlinked)
	}
	sel := l.AddUncovered(nil, changed, nil)
	if !slices.Contains(sel.Tests, "e2e/cart.spec.ts::pays") || !reflect.DeepEqual(sel.Unlinked, []Span{{"web/src/pay.ts", 31, 45}}) {
		t.Fatalf("add uncovered = %+v", sel)
	}
	if r := l.ShadowReplace([]Span{{"web/src/pay.ts", 5, 30}}, nil); !r.Eligible {
		t.Fatalf("a fully held block is eligible: %+v", r)
	}
}

func TestUncoveredParts(t *testing.T) {
	b := Span{"a.go", 10, 30}
	for _, c := range []struct {
		spans []Span
		want  []Span
	}{
		{nil, []Span{b}},
		{[]Span{{"a.go", 0, 0}}, nil},
		{[]Span{{"b.go", 0, 0}}, []Span{b}},
		{[]Span{{"a.go", 5, 40}}, nil},
		{[]Span{{"a.go", 12, 15}, {"a.go", 14, 20}, {"a.go", 25, 26}}, []Span{{"a.go", 10, 11}, {"a.go", 21, 24}, {"a.go", 27, 30}}},
		{[]Span{{"a.go", 10, 19}, {"a.go", 20, 30}}, nil},
	} {
		if got := uncoveredParts(b, c.spans); !reflect.DeepEqual(got, c.want) {
			t.Errorf("uncoveredParts(%v, %v) = %v, want %v", b, c.spans, got, c.want)
		}
	}
	if got := uncoveredParts(Span{Path: "a.go"}, []Span{{"a.go", 1, 100}}); !reflect.DeepEqual(got, []Span{{Path: "a.go"}}) {
		t.Errorf("whole-file block held by a range = %v", got)
	}
}
