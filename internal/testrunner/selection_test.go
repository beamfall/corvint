package testrunner

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

const (
	pass = "tests/sample.js::sample::default::pass"
	skip = "tests/sample.js::sample::default::skip"
)

func nightwatchSelection(keys ...string) *Selection {
	return &Selection{Version: SelectionVersion, Matcher: NightwatchSessionElided, Tests: keys}
}

// nightwatchTests mirrors the dynamic Nightwatch identity: the WebDriver
// session stays inside every native ID.
func nightwatchTests(session string, cases ...string) []Test {
	var out []Test
	for _, c := range cases {
		out = append(out, Test{ID: "tests/sample.js::sample::default::" + session + "::" + c, Name: c, File: "tests/sample.js", Suite: "sample", State: Passed, Attempts: []Attempt{{State: Passed}}})
	}
	return out
}

func TestSelectionAdmissionIsClosed(t *testing.T) {
	ok := Request{Runner: "nightwatch", ExpectedSelection: nightwatchSelection(pass, skip)}
	if e := AdmitSelection(ok); e != nil {
		t.Fatal(e)
	}
	if e := AdmitSelection(Request{Runner: "nightwatch", ExpectedTests: []string{"x"}}); e != nil {
		t.Fatal("absent selection must not change ExpectedTests admission", e)
	}
	many := make([]string, MaxTests+1)
	for i := range many {
		many[i] = fmt.Sprintf("m::s::e::c%d", i)
	}
	for name, r := range map[string]Request{
		"version":         {Runner: "nightwatch", ExpectedSelection: &Selection{Version: "corvint-test-selection/2", Matcher: NightwatchSessionElided, Tests: []string{pass}}},
		"matcher":         {Runner: "nightwatch", ExpectedSelection: &Selection{Version: SelectionVersion, Matcher: "nightwatch-session-elided/2", Tests: []string{pass}}},
		"runner-binding":  {Runner: "webdriverio", ExpectedSelection: nightwatchSelection(pass)},
		"exclusive":       {Runner: "nightwatch", ExpectedTests: []string{pass}, ExpectedSelection: nightwatchSelection(pass)},
		"empty":           {Runner: "nightwatch", ExpectedSelection: nightwatchSelection()},
		"bound":           {Runner: "nightwatch", ExpectedSelection: nightwatchSelection(many...)},
		"duplicate":       {Runner: "nightwatch", ExpectedSelection: nightwatchSelection(pass, pass)},
		"session-bearing": {Runner: "nightwatch", ExpectedSelection: nightwatchSelection("tests/sample.js::sample::default::wd-1::pass")},
		"three-parts":     {Runner: "nightwatch", ExpectedSelection: nightwatchSelection("sample::default::pass")},
		"empty-part":      {Runner: "nightwatch", ExpectedSelection: nightwatchSelection("tests/sample.js::sample::::pass")},
		"edge-colon":      {Runner: "nightwatch", ExpectedSelection: nightwatchSelection("tests/sample.js::sample::default:::pass")},
		"control":         {Runner: "nightwatch", ExpectedSelection: nightwatchSelection("tests/sample.js::sample::default::pa\nss")},
		"tab":             {Runner: "nightwatch", ExpectedSelection: nightwatchSelection("tests/sample.js::sample::default::pa\tss")},
		"escape":          {Runner: "nightwatch", ExpectedSelection: nightwatchSelection("tests/sample.js::sample::default::pa\x1bss")},
		"delete":          {Runner: "nightwatch", ExpectedSelection: nightwatchSelection("tests/sample.js::sample::default::pa\x7fss")},
		"c1-control":      {Runner: "nightwatch", ExpectedSelection: nightwatchSelection("tests/sample.js::sample::default::pa\u0085ss")},
		"length":          {Runner: "nightwatch", ExpectedSelection: nightwatchSelection("m::s::e::" + strings.Repeat("x", maxSelectionKeyBytes))},
	} {
		if e := AdmitSelection(r); e == nil {
			t.Fatalf("%s admitted", name)
		}
	}
}

func TestSelectionMatchesFreshSessionsAndKeepsNativeIdentity(t *testing.T) {
	in := Input{Runner: "nightwatch", ExitCode: 0, ExpectedSelection: nightwatchSelection(pass, skip)}
	ids := map[string]bool{}
	for _, session := range []string{"4f1c0a8e2b7d", "9a3e55c1d0f6"} {
		o := Normalize(in, Observation{Complete: true, RetryInformation: Retained, Tests: nightwatchTests(session, "pass", "skip")})
		if !o.Complete || len(o.Problems) != 0 {
			t.Fatalf("session %s: %+v", session, o.Problems)
		}
		for _, x := range o.Tests {
			if !strings.Contains(x.ID, "::"+session+"::") || x.State != Passed {
				t.Fatalf("native session identity or state lost: %+v", x)
			}
			ids[x.ID] = true
		}
	}
	if len(ids) != 4 {
		t.Fatalf("differing sessions collapsed native identities: %v", ids)
	}
}

func TestSelectionRefusesInexactMatches(t *testing.T) {
	odd := nightwatchTests("wd-1", "pass")[0]
	for name, c := range map[string]struct {
		in    Input
		tests []Test
		code  string
	}{
		"missing":          {Input{ExpectedSelection: nightwatchSelection(pass, skip)}, nightwatchTests("wd-1", "pass"), "missing-selected-test"},
		"wrong-expected":   {Input{ExpectedSelection: nightwatchSelection(pass, "tests/sample.js::sample::default::other")}, nightwatchTests("wd-1", "pass", "skip"), "missing-selected-test"},
		"extra":            {Input{ExpectedSelection: nightwatchSelection(pass)}, nightwatchTests("wd-1", "pass", "skip"), "extra-selected-test"},
		"aliased-sessions": {Input{ExpectedSelection: nightwatchSelection(pass)}, append(nightwatchTests("wd-1", "pass"), nightwatchTests("wd-2", "pass")...), "aliased-selected-test"},
		"no-session":       {Input{ExpectedSelection: nightwatchSelection(pass)}, nightwatchTests("", "pass"), "unmatchable-selected-test"},
		"session-edge":     {Input{ExpectedSelection: nightwatchSelection(pass)}, nightwatchTests(":wd", "pass"), "unmatchable-selected-test"},
		"separator-field":  {Input{ExpectedSelection: nightwatchSelection(pass)}, []Test{{ID: "a::b::sample::default::wd::pass", File: "a::b", Suite: "sample", Name: "pass", State: Passed}}, "unmatchable-selected-test"},
		"fields-disagree":  {Input{ExpectedSelection: nightwatchSelection(pass)}, []Test{{ID: odd.ID, File: odd.File, Suite: "other", Name: odd.Name, State: Passed}}, "unmatchable-selected-test"},
		"duplicate":        {Input{ExpectedSelection: nightwatchSelection(pass, pass)}, nightwatchTests("wd-1", "pass"), "invalid-test-selection"},
		"both-sources":     {Input{Expected: []string{odd.ID}, ExpectedSelection: nightwatchSelection(pass)}, nightwatchTests("wd-1", "pass"), "invalid-test-selection"},
		"runner":           {Input{Runner: "webdriverio", ExpectedSelection: nightwatchSelection(pass)}, nightwatchTests("wd-1", "pass"), "invalid-test-selection"},
	} {
		if c.in.Runner == "" {
			c.in.Runner = "nightwatch"
		}
		o := Normalize(c.in, Observation{Complete: true, RetryInformation: Retained, Tests: c.tests})
		found := false
		for _, p := range o.Problems {
			found = found || p.Code == c.code
		}
		if o.Complete || !found || o.Tests[0].State != Unknown {
			t.Fatalf("%s: complete=%v problems=%+v", name, o.Complete, o.Problems)
		}
	}
}

func TestSelectionIsAdditiveToHistoricalBytes(t *testing.T) {
	b, e := json.Marshal(PlanDocument{Request: Request{Runner: "nightwatch", ExpectedTests: []string{"x"}}})
	if e != nil || strings.Contains(string(b), "expectedSelection") {
		t.Fatalf("absent selection changed plan bytes: %s %v", b, e)
	}
	b, _ = json.Marshal(Input{Runner: "nightwatch"})
	if strings.Contains(string(b), "expectedSelection") {
		t.Fatalf("absent selection changed input bytes: %s", b)
	}
	var r Request
	for _, raw := range []string{
		`{"runner":"nightwatch","expectedSelection":{"version":"corvint-test-selection/1","matcher":"nightwatch-session-elided/1","tests":["a::b::c::d"],"alias":true}}`,
		`{"runner":"nightwatch","expectedSelection":{"version":"corvint-test-selection/1","version":"corvint-test-selection/1","matcher":"nightwatch-session-elided/1","tests":["a::b::c::d"]}}`,
	} {
		if DecodeDocument([]byte(raw), &r) == nil {
			t.Fatalf("open selection document admitted: %s", raw)
		}
	}
}
