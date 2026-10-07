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
	// Freshness is the method anchor's state; a STALE method is listed but never called.
	Freshness string `json:"freshness"`
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
		if st.Selector == nil {
			continue // a step with no locator has nothing to reuse, whatever the map claims
		}
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
	// A method whose anchor changed may no longer exist or do what the step needs, and a method
	// that takes parameters needs arguments the flow does not supply, so neither is called; the
	// step falls back to a TODO and the skipped reuse is reported.
	type skippedCall struct {
		call
		reason, note string
	}
	skipped := map[string]skippedCall{}
	for id, c := range calls {
		switch {
		case p.state(c.meth.Anchor) == Stale:
			skipped[id] = skippedCall{c, "stale-reuse", "is STALE at the evaluated revision"}
		case !c.meth.NoArgs:
			skipped[id] = skippedCall{c, "reuse-takes-arguments", "takes arguments the flow does not supply"}
		default:
			continue
		}
		delete(calls, id)
	}
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
	// Page objects the steps reuse but the closest spec does not import. Every name the scaffold
	// binds (a borrowed import, a generated import, a page-object variable, page) is reserved for
	// one file; a file whose class or variable name is already taken is not called at all, and its
	// steps fall back to TODO lines, rather than emitting code that rebinds a name.
	vars := map[string]string{}
	order := []string{}
	blocked := map[string]bool{}
	owner := map[string]string{"page": "", "test": "", "expect": ""}
	for n := range locals {
		owner[n] = ""
	}
	for _, f := range sortedKeys(imported) {
		for n := range imported[f] {
			if _, taken := owner[n]; !taken || owner[n] == "" {
				owner[n] = f
			}
		}
	}
	collide := func(f *TestFile, name string) {
		blocked[f.Path] = true
		imports = append(imports, "// UNRESOLVED import { "+f.Class+" } collides with the binding "+name+";")
		unknowns = append(unknowns, Unknown{Kind: "scaffold-import", Ref: f.ID, Reason: "binding-collision", Path: f.Path})
	}
	for _, st := range fl.Steps {
		c, ok := calls[st.ID]
		if !ok || blocked[c.file.Path] {
			continue
		}
		if _, seen := vars[c.file.Path]; seen {
			continue
		}
		f := c.file
		if o, taken := owner[f.Class]; taken && o != f.Path {
			collide(f, f.Class)
			continue
		}
		v := lowerFirst(f.Class)
		if _, taken := owner[v]; taken || v == f.Class {
			collide(f, v)
			continue
		}
		if !imported[f.Path][f.Class] {
			// Not bound under its own name: absent, aliased (import { X as Y }), default or namespace.
			if dir == "" {
				imports = append(imports, "// UNRESOLVED import { "+f.Class+" } from <no proposed path>;")
			} else {
				spec, _ := relSpecifier(dir, f.Path)
				imports = append(imports, "import { "+f.Class+" } from "+quote(spec)+";")
			}
		}
		owner[f.Class], owner[v] = f.Path, f.Path
		vars[f.Path] = v
		order = append(order, f.Path)
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
		if c, ok := calls[st.ID]; ok && !blocked[c.file.Path] {
			lines = append(lines, fmt.Sprintf("  await %s.%s(); // reuse %s [%s]", vars[c.file.Path], c.meth.Name, c.ref, st.Selector.Strength))
			reuse = append(reuse, reuseView{Step: st.ID, Method: c.meth.ID, Ref: c.ref, Strength: st.Selector.Strength, Freshness: p.state(c.meth.Anchor)})
			continue
		}
		if c, ok := skipped[st.ID]; ok {
			fresh := p.state(c.meth.Anchor)
			lines = append(lines, fmt.Sprintf("  // reuse %s %s; not called", c.ref, c.note))
			reuse = append(reuse, reuseView{Step: st.ID, Method: c.meth.ID, Ref: c.ref, Strength: st.Selector.Strength, Freshness: fresh})
			unknowns = append(unknowns, Unknown{Kind: "scaffold-reuse", Ref: c.meth.ID, Reason: c.reason, Path: c.file.Path, Line: c.meth.Anchor.Start})
			if fresh == Stale {
				stale[c.meth.ID] = true
			}
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
