package intent

import "github.com/Beamfall/corvint/internal/tasks/wire"

// SupervisedStages are the native supervised stages a policy effort
// allowlist may name (CAL-V0-059).
var SupervisedStages = []string{"implement", "integrate", "review"}

// SupervisedEfforts are the Codex reasoning efforts an owner may allow.
var SupervisedEfforts = []string{"high", "low", "medium"}

// DefaultStageWallMinutes is the per-stage wall bound when the policy does
// not declare stageWallMinutes; it preserves the original one-hour cap.
const DefaultStageWallMinutes = 60

// MaxStageWallMinutes bounds the optional per-stage wall allowance (24 h).
const MaxStageWallMinutes = 1440

type SupervisionPolicy struct {
	MaxRepairCycles, Turns, WallClockMinutes wire.Count
	InputTokens, OutputTokens                wire.Size
	// Efforts is the owner's per-stage effort allowlist (CAL-V0-059). A
	// stage without an entry allows only "low".
	Efforts map[string][]string
	// StageWallMinutes bounds one supervised stage's configured wall time
	// (CAL-V0-060); empty means DefaultStageWallMinutes.
	StageWallMinutes wire.Count
}

// AllowsEffort reports whether the policy admits effort for stage. A nil
// policy, or a stage without an allowlist, admits only "low".
func (p *SupervisionPolicy) AllowsEffort(stage, effort string) bool {
	if p == nil || p.Efforts[stage] == nil {
		return effort == "low"
	}
	for _, e := range p.Efforts[stage] {
		if e == effort {
			return true
		}
	}
	return false
}

// StageWallSeconds is the largest configured wall time one stage may use.
func (p *SupervisionPolicy) StageWallSeconds() int {
	if p == nil || p.StageWallMinutes == "" {
		return DefaultStageWallMinutes * 60
	}
	return int(p.StageWallMinutes.Int()) * 60
}

func readSupervisionPolicy(r *wire.Reader) *SupervisionPolicy {
	r.Closed(wire.OptionalKeys(r.Value(), []string{"profile", "maxRepairCycles", "contextRequired", "program"}, "efforts", "stageWallMinutes")...)
	if r.Field("profile").String() != "taskman-codex-supervisor/0" || !r.Field("contextRequired").Bool() {
		r.Fail(wire.CodeUnsupported, "supervision profile/context")
	}
	p := &SupervisionPolicy{MaxRepairCycles: r.Field("maxRepairCycles").Count()}
	b := r.Field("program")
	b.Closed("turns", "wallClockMinutes", "inputTokens", "outputTokens")
	p.Turns = b.Field("turns").Count()
	p.WallClockMinutes = b.Field("wallClockMinutes").Count()
	p.InputTokens = b.Field("inputTokens").Size()
	p.OutputTokens = b.Field("outputTokens").Size()
	if p.MaxRepairCycles.Int() > 2 || p.Turns.Int() < 1 || p.WallClockMinutes.Int() < 1 {
		r.Fail(wire.CodeMalformed, "supervision bounds")
	}
	if wire.Has(r.Value(), "efforts") {
		e := r.Field("efforts")
		e.Closed(wire.OptionalKeys(e.Value(), nil, SupervisedStages...)...)
		p.Efforts = map[string][]string{}
		for _, stage := range SupervisedStages {
			if !wire.Has(e.Value(), stage) {
				continue
			}
			list := []string{}
			for _, v := range e.Field(stage).Array(len(SupervisedEfforts), false) {
				list = append(list, v.Enum(SupervisedEfforts...))
			}
			if len(list) == 0 {
				e.Fail(wire.CodeMalformed, "effort allowlist for %s is empty", stage)
			}
			p.Efforts[stage] = list
		}
		if len(p.Efforts) == 0 {
			e.Fail(wire.CodeMalformed, "efforts names no stage")
		}
	}
	if wire.Has(r.Value(), "stageWallMinutes") {
		p.StageWallMinutes = r.Field("stageWallMinutes").Count()
		if n := p.StageWallMinutes.Int(); n < 1 || n > MaxStageWallMinutes {
			r.Fail(wire.CodeMalformed, "stageWallMinutes outside 1..%d", MaxStageWallMinutes)
		}
	}
	return p
}
