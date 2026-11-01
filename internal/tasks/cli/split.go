// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Ticket split (TLF-V0-884, TLF-V0-885, V1-3577): an ordered sequence of
// ordinary single-ticket mutations, child CREATE, one ATTACH_EVIDENCE per
// copied parent entry, then the parent REFINE, under request IDs derived
// from --request-id and one shared issuedAt. The parent changes last, so an
// interrupted split duplicates the criteria and a rerun replays each
// recorded step and completes the rest.
const (
	profileTicketSplit = "taskman-ticket-split/0"
	// splitSuffixBytes is the longest derived suffix, "/evidence/NN".
	splitSuffixBytes = len("/evidence/") + 2
)

// splitAfterStep, when set by a test, runs after each completed step; an
// error stops the split there, as an interruption would.
var splitAfterStep func(step string) error

type splitStep struct{ step, requestID, receipt string }

func ticketSplit(env Env, args []string) *wire.Result {
	cmd := []string{"ticket", "split"}
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return usage(cmd, "ticket split takes a ticket id first: ticket split ID --criteria N,M --expected-revision R --request-id Q")
	}
	arg := args[0]
	fl, set, res := listFlags(cmd, args[1:], []string{"criteria", "expected-revision", "request-id", "issued-at", "role"}, nil, []string{"dry-run"})
	if res != nil {
		return res
	}
	first := func(k string) string {
		if v := fl[k]; len(v) > 0 {
			return v[0]
		}
		return ""
	}
	dryRun := set["dry-run"]
	picked, err := parseCriteria(first("criteria"))
	if err != nil {
		return errorResult(cmd, err)
	}
	if first("expected-revision") == "" {
		return usage(cmd, "--expected-revision is required: it is the parent revision the criteria are numbered against")
	}
	requestID := first("request-id")
	if !dryRun && requestID == "" {
		return usage(cmd, "--request-id is required: the steps replay under <request-id>/create, /evidence/I and /refine")
	}
	if len(requestID)+splitSuffixBytes > wire.MaxRequestIDBytes {
		return usage(cmd, "--request-id is at most "+strconv.Itoa(wire.MaxRequestIDBytes-splitSuffixBytes)+" bytes, so each derived step ID fits")
	}
	role := first("role")
	if role == "" {
		role = "OWNER"
	}
	actor, err := initActor(role)
	if err != nil {
		return errorResult(cmd, err)
	}
	issuedAt := first("issued-at")
	if issuedAt == "" {
		issuedAt = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	} else if _, err := wire.ParseTimestamp("issuedAt", issuedAt); err != nil {
		return errorResult(cmd, err)
	}

	var parent *ticket.Record
	var repo *intent.Repository
	var queueID, doneChild string
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		parent, doneChild = nil, ""
		id, err := resolveTicketArg(rc, arg)
		if err != nil {
			return err
		}
		rec, ok := rc.store.Inventory.Get(id)
		if !ok {
			return wire.Errorf(wire.CodeMalformed, "argv", "ticket %s is not in the queue", prose(arg))
		}
		repo, queueID = rc.repo, rc.store.Queue.QueueID.Raw
		if !dryRun && !rc.journalAbsent {
			idx := journal.RequestIndex{Reader: journal.Reader{Source: journal.Native{StateDir: rc.repo.StateDir, PrimaryWorktree: rc.repo.PrimaryWorktree}, QueueID: rc.store.Queue.QueueID, PrimaryWorktree: rc.repo.PrimaryWorktree}}
			if _, found, err := idx.Lookup(requestID + "/refine"); err == nil && found {
				if idx.TicketID != id {
					return wire.Errorf(wire.CodeRequestIDConflict, "--request-id", "request %s/refine is already recorded for %s; use a new request ID", prose(requestID), idx.TicketID)
				}
				if _, found, err := idx.Lookup(requestID + "/create"); err == nil && found {
					doneChild = idx.TicketID
				}
				return nil
			}
		}
		parent = rec
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	if parent == nil {
		return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(wire.NewObject().Set("profile", wire.String(profileTicketSplit)).Set("child", wire.String(doneChild)).Set("alreadySplit", wire.Bool(true)))},
			Warnings: []string{"request " + requestID + "/refine is already recorded: the split is complete and nothing was written"}}
	}
	if err := checkSplitParent(parent, first("expected-revision"), picked); err != nil {
		return errorResult(cmd, err)
	}
	moved, kept := partitionCriteria(parent.AcceptanceCriteria, picked)
	child, err := mutation.CanonicalPayload(mutation.OpCreate, mutation.PayloadValue(splitChildPayload(parent, queueID, picked, moved)))
	if err != nil {
		return errorResult(cmd, err)
	}
	if dryRun {
		o := splitHead(parent, picked, kept, issuedAt)
		o.Set("dryRun", wire.Bool(true))
		o.Set("childPayload", child)
		o.Set("human", wire.Strings([]string{"split (dry run): " + strconv.Itoa(len(moved)) + " criterion(s) of " + parent.TicketID.Local + " to a new child, " + strconv.Itoa(len(kept)) + " kept; nothing was written"}))
		return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(o)}}
	}

	base := mutateFlags{role: role, issuedAt: issuedAt}
	issued, _ := wire.ParseTimestamp("issuedAt", issuedAt)
	var steps []splitStep
	run := func(step, operation, target, expected string, payload wire.Value) (*store.Report, *wire.Result) {
		f := base
		f.requestID, f.target, f.expected = requestID+"/"+step, target, expected
		envelope, err := buildEnvelope(operation, queueID, actor, f, payload, issued)
		if err != nil {
			return nil, errorResult(cmd, err)
		}
		now, _ := wire.ParseTimestamp("recordedAt", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
		report, err := store.Mutate(writerContext(), repo, actor, envelope, now)
		if err != nil {
			return nil, errorResult(cmd, err)
		}
		if report.Outcome.Outcome != mutation.OutcomeCompleted {
			r := mutateResult(cmd, report)
			r.Warnings = append(r.Warnings, "split stopped at step "+f.requestID+"; earlier steps stay recorded and a rerun with the same --request-id and --issued-at "+issuedAt+" replays them")
			return nil, r
		}
		steps = append(steps, splitStep{step: step, requestID: f.requestID, receipt: report.Receipt})
		if splitAfterStep != nil {
			if err := splitAfterStep(step); err != nil {
				return nil, errorResult(cmd, err)
			}
		}
		return report, nil
	}

	createFlags := base
	createFlags.requestID, createFlags.notDuplicateOf = requestID+"/create", []string{parent.TicketID.Raw}
	if refused := createDuplicateGuard(env, cmd, createFlags, child); refused != nil {
		return refused
	}
	report, stop := run("create", mutation.OpCreate, "", "", child)
	if stop != nil {
		return stop
	}
	childID := report.Ticket
	for i, ev := range splitEvidence(parent) {
		if report.Outcome.ResultingRevision == nil {
			return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "child", "child %s reported no resulting revision", childID))
		}
		payload, err := mutation.CanonicalPayload(mutation.OpAttachEvidence, ev)
		if err != nil {
			return errorResult(cmd, err)
		}
		if report, stop = run("evidence/"+strconv.Itoa(i), mutation.OpAttachEvidence, childID, string(*report.Outcome.ResultingRevision), payload); stop != nil {
			return stop
		}
	}
	body := ""
	if parent.Body != nil {
		body = strings.TrimRight(*parent.Body, "\n") + "\n\n"
	}
	body += "Split: criteria " + criteriaList(picked) + " of revision " + string(parent.Revision) + " moved to " + childID + "."
	refine, err := mutation.CanonicalPayload(mutation.OpRefine, wire.ObjectValue(wire.NewObject().Set("acceptanceCriteria", wire.Strings(kept)).Set("body", wire.String(body))))
	if err != nil {
		return errorResult(cmd, err)
	}
	if _, stop = run("refine", mutation.OpRefine, parent.TicketID.Raw, string(parent.Revision), refine); stop != nil {
		return stop
	}
	o := splitHead(parent, picked, kept, issuedAt)
	o.Set("child", wire.String(childID))
	rows := make([]wire.Value, 0, len(steps))
	for _, s := range steps {
		rows = append(rows, wire.ObjectValue(wire.NewObject().Set("step", wire.String(s.step)).Set("requestId", wire.String(s.requestID)).Set("receipt", wire.String(s.receipt))))
	}
	o.Set("steps", wire.Array(rows...))
	o.Set("human", wire.Strings([]string{"split: " + strconv.Itoa(len(moved)) + " criterion(s) of " + parent.TicketID.Local + " moved to " + childID + " in " + strconv.Itoa(len(steps)) + " receipt(s)"}))
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(o)}}
}

// parseCriteria reads --criteria as 1-based indexes, sorted; a repeat or a
// non-positive index refuses.
func parseCriteria(s string) ([]int, error) {
	if s == "" {
		return nil, wire.Errorf(wire.CodeMalformed, "--criteria", "--criteria is required: the 1-based acceptance criteria to move, e.g. --criteria 2,3")
	}
	var out []int
	for _, f := range strings.Split(s, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil || n < 1 {
			return nil, wire.Errorf(wire.CodeMalformed, "--criteria", "criterion %q is not a positive 1-based index", prose(f))
		}
		if slices.Contains(out, n) {
			return nil, wire.Errorf(wire.CodeMalformed, "--criteria", "criterion %d is named twice", n)
		}
		out = append(out, n)
	}
	slices.Sort(out)
	return out, nil
}

func checkSplitParent(rec *ticket.Record, expected string, picked []int) error {
	if rec.Status != ticket.StatusOpen && rec.Status != ticket.StatusDraft {
		return wire.Errorf(wire.CodeTicketState, "ticket", "ticket %s is %s: only an OPEN or DRAFT ticket splits", rec.TicketID.Raw, rec.Status)
	}
	if string(rec.Revision) != expected {
		return wire.Errorf(wire.CodeStaleTicket, "--expected-revision", "ticket %s is at revision %s, not %s; reread it and renumber the criteria", rec.TicketID.Raw, rec.Revision, prose(expected))
	}
	n := len(rec.AcceptanceCriteria)
	if picked[len(picked)-1] > n {
		return wire.Errorf(wire.CodeMalformed, "--criteria", "ticket %s has %d acceptance criteria; criterion %d is out of range", rec.TicketID.Raw, n, picked[len(picked)-1])
	}
	if len(picked) == n {
		return wire.Errorf(wire.CodeMalformed, "--criteria", "--criteria names every criterion of %s; a split keeps at least one on the parent", rec.TicketID.Raw)
	}
	return nil
}

func partitionCriteria(all []string, picked []int) (moved, kept []string) {
	for i, c := range all {
		if slices.Contains(picked, i+1) {
			moved = append(moved, c)
		} else {
			kept = append(kept, c)
		}
	}
	return moved, kept
}

func criteriaList(picked []int) string {
	parts := make([]string, len(picked))
	for i, n := range picked {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ",")
}

// splitChildPayload is the child CREATE: the parent's classification,
// effects, gates and dependencies, the moved criteria and a supersedes
// link to the parent. Copies are fresh so the payload aliases nothing.
func splitChildPayload(rec *ticket.Record, queueID string, picked []int, moved []string) *mutation.CreatePayload {
	title := truncateUTF8(rec.Title+" (split from "+rec.TicketID.Local+")", wire.MaxTitleBytes)
	body := "Split from " + rec.TicketID.Raw + " at revision " + string(rec.Revision) + " (criteria " + criteriaList(picked) + ")."
	if rec.Body != nil {
		body = truncateUTF8(body+"\n\n"+*rec.Body, wire.MaxBodyBytes)
	}
	parent := rec.TicketID
	effects := rec.Effects
	effects.TouchPaths = append([]string{}, rec.Effects.TouchPaths...)
	effects.Resources = append([]ticket.Resource{}, rec.Effects.Resources...)
	effects.VerdictPaths = slices.Clone(rec.Effects.VerdictPaths)
	return &mutation.CreatePayload{
		Title:              title,
		Body:               &body,
		Kind:               rec.Kind,
		Owner:              rec.Owner,
		Milestone:          rec.Milestone,
		Priority:           rec.Priority,
		Order:              rec.Order,
		Labels:             append([]string{}, rec.Labels...),
		Dependencies:       append([]ticket.Dependency{}, rec.Dependencies...),
		AcceptanceCriteria: moved,
		RequirementRefs:    append([]string{}, rec.RequirementRefs...),
		Source:             ticket.Source{Kind: "NATIVE", SourceQueueID: queueID},
		Effects:            effects,
		Capabilities:       append([]string{}, rec.Capabilities...),
		RequiredGates:      append([]string{}, rec.RequiredGates...),
		ExecutionClass:     rec.ExecutionClass,
		DueDate:            rec.DueDate,
		EstimateMinutes:    rec.EstimateMinutes,
		Supersedes:         &parent,
	}
}

// splitEvidence is one ATTACH_EVIDENCE payload per parent entry, each
// digest copied once: all copies land at the child's one acceptance
// revision, where a repeated digest would refuse DUPLICATE_ID.
func splitEvidence(rec *ticket.Record) []wire.Value {
	seen := map[wire.Digest]bool{}
	var out []wire.Value
	for _, e := range rec.AttachedEvidence {
		var ds []string
		for _, d := range e.Evidence {
			if !seen[d] {
				seen[d] = true
				ds = append(ds, string(d))
			}
		}
		if len(ds) == 0 {
			continue
		}
		reason := truncateUTF8("split from "+rec.TicketID.Local+": "+e.Reason, wire.AttachedEvidenceMaxReasonBytes)
		out = append(out, wire.ObjectValue(wire.NewObject().Set("evidence", wire.Strings(ds)).Set("reason", wire.String(reason))))
	}
	return out
}

func splitHead(rec *ticket.Record, picked []int, kept []string, issuedAt string) *wire.Object {
	o := wire.NewObject()
	o.Set("profile", wire.String(profileTicketSplit))
	o.Set("parent", wire.String(rec.TicketID.Raw))
	o.Set("expectedRevision", wire.String(string(rec.Revision)))
	o.Set("criteria", wire.String(criteriaList(picked)))
	o.Set("parentAcceptanceCriteria", wire.Strings(kept))
	o.Set("issuedAt", wire.String(issuedAt))
	return o
}
