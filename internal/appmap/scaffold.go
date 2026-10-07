package appmap

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type closestView struct {
	File          string     `json:"file,omitempty"`
	Status        string     `json:"status"`
	Reason        string     `json:"reason,omitempty"`
	Assertions    int        `json:"assertions,omitempty"`
	ScreensShared int        `json:"screens_shared,omitempty"`
	ReuseShared   int        `json:"reuse_shared,omitempty"`
	Anchor        anchorView `json:"anchor"`
}

type reuseView struct {
	Step     string `json:"step"`
	Method   string `json:"method"`
	Ref      string `json:"ref"`
	Strength string `json:"strength"`
}

// closestSpec picks the spec the scaffold borrows from (AMAP-V0-013): a spec with a complete
// import join and at least one assertion, attributed to a screen on the flow; ranked by flow
// screens reached, then reused page-object files on its import chains, then fewest import hops
// to those screens, then path.
func (m *Map) closestSpec(fl *Flow) (*TestFile, int, int) {
	onFlow := map[string]bool{}
	reuseFiles := map[string]bool{}
	for _, st := range fl.Steps {
		if st.Screen != "" {
			onFlow[st.Screen] = true
		}
		for _, r := range st.Reuse {
			if mp, _, ok := strings.Cut(strings.TrimPrefix(r, "method:"), "#"); ok && strings.HasPrefix(r, "method:") {
				reuseFiles[mp] = true
			}
		}
	}
	screens, reuse, hops := map[string]int{}, map[string]map[string]bool{}, map[string]int{}
	for _, id := range sortedKeys(onFlow) {
		s := m.screen(id)
		if s == nil {
			continue
		}
		for _, a := range s.Specs {
			screens[a.File]++
			hops[a.File] += len(a.Via)
			if reuse[a.File] == nil {
				reuse[a.File] = map[string]bool{}
			}
			for _, v := range a.Via {
				if reuseFiles[v] {
					reuse[a.File][v] = true
				}
			}
		}
	}
	var best *TestFile
	bs, br, bh := 0, 0, 0
	for _, p := range sortedKeys(screens) {
		f := m.file(p)
		if f == nil || f.Role != roleSpec || f.Join != StatusResolved || f.Assertions == 0 {
			continue
		}
		s, r, h := screens[p], len(reuse[p]), hops[p]
		if best == nil || s > bs || (s == bs && (r > br || (r == br && h < bh))) {
			best, bs, br, bh = f, s, r, h
		}
	}
	return best, bs, br
}

// relSpecifier is the relative import specifier from a file in dir to target, without extension.
func relSpecifier(dir, target string) (string, bool) {
	rel, err := filepath.Rel(filepath.FromSlash(dir), filepath.FromSlash(target))
	if err != nil {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if ext := path.Ext(rel); webSuffix[ext] {
		rel = strings.TrimSuffix(rel, ext)
	}
	if !strings.HasPrefix(rel, "../") {
		rel = "./" + rel
	}
	return rel, true
}

func quote(s string) string {
	raw, _ := json.Marshal(s)
	return string(raw)
}

// ProjectScaffold is map_scaffold (AMAP-V0-013): a draft spec skeleton for one flow, borrowing the
// imports of the closest asserting spec and calling existing page-object methods where a step's
// selector is already implemented. Every borrowed import is re-checked against the map; one that
// does not resolve is emitted commented out and marked UNRESOLVED.
func ProjectScaffold(ctx context.Context, m *Map, query string, o Options) ([]byte, error) {
	budget, err := o.budget(DefaultScaffoldBudget)
	if err != nil {
		return nil, err
	}
	p := newProjection(ctx, m, o)
	fl := m.flow(strings.TrimSpace(query))
	if fl == nil {
		return unknownDoc(p, ScaffoldSchema, budget, query, "no-matching-flow", []any{})
	}
	p.cite(fl.Anchor)
	p.element(fl.ID)
	closest, screensShared, reuseShared := m.closestSpec(fl)
	if closest != nil {
		p.cite(closest.Anchor)
		p.element(closest.ID)
	}
	type call struct {
		file *TestFile
		meth *Method
		ref  string
	}
	calls := map[string]call{}
	for _, st := range fl.Steps {
		p.element(st.ID)
		for _, r := range st.Reuse {
			f, me := m.method(r)
			if me == nil || f.Class == "" || (f.Role != rolePageObject && f.Role != roleWorkflow) {
				continue
			}
			calls[st.ID] = call{f, me, refAt(f.Path, me.Anchor.Start)}
			p.cite(me.Anchor)
			p.element(me.ID)
			break
		}
	}
	p.check()
	stale := map[string]bool{}
	cv := closestView{Status: StatusUnknown, Reason: "no-asserting-spec"}
	proposed, dir := "", ""
	imports, unknowns := []any{}, []any{}
	// imported holds, per resolved file, the local names the borrowed imports bind from it; locals
	// holds every name they bind. A class is usable only under a name actually bound to it.
	imported, locals := map[string]map[string]bool{}, map[string]bool{}
	if closest != nil {
		cv = closestView{File: closest.Path, Status: StatusResolved, Assertions: closest.Assertions, ScreensShared: screensShared,
			ReuseShared: reuseShared, Anchor: p.anchorView(closest.Anchor)}
		if cv.Anchor.Freshness == Stale {
			stale[closest.ID] = true
		}
		dir = path.Dir(closest.Path)
		proposed = path.Join(dir, fl.FlowID+".spec.ts")
		borrowed := append([]Import{}, closest.Imports...)
		sort.SliceStable(borrowed, func(i, j int) bool { return borrowed[i].Line < borrowed[j].Line })
		for _, imp := range borrowed {
			// The proposed file sits beside the closest spec, so a specifier the index resolved
			// there resolves identically from the proposed path.
			ok := imp.Status == importExternal || (imp.Status == importResolved && m.file(imp.Resolved) != nil)
			if ok && imp.Status == importResolved {
				if imported[imp.Resolved] == nil {
					imported[imp.Resolved] = map[string]bool{}
				}
				for _, n := range imp.Names {
					imported[imp.Resolved][n] = true
				}
			}
			for _, n := range imp.Names {
				locals[n] = true
			}
			stmt := imp.Statement
			if stmt == "" {
				stmt = "import " + quote(imp.Specifier) + ";"
			}
			if !ok {
				stmt = "// UNRESOLVED " + strings.ReplaceAll(stmt, "\n", "\n// ")
				unknowns = append(unknowns, Unknown{Kind: "scaffold-import", Ref: closest.ID, Reason: "unresolved-import", Path: closest.Path, Line: imp.Line})
			}
			imports = append(imports, stmt)
		}
	} else {
		unknowns = append(unknowns, Unknown{Kind: "scaffold", Ref: fl.ID, Reason: "no-asserting-spec"})
	}
	// Page objects the steps reuse but the closest spec does not import.
	vars := map[string]string{}
	order := []string{}
	for _, st := range fl.Steps {
		c, ok := calls[st.ID]
		if !ok {
			continue
		}
		if _, seen := vars[c.file.Path]; !seen {
			vars[c.file.Path] = lowerFirst(c.file.Class)
			order = append(order, c.file.Path)
		}
		if !imported[c.file.Path][c.file.Class] {
			// Not bound under its own name: absent, aliased (import { X as Y }), default or namespace.
			if imported[c.file.Path] == nil {
				imported[c.file.Path] = map[string]bool{}
			}
			imported[c.file.Path][c.file.Class] = true
			switch {
			case dir == "":
				imports = append(imports, "// UNRESOLVED import { "+c.file.Class+" } from <no proposed path>;")
			case locals[c.file.Class]:
				imports = append(imports, "// UNRESOLVED import { "+c.file.Class+" } collides with a borrowed binding;")
				unknowns = append(unknowns, Unknown{Kind: "scaffold-import", Ref: c.file.ID, Reason: "binding-collision", Path: c.file.Path})
			default:
				spec, _ := relSpecifier(dir, c.file.Path)
				imports = append(imports, "import { "+c.file.Class+" } from "+quote(spec)+";")
			}
		}
	}
	lines := []any{fmt.Sprintf("test(%s, async ({ page }) => {", quote(fl.FlowID))}
	for _, pre := range fl.Preconditions {
		lines = append(lines, "  // precondition: "+pre)
	}
	for _, f := range order {
		lines = append(lines, fmt.Sprintf("  const %s = new %s(page);", vars[f], m.file(f).Class))
	}
	reuse := []any{}
	for _, st := range fl.Steps {
		where := st.Screen
		if where == "" {
			where = "UNKNOWN " + st.Reason
		}
		lines = append(lines, fmt.Sprintf("  // step %s: %s [%s]", st.ID, st.Action, where))
		if c, ok := calls[st.ID]; ok {
			lines = append(lines, fmt.Sprintf("  await %s.%s(); // reuse %s [%s]", vars[c.file.Path], c.meth.Name, c.ref, st.Selector.Strength))
			reuse = append(reuse, reuseView{Step: st.ID, Method: c.meth.ID, Ref: c.ref, Strength: st.Selector.Strength})
			if p.state(c.meth.Anchor) == Stale {
				stale[c.meth.ID] = true
			}
			continue
		}
		switch {
		case st.Selector == nil:
			lines = append(lines, "  // TODO: no locator declared for this step")
		case st.Selector.Kind == "test-id":
			lines = append(lines, fmt.Sprintf("  await page.getByTestId(%s); // TODO: perform %q [%s]", quote(st.Selector.Value), st.Action, st.Selector.Strength))
		default:
			lines = append(lines, fmt.Sprintf("  await page.getByRole(%s, { name: %s }); // TODO: perform %q [%s]", quote(st.Selector.Value), quote(st.Selector.Name), st.Action, st.Selector.Strength))
		}
	}
	for _, oc := range fl.Outcomes {
		lines = append(lines, fmt.Sprintf("  // TODO: expect outcome %s: %s (%s %s %s)", oc.ID, oc.Behavior, oc.Matcher, oc.Locator, oc.Value))
	}
	lines = append(lines, "});")
	if p.state(fl.Anchor) == Stale {
		stale[fl.ID] = true
	}
	sort.SliceStable(unknowns, func(i, j int) bool { return unknownLess(unknowns[i].(Unknown), unknowns[j].(Unknown)) })
	head := append(p.envelope(ScaffoldSchema, budget), field{"query", query}, field{"status", StatusResolved},
		field{"flow", fl.ID}, field{"flow_anchor", p.anchorView(fl.Anchor)}, field{"proposed_path", proposed}, field{"closest", cv})
	learned, overlayUnknowns := p.learned(stale)
	return render(head, []section{
		{"imports", imports},
		{"lines", lines},
		{"reuse", reuse},
		{"learned", learned},
		{"unknowns", append(unknowns, overlayUnknowns...)},
	}, budget)
}
