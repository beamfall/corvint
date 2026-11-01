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
// ordinary single-ticket mutations, child CREATE, a child REFINE when the
// parent has executionPrerequisites, one ATTACH_EVIDENCE per copied parent
// entry, then the parent REFINE, under request IDs derived from --request-id
// and one shared issuedAt. The parent changes last, guarded under the writer
// lock by the child's state, so an interrupted split duplicates the criteria
// and a rerun replays each recorded step and completes the rest.
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
		return usage(cmd, "--request-id is required: the steps replay under <request-id>/create, /child, /evidence/I and /refine")
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
	read := func() (*readCtx, error) {
		return withInventoryStore(env, func(rc *readCtx) error {
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
				entry, found, err := idx.Lookup(requestID + "/refine")
				if err != nil {
					return err
				}
				if found {
					doneChild, err = recordedSplit(rc, &idx, entry, rec, requestID, first("expected-revision"), picked)
					return err
				}
			}
			parent = rec
			return nil
		})
	}
	rc, err := read()
	var warnings []string
	if wire.CodeOf(err) == wire.CodeRedoPending && !dryRun && rc != nil && rc.repo != nil {
		// A receipt linked before an interruption advanced head.json is
		// settled under the writer lock first, as store.Mutate would.
		var redone bool
		if redone, err = store.Settle(writerContext(), rc.repo); err == nil {
			if redone {
				warnings = append(warnings, "a receipt left pending by an interrupted run was completed before this mutation (§5.2 redo)")
			}
			rc, err = read()
		}
	}
	if err != nil {
		return failure(cmd, rc, err)
	}
	if parent == nil {
		return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(wire.NewObject().Set("profile", wire.String(profileTicketSplit)).Set("child", wire.String(doneChild)).Set("alreadySplit", wire.Bool(true)))},
			Warnings: append(warnings, "request "+requestID+"/refine is already recorded for this split: it is complete and nothing was written")}
	}
	if err := checkSplitParent(parent, first("expected-revision"), picked); err != nil {
		return errorResult(cmd, err)
	}
	moved, kept := partitionCriteria(parent.AcceptanceCriteria, picked)
	// The parent's final payload is checked before anything is written, with
	// the longest child ID standing in for the one the CREATE assigns.
	if _, err := splitParentRefine(parent, picked, kept, splitLongestChildID(parent.TicketID)); err != nil {
		return errorResult(cmd, err)
	}
	createPayload, childRefine := splitChildPayload(parent, queueID, picked, moved), wire.Value{}
	if len(parent.ExecutionPrerequisites) > 0 {
		// CREATE carries no executionPrerequisites, so the child is created
		// DRAFT, without criteria, and opens only with the REFINE that sets
		// its criteria and prerequisites together.
		if parent.Kind == "MANUAL" || parent.Kind == "EXTERNAL" {
			return errorResult(cmd, wire.Errorf(wire.CodeTicketState, "ticket", "ticket %s is %s with executionPrerequisites: its child would open at CREATE, before the prerequisites are set; split it by hand", parent.TicketID.Raw, parent.Kind))
		}
		createPayload.AcceptanceCriteria = nil
		if childRefine, err = mutation.CanonicalPayload(mutation.OpRefine, wire.ObjectValue(wire.NewObject().Set("acceptanceCriteria", wire.Strings(moved)).Set("executionPrerequisites", ticket.PrerequisitesValue(parent.ExecutionPrerequisites)))); err != nil {
			return errorResult(cmd, err)
		}
	}
	child, err := mutation.CanonicalPayload(mutation.OpCreate, mutation.PayloadValue(createPayload))
	if err != nil {
		return errorResult(cmd, err)
	}
	if dryRun {
		o := splitHead(parent, picked, kept, issuedAt)
		o.Set("dryRun", wire.Bool(true))
		o.Set("childPayload", child)
		if childRefine.Kind != wire.KindNull {
			o.Set("childRefinePayload", childRefine)
		}
		o.Set("human", wire.Strings([]string{"split (dry run): " + strconv.Itoa(len(moved)) + " criterion(s) of " + parent.TicketID.Local + " to a new child, " + strconv.Itoa(len(kept)) + " kept; nothing was written"}))
		return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(o)}}
	}

	base := mutateFlags{role: role, issuedAt: issuedAt}
	issued, _ := wire.ParseTimestamp("issuedAt", issuedAt)
	var steps []splitStep
	run := func(step, operation, target, expected string, payload wire.Value, guard *store.TicketGuard) (*store.Report, *wire.Result) {
		f := base
		f.requestID, f.target, f.expected = requestID+"/"+step, target, expected
		envelope, err := buildEnvelope(operation, queueID, actor, f, payload, issued)
		if err != nil {
			return nil, errorResult(cmd, err)
		}
		now, _ := wire.ParseTimestamp("recordedAt", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
		report, err := store.MutateGuarded(writerContext(), repo, actor, envelope, now, guard)
		if err != nil {
			r := errorResult(cmd, err)
			r.Warnings = append(r.Warnings, "split stopped at step "+f.requestID+"; earlier steps stay recorded")
			return nil, r
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
	report, stop := run("create", mutation.OpCreate, "", "", child, nil)
	if stop != nil {
		return stop
	}
	childID := report.Ticket
	if childRefine.Kind != wire.KindNull {
		if report.Outcome.ResultingRevision == nil {
			return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "child", "child %s reported no resulting revision", childID))
		}
		if report, stop = run("child", mutation.OpRefine, childID, string(*report.Outcome.ResultingRevision), childRefine, nil); stop != nil {
			return stop
		}
	}
	acceptance := report.Outcome.ResultingAcceptanceRevision
	for i, ev := range splitEvidence(parent) {
		if report.Outcome.ResultingRevision == nil {
			return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "child", "child %s reported no resulting revision", childID))
		}
		payload, err := mutation.CanonicalPayload(mutation.OpAttachEvidence, ev)
		if err != nil {
			return errorResult(cmd, err)
		}
		if report, stop = run("evidence/"+strconv.Itoa(i), mutation.OpAttachEvidence, childID, string(*report.Outcome.ResultingRevision), payload, nil); stop != nil {
			return stop
		}
	}
	refine, err := splitParentRefine(parent, picked, kept, childID)
	if err != nil {
		return errorResult(cmd, err)
	}
	guard, err := splitChildGuard(childID, moved, acceptance)
	if err != nil {
		return errorResult(cmd, err)
	}
	if _, stop = run("refine", mutation.OpRefine, parent.TicketID.Raw, string(parent.Revision), refine, guard); stop != nil {
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
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(o)}, Warnings: warnings}
}

// recordedSplit confirms that a recorded Q/refine is this split's last step,
// not another refinement under the same request ID: Q/create named a child
// of the parent that carries this split's note, and the refine moved the
// parent from --expected-revision. It returns the child; anything else is
// REQUEST_ID_CONFLICT.
func recordedSplit(rc *readCtx, idx *journal.RequestIndex, refine mutation.IndexEntry, parent *ticket.Record, requestID, expected string, picked []int) (string, error) {
	conflict := func(why string) (string, error) {
		return "", wire.Errorf(wire.CodeRequestIDConflict, "--request-id", "request %s/refine is already recorded, but not for this split (%s); use a new request ID", prose(requestID), why)
	}
	if idx.TicketID != parent.TicketID.Raw {
		return conflict("it changed " + idx.TicketID)
	}
	n, err := strconv.Atoi(expected)
	if err != nil || refine.Outcome.ResultingRevision == nil || string(*refine.Outcome.ResultingRevision) != strconv.Itoa(n+1) {
		return conflict("its parent revision differs from --expected-revision " + prose(expected))
	}
	if _, found, err := idx.Lookup(requestID + "/create"); err != nil {
		return "", err
	} else if !found {
		return conflict("no " + requestID + "/create is recorded")
	}
	childID := idx.TicketID
	child, ok := rc.store.Inventory.Get(childID)
	if !ok || child.Supersedes == nil || child.Supersedes.Raw != parent.TicketID.Raw || child.Body == nil || !strings.HasPrefix(*child.Body, splitChildNote(parent.TicketID.Raw, expected, picked)) {
		return conflict("child " + childID + " is not a split of " + parent.TicketID.Raw + " at revision " + prose(expected) + " for criteria " + criteriaList(picked))
	}
	return childID, nil
}

// splitChildGuard is the parent REFINE's precondition, checked under the
// writer lock that commits it: the child still carries the moved criteria at
// the acceptance revision the split gave it. A writer that refined the child
// after an interruption refuses the REFINE, so the parent keeps the criteria.
func splitChildGuard(childID string, moved []string, acceptance *wire.Count) (*store.TicketGuard, error) {
	id, err := wire.ParseTicketID("child", childID)
	if err != nil {
		return nil, err
	}
	if acceptance == nil {
		return nil, wire.Errorf(wire.CodeMalformed, "child", "child %s reported no acceptance revision", childID)
	}
	return &store.TicketGuard{TicketID: id, Check: func(rec *ticket.Record) error {
		if rec.Status == ticket.StatusArchived || rec.AcceptanceRevision != *acceptance || !slices.Equal(rec.AcceptanceCriteria, moved) {
			return wire.Errorf(wire.CodeStaleTicket, "child", "child %s is %s at acceptanceRevision %s, not carrying the moved criteria at acceptanceRevision %s; the parent keeps them, so reconcile the two by hand", childID, rec.Status, rec.AcceptanceRevision, *acceptance)
		}
		return nil
	}}, nil
}

// splitParentRefine is the parent's REFINE: the kept criteria and its body
// with one bounded split note. A body that cannot take the note refuses
// LIMIT_EXCEEDED.
func splitParentRefine(rec *ticket.Record, picked []int, kept []string, childID string) (wire.Value, error) {
	body := ""
	if rec.Body != nil {
		body = strings.TrimRight(*rec.Body, "\n") + "\n\n"
	}
	body += "Split: criteria " + criteriaList(picked) + " of revision " + string(rec.Revision) + " moved to " + childID + "."
	if len(body) > wire.MaxBodyBytes {
		return wire.Value{}, wire.Errorf(wire.CodeLimitExceeded, "body", "ticket %s's body plus the split note is %d bytes, over the %d-byte limit; shorten the body first", rec.TicketID.Raw, len(body), wire.MaxBodyBytes)
	}
	return mutation.CanonicalPayload(mutation.OpRefine, wire.ObjectValue(wire.NewObject().Set("acceptanceCriteria", wire.Strings(kept)).Set("body", wire.String(body))))
}

// splitLongestChildID is a child ID in the parent's queue with the longest
// local token, so a parent payload that fits with it fits with the real one.
func splitLongestChildID(parent wire.TicketID) string {
	return strings.TrimSuffix(parent.Raw, parent.Local) + strings.Repeat("x", wire.MaxLocalTokenBytes)
}

func splitChildNote(parent, revision string, picked []int) string {
	return "Split from " + parent + " at revision " + revision + " (criteria " + criteriaList(picked) + ")."
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
	body := splitChildNote(rec.TicketID.Raw, string(rec.Revision), picked)
	if rec.Body != nil {
		body = truncateUTF8(body+"\n\n"+*rec.Body, wire.MaxBodyBytes)
	}
	parent := rec.TicketID
	effects := rec.Effects
	effects.TouchPaths = append([]string{}, rec.Effects.TouchPaths...)
	effects.Resources = append([]ticket.Resource{}, rec.Effects.Resources...)
	effects.VerdictPaths = slices.Clone(rec.Effects.VerdictPaths)
	var roles map[string][]string
	if rec.RequiredRoles != nil {
		roles = make(map[string][]string, len(rec.RequiredRoles))
		for stage, rs := range rec.RequiredRoles {
			roles[stage] = slices.Clone(rs)
		}
	}
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
		RequiredRoles:      roles,
		RequiresPool:       rec.RequiresPool,
		DueDate:            rec.DueDate,
		EstimateMinutes:    rec.EstimateMinutes,
		Supersedes:         &parent,
	}
}

// splitEvidence copies every parent entry. All copies land at the child's
// one acceptance revision, where a repeated digest would refuse DUPLICATE_ID,
// so each digest is copied once; an entry whose digests were all copied
// already keeps its provenance in the reason of the copy that carries its
// first digest.
func splitEvidence(rec *ticket.Record) []wire.Value {
	type copied struct {
		digests  []string
		segments []string
	}
	var out []*copied
	carrier := map[wire.Digest]*copied{}
	for _, e := range rec.AttachedEvidence {
		segment := "[acceptanceRevision " + string(e.AcceptanceRevision) + ", " + e.Actor + "] " + e.Reason
		c := &copied{}
		for _, d := range e.Evidence {
			if carrier[d] == nil {
				carrier[d] = c
				c.digests = append(c.digests, string(d))
			}
		}
		if len(c.digests) == 0 && len(e.Evidence) > 0 {
			first := carrier[e.Evidence[0]]
			first.segments = append(first.segments, segment)
			continue
		}
		if len(c.digests) == 0 {
			continue
		}
		c.segments = []string{segment}
		out = append(out, c)
	}
	values := make([]wire.Value, 0, len(out))
	for _, c := range out {
		values = append(values, wire.ObjectValue(wire.NewObject().Set("evidence", wire.Strings(c.digests)).Set("reason", wire.String(splitReason(rec.TicketID.Local, c.segments)))))
	}
	return values
}

// splitReason joins provenance segments within the reason bound, giving
// each segment an equal share so a later entry is never cut away whole.
func splitReason(parent string, segments []string) string {
	head := "split from " + parent + ": "
	share := (wire.AttachedEvidenceMaxReasonBytes - len(head) - 2*(len(segments)-1)) / len(segments)
	parts := make([]string, len(segments))
	for i, s := range segments {
		parts[i] = truncateUTF8(s, max(share, 0))
	}
	return truncateUTF8(head+strings.Join(parts, "; "), wire.AttachedEvidenceMaxReasonBytes)
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
