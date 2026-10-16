package cli

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0310_UnknownFieldMessageAcrossItems pins the CAL-V0-310 text over
// heterogeneous items: keys are the sorted union, every parent that carries
// the key one level down is suggested in sorted order, and an absent sub-key
// lists the sorted union of that key's sub-keys.
func TestCALV0310_UnknownFieldMessageAcrossItems(t *testing.T) {
	t.Run("CAL-V0-310", func(t *testing.T) {
		obj := func(kv ...any) wire.Value {
			o := wire.NewObject()
			for i := 0; i < len(kv); i += 2 {
				o.Set(kv[i].(string), kv[i+1].(wire.Value))
			}
			return wire.ObjectValue(o)
		}
		s := wire.String("x")
		items := []wire.Value{
			obj("zeta", s, "record", obj("title", s, "body", s)),
			obj("alpha", s, "entries", wire.Array(s, obj("body", s, "state", s)), "record", obj("owner", s)),
		}
		cases := []struct{ fields, want string }{
			{"body", `field "body" is not in this read's items; its keys are alpha, entries, record, zeta; did you mean "entries.body" or "record.body"?`},
			{"nope", `field "nope" is not in this read's items; its keys are alpha, entries, record, zeta`},
			{"body.sub", `field "body" is not in this read's items; its keys are alpha, entries, record, zeta`},
			{"record.nope", `field "record.nope" is not in any element of "record"; its sub-keys are body, owner, title`},
			{"zeta.sub", `field "zeta" is a scalar and has no sub-key`},
			{"retries", `field "retries" is not in this read's items; its keys are alpha, entries, record, zeta; queue status includes "retries" only with --retries`},
		}
		for _, c := range cases {
			fields, msg := parseCompactFields(c.fields)
			if msg != "" {
				t.Fatalf("%s: %s", c.fields, msg)
			}
			if got := checkCompactFields(items, fields); got != c.want {
				t.Errorf("%s:\n got %s\nwant %s", c.fields, got, c.want)
			}
		}
		if got := checkCompactFields(items, []string{"record.body", "entries.state"}); got != "" {
			t.Fatalf("valid nested keys refused: %s", got)
		}
	})
}
