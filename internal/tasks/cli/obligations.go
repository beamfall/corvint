package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/evidencestore"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/obligation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// obligationQualifiedVersions is the TOL-V0-009 qualified Playwright version
// list a report is admitted against. It is a variable only so tests can
// exercise the admitted path with a synthetic version; the shipped list is
// obligation.QualifiedVersions (1.63.0, TOL-V0-126).
var obligationQualifiedVersions = obligation.QualifiedVersions

// admittedVersions is the shipped TOL-V0-009 list with the store-local
// allow-list an operator recorded with qualify-version (TOL-V0-063). An
// unreadable or malformed allow-list refuses; it never admits nothing
// silently or everything.
func admittedVersions(commonDir string) ([]string, error) {
	a, err := obligation.ReadAllowList(obligation.AllowListPath(commonDir))
	if err != nil {
		return nil, err
	}
	return append(append([]string{}, obligationQualifiedVersions()...), a.Admitted()...), nil
}

// admittedVersionsAt is admittedVersions for the store holding cwd.
func admittedVersionsAt(cwd string) ([]string, error) {
	repo, err := intent.Resolve(cwd)
	if err != nil {
		return nil, err
	}
	return admittedVersions(repo.CommonDir)
}

// declaredTestRepository resolves a policy-declared test repository name to
// its worktree root and the path the policy declares for it (TOL-V0-064,
// TOL-V0-065). An undeclared name refuses MALFORMED; a declared path that is
// not itself the root of a Git worktree refuses MISSING_EVIDENCE.
func declaredTestRepository(rc *readCtx, name string) (root, declared string, err error) {
	if _, err := wire.ParseLabel("--test-repository", name); err != nil {
		return "", "", err
	}
	var policy *intent.Policy
	if rc != nil && rc.store != nil {
		policy = rc.store.Policy
	}
	dir, ok := policy.ObligationTestRepository(name)
	if !ok {
		return "", "", wire.Errorf(wire.CodeMalformed, "--test-repository", "%s %s is not declared by the policy's obligations.testRepositories", ticket.ObligationTestRepositoryDetail, name)
	}
	root, err = store.ExactWorktreeRoot(intent.ResolveTestRepository(rc.repo.PrimaryWorktree, dir))
	if err != nil {
		return "", "", wire.Errorf(wire.CodeMissingEvidence, "--test-repository", "test repository %s (%s) is not the root of a Git worktree", name, prose(dir))
	}
	return root, dir, nil
}

// obligationsCommand runs `ticket obligations seed|set|witness|show|plan`
// (TOL-V0-005..009, TOL-V0-020) and boundary and worklist (TOL-V0-083,
// TOL-V0-086). The writes are ordinary OBLIGATIONS_* mutations; show, plan,
// boundary and worklist are reads that write nothing.
func obligationsCommand(env Env, args []string) *wire.Result {
	if len(args) == 0 {
		return usage([]string{"ticket", "obligations"}, "ticket obligations needs a verb: seed, set, witness, show, plan, boundary, worklist, capability or qualify-version")
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
	case "boundary":
		return obligationsBoundary(env, cmd, args[1:])
	case "worklist":
		return obligationsWorklist(env, cmd, args[1:])
	case "qualify-version":
		return obligationsQualifyVersion(env, cmd, args[1:])
	case "capability":
		return obligationsCapability(env, cmd, args[1:])
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
// UNKNOWN with a typed diagnostic, never an empty or partial ledger. A
// ledger whose entries record review sources adds sourceReview, the
// TOL-V0-042 stale-source listing; --legacy-cases names the
// corvint-legacy-cases/1 record legacy and manual case refs resolve in.
// reviewCandidate is the ticket's newest submitted candidate branch, read
// only when the ledger holds a reviewed entry, so staleness is judged at the
// revision under review rather than at HEAD (TOL-V0-131); "" when the queue
// has no journal or the ticket has no candidate branch.
func reviewCandidate(rc *readCtx, id string, l *ticket.ObligationLedger) (string, error) {
	if l == nil || rc.journalAbsent || rc.snap == nil {
		return "", nil
	}
	for _, e := range l.Entries {
		if e.Review != nil {
			_, branch, _, err := packetAttempts(rc, id)
			return branch, err
		}
	}
	return "", nil
}

func obligationsShow(env Env, cmd []string, args []string) *wire.Result {
	var target, casesFile string
	if res := pairFlags(cmd, args, map[string]*string{"--target": &target, "--legacy-cases": &casesFile}, nil, nil); res != nil {
		return res
	}
	if target == "" {
		return usage(cmd, "--target is required")
	}
	var item *wire.Value
	var known *ticket.ObligationLedger
	var candidate string
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		item, known, candidate = nil, nil, ""
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
		// TOL-V0-121: mirror entries resolved against their owner ledgers,
		// present only when the ledger declares a mirror.
		if v, ok := mirrorsValue(newMirrorFolds(rc, rc.store.Inventory, nil), l); ok {
			o.Set("mirrors", v)
		}
		v := wire.ObjectValue(o)
		item, known = &v, l
		var cerr error
		candidate, cerr = reviewCandidate(rc, rec.TicketID.Raw, l)
		return cerr
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	if known != nil {
		review, ok, err := sourceReviewValue(env, known, casesFile)
		if err != nil {
			return errorResult(cmd, err)
		}
		if ok {
			item.Obj.Set("sourceReview", review)
		}
		wr, ok, err := witnessReviewValue(env, rc, known, candidate)
		if err != nil {
			return errorResult(cmd, err)
		}
		if ok {
			item.Obj.Set("witnessReview", wr)
		}
		if out, prop, ok := obligationScopeValue(known); ok {
			item.Obj.Set("outOfScope", out).Set("scopeProposals", prop)
		}
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
// when every open obligation is assigned to exactly one planned test. A
// ledger with review sources adds the TOL-V0-042 sourceReview listing,
// which never changes ok.
func obligationsPlan(env Env, cmd []string, args []string) *wire.Result {
	var target, file, casesFile string
	var withWitnesses bool
	if res := pairFlags(cmd, args, map[string]*string{"--target": &target, "--plan": &file, "--legacy-cases": &casesFile}, nil, map[string]*bool{"--witnesses": &withWitnesses}); res != nil {
		return res
	}
	if target == "" || file == "" {
		return usage(cmd, "--target and --plan FILE are required")
	}
	raw, err := readPlanFile(env.Cwd, file, ticket.MaxObligationPlanBytes)
	if err != nil {
		return errorResult(cmd, err)
	}
	plan, err := ticket.DecodeObligationPlan(raw)
	if err != nil {
		return errorResult(cmd, err)
	}
	var findings, reruns []ticket.ObligationPlanFinding
	var ledger *ticket.ObligationLedger
	var candidate string
	ok := false
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		candidate = ""
		rec, l, ferr := obligationTarget(rc, target)
		if ferr != nil {
			return ferr
		}
		if plan.TicketID.Raw != rec.TicketID.Raw {
			return wire.Errorf(wire.CodeMalformed, "/ticketId", "the plan names %s, not %s", plan.TicketID.Raw, rec.TicketID.Raw)
		}
		findings, ok = ticket.CheckObligationPlan(l, plan)
		reruns = ticket.ObligationPlanReruns(l, plan)
		ledger = l
		if !withWitnesses {
			return nil
		}
		var cerr error
		candidate, cerr = reviewCandidate(rc, rec.TicketID.Raw, l)
		return cerr
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	review, hasReview, err := sourceReviewValue(env, ledger, casesFile)
	if err != nil {
		return errorResult(cmd, err)
	}
	// --witnesses (TOL-V0-104..106) judges the plan against the ledger's
	// witnesses of every test type: a stale or unresolved reviewed witness is
	// a STALE_REVIEW finding, and each UNASSIGNED row names the witness it
	// considered. Without it the output keeps its exact earlier bytes.
	var wr planWitnessReport
	if withWitnesses {
		if wr, err = planWitnesses(env, rc, ledger, findings, candidate); err != nil {
			return errorResult(cmd, err)
		}
		if len(wr.stale) > 0 {
			findings = append(findings, wr.stale...)
			sort.SliceStable(findings, func(i, j int) bool {
				if findings[i].ID != findings[j].ID {
					return findings[i].ID < findings[j].ID
				}
				return findings[i].Kind < findings[j].Kind
			})
			ok = false
		}
	}
	items := make([]wire.Value, 0, len(findings))
	for _, f := range findings {
		item := wire.NewObject().Set("kind", wire.String(f.Kind)).Set("id", wire.String(f.ID)).
			Set("testCount", wire.String(string(wire.CountOf(int64(len(f.Tests)))))).Set("tests", wire.Strings(f.Tests))
		if c, has := wr.considered[f.ID]; has && f.Kind == ticket.PlanUnassigned {
			item.Set("considered", c)
		}
		items = append(items, wire.ObjectValue(item))
	}
	res := success(cmd, rc)
	po := wire.NewObject().Set("ok", wire.Bool(ok)).Set("findings", wire.Array(items...))
	if withWitnesses {
		po.Set("ledgerWitnesses", wire.Array(wr.witnesses...))
	}
	// A plan with a rerun member also reports the WITNESSED regression rows
	// it re-runs (TOL-V0-056); an older plan keeps its exact output.
	if ticket.PlanHasRerun(plan) {
		rerunItems := make([]wire.Value, 0, len(reruns))
		for _, f := range reruns {
			rerunItems = append(rerunItems, wire.ObjectValue(wire.NewObject().Set("id", wire.String(f.ID)).Set("tests", wire.Strings(f.Tests))))
		}
		po.Set("reruns", wire.Array(rerunItems...))
	}
	if hasReview {
		po.Set("sourceReview", review)
	}
	res.Items = []wire.Value{wire.ObjectValue(po)}
	res.Untrusted = true
	if !ok {
		res.Outcome = wire.OutcomeRefused
		res.Warnings = append(res.Warnings, "the plan leaves core open obligations unassigned, splits an obligation, names unknown or closed ids, or reruns an unwitnessed id")
	}
	if len(wr.stale) > 0 {
		res.Warnings = append(res.Warnings, "a reviewed ledger witness is stale or unresolved at HEAD (STALE_REVIEW)")
	}
	return res
}

// witnessProvenance builds the optional witness provenance (TOL-V0-059)
// from the flags given, or nil when none is given so the payload keeps its
// exact pre-TOL-V0-059 bytes. A given flag is a member even when its value
// is empty, so an empty value is refused MALFORMED (an OID by the verb, prose
// by the writer's payload reader, which bounds each member) and never drops
// silently out of the evidence.
func witnessProvenance(given map[string]bool, testCommit, envTree, evidenceRef, testTitle string) *ticket.ObligationProvenance {
	opt := func(flag, s string) *string {
		if !given[flag] {
			return nil
		}
		return &s
	}
	p := &ticket.ObligationProvenance{TestCommit: opt("--test-commit", testCommit), EnvironmentTree: opt("--environment-tree", envTree),
		EvidenceRef: opt("--evidence-ref", evidenceRef), TestTitle: opt("--test-title", testTitle)}
	if p.TestCommit == nil && p.EnvironmentTree == nil && p.EvidenceRef == nil && p.TestTitle == nil {
		return nil
	}
	return p
}

// givenFlags names the flags of an argument list that pairFlags accepted
// with only single-valued flags, so a verb can tell an explicitly empty value
// from an absent flag.
func givenFlags(args []string) map[string]bool {
	given := map[string]bool{}
	for i := 0; i < len(args); i += 2 {
		given[args[i]] = true
	}
	return given
}

// readPlanFile reads at most max bytes of file, refusing a larger one. A
// relative file resolves against cwd (commandPath).
func readPlanFile(cwd, file string, max int) ([]byte, error) {
	f, err := os.Open(commandPath(cwd, file))
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
// payload that differs (TOL-V0-013). The report itself is retained only
// with --retain, in the witness receipt store under its digest (TOL-V0-135).
func obligationsWitness(env Env, cmd []string, args []string) *wire.Result {
	return obligationsWitnessJob(context.Background(), "", env, cmd, args)
}

// witnessEvidenceReadHook runs after a report witness has read the commit's
// source evidence; tests use it to interrupt a batch job at that point.
var witnessEvidenceReadHook = func() {}

// obligationsWitnessJob is the witness verb under a run-batch job context:
// a job interrupted before the mutation is submitted credits nothing and
// refuses GATE_FAILED with LANE_FAILED; a submitted mutation is atomic
// (TOL-V0-033). The plain verb passes a context that never ends and no
// report digest. A job passes the digest of the report its test command
// wrote: the witness then reads the report and post-check log only as
// regular files and refuses a report that is no longer that one, since a
// capture command runs between the test run and the witness.
func obligationsWitnessJob(job context.Context, wantReport wire.Digest, env Env, cmd []string, args []string) *wire.Result {
	open, readReport := os.Open, obligation.ReadReportRaw
	if wantReport != "" {
		open = func(p string) (*os.File, error) {
			f, _, err := openRegular(p)
			return f, err
		}
		readReport = readJobReportRaw
	}
	f := mutateFlags{role: "OWNER"}
	var report, commit, idsArg, declared, manifest, testID, reason, attempt, generation, postCheck, postStatus string
	var testCommit, envTree, evidenceRef, testTitle, testRepo string
	var reviewed, reviewFile, excludeAuthor string
	var retain bool
	if res := pairFlags(cmd, args, map[string]*string{
		"--reviewed": &reviewed, "--review": &reviewFile, "--exclude-author": &excludeAuthor,
		"--test-repository": &testRepo,
		"--test-commit":     &testCommit, "--environment-tree": &envTree, "--evidence-ref": &evidenceRef, "--test-title": &testTitle,
		"--target": &f.target, "--role": &f.role, "--request-id": &f.requestID, "--expected-revision": &f.expected,
		"--issued-at": &f.issuedAt, "--from-playwright-report": &report, "--commit": &commit, "--ids": &idsArg,
		"--declared": &declared, "--manifest-sha256": &manifest, "--test-id": &testID, "--reason": &reason,
		"--attempt": &attempt, "--generation": &generation, "--post-check": &postCheck, "--post-check-status": &postStatus,
	}, nil, map[string]*bool{"--retain": &retain}); res != nil {
		return res
	}
	given := givenFlags(args)
	if f.requestID == "" {
		return usage(cmd, "--request-id is required: it is the idempotency key of this mutation")
	}
	if f.target == "" || f.expected == "" || commit == "" {
		return usage(cmd, "--target, --expected-revision and --commit are required")
	}
	if n := btoi(report != "") + btoi(declared != "") + btoi(reviewed != ""); n != 1 {
		return usage(cmd, "exactly one of --from-playwright-report FILE, --declared ID,... and --reviewed ID is required")
	}
	if (reviewed != "") != (reviewFile != "") || (excludeAuthor != "" && reviewed == "") {
		return usage(cmd, "--reviewed ID needs --review FILE; --review and --exclude-author belong to --reviewed")
	}
	if reviewed != "" && (manifest != "" || testID != "" || reason != "" || idsArg != "" || attempt != "" || postCheck != "") {
		return usage(cmd, "--reviewed takes no --ids, declaration, attempt or post-check flags")
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
	if (postCheck == "") != (postStatus == "") {
		return usage(cmd, "--post-check LOG and --post-check-status N are given together or not at all")
	}
	if postCheck != "" && report == "" {
		return usage(cmd, "--post-check belongs to --from-playwright-report")
	}
	if retain && report == "" {
		return usage(cmd, "--retain belongs to --from-playwright-report")
	}
	if postCheck != "" {
		if err := checkPostCheck(postCheck, postStatus, open); err != nil {
			return errorResult(cmd, err)
		}
	}
	if _, err := wire.ParseOID("--commit", commit); err != nil {
		return errorResult(cmd, err)
	}
	for flag, oid := range map[string]string{"--test-commit": testCommit, "--environment-tree": envTree} {
		if !given[flag] {
			continue
		}
		if _, err := wire.ParseOID(flag, oid); err != nil {
			return errorResult(cmd, err)
		}
	}
	actor, err := initActor(f.role)
	if err != nil {
		return errorResult(cmd, err)
	}
	var ids []string
	if idsArg != "" || declared != "" || reviewed != "" {
		if ids, err = obligationIDList(idsArg + declared + reviewed); err != nil {
			return errorResult(cmd, err)
		}
	}
	var rec *ticket.Record
	var ledger *ticket.ObligationLedger
	var recorded bool
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
		recorded = found
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
	testRepoPath := ""
	w := ticket.ObligationWitnessPayload{Commit: commit, Provenance: witnessProvenance(given, testCommit, envTree, evidenceRef, testTitle)}
	if given["--test-repository"] {
		// TOL-V0-065: commit resolution and source presence run in the
		// policy-declared test repository.
		if root, testRepoPath, err = declaredTestRepository(rc, testRepo); err != nil {
			return errorResult(cmd, err)
		}
		w.TestRepository = &testRepo
	}
	if attempt != "" {
		g := wire.Size(generation)
		w.Attempt, w.Generation = &attempt, &g
	}
	lists := witnessLists{}
	var check *mutation.ObligationReportCheck
	var reportRaw []byte
	if reviewed != "" {
		review, resolved, err := reviewedWitness(env, root, commit, rec.TicketID.Raw, reviewFile, excludeAuthor)
		if err != nil {
			return errorResult(cmd, err)
		}
		if e := ledger.Entry(reviewed); e == nil {
			return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "--reviewed", "%s obligation %s is not in the ledger", ticket.ObligationUnknownDetail, reviewed))
		}
		w.Source, w.Commit, w.Review = ticket.ObligationSourceReviewed, resolved, review
		w.Credits = []ticket.ObligationCredit{{ID: reviewed}}
		if outcome, _ := review.Outcome(); outcome == ticket.ObligationReviewCredited {
			lists.credited = []string{reviewed}
		}
	} else if declared != "" {
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
		qualified, err := admittedVersions(rc.repo.CommonDir)
		if err != nil {
			return errorResult(cmd, err)
		}
		rep, raw, err := readReport(report, qualified)
		if err != nil {
			return errorResult(cmd, err)
		}
		reportRaw = raw
		if wantReport != "" && rep.Sha256 != wantReport {
			return errorResult(cmd, wire.Errorf(wire.CodeGateFailed, "--from-playwright-report", "%s the report changed after the test run; nothing was credited", laneFailedDetail))
		}
		if resolved, err := filepath.EvalSymlinks(rep.RootDir); err == nil {
			rep.RootDir = resolved
		}
		paths := rep.SourcePaths(obligation.NamesOf(ledger), root)
		resolved, present, content, err := store.FilesAtCommit(root, commit, paths)
		witnessEvidenceReadHook()
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
		check = &mutation.ObligationReportCheck{ReportSha256: rep.Sha256, PlaywrightVersion: rep.Version, Commit: resolved, IDs: ids, Eligible: res.Eligible, TestRepository: testRepo, TestRepositoryPath: testRepoPath}
		version, sum := rep.Version, rep.Sha256
		w.Source, w.Commit, w.ReportSha256, w.PlaywrightVersion = ticket.ObligationSourceReport, resolved, &sum, &version
		w.Credits = check.ExpectedCredits(ledger)
		lists = witnessLists{credited: res.Credited, alreadyWitnessed: res.AlreadyWitnessed, conflicting: res.Conflicting,
			failed: res.Failed, unknown: res.Unknown, unknownTruncated: res.UnknownTruncated, unbound: res.Unbound, unmatched: res.Unmatched,
			defectConfirmed: res.DefectConfirmed, mixed: res.MixedExpectedFail}
	}
	if job.Err() != nil {
		// TOL-V0-033: an interrupted witness phase is LANE_FAILED even when
		// it would have credited nothing.
		return errorResult(cmd, wire.Errorf(wire.CodeGateFailed, "--request-id", "%s interrupted before the witness was submitted; nothing was credited", laneFailedDetail))
	}
	if len(w.Credits) == 0 && recorded {
		// A recorded witness always carries credits, so a request that
		// derives none from the ledger before it cannot be that request.
		return errorResult(cmd, wire.Errorf(wire.CodeRequestIDConflict, "--request-id", "request %s is retained on ticket %s with different inputs", f.requestID, rec.TicketID.Raw))
	}
	if len(w.Credits) == 0 {
		out := success(cmd, rc)
		o := wire.NewObject().Set("written", wire.Bool(false)).Set("ticketId", wire.String(rec.TicketID.Raw))
		lists.set(o)
		out.Items = []wire.Value{wire.ObjectValue(o)}
		out.Untrusted = true
		return out
	}
	var retained *evidencestore.PutResult
	if retain {
		// TOL-V0-135: the exact bytes whose digest the witness records are
		// stored before the ledger write; a report that cannot be kept
		// credits nothing, so --retain never yields an unresolvable witness.
		if wire.Sum(reportRaw) != *w.ReportSha256 {
			return errorResult(cmd, wire.Errorf(wire.CodeMalformed, "--retain", "the retained bytes differ from the witnessed report digest; nothing was credited"))
		}
		if retained, err = evidencestore.Put(rc.repo.StateDir, reportRaw); err != nil {
			return errorResult(cmd, err)
		}
	}
	reportRaw = nil
	if check == nil && w.TestRepository != nil {
		// A DECLARED witness binds only the test repository mapping it used.
		check = &mutation.ObligationReportCheck{TestRepository: testRepo, TestRepositoryPath: testRepoPath}
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
		if retained != nil {
			out.Items[0].Obj.Set("retained", retainedValue(retained))
		}
		if w.Review != nil {
			outcome, axes := w.Review.Outcome()
			av := make([]wire.Value, 0, len(axes))
			for _, x := range axes {
				av = append(av, wire.String(x))
			}
			out.Items[0].Obj.Set("reviewOutcome", wire.String(outcome)).Set("reviewAxes", wire.Array(av...))
		}
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
	defectConfirmed                                                     []obligation.DefectConfirmed
	mixed                                                               []obligation.MixedExpectedFail
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
	// TOL-V0-022/023: expected failures are reported, never credited, and
	// never set DEFECT (TOL-V0-011).
	optional := func(s string) wire.Value {
		if s == "" {
			return wire.Null()
		}
		return wire.String(prose(s))
	}
	defects := make([]wire.Value, 0, len(l.defectConfirmed))
	for _, d := range l.defectConfirmed {
		defects = append(defects, wire.ObjectValue(wire.NewObject().Set("id", wire.String(d.ID)).Set("defect", optional(d.Defect)).Set("error", optional(d.Error))))
	}
	mixed := make([]wire.Value, 0, len(l.mixed))
	for _, m := range l.mixed {
		mixed = append(mixed, wire.ObjectValue(wire.NewObject().Set("id", wire.String(m.ID)).Set("tests", wire.Strings(m.Tests)).
			Set("remedy", wire.String(obligation.MixedRemedy))))
	}
	o.Set("defectConfirmed", wire.Array(defects...)).Set("mixedExpectedFail", wire.Array(mixed...))
}

// maxPostCheckLogBytes bounds the post-check log read by a report witness.
const maxPostCheckLogBytes = 1 << 20

// checkPostCheck refuses a report witness whose run's post-check exited
// non-zero, quoting the log's first actionable line (TOL-V0-024). The log
// must exist either way; only its first MiB is read. open is os.Open for the
// plain verb and a regular-file-only open under a job (TOL-V0-033).
func checkPostCheck(file, status string, open func(string) (*os.File, error)) error {
	n, err := strconv.ParseUint(status, 10, 8)
	if err != nil {
		return wire.Errorf(wire.CodeMalformed, "--post-check-status", "%s --post-check-status takes an exit status 0..255", ticket.ObligationPostCheckDetail)
	}
	f, err := open(file)
	if err != nil {
		return wire.Errorf(wire.CodeMissingEvidence, "--post-check", "%s the post-check log is unreadable", ticket.ObligationPostCheckDetail)
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, maxPostCheckLogBytes))
	if err != nil {
		return wire.Errorf(wire.CodeMissingEvidence, "--post-check", "%s the post-check log is unreadable", ticket.ObligationPostCheckDetail)
	}
	if n == 0 {
		return nil
	}
	line := obligation.FirstActionableLine(string(raw))
	if line == "" {
		line = "the log is empty"
	}
	return wire.Errorf(wire.CodeGateFailed, "--post-check", "%s the run's post-check exited %d, so nothing is credited from this report: %s", ticket.ObligationPostCheckDetail, n, line)
}
