package flowcoverage

import (
	"context"
	"fmt"
	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/flowdocs"
	"sort"
	"strings"
)

type Options struct{ Denominator, Receipts, Revision string }

func Compile(ctx context.Context, root string, o Options) (*Result, error) {
	if o.Revision != "" && !wire.IsGitOid(o.Revision) {
		return nil, fmt.Errorf("coverage revision must be a full commit ID")
	}
	if o.Revision == "" {
		o.Revision = "HEAD"
	}
	revision, e := appflows.ResolveRevision(ctx, root, o.Revision)
	if e != nil {
		return nil, e
	}
	a, e := gitauth.Open(root, gitrun.NewDefaultBudget())
	if e != nil {
		return nil, e
	}
	release := a.BeginObjectSession()
	defer release()
	s := &source{ctx: ctx, auth: a, root: root, current: revision, checked: map[string]bool{}}
	s.indexChanged, e = s.indexChanges()
	if e != nil {
		return nil, e
	}
	db, _, e := s.blob(revision, o.Denominator, MaxReportBytes)
	if e != nil {
		return nil, e
	}
	rb, _, e := s.blob(revision, o.Receipts, MaxReportBytes)
	if e != nil {
		return nil, e
	}
	var d Denominator
	var runs Runs
	if e = decode(db, &d); e != nil {
		return nil, e
	}
	if e = decode(rb, &runs); e != nil {
		return nil, e
	}
	if d.Schema != DenominatorSchema || runs.Schema != RunsSchema {
		return nil, fmt.Errorf("unsupported coverage schema")
	}
	s.sourceRevision = d.Source.Revision
	s.repository, e = contextindex.CorpusRepositoryID(ctx, root, d.Source.Revision)
	if e != nil {
		return nil, e
	}
	if d.Source.ID != s.repository {
		return nil, fmt.Errorf("source repository mismatch")
	}
	set, e := appflows.LoadIntentsAt(ctx, root, d.Intents.Directory, d.Intents.Revision)
	if e != nil {
		return nil, e
	}
	r := &Result{Report: Report{Schema: ReportSchema, Revision: revision, DenominatorSHA256: digest(db), RunsSHA256: digest(rb), Passed: true, Rows: []Row{}, Limitations: []string{"Declared-scope completeness only; unretained runs cannot be discovered.", "PROVEN binds immutable source/test/config/package/served-build bytes and caller-declared application identity; no attested deployment lineage or observed clean execution tree.", "Negative control is the accepted declared control observed failing; /3 does not localize its failed assertion or independently establish control adequacy.", "External server cleanup belongs to the caller; runner cleanup alone is observed. Generated documentation cannot accept intent."}}}
	claims := map[string][]doccorpus.Anchor{}
	documentBindings := map[string][]doccorpus.Anchor{}
	documented := map[string]bool{}
	switch d.Inventory.Kind {
	case "generation":
		b, e := s.ref(d.Inventory.Ref, flowdocs.MaxManifestBytes)
		if e != nil {
			return nil, e
		}
		r.Generation, e = flowdocs.Open(ctx, root, b)
		if e != nil {
			return nil, e
		}
		if r.Generation.Manifest.Source != d.Source {
			return nil, fmt.Errorf("generation source mismatch")
		}
		for _, f := range r.Generation.Manifest.Flows {
			documented[f.ID] = true
			for _, p := range f.Paragraphs {
				documentBindings[f.ID] = append(documentBindings[f.ID], p.Anchor)
				documentBindings[f.ID] = append(documentBindings[f.ID], p.Bindings...)
			}
		}
		for _, c := range r.Generation.Manifest.Provider.Claims {
			claims[c.ID] = c.Evidence.Anchors
		}
	case "corpus":
		b, e := s.ref(d.Inventory.Ref, doccorpus.MaxCorpusBytes)
		if e != nil {
			return nil, e
		}
		c, e := doccorpus.Open(ctx, root, b)
		if e != nil {
			return nil, e
		}
		if c.Manifest.Repository != d.Source {
			return nil, fmt.Errorf("corpus source mismatch")
		}
		for _, f := range c.Subjects {
			if f.Kind == "flow" {
				documented[f.ID] = true
				documentBindings[f.ID] = append(documentBindings[f.ID], f.Evidence.Anchors...)
			}
		}
		for _, c := range c.Claims {
			claims[c.ID] = c.Evidence.Anchors
		}
	case "intents":
		if d.Inventory.Path != d.Intents.Directory || d.Inventory.Revision != d.Intents.Revision {
			return nil, fmt.Errorf("intent inventory mismatch")
		}
		b, e := Encode(set.Flows)
		if e != nil || digest(b) != d.Inventory.SHA256 {
			return nil, fmt.Errorf("intent inventory digest mismatch")
		}
		for _, f := range set.Flows {
			documented[f.FlowID] = true
		}
	default:
		return nil, fmt.Errorf("unsupported documented inventory")
	}
	if len(documented) == 0 || len(documented) > 512 || len(d.Flows) != len(documented) || len(d.Variations) > MaxRows {
		return nil, fmt.Errorf("documented inventory missing or exceeds bound")
	}
	flows := map[string]appflows.FlowIntent{}
	vars := map[string]Variation{}
	wanted := map[string]bool{}
	for _, f := range set.Flows {
		flows[f.FlowID] = f
		for _, v := range f.Variations {
			wanted[f.FlowID+":"+v.VariationID] = true
		}
	}
	for _, v := range d.Variations {
		k := v.FlowID + ":" + v.ID
		if !wanted[k] {
			return nil, fmt.Errorf("unknown or duplicate variation")
		}
		delete(wanted, k)
		vars[k] = v
	}
	if len(wanted) > 0 {
		return nil, fmt.Errorf("omitted accepted variation")
	}
	seen := map[string]bool{}
	mapped := map[string]bool{}
	for _, m := range d.Flows {
		if !documented[m.DocumentedID] || seen[m.DocumentedID] {
			return nil, fmt.Errorf("unknown or duplicate documented flow")
		}
		seen[m.DocumentedID] = true
		r.Documented = append(r.Documented, m.DocumentedID)
		local := map[string]bool{}
		for _, id := range m.IntentFlowIDs {
			if _, ok := flows[id]; !ok || local[id] {
				return nil, fmt.Errorf("unknown or duplicate intent mapping")
			}
			local[id] = true
			mapped[id] = true
		}
	}
	for id := range flows {
		if !mapped[id] {
			return nil, fmt.Errorf("accepted flow omitted from mappings")
		}
	}
	observed, e := s.loadRuns(runs)
	if e != nil {
		return nil, e
	}
	inventoryFresh := s.unchanged(revision, o.Denominator) && s.unchanged(revision, o.Receipts) && s.directoryFresh(d.Intents.Revision, d.Intents.Directory) && s.directoryFresh(runs.Revision, runs.Directory)
	if d.Inventory.Kind != "intents" {
		inventoryFresh = inventoryFresh && s.unchanged(d.Inventory.Revision, d.Inventory.Path)
	}
	for _, m := range d.Flows {
		if len(m.IntentFlowIDs) == 0 {
			r.Report.Rows = append(r.Report.Rows, Row{DocumentedID: m.DocumentedID, Status: "MISSING_TEST", Reasons: []string{"accepted variation matrix unavailable"}})
		}
		for _, id := range m.IntentFlowIDs {
			f := flows[id]
			if len(f.Variations) == 0 {
				r.Report.Rows = append(r.Report.Rows, Row{DocumentedID: m.DocumentedID, FlowID: id, Status: "MISSING_TEST", Reasons: []string{"accepted variation matrix empty"}})
			}
			for _, v := range f.Variations {
				row, e := s.evaluate(f, v, vars[id+":"+v.VariationID], observed, claims, flows)
				if e != nil {
					return nil, e
				}
				row.DocumentedID = m.DocumentedID
				for _, anchor := range documentBindings[m.DocumentedID] {
					fresh, err := s.anchor(anchor)
					if err != nil {
						return nil, err
					}
					if !fresh {
						row.Status = "STALE"
						row.Reasons = append(row.Reasons, "documented source binding changed")
					}
				}
				if !inventoryFresh || !s.unchanged(d.Intents.Revision, set.IntentPath(id)) {
					row.Status = "STALE"
					row.Reasons = append(row.Reasons, "denominator, run inventory or accepted intent changed")
				}
				r.Report.Rows = append(r.Report.Rows, row)
			}
		}
	}
	if len(r.Report.Rows) == 0 || len(r.Report.Rows) > MaxRows {
		return nil, fmt.Errorf("coverage row bound exceeded")
	}
	sort.Slice(r.Report.Rows, func(i, j int) bool {
		a, b := r.Report.Rows[i], r.Report.Rows[j]
		return a.DocumentedID+":"+a.FlowID+":"+a.VariationID < b.DocumentedID+":"+b.FlowID+":"+b.VariationID
	})
	sort.Strings(r.Documented)
	for _, row := range r.Report.Rows {
		if row.Status != "PROVEN" && row.Status != "API_PROVEN" && row.Status != "EXCLUDED" {
			r.Report.Gaps++
		}
	}
	r.Report.Passed = r.Report.Gaps == 0
	_, e = Encode(r.Report)
	return r, e
}
func acceptance(a doccorpus.Anchor) bool {
	return a.Authority == "source-document" && a.Kind == "review"
}
func (s *source) evaluate(f appflows.FlowIntent, v appflows.FlowVariation, c Variation, runs []boundRun, claims map[string][]doccorpus.Anchor, flows map[string]appflows.FlowIntent) (Row, error) {
	row := Row{FlowID: f.FlowID, VariationID: v.VariationID, Status: "PROVEN", Actor: f.Actor, Preconditions: append(append([]string{}, f.Preconditions...), v.Preconditions...), Facts: v.ObservableFacts, Citations: c.Citations, Tests: c.Tests, Observations: []Observation{}, Reasons: []string{}}
	missing, stale, flaky := false, false, false
	reason := func(t string) { missing = true; row.Reasons = append(row.Reasons, t) }
	check := func(a doccorpus.Anchor) error {
		fresh, e := s.anchor(a)
		if !fresh {
			stale = true
		}
		return e
	}
	if len(v.Outcomes) == 0 {
		reason("accepted observable outcome matrix empty")
	}
	if f.Proposed {
		reason("intent is proposed, not accepted")
	}
	for _, id := range v.Steps {
		for _, x := range f.Steps {
			if x.StepID == id {
				row.Actions = append(row.Actions, x)
			}
		}
	}
	for _, id := range v.Outcomes {
		for _, x := range f.Outcomes {
			if x.OutcomeID == id {
				row.Outcomes = append(row.Outcomes, x)
			}
		}
	}
	if f.Navigation != nil {
		for _, step := range f.Navigation.Steps {
			for _, id := range v.Steps {
				if step.StepID == id && row.Entry == nil {
					x := step.Locator
					row.Entry = &x
				}
			}
		}
	}
	if row.Entry == nil && c.Entry != nil {
		if !acceptance(c.Entry.Acceptance) {
			return row, fmt.Errorf("entry supplement requires human acceptance anchor")
		}
		if e := check(c.Entry.Acceptance); e != nil {
			return row, e
		}
		if !validEntry(f.Kind, c.Entry.Locator) {
			return row, fmt.Errorf("invalid accepted entry locator")
		}
		row.Entry = &c.Entry.Locator
	}
	if row.Entry == nil {
		reason("accepted entry point unavailable")
	}
	cats := []Citation{c.Citations.Docs, c.Citations.Claims, c.Citations.SourceBranches, c.Citations.LegacyTests, c.Citations.DownstreamFlows}
	for i, x := range cats {
		if len(x.Anchors)+len(x.IDs) == 0 && x.Unavailable == "" {
			return row, fmt.Errorf("citation category needs evidence or explicit unavailable reason")
		}
		if x.Unavailable != "" && (len(x.Anchors)+len(x.IDs) > 0) {
			return row, fmt.Errorf("contradictory citation category")
		}
		for _, a := range x.Anchors {
			if e := check(a); e != nil {
				return row, e
			}
		}
		for _, id := range x.IDs {
			if i == 1 {
				anchors, ok := claims[id]
				if !ok {
					return row, fmt.Errorf("unknown claim citation")
				}
				for _, a := range anchors {
					if e := check(a); e != nil {
						return row, e
					}
				}
			} else if i == 4 {
				if _, ok := flows[id]; !ok {
					return row, fmt.Errorf("unknown downstream flow")
				}
			} else {
				return row, fmt.Errorf("ID citation unsupported in category")
			}
		}
	}
	if len(c.Tests) > 32 {
		return row, fmt.Errorf("variation test bound exceeded")
	}
	if len(c.Tests) == 0 {
		reason("no exact asserting test")
	}
	covered := map[string]bool{}
	mode := ""
	fixtures := []doccorpus.Anchor{}
	pages := []doccorpus.Anchor{}
	for _, t := range c.Tests {
		if len(t.Controls) > 32 || len(t.Assertions) > 256 || len(t.Fixtures) > 256 || len(t.PageObjects) > 256 {
			return row, fmt.Errorf("test binding bound exceeded")
		}
		if t.Mode != "browser" && t.Mode != "api" || t.Mode == "api" && f.Kind != "api" || t.Mode == "browser" && f.Kind != "ui" {
			return row, fmt.Errorf("test mode differs from accepted flow kind")
		}
		if mode != "" && mode != t.Mode {
			reason("mixed browser/API proof contracts")
		}
		mode = t.Mode
		if !acceptance(t.Acceptance) {
			return row, fmt.Errorf("test mapping needs accepted contract anchor")
		}
		if e := check(t.Acceptance); e != nil {
			return row, e
		}
		for _, a := range append(append([]doccorpus.Anchor{}, t.Fixtures...), t.PageObjects...) {
			if e := check(a); e != nil {
				return row, e
			}
		}
		fixtures = append(fixtures, t.Fixtures...)
		pages = append(pages, t.PageObjects...)
		for _, id := range t.Outcomes {
			found := false
			for _, x := range v.Outcomes {
				found = found || id == x
			}
			if !found || covered[id] {
				return row, fmt.Errorf("unknown or repeated outcome mapping")
			}
			covered[id] = true
		}
		groups := map[string]string{}
		all := append([]ExactTest{t.ExactTest}, t.Controls...)
		if len(t.Controls) == 0 {
			reason("required negative control unavailable")
		}
		for i, exact := range all {
			if exact.Project == nil || !safe(exact.File) || exact.FullTitle == "" || exact.Anchor.Path != exact.File || len(exact.Assertions) == 0 {
				return row, fmt.Errorf("test identity/assertion anchors incomplete")
			}
			if i > 0 && testKey(exact) == testKey(t.ExactTest) {
				return row, fmt.Errorf("control must name a distinct exact test")
			}
			for _, a := range append([]doccorpus.Anchor{exact.Anchor}, exact.Assertions...) {
				if e := check(a); e != nil {
					return row, e
				}
			}
			obs := s.observe(exact, runs)
			row.Observations = append(row.Observations, obs...)
			qualified := false
			qualifiedGroups := map[string]string{}
			states := map[string]map[string]bool{}
			historical := false
			for _, o := range obs {
				if o.Historical {
					historical = true
					continue
				}
				if states[o.HistoryGroup] == nil {
					states[o.HistoryGroup] = map[string]bool{}
				}
				states[o.HistoryGroup][o.State] = true
				if o.Attempts > 1 || o.State == "flaky" {
					flaky = true
				}
				expected := "passed"
				if i > 0 {
					expected = "failed"
				}
				if o.Qualified && o.State == expected {
					qualified = true
					qualifiedGroups[o.BindingGroup] = o.SourceRevision
				}
			}
			if i == 0 {
				groups = qualifiedGroups
			} else {
				for group := range groups {
					if _, ok := qualifiedGroups[group]; !ok {
						delete(groups, group)
					}
				}
			}
			for _, groupStates := range states {
				if len(groupStates) > 1 {
					flaky = true
				}
			}
			if !qualified {
				if historical {
					stale = true
				}
				reason("required exact test/control lacks qualified observation: " + exact.File + " › " + exact.FullTitle)
			}
		}
		if len(groups) == 0 {
			reason("subject and controls lack a common immutable execution binding")
		} else {
			keys := []string{}
			for group := range groups {
				keys = append(keys, group)
			}
			sort.Strings(keys)
			row.LastVerifiedRevision = groups[keys[0]]
		}
	}
	for _, id := range v.Outcomes {
		if !covered[id] {
			reason("observable outcome lacks asserting test: " + id)
		}
	}
	excluded := false
	if c.Exclusion != nil {
		ok, fresh, e := s.exclusion(*c.Exclusion, f, v)
		if e != nil {
			return row, e
		}
		excluded = ok
		stale = stale || !fresh
	}
	if mode == "api" {
		row.Status = "API_PROVEN"
	}
	if missing {
		row.Status = "MISSING_TEST"
	}
	if flaky {
		row.Status = "FLAKY"
		row.Reasons = append(row.Reasons, "retained retry or conflicting run history")
	}
	if excluded && !f.Proposed {
		row.Status = "EXCLUDED"
	}
	if stale {
		row.Status = "STALE"
		row.Reasons = append(row.Reasons, "cited immutable input changed")
	}
	if row.Status != "PROVEN" && row.Status != "API_PROVEN" {
		row.LastVerifiedRevision = ""
	}
	if row.Status == "MISSING_TEST" {
		row.Skeleton = &Skeleton{Entry: row.Entry, Outcomes: row.Outcomes, Fixtures: fixtures, PageObjects: pages, Unknown: []string{}}
		if len(fixtures) == 0 {
			row.Skeleton.Unknown = append(row.Skeleton.Unknown, "existing fixtures not supplied")
		}
		if len(pages) == 0 {
			row.Skeleton.Unknown = append(row.Skeleton.Unknown, "existing page objects not supplied")
		}
	}
	return row, nil
}
func (s *source) exclusion(ref Ref, f appflows.FlowIntent, v appflows.FlowVariation) (bool, bool, error) {
	b, e := s.ref(ref, 1<<20)
	if e != nil {
		return false, false, e
	}
	var x struct {
		Schema         string `json:"schema"`
		FlowID         string `json:"flow_id"`
		VariationID    string `json:"variation_id"`
		MatrixSHA256   string `json:"matrix_sha256"`
		SourceRevision string `json:"source_revision"`
		Author         string `json:"author"`
		Rationale      string `json:"rationale"`
		Generated      bool   `json:"generated"`
		Accepted       bool   `json:"accepted"`
	}
	if e = decode(b, &x); e != nil {
		return false, false, e
	}
	matrix, _ := Encode(f)
	if x.Schema != "application-flow-coverage-exclusion/1" || x.FlowID != f.FlowID || x.VariationID != v.VariationID || x.Author == "" || x.Rationale == "" || x.Generated || !x.Accepted {
		return false, false, fmt.Errorf("invalid human exclusion")
	}
	fresh := x.MatrixSHA256 == digest(matrix) && x.SourceRevision == s.sourceRevision && s.unchanged(ref.Revision, ref.Path)
	if _, e := s.auth.CommitTree(s.ctx, x.SourceRevision); e != nil {
		return false, false, e
	}
	return true, fresh, nil
}

func validEntry(kind string, l appflows.NavLocator) bool {
	if kind == "api" {
		return strings.Contains(" GET HEAD POST PUT PATCH DELETE OPTIONS ", " "+l.Method+" ") && strings.HasPrefix(l.Path, "/") && l.Role == "" && l.Name == "" && l.TestID == ""
	}
	return l.Method == "" && l.Path == "" && ((l.Role != "" && l.Name != "" && l.TestID == "") || (l.TestID != "" && l.Role == "" && l.Name == ""))
}
