package ticket

import "github.com/Beamfall/corvint/internal/tasks/wire"

func ReadStageRoles(r *wire.Reader) map[string][]string {
	r.Closed("implement", "review", "integrate")
	out := map[string][]string{}
	for _, stage := range []string{"implement", "review", "integrate"} {
		f := r.Field(stage)
		out[stage] = f.Strings(-1, false, func(x *wire.Reader) string {
			return x.Enum([]string{"BUILDER", "REVIEWER", "VERIFIER", "REPAIR", "DOCS"}...)
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
