package cli

// The read verbs `ticket obligations boundary` (TOL-V0-079, TOL-V0-083) and
// `ticket obligations worklist` (TOL-V0-086..088), and the scope listing
// show adds (TOL-V0-081). Each writes nothing.
import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/obligation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// obligationWorklistNow is the worklist's clock for review age; a variable
// only so tests can pin it.
var obligationWorklistNow = time.Now

// obligationScopeValue is the TOL-V0-081 show listing: entries out of scope
// by an accepted decision, and pending proposals, each with its decision.
// ok is false when no entry records a scope decision, so show keeps its
// exact earlier output.
func obligationScopeValue(l *ticket.ObligationLedger) (outOfScope, proposals wire.Value, ok bool) {
	var out, prop []wire.Value
	for _, e := range l.Entries {
		if e.Scope == nil {
			continue
		}
		ok = true
		v := wire.ObjectValue(wire.NewObject().Set("id", wire.String(e.ID)).Set("core", wire.Bool(e.Core)).Set("state", wire.String(e.State)).Set("scope", e.Scope.Value()))
		switch {
		case e.Scope.Status == ticket.ObligationScopeProposed:
			prop = append(prop, v)
		case !e.Core:
			out = append(out, v)
		}
	}
	return wire.Array(out...), wire.Array(prop...), ok
}

// obligationRepoRoot resolves a cite or witness repository to a worktree
// root: the store's own worktree for nil, else the declared test
// repository.
func obligationRepoRoot(env Env, rc *readCtx, repo *string) (string, error) {
	if repo == nil {
		return store.WorktreeRoot(env.Cwd)
	}
	root, _, err := declaredTestRepository(rc, *repo)
	return root, err
}

// obligationsBoundary is the TOL-V0-079/083 read. It lists every repository
// a pending entry cites at its HEAD, reports each one's test inventory, and
// classifies each pending entry; out-of-scope entries that are now
// TESTABLE are listed as re-scope candidates.
func obligationsBoundary(env Env, cmd []string, args []string) *wire.Result {
	var target string
	if res := pairFlags(cmd, args, map[string]*string{"--target": &target}, nil, nil); res != nil {
		return res
	}
	if target == "" {
		return usage(cmd, "--target is required")
	}
	var rec *ticket.Record
	var ledger *ticket.ObligationLedger
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		r, l, ferr := obligationTarget(rc, target)
		if ferr != nil {
			return ferr
		}
		rec, ledger = r, l
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	res := success(cmd, rc)
	res.Items = []wire.Value{}
	if ledger == nil {
		return res
	}
	// The primary repository is always checked; each cited label once.
	cited := map[string][]string{"": nil}
	repos := map[string]*string{"": nil}
	for _, e := range ledger.Entries {
		if !obligation.BoundaryPending(e) {
			continue
		}
		for _, c := range e.Cites {
			k := obligation.BoundaryRepoKey(c.Repository)
			cited[k] = append(cited[k], c.Path)
			repos[k] = c.Repository
		}
	}
	keys := make([]string, 0, len(repos))
	for k := range repos {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	inv := map[string]*obligation.BoundaryInventory{}
	repoItems := make([]wire.Value, 0, len(keys))
	for _, k := range keys {
		b := obligation.NewBoundaryInventory(repos[k], cited[k])
		b.Patterns = obligationPolicy(rc).ObligationTestFilePatterns()
		inv[k] = b
		root, err := obligationRepoRoot(env, rc, repos[k])
		if err == nil {
			b.Commit, _, _, err = store.FilesAtCommit(root, "HEAD", nil)
			if err == nil && b.Commit == "" {
				err = wire.Errorf(wire.CodeMissingEvidence, "HEAD", "the repository has no HEAD commit")
			}
		}
		if err == nil {
			_, err = store.TreePathsAtCommit(root, b.Commit, nil, b.Visit)
		}
		if err != nil {
			b.Reason = prose(err.Error())
		}
		repoItems = append(repoItems, b.Value())
	}
	var reviewed []*ticket.ObligationReviewRecord
	for _, e := range ledger.Entries {
		if obligation.BoundaryPending(e) && e.Review != nil {
			reviewed = append(reviewed, e.Review)
		}
	}
	subjects := reviewedSubjectValues(env, rc, reviewed)
	var entries, none, rescope []wire.Value
	for _, e := range ledger.Entries {
		if !obligation.BoundaryPending(e) {
			continue
		}
		verdict, reason := obligation.BoundaryVerdict(e, inv)
		cites := make([]wire.Value, 0, len(e.Cites))
		for _, c := range e.Cites {
			tracked := wire.Null()
			if t, ok := obligation.BoundaryCiteTracked(c, inv); ok {
				tracked = wire.Bool(t)
			}
			cites = append(cites, wire.ObjectValue(wire.NewObject().Set("repository", wire.StringOrNull(c.Repository)).Set("path", wire.String(c.Path)).Set("tracked", tracked)))
		}
		o := wire.NewObject().Set("id", wire.String(e.ID)).Set("state", wire.String(e.State)).Set("core", wire.Bool(e.Core)).
			Set("verdict", wire.String(verdict)).Set("reason", wire.String(reason)).Set("cites", wire.Array(cites...))
		if e.Scope != nil {
			o.Set("scope", e.Scope.Value())
		}
		if e.Review != nil {
			o.Set("reviewedSubject", subjects[e.Review])
		}
		entries = append(entries, wire.ObjectValue(o))
		if verdict == obligation.BoundaryNoTestBoundary {
			none = append(none, wire.String(e.ID))
		}
		if obligation.BoundaryRescopeCandidate(e, verdict) {
			rescope = append(rescope, wire.ObjectValue(wire.NewObject().Set("id", wire.String(e.ID)).Set("scope", e.Scope.Value())))
		}
	}
	res.Items = []wire.Value{wire.ObjectValue(wire.NewObject().Set("ticketId", wire.String(rec.TicketID.Raw)).
		Set("repositories", wire.Array(repoItems...)).Set("entries", wire.Array(entries...)).
		Set("noTestBoundary", wire.Array(none...)).Set("rescopeCandidates", wire.Array(rescope...)))}
	res.Untrusted = true
	return res
}

// obligationsWorklist is the TOL-V0-086..088 read: the ranked queue of
// reviewed shortfalls of --target, or of every OPEN or HELD ticket with a
// ledger, with the excluded entries and the needs-obligation-text listing
// beside it. --next K returns the head plus up to K items citing the same
// test file.
func obligationsWorklist(env Env, cmd []string, args []string) *wire.Result {
	var target, rank, next string
	if res := pairFlags(cmd, args, map[string]*string{"--target": &target, "--rank": &rank, "--next": &next}, nil, nil); res != nil {
		return res
	}
	keys, err := obligation.ParseWorklistRank(rank)
	if err != nil {
		return errorResult(cmd, err)
	}
	k := -1
	if next != "" {
		n, err := strconv.Atoi(next)
		if err != nil || n < 0 || n > obligation.MaxWorklistNext {
			return usage(cmd, "--next takes a whole number from 0 to "+strconv.Itoa(obligation.MaxWorklistNext))
		}
		k = n
	}
	var inputs []obligation.WorklistInput
	var unknown, findings []wire.Value
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		inputs, unknown, findings = nil, nil, nil
		read := store.ObligationEventReader(rc.repo)
		mirrors := newMirrorFolds(rc, rc.store.Inventory, nil)
		var ids []string
		if target != "" {
			id, err := resolveTicketArg(rc, target)
			if err != nil {
				return err
			}
			if _, ok := rc.store.Inventory.Get(id); !ok {
				return wire.Errorf(wire.CodeMalformed, "--target", "ticket %s does not exist in this queue", id)
			}
			ids = []string{id}
		} else {
			ids = rc.store.Inventory.IDs()
		}
		for _, id := range ids {
			rec, _ := rc.store.Inventory.Get(id)
			if rec.ObligationsRef == nil || (target == "" && rec.Status != ticket.StatusOpen && rec.Status != ticket.StatusHeld) {
				continue
			}
			l, ferr := mirrors.fold(rec)
			if ferr != nil {
				unknown = append(unknown, wire.ObjectValue(wire.NewObject().Set("ticketId", wire.String(id)).Set("code", wire.String(chainCode(ferr))).Set("detail", wire.String(prose(ferr.Error())))))
				continue
			}
			// TOL-V0-122: a decided mirror is not a shortfall of this ticket.
			rs := mirrors.resolve(l)
			findings = append(findings, obligation.MirrorFindings(id, rs, obligation.MirrorsCompletable(l, rs))...)
			decided := map[string]bool{}
			for _, r := range rs {
				decided[r.ID] = r.Decided
			}
			var remaining int64
			for _, e := range l.Entries {
				if e.Core && obligation.BoundaryPending(e) {
					remaining++
				}
			}
			for _, e := range l.Entries {
				if e.Review == nil || e.Review.Outcome != ticket.ObligationReviewHeld || !obligation.BoundaryPending(e) {
					continue
				}
				in := obligation.WorklistInput{TicketID: id, Priority: rec.Priority, Remaining: remaining, Entry: e, MirrorDecided: decided[e.ID]}
				raw, err := read(e.Review.EventSha256)
				if err != nil || raw == nil {
					return wire.Errorf(wire.CodeMissingEvidence, "evidence", "review event %s of %s %s is unreadable", e.Review.EventSha256, id, e.ID)
				}
				ev, err := ticket.DecodeObligationEvent(raw)
				if err != nil {
					return err
				}
				in.ReviewedAt = ev.Decoded.IssuedAt
				in.TestRepository = e.Review.TestRepository
				inputs = append(inputs, in)
			}
		}
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	// The cited example must be unchanged at its repository's HEAD; a
	// repository that cannot be read leaves the item unverified.
	byRepo := map[string][]int{}
	for i, in := range inputs {
		key := obligation.BoundaryRepoKey(in.TestRepository)
		byRepo[key] = append(byRepo[key], i)
	}
	for _, idx := range byRepo {
		repo := inputs[idx[0]].TestRepository
		var paths []string
		for _, i := range idx {
			paths = append(paths, inputs[i].Entry.Review.Review.Subject.Path)
		}
		root, err := obligationRepoRoot(env, rc, repo)
		var content map[string][]byte
		if err == nil {
			var commit string
			commit, _, content, err = store.FilesAtCommit(root, "HEAD", paths)
			if err == nil && commit == "" {
				err = wire.Errorf(wire.CodeMissingEvidence, "HEAD", "the repository has no HEAD commit")
			}
		}
		for _, i := range idx {
			if err != nil {
				inputs[i].Unverified = true
				continue
			}
			s := inputs[i].Entry.Review.Review.Subject
			body, ok := content[s.Path]
			sum := sha256.Sum256(body)
			inputs[i].Stale = !ok || wire.Digest(hex.EncodeToString(sum[:])) != s.BlobSha256
		}
	}
	reviewed := make([]*ticket.ObligationReviewRecord, 0, len(inputs))
	for _, in := range inputs {
		reviewed = append(reviewed, in.Entry.Review)
	}
	subjects := reviewedSubjectValues(env, rc, reviewed)
	now := obligationWorklistNow().UTC()
	var items []obligation.WorklistInput
	var excluded, needsText []wire.Value
	for _, in := range inputs {
		class, reason := obligation.ClassifyWorklist(in)
		switch class {
		case obligation.WorklistItem:
			items = append(items, in)
		case obligation.WorklistExcluded:
			excluded = append(excluded, wire.ObjectValue(wire.NewObject().Set("ticketId", wire.String(in.TicketID)).Set("id", wire.String(in.Entry.ID)).Set("reason", wire.String(reason)).
				Set("reviewedSubject", subjects[in.Entry.Review])))
		case obligation.WorklistNeedsText:
			needsText = append(needsText, obligation.WorklistNeedsTextValue(in))
		}
	}
	obligation.RankWorklist(items, keys)
	if k >= 0 {
		items = obligation.WorklistNext(items, k)
	}
	vs := make([]wire.Value, 0, len(items))
	for _, in := range items {
		v := obligation.WorklistItemValue(in, now)
		v.Obj.Set("reviewedSubject", subjects[in.Entry.Review])
		vs = append(vs, v)
	}
	rankVals := make([]wire.Value, 0, len(keys))
	for _, key := range keys {
		rankVals = append(rankVals, wire.String(key))
	}
	res := success(cmd, rc)
	res.Items = []wire.Value{wire.ObjectValue(wire.NewObject().Set("rank", wire.Array(rankVals...)).Set("items", wire.Array(vs...)).
		Set("excluded", wire.Array(excluded...)).Set("needsObligationText", wire.Array(needsText...)).Set("unknown", wire.Array(unknown...)))}
	if len(findings) > 0 {
		res.Items[0].Obj.Set("findings", wire.Array(findings...))
	}
	res.Untrusted = true
	return res
}
