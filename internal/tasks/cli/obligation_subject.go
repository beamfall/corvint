package cli

// Reviewed-subject resolution, the plan's ledger-witness listing and the
// environment-baseline capability read (TOL-V0-100..112; V1-1945, V1-1949,
// GitHub #1202). Each is a read: nothing here writes the store.
import (
	"crypto/sha256"
	"encoding/hex"
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/obligation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Reviewed-subject statuses and reasons (TOL-V0-101).
const (
	subjectResolved              = "RESOLVED"
	subjectUnavailable           = "UNAVAILABLE"
	reviewedCommitUnavailable    = "REVIEWED_COMMIT_UNAVAILABLE"
	subjectAbsentAtReview        = "SUBJECT_ABSENT_AT_REVIEW"
	subjectUnreadableAtReview    = "SUBJECT_UNREADABLE_AT_REVIEW"
	subjectMismatchAtReview      = "SUBJECT_MISMATCH_AT_REVIEW"
	environmentBaselineUndecl    = "ENVIRONMENT_BASELINE_UNDECLARED"
	environmentBaselineUnresolve = "ENVIRONMENT_BASELINE_UNRESOLVED"
)

// obligationPolicy is the store's policy, nil when no store was read.
func obligationPolicy(rc *readCtx) *intent.Policy {
	if rc == nil || rc.store == nil {
		return nil
	}
	return rc.store.Policy
}

// reviewedSubjectValues resolves each review's cited test file at the
// commit the review recorded, in the repository the review read it from
// (TOL-V0-101). Each value is {repository, path, commit, status, reason}:
// RESOLVED when the file there still has the reviewed bytes, else
// UNAVAILABLE with TEST_REPOSITORY_UNRESOLVED, REVIEWED_COMMIT_UNAVAILABLE,
// SUBJECT_ABSENT_AT_REVIEW, SUBJECT_UNREADABLE_AT_REVIEW or
// SUBJECT_MISMATCH_AT_REVIEW. It never falls back to HEAD.
func reviewedSubjectValues(env Env, rc *readCtx, reviews []*ticket.ObligationReviewRecord) map[*ticket.ObligationReviewRecord]wire.Value {
	type group struct {
		repo   *string
		commit string
		rvs    []*ticket.ObligationReviewRecord
	}
	groups := map[[2]string]*group{}
	var order [][2]string
	for _, rv := range reviews {
		k := [2]string{reviewRepository(rv), rv.Commit}
		g, ok := groups[k]
		if !ok {
			g = &group{repo: rv.TestRepository, commit: rv.Commit}
			groups[k] = g
			order = append(order, k)
		}
		g.rvs = append(g.rvs, rv)
	}
	out := map[*ticket.ObligationReviewRecord]wire.Value{}
	for _, k := range order {
		g := groups[k]
		paths := make([]string, 0, len(g.rvs))
		for _, rv := range g.rvs {
			paths = append(paths, rv.Review.Subject.Path)
		}
		reason := ""
		var present map[string]bool
		var content map[string][]byte
		root, err := obligationRepoRoot(env, rc, g.repo)
		if err != nil {
			reason = testRepositoryUnresolved
		} else {
			var commit string
			commit, present, content, err = store.FilesAtCommit(root, g.commit, paths)
			if err != nil || commit == "" {
				reason = reviewedCommitUnavailable
			}
		}
		for _, rv := range g.rvs {
			r := reason
			if r == "" {
				s := rv.Review.Subject
				body, ok := content[s.Path]
				sum := sha256.Sum256(body)
				switch {
				case !present[s.Path]:
					r = subjectAbsentAtReview
				case !ok:
					r = subjectUnreadableAtReview
				case wire.Digest(hex.EncodeToString(sum[:])) != s.BlobSha256:
					r = subjectMismatchAtReview
				}
			}
			o := wire.NewObject().Set("repository", wire.StringOrNull(rv.TestRepository)).Set("path", wire.String(rv.Review.Subject.Path)).
				Set("commit", wire.String(rv.Commit))
			if r == "" {
				o.Set("status", wire.String(subjectResolved)).Set("reason", wire.Null())
			} else {
				o.Set("status", wire.String(subjectUnavailable)).Set("reason", wire.String(r))
			}
			out[rv] = wire.ObjectValue(o)
		}
	}
	return out
}

// Plan witness test types and insufficiency reasons (TOL-V0-104..106).
const (
	planTestTypeBrowser      = "browser"
	planTestTypeDeclared     = "declared"
	planTestTypeUnclassified = "unclassified"

	planStaleReview            = "STALE_REVIEW"
	planNoLedgerWitness        = "NO_LEDGER_WITNESS"
	planReviewHeld             = "REVIEW_HELD"
	planCandidateAwaitingRvw   = "CANDIDATE_AWAITING_REVIEW"
	planReviewStaleAtHead      = "REVIEW_STALE_AT_HEAD"
	planReviewUnresolvedAtHead = "REVIEW_UNRESOLVED_AT_HEAD"
)

// planWitnessType names the test type a ledger witness came from: browser
// for a Playwright report, declared for an OWNER declaration, and for a
// reviewed witness the naming kind of its reviewed file (TOL-V0-103), or
// unclassified when no kind matches.
func planWitnessType(e ticket.ObligationEntry, patterns []intent.ObligationTestFilePattern) string {
	switch e.Evidence.Source {
	case ticket.ObligationSourceReport:
		return planTestTypeBrowser
	case ticket.ObligationSourceDeclared:
		return planTestTypeDeclared
	}
	if e.Review != nil {
		if k := obligation.BoundaryTestKindWith(e.Review.Review.Subject.Path, patterns); k != "" {
			return k
		}
	}
	return planTestTypeUnclassified
}

// planWitnessReport is the TOL-V0-104..106 judgement of a plan against the
// ledger's witnesses: the witnesses it counted, the STALE_REVIEW findings,
// and what each UNASSIGNED row's considered witness was.
type planWitnessReport struct {
	witnesses  []wire.Value
	stale      []ticket.ObligationPlanFinding
	considered map[string]wire.Value
}

// planWitnesses judges each reviewed witness where obligations show judges
// it: at the ticket's candidate branch when candidate resolves, else at HEAD,
// naming the basis in staleBasis (TOL-V0-132).
func planWitnesses(env Env, rc *readCtx, l *ticket.ObligationLedger, findings []ticket.ObligationPlanFinding, candidate string) (planWitnessReport, error) {
	rep := planWitnessReport{witnesses: []wire.Value{}, considered: map[string]wire.Value{}}
	if l == nil {
		return rep, nil
	}
	heads, err := reviewHeadStates(env, rc, l, candidate)
	if err != nil {
		return rep, err
	}
	patterns := obligationPolicy(rc).ObligationTestFilePatterns()
	for _, e := range l.Entries {
		if e.State != ticket.ObligationWitnessed || e.Evidence == nil {
			continue
		}
		ev := e.Evidence
		o := wire.NewObject().Set("id", wire.String(e.ID)).Set("source", wire.String(ev.Source)).
			Set("testType", wire.String(planWitnessType(e, patterns))).Set("commit", wire.String(ev.Commit)).
			Set("testRepository", wire.StringOrNull(ev.TestRepository)).Set("eventSha256", wire.String(string(ev.EventSha256)))
		status := "COVERED"
		if ev.Source == ticket.ObligationSourceReviewed {
			h, ok := heads[e.ID]
			switch {
			case !ok || !h.resolved:
				status = planReviewUnresolvedAtHead
			case h.stale:
				status = planReviewStaleAtHead
			}
			if status != "COVERED" {
				rep.stale = append(rep.stale, ticket.ObligationPlanFinding{Kind: planStaleReview, ID: e.ID, Tests: []string{}})
			}
			if ok {
				o.Set("staleBasis", staleBasisValue(h))
			}
		}
		rep.witnesses = append(rep.witnesses, wire.ObjectValue(o.Set("status", wire.String(status))))
	}
	for _, f := range findings {
		if f.Kind != ticket.PlanUnassigned {
			continue
		}
		e := l.Entry(f.ID)
		if e == nil {
			continue
		}
		c := wire.NewObject()
		switch {
		case e.Candidate != nil:
			c.Set("witness", wire.ObjectValue(wire.NewObject().Set("kind", wire.String("CANDIDATE")).Set("path", wire.String(e.Candidate.Path)))).
				Set("insufficient", wire.String(planCandidateAwaitingRvw))
		case e.Review != nil && e.Review.Outcome == ticket.ObligationReviewHeld:
			axes := make([]wire.Value, 0, len(e.Review.Axes))
			for _, a := range e.Review.Axes {
				axes = append(axes, wire.String(a))
			}
			c.Set("witness", wire.ObjectValue(wire.NewObject().Set("kind", wire.String("REVIEWED")).Set("path", wire.String(e.Review.Review.Subject.Path)).
				Set("commit", wire.String(e.Review.Commit)).Set("testRepository", wire.StringOrNull(e.Review.TestRepository)).
				Set("eventSha256", wire.String(string(e.Review.EventSha256))).Set("axes", wire.Array(axes...)))).
				Set("insufficient", wire.String(planReviewHeld))
		default:
			c.Set("witness", wire.Null()).Set("insufficient", wire.String(planNoLedgerWitness))
		}
		rep.considered[f.ID] = wire.ObjectValue(c)
	}
	return rep, nil
}

// Bounds of the TOL-V0-107..109 capability read.
const (
	maxCapabilityFiles   = 1024
	maxCapabilityMatches = 16
	maxCapabilityText    = 256
)

// obligationsCapability is the TOL-V0-107..109 read: whether a path, and
// optionally a text inside it, exists at the environment baseline the
// policy declares for the repository. It abstains UNKNOWN, naming why,
// when the baseline is undeclared or unresolved, and never reads HEAD.
func obligationsCapability(env Env, cmd []string, args []string) *wire.Result {
	var p string
	var repos, texts []string
	if res := pairFlags(cmd, args, map[string]*string{"--path": &p}, map[string]*[]string{"--repository": &repos, "--contains": &texts}, nil); res != nil {
		return res
	}
	if len(repos) > 1 || len(texts) > 1 {
		return usage(cmd, "--repository and --contains may each be given once")
	}
	given := map[string]bool{"--repository": len(repos) == 1, "--contains": len(texts) == 1}
	var repo, contains string
	if given["--repository"] {
		repo = repos[0]
	}
	if given["--contains"] {
		contains = texts[0]
	}
	if p == "" {
		return usage(cmd, "--path is required")
	}
	if strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\t\n\r") || path.Clean(p) != p || p == ".." || strings.HasPrefix(p, "../") {
		return usage(cmd, "--path must be a clean repository-relative path")
	}
	if given["--contains"] && (contains == "" || len(contains) > maxCapabilityText || strings.ContainsAny(contains, "\n\r")) {
		return usage(cmd, "--contains takes 1..256 bytes on one line")
	}
	key := "."
	var repoPtr *string
	if given["--repository"] {
		if _, err := wire.ParseLabel("--repository", repo); err != nil {
			return errorResult(cmd, err)
		}
		key, repoPtr = repo, &repo
	}
	rc, err := withInventoryStore(env, func(*readCtx) error { return nil })
	if err != nil {
		return failure(cmd, rc, err)
	}
	ref, declared := obligationPolicy(rc).ObligationEnvironmentBaseline(key)
	baseline := wire.NewObject().Set("repository", wire.StringOrNull(repoPtr)).Set("ref", wire.Null()).Set("commit", wire.Null())
	o := wire.NewObject().Set("baseline", wire.ObjectValue(baseline)).Set("path", wire.String(p))
	if given["--contains"] {
		o.Set("contains", wire.String(contains))
	}
	res := success(cmd, rc)
	res.Untrusted = true
	abstain := func(reason string) *wire.Result {
		res.Items = []wire.Value{wire.ObjectValue(o.Set("status", wire.String(obligation.BoundaryUnknown)).Set("reason", wire.String(reason)).
			Set("matches", wire.Array()).Set("truncated", wire.Bool(false)))}
		return res
	}
	if !declared {
		return abstain(environmentBaselineUndecl)
	}
	baseline.Set("ref", wire.String(ref))
	root, err := obligationRepoRoot(env, rc, repoPtr)
	if err != nil {
		return abstain(testRepositoryUnresolved)
	}
	commit, err := store.ResolveCommit(root, ref)
	if err != nil || commit == "" {
		return abstain(environmentBaselineUnresolve)
	}
	baseline.Set("commit", wire.String(commit))
	// The listing stops at the first file past the bound, so a large tree
	// costs no more than the bound (TOL-V0-941).
	files, over, err := store.TreePathsAtCommitUpTo(root, commit, []string{p}, maxCapabilityFiles, func(f string) bool {
		return p == "." || f == p || strings.HasPrefix(f, p+"/")
	})
	if err != nil {
		baseline.Set("commit", wire.Null())
		return abstain(environmentBaselineUnresolve)
	}
	sort.Strings(files)
	matched, unread := files, false
	if given["--contains"] && len(files) > 0 {
		var present map[string]bool
		var content map[string][]byte
		_, present, content, err = store.FilesAtCommit(root, commit, files)
		if err != nil {
			baseline.Set("commit", wire.Null())
			return abstain(environmentBaselineUnresolve)
		}
		matched = nil
		for _, f := range files {
			body, ok := content[f]
			if !ok {
				unread = unread || present[f]
				continue
			}
			if strings.Contains(string(body), contains) {
				matched = append(matched, f)
			}
		}
	}
	if len(matched) == 0 && (over || unread) {
		// A listing or a blob too large to read may hold the match: abstain.
		reason := "SEARCH_BOUND_EXCEEDED"
		return abstain(reason)
	}
	status, reason := "PRESENT", wire.Null()
	if len(matched) == 0 {
		status = "ABSENT"
		reason = wire.String("NOT_FOUND_AT_BASELINE")
	}
	// A bound that left files unlisted or unread may hide further matches.
	truncated := len(matched) > maxCapabilityMatches || over || unread
	if len(matched) > maxCapabilityMatches {
		matched = matched[:maxCapabilityMatches]
	}
	res.Items = []wire.Value{wire.ObjectValue(o.Set("status", wire.String(status)).Set("reason", reason).
		Set("matches", wire.Strings(matched)).Set("truncated", wire.Bool(truncated)))}
	return res
}
