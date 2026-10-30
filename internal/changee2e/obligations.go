package changee2e

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/appmap"
)

// Obligation states (CCE-V0-004).
const (
	Reached = "REACHED"
	Unknown = "UNKNOWN"
	None    = "NONE"
)

// Obligation is one outcome a diff puts at risk, with the changed span and the chain reaching it.
type Obligation struct {
	ID      string   `json:"id"`
	State   string   `json:"state"`
	Outcome string   `json:"outcome"`
	Screen  string   `json:"screen"`
	Spans   []Span   `json:"spans"`
	Chain   []string `json:"chain"`
}

// UnknownSpan is a changed span whose reach could not be bounded. Reason is one of
// UnknownReasons; Detail names what made it unknown (the impact reason, the map unknown, the
// removed entity, the screen or the dropped obligation) when there is one.
type UnknownSpan struct {
	Span   Span   `json:"span"`
	Reason string `json:"reason"`
	Detail string `json:"detail,omitempty"`
	// Remedy names the declaration change that would bound the span, or states that none is
	// inferred (CCE-V0-150).
	Remedy *appmap.Remedy `json:"remedy,omitempty"`
}

// Unknown-span reasons, the closed CCE-V0-020 set.
const (
	ReasonUnplacedPath         = "unplaced-path"
	ReasonScreenWithoutOutcome = "screen-without-outcome"
	ReasonMapUnknown           = "map-unknown"
	ReasonImpactTruncated      = "impact-truncated"
	ReasonRemovedAtHead        = "removed-at-head"
	ReasonFanOutOverLimit      = "fan-out-over-limit"
	ReasonBlocksNotDeclared    = "blocks-not-declared"
)

// UnknownReasons is the closed CCE-V0-020 set; any other reason is a defect.
var UnknownReasons = []string{ReasonUnplacedPath, ReasonScreenWithoutOutcome, ReasonMapUnknown, ReasonImpactTruncated,
	ReasonRemovedAtHead, ReasonFanOutOverLimit, ReasonBlocksNotDeclared}

// Exemption is a changed path a base `reach_exempt` glob exempted, citing that glob (CCE-V0-020).
type Exemption struct {
	Path   string `json:"path"`
	Glob   string `json:"glob"`
	Reason string `json:"reason"`
}

// Relations names the relations an obligations report consulted and those it did not (CCE-V0-004).
type Relations struct {
	Consulted    []string `json:"consulted"`
	NotConsulted []string `json:"not_consulted"`
}

// Obligations is the e2e-obligations/0 document.
type Obligations struct {
	Schema        string `json:"schema"`
	Base          string `json:"base"`
	Head          string `json:"head"`
	CatalogDigest string `json:"catalog_digest"`
	// Roots are the head map's aliased-root pins (CCE-V0-130); the diff is always --root's.
	Roots        []appmap.RootPin `json:"roots,omitempty"`
	ImpactDigest string           `json:"impact_digest"`
	Relations    Relations        `json:"relations"`
	State        string           `json:"state"`
	Obligations  []Obligation     `json:"obligations"`
	Unknown      []UnknownSpan    `json:"unknown"`
	Exempt       []Exemption      `json:"exempt"`
	// AcceptancesInDiff lists accepted proposals the change itself accepts (CCE-V0-044).
	AcceptancesInDiff []string `json:"acceptances_in_diff,omitempty"`
	// CoverageDigest is the SHA-256 of the runtime coverage document consulted (CCE-V0-045).
	CoverageDigest string `json:"coverage_digest,omitempty"`
	// ModuleGraph records the consulted TJAA module graph (CCE-V0-054).
	ModuleGraph *ModuleGraphRelation `json:"module_graph,omitempty"`
	// LedgerDigest is the digest of the ANL edge ledger bound to the impact relation (CCE-V0-103).
	LedgerDigest string `json:"ledger_digest,omitempty"`
	Truncated    int    `json:"truncated,omitempty"` // obligations dropped by the byte cap (CCE-V0-014)
	Digest       string `json:"digest"`

	catalog *Catalog
	// ledger and routes hold the bound ANL ledger and its proven entry routes (CCE-V0-103/104).
	ledger *appmap.EdgeLedger
	routes map[string]provenRoute
	// baseOutcomes holds each outcome of the base map by ID, for oracle_origin (CCE-V0-016).
	baseOutcomes map[string]baseOutcome
	encoded      []byte
}

// baseOutcome is one outcome declared in the base map; admitted reads the CCE-V0-005/024
// admission rule at base.
type baseOutcome struct {
	admitted                bool
	matcher, locator, value string
	// typed is the outcome's declared role or label locator at base (CCE-V0-109), if any.
	typed *TypedLocator
}

// Encoded returns the sealed bytes of the report.
func (o *Obligations) Encoded() []byte { return o.encoded }

// Catalog returns the head catalog the report was joined against.
func (o *Obligations) Catalog() *Catalog { return o.catalog }

// DiffOptions name a base..head change.
type DiffOptions struct {
	Root, Base, Head string
	Manifest         string
	// Coverage names an optional runtime coverage document that may only widen reach (CCE-V0-045).
	Coverage string
	// ModuleGraph consults the head tree's TypeScript/JavaScript module graph, which may only widen
	// reach (CCE-V0-054).
	ModuleGraph bool
	// Ledger names an optional application-edge-ledger/0 document built from the head map at head;
	// it is bound to the impact relation instead of nil (CCE-V0-103).
	Ledger string
	// Roots binds the manifest's aliased roots for the head and base maps (CCE-V0-130).
	Roots appmap.Roots
	// Endpoints, when set, joins declared server endpoints to their client calls so a changed
	// controller or endpoint declaration reaches the calling screens (CCE-V0-152).
	Endpoints *appmap.EndpointsOptions
}

// EndpointsRelation names the AMAP-V0-220 endpoint join in an obligations report's relations.
const EndpointsRelation = "amap-endpoints"

// maxObligations is MaxObligations; tests lower it to reach the fan-out case.
var maxObligations = MaxObligations

// impactBudget is the impact relation's byte budget; zero reads it in full. Tests lower it to
// reach the truncation case.
var impactBudget = 0

// reasonRank orders the reasons when one span carries several: the report keeps the first, so
// every span has exactly one reason (CCE-V0-020).
var reasonRank = map[string]int{ReasonRemovedAtHead: 0, ReasonImpactTruncated: 1, ReasonFanOutOverLimit: 2, ReasonMapUnknown: 3,
	ReasonScreenWithoutOutcome: 4, ReasonUnplacedPath: 5, ReasonBlocksNotDeclared: 6}

// BuildObligations joins the base..head diff to outcomes through the AMAP/ANL impact relation and
// the head map's flows (CCE-V0-004). A changed screen reaches each declared outcome that a flow
// step on that screen, or any later step of the same flow, expects; the chain is the changed paths,
// the screen and its basis, the nearest step on that screen, the expecting step when it differs,
// and the outcome. Reach that cannot be bounded is UNKNOWN with one CCE-V0-020 reason per span
// (CCE-V0-019): a changed path impact neither places on a screen nor exempts (`unplaced-path`,
// the impact reason as detail when impact listed it unknown); a reached screen no flow step
// expecting an outcome follows (`screen-without-outcome`); a map unknown on a reached screen, in
// a changed file or in a flow through a reached screen, and every map unknown when the report
// would otherwise read NONE (`map-unknown`); any impact omission, which makes every changed path
// unbounded (`impact-truncated`); a screen, flow or outcome of the base map absent from the head
// map (`removed-at-head`); and the obligations past the limit (`fan-out-over-limit`). A changed
// path impact places on no screen is exempt only by a `reach_exempt` glob read at base, and the
// report cites it. NONE is read only when nothing is reached and nothing is unknown.
func BuildObligations(ctx context.Context, o DiffOptions) (*Obligations, error) {
	if o.Root == "" {
		return nil, errNoRoot
	}
	root, err := resolveExisting(o.Root)
	if err != nil {
		return nil, err
	}
	o.Root = root
	cat, err := BuildCatalog(ctx, CatalogOptions{Root: o.Root, Rev: o.Head, Manifest: o.Manifest, Roots: o.Roots})
	if err != nil {
		return nil, err
	}
	base, err := resolveRev(ctx, o.Root, o.Base)
	if err != nil {
		return nil, err
	}
	ledger, err := loadLedger(o.Ledger, cat)
	if err != nil {
		return nil, err
	}
	// The endpoint join reads an ALIAS:PATH input from the bound root at the map's pin (AMAP-V0-222).
	opts := appmap.Options{Root: o.Root, Revision: cat.Revision, Full: impactBudget == 0, Budget: impactBudget, Endpoints: o.Endpoints, Repos: o.Roots.Repos}
	raw, err := appmap.Impact(ctx, cat.m, ledger, base, opts)
	if err != nil {
		return nil, refuse("e2e-impact-unavailable", "%v", err)
	}
	var impact appmap.ImpactReport
	if err := json.Unmarshal(raw, &impact); err != nil {
		return nil, refuse("e2e-impact-unavailable", "%v", err)
	}
	baseDecl, err := readBaseDeclarations(ctx, o.Root, base)
	if err != nil {
		return nil, err
	}
	exemptions, contracts := baseDecl.ReachExempt, baseDecl.APIContracts
	removed, baseOutcomes, err := removedAtHead(ctx, o.Root, base, o.Manifest, o.Roots, cat.m)
	if err != nil {
		return nil, err
	}
	for _, l := range baseDecl.Locators {
		// The effective base locator includes a declared typed override, so adding, changing
		// or removing one changes the oracle (CCE-V0-016, CCE-V0-109).
		if bo, ok := baseOutcomes[l.Block]; ok {
			t := l.TypedLocator
			bo.typed = &t
			baseOutcomes[l.Block] = bo
		}
	}
	inDiff, err := acceptancesInDiff(ctx, o.Root, base, cat)
	if err != nil {
		return nil, err
	}
	consulted := []string{"amap-impact", "amap-map:base", "amap-map:head", "e2e-blocks:reach_exempt@base"}
	notConsulted := []string{"affected-plan"}
	if ledger != nil {
		consulted = append(consulted, LedgerBasis)
	} else {
		notConsulted = append(notConsulted, LedgerBasis)
	}
	var coverageDigest string
	var covered map[string][]string
	if o.Coverage != "" {
		if coverageDigest, covered, err = readCoverage(ctx, o.Root, o.Coverage, cat.Revision, cat.m); err != nil {
			return nil, err
		}
		consulted = append(consulted, CoverageBasis)
	} else {
		notConsulted = append(notConsulted, CoverageBasis)
	}
	if len(contracts) != 0 {
		consulted = append(consulted, APIContractRelation)
	}
	if o.Endpoints != nil {
		consulted = append(consulted, EndpointsRelation)
	} else {
		notConsulted = append(notConsulted, EndpointsRelation)
	}
	r := &Obligations{LedgerDigest: impact.LedgerDigest, CoverageDigest: coverageDigest, AcceptancesInDiff: inDiff, Schema: ObligationsSchema, Base: base, Head: cat.Revision, CatalogDigest: cat.Digest, Roots: cat.Roots, ImpactDigest: impact.Digest,
		Relations:   Relations{Consulted: consulted, NotConsulted: notConsulted},
		Obligations: []Obligation{}, Unknown: []UnknownSpan{}, Exempt: []Exemption{}, catalog: cat, baseOutcomes: baseOutcomes, ledger: ledger}
	if ledger != nil {
		if r.routes, err = provenRoutes(ctx, o.Root, cat, ledger); err != nil {
			return nil, err
		}
	}
	changedRaw, err := git(ctx, o.Root, "diff", "--name-only", "-z", "--no-renames", "--no-ext-diff", base, cat.Revision, "--")
	if err != nil {
		return nil, err
	}
	var changed []string
	changedSet := map[string]bool{}
	for _, p := range strings.Split(string(changedRaw), "\x00") {
		if p != "" {
			changed = append(changed, p)
			changedSet[p] = true
		}
	}
	var pending []pathUnknown // path-level unknowns, expanded to head-side spans below
	var lined []UnknownSpan   // unknowns already located by line
	omitted := 0
	for _, n := range impact.Omitted {
		omitted += n
	}
	placed := map[string]bool{}
	for _, hit := range impact.Screens {
		for _, p := range hit.Paths {
			placed[p] = true
		}
	}
	removedPaths := map[string]bool{}
	for _, rm := range removed {
		pending = append(pending, pathUnknown{rm.path, ReasonRemovedAtHead, rm.entity, remedyNone(ReasonRemovedAtHead)})
		removedPaths[rm.path] = true
	}
	if omitted != 0 {
		keys := make([]string, 0, len(impact.Omitted))
		for k, n := range impact.Omitted {
			if n != 0 {
				keys = append(keys, k+"="+strconv.Itoa(n))
			}
		}
		sort.Strings(keys)
		for _, p := range changed {
			pending = append(pending, pathUnknown{p, ReasonImpactTruncated, "impact omitted " + strings.Join(keys, ","), remedyNone(ReasonImpactTruncated)})
		}
	}
	impactWhy, impactRemedy := map[string]string{}, map[string]*appmap.Remedy{}
	for _, u := range impact.Unknown {
		impactWhy[u.Path], impactRemedy[u.Path] = u.Reason, u.Remedy
	}
	if omitted == 0 {
		// An endpoint-placed path keeps the endpoint join's own unknowns: a caller the join may
		// miss is not bounded by the screens it did place (CCE-V0-152).
		for _, u := range impact.Unknown {
			if placed[u.Path] && strings.HasPrefix(u.Reason, "endpoint-") {
				pending = append(pending, pathUnknown{u.Path, ReasonMapUnknown, u.Reason + ": " + u.Detail, u.Remedy})
			}
		}
	}
	for _, p := range changed {
		if placed[p] || omitted != 0 {
			continue
		}
		if g, ok := exemptGlob(exemptions, p); ok {
			r.Exempt = append(r.Exempt, Exemption{Path: p, Glob: g.Glob, Reason: g.Reason})
			continue
		}
		if removedPaths[p] && impactWhy[p] == "" {
			continue
		}
		pending = append(pending, pathUnknown{p, ReasonUnplacedPath, impactWhy[p], unplacedRemedy(cat.m, p, impactWhy[p], impactRemedy[p], o.Endpoints != nil)})
	}
	reached := map[string]bool{}
	byID := map[string]int{}
	// Runtime coverage hits come after impact's, so an obligation impact reaches keeps its chain;
	// they were not counted as placed above, so every unknown span stands (CCE-V0-046).
	hits := append(append([]appmap.ImpactScreen(nil), impact.Screens...), coverageHits(covered, changedSet, impact.Screens)...)
	if o.ModuleGraph {
		// Module-graph hits come last and, like coverage, were not counted as placed (CCE-V0-054).
		var more []appmap.ImpactScreen
		more, r.ModuleGraph = moduleGraphHits(ctx, o.Root, cat.Revision, cat.m, changed, hits)
		hits = append(hits, more...)
		r.Relations.Consulted = append(r.Relations.Consulted, ModuleGraphBasis)
	} else {
		r.Relations.NotConsulted = append(r.Relations.NotConsulted, ModuleGraphBasis)
	}
	// API contract joins come last and, like the module graph, were not counted as placed
	// (CCE-V0-114, CCE-V0-115).
	more, err := apiContractHits(ctx, o.Root, cat.Revision, cat.m, contracts, changed, hits)
	if err != nil {
		return nil, err
	}
	hits = append(hits, more...)
	for _, hit := range hits {
		reached[hit.ID] = true
		var spans []Span
		for _, p := range hit.Paths {
			ss, err := headSpans(ctx, o.Root, base, cat.Revision, p)
			if err != nil {
				return nil, err
			}
			spans = append(spans, ss...)
		}
		reaches := false
		for _, f := range cat.m.Flows {
			onScreen := "" // the nearest step so far on the changed screen; later steps' outcomes are at risk too
			for _, st := range f.Steps {
				if st.Screen == hit.ID {
					onScreen = st.ID
				}
				if onScreen == "" {
					continue
				}
				chain := []string{"paths:" + strings.Join(hit.Paths, ","), hit.ID + " (" + hit.Basis + ")", onScreen}
				if st.ID != onScreen {
					chain = append(chain, st.ID)
				}
				for _, oc := range cat.expects[st.ID] {
					reaches = true
					id := "obligation:" + f.FlowID + "/" + oc
					if i, ok := byID[id]; ok {
						r.Obligations[i].Spans = append(r.Obligations[i].Spans, spans...)
						continue
					}
					byID[id] = len(r.Obligations)
					r.Obligations = append(r.Obligations, Obligation{ID: id, State: Reached, Outcome: "outcome:" + f.FlowID + "/" + oc,
						Screen: hit.ID, Spans: append([]Span(nil), spans...),
						Chain: append(append([]string(nil), chain...), "outcome:"+f.FlowID+"/"+oc)})
				}
			}
		}
		// A module-graph or API contract hit on a screen without an outcome adds no span, so the
		// changed path's own unknown span keeps its reason (CCE-V0-054, CCE-V0-114).
		if !reaches && !strings.HasPrefix(hit.Basis, ModuleGraphBasis+":") && !strings.HasPrefix(hit.Basis, APIContractBasis+":") {
			for _, s := range spans {
				lined = append(lined, UnknownSpan{Span: s, Reason: ReasonScreenWithoutOutcome, Detail: hit.ID, Remedy: outcomeRemedy(cat.m, hit.ID)})
			}
		}
	}
	for i := range r.Obligations {
		r.Obligations[i].Spans = uniqueSpans(r.Obligations[i].Spans)
	}
	sort.Slice(r.Obligations, func(i, j int) bool { return r.Obligations[i].ID < r.Obligations[j].ID })
	if len(r.Obligations) > maxObligations {
		for _, ob := range r.Obligations[maxObligations:] {
			for _, s := range ob.Spans {
				lined = append(lined, UnknownSpan{Span: s, Reason: ReasonFanOutOverLimit, Detail: ob.ID, Remedy: remedyNone(ReasonFanOutOverLimit)})
			}
		}
		r.Obligations = r.Obligations[:maxObligations]
	}
	for _, p := range pending {
		spans, err := headSpans(ctx, o.Root, base, cat.Revision, p.path)
		if err != nil {
			return nil, err
		}
		for _, s := range spans {
			lined = append(lined, UnknownSpan{Span: s, Reason: p.reason, Detail: p.detail, Remedy: p.remedy})
		}
	}
	otherwiseNone := len(r.Obligations) == 0 && len(lined) == 0
	through := flowsThrough(cat.m, reached)
	manifest := cat.m.Manifest.Path
	for _, u := range cat.m.Unknowns {
		// The diff is --root's, so only a --root unknown can sit in a changed file.
		if !otherwiseNone && !reached[u.Ref] && !(u.Repo == "" && changedSet[u.Path]) && !through[stepFlow(u)] {
			continue
		}
		s := Span{Repo: u.Repo, Path: u.Path, Start: u.Line, End: u.Line}
		if u.Path == "" {
			s = Span{Repo: cat.m.Manifest.Repo, Path: manifest}
		}
		lined = append(lined, UnknownSpan{Span: s, Reason: ReasonMapUnknown, Detail: u.Kind + " " + u.Ref + ": " + u.Reason, Remedy: appmap.UnknownRemedy(cat.m, u)})
	}
	r.Unknown = oneReasonPerSpan(lined)
	for _, u := range r.Unknown {
		if _, ok := reasonRank[u.Reason]; !ok {
			return nil, fmt.Errorf("changee2e: unlisted unknown reason %q", u.Reason)
		}
	}
	switch {
	case len(r.Unknown) != 0:
		r.State = Unknown
	case len(r.Obligations) != 0:
		r.State = Reached
	default:
		r.State = None
	}
	r.encoded, err = capList(&r.Obligations, &r.Truncated, func() ([]byte, error) { return seal(r, &r.Digest) })
	return r, err
}

type pathUnknown struct {
	path, reason, detail string
	remedy               *appmap.Remedy
}

type removal struct{ path, entity string }

// removedAtHead joins the base map: each screen, flow and outcome present at base and absent at head
// is a removal located at its base anchor path (CCE-V0-019). A manifest absent at base has nothing
// to remove; a base map that does not build refuses `e2e-map-unavailable`. It also returns every
// base outcome by ID, which oracle_origin joins (CCE-V0-016).
func removedAtHead(ctx context.Context, root, base, manifest string, roots appmap.Roots, head *appmap.Map) ([]removal, map[string]baseOutcome, error) {
	if manifest == "" {
		manifest = "appmap.json"
	}
	// A manifest read from an aliased root is present at its pin, so the base map must build
	// (CCE-V0-130); only a --root manifest can be absent at base.
	if roots.ManifestRepo == "" {
		if _, _, ok, err := blobAt(ctx, root, base, manifest); err != nil || !ok {
			return nil, nil, err
		}
	}
	bm, err := appmap.BuildRoots(ctx, root, manifest, base, roots)
	if err != nil {
		return nil, nil, refuseMapUnavailable("base: ", err)
	}
	outcomes := map[string]baseOutcome{}
	for _, f := range bm.Flows {
		for _, oc := range f.Outcomes {
			outcomes["outcome:"+f.FlowID+"/"+oc.ID] = baseOutcome{admitted: outcomeAdmissionReason(oc.Matcher, oc.Locator, oc.Value) == "",
				matcher: oc.Matcher, locator: oc.Locator, value: oc.Value}
		}
	}
	have := map[string]bool{}
	for _, s := range head.Screens {
		have[s.ID] = true
	}
	for _, f := range head.Flows {
		have["flow:"+f.FlowID] = true
		for _, oc := range f.Outcomes {
			have["outcome:"+f.FlowID+"/"+oc.ID] = true
		}
	}
	var out []removal
	for _, s := range bm.Screens {
		if !have[s.ID] {
			out = append(out, removal{s.Anchor.Path, s.ID})
		}
	}
	for _, f := range bm.Flows {
		if !have["flow:"+f.FlowID] {
			out = append(out, removal{f.Anchor.Path, "flow:" + f.FlowID})
			continue
		}
		for _, oc := range f.Outcomes {
			if id := "outcome:" + f.FlowID + "/" + oc.ID; !have[id] {
				out = append(out, removal{f.Anchor.Path, id})
			}
		}
	}
	return out, outcomes, nil
}

// oracleOrigin joins a compiled outcome block with the base map (CCE-V0-016): ADMITTED_AT_BASE
// when an admitted base outcome has the same ID, matcher, locator and value; CHANGED_IN_DIFF when
// the ID exists at base otherwise (including an outcome present but unadmitted at base);
// NEW_IN_DIFF when the base map lacks the ID or there is no base map.
func (o *Obligations) oracleOrigin(b *Block) string {
	bo, ok := o.baseOutcomes[b.ID]
	switch {
	case !ok:
		return OriginNewInDiff
	case bo.admitted && bo.matcher == b.Matcher && bo.locator == b.Locator && bo.value == b.Value && sameTyped(bo.typed, b.Typed):
		return OriginAdmittedAtBase
	}
	return OriginChangedInDiff
}

// sameTyped reports whether two typed locators are both absent or equal in form, role and name.
func sameTyped(a, b *TypedLocator) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// readBaseDeclarations reads the declarations file at base, whose `reach_exempt`, `api_contracts`
// and `locators` members a diff cannot rewrite for itself (CCE-V0-020, CCE-V0-114, CCE-V0-109). A
// missing file exempts, joins and overrides nothing.
func readBaseDeclarations(ctx context.Context, root, base string) (declarations, error) {
	data, _, ok, err := blobAt(ctx, root, base, DeclarationsPath)
	if err != nil || !ok {
		return declarations{}, err
	}
	return decodeDeclarations(data)
}

func exemptGlob(list []ReachExemption, p string) (ReachExemption, bool) {
	for _, e := range list {
		if ok, _ := path.Match(e.Glob, p); ok {
			return e, true
		}
	}
	return ReachExemption{}, false
}

// flowsThrough names each flow with a step on a reached screen.
func flowsThrough(m *appmap.Map, reached map[string]bool) map[string]bool {
	out := map[string]bool{}
	for _, f := range m.Flows {
		for _, st := range f.Steps {
			if reached[st.Screen] {
				out[f.FlowID] = true
			}
		}
	}
	return out
}

// stepFlow returns the flow ID of a step unknown (`step:<flow>/<step>`), or "".
func stepFlow(u appmap.Unknown) string {
	if u.Kind != "step" || !strings.HasPrefix(u.Ref, "step:") {
		return ""
	}
	flow, _, _ := strings.Cut(strings.TrimPrefix(u.Ref, "step:"), "/")
	return flow
}

// oneReasonPerSpan sorts the unknown spans and keeps, for each span, the highest-ranked reason.
func oneReasonPerSpan(in []UnknownSpan) []UnknownSpan {
	sort.SliceStable(in, func(i, j int) bool {
		a, b := in[i].Span, in[j].Span
		if a != b {
			return spanLess(a, b)
		}
		if ra, rb := reasonRank[in[i].Reason], reasonRank[in[j].Reason]; ra != rb {
			return ra < rb
		}
		return in[i].Detail < in[j].Detail
	})
	out := []UnknownSpan{}
	for i, u := range in {
		if i == 0 || u.Span != in[i-1].Span {
			out = append(out, u)
		}
	}
	return out
}

var hunkHeader = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

// headSpans returns the head-side line spans of path's change; a deletion-only hunk or a removed
// file is a zero-width span at its head position.
func headSpans(ctx context.Context, root, base, head, path string) ([]Span, error) {
	out, err := git(ctx, root, "diff", "--no-ext-diff", "--no-color", "-U0", base, head, "--", path)
	if err != nil {
		return nil, err
	}
	spans := []Span{}
	for _, line := range strings.Split(string(out), "\n") {
		m := hunkHeader.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		n := 1
		if m[2] != "" {
			n, _ = strconv.Atoi(m[2])
		}
		end := start + n - 1
		if n == 0 {
			end = start
		}
		spans = append(spans, Span{Path: path, Start: start, End: end})
	}
	if len(spans) == 0 {
		spans = append(spans, Span{Path: path})
	}
	return spans, nil
}

// spanLess orders spans by root, path, then lines; --root spans (empty Repo) come first.
func spanLess(a, b Span) bool {
	if a.Repo != b.Repo {
		return a.Repo < b.Repo
	}
	if a.Path != b.Path {
		return a.Path < b.Path
	}
	if a.Start != b.Start {
		return a.Start < b.Start
	}
	return a.End < b.End
}

func uniqueSpans(in []Span) []Span {
	sort.Slice(in, func(i, j int) bool { return spanLess(in[i], in[j]) })
	out := []Span{}
	for i, s := range in {
		if i == 0 || s != in[i-1] {
			out = append(out, s)
		}
	}
	return out
}
