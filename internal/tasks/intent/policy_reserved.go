package intent

// ReservedPolicyFields names the taskman-policy/0 fields this binary parses
// but enforces nowhere, each only when the policy sets it to a non-default
// value (TCL-V0-802, V1-3948): reviewLane.required, docsLane.required,
// cemRequired and ocmRequired when true, and capacity.classes when
// non-empty. Setting one changes no claim, submit, gate or completion
// outcome, so the CLI warns instead of letting the policy read as enforced.
func ReservedPolicyFields(p *Policy) []string {
	if p == nil {
		return nil
	}
	var out []string
	for _, f := range []struct {
		name string
		set  bool
	}{
		{"reviewLane.required", p.ReviewLaneRequired},
		{"docsLane.required", p.DocsLaneRequired},
		{"cemRequired", p.CemRequired},
		{"ocmRequired", p.OcmRequired},
		{"capacity.classes", len(p.Classes) > 0},
	} {
		if f.set {
			out = append(out, f.name)
		}
	}
	return out
}

// ReservedPolicyWarning is the one warning for the reserved fields a policy
// sets, or "" when it sets none.
func ReservedPolicyWarning(p *Policy) string {
	fields := ReservedPolicyFields(p)
	if len(fields) == 0 {
		return ""
	}
	msg := "policy sets reserved field(s) "
	for i, f := range fields {
		if i > 0 {
			msg += ", "
		}
		msg += f
	}
	return msg + ": this binary parses them but enforces none of them, so they change no claim, submit, gate or completion outcome (TCL-V0-802)"
}
