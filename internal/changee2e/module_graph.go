package changee2e

import (
	"context"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

// ModuleGraphBasis names the TJAA module-graph reach source (CCE-V0-054).
const ModuleGraphBasis = "tjaa-module-graph"

// moduleGraphOperations bounds the Git operations of one head-tree read, as the delta compiler does.
const moduleGraphOperations = gitrun.DefaultOperations + 2*affected.MaxWalkEntries

// ModuleGraphRelation records the consulted module graph: its digest and frontier, or why it could
// not be built.
type ModuleGraphRelation struct {
	Digest      string   `json:"digest,omitempty"`
	Frontier    []string `json:"frontier"`
	Unavailable string   `json:"unavailable,omitempty"`
}

// moduleGraphHits widens reach through the TypeScript/JavaScript module graph of the head tree
// (CCE-V0-054): each changed path the graph owns reaches every screen that places a file of a unit
// importing it, directly or transitively, through a router anchor, a map test file or a flow
// anchor, and each lineage descendant of a router-anchored screen. The graph is read from the
// immutable head tree, never the worktree. A hit already in impact or coverage is not repeated.
func moduleGraphHits(ctx context.Context, root, head string, m *appmap.Map, changed []string, prior []appmap.ImpactScreen) ([]appmap.ImpactScreen, *ModuleGraphRelation) {
	rel := &ModuleGraphRelation{Frontier: []string{}}
	auth, err := gitauth.Open(root, gitrun.NewBudget(moduleGraphOperations, gitrun.DefaultTotalBudget))
	if err != nil {
		rel.Unavailable = "repository-unreadable"
		return nil, rel
	}
	source, err := auth.RevisionFS(ctx, head, affected.MaxSourceBytes)
	if err != nil {
		rel.Unavailable = "head-tree-unreadable"
		return nil, rel
	}
	lang := typescript.New()
	graph, err := affected.BuildFS(source, lang)
	if err != nil {
		rel.Unavailable = "module-graph-unbuildable"
		return nil, rel
	}
	rel.Digest, rel.Frontier = graph.Digest(), append(rel.Frontier, graph.Frontier()...)
	already := map[string]bool{}
	for _, h := range prior {
		for _, p := range h.Paths {
			already[h.ID+"\x00"+p] = true
		}
	}
	placements := screenPlacements(m)
	byScreen := map[string]map[string]bool{}
	via := map[string]string{}
	for _, p := range changed {
		if !lang.Owns(p) {
			continue
		}
		for _, id := range graph.ReachedUnitIDs([]string{p}) {
			u, ok := graph.Unit(id)
			if !ok {
				continue
			}
			for _, f := range append(append([]string(nil), u.Sources...), u.Tests...) {
				if f == p {
					continue // the changed file's own placement is impact's
				}
				for _, screen := range placements[f] {
					if already[screen+"\x00"+p] {
						continue
					}
					if byScreen[screen] == nil {
						byScreen[screen] = map[string]bool{}
					}
					byScreen[screen][p] = true
					if k := screen + "\x00" + p; via[k] == "" || f < via[k] {
						via[k] = f
					}
				}
			}
		}
	}
	var hits []appmap.ImpactScreen
	for screen, paths := range byScreen {
		ps := make([]string, 0, len(paths))
		for p := range paths {
			ps = append(ps, p)
		}
		sort.Strings(ps)
		vias := make([]string, 0, len(ps))
		for _, p := range ps {
			vias = append(vias, via[screen+"\x00"+p])
		}
		hits = append(hits, appmap.ImpactScreen{ID: screen, Basis: ModuleGraphBasis + ":" + strings.Join(uniqueStrings(sortedCopy(vias)), ","), Paths: ps})
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].ID < hits[j].ID })
	return hits, rel
}

func sortedCopy(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}

// screenPlacements maps each repository path the head map places to its screens: router anchors
// (with their lineage descendants), map test files and flow anchors, as impact places a changed
// path.
func screenPlacements(m *appmap.Map) map[string][]string {
	set := map[string]map[string]bool{}
	add := func(p, screen string) {
		if p == "" || screen == "" {
			return
		}
		if set[p] == nil {
			set[p] = map[string]bool{}
		}
		set[p][screen] = true
	}
	parent := map[string]string{}
	for _, s := range m.Screens {
		parent[s.ID] = s.Parent
	}
	anchored := map[string][]string{} // screen -> its router-anchor paths
	for _, s := range m.Screens {
		for _, a := range append(append([]appmap.Anchor{s.Anchor}, s.NameFrom...), s.ParentFrom...) {
			if a.Repo == "" {
				add(a.Path, s.ID)
				anchored[s.ID] = append(anchored[s.ID], a.Path)
			}
		}
	}
	for _, s := range m.Screens {
		seen := map[string]bool{s.ID: true}
		for cur := parent[s.ID]; cur != "" && !seen[cur]; cur = parent[cur] {
			seen[cur] = true
			if ps := anchored[cur]; len(ps) > 0 {
				for _, p := range ps {
					add(p, s.ID)
				}
				break
			}
		}
	}
	for _, f := range m.Files {
		if f.Anchor.Repo == "" {
			add(f.Path, f.Screen)
		}
	}
	for _, fl := range m.Flows {
		if fl.Anchor.Repo == "" {
			for _, st := range fl.Steps {
				add(fl.Anchor.Path, st.Screen)
			}
		}
	}
	out := make(map[string][]string, len(set))
	for p, screens := range set {
		for s := range screens {
			out[p] = append(out[p], s)
		}
		sort.Strings(out[p])
	}
	return out
}
