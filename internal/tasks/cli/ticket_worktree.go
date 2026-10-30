package cli

import (
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// worktreeCommand runs `worktree ensure <ticket> [--path PATH]` (TWT-V0-002..
// 007): it creates or reuses the ticket's worktree on its pattern branch in
// the policy's declared repository. It writes no Tasks state; its only effect
// is `git worktree add` when the worktree does not exist.
func worktreeCommand(env Env, args []string) *wire.Result {
	if len(args) == 0 || args[0] != "ensure" {
		return usage([]string{"worktree"}, "expected ensure")
	}
	cmd := []string{"worktree", "ensure"}
	args = args[1:]
	if len(args) == 0 || strings.HasPrefix(args[0], "--") {
		return usage(cmd, "the first argument is the ticket id or local token")
	}
	target := args[0]
	requested := ""
	if res := pairFlags(cmd, args[1:], map[string]*string{"--path": &requested}, nil, nil); res != nil {
		return res
	}
	if strings.ContainsAny(requested, "\x00\n\r") {
		return usage(cmd, "--path takes a path without line breaks")
	}
	var raw string
	var wt *store.TicketWorktree
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		p := rc.store.Policy.Worktrees
		if p == nil {
			return wire.Errorf(wire.CodePrerequisiteUnsatisfied, "/worktrees", "the policy declares no worktrees section; add worktrees {repository, base, branchPattern} with policy update")
		}
		id, err := resolveTicketArg(rc, target)
		if err != nil {
			return err
		}
		rec, ok := rc.store.Inventory.Get(id)
		if !ok {
			return wire.Errorf(wire.CodeMissingEvidence, "argv", "queue holds no ticket %s", id)
		}
		raw = rec.TicketID.Raw
		wt, err = store.EnsureTicketWorktree(rc.repo, p, rec.TicketID.Local, requested)
		return err
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	res := success(cmd, rc)
	res.Items = []wire.Value{wire.ObjectValue(wire.NewObject().Set("ticketId", wire.String(raw)).
		Set("repository", wire.ObjectValue(wire.NewObject().Set("root", wire.String(wt.RepositoryRoot)).Set("sha256", wire.String(string(wt.RepositorySha256))))).
		Set("branch", wire.String(wt.Branch)).Set("path", wire.String(wt.Path)).
		Set("base", wire.String(wt.Base)).Set("baseCommit", wire.String(wt.BaseCommit)).Set("baseAncestor", wire.Bool(wt.BaseAncestor)).
		Set("head", wire.String(wt.Head)).Set("created", wire.Bool(wt.Created)).
		Set("dirty", wire.Bool(wt.Dirty)).Set("dirtyEntries", wire.String(strconv.Itoa(wt.DirtyEntries))))}
	if wt.Dirty {
		res.Warnings = append(res.Warnings, "the worktree has "+strconv.Itoa(wt.DirtyEntries)+" uncommitted or untracked entries; ensure left them untouched")
	}
	if !wt.BaseAncestor {
		res.Warnings = append(res.Warnings, "the declared base is not an ancestor of the worktree head")
	}
	return res
}
