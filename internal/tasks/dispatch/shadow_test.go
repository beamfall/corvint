//go:build darwin || linux

package dispatch

import (
	"encoding/json"
	"slices"
	"testing"
)

// shadowConfig is the issue 994 repro: a broad role worker (priority 50)
// and a narrower role special (priority 92), both on work state "work".
func shadowConfig(t *testing.T) *Config {
	t.Helper()
	c := testConfig(t, "exit 0")
	c.WorkState = &WorkState{Kind: "status-line", Path: "/work/{ticketLocal}", Key: "state"}
	c.Roles = []Role{
		{Name: "worker", Host: "sh", Cap: 1, Priority: 50, Match: &Match{States: []string{"work"}}, Prompt: "p", IdleSeconds: 30, WallSeconds: 60},
		{Name: "special", Host: "sh", Cap: 1, Priority: 92, Match: &Match{IDGlob: "T-001[0-4]", States: []string{"work"}}, Prompt: "p", IdleSeconds: 30, WallSeconds: 60},
	}
	return c
}

// CAL-V0-290: the issue 994 configuration still loads, reports special as
// shadowed by worker, and the roster still launches T-0011 under worker.
func TestCALV0290_ShadowedRoleWarnsWithoutChangingOrder(t *testing.T) {
	c := shadowConfig(t)
	raw, _ := json.Marshal(c)
	loaded, err := DecodeConfig(raw)
	if err != nil {
		t.Fatalf("a shadowed role refused the configuration: %v", err)
	}
	want := []RoleShadow{{Role: "special", ShadowedBy: "worker", Priority: 92, ShadowedByPriority: 50}}
	if got := loaded.RoleShadows(); !slices.Equal(got, want) {
		t.Fatalf("RoleShadows = %+v, want %+v", got, want)
	}
	tk := ticket("T-0011", "P2", 1)
	tk.State = "work"
	got := Roster(loaded, &Observation{Tickets: []Ticket{tk}}, nil, nil)
	if len(got) != 1 || got[0].Role != "worker" {
		t.Fatalf("roster order changed: %+v", got)
	}
	// With worker at its cap the shadowed role still takes the ticket.
	got = Roster(loaded, &Observation{Tickets: []Ticket{tk}}, []Busy{{Role: "worker", Key: "other", Slot: 1}}, nil)
	if len(got) != 1 || got[0].Role != "special" {
		t.Fatalf("shadowed role could not take a ticket its shadow had no slot for: %+v", got)
	}
	// Correctly ordered: the narrower role wins, so nothing is shadowed.
	loaded.Roles[1].Priority = 10
	if s := loaded.RoleShadows(); len(s) != 0 {
		t.Fatalf("correctly ordered roles reported %+v", s)
	}
	if got := Roster(loaded, &Observation{Tickets: []Ticket{tk}}, nil, nil); len(got) != 1 || got[0].Role != "special" {
		t.Fatalf("roster = %+v", got)
	}
}

// CAL-V0-290: the coverage check reports only provable supersets. Each case
// edits the repro; covered says whether special must still be reported.
func TestCALV0290_ShadowCoverageIsConservative(t *testing.T) {
	cases := map[string]struct {
		edit    func(w, s *Role)
		covered bool
	}{
		"repro":                   {func(w, s *Role) {}, true},
		"equal priority":          {func(w, s *Role) { s.Priority = 50 }, false},
		"shadow disabled":         {func(w, s *Role) { w.Cap = 0 }, false},
		"shadowed disabled":       {func(w, s *Role) { s.Cap = 0 }, false},
		"different state":         {func(w, s *Role) { s.Match.States = []string{"review"} }, false},
		"state subset":            {func(w, s *Role) { w.Match.States = []string{"work", "review"} }, true},
		"shadowed admits UNKNOWN": {func(w, s *Role) { s.Match.States = nil }, false},
		"shadow admits any state": {func(w, s *Role) { w.Match.States = nil }, true},
		"exclusions narrower": {func(w, s *Role) {
			w.Match.States, w.Match.ExcludeStates, s.Match.States, s.Match.ExcludeStates = nil, []string{"done"}, nil, []string{"done", "hold"}
		}, true},
		"exclusions wider": {func(w, s *Role) {
			w.Match.States, w.Match.ExcludeStates, s.Match.States, s.Match.ExcludeStates = nil, []string{"done", "hold"}, nil, []string{"done"}
		}, false},
		"shadow excludes UNKNOWN": {func(w, s *Role) {
			w.Match.States, w.Match.ExcludeStates, s.Match.States, s.Match.ExcludeStates = nil, []string{StateUnknown}, nil, []string{"done"}
		}, true},
		"exclusion vs state list":    {func(w, s *Role) { s.Match.States, s.Match.ExcludeStates = nil, []string{"done"} }, false},
		"excluded listed state":      {func(w, s *Role) { s.Match.States, s.Match.ExcludeStates = []string{"work", "hold"}, []string{"hold"} }, true},
		"shadow narrower glob":       {func(w, s *Role) { w.Match.IDGlob = "T-0010" }, false},
		"different glob":             {func(w, s *Role) { w.Match.IDGlob = "T-*" }, false},
		"literal inside glob":        {func(w, s *Role) { w.Match.IDGlob, s.Match.IDGlob = "T-001[0-4]", "T-0011" }, true},
		"literal outside glob":       {func(w, s *Role) { w.Match.IDGlob, s.Match.IDGlob = "T-001[0-4]", "T-0019" }, false},
		"equal globs":                {func(w, s *Role) { w.Match.IDGlob = "T-001[0-4]" }, true},
		"shadow needs a label":       {func(w, s *Role) { w.Match.Labels = []string{"ui"} }, false},
		"shadowed needs more labels": {func(w, s *Role) { s.Match.Labels = []string{"ui"} }, true},
		"shared label":               {func(w, s *Role) { w.Match.Labels, s.Match.Labels = []string{"ui"}, []string{"api", "ui"} }, true},
		"shadow narrows kinds":       {func(w, s *Role) { w.Match.Kinds = []string{"BUG"} }, false},
		"kinds subset":               {func(w, s *Role) { w.Match.Kinds, s.Match.Kinds = []string{"BUG", "TASK"}, []string{"BUG"} }, true},
		"kinds not subset":           {func(w, s *Role) { w.Match.Kinds, s.Match.Kinds = []string{"BUG"}, []string{"BUG", "TASK"} }, false},
		"different pool":             {func(w, s *Role) { s.Match.Pool = "lanes" }, false},
		"same pool":                  {func(w, s *Role) { w.Match.Pool, s.Match.Pool = "lanes", "lanes" }, true},
		"shadow needs selection":     {func(w, s *Role) { w.Match.PlanSelected = true }, false},
		"both need selection":        {func(w, s *Role) { w.Match.PlanSelected, s.Match.PlanSelected = true, true }, true},
		"statuses subset":            {func(w, s *Role) { w.Match.Statuses = []string{"OPEN", "BLOCKED"} }, true},
		"statuses not subset":        {func(w, s *Role) { s.Match.Statuses = []string{"OPEN", "BLOCKED"} }, false},
		"explicit OPEN":              {func(w, s *Role) { w.Match.Statuses = []string{"OPEN"} }, true},
		"shadow gate absent":         {func(w, s *Role) { w.Match.Gates = []GateMatch{{Gate: "review", States: []string{GatePass}}} }, false},
		"gate states subset": {func(w, s *Role) {
			w.Match.Gates = []GateMatch{{Gate: "review", States: []string{GatePass, GateNone}}}
			s.Match.Gates = []GateMatch{{Gate: "review", States: []string{GatePass}}}
		}, true},
		"gate states wider": {func(w, s *Role) {
			w.Match.Gates = []GateMatch{{Gate: "review", States: []string{GatePass}}}
			s.Match.Gates = []GateMatch{{Gate: "review", States: []string{GatePass, GateNone}}}
		}, false},
		"shadowed gate only": {func(w, s *Role) { s.Match.Gates = []GateMatch{{Gate: "review", States: []string{GatePass}}} }, true},
		"lane shadow":        {func(w, s *Role) { w.Match, w.Lane = nil, &Lane{Pool: "lanes"} }, false},
	}
	for name, tc := range cases {
		c := shadowConfig(t)
		tc.edit(&c.Roles[0], &c.Roles[1])
		raw, _ := json.Marshal(c)
		loaded, err := DecodeConfig(raw)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got := loaded.RoleShadows()
		if covered := slices.ContainsFunc(got, func(s RoleShadow) bool { return s.Role == "special" && s.ShadowedBy == "worker" }); covered != tc.covered {
			t.Errorf("%s: shadows %+v, want covered %v", name, got, tc.covered)
		}
		if slices.ContainsFunc(got, func(s RoleShadow) bool { return s.Role == "worker" }) {
			t.Errorf("%s: the lower-valued role was reported: %+v", name, got)
		}
	}
}

// CAL-V0-290: with several covering roles the report names the one the
// roster prefers, and a role is reported once.
func TestCALV0290_ShadowNamesPreferredCover(t *testing.T) {
	c := shadowConfig(t)
	broad := c.Roles[0]
	broad.Name, broad.Priority = "broad", 20
	c.Roles = append(c.Roles, broad)
	want := []RoleShadow{
		{Role: "worker", ShadowedBy: "broad", Priority: 50, ShadowedByPriority: 20},
		{Role: "special", ShadowedBy: "broad", Priority: 92, ShadowedByPriority: 20},
	}
	if got := c.RoleShadows(); !slices.Equal(got, want) {
		t.Fatalf("RoleShadows = %+v, want %+v", got, want)
	}
}

// CAL-V0-290: the dispatcher appends one config warning event per shadowed
// role at start and again after an applied reload.
func TestCALV0290_ShadowWarningEvents(t *testing.T) {
	c := shadowConfig(t)
	f := newReloadFixture(t, c, &fakeQueue{})
	shadowed := func() []Event {
		var out []Event
		for _, e := range f.events("config") {
			if e.Detail["outcome"] == "ROLE_SHADOWED" {
				out = append(out, e)
			}
		}
		return out
	}
	ev := shadowed()
	if len(ev) != 1 || ev[0].Role != "special" || ev[0].Detail["shadowedBy"] != "worker" || ev[0].Message != (RoleShadow{"special", "worker", 92, 50}).Message() {
		t.Fatalf("start warning = %+v", ev)
	}
	next := cloneConfig(t, c)
	next.Roles[1].Prompt = "changed"
	f.write(next)
	f.tick()
	if ev := shadowed(); len(ev) != 2 || ev[1].Role != "special" {
		t.Fatalf("reload warning = %+v", ev)
	}
	fixed := cloneConfig(t, next)
	fixed.Roles[1].Priority = 10
	f.write(fixed)
	f.tick()
	if ev := shadowed(); len(ev) != 2 {
		t.Fatalf("a correctly ordered reload warned: %+v", ev)
	}
}
