package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Gated batch closeout (TCD-V0-185..189, V1-2984): without --manual,
// `ticket batch-close` closes each delivered ticket through the native
// lifecycle (claim with a diff-derived scope, submit, every required gate,
// complete), so the completion is VERIFIED against a gate manifest. The batch
// merge identity is attached to the ticket as evidence first. A ticket whose
// gates cannot run here is refused on its own row and is never closed
// manually; `--manual` keeps the taskman-batch-close/0 COMPLETE_MANUAL path.
const (
	profileBatchCloseGated = "taskman-batch-close/1"
	profileBatchMerge      = "taskman-batch-merge/0"
	closeUnrunnable        = "UNRUNNABLE"
	batchCloseHolder       = "batch-close"
	batchCloseLeaseMinutes = "180"
)

// ghRunRepo is the repository prefix of a GitHub Actions run URL.
var ghRunRepo = regexp.MustCompile(`^(https://github\.com/[A-Za-z0-9_.-]{1,100}/[A-Za-z0-9_.-]{1,100})/actions/runs/[0-9]+`)

// gatedBatch is the batch-wide input of a gated close.
type gatedBatch struct {
	root          string
	merge, parent string
	prURL         string
	runURL        string
	mergedAt      string
	ciRun         string
	role          string
	issued        string
	seals         map[string]string
	used          map[string]bool
}

// gateStep is one required gate of a gated row and how it ran.
type gateStep struct{ id, via, state string }

// gatedRow is the gated part of one batch-close row.
type gatedRow struct {
	candidate, base, seal string
	tree                  string
	gates                 []gateStep
	prior                 int
	attemptID, generation string
	step, completionKind  string
	doc                   []byte
	docSum                wire.Digest
	attached              bool
}

// parseSeals reads --seal LOCAL=COMMIT[,LOCAL=COMMIT...].
func parseSeals(v string) (map[string]string, error) {
	out := map[string]string{}
	for _, part := range strings.Split(v, ",") {
		local, rev, ok := strings.Cut(part, "=")
		if !ok || local == "" || rev == "" || strings.ContainsAny(part, " \t\r\n") {
			return nil, wire.Errorf(wire.CodeMalformed, "--seal", "--seal takes LOCAL=COMMIT[,LOCAL=COMMIT...]")
		}
		if _, dup := out[local]; dup {
			return nil, wire.Errorf(wire.CodeMalformed, "--seal", "--seal names %s twice", prose(local))
		}
		out[local] = rev
	}
	return out, nil
}

// batchPRURL is the batch PR's URL: the export's `url`, else derived from
// --repo, else from the cited run URL's repository (TCD-V0-188).
func batchPRURL(in *batchInput, repo, runURL string) (string, error) {
	n := strconv.FormatInt(in.number, 10)
	if u := in.batch.URL; strings.HasPrefix(u, "https://") && len(u) <= 512 && !strings.ContainsAny(u, " \t\r\n") {
		return u, nil
	}
	if repo != "" {
		return "https://github.com/" + repo + "/pull/" + n, nil
	}
	if m := ghRunRepo.FindStringSubmatch(runURL); m != nil {
		return m[1] + "/pull/" + n, nil
	}
	return "", wire.Errorf(wire.CodePrerequisiteUnsatisfied, "--prs", "batch PR #%d has no https url in the export; export `url` or pass --repo OWNER/NAME; nothing was closed", in.number)
}

// newGatedBatch resolves the merge commit and its first parent before any
// write: a merge this clone does not hold refuses the whole run.
func newGatedBatch(env Env, in *batchInput, fl map[string]string, repo, runURL, role, issued string) (*gatedBatch, error) {
	b := &gatedBatch{root: env.Cwd, runURL: runURL, mergedAt: in.batch.MergedAt, role: role, issued: issued, seals: map[string]string{}, used: map[string]bool{}}
	var err error
	if v, ok := fl["seal"]; ok {
		if b.seals, err = parseSeals(v); err != nil {
			return nil, err
		}
	}
	if v, ok := fl["ci-run"]; ok {
		if v == "" {
			return nil, wire.Errorf(wire.CodeMalformed, "--ci-run", "--ci-run needs a taskman-ci-run/0 file")
		}
		b.ciRun = v
	}
	if b.prURL, err = batchPRURL(in, repo, runURL); err != nil {
		return nil, err
	}
	if b.merge, err = store.ResolveCommit(b.root, in.batch.mergeOid()); err != nil {
		return nil, wire.Errorf(wire.CodePrerequisiteUnsatisfied, "mergeCommit", "batch PR #%d merge commit %s is not in this clone; run `git fetch origin` and rerun; nothing was closed", in.number, in.batch.mergeOid())
	}
	if b.parent, err = store.ResolveCommit(b.root, b.merge+"^1"); err != nil {
		return nil, wire.Errorf(wire.CodePrerequisiteUnsatisfied, "mergeCommit", "batch merge commit %s has no first parent to diff against; nothing was closed", b.merge)
	}
	return b, nil
}

// unknownSeal names a --seal ticket the batch did not deliver (every
// delivered ticket, closed or not, is marked used); it refuses the run before
// any write.
func (b *gatedBatch) unknownSeal(in *batchInput) error {
	keys := make([]string, 0, len(b.seals))
	for k := range b.seals {
		if !b.used[k] {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	slices.Sort(keys)
	return wire.Errorf(wire.CodeMalformed, "--seal", "--seal names %s, which batch PR #%d did not deliver; nothing was closed", prose(strings.Join(keys, ", ")), in.number)
}

// plan fills the gated part of one CLOSE row without writing (TCD-V0-186,
// TCD-V0-187): the candidate and base commits, each required gate's runner
// and the merge evidence document. A gate that cannot run here, or a seal
// outside the merge, turns the row UNRUNNABLE.
func (b *gatedBatch) plan(r *closeRow, rec *ticket.Record, policy *intent.Policy, pin transaction.PlanInput, batch int64) {
	g := &gatedRow{candidate: b.merge, base: b.parent}
	r.g = g
	for _, a := range pin.Attempts {
		if a.TicketID.Raw == rec.TicketID.Raw {
			g.prior++
		}
	}
	r.requestID = gatedRequestID(batch, rec.TicketID.Raw, rec.TicketID.Local, g.prior, "claim")
	key := rec.TicketID.Local
	rev, ok := b.seals[key]
	if !ok {
		key = rec.TicketID.Raw
		rev, ok = b.seals[key]
	}
	if ok {
		seal, err := store.ResolveCommit(b.root, rev)
		if err != nil {
			r.unrunnable(wire.CodePrerequisiteUnsatisfied, "seal "+rev+" does not resolve to a commit in this clone", "fetch the lane branch or correct --seal, then rerun")
			return
		}
		if !store.IsAncestor(b.root, seal, b.merge) {
			r.unrunnable(wire.CodePrerequisiteUnsatisfied, "seal "+seal+" is not contained in batch merge "+b.merge, "pass the sealed lane commit the batch merged, then rerun")
			return
		}
		base, err := store.MergeBase(b.root, seal, b.parent)
		if err != nil {
			r.unrunnable(wire.CodePrerequisiteUnsatisfied, "seal "+seal+" shares no merge base with "+b.parent, "pass the sealed lane commit the batch merged, then rerun")
			return
		}
		g.seal, g.candidate, g.base = seal, seal, base
	}
	tree, err := store.ResolveTree(b.root, g.candidate)
	if err != nil {
		r.unrunnable(wire.CodePrerequisiteUnsatisfied, "candidate "+g.candidate+" has no tree in this clone", "run `git fetch origin` and rerun")
		return
	}
	g.tree = tree
	for _, id := range transaction.RequiredGates(policy, rec) {
		if b.ciRun != "" && policy.CIEvidenceGate(id) != nil {
			g.gates = append(g.gates, gateStep{id: id, via: "CI", state: "PLANNED"})
			continue
		}
		if _, err := store.PreflightGateDefinitions(policy, []string{id}); err != nil {
			g.gates = append(g.gates, gateStep{id: id, via: "NONE", state: closeUnrunnable})
			r.unrunnable(wire.CodeOf(err), "required gate "+id+" cannot run here: "+prose(err.Error()),
				"run the gate where it can run and close the ticket with claim, submit, gate run and complete; pass --ci-run FILE when policy ciEvidence admits it; or, after review, close it explicitly with --manual or `corvint-tasks ticket complete-manual`")
			continue
		}
		g.gates = append(g.gates, gateStep{id: id, via: "LOCAL", state: "PLANNED"})
	}
	g.doc = b.mergeDocument(batch, r.hit.lane, g)
	g.docSum = wire.Sum(g.doc)
	for _, e := range rec.AttachedEvidence {
		if e.AcceptanceRevision == rec.AcceptanceRevision && slices.Contains(e.Evidence, g.docSum) {
			g.attached = true
		}
	}
}

// unrunnable refuses the row before any write, keeping the first code and
// every reason.
func (r *closeRow) unrunnable(code, detail, recovery string) {
	r.disposition, r.write = closeUnrunnable, false
	if r.code == "" {
		r.code, r.recovery = code, recovery
	}
	if r.detail != "" {
		detail = r.detail + "; " + detail
	}
	r.detail = detail
}

// mergeDocument is the canonical taskman-batch-merge/0 evidence document
// whose SHA-256 (of its file form, trailing LF included) is attached to the
// ticket (TCD-V0-188).
func (b *gatedBatch) mergeDocument(batch, lane int64, g *gatedRow) []byte {
	o := wire.NewObject()
	o.Set("profile", wire.String(profileBatchMerge))
	o.Set("batchPr", wire.String(strconv.FormatInt(batch, 10)))
	o.Set("prUrl", wire.String(b.prURL))
	o.Set("mergeCommit", wire.String(b.merge))
	o.Set("mergedAt", orNull(b.mergedAt))
	o.Set("runUrl", wire.String(b.runURL))
	o.Set("lanePr", nullableLane(lane))
	o.Set("sealCommit", orNull(g.seal))
	o.Set("candidateCommit", wire.String(g.candidate))
	o.Set("baseCommit", wire.String(g.base))
	return wire.EncodeFile(wire.ObjectValue(o))
}

// gatedRequestID derives one step's request id from the batch, the ticket,
// its count of earlier attempts and the step, so a rerun after a released
// attempt claims afresh while a rerun of the same attempt replays
// (TCD-V0-189).
func gatedRequestID(batch int64, ticketID, local string, prior int, step string) string {
	n := strconv.FormatInt(batch, 10)
	id := "batch-close-" + n + "-" + local + "-a" + strconv.Itoa(prior) + "-" + step
	if len(id) <= wire.MaxRequestIDBytes {
		return id
	}
	sum := sha256.Sum256([]byte(ticketID + "\x00" + strconv.Itoa(prior) + "\x00" + step))
	return "batch-close-" + n + "-" + hex.EncodeToString(sum[:12])
}

// runGated closes one planned row: attach the merge evidence, claim with a
// scope derived from the candidate's diff, submit its tree, run every
// required gate on a clean detached worktree of the candidate (or record the
// retained CI run), and complete at the candidate commit. A failure after
// the claim releases the attempt and leaves the ticket OPEN.
func (r *closeRow) runGated(env Env, cmd []string, actor mutation.Binding, b *gatedBatch, batch int64) {
	g := r.g
	// id names one step's request id; the row reports the last one sent
	// before any release.
	id := func(step string) string {
		v := gatedRequestID(batch, r.hit.ticketID, r.local, g.prior, step)
		if step != "release" {
			r.requestID = v
		}
		return v
	}
	lease := func(name string, args ...string) *wire.Result {
		return leaseCommand(env, name, append(args, "--role", b.role))
	}
	if !g.attached {
		g.step = "attach"
		reason := "Merged in batch PR #" + strconv.FormatInt(batch, 10) + " as " + b.merge + "; gated close of " + g.candidate + "; evidence is the " + profileBatchMerge + " document"
		payload := wire.ObjectValue(wire.NewObject().Set("reason", wire.String(reason)).Set("evidence", wire.Array(wire.String(string(g.docSum)))))
		payload, err := mutation.CanonicalPayload(mutation.OpAttachEvidence, payload)
		if err != nil {
			r.fail(err.Error(), wire.CodeOf(err))
			return
		}
		f := mutateFlags{role: actor.Role, requestID: id("evidence"), target: r.hit.ticketID, expected: r.revision, issuedAt: b.issued}
		if res := submitMutation(env, cmd, mutation.OpAttachEvidence, actor, f, payload); res.Outcome != wire.OutcomeOK {
			r.failResult(res)
			return
		}
		g.attached = true
	}
	g.step = "claim"
	res := lease("claim", r.hit.ticketID, "--holder", batchCloseHolder, "--request-id", id("claim"), "--base", g.base, "--scope-from-diff", g.candidate, "--lease-minutes", batchCloseLeaseMinutes)
	if res.Outcome != wire.OutcomeOK {
		r.failResult(res)
		if r.code == wire.CodeOutOfScope {
			r.recovery = "the candidate diff touches paths outside the ticket's declared scope, which batch-close never widens; pass --seal " + r.local + "=<sealed lane commit> so the diff is the lane's own, or close it with claim, submit, gate run and complete by hand"
		}
		return
	}
	g.attemptID, g.generation = resultString(res, "attemptId"), resultString(res, "generation")
	g.step = "submit"
	if res = lease("submit", "--attempt", g.attemptID, "--generation", g.generation, "--tree", g.tree, "--request-id", id("submit")); res.Outcome != wire.OutcomeOK {
		r.failResult(res)
		r.release(lease, id("release"))
		return
	}
	g.step = "gate"
	if !r.runGates(b, lease, id) {
		r.release(lease, id("release"))
		return
	}
	g.step = "complete"
	if res = lease("complete", "--attempt", g.attemptID, "--generation", g.generation, "--request-id", id("complete"), "--commit", g.candidate); res.Outcome != wire.OutcomeOK {
		r.failResult(res)
		r.release(lease, id("release"))
		return
	}
	r.disposition, r.code, r.recovery, g.completionKind = closeClosed, "", "", "VERIFIED"
	r.receipt = resultString(res, "receipt")
	r.resultingRevision = resultString(res, "resultingRevision")
}

// runGates runs each required gate in order; local gates share one clean
// detached worktree of the candidate that is removed on every path.
func (r *closeRow) runGates(b *gatedBatch, lease func(string, ...string) *wire.Result, id func(string) string) bool {
	g := r.g
	local := false
	for _, s := range g.gates {
		local = local || s.via == "LOCAL"
	}
	run := func(wt string) bool {
		for i := range g.gates {
			s := &g.gates[i]
			args := []string{"--attempt", g.attemptID, "--generation", g.generation, "--gate", s.id, "--request-id", id("gate" + strconv.Itoa(i))}
			if s.via == "CI" {
				args = append(args, "--ci-run", b.ciRun)
			} else {
				args = append(args, "--worktree", wt)
			}
			res := lease("gate run", args...)
			if res.Outcome != wire.OutcomeOK {
				s.state = "REFUSED"
				r.failResult(res)
				return false
			}
			item := res.Items[0]
			s.state = "PASSED"
			if v, ok := item.Obj.Get("reusedFrom"); ok && v.Kind != wire.KindNull {
				s.state = "REUSED"
			} else if v, ok := item.Obj.Get("gateResult"); ok && v.Kind != wire.KindNull {
				if st, _ := v.Obj.Get("state"); st.Str != "PASSED" {
					s.state = st.Str
					why := ""
					if c, ok := v.Obj.Get("outcomeClass"); ok && c.Kind == wire.KindString {
						why = " (" + c.Str
						if e, ok := v.Obj.Get("exitCode"); ok && e.Kind == wire.KindString {
							why += " " + e.Str
						}
						if f, ok := v.Obj.Get("failingLine"); ok && f.Kind == wire.KindString {
							why += ": " + f.Str
						}
						why += ")"
					}
					r.fail("required gate "+s.id+" is "+st.Str+why+" at "+g.candidate, wire.CodeGateFailed)
					r.recovery = "fix the failure and rerun; the attempt was released and the ticket stays OPEN"
					return false
				}
			}
		}
		return true
	}
	if !local {
		return run("")
	}
	ok := false
	if err := store.WithDetachedWorktree(b.root, g.candidate, func(wt string) error {
		ok = run(wt)
		return nil
	}); err != nil {
		if ok || r.disposition != closeFailed {
			r.fail(err.Error(), wire.CodeOf(err))
		}
		return false
	}
	return ok
}

// release returns a failed row's attempt so the ticket stays OPEN and
// claimable; a refused release is named on the row.
func (r *closeRow) release(lease func(string, ...string) *wire.Result, requestID string) {
	g := r.g
	args := []string{"--attempt", g.attemptID, "--generation", g.generation, "--request-id", requestID}
	if r.code != "" && r.code != wire.CodeHandoff && r.code != wire.CodeReviewReturned && slices.Contains(wire.Codes, r.code) {
		args = append(args, "--reason", r.code)
	}
	if res := lease("release", args...); res.Outcome != wire.OutcomeOK {
		r.detail += "; the attempt could not be released (" + strings.Join(res.Codes, ",") + "): release it with `corvint-tasks release --attempt " + g.attemptID + " --generation " + g.generation + "`"
		return
	}
	r.detail += "; attempt " + g.attemptID + " was released and the ticket stays OPEN"
}

// fail marks the row FAILED at its current step.
func (r *closeRow) fail(detail, code string) {
	if code == "" {
		code = wire.CodeMalformed
	}
	r.disposition, r.code, r.detail = closeFailed, code, prose(detail)
	r.recovery = "fix the cause and rerun the same command: closed tickets are skipped and this one claims a fresh attempt"
}

// failResult marks the row FAILED with a refused command's first code and
// its warnings.
func (r *closeRow) failResult(res *wire.Result) {
	code := ""
	if len(res.Codes) > 0 {
		code = res.Codes[0]
	}
	r.fail(strings.Join(res.Warnings, "; "), code)
}

// resultString is a string member of a result's single item, "" otherwise.
func resultString(res *wire.Result, key string) string {
	if len(res.Items) != 1 || res.Items[0].Obj == nil {
		return ""
	}
	v, ok := res.Items[0].Obj.Get(key)
	if !ok || v.Kind != wire.KindString {
		return ""
	}
	return v.Str
}

// gatedRowValue adds the gated members of one taskman-batch-close/1 row.
func gatedRowValue(o *wire.Object, r closeRow) {
	o.Set("mode", wire.String("GATED"))
	g := r.g
	if g == nil {
		for _, k := range []string{"step", "candidateCommit", "baseCommit", "sealCommit", "attemptId", "completionKind", "mergeEvidence"} {
			o.Set(k, wire.Null())
		}
		o.Set("gates", wire.Array())
		return
	}
	o.Set("step", orNull(g.step))
	o.Set("candidateCommit", wire.String(g.candidate))
	o.Set("baseCommit", wire.String(g.base))
	o.Set("sealCommit", orNull(g.seal))
	gates := make([]wire.Value, 0, len(g.gates))
	for _, s := range g.gates {
		gates = append(gates, wire.ObjectValue(wire.NewObject().Set("gateId", wire.String(s.id)).Set("via", wire.String(s.via)).Set("state", wire.String(s.state))))
	}
	o.Set("gates", wire.Array(gates...))
	o.Set("attemptId", orNull(g.attemptID))
	o.Set("completionKind", orNull(g.completionKind))
	doc, _ := wire.Parse(g.doc)
	o.Set("mergeEvidence", wire.ObjectValue(wire.NewObject().Set("sha256", wire.String(string(g.docSum))).Set("attached", wire.Bool(g.attached)).Set("document", doc)))
}
