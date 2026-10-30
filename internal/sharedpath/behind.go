package sharedpath

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// BehindCommit is one base commit, missing from head, that changes a shared
// path. ArrivedVia is the first-parent commit of base that introduced it, and
// ArrivedAt that commit's committer time: when the change reached base
// (SPC-V0-013).
type BehindCommit struct {
	Commit       string   `json:"commit"`
	Subject      string   `json:"subject"`
	CommittedAt  string   `json:"committedAt"`
	ArrivedVia   string   `json:"arrivedVia"`
	ArrivedAt    string   `json:"arrivedAt"`
	SharedPaths  []string `json:"sharedPaths"`
	HeadTouches  []string `json:"headTouches"`
	ConflictRisk bool     `json:"conflictRisk"`
	arrived      time.Time
}

// BaseObservation names the base as given, the full ref it resolves to ("" when
// it is not a ref) and when this checkout last observed that ref: the later of
// the ref's newest reflog entry and, for a remote-tracking ref, the FETCH_HEAD
// modification time. LastObservedAt is null and ObservedVia "none" when neither
// exists (SPC-V0-014).
type BaseObservation struct {
	Name           string  `json:"name"`
	Ref            string  `json:"ref"`
	LastObservedAt *string `json:"lastObservedAt"`
	ObservedVia    string  `json:"observedVia"`
	observed       time.Time
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
	BaseRef       BaseObservation `json:"baseRef"`
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
	if err := refuseShallowMergeBase(g, base, head); err != nil {
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
	baseRef, err := observeBase(g, request.Base)
	if err != nil {
		return BehindReport{}, err
	}
	r := BehindReport{Profile: BehindProfile, Base: base, BaseRef: baseRef, Head: head, MergeBase: mergeBase, ConfigDigest: request.Config.Digest(),
		ObservedAt: request.Now.UTC().Format(time.RFC3339), Counted: counted, Behind: []BehindCommit{}, SkippedMerges: []string{}, Findings: []BehindFinding{}}
	commits, err := g.commits(head, base)
	if err != nil {
		return BehindReport{}, err
	}
	arrivals := arrivalCommits(commits, base)
	arrivedAt := map[string]time.Time{}
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
		committed, err := m.committedAt()
		if err != nil {
			return BehindReport{}, err
		}
		bc.ArrivedVia = arrivals[rc.sha]
		if bc.ArrivedVia == "" {
			return BehindReport{}, errors.New(CodeGitUnavailable)
		}
		if bc.arrived, err = arrivalTime(g, bc.ArrivedVia, arrivedAt); err != nil {
			return BehindReport{}, err
		}
		bc.Subject, bc.CommittedAt, bc.ConflictRisk = m.subject(), committed.UTC().Format(time.RFC3339), len(bc.HeadTouches) > 0
		bc.ArrivedAt = bc.arrived.UTC().Format(time.RFC3339)
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

// arrivalCommits maps every commit of head..base to the first-parent commit of
// base that introduced it: the oldest commit on base's first-parent chain
// (inside the range) that reaches it. A commit made directly on base arrives
// as itself; a commit merged from a side branch arrives with its merge
// (SPC-V0-013). Every range commit is an ancestor of base, so each is mapped.
func arrivalCommits(commits []rangeCommit, base string) map[string]string {
	parents := make(map[string][]string, len(commits))
	for _, rc := range commits {
		parents[rc.sha] = rc.parents
	}
	var chain []string
	for c := base; ; {
		ps, ok := parents[c]
		if !ok {
			break
		}
		chain = append(chain, c)
		if len(ps) == 0 {
			break
		}
		c = ps[0]
	}
	arrivals := make(map[string]string, len(commits))
	for i := len(chain) - 1; i >= 0; i-- {
		via := chain[i]
		stack := []string{via}
		for len(stack) > 0 {
			c := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if _, inRange := parents[c]; !inRange || arrivals[c] != "" {
				continue
			}
			// An assigned commit's ancestors were reached by the same
			// earlier chain commit, so the walk stops there.
			arrivals[c] = via
			stack = append(stack, parents[c]...)
		}
	}
	return arrivals
}

// arrivalTime is the committer time of an arrival commit, read once per commit.
func arrivalTime(g git, via string, cache map[string]time.Time) (time.Time, error) {
	if t, ok := cache[via]; ok {
		return t, nil
	}
	m, err := g.message(via)
	if err != nil {
		return time.Time{}, err
	}
	t, err := m.committedAt()
	if err != nil {
		return time.Time{}, err
	}
	cache[via] = t
	return t, nil
}

// observeBase resolves the base argument to a full ref and reads, without
// writing, when the checkout last observed it: the ref's newest reflog entry
// and, for refs/remotes/, the FETCH_HEAD modification time, whichever is later
// (SPC-V0-014). Behind never fetches, so a stale base is visible here rather
// than corrected.
func observeBase(g git, name string) (BaseObservation, error) {
	o := BaseObservation{Name: name, ObservedVia: "none"}
	out, code, err := g.run(nil, nil, "rev-parse", "--symbolic-full-name", name)
	if err != nil {
		return BaseObservation{}, err
	}
	if ref := strings.TrimSpace(string(out)); code == 0 && strings.HasPrefix(ref, "refs/") && !strings.ContainsAny(ref, " \t\n\r") {
		o.Ref = ref
	}
	if o.Ref == "" {
		return o, nil
	}
	if out, code, err := g.run(nil, nil, "log", "--no-show-signature", "-g", "-n", "1", "--date=unix", "--format=%gd", "--end-of-options", o.Ref, "--"); err != nil {
		return BaseObservation{}, err
	} else if selector := strings.TrimSpace(string(out)); code == 0 && strings.HasSuffix(selector, "}") {
		if i := strings.LastIndex(selector, "@{"); i >= 0 {
			if seconds, err := strconv.ParseInt(selector[i+2:len(selector)-1], 10, 64); err == nil {
				o.observed, o.ObservedVia = time.Unix(seconds, 0).UTC(), "reflog"
			}
		}
	}
	if strings.HasPrefix(o.Ref, "refs/remotes/") {
		out, err := g.ok("rev-parse", "--git-path", "FETCH_HEAD")
		if err != nil {
			return BaseObservation{}, err
		}
		file := strings.TrimSpace(string(out))
		if !filepath.IsAbs(file) {
			file = filepath.Join(g.root, file)
		}
		if info, err := os.Lstat(file); err == nil && info.Mode().IsRegular() && info.ModTime().After(o.observed) {
			o.observed, o.ObservedVia = info.ModTime().UTC().Truncate(time.Second), "FETCH_HEAD"
		}
	}
	if o.ObservedVia != "none" {
		at := o.observed.Format(time.RFC3339)
		o.LastObservedAt = &at
	}
	return o, nil
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

// staleFinding names the ticket, the missing shared commit that reached base
// first and how long base has carried it, when that age reaches the threshold
// (SPC-V0-012, SPC-V0-013). The detail also names when the base ref was last
// observed (SPC-V0-014).
func staleFinding(request BehindRequest, r BehindReport) (BehindFinding, bool) {
	if request.Ticket == "" || len(r.Behind) == 0 {
		return BehindFinding{}, false
	}
	oldest := r.Behind[0]
	for _, bc := range r.Behind[1:] {
		if bc.arrived.Before(oldest.arrived) {
			oldest = bc
		}
	}
	age := request.Now.Sub(oldest.arrived)
	if age < time.Duration(request.StaleHours)*time.Hour {
		return BehindFinding{}, false
	}
	hours := int64(age / time.Hour)
	return BehindFinding{
		Kind: StaleKind, Who: request.Ticket, Commit: oldest.Commit, AgeSeconds: int64(age / time.Second),
		Detail: fmt.Sprintf("ticket %s lacks %d shared-path commit(s) from base %s; the oldest, %s, reached base %d h ago (threshold %d h); %s",
			request.Ticket, len(r.Behind), r.Base, oldest.Commit, hours, request.StaleHours, baseObservationDetail(r.BaseRef)),
		Remedy: "Merge the base into the ticket branch and rerun the affected tests before submitting.",
	}, true
}

// baseObservationDetail states when the base ref was last observed, so a stale
// remote-tracking base is visible in the doctor-plugin finding too.
func baseObservationDetail(o BaseObservation) string {
	if o.Ref == "" {
		return fmt.Sprintf("base %s is not a ref, so its last observation is unknown", strconv.Quote(o.Name))
	}
	if o.LastObservedAt == nil {
		return fmt.Sprintf("base %s has no reflog or FETCH_HEAD time, so its last observation is unknown", o.Ref)
	}
	return fmt.Sprintf("base %s was last observed at %s (%s); fetch to see newer base commits", o.Ref, *o.LastObservedAt, o.ObservedVia)
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
