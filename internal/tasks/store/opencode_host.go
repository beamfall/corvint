package store

import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// opencodeStageArgv is the pinned OpenCode invocation for one supervised
// stage (CAL-V0-077). --standalone keeps the session server, and with it
// every in-process plugin, inside the supervisor-owned process group instead
// of a shared background service. The stage effort is the model variant; the
// prompt arrives on standard input; a nonempty session continues that
// session. Permissions come from opencodeStageEnv, never from --auto.
func opencodeStageArgv(c ProgramConfig, stage, session string) []string {
	argv := []string{"run", "--standalone", "--format", "json", "--model", c.Model + "#" + c.StageEffort(stage)}
	if session != "" {
		argv = append(argv, "--session", session)
	}
	return argv
}

// opencodeStageEnv adds the OpenCode controls to the inherited stage
// environment (CAL-V0-077): no self-update of the pinned binary, no project
// configuration from the untrusted worktree, and an inline permission
// configuration that denies sub-agents and directories outside the worktree
// in every stage and file edits in the read-only stages. Rules OpenCode
// would ask about are rejected, because the run never passes --auto.
func opencodeStageEnv(env []string, stage string) []string {
	permission := `{"permission":{"edit":"deny","external_directory":"deny","task":"deny"}}`
	if stage == "implement" {
		permission = `{"permission":{"external_directory":"deny","task":"deny"}}`
	}
	return append(env, "OPENCODE_DISABLE_AUTOUPDATE=1", "OPENCODE_DISABLE_PROJECT_CONFIG=1", "OPENCODE_CONFIG_CONTENT="+permission)
}

// checkOpenCodeConfig refuses an OpenCode program whose model is not one
// provider/model reference without a variant, since the stage effort becomes
// the variant, or which spans repositories, since every stage denies
// directories outside its worktree (CAL-V0-076).
func checkOpenCodeConfig(c ProgramConfig) error {
	provider, model, ok := strings.Cut(c.Model, "/")
	if !ok || provider == "" || model == "" || strings.Contains(c.Model, "#") {
		return wire.Errorf(wire.CodeUnsupported, "model", "opencode model %q must be provider/model without a variant", c.Model)
	}
	if len(c.Repositories) > 0 {
		return wire.Errorf(wire.CodeUnsupported, "repositories", "opencode host does not supervise multi-repository programs")
	}
	return nil
}
