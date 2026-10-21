package cli

// Reviewer-verdict witnesses (TOL-V0-073..078, GitHub #1170): the
// `witness --reviewed` input, its author derivation through the CAL-V0-107
// exclude-authors rule, and the read-only `witnessReview` staleness view.
import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// maxReviewInputBytes bounds the --review file.
const maxReviewInputBytes = 65536

// readReviewInput reads the --review file: {policy, subject {path,
// startLine, endLine}, verdicts}. The command adds the subject blob digest
// and the authors; verdicts are sorted by reviewer at this input boundary.
func readReviewInput(file string) (*ticket.ObligationReview, error) {
	raw, err := readBoundedFile("--review", file, maxReviewInputBytes)
	if err != nil {
		return nil, err
	}
	v, err := wire.ParseInput(raw)
	if err != nil {
		return nil, err
	}
	if v.Kind != wire.KindObject {
		return nil, wire.Errorf(wire.CodeMalformed, "--review", "the review file is a JSON object")
	}
	r := wire.NewReader(v, "--review")
	r.Closed("policy", "subject", "verdicts")
	sub := r.Field("subject")
	sub.Closed("path", "startLine", "endLine")
	out := &ticket.ObligationReview{Subject: ticket.ObligationReviewSubject{Path: sub.Field("path").Path(),
		StartLine: sub.Field("startLine").Count().Int(), EndLine: sub.Field("endLine").Count().Int()}}
	pol := r.Field("policy")
	pol.Closed("reviewers", "maxStubRisk")
	out.Reviewers = pol.Field("reviewers").Count().Int()
	out.MaxStubRisk = pol.Field("maxStubRisk").String()
	for _, it := range r.Field("verdicts").Array(ticket.MaxObligationReviewers, true) {
		it.Closed(wire.OptionalKeys(it.Value(), []string{"reviewer", "verdict", "stubRisk", "reason"}, "finding")...)
		v := ticket.ObligationVerdict{Reviewer: it.Field("reviewer").String(), Verdict: it.Field("verdict").String(),
			StubRisk: it.Field("stubRisk").String(), Reason: it.Field("reason").String()}
		if wire.Has(it.Value(), "finding") {
			v.Finding = it.Field("finding").Enum(ticket.VerdictFindingNoFalsifiableText)
		}
		out.Verdicts = append(out.Verdicts, v)
	}
	if err := r.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out.Verdicts, func(i, j int) bool { return out.Verdicts[i].Reviewer < out.Verdicts[j].Reviewer })
	return out, nil
}

// reviewAuthors is the sorted unique set of member labels that may not
// review ticketID: every pool member the CAL-V0-107 derivation records for
// its implement generations (mode ALL), plus the caller's explicit
// exclusions. An unverified derivation with no explicit exclusion refuses
// INDEPENDENCE_UNVERIFIED (TOL-V0-074).
func reviewAuthors(env Env, ticketID string, explicit []string) ([]string, error) {
	attempts := map[string]*snapshot.Attempt{}
	if _, err := withStore(env, func(rc *readCtx) error {
		if rc.snap.Head.Generation.Uint64() == 0 {
			return nil
		}
		proof, err := auditState(rc, "reservations.json")
		if err != nil {
			return err
		}
		for _, p := range attemptPaths(proof) {
			a, err := snapshot.DecodeAttempt(proof.Records[p].Raw)
			if err != nil {
				return err
			}
			attempts[a.AttemptID] = a
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return reviewAuthorSet(attempts, ticketID, explicit)
}

// reviewAuthorSet is the pure part of reviewAuthors: the sorted union of the
// explicit exclusions and every member label DeriveAuthors (mode ALL, any
// pool) records for ticketID's implement generations. A label is a member
// ID, so an explicit alias equal to a derived member, or one member that
// implemented in two pools, appears once.
func reviewAuthorSet(attempts map[string]*snapshot.Attempt, ticketID string, explicit []string) ([]string, error) {
	x, why := transaction.DeriveAuthors(attempts, ticketID, transaction.ExcludeAuthorsAll, "", explicit)
	if why != "" {
		return nil, wire.Errorf(wire.CodeIndependenceUnverified, "--exclude-author", "reviewer independence cannot be derived: %s; name the authors with --exclude-author", why)
	}
	set := map[string]bool{}
	for _, m := range explicit {
		set[m] = true
	}
	for _, a := range x.Authors {
		set[a.MemberID] = true
	}
	out := make([]string, 0, len(set))
	for m := range set {
		out = append(out, m)
	}
	sort.Strings(out)
	return out, nil
}

// reviewedWitness builds the REVIEWED payload member: the subject blob is
// read from Git at commit, and the payload is checked through the
// authoritative reader so an author reviewer refuses before any write.
func reviewedWitness(env Env, root, commit, ticketID, file, excludeArg string) (*ticket.ObligationReview, string, error) {
	review, err := readReviewInput(file)
	if err != nil {
		return nil, "", err
	}
	var explicit []string
	if excludeArg != "" {
		for _, m := range strings.Split(excludeArg, ",") {
			if _, err := wire.ParseLabel("--exclude-author", m); err != nil {
				return nil, "", err
			}
			explicit = append(explicit, m)
		}
	}
	if review.Authors, err = reviewAuthors(env, ticketID, explicit); err != nil {
		return nil, "", err
	}
	resolved, present, content, err := store.FilesAtCommit(root, commit, []string{review.Subject.Path})
	if err != nil {
		return nil, "", err
	}
	if resolved == "" {
		return nil, "", wire.Errorf(wire.CodeMissingEvidence, "--commit", "the repository holds no commit %s", commit)
	}
	body, ok := content[review.Subject.Path]
	if !present[review.Subject.Path] || !ok {
		return nil, "", wire.Errorf(wire.CodeMissingEvidence, "--review", "subject %s is not a readable file at commit %s", review.Subject.Path, resolved)
	}
	sum := sha256.Sum256(body)
	review.Subject.BlobSha256 = wire.Digest(hex.EncodeToString(sum[:]))
	checked := ticket.ReadObligationReview(wire.NewReader(review.Value(), "/payload/review"))
	if checked == nil {
		r := wire.NewReader(review.Value(), "/payload/review")
		ticket.ReadObligationReview(r)
		return nil, "", r.Err()
	}
	return checked, resolved, nil
}

// witnessReviewValue is the TOL-V0-077 read-only view: each reviewed entry's
// outcome and verdicts, and whether the cited example changed since review
// at HEAD of the repository the review read it from: the store worktree, or
// the policy-declared test repository the record names. A test repository
// that no longer resolves leaves stale unknown and the credit uncounted
// (TEST_REPOSITORY_UNRESOLVED). ok is false when no entry was reviewed, so
// show keeps its exact earlier output.
func witnessReviewValue(env Env, rc *readCtx, l *ticket.ObligationLedger) (wire.Value, bool, error) {
	if l == nil {
		return wire.Value{}, false, nil
	}
	heads, err := reviewHeadStates(env, rc, l)
	if err != nil || len(heads) == 0 {
		return wire.Value{}, false, err
	}
	witnessed := l.Counts().Witnessed
	var items []wire.Value
	for _, e := range l.Entries {
		if e.Review == nil {
			continue
		}
		rv := e.Review
		h := heads[e.ID]
		stale, resolved := h.stale, h.resolved
		credited := e.State == ticket.ObligationWitnessed && e.Evidence != nil && e.Evidence.Source == ticket.ObligationSourceReviewed
		if credited && (stale || !resolved) {
			witnessed--
			credited = false
		}
		axes := make([]wire.Value, 0, len(rv.Axes))
		for _, a := range rv.Axes {
			axes = append(axes, wire.String(a))
		}
		verdicts := make([]wire.Value, 0, len(rv.Review.Verdicts))
		for _, v := range rv.Review.Verdicts {
			verdicts = append(verdicts, wire.ObjectValue(wire.NewObject().Set("reviewer", wire.String(v.Reviewer)).Set("verdict", wire.String(v.Verdict)).
				Set("stubRisk", wire.String(v.StubRisk)).Set("reason", wire.String(v.Reason))))
		}
		item := wire.NewObject().Set("id", wire.String(e.ID)).Set("outcome", wire.String(rv.Outcome)).
			Set("axes", wire.Array(axes...)).Set("credited", wire.Bool(credited)).Set("stale", wire.Bool(stale)).
			Set("path", wire.String(rv.Review.Subject.Path)).Set("verdicts", wire.Array(verdicts...))
		if rv.TestRepository != nil {
			item.Set("testRepository", wire.String(*rv.TestRepository))
		}
		if !resolved {
			item.Set("stale", wire.Null()).Set("staleUnknown", wire.String(testRepositoryUnresolved))
		}
		items = append(items, wire.ObjectValue(item))
	}
	return wire.ObjectValue(wire.NewObject().Set("effectiveWitnessed", wire.String(string(wire.CountOf(witnessed)))).Set("entries", wire.Array(items...))), true, nil
}

// reviewHeadState is one reviewed entry's TOL-V0-077 read-time judgement:
// whether its review's repository resolved at HEAD, and whether the cited
// example there differs from the reviewed bytes.
type reviewHeadState struct {
	resolved bool
	stale    bool
}

// reviewHeadStates judges every reviewed entry of l at HEAD of the
// repository its review read: the store worktree, or the declared test
// repository. A test repository that is undeclared, unreadable or has an
// unborn HEAD leaves the entry unresolved (TOL-V0-077, TOL-V0-100); an
// unreadable store worktree is an error.
func reviewHeadStates(env Env, rc *readCtx, l *ticket.ObligationLedger) (map[string]reviewHeadState, error) {
	paths := map[string][]string{}
	for _, e := range l.Entries {
		if e.Review != nil {
			repo := reviewRepository(e.Review)
			paths[repo] = append(paths[repo], e.Review.Review.Subject.Path)
		}
	}
	if len(paths) == 0 {
		return nil, nil
	}
	content := map[string]map[string][]byte{}
	for repo, ps := range paths {
		var root string
		var err error
		if repo == "" {
			root, err = store.WorktreeRoot(env.Cwd)
		} else if root, _, err = declaredTestRepository(rc, repo); err != nil {
			continue
		}
		if err != nil {
			return nil, err
		}
		commit, _, files, err := store.FilesAtCommit(root, "HEAD", ps)
		if err != nil {
			if repo != "" {
				continue
			}
			return nil, err
		}
		if commit == "" && repo != "" {
			// TOL-V0-100: an unborn test-repository HEAD names no commit, so the
			// review cannot be checked there; it is unresolved, never stale.
			continue
		}
		content[repo] = files
	}
	out := map[string]reviewHeadState{}
	for _, e := range l.Entries {
		if e.Review == nil {
			continue
		}
		files, resolved := content[reviewRepository(e.Review)]
		stale := true
		if body, ok := files[e.Review.Review.Subject.Path]; ok {
			sum := sha256.Sum256(body)
			stale = wire.Digest(hex.EncodeToString(sum[:])) != e.Review.Review.Subject.BlobSha256
		}
		out[e.ID] = reviewHeadState{resolved: resolved, stale: stale}
	}
	return out, nil
}

// testRepositoryUnresolved names why a review's staleness is unknown: its
// test repository is no longer declared or no longer readable.
const testRepositoryUnresolved = "TEST_REPOSITORY_UNRESOLVED"

// reviewRepository is the review's test repository name, or "" for the
// store's own worktree.
func reviewRepository(rv *ticket.ObligationReviewRecord) string {
	if rv.TestRepository == nil {
		return ""
	}
	return *rv.TestRepository
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
