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
	}}
	obs, _ := observeOnce(t, m, d, Options{})
	tr := obs.Traces[0]
	if tr.Visits[0].Screen != screenID("shop", "item") || tr.Visits[3].Screen != screenID("shop", "itemModal") {
		t.Fatal(tr.Visits)
	}
	if tr.Edges[0].Trigger != "redirect" || tr.Edges[1].Trigger != "click" || tr.Edges[1].Selector != "#go" {
		t.Fatal(tr.Edges)
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
