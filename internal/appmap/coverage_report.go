package appmap

import (
	"context"
	"sort"
)

// CoverageSchema identifies the per-module screen coverage report (AMAP-V0-601).
const CoverageSchema = "application-map-coverage/0"

type coverageUnknown struct {
	Screen string `json:"screen"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type coverageModule struct {
	Repo            string            `json:"repo,omitempty"`
	Module          string            `json:"module"`
	Total           int               `json:"total"`
	Reached         int               `json:"reached"`
	ReachedByBasis  map[string]int    `json:"reached_by_basis"`
	Abstract        int               `json:"abstract"`
	UnreachedStatus string            `json:"unreached_status"`
	UnreachedReason string            `json:"unreached_reason,omitempty"`
	Unreached       []string          `json:"unreached"`
	Unknown         []coverageUnknown `json:"unknown"`
}

// coverageModules groups the map's screens by declaring router file (AMAP-V0-601).
func coverageModules(m *Map) []coverageModule {
	status, reason := StatusResolved, ""
	if m.testJoin() != StatusResolved {
		status, reason = StatusUnknown, "test-join-unknown"
	}
	byKey := map[[2]string]*coverageModule{}
	var keys [][2]string
	for _, s := range m.Screens {
		k := [2]string{s.Anchor.Repo, s.Anchor.Path}
		row := byKey[k]
		if row == nil {
			row = &coverageModule{Repo: k[0], Module: k[1], ReachedByBasis: map[string]int{},
				UnreachedStatus: status, UnreachedReason: reason, Unreached: []string{}, Unknown: []coverageUnknown{}}
			byKey[k] = row
			keys = append(keys, k)
		}
		switch {
		case s.Status != StatusResolved:
			row.Unknown = append(row.Unknown, coverageUnknown{Screen: s.ID, Status: s.Status, Reason: s.Reason})
		case s.Abstract:
			row.Abstract++
		case len(s.Specs) == 0:
			row.Total++
			row.Unreached = append(row.Unreached, s.ID)
		default:
			row.Total++
			row.Reached++
			seen := map[string]bool{}
			for _, a := range s.Specs {
				if !seen[a.Basis] {
					seen[a.Basis] = true
					row.ReachedByBasis[a.Basis]++
				}
			}
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][0] != keys[j][0] {
			return keys[i][0] < keys[j][0]
		}
		return keys[i][1] < keys[j][1]
	})
	out := make([]coverageModule, 0, len(keys))
	for _, k := range keys {
		row := byKey[k]
		sort.Strings(row.Unreached)
		sort.Slice(row.Unknown, func(i, j int) bool { return row.Unknown[i].Screen < row.Unknown[j].Screen })
		out = append(out, *row)
	}
	return out
}

// ProjectCoverage renders the per-module reached/total screen report from the map file alone
// (AMAP-V0-601). Every count names the map field it is read from in `count_sources`.
func ProjectCoverage(ctx context.Context, m *Map, o Options) ([]byte, error) {
	budget, err := o.budget(DefaultFlowBudget)
	if err != nil {
		return nil, err
	}
	modules := coverageModules(m)
	total, reached, unreached, unknown := 0, 0, 0, 0
	items := make([]any, len(modules))
	for i, row := range modules {
		total += row.Total
		reached += row.Reached
		unreached += len(row.Unreached)
		unknown += len(row.Unknown)
		items[i] = row
	}
	// The report reads the map alone: no checkout, so its head names no evaluated revision.
	p := newProjection(ctx, m, o)
	p.fresh = &freshness{oids: map[string]string{}, data: map[string][]byte{}, evaluated: "NOT_READ"}
	head := append(p.envelope(CoverageSchema, budget),
		field{"test_join", m.testJoin()},
		field{"count_sources", map[string]string{
			"module":           "screens[].anchor.repo, screens[].anchor.path",
			"total":            "screens[].status RESOLVED and not screens[].abstract",
			"reached":          "screens[].specs non-empty",
			"reached_by_basis": "screens[].specs[].basis",
			"unreached":        "screens[].specs empty",
			"unknown":          "screens[].status not RESOLVED, screens[].reason",
			"unreached_status": "test_join",
		}},
		field{"modules_total", len(modules)}, field{"screens_total", total}, field{"screens_reached", reached},
		field{"screens_unreached", unreached}, field{"screens_unknown", unknown})
	return render(head, []section{{"modules", items}}, budget)
}
