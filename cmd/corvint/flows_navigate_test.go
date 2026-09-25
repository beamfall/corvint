package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
)

// navFixture is a small shop with navigation blocks: sign-in and search are verified at HEAD,
// checkout and profile need sign-in first, orders-api leaves its effect undeclared, and returns has
// no navigation and no link.
type navFixture struct {
	root, evidence, traffic string
}

type navStepSpec struct {
	id, action string
	nav        *appflows.NavStep
}

func navIntent(id, kind string, pre []string, steps []navStepSpec, outcomes map[string]string, variation []string, testKey string) appflows.FlowIntent {
	f := appflows.FlowIntent{Schema: appflows.FlowIntentSchema, FlowID: id, Revision: 1, Kind: kind, Actor: "shopper", Preconditions: []string{},
		Steps: []appflows.FlowStep{}, Outcomes: []appflows.FlowOutcome{}, Variations: []appflows.FlowVariation{}, Links: []appflows.FlowLink{}}
	nav := &appflows.FlowNavigation{PreconditionFlows: pre, Steps: []appflows.NavStep{}}
	for _, s := range steps {
		f.Steps = append(f.Steps, appflows.FlowStep{StepID: s.id, Action: s.action})
		if s.nav != nil {
			s.nav.StepID = s.id
			nav.Steps = append(nav.Steps, *s.nav)
		}
	}
	for _, o := range slices.Sorted(maps.Keys(outcomes)) {
		f.Outcomes = append(f.Outcomes, appflows.FlowOutcome{OutcomeID: o, Behavior: o + " shown", Matcher: "toBeVisible", Locator: outcomes[o], Value: o})
	}
	if pre != nil {
		f.Navigation = nav
	}
	if variation != nil {
		f.Variations = append(f.Variations, appflows.FlowVariation{VariationID: id + ".happy", Preconditions: []string{}, Steps: variation, ObservableFacts: []string{}, Outcomes: []string{}, Projects: []string{}})
		f.Links = append(f.Links, shopLink(id+".happy", "declared", "", appflows.LinkTarget{Type: "test", Path: "e2e/" + id + ".spec.ts", TestKey: testKey}))
	}
	return f
}

func ui(state string, l appflows.NavLocator, effect string, expect ...string) *appflows.NavStep {
	return &appflows.NavStep{State: state, Locator: l, Effect: effect, Expect: append([]string{}, expect...)}
}

func role(r, name string) appflows.NavLocator { return appflows.NavLocator{Role: r, Name: name} }
func testID(id string) appflows.NavLocator    { return appflows.NavLocator{TestID: id} }

func navIntents() []appflows.FlowIntent {
	withFixture := func(s *appflows.NavStep, fixture string) *appflows.NavStep { s.InputFixture = fixture; return s }
	submit := ui("/login", role("button", "Sign in"), appflows.EffectWriteReversible, "signed-in")
	submit.Ready = &appflows.NavLocator{TestID: "login-form"}
	pay := ui("/cart", role("button", "Pay"), appflows.EffectExternal, "paid")
	pay.Recovery = "dismiss-error"
	flows := []appflows.FlowIntent{
		navIntent("sign-in", "ui", []string{}, []navStepSpec{
			{"open-login", "open the login page", ui("/login", role("heading", "Sign in"), appflows.EffectRead)},
			{"enter-user", "enter the username", withFixture(ui("/login", testID("username"), appflows.EffectRead), "shopper-username")},
			{"enter-password", "enter the password", withFixture(ui("/login", testID("password"), appflows.EffectRead), "shopper-password")},
			{"submit-login", "press sign in", submit},
		}, map[string]string{"signed-in": "account-menu"}, []string{"open-login", "enter-user", "enter-password", "submit-login"}, "sign-in.spec.ts > signs in"),
		navIntent("search", "ui", []string{}, []navStepSpec{
			{"enter-query", "type a query", withFixture(ui("/search", testID("search-box"), appflows.EffectRead), "query-text")},
			{"run-search", "press search", ui("/search", role("button", "Search"), appflows.EffectRead, "listed")},
		}, map[string]string{"listed": "results"}, []string{"enter-query", "run-search"}, "search.spec.ts > finds"),
		navIntent("checkout", "ui", []string{"sign-in"}, []navStepSpec{
			{"open-cart", "open the cart", ui("/cart", role("link", "Cart"), appflows.EffectRead)},
			{"pay", "press pay", pay},
			{"dismiss-error", "dismiss the payment error", ui("/cart", role("button", "Dismiss"), appflows.EffectRead)},
		}, map[string]string{"paid": "order-paid"}, []string{"open-cart", "pay"}, "checkout.spec.ts > pays"),
		navIntent("orders-api", "api", []string{}, []navStepSpec{
			{"create-order", "create an order", &appflows.NavStep{Locator: appflows.NavLocator{Method: "POST", Path: "/api/orders"}, Expect: []string{"created"}}},
		}, map[string]string{"created": "order-created"}, nil, ""),
		navIntent("profile", "ui", []string{"sign-in"}, []navStepSpec{
			{"edit-name", "edit the display name", withFixture(ui("/profile", testID("display-name"), appflows.EffectRead), "display-name")},
			{"save-profile", "press save", ui("/profile", role("button", "Save"), appflows.EffectRead, "saved")},
		}, map[string]string{"saved": "profile-saved"}, nil, ""),
		navIntent("returns", "ui", nil, []navStepSpec{{"request-return", "request a return", nil}}, map[string]string{"refunded": "refund-note"}, nil, ""),
	}
	flows[0].Preconditions = []string{"a shopper account exists"}
	flows[2].Preconditions = []string{"the cart holds one item"}
	return flows
}

func newNavFixture(t *testing.T) navFixture {
	t.Helper()
	root := t.TempDir()
	shopGit(t, root, "init", "-q")
	files := map[string]string{"e2e/sign-in.spec.ts": "test('signs in', () => {});\n", "e2e/search.spec.ts": "test('finds', () => {});\n"}
	for _, f := range navIntents() {
		raw, err := json.Marshal(f)
		if err != nil {
			t.Fatal(err)
		}
		files["flows/"+f.FlowID+".json"] = string(raw)
	}
	shopWrite(t, root, files)
	head := shopCommit(t, root, "navigation fixture")
	at := appflows.RunSource{Commit: head, Tree: shopGit(t, root, "rev-parse", "HEAD^{tree}"), Clean: true}
	var lines bytes.Buffer
	for _, key := range []string{"sign-in.spec.ts > signs in", "search.spec.ts > finds"} {
		line, err := appflows.EncodeRunEvidence(shopRecord(key, "chromium", at, "done", []string{"passed"}, []appflows.NegativeControl{}))
		if err != nil {
			t.Fatal(err)
		}
		lines.Write(line)
	}
	dir := t.TempDir()
	fx := navFixture{root: root, evidence: filepath.Join(dir, "runs.jsonl"), traffic: filepath.Join(dir, "traffic.jsonl")}
	traffic := `{"schema":"application-flow-traffic/0","flow_id":"profile","step_id":"save-profile","method":"POST","form_submit":false}
{"schema":"application-flow-traffic/0","flow_id":"checkout","step_id":"pay","method":"GET","form_submit":false}
{"schema":"application-flow-traffic/0","flow_id":"sign-in","step_id":"submit-login","method":"POST","form_submit":true}
{"schema":"application-flow-traffic/0","flow_id":"search","step_id":"run-search","method":"GET","form_submit":false}
{"schema":"application-flow-traffic/0","flow_id":"search","step_id":"ghost","method":"DELETE","form_submit":false}
`
	if err := os.WriteFile(fx.evidence, lines.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fx.traffic, []byte(traffic), 0600); err != nil {
		t.Fatal(err)
	}
	return fx
}

func navGolden(t *testing.T, name, out string) {
	t.Helper()
	want, err := os.ReadFile(filepath.Join("testdata", "flows", name+".golden.json"))
	if err != nil || out != string(want) {
		t.Errorf("%s golden mismatch (%v); got:\n%s", name, err, out)
	}
}

func navigate(t *testing.T, fx navFixture, args ...string) string {
	t.Helper()
	code, out, diagnostic := runFlowsCLI(fx.root, append([]string{"navigate", "--flows", "flows", "--evidence", fx.evidence, "--traffic", fx.traffic}, args...)...)
	if code != 0 {
		t.Fatalf("navigate %v exited %d: %s", args, code, diagnostic)
	}
	return out
}

func navTransition(t *testing.T, out, flow, step string) appflows.Transition {
	t.Helper()
	var m appflows.NavigationMap
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	for _, f := range m.Flows {
		for _, tr := range f.Transitions {
			if f.FlowID == flow && tr.StepID == step {
				return tr
			}
		}
	}
	t.Fatalf("no transition %s/%s", flow, step)
	return appflows.Transition{}
}

// AFU-V1-025: the map carries states, transitions with locator, preconditions, readiness, expected
// observations, effect class, recovery and verified state, and names credentials only by fixture ID.
func TestAFUV1025NavigationMapGolden(t *testing.T) {
	fx := newNavFixture(t)
	before := flowSnapshot(t, fx.root)
	out := navigate(t, fx)
	navGolden(t, "navigate", out)
	if !strings.Contains(out, `"schema":"application-navigation-map/0"`) || !strings.Contains(out, `"input_fixture":"shopper-password"`) {
		t.Fatalf("map lacks schema or fixture ID: %s", out)
	}
	if tr := navTransition(t, out, "sign-in", "submit-login"); tr.Verification != "verified" || tr.Ready == nil || len(tr.Expect) != 1 {
		t.Fatalf("verified sign-in transition: %+v", tr)
	}
	if tr := navTransition(t, out, "checkout", "pay"); tr.Verification != "unverified" || tr.Recovery != "dismiss-error" {
		t.Fatalf("unverified checkout transition: %+v", tr)
	}
	if !maps.Equal(flowSnapshot(t, fx.root), before) {
		t.Fatal("flows navigate changed the repository")
	}
}

// AFU-V1-026: locators come only from the intent; a step without a navigation entry gets none, even
// though its outcome names a locator string, and a traffic record cannot carry page text.
func TestAFUV1026LocatorsOnlyFromIntent(t *testing.T) {
	fx := newNavFixture(t)
	out := navigate(t, fx)
	if tr := navTransition(t, out, "returns", "request-return"); tr.Locator != nil || tr.State != "" {
		t.Fatalf("unmapped step served a locator: %+v", tr)
	}
	if tr := navTransition(t, out, "orders-api", "create-order"); tr.Locator == nil || tr.Locator.Method != "POST" || tr.Locator.Path != "/api/orders" || tr.State != "api:POST /api/orders" {
		t.Fatalf("api locator: %+v", tr)
	}
	scraped := filepath.Join(t.TempDir(), "scraped.jsonl")
	body := `{"schema":"application-flow-traffic/0","flow_id":"search","step_id":"run-search","method":"GET","form_submit":false,"text":"Search now"}` + "\n"
	if err := os.WriteFile(scraped, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	if code, out, _ := runFlowsCLI(fx.root, "navigate", "--flows", "flows", "--traffic", scraped); code != 2 || out != "" {
		t.Fatalf("traffic with page text accepted: %d %s", code, out)
	}
}

// AFU-V1-027: observed non-GET traffic raises a declared read, never lowers a class, and an
// undeclared class is write-irreversible.
func TestAFUV1027EffectClasses(t *testing.T) {
	fx := newNavFixture(t)
	out := navigate(t, fx)
	cases := []struct{ flow, step, class, basis string }{
		{"profile", "save-profile", appflows.EffectWriteIrreversible, "observed-traffic"},
		{"checkout", "pay", appflows.EffectExternal, "declared"},
		{"sign-in", "submit-login", appflows.EffectWriteReversible, "declared"},
		{"search", "run-search", appflows.EffectRead, "declared"},
		{"orders-api", "create-order", appflows.EffectWriteIrreversible, "undeclared"},
		{"returns", "request-return", appflows.EffectWriteIrreversible, "undeclared"},
	}
	for _, c := range cases {
		if tr := navTransition(t, out, c.flow, c.step); tr.EffectClass != c.class || tr.EffectBasis != c.basis {
			t.Errorf("%s/%s: %s %s, want %s %s", c.flow, c.step, tr.EffectClass, tr.EffectBasis, c.class, c.basis)
		}
	}
}

// AFU-V1-028: the goal packet lists the precondition flows then the goal, labels unverified steps,
// and marks each step above --max-effect requires-grant with the class it needs.
func TestAFUV1028PacketRequiresGrant(t *testing.T) {
	fx := newNavFixture(t)
	out := navigate(t, fx, "--goal", "checkout")
	navGolden(t, "navigate-checkout", out)
	packet := decodePacket(t, out)
	var ids []string
	for _, s := range packet.Steps {
		ids = append(ids, s.FlowID+"/"+s.StepID+"/"+s.Grant+"/"+s.GrantNeeded+"/"+s.Verification)
	}
	want := []string{"sign-in/open-login/granted//verified", "sign-in/enter-user/granted//verified", "sign-in/enter-password/granted//verified",
		"sign-in/submit-login/requires-grant/write-reversible/verified", "checkout/open-cart/granted//unverified", "checkout/pay/requires-grant/external-side-effect/unverified"}
	if !slices.Equal(ids, want) || len(packet.Recovery) != 1 || packet.Recovery[0].StepID != "dismiss-error" || packet.MaxEffect != "read" {
		t.Fatalf("packet steps %v recovery %+v", ids, packet.Recovery)
	}
	granted := decodePacket(t, navigate(t, fx, "--goal", "checkout", "--max-effect", "write-irreversible"))
	if g := granted.Steps[len(granted.Steps)-1]; g.Grant != "requires-grant" || g.GrantNeeded != "external-side-effect" || granted.Steps[3].Grant != "granted" {
		t.Fatalf("write-irreversible grant: %+v", granted.Steps)
	}
	for _, args := range [][]string{{"--goal", "checkout", "--max-effect", "write"}, {"--max-effect", "read"}, {"--goal", "nowhere"}} {
		if code, out, _ := runFlowsCLI(fx.root, append([]string{"navigate", "--flows", "flows"}, args...)...); code != 2 || out != "" {
			t.Fatalf("%v: %d %s", args, code, out)
		}
	}
}

func decodePacket(t *testing.T, out string) appflows.NavigationPacket {
	t.Helper()
	var p appflows.NavigationPacket
	if err := json.Unmarshal([]byte(out), &p); err != nil || p.Schema != appflows.NavigationPacketSchema {
		t.Fatalf("packet %v: %s", err, out)
	}
	return p
}

// simApp is a deterministic stand-in application. /cart and /profile redirect to /login until the
// shopper signs in; the first payment fails and shows a dismissible error.
type simApp struct {
	route    string
	fields   map[string]string
	visible  map[string]bool
	signedIn bool
	payments int
	writes   int
}

var simRoutes = map[string][]string{
	"/login":   {"role:heading|Sign in", "test:username", "test:password", "test:login-form", "role:button|Sign in"},
	"/search":  {"test:search-box", "role:button|Search"},
	"/cart":    {"role:link|Cart", "role:button|Pay", "role:button|Dismiss"},
	"/profile": {"test:display-name", "role:button|Save"},
}

var simProtected = []string{"/cart", "/profile"}

func simKey(l appflows.NavLocator) string {
	if l.TestID != "" {
		return "test:" + l.TestID
	}
	if l.Method != "" {
		return "api:" + l.Method + " " + l.Path
	}
	return "role:" + l.Role + "|" + l.Name
}

func (a *simApp) open(route string) {
	a.route = route
	if slices.Contains(simProtected, route) && !a.signedIn {
		a.route = "/login"
	}
}

func (a *simApp) present(l appflows.NavLocator) bool {
	return strings.HasPrefix(simKey(l), "api:") || slices.Contains(simRoutes[a.route], simKey(l))
}

var simHandlers = map[string]func(a *simApp){
	"role:button|Sign in": func(a *simApp) {
		a.writes++
		a.signedIn = a.fields["test:username"] == "fixture-user" && a.fields["test:password"] == "fixture-pass"
		a.visible["account-menu"] = a.signedIn
	},
	"role:button|Search": func(a *simApp) { a.visible["results"] = a.fields["test:search-box"] != "" },
	"role:button|Pay": func(a *simApp) {
		a.writes++
		a.payments++
		a.visible["payment-error"] = a.payments == 1
		a.visible["order-paid"] = a.payments > 1
	},
	"role:button|Dismiss":  func(a *simApp) { a.visible["payment-error"] = false },
	"role:button|Save":     func(a *simApp) { a.writes++; a.visible["profile-saved"] = a.fields["test:display-name"] != "" },
	"api:POST /api/orders": func(a *simApp) { a.writes++; a.visible["order-created"] = true },
}

// scriptedAgent drives simApp from the packet alone plus fixture values keyed by fixture ID. It
// stops at the first requires-grant step and at any step it cannot perform after its recovery.
type scriptedAgent struct {
	app      *simApp
	states   map[string]appflows.NavState
	recovery map[string]appflows.Transition
	fixtures map[string]string
}

var errStop = errors.New("step not performed")

func (g scriptedAgent) perform(s appflows.Transition) error {
	if s.Grant != appflows.GrantGranted || s.Locator == nil {
		return errStop
	}
	if st := g.states[s.State]; st.Kind == "route" {
		g.app.open(st.Template)
	}
	value, fixtureKnown := g.fixtures[s.InputFixture]
	ready := s.Ready == nil || g.app.present(*s.Ready)
	if !g.app.present(*s.Locator) || !ready || (s.InputFixture != "" && !fixtureKnown) {
		return errStop
	}
	key := simKey(*s.Locator)
	if s.InputFixture != "" {
		g.app.fields[key] = value
	}
	if handler := simHandlers[key]; handler != nil {
		handler(g.app)
	}
	if slices.ContainsFunc(s.Expect, func(o appflows.FlowOutcome) bool { return !g.app.visible[o.Locator] }) {
		return errStop
	}
	return nil
}

func (g scriptedAgent) run(steps []appflows.Transition) (int, error) {
	for i, s := range steps {
		err := g.perform(s)
		rec, hasRecovery := g.recovery[s.Recovery]
		if err != nil && hasRecovery && g.perform(rec) == nil {
			err = g.perform(s)
		}
		if err != nil {
			return i, err
		}
	}
	return len(steps), nil
}

func runScriptedAgent(t *testing.T, out string) (*simApp, int, error) {
	t.Helper()
	packet := decodePacket(t, out)
	g := scriptedAgent{app: &simApp{route: "/", fields: map[string]string{}, visible: map[string]bool{}}, states: map[string]appflows.NavState{},
		recovery: map[string]appflows.Transition{}, fixtures: map[string]string{"shopper-username": "fixture-user", "shopper-password": "fixture-pass",
			"query-text": "lamp", "display-name": "Ada"}}
	for _, s := range packet.States {
		g.states[s.StateID] = s
	}
	for _, r := range packet.Recovery {
		g.recovery[r.StepID] = r
	}
	for _, v := range g.fixtures {
		if strings.Contains(out, `"`+v+`"`) {
			t.Fatalf("packet serves fixture value %q", v)
		}
	}
	done, err := g.run(packet.Steps)
	return g.app, done, err
}

// AFU-V1-028: a deterministic scripted agent completes every mapped fixture goal from the packet
// alone when granted, performs no write under the default read grant, and cannot run an unmapped goal.
func TestAFUV1028ScriptedAgentCompletesGoals(t *testing.T) {
	fx := newNavFixture(t)
	goals := map[string]string{"sign-in": "account-menu", "search": "results", "checkout": "order-paid", "orders-api": "order-created", "profile": "profile-saved"}
	for goal, outcome := range goals {
		app, done, err := runScriptedAgent(t, navigate(t, fx, "--goal", goal, "--max-effect", "external-side-effect"))
		if err != nil || !app.visible[outcome] {
			t.Errorf("goal %s stopped after %d steps: %v", goal, done, err)
		}
		app, _, err = runScriptedAgent(t, navigate(t, fx, "--goal", goal))
		if app.writes != 0 || (goal == "search") != (err == nil) {
			t.Errorf("goal %s under read: writes %d err %v", goal, app.writes, err)
		}
	}
	if _, done, err := runScriptedAgent(t, navigate(t, fx, "--goal", "returns", "--max-effect", "external-side-effect")); err == nil || done != 0 {
		t.Fatalf("unmapped goal ran %d steps: %v", done, err)
	}
}
