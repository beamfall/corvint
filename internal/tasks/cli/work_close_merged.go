package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/evidencestore"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"github.com/Beamfall/corvint/internal/tasks/worktrail"
)

// workGitTimeout bounds each git read close-merged runs.
const workGitTimeout = 120 * time.Second

// trailerCommit is one walked commit that carries a trailer block.
type trailerCommit struct {
	commit, issuedAt string
	t                worktrail.Transition
	err              error
}

// workGit runs one bounded, non-interactive git read in dir.
func workGit(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), workGitTimeout)
	defer cancel()
	c := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	c.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	var out, errb bytes.Buffer
	c.Stdout, c.Stderr = &out, &errb
	if err := c.Run(); err != nil {
		return "", wire.Errorf(wire.CodeMalformed, "git", "git %s: %v: %s", strings.Join(args, " "), err, prose(strings.TrimSpace(errb.String())))
	}
	return out.String(), nil
}

// isAncestor reports whether commit a is reachable from b.
func isAncestor(dir, a, b string) bool {
	_, err := workGit(dir, "merge-base", "--is-ancestor", a, b)
	return err == nil
}

// walkTrailers lists the trailer-bearing commits reachable from ref and not
// from any exclude, oldest first (TAW-V0-004).
func walkTrailers(dir, ref string, exclude ...string) ([]trailerCommit, error) {
	args := []string{"log", "--reverse", "-i", "-E", "--grep=^" + worktrail.KeyState + ":", "--format=%H%x1f%ct%x1f%(trailers:only,unfold)%x1e", ref}
	for _, x := range exclude {
		args = append(args, "^"+x)
	}
	args = append(args, "--")
	out, err := workGit(dir, args...)
	if err != nil {
		return nil, err
	}
	var commits []trailerCommit
	for _, rec := range strings.Split(out, "\x1e") {
		rec = strings.TrimLeft(rec, "\n")
		if rec == "" {
			continue
		}
		parts := strings.SplitN(rec, "\x1f", 3)
		if len(parts) != 3 {
			continue
		}
		tc := trailerCommit{commit: parts[0]}
		if secs, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
			tc.issuedAt = time.Unix(secs, 0).UTC().Format("2006-01-02T15:04:05Z")
		}
		t, ok, err := worktrail.Parse(strings.Split(strings.TrimSpace(parts[2]), "\n"))
		if !ok && err == nil {
			continue
		}
		tc.t, tc.err = t, err
		commits = append(commits, tc)
	}
	return commits, nil
}

// latestByTicket keeps each ticket's latest valid transition, in first-seen
// order, and returns the malformed commits apart.
func latestByTicket(commits []trailerCommit) ([]string, map[string]trailerCommit, []trailerCommit) {
	var order []string
	latest := map[string]trailerCommit{}
	var bad []trailerCommit
	for _, c := range commits {
		if c.err != nil {
			bad = append(bad, c)
			continue
		}
		for _, id := range c.t.Tickets {
			if _, seen := latest[id]; !seen {
				order = append(order, id)
			}
			latest[id] = c
		}
	}
	return order, latest, bad
}

// tawRequestID is the deterministic request id of one close-merged write:
// a rerun at the same revision replays it (TAW-V0-005).
func tawRequestID(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "taw-" + hex.EncodeToString(sum[:16])
}

type closeMergedRun struct {
	env      Env
	cmd      []string
	role     string
	stateDir string
	codes    []string
	ref      string
	refHead  string
	dryRun   bool
	failed   bool
	rows     []wire.Value
	warnings []string
}

func (r *closeMergedRun) row(id, state, commit, disposition, reason string, written bool, codes ...string) {
	if disposition == "FAILED" {
		r.failed = true
		for _, c := range codes {
			if !contains(r.codes, c) {
				r.codes = append(r.codes, c)
			}
		}
	}
	o := wire.NewObject().Set("ticket", wire.String(id)).Set("state", wire.String(state)).
		Set("commit", wire.String(commit)).Set("disposition", wire.String(disposition)).Set("written", wire.Bool(written))
	if reason != "" {
		o.Set("reason", wire.String(reason))
	}
	if len(codes) > 0 {
		o.Set("codes", wire.Strings(codes))
	}
	r.rows = append(r.rows, wire.ObjectValue(o))
}

// would prefixes a write disposition under --dry-run (TAW-V0-008).
func (r *closeMergedRun) would(d string) string {
	if r.dryRun {
		return "WOULD_" + d
	}
	return d
}

// submit sends one mutation through the §5.2 writer at the ticket's
// current revision; ok reports a committed or replayed write.
func (r *closeMergedRun) submit(op, id string, payload wire.Value, issuedAt string, key ...string) (bool, []string, string) {
	rev, err := readTargetRevision(r.env, id)
	if err != nil {
		return false, []string{wire.CodeOf(err)}, err.Error()
	}
	actor, err := initActor(r.role)
	if err != nil {
		return false, []string{wire.CodeOf(err)}, err.Error()
	}
	payload, err = mutation.CanonicalPayload(op, payload)
	if err != nil {
		return false, []string{wire.CodeOf(err)}, err.Error()
	}
	flags := mutateFlags{role: r.role, requestID: tawRequestID(append([]string{op, id, rev}, key...)...), target: id, expected: rev, issuedAt: issuedAt}
	out := submitMutation(r.env, []string{"ticket", strings.ToLower(op)}, op, actor, flags, payload)
	if out.Outcome != wire.OutcomeOK {
		why := ""
		if len(out.Warnings) > 0 {
			why = out.Warnings[0]
		}
		return false, out.Codes, why
	}
	return true, nil, ""
}

func progressValue(m ticket.ProgressMark) wire.Value {
	o := wire.NewObject()
	ticket.ProgressMarkValue(o, m)
	return wire.ObjectValue(o)
}

func strp(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func latestMark(rec *ticket.Record) *ticket.ProgressMark {
	if len(rec.Progress) == 0 {
		return nil
	}
	return &rec.Progress[len(rec.Progress)-1].ProgressMark
}

// mark records one progress mark unless it is already the latest
// (TAW-V0-006, TAW-V0-007).
func (r *closeMergedRun) mark(rec *ticket.Record, c trailerCommit, m ticket.ProgressMark, disposition, reason string) {
	id := rec.TicketID.Raw
	if rec.Status == ticket.StatusCompleted || rec.Status == ticket.StatusArchived {
		r.row(id, c.t.State, c.commit, "NOT_OPEN", "ticket is "+rec.Status, false)
		return
	}
	if l := latestMark(rec); l != nil && l.Equal(m) {
		r.row(id, c.t.State, c.commit, disposition, reason, false)
		return
	}
	if r.dryRun {
		r.row(id, c.t.State, c.commit, r.would(disposition), reason, false)
		return
	}
	if ok, codes, why := r.submit(mutation.OpProgressSet, id, progressValue(m), c.issuedAt, c.t.State, c.commit, m.Phase); !ok {
		r.row(id, c.t.State, c.commit, "FAILED", why, false, codes...)
		return
	}
	r.row(id, c.t.State, c.commit, disposition, reason, true)
}

// deliver is TAW-V0-005: progress MERGED_AWAITING_CLOSEOUT, one integration
// evidence document, then COMPLETE_MANUAL citing its digest.
func (r *closeMergedRun) deliver(rec *ticket.Record, c trailerCommit) {
	id := rec.TicketID.Raw
	switch rec.Status {
	case ticket.StatusCompleted:
		r.row(id, c.t.State, c.commit, "ALREADY_COMPLETED", "", false)
		return
	case ticket.StatusArchived:
		r.row(id, c.t.State, c.commit, "NOT_OPEN", "ticket is ARCHIVED", false)
		return
	}
	doc := wire.Encode(wire.ObjectValue(wire.NewObject().
		Set("commit", wire.String(c.commit)).
		Set("integrationRef", wire.String(r.ref)).
		Set("refCommit", wire.String(r.refHead)).
		Set("ticket", wire.String(id))))
	digest := string(wire.Sum(doc))
	reason := "[TAW-REASON:INTEGRATED] delivered commit " + c.commit + " is on " + r.ref + " (Corvint-Ticket trailer; evidence " + digest + ")"
	if r.dryRun {
		r.row(id, c.t.State, c.commit, r.would("COMPLETED"), reason, false)
		return
	}
	m := ticket.ProgressMark{Phase: ticket.PhaseMergedAwaitingCloseout, Lane: strp(c.t.Lane), Head: strp(c.commit)}
	if l := latestMark(rec); l == nil || !l.Equal(m) {
		if ok, codes, why := r.submit(mutation.OpProgressSet, id, progressValue(m), c.issuedAt, c.t.State, c.commit, m.Phase); !ok {
			r.row(id, c.t.State, c.commit, "FAILED", why, false, codes...)
			return
		}
	}
	if _, err := evidencestore.Put(r.stateDir, doc); err != nil {
		r.row(id, c.t.State, c.commit, "FAILED", err.Error(), false, wire.CodeOf(err))
		return
	}
	payload := wire.ObjectValue(wire.NewObject().Set("evidence", wire.Array(wire.String(digest))).Set("reason", wire.String(reason)))
	if ok, codes, why := r.submit(mutation.OpCompleteManual, id, payload, c.issuedAt, c.t.State, c.commit, r.ref); !ok {
		r.row(id, c.t.State, c.commit, "FAILED", why, false, codes...)
		return
	}
	r.row(id, c.t.State, c.commit, "COMPLETED", reason, true)
}

// cursorPath is the private per-REF cursor file (TAW-V0-008). It sits
// beside the state directory, not in it: the store audit fixes that
// directory's layout and refuses an unexpected entry.
func cursorPath(stateDir, ref string) string {
	sum := sha256.Sum256([]byte(ref))
	return filepath.Join(filepath.Dir(stateDir), "taskman-work", "close-merged-"+hex.EncodeToString(sum[:8])+".json")
}

func readCursor(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	v, err := wire.Parse(raw)
	if err != nil || v.Kind != wire.KindObject {
		return ""
	}
	c, ok := v.Obj.Get("commit")
	if !ok || c.Kind != wire.KindString {
		return ""
	}
	return c.Str
}

func writeCursor(path, ref, commit string) error {
	raw := wire.EncodeFile(wire.ObjectValue(wire.NewObject().Set("commit", wire.String(commit)).Set("integrationRef", wire.String(ref))))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// branchName is a lane ref as progress records it.
func branchName(ref string) string {
	return strings.TrimPrefix(ref, "refs/heads/")
}

// workCloseMerged is TAW-V0-004..009.
func workCloseMerged(env Env, cmd []string, args []string) *wire.Result {
	fl, set, res := listFlags(cmd, args, []string{"integration", "since", "role"}, []string{"lane-ref"}, []string{"dry-run"})
	if res != nil {
		return res
	}
	first := func(k string) string {
		if v := fl[k]; len(v) > 0 {
			return v[0]
		}
		return ""
	}
	ref := first("integration")
	if ref == "" {
		return usage(cmd, "--integration REF is required: "+commandUsage["work close-merged"])
	}
	role := first("role")
	if role == "" {
		role = "OWNER"
	}
	run := &closeMergedRun{env: env, cmd: cmd, role: role, ref: ref, dryRun: set["dry-run"]}
	var stateDir, queueID string
	var inv func(id string) (*ticket.Record, bool)
	var sorted []string
	load := func() *wire.Result {
		rc, err := withInventoryStore(env, func(rc *readCtx) error {
			stateDir, queueID = rc.repo.StateDir, rc.store.Queue.QueueID.Raw
			sorted = rc.store.Inventory.Sorted()
			inv = rc.store.Inventory.Get
			return nil
		})
		if err != nil {
			return failure(cmd, rc, err)
		}
		return nil
	}
	if res := load(); res != nil {
		return res
	}
	run.stateDir = stateDir
	head, err := workGit(env.Cwd, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err != nil {
		return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "--integration", "%s does not name a commit", prose(ref)))
	}
	run.refHead = strings.TrimSpace(head)
	since := first("since")
	cpath := cursorPath(stateDir, ref)
	if since == "" {
		if c := readCursor(cpath); c != "" && isAncestor(env.Cwd, c, run.refHead) {
			since = c
		}
	}
	var exclude []string
	if since != "" {
		exclude = append(exclude, since)
	}
	commits, err := walkTrailers(env.Cwd, ref, exclude...)
	if err != nil {
		return errorResult(cmd, err)
	}
	order, latest, bad := latestByTicket(commits)
	for _, c := range bad {
		run.row("", "", c.commit, "MALFORMED", c.err.Error(), false, wire.CodeMalformed)
	}
	onRef := map[string]bool{}
	for _, local := range order {
		c := latest[local]
		id := qualifyTicket(queueID, local)
		onRef[id] = true
		rec, ok := inv(id)
		if !ok {
			run.row(id, c.t.State, c.commit, "UNKNOWN_TICKET", "no such ticket in the store", false)
			continue
		}
		switch c.t.State {
		case worktrail.StateDelivered:
			run.deliver(rec, c)
		case worktrail.StateBlocked:
			run.mark(rec, c, ticket.ProgressMark{Phase: ticket.PhaseBlocked, Lane: strp(c.t.Lane), Head: strp(c.commit), Reason: strp(c.t.Reason)}, "BLOCKED", c.t.Reason)
		case worktrail.StateStarted:
			run.mark(rec, c, ticket.ProgressMark{Phase: ticket.PhaseInProgress, Lane: strp(c.t.Lane), Head: strp(c.commit), Reason: strp(c.t.Reason)}, "IN_PROGRESS", c.t.Reason)
		}
	}
	// Lane refs: a delivered trailer not yet on REF reads AWAITING_INTEGRATION
	// (TAW-V0-007). A rebased commit is ingested by its new trailer.
	if len(fl["lane-ref"]) > 0 && !run.dryRun {
		if res := load(); res != nil {
			return res
		}
	}
	laneSeen := map[string]bool{}
	for _, lref := range fl["lane-ref"] {
		lhead, err := workGit(env.Cwd, "rev-parse", "--verify", "--quiet", lref+"^{commit}")
		if err != nil {
			return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "--lane-ref", "%s does not name a commit", prose(lref)))
		}
		lhead = strings.TrimSpace(lhead)
		lcommits, err := walkTrailers(env.Cwd, lref, ref)
		if err != nil {
			return errorResult(cmd, err)
		}
		lorder, llatest, lbad := latestByTicket(lcommits)
		for _, c := range lbad {
			run.row("", "", c.commit, "MALFORMED", c.err.Error(), false, wire.CodeMalformed)
		}
		for _, local := range lorder {
			c := llatest[local]
			id := qualifyTicket(queueID, local)
			if onRef[id] || c.t.State != worktrail.StateDelivered {
				continue
			}
			laneSeen[id+"\x00"+branchName(lref)] = true
			rec, ok := inv(id)
			if !ok {
				run.row(id, c.t.State, c.commit, "UNKNOWN_TICKET", "no such ticket in the store", false)
				continue
			}
			m := ticket.ProgressMark{Phase: ticket.PhaseDeliveredOnBranch, Lane: strp(c.t.Lane), Branch: strp(branchName(lref)), Head: strp(c.commit)}
			run.mark(rec, c, m, "DELIVERED_ON_BRANCH", "NOT_ON_INTEGRATION_REF")
		}
		// A recorded branch delivery whose head left the branch, with no
		// trailer now, was dropped or rebased away: report it, write nothing.
		for _, id := range sorted {
			rec, _ := inv(id)
			l := latestMark(rec)
			if onRef[id] || rec.Status != ticket.StatusOpen || l == nil || l.Phase != ticket.PhaseDeliveredOnBranch || l.Branch == nil || l.Head == nil {
				continue
			}
			if *l.Branch != branchName(lref) || laneSeen[id+"\x00"+*l.Branch] {
				continue
			}
			if !isAncestor(env.Cwd, *l.Head, lhead) {
				run.row(id, worktrail.StateDelivered, *l.Head, "HEAD_NOT_ON_REF", "recorded head is no longer on "+*l.Branch+" and no delivered trailer replaces it", false)
			}
		}
	}
	if !run.dryRun && !run.failed {
		if err := writeCursor(cpath, ref, run.refHead); err != nil {
			run.warnings = append(run.warnings, "the cursor was not written ("+prose(err.Error())+"); the next run re-walks and replays (TAW-V0-008)")
		}
	}
	if run.failed {
		run.warnings = append(run.warnings, "at least one write FAILED; the cursor was not advanced, so a rerun retries it (TAW-V0-008)")
	}
	summary := wire.NewObject().Set("integrationRef", wire.String(ref)).Set("refCommit", wire.String(run.refHead)).
		Set("since", wire.String(since)).Set("walked", countValue(int64(len(commits)))).Set("dryRun", wire.Bool(run.dryRun))
	items := append([]wire.Value{wire.ObjectValue(summary)}, run.rows...)
	out := &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: items, Warnings: run.warnings}
	if run.failed {
		out.Outcome = wire.OutcomeRefused
		out.Codes = run.codes
		if len(out.Codes) == 0 {
			out.Codes = []string{wire.CodeTicketState}
		}
	}
	return out
}
