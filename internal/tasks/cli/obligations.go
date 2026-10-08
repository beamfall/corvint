package cli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/obligation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// obligationQualifiedVersions is the TOL-V0-009 qualified Playwright version
// list a report is admitted against. It is a variable only so tests can
// exercise the admitted path with a synthetic version; the shipped list is
// obligation.QualifiedVersions, empty until the live fixture passes.
var obligationQualifiedVersions = obligation.QualifiedVersions

// obligationsCommand runs `ticket obligations seed|set|witness|show|plan`
// (TOL-V0-005..009, TOL-V0-020). The writes are ordinary OBLIGATIONS_*
// mutations; show and plan are reads that write nothing.
func obligationsCommand(env Env, args []string) *wire.Result {
	if len(args) == 0 {
		return usage([]string{"ticket", "obligations"}, "ticket obligations needs a verb: seed, set, witness, show or plan")
	}
	cmd := []string{"ticket", "obligations", args[0]}
	switch args[0] {
	case "seed":
		return obligationsPayloadWrite(env, cmd, ticket.OpObligationsSeed, args[1:])
	case "set":
		return obligationsPayloadWrite(env, cmd, ticket.OpObligationsSet, args[1:])
	case "witness":
		return obligationsWitness(env, cmd, args[1:])
	case "show":
		return obligationsShow(env, cmd, args[1:])
	case "plan":
		return obligationsPlan(env, cmd, args[1:])
	}
	return usage([]string{"ticket", "obligations"}, "unknown ticket obligations verb "+args[0])
}

// obligationsPayloadWrite submits a caller-composed SEED or SET payload. The
// id-keyed array is sorted at this input boundary; every other check is the
// writer's.
func obligationsPayloadWrite(env Env, cmd []string, op string, args []string) *wire.Result {
	f, res := parseMutateFlags(cmd, args)
	if res != nil {
		return res
	}
	if f.help || f.template || f.batch || f.verbose {
		return usage(cmd, "ticket obligations takes --target, --expected-revision, --request-id, --payload or --payload-stdin, --issued-at and --role")
	}
	if f.requestID == "" {
		return usage(cmd, "--request-id is required: it is the idempotency key of this mutation")
	}
	if f.target == "" || f.expected == "" {
		return usage(cmd, "--target and --expected-revision are required")
	}
	actor, err := initActor(f.role)
	if err != nil {
		return errorResult(cmd, err)
	}
	payload, err := readPayload(env, f)
	if err == nil {
		ticket.SortObligationPayload(op, payload)
		payload, err = mutation.CanonicalPayload(op, payload)
	}
	if err != nil {
		return errorResult(cmd, err)
	}
	return submitMutation(env, cmd, op, actor, f, payload)
}

// obligationTarget reads the target's record and folded ledger. A ticket
// without a ledger returns a nil ledger; a broken chain returns the typed
// chain error.
func obligationTarget(rc *readCtx, target string) (*ticket.Record, *ticket.ObligationLedger, error) {
	id, err := resolveTicketArg(rc, target)
	if err != nil {
		return nil, nil, err
	}
	rec, ok := rc.store.Inventory.Get(id)
	if !ok {
		return nil, nil, wire.Errorf(wire.CodeMalformed, "--target", "ticket %s does not exist in this queue", id)
	}
	if rec.ObligationsRef == nil {
		return rec, nil, nil
	}
	l, err := ticket.FoldObligationChain(rec.TicketID, *rec.ObligationsRef, store.ObligationEventReader(rc.repo))
	return rec, l, err
}

// chainCode returns the closed code of a fold error.
func chainCode(err error) string {
	var ce *ticket.ChainError
	if errors.As(err, &ce) {
		return ce.Code
	}
	return wire.CodeOf(err)
}

// obligationsShow is the TOL-V0-006 read: the folded ledger, the reference
// counts and the head with the receipt sequence that posted it. A ticket
// without a ledger is an empty result; a chain that cannot be folded is
// UNKNOWN with a typed diagnostic, never an empty or partial ledger.
func obligationsShow(env Env, cmd []string, args []string) *wire.Result {
	var target string
	if res := pairFlags(cmd, args, map[string]*string{"--target": &target}, nil, nil); res != nil {
		return res
	}
	if target == "" {
		return usage(cmd, "--target is required")
	}
	var item *wire.Value
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		item = nil
		rec, l, ferr := obligationTarget(rc, target)
		if rec == nil {
			return ferr
		}
		ref := rec.ObligationsRef
		if ref == nil {
			return nil
		}
		o := wire.NewObject().Set("ticketId", wire.String(rec.TicketID.Raw)).Set("reference", ref.Value())
		if ferr != nil {
			o.Set("ledger", wire.String("UNKNOWN")).Set("entries", wire.Null()).Set("headSeq", wire.Null())
			o.Set("diagnostic", wire.ObjectValue(wire.NewObject().Set("code", wire.String(chainCode(ferr))).Set("detail", wire.String(prose(ferr.Error())))))
			v := wire.ObjectValue(o)
			item = &v
			return nil
		}
		seq := wire.String(string(ticket.NotObserved))
		if !rc.journalAbsent {
			path := "evidence/" + string(ref.Head)
			proof, err := auditState(rc, path)
			if err != nil {
				return err
			}
			if r, ok := proof.Records[path]; ok && r.Sha256 != nil && *r.Sha256 == ref.Head {
				seq = wire.String(string(r.Seq))
			} else {
				seq = wire.String("UNKNOWN")
			}
		}
		o.Set("ledger", wire.String("KNOWN")).Set("entries", l.EntriesValue()).Set("headSeq", seq).Set("diagnostic", wire.Null())
		v := wire.ObjectValue(o)
		item = &v
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	res := success(cmd, rc)
	res.Items = []wire.Value{}
	if item != nil {
		res.Items = []wire.Value{*item}
		res.Untrusted = true
	}
	return res
}

// obligationsPlan is the TOL-V0-020 read-only plan check. It exits 0 only
// when every open obligation is assigned to exactly one planned test.
func obligationsPlan(env Env, cmd []string, args []string) *wire.Result {
	var target, file string
	if res := pairFlags(cmd, args, map[string]*string{"--target": &target, "--plan": &file}, nil, nil); res != nil {
		return res
	}
	if target == "" || file == "" {
		return usage(cmd, "--target and --plan FILE are required")
	}
	raw, err := readPlanFile(file, ticket.MaxObligationPlanBytes)
	if err != nil {
		return errorResult(cmd, err)
	}
	plan, err := ticket.DecodeObligationPlan(raw)
	if err != nil {
		return errorResult(cmd, err)
	}
	var findings []ticket.ObligationPlanFinding
	ok := false
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		rec, l, ferr := obligationTarget(rc, target)
		if ferr != nil {
			return ferr
		}
		if plan.TicketID.Raw != rec.TicketID.Raw {
			return wire.Errorf(wire.CodeMalformed, "/ticketId", "the plan names %s, not %s", plan.TicketID.Raw, rec.TicketID.Raw)
		}
		findings, ok = ticket.CheckObligationPlan(l, plan)
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	items := make([]wire.Value, 0, len(findings))
	for _, f := range findings {
		items = append(items, wire.ObjectValue(wire.NewObject().Set("kind", wire.String(f.Kind)).Set("id", wire.String(f.ID)).
			Set("testCount", wire.String(string(wire.CountOf(int64(len(f.Tests)))))).Set("tests", wire.Strings(f.Tests))))
	}
	res := success(cmd, rc)
	res.Items = []wire.Value{wire.ObjectValue(wire.NewObject().Set("ok", wire.Bool(ok)).Set("findings", wire.Array(items...)))}
	res.Untrusted = true
	if !ok {
		res.Outcome = wire.OutcomeRefused
		res.Warnings = append(res.Warnings, "the plan leaves open obligations unassigned, split or names unknown or closed ids")
	}
	return res
}

// readPlanFile reads at most max bytes of file, refusing a larger one.
func readPlanFile(file string, max int) ([]byte, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, wire.Errorf(wire.CodeMissingEvidence, "--plan", "the plan file is unreadable")
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, int64(max)+1))
	if err != nil {
		return nil, wire.Errorf(wire.CodeMissingEvidence, "--plan", "the plan file is unreadable")
	}
	if len(raw) > max {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "--plan", "the plan exceeds %d bytes", max)
	}
	return raw, nil
}

// obligationsWitness composes and submits OBLIGATIONS_WITNESS (TOL-V0-008).
// A report witness reads the report once, recomputes TOL-V0-010..012 at the
// declared commit and hands the recomputation to the writer, which refuses a
// payload that differs (TOL-V0-013). The report itself is never retained.
func obligationsWitness(env Env, cmd []string, args []string) *wire.Result {
	f := mutateFlags{role: "OWNER"}
	var report, commit, idsArg, declared, manifest, testID, reason, attempt, generation string
	if res := pairFlags(cmd, args, map[string]*string{
		"--target": &f.target, "--role": &f.role, "--request-id": &f.requestID, "--expected-revision": &f.expected,
		"--issued-at": &f.issuedAt, "--from-playwright-report": &report, "--commit": &commit, "--ids": &idsArg,
		"--declared": &declared, "--manifest-sha256": &manifest, "--test-id": &testID, "--reason": &reason,
		"--attempt": &attempt, "--generation": &generation,
	}, nil, nil); res != nil {
		return res
	}
	if f.requestID == "" {
		return usage(cmd, "--request-id is required: it is the idempotency key of this mutation")
	}
	if f.target == "" || f.expected == "" || commit == "" {
		return usage(cmd, "--target, --expected-revision and --commit are required")
	}
	if (report == "") == (declared == "") {
		return usage(cmd, "exactly one of --from-playwright-report FILE and --declared ID,... is required")
	}
	if report != "" && (manifest != "" || testID != "" || reason != "") {
		return usage(cmd, "--manifest-sha256, --test-id and --reason belong to --declared")
	}
	if declared != "" && (manifest == "" || testID == "" || reason == "" || idsArg != "") {
		return usage(cmd, "--declared needs --manifest-sha256, --test-id and --reason, and takes no --ids")
	}
	if (attempt == "") != (generation == "") {
		return usage(cmd, "--attempt and --generation are given together or not at all")
	}
	if _, err := wire.ParseOID("--commit", commit); err != nil {
		return errorResult(cmd, err)
	}
	actor, err := initActor(f.role)
	if err != nil {
		return errorResult(cmd, err)
	}
	var ids []string
	if idsArg != "" || declared != "" {
		if ids, err = obligationIDList(idsArg + declared); err != nil {
			return errorResult(cmd, err)
		}
	}
	var rec *ticket.Record
	var ledger *ticket.ObligationLedger
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		var ferr error
		rec, ledger, ferr = obligationTarget(rc, f.target)
		if ferr != nil {
			return ferr
		}
		if ledger == nil {
			return wire.Errorf(wire.CodeMalformed, "--target", "%s ticket %s has no obligation ledger; seed it first", ticket.ObligationUnknownDetail, rec.TicketID.Raw)
		}
		// A retry of a recorded witness derives its credits from the ledger
		// before that write, so it rebuilds the same request bytes and
		// replays (or conflicts) rather than reporting written:false.
		before, found, ferr := ticket.ObligationLedgerBeforeRequest(rec.TicketID, *rec.ObligationsRef, store.ObligationEventReader(rc.repo), f.requestID)
		if ferr != nil {
			return ferr
		}
		if found && before != nil {
			ledger = before
		}
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	root, err := store.WorktreeRoot(env.Cwd)
	if err != nil {
		return errorResult(cmd, err)
	}
	w := ticket.ObligationWitnessPayload{Commit: commit}
	if attempt != "" {
		g := wire.Size(generation)
		w.Attempt, w.Generation = &attempt, &g
	}
	lists := witnessLists{}
	var check *mutation.ObligationReportCheck
	if declared != "" {
		resolved, _, _, err := store.FilesAtCommit(root, commit, nil)
		if err != nil {
			return errorResult(cmd, err)
		}
		if resolved == "" {
			return errorResult(cmd, wire.Errorf(wire.CodeMissingEvidence, "--commit", "the repository holds no commit %s", commit))
		}
		w.Source, w.Commit = ticket.ObligationSourceDeclared, resolved
		w.Declaration = &ticket.ObligationDeclaration{ManifestSha256: wire.Digest(manifest), TestID: testID, Reason: reason}
		for _, id := range ids {
			if e := ledger.Entry(id); e != nil && e.State == ticket.ObligationWitnessed {
				lists.alreadyWitnessed = append(lists.alreadyWitnessed, id)
				continue
			} else if e != nil && e.State == ticket.ObligationDeferred {
				// TOL-V0-008: a DEFERRED entry is not credited, as in the
				// report path, so it neither fails the batch nor writes.
				continue
			}
			w.Credits = append(w.Credits, ticket.ObligationCredit{ID: id})
			lists.credited = append(lists.credited, id)
		}
	} else {
		rep, err := obligation.ReadReport(report, obligationQualifiedVersions())
		if err != nil {
			return errorResult(cmd, err)
		}
		if resolved, err := filepath.EvalSymlinks(rep.RootDir); err == nil {
			rep.RootDir = resolved
		}
		paths := rep.SourcePaths(ledger.Prefix, root)
		resolved, present, content, err := store.FilesAtCommit(root, commit, paths)
		if err != nil {
			return errorResult(cmd, err)
		}
		if resolved == "" {
			return errorResult(cmd, wire.Errorf(wire.CodeMissingEvidence, "--commit", "the repository holds no commit %s", commit))
		}
		res, err := obligation.Classify(rep, ledger, root, obligation.Source{Present: present, Content: content}, ids)
		if err != nil {
			return errorResult(cmd, err)
		}
		check = &mutation.ObligationReportCheck{ReportSha256: rep.Sha256, PlaywrightVersion: rep.Version, Commit: resolved, IDs: ids, Eligible: res.Eligible}
		version, sum := rep.Version, rep.Sha256
		w.Source, w.Commit, w.ReportSha256, w.PlaywrightVersion = ticket.ObligationSourceReport, resolved, &sum, &version
		w.Credits = check.ExpectedCredits(ledger)
		lists = witnessLists{credited: res.Credited, alreadyWitnessed: res.AlreadyWitnessed, conflicting: res.Conflicting,
			failed: res.Failed, unknown: res.Unknown, unknownTruncated: res.UnknownTruncated, unbound: res.Unbound, unmatched: res.Unmatched}
	}
	if len(w.Credits) == 0 {
		out := success(cmd, rc)
		o := wire.NewObject().Set("written", wire.Bool(false)).Set("ticketId", wire.String(rec.TicketID.Raw))
		lists.set(o)
		out.Items = []wire.Value{wire.ObjectValue(o)}
		out.Untrusted = true
		return out
	}
	ctx := writerContext()
	if check != nil {
		ctx = store.WithObligationReport(ctx, check)
	}
	out := submitMutationContext(ctx, env, cmd, ticket.OpObligationsWitness, actor, f, w.Value())
	if len(out.Items) == 1 && out.Items[0].Kind == wire.KindObject && out.Items[0].Obj != nil {
		written := out.Outcome == wire.OutcomeOK
		out.Items[0].Obj.Set("written", wire.Bool(written))
		lists.set(out.Items[0].Obj)
	}
	return out
}

// obligationIDList parses a comma-separated id list into a sorted, unique
// list of at most the ledger bound.
func obligationIDList(s string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, id := range strings.Split(s, ",") {
		if _, err := wire.ParseObligationID("--ids", id); err != nil {
			return nil, err
		}
		if seen[id] {
			return nil, wire.Errorf(wire.CodeDuplicateID, "--ids", "obligation id %s is named twice", id)
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) > wire.ObligationsMaxEntries {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "--ids", "at most %d ids", wire.ObligationsMaxEntries)
	}
	sort.Strings(out)
	return out, nil
}

// witnessLists are the TOL-V0-013 id lists a witness returns, each sorted
// and bounded by the ledger bound (unknown is truncated with a flag).
type witnessLists struct {
	credited, alreadyWitnessed, conflicting, failed, unknown, unmatched []string
	unknownTruncated                                                    bool
	unbound                                                             []obligation.UnboundID
}

func (l witnessLists) set(o *wire.Object) {
	strs := func(s []string) wire.Value {
		if s == nil {
			s = []string{}
		}
		return wire.Strings(s)
	}
	o.Set("credited", strs(l.credited)).Set("alreadyWitnessed", strs(l.alreadyWitnessed)).Set("conflicting", strs(l.conflicting))
	o.Set("failed", strs(l.failed)).Set("unknown", strs(l.unknown)).Set("unknownTruncated", wire.Bool(l.unknownTruncated))
	unbound := make([]wire.Value, 0, len(l.unbound))
	for _, u := range l.unbound {
		unbound = append(unbound, wire.ObjectValue(wire.NewObject().Set("id", wire.String(u.ID)).Set("reason", wire.String(u.Reason))))
	}
	o.Set("unbound", wire.Array(unbound...)).Set("unmatched", strs(l.unmatched))
}
