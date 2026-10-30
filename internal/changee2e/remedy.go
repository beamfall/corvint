package changee2e

import (
	"context"
	"errors"
	"maps"
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
)

// Remedy keys an unknown span can name beyond the manifest keys of AMAP-V0-210 (CCE-V0-150).
const (
	RemedyKeyReachExempt = "reach_exempt"
	RemedyKeyFlows       = "flows"
)

// remedyNone states that no declaration change bounds an unknown of this reason (CCE-V0-150).
func remedyNone(reason string) *appmap.Remedy {
	why := map[string]string{
		ReasonRemovedAtHead:   "a removed screen, flow or outcome is bounded by proving its removal, not by a declaration",
		ReasonImpactTruncated: "the impact report was truncated; narrow the change, no declaration bounds it",
		ReasonFanOutOverLimit: "the change reaches more obligations than the limit; narrow the change, no declaration bounds it",
	}[reason]
	return &appmap.Remedy{Uncertainty: why}
}

// globEscape escapes p so path.Match matches exactly p.
func globEscape(p string) string {
	var b strings.Builder
	for _, r := range p {
		if strings.ContainsRune(`*?[\`, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// serverDeclaration reports whether p is a Rails controller, a Rails routes file or an OpenAPI
// document: a path that may serve endpoints a client calls (CCE-V0-296).
func serverDeclaration(p string) bool {
	base := strings.ToLower(path.Base(p))
	ext := path.Ext(base)
	switch {
	case strings.HasSuffix(p, "_controller.rb") && (strings.HasPrefix(p, "app/controllers/") || strings.Contains(p, "/app/controllers/")):
		return true
	case p == "config/routes.rb" || strings.HasSuffix(p, "/config/routes.rb") || strings.Contains("/"+p, "/config/routes/") && ext == ".rb":
		return true
	}
	return (strings.HasPrefix(base, "openapi") || strings.HasPrefix(base, "swagger")) && (ext == ".json" || ext == ".yaml" || ext == ".yml")
}

// unplacedRemedy is the remedy of an unplaced changed path: a reach_exempt candidate for a non-UI
// path, the impact unknown's own remedy otherwise, or a map unknown's keyed remedy at that path. A
// server controller or route declaration names the endpoint join's inputs instead, with no key:
// its endpoints may be called by a client, which an exemption would hide (CCE-V0-296).
func unplacedRemedy(m *appmap.Map, p, impactReason string, impact *appmap.Remedy, joined bool) *appmap.Remedy {
	if impactReason == "non-ui-path" && serverDeclaration(p) {
		why := "a server controller or route declaration may serve endpoints a client calls; pass --endpoint-source DIR with --rails-routes FILE or --openapi FILE so obligations places it through the endpoint join (CCE-V0-152)"
		if joined {
			why = "the endpoint join placed no client call of this server controller or route declaration; check the --endpoint-source directories and route inputs"
		}
		return &appmap.Remedy{Uncertainty: why + "; reach_exempt applies only if no client calls it, which is not inferred"}
	}
	if impactReason == "non-ui-path" {
		return &appmap.Remedy{File: DeclarationsPath, Key: RemedyKeyReachExempt, Candidates: []string{globEscape(p)},
			Basis:       "a non-UI path the map places on no screen",
			Uncertainty: "an exemption asserts the path reaches no E2E outcome and takes effect from the base revision; its reviewed reason is not inferred"}
	}
	if impact != nil {
		return impact
	}
	for _, u := range m.Unknowns {
		if u.Repo == "" && u.Path == p {
			if r := appmap.UnknownRemedy(m, u); r.Key != "" {
				return r
			}
		}
	}
	return &appmap.Remedy{Uncertainty: "no declaration change is inferred for an unplaced path"}
}

// outcomeRemedy is the remedy of a reached screen no flow outcome covers.
func outcomeRemedy(m *appmap.Map, screen string) *appmap.Remedy {
	return &appmap.Remedy{File: m.Manifest.Path, Key: RemedyKeyFlows, Candidates: []string{screen},
		Basis:       "the change reaches this screen and no flow step on it declares an outcome",
		Uncertainty: "the outcome a flow step on the screen should expect is not inferred"}
}

// RemedySummarySchema is the range remedy summary (CCE-V0-151).
const RemedySummarySchema = "e2e-remedy-summary/0"

// MaxRemedyCommits bounds the first-parent commits a remedy summary evaluates (CCE-V0-151).
const MaxRemedyCommits = 50

// RemedySummary ranks the remedies that would bound the most merges of a range (CCE-V0-151).
type RemedySummary struct {
	Schema         string         `json:"schema"`
	Base           string         `json:"base"`
	Head           string         `json:"head"`
	Evaluated      int            `json:"evaluated"`
	OmittedCommits int            `json:"omitted_commits"`
	Merges         []RemedyMerge  `json:"merges"`
	Remedies       []RankedRemedy `json:"remedies"`
	Cover          []CoverStep    `json:"cover"`
	Unremediable   int            `json:"unremediable"`
	Digest         string         `json:"digest"`
}

// RemedyMerge is one evaluated first-parent commit: its obligations state, or the refusal that
// stopped it, and the one remedy that alone bounds it when there is one.
type RemedyMerge struct {
	Commit       string   `json:"commit"`
	State        string   `json:"state,omitempty"`
	Refusal      string   `json:"refusal,omitempty"`
	Unknown      int      `json:"unknown"`
	BoundedBy    string   `json:"bounded_by,omitempty"`
	RemedySet    []string `json:"remedy_set,omitempty"`
	Unremediable bool     `json:"unremediable,omitempty"`
}

// CoverStep is one step of the greedy remedy cover: the remedies it adds and the merges whose
// remedy sets the remedies chosen so far then cover (CCE-V0-297).
type CoverStep struct {
	Add    []string `json:"add"`
	Bounds []string `json:"bounds"`
	Total  int      `json:"total_bounded"`
}

// RankedRemedy is one keyed remedy with the merges it appears in and the merges it alone bounds.
type RankedRemedy struct {
	ID       string        `json:"id"`
	Remedy   appmap.Remedy `json:"remedy"`
	Bounds   int           `json:"bounds"`
	Appears  int           `json:"appears"`
	Unknowns int           `json:"unknowns"`
	Commits  []string      `json:"commits"`
}

// remedyID identifies a keyed remedy by its file, key and candidates.
func remedyID(r *appmap.Remedy) string {
	return r.File + "#" + r.Key + "=" + strings.Join(r.Candidates, "|")
}

// BuildRemedySummary evaluates the newest max first-parent commits of base..head, each against its
// first parent, and ranks the keyed remedies of their unknown spans (CCE-V0-151). A commit whose
// obligations refuse is recorded with its refusal code and counts as unremediable; any other
// error aborts.
func BuildRemedySummary(ctx context.Context, o DiffOptions, max int) ([]byte, error) {
	if o.Coverage != "" || o.Ledger != "" {
		return nil, refuse("invalid-arguments", "--remedy-summary does not combine with --coverage or --ledger")
	}
	if max < 1 || max > MaxRemedyCommits {
		return nil, refuse("invalid-arguments", "--remedy-commits must be 1 to %d", MaxRemedyCommits)
	}
	base, err := resolveRev(ctx, o.Root, o.Base)
	if err != nil {
		return nil, err
	}
	head, err := resolveRev(ctx, o.Root, o.Head)
	if err != nil {
		return nil, err
	}
	out, err := git(ctx, o.Root, "rev-list", "--first-parent", "--reverse", base+".."+head)
	if err != nil {
		return nil, err
	}
	commits := strings.Fields(string(out))
	s := &RemedySummary{Schema: RemedySummarySchema, Base: base, Head: head, Merges: []RemedyMerge{}, Remedies: []RankedRemedy{}}
	if len(commits) > max {
		s.OmittedCommits, commits = len(commits)-max, commits[len(commits)-max:]
	}
	ranked := map[string]*RankedRemedy{}
	for _, c := range commits {
		d := o
		d.Base, d.Head = c+"^", c
		m := RemedyMerge{Commit: c}
		ob, err := BuildObligations(ctx, d)
		var refusal *Refusal
		switch {
		case errors.As(err, &refusal):
			m.Refusal, m.Unremediable = refusal.Code, true
		case err != nil:
			return nil, err
		default:
			m.State, m.Unknown = ob.State, len(ob.Unknown)
			ids := map[string]bool{}
			for _, u := range ob.Unknown {
				if u.Remedy == nil || u.Remedy.Key == "" {
					m.Unremediable = true
					continue
				}
				id := remedyID(u.Remedy)
				if ids[id] {
					ranked[id].Unknowns++
					continue
				}
				ids[id] = true
				rr := ranked[id]
				if rr == nil {
					rr = &RankedRemedy{ID: id, Remedy: *u.Remedy, Commits: []string{}}
					ranked[id] = rr
				}
				rr.Appears++
				rr.Unknowns++
				rr.Commits = append(rr.Commits, c)
			}
			if !m.Unremediable && len(ids) > 0 {
				m.RemedySet = slices.Sorted(maps.Keys(ids))
			}
			if !m.Unremediable && len(ids) == 1 {
				m.BoundedBy = m.RemedySet[0]
				ranked[m.BoundedBy].Bounds++
			}
		}
		if m.Unremediable {
			s.Unremediable++
		}
		s.Merges = append(s.Merges, m)
	}
	s.Evaluated = len(s.Merges)
	for _, rr := range ranked {
		s.Remedies = append(s.Remedies, *rr)
	}
	s.Cover = greedyCover(s.Merges)
	sort.Slice(s.Remedies, func(i, j int) bool {
		a, b := s.Remedies[i], s.Remedies[j]
		if a.Bounds != b.Bounds {
			return a.Bounds > b.Bounds
		}
		if a.Appears != b.Appears {
			return a.Appears > b.Appears
		}
		return a.ID < b.ID
	})
	return seal(s, &s.Digest)
}

// greedyCover orders the remedies that bound the most merges with the fewest declarations: each
// step adds the remedies still missing from the remediable merge that needs the fewest (the
// earliest on a tie) and records every merge the chosen remedies then cover (CCE-V0-297).
// Merges with no unknown, or with an unremediable one, take no part.
func greedyCover(merges []RemedyMerge) []CoverStep {
	steps := []CoverStep{}
	chosen := map[string]bool{}
	open := []RemedyMerge{}
	for _, m := range merges {
		if len(m.RemedySet) > 0 {
			open = append(open, m)
		}
	}
	missing := func(m RemedyMerge) []string {
		return slices.DeleteFunc(slices.Clone(m.RemedySet), func(id string) bool { return chosen[id] })
	}
	total := 0
	for len(open) > 0 {
		best := 0
		for i := range open {
			if len(missing(open[i])) < len(missing(open[best])) {
				best = i
			}
		}
		step := CoverStep{Add: missing(open[best]), Bounds: []string{}}
		for _, id := range step.Add {
			chosen[id] = true
		}
		rest := open[:0]
		for _, m := range open {
			if len(missing(m)) == 0 {
				step.Bounds = append(step.Bounds, m.Commit)
			} else {
				rest = append(rest, m)
			}
		}
		open = rest
		total += len(step.Bounds)
		step.Total = total
		steps = append(steps, step)
	}
	return steps
}
