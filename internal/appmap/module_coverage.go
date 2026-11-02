package appmap

import (
	"context"
	"sort"
)

// ModuleCoverageSchema identifies the per-module screen coverage report (AMAP-V0-810..812).
const ModuleCoverageSchema = "application-map-coverage/0"

type moduleCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// moduleUnknown is an unknown screen, or (Ref set, Screen empty) a router-level unknown such as a
// non-literal state name (AMAP-V0-811).
type moduleUnknown struct {
	Screen string `json:"screen,omitempty"`
	Ref    string `json:"ref,omitempty"`
	Reason string `json:"reason"`
}

// moduleRow is one router file's coverage. Counts are pointers so an undeclared router file reads
// null rather than a count the map cannot support (AMAP-V0-811).
type moduleRow struct {
	Module      string          `json:"module"`
	Status      string          `json:"status"`
	TotalSource string          `json:"total_source"`
	Total       *int            `json:"screens_total"`
	Reached     *int            `json:"screens_reached"`
	Unreached   *int            `json:"screens_unreached"`
	UnknownN    *int            `json:"screens_unknown"`
	Abstract    *int            `json:"screens_abstract"`
	ReachedBy   []moduleCount   `json:"reached_by"`
	UnreachedID []string        `json:"unreached"`
	Unknown     []moduleUnknown `json:"unknown"`
}

func moduleKey(a Anchor) string {
	if a.Repo != "" {
		return a.Repo + ":" + a.Path
	}
	return a.Path
}

// moduleCoverage groups the map's screens by declaring router file and counts, per module, the
// resolved screens a test is statically attributed to (AMAP-V0-810). Unknown screens and
// undeclared router files read UNKNOWN, never a count (AMAP-V0-811).
func moduleCoverage(m *Map) []moduleRow {
	rows := map[string]*moduleRow{}
	bases := map[string]map[string]int{}
	row := func(key string) *moduleRow {
		if rows[key] == nil {
			z := func() *int { n := 0; return &n }
			rows[key] = &moduleRow{Module: key, Status: StatusResolved, TotalSource: "declared-router",
				Total: z(), Reached: z(), Unreached: z(), UnknownN: z(), Abstract: z(),
				ReachedBy: []moduleCount{}, UnreachedID: []string{}, Unknown: []moduleUnknown{}}
			bases[key] = map[string]int{}
		}
		return rows[key]
	}
	for _, s := range m.Screens {
		r := row(moduleKey(s.Anchor))
		switch {
		case s.Status != StatusResolved:
			r.Status = StatusUnknown
			*r.UnknownN++
			r.Unknown = append(r.Unknown, moduleUnknown{Screen: s.ID, Reason: s.Reason})
		case s.Abstract:
			*r.Abstract++
		case len(s.Specs) > 0:
			*r.Total++
			*r.Reached++
			seen := map[string]bool{}
			for _, a := range s.Specs {
				if !seen[a.Basis] {
					seen[a.Basis] = true
					bases[r.Module][a.Basis]++
				}
			}
		default:
			*r.Total++
			*r.Unreached++
			r.UnreachedID = append(r.UnreachedID, s.ID)
		}
	}
	// A router-level unknown is never a screen count, but its file is a module that the map could
	// not read whole, even when it declared no screen (AMAP-V0-811).
	for _, u := range m.Unknowns {
		if (u.Kind == "router" || u.Kind == "state") && u.Reason != "router-not-declared" && u.Path != "" {
			r := row(moduleKey(Anchor{Repo: u.Repo, Path: u.Path}))
			r.Status = StatusUnknown
			r.Unknown = append(r.Unknown, moduleUnknown{Ref: u.Ref, Reason: u.Reason})
		}
	}
	if m.AppCoverage != nil {
		for _, p := range m.AppCoverage.Undeclared {
			if rows[p] == nil {
				rows[p] = &moduleRow{Module: p, Status: StatusUnknown, TotalSource: "router-not-declared",
					ReachedBy: []moduleCount{}, UnreachedID: []string{}, Unknown: []moduleUnknown{}}
			}
		}
	}
	keys := make([]string, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]moduleRow, len(keys))
	for i, k := range keys {
		r := rows[k]
		for _, c := range statsCounts(bases[k]) {
			sc := c.(statsCount)
			r.ReachedBy = append(r.ReachedBy, moduleCount(sc))
		}
		sort.Strings(r.UnreachedID)
		sort.Slice(r.Unknown, func(a, b int) bool {
			if r.Unknown[a].Screen != r.Unknown[b].Screen {
				return r.Unknown[a].Screen < r.Unknown[b].Screen
			}
			return r.Unknown[a].Ref < r.Unknown[b].Ref
		})
		out[i] = *r
	}
	return out
}

// ProjectModuleCoverage is the capped per-module coverage report; it reads only the map file
// (AMAP-V0-810..812).
func ProjectModuleCoverage(ctx context.Context, m *Map, o Options) ([]byte, error) {
	budget, err := o.budget(DefaultFlowBudget)
	if err != nil {
		return nil, err
	}
	mods := moduleCoverage(m)
	status, total, reached, unreached, unknown, abstract := StatusResolved, 0, 0, 0, 0, 0
	items := make([]any, len(mods))
	for i, r := range mods {
		items[i] = r
		if r.Status != StatusResolved {
			status = StatusUnknown
		}
		if r.Total != nil {
			total, reached, unreached, unknown, abstract = total+*r.Total, reached+*r.Reached, unreached+*r.Unreached, unknown+*r.UnknownN, abstract+*r.Abstract
		}
	}
	p := newProjection(ctx, m, o)
	p.fresh = &freshness{oids: map[string]string{}, data: map[string][]byte{}, evaluated: "NOT_READ"}
	head := append(p.envelope(ModuleCoverageSchema, budget), field{"status", status},
		field{"modules_total", len(mods)}, field{"screens_total", total}, field{"screens_reached", reached},
		field{"screens_unreached", unreached}, field{"screens_unknown", unknown}, field{"screens_abstract", abstract})
	return render(head, []section{{"modules", items}}, budget)
}
