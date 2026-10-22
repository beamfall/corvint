package cli

import (
	"errors"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// relocateCommand runs `corvint-tasks relocate --from OLD --request-id ID`
// from the new primary worktree (TRL-V0-001): one RELOCATE receipt re-pins
// head.primaryWorktree from OLD, which must be the recorded primary, to the
// resolved primary. With --dispatch-config FILE it holds every dispatcher
// lock under that configuration's state directory across the write and
// refuses when one is held (TRL-V0-003); without it the dispatcher check is
// reported NOT_OBSERVED.
func relocateCommand(env Env, args []string) *wire.Result {
	cmd := []string{"relocate"}
	role, request, from, config := "OWNER", "", "", ""
	values := map[string]*string{"--role": &role, "--request-id": &request, "--from": &from, "--dispatch-config": &config}
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		key := args[i]
		destination, ok := values[key]
		if !ok || seen[key] || i+1 >= len(args) {
			return usage(cmd, "relocate requires --from OLD and --request-id ID, with optional --role OWNER|OPERATOR and --dispatch-config FILE; duplicate and unknown flags refuse")
		}
		seen[key] = true
		i++
		*destination = args[i]
	}
	if from == "" {
		return usage(cmd, "relocate requires --from OLD, the primary worktree head.json records")
	}
	if _, err := mutation.ParseRequestID("requestId", request); err != nil {
		return errorResult(cmd, err)
	}
	if role != "OWNER" && role != "OPERATOR" {
		return usage(cmd, "relocation requires OWNER or OPERATOR")
	}
	actor, err := initActor(role)
	if err != nil {
		return errorResult(cmd, err)
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return errorResult(cmd, err)
	}
	dispatcher := "NOT_OBSERVED"
	if config != "" {
		c, _, refused := dispatchConfig(cmd, map[string]string{"--config": commandPath(env.Cwd, config)})
		if refused != nil {
			return refused
		}
		release, err := dispatch.HoldIdle(c)
		if errors.Is(err, dispatch.ErrDispatcherActive) {
			return errorResult(cmd, wire.Errorf(wire.CodeQuiescenceUnproved, c.StateDir, "%v; stop the dispatcher before relocating", err))
		}
		if err != nil {
			return errorResult(cmd, err)
		}
		defer release()
		dispatcher = "HELD_IDLE"
	}
	observed, _, err := writerProbe(nil, repo.StateDir)
	if err != nil {
		return errorResult(cmd, store.RelocationRefusal(err))
	}
	now, err := wire.ParseTimestamp("recordedAt", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	if err != nil {
		return errorResult(cmd, err)
	}
	report, err := store.Relocate(writerContext(), repo, actor, store.RelocateRequest{QueueID: observed.Head.QueueID.Raw, RequestID: request, From: commandPath(env.Cwd, from)}, now)
	if err != nil {
		return errorResult(cmd, err)
	}
	res := mutateResult(cmd, report)
	if dispatcher == "NOT_OBSERVED" {
		res.Warnings = append(res.Warnings, "no --dispatch-config was given, so whether a dispatcher holds the store was NOT_OBSERVED; stop every dispatcher before relocating")
	}
	return res
}
