package cli

import (
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"os/signal"
	"syscall"
)

// poolSweepCommand owns foreground signal cancellation for the public route.
func poolSweepCommand(env Env, args []string) *wire.Result {
	cmd := []string{"pool", "sweep"}
	values := map[string]string{}
	for i := 0; i < len(args); i += 2 {
		key := args[i]
		if i+1 >= len(args) || (key != "--member" && key != "--request-id" && key != "--timeout-seconds" && key != "--role") || values[key] != "" || args[i+1] == "" {
			return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "pool sweep", "invalid flags"))
		}
		values[key] = args[i+1]
	}
	if values["--request-id"] == "" || values["--timeout-seconds"] == "" {
		return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "pool sweep", "request-id and explicit timeout required"))
	}
	timeout, e := wire.ParseCount("timeoutSeconds", values["--timeout-seconds"])
	if e != nil {
		return errorResult(cmd, e)
	}
	repo, e := intent.Resolve(env.Cwd)
	if e != nil {
		return errorResult(cmd, e)
	}
	state, e := snapshotQueueForSweep(repo)
	if e != nil {
		return errorResult(cmd, e)
	}
	role := values["--role"]
	if role == "" {
		role = "OPERATOR"
	}
	actor, e := initActor(role)
	if e != nil {
		return errorResult(cmd, e)
	}
	ctx, stop := signal.NotifyContext(writerContext(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	report, e := store.PoolSweep(ctx, repo, actor, store.PoolSweepChoice{QueueID: state, RequestID: values["--request-id"], Root: env.Cwd, Member: values["--member"], TimeoutSeconds: timeout})
	if e != nil {
		return errorResult(cmd, e)
	}
	result := leaseResult(cmd, report.Report)
	if report.Pending {
		result.Outcome = wire.OutcomeNotRun
		result.Warnings = append(result.Warnings, "Original sweep is pending; no command was repeated.")
		if len(result.Items) > 0 && result.Items[0].Obj != nil {
			result.Items[0].Obj.Set("outcome", wire.String("PENDING"))
			result.Items[0].Obj.Set("pending", wire.Bool(true))
			if report.Report.Outcome.ReceiptSeq != nil {
				result.Items[0].Obj.Set("receiptSeq", wire.String(string(*report.Report.Outcome.ReceiptSeq)))
			}
		}
	}
	if len(report.Result) > 0 {
		v, e := wire.Parse(report.Result)
		if e != nil {
			return errorResult(cmd, e)
		}
		result.Items = append(result.Items, v)
	}
	return result
}

func snapshotQueueForSweep(repo *intent.Repository) (string, error) {
	probe, e := snapshot.Probe(repo.StateDir)
	if e != nil {
		return "", e
	}
	return probe.Head.QueueID.Raw, nil
}
