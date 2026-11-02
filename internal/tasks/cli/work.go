package cli

import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"github.com/Beamfall/corvint/internal/tasks/worktrail"
)

// The role-scoped agent workflow verbs (TAW-V0). A lane reports a ticket
// transition as commit trailers (work start|deliver|block, offline, no
// write); the store host ingests them (work close-merged) and keeps release
// membership in step with the milestone (work release-sync); work finding
// dedupes a finding against the store before it files anything.

const workFamilyUsage = "expected start, deliver, block, finding, close-merged or release-sync"

func init() {
	lane := " TICKET... [--reason TEXT] [--lane LABEL] [--allow-unknown]   (offline; prints the Corvint-Ticket trailers and the git commit --trailer arguments; writes nothing; TAW-V0-003)"
	commandUsage["work start"] = "corvint-tasks work start" + lane
	commandUsage["work deliver"] = "corvint-tasks work deliver" + lane
	commandUsage["work block"] = "corvint-tasks work block TICKET... --reason TEXT [--lane LABEL] [--allow-unknown]   (offline; writes nothing; TAW-V0-003)"
	commandUsage["work finding"] = "corvint-tasks work finding (--payload FILE|- --request-id R [--new-despite TICKET] | --ingest FILE|- --request-id R | --supersede DUP --by KEEP --request-id R) [--issued-at TS] [--role ROLE]   (similar check first; a duplicate attaches as a recurrence; TAW-V0-010..012)"
	commandUsage["work close-merged"] = "corvint-tasks work close-merged --integration REF [--since COMMIT] [--lane-ref REF]... [--dry-run] [--role ROLE]   (store host, OPERATOR or OWNER class; ingests Corvint-Ticket trailers; TAW-V0-004..009)"
	commandUsage["work release-sync"] = "corvint-tasks work release-sync --release RELEASE --milestone LABEL --request-id ID [--dry-run] [--role ROLE]   (store host, OPERATOR or OWNER class; TAW-V0-014)"
}

func workCommand(env Env, args []string) *wire.Result {
	if len(args) == 0 {
		return usage([]string{"work"}, workFamilyUsage)
	}
	cmd := []string{"work", args[0]}
	switch args[0] {
	case "start":
		return workLane(env, cmd, worktrail.StateStarted, args[1:])
	case "deliver":
		return workLane(env, cmd, worktrail.StateDelivered, args[1:])
	case "block":
		return workLane(env, cmd, worktrail.StateBlocked, args[1:])
	case "finding":
		if res := workClassRefusal(env, cmd, false); res != nil {
			return res
		}
		return workFinding(env, cmd, args[1:])
	case "close-merged":
		if res := workClassRefusal(env, cmd, true); res != nil {
			return res
		}
		return workCloseMerged(env, cmd, args[1:])
	case "release-sync":
		if res := workClassRefusal(env, cmd, true); res != nil {
			return res
		}
		return workReleaseSync(env, cmd, args[1:])
	}
	return usage([]string{"work"}, workFamilyUsage)
}

// splitPositional returns the leading arguments before the first flag.
func splitPositional(args []string) ([]string, []string) {
	for i, a := range args {
		if strings.HasPrefix(a, "-") {
			return args[:i], args[i:]
		}
	}
	return args, nil
}

// localTicket reduces a full ticket ID to its local token; a local token is
// returned as given.
func localTicket(id string) string {
	if strings.HasPrefix(id, "ticket:") {
		return id[strings.LastIndex(id, ":")+1:]
	}
	return id
}

// workLane is work start|deliver|block (TAW-V0-003): it validates the
// transition, checks it against the local store copy when one loads, and
// prints the trailers. It writes nothing.
func workLane(env Env, cmd []string, state string, args []string) *wire.Result {
	if res := workClassRefusal(env, cmd, false); res != nil {
		return res
	}
	ids, rest := splitPositional(args)
	fl, set, res := listFlags(cmd, rest, []string{"reason", "lane"}, nil, []string{"allow-unknown"})
	if res != nil {
		return res
	}
	if len(ids) == 0 {
		return usage(cmd, "name at least one ticket: "+commandUsage[strings.Join(cmd, " ")])
	}
	t := worktrail.Transition{State: state}
	for _, id := range ids {
		t.Tickets = append(t.Tickets, localTicket(id))
	}
	if v := fl["reason"]; len(v) > 0 {
		t.Reason = v[0]
	}
	if v := fl["lane"]; len(v) > 0 {
		t.Lane = v[0]
	}
	if err := t.Validate(); err != nil {
		return errorResult(cmd, err)
	}
	var warnings []string
	st, why := localStore(env)
	if st == nil {
		warnings = append(warnings, "STORE_NOT_PRESENT: no local store copy loaded ("+prose(why)+"); the ticket IDs were checked for form only (TAW-V0-003)")
	} else {
		for _, local := range t.Tickets {
			rec, ok := st.Inventory.Get(qualifyTicket(st.Queue.QueueID.Raw, local))
			switch {
			case !ok && set["allow-unknown"]:
				warnings = append(warnings, "TAW_TICKET_UNKNOWN: "+local+" is not in the local store copy; --allow-unknown kept it")
			case !ok:
				return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "ticket", "TAW_TICKET_UNKNOWN: %s is not in the local store copy; check the ID, or pass --allow-unknown for a ticket filed after this copy (TAW-V0-003)", prose(local)))
			case rec.Status == ticket.StatusCompleted || rec.Status == ticket.StatusArchived:
				return errorResult(cmd, wire.Errorf(wire.CodeTicketState, "ticket", "TAW_TICKET_CLOSED: %s is %s in the local store copy; a lane transition names open work (TAW-V0-003)", local, rec.Status))
			}
		}
	}
	ga := t.GitArgs()
	gitArgs := make([]wire.Value, 0, len(ga))
	for _, a := range ga {
		gitArgs = append(gitArgs, wire.String(a))
	}
	tl := t.Lines()
	lines := make([]wire.Value, 0, len(tl))
	for _, l := range tl {
		lines = append(lines, wire.String(l))
	}
	tickets := make([]wire.Value, 0, len(t.Tickets))
	for _, id := range t.Tickets {
		tickets = append(tickets, wire.String(id))
	}
	item := wire.NewObject().
		Set("state", wire.String(state)).
		Set("tickets", wire.Array(tickets...)).
		Set("trailers", wire.Array(lines...)).
		Set("gitArgs", wire.Array(gitArgs...)).
		Set("storeChecked", wire.Bool(st != nil))
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(item)}, Warnings: warnings}
}

// localStore loads the intent store under cwd without the snapshot
// protocol: a lane host may have no state directory. nil and the reason
// when none loads.
func localStore(env Env) (*intent.Store, string) {
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return nil, err.Error()
	}
	st, err := intent.Load(repo.IntentRoot())
	if err != nil {
		return nil, err.Error()
	}
	return st, ""
}
