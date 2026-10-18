package sharedpath

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// BehindProfile is the changes behind report profile (SPC-V0-011).
const BehindProfile = "corvint-shared-path-behind/0"

// CodeRangeTooLarge refuses a head..base range over the commit bound (SPC-V0-009).
const CodeRangeTooLarge = "changes-shared-range-too-large"

// StaleKind is the finding kind for a ticket branch that lacks old base
// shared-path commits (SPC-V0-012).
const StaleKind = "SHARED_BASE_BEHIND"

const maxStaleHours = 8760

// RangeTooLargeError refuses a range holding more commits than the bound; it
// is returned after counting and before any commit is listed (SPC-V0-009).
type RangeTooLargeError struct{ Limit, Commits int }

func (*RangeTooLargeError) Error() string { return CodeRangeTooLarge }

// BehindCommit is one base commit, missing from head, that changes a shared path.
type BehindCommit struct {
	Commit       string   `json:"commit"`
	Subject      string   `json:"subject"`
	CommittedAt  string   `json:"committedAt"`
	SharedPaths  []string `json:"sharedPaths"`
	HeadTouches  []string `json:"headTouches"`
	ConflictRisk bool     `json:"conflictRisk"`
	committed    time.Time
}

// DryMerge is the merge-tree probe of merging head into base (SPC-V0-010).
type DryMerge struct {
	Result             string   `json:"result"`
	Conflicts          []string `json:"conflicts"`
	GeneratedConflicts []string `json:"generatedConflicts"`
}

// BehindFinding names the ticket, the oldest missing shared commit and its age.
type BehindFinding struct {
	Kind       string `json:"kind"`
	Who        string `json:"who"`
	Commit     string `json:"commit"`
	AgeSeconds int64  `json:"ageSeconds"`
	Detail     string `json:"detail"`
	Remedy     string `json:"remedy"`
}

// BehindReport is the changes behind result (SPC-V0-011).
type BehindReport struct {
	Profile       string          `json:"profile"`
	Mutates       bool            `json:"mutates"`
	Base          string          `json:"base"`
	Head          string          `json:"head"`
	MergeBase     string          `json:"mergeBase"`
	ConfigDigest  string          `json:"configDigest"`
	ObservedAt    string          `json:"observedAt"`
	Counted       int             `json:"counted"`
	Behind        []BehindCommit  `json:"behind"`
	SkippedMerges []string        `json:"skippedMerges"`
	DryMerge      DryMerge        `json:"dryMerge"`
	Findings      []BehindFinding `json:"findings"`
}

// BehindRequest selects the checkout, policy, range and optional stale check.
type BehindRequest struct {
	Root, Base, Head, Ticket string
	StaleHours               int
	Now                      time.Time
	Config                   Config
}

var ticketID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// ValidTicket reports whether id is an acceptable --ticket value (SPC-V0-012).
func ValidTicket(id string) bool { return ticketID.MatchString(id) }

// ValidStaleHours reports whether n is an acceptable --stale-hours value.
func ValidStaleHours(n int) bool { return n >= 1 && n <= maxStaleHours }

// Behind lists the base commits not reachable from head that change a
// configured shared path. It reads only local Git state and writes nothing:
// the dry merge uses the SPC-V0-003 private object directory.
func Behind(ctx context.Context, request BehindRequest) (BehindReport, error) {
	if (request.Ticket == "") != (request.StaleHours == 0) || (request.Ticket != "" && (!ValidTicket(request.Ticket) || !ValidStaleHours(request.StaleHours))) {
		return BehindReport{}, errors.New(CodeInvalidArguments)
	}
	g := git{ctx: ctx, root: request.Root}
	if request.Head == "" {
		request.Head = "HEAD"
	}
	if request.Now.IsZero() {
		request.Now = time.Now()
	}
	base, err := g.commit(request.Base)
	if err != nil {
		return BehindReport{}, err
	}
	head, err := g.commit(request.Head)
	if err != nil {
		return BehindReport{}, err
	}
	counted, err := countBehind(g, base, head)
	if err != nil {
		return BehindReport{}, err
	}
	out, code, err := g.run(nil, nil, "merge-base", base, head)
	if err != nil {
		return BehindReport{}, err
	}
	mergeBase := strings.TrimSpace(string(out))
	if code == 1 {
		return BehindReport{}, errors.New(CodeRangeUnavailable)
	}
	if code != 0 || !fullSHA.MatchString(mergeBase) {
		return BehindReport{}, errors.New(CodeGitUnavailable)
	}
	headChanged, err := g.ok("diff-tree", "-r", "--no-renames", "--name-only", "-z", mergeBase, head)
	if err != nil {
		return BehindReport{}, err
	}
	touched := map[string]bool{}
	for _, p := range strings.Split(string(headChanged), "\x00") {
		if p != "" {
			touched[p] = true
		}
	}
	r := BehindReport{Profile: BehindProfile, Base: base, Head: head, MergeBase: mergeBase, ConfigDigest: request.Config.Digest(),
		ObservedAt: request.Now.UTC().Format(time.RFC3339), Counted: counted, Behind: []BehindCommit{}, SkippedMerges: []string{}, Findings: []BehindFinding{}}
	commits, err := g.commits(head, base)
	if err != nil {
		return BehindReport{}, err
	}
	for _, rc := range commits {
		if len(rc.parents) > 1 {
			r.SkippedMerges = append(r.SkippedMerges, rc.sha)
			continue
		}
		changes, err := g.changes(rc.sha)
		if err != nil {
			return BehindReport{}, err
		}
		bc := BehindCommit{Commit: rc.sha, SharedPaths: []string{}, HeadTouches: []string{}}
		for _, ch := range changes {
			if request.Config.Class(ch.path) != "shared" {
				continue
			}
			bc.SharedPaths = append(bc.SharedPaths, ch.path)
			if touched[ch.path] {
				bc.HeadTouches = append(bc.HeadTouches, ch.path)
			}
		}
		if len(bc.SharedPaths) == 0 {
			continue
		}
		m, err := g.message(rc.sha)
		if err != nil {
			return BehindReport{}, err
		}
		if bc.committed, err = m.committedAt(); err != nil {
			return BehindReport{}, err
		}
		bc.Subject, bc.CommittedAt, bc.ConflictRisk = m.subject(), bc.committed.UTC().Format(time.RFC3339), len(bc.HeadTouches) > 0
		r.Behind = append(r.Behind, bc)
	}
	merge, err := mergeConflicts(g, base, head)
	if err != nil {
		return BehindReport{}, err
	}
	r.DryMerge = dryMergeOf(request.Config, merge)
	if f, ok := staleFinding(request, r); ok {
		r.Findings = append(r.Findings, f)
	}
	return r, nil
}

// countBehind counts head..base and refuses past the bound before any commit
// is listed (SPC-V0-009).
func countBehind(g git, base, head string) (int, error) {
	out, err := g.ok("rev-list", "--count", "--end-of-options", base, "^"+head)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || n < 0 {
		return 0, errors.New(CodeGitUnavailable)
	}
	if n > maxCommits {
		return n, &RangeTooLargeError{Limit: maxCommits, Commits: n}
	}
	return n, nil
}

// dryMergeOf classifies the probe as clean, generated-only or conflict; a
// conflict Git reports without naming a path is a conflict (SPC-V0-010).
func dryMergeOf(c Config, merge mergeOutcome) DryMerge {
	d := DryMerge{Result: "clean", Conflicts: []string{}, GeneratedConflicts: []string{}}
	for _, p := range merge.paths {
		if c.Class(p) == "generated" {
			d.GeneratedConflicts = append(d.GeneratedConflicts, p)
		} else {
			d.Conflicts = append(d.Conflicts, p)
		}
	}
	sort.Strings(d.Conflicts)
	sort.Strings(d.GeneratedConflicts)
	switch {
	case len(d.Conflicts) > 0, merge.conflicted && len(d.GeneratedConflicts) == 0:
		d.Result = "conflict"
	case len(d.GeneratedConflicts) > 0:
		d.Result = "generated-only"
	}
	return d
}

// staleFinding names the ticket, the oldest missing shared commit and its age
// when that age reaches the threshold (SPC-V0-012).
func staleFinding(request BehindRequest, r BehindReport) (BehindFinding, bool) {
	if request.Ticket == "" || len(r.Behind) == 0 {
		return BehindFinding{}, false
	}
	oldest := r.Behind[0]
	for _, bc := range r.Behind[1:] {
		if bc.committed.Before(oldest.committed) {
			oldest = bc
		}
	}
	age := request.Now.Sub(oldest.committed)
	if age < time.Duration(request.StaleHours)*time.Hour {
		return BehindFinding{}, false
	}
	hours := int64(age / time.Hour)
	return BehindFinding{
		Kind: StaleKind, Who: request.Ticket, Commit: oldest.Commit, AgeSeconds: int64(age / time.Second),
		Detail: fmt.Sprintf("ticket %s lacks %d shared-path commit(s) from base %s; the oldest, %s, is %d h old (threshold %d h)",
			request.Ticket, len(r.Behind), r.Base, oldest.Commit, hours, request.StaleHours),
		Remedy: "Merge the base into the ticket branch and rerun the affected tests before submitting.",
	}, true
}

// committedAt parses the committer timestamp of a commit header.
func (m message) committedAt() (time.Time, error) {
	fields := strings.Fields(m.committer)
	if len(fields) < 2 {
		return time.Time{}, errors.New(CodeGitUnavailable)
	}
	seconds, err := strconv.ParseInt(fields[len(fields)-2], 10, 64)
	if err != nil {
		return time.Time{}, errors.New(CodeGitUnavailable)
	}
	return time.Unix(seconds, 0).UTC(), nil
}
