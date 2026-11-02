package appmap

import (
	"testing"
)

// AMAP-V0-881 (V1-3954): the persisted map records each screen's route dialect, so every reader
// that rebuilds a screen index from the map ranks a React URL match as the builder did; a map an
// earlier builder wrote, with no dialect, reads unranked as before.
func TestAMAPV0881ReadersRankAsBuilder_V13954(t *testing.T) {
	built, _ := v13932Repo(t)
	raw, err := Encode(built)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseMap(raw)
	if err != nil {
		t.Fatal(err)
	}
	byTemplate := map[string]string{}
	for _, s := range m.Screens {
		if s.Dialect != DialectReact {
			t.Fatalf("%s: dialect %q, want %s", s.ID, s.Dialect, DialectReact)
		}
		byTemplate[s.Template] = s.ID
	}
	// The builder's own placement is the spec attribution its goto gave.
	builder := func(template, spec string) string {
		for _, s := range built.Screens {
			for _, a := range s.Specs {
				if a.File == spec && a.Basis == "goto" && s.Template == template {
					return s.ID
				}
			}
		}
		return ""
	}
	admission, kindred := "apps/web/e2e/journey-admission.spec.ts", "apps/web/e2e/journey-kindred.spec.ts"
	cases := []struct{ url, template, spec string }{
		{"/new", "/new", admission},
		{"/dynamic/new", "/dynamic/new", admission},
		{"/dynamic/42", "/dynamic/{id}", kindred},
		{"/kindred", "/{*splat}", kindred},
	}
	resolver := &uiResolver{m: m, x: newScreenIndex(m.HashPrefix, m.BasePath, m.Screens), byID: map[string]*Screen{}}
	for i := range m.Screens {
		resolver.byID[m.Screens[i].ID] = &m.Screens[i]
	}
	events := NavigationEvents{Schema: NavigationEventsSchema, TestKey: "k"}
	for _, c := range cases {
		events.Events = append(events.Events, NavigationEvent{Kind: "navigate", URL: "http://localhost:5173" + c.url})
	}
	obs, _ := observeOnce(t, m, events, Options{})
	for n, c := range cases {
		want := byTemplate[c.template]
		if got := builder(c.template, c.spec); got != want || want == "" {
			t.Fatalf("%s: builder placed %q, want %q", c.url, got, want)
		}
		if got, why := newScreenIndex(m.HashPrefix, m.BasePath, m.Screens).byURL(c.url); got != want {
			t.Fatalf("%s: rebuilt index %q %q, want %q", c.url, got, why, want)
		}
		if got, why, _ := m.findScreen(c.url); got != want {
			t.Fatalf("%s: find %q %q, want %q", c.url, got, why, want)
		}
		if v := obs.Traces[0].Visits[n]; v.Screen != want {
			t.Fatalf("%s: observe %+v, want %q", c.url, v, want)
		}
		if got := resolver.screenFor("http://localhost:5173" + c.url); got.ID != want {
			t.Fatalf("%s: ui records %+v, want %q", c.url, got, want)
		}
	}
	// A map an earlier builder wrote carries no dialect: its readers keep the unranked reading.
	old := *m
	old.Screens = append([]Screen{}, m.Screens...)
	for i := range old.Screens {
		old.Screens[i].Dialect = ""
	}
	if old.Digest, err = mapDigest(&old); err != nil {
		t.Fatal(err)
	}
	raw, err = Encode(&old)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := ParseMap(raw)
	if err != nil {
		t.Fatalf("a map without dialects must still parse: %v", err)
	}
	if got, why, _ := legacy.findScreen("/new"); got != "" || why != "ambiguous-template" {
		t.Fatalf("legacy find /new: %q %q, want ambiguous-template", got, why)
	}
}
