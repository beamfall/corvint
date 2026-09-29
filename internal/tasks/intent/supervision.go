package intent

import "github.com/Beamfall/corvint/internal/tasks/wire"

type SupervisionPolicy struct {
	MaxRepairCycles, Turns, WallClockMinutes wire.Count
	InputTokens, OutputTokens                wire.Size
}

func readSupervisionPolicy(r *wire.Reader) *SupervisionPolicy {
	r.Closed("profile", "maxRepairCycles", "contextRequired", "program")
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
	return p
}
