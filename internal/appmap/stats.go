package appmap

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
)

// StatsSchema identifies the adopter qualification report (AMAP-V0-060..065).
const StatsSchema = "application-map-stats/0"

const (
	statsTopReasons   = 5
	statsExamplesEach = 3
)

type statsExample struct {
	Repo string `json:"repo,omitempty"`
	Path string `json:"path"`
	Line int    `json:"line"`
	Ref  string `json:"ref"`
	// Detail is the unknown's refusal detail, when it carries one (AMAP-V0-202).
	Detail string `json:"detail,omitempty"`
}

type statsReason struct {
	Source   string         `json:"source"`
	Kind     string         `json:"kind"`
	Reason   string         `json:"reason"`
	Count    int            `json:"count"`
	Examples []statsExample `json:"examples"`
}

type statsCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// StatsSummary is the resolution summary of a map, used by the projection and the gate.
type StatsSummary struct {
	Total, Resolved int
	Dominant        []string
}

// ResolvedPercent is resolved screens as a percentage; an empty map reads 0.
func (s StatsSummary) ResolvedPercent() float64 {
	if s.Total == 0 {
		return 0
	}
	return float64(s.Resolved) * 100 / float64(s.Total)
}

// Stats summarises how much of the map resolved, from the map file alone (AMAP-V0-060..063).
func Stats(m *Map) StatsSummary {
	s := StatsSummary{Total: len(m.Screens)}
	for _, r := range statsReasons(m) {
		s.Dominant = append(s.Dominant, r.Source+":"+r.Kind+":"+r.Reason)
	}
	for _, sc := range m.Screens {
		if sc.Status == StatusResolved {
			s.Resolved++
		}
	}
	if len(s.Dominant) > statsTopReasons {
		s.Dominant = s.Dominant[:statsTopReasons]
	}
	return s
}

// statsReasons groups every map unknown (unresolved screens carry one) by source, kind and reason,
// most frequent first, with deterministic example anchors.
func statsReasons(m *Map) []statsReason {
	type key struct{ source, kind, reason string }
	groups := map[key]*statsReason{}
	add := func(k key, e statsExample) {
		g := groups[k]
		if g == nil {
			g = &statsReason{Source: k.source, Kind: k.kind, Reason: k.reason, Examples: []statsExample{}}
			groups[k] = g
		}
		g.Count++
		g.Examples = append(g.Examples, e)
	}
	for _, u := range m.Unknowns {
		add(key{"unknown", u.Kind, u.Reason}, statsExample{Repo: u.Repo, Path: u.Path, Line: u.Line, Ref: u.Ref, Detail: u.Detail})
	}
	out := make([]statsReason, 0, len(groups))
	for _, g := range groups {
		sort.Slice(g.Examples, func(i, j int) bool {
			a, b := g.Examples[i], g.Examples[j]
			if a.Path != b.Path {
				return a.Path < b.Path
			}
			if a.Line != b.Line {
				return a.Line < b.Line
			}
			if a.Ref != b.Ref {
				return a.Ref < b.Ref
			}
			return a.Detail < b.Detail
		})
		if len(g.Examples) > statsExamplesEach {
			g.Examples = g.Examples[:statsExamplesEach]
		}
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Reason < b.Reason
	})
	return out
}

func statsCounts(m map[string]int) []any {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	rows := make([]any, 0, len(names))
	for _, n := range names {
		rows = append(rows, statsCount{Name: n, Count: m[n]})
	}
	return rows
}

// ProjectStats is the capped adopter qualification report. It reads only the map file; every
// count reconciles with the map's own lists (AMAP-V0-060..065).
func ProjectStats(ctx context.Context, m *Map, o Options) ([]byte, error) {
	return ProjectStatsWith(ctx, m, o, false)
}

// statsRemedy is one keyed remedy of the map's unknowns and how many unknowns it names
// (AMAP-V0-343).
type statsRemedy struct {
	Remedy   Remedy `json:"remedy"`
	Unknowns int    `json:"unknowns"`
}

// mapRemedies groups the remedies of every map unknown that name a key and candidates by file,
// key and candidates, most unknowns first, and counts the unknowns no declaration names: those
// without a key or without a candidate value (AMAP-V0-343).
func mapRemedies(m *Map) ([]any, int) {
	byID, keyless := map[string]*statsRemedy{}, 0
	for _, u := range m.Unknowns {
		r := UnknownRemedy(m, u)
		if r.Key == "" || len(r.Candidates) == 0 { // a key with no value names no declaration
			keyless++
			continue
		}
		id := r.File + "#" + r.Key + "=" + strings.Join(r.Candidates, "|")
		if byID[id] == nil {
			byID[id] = &statsRemedy{Remedy: *r}
		}
		byID[id].Unknowns++
	}
	ids := slices.Sorted(maps.Keys(byID))
	sort.SliceStable(ids, func(i, j int) bool { return byID[ids[i]].Unknowns > byID[ids[j]].Unknowns })
	rows := make([]any, len(ids))
	for i, id := range ids {
		rows[i] = *byID[id]
	}
	return rows, keyless
}

// ProjectStatsWith is ProjectStats, adding with remedies the map-level remedy listing
// (AMAP-V0-343).
func ProjectStatsWith(ctx context.Context, m *Map, o Options, remedies bool) ([]byte, error) {
	budget, err := o.budget(DefaultFlowBudget)
	if err != nil {
		return nil, err
	}
	sum := Stats(m)
	reasons := statsReasons(m)
	unknownTotal := 0
	for _, r := range reasons {
		unknownTotal += r.Count
	}
	omittedReasons := 0
	if len(reasons) > statsTopReasons {
		omittedReasons = len(reasons) - statsTopReasons
		reasons = reasons[:statsTopReasons]
	}
	edges, strengths := map[string]int{}, map[string]int{}
	joinResolved, joinUnknown, pageObjects, unbound := 0, 0, 0, 0
	for _, f := range m.Files {
		if f.Join == StatusResolved {
			joinResolved++
		} else {
			joinUnknown++
		}
		if f.Role == rolePageObject {
			pageObjects++
			if f.Screen == "" {
				unbound++
			}
		}
		for _, sel := range f.Selectors {
			strengths[sel.Strength]++
		}
	}
	for _, e := range m.Edges {
		edges[e.Basis]++
	}
	p := newProjection(ctx, m, o)
	p.fresh = &freshness{oids: map[string]string{}, data: map[string][]byte{}, evaluated: "NOT_READ"}
	head := append(p.envelope(StatsSchema, budget),
		field{"screens_total", sum.Total}, field{"screens_resolved", sum.Resolved},
		field{"screens_unknown", sum.Total - sum.Resolved},
		field{"resolved_percent", fmt.Sprintf("%.2f", sum.ResolvedPercent())},
		field{"unknown_total", unknownTotal}, field{"unknown_reasons_omitted", omittedReasons},
		field{"test_files_join_resolved", joinResolved}, field{"test_files_join_unknown", joinUnknown},
		field{"page_objects", pageObjects}, field{"page_objects_unbound", unbound},
		field{"edges_total", len(m.Edges)})
	rows := make([]any, len(reasons))
	for i, r := range reasons {
		rows[i] = r
	}
	sections := []section{{"unknown_reasons", rows}, {"edges_by_basis", statsCounts(edges)}, {"selector_strength", statsCounts(strengths)}}
	if remedies {
		listed, keyless := mapRemedies(m)
		head = append(head, field{"unknowns_without_remedy_key", keyless})
		sections = append(sections, section{"remedies", listed})
	}
	return render(head, sections, budget)
}
