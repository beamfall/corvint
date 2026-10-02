package intent_test

import (
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"strings"
	"testing"
)

// supervisionPolicy builds a canonical supervision object: extra precedes
// maxRepairCycles (efforts), tail follows program (stageWallMinutes).
func supervisionPolicy(t *testing.T, extra string, tail ...string) (*intent.Policy, error) {
	t.Helper()
	raw := `{"contextRequired":true,` + extra + `"maxRepairCycles":"1","profile":"taskman-codex-supervisor/0","program":{"inputTokens":"0","outputTokens":"0","turns":"8","wallClockMinutes":"600"}` + strings.Join(tail, "") + "}\n"
	sup, e := wire.Parse([]byte(raw))
	if e != nil {
		t.Fatalf("fixture supervision %s: %v", raw, e)
	}
	v := fixture.PolicyValue()
	v.Obj.Set("supervision", sup)
	return intent.DecodePolicy(wire.EncodeFile(v))
}

func TestCALV0062_PolicyEffortAllowlist(t *testing.T) {
	t.Run("absent allowlist admits only low", func(t *testing.T) {
		p, e := supervisionPolicy(t, "")
		if e != nil {
			t.Fatal(e)
		}
		s := p.Supervision
		if !s.AllowsEffort("implement", "low") || s.AllowsEffort("implement", "medium") || s.AllowsEffort("review", "high") {
			t.Fatalf("default allowlist %+v", s.Efforts)
		}
		var none *intent.SupervisionPolicy
		if !none.AllowsEffort("review", "low") || none.AllowsEffort("review", "medium") {
			t.Fatal("nil supervision must admit only low")
		}
	})
	t.Run("declared stages", func(t *testing.T) {
		p, e := supervisionPolicy(t, `"efforts":{"implement":["high","medium"],"review":["high","low","medium"]},`)
		if e != nil {
			t.Fatal(e)
		}
		s := p.Supervision
		if !s.AllowsEffort("implement", "high") || !s.AllowsEffort("implement", "medium") || s.AllowsEffort("implement", "low") {
			t.Fatalf("implement allowlist %+v", s.Efforts)
		}
		if !s.AllowsEffort("review", "low") || !s.AllowsEffort("integrate", "low") || s.AllowsEffort("integrate", "medium") {
			t.Fatalf("review/integrate allowlist %+v", s.Efforts)
		}
	})
	// A known stage beside an unknown one must not hide the unknown key.
	if _, e := supervisionPolicy(t, `"efforts":{"implement":["low"],"repair":["high"]},`); wire.CodeOf(e) != wire.CodeMalformed || !strings.Contains(e.Error(), "repair") {
		t.Fatalf("mixed known/unknown stages: %v", e)
	}
	for name, extra := range map[string]string{
		"unknown stage":   `"efforts":{"repair":["low"]},`,
		"unknown effort":  `"efforts":{"implement":["xhigh"]},`,
		"empty list":      `"efforts":{"implement":[]},`,
		"no stage":        `"efforts":{},`,
		"unsorted":        `"efforts":{"implement":["medium","high"]},`,
		"duplicate":       `"efforts":{"implement":["low","low"]},`,
		"not an array":    `"efforts":{"implement":"low"},`,
		"unknown sibling": `"maxEffort":"high",`,
	} {
		t.Run("refuses "+name, func(t *testing.T) {
			if _, e := supervisionPolicy(t, extra); e == nil {
				t.Fatalf("accepted %s", extra)
			}
		})
	}
}

func TestCALV0063_PolicyStageWallBound(t *testing.T) {
	p, e := supervisionPolicy(t, "")
	if e != nil || p.Supervision.StageWallSeconds() != 3600 {
		t.Fatalf("default stage wall %v", e)
	}
	var none *intent.SupervisionPolicy
	if none.StageWallSeconds() != 3600 {
		t.Fatal("nil supervision must keep the one-hour bound")
	}
	p, e = supervisionPolicy(t, "", `,"stageWallMinutes":"240"`)
	if e != nil || p.Supervision.StageWallSeconds() != 240*60 || intent.MaxStageWallMinutes != wire.MaxLaneWallMinutes {
		t.Fatalf("declared stage wall %v", e)
	}
	// Above the lane wallClockMinutes ceiling a stage wall could never take
	// effect, so it is refused with LIMIT_EXCEEDED rather than accepted.
	for _, over := range []string{`"0"`, `"241"`, `"1440"`} {
		if _, e := supervisionPolicy(t, "", `,"stageWallMinutes":`+over); wire.CodeOf(e) != wire.CodeLimitExceeded {
			t.Fatalf("stageWallMinutes %s: %v", over, e)
		}
	}
	for _, bad := range []string{`"-1"`, `"01"`} {
		if _, e := supervisionPolicy(t, "", `,"stageWallMinutes":`+bad); e == nil {
			t.Fatalf("accepted stageWallMinutes %s", bad)
		}
	}
}
