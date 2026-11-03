package funclink

import (
	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/appmap"
)

// Unit identities follow the application map: "flow:<flow_id>", "step:<flow_id>/<step_id>",
// "screen:<app>:<state>", and the SKH-V0 page-object, method and setup record IDs.
func flowRef(flowID string) Ref { return Ref{KindFlow, "flow:" + flowID} }
func stepRef(flowID, stepID string) Ref {
	return Ref{KindStep, "step:" + flowID + "/" + stepID}
}

// anchorSpan is an anchor's span; an aliased root's path is written ROOT:PATH, the AFU-V1-071
// qualified form.
func anchorSpan(a appmap.Anchor) Span {
	p := a.Path
	if a.Repo != "" {
		p = a.Repo + ":" + p
	}
	return Span{Path: p, Start: a.Start, End: a.End}
}

// FromIntents offers each flow intent's declared AFU-V1 links: a source target joins the code
// set and a test target the test set of the flow and, when the link starts at a step, of that
// step. The intent file is the flow's declaration surface, used by the flow's declared tests.
// Inferred links are not declared and are not read.
func (b *Builder) FromIntents(set appflows.IntentSet) {
	for _, f := range set.Flows {
		fr := flowRef(f.FlowID)
		b.Unit(fr)
		steps := map[string]bool{}
		for _, s := range f.Steps {
			steps[s.StepID] = true
			b.Unit(stepRef(f.FlowID, s.StepID))
		}
		var tests []string
		for _, l := range f.Links {
			if l.Basis != "declared" {
				continue
			}
			refs := []Ref{fr}
			if steps[l.From] {
				refs = append(refs, stepRef(f.FlowID, l.From))
			}
			for _, r := range refs {
				switch l.Target.Type {
				case "source":
					b.AddCode(r, Span{Path: l.Target.Path, Start: l.Target.StartLine, End: l.Target.EndLine}, Declared)
				case "test":
					b.AddTest(r, l.Target.TestKey, Declared)
				}
			}
			if l.Target.Type == "test" {
				tests = append(tests, l.Target.TestKey)
			}
		}
		if set.Dir != "" {
			b.AddSurface(SurfaceFlow, fr.ID, fr, Span{Path: set.IntentPath(f.FlowID)}, tests, Declared)
		}
	}
}

// FromMap offers the application map's links: each screen's router declaration span as static
// code, and each flow step's declared test links (RVN-V0-002) as declared tests of the step and its
// flow. A flow's anchor is its declaration surface.
func (b *Builder) FromMap(m *appmap.Map) {
	if m == nil {
		return
	}
	for _, s := range m.Screens {
		r := Ref{KindScreen, s.ID}
		b.Unit(r)
		if s.Anchor.Path != "" {
			b.AddCode(r, anchorSpan(s.Anchor), Static)
		}
	}
	for _, f := range m.Flows {
		fr := flowRef(f.FlowID)
		b.Unit(fr)
		var tests []string
		for _, st := range f.Steps {
			sr := Ref{KindStep, st.ID}
			b.Unit(sr)
			for _, t := range st.Tests {
				b.AddTest(sr, t, Declared)
				b.AddTest(fr, t, Declared)
				tests = append(tests, t)
			}
		}
		if f.Anchor.Path != "" {
			b.AddSurface(SurfaceFlow, fr.ID, fr, anchorSpan(f.Anchor), tests, Declared)
		}
	}
}

// FromImpact offers the AMAP-V0 impact join's route-to-component and handler links: every path
// the report places on a screen joins that screen's code set as a whole file, static.
func (b *Builder) FromImpact(rep *appmap.ImpactReport) {
	if rep == nil {
		return
	}
	for _, s := range rep.Screens {
		for _, p := range s.Paths {
			b.AddCode(Ref{KindScreen, s.ID}, Span{Path: p}, Static)
		}
	}
}

func isModuleForm(form string) bool {
	return form == appmap.FormPlaywrightModule || form == appmap.FormCypressModule
}

func sameSelector(a, b *appmap.Selector) bool {
	if a == nil || b == nil {
		return false
	}
	if a.ID != "" && b.ID != "" {
		return a.ID == b.ID
	}
	return a.Kind == b.Kind && a.Value == b.Value
}

// FromSiteKnowledge offers the SKH-V0 static links. A page-object method's tests are the tests
// with a step whose call chain (SKStep.Via) holds the method, and the users of the method's step
// record (SKRecord.Tests). A binding's tests are the tests with a step on that locator through a
// method of its page object. A page object's tests are its methods' tests, and a page object
// joined to a screen gives the screen those tests. When m is given, a screen's attributed spec
// files give it the tests read from those files. All of it is static authority.
func (b *Builder) FromSiteKnowledge(k *appmap.SKModel, m *appmap.Map) {
	if k == nil {
		return
	}
	callers := map[string]map[string]bool{}
	add := func(method, test string) {
		if callers[method] == nil {
			callers[method] = map[string]bool{}
		}
		callers[method][test] = true
	}
	byPath := map[string][]string{}
	for _, t := range k.Tests {
		byPath[t.Path] = append(byPath[t.Path], t.ID)
		for _, s := range t.Steps {
			for _, v := range s.Via {
				add(v, t.ID)
			}
		}
	}
	for _, r := range k.Records {
		if r.Source == "test-body" {
			continue
		}
		for _, t := range r.Tests {
			add(r.Source, t)
		}
	}
	bindingUsers := bindingUsers(k)
	for _, po := range k.PageObjects {
		pr := Ref{KindPageObject, po.ID}
		b.Unit(pr)
		all := map[string]bool{}
		for _, meth := range po.Methods {
			mr := Ref{KindMethod, meth.ID}
			kind := SurfaceMethod
			if isModuleForm(po.Form) {
				kind = SurfaceStepHelper
			}
			tests := sortedSet(callers[meth.ID])
			b.AddSurface(kind, meth.ID, mr, anchorSpan(meth.Anchor), tests, Static)
			for _, t := range tests {
				all[t] = true
			}
		}
		for i, bind := range po.Bindings {
			users := bindingUsers[bindingKey{po.ID, i}]
			for t := range users {
				all[t] = true
			}
			b.AddSurface(SurfaceBinding, po.ID+"#"+bind.Name, pr, anchorSpan(bind.Anchor), sortedSet(users), Static)
		}
		tests := sortedSet(all)
		b.AddSurface(SurfacePageObject, po.ID, pr, anchorSpan(po.Anchor), tests, Static)
		if po.Join == appmap.StatusResolved && po.Screen != "" {
			sr := Ref{KindScreen, po.Screen}
			b.Unit(sr)
			for _, t := range tests {
				b.AddTest(sr, t, Static)
			}
		}
	}
	if m == nil {
		return
	}
	for _, s := range m.Screens {
		for _, a := range s.Specs {
			for _, t := range byPath[a.File] {
				b.AddTest(Ref{KindScreen, s.ID}, t, Static)
			}
		}
	}
}

type bindingKey struct {
	po    string
	index int
}

// bindingUsers maps each page-object binding to the tests with a step on its locator. A step whose
// call chain names a method of a page object holding a binding on that locator belongs to those
// bindings only. Any other step on the locator, such as a direct `login.email.fill(...)` from a
// test body or helper, cannot be told apart, so it counts as a user of every binding on that
// locator: a binding change may select a test too many, never one too few.
func bindingUsers(k *appmap.SKModel) map[bindingKey]map[string]bool {
	methods := make([]map[string]bool, len(k.PageObjects))
	for i, po := range k.PageObjects {
		methods[i] = map[string]bool{}
		for _, m := range po.Methods {
			methods[i][m.ID] = true
		}
	}
	users := map[bindingKey]map[string]bool{}
	for _, t := range k.Tests {
		for _, s := range t.Steps {
			if s.Locator == nil {
				continue
			}
			var matching, owned []bindingKey
			for i, po := range k.PageObjects {
				for j := range po.Bindings {
					if !sameSelector(s.Locator, &po.Bindings[j].Locator) {
						continue
					}
					key := bindingKey{po.ID, j}
					matching = append(matching, key)
					if viaAny(s.Via, methods[i]) {
						owned = append(owned, key)
					}
				}
			}
			if len(owned) == 0 {
				owned = matching
			}
			for _, key := range owned {
				if users[key] == nil {
					users[key] = map[string]bool{}
				}
				users[key][t.ID] = true
			}
		}
	}
	return users
}

func viaAny(via []string, set map[string]bool) bool {
	for _, v := range via {
		if set[v] {
			return true
		}
	}
	return false
}

// FromSetup offers each SKH-V0 setup record as a setup unit whose surface is its span and whose
// tests are the tests that use it (SKSetupUse, SKH-V0-031), static.
func (b *Builder) FromSetup(records []appmap.SKSetup, uses []appmap.SKSetupUse) {
	users := map[string]map[string]bool{}
	for _, u := range uses {
		for _, id := range u.Uses {
			if users[id] == nil {
				users[id] = map[string]bool{}
			}
			users[id][u.Test] = true
		}
	}
	for _, rec := range records {
		r := Ref{KindSetup, rec.ID}
		span := anchorSpan(rec.Anchor)
		if span.Path == "" {
			span = Span{Path: rec.Path}
		}
		b.AddSurface(SurfaceSetup, rec.ID, r, span, sortedSet(users[rec.ID]), Static)
	}
}

// ObservedLink is one unit's observed links (MCJ-V0-020 tier 3): the product-side spans its bound
// segments reached and the tests owning those segments.
type ObservedLink struct {
	Unit  Ref
	Code  []Span
	Tests []string
}

// ObservedSource yields observed links from the linecov segment join (MCJ-V0-002, MCJ-V0-004).
// That join is built separately; until it exists a caller passes nil or a fixture.
type ObservedSource interface {
	ObservedLinks() ([]ObservedLink, error)
}

// FromObserved offers the observed links of src; a nil source offers none. An error leaves the
// builder without any of src's links.
func (b *Builder) FromObserved(src ObservedSource) error {
	if src == nil {
		return nil
	}
	links, err := src.ObservedLinks()
	if err != nil {
		return err
	}
	for _, l := range links {
		b.Unit(l.Unit)
		for _, s := range l.Code {
			b.AddCode(l.Unit, s, Observed)
		}
		for _, t := range l.Tests {
			b.AddTest(l.Unit, t, Observed)
		}
	}
	return nil
}
