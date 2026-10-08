package appmap

import "context"

// LineageFreshness evaluates, at revision (default HEAD) of the repository at root, each named
// screen's lineage freshness by the AMAP-V0 rule the projections use: STALE when any anchor of the
// screen, its ancestors or their constant-table declarations is stale, else UNKNOWN when any is
// unknown, else FRESH. A screen that m does not hold is omitted. It reads only Git
// (TCN-V0-003).
func LineageFreshness(ctx context.Context, root, revision string, m *Map, screenIDs []string) map[string]string {
	lineages := map[string][]Anchor{}
	anchors := []Anchor{}
	for _, id := range screenIDs {
		s := m.screen(id)
		if s == nil {
			continue
		}
		if _, done := lineages[id]; done {
			continue
		}
		lineages[id] = m.lineage(s)
		anchors = append(anchors, lineages[id]...)
	}
	out := map[string]string{}
	if len(lineages) == 0 {
		return out
	}
	p := &projection{m: m, fresh: newFreshness(ctx, Options{Root: root, Revision: revision}, anchors)}
	for id, lineage := range lineages {
		out[id] = p.worst(lineage)
	}
	return out
}
