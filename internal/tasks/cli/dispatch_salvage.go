package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// salvageRoleEvents bounds the events read for a stopped worker's role.
const salvageRoleEvents = 1000

// dispatchSalvage is `dispatch salvage --attempt A` (ATR-V0-126): it reads
// attempt A from the native store, takes its holder as the program's
// worker, and writes that worker's secret-screened hand-off note with the
// same bounds and screen as the hard-stop salvage (ATR-V0-104), whether the
// session still runs or something else stopped it. It never changes the
// attempt.
func dispatchSalvage(env Env, args []string) *wire.Result {
	cmd := []string{"dispatch", "salvage"}
	values, msg := dispatchFlags(env.Cwd, args, map[string]bool{"--program": true, "--config": true, "--attempt": true}, nil)
	if msg != "" {
		return usage(cmd, msg)
	}
	if values["--attempt"] == "" {
		return usage(cmd, "salvage requires --attempt")
	}
	c, res, _ := dispatchAuxConfig(cmd, "salvage", values)
	if res != nil {
		return res
	}
	repo, e := intent.Resolve(env.Cwd)
	if e != nil {
		return errorResult(cmd, e)
	}
	if work, e := intent.Resolve(c.WorkRoot); e != nil || work.StateDir != repo.StateDir {
		return usage(cmd, "workRoot must resolve to the same task store as the command's working directory")
	}
	var holder, ticket, phase string
	var checkpoint dispatch.Attempt
	rc, err := withStore(env, func(rc *readCtx) error {
		if _, err := snapshot.AttemptQueue(values["--attempt"]); err != nil {
			return err
		}
		path := "attempts/" + values["--attempt"] + ".json"
		proof, err := auditState(rc, path)
		if err != nil {
			return err
		}
		record, ok := proof.Records[path]
		if !ok || record.Sha256 == nil {
			return wire.Errorf(wire.CodeMalformed, "attemptId", "attempt does not exist")
		}
		a, err := snapshot.DecodeAttempt(record.Raw)
		if err != nil {
			return err
		}
		ticket, phase = a.TicketID.Raw, a.Phase
		checkpoint.Branch = a.Branch
		if a.WorktreePath != nil {
			checkpoint.Worktree = *a.WorktreePath
		}
		if a.Lease != nil {
			holder = a.Lease.Holder
		}
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	dir := dispatch.ProgramDir(c, values["--program"])
	if holder == "" || holder == "." || holder == ".." || strings.ContainsAny(holder, `/\`) {
		return usage(cmd, "the attempt's holder names no worker of this program")
	}
	l, e := dispatch.LoadLedger(dir, values["--program"])
	if e != nil {
		return errorResult(cmd, e)
	}
	var w dispatch.Worker
	state := "STOPPED"
	found := false
	for _, x := range l.Workers {
		if x.ID == holder {
			w, state, found = *x, "RUNNING", true
		}
	}
	if !found {
		// A stopped worker keeps its directory until it is retired.
		if fi, e := os.Lstat(filepath.Join(dir, "workers", holder)); e != nil || !fi.IsDir() {
			return usage(cmd, "the attempt's holder names no worker of this program")
		}
		w = dispatch.Worker{ID: holder, Role: dispatch.UsageUnknown}
		if events, e := dispatch.ReadEvents(dir, salvageRoleEvents); e == nil {
			for _, ev := range events {
				if ev.Kind == "launched" && ev.Worker == holder && ev.Role != "" {
					w.Role = ev.Role
				}
			}
		}
	}
	// The note names the attempt's ticket, which ATR-V0-105 matches.
	w.Ticket = ticket
	path, n, e := dispatch.SalvageWorker(dir, &w, time.Now(), dispatch.CheckpointCommit(store.ResolveCommit, c.WorkRoot, checkpoint)) // ERG-V0-028
	if e != nil {
		return errorResult(cmd, e)
	}
	o := wire.NewObject()
	o.Set("profile", wire.String("taskman-dispatch-salvage/0"))
	o.Set("attempt", wire.String(values["--attempt"]))
	o.Set("attemptPhase", wire.String(phase))
	o.Set("worker", wire.String(holder))
	o.Set("workerState", wire.String(state))
	o.Set("ticket", wire.String(ticket))
	o.Set("note", wire.String(path))
	o.Set("messages", wire.String(strconv.Itoa(len(n.Messages))))
	o.Set("files", wire.String(strconv.Itoa(len(n.Files))))
	o.Set("redactedLines", wire.String(strconv.Itoa(n.RedactedLines)))
	o.Set("truncated", wire.Bool(n.Truncated))
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Codes: []string{}, Items: []wire.Value{{Kind: wire.KindObject, Obj: o}}}
}
