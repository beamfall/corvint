package cli

import (
	"context"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"sort"
)

func programRead(env Env, pending bool, args []string) *wire.Result {
	cmd := []string{"program", "show"}
	if pending {
		cmd = []string{"pending"}
	}
	if len(args) != 0 {
		return usage(cmd, "read accepts no arguments")
	}
	repo, e := intent.Resolve(env.Cwd)
	if e != nil {
		return errorResult(cmd, e)
	}
	items := []wire.Value{}
	if pending {
		attempts, e := store.ProgramAttempts(context.Background(), repo)
		if e != nil {
			return errorResult(cmd, e)
		}
		keys := []string{}
		for id := range attempts {
			keys = append(keys, id)
		}
		sort.Strings(keys)
		for _, id := range keys {
			a := attempts[id]
			if a.Phase == "WAITING" || a.Phase == "RETURNED" || a.Phase == "READY_FOR_INTEGRATION" {
				raw, _ := a.Encode()
				v, _ := wire.Parse(raw)
				items = append(items, v)
			}
		}
	} else {
		ps, e := store.ProgramRecords(context.Background(), repo)
		if e != nil {
			return errorResult(cmd, e)
		}
		for _, p := range ps {
			raw, _ := snapshot.EncodeProgramJSON(p)
			v, _ := wire.Parse(raw)
			items = append(items, v)
		}
	}
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Codes: []string{}, Items: items}
}
