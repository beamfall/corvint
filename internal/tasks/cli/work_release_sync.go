package cli

import (
	"sort"

	"github.com/Beamfall/corvint/internal/tasks/release"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// releaseSyncFlags are the work release-sync flags (TAW-V0-016).
var releaseSyncFlags = workFlagSpec{single: []string{"release", "milestone", "request-id", "issued-at", "role"}, bools: []string{"dry-run"}}

// workReleaseSync is TAW-V0-014: it adds every OPEN ticket whose milestone
// is M and that is not a member of release R through one release membership
// mutation, and reports members whose milestone is not M without removing
// them. In sync it writes nothing, so a rerun is a no-op.
func workReleaseSync(env Env, cmd []string, args []string) *wire.Result {
	fl, set, res := releaseSyncFlags.parse(cmd, args)
	if res != nil {
		return res
	}
	first := func(k string) string {
		if v := fl[k]; len(v) > 0 {
			return v[0]
		}
		return ""
	}
	rel, milestone, requestID := first("release"), first("milestone"), first("request-id")
	if rel == "" || milestone == "" || (requestID == "" && !set["dry-run"]) {
		return usage(cmd, "--release, --milestone and --request-id are required: "+commandUsage["work release-sync"])
	}
	if _, err := wire.ParseLabel("--milestone", milestone); err != nil {
		return errorResult(cmd, err)
	}
	var missing, outside []string
	var revision string
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		missing, outside = nil, nil
		var r *release.Record
		for _, x := range rc.store.Releases {
			if x.ReleaseID == rel {
				r = x
			}
		}
		if r == nil {
			return wire.Errorf(wire.CodeMalformed, "--release", "release %s is not in the store", prose(rel))
		}
		revision = string(r.Revision)
		member := make(map[string]bool, len(r.TicketIDs))
		for _, id := range r.TicketIDs {
			member[id.Raw] = true
			t, ok := rc.store.Inventory.Get(id.Raw)
			if ok && (t.Milestone == nil || *t.Milestone != milestone) {
				outside = append(outside, id.Raw)
			}
		}
		for _, id := range rc.store.Inventory.Sorted() {
			t, _ := rc.store.Inventory.Get(id)
			if t.Status == ticket.StatusOpen && t.Milestone != nil && *t.Milestone == milestone && !member[id] {
				missing = append(missing, id)
			}
		}
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	sort.Strings(missing)
	sort.Strings(outside)
	var warnings []string
	written := false
	var receipt wire.Value = wire.Null()
	if len(missing) > 0 && !set["dry-run"] {
		margs := []string{"--request-id", requestID, "--target", rel, "--expected-revision", revision}
		for _, id := range missing {
			margs = append(margs, "--add", id)
		}
		if v := first("issued-at"); v != "" {
			margs = append(margs, "--issued-at", v)
		}
		if v := first("role"); v != "" {
			margs = append(margs, "--role", v)
		}
		out := releaseCommand(env, "membership", margs)
		if out.Outcome != wire.OutcomeOK {
			out.Command = cmd
			return out
		}
		written = true
		warnings = append(warnings, out.Warnings...)
		if len(out.Items) > 0 {
			if v, ok := out.Items[0].Obj.Get("receipt"); ok {
				receipt = v
			}
		}
	}
	add := "ADDED"
	if set["dry-run"] {
		add = "WOULD_ADD"
	}
	summary := wire.NewObject().
		Set("release", wire.String(rel)).
		Set("milestone", wire.String(milestone)).
		Set("added", wire.Strings(missing)).
		Set("outsideMilestone", wire.Strings(outside)).
		Set("written", wire.Bool(written)).
		Set("receipt", receipt)
	items := []wire.Value{wire.ObjectValue(summary)}
	for _, id := range missing {
		items = append(items, workRow(id, add, ""))
	}
	for _, id := range outside {
		items = append(items, workRow(id, "OUTSIDE_MILESTONE", "member of "+rel+" whose milestone is not "+milestone+"; not removed"))
	}
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: items, Warnings: warnings}
}

// workRow is one per-ticket disposition row of a store-host work verb.
func workRow(id, disposition, reason string) wire.Value {
	o := wire.NewObject().Set("ticket", wire.String(id)).Set("disposition", wire.String(disposition))
	if reason != "" {
		o.Set("reason", wire.String(reason))
	}
	return wire.ObjectValue(o)
}
