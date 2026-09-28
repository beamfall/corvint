package cli

import (
	"context"
	"path/filepath"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// cutoverCommand runs `corvint-tasks cutover --decision REF` (CAL-V0-004): an
// OWNER switches the queue's canonical writer to NATIVE under one
// AUTHORITY_SWITCH receipt whose requestId is the decision reference. With
// `--execution --decision REF --qualification FILE` it records the execution
// cutover instead (CAL-V0-020): FILE is the `go test -json` run of the
// CAL-V0-019 suite, posted under one QUALIFICATION receipt.
func cutoverCommand(env Env, args []string) *wire.Result {
	cmd := []string{"cutover"}
	var run []byte
	switch {
	case len(args) == 2 && args[0] == "--decision":
	case len(args) == 5 && args[0] == "--execution" && args[1] == "--decision" && args[3] == "--qualification":
		file := args[4]
		if !filepath.IsAbs(file) {
			file = filepath.Join(env.Cwd, file)
		}
		raw, err := intent.ReadFile(file, wire.MaxGateOutputBytes)
		if err != nil {
			return errorResult(cmd, err)
		}
		run, args = append([]byte{}, raw...), args[1:3]
	default:
		return usage(cmd, "cutover requires --decision REF, or --execution --decision REF --qualification FILE")
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
	report, err := cutoverWrite(repo, actor, observed.Head.QueueID.Raw, decision, run, now)
	if err != nil {
		return errorResult(cmd, err)
	}
	return mutateResult(cmd, report)
}

// cutoverWrite commits the authority switch, or the execution cutover when a
// qualification run is given.
func cutoverWrite(repo *intent.Repository, actor mutation.Binding, queueID, decision string, run []byte, now wire.Timestamp) (*store.Report, error) {
	if run == nil {
		return store.Cutover(context.Background(), repo, actor, queueID, decision, now)
	}
	return store.ExecutionCutover(context.Background(), repo, actor, queueID, decision, run, now)
}
