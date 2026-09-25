package appflows

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
)

func navSample(id string, pre ...string) FlowIntent {
	f := sampleIntent(id)
	f.Navigation = &FlowNavigation{PreconditionFlows: append([]string{}, pre...), Steps: []NavStep{
		{StepID: "open", State: "/cart", Locator: NavLocator{Role: "link", Name: "Cart"}, Expect: []string{}, Effect: EffectRead},
		{StepID: "pay", State: "/cart", Locator: NavLocator{TestID: "pay"}, Ready: &NavLocator{TestID: "cart"}, Expect: []string{"paid"}, Recovery: "open"},
	}}
	return f
}

func navRepo(t *testing.T, flows ...FlowIntent) string {
	t.Helper()
	root := t.TempDir()
	writeRaw(t, root, "app.js", []byte("function pay() {}\n"))
	for _, f := range flows {
		writeIntent(t, root, "flows/"+f.FlowID+".json", f)
	}
	gitTest(t, root, "init", "-q")
	commitAll(t, root, "fixture")
	return root
}

func codeOf(err error) string {
	var coded *gokernel.Error
	if errors.As(err, &coded) {
		return coded.Code
	}
	return ""
}

// AFU-V1-025: a precondition flow that is not declared, or that closes a cycle, refuses the map.
func TestAFUV1025PreconditionFlowsResolve(t *testing.T) {
	ctx := context.Background()
	cases := map[string][]FlowIntent{
		"not declared": {navSample("checkout", "sign-in")},
		"cycle":        {navSample("checkout", "sign-in"), navSample("sign-in", "checkout")},
	}
	for name, flows := range cases {
		root := navRepo(t, flows...)
		set, err := LoadIntentsAt(ctx, root, "flows", "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = FlowNavigationMap(ctx, root, set, nil, nil); err == nil || !strings.Contains(err.Error(), "precondition") {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// AFU-V1-026: navigation locators are only an accessible role and name, a test ID, or an API
// method and path template; a closed intent refuses any other shape, including page text.
func TestAFUV1026NavigationLocatorShapes(t *testing.T) {
	bad := map[string]func(n *FlowNavigation){
		"role and test id":   func(n *FlowNavigation) { n.Steps[0].Locator.TestID = "cart" },
		"ui method":          func(n *FlowNavigation) { n.Steps[0].Locator = NavLocator{Method: "GET", Path: "/cart"} },
		"name without role":  func(n *FlowNavigation) { n.Steps[0].Locator = NavLocator{Name: "Cart"} },
		"relative route":     func(n *FlowNavigation) { n.Steps[0].State = "cart" },
		"ready api form":     func(n *FlowNavigation) { n.Steps[1].Ready = &NavLocator{Method: "GET", Path: "/x"} },
		"unknown step":       func(n *FlowNavigation) { n.Steps[0].StepID = "ghost" },
		"duplicate step":     func(n *FlowNavigation) { n.Steps[1].StepID = "open" },
		"unknown outcome":    func(n *FlowNavigation) { n.Steps[0].Expect = []string{"ghost"} },
		"literal fixture":    func(n *FlowNavigation) { n.Steps[0].InputFixture = "pass word" },
		"unknown effect":     func(n *FlowNavigation) { n.Steps[0].Effect = "write" },
		"self recovery":      func(n *FlowNavigation) { n.Steps[1].Recovery = "pay" },
		"self precondition":  func(n *FlowNavigation) { n.PreconditionFlows = []string{"checkout"} },
		"invalid flow id":    func(n *FlowNavigation) { n.PreconditionFlows = []string{"Sign In"} },
		"repeated condition": func(n *FlowNavigation) { n.PreconditionFlows = []string{"a", "a"} },
	}
	for name, mutate := range bad {
		f := navSample("checkout")
		mutate(f.Navigation)
		if err := ValidateIntent(f); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	api := sampleIntent("orders")
	api.Kind = "api"
	api.Navigation = &FlowNavigation{PreconditionFlows: []string{}, Steps: []NavStep{{StepID: "pay", Locator: NavLocator{Method: "POST", Path: "/api/orders/{id}"}, Expect: []string{}}}}
	if err := ValidateIntent(api); err != nil {
		t.Fatalf("api locator refused: %v", err)
	}
	for name, s := range map[string]NavStep{
		"state on api":  {StepID: "pay", State: "/x", Locator: NavLocator{Method: "POST", Path: "/x"}, Expect: []string{}},
		"read on POST":  {StepID: "pay", Locator: NavLocator{Method: "POST", Path: "/x"}, Expect: []string{}, Effect: EffectRead},
		"unknown verb":  {StepID: "pay", Locator: NavLocator{Method: "TRACE", Path: "/x"}, Expect: []string{}},
		"role on api":   {StepID: "pay", Locator: NavLocator{Method: "GET", Path: "/x", Role: "button"}, Expect: []string{}},
		"query in path": {StepID: "pay", Locator: NavLocator{Method: "GET", Path: "/x?q=1"}, Expect: []string{}},
	} {
		api.Navigation.Steps = []NavStep{s}
		if err := ValidateIntent(api); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	raw, err := encodeIntent(navSample("checkout"))
	if err != nil {
		t.Fatal(err)
	}
	scraped := strings.Replace(string(raw), `"test_id": "pay"`, `"test_id": "pay", "text": "Pay now"`, 1)
	if scraped == string(raw) || Decode([]byte(scraped), new(FlowIntent)) == nil {
		t.Fatal("a locator carrying page text decoded")
	}
}

// AFU-V1-027: an undeclared class is write-irreversible; non-GET traffic or a form submit raises a
// declared read to write-irreversible; no observation lowers a class.
func TestAFUV1027EffectRaisedNeverLowered(t *testing.T) {
	get := TrafficRecord{Method: "GET"}
	post := TrafficRecord{Method: "POST"}
	form := TrafficRecord{Method: "GET", FormSubmit: true}
	head := TrafficRecord{Method: "HEAD"}
	cases := []struct {
		declared     string
		observed     []TrafficRecord
		class, basis string
	}{
		{"", nil, EffectWriteIrreversible, "undeclared"},
		{"", []TrafficRecord{get}, EffectWriteIrreversible, "undeclared"},
		{EffectRead, nil, EffectRead, "declared"},
		{EffectRead, []TrafficRecord{get}, EffectRead, "declared"},
		{EffectRead, []TrafficRecord{get, post}, EffectWriteIrreversible, "observed-traffic"},
		{EffectRead, []TrafficRecord{form}, EffectWriteIrreversible, "observed-traffic"},
		{EffectRead, []TrafficRecord{head}, EffectWriteIrreversible, "observed-traffic"},
		{EffectWriteReversible, []TrafficRecord{post}, EffectWriteReversible, "declared"},
		{EffectWriteIrreversible, []TrafficRecord{get}, EffectWriteIrreversible, "declared"},
		{EffectExternal, []TrafficRecord{get, form}, EffectExternal, "declared"},
	}
	for _, c := range cases {
		class, basis := effectOf(c.declared, c.observed)
		if class != c.class || basis != c.basis || effectRank[class] < effectRank[c.declared] {
			t.Errorf("%q %v: %s %s, want %s %s", c.declared, c.observed, class, basis, c.class, c.basis)
		}
	}
	for _, line := range []string{
		`{"schema":"application-flow-traffic/0","flow_id":"a","step_id":"b","method":"post","form_submit":false}`,
		`{"schema":"application-flow-traffic/1","flow_id":"a","step_id":"b","method":"POST","form_submit":false}`,
		`{"schema":"application-flow-traffic/0","flow_id":"a","step_id":"b","method":"POST"}` + `{}`,
	} {
		if _, err := decodeTraffic([]byte(line)); err == nil {
			t.Errorf("traffic %s accepted", line)
		}
	}
}

// AFU-V1-028: the packet is bounded; more than 256 steps along the precondition chain is a coded refusal.
func TestAFUV1028PacketBound(t *testing.T) {
	ctx := context.Background()
	big := navSample("checkout", "sign-in")
	big.Variations, big.Links = []FlowVariation{}, []FlowLink{}
	for i := len(big.Steps); i < maxFlowList; i++ {
		big.Steps = append(big.Steps, FlowStep{StepID: fmt.Sprintf("s%d", i), Action: "step"})
	}
	root := navRepo(t, big, navSample("sign-in"))
	set, err := LoadIntentsAt(ctx, root, "flows", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = FlowNavigationPacket(ctx, root, set, nil, nil, "sign-in", EffectRead); err != nil {
		t.Fatalf("small packet: %v", err)
	}
	if _, err = FlowNavigationPacket(ctx, root, set, nil, nil, "checkout", EffectRead); codeOf(err) != "navigation-bound-exceeded" {
		t.Fatalf("oversized packet: %v", err)
	}
	if _, err = FlowNavigationPacket(ctx, root, set, nil, nil, "sign-in", "write"); err == nil {
		t.Fatal("unknown --max-effect accepted")
	}
}

// AFU-V1-029: the observer gate admits a write-irreversible, external or unknown-class transition
// only against an origin listed as disposable in the committed origins.json of the flows directory.
func TestAFUV1029ObserverRefusesNonDisposableOrigin(t *testing.T) {
	ctx := context.Background()
	root := navRepo(t, navSample("checkout"))
	absent, err := LoadOriginsAt(ctx, root, "flows", "HEAD")
	if err != nil || len(absent.Disposable) != 0 {
		t.Fatalf("absent origins.json: %v %+v", err, absent)
	}
	if codeOf(AdmitTransition(absent, "http://127.0.0.1:3000", EffectWriteIrreversible)) != "observer-origin-not-disposable" {
		t.Fatal("irreversible transition admitted with no origins.json")
	}
	writeRaw(t, root, "flows/origins.json", []byte(`{"schema":"application-flow-origins/0","disposable":["http://127.0.0.1:3000"]}`))
	if uncommitted, err := LoadOriginsAt(ctx, root, "flows", "HEAD"); err != nil || len(uncommitted.Disposable) != 0 {
		t.Fatalf("an uncommitted origins.json was honored: %v %+v", err, uncommitted)
	}
	commitAll(t, root, "origins")
	o, err := LoadOriginsAt(ctx, root, "flows", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		origin, class string
		admitted      bool
	}{
		{"http://127.0.0.1:3000", EffectWriteIrreversible, true},
		{"http://127.0.0.1:3000", EffectExternal, true},
		{"http://127.0.0.1:3000", "write", true},
		{"https://shop.example", EffectWriteIrreversible, false},
		{"https://shop.example", EffectExternal, false},
		{"https://shop.example", "", false},
		{"http://127.0.0.1:3000/", EffectExternal, false},
		{"HTTP://127.0.0.1:3000", EffectExternal, false},
		{"https://shop.example", EffectRead, true},
		{"https://shop.example", EffectWriteReversible, true},
	}
	for _, c := range cases {
		err := AdmitTransition(o, c.origin, c.class)
		if (err == nil) != c.admitted || (err != nil && codeOf(err) != "observer-origin-not-disposable") {
			t.Errorf("%s %q: %v", c.origin, c.class, err)
		}
	}
	for _, body := range []string{
		`{"schema":"application-flow-origins/0","disposable":["http://127.0.0.1:3000/path"]}`,
		`{"schema":"application-flow-origins/0","disposable":["ftp://127.0.0.1"]}`,
		`{"schema":"application-flow-origins/0","disposable":["http://a.test","http://a.test"]}`,
		`{"schema":"application-flow-origins/0","disposable":["http://user@a.test"]}`,
		`{"schema":"application-flow-origins/1","disposable":[]}`,
		`{"schema":"application-flow-origins/0","disposable":[],"extra":true}`,
	} {
		writeRaw(t, root, "flows/origins.json", []byte(body))
		commitAll(t, root, "malformed origins")
		if _, err := LoadOriginsAt(ctx, root, "flows", "HEAD"); err == nil {
			t.Errorf("origins.json %s accepted", body)
		}
	}
}
