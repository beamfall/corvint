package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Batch closeout (TCD-V0-169, V1-2396): `ticket batch-close` closes every
// OPEN ticket a merged batch PR delivered with one COMPLETE_MANUAL each,
// through the same §5.2 writer as `ticket complete-manual`. It reads the
// same exports as `ticket batch-delivered` and never queries GitHub.
const (
	profileBatchClose = "taskman-batch-close/0"

	// Per-ticket dispositions of the batch-close plan.
	closeWillClose        = "CLOSE"
	closeClosed           = "CLOSED"
	closeReplayed         = "REPLAYED"
	closeAlreadyCompleted = "ALREADY_COMPLETED"
	closeHeld             = "HELD"
	closeAttemptLive      = "ATTEMPT_LIVE"
	closeNotOpen          = "NOT_OPEN"
	closeMentionedOnly    = "MENTIONED_ONLY"
	closeFailed           = "FAILED"
)

// ghRepoName is an OWNER/NAME GitHub repository for --repo.
var ghRepoName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}/[A-Za-z0-9_.-]{1,100}$`)

// closeRow is one ticket of the batch-close plan.
type closeRow struct {
	hit                  batchHit
	g                    *gatedRow
	local, status        string
	revision             string
	disposition, code    string
	recovery, detail     string
	requestID, receipt   string
	resultingRevision    string
	attempt              *transaction.AttemptSummary
	holds                []ticket.Hold
	mentionedOnly, write bool
}

// batchCloseCommand is `corvint-tasks ticket batch-close`. A refusal before
// any write (no green run, no run URL, unmerged batch) writes nothing. Each
// close is independent: one failed ticket never stops the rest, and the
// result is OK only when every delivered ticket ends COMPLETED.
func batchCloseCommand(env Env, args []string) *wire.Result {
	cmd := []string{"ticket", "batch-close"}
	dryRun, manual := false, false
	rest := make([]string, 0, len(args))
	for _, a := range args {
		switch a {
		case "--dry-run":
			dryRun = true
			continue
		case "--manual":
			manual = true
			continue
		}
		rest = append(rest, a)
	}
	fl, pos, err := flags(rest, "pr", "prs", "runs", "repo", "role", "issued-at", "seal", "ci-run")
	for _, k := range []string{"seal", "ci-run"} {
		if _, ok := fl[k]; err == nil && ok && manual {
			err = wire.Errorf(wire.CodeMalformed, "--"+k, "--%s applies to the gated close; --manual records COMPLETE_MANUAL and runs no gate", k)
		}
	}
	var in *batchInput
	if err == nil {
		in, err = parseBatchInput(env, "batch-close", fl, pos)
	}
	if _, ok := fl["runs"]; err == nil && !ok {
		err = wire.Errorf(wire.CodeMalformed, "--runs", "ticket batch-close needs a --runs export: a close cites a green CI run")
	}
	repo, repoGiven := fl["repo"]
	if err == nil && repoGiven && !ghRepoName.MatchString(repo) {
		err = wire.Errorf(wire.CodeMalformed, "--repo", "--repo must be OWNER/NAME")
	}
	var runURL string
	if err == nil {
		runURL, err = in.greenRunURL(repo)
	}
	role := "OWNER"
	if v, ok := fl["role"]; ok {
		role = v
	}
	var actor mutation.Binding
	if err == nil {
		actor, err = initActor(role)
	}
	now := wire.Timestamp(time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	issued := now
	if err == nil {
		issued, err = batchCloseIssuedAt(fl, in.batch, now)
	}
	var gb *gatedBatch
	if err == nil && !manual {
		gb, err = newGatedBatch(env, in, fl, repo, runURL, role, string(issued))
	}
	if err != nil {
		return failure(cmd, nil, err)
	}
	merge := in.batch.mergeOid()
	if gb != nil {
		merge = gb.merge
	}
	evidence := []wire.Digest{wire.Sum([]byte(merge)), wire.Sum([]byte(runURL))}
	slices.Sort(evidence)
	var rows []closeRow
	rc, err := withStore(env, func(rc *readCtx) error {
		rows = nil
		if rc.journalAbsent {
			return wire.Errorf(wire.CodePrerequisiteUnsatisfied, "journal", "ticket batch-close needs an initialized journal")
		}
		pin, _, e := planInput(rc)
		if e != nil {
			return e
		}
		pin.Now = now
		delivered, mentioned := in.batchHits(rc)
		for _, h := range delivered {
			rec, _ := rc.store.Inventory.Get(h.ticketID)
			r := planClose(h, rec, transaction.CurrentAttempt(pin, h.ticketID), in.number)
			if gb != nil {
				gb.used[rec.TicketID.Local], gb.used[rec.TicketID.Raw] = true, true
				if r.write {
					gb.plan(&r, rec, rc.store.Policy, pin, in.number)
				}
			}
			rows = append(rows, r)
		}
		if gb != nil {
			if e := gb.unknownSeal(in); e != nil {
				return e
			}
			for k := range gb.used {
				delete(gb.used, k)
			}
		}
		for _, h := range mentioned {
			rec, _ := rc.store.Inventory.Get(h.ticketID)
			r := closeRow{hit: h, local: rec.TicketID.Local, status: rec.Status, revision: string(rec.Revision), mentionedOnly: true, disposition: closeMentionedOnly}
			r.recovery = "the batch only mentions this ticket; if it was delivered, add a `Refs " + rec.TicketID.Local + "` line to the lane PR body and rerun, or close it with `corvint-tasks ticket complete-manual`"
			rows = append(rows, r)
		}
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	failed := false
	var codes []string
	for i := range rows {
		r := &rows[i]
		if r.mentionedOnly {
			continue
		}
		if !r.write {
			failed = failed || r.disposition != closeAlreadyCompleted
		}
		if r.write && !dryRun {
			if gb != nil {
				r.runGated(env, cmd, actor, gb, in.number)
			} else {
				r.submit(env, cmd, actor, string(issued), in, merge, runURL, evidence)
			}
		}
		if r.disposition == closeFailed {
			failed = true
		}
		if r.code != "" && !slices.Contains(codes, r.code) {
			codes = append(codes, r.code)
		}
	}
	slices.Sort(codes)
	res := success(cmd, rc)
	res.Items = []wire.Value{batchCloseValue(in, rows, merge, runURL, evidence, string(issued), dryRun, gb)}
	res.Untrusted = true
	res.Warnings = append(res.Warnings, in.laneWarnings()...)
	if w := in.excludedWarning(); w != "" {
		res.Warnings = append(res.Warnings, w)
	}
	if dryRun {
		res.Warnings = append(res.Warnings, "dry run: nothing was written")
		return res
	}
	if failed {
		res.Outcome = wire.OutcomeRefused
		res.Codes = codes
		res.Warnings = append(res.Warnings, "not every delivered ticket is COMPLETED; each row names its code and recovery")
	}
	return res
}

// planClose classifies one delivered ticket before any write: COMPLETED is
// skipped, a held, non-OPEN or live-attempt ticket is never closed and is
// reported with its recovery, and only an OPEN, unheld ticket with no live
// attempt is closed at its current revision.
func planClose(h batchHit, rec *ticket.Record, cur *transaction.AttemptSummary, batch int64) closeRow {
	r := closeRow{hit: h, local: rec.TicketID.Local, status: rec.Status, revision: string(rec.Revision), holds: slices.Clone(rec.Holds), attempt: cur}
	r.requestID = batchCloseRequestID(batch, rec.TicketID.Raw, rec.TicketID.Local)
	switch {
	case rec.Status == ticket.StatusCompleted:
		r.disposition = closeAlreadyCompleted
	case rec.Status == ticket.StatusHeld || len(rec.Holds) > 0:
		r.disposition, r.code = closeHeld, wire.CodeTicketState
		ids := make([]string, 0, len(rec.Holds))
		for _, x := range rec.Holds {
			ids = append(ids, x.HoldID)
		}
		r.recovery = "release the hold (" + strings.Join(ids, ", ") + ") with `corvint-tasks ticket release-hold` if the delivery stands, then rerun"
	case rec.Status != ticket.StatusOpen:
		r.disposition, r.code = closeNotOpen, wire.CodeTicketState
		r.recovery = "COMPLETE_MANUAL needs an OPEN ticket; inspect it with `corvint-tasks ticket show " + rec.TicketID.Local + "`"
	case cur != nil:
		r.disposition, r.code = closeAttemptLive, wire.CodeAttemptLive
		r.recovery = cur.Recovery + "; batch-close never reaps an attempt"
	default:
		r.disposition, r.write = closeWillClose, true
	}
	return r
}

// submit commits one COMPLETE_MANUAL through the shared writer path and
// records its outcome on the row; a refusal stays on this row only.
func (r *closeRow) submit(env Env, cmd []string, actor mutation.Binding, issued string, in *batchInput, merge, runURL string, evidence []wire.Digest) {
	reason := "Delivered in batch PR #" + strconv.FormatInt(in.number, 10) + "; merged at " + merge + "; CI green: " + runURL
	if r.hit.commit != "" {
		reason = "Delivered by lane commit " + r.hit.commit + "; merged in batch PR #" + strconv.FormatInt(in.number, 10) + " at " + merge + "; CI green: " + runURL
	}
	if r.hit.lane != 0 {
		reason = "Delivered by lane PR #" + strconv.FormatInt(r.hit.lane, 10) + "; merged in batch PR #" + strconv.FormatInt(in.number, 10) + " at " + merge + "; CI green: " + runURL
	}
	ev := make([]wire.Value, 0, len(evidence))
	for _, d := range evidence {
		ev = append(ev, wire.String(string(d)))
	}
	payload := wire.ObjectValue(wire.NewObject().Set("reason", wire.String(reason)).Set("evidence", wire.Array(ev...)))
	payload, err := mutation.CanonicalPayload(mutation.OpCompleteManual, payload)
	if err != nil {
		r.disposition, r.code, r.detail = closeFailed, wire.CodeOf(err), prose(err.Error())
		return
	}
	f := mutateFlags{role: actor.Role, requestID: r.requestID, target: r.hit.ticketID, expected: r.revision, issuedAt: issued}
	res := submitMutation(env, cmd, mutation.OpCompleteManual, actor, f, payload)
	if res.Outcome != wire.OutcomeOK {
		r.disposition, r.detail = closeFailed, strings.Join(res.Warnings, "; ")
		if len(res.Codes) > 0 {
			r.code = res.Codes[0]
		}
		if r.code == "" {
			r.code = wire.CodeMalformed
		}
		r.recovery = "rerun the same command: closed tickets are skipped and this one retries with the same request id"
		return
	}
	r.disposition = closeClosed
	if len(res.Items) == 1 {
		item := res.Items[0]
		if v, ok := item.Obj.Get("replayed"); ok && v.Bool {
			r.disposition = closeReplayed
		}
		if v, ok := item.Obj.Get("receipt"); ok {
			r.receipt = v.Str
		}
		if v, ok := item.Obj.Get("resultingRevision"); ok {
			r.resultingRevision = v.Str
		}
	}
}

// greenRunURL picks the cited green run: a successful run at the merge
// commit before one at the batch head, the highest databaseId first. Its URL
// is the export's `url`, else https://github.com/<--repo>/actions/runs/<id>.
func (in *batchInput) greenRunURL(repo string) (string, error) {
	var best *ghRun
	for i := range in.runs {
		r := &in.runs[i]
		if r.Conclusion != "success" || (r.HeadSha != in.batch.mergeOid() && r.HeadSha != in.batch.HeadRefOid) {
			continue
		}
		atMerge := r.HeadSha == in.batch.mergeOid()
		if best == nil || (atMerge && best.HeadSha != in.batch.mergeOid()) || (atMerge == (best.HeadSha == in.batch.mergeOid()) && r.DatabaseID > best.DatabaseID) {
			best = r
		}
	}
	if best == nil {
		return "", wire.Errorf(wire.CodePrerequisiteUnsatisfied, "--runs", "no successful CI run at batch PR #%d merge commit or head; nothing was closed", in.number)
	}
	if u := best.URL; strings.HasPrefix(u, "https://") && len(u) <= 512 && !strings.ContainsAny(u, " \t\r\n") {
		return u, nil
	}
	if repo != "" {
		return "https://github.com/" + repo + "/actions/runs/" + strconv.FormatInt(best.DatabaseID, 10), nil
	}
	return "", wire.Errorf(wire.CodePrerequisiteUnsatisfied, "--runs", "green run %d has no https url in the export; export `url` or pass --repo OWNER/NAME; nothing was closed", best.DatabaseID)
}

// batchCloseIssuedAt is --issued-at, else the batch PR's mergedAt, else now:
// a rerun over the same export reproduces the same envelope and replays.
func batchCloseIssuedAt(fl map[string]string, batch *ghPR, now wire.Timestamp) (wire.Timestamp, error) {
	if v, ok := fl["issued-at"]; ok {
		return wire.ParseTimestamp("issuedAt", v)
	}
	if ts, err := wire.ParseTimestamp("mergedAt", batch.MergedAt); err == nil {
		return ts, nil
	}
	return now, nil
}

// batchCloseRequestID derives the per-ticket request id from the batch PR
// and the ticket, so a rerun replays instead of writing twice.
func batchCloseRequestID(batch int64, ticketID, local string) string {
	id := "batch-close-" + strconv.FormatInt(batch, 10) + "-" + local
	if len(id) <= wire.MaxRequestIDBytes {
		return id
	}
	sum := sha256.Sum256([]byte(ticketID))
	return "batch-close-" + strconv.FormatInt(batch, 10) + "-" + hex.EncodeToString(sum[:12])
}

// batchCloseValue renders the taskman-batch-close/0 item for --manual, or
// with gb the additive taskman-batch-close/1 item of a gated close.
func batchCloseValue(in *batchInput, rows []closeRow, merge, runURL string, evidence []wire.Digest, issued string, dryRun bool, gb *gatedBatch) wire.Value {
	tickets := make([]wire.Value, 0, len(rows))
	counts := map[string]int{}
	human := []string{}
	for _, r := range rows {
		counts[r.disposition]++
		o := wire.NewObject()
		o.Set("ticketId", wire.String(r.hit.ticketID))
		o.Set("local", wire.String(r.local))
		o.Set("status", wire.String(r.status))
		o.Set("lanePr", nullableLane(r.hit.lane))
		o.Set("disposition", wire.String(r.disposition))
		o.Set("code", orNull(r.code))
		o.Set("recovery", orNull(r.recovery))
		o.Set("detail", orNull(r.detail))
		o.Set("requestId", orNull(r.requestID))
		o.Set("expectedRevision", orNull(r.revision))
		o.Set("receipt", orNull(r.receipt))
		o.Set("resultingRevision", orNull(r.resultingRevision))
		holds := make([]wire.Value, 0, len(r.holds))
		for _, h := range r.holds {
			holds = append(holds, wire.ObjectValue(wire.NewObject().Set("holdId", wire.String(h.HoldID)).Set("reason", wire.String(h.Reason))))
		}
		o.Set("holds", wire.Array(holds...))
		if r.attempt != nil {
			o.Set("attempt", r.attempt.Value())
		} else {
			o.Set("attempt", wire.Null())
		}
		if gb != nil {
			gatedRowValue(o, r)
		}
		tickets = append(tickets, wire.ObjectValue(o))
		line := r.local + " " + r.disposition + " lane=" + lanePRText(r.hit.lane)
		if r.code != "" {
			line += " code=" + r.code
		}
		human = append(human, line)
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	summary := make([]string, 0, len(keys))
	for _, k := range keys {
		summary = append(summary, k+"="+strconv.Itoa(counts[k]))
	}
	head := "batch-close #" + strconv.FormatInt(in.number, 10) + " at " + merge + ": " + strings.Join(summary, " ")
	if dryRun {
		head += " (dry run)"
	}
	ev := make([]wire.Value, 0, len(evidence))
	for _, d := range evidence {
		ev = append(ev, wire.String(string(d)))
	}
	o := wire.NewObject()
	if gb != nil {
		o.Set("profile", wire.String(profileBatchCloseGated))
	} else {
		o.Set("profile", wire.String(profileBatchClose))
	}
	o.Set("batchPr", wire.String(strconv.FormatInt(in.number, 10)))
	o.Set("mergeCommit", wire.String(merge))
	o.Set("runUrl", wire.String(runURL))
	o.Set("evidence", wire.Array(ev...))
	if gb != nil {
		o.Set("mode", wire.String("GATED"))
		o.Set("prUrl", wire.String(gb.prURL))
		o.Set("mergedAt", orNull(gb.mergedAt))
		seals := make([]string, 0, len(gb.seals))
		for k, v := range gb.seals {
			seals = append(seals, k+"="+v)
		}
		slices.Sort(seals)
		o.Set("seals", wire.Strings(seals))
	}
	o.Set("issuedAt", wire.String(issued))
	o.Set("dryRun", wire.Bool(dryRun))
	o.Set("excludedPrs", in.excludedValue())
	o.Set("tickets", wire.Array(tickets...))
	o.Set("human", wire.Strings(append([]string{head}, human...)))
	return wire.ObjectValue(o)
}

func nullableLane(n int64) wire.Value {
	if n == 0 {
		return wire.Null()
	}
	return count(int(n))
}
