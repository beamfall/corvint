// Package observedstale is the shared staleness evaluator for observed (LOCALLY_OBSERVED or
// INGESTED) records: a record observed at commit C stays current at R only while none of its
// linked source paths changed in C..R (docs/specs/observed-ui-evidence-v0.md, OUE-V0-030..036).
// It never upgrades a record on missing data: an unresolved revision, a non-ancestor, an
// untrusted ancestry, an unavailable diff or a record with no linked repository path reads
// UNKNOWN, and the report counts every record in exactly one state.
package observedstale

import (
	"context"
	"errors"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/gitancestry"
	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/procgroup"
)

const (
	Schema = "observed-staleness/0"

	Current          = "current"
	StaleUnconfirmed = "stale-unconfirmed"
	Retired          = "retired"
	Unknown          = "UNKNOWN"

	maxChangedPaths = 1 << 20
)

// Record is one observed record: its ID, the commit it was observed at and its linked
// repository-relative source paths. A path that starts with `@` names an aliased root this
// evaluator cannot diff.
type Record struct {
	ID       string
	Kind     string
	Revision string
	Paths    []string
}

// History answers the Git questions the evaluator asks. Any error means the answer is missing.
type History interface {
	Resolve(rev string) (string, error)
	IsAncestor(older, newer string) (bool, error)
	Distance(older, newer string) (int, error)
	Changed(older, newer string) ([]string, error)
	// Untrusted names why ancestry answers cannot be trusted (grafts, shallow), or "".
	Untrusted() string
}

// Options configures one evaluation. RetireAfter 0 never retires.
type Options struct {
	RetireAfter int
}

// Result is one record's state.
type Result struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind,omitempty"`
	State    string   `json:"state"`
	Reason   string   `json:"reason,omitempty"`
	Revision string   `json:"observed_revision"`
	Distance *int     `json:"distance,omitempty"`
	Changed  []string `json:"changed_paths"`
	Paths    []string `json:"paths"`
}

// Counts is the denominator: Total equals the sum of the four states.
type Counts struct {
	Total            int `json:"total"`
	Current          int `json:"current"`
	StaleUnconfirmed int `json:"stale_unconfirmed"`
	Retired          int `json:"retired"`
	Unknown          int `json:"unknown"`
}

// Report is the observed-staleness/0 document.
type Report struct {
	Schema      string   `json:"schema"`
	At          string   `json:"at"`
	RetireAfter int      `json:"retire_after"`
	Source      string   `json:"source,omitempty"`
	Counts      Counts   `json:"counts"`
	Records     []Result `json:"records"`
}

type revisionFacts struct {
	reason   string
	distance int
	changed  map[string]bool
}

// Evaluate states every record at the resolved commit at.
func Evaluate(h History, at string, recs []Record, o Options) Report {
	rep := Report{Schema: Schema, At: at, RetireAfter: o.RetireAfter, Records: []Result{}}
	untrusted := h.Untrusted()
	facts := map[string]*revisionFacts{}
	factsFor := func(rev string) *revisionFacts {
		if f := facts[rev]; f != nil {
			return f
		}
		f := &revisionFacts{}
		facts[rev] = f
		c, err := h.Resolve(rev)
		switch {
		case untrusted != "":
			f.reason = "ancestry-untrusted: " + untrusted
		case err != nil:
			f.reason = "revision-unresolved"
		default:
			ok, err := h.IsAncestor(c, at)
			if err != nil {
				f.reason = "ancestry-unavailable"
				break
			}
			if !ok {
				f.reason = "not-ancestor"
				break
			}
			d, err := h.Distance(c, at)
			if err != nil {
				f.reason = "distance-unavailable"
				break
			}
			changed, err := h.Changed(c, at)
			if err != nil {
				f.reason = "missing-diff"
				break
			}
			f.distance, f.changed = d, map[string]bool{}
			for _, p := range changed {
				f.changed[p] = true
			}
		}
		return f
	}
	for _, r := range recs {
		res := Result{ID: r.ID, Kind: r.Kind, Revision: r.Revision, Changed: []string{}, Paths: append([]string{}, r.Paths...)}
		sort.Strings(res.Paths)
		switch {
		case len(r.Paths) == 0:
			res.State, res.Reason = Unknown, "no-linked-paths"
		case anyAliased(r.Paths):
			res.State, res.Reason = Unknown, "linked-path-outside-root"
		default:
			f := factsFor(r.Revision)
			if f.reason != "" {
				res.State, res.Reason = Unknown, f.reason
				break
			}
			d := f.distance
			res.Distance = &d
			for _, p := range res.Paths {
				if linkedChanged(f.changed, p) {
					res.Changed = append(res.Changed, p)
				}
			}
			switch {
			case len(res.Changed) == 0:
				res.State = Current
			case o.RetireAfter > 0 && d > o.RetireAfter:
				res.State = Retired
			default:
				res.State = StaleUnconfirmed
			}
		}
		switch res.State {
		case Current:
			rep.Counts.Current++
		case StaleUnconfirmed:
			rep.Counts.StaleUnconfirmed++
		case Retired:
			rep.Counts.Retired++
		default:
			rep.Counts.Unknown++
		}
		rep.Records = append(rep.Records, res)
	}
	rep.Counts.Total = len(rep.Records)
	return rep
}

func anyAliased(paths []string) bool {
	for _, p := range paths {
		if strings.HasPrefix(p, "@") {
			return true
		}
	}
	return false
}

// linkedChanged reports whether p, or any file under p when p names a directory, changed.
func linkedChanged(changed map[string]bool, p string) bool {
	if changed[p] {
		return true
	}
	prefix := strings.TrimSuffix(p, "/") + "/"
	for c := range changed {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

// GitHistory answers History from one local checkout, read-only.
type GitHistory struct {
	Ctx  context.Context
	Root string
}

func (g GitHistory) run(limit int, args ...string) ([]byte, int, error) {
	gitPath := gitstatus.Executable()
	if !filepath.IsAbs(gitPath) {
		return nil, -1, errors.New("Git unavailable")
	}
	argv := append([]string{gitPath, "--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "credential.helper=", "-C", g.Root}, args...)
	o := procgroup.Run(g.Ctx, procgroup.Spec{Argv: argv, Dir: g.Root, Env: gokernel.SanitizedGitEnvironment(), Timeout: 60 * time.Second, OutputLimit: limit})
	if o.Err != nil {
		return nil, -1, o.Err
	}
	return o.Stdout, o.ExitStatus, nil
}

func (g GitHistory) ok(limit int, args ...string) ([]byte, error) {
	out, status, err := g.run(limit, args...)
	if err != nil || status != 0 {
		return nil, errors.New("Git answer unavailable")
	}
	return out, nil
}

// Resolve names the commit rev denotes.
func (g GitHistory) Resolve(rev string) (string, error) {
	if rev == "" || strings.HasPrefix(rev, "-") {
		return "", errors.New("invalid revision")
	}
	out, err := g.ok(4096, "rev-parse", "--verify", "--quiet", "--end-of-options", rev+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// IsAncestor reports whether older is an ancestor of (or equal to) newer.
func (g GitHistory) IsAncestor(older, newer string) (bool, error) {
	_, status, err := g.run(4096, "merge-base", "--is-ancestor", older, newer)
	switch {
	case err != nil:
		return false, err
	case status == 0:
		return true, nil
	case status == 1:
		return false, nil
	}
	return false, errors.New("ancestry unavailable")
}

// Distance counts the commits in older..newer.
func (g GitHistory) Distance(older, newer string) (int, error) {
	out, err := g.ok(4096, "rev-list", "--count", older+".."+newer)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

// Changed lists every path that differs between older and newer, renames split into both sides.
func (g GitHistory) Changed(older, newer string) ([]string, error) {
	out, err := g.ok(64<<20, "-c", "core.quotePath=false", "diff", "--no-renames", "--no-ext-diff", "--name-only", "-z", older, newer, "--")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, p := range strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00") {
		if p != "" {
			paths = append(paths, path.Clean(p))
		}
	}
	if len(paths) > maxChangedPaths {
		return nil, errors.New("diff exceeds its path bound")
	}
	return paths, nil
}

// Untrusted names why the checkout's ancestry cannot be trusted, or "".
func (g GitHistory) Untrusted() string {
	return gitancestry.Rewritten(g.Ctx, func(ctx context.Context, args ...string) ([]byte, error) {
		return g.ok(4096, args...)
	})
}
