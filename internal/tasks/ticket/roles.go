package ticket

import "github.com/Beamfall/corvint/internal/tasks/wire"

// Stages and StageRoles are the closed requiredRoles keys and role values.
var (
	Stages     = []string{"implement", "review", "integrate"}
	StageRoles = []string{"BUILDER", "REVIEWER", "VERIFIER", "REPAIR", "DOCS"}
)

func ReadStageRoles(r *wire.Reader) map[string][]string {
	r.Closed(Stages...)
	out := map[string][]string{}
	for _, stage := range Stages {
		f := r.Field(stage)
		out[stage] = f.Strings(-1, false, func(x *wire.Reader) string {
			return x.Enum(StageRoles...)
		})
		if len(out[stage]) == 0 {
			f.Fail(wire.CodeMalformed, "stage requires a role")
		}
	}
	return out
}
func StageRolesValue(roles map[string][]string) wire.Value {
	o := wire.NewObject()
	for k, v := range roles {
		o.Set(k, wire.Strings(v))
	}
	return wire.ObjectValue(o)
}
