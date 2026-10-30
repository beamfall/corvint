package appmap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func observeMap(hash string) *Map {
	mk := func(state, tpl string) Screen {
		t, _ := normalizeTemplate(tpl)
		return Screen{ID: screenID("shop", state), State: state, Status: StatusResolved, Template: t, Key: templateKey(t), Params: []string{}}
	}
	return &Map{Schema: MapSchema, App: "shop", Revision: "abc", HashPrefix: hash, Digest: "sha256:m",
		Screens: []Screen{mk("login", "/login"), mk("home", "/home"), mk("item", "/items/:id"), mk("itemModal", "/items/:id/edit"), mk("dup1", "/dup"), mk("dup2", "/dup")}}
}

func observeOnce(t *testing.T, m *Map, d NavigationEvents, o Options) (Observation, []byte) {
	t.Helper()
	raw, _ := json.Marshal(d)
	out, err := Observe(m, []NavigationEvents{d}, [][]byte{raw}, "abc", o)
	if err != nil {
		t.Fatal(err)
	}
	var obs Observation
	if err = json.Unmarshal(out, &obs); err != nil {
		t.Fatal(err)
	}
	return obs, out
}

// ANL-V0-006: guard redirect, modal route, ambiguous and unmapped URLs; static nodes untouched.
func TestANLV0006Observe(t *testing.T) {
	m := observeMap("")
	before, _ := json.Marshal(m)
	d := NavigationEvents{Schema: NavigationEventsSchema, TestKey: "t1", Events: []NavigationEvent{
		{Kind: "navigate", URL: "https://x.test/items/7", Action: &NavigationAction{Kind: "goto"}},
		{Kind: "redirect", URL: "https://x.test/login"},
		{Kind: "navigate", URL: "https://x.test/home", Action: &NavigationAction{Kind: "click", Selector: "#go"}},
		{Kind: "navigate", URL: "https://x.test/items/7/edit", Action: &NavigationAction{Kind: "click", Selector: "#edit"}},
		{Kind: "navigate", URL: "https://x.test/dup"},
		{Kind: "navigate", URL: "https://x.test/nowhere"},
		{Kind: "navigate", URL: "https://x.test/home", Action: &NavigationAction{Kind: "hover"}},
	}}
	obs, _ := observeOnce(t, m, d, Options{})
	tr := obs.Traces[0]
	if tr.Visits[0].Screen != screenID("shop", "item") || tr.Visits[3].Screen != screenID("shop", "itemModal") {
		t.Fatal(tr.Visits)
	}
	if tr.Edges[0].Trigger != "redirect" || tr.Edges[1].Trigger != "click" || tr.Edges[1].Selector != "#go" {
		t.Fatal(tr.Edges)
	}
	if tr.Edges[3].Trigger != StatusUnknown || tr.Edges[5].Trigger != StatusUnknown {
		t.Fatal("trigger invented without action evidence", tr.Edges)
	}
	if v := tr.Visits[4]; v.Status != StatusUnknown || len(v.Candidates) != 2 {
		t.Fatal(v)
	}
	if v := tr.Visits[5]; v.Status != UnmappedURL || tr.Edges[4].Status != StatusUnknown {
		t.Fatal(v, tr.Edges[4])
	}
	if after, _ := json.Marshal(m); string(after) != string(before) || obs.Authority != AuthorityLearned {
		t.Fatal("static map changed")
	}
}

func TestANLV0006HashRouted(t *testing.T) {
	m := observeMap("#!")
	d := NavigationEvents{Schema: NavigationEventsSchema, TestKey: "t", Events: []NavigationEvent{
		{Kind: "navigate", URL: "https://x.test/#!/login"}, {Kind: "navigate", URL: "https://x.test/#!/home?tab=1"}}}
	obs, _ := observeOnce(t, m, d, Options{})
	if obs.Traces[0].Visits[0].Screen != screenID("shop", "login") || obs.Traces[0].Visits[1].Screen != screenID("shop", "home") {
		t.Fatal(obs.Traces[0].Visits)
	}
}

// ANL-V0-007: secret-shaped query values and typed input values never reach the output.
func TestANLV0007Redaction(t *testing.T) {
	m := observeMap("#!")
	d := NavigationEvents{Schema: NavigationEventsSchema, TestKey: "t", Events: []NavigationEvent{
		{Kind: "navigate", URL: "https://user:pw@x.test/login?next=%2Fhome&access_token=sekret1&x=" + strings.Repeat("a", 30)},
		{Kind: "navigate", URL: "https://x.test/#!/home?apikey=hunter2", Action: &NavigationAction{Kind: "fill", Value: "p4ssw0rd-typed"}}}}
	_, out := observeOnce(t, m, d, Options{})
	for _, leak := range []string{"sekret1", "hunter2", "p4ssw0rd", "user:pw", strings.Repeat("a", 30)} {
		if strings.Contains(string(out), leak) {
			t.Fatalf("leaked %s: %s", leak, out)
		}
	}
	if !strings.Contains(string(out), "next=") {
		t.Fatal("ordinary query dropped", out)
	}
}

// ANL-V0-008: deterministic digest, byte cap with omitted counts, closed input.
func TestANLV0008InputsAndBudget(t *testing.T) {
	m := observeMap("")
	var evs []NavigationEvent
	for i := 0; i < 80; i++ {
		evs = append(evs, NavigationEvent{Kind: "navigate", URL: "https://x.test/home"})
	}
	d := NavigationEvents{Schema: NavigationEventsSchema, TestKey: "t", Events: evs}
	a, _ := observeOnce(t, m, d, Options{})
	b, _ := observeOnce(t, m, d, Options{})
	if a.Digest != b.Digest || a.Digest == "" {
		t.Fatal("digest not deterministic")
	}
	small, out := observeOnce(t, m, d, Options{Budget: 1024})
	if len(out) > 1024 || small.Omitted["edges"] == 0 || small.Digest != a.Digest {
		t.Fatal(len(out), small.Omitted)
	}
	dir := t.TempDir()
	write := func(n, body string) {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("a.json", `{"schema":"`+NavigationEventsSchema+`","test_key":"k","events":[{"kind":"navigate","url":"/home"}]}`)
	if docs, _, err := LoadNavigationEvents(dir); err != nil || len(docs) != 1 {
		t.Fatal(err)
	}
	write("b.json", `{"schema":"`+NavigationEventsSchema+`","test_key":"k","events":[],"extra":1}`)
	if _, _, err := LoadNavigationEvents(dir); err == nil {
		t.Fatal("unknown field accepted")
	}
	os.Remove(filepath.Join(dir, "b.json"))
	write("c.zip", "PK")
	if _, _, err := LoadNavigationEvents(dir); err == nil {
		t.Fatal("zip accepted")
	}
}

// ANL-V0-021: observed action kinds map to typed triggers; an annotation is echoed as a declared
// trigger and never replaces UNKNOWN; an annotation outside the vocabulary is refused.
func TestANLV0021TypedTriggersAndAnnotations(t *testing.T) {
	m := observeMap("")
	nav := func(kind, url string) NavigationEvent {
		return NavigationEvent{Kind: "navigate", URL: "https://x.test" + url, Action: &NavigationAction{Kind: kind}}
	}
	d := NavigationEvents{Schema: NavigationEventsSchema, TestKey: "t", Events: []NavigationEvent{
		nav("goto", "/login"), nav("press", "/home"), nav("goBack", "/login"), nav("goForward", "/home"),
		nav("reload", "/home"), nav("submit", "/login"), nav("fill", "/home"),
		{Kind: "navigate", URL: "https://x.test/login", Annotation: &NavigationAnnotation{Trigger: "form submit"}},
		{Kind: "navigate", URL: "https://x.test/home", Action: &NavigationAction{Kind: "press"}, Annotation: &NavigationAnnotation{Trigger: "form submit"}},
	}}
	obs, _ := observeOnce(t, m, d, Options{})
	want := []string{"key press", "history back", "history forward", "reload", "form submit", StatusUnknown, StatusUnknown, "key press"}
	for i, e := range obs.Traces[0].Edges {
		if e.Trigger != want[i] {
			t.Fatalf("edge %d trigger %q want %q", i, e.Trigger, want[i])
		}
	}
	if e := obs.Traces[0].Edges[6]; e.DeclaredTrigger != "form submit" || e.Trigger != StatusUnknown {
		t.Fatal("annotation replaced the observed trigger", e)
	}
	dir := t.TempDir()
	f := filepath.Join(dir, "a.json")
	for body, ok := range map[string]bool{
		`{"schema":"` + NavigationEventsSchema + `","test_key":"k","events":[{"kind":"navigate","url":"/home","annotation":{"trigger":"reload"}}]}`:  true,
		`{"schema":"` + NavigationEventsSchema + `","test_key":"k","events":[{"kind":"navigate","url":"/home","annotation":{"trigger":"UNKNOWN"}}]}`: false,
		`{"schema":"` + NavigationEventsSchema + `","test_key":"k","events":[{"kind":"navigate","url":"/home","annotation":{"trigger":"tap"}}]}`:     false,
	} {
		if err := os.WriteFile(f, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := LoadNavigationEvents(f); (err == nil) != ok {
			t.Fatal(body, err)
		}
	}
}

// ANL-V0-073: a fragment without '?' that carries key=value pairs or an opaque token is redacted
// like a query; a route-only fragment keeps its bytes and its screen.
func TestANLV0073FragmentRedaction(t *testing.T) {
	cases := map[string]string{
		"https://x.test/cb#access_token=SECRETVALUE1&token_type=bearer": "https://x.test/cb#access_token=REDACTED&token_type=REDACTED",
		"https://x.test/cb#state=1&id_token=" + strings.Repeat("b", 30): "https://x.test/cb#id_token=REDACTED&state=1",
		"https://x.test/cb#" + strings.Repeat("c", 40):                  "https://x.test/cb#REDACTED",
		"https://x.test/cb#a=1;b=SECRETVALUE2":                          "https://x.test/cb#_=REDACTED",
		"https://x.test/#/home":                                         "https://x.test/#/home",
		"https://x.test/#/settings/profile/security-keys":               "https://x.test/#/settings/profile/security-keys",
		"https://x.test/#/r=1":                                          "https://x.test/#/r=1",
		"https://x.test/items/7#tab":                                    "https://x.test/items/7#tab",
	}
	for raw, want := range cases {
		if got := redactURL(raw); got != want {
			t.Errorf("redactURL(%s) = %s, want %s", raw, got, want)
		}
	}
	m := observeMap("#")
	d := NavigationEvents{Schema: NavigationEventsSchema, TestKey: "t", Events: []NavigationEvent{
		{Kind: "navigate", URL: "https://x.test/#/home"},
		{Kind: "navigate", URL: "https://x.test/cb#access_token=SECRETVALUE1"}}}
	obs, out := observeOnce(t, m, d, Options{})
	if strings.Contains(string(out), "SECRETVALUE") {
		t.Fatalf("fragment token leaked: %s", out)
	}
	if v := obs.Traces[0].Visits[0]; v.Screen != screenID("shop", "home") || v.URL != "https://x.test/#/home" {
		t.Fatalf("route-only fragment visit changed: %+v", v)
	}
}

// ANL-V0-008 (V1-3027): trimming to the budget marshals the observation a bounded number of times,
// not once per dropped item, and its bytes equal the one-item-at-a-time reference trim.
func TestANLV0008BudgetTrimIsLinear(t *testing.T) {
	m := observeMap("")
	var docs []NavigationEvents
	var raws [][]byte
	for d := 0; d < 2; d++ {
		evs := make([]NavigationEvent, 0, 120)
		for i := 0; i < 120; i++ {
			u := "https://x.test/items/" + strings.Repeat("7", 1+i%13)
			if i%3 == 0 {
				u = "https://x.test/home"
			}
			evs = append(evs, NavigationEvent{Kind: "navigate", URL: u})
		}
		doc := NavigationEvents{Schema: NavigationEventsSchema, TestKey: "t" + strings.Repeat("k", d), Events: evs}
		raw, _ := json.Marshal(doc)
		docs, raws = append(docs, doc), append(raws, raw)
	}
	for _, budget := range []int{512, 1024, 4096, 20000, 40000, 65536} {
		got, err := Observe(m, docs, raws, "abc", Options{Budget: budget})
		if err != nil {
			t.Fatal(budget, err)
		}
		marshals := observeMarshals
		want := referenceTrim(t, m, docs, raws, budget)
		if string(got) != string(want) {
			t.Fatalf("budget %d: trimmed bytes differ from the reference\n got %d bytes\nwant %d bytes", budget, len(got), len(want))
		}
		if marshals > 3 {
			t.Fatalf("budget %d: %d whole-observation marshals, want at most 3", budget, marshals)
		}
	}
	// 50,000 items over the smallest budgets still costs three whole-observation marshals.
	big := make([]NavigationEvent, 4096)
	for i := range big {
		big[i] = NavigationEvent{Kind: "navigate", URL: "https://x.test/home"}
	}
	docs, raws = nil, nil
	for d := 0; d < 7; d++ {
		doc := NavigationEvents{Schema: NavigationEventsSchema, TestKey: "big" + strings.Repeat("k", d), Events: big}
		raw, _ := json.Marshal(doc)
		docs, raws = append(docs, doc), append(raws, raw)
	}
	if out, err := Observe(m, docs, raws, "abc", Options{Budget: 4096}); err != nil || len(out) > 4096 || observeMarshals > 3 {
		t.Fatalf("57,337 items: %d bytes, %d marshals, %v", len(out), observeMarshals, err)
	}
	if _, err := Observe(m, docs, raws, "abc", Options{Budget: 256}); err == nil || !strings.Contains(err.Error(), "observation header exceeds --budget") {
		t.Fatalf("header error = %v", err)
	}
}

// referenceTrim is the pre-V1-3027 trim: drop one trailing edge or visit, re-marshal, repeat.
func referenceTrim(t *testing.T, m *Map, docs []NavigationEvents, raws [][]byte, budget int) []byte {
	t.Helper()
	full, err := Observe(m, docs, raws, "abc", Options{Budget: 65536})
	if err != nil {
		t.Fatal(err)
	}
	var obs Observation
	if err := json.Unmarshal(full, &obs); err != nil {
		t.Fatal(err)
	}
	for {
		raw, _ := json.Marshal(obs)
		raw = append(raw, '\n')
		if len(raw) <= budget {
			return raw
		}
		trimmed := false
		for j := len(obs.Traces) - 1; j >= 0 && !trimmed; j-- {
			tr := &obs.Traces[j]
			switch {
			case len(tr.Edges) > 0:
				tr.Edges = tr.Edges[:len(tr.Edges)-1]
				obs.Omitted["edges"]++
				trimmed = true
			case len(tr.Visits) > 0:
				tr.Visits = tr.Visits[:len(tr.Visits)-1]
				obs.Omitted["visits"]++
				trimmed = true
			}
		}
		if !trimmed {
			t.Fatal("reference: header exceeds budget")
		}
	}
}
