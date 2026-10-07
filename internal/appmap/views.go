package appmap

import (
	"context"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// The projections below are pure functions of one loaded map, one query and Options. Each returns
// one budgeted JSON document whose head names the map digest and the evaluated revision, so a later
// consumer (the V1-0959 planner) can compose several calls without re-reading the map.

type screenRef struct {
	ID       string `json:"id"`
	State    string `json:"state"`
	Template string `json:"template,omitempty"`
	URL      string `json:"url,omitempty"`
	Status   string `json:"status"`
	Reason   string `json:"reason,omitempty"`
}

// lineage returns the anchors a screen's template, query, permissions and flags derive from: its
// own state and every ancestor's (AMAP-V0-010).
func (m *Map) lineage(s *Screen) []Anchor {
	out, seen := []Anchor{}, map[string]bool{}
	for cur := s; cur != nil && !seen[cur.ID]; cur = m.screen(cur.Parent) {
		seen[cur.ID] = true
		out = append(out, cur.Anchor)
	}
	return out
}

// worst folds the freshness of several anchors: STALE if any is stale, else UNKNOWN if any is
// unknown, else FRESH.
func (p *projection) worst(anchors []Anchor) string {
	state := Fresh
	for _, a := range anchors {
		switch p.state(a) {
		case Stale:
			return Stale
		case FreshUnknown:
			state = FreshUnknown
		}
	}
	return state
}

func (m *Map) ref(s *Screen) screenRef {
	return screenRef{ID: s.ID, State: s.State, Template: s.Template, URL: m.url(s), Status: s.Status, Reason: s.Reason}
}

// candidates lists screens whose state or template contains q, for a miss or an ambiguity.
func (m *Map) candidates(q string) []any {
	q = strings.ToLower(q)
	out := []any{}
	for i := range m.Screens {
		s := &m.Screens[i]
		if strings.Contains(strings.ToLower(s.State), q) || (s.Template != "" && strings.Contains(strings.ToLower(s.Template), q)) {
			out = append(out, m.ref(s))
		}
	}
	return out
}

func (m *Map) step(id string) (*Flow, *Step) {
	for i := range m.Flows {
		for j := range m.Flows[i].Steps {
			if m.Flows[i].Steps[j].ID == id {
				return &m.Flows[i], &m.Flows[i].Steps[j]
			}
		}
	}
	return nil, nil
}

func (m *Map) method(id string) (*TestFile, *Method) {
	p, _, ok := strings.Cut(strings.TrimPrefix(id, "method:"), "#")
	if !ok {
		return nil, nil
	}
	f := m.file(p)
	if f == nil {
		return nil, nil
	}
	for i := range f.Methods {
		if f.Methods[i].ID == id {
			return f, &f.Methods[i]
		}
	}
	return f, nil
}

// testJoin is RESOLVED only when every test file was read, every import chain resolved and every
// page object is bound to exactly one screen (AMAP-V0-005). A spec with an incomplete import
// closure may reach any screen; a file the index excluded or could not load has no TestFile at
// all, yet may be the spec, page object or workflow that reaches a screen; and a spec reaching a
// screen only through an unbound or ambiguous page object is attributed to none. Each keeps the
// join UNKNOWN, so no screen's spec list is called complete.
func (m *Map) testJoin() string {
	for _, f := range m.Files {
		if f.Join != StatusResolved {
			return StatusUnknown
		}
	}
	for _, u := range m.Unknowns {
		if u.Kind == "file" || u.Kind == "page-object" {
			return StatusUnknown
		}
	}
	return StatusResolved
}

func unknownDoc(p *projection, schema string, budget int, query, reason string, candidates []any) ([]byte, error) {
	p.check()
	head := append(p.envelope(schema, budget), field{"query", query}, field{"status", StatusUnknown}, field{"reason", reason})
	return render(head, []section{{"candidates", candidates}}, budget)
}

type methodView struct {
	ID        string         `json:"id"`
	File      string         `json:"file"`
	Name      string         `json:"name"`
	Anchor    anchorView     `json:"anchor"`
	Selectors []selectorView `json:"selectors"`
}

// ProjectScreen is map_screen (AMAP-V0-011): one screen node with its hierarchy, steps,
// page-object methods, specs, edges and unknowns.
func ProjectScreen(ctx context.Context, m *Map, query string, o Options) ([]byte, error) {
	budget, err := o.budget(DefaultScreenBudget)
	if err != nil {
		return nil, err
	}
	p := newProjection(ctx, m, o)
	id, reason, collided := m.findScreen(strings.TrimSpace(query))
	if id == "" {
		cands := m.candidates(query)
		if len(collided) > 0 {
			cands = []any{}
			for _, c := range collided {
				cands = append(cands, m.ref(m.screen(c)))
			}
		}
		return unknownDoc(p, ScreenSchema, budget, query, reason, cands)
	}
	s := m.screen(id)
	lineage := m.lineage(s)
	for _, a := range lineage {
		p.cite(a)
	}
	p.element(s.ID)
	hierarchy := []any{}
	seen := map[string]bool{}
	for cur := m.screen(s.Parent); cur != nil && !seen[cur.ID]; cur = m.screen(cur.Parent) {
		seen[cur.ID] = true
		hierarchy = append([]any{m.ref(cur)}, hierarchy...)
	}
	type pending struct {
		file *TestFile
		meth *Method
	}
	methods := []pending{}
	for _, po := range s.PageObjects {
		if f := m.file(po); f != nil {
			p.cite(f.Anchor)
			if f.ScreenBasis == "declared" {
				p.cite(m.Manifest)
			}
			p.element(f.ID)
			for i := range f.Methods {
				p.cite(f.Methods[i].Anchor)
				p.element(f.Methods[i].ID)
				methods = append(methods, pending{f, &f.Methods[i]})
			}
		}
	}
	for _, a := range s.Specs {
		anchors, _ := m.chain(a)
		for _, an := range anchors {
			p.cite(an)
		}
		if f := m.file(a.File); f != nil {
			p.element(f.ID)
		}
	}
	steps := []*Step{}
	flows := map[string]*Flow{}
	for _, sid := range s.Steps {
		if fl, st := m.step(sid); st != nil {
			p.cite(fl.Anchor)
			p.element(st.ID)
			steps = append(steps, st)
			flows[st.ID] = fl
		}
	}
	out, in := []Edge{}, []Edge{}
	for _, e := range m.Edges {
		if e.From == s.ID {
			out = append(out, e)
		}
		if e.To == s.ID {
			in = append(in, e)
		}
		if e.From == s.ID || e.To == s.ID {
			p.cite(e.Anchor)
		}
	}
	p.check()
	stale := map[string]bool{}
	head := append(p.envelope(ScreenSchema, budget), field{"query", query}, field{"status", StatusResolved})
	sv := struct {
		screenRef
		Parent          string     `json:"parent,omitempty"`
		Abstract        bool       `json:"abstract,omitempty"`
		Params          []string   `json:"params"`
		Query           []string   `json:"query"`
		Permissions     []string   `json:"permissions"`
		PermissionsFrom string     `json:"permissions_from,omitempty"`
		Flags           []string   `json:"flags"`
		FlagsFrom       string     `json:"flags_from,omitempty"`
		Anchor          anchorView `json:"anchor"`
		Lineage         string     `json:"lineage_freshness"`
		TestJoin        string     `json:"test_join"`
	}{m.ref(s), s.Parent, s.Abstract, s.Params, s.Query, s.Permissions, s.PermissionsFrom, s.Flags, s.FlagsFrom, p.anchorView(s.Anchor),
		p.worst(lineage), m.testJoin()}
	if sv.Lineage == Stale {
		stale[s.ID] = true
	}
	head = append(head, field{"screen", sv})
	stepItems := []any{}
	for _, st := range steps {
		v := p.stepView(flows[st.ID], *st)
		if v.Freshness == Stale {
			stale[st.ID] = true
		}
		stepItems = append(stepItems, v)
	}
	methodItems := []any{}
	for _, pm := range methods {
		v := methodView{ID: pm.meth.ID, File: pm.file.Path, Name: pm.meth.Name, Anchor: p.anchorView(pm.meth.Anchor), Selectors: []selectorView{}}
		for i := range pm.meth.Selectors {
			v.Selectors = append(v.Selectors, *viewSelector(&pm.meth.Selectors[i]))
		}
		if v.Anchor.Freshness == Stale {
			stale[v.ID] = true
		}
		methodItems = append(methodItems, v)
	}
	specs := []any{}
	for _, a := range s.Specs {
		v := p.specView(a)
		if v.Chain == Stale {
			stale[fileID(a.File)] = true
		}
		specs = append(specs, v)
	}
	edgeItems := func(es []Edge) []any {
		items := []any{}
		for _, e := range es {
			items = append(items, edgeView{From: e.From, To: e.To, Basis: e.Basis, Source: e.Source, Anchor: p.anchorView(e.Anchor)})
		}
		return items
	}
	preconditions := []any{}
	for _, r := range s.Preconditions {
		preconditions = append(preconditions, r)
	}
	refs := map[string]bool{s.ID: true}
	for _, st := range s.Steps {
		refs[st] = true
	}
	for _, po := range s.PageObjects {
		refs[fileID(po)] = true
	}
	learned, overlayUnknowns := p.learned(stale)
	return render(head, []section{
		{"specs", specs},
		{"steps", stepItems},
		{"page_object_methods", methodItems},
		{"edges_in", edgeItems(in)},
		{"edges_out", edgeItems(out)},
		{"hierarchy", hierarchy},
		{"flows", strings2any(s.Flows)},
		{"preconditions", preconditions},
		{"page_objects", strings2any(s.PageObjects)},
		{"workflows", strings2any(s.Workflows)},
		{"scenarios", strings2any(s.Scenarios)},
		{"learned", learned},
		{"unknowns", append(m.unknownsFor(refs), overlayUnknowns...)},
		{"unresolved_tests", m.unresolvedSpecs()},
	}, budget)
}

// ProjectFlow is map_flow (AMAP-V0-011): one flow's steps on the screen graph with the screens,
// edges, outcomes and specs they touch.
func ProjectFlow(ctx context.Context, m *Map, query string, o Options) ([]byte, error) {
	budget, err := o.budget(DefaultFlowBudget)
	if err != nil {
		return nil, err
	}
	p := newProjection(ctx, m, o)
	fl := m.flow(strings.TrimSpace(query))
	if fl == nil {
		cands := []any{}
		for _, f := range m.Flows {
			if strings.Contains(f.FlowID, strings.ToLower(strings.TrimPrefix(query, "flow:"))) {
				cands = append(cands, f.ID)
			}
		}
		return unknownDoc(p, FlowSchema, budget, query, "no-matching-flow", cands)
	}
	p.cite(fl.Anchor)
	p.element(fl.ID)
	screens := []*Screen{}
	onFlow := map[string]bool{}
	for _, st := range fl.Steps {
		p.element(st.ID)
		if s := m.screen(st.Screen); s != nil && !onFlow[s.ID] {
			onFlow[s.ID] = true
			screens = append(screens, s)
			for _, a := range m.lineage(s) {
				p.cite(a)
			}
			p.element(s.ID)
		}
	}
	type specOnFlow struct {
		Screen string `json:"screen"`
		specView
	}
	specFiles := []Attribution{}
	specScreens := []string{}
	seenSpec := map[string]bool{}
	for _, s := range screens {
		for _, a := range s.Specs {
			if !seenSpec[a.File] {
				seenSpec[a.File] = true
				specFiles = append(specFiles, a)
				specScreens = append(specScreens, s.ID)
				anchors, _ := m.chain(a)
				for _, an := range anchors {
					p.cite(an)
				}
			}
		}
	}
	edges := []Edge{}
	for _, e := range m.Edges {
		if onFlow[e.From] && onFlow[e.To] {
			edges = append(edges, e)
			p.cite(e.Anchor)
		}
	}
	p.check()
	stale := map[string]bool{}
	fv := struct {
		ID       string     `json:"id"`
		FlowID   string     `json:"flow_id"`
		Anchor   anchorView `json:"anchor"`
		TestJoin string     `json:"test_join"`
	}{fl.ID, fl.FlowID, p.anchorView(fl.Anchor), m.testJoin()}
	if fv.Anchor.Freshness == Stale {
		stale[fl.ID] = true
		for _, st := range fl.Steps {
			stale[st.ID] = true
		}
	}
	head := append(p.envelope(FlowSchema, budget), field{"query", query}, field{"status", StatusResolved}, field{"flow", fv})
	stepItems := []any{}
	for _, st := range fl.Steps {
		stepItems = append(stepItems, p.stepView(fl, st))
	}
	screenItems := []any{}
	for _, s := range screens {
		v := struct {
			screenRef
			Permissions []string   `json:"permissions"`
			Flags       []string   `json:"flags"`
			Anchor      anchorView `json:"anchor"`
			Lineage     string     `json:"lineage_freshness"`
		}{m.ref(s), s.Permissions, s.Flags, p.anchorView(s.Anchor), p.worst(m.lineage(s))}
		if v.Lineage == Stale {
			stale[s.ID] = true
		}
		screenItems = append(screenItems, v)
	}
	edgeItems := []any{}
	for _, e := range edges {
		edgeItems = append(edgeItems, edgeView{From: e.From, To: e.To, Basis: e.Basis, Source: e.Source, Anchor: p.anchorView(e.Anchor)})
	}
	outcomes := []any{}
	for _, oc := range fl.Outcomes {
		outcomes = append(outcomes, oc)
	}
	specs := []any{}
	for i, a := range specFiles {
		specs = append(specs, specOnFlow{Screen: specScreens[i], specView: p.specView(a)})
	}
	preconditions := strings2any(fl.Preconditions)
	refs := map[string]bool{fl.ID: true}
	for _, st := range fl.Steps {
		refs[st.ID] = true
	}
	for _, s := range screens {
		refs[s.ID] = true
	}
	learned, overlayUnknowns := p.learned(stale)
	return render(head, []section{
		{"steps", stepItems},
		{"screens", screenItems},
		{"edges", edgeItems},
		{"outcomes", outcomes},
		{"specs", specs},
		{"preconditions", preconditions},
		{"learned", learned},
		{"unknowns", append(m.unknownsFor(refs), overlayUnknowns...)},
		{"unresolved_tests", m.unresolvedSpecs()},
	}, budget)
}

type findItem struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Ref   string `json:"ref,omitempty"`
}

// ProjectFind is map_find (AMAP-V0-012): a case-insensitive substring search over element IDs and
// labels. It cites no anchors and runs no freshness check; map_screen and map_flow do.
func ProjectFind(ctx context.Context, m *Map, text string, o Options) ([]byte, error) {
	budget, err := o.budget(DefaultFindBudget)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(text))
	if len(q) < 2 || len(q) > 128 {
		return nil, &gokernel.Error{Code: "appmap-invalid-query", Message: "--text must be 2 to 128 bytes"}
	}
	p := newProjection(ctx, m, o)
	p.fresh = &freshness{evaluated: "NOT_EVALUATED"}
	hit := func(parts ...string) bool {
		for _, s := range parts {
			if s != "" && strings.Contains(strings.ToLower(s), q) {
				return true
			}
		}
		return false
	}
	screens, flows, steps, files, methods, selectors := []any{}, []any{}, []any{}, []any{}, []any{}, []any{}
	for i := range m.Screens {
		s := &m.Screens[i]
		if hit(s.ID, s.Template) {
			screens = append(screens, findItem{s.ID, s.Template, anchorRef(s.Anchor)})
			p.element(s.ID)
		}
	}
	seenSel := map[string]bool{}
	addSel := func(sel Selector, ref string) {
		if !seenSel[sel.ID] && hit(sel.ID, sel.Value, sel.Name) {
			seenSel[sel.ID] = true
			selectors = append(selectors, findItem{sel.ID, sel.Kind + ":" + sel.Value + nameSuffix(sel.Name), ref})
			p.element(sel.ID)
		}
	}
	for _, fl := range m.Flows {
		if hit(fl.ID, fl.FlowID) {
			flows = append(flows, findItem{fl.ID, fl.FlowID, anchorRef(fl.Anchor)})
			p.element(fl.ID)
		}
		for _, st := range fl.Steps {
			label := st.Action
			if hit(st.ID, st.Action) {
				steps = append(steps, findItem{st.ID, label, st.Screen})
				p.element(st.ID)
			}
			if st.Selector != nil {
				addSel(*st.Selector, st.ID)
			}
		}
	}
	for _, f := range m.Files {
		if hit(append([]string{f.ID, f.Path, f.Class}, f.Tests...)...) {
			files = append(files, findItem{f.ID, f.Role, anchorRef(f.Anchor)})
			p.element(f.ID)
		}
		for _, me := range f.Methods {
			if hit(me.ID, me.Name) {
				methods = append(methods, findItem{me.ID, me.Name, anchorRef(me.Anchor)})
				p.element(me.ID)
			}
		}
		for _, sel := range f.Selectors {
			addSel(sel, refAt(f.Path, sel.Line))
		}
	}
	head := append(p.envelope(FindSchema, budget), field{"query", text})
	learned, overlayUnknowns := p.learned(nil)
	return render(head, []section{
		{"screens", screens},
		{"flows", flows},
		{"steps", steps},
		{"files", files},
		{"methods", methods},
		{"selectors", selectors},
		{"learned", learned},
		{"unknowns", overlayUnknowns},
	}, budget)
}

func nameSuffix(name string) string {
	if name == "" {
		return ""
	}
	return " name=" + name
}
