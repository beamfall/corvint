package wire

// RoleEnv names the session principal class when the CLI Env.Role is empty
// (TAW-V0-001). It is a cooperative guard for agents, not authentication.
// It lives here so the corvint-mcp tasks group, which may import only this
// Tasks package, sets it on the corvint-tasks process it runs (TAW-V0-025).
const RoleEnv = "CORVINT_TASKS_ROLE"

// The principal classes of TAW-V0-001; the empty class is today's behavior.
const (
	ClassAgent    = "AGENT"
	ClassOperator = "OPERATOR"
	ClassOwner    = "OWNER"
)
