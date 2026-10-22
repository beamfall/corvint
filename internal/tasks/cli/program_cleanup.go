package cli

import (
	"encoding/json"
	"os"
	"os/signal"
	"syscall"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// programCleanup removes a recorded program's registered stage worktrees once
// no live attempt can reference them, one item per worktree it removed or
// kept with the reason (ATR-V0-296).
func programCleanup(env Env, args []string) *wire.Result {
	cmd := []string{"program", "cleanup"}
	values := map[string]string{}
	for i := 0; i < len(args); i += 2 {
		if i+1 >= len(args) || values[args[i]] != "" || args[i] != "--program" && args[i] != "--config" {
			return usage(cmd, "cleanup accepts --program ID and --config FILE once each")
		}
		values[args[i]] = args[i+1]
	}
	if values["--program"] == "" || values["--config"] == "" {
		return usage(cmd, "cleanup requires --program and --config")
	}
	c, bad := readProgramConfig(env, cmd, values["--config"])
	if bad != nil {
		return bad
	}
	repo, e := intent.Resolve(env.Cwd)
	if e != nil {
		return errorResult(cmd, e)
	}
	actor, e := initActor("OPERATOR")
	if e != nil {
		return errorResult(cmd, e)
	}
	ctx, cancel := signal.NotifyContext(writerContext(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	results, e := store.CleanupProgram(ctx, repo, actor, values["--program"], c)
	if e != nil {
		return errorResult(cmd, e)
	}
	items := []wire.Value{}
	for _, r := range results {
		raw, _ := json.Marshal(r)
		v, e := wire.Parse(raw)
		if e != nil {
			return errorResult(cmd, e)
		}
		items = append(items, v)
	}
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Codes: []string{}, Items: items}
}
