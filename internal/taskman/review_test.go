package taskman

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

func TestNTPV0005WholeRepositoryEmptyEffects(t *testing.T) {
	t.Run("NTP-V0-005 whole scope excludes empty effects", func(t *testing.T) {
		for _, mode := range []string{"selected whole", "selected fallback", "live whole", "live empty"} {
			t.Run(mode, func(t *testing.T) {
				c := testCapture(t)
				for i := range c.tickets {
					c.tickets[i].paths = nil
					c.tickets[i].resources = nil
				}
				whole := []Resource{{"WHOLE_REPOSITORY", "queue:acme:main"}}
				switch mode {
				case "selected whole":
					c.tickets[0].resources = whole
				case "selected fallback":
					c.tickets[0].paths = []string{"missing.go"}
					setString(c.policy, "serialFallback", "WHOLE_REPOSITORY")
				case "live whole":
					c.observed.reservations = []reservation{{ticket: "ticket:acme:main:other", workers: 1, resources: whole}}
				case "live empty":
					c.tickets[0].resources = whole
					c.observed.reservations = []reservation{{ticket: "ticket:acme:main:other", workers: 1}}
				}
				p := testPlan(t, c)
				if strings.HasPrefix(mode, "selected") {
					if p.Entries[0].State != "SELECTED" || p.Entries[1].State != "DEFERRED" {
						t.Fatal(p)
					}
				} else if p.Entries[0].State != "DEFERRED" || p.Entries[0].Reason != "RESOURCE_COLLISION" {
					t.Fatal(p)
				}
			})
		}
	})
}
func TestNTPV0002ReservationSchema(t *testing.T) {
	t.Run("NTP-V0-002 native reservation observation integrity", func(t *testing.T) {
		for _, mode := range []string{"valid", "attempt", "foreign queue", "unknown ticket", "future revision", "older revision", "missing resource", "whole coverage", "generation zero", "worker zero", "future sequence", "unknown state"} {
			t.Run(mode, func(t *testing.T) {
				c := testCapture(t)
				r := testValue(t, map[string]any{"attemptId": "attempt:acme:main:" + strings.Repeat("a", 32), "generation": "1", "ticketId": c.tickets[0].id, "ticketRevision": "1", "resources": []Resource{{"PATH", "a.go"}, {"PATH", "b.go"}}, "capacityUses": []any{}, "workers": "1", "state": "ACTIVE", "createdSeq": "1", "coverage": "QUALIFIED"})
				switch mode {
				case "attempt":
					setString(r, "attemptId", "attempt:fixture:1")
				case "foreign queue":
					setString(r, "attemptId", "attempt:foreign:queue:"+strings.Repeat("a", 32))
				case "unknown ticket":
					setString(r, "ticketId", "ticket:acme:main:unknown")
				case "future revision":
					setString(r, "ticketRevision", "999")
				case "older revision":
					c.tickets[0].revision = "2"
				case "missing resource":
					r.Obj.Values["resources"] = testValue(t, []any{})
				case "whole coverage":
					r.Obj.Values["resources"] = testValue(t, []any{})
					setString(r, "coverage", "WHOLE_REPOSITORY")
				case "generation zero":
					setString(r, "generation", "0")
				case "worker zero":
					setString(r, "workers", "0")
				case "future sequence":
					setString(r, "createdSeq", "999")
				case "unknown state":
					setString(r, "state", "TERMINAL")
				}
				v := observationValue(t, c)
				value(v, "reservationSet").Obj.Values["entries"] = wire.Value{Kind: wire.KindArray, Arr: []wire.Value{r}}
				o, err := decodeObservations(append(canonical(v), '\n'), c, strings.Repeat("a", 40), strings.Repeat("b", 40))
				valid := mode == "valid" || mode == "whole coverage"
				if (err == nil) != valid {
					t.Fatalf("mode=%s err=%v", mode, err)
				}
				if valid && len(o.reservations) != 1 {
					t.Fatal("reservation lost")
				}
			})
		}
	})
}
func TestNTPV0007HistorySchema(t *testing.T) {
	t.Run("NTP-V0-007 native history integrity", func(t *testing.T) {
		for _, mode := range []string{"valid", "revision zero", "future revision", "future deferral", "unknown reason", "unknown blocker", "wrong ticket", "head zero", "foreign queue"} {
			t.Run(mode, func(t *testing.T) {
				c := testCapture(t)
				p := testPlan(t, c)
				switch mode {
				case "revision zero":
					p.Entries[0].TicketRevision = "0"
				case "future revision":
					p.Entries[0].TicketRevision = "999"
				case "future deferral":
					s := "999"
					p.Entries[0].DeferredSinceSeq = &s
				case "unknown reason":
					p.Entries[0].Reason = "SELECTED"
				case "unknown blocker":
					p.Entries[0].Blockers = []string{"RESERVATIONS_UNKNOWN"}
				case "wrong ticket":
					p.Entries[0].TicketID = "ticket:acme:main:unknown"
				case "head zero":
					p.HeadSeq = "0"
				case "foreign queue":
					p.QueueID = "queue:foreign:main"
				}
				v := observationValue(t, c)
				v.Obj.Values["history"] = testValue(t, []Plan{p})
				_, err := decodeObservations(append(canonical(v), '\n'), c, strings.Repeat("a", 40), strings.Repeat("b", 40))
				if (err == nil) != (mode == "valid") {
					t.Fatalf("mode=%s err=%v", mode, err)
				}
			})
		}
	})
}
func TestNTPV0001NativePrimitiveBounds(t *testing.T) {
	t.Run("NTP-V0-001 native primitive bounds", func(t *testing.T) {
		if !identifier(strings.Repeat("x", 128)) || identifier(strings.Repeat("x", 129)) {
			t.Fatal("identifier bound")
		}
		if !validPath(strings.Repeat("x", 512)) || validPath(strings.Repeat("x", 513)) {
			t.Fatal("path bound")
		}
		if _, err := resources(testValue(t, []Resource{{"OTHER", strings.Repeat("x", 129)}})); err == nil {
			t.Fatal("resource identifier bound")
		}
	})
}
