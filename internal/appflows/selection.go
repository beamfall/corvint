package appflows

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/extevidence"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

// The e2e-safe profile of `corvint affected --selection-profile` (AFU-V1-019..024).
const (
	E2ESafeProfile          = "e2e-safe"
	E2ESafeSchema           = "e2e-safe-selection/1"
	SelectionProviderSchema = "application-flow-selection-provider/1"
	CoverageSchema          = "application-flow-coverage/0"
	BasisCoverage           = "coverage"
	BasisReviewedLinks      = "reviewed-links"
	proofDigestPrefix       = "sha256:"
)

// The documented selection ceilings (AFU-V1-065). Above one, the result is the closed
// e2e-bound-exceeded full-suite fallback; the 5 MiB document bound still applies first.
// MaxSelectionPaths bounds every single path list, each coverage tier's included;
// MaxSelectionEvidencePaths bounds the changed paths held across all evidence commits read.
const (
	MaxSelectionTests         = 16384
	MaxSelectionPaths         = 65536
	MaxSelectionCoveragePaths = 131072
	MaxSelectionEvidencePaths = 131072
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

// unbounded reports a plan unknown that leaves the obligation closure unbounded. A TypeScript
// frontier entry may name its site after the reason (`reason:path:line`, TJAA-V0-052), so the
// reason is its first two colon-separated fields.
func unbounded(u affected.Unknown) bool {
	return !closureUnknowns[u.Reason] && !(u.Reason == affected.UnknownLanguageFrontier && e2eFrontiers[frontierReason(u.Detail)])
}

// planUnbounded is unbounded, except that a TypeScript over-bound-source frontier naming one
// file no changed path touches is a per-path unknown: no import of it was resolved, so it adds
// no graph edge, and an omission is still proven per test by coverage or reviewed links. A
// change touching that file, or a frontier entry that names no single file, stays unbounded
// (AFP-V0-444).
func (s *e2eSelector) planUnbounded(u affected.Unknown) bool {
	if u.Reason == affected.UnknownLanguageFrontier && frontierReason(u.Detail) == typescript.FrontierOverBoundSource {
		site := strings.TrimPrefix(u.Detail, typescript.FrontierOverBoundSource+":")
		return site == u.Detail || strings.HasPrefix(site, "+") || !affected.ValidRelativePath(site) || slices.Contains(s.changed, site)
	}
	return unbounded(u)
}

// frontierReason returns the `namespace:reason` prefix of a frontier entry.
func frontierReason(entry string) string {
	fields := strings.SplitN(entry, ":", 3)
	if len(fields) < 2 {
		return entry
	}
	return fields[0] + ":" + fields[1]
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
	// Repositories are the application repositories whose changes the profile reads (AFU-V1-067).
	Repositories []SelectionRepository `json:"repositories,omitempty"`
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

// UnmarshalJSON keeps an omitted or null project distinct from Playwright's explicit default identity.
// It runs only inside Decode of the whole provider, which already screened these bytes (V1-2164).
func (t *InventoryTest) UnmarshalJSON(raw []byte) error {
	var input struct {
		TestKey string  `json:"test_key"`
		Project *string `json:"project"`
		Path    string  `json:"path"`
	}
	if err := decodeElement(raw, &input); err != nil {
		return err
	}
	if input.Project == nil {
		return errors.New("selection inventory requires an explicit project string")
	}
	*t = InventoryTest{TestKey: input.TestKey, Project: *input.Project, Path: input.Path}
	return nil
}

// CoverageRecord is one test's ingested per-tier coverage from its run at Commit.
type CoverageRecord struct {
	TestKey string         `json:"test_key"`
	Commit  string         `json:"commit"`
	Tiers   []CoverageTier `json:"tiers"`
	// Repositories maps each declared application repository to the commit the run observed (AFU-V1-070).
	Repositories map[string]string `json:"repositories,omitempty"`
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
	// Repositories are the operator's application repository bindings (AFU-V1-067).
	Repositories []E2ERepositoryInput
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
	Proofs              []E2EProof     `json:"proofs"`
	DisjointFrom        []string       `json:"disjoint_from"`
	OmissionsPerBasis   map[string]int `json:"omissions_per_basis"`
	Fallback            []E2EFallback  `json:"fallback"`
	Note                string         `json:"note"`
	UntrustedTextFields []string       `json:"untrusted_text_fields"`
	// Repositories is absent unless the provider declares or the operator binds one (AFU-V1-072).
	Repositories []E2ERepository `json:"repositories,omitempty"`
	// Refusal is present only on a blocked provider-unavailable result: which document was refused
	// and its closed refusal class, never the refused text (AFU-V1-127).
	Refusal *E2EFallback `json:"refusal,omitempty"`
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

// E2EOmission names its exclusion proof by digest; the proof itself is stated once in proofs
// (e2e-safe-selection/1, AFU-V1-066).
type E2EOmission struct {
	TestKey string `json:"test_key"`
	Project string `json:"project"`
	Path    string `json:"path"`
	Basis   string `json:"basis"`
	Proof   string `json:"proof"`
}

// E2EProof names what an exclusion rests on. Every proof is disjoint from the result's
// disjoint_from, the changed paths. Digest is over the canonical JSON of the proof without it.
type E2EProof struct {
	Digest      string          `json:"digest"`
	Links       []ProofLink     `json:"links,omitempty"`
	Coverage    *CoverageRecord `json:"coverage,omitempty"`
	Attestation string          `json:"attestation,omitempty"`
}

type ProofLink struct {
	Flow                 string     `json:"flow"`
	From                 string     `json:"from"`
	Target               LinkTarget `json:"target"`
	ReviewedAt           string     `json:"reviewed_at"`
	ReviewedAtRepository string     `json:"reviewed_at_repository,omitempty"`
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
	// changed is every changed path: the plan's root paths, then each bound repository's qualified paths.
	changed   []string
	repos     map[string]boundRepository
	declared  map[string]bool
	dirty     map[string]bool
	selected  map[string]bool
	coverage  map[string]CoverageRecord
	closure   map[string]bool
	globals   map[string]bool
	evidence  map[string]*evidenceDiff
	proofs    map[string]E2EProof
	flowProof map[string]string
	// evidencePaths counts the changed paths cached across evidence commits; above
	// MaxSelectionEvidencePaths the cache is dropped and evidenceExceeded refuses the selection.
	// reachMemo counts memoized reach answers, which stop being cached at the same bound.
	evidencePaths    int
	evidenceExceeded bool
	reachMemo        int
}

// evidenceDiff is what changed between one coverage evidence commit and base, computed once per
// commit; ok is false when the commit is not an ancestor of base or the diff is unreadable.
type evidenceDiff struct {
	ok        bool
	global    bool
	set       map[string]bool
	changedBy map[string]string
	reached   map[string]bool
}

// SelectE2E decides the e2e-safe selection. It narrows only when every inventoried test is
// selected or carries an exclusion proof; otherwise it returns the full relevant suite with codes.
func SelectE2E(ctx context.Context, root string, in E2EInput) E2ESelection {
	out := E2ESelection{Schema: E2ESafeSchema, Profile: E2ESafeProfile, Base: in.Base, Mandatory: in.Mandatory,
		Selected: []E2ETest{}, OmittedTests: []E2EOmission{}, Proofs: []E2EProof{}, DisjointFrom: []string{}, OmissionsPerBasis: map[string]int{BasisCoverage: 0, BasisReviewedLinks: 0},
		Fallback: []E2EFallback{}, Note: extevidence.SelectionNote + "; " + e2eSafeNote,
		UntrustedTextFields: []string{"test_selection.selected[].test_key", "test_selection.selected[].project", "test_selection.omitted_tests[].test_key", "test_selection.omitted_tests[].project"}}
	provider, records, err := readSelectionProvider(root, in.Provider)
	if err != nil {
		out.State, out.StateReason = extevidence.SelectionBlocked, "provider-unavailable"
		out.Refusal = &E2EFallback{Code: "provider-unavailable", Subject: selectionRefusalSubject(err)}
		return out
	}
	rows, repos, repoCodes := bindRepositories(ctx, root, provider.Repositories, in.Repositories)
	if len(rows) != 0 {
		out.Repositories = rows
	}
	s := newE2ESelector(ctx, root, in, provider, records, repos)
	out.Base = s.base
	discovery, discovered := s.discover()
	out.Discovery = discovery
	codes := s.boundCodes()
	if len(codes) == 0 {
		codes = append(append(repoCodes, s.globalCodes()...), inventoryCodes(provider.Inventory, discovered, out.Discovery.State)...)
	}
	if len(codes) == 0 {
		out.Selected, out.OmittedTests, codes = s.prove()
		if s.evidenceExceeded {
			codes = []E2EFallback{{Code: CodeBoundExceeded, Subject: "evidence-paths"}}
		}
	}
	if len(codes) != 0 {
		out.State, out.StateReason = extevidence.SelectionFull, "e2e-fallback"
		out.Selected, out.OmittedTests, out.Fallback = fullSuite(provider.Inventory, discovered), []E2EOmission{}, sortedFallback(codes)
		return out
	}
	for _, o := range out.OmittedTests {
		out.OmissionsPerBasis[o.Basis]++
	}
	out.Proofs, out.DisjointFrom = s.proofList(), append([]string{}, s.changed...)
	out.State, out.StateReason = extevidence.SelectionNarrow, "exclusions-proven"
	return out
}

func newE2ESelector(ctx context.Context, root string, in E2EInput, provider SelectionProvider, records []CoverageRecord, repos map[string]boundRepository) *e2eSelector {
	s := &e2eSelector{ctx: ctx, root: root, in: in, base: in.Base, provider: provider, records: records, selected: map[string]bool{}, coverage: map[string]CoverageRecord{},
		dirty: map[string]bool{}, closure: map[string]bool{}, globals: map[string]bool{}, evidence: map[string]*evidenceDiff{}, proofs: map[string]E2EProof{}, flowProof: map[string]string{},
		repos: repos, declared: map[string]bool{}}
	if s.base == "" {
		s.base = in.Revision
	}
	s.changedBy = changedUnits(in.Graph, in.Plan.Dirty)
	s.changed = append([]string{}, in.Plan.Dirty...)
	for _, repo := range provider.Repositories {
		s.declared[repo.ID] = true
		s.changed = append(s.changed, repos[repo.ID].changed...)
	}
	for _, p := range s.changed {
		s.dirty[p] = true
	}
	for _, sel := range in.Plan.Selected {
		s.selected[sel.UnitID] = true
	}
	for _, record := range records {
		s.coverage[record.TestKey] = record
	}
	for _, g := range provider.GlobalPaths {
		s.globals[g] = true
	}
	return s
}

// readSelectionProvider reads a repository-relative provider, and the coverage file it names, as
// closed, bounded, secret-screened documents.
func readSelectionProvider(root, name string) (SelectionProvider, []CoverageRecord, error) {
	var p SelectionProvider
	var coverage CoverageFile
	if err := readSelectionDocument(root, name, &p, maskFlowTierKeys); err != nil {
		return p, nil, selectionInputError{"provider", err}
	}
	if err := validSelectionProvider(p); err != nil {
		return p, nil, selectionInputError{"provider", errInvalidContent}
	}
	if p.Coverage == "" {
		return p, nil, nil
	}
	if err := readSelectionDocument(root, p.Coverage, &coverage, nil); err != nil {
		return p, nil, selectionInputError{"coverage", err}
	}
	if err := validCoverage(p, coverage); err != nil {
		return p, nil, selectionInputError{"coverage", errInvalidContent}
	}
	return p, coverage.Records, nil
}

// errInvalidPath and errInvalidContent are the selection-input refusals outside Decode: a name that
// is not repository-relative, and a decoded document that fails its semantic validation.
var (
	errInvalidPath    = errors.New("selection input must be a repository-relative path")
	errInvalidContent = errors.New("invalid selection input")
)

// selectionInputError names the refused selection document, provider or coverage (AFU-V1-127).
type selectionInputError struct {
	document string
	err      error
}

func (e selectionInputError) Error() string { return e.document + ": " + e.err.Error() }
func (e selectionInputError) Unwrap() error { return e.err }

// selectionRefusalSubject is `document:class` with a closed class; an unreadable file is any read
// refusal other than the byte bound (AFU-V1-127).
func selectionRefusalSubject(err error) string {
	var input selectionInputError
	document := "provider"
	if errors.As(err, &input) {
		document = input.document
	}
	class := "unreadable"
	for _, c := range []struct {
		err   error
		class string
	}{
		{errInputBytes, "byte-limit"}, {errDecodeBytes, "byte-limit"}, {errSecretShaped, "secret-shaped"},
		{errInvalidJSON, "invalid-json"}, {errInvalidSchema, "invalid-schema"}, {errTrailingInput, "trailing-input"},
		{errInvalidPath, "invalid-path"}, {errInvalidContent, "invalid-content"},
	} {
		if errors.Is(err, c.err) {
			class = c.class
			break
		}
	}
	return document + ":" + class
}

func readSelectionDocument(root, name string, out any, exempt func([]byte) string) error {
	if !safePath(name) {
		return errInvalidPath
	}
	raw, err := ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		return err
	}
	return decode(raw, out, exempt)
}

func validSelectionProvider(p SelectionProvider) error {
	paths := []string{p.Flows, p.RunnerConfig}
	for _, optional := range []string{p.Discovery, p.Coverage} {
		if optional != "" {
			paths = append(paths, optional)
		}
	}
	declared := map[string]bool{}
	for _, repo := range p.Repositories {
		if declared[repo.ID] || !extevidence.ValidRepositoryID(repo.ID) || !oidPattern.MatchString(repo.Origin) {
			return errors.New("invalid selection repository")
		}
		declared[repo.ID] = true
	}
	if p.Schema != SelectionProviderSchema || slices.ContainsFunc(paths, func(s string) bool { return !safePath(s) }) ||
		slices.ContainsFunc(p.GlobalPaths, func(s string) bool { return !safePath(s) && !validQualified(s, declared) }) {
		return errors.New("invalid selection provider")
	}
	keys := map[string]bool{}
	for _, t := range p.Inventory {
		if !flowText(t.TestKey) || (t.Project != "" && !flowText(t.Project)) || !safePath(t.Path) || keys[t.TestKey] {
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
	keys, seen, declared := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, t := range p.Inventory {
		keys[t.TestKey] = true
	}
	for _, r := range p.Repositories {
		declared[r.ID] = true
	}
	for _, record := range coverage.Records {
		if seen[record.TestKey] {
			return errors.New("duplicate coverage record")
		}
		seen[record.TestKey] = true
		if !keys[record.TestKey] || !oidPattern.MatchString(record.Commit) || slices.ContainsFunc(record.Tiers, func(t CoverageTier) bool { return !flowText(t.Tier) }) {
			return errors.New("invalid coverage record")
		}
		// A tier path is a canonical root path or a canonical path of a declared repository; any other
		// path could miss the change it names and become an exclusion proof (AFU-V1-070).
		for _, tier := range record.Tiers {
			if slices.ContainsFunc(tier.Paths, func(path string) bool { return !safePath(path) && !validQualified(path, declared) }) {
				return errors.New("invalid coverage tier path")
			}
		}
		for id, commit := range record.Repositories {
			if !declared[id] || !oidPattern.MatchString(commit) {
				return errors.New("invalid coverage record repository")
			}
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
	// The declared generated paths are pruned from the source walk, as `affected discovery`
	// prunes them, so an excluded subtree never reaches a walk bound (AFP-V0-444).
	if _, generated, err := affected.ReadGeneratedDeclaration(s.root); err == nil && len(generated) != 0 {
		if release, err := affected.HoldIgnoredExclusion(s.root, generated); err == nil {
			defer release()
		}
	}
	units, state := typescript.VerifyPlaywrightDiscovery(s.root, s.provider.RunnerConfig, s.in.Revision, raw)
	return E2EDiscovery{Path: name, State: state}, append([]typescript.PlaywrightDiscoveryUnit{}, units...)
}

func (s *e2eSelector) boundCodes() []E2EFallback {
	counts := map[string]int{"changed-paths": len(s.changed), "inventory": len(s.provider.Inventory), "coverage": len(s.records), "global-paths": len(s.provider.GlobalPaths)}
	for _, record := range s.records {
		for _, tier := range record.Tiers {
			counts["coverage-paths"] += len(tier.Paths)
			counts["coverage-tier-paths"] = max(counts["coverage-tier-paths"], len(tier.Paths))
		}
	}
	limits := map[string]int{"changed-paths": MaxSelectionPaths, "inventory": MaxSelectionTests, "coverage": MaxSelectionTests, "global-paths": MaxSelectionPaths,
		"coverage-paths": MaxSelectionCoveragePaths, "coverage-tier-paths": MaxSelectionPaths}
	codes := []E2EFallback{}
	for name, count := range counts {
		if count > limits[name] {
			codes = append(codes, E2EFallback{Code: CodeBoundExceeded, Subject: name})
		}
	}
	return codes
}

// globalCodes forbids every omission when a changed path is global (AFU-V1-022), and when a root
// changed path could be mistaken for a declared repository's qualified path (AFU-V1-069).
func (s *e2eSelector) globalCodes() []E2EFallback {
	codes := []E2EFallback{}
	for _, p := range s.in.Plan.Dirty {
		if id, _, ok := strings.Cut(p, ":"); ok && s.declared[id] {
			codes = append(codes, E2EFallback{Code: CodeUnmappedChange, Subject: p})
		}
	}
	for _, p := range s.changed {
		if s.global(p) {
			codes = append(codes, E2EFallback{Code: CodeGlobalPathChanged, Subject: p})
		}
	}
	return codes
}

// qualified reports a path in a declared repository's namespace, whether or not it parses as a
// canonical qualified path: such a path never takes root impact-graph ownership or reach (AFU-V1-069).
func (s *e2eSelector) qualified(p string) bool {
	id, _, ok := strings.Cut(p, ":")
	return ok && s.declared[id]
}

func (s *e2eSelector) global(p string) bool {
	name := path.Base(p)
	if id, rel, ok := splitQualified(p); ok && s.declared[id] {
		name = path.Base(rel)
	}
	if builtinGlobalNames[name] || strings.HasPrefix(name, "Dockerfile") || strings.HasPrefix(name, ".env.") {
		return true
	}
	if p == s.in.Provider || p == s.provider.RunnerConfig || under(p, s.provider.Flows) {
		return true
	}
	// under(p, g) holds exactly when g is p or p's prefix before one of its slashes, so looking up
	// those prefixes in the global_paths set matches without scanning every global path.
	if s.globals[p] {
		return true
	}
	for i := range len(p) {
		if p[i] == '/' && s.globals[p[:i]] {
			return true
		}
	}
	return false
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
		pins := map[string]RepositoryPin{}
		for id, repo := range s.repos {
			pins[id] = repo.pin
		}
		links, err = EvaluateLinksAcross(s.ctx, s.root, set, s.base, pins)
	}
	if err != nil {
		return nil, nil, []E2EFallback{{Code: CodeMapStale, Subject: s.provider.Flows}}
	}
	unmapped, unexplained := s.unmapped(links), s.unexplained(links)
	index := indexLinks(links)
	selected, omitted, codes := []E2ETest{}, []E2EOmission{}, []E2EFallback{}
	for _, t := range s.provider.Inventory {
		testLinks := index.ofTest(t.TestKey)
		if reasons := s.reasons(t, testLinks.links); len(reasons) != 0 {
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
	if slices.ContainsFunc(s.in.Plan.Unknown, s.planUnbounded) {
		return append([]string{}, s.changed...)
	}
	reviewed := map[string]bool{}
	for _, l := range links {
		reviewed[l.Target.Path] = reviewed[l.Target.Path] || l.ReviewState == ReviewReviewed
	}
	out := []string{}
	for _, p := range s.changed {
		if !reviewed[p] {
			out = append(out, p)
		}
	}
	return out
}

// unexplained lists the changed paths neither the impact graph, a reviewed link nor any coverage
// record describes, such as a fixture or seed file a flow names: coverage cannot show them unused.
func (s *e2eSelector) unexplained(links []EvaluatedLink) []string {
	if slices.ContainsFunc(s.in.Plan.Unknown, s.planUnbounded) {
		return append([]string{}, s.changed...)
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
	for _, p := range s.changed {
		if covered[p] {
			continue
		}
		if _, owned := s.in.Graph.OwnerOf(p); s.qualified(p) || !owned {
			out = append(out, p)
		}
	}
	return out
}

// linkIndex finds, per test key, every link of every flow that links it, without rescanning all
// links per test.
type linkIndex struct {
	byFlow  map[string][]EvaluatedLink
	order   map[string]int
	flowsOf map[string][]string
}

// testLinks is every link of the flows that link one test, in link order; key names that flow set.
type testLinks struct {
	key   string
	links []EvaluatedLink
}

func indexLinks(links []EvaluatedLink) linkIndex {
	index := linkIndex{byFlow: map[string][]EvaluatedLink{}, order: map[string]int{}, flowsOf: map[string][]string{}}
	for _, l := range links {
		if _, seen := index.order[l.Flow]; !seen {
			index.order[l.Flow] = len(index.order)
		}
		index.byFlow[l.Flow] = append(index.byFlow[l.Flow], l)
		if l.Target.TestKey != "" && !slices.Contains(index.flowsOf[l.Target.TestKey], l.Flow) {
			index.flowsOf[l.Target.TestKey] = append(index.flowsOf[l.Target.TestKey], l.Flow)
		}
	}
	return index
}

func (index linkIndex) ofTest(key string) testLinks {
	flows := slices.Clone(index.flowsOf[key])
	slices.SortFunc(flows, func(a, b string) int { return index.order[a] - index.order[b] })
	out := testLinks{key: strings.Join(flows, "\x00"), links: []EvaluatedLink{}}
	for _, flow := range flows {
		out.links = append(out.links, index.byFlow[flow]...)
	}
	return out
}

// reasons names why the change requires the test; any basis of link, and observed coverage, may add a test.
func (s *e2eSelector) reasons(t InventoryTest, links []EvaluatedLink) []string {
	reasons := []string{}
	if s.dirty[t.Path] {
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
	if in, ok := s.closure[p]; ok {
		return in
	}
	// The dirty set answers reach's own changed-path membership check without its linear scan. A
	// qualified path has no impact-graph node, so only its own change puts it in the closure.
	in := s.dirty[p] || (!s.qualified(p) && reach(s.in.Graph, s.changedBy, nil, p) != nil)
	if !in && !s.qualified(p) {
		owner, ok := s.in.Graph.OwnerOf(p)
		in = ok && s.selected[owner]
	}
	s.closure[p] = in
	return in
}

func (s *e2eSelector) touchesChange(record CoverageRecord) bool {
	return slices.ContainsFunc(record.Tiers, func(t CoverageTier) bool {
		return slices.ContainsFunc(t.Paths, func(p string) bool { return s.dirty[p] })
	})
}

// exclusion proves an unselected test unneeded on the coverage basis, else on the reviewed-links basis.
func (s *e2eSelector) exclusion(t InventoryTest, tl testLinks, unmapped, unexplained []string) ([]E2EOmission, []E2EFallback) {
	record, hasRecord := s.coverage[t.TestKey]
	links := tl.links
	failures := []E2EFallback{}
	if hasRecord {
		code := s.coverageFailure(t.Path, record, links, unexplained)
		if code == "" {
			digest := s.addProof(E2EProof{Coverage: &record})
			return []E2EOmission{{TestKey: t.TestKey, Project: t.Project, Path: t.Path, Basis: BasisCoverage, Proof: digest}}, nil
		}
		failures = append(failures, E2EFallback{Code: code, Subject: t.TestKey})
	}
	if len(links) != 0 {
		code := reviewedFailure(links, unmapped)
		if code == "" {
			digest, ok := s.flowProof[tl.key]
			if !ok {
				digest = s.addProof(E2EProof{Links: proofLinks(links), Attestation: linkAttestation})
				s.flowProof[tl.key] = digest
			}
			return []E2EOmission{{TestKey: t.TestKey, Project: t.Project, Path: t.Path, Basis: BasisReviewedLinks, Proof: digest}}, nil
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

// addProof states a proof once, keyed by its digest, and returns the digest.
func (s *e2eSelector) addProof(proof E2EProof) string {
	proof.Digest = proofDigest(proof)
	s.proofs[proof.Digest] = proof
	return proof.Digest
}

// proofDigest is `sha256:` and the hex SHA-256 of the proof's canonical JSON without its digest
// member: keys sorted at every level, no insignificant whitespace, UTF-8 without HTML escaping and
// no trailing newline (AFU-V1-066), so a consumer recomputes it from the emitted proof alone.
func proofDigest(proof E2EProof) string {
	raw, err := json.Marshal(proof)
	if err != nil {
		panic(err) // a proof is plain strings, numbers and booleans and always encodes
	}
	var value map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		panic(err)
	}
	delete(value, "digest")
	canonical, err := gokernel.CanonicalJSON(value)
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(canonical)
	return proofDigestPrefix + hex.EncodeToString(sum[:])
}

// proofList is every stated proof, sorted by digest.
func (s *e2eSelector) proofList() []E2EProof {
	out := make([]E2EProof, 0, len(s.proofs))
	for _, proof := range s.proofs {
		out = append(out, proof)
	}
	slices.SortFunc(out, func(a, b E2EProof) int { return strings.Compare(a.Digest, b.Digest) })
	return out
}

func proofLinks(links []EvaluatedLink) []ProofLink {
	out := []ProofLink{}
	for _, l := range links {
		out = append(out, ProofLink{Flow: l.Flow, From: l.From, Target: l.Target, ReviewedAt: l.ReviewedAt, ReviewedAtRepository: l.ReviewedAtRepository})
	}
	return out
}

// coverageFailure applies AFU-V1-021: complete coverage for every tier each linked flow declares,
// holding at base under the Verified carry-forward rule.
func (s *e2eSelector) coverageFailure(test string, record CoverageRecord, links []EvaluatedLink, unexplained []string) string {
	if len(unexplained) != 0 {
		return CodeUnmappedChange
	}
	if !s.tiersComplete(record, links) {
		return CodeExclusionUnproven
	}
	if s.coverageStale(test, record, links) {
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

// coverageStale reports evidence-stale coverage: a covered path, a global path, the test's static
// reach, or a link target of its flows or a path the impact graph reaches from one changed between
// the evidence commit and base.
func (s *e2eSelector) coverageStale(test string, record CoverageRecord, links []EvaluatedLink) bool {
	diff := s.evidenceDiff(record.Commit)
	if !diff.ok {
		return true
	}
	covered := slices.ContainsFunc(record.Tiers, func(t CoverageTier) bool {
		return slices.ContainsFunc(t.Paths, func(p string) bool { return diff.set[p] })
	})
	return covered || diff.global || s.reachedSince(diff, test) ||
		slices.ContainsFunc(links, func(l EvaluatedLink) bool { return s.reachedSince(diff, l.Target.Path) }) ||
		s.repositoryStale(record, links)
}

// repositoryStale applies AFU-V1-070: the record names every declared repository's commit, each an
// ancestor of that repository's base with no covered path, global path or link target changed since.
func (s *e2eSelector) repositoryStale(record CoverageRecord, links []EvaluatedLink) bool {
	for _, repo := range s.provider.Repositories {
		commit, ok := record.Repositories[repo.ID]
		bound := s.repos[repo.ID]
		if !ok || bound.pin.Dir == "" {
			return true
		}
		diff := s.evidenceDiffAt(repo.ID+"\x00"+commit, bound.pin.Dir, bound.pin.Revision, repo.ID+":", commit)
		if !diff.ok || diff.global {
			return true
		}
		if slices.ContainsFunc(record.Tiers, func(t CoverageTier) bool {
			return slices.ContainsFunc(t.Paths, func(p string) bool { return diff.set[p] })
		}) || slices.ContainsFunc(links, func(l EvaluatedLink) bool { return diff.set[l.Target.Path] }) {
			return true
		}
	}
	return false
}

// evidenceDiff reads the change between one evidence commit and base once. The changed paths cached
// across commits are bounded by MaxSelectionEvidencePaths (AFU-V1-065): the read that would exceed it
// drops the whole cache and marks the selection refused, so later reads return a failed diff.
func (s *e2eSelector) evidenceDiff(commit string) *evidenceDiff {
	return s.evidenceDiffAt(commit, s.root, s.base, "", commit)
}

// evidenceDiffAt reads, once per key, the change between commit and base in dir, each path prefixed
// with prefix: empty for the root, `ID:` for a bound application repository (AFU-V1-070).
func (s *e2eSelector) evidenceDiffAt(key, dir, base, prefix, commit string) *evidenceDiff {
	if diff, ok := s.evidence[key]; ok {
		return diff
	}
	diff := &evidenceDiff{set: map[string]bool{}, reached: map[string]bool{}}
	if s.evidenceExceeded {
		return diff
	}
	s.evidence[key] = diff
	if _, err := git(s.ctx, dir, "merge-base", "--is-ancestor", commit, base); err != nil {
		return diff
	}
	out, err := git(s.ctx, dir, "diff", "--no-renames", "--name-only", "-z", commit, base, "--")
	if err != nil {
		return diff
	}
	changed := slices.DeleteFunc(strings.Split(string(out), "\x00"), func(p string) bool { return p == "" })
	if s.evidencePaths+len(changed) > MaxSelectionEvidencePaths {
		s.evidenceExceeded, s.evidence, s.evidencePaths = true, map[string]*evidenceDiff{}, 0
		return &evidenceDiff{}
	}
	s.evidencePaths += len(changed)
	diff.ok = true
	for _, p := range changed {
		diff.set[prefix+p] = true
		diff.global = diff.global || s.global(prefix+p)
	}
	if prefix == "" {
		diff.changedBy = changedUnits(s.in.Graph, changed)
	}
	return diff
}

// reachedSince memoizes per evidence commit until MaxSelectionEvidencePaths answers are cached in
// total, then answers uncached; the changed-path set replaces reach's linear membership scan.
func (s *e2eSelector) reachedSince(diff *evidenceDiff, p string) bool {
	if in, ok := diff.reached[p]; ok {
		return in
	}
	in := diff.set[p] || (!s.qualified(p) && reach(s.in.Graph, diff.changedBy, nil, p) != nil)
	if s.reachMemo < MaxSelectionEvidencePaths {
		diff.reached[p] = in
		s.reachMemo++
	}
	return in
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
