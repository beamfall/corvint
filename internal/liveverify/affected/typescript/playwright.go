package typescript

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

const (
	PlaywrightProfile           = "playwright-affected/0"
	PlaywrightFallbackNone      = "NONE"
	PlaywrightFallbackFullSuite = "FULL_RELEVANT_SUITE"
	PlaywrightAxisSelection     = "SELECTION"
	PlaywrightAxisExecution     = "EXECUTION"
)

const (
	PlaywrightUnknownConfigSyntax        = "playwright:config-syntax-unresolved"
	PlaywrightUnknownProjectSet          = "playwright:project-set-unresolved"
	PlaywrightUnknownProjectMembership   = "playwright:project-membership-unresolved"
	PlaywrightUnknownProjectDependency   = "playwright:project-dependency-unresolved"
	PlaywrightUnknownBrowserIdentity     = "playwright:browser-identity-unresolved"
	PlaywrightUnknownDynamicSource       = "playwright:dynamic-source-reachability"
	PlaywrightUnknownExternalApplication = "playwright:external-application-state"
	PlaywrightUnknownBrowserExecutable   = "playwright:browser-executable-unresolved"
	PlaywrightUnknownUseValueUnverified  = "playwright:use-value-unverified"
	PlaywrightUnknownDiscoveryOmission   = "playwright:discovery-omission-unverified"
)

type PlaywrightConfigIdentity struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// playwrightMetadataDynamic ends the detail of the config-syntax unknown a project whose metadata
// is not a static literal raises; closures widen on it instead of failing (PWR-V0-185).
const playwrightMetadataDynamic = " metadata is dynamic"

type PlaywrightProject struct {
	Argv           []string `json:"argv"`
	Browser        string   `json:"browser"`
	ConfigFragment string   `json:"configFragmentSha256"`
	Dependencies   []string `json:"dependencies"`
	Device         string   `json:"device"`
	Grep           string   `json:"grep"`
	GrepInvert     string   `json:"grepInvert"`
	Metadata       string   `json:"metadataSha256"`
	Name           string   `json:"name"`
	Teardown       string   `json:"teardown"`
	TestDir        string   `json:"testDir"`
	TestIgnore     []string `json:"testIgnore"`
	TestMatch      []string `json:"testMatch"`

	matchers []playwrightMatcher
	ignores  []playwrightMatcher
	// hasGrep and hasGrepInvert record that the project declares the member, even when its
	// value is dynamic and Grep or GrepInvert is therefore empty (TJAA-V0-021).
	hasGrep, hasGrepInvert bool
	// unresolved holds the selection unknowns scoped to this one project, so advisory narrowing
	// can keep the projects that resolve statically (TJAA-V0-032).
	unresolved []PlaywrightUnknown
	// execution holds EXECUTION-axis unknowns of this project's `use`, such as a computed
	// launchOptions.executablePath, which change the binary but not test membership (TJAA-V0-035).
	execution []PlaywrightUnknown
	// executable is the computed executablePath expression behind execution, or "" (AFP-V0-384).
	executable string
	// Executable records how a caller-observed environment variable resolved the computed
	// executablePath; it is absent unless --playwright-env resolved it (AFP-V0-385).
	Executable *PlaywrightExecutable `json:"executable,omitzero"`
}

type PlaywrightWitness struct {
	DirtyPath string   `json:"dirtyPath"`
	Kind      string   `json:"kind"`
	Via       []string `json:"via"`
}

type PlaywrightSelection struct {
	Argv    []string          `json:"argv"`
	Browser string            `json:"browser"`
	Device  string            `json:"device"`
	ID      string            `json:"id"`
	Project string            `json:"project"`
	Test    string            `json:"test"`
	Witness PlaywrightWitness `json:"witness"`
}

type PlaywrightExclusion struct {
	// Unreached names the PlaywrightUnreachedPathsPath declaration whose matched dirty paths would
	// otherwise be an out-of-suite source unknown; it is present only on a plan that applied one
	// (TJAA-V0-107).
	Unreached string `json:"unreached,omitzero"`
	// Declaration names the project-owned path declaration whose matched dirty paths would
	// otherwise leave the plan unbounded; it is present only on a plan that applied one
	// (TJAA-V0-101).
	Declaration string `json:"declaration,omitzero"`
	ID          string `json:"id"`
	Project     string `json:"project"`
	Reason      string `json:"reason"`
	Test        string `json:"test"`
}

type PlaywrightUnknown struct {
	Axis   string `json:"axis"`
	Detail string `json:"detail"`
	Reason string `json:"reason"`
}

type PlaywrightPlan struct {
	Discovery    PlaywrightDiscoverySummary `json:"discovery"`
	FallbackArgv []string                   `json:"fallbackArgv"`
	Config       PlaywrightConfigIdentity   `json:"config"`
	// Declaration records the .corvint/documentation-paths.json declaration that matched a dirty
	// path, its SHA-256, the matched paths and whether the plan applied or refused it; it is
	// present only when it matched one (TJAA-V0-101).
	Declaration  *PlaywrightPathDeclaration `json:"declaration,omitzero"`
	Dirty        []string                   `json:"dirty"`
	Excluded     []PlaywrightExclusion      `json:"excluded"`
	Fallback     string                     `json:"fallback"`
	GraphDigest  string                     `json:"graphDigest"`
	Projects     []PlaywrightProject        `json:"projects"`
	Scope        string                     `json:"scope"`
	Selected     []PlaywrightSelection      `json:"selected"`
	SourceDigest string                     `json:"sourceDigest"`
	Unknown      []PlaywrightUnknown        `json:"unknown"`
	// Narrowed is advisory evidence present only beside FULL_RELEVANT_SUITE; it never changes
	// Scope, Fallback, Selected or Excluded (TJAA-V0-030).
	Narrowed *PlaywrightNarrowing `json:"narrowed,omitzero"`
	// Environment holds the variables the caller named with --playwright-env, present only when
	// it named one (AFP-V0-383).
	Environment []PlaywrightEnvironmentInput `json:"environment,omitzero"`
	// EnvironmentFiles are the env files the config's modelled dotenv loads read, in load order,
	// present only when the plan records an environment and the config makes one (AFP-V0-406).
	EnvironmentFiles []PlaywrightEnvironmentFile `json:"environmentFiles,omitzero"`
	// Generated records the .corvint/generated-paths.json declaration whose entries the source
	// walk excluded, present only when the repository declares one (AFP-V0-444).
	Generated *PlaywrightGenerated `json:"generated,omitzero"`
	// Informational lists the shared TypeScript graph frontier reasons that cannot change this
	// selection, each with its sites; they never widen the plan (TJAA-V0-109).
	Informational []PlaywrightUnknown `json:"informational,omitzero"`
	// Unreached records the .corvint/playwright-unreached-paths.json declaration that matched a
	// dirty path with no Playwright test, its SHA-256, the matched paths and whether the plan
	// applied or refused it; it is present only when it matched one (TJAA-V0-107).
	Unreached *PlaywrightPathDeclaration `json:"unreached,omitzero"`
	// NonTestAssets records the .corvint/non-test-asset-paths.json declaration, present only when
	// the repository declares one (TJAA-V0-111).
	NonTestAssets *PlaywrightGenerated `json:"nonTestAssets,omitzero"`
}

type playwrightMatcher struct {
	raw string
	re  *regexp.Regexp
}

type observedTypeScript struct {
	result affected.Result
}

func (observedTypeScript) Name() string                     { return "typescript" }
func (observedTypeScript) Owns(relative string) bool        { return New().Owns(relative) }
func (observedTypeScript) CrossLanguageFrontiers() []string { return New().CrossLanguageFrontiers() }
func (language observedTypeScript) Units(context.Context, string) (affected.Result, error) {
	return language.result, nil
}

// selectPlaywrightStatic computes candidates; only receipt reconciliation may
// expose these candidates through the public selection API.
func selectPlaywrightStatic(root, configPath string, dirty []string) (PlaywrightPlan, error) {
	plan, _, err := selectPlaywrightStaticDraft(root, configPath, dirty, nil)
	return plan, err
}

// selectPlaywrightStaticDraft also returns the advisory narrowing draft that SelectPlaywright
// publishes only when the final plan falls back to the full suite (TJAA-V0-030).
func selectPlaywrightStaticDraft(root, configPath string, dirty []string, environment []PlaywrightEnvironmentInput) (PlaywrightPlan, playwrightNarrowingDraft, error) {
	none := playwrightNarrowingDraft{}
	prepared, err := preparePlaywrightGraph(root, configPath, environment)
	if err != nil {
		return PlaywrightPlan{}, none, err
	}
	sourceDigest, configSum, projects, globalTestDir := prepared.sourceDigest, prepared.configSum, prepared.projects, prepared.globalTestDir
	configUnknown, environment, environmentFiles := prepared.configUnknown, prepared.environment, prepared.environmentFiles
	result, sites, observed, units := prepared.result, prepared.sites, prepared.observed, prepared.units
	filtered, graph := prepared.filtered, prepared.graph
	normalizedDirty := affected.NormalizePaths(dirty)
	base := affected.Select(graph, normalizedDirty)
	// Only a frontier reason that can change the Playwright answer widens the plan; the rest are
	// reported as informational (TJAA-V0-108, TJAA-V0-109).
	closure := newPlaywrightClosure(graph, configPath, nil, sites)
	informational := playwrightInformationalFrontier(closure, filtered.Frontier, sites, normalizedDirty, projects, root, globalTestDir)
	closure.frontier = relevantPlaywrightFrontier(filtered.Frontier, informational)
	selectionUnknown, executionUnknown := classifyPlaywrightFrontier(relevantPlaywrightFrontier(result.Frontier, informational), sites)
	selectionUnknown = append(selectionUnknown, configUnknown...)
	// A dirty path outside the TypeScript graph is an out-of-suite unknown in its own right, so
	// fail closed never depends on another frontier (TJAA-V0-121); one the repository declares
	// unreached, and the plan applies, no longer leaves the shared graph frontier unresolved.
	unreached, outOfSuite, appliedUnowned, err := playwrightOutOfSuiteSources(root, closure, base, normalizedDirty, projects, globalTestDir)
	if err != nil {
		return PlaywrightPlan{}, none, err
	}
	if graphFrontierUnresolved(base, informational, appliedUnowned) {
		selectionUnknown = append(selectionUnknown, PlaywrightUnknown{Axis: PlaywrightAxisSelection, Reason: PlaywrightUnknownDynamicSource, Detail: graphFrontierDetail(base, sites, informational, appliedUnowned)})
	}
	declaration, contradicted, err := playwrightDeclaredPaths(root, configPath, graph, base, closure)
	if err != nil {
		return PlaywrightPlan{}, none, err
	}
	selectionUnknown = append(selectionUnknown, contradicted...)
	selectionUnknown = append(selectionUnknown, outOfSuite...)
	selected := selectPlaywrightUnits(configPath, base, projects, units, normalizedDirty)
	draft, err := draftPlaywrightNarrowing(root, configPath, observed, graph, base, configUnknown, projects, units, normalizedDirty, informational)
	if err != nil {
		return PlaywrightPlan{}, none, err
	}
	selectionUnknown = canonicalPlaywrightUnknowns(selectionUnknown)
	if len(configUnknown) != 0 {
		units = []PlaywrightSelection{}
		selected = []PlaywrightSelection{}
	} else if len(selectionUnknown) != 0 {
		selected = allPlaywrightUnits(units)
	}
	selected = expandPlaywrightProjectEdges(selected, projects, units)
	for _, project := range projects {
		executionUnknown = append(executionUnknown, project.execution...)
	}
	unknown := append(selectionUnknown, executionUnknown...)
	unknown = append(unknown, PlaywrightUnknown{Axis: PlaywrightAxisExecution, Reason: PlaywrightUnknownExternalApplication, Detail: "application state and runtime feature flags are not observed by static affected selection"})
	unknown = canonicalPlaywrightUnknowns(unknown)
	plan := PlaywrightPlan{
		Config:      PlaywrightConfigIdentity{Path: configPath, SHA256: hex.EncodeToString(configSum[:])},
		Declaration: declaration,
		Dirty:       normalizedDirty, Excluded: []PlaywrightExclusion{}, Fallback: PlaywrightFallbackNone,
		Projects: publicPlaywrightProjects(projects), Scope: affected.ScopeBounded,
		Selected: selected, SourceDigest: sourceDigest, Unknown: unknown,
		FallbackArgv: []string{}, Unreached: unreached,
		Informational: playwrightInformationalUnknowns(filtered.Frontier, sites, informational),
	}
	if sum, entries, declErr := affected.ReadGeneratedDeclaration(root); declErr != nil {
		return PlaywrightPlan{}, none, declErr
	} else if entries != nil {
		plan.Generated = &PlaywrightGenerated{Path: affected.GeneratedPathsPath, Paths: entries, SHA256: sum}
	}
	if sum, entries, declErr := readPathDeclaration(affected.DiskSource(root), NonTestAssetPathsPath, NonTestAssetPathsProfile); declErr != nil {
		return PlaywrightPlan{}, none, declErr
	} else if entries != nil {
		plan.NonTestAssets = &PlaywrightGenerated{Path: NonTestAssetPathsPath, Paths: entries, SHA256: sum}
	}
	if len(environment) != 0 {
		plan.Environment = canonicalPlaywrightEnvironment(environment)
		plan.EnvironmentFiles = environmentFiles
	}
	if len(selectionUnknown) != 0 {
		plan.Scope = affected.ScopeUnknown
		plan.Fallback = PlaywrightFallbackFullSuite
	}
	plan.Excluded = excludedPlaywrightUnits(units, selected)
	if declaration != nil && declaration.State == PlaywrightDeclarationApplied {
		for index := range plan.Excluded {
			plan.Excluded[index].Declaration = declaration.Path
		}
	}
	if unreached != nil && unreached.State == PlaywrightDeclarationApplied {
		for index := range plan.Excluded {
			plan.Excluded[index].Unreached = unreached.Path
		}
	}
	plan.GraphDigest, err = playwrightGraphDigest(graph.Digest(), plan.Config, plan.Projects, units)
	if err != nil {
		return PlaywrightPlan{}, none, err
	}
	after, err := ObservePlaywrightSources(root, configPath)
	if err != nil {
		return PlaywrightPlan{}, none, err
	}
	if after != sourceDigest {
		return PlaywrightPlan{}, none, errors.New("Playwright source changed while the plan was compiled")
	}
	return plan, draft, nil
}

// ObservePlaywrightSources returns a bounded content identity for every input
// the TypeScript observer may use, including every tsconfig*.json an `extends` may name
// (TJAA-V0-114), every jsconfig*.json, which Playwright reads for aliases (AFP-V0-418), plus
// the explicitly selected config.
func ObservePlaywrightSources(root, configPath string) (string, error) {
	files, generated, err := playwrightBoundSourceFiles(root)
	if err != nil {
		return "", err
	}
	if !containsString(files, configPath) {
		files = append(files, configPath)
		sort.Strings(files)
	}
	// An over-bound file is bound by a streamed hash of its bytes rather than refused, so one
	// generated bundle is a per-path unknown and never aborts the run (AFP-V0-444).
	type sourceIdentity struct {
		OverBound bool   `json:"over_bound,omitzero"`
		Path      string `json:"path"`
		SHA256    string `json:"sha256"`
	}
	identities := make([]sourceIdentity, 0, len(files)+1)
	if generated != nil {
		identities = append(identities, sourceIdentity{Path: generated.Path, SHA256: generated.SHA256})
	}
	// The path declarations that classify dirty paths or test assets are plan inputs too
	// (TJAA-V0-107, TJAA-V0-111).
	for _, declared := range [][2]string{{NonTestAssetPathsPath, NonTestAssetPathsProfile}, {PlaywrightUnreachedPathsPath, PlaywrightUnreachedPathsProfile}} {
		sum, entries, declErr := readPathDeclaration(affected.DiskSource(root), declared[0], declared[1])
		if declErr != nil {
			return "", declErr
		}
		if entries != nil {
			identities = append(identities, sourceIdentity{Path: declared[0], SHA256: sum})
		}
	}
	// The runtime-edges declaration changes which specs a plan selects, so its presence and
	// bytes are a plan input too (AFP-V0-819); without the file the identity is unchanged.
	switch body, readErr := affected.ReadSource(root, affected.RuntimeEdgesPath); {
	case readErr == nil:
		sum := sha256.Sum256(body)
		identities = append(identities, sourceIdentity{Path: affected.RuntimeEdgesPath, SHA256: hex.EncodeToString(sum[:])})
	case !errors.Is(readErr, fs.ErrNotExist):
		return "", readErr
	}
	for _, relative := range files {
		body, readErr := affected.ReadSource(root, relative)
		if errors.Is(readErr, affected.ErrWalkLimit) && relative != configPath {
			sum, hashErr := affected.HashOverBoundSource(root, relative)
			if hashErr != nil {
				return "", hashErr
			}
			identities = append(identities, sourceIdentity{OverBound: true, Path: relative, SHA256: sum})
			continue
		}
		if readErr != nil {
			return "", readErr
		}
		sum := sha256.Sum256(body)
		identities = append(identities, sourceIdentity{Path: relative, SHA256: hex.EncodeToString(sum[:])})
	}
	body, err := json.Marshal(identities, json.Deterministic(true))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return "playwright-sources:sha256:" + hex.EncodeToString(sum[:]), nil
}

// playwrightObservedName is observedName plus jsconfig.json, an alias config Playwright reads,
// and the jsconfig*.json spellings its `extends` may name (TJAA-V0-114).
func playwrightObservedName(name string) bool {
	return observedName(name) || javaScriptConfigName(name)
}

// javaScriptConfigName reports whether base is jsconfig.json or a sibling spelling an `extends`
// chain may name (TJAA-V0-114).
func javaScriptConfigName(base string) bool {
	return strings.HasPrefix(base, "jsconfig") && strings.HasSuffix(base, ".json")
}

func hasPlaywrightUnknownReason(unknown []PlaywrightUnknown, reason string) bool {
	for _, entry := range unknown {
		if entry.Reason == reason {
			return true
		}
	}
	return false
}

func classifyPlaywrightFrontier(frontier []string, sites frontierSet) (selection, execution []PlaywrightUnknown) {
	for _, reason := range frontier {
		unknown := PlaywrightUnknown{Axis: PlaywrightAxisSelection, Reason: reason, Detail: frontierDetail(sites[reason])}
		switch reason {
		case FrontierOverBoundSource:
			unknown.Detail += "; " + affected.SourceBoundRemedy
			selection = append(selection, unknown)
		case FrontierE2ERuntimeDependency:
			unknown.Axis = PlaywrightAxisExecution
			unknown.Detail = playwrightRuntimeDetail(sites[reason])
			execution = append(execution, unknown)
		case FrontierConfig:
			// The selected Playwright config is parsed by the closed profile below.
		default:
			selection = append(selection, unknown)
		}
	}
	return selection, execution
}

// playwrightRuntimeDetail is frontierDetail for the e2e-runtime-dependency sites with every site
// whose token is the Playwright runner itself (an `@playwright/test` import, or the runner identity
// of a file that reaches it through another module) folded into one count: in Playwright mode the
// runner is the framework the plan runs, so naming it at each test file adds no evidence. Any
// other runtime dependency, such as puppeteer, stays named (AFP-V0-426).
func playwrightRuntimeDetail(sites map[string]string) string {
	rest := map[string]string{}
	runner := 0
	for site, token := range sites {
		bare := strings.TrimPrefix(token, pinnedSite)
		if bare == runnerPlaywright || bare == "@playwright/test" || strings.HasPrefix(bare, "@playwright/test/") {
			runner++
			continue
		}
		rest[site] = token
	}
	if runner == 0 {
		return frontierDetail(sites)
	}
	summary := strconv.Itoa(runner) + " test file(s) depend on the Playwright runner @playwright/test"
	if len(rest) == 0 {
		return "shared TypeScript graph frontier: " + summary
	}
	return frontierDetail(rest) + "; " + summary
}

// playwrightFrontierSiteBound caps the sites one frontier detail names; the rest are counted.
const playwrightFrontierSiteBound = 8

// frontierDetail names the sorted sites that raised one shared TypeScript graph frontier reason,
// `path:line` or `path`, each followed by its backquoted token when one is known, at most
// playwrightFrontierSiteBound of them (TJAA-V0-040, TJAA-V0-052).
func frontierDetail(sites map[string]string) string {
	if named := namedFrontierSites(sites); named != "" {
		return "shared TypeScript graph frontier at " + named
	}
	return "shared TypeScript graph frontier"
}

// namedFrontierSites lists at most playwrightFrontierSiteBound of sites, pinned first and then
// sorted, each with its backquoted token, and counts the rest; it is empty for no site.
func namedFrontierSites(sites map[string]string) string {
	ordered := orderedSites(sites)
	if len(ordered) == 0 {
		return ""
	}
	count := boundedSites(sites, ordered, playwrightFrontierSiteBound)
	named := make([]string, 0, count)
	for _, site := range ordered[:count] {
		named = append(named, siteWithToken(site, sites[site]))
	}
	detail := strings.Join(named, ", ")
	if extra := len(ordered) - count; extra > 0 {
		detail += " and " + strconv.Itoa(extra) + " more"
	}
	return detail
}

// graphFrontierDetail names what left the shared TypeScript graph's selection unresolved: each
// widening unknown of base other than NO_SELECTABLE_TEST, as its reason and path, or for a
// LANGUAGE_FRONTIER reason the sites that raised it, at most playwrightFrontierSiteBound entries
// with the rest counted (AFP-V0-424). An applied unreached declaration's paths are left out
// (TJAA-V0-121).
func graphFrontierDetail(base affected.Plan, sites frontierSet, informational, appliedUnowned map[string]bool) string {
	entries := []string{}
	for _, unknown := range base.Unknown {
		switch {
		case unknown.Reason == affected.UnknownNoSelectableTest:
		case unknown.Reason == affected.UnknownUnownedDirtyPath && appliedUnowned[unknown.Detail]:
		case unknown.Reason == affected.UnknownLanguageFrontier && informational[unknown.Detail]:
		case unknown.Reason == affected.UnknownLanguageFrontier && namedFrontierSites(sites[unknown.Detail]) != "":
			entries = append(entries, unknown.Detail+" at "+namedFrontierSites(sites[unknown.Detail]))
		case unknown.Hint != "":
			// AFP-V0-494: the base plan's operator hint travels with its entry.
			entries = append(entries, unknown.Reason+" "+unknown.Detail+" ("+unknown.Hint+")")
		default:
			entries = append(entries, unknown.Reason+" "+unknown.Detail)
		}
	}
	detail := "the shared TypeScript graph has an unresolved selection frontier"
	if len(entries) == 0 {
		return detail
	}
	count := min(len(entries), playwrightFrontierSiteBound)
	detail += ": " + strings.Join(entries[:count], "; ")
	if extra := len(entries) - count; extra > 0 {
		detail += " and " + strconv.Itoa(extra) + " more"
	}
	return detail
}

func playwrightSourcePaths(result affected.Result) []string {
	set := map[string]bool{}
	for _, unit := range result.Units {
		for _, test := range append(append([]string{}, unit.Tests...), unit.Sources...) {
			set[test] = true
		}
	}
	return sortedKeys(set)
}

func bindPlaywrightTestMembership(result *affected.Result, units []PlaywrightSelection, configPath string) {
	tests := map[string]bool{}
	for _, unit := range units {
		tests[unit.Test] = true
	}
	configID := ""
	for _, unit := range result.Units {
		if containsString(unit.Sources, configPath) || containsString(unit.Tests, configPath) {
			configID = unit.ID
		}
	}
	for index := range result.Units {
		unit := &result.Units[index]
		paths := append(append([]string{}, unit.Sources...), unit.Tests...)
		unit.Sources, unit.Tests = nil, nil
		for _, name := range paths {
			if tests[name] {
				unit.Tests = append(unit.Tests, name)
			} else {
				unit.Sources = append(unit.Sources, name)
			}
		}
		// The explicit config may have an unconventional name that nearestConfigs
		// cannot discover. Its hooks govern every admitted physical test.
		if len(unit.Tests) != 0 && configID != "" && unit.ID != configID {
			unit.Imports = sortedUnique(append(unit.Imports, configID))
		}
	}
}

func playwrightUnits(root, configPath string, projects []PlaywrightProject, globalTestDir string, tests []string) []PlaywrightSelection {
	units := []PlaywrightSelection{}
	for _, project := range projects {
		for _, test := range tests {
			if !playwrightProjectOwns(root, project, globalTestDir, test) {
				continue
			}
			units = append(units, newPlaywrightSelection(configPath, project, test, PlaywrightWitness{}))
		}
	}
	sort.Slice(units, func(i, j int) bool { return units[i].ID < units[j].ID })
	return units
}

func newPlaywrightSelection(configPath string, project PlaywrightProject, test string, witness PlaywrightWitness) PlaywrightSelection {
	argv := []string{"npx", "playwright", "test", test, "--config=" + configPath, "--project=" + project.Name}
	return PlaywrightSelection{
		Argv: argv, Browser: project.Browser, Device: project.Device,
		ID:      "typescript:playwright:" + playwrightIDEscape(project.Name) + ":" + test,
		Project: project.Name, Test: test, Witness: witness,
	}
}

func selectPlaywrightUnits(configPath string, base affected.Plan, projects []PlaywrightProject, units []PlaywrightSelection, dirty []string) []PlaywrightSelection {
	all := map[string]PlaywrightSelection{}
	for _, unit := range units {
		all[unit.ID] = unit
	}
	selected := map[string]PlaywrightSelection{}
	if containsString(dirty, configPath) {
		for id, unit := range all {
			unit.Witness = PlaywrightWitness{Kind: "CONFIG_CHANGE", DirtyPath: configPath, Via: []string{configPath}}
			selected[id] = unit
		}
		return sortedPlaywrightSelections(selected)
	}
	for _, physical := range base.Selected {
		for _, test := range physical.Tests {
			for _, project := range projects {
				id := "typescript:playwright:" + playwrightIDEscape(project.Name) + ":" + test
				unit, exists := all[id]
				if !exists {
					continue
				}
				unit.Witness = PlaywrightWitness{Kind: physical.Witness.Kind, DirtyPath: physical.Witness.DirtyPath, Via: append([]string(nil), physical.Witness.Via...)}
				selected[id] = unit
			}
		}
	}
	return sortedPlaywrightSelections(selected)
}

func allPlaywrightUnits(units []PlaywrightSelection) []PlaywrightSelection {
	result := append([]PlaywrightSelection(nil), units...)
	for index := range result {
		result[index].Witness = PlaywrightWitness{Kind: "FULL_SUITE_WIDENING", Via: []string{}}
	}
	return result
}

func expandPlaywrightProjectEdges(selected []PlaywrightSelection, projects []PlaywrightProject, units []PlaywrightSelection) []PlaywrightSelection {
	byProject := map[string][]PlaywrightSelection{}
	for _, unit := range units {
		byProject[unit.Project] = append(byProject[unit.Project], unit)
	}
	projectByName := map[string]PlaywrightProject{}
	dependents := map[string][]string{}
	for _, project := range projects {
		projectByName[project.Name] = project
		for _, dependency := range project.Dependencies {
			dependents[dependency] = append(dependents[dependency], project.Name)
		}
	}
	result := map[string]PlaywrightSelection{}
	type queuedProject struct {
		name             string
		expandDependents bool
	}
	queue := []queuedProject{}
	for _, unit := range selected {
		result[unit.ID] = unit
		queue = append(queue, queuedProject{name: unit.Project, expandDependents: true})
	}
	seenProject := map[queuedProject]bool{}
	for len(queue) != 0 {
		current := queue[0]
		queue = queue[1:]
		if seenProject[current] {
			continue
		}
		seenProject[current] = true
		project := projectByName[current.name]
		type relation struct {
			name             string
			expandDependents bool
		}
		related := make([]relation, 0, len(project.Dependencies)+len(dependents[current.name])+1)
		for _, dependency := range project.Dependencies {
			related = append(related, relation{name: dependency})
		}
		if current.expandDependents {
			for _, dependent := range dependents[current.name] {
				related = append(related, relation{name: dependent, expandDependents: true})
			}
		}
		if project.Teardown != "" {
			related = append(related, relation{name: project.Teardown})
		}
		for _, next := range related {
			queue = append(queue, queuedProject{name: next.name, expandDependents: next.expandDependents})
			for _, unit := range byProject[next.name] {
				if _, exists := result[unit.ID]; exists {
					continue
				}
				unit.Witness = PlaywrightWitness{Kind: "PROJECT_RELATION", DirtyPath: current.name, Via: []string{current.name, next.name}}
				result[unit.ID] = unit
			}
		}
	}
	return sortedPlaywrightSelections(result)
}

func excludedPlaywrightUnits(units, selected []PlaywrightSelection) []PlaywrightExclusion {
	chosen := map[string]bool{}
	for _, unit := range selected {
		chosen[unit.ID] = true
	}
	excluded := []PlaywrightExclusion{}
	for _, unit := range units {
		if !chosen[unit.ID] {
			excluded = append(excluded, PlaywrightExclusion{ID: unit.ID, Project: unit.Project, Reason: "NO_DEPENDENCY_PATH_TO_DIRTY_UNIT", Test: unit.Test})
		}
	}
	return excluded
}

func publicPlaywrightProjects(projects []PlaywrightProject) []PlaywrightProject {
	public := append([]PlaywrightProject(nil), projects...)
	for index := range public {
		public[index].matchers = nil
		public[index].ignores = nil
		public[index].unresolved = nil
	}
	return public
}

func playwrightGraphDigest(base string, config PlaywrightConfigIdentity, projects []PlaywrightProject, units []PlaywrightSelection) (string, error) {
	body, err := json.Marshal(struct {
		Base     string                   `json:"base"`
		Config   PlaywrightConfigIdentity `json:"config"`
		Projects []PlaywrightProject      `json:"projects"`
		Units    []PlaywrightSelection    `json:"units"`
	}{Base: base, Config: config, Projects: projects, Units: units}, json.Deterministic(true))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return "playwright-affected-graph:sha256:" + hex.EncodeToString(sum[:]), nil
}

func (plan PlaywrightPlan) Canonical() ([]byte, error) {
	return json.Marshal(plan, json.Deterministic(true))
}

func parsePlaywrightConfig(configPath, source string) ([]PlaywrightProject, string, []PlaywrightUnknown) {
	clean, err := stripComments(source, false)
	if err != nil {
		return nil, "", []PlaywrightUnknown{selectionUnknown(PlaywrightUnknownConfigSyntax, "config tokenization failed")}
	}
	object, ok := playwrightConfigObject(clean)
	if !ok {
		return nil, "", []PlaywrightUnknown{selectionUnknown(PlaywrightUnknownConfigSyntax, "exported config is not one static object")}
	}
	properties, ok := playwrightObjectProperties(object)
	if !ok {
		return nil, "", []PlaywrightUnknown{selectionUnknown(PlaywrightUnknownConfigSyntax, "top-level config object is dynamic")}
	}
	uses := []string{properties["use"]}
	if items, ok := playwrightArrayItems(properties["projects"]); ok {
		for _, item := range items {
			if project, ok := playwrightObjectProperties(item); ok {
				uses = append(uses, project["use"])
			}
		}
	}
	scope := newPlaywrightPureScopeConfined(clean, playwrightNonIdentityUseSpans(clean, uses))
	globalTestDir := "."
	unknown := []PlaywrightUnknown{}
	if raw, exists := properties["testDir"]; exists {
		value, literal := playwrightString(raw)
		if joined, inside := playwrightJoinInsideRoot(path.Dir(configPath), value); !literal || !inside {
			unknown = append(unknown, selectionUnknown(PlaywrightUnknownProjectMembership, "global testDir is not one static relative path"))
		} else {
			globalTestDir = joined
		}
	}
	for _, member := range []string{"grep", "grepInvert", "testIgnore", "testMatch", "tsconfig"} {
		if _, exists := properties[member]; exists {
			unknown = append(unknown, selectionUnknown(PlaywrightUnknownConfigSyntax, "global "+member+" is outside the static project subset"))
		}
	}
	rawProjects, exists := properties["projects"]
	if !exists {
		return nil, globalTestDir, append(unknown, selectionUnknown(PlaywrightUnknownProjectSet, "projects is absent"))
	}
	items, ok := playwrightArrayItems(rawProjects)
	if !ok || len(items) == 0 {
		return nil, globalTestDir, append(unknown, selectionUnknown(PlaywrightUnknownProjectSet, "projects is not one nonempty static array"))
	}
	projects := make([]PlaywrightProject, 0, len(items))
	names := map[string]bool{}
	for _, item := range items {
		if strings.TrimSpace(item) == "" {
			continue
		}
		project, itemUnknown := parsePlaywrightProject(scope, configPath, globalTestDir, item, properties["use"])
		unknown = append(unknown, itemUnknown...)
		if project.Name == "" {
			continue
		}
		project.unresolved = itemUnknown
		if names[project.Name] {
			unknown = append(unknown, selectionUnknown(PlaywrightUnknownProjectSet, "duplicate project name "+project.Name))
			continue
		}
		names[project.Name] = true
		projects = append(projects, project)
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })
	for index, project := range projects {
		for _, dependency := range append(append([]string{}, project.Dependencies...), project.Teardown) {
			if dependency != "" && !names[dependency] {
				missing := selectionUnknown(PlaywrightUnknownProjectDependency, project.Name+" references unknown project "+dependency)
				unknown = append(unknown, missing)
				projects[index].unresolved = append(projects[index].unresolved, missing)
			}
		}
	}
	if playwrightProjectCycle(projects) {
		unknown = append(unknown, selectionUnknown(PlaywrightUnknownProjectDependency, "project dependency or teardown graph has a cycle"))
	}
	if len(projects) == 0 {
		unknown = append(unknown, selectionUnknown(PlaywrightUnknownProjectSet, "no static project identity was recovered"))
	}
	return projects, globalTestDir, canonicalPlaywrightUnknowns(unknown)
}

// playwrightNonIdentityUseSpans returns the spans of clean that hold one value of a non-identity
// key of a `use` layer in uses (TJAA-V0-060). A layer is located only when its text occurs once in
// clean, so each span is exact; any other layer contributes none.
func playwrightNonIdentityUseSpans(clean string, uses []string) [][2]int {
	spans := [][2]int{}
	for _, raw := range uses {
		raw = strings.TrimSpace(raw)
		if raw == "" || strings.Count(clean, raw) != 1 {
			continue
		}
		if _, exact := playwrightObject(raw); !exact || len(raw) < 2 || raw[len(raw)-1] != '}' {
			continue
		}
		inner := raw[1 : len(raw)-1]
		innerAt := strings.Index(clean, raw) + 1
		items, split := playwrightSplitTopLevel(inner)
		if !split {
			continue
		}
		cursor := 0
		for _, item := range items {
			found := strings.Index(inner[cursor:], item)
			if found < 0 {
				break
			}
			itemAt := cursor + found
			cursor = itemAt + len(item)
			trimmed := strings.TrimSpace(item)
			colon := playwrightTopLevelColon(trimmed)
			if trimmed == "" || strings.HasPrefix(trimmed, "...") || colon < 0 {
				continue
			}
			key := strings.TrimSpace(trimmed[:colon])
			if quoted, literal := playwrightString(key); literal {
				key = quoted
			} else if _, end := playwrightIdentifier(key, 0); key == "" || end != len(key) {
				continue
			}
			if key == "browserName" || slices.Contains(PlaywrightUseIdentityKeys, key) {
				continue
			}
			start := innerAt + itemAt + strings.Index(item, trimmed)
			spans = append(spans, [2]int{start + colon + 1, start + len(trimmed)})
		}
	}
	return spans
}

func parsePlaywrightProject(scope playwrightPureScope, configPath, globalTestDir, raw, globalUse string) (PlaywrightProject, []PlaywrightUnknown) {
	properties, ok := playwrightObjectProperties(raw)
	if !ok {
		return PlaywrightProject{}, []PlaywrightUnknown{selectionUnknown(PlaywrightUnknownProjectSet, "project entry is not a static object")}
	}
	name, ok := playwrightString(properties["name"])
	if !ok || name == "" {
		return PlaywrightProject{}, []PlaywrightUnknown{selectionUnknown(PlaywrightUnknownProjectSet, "project name is not one nonempty string")}
	}
	project := PlaywrightProject{Name: name, TestDir: globalTestDir, TestIgnore: []string{}, TestMatch: []string{}, Dependencies: []string{}}
	project.Argv = []string{"npx", "playwright", "test", "--config=" + configPath, "--project=" + name}
	fragmentSum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	if globalUse != "" {
		fragmentSum = sha256.Sum256([]byte(strings.TrimSpace(globalUse) + "\n" + strings.TrimSpace(raw)))
	}
	project.ConfigFragment = hex.EncodeToString(fragmentSum[:])
	unknown := []PlaywrightUnknown{}
	if value, exists := properties["testDir"]; exists {
		directory, literal := playwrightString(value)
		if joined, inside := playwrightJoinInsideRoot(path.Dir(configPath), directory); !literal || !inside {
			unknown = append(unknown, selectionUnknown(PlaywrightUnknownProjectMembership, name+" testDir is dynamic"))
		} else {
			project.TestDir = joined
		}
	}
	project.TestMatch, project.matchers, unknown = parsePlaywrightMatchers(name, "testMatch", properties, unknown)
	project.TestIgnore, project.ignores, unknown = parsePlaywrightMatchers(name, "testIgnore", properties, unknown)
	if value, exists := properties["dependencies"]; exists {
		dependencies, literal := playwrightStringArray(value)
		if bound, ok := scope.arrays[strings.TrimSpace(value)]; !literal && ok {
			// A const of literal arrays resolves to the union of its branches (TJAA-V0-036).
			dependencies, literal = bound, true
		}
		if !literal {
			unknown = append(unknown, selectionUnknown(PlaywrightUnknownProjectDependency, name+" dependencies are dynamic"))
		} else {
			project.Dependencies = sortedUnique(dependencies)
		}
	}
	if value, exists := properties["teardown"]; exists {
		teardown, literal := playwrightString(value)
		if !literal || teardown == "" {
			unknown = append(unknown, selectionUnknown(PlaywrightUnknownProjectDependency, name+" teardown is dynamic"))
		} else {
			project.Teardown = teardown
		}
	}
	project.Grep, unknown = playwrightStaticIdentity(name, "grep", properties, unknown)
	project.GrepInvert, unknown = playwrightStaticIdentity(name, "grepInvert", properties, unknown)
	_, project.hasGrep = properties["grep"]
	_, project.hasGrepInvert = properties["grepInvert"]
	if metadata, exists := properties["metadata"]; exists {
		if !playwrightStaticValue(metadata) {
			unknown = append(unknown, selectionUnknown(PlaywrightUnknownConfigSyntax, name+playwrightMetadataDynamic))
		} else {
			sum := sha256.Sum256([]byte(strings.TrimSpace(metadata)))
			project.Metadata = hex.EncodeToString(sum[:])
		}
	}
	var block playwrightUseBlock
	var unverified []playwrightUseBlock
	project.Browser, project.Device, project.executable, ok, block, unverified = playwrightInheritedUse(scope, globalUse, properties["use"])
	if !ok {
		project.executable = ""
	}
	if project.executable != "" {
		project.execution = []PlaywrightUnknown{{Axis: PlaywrightAxisExecution, Reason: PlaywrightUnknownBrowserExecutable, Detail: name + " use.launchOptions.executablePath is computed, so the browser binary that runs is not observed"}}
	}
	if ok {
		// TJAA-V0-054: a non-identity value that is not side-effect free does not block the
		// identity, but its evaluation can run code the static model does not observe (a
		// module-resolution hook, a descriptor rewrite), so it keeps the selection uncertain.
		for _, value := range unverified {
			unknown = append(unknown, selectionUnknown(PlaywrightUnknownUseValueUnverified, name+" use value can run code the static selection does not model"+value.detail(scope, configPath)))
		}
	}
	if !ok {
		unknown = append(unknown, selectionUnknown(PlaywrightUnknownBrowserIdentity, name+" use.browserName/device is dynamic or unsupported"+block.detail(scope, configPath)))
	}
	return project, unknown
}

func parsePlaywrightMatchers(project, member string, properties map[string]string, unknown []PlaywrightUnknown) ([]string, []playwrightMatcher, []PlaywrightUnknown) {
	raw, exists := properties[member]
	if !exists {
		return []string{}, nil, unknown
	}
	items, array := playwrightArrayItems(raw)
	if !array {
		items = []string{raw}
	}
	values := []string{}
	matchers := []playwrightMatcher{}
	for _, item := range items {
		matcher, ok := compilePlaywrightMatcher(item)
		if !ok {
			unknown = append(unknown, selectionUnknown(PlaywrightUnknownProjectMembership, project+" "+member+" contains an unsupported matcher"))
			continue
		}
		values = append(values, matcher.raw)
		matchers = append(matchers, matcher)
	}
	sort.Strings(values)
	sort.Slice(matchers, func(i, j int) bool { return matchers[i].raw < matchers[j].raw })
	return values, matchers, unknown
}

func playwrightStaticIdentity(project, member string, properties map[string]string, unknown []PlaywrightUnknown) (string, []PlaywrightUnknown) {
	raw, exists := properties[member]
	if !exists {
		return "", unknown
	}
	static := false
	if _, ok := compilePlaywrightMatcher(raw); ok {
		static = true
	} else if items, array := playwrightArrayItems(raw); array {
		// An array of static matchers is static identity too: Playwright matches any of them.
		static = true
		for _, item := range items {
			if _, ok := compilePlaywrightMatcher(item); !ok {
				static = false
				break
			}
		}
	}
	if !static {
		return "", append(unknown, selectionUnknown(PlaywrightUnknownConfigSyntax, project+" "+member+" is dynamic"))
	}
	return strings.TrimSpace(raw), unknown
}

func playwrightInheritedUseIdentity(scope playwrightPureScope, global, raw string) (browser, device string, computedExecutable, ok bool) {
	browser, device, executable, ok, _, _ := playwrightInheritedUse(scope, global, raw)
	return browser, device, executable != "", ok
}

// playwrightInheritedUse also returns the block that left the identity unresolved, and the
// non-identity values of both layers that are not side-effect free (TJAA-V0-054). executable is
// the computed launchOptions.executablePath expression that applies, or "" (AFP-V0-384).
func playwrightInheritedUse(scope playwrightPureScope, global, raw string) (browser, device, executable string, ok bool, block playwrightUseBlock, unverified []playwrightUseBlock) {
	browser, device, executable, ok, block, unverified = playwrightUseLayer(scope, global, "", "")
	if !ok {
		return "", "", "", false, block, nil
	}
	var localExecutable string
	var local []playwrightUseBlock
	browser, device, localExecutable, ok, block, local = playwrightUseLayer(scope, raw, browser, device)
	unverified = append(unverified, local...)
	if browser == "" && device != "" {
		browser = playwrightDeviceBrowser(device)
	}
	if browser == "" {
		browser = "chromium"
	}
	// A local launchOptions replaces the inherited one, executablePath included.
	if localExecutable != "" || playwrightUseDeclares(raw, "launchOptions") {
		executable = localExecutable
	}
	return browser, device, executable, ok, block, unverified
}

// playwrightUseDeclares reports whether the `use` object literal raw declares key.
func playwrightUseDeclares(raw, key string) bool {
	properties, ok := playwrightObjectProperties(raw)
	_, declared := properties[key]
	return ok && declared
}

// playwrightUseBlock names what left a `use` layer's identity unresolved: why, and the token at
// offset in fragment, a verbatim substring of the layer text raw. root names the scope root
// (devices, process or a numeric conversion) the fragment needed, if any (TJAA-V0-044).
type playwrightUseBlock struct {
	raw, fragment, why, root string
	offset                   int
}

// playwrightUseLayer reads one `use` layer. A device spread keeps its identity only when the
// devices import is sound in scope: any other occurrence of `devices` may rewrite a descriptor.
// A launchOptions whose only computed property is a side-effect-free executablePath keeps the
// identity and reports its expression, an EXECUTION-axis unknown (TJAA-V0-035).
func playwrightUseLayer(scope playwrightPureScope, raw, browser, device string) (string, string, string, bool, playwrightUseBlock, []playwrightUseBlock) {
	browser, device, executable, block, unverified := playwrightUseLayerItems(scope, raw, browser, device)
	ok := block.why == ""
	block.raw = strings.TrimSpace(raw)
	for index := range unverified {
		unverified[index].raw = block.raw
	}
	if !ok {
		executable = ""
	}
	return browser, device, executable, ok, block, unverified
}

func playwrightUseLayerItems(scope playwrightPureScope, raw, browser, device string) (_, _, executable string, block playwrightUseBlock, unverified []playwrightUseBlock) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return browser, device, "", block, nil
	}
	refuse := func(fragment string, offset int, why string) playwrightUseBlock {
		return playwrightUseBlock{fragment: fragment, offset: offset, why: why, root: playwrightScopeRoot(fragment)}
	}
	object, exact := playwrightObject(raw)
	if !exact {
		return "", "", "", refuse(raw, 0, "use is not one static object literal"), nil
	}
	items, split := playwrightSplitTopLevel(object[1 : len(object)-1])
	if !split {
		return "", "", "", refuse(raw, 0, "use does not split into properties"), nil
	}
	seenBrowser := false
	seenDevice := false
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.HasPrefix(item, "...") {
			name, literal := playwrightDeviceSpread(strings.TrimSpace(strings.TrimPrefix(item, "...")))
			switch {
			case !literal:
				return "", "", "", refuse(item, 0, "a spread is not one literal devices[...] descriptor"), nil
			case seenDevice:
				return "", "", "", refuse(item, 0, "a second device spread"), nil
			case !scope.devices:
				block := refuse(item, 0, "the devices import is not sound")
				block.root = "devices"
				return "", "", "", block, nil
			}
			device = name
			seenDevice = true
			if playwrightDeviceBrowser(device) == "" {
				return "", device, "", refuse(item, 0, "device "+strconv.Quote(device)+" has no modelled browser"), nil
			}
			continue
		}
		colon := playwrightTopLevelColon(item)
		if colon < 0 {
			return "", device, "", refuse(item, 0, "a property is not `key: value`"), nil
		}
		key := strings.TrimSpace(item[:colon])
		if quoted, literal := playwrightString(key); literal {
			key = quoted
		} else {
			for index, character := range key {
				if character != '_' && character != '$' && !unicode.IsLetter(character) && (index == 0 || !unicode.IsDigit(character)) {
					return "", device, "", refuse(item, 0, "a property key is computed or not an identifier"), nil
				}
			}
			if key == "" {
				return "", device, "", refuse(item, 0, "a property key is empty"), nil
			}
		}
		if key == "defaultBrowserType" {
			return "", device, "", refuse(item, 0, "defaultBrowserType is not modelled"), nil
		}
		value := strings.TrimSpace(item[colon+1:])
		valueAt := colon + 1 + strings.Index(item[colon+1:], value)
		if key != "browserName" {
			// An identity value must be a static literal. Another option's value (baseURL,
			// storageState, trace, ...) never decides the identity; one whose evaluation can run
			// code or write state is the selection unknown playwright:use-value-unverified
			// (TJAA-V0-054).
			if key == "launchOptions" && !playwrightStaticValue(value) && playwrightComputedExecutableOnly(scope, value) {
				executable = strings.TrimSpace(playwrightLaunchExecutable(value))
				continue
			}
			if slices.Contains(PlaywrightUseIdentityKeys, key) && !playwrightStaticValue(value) {
				return "", device, "", refuse(item, valueAt, "identity key "+key+" is not a static literal"), nil
			}
			// TJAA-V0-054: a non-identity value never decides the identity, whatever its form. One
			// that is not side-effect free is a selection unknown, because it could run code.
			if failure := playwrightPureFailure(value, scope); failure >= 0 {
				unverified = append(unverified, refuse(item, valueAt+failure, key+" is not a side-effect-free value"))
			}
			continue
		}
		literalValue, literal := playwrightString(value)
		if !literal || seenBrowser || literalValue != "chromium" && literalValue != "firefox" && literalValue != "webkit" {
			return "", device, "", refuse(item, valueAt, "browserName is not one literal chromium, firefox or webkit"), nil
		}
		browser, seenBrowser = literalValue, true
	}
	return browser, device, executable, block, unverified
}

// playwrightScopeRoot returns the first scope root fragment names: process, a numeric conversion
// or devices, or "".
func playwrightScopeRoot(fragment string) string {
	first, root := -1, ""
	for _, name := range []string{"process", "Number", "parseInt", "parseFloat", "devices"} {
		if at := playwrightTokenOffsets(fragment, name); len(at) != 0 && (first < 0 || at[0] < first) {
			first, root = at[0], name
		}
	}
	return root
}

// detail renders the block for one project in configPath: why, the blocking token and its line,
// and the scope token that refused a root the fragment needs (TJAA-V0-044).
func (block playwrightUseBlock) detail(scope playwrightPureScope, configPath string) string {
	if block.why == "" {
		return ""
	}
	token := playwrightBlockToken(block.fragment[min(block.offset, len(block.fragment)):])
	detail := ": " + block.why + " at `" + token + "` (" + playwrightBlockLines(scope.source, configPath, block) + ")"
	var refused playwrightScopeBlock
	switch block.root {
	case "process":
		if !scope.env {
			refused = scope.envBlock
		}
	case "Number", "parseInt", "parseFloat":
		if !scope.numeric[block.root] {
			refused = scope.numericBlock
		}
	case "devices":
		if !scope.devices {
			refused = scope.devicesBlock
		}
	}
	if refused.why != "" {
		detail += "; " + block.root + " is not admitted: " + refused.why
		if refused.offset >= 0 && refused.offset < len(scope.source) {
			detail += " at `" + playwrightBlockToken(scope.source[refused.offset:]) + "` (" + configPath + ":" + strconv.Itoa(lineAt(scope.source, refused.offset)) + ")"
		}
	}
	return detail
}

// playwrightBlockLines names configPath:line for every occurrence of the layer text in the
// config, at most four, or says the location was not observed.
func playwrightBlockLines(source, configPath string, block playwrightUseBlock) string {
	within := strings.Index(block.raw, block.fragment)
	if within < 0 || block.raw == "" {
		return configPath + ", line not observed"
	}
	lines := []string{}
	for from := 0; len(lines) < 4; {
		found := strings.Index(source[from:], block.raw)
		if found < 0 {
			break
		}
		at := from + found
		lines = append(lines, configPath+":"+strconv.Itoa(lineAt(source, at+within+block.offset)))
		from = at + len(block.raw)
	}
	if len(lines) == 0 {
		return configPath + ", line not observed"
	}
	return strings.Join(lines, ", ")
}

// playwrightBlockToken returns the identifier at the start of text, or its first non-space run of
// at most 24 bytes.
func playwrightBlockToken(text string) string {
	text = strings.TrimLeft(text, " \t\r\n")
	if word, end := playwrightIdentifier(text, 0); end > 0 {
		return word
	}
	end := 0
	for end < len(text) && end < 24 && strings.IndexByte(" \t\r\n", text[end]) < 0 {
		end++
	}
	for end > 0 && !utf8.ValidString(text[:end]) {
		end--
	}
	return text[:end]
}

// playwrightComputedExecutableOnly reports a launchOptions object literal whose properties are
// static except executablePath, which is side-effect free: it chooses the browser binary, not the
// project's browser identity or test membership (TJAA-V0-035).
func playwrightComputedExecutableOnly(scope playwrightPureScope, raw string) bool {
	properties, ok := playwrightObjectProperties(raw)
	executable, declared := properties["executablePath"]
	if !ok || !declared || !playwrightPureExpression(executable, scope) || playwrightReadsWrittenEnv(scope, executable) {
		return false
	}
	for key, value := range properties {
		if key != "executablePath" && !playwrightStaticValue(value) {
			return false
		}
	}
	return true
}

// playwrightReadsWrittenEnv reports whether raw reads a process.env key the config writes, which an
// identity key may not do (TJAA-V0-042).
func playwrightReadsWrittenEnv(scope playwrightPureScope, raw string) bool {
	for _, at := range playwrightTokenOffsets(raw, "process") {
		if keys, _, ok := playwrightMemberChain(raw, at+len("process")); !ok || playwrightEnvRead(keys) && scope.envWritten[keys[1]] {
			return true
		}
	}
	return false
}

// PlaywrightUseIdentityKeys are the `use` options that make up a project's browser/device
// identity. The external provider's qualified reporter resolves the same set at runtime;
// jstestprovider's tests pin its identityKeys list to this one.
var PlaywrightUseIdentityKeys = []string{"browserName", "defaultBrowserType", "channel", "headless", "connectOptions", "viewport", "screen", "userAgent", "isMobile", "hasTouch", "deviceScaleFactor", "locale", "timezoneId", "colorScheme", "permissions", "contextOptions", "launchOptions"}

func playwrightDeviceSpread(raw string) (string, bool) {
	if !strings.HasPrefix(raw, "devices[") || !strings.HasSuffix(raw, "]") {
		return "", false
	}
	return playwrightString(strings.TrimSpace(raw[len("devices[") : len(raw)-1]))
}

func playwrightDeviceBrowser(device string) string {
	switch device {
	case "Desktop Firefox":
		return "firefox"
	case "Desktop Safari":
		return "webkit"
	case "Desktop Chrome":
		return "chromium"
	}
	return ""
}

func playwrightProjectOwns(root string, project PlaywrightProject, globalTestDir, test string) bool {
	directory := project.TestDir
	if directory == "" {
		directory = globalTestDir
	}
	if directory != "." && !inside(test, directory) {
		return false
	}
	relative := strings.TrimPrefix(strings.TrimPrefix(test, directory), "/")
	absolute := filepath.ToSlash(filepath.Join(root, filepath.FromSlash(test)))
	if len(project.matchers) != 0 {
		if !playwrightAnyMatcher(project.matchers, absolute) {
			return false
		}
	} else if !playwrightDefaultTest(relative) {
		return false
	}
	return !playwrightAnyMatcher(project.ignores, absolute)
}

// playwrightPathOutsideMatcherModel reports whether a path holds CR, LF, U+2028, U+2029 or a
// character outside the Basic Multilingual Plane. Go's `.` (without `s`) and `(?m)` anchors treat
// only LF as a line terminator while JavaScript treats all four, and a JavaScript regular
// expression without the u flag matches UTF-16 code units, so `.`, `[^/]` or a negated class
// consumes half of a surrogate pair where Go consumes the whole rune. A matcher's verdict on such
// a path is not decided statically and membership widens.
func playwrightPathOutsideMatcherModel(value string) bool {
	return strings.ContainsAny(value, "\r\n\u2028\u2029") || playwrightOutsideBMP(value)
}

// playwrightOutsideBMP reports whether value holds a character above U+FFFF, which a JavaScript
// regular expression without the u flag reads as two UTF-16 code units and Go RE2 as one rune.
func playwrightOutsideBMP(value string) bool {
	return strings.ContainsFunc(value, func(character rune) bool { return character > 0xFFFF })
}

// playwrightDefaultTest applies the `.spec.`/`.test.` markers of Playwright's default testMatch
// glob, which createFileMatcher matches case-insensitively (minimatch nocase).
func playwrightDefaultTest(relative string) bool {
	base := strings.ToLower(path.Base(relative))
	return (strings.Contains(base, ".spec.") || strings.Contains(base, ".test.")) && playwrightLoadableName(base)
}

// playwrightLoadableName reports whether name has an extension Playwright's default testMatch
// (`**/*.@(spec|test).?(c|m)[jt]s?(x)`) accepts, including .mts and .cts, which the static profile
// does not parse.
func playwrightLoadableName(name string) bool {
	extension := strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")
	if strings.HasPrefix(extension, "c") || strings.HasPrefix(extension, "m") {
		extension = extension[1:]
	}
	extension = strings.TrimSuffix(extension, "x")
	return extension == "js" || extension == "ts"
}

func playwrightAnyMatcher(matchers []playwrightMatcher, values ...string) bool {
	for _, matcher := range matchers {
		for _, value := range values {
			if matcher.re.MatchString(value) {
				return true
			}
		}
	}
	return false
}

func compilePlaywrightMatcher(raw string) (playwrightMatcher, bool) {
	raw = strings.TrimSpace(raw)
	if value, ok := playwrightString(raw); ok {
		// Playwright's createFileMatcher prefixes `**/` to a string glob that lacks it and
		// matches it case-insensitively against the absolute path (minimatch nocase, dot).
		if !strings.HasPrefix(value, "**/") {
			value = "**/" + value
		}
		// minimatch splits the glob and the path on runs of `/`, so `e2e//*.spec.ts`, and a
		// leading `/` (which becomes `**//`), match where a literal `//` would not.
		pattern, valid := playwrightGlobPattern(value)
		if !valid || playwrightOutsideBMP(value) || strings.Contains(value, "//") {
			return playwrightMatcher{}, false
		}
		re, err := regexp.Compile("(?i)^(?:" + pattern + ")$")
		return playwrightMatcher{raw: raw, re: re}, err == nil
	}
	pattern, flags, ok := playwrightRegexLiteral(raw)
	if !ok || !playwrightRegexBodyStatic(pattern) {
		return playwrightMatcher{}, false
	}
	// Only flags Go maps exactly are modelled. Sticky `y` anchors at lastIndex 0, which Playwright
	// resets before each test, so it cannot be dropped; g, u, d, v and any other flag widen too.
	prefix := ""
	for _, flag := range flags {
		switch flag {
		case 'i':
			prefix += "(?i)"
		case 'm':
			prefix += "(?m)"
		case 's':
			prefix += "(?s)"
		default:
			return playwrightMatcher{}, false
		}
	}
	re, err := regexp.Compile(prefix + pattern)
	return playwrightMatcher{raw: raw, re: re}, err == nil
}

// playwrightRegexEscapes are the escapes whose meaning is identical in a JavaScript regular
// expression without the u flag and in Go RE2: ASCII digit, word and boundary classes, five
// control characters, and a backslash before ASCII syntax punctuation.
const (
	playwrightRegexEscapes      = "dDwWbBtnrfv"
	playwrightRegexPunctuation  = "^$\\.*+?()[]{}|/-"
	playwrightRegexMaxRepeat    = 1000
	playwrightRegexRepeatDigits = 4
)

// playwrightRegexBodyStatic reports whether a regular-expression literal body uses only constructs
// that JavaScript (without the u flag) and Go RE2 read identically: literal characters, `.`, `^`,
// `$`, `|`, `(...)` and `(?:...)` groups, the quantifiers `*`, `+`, `?`, `{n}`, `{n,}` and
// `{n,m}` (n <= m <= 1000, each bound `0` or digits without a leading zero) after an atom, each
// optionally lazy, classes of literals, ranges and allowed escapes, and the escapes in
// playwrightRegexEscapes and playwrightRegexPunctuation. Every other construct differs or may
// differ (`\A` is a literal A in JavaScript and an anchor in Go; `\s` includes Unicode spaces only
// in JavaScript; `{01}` repeats in JavaScript and is literal text in Go; a character above U+FFFF
// is a surrogate pair in JavaScript and one rune in Go; `(?i)`, named groups, lookaround,
// backreferences, `\x`, `\u`, `\p` and POSIX classes), so the matcher is not static.
func playwrightRegexBodyStatic(body string) bool {
	if playwrightOutsideBMP(body) {
		return false
	}
	atom, lazy := false, false
	for index := 0; index < len(body); index++ {
		character := body[index]
		switch {
		case character == '\\':
			if index+1 == len(body) {
				return false
			}
			index++
			escape := body[index]
			if !strings.ContainsRune(playwrightRegexEscapes, rune(escape)) && !strings.ContainsRune(playwrightRegexPunctuation, rune(escape)) {
				return false
			}
			atom, lazy = escape != 'b' && escape != 'B', false
		case character == '(':
			if index+1 < len(body) && body[index+1] == '?' {
				if index+2 == len(body) || body[index+2] != ':' {
					return false
				}
				index += 2
			}
			atom, lazy = false, false
		case character == '*' || character == '+' || character == '?':
			if character == '?' && lazy {
				atom, lazy = false, false
				continue
			}
			if !atom {
				return false
			}
			atom, lazy = false, true
		case character == '{':
			end, valid := playwrightRegexRepeatEnd(body, index)
			if !atom || !valid {
				return false
			}
			index = end - 1
			atom, lazy = false, true
		case character == '[':
			end, valid := playwrightRegexClassEnd(body, index)
			if !valid {
				return false
			}
			index = end - 1
			atom, lazy = true, false
		case character == '}' || character == ']':
			return false
		case character == '^' || character == '$' || character == '|':
			atom, lazy = false, false
		default: // a literal, `.` or a group's closing `)`
			atom, lazy = true, false
		}
	}
	return true
}

// playwrightRegexRepeatEnd returns the offset past a `{n}`, `{n,}` or `{n,m}` quantifier at
// body[start] with n <= m <= playwrightRegexMaxRepeat. Each bound is `0` or ASCII digits without a
// leading zero: JavaScript (Annex B) reads `{01}` as a repeat, while Go's parser rejects the
// leading zero and reads the braces as literal text. Spaces, a missing first bound (`{,n}`, a
// literal in both) and any other form are refused rather than modelled.
func playwrightRegexRepeatEnd(body string, start int) (int, bool) {
	closing := strings.IndexByte(body[start:], '}')
	if closing < 0 {
		return 0, false
	}
	bounds := strings.SplitN(body[start+1:start+closing], ",", 2)
	number := func(value string) (int, bool) {
		if value == "" || len(value) > playwrightRegexRepeatDigits || strings.Trim(value, "0123456789") != "" || len(value) > 1 && value[0] == '0' {
			return 0, false
		}
		parsed, err := strconv.Atoi(value)
		return parsed, err == nil && parsed <= playwrightRegexMaxRepeat
	}
	low, valid := number(bounds[0])
	if !valid {
		return 0, false
	}
	if len(bounds) == 2 && bounds[1] != "" {
		high, valid := number(bounds[1])
		if !valid || high < low {
			return 0, false
		}
	}
	return start + closing + 1, true
}

// playwrightRegexClassEnd returns the offset past a non-empty `[...]` or `[^...]` class at
// body[start] holding only literals, ranges and the allowed escapes other than `\b` and `\B`
// (a backspace in a JavaScript class). A nested `[` (Go's `[:alpha:]` form) is not static.
func playwrightRegexClassEnd(body string, start int) (int, bool) {
	index := start + 1
	if index < len(body) && body[index] == '^' {
		index++
	}
	if index < len(body) && body[index] == ']' {
		return 0, false // `[]` and `[^]` differ: Go reads `]` as a literal there
	}
	for ; index < len(body); index++ {
		switch body[index] {
		case ']':
			return index + 1, true
		case '[':
			return 0, false
		case '\\':
			if index+1 == len(body) {
				return 0, false
			}
			index++
			escape := body[index]
			if escape == 'b' || escape == 'B' || !strings.ContainsRune(playwrightRegexEscapes, rune(escape)) && !strings.ContainsRune(playwrightRegexPunctuation, rune(escape)) {
				return 0, false
			}
		}
	}
	return 0, false
}

func playwrightGlobPattern(glob string) (string, bool) {
	var pattern strings.Builder
	for index := 0; index < len(glob); {
		switch glob[index] {
		case '*':
			if index+1 < len(glob) && glob[index+1] == '*' {
				// minimatch's globstar is `**` as a whole path component; anywhere else `**` acts
				// as `*`, which is not modelled, so the glob is not static.
				if index > 0 && glob[index-1] != '/' {
					return "", false
				}
				if index+2 == len(glob) {
					pattern.WriteString(".*")
					index += 2
				} else if glob[index+2] == '/' {
					pattern.WriteString("(?:.*/)?")
					index += 3
				} else {
					return "", false
				}
			} else {
				pattern.WriteString("[^/]*")
				index++
			}
		case '?':
			pattern.WriteString("[^/]")
			index++
		case '{':
			end := strings.IndexByte(glob[index+1:], '}')
			if end < 0 {
				return "", false
			}
			parts := strings.Split(glob[index+1:index+1+end], ",")
			if len(parts) < 2 {
				return "", false // `{a}` is literal and `{1..3}` a range in minimatch
			}
			pattern.WriteString("(?:")
			for partIndex, part := range parts {
				// An alternative holding `/` can expand to `//`, which minimatch collapses.
				if part == "" || strings.ContainsAny(part, "*?{}[]()\\/") || strings.Contains(part, "..") {
					return "", false
				}
				if partIndex != 0 {
					pattern.WriteByte('|')
				}
				pattern.WriteString(regexp.QuoteMeta(part))
			}
			pattern.WriteByte(')')
			index += end + 2
		case '[', ']', '(', ')', '\\':
			return "", false // classes, extglobs and escapes are not modelled
		default:
			start := index
			for index < len(glob) && !strings.ContainsRune("*?{[]()\\", rune(glob[index])) {
				index++
			}
			pattern.WriteString(regexp.QuoteMeta(glob[start:index]))
		}
	}
	return pattern.String(), true
}

func playwrightRegexLiteral(raw string) (pattern, flags string, ok bool) {
	if len(raw) < 2 || raw[0] != '/' {
		return "", "", false
	}
	escaped := false
	class := false
	for index := 1; index < len(raw); index++ {
		switch raw[index] {
		case '\\':
			escaped = !escaped
		case '[':
			if !escaped {
				class = true
			}
			escaped = false
		case ']':
			if !escaped {
				class = false
			}
			escaped = false
		case '/':
			if !escaped && !class {
				return raw[1:index], raw[index+1:], true
			}
			escaped = false
		default:
			escaped = false
		}
	}
	return "", "", false
}

func playwrightConfigObject(source string) (string, bool) {
	export := regexp.MustCompile(`(?m)^[\t ]*export[\t ]+default\b`).FindStringIndex(source)
	if export == nil {
		return "", false
	}
	rest := strings.TrimSpace(source[export[1]:])
	if strings.HasPrefix(rest, "defineConfig") && len(rest) > len("defineConfig") && (rest[len("defineConfig")] == '(' || unicode.IsSpace(rune(rest[len("defineConfig")]))) {
		rest = rest[len("defineConfig"):]
		open := strings.IndexByte(rest, '(')
		if open < 0 {
			return "", false
		}
		value, next, ok := playwrightBalancedValue(rest, open+1, ')')
		if !ok {
			return "", false
		}
		if trailing := strings.TrimSpace(rest[next:]); trailing != "" && trailing != ";" {
			return "", false
		}
		return playwrightObject(value)
	}
	return playwrightObject(rest)
}

func playwrightObject(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw[0] != '{' {
		return "", false
	}
	value, next, ok := playwrightBalancedValue(raw, 1, '}')
	if !ok {
		return "", false
	}
	if trailing := strings.TrimSpace(raw[next:]); trailing != "" && trailing != ";" {
		return "", false
	}
	return "{" + value + "}", true
}

func playwrightObjectProperties(raw string) (map[string]string, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != '{' || raw[len(raw)-1] != '}' {
		return nil, false
	}
	body := raw[1 : len(raw)-1]
	items, ok := playwrightSplitTopLevel(body)
	if !ok {
		return nil, false
	}
	properties := map[string]string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.HasPrefix(item, "...") {
			return nil, false
		}
		colon := playwrightTopLevelColon(item)
		if colon < 0 {
			return nil, false
		}
		keyRaw := strings.TrimSpace(item[:colon])
		key, literal := playwrightString(keyRaw)
		if !literal {
			key = keyRaw
			for _, character := range key {
				if character != '_' && character != '$' && !unicode.IsLetter(character) && !unicode.IsDigit(character) {
					return nil, false
				}
			}
		}
		if key == "" || strings.TrimSpace(item[colon+1:]) == "" {
			return nil, false
		}
		if _, duplicate := properties[key]; duplicate {
			return nil, false
		}
		properties[key] = strings.TrimSpace(item[colon+1:])
	}
	return properties, true
}

// playwrightArrayItems splits an array literal. One trailing comma after an element ends the
// list, as in ECMAScript, so `['a', 'b',]` has two items (TJAA-V0-035); `[]` stays one empty item.
func playwrightArrayItems(raw string) ([]string, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != '[' || raw[len(raw)-1] != ']' {
		return nil, false
	}
	items, ok := playwrightSplitTopLevel(raw[1 : len(raw)-1])
	if ok && len(items) > 1 && items[len(items)-1] == "" && items[len(items)-2] != "" {
		items = items[:len(items)-1]
	}
	return items, ok
}

func playwrightSplitTopLevel(raw string) ([]string, bool) {
	items := []string{}
	start := 0
	stack := []byte{}
	quote := byte(0)
	escaped := false
	regex := false
	class := false
	for index := 0; index < len(raw); index++ {
		character := raw[index]
		if quote != 0 {
			if escaped {
				escaped = false
			} else if character == '\\' {
				escaped = true
			} else if character == quote {
				quote = 0
			}
			continue
		}
		if regex {
			if escaped {
				escaped = false
			} else if character == '\\' {
				escaped = true
			} else if character == '[' {
				class = true
			} else if character == ']' {
				class = false
			} else if character == '/' && !class {
				regex = false
			}
			continue
		}
		switch character {
		case '\'', '"':
			quote = character
		case '`':
			end := templateEnd(raw, index)
			if end > len(raw) {
				return nil, false
			}
			index = end - 1
		case '/':
			var known bool
			if regex, known = playwrightSlashStartsRegex(raw, index); !known {
				return nil, false
			}
		case '{', '[', '(':
			stack = append(stack, character)
		case '}', ']', ')':
			if len(stack) == 0 || !playwrightPair(stack[len(stack)-1], character) {
				return nil, false
			}
			stack = stack[:len(stack)-1]
		case ',':
			if len(stack) == 0 {
				items = append(items, strings.TrimSpace(raw[start:index]))
				start = index + 1
			}
		}
	}
	if quote != 0 || regex || len(stack) != 0 {
		return nil, false
	}
	items = append(items, strings.TrimSpace(raw[start:]))
	return items, true
}

func playwrightTopLevelColon(raw string) int {
	depth := 0
	quote := byte(0)
	escaped := false
	for index := 0; index < len(raw); index++ {
		character := raw[index]
		if quote != 0 {
			if escaped {
				escaped = false
			} else if character == '\\' {
				escaped = true
			} else if character == quote {
				quote = 0
			}
			continue
		}
		switch character {
		case '\'', '"':
			quote = character
		case '`':
			end := templateEnd(raw, index)
			if end > len(raw) {
				return -1
			}
			index = end - 1
		case '{', '[', '(':
			depth++
		case '}', ']', ')':
			depth--
		case ':':
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

func playwrightBalancedValue(raw string, start int, closer byte) (string, int, bool) {
	depth := 0
	quote := byte(0)
	escaped := false
	for index := start; index < len(raw); index++ {
		character := raw[index]
		if quote != 0 {
			if escaped {
				escaped = false
			} else if character == '\\' {
				escaped = true
			} else if character == quote {
				quote = 0
			}
			continue
		}
		switch character {
		case '\'', '"':
			quote = character
		case '`':
			end := templateEnd(raw, index)
			if end > len(raw) {
				return "", len(raw), false
			}
			index = end - 1
		case '{', '[', '(':
			depth++
		case '}', ']', ')':
			if depth == 0 && character == closer {
				return strings.TrimSpace(raw[start:index]), index + 1, true
			}
			depth--
		}
	}
	return "", len(raw), false
}

func playwrightPair(open, close byte) bool {
	return open == '{' && close == '}' || open == '[' && close == ']' || open == '(' && close == ')'
}

// playwrightRegexKeywords are the reserved words after which a `/` starts a regular expression.
var playwrightRegexKeywords = []string{"return", "typeof", "instanceof", "in", "new", "delete", "void", "throw", "case", "do", "else"}

// playwrightEndsCodePointEscape reports whether the `}` at index closes a `\u{hex}` escape.
func playwrightEndsCodePointEscape(raw string, index int) bool {
	if raw[index] != '}' {
		return false
	}
	start := index
	for start > 0 && strings.IndexByte("0123456789abcdefABCDEF", raw[start-1]) >= 0 {
		start--
	}
	return start >= 3 && start < index && raw[start-3:start] == "\\u{"
}

// playwrightSlashStartsRegex reports whether the `/` at index starts a regular expression: after
// an operator or opening punctuation, `=>`, a reserved word in playwrightRegexKeywords, or nothing.
// After an identifier, a number, `)`, `]`, `}` or a property name (`x.return`) it is division.
// known is false after `of`, `yield` or `await`: each may be an identifier operand of a division
// or a keyword before a regular expression, so the caller refuses the source rather than guess.
// It is also false after a non-ASCII byte (an identifier, a space or a line terminator) and after
// an identifier escape, which this ASCII classifier cannot read.
func playwrightSlashStartsRegex(raw string, index int) (regex, known bool) {
	previous := index - 1
	for previous >= 0 && (raw[previous] == ' ' || raw[previous] == '\t' || raw[previous] == '\n' || raw[previous] == '\r') {
		previous--
	}
	if previous < 0 {
		return true, true
	}
	if raw[previous] == '>' && previous > 0 && raw[previous-1] == '=' {
		return true, true
	}
	if raw[previous] >= utf8.RuneSelf || raw[previous] == '\\' || playwrightEndsCodePointEscape(raw, previous) {
		return false, false // a non-ASCII identifier, space or punctuator, or an escaped identifier
	}
	from := previous + 1
	for from > 0 && strings.IndexByte("_$abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", raw[from-1]) >= 0 {
		from--
	}
	if word := raw[from : previous+1]; word != "" {
		before := from - 1
		for before >= 0 && (raw[before] == ' ' || raw[before] == '\t' || raw[before] == '\n' || raw[before] == '\r') {
			before--
		}
		if before >= 0 && raw[before] == '.' && (before == 0 || raw[before-1] != '.') {
			return false, true // a property name, never a keyword
		}
		if from > 0 && (raw[from-1] >= utf8.RuneSelf || raw[from-1] == '\\') {
			return false, false // part of a non-ASCII or escaped identifier
		}
		switch {
		case word == "of" || word == "yield" || word == "await":
			return false, false
		case slices.Contains(playwrightRegexKeywords, word):
			return true, true
		}
		return false, true
	}
	return strings.ContainsRune("([{:;,=!?&|", rune(raw[previous])), true
}

func playwrightString(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != raw[len(raw)-1] || raw[0] != '\'' && raw[0] != '"' {
		return "", false
	}
	if raw[0] == '"' {
		value, err := strconv.Unquote(raw)
		return value, err == nil
	}
	body := raw[1 : len(raw)-1]
	escaped := false
	for _, character := range body {
		if escaped {
			escaped = false
			continue
		}
		if character == '\\' {
			escaped = true
			continue
		}
		if character == '\'' {
			return "", false
		}
	}
	if escaped {
		return "", false
	}
	body = strings.ReplaceAll(body, `\'`, `'`)
	if strings.Contains(body, "\\") {
		quoted := `"` + strings.ReplaceAll(body, `"`, `\"`) + `"`
		value, err := strconv.Unquote(quoted)
		return value, err == nil
	}
	return body, true
}

func playwrightStringArray(raw string) ([]string, bool) {
	items, ok := playwrightArrayItems(raw)
	if !ok {
		return nil, false
	}
	if len(items) == 1 && items[0] == "" {
		return []string{}, true // []
	}
	values := make([]string, 0, len(items))
	for _, item := range items {
		value, literal := playwrightString(item)
		if !literal || value == "" {
			return nil, false
		}
		values = append(values, value)
	}
	return values, true
}

// playwrightStaticValue reports a literal value, with arrays and objects nested at most
// playwrightPureMaxDepth deep so adversarial nesting is refused in bounded work.
func playwrightStaticValue(raw string) bool {
	return playwrightStaticValueAt(raw, 0)
}

func playwrightStaticValueAt(raw string, depth int) bool {
	if depth > playwrightPureMaxDepth {
		return false
	}
	raw = strings.TrimSpace(raw)
	if _, ok := playwrightString(raw); ok {
		return true
	}
	if raw == "true" || raw == "false" || raw == "null" {
		return true
	}
	if _, err := strconv.ParseFloat(raw, 64); err == nil {
		return true
	}
	if items, ok := playwrightArrayItems(raw); ok {
		if len(items) == 1 && items[0] == "" {
			return true // []
		}
		for _, item := range items {
			if !playwrightStaticValueAt(item, depth+1) {
				return false
			}
		}
		return true
	}
	if properties, ok := playwrightObjectProperties(raw); ok {
		for _, value := range properties {
			if !playwrightStaticValueAt(value, depth+1) {
				return false
			}
		}
		return true
	}
	return false
}

func playwrightProjectCycle(projects []PlaywrightProject) bool {
	edges := map[string][]string{}
	for _, project := range projects {
		edges[project.Name] = append(append([]string{}, project.Dependencies...), project.Teardown)
	}
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var visit func(string) bool
	visit = func(name string) bool {
		if visiting[name] {
			return true
		}
		if visited[name] {
			return false
		}
		visiting[name] = true
		for _, next := range edges[name] {
			if visit(next) {
				return true
			}
		}
		visiting[name] = false
		visited[name] = true
		return false
	}
	for name := range edges {
		if visit(name) {
			return true
		}
	}
	return false
}

func selectionUnknown(reason, detail string) PlaywrightUnknown {
	return PlaywrightUnknown{Axis: PlaywrightAxisSelection, Reason: reason, Detail: detail}
}

func canonicalPlaywrightUnknowns(values []PlaywrightUnknown) []PlaywrightUnknown {
	seen := map[string]bool{}
	result := []PlaywrightUnknown{}
	for _, value := range values {
		key := value.Axis + "\x00" + value.Reason + "\x00" + value.Detail
		if !seen[key] {
			seen[key] = true
			result = append(result, value)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Axis != result[j].Axis {
			return result[i].Axis < result[j].Axis
		}
		if result[i].Reason != result[j].Reason {
			return result[i].Reason < result[j].Reason
		}
		return result[i].Detail < result[j].Detail
	})
	return result
}

func sortedPlaywrightSelections(values map[string]PlaywrightSelection) []PlaywrightSelection {
	result := make([]PlaywrightSelection, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func playwrightIDEscape(value string) string {
	const hexDigits = "0123456789ABCDEF"
	var result strings.Builder
	for _, raw := range []byte(value) {
		if raw >= 'a' && raw <= 'z' || raw >= 'A' && raw <= 'Z' || raw >= '0' && raw <= '9' || strings.ContainsRune("-_.", rune(raw)) {
			result.WriteByte(raw)
			continue
		}
		result.WriteByte('%')
		result.WriteByte(hexDigits[raw>>4])
		result.WriteByte(hexDigits[raw&15])
	}
	return result.String()
}

func playwrightRelativeDirectory(value string) bool {
	clean := path.Clean(value)
	return value != "" && !strings.HasPrefix(clean, "../") && clean != ".." && !strings.HasPrefix(clean, "/")
}

// playwrightJoinInsideRoot joins a relative value to the repository-relative directory that
// declares it and reports whether the cleaned result stays inside the repository root, so a
// `../` that climbs only within the root is accepted and an escape or absolute value is refused
// (TJAA-V0-113).
func playwrightJoinInsideRoot(directory, value string) (string, bool) {
	if value == "" || strings.HasPrefix(value, "/") {
		return "", false
	}
	joined := path.Clean(path.Join(directory, value))
	return joined, playwrightRelativeDirectory(joined)
}

func sortedUnique(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	return sortedKeys(set)
}

func containsString(values []string, want string) bool {
	index := sort.SearchStrings(values, want)
	return index < len(values) && values[index] == want
}

// graphFrontierUnresolved reports whether a shared graph unknown other than NO_SELECTABLE_TEST
// and an informational LANGUAGE_FRONTIER reason leaves the selection unresolved (AFP-V0-020,
// TJAA-V0-108), other than a dirty path outside the graph the repository declares unreached and
// the plan applied (TJAA-V0-121). A changed unit no spec reaches (NO_SELECTABLE_TEST) is handled by
// playwrightOutOfSuiteSources instead: an end-to-end run may load it through the application
// under test, so it is a selection unknown unless declared unreached (TJAA-V0-106).
func graphFrontierUnresolved(base affected.Plan, informational, appliedUnowned map[string]bool) bool {
	for _, unknown := range base.Unknown {
		switch {
		case unknown.Reason == affected.UnknownNoSelectableTest:
		case unknown.Reason == affected.UnknownUnownedDirtyPath && appliedUnowned[unknown.Detail]:
		case unknown.Reason == affected.UnknownLanguageFrontier && informational[unknown.Detail]:
		default:
			return true
		}
	}
	return false
}
