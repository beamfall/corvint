package cli

import (
	"context"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func barrierCommand(env Env, verb string, args []string) *wire.Result {
	cmd := []string{verb}
	role, request := "OWNER", ""
	values := map[string]*string{"--role": &role, "--request-id": &request}
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		key := args[i]
		destination, ok := values[key]
		if !ok || seen[key] || i+1 >= len(args) {
			return usage(cmd, "pause/unpause require --request-id ID and optional --role OWNER|OPERATOR; duplicate and unknown flags refuse")
		}
		seen[key] = true
		i++
		*destination = args[i]
	}
	if _, err := mutation.ParseRequestID("requestId", request); err != nil {
		return errorResult(cmd, err)
	}
	if role != "OWNER" && role != "OPERATOR" {
		return usage(cmd, "barrier changes require OWNER or OPERATOR")
	}
	actor, err := initActor(role)
	if err != nil {
		return errorResult(cmd, err)
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return errorResult(cmd, err)
	}
	observed, err := snapshot.Probe(repo.StateDir)
	if err != nil {
		return errorResult(cmd, err)
	}
	now, err := wire.ParseTimestamp("recordedAt", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	if err != nil {
		return errorResult(cmd, err)
	}
	operation := transaction.Pause
	if verb == "unpause" {
		operation = transaction.Unpause
	}
	report, err := store.Barrier(context.Background(), repo, actor, store.BarrierRequest{QueueID: observed.Head.QueueID.Raw, RequestID: request, Operation: operation}, now)
	if err != nil {
		return errorResult(cmd, err)
	}
	return mutateResult(cmd, report)
}
