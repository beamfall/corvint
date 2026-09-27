package cli

import (
	"context"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// cutoverCommand runs `corvint-tasks cutover --decision REF` (CAL-V0-004): an
// OWNER switches the queue's canonical writer to NATIVE under one
// AUTHORITY_SWITCH receipt whose requestId is the decision reference.
func cutoverCommand(env Env, args []string) *wire.Result {
	cmd := []string{"cutover"}
	if len(args) != 2 || args[0] != "--decision" {
		return usage(cmd, "cutover requires --decision REF")
	}
	decision := args[1]
	if _, err := mutation.ParseRequestID("decision", decision); err != nil {
		return errorResult(cmd, err)
	}
	actor, err := initActor("OWNER")
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
	report, err := store.Cutover(context.Background(), repo, actor, observed.Head.QueueID.Raw, decision, now)
	if err != nil {
		return errorResult(cmd, err)
	}
	return mutateResult(cmd, report)
}
