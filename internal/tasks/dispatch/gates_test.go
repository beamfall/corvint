//go:build darwin || linux

package dispatch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

// TestERGV0009_GatePredicates covers the typed gate predicate: its closed
// configuration, NONE for an absent record, no match for STALE or UNKNOWN,
// an unchanged fingerprint without gates, and a workState program that
// cannot supply gates.
func TestERGV0009_GatePredicates(t *testing.T) {
	c := testConfig(t, "exit 0")
	raw, _ := json.Marshal(c)
	raw = []byte(strings.Replace(string(raw), `"match":{}`, `"match":{"gates":[{"gate":"G1","states":["RETURN","NONE"]}]}`, 1))
	d, err := DecodeConfig(raw)
	if err != nil || len(d.Roles[0].Match.Gates) != 1 || d.Roles[0].Match.Gates[0].States[1] != GateNone {
		t.Fatalf("gate predicate %s: %v", raw, err)
	}
	var seventeen []GateMatch
	for i := 0; i < 17; i++ {
		seventeen = append(seventeen, GateMatch{Gate: "G" + strings.Repeat("x", i), States: []string{GateNone}})
	}
	bad := map[string][]GateMatch{
		"no states":      {{Gate: "G1"}},
		"five states":    {{Gate: "G1", States: []string{"PASS", "RETURN", "RESUBMITTED", "NONE", "PASS"}}},
		"stale state":    {{Gate: "G1", States: []string{"STALE"}}},
		"control byte":   {{Gate: "G\x01", States: []string{"PASS"}}},
		"17 predicates":  seventeen,
		"repeated state": {{Gate: "G1", States: []string{"PASS", "PASS"}}},
		"repeated gate":  {{Gate: "G1", States: []string{"PASS"}}, {Gate: "G1", States: []string{"NONE"}}},
		"not a label":    {{Gate: "", States: []string{"PASS"}}},
	}
	for name, gates := range bad {
		c := testConfig(t, "exit 0")
		c.Roles[0].Match.Gates = gates
		raw, _ := json.Marshal(c)
		if _, err := DecodeConfig(raw); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	m := &Match{Gates: []GateMatch{{Gate: "G1", States: []string{GatePass, GateReturn, GateResubmitted, GateNone}}}}
	tk := ticket("t1", "P1", 1)
	if matches(m, tk) || tk.GateState("G1") != StateUnknown {
		t.Fatal("an unobserved ticket's gate is not UNKNOWN")
	}
	tk.Gates = map[string]GateView{"G1": {Status: "CURRENT", Verdict: GateReturn}}
	if matches(m, tk) {
		t.Fatal("gates without GatesObserved matched")
	}
	tk.Gates, tk.GatesObserved = nil, true
	if !matches(m, tk) || tk.GateState("G1") != GateNone {
		t.Fatal("absent record of an observed ticket is not NONE")
	}
	for _, v := range []GateView{{Status: "STALE", Verdict: GateReturn}, {Status: "UNKNOWN"}, {Status: "CURRENT"}, {Status: "CURRENT", Verdict: "MAYBE"}} {
		tk.Gates = map[string]GateView{"G1": v}
		if matches(m, tk) {
			t.Fatalf("%+v matched a predicate", v)
		}
	}
	plain := ticket("t1", "P1", 1)
	obs := &Observation{Tickets: []Ticket{plain}}
	sum := sha256.Sum256([]byte("OPEN|1|NONE\n"))
	if Fingerprint(obs, plain.ID) != hex.EncodeToString(sum[:]) {
		t.Fatal("fingerprint without gates changed")
	}
	obs.Tickets[0].Gates = map[string]GateView{"G1": {Status: "CURRENT", Verdict: GateReturn, Generation: "1", Revision: "2", Head: "a"}}
	before := Fingerprint(obs, plain.ID)
	obs.Tickets[0].Gates["G1"] = GateView{Status: "CURRENT", Verdict: GateReturn, Generation: "2", Revision: "4", Head: "b"}
	if Fingerprint(obs, plain.ID) == before {
		t.Fatal("a new gate head is not progress")
	}
	if _, err := decodeCommandStates([]byte(`{"t1":{"state":"review","gates":{"G1":{"verdict":"PASS"}}}}`)); err == nil {
		t.Fatal("workState program supplied gates")
	}
	if s, err := decodeCommandStates([]byte(`{"t1":{"state":"review"},"t2":"done"}`)); err != nil || s["t1"].State != "review" || s["t2"].State != "done" {
		t.Fatalf("legacy workState output %v %v", s, err)
	}
}
