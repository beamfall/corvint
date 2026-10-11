package appflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
)

func navSample(id string, pre ...string) FlowIntent {
	f := sampleIntent(id)
	f.Navigation = &FlowNavigation{PreconditionFlows: append([]string{}, pre...), Steps: []NavStep{
		{StepID: "open", State: "/cart", Locator: NavLocator{Role: "link", Name: "Cart"}, Expect: []string{}, Effect: EffectRead},
		{StepID: "pay", State: "/cart", Locator: NavLocator{TestID: "pay"}, Ready: &NavLocator{TestID: "cart"}, Expect: []string{"paid"}},
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

// AFU-V1-025: link review is step-scoped, conservative, and independent of test verification.
func TestAFUV1025StepReviewState(t *testing.T) {
	f := navSample("checkout")
	entry := f.Navigation.Steps[1]
	cases := []struct {
		name  string
		links []EvaluatedLink
		want  string
	}{
		{"no links", nil, ""},
		{"unrelated step", []EvaluatedLink{{From: "open", ReviewState: ReviewStale}}, ""},
		{"direct stale", []EvaluatedLink{{From: "pay", ReviewState: ReviewStale}}, ReviewStale},
		{"variation unreviewed", []EvaluatedLink{{From: "checkout.happy", ReviewState: ReviewUnreviewed}}, ReviewUnreviewed},
		{"outcome stale", []EvaluatedLink{{From: "paid", ReviewState: ReviewStale}}, ReviewStale},
		{"inferred only", []EvaluatedLink{{From: "pay", ReviewState: ReviewInferred}}, ReviewInferred},
		{"reviewed plus inferred", []EvaluatedLink{{From: "pay", ReviewState: ReviewInferred}, {From: "paid", ReviewState: ReviewReviewed}}, ReviewReviewed},
		{"unreviewed before reviewed", []EvaluatedLink{{From: "pay", ReviewState: ReviewUnreviewed}, {From: "paid", ReviewState: ReviewReviewed}}, ReviewUnreviewed},
		{"stale wins", []EvaluatedLink{{From: "pay", ReviewState: ReviewUnreviewed}, {From: "paid", ReviewState: ReviewStale}}, ReviewStale},
	}
	for _, state := range []string{ReviewNotAncestor, ReviewNotIntentChange, ReviewAnchorUnknown, ReviewLinkNotAtAnchor, ReviewEvidenceUnavailable, ReviewTargetUnpinned} {
		cases = append(cases, struct {
			name  string
			links []EvaluatedLink
			want  string
		}{state, []EvaluatedLink{{From: "pay", ReviewState: state}}, ReviewUnreviewed})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stepReviewState(f, "pay", entry, tc.links); got != tc.want {
				t.Fatalf("review state = %q, want %q", got, tc.want)
			}
			// Aggregation does not depend on link declaration order.
			for i, j := 0, len(tc.links)-1; i < j; i, j = i+1, j-1 {
				tc.links[i], tc.links[j] = tc.links[j], tc.links[i]
			}
			if got := stepReviewState(f, "pay", entry, tc.links); got != tc.want {
				t.Fatalf("reversed review state = %q, want %q", got, tc.want)
			}
		})
	}
	// A variation's outcome supports the step even when navigation expect is empty.
	if got := stepReviewState(f, "open", f.Navigation.Steps[0], []EvaluatedLink{{From: "paid", ReviewState: ReviewStale}}); got != ReviewStale {
		t.Fatalf("variation-only outcome review = %q", got)
	}
	// Expected outcomes remain supporting links even outside a variation.
	f.Variations = nil
	if got := stepReviewState(f, "pay", entry, []EvaluatedLink{{From: "paid", ReviewState: ReviewUnreviewed}}); got != ReviewUnreviewed {
		t.Fatalf("navigation outcome review = %q", got)
	}
	// Old packets still decode and an absent review result adds no wire member.
	var old Transition
	if err := json.Unmarshal([]byte(`{"flow_id":"checkout","step_id":"pay","verification":"verified"}`), &old); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(old)
	if err != nil || strings.Contains(string(raw), "review_state") || old.Verification != "verified" {
		t.Fatalf("optional field compatibility: %s %v", raw, err)
	}
}

// AFU-V1-025: packets carry unreviewed step, variation and recovery links without granting trust.
func TestAFUV1025PacketReviewState(t *testing.T) {
	f := navSample("checkout")
	f.Steps = append(f.Steps, FlowStep{StepID: "recover", Action: "dismiss error"})
	f.Navigation.Steps[1].Recovery = "recover"
	f.Navigation.Steps = append(f.Navigation.Steps, NavStep{StepID: "recover", State: "/cart", Locator: NavLocator{TestID: "dismiss"}, Expect: []string{}, Effect: EffectRead})
	f.Links = append(f.Links, FlowLink{From: "recover", Basis: "declared", Target: LinkTarget{Type: "source", Path: "app.js", StartLine: 1, EndLine: 1}})
	root := navRepo(t, f)
	set, err := LoadIntentsAt(context.Background(), root, "flows", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := FlowNavigationPacket(context.Background(), root, set, nil, nil, "checkout", EffectRead, "")
	if err != nil {
		t.Fatal(err)
	}
	var packet NavigationPacket
	if err := json.Unmarshal(raw, &packet); err != nil {
		t.Fatal(err)
	}
	if len(packet.Steps) != 2 || len(packet.Recovery) != 1 {
		t.Fatalf("packet shape: %+v", packet)
	}
	for _, step := range append(packet.Steps, packet.Recovery...) {
		if step.ReviewState != ReviewUnreviewed || step.Verification != "unverified" {
			t.Errorf("step review: %+v", step)
		}
	}
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
		if _, err = FlowNavigationMap(ctx, root, set, nil, nil, ""); err == nil || !strings.Contains(err.Error(), "precondition") {
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
		"earlier recovery":   func(n *FlowNavigation) { n.Steps[1].Recovery = "open" },
		"path recovery":      func(n *FlowNavigation) { n.Steps[0].Recovery = "pay" },
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
	if _, err = FlowNavigationPacket(ctx, root, set, nil, nil, "sign-in", EffectRead, ""); err != nil {
		t.Fatalf("small packet: %v", err)
	}
	if _, err = FlowNavigationPacket(ctx, root, set, nil, nil, "checkout", EffectRead, ""); codeOf(err) != "navigation-bound-exceeded" {
		t.Fatalf("oversized packet: %v", err)
	}
	if _, err = FlowNavigationPacket(ctx, root, set, nil, nil, "sign-in", "write", ""); err == nil {
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

// AFU-V1-057: duplicates and empty files count; refusal precedes even an invalid first file.
func TestAFUV1057TrafficFileBound(t *testing.T) {
	empty := filepath.Join(t.TempDir(), "empty.jsonl")
	if err := os.WriteFile(empty, nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, count := range []int{0, maxTrafficFiles - 1, maxTrafficFiles} {
		files := make([]string, count)
		for i := range files {
			files[i] = empty
		}
		got, err := ReadTraffic(files)
		if err != nil || len(got) != 0 {
			t.Fatalf("%d files: %v %v", count, got, err)
		}
	}
	files := make([]string, maxTrafficFiles+1)
	for i := range files {
		files[i] = empty
	}
	files[0] = filepath.Join(t.TempDir(), "missing.jsonl")
	got, err := ReadTraffic(files)
	if got != nil || codeOf(err) != "navigation-bound-exceeded" || !strings.Contains(err.Error(), "traffic files") {
		t.Fatalf("file count must refuse before reading: %v %v", got, err)
	}
}

// AFU-V1-057: the shared 8,192-record limit remains inclusive across multiple files.
func TestAFUV1057TrafficRecordBound(t *testing.T) {
	root := t.TempDir()
	line := `{"schema":"application-flow-traffic/0","flow_id":"checkout","step_id":"open","method":"GET","form_submit":false}` + "\n"
	first, last := filepath.Join(root, "first.jsonl"), filepath.Join(root, "last.jsonl")
	if err := os.WriteFile(first, []byte(strings.Repeat(line, maxRunRecords-1)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(last, []byte(line), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadTraffic([]string{first, last})
	if err != nil || len(got) != maxRunRecords {
		t.Fatalf("record boundary: %d %v", len(got), err)
	}
	got, err = ReadTraffic([]string{first, last, last})
	if got != nil || codeOf(err) != "navigation-bound-exceeded" {
		t.Fatalf("excess records: %v %v", got, err)
	}
}

// AFU-V1-058: exact-pair indexing matches the old filter and does not alter caller records.
func TestAFUV1058TrafficIndexParity(t *testing.T) {
	traffic := []TrafficRecord{
		{FlowID: "checkout", StepID: "open", Method: "GET"},
		{FlowID: "other", StepID: "open", Method: "POST"},
		{FlowID: "checkout", StepID: "pay", Method: "POST"},
		{FlowID: "checkout", StepID: "open", Method: "GET", FormSubmit: true},
		{FlowID: "checkout", StepID: "open", Method: "POST"},
		{FlowID: "unknown", StepID: "unknown", Method: "POST"},
	}
	before := slices.Clone(traffic)
	index := indexTraffic(traffic)
	for _, id := range []string{"checkout", "other", "unknown", "absent"} {
		intent := navSample(id)
		got := flowTransitions(intent, nil, nil, index, head{})
		for i, step := range intent.Steps {
			observed := slices.DeleteFunc(slices.Clone(traffic), func(r TrafficRecord) bool { return r.FlowID != id || r.StepID != step.StepID })
			if !slices.Equal(index[trafficStep{id, step.StepID}], observed) {
				t.Fatalf("pair %s/%s lost order or isolation", id, step.StepID)
			}
			want := transition(intent, step, intent.Navigation.Steps[i], observed, false)
			if !reflect.DeepEqual(got[i], want) {
				t.Fatalf("pair %s/%s: got %+v want %+v", id, step.StepID, got[i], want)
			}
		}
	}
	if !slices.Equal(traffic, before) {
		t.Fatal("caller traffic mutated")
	}
	if len(indexTraffic(nil)) != 0 {
		t.Fatal("empty traffic indexed")
	}
}
