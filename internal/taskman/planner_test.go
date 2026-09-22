package taskman

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/workqueue"
)

func testValue(t *testing.T, v any) wire.Value {
	t.Helper()
	raw, e := encode(v)
	if e != nil {
		t.Fatal(e)
	}
	x, e := document(raw, 32<<20)
	if e != nil {
		t.Fatal(e)
	}
	return x
}
func testDocument(t *testing.T, name string) wire.Value {
	t.Helper()
	raw, e := os.ReadFile("testdata/" + name + ".json")
	if e != nil {
		t.Fatal(e)
	}
	v, e := document(raw, 32<<20)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func setString(v wire.Value, k, s string) {
	v.Obj.Values[k] = wire.Value{Kind: wire.KindString, Str: s}
}
func setBool(v wire.Value, k string, b bool) {
	v.Obj.Values[k] = wire.Value{Kind: wire.KindBool, Bool: b}
}
func testCapture(t *testing.T) captured {
	t.Helper()
	status := testDocument(t, "status")
	c := captured{queue: value(status, "items").Arr[0], snapshot: value(status, "snapshot"), policy: testDocument(t, "policy"), digests: map[string]string{}, observed: observation{complete: true, reservationDigest: strings.Repeat("a", 64)}, index: &contextindex.Index{Sources: map[string]contextindex.Source{"a.go": {Path: "a.go"}, "b.go": {Path: "b.go"}}, Tracked: map[string]struct{}{"a.go": {}, "b.go": {}}, Imports: map[string]map[string]struct{}{}}}
	for _, v := range value(testDocument(t, "tickets"), "items").Arr {
		x, e := decodeTicket(value(v, "record"))
		if e != nil {
			t.Fatal(e)
		}
		c.tickets = append(c.tickets, x)
	}
	return c
}
func testPlan(t *testing.T, c captured) Plan {
	t.Helper()
	p, e := plan(c)
	if e != nil {
		t.Fatal(e)
	}
	if _, err := decodePlan(testValue(t, p)); err != nil {
		t.Fatalf("emitted non-native plan: %v", err)
	}
	return p
}
func TestNTPV0003PriorityFirst(t *testing.T) {
	t.Run("NTP-V0-003 PriorityFirst", func(t *testing.T) {
		c := testCapture(t)
		p := testPlan(t, c)
		for i, want := range []string{"SELECTED", "DEFERRED", "DEFERRED"} {
			if p.Entries[i].State != want {
				t.Fatalf("%+v", p.Entries)
			}
		}
		if len(p.Entries[0].Resources) != 2 || p.MutationAuthority {
			t.Fatal(p)
		}
		c.tickets[0], c.tickets[2] = c.tickets[2], c.tickets[0]
		r := testPlan(t, c)
		a, _ := encode(p)
		b, _ := encode(r)
		if !bytes.Equal(a, b) {
			t.Fatal("input iteration affected plan")
		}
		for i := range c.tickets {
			c.tickets[i].priority = "P1"
			c.tickets[i].order = 4
		}
		r = testPlan(t, c)
		if r.Entries[0].TicketID != "ticket:acme:main:AT-0001" {
			t.Fatal("byte tie break")
		}
	})
}
func TestNTPV0004Eligibility(t *testing.T) {
	t.Run("NTP-V0-004 Eligibility", func(t *testing.T) {
		tests := []struct {
			name, want string
			mutate     func(*captured)
		}{
			{"held", "TICKET_STATE", func(c *captured) { c.tickets[0].status = "HELD" }},
			{"unknown reservations", "MISSING_EVIDENCE", func(c *captured) { c.observed.complete = false }},
			{"live", "ATTEMPT_LIVE", func(c *captured) { c.observed.reservations = []reservation{{ticket: c.tickets[0].id, workers: 1}} }},
			{"unbounded", "EXTERNAL_UNBOUNDED", func(c *captured) { setBool(value(c.tickets[0].raw, "effects"), "externalUnbounded", true) }},
			{"manual", "TICKET_STATE", func(c *captured) { setString(c.tickets[0].raw, "executionClass", "MANUAL") }},
			{"approval", "APPROVAL_MISSING", func(c *captured) { setString(c.tickets[0].raw, "executionClass", "APPROVAL_REQUIRED") }},
			{"gate", "GATE_UNKNOWN", func(c *captured) {
				c.tickets[0].raw.Obj.Values["dependencies"] = testValue(t, []any{map[string]any{"ticketId": c.tickets[1].id, "obligation": "GATE_PASSED", "gateId": "verify"}})
			}},
			{"incomplete dependency", "DEPENDENCY_UNSATISFIED", func(c *captured) {
				c.tickets[0].raw.Obj.Values["dependencies"] = testValue(t, []any{map[string]any{"ticketId": c.tickets[1].id, "obligation": "COMPLETED", "gateId": nil}})
			}},
			{"capability", "CAPABILITY_UNAVAILABLE", func(c *captured) {
				c.tickets[0].raw.Obj.Values["capabilities"] = testValue(t, []string{"runtime-probe"})
			}},
			{"barrier", "PAUSED", func(c *captured) {
				c.snapshot.Obj.Values["barrier"] = testValue(t, map[string]string{"scope": "ADMISSION", "reason": "OPERATOR"})
			}},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				c := testCapture(t)
				tc.mutate(&c)
				p := testPlan(t, c)
				if p.Entries[0].State != "BLOCKED" || p.Entries[0].Reason != tc.want {
					t.Fatal(p.Entries[0])
				}
				if !c.observed.complete && p.Capacity.AvailableWorkers != "0" {
					t.Fatal("invented headroom")
				}
			})
		}
		c := testCapture(t)
		c.tickets[1].status = "ARCHIVED"
		setString(c.tickets[1].raw, "archivedFrom", "COMPLETED")
		c.tickets[0].raw.Obj.Values["dependencies"] = testValue(t, []any{map[string]any{"ticketId": c.tickets[1].id, "obligation": "COMPLETED", "gateId": nil}})
		if testPlan(t, c).Entries[0].State != "SELECTED" {
			t.Fatal("completed tombstone dependency")
		}
		c = testCapture(t)
		setString(c.tickets[0].raw, "executionClass", "APPROVAL_REQUIRED")
		c.tickets[0].raw.Obj.Values["approvals"] = testValue(t, []any{map[string]any{"operation": "RUN", "targetRevision": "1", "revoked": false}})
		if testPlan(t, c).Entries[0].State != "SELECTED" {
			t.Fatal("current approval")
		}
		setString(value(c.tickets[0].raw, "approvals").Arr[0], "targetRevision", "2")
		if testPlan(t, c).Entries[0].Reason != "APPROVAL_MISSING" {
			t.Fatal("stale approval admitted")
		}
	})
}
func TestNTPV0005CoverageAndCollisions(t *testing.T) {
	t.Run("NTP-V0-005 CoverageAndCollisions", func(t *testing.T) {
		for _, mode := range []string{"untracked", "unparsed", "excluded", "directory", "resource", "absent index"} {
			t.Run(mode, func(t *testing.T) {
				c := testCapture(t)
				switch mode {
				case "untracked":
					c.tickets[0].paths = []string{"new.go"}
				case "unparsed":
					c.index.Unparsed = []contextindex.Unparsed{{Path: "b.go", Facts: "imports"}}
				case "excluded":
					c.index.Exclusions = []contextindex.Exclusion{{Path: "hidden.go"}}
				case "directory":
					c.tickets[0].paths = []string{"src/"}
				case "resource":
					c.tickets[0].resources = []Resource{{"PATH", "new.go"}}
				case "absent index":
					c.index = nil
				}
				if testPlan(t, c).Entries[0].Reason != "COVERAGE_UNKNOWN" {
					t.Fatal(mode)
				}
				setString(c.policy, "serialFallback", "WHOLE_REPOSITORY")
				p := testPlan(t, c)
				if p.Entries[0].State != "SELECTED" || p.Entries[1].State == "SELECTED" {
					t.Fatal(p.Entries)
				}
				c.observed.reservations = []reservation{{ticket: "ticket:acme:main:OTHER", workers: 1, resources: []Resource{{"PATH", "elsewhere.go"}}}}
				if testPlan(t, c).Entries[0].State != "DEFERRED" {
					t.Fatal("fallback ignored live reservation")
				}
			})
		}
		for _, tc := range []struct {
			a, b Resource
			want bool
		}{{Resource{"PATH", "src/"}, Resource{"PATH", "src/a.go"}, true}, {Resource{"PATH", "src"}, Resource{"PATH", "src/"}, true}, {Resource{"PATH", "src/"}, Resource{"PATH", "src2/a.go"}, false}, {Resource{"PORT", "4000"}, Resource{"PORT", "4000"}, true}, {Resource{"PORT", "4000"}, Resource{"OTHER", "4000"}, false}, {Resource{"WHOLE_REPOSITORY", "q"}, Resource{"SCHEMA", "x"}, true}} {
			if overlap(tc.a, tc.b) != tc.want || overlap(tc.b, tc.a) != tc.want {
				t.Fatal(tc)
			}
		}
		c := testCapture(t)
		c.tickets[0].paths = []string{"a.go"}
		c.index.Imports = map[string]map[string]struct{}{"a.go": {"b.go": {}}}
		r, complete := nativeClosure(c.tickets[0], c.index, workqueue.IndexCollisionSource(c.index))
		if !complete || len(r) != 2 {
			t.Fatal(r, complete)
		}
	})
}
func TestNTPV0006CapacityAndBindings(t *testing.T) {
	t.Run("NTP-V0-006 CapacityAndBindings", func(t *testing.T) {
		c := testCapture(t)
		setString(value(c.policy, "capacity"), "maxWorkersTotal", "1")
		c.observed.reservations = []reservation{{ticket: "ticket:acme:main:OTHER", workers: 1}}
		p := testPlan(t, c)
		if p.Entries[0].Reason != "LIMIT_EXCEEDED" || p.Capacity.AvailableWorkers != "0" {
			t.Fatal(p)
		}
		if p.HeadSeq != stringAt(c.snapshot, "headSeq") || p.PolicySHA256 != stringAt(c.queue, "policySha256") || p.ReservationSetSHA256 != c.observed.reservationDigest || p.Entries[0].TicketRevision != "1" {
			t.Fatal("plan identities")
		}
	})
}
func TestNTPV0007DeferralHistory(t *testing.T) {
	t.Run("NTP-V0-007 DeferralHistory", func(t *testing.T) {
		c := testCapture(t)
		base := testPlan(t, c)
		a := base
		a.HeadSeq = "1"
		b := base
		b.HeadSeq = "2"
		b.Entries = append([]Entry{}, base.Entries...)
		b.Entries[1].State = "SELECTED"
		d := base
		d.HeadSeq = "3"
		c.observed.history = []Plan{a, b, d}
		p := testPlan(t, c)
		if p.Entries[1].DeferredSinceSeq == nil || *p.Entries[1].DeferredSinceSeq != "3" {
			t.Fatal(p.Entries[1])
		}
		c.tickets[1].revision = "2"
		if testPlan(t, c).Entries[1].DeferredSinceSeq != nil {
			t.Fatal("deferral crossed acceptance revision")
		}
	})
}
func observationValue(t *testing.T, c captured) wire.Value {
	return testValue(t, map[string]any{"profile": "corvint-taskman-fixture-observations/0", "sourceCommit": strings.Repeat("a", 40), "sourceTree": strings.Repeat("b", 40), "queueId": stringAt(c.queue, "queueId"), "policySha256": stringAt(c.queue, "policySha256"), "headSeq": stringAt(c.snapshot, "headSeq"), "intentTreeSha256": stringAt(c.snapshot, "intentTreeSha256"), "reservationsComplete": true, "attemptsComplete": true, "historyComplete": true, "reservationSet": map[string]any{"profile": "taskman-reservation-set/0", "queueId": stringAt(c.queue, "queueId"), "entries": []any{}}, "history": []any{}})
}
func TestNTPV0002ObservationRefusals(t *testing.T) {
	t.Run("NTP-V0-002 ObservationRefusals", func(t *testing.T) {
		c := testCapture(t)
		for _, key := range []string{"sourceCommit", "sourceTree", "queueId", "policySha256", "headSeq", "intentTreeSha256"} {
			v := observationValue(t, c)
			setString(v, key, "wrong")
			if _, e := decodeObservations(append(canonical(v), '\n'), c, strings.Repeat("a", 40), strings.Repeat("b", 40)); e == nil {
				t.Fatal(key)
			}
		}
		v := observationValue(t, c)
		setBool(v, "historyComplete", false)
		if _, e := decodeObservations(append(canonical(v), '\n'), c, strings.Repeat("a", 40), strings.Repeat("b", 40)); e == nil {
			t.Fatal("unknown history accepted")
		}
		v = observationValue(t, c)
		setBool(v, "reservationsComplete", false)
		o, e := decodeObservations(append(canonical(v), '\n'), c, strings.Repeat("a", 40), strings.Repeat("b", 40))
		if e != nil || o.complete {
			t.Fatal(o, e)
		}
	})
}
func TestNTPV0001NativeJSON(t *testing.T) {
	t.Run("NTP-V0-001 NativeJSON", func(t *testing.T) {
		for _, raw := range []string{"{\"a\":true,\"a\":false}\n", "{\"a\": {\"b\":1,\"b\":2}}\n", "{\"a\":true} \n", "{\"a\":\"\\ud800\"}\n"} {
			if _, e := document([]byte(raw), 1000); e == nil {
				t.Fatal(raw)
			}
		}
		for _, s := range []string{"01", "-1", "2147483648"} {
			if _, e := number(wire.Value{Kind: wire.KindString, Str: s}, 2147483647); e == nil {
				t.Fatal(s)
			}
		}
		v := testValue(t, map[string]string{"prose": "ü\b\f\n\\b<script>"})
		raw := append(canonical(v), '\n')
		if bytes.Contains(raw, []byte(`\b`)) && !bytes.Contains(raw, []byte(`\\b`)) {
			t.Fatal("escape")
		}
		var decoded map[string]string
		if e := json.Unmarshal(raw, &decoded); e != nil || decoded["prose"] != "ü\b\f\n\\b<script>" {
			t.Fatal(string(raw), e)
		}
	})
}
