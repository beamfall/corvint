package extevidence

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// SelectionSchema versions the affected-plan test_selection member (ETS-V0-002).
const SelectionSchema = "external-test-selection/0"

// Selection profiles name which verification relation types may narrow a
// selection (ETS-V0-004). No profile admits a context type.
const (
	ProfileStrict   = "strict"
	ProfileCoverage = "coverage"
)

// Selection states (ETS-V0-003). The state is decided from obligations,
// never from how many tests were selected.
const (
	SelectionNarrow  = "narrow-selection-allowed"
	SelectionFull    = "full-relevant-suite-required"
	SelectionBlocked = "blocked"
	SelectionUnknown = "unknown"
)

// The obligation walk is bounded in hops and in entities per record; a walk
// either bound cuts is reported, never silently dropped (ETS-V1-001).
const (
	MaxObligationDepth    = 4
	MaxObligationEntities = 256
)

// confidenceUnscored is reported because no record schema carries a
// confidence; Core never invents one (ETS-V0-007).
const confidenceUnscored = "unscored"

var selectionProfiles = map[string]map[string]struct{}{
	ProfileStrict:   {"verifies": {}, "asserts": {}},
	ProfileCoverage: {"verifies": {}, "asserts": {}, "covers": {}},
}

// contextTypes may widen review context but never qualify (ETS-V0-005).
var contextTypes = map[string]string{"navigates": "navigation-only-evidence", "candidate": "candidate-only-evidence"}

// nonBlocking codes mark weak or context-only evidence: it cannot qualify an
// obligation, and it cannot veto one another relation qualified (ETS-V0-006).
var nonBlocking = map[string]struct{}{
	"navigation-only-evidence": {}, "candidate-only-evidence": {}, "profile-excluded-relation": {},
	"inferred-only-evidence": {}, "generated-only-evidence": {}, "excluded-evidence-kind": {},
}

// weakEvidence codes the evidence kinds that never qualify: a rule-derived
// or model-generated relation is listed, never selected on (ETS-V0-014).
var weakEvidence = map[string]string{EvidenceInferred: "inferred-only-evidence", EvidenceGenerated: "generated-only-evidence"}

var identityCodes = map[string]string{IdentityUnresolved: "unresolved-repository-identity", IdentityAmbiguous: "ambiguous-repository-identity"}

var bindingCodes = map[string]string{
	BindingUnresolved: "unresolved-repository-identity", BindingAmbiguous: "ambiguous-repository-identity",
	BindingMismatch: "repository-binding-mismatch", BindingUnbound: "unbound-test-repository", BindingUnavailable: "repository-unavailable",
}

var freshnessCodes = map[string]string{
	FreshnessEqual: "", FreshnessRevisionUnavailable: "revision-unavailable", FreshnessNotEvaluated: "revision-unavailable",
}

var verificationCodes = map[string]string{
	VerificationVerified: "", VerificationMissing: "missing-path-reference", VerificationDeleted: "missing-path-reference",
	VerificationStale: "stale-path-reference", VerificationNotVerified: "unbound-test-repository",
}

// sourceCodes renames a side code when the side is the verified subject of a
// path-to-path relation rather than its test (EEP-V2-007).
var sourceCodes = map[string]string{"unbound-test-repository": "unbound-source-repository"}

var unknownCodes = map[string]string{
	unknownUnresolved: "unresolved-endpoint", unknownUnsupported: "unsupported-relation", unknownExcluded: "excluded-evidence-kind",
}

// ValidSelectionProfile reports whether name is an accepted profile.
func ValidSelectionProfile(name string) bool {
	_, known := selectionProfiles[name]
	return known
}

// SelectionInput is what the affected plan contributes. Changed is every
// changed root path (plan.dirty); Worktree is its uncommitted subset, which no
// revision-bound record can describe; Incomplete names why the plan scope is
// not bounded; Mandatory is echoed unchanged (ETS-V0-009). CheckoutStatus
// lists a bound checkout's dirty paths; nil leaves every checkout
// uninspected (ETS-V1-005).
type SelectionInput struct {
	Changed        []string
	Worktree       []string
	Incomplete     []string
	Mandatory      []any
	Profile        string
	Limit          int
	CheckoutStatus func(ctx context.Context, dir string) ([]string, error)
}

// obligation is one changed path, affected entity, or widened path that
// narrowing must justify. It is covered only when qualified and never blocked.
// A widened path (EEP-V2-008) carries its record-local repository and path.
type obligation struct {
	provider, subject string
	repository, path  string
	qualified         bool
	blocked           bool
	reasons           map[string]struct{}
}

type selectionRow struct {
	key string
	row map[string]any
}

type selector struct {
	input              SelectionInput
	admitted           map[string]struct{}
	changed, worktree  map[string]struct{}
	paths, entities    map[string]*obligation
	selected, excluded []selectionRow
	blocking           map[string]map[string]any
	unknowns           []selectionRow
	checkouts          map[string]string
	failed             int
}

// Selection compiles the test_selection advice for an affected plan at the
// root commit revision. It reads provider records and Git objects only,
// executes nothing, and never fails: every problem is a structured entry.
func Selection(ctx context.Context, dir, revision string, sources []string, checkouts []Checkout, input SelectionInput) map[string]any {
	root := headRoot(dir, revision)
	root.changed, root.selecting = input.Changed, true
	providers, repository, bound := loadAll(ctx, root, sources, checkouts)
	s := newSelector(input)
	s.inspect(ctx, bound)
	for _, entry := range providers {
		if entry.state != StateLoaded {
			s.failed++
			s.block("provider-"+entry.state, entry.source, "", entry.reason)
			continue
		}
		s.add(entry, entry.viewOf(ctx, root, repository))
	}
	return s.result(providers)
}

func newSelector(input SelectionInput) *selector {
	s := &selector{
		input: input, admitted: selectionProfiles[input.Profile],
		changed: stringSet(input.Changed), worktree: stringSet(input.Worktree),
		paths: map[string]*obligation{}, entities: map[string]*obligation{}, blocking: map[string]map[string]any{},
	}
	for _, path := range input.Changed {
		s.paths[path] = &obligation{subject: path, reasons: map[string]struct{}{}}
	}
	return s
}

// inspect reads each resolved checkout's worktree once. A checkout with any
// dirty path, or one whose status cannot be read, blocks every side it binds
// (ETS-V1-005, ETS-V1-006).
func (s *selector) inspect(ctx context.Context, bound *bindings) {
	s.checkouts = map[string]string{}
	if s.input.CheckoutStatus == nil || bound == nil {
		return
	}
	for id, entry := range bound.checkouts {
		if entry.state != "resolved" {
			continue
		}
		dirty, err := s.input.CheckoutStatus(ctx, entry.dir)
		switch {
		case err != nil:
			s.checkouts[id] = "checkout-worktree-unreadable"
		case len(dirty) != 0:
			s.checkouts[id] = "checkout-worktree-dirty"
		default:
			s.checkouts[id] = ""
		}
	}
}

func stringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

// add evaluates one loaded record: joins map changed paths to entities,
// entity relations widen the obligations transitively, and every verification or
// context relation on an obligation is selected or excluded with a reason.
func (s *selector) add(entry provider, v *view) {
	s.unrooted(v)
	obligations := s.join(entry, v)
	s.walk(v, obligations)
	s.widenPaths(v)
	for id := range obligations {
		s.entity(v.provider, id)
	}
	for _, candidate := range v.links {
		s.test(entry, v, candidate, obligations)
	}
	for _, failure := range v.unknowns {
		s.unknown(v, failure, obligations)
	}
}

// unrooted blocks every changed path when a V1 record cannot say which of its
// repositories is the root: nothing it relates can then be joined, and
// silence would read as absence of evidence (ETS-V0-006).
func (s *selector) unrooted(v *view) {
	if v.repositories == nil || v.primary != "" {
		return
	}
	code := "unbound-root-repository"
	for _, state := range v.repositories {
		if state.identity == IdentityAmbiguous || state.binding == BindingAmbiguous {
			code = "ambiguous-repository-identity"
		}
	}
	for _, path := range s.input.Changed {
		s.record(s.paths[path], code, v.provider, path)
	}
}

// join maps every changed root path a relation names to its entity. Any join
// creates an obligation, whatever its evidence: weak evidence may widen.
func (s *selector) join(entry provider, v *view) map[string]struct{} {
	obligations := map[string]struct{}{}
	for _, candidate := range v.links {
		path, entity, ok := pathAndEntity(candidate)
		if !ok || !v.isChanged(path, s.changed) {
			continue
		}
		obligations[entity] = struct{}{}
		code := s.qualify(entry, v, candidate, path)
		s.record(s.paths[path.path], code, v.provider, path.path)
	}
	return obligations
}

// walk widens the obligations breadth-first over provider-declared relations
// between entities; a context or verification relation between entities is
// not a dependency. A walk cut by the depth or entity bound blocks the
// entities at its edge and reports an unknown, so a cut only widens (ETS-V1-001..003).
func (s *selector) walk(v *view, obligations map[string]struct{}) {
	adjacent := s.adjacency(v)
	frontier := sortedKeys(obligations)
	for depth := 1; ; depth++ {
		next := reach(adjacent, obligations, frontier)
		if len(next) == 0 {
			return
		}
		if depth > MaxObligationDepth {
			s.truncate(v, adjacent, frontier, next, "obligation-depth-truncated", depth-1)
			return
		}
		if len(obligations)+len(next) > MaxObligationEntities {
			s.truncate(v, adjacent, frontier, next, "obligation-budget-exhausted", depth-1)
			return
		}
		for _, id := range next {
			obligations[id] = struct{}{}
		}
		frontier = next
	}
}

// adjacency is the undirected entity graph of v's dependency relations.
func (s *selector) adjacency(v *view) map[string][]string {
	adjacent := map[string][]string{}
	for _, candidate := range v.links {
		if candidate.from.isPath() || candidate.to.isPath() || s.isTestOrContext(candidate) {
			continue
		}
		adjacent[candidate.from.entity] = append(adjacent[candidate.from.entity], candidate.to.entity)
		adjacent[candidate.to.entity] = append(adjacent[candidate.to.entity], candidate.from.entity)
	}
	return adjacent
}

// reach lists, sorted, the entities one hop from frontier not yet obligations.
func reach(adjacent map[string][]string, obligations map[string]struct{}, frontier []string) []string {
	found := map[string]struct{}{}
	for _, id := range frontier {
		for _, neighbour := range adjacent[id] {
			found[neighbour] = struct{}{}
		}
	}
	for id := range obligations {
		delete(found, id)
	}
	return sortedKeys(found)
}

// truncate blocks each frontier entity that has an unwalked neighbour and
// reports the cut as an unknown.
func (s *selector) truncate(v *view, adjacent map[string][]string, frontier, next []string, code string, depth int) {
	cut := stringSet(next)
	for _, id := range frontier {
		if !slices.ContainsFunc(adjacent[id], func(neighbour string) bool { _, open := cut[neighbour]; return open }) {
			continue
		}
		subject := v.provider + ":" + id
		s.record(s.entity(v.provider, id), code, v.provider, subject)
		row := map[string]any{"code": code, "provider": v.provider, "entity": subject, "depth": depth, "unwalked": len(next)}
		s.unknowns = append(s.unknowns, selectionRow{key: v.provider + "\x00" + id + "\x00" + code, row: row})
	}
}

// widenPaths makes the other side of a path-to-path relation on a changed
// root path a path obligation, one hop, as widen does for entities. Only a V2
// record composes such a relation, so V0 and V1 never widen here (EEP-V2-008).
func (s *selector) widenPaths(v *view) {
	for _, candidate := range v.links {
		if !candidate.from.isPath() || !candidate.to.isPath() || s.isTestOrContext(candidate) {
			continue
		}
		s.widenPath(v, candidate.from, candidate.to)
		s.widenPath(v, candidate.to, candidate.from)
	}
}

func (s *selector) widenPath(v *view, changed, other endpoint) {
	if !s.touchesChanged(v, changed) || s.pathObligation(v, other) != nil {
		return
	}
	key := v.provider + "\x00" + other.repository + "\x00" + other.path
	s.paths[key] = &obligation{provider: v.provider, subject: other.repository + ":" + other.path, repository: other.repository, path: other.path, reasons: map[string]struct{}{}}
}

// touchesChanged reports a changed root path, or a directory scope that holds
// one (EEP-V2-012).
func (s *selector) touchesChanged(v *view, side endpoint) bool {
	if v.isChanged(side, s.changed) {
		return true
	}
	for _, path := range s.input.Changed {
		if side.contains(endpoint{repository: v.primary, path: path}) {
			return true
		}
	}
	return false
}

// descendants lists, in key order, the path obligations a directory scope
// holds: changed root paths and this record's widened paths (EEP-V2-012).
func (s *selector) descendants(v *view, scope endpoint) []*obligation {
	if !v.pathToPath || !scope.isScope() {
		return nil
	}
	keys := make([]string, 0, len(s.paths))
	for key := range s.paths {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var out []*obligation
	for _, key := range keys {
		if at, ok := s.paths[key].at(v); ok && scope.contains(at) {
			out = append(out, s.paths[key])
		}
	}
	return out
}

// at is the endpoint a path obligation names in v: a changed root path, or a
// path v's own provider widened to.
func (target *obligation) at(v *view) (endpoint, bool) {
	if target.repository == "" {
		return endpoint{repository: v.primary, path: target.subject}, v.primary != ""
	}
	return endpoint{repository: target.repository, path: target.path}, target.provider == v.provider
}

// pathObligation returns the obligation a path endpoint names, if any: a
// changed root path, or a path an earlier relation widened to.
func (s *selector) pathObligation(v *view, side endpoint) *obligation {
	if target := s.paths[side.path]; target != nil && side.repository == v.primary {
		return target
	}
	return s.paths[v.provider+"\x00"+side.repository+"\x00"+side.path]
}

func (s *selector) isTestOrContext(candidate link) bool {
	_, verifying := verificationTypes[candidate.relation.Type]
	_, context := contextTypes[candidate.relation.Type]
	return verifying || context
}

func (s *selector) entity(provider, id string) *obligation {
	key := provider + ":" + id
	if s.entities[key] == nil {
		s.entities[key] = &obligation{provider: provider, subject: key, reasons: map[string]struct{}{}}
	}
	return s.entities[key]
}

// test evaluates one verification or context relation on an obligation. An
// entity-only relation names no runnable test, so it can never qualify.
func (s *selector) test(entry provider, v *view, candidate link, obligations map[string]struct{}) {
	if !s.isTestOrContext(candidate) {
		return
	}
	if candidate.from.isPath() && candidate.to.isPath() {
		s.pathTest(entry, v, candidate)
		return
	}
	side, entity, ok := pathAndEntity(candidate)
	if !ok {
		s.entityTest(entry, v, candidate, obligations)
		return
	}
	if _, affected := obligations[entity]; !affected {
		return
	}
	code := s.qualify(entry, v, candidate, side)
	s.record(s.entity(v.provider, entity), code, v.provider, v.provider+":"+entity)
	row := s.row(entry, v, candidate, side, entity, code)
	if code == "" {
		s.selected = append(s.selected, row)
		return
	}
	s.excluded = append(s.excluded, row)
}

// pathTest evaluates one path-to-path verification or context relation: from
// is the test, to is the subject it verifies. A directory-scope subject is
// evaluated once per obligation it holds; a test side inside that scope is one
// of them, so only a test side outside it is evaluated against the scope
// itself (EEP-V2-012).
func (s *selector) pathTest(entry provider, v *view, candidate link) {
	if !candidate.to.contains(candidate.from) {
		s.exactPathTest(entry, v, candidate)
	}
	for _, target := range s.descendants(v, candidate.to) {
		s.scopeTest(entry, v, candidate, target)
	}
}

// scopeTest checks the subject side as the held obligation's own path, so
// every covered path passes identity, freshness, verification, and the
// worktree on its own, and gets its own row.
func (s *selector) scopeTest(entry provider, v *view, candidate link, target *obligation) {
	scoped := candidate
	scoped.to, _ = target.at(v)
	code := s.pathCode(entry, v, scoped)
	s.record(target, code, v.provider, target.subject)
	row := s.pathRow(entry, v, scoped, code)
	row.row["subject_scope"] = v.label(candidate.to)
	row.row["reason"] = fmt.Sprintf("%s, covered by directory scope %s", row.row["reason"], v.label(candidate.to))
	s.place(row, code)
}

// exactPathTest folds a relation into every path obligation either side
// names, and qualifies only when both sides pass (EEP-V2-006, EEP-V2-007).
func (s *selector) exactPathTest(entry provider, v *view, candidate link) {
	var targets []*obligation
	for _, side := range []endpoint{candidate.from, candidate.to} {
		if target := s.pathObligation(v, side); target != nil && (len(targets) == 0 || targets[0] != target) {
			targets = append(targets, target)
		}
	}
	if len(targets) == 0 {
		return
	}
	code := s.pathCode(entry, v, candidate)
	for _, target := range targets {
		s.record(target, code, v.provider, target.subject)
	}
	s.place(s.pathRow(entry, v, candidate, code), code)
}

// pathCode applies the relation checks, then the test side, then the subject.
func (s *selector) pathCode(entry provider, v *view, candidate link) string {
	return firstCode(
		func() string { return s.weakCode(candidate.relation.Type, candidate.relation.Evidence) },
		func() string { return unsupportedCode(candidate.relation.Type) },
		func() string { return s.sideCode(entry, v, candidate.from) },
		func() string { return renamed(sourceCodes, s.sideCode(entry, v, candidate.to)) },
	)
}

func (s *selector) place(row selectionRow, code string) {
	if code == "" {
		s.selected = append(s.selected, row)
		return
	}
	s.excluded = append(s.excluded, row)
}

func renamed(names map[string]string, code string) string {
	if name, found := names[code]; found {
		return name
	}
	return code
}

func (s *selector) entityTest(entry provider, v *view, candidate link, obligations map[string]struct{}) {
	if candidate.from.isPath() || candidate.to.isPath() {
		return
	}
	code := s.weakCode(candidate.relation.Type, candidate.relation.Evidence)
	if code == "" {
		code = "entity-test-endpoint"
	}
	for _, id := range []string{candidate.from.entity, candidate.to.entity} {
		if _, affected := obligations[id]; affected {
			s.record(s.entity(v.provider, id), code, v.provider, v.provider+":"+id)
		}
	}
}

// qualify returns "" when the relation meets every ETS-V0-005 condition for
// its path side, else the first failing condition's code in a fixed order.
func (s *selector) qualify(entry provider, v *view, candidate link, side endpoint) string {
	return firstCode(
		func() string { return s.weakCode(candidate.relation.Type, candidate.relation.Evidence) },
		func() string { return unsupportedCode(candidate.relation.Type) },
		func() string { return s.sideCode(entry, v, side) },
	)
}

// sideCode applies the per-path conditions: identity, binding, freshness,
// verification, and the root worktree.
func (s *selector) sideCode(entry provider, v *view, side endpoint) string {
	return firstCode(
		func() string { return identityCode(v, side) },
		func() string { return freshnessCode(entry, v, side) },
		func() string { return verificationCodes[v.verify(side)] },
		func() string { return s.worktreeCode(v, side) },
		func() string { return s.checkoutCode(v, side) },
	)
}

func firstCode(checks ...func() string) string {
	for _, check := range checks {
		if code := check(); code != "" {
			return code
		}
	}
	return ""
}

// weakCode applies conditions 1 and 2. Weak evidence is decided before any
// resolution, so it never blocks whether or not its endpoints resolved.
func (s *selector) weakCode(relationType, evidence string) string {
	if code := contextTypes[relationType]; code != "" {
		return code
	}
	_, verifying := verificationTypes[relationType]
	_, admitted := s.admitted[relationType]
	if verifying && !admitted {
		return "profile-excluded-relation"
	}
	return weakEvidence[evidence]
}

// unsupportedCode refuses a namespaced relation type: its meaning belongs to
// the provider, so Core can neither map nor qualify with it (ETS-V0-005).
func unsupportedCode(relationType string) string {
	if strings.Contains(relationType, ":") {
		return "unsupported-relation"
	}
	return ""
}

// identityCode applies conditions 3 and 4: a V0 record declares no
// repository identity, so its paths can never be bound to one.
func identityCode(v *view, side endpoint) string {
	if v.repositories == nil {
		return "no-repository-identity"
	}
	state := v.repositories[side.repository]
	if code := identityCodes[state.identity]; code != "" {
		return code
	}
	return bindingCodes[state.binding]
}

func freshnessCode(entry provider, v *view, side endpoint) string {
	freshness := entry.freshness
	if v.repositories != nil {
		freshness = v.repositories[side.repository].freshness
	}
	code, known := freshnessCodes[freshness]
	if !known {
		return "stale-provider-revision"
	}
	return code
}

// worktreeCode refuses a root path with uncommitted changes: the record
// describes a commit, never the working tree.
func (s *selector) worktreeCode(v *view, side endpoint) string {
	if side.repository != v.primary {
		return ""
	}
	if _, dirty := s.worktree[side.path]; dirty {
		return "worktree-dirty-path"
	}
	return ""
}

// checkoutCode refuses a side bound to a checkout whose worktree was dirty or
// unreadable: the record describes its commit, not those changes (ETS-V1-005).
func (s *selector) checkoutCode(v *view, side endpoint) string {
	state := v.repositories[side.repository]
	if state == nil || state.binding != BindingCheckout {
		return ""
	}
	return s.checkouts[state.declared.ID]
}

// inspected drops checkout-worktree-not-inspected once every checkout side of
// a row had its worktree read (ETS-V1-007).
func (s *selector) inspected(limits []any, v *view, sides ...endpoint) []any {
	for _, side := range sides {
		state := v.repositories[side.repository]
		if state == nil || state.binding != BindingCheckout {
			continue
		}
		if _, read := s.checkouts[state.declared.ID]; !read {
			return limits
		}
	}
	return slices.DeleteFunc(limits, func(limit any) bool { return limit == "checkout-worktree-not-inspected" })
}

// record folds one evaluated relation into an obligation.
func (s *selector) record(target *obligation, code, provider, subject string) {
	if code == "" {
		target.qualified = true
		return
	}
	target.reasons[code] = struct{}{}
	if _, weak := nonBlocking[code]; weak {
		return
	}
	target.blocked = true
	s.block(code, provider, subject, "")
}

func (s *selector) block(code, provider, subject, detail string) {
	key := code + "\x00" + provider + "\x00" + subject + "\x00" + detail
	row := map[string]any{"code": code, "provider": provider}
	optional := map[string]string{"subject": subject, "detail": detail}
	for name, value := range optional {
		if value != "" {
			row[name] = value
		}
	}
	s.blocking[key] = row
}

// unknown folds a relation composition could not resolve. It touches the
// selection when either raw endpoint names a changed root path or an
// obligation; an unresolved or unsupported one then blocks what it touches.
func (s *selector) unknown(v *view, failure unknown, obligations map[string]struct{}) {
	targets := s.touched(v, failure, obligations)
	if len(targets) == 0 {
		return
	}
	code := unknownCodes[failure.state]
	if weak := s.weakCode(failure.relation.Type, failure.relation.Evidence); weak != "" && failure.state != unknownExcluded {
		code = weak
	}
	for _, target := range targets {
		s.record(target, code, v.provider, target.subject)
	}
	row := failure.toMap()
	row["code"] = code
	s.unknowns = append(s.unknowns, selectionRow{key: v.provider + "\x00" + failure.relation.From + "\x00" + failure.relation.To + "\x00" + failure.relation.Type, row: row})
}

// touched lists the obligations an unknown's raw endpoints name: a changed or
// widened path, or an affected entity of the record's own provider.
func (s *selector) touched(v *view, failure unknown, obligations map[string]struct{}) []*obligation {
	var targets []*obligation
	for _, raw := range rawEndpoints(failure) {
		side := endpoint{repository: raw.Repository, path: raw.Path}
		if target := s.pathObligation(v, side); raw.Path != "" && target != nil {
			targets = append(targets, target)
		}
		targets = append(targets, s.descendants(v, side)...)
		if _, affected := obligations[raw.Entity]; raw.Entity != "" && affected && raw.Provider == v.provider {
			targets = append(targets, s.entity(v.provider, raw.Entity))
		}
	}
	return targets
}

// rawEndpoints reads an unknown's endpoints in the V1 form; a V0 endpoint is
// `path:P` or `provider:entity` in the one root repository.
func rawEndpoints(failure unknown) []Endpoint1 {
	if failure.structured != nil {
		return []Endpoint1{failure.structured.From, failure.structured.To}
	}
	out := make([]Endpoint1, 0, 2)
	for _, raw := range []string{failure.relation.From, failure.relation.To} {
		if path, isPath := strings.CutPrefix(raw, "path:"); isPath {
			out = append(out, Endpoint1{Path: path})
			continue
		}
		provider, id, _ := strings.Cut(raw, ":")
		out = append(out, Endpoint1{Provider: provider, Entity: id})
	}
	return out
}

// row is one selected or excluded test with its full provenance (ETS-V0-007).
func (s *selector) row(entry provider, v *view, candidate link, side endpoint, entity, code string) selectionRow {
	out := map[string]any{
		"authority": Authority, "confidence": confidenceUnscored,
		"provider": v.provider, "provider_revision": entry.providerRevision(),
		"entity": v.provider + ":" + entity, "entity_kind": v.entities[entity].Kind,
		"test":          map[string]any{"path": side.path},
		"relation":      relationMap(candidate.relation, candidate.structured),
		"relation_type": candidate.relation.Type, "evidence": candidate.relation.Evidence,
		"verification": v.verify(side), "limitations": s.inspected(limitations(v, side), v, side),
	}
	s.provenance(out, entry, v, side)
	if code == "" {
		out["reason"] = fmt.Sprintf("%s %s relation from %s to affected entity %s:%s meets every qualifying condition", candidate.relation.Evidence, candidate.relation.Type, v.label(side), v.provider, entity)
	} else {
		_, weak := nonBlocking[code]
		out["code"], out["blocking"] = code, !weak
		out["reason"] = fmt.Sprintf("%s %s relation from %s to affected entity %s:%s does not qualify: %s", candidate.relation.Evidence, candidate.relation.Type, v.label(side), v.provider, entity, code)
	}
	key := strings.Join([]string{v.provider + ":" + entity, side.repository, side.path, candidate.relation.Type, candidate.relation.Evidence, candidate.relation.From, candidate.relation.To, code, entry.source}, "\x00")
	return selectionRow{key: key, row: out}
}

// provenance adds the repository, revision, freshness, and binding evidence of
// the test side; a V0 record has one provider-wide freshness and no identity.
func (s *selector) provenance(out map[string]any, entry provider, v *view, side endpoint) {
	if v.repositories == nil {
		out["schema"], out["freshness"] = Schema, entry.freshness
		out["source_revision"], out["test_revision"] = entry.record.Repository.Revision, entry.record.Repository.Revision
		return
	}
	state := v.repositories[side.repository]
	out["schema"] = entry.record1.Schema
	out["test"].(map[string]any)["repository"] = side.repository
	out["identity"], out["binding"], out["freshness"] = state.identity, state.binding, state.freshness
	out["test_revision"] = state.declared.Revision
	if primary := v.repositories[v.primary]; primary != nil {
		out["source_revision"] = primary.declared.Revision
	}
	if state.captured != "" {
		out["captured_revision"] = state.captured
	}
	if side.blob != "" {
		out["test"].(map[string]any)["blob"] = side.blob
	}
	out["relation_state"] = worse(RelationFresh, sideState(state, v.verify(side)))
	out["crosses_repositories"] = side.repository != v.primary
}

// pathRow is one selected or excluded path-to-path test: both sides carry
// their own repository evidence, and the relation state is the worse side
// (EEP-V2-009).
func (s *selector) pathRow(entry provider, v *view, candidate link, code string) selectionRow {
	test, subject := candidate.from, candidate.to
	testState, subjectState := v.repositories[test.repository], v.repositories[subject.repository]
	testVerification, subjectVerification := v.verify(test), v.verify(subject)
	out := map[string]any{
		"authority": Authority, "confidence": confidenceUnscored, "schema": entry.record1.Schema,
		"provider": v.provider, "provider_revision": entry.providerRevision(),
		"test":          testState.endpointMap(test, testVerification),
		"subject":       subjectState.endpointMap(subject, subjectVerification),
		"relation":      relationMap(candidate.relation, candidate.structured),
		"relation_type": candidate.relation.Type, "evidence": candidate.relation.Evidence,
		"verification": testVerification, "limitations": s.inspected(pathLimitations(v, candidate), v, candidate.from, candidate.to),
		"identity": testState.identity, "binding": testState.binding, "freshness": testState.freshness,
		"test_revision": testState.declared.Revision, "source_revision": subjectState.declared.Revision,
		"relation_state":       worse(sideState(testState, testVerification), sideState(subjectState, subjectVerification)),
		"crosses_repositories": test.repository != v.primary || subject.repository != v.primary,
	}
	if testState.captured != "" {
		out["captured_revision"] = testState.captured
	}
	if code == "" {
		out["reason"] = fmt.Sprintf("%s %s path-to-path relation from test %s to %s meets every qualifying condition on both sides", candidate.relation.Evidence, candidate.relation.Type, v.label(test), v.label(subject))
	} else {
		_, weak := nonBlocking[code]
		out["code"], out["blocking"] = code, !weak
		out["reason"] = fmt.Sprintf("%s %s path-to-path relation from test %s to %s does not qualify: %s", candidate.relation.Evidence, candidate.relation.Type, v.label(test), v.label(subject), code)
	}
	key := strings.Join([]string{"path:" + v.label(subject), test.repository, test.path, candidate.relation.Type, candidate.relation.Evidence, candidate.relation.From, candidate.relation.To, code, entry.source}, "\x00")
	return selectionRow{key: key, row: out}
}

func limitations(v *view, side endpoint) []any {
	out := []any{"not-coverage-proof", "not-executed"}
	if v.repositories != nil && v.repositories[side.repository].binding == BindingCheckout {
		out = append(out, "checkout-worktree-not-inspected")
	}
	return out
}

func (entry provider) providerRevision() string {
	if entry.record1 != nil {
		return entry.record1.Provider.Revision
	}
	return entry.record.Provider.Revision
}

// state applies the ETS-V0-003 precedence: a failed provider blocks, an
// incomplete or empty plan is unknown, any open obligation or blocking reason
// requires the full relevant suite, and only then may selection narrow.
func (s *selector) state() (string, string) {
	uncoveredPaths, uncoveredEntities := s.uncovered(s.paths), s.uncovered(s.entities)
	switch {
	case s.failed > 0:
		return SelectionBlocked, "provider-unavailable"
	case len(s.input.Incomplete) > 0:
		return SelectionUnknown, "incomplete-affected-scope"
	case len(s.input.Changed) == 0:
		return SelectionUnknown, "no-changed-paths"
	case len(uncoveredPaths)+len(uncoveredEntities)+len(s.blocking) > 0:
		return SelectionFull, "open-obligations"
	}
	return SelectionNarrow, "every-obligation-qualified"
}

func (s *selector) uncovered(obligations map[string]*obligation) []selectionRow {
	var out []selectionRow
	for key, target := range obligations {
		if target.qualified && !target.blocked {
			continue
		}
		if len(target.reasons) == 0 {
			target.reasons["no-external-evidence"] = struct{}{}
		}
		reasons := make([]string, 0, len(target.reasons))
		for code := range target.reasons {
			reasons = append(reasons, code)
		}
		sort.Strings(reasons)
		row := map[string]any{"reasons": toAny(reasons), "blocked": target.blocked}
		if target.repository != "" {
			row["provider"], row["repository"], row["path"] = target.provider, target.repository, target.path
		} else if target.provider == "" {
			row["path"] = target.subject
		} else {
			row["provider"], row["entity"] = target.provider, target.subject
		}
		out = append(out, selectionRow{key: key, row: row})
	}
	return out
}

func (s *selector) result(providers []provider) map[string]any {
	state, reason := s.state()
	limit := max(s.input.Limit, 0)
	lists := map[string][]selectionRow{
		"selected": s.selected, "candidates": s.excluded, "uncovered_paths": s.uncovered(s.paths),
		"uncovered_entities": s.uncovered(s.entities), "blocking_reasons": blockingRows(s.blocking), "unknowns": s.unknowns,
	}
	out := map[string]any{
		"schema": SelectionSchema, "profile": s.input.Profile, "admitted_types": toAny(sortedKeys(s.admitted)),
		"state": state, "state_reason": reason, "authority": Authority,
		"mandatory": s.mandatory(), "provider_evidence": providerRows(providers),
		"scope": toAny(append([]string{}, s.input.Incomplete...)),
		"note":  "advice only; no test was executed; mandatory checks stay required; an empty or narrow selection is never proof that other tests are unneeded",
		"untrusted_text_fields": toAny([]string{
			"test_selection.selected[].relation.rule", "test_selection.selected[].relation.reference",
			"test_selection.candidates[].relation.rule", "test_selection.candidates[].relation.reference",
		}),
	}
	omitted := map[string]any{}
	for name, rows := range lists {
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].key < rows[j].key })
		kept := min(len(rows), limit)
		values := make([]any, 0, kept)
		for _, entry := range rows[:kept] {
			values = append(values, entry.row)
		}
		out[name], omitted[name] = values, len(rows)-kept
	}
	out["omitted"] = omitted
	return out
}

func (s *selector) mandatory() []any {
	if s.input.Mandatory == nil {
		return []any{}
	}
	return s.input.Mandatory
}

func blockingRows(rows map[string]map[string]any) []selectionRow {
	out := make([]selectionRow, 0, len(rows))
	for key, row := range rows {
		out = append(out, selectionRow{key: key, row: row})
	}
	return out
}

func sortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func toAny(values []string) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}
