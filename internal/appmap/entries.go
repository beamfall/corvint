package appmap

import (
	"context"
	"sort"
)

// EntryDeclaration retains what the bounded UI-router reader knows about entering a state.
// It is optional so older application-map/0 artifacts remain readable (AMAP-V0-053/058).
type EntryDeclaration struct {
	URLDeclared bool     `json:"url_declared"`
	Constraints []string `json:"constraints"`
}

func readEntryDeclaration(config jsValue) *EntryDeclaration {
	e := &EntryDeclaration{Constraints: []string{}}
	if config.kind != "object" {
		e.Constraints = append(e.Constraints, "non-literal-entry-config")
		return e
	}
	for _, p := range config.obj {
		switch p.key {
		case "url":
			e.URLDeclared = p.value.kind == "string"
		case "resolve", "params":
			if p.value.kind != "object" || len(p.value.obj) != 0 {
				e.Constraints = append(e.Constraints, "unsupported-"+p.key)
			}
		case "onEnter", "onExit", "onRetain", "redirectTo", "lazyLoad":
			e.Constraints = append(e.Constraints, "unsupported-"+p.key)
		}
	}
	e.Constraints = uniqueSorted(e.Constraints)
	return e
}

type entryGuard struct {
	Permissions []string `json:"permissions"`
	Flags       []string `json:"flags"`
}

type entryView struct {
	ID                   string     `json:"id"`
	State                string     `json:"state"`
	Classification       string     `json:"classification"`
	Reasons              []string   `json:"reasons"`
	URL                  string     `json:"entry_url_template"`
	RequiredParams       []string   `json:"required_params"`
	ParentParams         []string   `json:"parent_params"`
	QueryParams          []string   `json:"query_params"`
	Guard                entryGuard `json:"guard"`
	Verification         string     `json:"verification"`
	LastVerifiedRevision string     `json:"last_verified_revision"`
	ProvenPath           []string   `json:"proven_path"`
	ProvenBy             []string   `json:"proven_by,omitempty"`
	PathStatus           string     `json:"path_status"`
	PathReason           string     `json:"path_reason"`
}

// ProjectEntries is a static candidate projection, never a claim of runtime loading.
// Complete source blobs bind its entry constraints, conservatively invalidating even changes
// outside the old spans (AMAP-V0-050..059). Existing edge bases do not prove execution; only a
// revision-bound edge ledger l, when given, supplies shortest PROVEN paths (AMAP-V0-080..082).
func ProjectEntries(ctx context.Context, m *Map, l *EdgeLedger, o Options) ([]byte, error) {
	budget, err := o.budget(DefaultFlowBudget)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*Screen, len(m.Screens))
	ordered := make([]*Screen, 0, len(m.Screens))
	anchors := []Anchor{m.Manifest}
	for i := range m.Screens {
		s := &m.Screens[i]
		byID[s.ID] = s
		ordered = append(ordered, s)
		anchors = append(anchors, s.Anchor)
		anchors = append(anchors, s.NameFrom...)
		anchors = append(anchors, s.ParentFrom...)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	fresh := newFreshness(ctx, o, anchors)
	if err = checkLedgerBinding(m, l, fresh.evaluated); err != nil {
		return nil, err
	}
	sourceState := func(a Anchor) string {
		if fresh.failed || a.Repo != "" || a.Path == "" || a.Blob == "" {
			return FreshUnknown
		}
		if fresh.oids[a.Path] != a.Blob {
			return Stale
		}
		return Fresh
	}
	ambiguous := map[string]bool{}
	for _, key := range newScreenIndex(m.HashPrefix, m.BasePath, m.Screens).ambiguousKeys() {
		ambiguous[key] = true
	}
	guardUnknown := map[string]bool{}
	for _, u := range m.Unknowns {
		if u.Kind == "screen-permissions" || u.Kind == "screen-flags" {
			guardUnknown[u.Ref] = true
		}
	}
	views := []entryView{}
	for _, s := range ordered {
		row := entryView{ID: s.ID, State: s.State, URL: m.url(s),
			Classification: "deep-linkable", Reasons: []string{}, RequiredParams: uniqueSorted(s.Params),
			ParentParams: []string{}, QueryParams: uniqueSorted(s.Query),
			Guard:        entryGuard{Permissions: uniqueSorted(s.Permissions), Flags: uniqueSorted(s.Flags)},
			Verification: "STATIC_ONLY", ProvenPath: []string{}, PathStatus: StatusUnknown,
			PathReason: "no-proven-edge-evidence"}
		state := sourceState(m.Manifest)
		seen := map[string]bool{}
		for cur := s; cur != nil; cur = byID[cur.Parent] {
			if seen[cur.ID] || len(seen) >= maxRouteDepth {
				row.Reasons = append(row.Reasons, "entry-lineage-unresolved")
				break
			}
			seen[cur.ID] = true
			if cur.Status != StatusResolved {
				row.Reasons = append(row.Reasons, "unresolved-entry-lineage")
			}
			if cur.Entry == nil {
				row.Reasons = append(row.Reasons, "entry-metadata-unavailable")
			} else {
				row.Reasons = append(row.Reasons, cur.Entry.Constraints...)
			}
			if guardUnknown[cur.ID] {
				row.Reasons = append(row.Reasons, "dynamic-entry-guard")
			}
			if cur != s {
				row.ParentParams = append(row.ParentParams, cur.Params...)
			}
			aa := append([]Anchor{cur.Anchor}, cur.NameFrom...)
			aa = append(aa, cur.ParentFrom...)
			for _, a := range aa {
				f := sourceState(a)
				if f == Stale || (f == FreshUnknown && state != Stale) {
					state = f
				}
			}
			if cur.Parent != "" && byID[cur.Parent] == nil {
				row.Reasons = append(row.Reasons, "entry-lineage-unresolved")
			}
		}
		row.ParentParams = uniqueSorted(row.ParentParams)
		row.RequiredParams = uniqueSorted(append(row.RequiredParams, row.ParentParams...))
		if s.Entry != nil && !s.Entry.URLDeclared {
			row.Reasons = append(row.Reasons, "no-entry-url")
		}
		switch {
		case s.Status != StatusResolved:
			row.Classification = StatusUnknown
			row.Reasons = append(row.Reasons, s.Reason)
		case ambiguous[s.Key]:
			row.Classification = StatusUnknown
			row.Reasons = append(row.Reasons, "ambiguous-template")
		case s.Abstract:
			row.Classification = "abstract"
		case len(row.Reasons) != 0:
			row.Classification = StatusUnknown
		case len(s.Permissions) != 0 || len(s.Flags) != 0:
			row.Classification = "guarded(permission/flag)"
		case len(row.ParentParams) != 0:
			row.Classification = "requires-parent-params"
		}
		switch state {
		case Stale:
			row.Classification, row.Verification = StatusUnknown, "UNVERIFIED_AT_HEAD"
			row.Reasons = append(row.Reasons, "entry-source-changed")
		case FreshUnknown:
			row.Classification, row.Verification = StatusUnknown, StatusUnknown
			row.Reasons = append(row.Reasons, "entry-source-unavailable")
		}
		row.Reasons = uniqueSorted(row.Reasons)
		views = append(views, row)
	}
	p := newProjection(ctx, m, o)
	p.fresh = fresh
	head := append(p.envelope("application-map-entries/0", budget), field{"basis", "static-only"})
	if l != nil {
		provenPaths(views, l)
		head[len(head)-1] = field{"basis", "static-entries+edge-ledger"}
		head = append(head, field{"ledger_digest", l.Digest})
	}
	rows := make([]any, len(views))
	for i := range views {
		rows[i] = views[i]
	}
	return render(head, []section{{"entries", rows}}, budget)
}

// Path statuses with a ledger (AMAP-V0-081).
const (
	PathEntry       = "ENTRY"
	PathNotObserved = "NOT_OBSERVED"
)

// provenPaths gives each row its shortest path of PROVEN, receipt-backed ledger edges from a
// deep-linkable entry (AMAP-V0-081). Ties go to the lower entry ID, then the lower next screen ID
// and test key. A path never crosses a screen whose entry evidence is not STATIC_ONLY at the
// evaluated revision; nothing here visits a URL (AMAP-V0-082).
func provenPaths(rows []entryView, l *EdgeLedger) {
	type hop struct{ screen, test string }
	fresh := map[string]bool{}
	for _, r := range rows {
		fresh[r.ID] = r.Verification == "STATIC_ONLY"
	}
	adj := map[string][]hop{}
	for _, e := range l.Edges {
		test, ok := provenBy(e)
		if !ok || e.From == e.To || !fresh[e.From] || !fresh[e.To] {
			continue
		}
		adj[e.From] = append(adj[e.From], hop{e.To, test})
	}
	for k := range adj {
		hops := adj[k]
		sort.Slice(hops, func(i, j int) bool {
			if hops[i].screen != hops[j].screen {
				return hops[i].screen < hops[j].screen
			}
			return hops[i].test < hops[j].test
		})
	}
	prev := map[string]hop{}
	queue := []string{}
	reached := map[string]bool{}
	for _, r := range rows { // rows are in screen ID order
		if r.Classification == "deep-linkable" {
			reached[r.ID] = true
			queue = append(queue, r.ID)
		}
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, h := range adj[cur] {
			if !reached[h.screen] {
				reached[h.screen] = true
				prev[h.screen] = hop{cur, h.test}
				queue = append(queue, h.screen)
			}
		}
	}
	for i := range rows {
		r := &rows[i]
		switch {
		case !fresh[r.ID]:
			r.PathStatus, r.PathReason = StatusUnknown, "screen-source-not-fresh"
		case !reached[r.ID]:
			r.PathStatus, r.PathReason = PathNotObserved, "no-proven-path-from-entry"
		case r.Classification == "deep-linkable":
			r.ProvenPath, r.PathStatus, r.PathReason = []string{r.ID}, PathEntry, "static-deep-link"
		default:
			path, by := []string{r.ID}, []string{}
			for cur := r.ID; ; {
				h, ok := prev[cur]
				if !ok {
					break
				}
				path, by = append(path, h.screen), append(by, h.test)
				cur = h.screen
			}
			for a, z := 0, len(path)-1; a < z; a, z = a+1, z-1 {
				path[a], path[z] = path[z], path[a]
			}
			for a, z := 0, len(by)-1; a < z; a, z = a+1, z-1 {
				by[a], by[z] = by[z], by[a]
			}
			r.ProvenPath, r.ProvenBy, r.PathStatus, r.PathReason = path, by, EdgeProven, "ledger-edges"
		}
	}
}
