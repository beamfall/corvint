package appflows

import (
	"context"
	"errors"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/extevidence"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

// The e2e-safe profile of `corvint affected --selection-profile` (AFU-V1-019..024).
const (
	E2ESafeProfile          = "e2e-safe"
	E2ESafeSchema           = "e2e-safe-selection/0"
	SelectionProviderSchema = "application-flow-selection-provider/1"
	CoverageSchema          = "application-flow-coverage/0"
	BasisCoverage           = "coverage"
	BasisReviewedLinks      = "reviewed-links"
	maxSelectionTests       = 1024
	maxSelectionPaths       = 4096
)

// The closed fallback codes (AFU-V1-022). Any one of them yields the full relevant suite.
const (
	CodeUnmappedChange      = "e2e-unmapped-change"
	CodeInventoryIncomplete = "e2e-inventory-incomplete"
	CodeExclusionUnproven   = "e2e-exclusion-unproven"
	CodeGlobalPathChanged   = "e2e-global-path-changed"
	CodeMapStale            = "e2e-map-stale"
	CodeInferredLinkOnly    = "e2e-inferred-link-only"
	CodeBoundExceeded       = "e2e-bound-exceeded"
)

const (
	e2eSafeNote     = "each omitted test is proven unneeded only on its named basis, never in general"
	linkAttestation = "link completeness is an author attestation"
)

// builtinGlobalNames are the lockfile, manifest, build and container file names that are global
// paths wherever they sit (AFU-V1 Definitions).
var builtinGlobalNames = map[string]bool{
	"package.json": true, "package-lock.json": true, "npm-shrinkwrap.json": true, "yarn.lock": true,
	"pnpm-lock.yaml": true, "pnpm-workspace.yaml": true, "bun.lockb": true, "go.mod": true, "go.sum": true,
	"Cargo.toml": true, "Cargo.lock": true, "pyproject.toml": true, "poetry.lock": true, "requirements.txt": true,
	"Pipfile": true, "Pipfile.lock": true, "Gemfile": true, "Gemfile.lock": true, "composer.json": true,
	"composer.lock": true, "Makefile": true, "docker-compose.yml": true, "docker-compose.yaml": true,
	"compose.yml": true, "compose.yaml": true, ".env": true,
}

// closureUnknowns are the plan unknowns that leave the obligation closure bounded: an unowned changed
// path must itself be a reviewed link target, and an untested changed unit still has its dependents.
var closureUnknowns = map[string]bool{affected.UnknownUnownedDirtyPath: true, affected.UnknownNoSelectableTest: true}

// e2eFrontiers are the TypeScript frontiers this profile answers itself, as the Playwright profile
// does: an E2E test's runtime dependency on the application is what reviewed links and coverage
// prove, and the executable runner config is a global path reconciled through discovery.
var e2eFrontiers = map[string]bool{typescript.FrontierE2ERuntimeDependency: true, typescript.FrontierConfig: true}

// unbounded reports a plan unknown that leaves the obligation closure unbounded.
func unbounded(u affected.Unknown) bool {
	return !closureUnknowns[u.Reason] && !(u.Reason == affected.UnknownLanguageFrontier && e2eFrontiers[u.Detail])
}

// SelectionProvider is one closed application-flow-selection-provider/1 document.
type SelectionProvider struct {
	Schema       string              `json:"schema"`
	Flows        string              `json:"flows"`
	RunnerConfig string              `json:"runner_config"`
	Discovery    string              `json:"discovery,omitempty"`
	GlobalPaths  []string            `json:"global_paths"`
	Inventory    []InventoryTest     `json:"inventory"`
	FlowTiers    map[string][]string `json:"flow_tiers,omitempty"`
	Coverage     string              `json:"coverage,omitempty"`
}

// CoverageFile is the ingested coverage the provider names; it lives outside the provider so that a
// new record does not change a global path after its own evidence commit.
type CoverageFile struct {
	Schema  string           `json:"schema"`
	Records []CoverageRecord `json:"records"`
}

// InventoryTest is one E2E test the provider declares, by test key, runner project and file.
type InventoryTest struct {
	TestKey string `json:"test_key"`
	Project string `json:"project"`
	Path    string `json:"path"`
}

// CoverageRecord is one test's ingested per-tier coverage from its run at Commit.
type CoverageRecord struct {
	TestKey string         `json:"test_key"`
	Commit  string         `json:"commit"`
	Tiers   []CoverageTier `json:"tiers"`
}

type CoverageTier struct {
	Tier     string   `json:"tier"`
	Complete bool     `json:"complete"`
	Paths    []string `json:"paths"`
}

// E2EInput is what `affected` hands the profile. Provider and Discovery are repository-relative;
// Base is empty for a worktree-only run; Changed is the plan's changed paths.
type E2EInput struct {
	Provider  string
	Discovery string
	Revision  string
	Base      string
	Graph     *affected.Graph
	Plan      affected.Plan
	Mandatory []any
}

// E2ESelection is the e2e-safe test_selection member (AFU-V1-023).
type E2ESelection struct {
	Schema              string         `json:"schema"`
	Profile             string         `json:"profile"`
	State               string         `json:"state"`
	StateReason         string         `json:"state_reason"`
	Base                string         `json:"base"`
	Mandatory           []any          `json:"mandatory"`
	Discovery           E2EDiscovery   `json:"discovery"`
	Selected            []E2ETest      `json:"selected"`
	OmittedTests        []E2EOmission  `json:"omitted_tests"`
	OmissionsPerBasis   map[string]int `json:"omissions_per_basis"`
	Fallback            []E2EFallback  `json:"fallback"`
	Note                string         `json:"note"`
	UntrustedTextFields []string       `json:"untrusted_text_fields"`
}

type E2EDiscovery struct {
	Path  string `json:"path"`
	State string `json:"state"`
}

// E2ETest is one selected test; a discovered test missing from the inventory has no test key.
type E2ETest struct {
	TestKey string   `json:"test_key"`
	Project string   `json:"project"`
	Path    string   `json:"path"`
	Reasons []string `json:"reasons"`
}

type E2EOmission struct {
	TestKey string   `json:"test_key"`
	Project string   `json:"project"`
	Path    string   `json:"path"`
	Basis   string   `json:"basis"`
	Proof   E2EProof `json:"proof"`
}

// E2EProof names what an exclusion rests on and the changed paths it is disjoint from.
type E2EProof struct {
	Links        []ProofLink     `json:"links,omitempty"`
	Coverage     *CoverageRecord `json:"coverage,omitempty"`
	DisjointFrom []string        `json:"disjoint_from"`
	Attestation  string          `json:"attestation,omitempty"`
}

type ProofLink struct {
	Flow       string     `json:"flow"`
	From       string     `json:"from"`
	Target     LinkTarget `json:"target"`
	ReviewedAt string     `json:"reviewed_at"`
}

type E2EFallback struct {
	Code    string `json:"code"`
	Subject string `json:"subject"`
}

type e2eSelector struct {
	ctx       context.Context
	root      string
	in        E2EInput
	base      string
	provider  SelectionProvider
	records   []CoverageRecord
	changedBy map[string]string
	selected  map[string]bool
	coverage  map[string]CoverageRecord
}

// SelectE2E decides the e2e-safe selection. It narrows only when every inventoried test is
// selected or carries an exclusion proof; otherwise it returns the full relevant suite with codes.
func SelectE2E(ctx context.Context, root string, in E2EInput) E2ESelection {
	out := E2ESelection{Schema: E2ESafeSchema, Profile: E2ESafeProfile, Base: in.Base, Mandatory: in.Mandatory,
		Selected: []E2ETest{}, OmittedTests: []E2EOmission{}, OmissionsPerBasis: map[string]int{BasisCoverage: 0, BasisReviewedLinks: 0},
		Fallback: []E2EFallback{}, Note: extevidence.SelectionNote + "; " + e2eSafeNote,
		UntrustedTextFields: []string{"test_selection.selected[].test_key", "test_selection.selected[].project", "test_selection.omitted_tests[].test_key", "test_selection.omitted_tests[].project"}}
	provider, records, err := readSelectionProvider(root, in.Provider)
	if err != nil {
		out.State, out.StateReason = extevidence.SelectionBlocked, "provider-unavailable"
		return out
	}
	s := newE2ESelector(ctx, root, in, provider, records)
	out.Base = s.base
	discovery, discovered := s.discover()
	out.Discovery = discovery
	codes := s.boundCodes()
	if len(codes) == 0 {
		codes = append(s.globalCodes(), inventoryCodes(provider.Inventory, discovered, out.Discovery.State)...)
	}
	if len(codes) == 0 {
		out.Selected, out.OmittedTests, codes = s.prove()
	}
	if len(codes) != 0 {
		out.State, out.StateReason = extevidence.SelectionFull, "e2e-fallback"
		out.Selected, out.OmittedTests, out.Fallback = fullSuite(provider.Inventory, discovered), []E2EOmission{}, sortedFallback(codes)
		return out
	}
	for _, o := range out.OmittedTests {
		out.OmissionsPerBasis[o.Basis]++
	}
	out.State, out.StateReason = extevidence.SelectionNarrow, "exclusions-proven"
	return out
}

func newE2ESelector(ctx context.Context, root string, in E2EInput, provider SelectionProvider, records []CoverageRecord) *e2eSelector {
	s := &e2eSelector{ctx: ctx, root: root, in: in, base: in.Base, provider: provider, records: records, selected: map[string]bool{}, coverage: map[string]CoverageRecord{}}
	if s.base == "" {
		s.base = in.Revision
	}
	s.changedBy = changedUnits(in.Graph, in.Plan.Dirty)
	for _, sel := range in.Plan.Selected {
		s.selected[sel.UnitID] = true
	}
	for _, record := range records {
		s.coverage[record.TestKey] = record
	}
	return s
}

// readSelectionProvider reads a repository-relative provider, and the coverage file it names, as
// closed, bounded, secret-screened documents.
func readSelectionProvider(root, name string) (SelectionProvider, []CoverageRecord, error) {
	var p SelectionProvider
	var coverage CoverageFile
	if err := readSelectionDocument(root, name, &p); err != nil {
		return p, nil, err
	}
	if err := validSelectionProvider(p); err != nil || p.Coverage == "" {
		return p, nil, err
	}
	if err := readSelectionDocument(root, p.Coverage, &coverage); err != nil {
		return p, nil, err
	}
	return p, coverage.Records, validCoverage(p, coverage)
}

func readSelectionDocument(root, name string, out any) error {
	if !safePath(name) {
		return errors.New("selection input must be a repository-relative path")
	}
	raw, err := ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		return err
	}
	return Decode(raw, out)
}

func validSelectionProvider(p SelectionProvider) error {
	paths := append([]string{p.Flows, p.RunnerConfig}, p.GlobalPaths...)
	for _, optional := range []string{p.Discovery, p.Coverage} {
		if optional != "" {
			paths = append(paths, optional)
		}
	}
	if p.Schema != SelectionProviderSchema || slices.ContainsFunc(paths, func(s string) bool { return !safePath(s) }) {
		return errors.New("invalid selection provider")
	}
	keys := map[string]bool{}
	for _, t := range p.Inventory {
		if !flowText(t.TestKey) || !flowText(t.Project) || !safePath(t.Path) || keys[t.TestKey] {
			return errors.New("invalid selection inventory")
		}
		keys[t.TestKey] = true
	}
	return nil
}

func validCoverage(p SelectionProvider, coverage CoverageFile) error {
	if coverage.Schema != CoverageSchema {
		return errors.New("invalid coverage schema")
	}
	keys, seen := map[string]bool{}, map[string]bool{}
	for _, t := range p.Inventory {
		keys[t.TestKey] = true
	}
	for _, record := range coverage.Records {
		if seen[record.TestKey] {
			return errors.New("duplicate coverage record")
		}
		seen[record.TestKey] = true
		if !keys[record.TestKey] || !oidPattern.MatchString(record.Commit) || slices.ContainsFunc(record.Tiers, func(t CoverageTier) bool { return !flowText(t.Tier) }) {
			return errors.New("invalid coverage record")
		}
	}
	return nil
}

// discover reads the discovery record (the --playwright-discovery file, else the provider's) and binds it to HEAD.
func (s *e2eSelector) discover() (E2EDiscovery, []typescript.PlaywrightDiscoveryUnit) {
	name := s.in.Discovery
	if name == "" {
		name = s.provider.Discovery
	}
	var raw []byte
	if safePath(name) {
		raw, _ = ReadFile(filepath.Join(s.root, filepath.FromSlash(name)))
	}
	units, state := typescript.VerifyPlaywrightDiscovery(s.root, s.provider.RunnerConfig, s.in.Revision, raw)
	return E2EDiscovery{Path: name, State: state}, append([]typescript.PlaywrightDiscoveryUnit{}, units...)
}

func (s *e2eSelector) boundCodes() []E2EFallback {
	counts := map[string]int{"changed-paths": len(s.in.Plan.Dirty), "inventory": len(s.provider.Inventory), "coverage": len(s.records), "global-paths": len(s.provider.GlobalPaths)}
	for _, record := range s.records {
		for _, tier := range record.Tiers {
			counts["coverage-paths"] += len(tier.Paths)
		}
	}
	limits := map[string]int{"changed-paths": maxSelectionPaths, "inventory": maxSelectionTests, "coverage": maxSelectionTests, "global-paths": maxSelectionPaths, "coverage-paths": maxSelectionPaths}
	codes := []E2EFallback{}
	for name, count := range counts {
		if count > limits[name] {
			codes = append(codes, E2EFallback{Code: CodeBoundExceeded, Subject: name})
		}
	}
	return codes
}

// globalCodes forbids every omission when a changed path is global (AFU-V1-022).
func (s *e2eSelector) globalCodes() []E2EFallback {
	codes := []E2EFallback{}
	for _, p := range s.in.Plan.Dirty {
		if s.global(p) {
			codes = append(codes, E2EFallback{Code: CodeGlobalPathChanged, Subject: p})
		}
	}
	return codes
}

func (s *e2eSelector) global(p string) bool {
	name := path.Base(p)
	if builtinGlobalNames[name] || strings.HasPrefix(name, "Dockerfile") || strings.HasPrefix(name, ".env.") {
		return true
	}
	if p == s.in.Provider || p == s.provider.RunnerConfig || under(p, s.provider.Flows) {
		return true
	}
	return slices.ContainsFunc(s.provider.GlobalPaths, func(g string) bool { return under(p, g) })
}

func under(p, dir string) bool { return p == dir || strings.HasPrefix(p, dir+"/") }

// inventoryCodes forbids narrowing unless a matched discovery record lists exactly the inventoried project and file pairs.
func inventoryCodes(inventory []InventoryTest, discovered []typescript.PlaywrightDiscoveryUnit, state string) []E2EFallback {
	if state != "MATCHED" {
		return []E2EFallback{{Code: CodeInventoryIncomplete, Subject: "discovery:" + state}}
	}
	declared := map[typescript.PlaywrightDiscoveryUnit]bool{}
	for _, t := range inventory {
		declared[typescript.PlaywrightDiscoveryUnit{Project: t.Project, Test: t.Path}] = true
	}
	codes := []E2EFallback{}
	for _, u := range discovered {
		if !declared[u] {
			codes = append(codes, E2EFallback{Code: CodeInventoryIncomplete, Subject: "not-inventoried:" + u.Project + ":" + u.Test})
		}
		delete(declared, u)
	}
	for u := range declared {
		codes = append(codes, E2EFallback{Code: CodeInventoryIncomplete, Subject: "not-discovered:" + u.Project + ":" + u.Test})
	}
	return codes
}

// prove selects each test the change reaches and proves each other test's exclusion, or returns codes.
func (s *e2eSelector) prove() ([]E2ETest, []E2EOmission, []E2EFallback) {
	set, err := LoadIntentsAt(s.ctx, s.root, s.provider.Flows, s.base)
	links := []EvaluatedLink{}
	if err == nil {
		links, err = EvaluateLinks(s.ctx, s.root, set, s.base)
	}
	if err != nil {
		return nil, nil, []E2EFallback{{Code: CodeMapStale, Subject: s.provider.Flows}}
	}
	unmapped, unexplained := s.unmapped(links), s.unexplained(links)
	selected, omitted, codes := []E2ETest{}, []E2EOmission{}, []E2EFallback{}
	for _, t := range s.provider.Inventory {
		testLinks := linksOfTest(links, t.TestKey)
		if reasons := s.reasons(t, testLinks); len(reasons) != 0 {
			selected = append(selected, E2ETest{TestKey: t.TestKey, Project: t.Project, Path: t.Path, Reasons: reasons})
			continue
		}
		omission, failures := s.exclusion(t, testLinks, unmapped, unexplained)
		omitted = append(omitted, omission...)
		codes = append(codes, failures...)
	}
	return selected, omitted, codes
}

// unmapped lists the changed non-global paths no reviewed link targets, or every changed path when
// the impact graph cannot bound the obligation closure.
func (s *e2eSelector) unmapped(links []EvaluatedLink) []string {
	if slices.ContainsFunc(s.in.Plan.Unknown, unbounded) {
		return append([]string{}, s.in.Plan.Dirty...)
	}
	out := []string{}
	for _, p := range s.in.Plan.Dirty {
		if !slices.ContainsFunc(links, func(l EvaluatedLink) bool { return l.ReviewState == ReviewReviewed && l.Target.Path == p }) {
			out = append(out, p)
		}
	}
	return out
}

// unexplained lists the changed paths neither the impact graph, a reviewed link nor any coverage
// record describes, such as a fixture or seed file a flow names: coverage cannot show them unused.
func (s *e2eSelector) unexplained(links []EvaluatedLink) []string {
	if slices.ContainsFunc(s.in.Plan.Unknown, unbounded) {
		return append([]string{}, s.in.Plan.Dirty...)
	}
	covered := map[string]bool{}
	for _, record := range s.records {
		for _, tier := range record.Tiers {
			for _, p := range tier.Paths {
				covered[p] = true
			}
		}
	}
	for _, l := range links {
		covered[l.Target.Path] = covered[l.Target.Path] || l.ReviewState == ReviewReviewed
	}
	out := []string{}
	for _, p := range s.in.Plan.Dirty {
		if _, owned := s.in.Graph.OwnerOf(p); !owned && !covered[p] {
			out = append(out, p)
		}
	}
	return out
}

// linksOfTest returns every link of every flow that links the test key.
func linksOfTest(links []EvaluatedLink, key string) []EvaluatedLink {
	flows := map[string]bool{}
	for _, l := range links {
		if l.Target.TestKey == key {
			flows[l.Flow] = true
		}
	}
	out := []EvaluatedLink{}
	for _, l := range links {
		if flows[l.Flow] {
			out = append(out, l)
		}
	}
	return out
}

// reasons names why the change requires the test; any basis of link, and observed coverage, may add a test.
func (s *e2eSelector) reasons(t InventoryTest, links []EvaluatedLink) []string {
	reasons := []string{}
	if slices.Contains(s.in.Plan.Dirty, t.Path) {
		reasons = append(reasons, "test-file-changed")
	}
	if s.inClosure(t.Path) {
		reasons = append(reasons, "static-reach")
	}
	if slices.ContainsFunc(links, func(l EvaluatedLink) bool { return s.inClosure(l.Target.Path) }) {
		reasons = append(reasons, "linked-to-closure")
	}
	if record, ok := s.coverage[t.TestKey]; ok && s.touchesChange(record) {
		reasons = append(reasons, "observed-coverage")
	}
	return reasons
}

// inClosure reports whether a path is changed or depends on a changed path in the impact graph.
func (s *e2eSelector) inClosure(p string) bool {
	if reach(s.in.Graph, s.changedBy, s.in.Plan.Dirty, p) != nil {
		return true
	}
	owner, ok := s.in.Graph.OwnerOf(p)
	return ok && s.selected[owner]
}

func (s *e2eSelector) touchesChange(record CoverageRecord) bool {
	return slices.ContainsFunc(record.Tiers, func(t CoverageTier) bool {
		return slices.ContainsFunc(t.Paths, func(p string) bool { return slices.Contains(s.in.Plan.Dirty, p) })
	})
}

// exclusion proves an unselected test unneeded on the coverage basis, else on the reviewed-links basis.
func (s *e2eSelector) exclusion(t InventoryTest, links []EvaluatedLink, unmapped, unexplained []string) ([]E2EOmission, []E2EFallback) {
	record, hasRecord := s.coverage[t.TestKey]
	failures := []E2EFallback{}
	if hasRecord {
		code := s.coverageFailure(record, links, unexplained)
		if code == "" {
			proof := E2EProof{Coverage: &record, DisjointFrom: s.in.Plan.Dirty}
			return []E2EOmission{{TestKey: t.TestKey, Project: t.Project, Path: t.Path, Basis: BasisCoverage, Proof: proof}}, nil
		}
		failures = append(failures, E2EFallback{Code: code, Subject: t.TestKey})
	}
	if len(links) != 0 {
		code := reviewedFailure(links, unmapped)
		if code == "" {
			proof := E2EProof{Links: proofLinks(links), DisjointFrom: s.in.Plan.Dirty, Attestation: linkAttestation}
			return []E2EOmission{{TestKey: t.TestKey, Project: t.Project, Path: t.Path, Basis: BasisReviewedLinks, Proof: proof}}, nil
		}
		failures = append(failures, E2EFallback{Code: code, Subject: t.TestKey})
	}
	if len(failures) == 0 {
		failures = append(failures, E2EFallback{Code: CodeExclusionUnproven, Subject: t.TestKey})
	}
	return nil, failures
}

// reviewedFailure applies AFU-V1-020: every link reviewed and not stale at base, and every changed
// non-global path a reviewed link target. Disjointness from the closure holds for an unselected test.
func reviewedFailure(links []EvaluatedLink, unmapped []string) string {
	if !slices.ContainsFunc(links, func(l EvaluatedLink) bool { return l.ReviewState != ReviewInferred }) {
		return CodeInferredLinkOnly
	}
	if slices.ContainsFunc(links, func(l EvaluatedLink) bool {
		return l.ReviewState == ReviewStale || l.ReviewState == ReviewLinkNotAtAnchor
	}) {
		return CodeMapStale
	}
	if slices.ContainsFunc(links, func(l EvaluatedLink) bool { return l.ReviewState != ReviewReviewed }) {
		return CodeExclusionUnproven
	}
	if len(unmapped) != 0 {
		return CodeUnmappedChange
	}
	return ""
}

func proofLinks(links []EvaluatedLink) []ProofLink {
	out := []ProofLink{}
	for _, l := range links {
		out = append(out, ProofLink{Flow: l.Flow, From: l.From, Target: l.Target, ReviewedAt: l.ReviewedAt})
	}
	return out
}

// coverageFailure applies AFU-V1-021: complete coverage for every tier each linked flow declares,
// holding at base because no covered or global path changed since the evidence commit.
func (s *e2eSelector) coverageFailure(record CoverageRecord, links []EvaluatedLink, unexplained []string) string {
	if len(unexplained) != 0 {
		return CodeUnmappedChange
	}
	if !s.tiersComplete(record, links) {
		return CodeExclusionUnproven
	}
	if s.coverageStale(record) {
		return CodeMapStale
	}
	return ""
}

func (s *e2eSelector) tiersComplete(record CoverageRecord, links []EvaluatedLink) bool {
	complete := map[string]bool{}
	for _, t := range record.Tiers {
		complete[t.Tier] = t.Complete
	}
	flows := map[string]bool{}
	for _, l := range links {
		flows[l.Flow] = true
	}
	for flow := range flows {
		tiers := s.provider.FlowTiers[flow]
		if len(tiers) == 0 || slices.ContainsFunc(tiers, func(t string) bool { return !complete[t] }) {
			return false
		}
	}
	return len(flows) != 0
}

func (s *e2eSelector) coverageStale(record CoverageRecord) bool {
	if _, err := git(s.ctx, s.root, "merge-base", "--is-ancestor", record.Commit, s.base); err != nil {
		return true
	}
	out, err := git(s.ctx, s.root, "diff", "--no-renames", "--name-only", "-z", record.Commit, s.base, "--")
	if err != nil {
		return true
	}
	covered := map[string]bool{}
	for _, t := range record.Tiers {
		for _, p := range t.Paths {
			covered[p] = true
		}
	}
	return slices.ContainsFunc(strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00"), func(p string) bool {
		return covered[p] || (p != "" && s.global(p))
	})
}

// fullSuite is every inventoried test plus every discovered test the inventory misses.
func fullSuite(inventory []InventoryTest, discovered []typescript.PlaywrightDiscoveryUnit) []E2ETest {
	out := []E2ETest{}
	known := map[typescript.PlaywrightDiscoveryUnit]bool{}
	for _, t := range inventory {
		out = append(out, E2ETest{TestKey: t.TestKey, Project: t.Project, Path: t.Path, Reasons: []string{"full-relevant-suite"}})
		known[typescript.PlaywrightDiscoveryUnit{Project: t.Project, Test: t.Path}] = true
	}
	for _, u := range discovered {
		if !known[u] {
			out = append(out, E2ETest{Project: u.Project, Path: u.Test, Reasons: []string{"full-relevant-suite"}})
		}
	}
	slices.SortFunc(out, func(a, b E2ETest) int {
		return strings.Compare(a.Project+"\x00"+a.Path+"\x00"+a.TestKey, b.Project+"\x00"+b.Path+"\x00"+b.TestKey)
	})
	return out
}

func sortedFallback(codes []E2EFallback) []E2EFallback {
	slices.SortFunc(codes, func(a, b E2EFallback) int { return strings.Compare(a.Code+"\x00"+a.Subject, b.Code+"\x00"+b.Subject) })
	return slices.Compact(codes)
}
