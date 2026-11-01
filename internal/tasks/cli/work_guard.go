package cli

import (
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// RoleEnv names the session principal class when Env.Role is empty
// (TAW-V0-001). It is a cooperative guard for agents, not authentication:
// the store role (--role) and the trusted actor binding still decide
// permission.
const RoleEnv = "CORVINT_TASKS_ROLE"

// The principal classes of TAW-V0-001; the empty class is today's behavior.
const (
	ClassAgent    = "AGENT"
	ClassOperator = "OPERATOR"
	ClassOwner    = "OWNER"
)

// completeReasonCodes and archiveReasonCodes are the fixed reason codes an
// AGENT session must lead a COMPLETE_MANUAL or ARCHIVE reason with
// (TAW-V0-013).
var (
	completeReasonCodes = []string{"ALREADY_DELIVERED", "INTEGRATED", "OWNER_DIRECTED", "NO_CODE_CHANGE"}
	archiveReasonCodes  = []string{"DUPLICATE", "SUPERSEDED", "OBSOLETE", "OWNER_DIRECTED", "NOT_REPRODUCIBLE"}
)

// taskClass is the session class: Env.Role, else RoleEnv (TAW-V0-001).
func taskClass(env Env) (string, error) {
	c := env.Role
	if c == "" {
		c = os.Getenv(RoleEnv)
	}
	switch c {
	case "", ClassAgent, ClassOperator, ClassOwner:
		return c, nil
	}
	return "", wire.Errorf(wire.CodeMalformed, RoleEnv, "TAW_ROLE_UNKNOWN: class %s is not AGENT, OPERATOR or OWNER (TAW-V0-001)", prose(c))
}

func reasonCodes(op string) []string {
	switch op {
	case mutation.OpCompleteManual:
		return completeReasonCodes
	case mutation.OpArchive:
		return archiveReasonCodes
	}
	return nil
}

func reasonPrefix(code string) string { return "[TAW-REASON:" + code + "] " }

// reasonCoded reports whether reason leads with a listed code for op and
// carries nonblank text after it.
func reasonCoded(op, reason string) bool {
	for _, c := range reasonCodes(op) {
		if rest, ok := strings.CutPrefix(reason, reasonPrefix(c)); ok && strings.TrimSpace(rest) != "" {
			return true
		}
	}
	return false
}

func payloadReason(v wire.Value) string {
	if v.Kind != wire.KindObject || v.Obj == nil {
		return ""
	}
	if r, ok := v.Obj.Get("reason"); ok && r.Kind == wire.KindString {
		return r.Str
	}
	return ""
}

// withReasonCode is `--reason-code CODE`: it leads the payload reason with
// the code's prefix, once (TAW-V0-013).
func withReasonCode(op, code string, payload wire.Value) (wire.Value, error) {
	if !contains(reasonCodes(op), code) {
		return payload, wire.Errorf(wire.CodeMalformed, "--reason-code", "TAW_REASON_CODE_UNKNOWN: %s is not one of %s (TAW-V0-013)", prose(code), strings.Join(reasonCodes(op), ", "))
	}
	reason := payloadReason(payload)
	if strings.HasPrefix(reason, reasonPrefix(code)) || payload.Obj == nil {
		return payload, nil
	}
	obj := wire.NewObject()
	for _, k := range payload.Obj.Keys {
		obj.Set(k, payload.Obj.Vals[k])
	}
	obj.Set("reason", wire.String(reasonPrefix(code)+reason))
	return mutation.CanonicalPayload(op, wire.ObjectValue(obj))
}

// agentGuard runs at the §5.2 writer entry, so every caller is covered
// (TAW-V0-013): nil unless the session class is unknown, or AGENT and the
// operation edits status outside the work verbs' paths.
func agentGuard(env Env, op string, payload wire.Value) error {
	class, err := taskClass(env)
	if err != nil || class != ClassAgent {
		return err
	}
	switch op {
	case mutation.OpCompleteManual, mutation.OpArchive:
		if !reasonCoded(op, payloadReason(payload)) {
			return wire.Errorf(wire.CodeMalformed, "reason", "TAW_REASON_CODE_REQUIRED: an AGENT session's %s reason must lead with [TAW-REASON:CODE] and text, CODE one of %s; pass --reason-code CODE (TAW-V0-013)", op, strings.Join(reasonCodes(op), ", "))
		}
	case mutation.OpReopen, mutation.OpRestore, mutation.OpHold, mutation.OpReleaseHold:
		return wire.Errorf(wire.CodeMalformed, RoleEnv, "TAW_ROLE_NOT_PERMITTED: an AGENT session may not run %s; report the transition with a work verb or ask an operator (TAW-V0-013)", op)
	}
	return nil
}

// workClassRefusal refuses the store-host work verbs under class AGENT
// (TAW-V0-009), and every work verb under an unknown class (TAW-V0-001).
func workClassRefusal(env Env, cmd []string, hostOnly bool) *wire.Result {
	class, err := taskClass(env)
	if err != nil {
		return errorResult(cmd, err)
	}
	if hostOnly && class == ClassAgent {
		return errorResult(cmd, wire.Errorf(wire.CodeMalformed, RoleEnv, "TAW_ROLE_NOT_PERMITTED: %s runs on the store host under OPERATOR or OWNER, not AGENT (TAW-V0-009)", strings.Join(cmd, " ")))
	}
	return nil
}
