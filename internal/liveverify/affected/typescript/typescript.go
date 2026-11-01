// Package typescript is the JavaScript and TypeScript implementation of the
// affected-selection language seam.
//
// A unit is one source or test file because that is the smallest common unit
// Vitest, Jest, Mocha, AVA, node:test, Playwright, Bun, Deno, Cypress, WebdriverIO,
// TestCafe, Nightwatch, and Detox can address. The legacy Storybook test-runner
// is the exception: it can address only one configured story set, so its story
// files are one aggregate unit. Imports are observed from source text; no
// runtime, package manager, or executable project configuration is started.
package typescript

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// Frontier reasons this plugin can raise.
const (
	FrontierUnreadableSource = "typescript:unreadable-source"
	FrontierUnparsedSource   = "typescript:unparsed-source"
	FrontierDynamicImport    = "typescript:dynamic-import"
	FrontierScanUnresolved   = "typescript:scan-unresolved"
	FrontierUnresolvedImport = "typescript:relative-import-unresolved"
	FrontierPathAlias        = "typescript:path-alias-unresolved"
	// FrontierBareImportUndeclared reports a bare specifier no package.json at or above the
	// importing file declares and no node_modules installs: an npm dependency the manifest
	// omits, or a bundler alias or virtual module (a Vite resolve.alias), either of which may
	// hide an edge, so it fails closed under its own name rather than as a tsconfig alias
	// (TJAA-V0-771).
	FrontierBareImportUndeclared   = "typescript:bare-import-undeclared"
	FrontierAmbiguousRunner        = "typescript:test-runner-ambiguous"
	FrontierUnknownRunner          = "typescript:test-runner-unknown"
	FrontierConfig                 = "typescript:executable-config-unresolved"
	FrontierRuntimeFlags           = "typescript:runtime-flags-unresolved"
	FrontierE2ERuntimeDependency   = "typescript:e2e-runtime-dependency"
	FrontierPuppeteerHost          = "typescript:puppeteer-host-runner-unresolved"
	FrontierDetoxConfiguration     = "typescript:detox-configuration-unresolved"
	FrontierStorybookAddressing    = "typescript:storybook-story-set-addressing"
	FrontierCrossLanguageTestAsset = "typescript:cross-language-test-asset-unowned"
	// FrontierOverBoundSource names one file over affected.MaxSourceBytes, by path, so a
	// consumer can fail closed only for a change that touches it (AFP-V0-444). In Playwright mode
	// it, like every other reason, widens the plan only when a site can change the Playwright
	// answer; elsewhere it is informational (TJAA-V0-108).
	FrontierOverBoundSource = "typescript:over-bound-source"
)

const (
	runnerVitest          = "vitest"
	runnerJest            = "jest"
	runnerMocha           = "mocha"
	runnerAVA             = "ava"
	runnerNode            = "node-test"
	runnerPlaywright      = "playwright"
	runnerBun             = "bun-test"
	runnerDeno            = "deno-test"
	runnerCypress         = "cypress"
	runnerWebdriverIO     = "webdriverio"
	runnerTestCafe        = "testcafe"
	runnerNightwatch      = "nightwatch"
	runnerDetox           = "detox"
	runnerStorybook       = "storybook-test-runner"
	runnerStorybookVitest = "storybook-vitest"
	runnerUnknown         = "unknown"
)

var sourceExtensions = []string{".cjs", ".js", ".jsx", ".mjs", ".ts", ".tsx"}

// Language observes JavaScript and TypeScript modules under one repository
// root.
type Language struct{}

// New returns the JavaScript and TypeScript language plugin.
func New() Language { return Language{} }

// Name is the plugin namespace.
func (Language) Name() string { return "typescript" }

// Owns reports whether a path is JavaScript or TypeScript source text. Data,
// Gherkin, YAML, JSON, MDX, and host-independent E2E assets deliberately remain
// unowned because the seam has no shared-input ownership model.
func (Language) Owns(relative string) bool { return hasSourceExtension(relative) }

type observation struct {
	path      string
	body      string
	refs      []string
	runner    string
	configs   []string
	puppeteer bool
	test      bool
}

type config struct {
	path      string
	directory string
	runner    string
	// membership records that the config names test-membership or project markers the general
	// adapter does not model (dynamicConfigMarker); tsconfigAt is the offset of the first
	// `tsconfig` token outside comments, which can select a module-resolution config other than
	// the nearest tsconfig.json, or -1 (TJAA-V0-050).
	membership bool
	tsconfigAt int
	body       string
	// literalProjects holds a Playwright config's parsed projects, and literalTestDir its global
	// testDir, when every testMatch in it is a literal root-free string glob, or nil (TJAA-V0-092).
	literalProjects []PlaywrightProject
	literalTestDir  string
}

type packageScope struct {
	directory string
	// manifest is the package.json or runtime manifest that declared the scope.
	manifest   string
	name       string
	runners    map[string]bool
	configured map[string]bool
	packages   map[string]bool
	// localSpecifiers holds the dependencies whose file:, link:, portal: or workspace:
	// specifier names repository source (TJAA-V0-710): they are no external package.
	localSpecifiers map[string]bool
}

// Units observes every owned module and every supported test runner that can
// be identified without executing repository code.
func (language Language) Units(ctx context.Context, root string) (affected.Result, error) {
	return language.UnitsSource(affected.DiskSourceContext(ctx, root))
}

func (language Language) UnitsSource(root *affected.Source) (affected.Result, error) {
	return language.unitsSource(root, false)
}

func (language Language) units(root string, resolveAliases bool) (affected.Result, frontierSet, error) {
	return language.observe(affected.DiskSource(root), resolveAliases)
}

// unitsSource is the general affected-plan/0 observation: its frontier entries name every site
// and token, bounded per reason (TJAA-V0-052). The Playwright profile reads the bare reasons and
// the site set from units instead.
func (language Language) unitsSource(root *affected.Source, resolveAliases bool) (affected.Result, error) {
	result, frontier, err := language.observe(root, resolveAliases)
	if err != nil {
		return result, err
	}
	localizeFixtureFrontier(&result, frontier)
	result.Frontier = locatedFrontier(frontier)
	return result, nil
}

// frontierSet holds, for each raised frontier reason, the sites that raised it: `path:line` when
// one line holds the cause and `path` when only the file is known (TJAA-V0-040), each with the
// specifier or source token that raised it, or "" when none is known (TJAA-V0-052).
type frontierSet map[string]map[string]string

func (frontier frontierSet) raise(reason, site string) {
	frontier.raiseToken(reason, site, "")
}

// raiseToken records token for site; of several tokens for one site the smallest non-empty one
// is kept, so the set does not depend on raise order.
func (frontier frontierSet) raiseToken(reason, site, token string) {
	if frontier[reason] == nil {
		frontier[reason] = map[string]string{}
	}
	if site == "" {
		return
	}
	previous, seen := frontier[reason][site]
	pinned := strings.HasPrefix(previous, pinnedSite)
	previous = strings.TrimPrefix(previous, pinnedSite)
	if !seen || previous == "" || token != "" && token < previous {
		previous = token
	}
	if pinned {
		previous = pinnedSite + previous
	}
	frontier[reason][site] = previous
}

// pinnedSite prefixes the stored token of a site that explains a whole reason, such as the
// identity that blocked tsconfig alias resolution, so the site bound never hides it (TJAA-V0-058).
const pinnedSite = "\x00"

// pin marks an existing site of reason as one that every rendering names before the bound applies.
func (frontier frontierSet) pin(reason, site string) {
	if token, seen := frontier[reason][site]; seen && !strings.HasPrefix(token, pinnedSite) {
		frontier[reason][site] = pinnedSite + token
	}
}

// boundedSites returns how many of ordered (from orderedSites) to name: every pinned site, then
// unpinned ones up to bound.
func boundedSites(sites map[string]string, ordered []string, bound int) int {
	pinned := 0
	for pinned < len(ordered) && strings.HasPrefix(sites[ordered[pinned]], pinnedSite) {
		pinned++
	}
	return min(len(ordered), pinned+bound)
}

// orderedSites returns the pinned sites, then the others, each group sorted.
func orderedSites(sites map[string]string) []string {
	ordered := sortedKeys(sites)
	sort.SliceStable(ordered, func(left, right int) bool {
		return strings.HasPrefix(sites[ordered[left]], pinnedSite) && !strings.HasPrefix(sites[ordered[right]], pinnedSite)
	})
	return ordered
}

// has reports whether reason was raised at site.
func (frontier frontierSet) has(reason, site string) bool {
	_, raised := frontier[reason][site]
	return raised
}

// at raises reason at the line of offset in body with the source token there, or at the file when
// offset is negative.
func (frontier frontierSet) at(reason, relative, body string, offset int) {
	token := ""
	if offset >= 0 && offset < len(body) {
		token = sourceToken(body[offset:])
	}
	frontier.raiseToken(reason, lineSite(relative, body, offset), token)
}

// sourceTokenBound caps a named source token in bytes.
const sourceTokenBound = 24

// sourceToken returns the identifier at the start of text, or its first run of at most
// sourceTokenBound bytes without space or control bytes, cut back to whole UTF-8 characters.
func sourceToken(text string) string {
	end := 0
	for end < len(text) && end < sourceTokenBound && isIdentifier(text[end]) {
		end++
	}
	if end == 0 {
		for end < len(text) && end < sourceTokenBound && text[end] > ' ' && text[end] != 0x7f && !unicodeLineTerminatorAt(text, end) {
			end++
		}
	}
	for end > 0 && !utf8.ValidString(text[:end]) {
		end--
	}
	return text[:end]
}

// frontierSiteBound caps the unpinned sites the general adapter names for one reason; one
// `reason:+N` entry counts the rest (TJAA-V0-052, as RUBY-AFFECTED-V0-046 bounds Ruby's). Pinned
// sites are always named (TJAA-V0-058).
const frontierSiteBound = 16

// locatedFrontier renders frontier as the general adapter's frontier entries, grouped by reason in
// sorted reason order: per reason, `reason:site` followed by a space and the backquoted token when
// one is known, every pinned site first, then at most frontierSiteBound sorted sites, then
// `reason:+N` for the rest; a reason without a site stays bare. The entries are not re-sorted, so
// the pinned sites stay first (TJAA-V0-058).
func locatedFrontier(frontier frontierSet) []string {
	entries := []string{}
	for _, reason := range sortedKeys(frontier) {
		sites := orderedSites(frontier[reason])
		if len(sites) == 0 {
			entries = append(entries, reason)
			continue
		}
		named := boundedSites(frontier[reason], sites, frontierSiteBound)
		for _, site := range sites[:named] {
			entries = append(entries, reason+":"+siteWithToken(site, frontier[reason][site]))
		}
		if extra := len(sites) - named; extra > 0 {
			entries = append(entries, reason+":+"+strconv.Itoa(extra))
		}
	}
	return entries
}

// siteWithToken appends the backquoted token to site when one is known.
func siteWithToken(site, token string) string {
	token = strings.TrimPrefix(token, pinnedSite)
	if token == "" {
		return site
	}
	return site + " `" + token + "`"
}

func lineSite(relative, body string, offset int) string {
	if offset < 0 {
		return relative
	}
	return relative + ":" + strconv.Itoa(lineAt(body, offset))
}

// sourceSites locates parsed import specifiers: for each owned file, the offsets its import scan
// kept and its line starts, indexed once, so naming any number of sites stays linear in the
// file (TJAA-V0-040).
type sourceSites map[string]*sourceIndex

type sourceIndex struct {
	body    string
	offsets map[string]int
	// specifiers maps the content offset of every literal import, export-from, require and
	// literal dynamic-import specifier to its text (TJAA-V0-126).
	specifiers map[int]string
	starts     []int
	// spelled maps a reference an alias rewrote to the specifier the module spelled.
	spelled map[string]string
}

// raiseRef raises reason at ref's parsed specifier in relative, naming the specifier the module
// spelled as the token (TJAA-V0-052).
func (sites sourceSites) raiseRef(frontier frontierSet, reason, relative, ref string) {
	frontier.raiseToken(reason, sites.ref(relative, ref), sites.spelling(relative, ref))
}

// spelling returns the specifier relative spelled for ref: the original of an alias rewrite, or
// ref itself.
func (sites sourceSites) spelling(relative, ref string) string {
	if index := sites[relative]; index != nil {
		if original, rewritten := index.spelled[ref]; rewritten {
			return original
		}
	}
	return ref
}

// ref names the line of ref's parsed specifier in relative, or relative alone when the scan kept
// no offset for it (an alias rewritten to a path the module never spelled, for example).
func (sites sourceSites) ref(relative, ref string) string {
	index := sites[relative]
	if index == nil {
		return relative
	}
	offset, known := index.offsets[ref]
	if !known {
		return relative
	}
	if index.starts == nil {
		index.starts = lineStarts(index.body)
	}
	return relative + ":" + strconv.Itoa(sort.SearchInts(index.starts, offset+1)+1)
}

// alias records that rewritten names the specifier original spelled in relative.
func (sites sourceSites) alias(relative, original, rewritten string) {
	if index := sites[relative]; index != nil {
		_, scanned := index.offsets[rewritten]
		if _, taken := index.spelled[rewritten]; !taken && !scanned {
			if index.spelled == nil {
				index.spelled = map[string]string{}
			}
			index.spelled[rewritten] = original
		}
		if offset, known := index.offsets[original]; known {
			if _, taken := index.offsets[rewritten]; !taken {
				index.offsets[rewritten] = offset
			}
		}
	}
}

// lineStarts returns the offsets where lines 2, 3, ... begin, with the terminators of lineAt.
func lineStarts(body string) []int {
	starts := []int{}
	for index := 0; index < len(body); index++ {
		switch {
		case body[index] == '\n':
			starts = append(starts, index+1)
		case body[index] == '\r' && (index+1 == len(body) || body[index+1] != '\n'):
			starts = append(starts, index+1)
		case unicodeLineTerminatorAt(body, index):
			index += 2
			starts = append(starts, index+1)
		}
	}
	return starts
}

func (language Language) observe(root *affected.Source, resolveAliases bool) (affected.Result, frontierSet, error) {
	files, err := root.Files(observedName)
	if err != nil {
		return affected.Result{}, nil, err
	}
	// A path the repository declares as no test asset raises no cross-language test-asset
	// frontier (TJAA-V0-111).
	_, nonTestAssets, err := readPathDeclaration(root, NonTestAssetPathsPath, NonTestAssetPathsProfile)
	if err != nil {
		return affected.Result{}, nil, err
	}
	frontier := frontierSet{}
	bodies := make(map[string]string, len(files))
	configs := make([]config, 0, 16)
	scopes := make([]packageScope, 0, 16)
	owned := make([]string, 0, len(files))
	aliasConfigs := []string{}
	sites := sourceSites{}
	// opaque holds the owned modules whose imports are not all statically observed (AFP-V0-389).
	opaque := map[string]bool{}
	for _, relative := range files {
		if unreadModuleSource(relative) {
			frontier.raise(FrontierUnparsedSource, relative)
			continue
		}
		body, readErr := root.Read(relative)
		if errors.Is(readErr, affected.ErrWalkLimit) {
			frontier.raise(FrontierOverBoundSource, relative)
			continue
		}
		if readErr != nil || !utf8.Valid(body) {
			frontier.raise(FrontierUnreadableSource, relative)
			continue
		}
		text := string(body)
		bodies[relative] = text
		configIdentity, _ := configRunner(relative, text)
		if ambiguousTestAsset(relative) && configIdentity != runnerMocha {
			if !affected.IgnoredMatch(nonTestAssets, relative) {
				frontier.raise(FrontierCrossLanguageTestAsset, relative)
			}
			continue
		}
		if path.Base(relative) == "package.json" {
			scope, parseErr := readPackageScope(relative, body)
			if parseErr != nil {
				frontier.raiseToken(FrontierConfig, relative, "package.json")
				continue
			}
			if scope.configured[runnerMocha] {
				frontier.at(FrontierConfig, relative, text, strings.Index(text, `"mocha"`))
			}
			if scope.name != "" || len(scope.runners) != 0 || len(scope.configured) != 0 || len(scope.packages) != 0 {
				scopes = append(scopes, scope)
			}
			continue
		}
		if runner, known := configRunner(relative, text); known {
			marker := dynamicConfigMarker(text)
			configs = append(configs, config{path: relative, directory: path.Dir(relative), runner: runner, membership: marker >= 0, tsconfigAt: tsconfigToken(text), body: text})
			if marker >= 0 && runner == runnerPlaywright {
				configs[len(configs)-1].literalProjects, configs[len(configs)-1].literalTestDir = playwrightLiteralProjects(relative, text)
			}
			switch {
			case marker >= 0 && runner == runnerPlaywright:
				// AFP-V0-404: decided once the tests are classified, below.
			case marker >= 0:
				frontier.at(FrontierConfig, relative, text, marker)
			case runner == runnerMocha:
				frontier.raiseToken(FrontierConfig, relative, runner)
			}
		}
		if manifestRunner := runtimeManifestRunner(relative); manifestRunner != "" {
			scopes = append(scopes, packageScope{directory: path.Dir(relative), manifest: relative, runners: map[string]bool{manifestRunner: true}, configured: map[string]bool{manifestRunner: true}})
		}
		if language.Owns(relative) {
			owned = append(owned, relative)
		}
	}
	// An alias config (a tsconfig.json, or a jsconfig.json with none beside it, TJAA-V0-960)
	// declares aliases when it or a config its observed `extends` chain names holds a paths or
	// baseUrl key; the declaring config is the frontier site (TJAA-V0-114).
	for _, relative := range files {
		if _, read := bodies[relative]; !read || !aliasConfigFile(relative, bodies) || resolveAliases {
			continue
		}
		if declaring := typeScriptAliasDeclaringConfig(relative, bodies); declaring != "" && !slices.Contains(aliasConfigs, declaring) {
			aliasConfigs = append(aliasConfigs, declaring)
		}
	}
	sort.Slice(configs, func(left, right int) bool { return configs[left].path < configs[right].path })
	sort.Slice(scopes, func(left, right int) bool { return scopes[left].directory < scopes[right].directory })
	observations := make([]observation, 0, len(owned))
	for _, relative := range owned {
		body, readable := bodies[relative]
		if !readable {
			continue
		}
		scan := scanImportSites(relative, body)
		sites[relative] = &sourceIndex{body: body, offsets: scan.offsets, specifiers: scan.specifiers}
		opaque[relative] = scan.err != nil || scan.dynamic >= 0 || scan.unresolved >= 0
		if scan.err != nil {
			frontier.at(FrontierUnparsedSource, relative, body, scan.syntax)
		}
		if scan.dynamic >= 0 {
			frontier.at(FrontierDynamicImport, relative, body, scan.dynamic)
		}
		if scan.unresolved >= 0 {
			frontier.at(FrontierScanUnresolved, relative, body, scan.unresolved)
		}
		observation := classify(relative, body, scan.refs, configs, scopes, sites, frontier)
		observation.refs = scan.refs
		observation.body = body
		observations = append(observations, observation)
	}
	for _, config := range configs {
		if config.runner != runnerPlaywright || !config.membership {
			continue
		}
		if site, token, static := playwrightStaticMembership(config, files, observations); !static {
			frontier.raiseToken(FrontierConfig, site, token)
		}
	}
	// TJAA-V0-037: the general adapter resolves tsconfig aliases as the Playwright config mode
	// does only when Playwright is the sole runner identity observed, because another runner's
	// loader (a Jest moduleNameMapper, a Vite resolve.alias) need not follow tsconfig paths.
	// TJAA-V0-057: otherwise it still adds the edges tsconfig resolution names, and the frontier
	// keeps the tsconfig and the blocking identity as pinned sites (TJAA-V0-058).
	// Under the blocked gate the tsconfig edges are speculative: a bare reference that names a
	// workspace package keeps its workspace link beside the tsconfig edge, so the selection is a
	// superset of the gate's own (TJAA-V0-059).
	var keep func(string) bool
	if len(aliasConfigs) != 0 && !resolveAliases {
		if blocker, only := playwrightOnlyRunners(configs, scopes, observations, aliasFreeTests(observations, bodies, scopes, opaque, root.HeldIgnored())); !only {
			keep = func(ref string) bool { return localPackage(importedPackage(ref), scopes) }
			for _, relative := range aliasConfigs {
				body := bodies[relative]
				offset := pathAliasMarker(body)
				frontier.at(FrontierPathAlias, relative, body, offset)
				frontier.pin(FrontierPathAlias, lineSite(relative, body, offset))
			}
			// TJAA-V0-050: the identity that kept the aliases unresolved is a site too.
			frontier.raiseToken(FrontierPathAlias, blocker.site, blocker.token)
			frontier.pin(FrontierPathAlias, blocker.site)
		}
		resolveAliases = true
	}
	var aliases []typeScriptAliases
	if resolveAliases {
		aliases = readTypeScriptAliases(bodies, frontier)
	}
	// declared is every alias config, read once for the reason of an unresolved bare import, so an
	// alias-shaped specifier no config claims is classified the same way wherever it is raised.
	var declared []typeScriptAliases
	reason := func(owner, ref string) string {
		if declared == nil {
			declared = readTypeScriptAliases(bodies, frontierSet{})
		}
		return bareImportReason(owner, ref, declared, bodies)
	}
	for index := range observations {
		observation := &observations[index]
		if resolveAliases {
			observation.refs = resolveTypeScriptAliases(observation.path, observation.refs, aliases, bodies, sites, frontier, keep)
		}
		if ref, unresolved := unresolvedBareImport(root, observation.path, observation.refs, scopes); unresolved {
			sites.raiseRef(frontier, reason(observation.path, ref), observation.path, ref)
		}
	}
	return buildResult(observations, configs, scopes, sites, frontier, root.HeldIgnored(), reason), frontier, nil
}

// pathAliasMarker returns the offset of the first `"paths"` or `"baseUrl"` key in a tsconfig.
func pathAliasMarker(body string) int {
	offset := -1
	for _, key := range []string{`"paths"`, `"baseUrl"`} {
		if found := strings.Index(body, key); found >= 0 && (offset < 0 || found < offset) {
			offset = found
		}
	}
	return offset
}

// runnerBlocker names the first runner identity that kept tsconfig aliases unresolved: its site
// and the runner or token there.
type runnerBlocker struct{ site, token string }

// playwrightOnlyRunners reports whether every runner identity the observation found is
// Playwright: each recognized runner config, each configured runner (a package.json `mocha`,
// `jest` or `detox` member, a Deno or Bun manifest), and each test observation. A package
// dependency alone assigns no runner (TJAA-V0-003) and loads no module, so a non-Playwright one
// does not block while every config and test is Playwright's (TJAA-V0-050). A Playwright config
// that names a tsconfig outside comments is excluded because it can select a module-resolution
// config other than the nearest tsconfig.json. A test of another runner in aliasFree, whose
// import closure meets no tsconfig alias, does not block once a Playwright identity is seen
// (AFP-V0-389). Otherwise it returns the first blocking identity.
func playwrightOnlyRunners(configs []config, scopes []packageScope, observations []observation, aliasFree map[string]bool) (runnerBlocker, bool) {
	seen := false
	for _, config := range configs {
		if config.runner != runnerPlaywright {
			return runnerBlocker{site: config.path, token: config.runner}, false
		}
		if config.tsconfigAt >= 0 {
			return runnerBlocker{site: lineSite(config.path, config.body, config.tsconfigAt), token: "tsconfig"}, false
		}
		seen = true
	}
	for _, scope := range scopes {
		for _, runner := range sortedKeys(scope.configured) {
			if runner != runnerPlaywright {
				return runnerBlocker{site: scope.manifest, token: runner}, false
			}
			seen = true
		}
		seen = seen || scope.runners[runnerPlaywright]
	}
	exempt := runnerBlocker{}
	for _, observation := range observations {
		if !observation.test {
			continue
		}
		if observation.runner != runnerPlaywright {
			if aliasFree[observation.path] {
				if exempt.site == "" {
					exempt = runnerBlocker{site: observation.path, token: observation.runner}
				}
				continue
			}
			return runnerBlocker{site: observation.path, token: observation.runner}, false
		}
		seen = true
	}
	if !seen && exempt.site != "" {
		return exempt, false
	}
	return runnerBlocker{}, seen
}

// tsconfigToken returns the offset of the first `tsconfig` text outside comments, or -1. A config
// that does not tokenize, or whose comment boundaries an ambiguous slash could move, is searched
// whole, so a comment cannot hide a real option.
func tsconfigToken(body string) int {
	if !strings.Contains(body, "tsconfig") {
		return -1
	}
	clean, err := stripComments(body, false)
	if err != nil || ambiguousSlash(body, clean, false) >= 0 {
		return strings.Index(body, "tsconfig")
	}
	return strings.Index(clean, "tsconfig")
}

func buildResult(observations []observation, configs []config, scopes []packageScope, sites sourceSites, frontier frontierSet, excluded func(string) bool, aliasReason func(owner, ref string) string) affected.Result {
	units := make([]affected.Unit, 0, len(observations))
	pathToID := make(map[string]string, len(observations))
	refs := make(map[string]map[string]bool, len(observations))
	direct := make(map[string]map[string]bool, len(observations))
	storybookGroups := make(map[string]*affected.Unit)
	storybookRefs := make(map[string]map[string]bool)
	storybookTokens := make(map[string]map[string]bool)

	for _, observation := range observations {
		if observation.runner == runnerStorybook {
			groupID := storybookUnitID(observation.path)
			unit := storybookGroups[groupID]
			if unit == nil {
				unit = &affected.Unit{ID: groupID}
				storybookGroups[groupID] = unit
				storybookRefs[groupID] = make(map[string]bool)
				storybookTokens[groupID] = make(map[string]bool)
			}
			unit.Tests = append(unit.Tests, observation.path)
			tokens, bounded := pathTokens(observation.body)
			unit.PathTokensBounded = unit.PathTokensBounded || bounded
			for _, token := range tokens {
				storybookTokens[groupID][token] = true
			}
			pathToID[observation.path] = groupID
			for _, ref := range observation.refs {
				storybookRefs[groupID][ref] = true
			}
			continue
		}
		id := unitID(observation.runner, observation.path)
		unit := affected.Unit{ID: id}
		unit.PathTokens, unit.PathTokensBounded = pathTokens(observation.body)
		if observation.test {
			unit.Tests = []string{observation.path}
		} else {
			unit.Sources = []string{observation.path}
		}
		units = append(units, unit)
		pathToID[observation.path] = id
		refs[id] = stringSet(observation.refs)
		direct[id] = make(map[string]bool)
		for _, configPath := range observation.configs {
			direct[id][configPath] = true
		}
	}
	for id, unit := range storybookGroups {
		sort.Strings(unit.Tests)
		if unit.PathTokensBounded || len(storybookTokens[id]) > maxPathTokens {
			unit.PathTokens, unit.PathTokensBounded = nil, true
		} else if len(storybookTokens[id]) != 0 {
			unit.PathTokens = sortedKeys(storybookTokens[id])
		}
		units = append(units, *unit)
		refs[id] = storybookRefs[id]
		direct[id] = make(map[string]bool)
		for _, config := range configs {
			if config.runner == runnerStorybook && inside(unit.Tests[0], config.directory) {
				direct[id][config.path] = true
			}
		}
	}

	packages := newWorkspace(scopes, units)
	for index := range units {
		unit := &units[index]
		edges := make(map[string]bool)
		owner := firstPath(*unit)
		resolved := make(map[string]bool)
		for ref := range refs[unit.ID] {
			if !packages.link(ref, edges) {
				site, token := unitRefSite(*unit, sites, ref)
				frontier.raiseToken(FrontierPathAlias, site, token)
			}
			target, local, unresolved := resolveImport(owner, ref, pathToID, excluded)
			if target != "" && target != unit.ID {
				edges[target] = true
			}
			if target != "" {
				resolved[ref] = true
			}
			if local && unresolved {
				site, token := unitRefSite(*unit, sites, ref)
				frontier.raiseToken(FrontierUnresolvedImport, site, token)
			}
			if !local && unresolved && looksLikeAlias(ref) {
				site, token := unitRefSite(*unit, sites, ref)
				frontier.raiseToken(aliasReason(owner, ref), site, token)
			}
		}
		for configPath := range direct[unit.ID] {
			if target := pathToID[configPath]; target != "" && target != unit.ID {
				edges[target] = true
			}
		}
		unit.Imports = sortedKeys(edges)
		unresolvedSpecifierTokens(unit, sites, resolved)
	}
	units = append(units, packages.built()...)
	sort.Slice(units, func(left, right int) bool { return units[left].ID < units[right].ID })
	return affected.Result{Units: units, Frontier: sortedKeys(frontier)}
}

// unresolvedSpecifierTokens recomputes a single-module unit's path tokens without the literal
// specifiers that resolved to a unit (TJAA-V0-126): such a module is reached through its import
// edge, so its specifier text naming a same-named file elsewhere would only over-select. A
// specifier left unresolved, an alias-rewritten one and a Storybook group keep their tokens. A
// module that builds its own require with createRequire keeps every token: its resolution base
// is unknown, so a resolved specifier may name another file.
func unresolvedSpecifierTokens(unit *affected.Unit, sites sourceSites, resolved map[string]bool) {
	if len(resolved) == 0 || len(unit.Sources)+len(unit.Tests) != 1 {
		return
	}
	index := sites[firstPath(*unit)]
	if index == nil || strings.Contains(index.body, "createRequire") {
		return
	}
	skip := make(map[int]string)
	for offset, text := range index.specifiers {
		if resolved[text] {
			skip[offset] = text
		}
	}
	if len(skip) != 0 {
		unit.PathTokens, unit.PathTokensBounded = pathTokensSkipping(index.body, skip)
	}
}

// unitRefSite names the first file of unit whose scan parsed ref, at that line, or the unit's
// first file when none did, and the specifier that file spelled for ref.
func unitRefSite(unit affected.Unit, sites sourceSites, ref string) (string, string) {
	for _, relative := range append(append([]string(nil), unit.Sources...), unit.Tests...) {
		if site := sites.ref(relative, ref); site != relative {
			return site, sites.spelling(relative, ref)
		}
	}
	return firstPath(unit), sites.spelling(firstPath(unit), ref)
}

func classify(relative, body string, refs []string, configs []config, scopes []packageScope, sites sourceSites, frontier frontierSet) observation {
	observation := observation{path: relative}
	explicit := explicitRunners(body, refs)
	possible := possibleRunners(relative, configs, scopes)
	observation.puppeteer = importsPackage(refs, "puppeteer") || importsPackage(refs, "puppeteer-core")
	candidate := isTestCandidate(relative) || len(explicit) != 0 || observation.puppeteer || conventionCandidate(relative, conventionRunners(relative, possible, scopes))
	if !candidate || isConfigPath(relative, body) {
		return observation
	}
	observation.test = true
	// Mocha and another configured/imported runner may both discover this path.
	// Preserve that competing ownership rather than prioritizing an import.
	mochaConflict := false
	if explicit[runnerMocha] || possible[runnerMocha] {
		owners := make(map[string]bool)
		for runner := range explicit {
			owners[runner] = true
		}
		for runner := range possible {
			owners[runner] = true
		}
		mochaConflict = len(owners) > 1
	}
	// TJAA-V0-052: a runner classification names the competing runners as its token.
	competing := func(groups ...map[string]bool) string {
		owners := map[string]bool{}
		for _, group := range groups {
			for runner := range group {
				owners[runner] = true
			}
		}
		return strings.Join(sortedKeys(owners), ",")
	}
	if mochaConflict {
		observation.runner = runnerUnknown
		frontier.raiseToken(FrontierAmbiguousRunner, relative, competing(explicit, possible))
	} else if len(explicit) == 1 {
		observation.runner = sortedKeys(explicit)[0]
	} else if len(explicit) > 1 {
		observation.runner = runnerUnknown
		frontier.raiseToken(FrontierAmbiguousRunner, relative, competing(explicit))
	} else {
		if len(possible) == 1 {
			observation.runner = sortedKeys(possible)[0]
		} else if len(possible) > 1 {
			observation.runner = runnerUnknown
			frontier.raiseToken(FrontierAmbiguousRunner, relative, competing(possible))
		} else {
			observation.runner = runnerUnknown
			frontier.raise(FrontierUnknownRunner, relative)
		}
	}
	observation.configs = nearestConfigs(relative, observation.runner, configs)
	if mochaConflict {
		for _, config := range configs {
			if inside(relative, config.directory) {
				observation.configs = append(observation.configs, config.path)
			}
		}
	}
	if len(observation.configs) > 1 {
		frontier.raiseToken(FrontierAmbiguousRunner, relative, strings.Join(observation.configs, ","))
	}
	// TJAA-V0-037: a Playwright-owned module outside the default testMatch is support code, such
	// as a page object importing @playwright/test, not a test unit. Under a config whose
	// membership markers the general adapter does not model (which already sets FrontierConfig),
	// a conventional test path stays a test so a custom-matched test is not silently demoted;
	// the Playwright config mode re-splits membership from the parsed matchers either way.
	// TJAA-V0-092: a path a literal project testMatch of its config owns is a test wherever it lives.
	if observation.runner == runnerPlaywright && !observation.puppeteer && !playwrightDefaultTest(relative) && !(customMembership(observation.configs, configs) && isTestCandidate(relative)) && !playwrightLiteralOwned(relative, observation.configs, configs) {
		observation.test, observation.runner, observation.configs = false, "", nil
		return observation
	}
	// TJAA-V0-052: a whole-file classification is named at the runner package's parsed import
	// specifier when the module imports it, and otherwise at the file with the runner as token.
	runnerSite := func(reason, runner string, packages ...string) {
		for _, ref := range refs {
			for _, name := range packages {
				if ref == name || strings.HasPrefix(ref, name+"/") {
					sites.raiseRef(frontier, reason, relative, ref)
					return
				}
			}
		}
		frontier.raiseToken(reason, relative, runner)
	}
	if observation.puppeteer && observation.runner == runnerUnknown {
		runnerSite(FrontierPuppeteerHost, "puppeteer", "puppeteer", "puppeteer-core")
	}
	if requiresRuntimeFlags(relative, observation.runner) {
		runnerSite(FrontierRuntimeFlags, observation.runner, runnerPackages[observation.runner]...)
	}
	if isE2ERunner(observation.runner) || observation.puppeteer {
		packages := runnerPackages[observation.runner]
		if observation.puppeteer {
			packages = append(append([]string(nil), packages...), "puppeteer", "puppeteer-core")
		}
		runner := observation.runner
		if !isE2ERunner(runner) {
			runner = "puppeteer"
		}
		runnerSite(FrontierE2ERuntimeDependency, runner, packages...)
	}
	if observation.runner == runnerDetox {
		runnerSite(FrontierDetoxConfiguration, runnerDetox, runnerPackages[runnerDetox]...)
	}
	if observation.runner == runnerStorybook {
		frontier.raiseToken(FrontierStorybookAddressing, relative, runnerStorybook)
	}
	return observation
}

// runnerPackages are the import specifiers that name each runner a whole-file frontier names.
var runnerPackages = map[string][]string{
	runnerPlaywright: {"@playwright/test"}, runnerCypress: {"cypress"}, runnerWebdriverIO: {"@wdio/globals"},
	runnerTestCafe: {"testcafe"}, runnerNightwatch: {"nightwatch"}, runnerDetox: {"detox"},
	runnerMocha: {"mocha"}, runnerNode: {"node:test"},
}

func customMembership(paths []string, configs []config) bool {
	for _, config := range configs {
		if config.membership && containsString(paths, config.path) {
			return true
		}
	}
	return false
}

func explicitRunners(body string, refs []string) map[string]bool {
	runners := make(map[string]bool)
	for _, ref := range refs {
		switch {
		case importsPackage([]string{ref}, "vitest"):
			runners[runnerVitest] = true
		case ref == "mocha":
			runners[runnerMocha] = true
		case ref == "@jest/globals":
			runners[runnerJest] = true
		case importsPackage([]string{ref}, "ava"):
			runners[runnerAVA] = true
		case ref == "node:test" || strings.HasPrefix(ref, "node:test/"):
			runners[runnerNode] = true
		case ref == "@playwright/test":
			runners[runnerPlaywright] = true
		case ref == "bun:test":
			runners[runnerBun] = true
		case importsPackage([]string{ref}, "cypress"):
			runners[runnerCypress] = true
		case ref == "@wdio/globals":
			runners[runnerWebdriverIO] = true
		case importsPackage([]string{ref}, "testcafe"):
			runners[runnerTestCafe] = true
		case importsPackage([]string{ref}, "nightwatch"):
			runners[runnerNightwatch] = true
		case importsPackage([]string{ref}, "detox"):
			runners[runnerDetox] = true
		case ref == "@storybook/addon-vitest/vitest-plugin":
			runners[runnerStorybookVitest] = true
		}
	}
	if strings.Contains(body, "Deno.test") {
		runners[runnerDeno] = true
	}
	return runners
}

func possibleRunners(relative string, configs []config, scopes []packageScope) map[string]bool {
	runners := make(map[string]bool)
	for _, config := range configs {
		if inside(relative, config.directory) {
			runners[config.runner] = true
		}
	}
	if scope, known := nearestScope(relative, scopes); known {
		for runner := range scope.configured {
			runners[runner] = true
		}
		for _, runner := range []string{runnerStorybook, runnerStorybookVitest} {
			if scope.runners[runner] {
				runners[runner] = true
			}
		}
	}
	base := path.Base(relative)
	slashed := "/" + relative
	if strings.Contains(base, ".cy.") || strings.Contains(slashed, "/cypress/") {
		return keepRunner(runners, runnerCypress)
	}
	if strings.Contains(base, ".stories.") {
		if runners[runnerStorybookVitest] {
			return map[string]bool{runnerStorybookVitest: true}
		}
		return keepRunner(runners, runnerStorybook)
	}
	if strings.Contains(slashed, "/playwright/") {
		return keepRunner(runners, runnerPlaywright)
	}
	return runners
}

func nearestConfigs(relative, runner string, configs []config) []string {
	best := -1
	paths := make([]string, 0, 2)
	for _, config := range configs {
		if config.runner != runner || !inside(relative, config.directory) {
			continue
		}
		depth := len(strings.Split(config.directory, "/"))
		if depth < best {
			continue
		}
		if depth > best {
			best = depth
			paths = paths[:0]
		}
		paths = append(paths, config.path)
	}
	sort.Strings(paths)
	return paths
}

func scanImports(relative, body string) ([]string, bool, error) {
	scan := scanImportSites(relative, body)
	return scan.refs, scan.dynamic >= 0, scan.err
}

// importScan is what one module's import scan observed. dynamic, unresolved and syntax are byte
// offsets into the module of the first computed loading call, the first ambiguous slash and the
// tokenization failure, or -1 when there is none or the offset is not known (TJAA-V0-039).
//
// offsets holds, for each reference, the byte offset of its first parsed specifier, so a site is
// the specifier's own line, found without searching the text again (TJAA-V0-040).
type importScan struct {
	refs       []string
	offsets    map[string]int
	specifiers map[int]string
	dynamic    int
	unresolved int
	syntax     int
	err        error
}

func scanImportSites(relative, body string) importScan {
	scan := importScan{dynamic: -1, unresolved: -1, syntax: -1}
	withoutComments, parseErr := stripComments(body, jsxSource(relative))
	var located sourceSyntaxError
	if errors.As(parseErr, &located) {
		scan.syntax = located.offset
	}
	offsets := make(map[string]int)
	scan.specifiers = make(map[int]string)
	add := func(ref string, offset int) {
		ref = withoutBundlerQuery(ref)
		if previous, seen := offsets[ref]; !seen || offset < previous {
			offsets[ref] = offset
		}
	}
	for _, pattern := range staticImportPatterns {
		for _, match := range pattern.FindAllStringSubmatchIndex(withoutComments, -1) {
			add(withoutComments[match[2]:match[3]], match[2])
			scan.specifiers[match[2]] = withoutComments[match[2]:match[3]]
		}
	}
	requires, computedLoading, requiresParsed := scanRequires(withoutComments, typedSource(relative))
	if !requiresParsed && parseErr == nil {
		parseErr = strconv.ErrSyntax
	}
	scan.dynamic = computedLoading
	// A slash the lexer keeps as division although it may open a regular expression can
	// misplace string, comment and code boundaries after it, hiding an import, require or
	// comment-stripped static import. Then any loading word keeps the module unresolved
	// (TJAA-V0-034, TJAA-V0-038): the frontier stays rather than an edge being dropped.
	if parseErr == nil && computedLoading < 0 && loadingWord.MatchString(body) {
		scan.unresolved = ambiguousSlash(body, withoutComments, jsxSource(relative))
	}
	for _, ref := range requires {
		add(ref.text, ref.offset)
		scan.specifiers[ref.offset] = ref.text
	}
	unified := unicodeLineTerminators.Replace(body)
	for _, match := range referencePathPattern.FindAllStringSubmatchIndex(unified, -1) {
		add(relativeReference(unified[match[2]:match[3]]), match[2])
	}
	scan.refs, scan.offsets, scan.err = sortedKeys(offsets), offsets, parseErr
	return scan
}

// withoutBundlerQuery drops a bundler query suffix, such as Vite's `?url`, `?raw` or
// `?worker`, from a specifier that names no URL scheme, so the module it loads is resolved as the
// path or package before it (TJAA-V0-771).
func withoutBundlerQuery(ref string) string {
	if externalScheme(ref) {
		return ref
	}
	if cut := strings.IndexByte(ref, '?'); cut > 0 {
		return ref[:cut]
	}
	return ref
}

// loadingWord matches a word that can begin module loading anywhere, literal text included.
var loadingWord = regexp.MustCompile(`\b(?:import|require)\b`)

// ambiguousSlash returns the offset of the first code `/` that regexStart keeps as an ambiguous
// division although reading it as a regular-expression literal that closes on the same line would
// change where literals, substitutions or comments end: the literal holds a quote, backtick or
// brace, or its closing slash is followed by `/` or `*`. It returns -1 when there is none.
// The walk follows the import scan's lexer over clean, the comment-stripped text the scanners
// read, so a slash inside a string, template text, a comment or a regular-expression literal that
// regexStart admits is not a token and is skipped (TJAA-V0-038). Up to the first ambiguous slash
// that lexer reads the boundaries the language does, so that slash is the first place the two
// readings can diverge. The regular-expression reading is taken over the raw body, where the
// division reading may have blanked a comment. A comment opener, and in a JSX source the slash of
// a `</` or `/>` tag, is not ambiguous.
func ambiguousSlash(body, clean string, jsx bool) int {
	_, found := ambiguousSlashCode(body, clean, 0, false, 0, jsx)
	return found
}

func ambiguousSlashCode(body, clean string, index int, stopAtBrace bool, nesting int, jsx bool) (int, int) {
	depth := 0
	for index < len(clean) {
		switch character := clean[index]; {
		case character == '\'' || character == '"':
			index = quotedEnd(clean, index)
		case character == '`':
			end, found := ambiguousSlashTemplate(body, clean, index+1, nesting, jsx)
			if found >= 0 {
				return end, found
			}
			index = end
		case character == '/':
			if end, regex := regexEnd(clean, index); regex {
				index = end
				continue
			}
			if slashRereads(body, clean, index, jsx) {
				return index, index
			}
			index++
		case character == '{':
			depth++
			index++
		case character == '}':
			if stopAtBrace && depth == 0 {
				return index + 1, -1
			}
			if depth > 0 {
				depth--
			}
			index++
		default:
			index++
		}
	}
	return len(clean), -1
}

func ambiguousSlashTemplate(body, clean string, index, nesting int, jsx bool) (int, int) {
	for index < len(clean) {
		switch {
		case clean[index] == '\\':
			index += 2
		case strings.HasPrefix(clean[index:], "${"):
			if nesting == jsTemplateMaxDepth {
				return len(clean), -1
			}
			end, found := ambiguousSlashCode(body, clean, index+2, true, nesting+1, jsx)
			if found >= 0 {
				return end, found
			}
			index = end
		case clean[index] == '`':
			return index + 1, -1
		default:
			index++
		}
	}
	return len(clean), -1
}

// slashRereads reports whether the division slash at index of clean is ambiguous and reading it
// as a same-line regular-expression literal of the raw body would move a literal, substitution or
// comment boundary.
func slashRereads(body, clean string, index int, jsx bool) bool {
	after := byte(0)
	if index+1 < len(body) {
		after = body[index+1]
	}
	tag := jsx && (after == '>' || index > 0 && clean[index-1] == '<')
	if _, ambiguous := regexStart(clean, index); !ambiguous || after == '/' || after == '*' || tag {
		return false
	}
	end, closed := regexLiteralEnd(body, index)
	if !closed {
		return false
	}
	closing := end - 1
	for body[closing] != '/' {
		closing--
	}
	// The division reading turns an inner `/` next to another `/` or `*` (the closing slash
	// included, as in `/a\//`) into a comment opener.
	span := body[index+1 : min(closing+2, len(body))]
	return strings.ContainsAny(body[index+1:closing], "'\"`{}") || strings.Contains(span, "//") || strings.Contains(span, "/*")
}

// sourceSyntaxError is a tokenization failure at a byte offset of the scanned text.
type sourceSyntaxError struct{ offset int }

func (failure sourceSyntaxError) Error() string {
	return "invalid syntax at byte offset " + strconv.Itoa(failure.offset)
}

func (sourceSyntaxError) Unwrap() error { return strconv.ErrSyntax }

// lineAt returns the 1-based line holding offset. CR LF, a lone CR, LF, U+2028 and U+2029 each
// end a line, as in ECMAScript.
func lineAt(body string, offset int) int {
	line := 1
	for index := 0; index < offset && index < len(body); index++ {
		switch {
		case body[index] == '\n':
			line++
		case body[index] == '\r' && (index+1 == len(body) || body[index+1] != '\n'):
			line++
		case unicodeLineTerminatorAt(body, index):
			line++
			index += 2
		}
	}
	return line
}

// unicodeLineTerminators maps U+2028 and U+2029 to LF plus two spaces (the same three bytes), so
// the line-anchored reference pattern sees the lines ECMAScript and TypeScript see at unchanged
// offsets.
var unicodeLineTerminators = strings.NewReplacer("\u2028", "\n  ", "\u2029", "\n  ")

// typedSource reports whether relative is TypeScript, the only source where `import('x')` can be
// a type-position import type rather than a runtime dynamic import.
func typedSource(relative string) bool {
	lower := strings.ToLower(relative)
	return strings.HasSuffix(lower, ".ts") || strings.HasSuffix(lower, ".tsx") || strings.HasSuffix(lower, ".mts") || strings.HasSuffix(lower, ".cts")
}

// referencePathPattern reads `/// <reference path="...">` from the unstripped body, because the
// directive is a line comment. Its path is always file-relative, even without a leading `./`.
var referencePathPattern = regexp.MustCompile(`(?m)^[\t ]*///[\t ]*<reference[\t ]+path[\t ]*=[\t ]*["']([^"'\r\n]+)["']`)

func relativeReference(reference string) string {
	if strings.HasPrefix(reference, ".") || strings.HasPrefix(reference, "/") {
		return reference
	}
	return "./" + reference
}

var staticImportPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?m)(?:^|;)[\t ]*import(?:[\t ]+type)?[\t ]*["']([^"'\r\n]+)["']`),
	// The from keyword follows ECMAScript whitespace, `}` or `*`, never a quote, so a string
	// "from" in an exported word list is no clause (TJAA-V0-771).
	regexp.MustCompile(`(?ms)(?:^|;)[\t ]*(?:import|export)\b[^;]*?[` + ecmaScriptSpace + `}*]from[` + ecmaScriptSpace + `]*["']([^"'\r\n]+)["']`),
}

// ecmaScriptSpace is a regexp character-class body for ECMAScript WhiteSpace and LineTerminator:
// TAB, VT, FF, SP, U+00A0, U+FEFF, every Zs code point, LF and CR (U+2028 and U+2029 are already
// rewritten to LF), wider than Go's ASCII `\s` (TJAA-V0-771).
const ecmaScriptSpace = `\t\x{0B}\f \x{A0}\x{FEFF}\p{Zs}\n\r`

// scanRequires reads literal `require("x")` calls and reports computed loading: a `require` or
// `import(` call in code position (TJAA-V0-005). String and template text is skipped, while `${}`
// substitutions are scanned as code. In TypeScript (typed) a type-position import type with a
// literal specifier is recorded as a static reference instead (TJAA-V0-034).
func scanRequires(body string, typed bool) ([]importRef, int, bool) {
	refs := make([]importRef, 0, 4)
	dynamic := -1
	_, parsed := scanRequireCode(body, 0, false, 0, typed, &refs, &dynamic)
	return refs, dynamic, parsed
}

// importRef is one literal specifier and the byte offset where it starts.
type importRef struct {
	text   string
	offset int
}

// markDynamic records the offset of the first computed loading call.
func markDynamic(dynamic *int, offset int) {
	if *dynamic < 0 {
		*dynamic = offset
	}
}

// scanRequireCode scans code, recursing into templates; nesting counts the enclosing `${`
// substitutions and is bounded by jsTemplateMaxDepth.
func scanRequireCode(body string, index int, stopAtBrace bool, nesting int, typed bool, refs *[]importRef, dynamic *int) (int, bool) {
	depth := 0
	for index < len(body) {
		if body[index] == '\'' || body[index] == '"' {
			index = quotedEnd(body, index)
			continue
		}
		if body[index] == '`' {
			var closed bool
			index, closed = scanRequireTemplate(body, index+1, nesting, typed, refs, dynamic)
			if !closed {
				return len(body), false
			}
			continue
		}
		if index+1 < len(body) && body[index] == '/' && (body[index+1] == '/' || body[index+1] == '*') {
			var closed bool
			index, closed = jsCommentEnd(body, index)
			if !closed {
				return len(body), false
			}
			continue
		}
		if end, regex := regexEnd(body, index); regex {
			index = end
			continue
		}
		if body[index] == '{' {
			depth++
			index++
			continue
		}
		if body[index] == '}' {
			if stopAtBrace && depth == 0 {
				return index + 1, true
			}
			if depth > 0 {
				depth--
			}
			index++
			continue
		}
		if strings.HasPrefix(body[index:], "import") && !(index > 0 && (isIdentifier(body[index-1]) || body[index-1] == '.')) {
			end, ok := scanImportCall(body, index, typed, refs, dynamic)
			if !ok {
				return len(body), false
			}
			index = end
			continue
		}
		if !strings.HasPrefix(body[index:], "require") || index > 0 && (isIdentifier(body[index-1]) || body[index-1] == '.') {
			index++
			continue
		}
		after := index + len("require")
		if after < len(body) && isIdentifier(body[after]) {
			index = after
			continue
		}
		after = skipSpace(body, after)
		if after == len(body) || body[after] != '(' {
			index = after
			continue
		}
		after = skipSpace(body, after+1)
		if after == len(body) || body[after] != '\'' && body[after] != '"' {
			markDynamic(dynamic, index)
			index = after
			continue
		}
		end := quotedEnd(body, after)
		if end > len(body) {
			return len(body), false
		}
		*refs = append(*refs, importRef{text: body[after+1 : end-1], offset: after + 1})
		index = end
	}
	return len(body), !stopAtBrace
}

// scanImportCall inspects the `import` keyword at index. Any whitespace may precede the call
// parenthesis, including the spaces a stripped block comment leaves (`import /* c */ ("./x")`).
// A call is computed loading unless typed source proves a type-position import type: a literal
// specifier followed by a qualified type name (`import('x').Page`, not then/catch/finally, not
// called, tagged or awaited) or preceded by `typeof`. That specifier is kept as a static
// reference, so the edge survives even if the position was in fact a runtime member read.
func scanImportCall(body string, index int, typed bool, refs *[]importRef, dynamic *int) (int, bool) {
	after := index + len("import")
	if after < len(body) && isIdentifier(body[after]) {
		return after, true
	}
	open := skipSpace(body, after)
	if open == len(body) || body[open] != '(' {
		return open, true
	}
	literal := skipSpace(body, open+1)
	if !typed || literal == len(body) || body[literal] != '\'' && body[literal] != '"' {
		markDynamic(dynamic, index)
		return open + 1, true
	}
	end := quotedEnd(body, literal)
	if end > len(body) {
		return len(body), false
	}
	closeParen := skipSpace(body, end)
	if closeParen == len(body) || body[closeParen] != ')' {
		markDynamic(dynamic, index)
		return end, true
	}
	if importTypeQualified(body, closeParen+1) && !previousWordIs(body, index, "await") || previousWordIs(body, index, "typeof") {
		*refs = append(*refs, importRef{text: body[literal+1 : end-1], offset: literal + 1})
	} else {
		markDynamic(dynamic, index)
	}
	return closeParen + 1, true
}

// importTypeQualified reports whether body[index:] is `.Name(.Name)*` that is not followed by a
// call, template tag, optional chain or non-null assertion, and names no promise method.
func importTypeQualified(body string, index int) bool {
	qualified := false
	for {
		dot := skipSpace(body, index)
		if dot == len(body) || body[dot] != '.' {
			break
		}
		start := skipSpace(body, dot+1)
		end := start
		for end < len(body) && isIdentifier(body[end]) {
			end++
		}
		switch name := body[start:end]; {
		case name == "" || name[0] >= '0' && name[0] <= '9':
			return false
		case name == "then" || name == "catch" || name == "finally":
			return false
		}
		qualified = true
		index = end
	}
	next := skipSpace(body, index)
	if next < len(body) && strings.IndexByte("(`!", body[next]) >= 0 || strings.HasPrefix(body[next:], "?.") {
		return false
	}
	return qualified
}

// previousWordIs reports whether the code before index, ignoring whitespace, ends with the whole
// identifier word.
func previousWordIs(body string, index int, word string) bool {
	end := index
	for end > 0 && strings.IndexByte(" \t\r\n", body[end-1]) >= 0 {
		end--
	}
	start := end - len(word)
	return start >= 0 && body[start:end] == word && (start == 0 || !isIdentifier(body[start-1]) && body[start-1] != '.')
}

func scanRequireTemplate(body string, index, nesting int, typed bool, refs *[]importRef, dynamic *int) (int, bool) {
	for index < len(body) {
		switch {
		case body[index] == '\\':
			index += 2
		case strings.HasPrefix(body[index:], "${"):
			if nesting == jsTemplateMaxDepth {
				return len(body), false
			}
			var closed bool
			index, closed = scanRequireCode(body, index+2, true, nesting+1, typed, refs, dynamic)
			if !closed {
				return len(body), false
			}
		case body[index] == '`':
			return index + 1, true
		default:
			index++
		}
	}
	return len(body), false
}

// unicodeLineTerminatorAt reports whether body[index:] starts with U+2028 or U+2029, which
// ECMAScript treats as line terminators alongside LF and CR: they end a `//` comment and a line.
func unicodeLineTerminatorAt[T string | []byte](body T, index int) bool {
	return index+2 < len(body) && body[index] == 0xE2 && body[index+1] == 0x80 && (body[index+2] == 0xA8 || body[index+2] == 0xA9)
}

func jsCommentEnd(body string, index int) (int, bool) {
	line := body[index+1] == '/'
	for index += 2; index < len(body); index++ {
		if line && (body[index] == '\n' || body[index] == '\r' || unicodeLineTerminatorAt(body, index)) {
			return index, true
		}
		if !line && index+1 < len(body) && body[index] == '*' && body[index+1] == '/' {
			return index + 2, true
		}
	}
	return len(body), line
}

// jsTemplateMaxDepth bounds how many `${` substitutions a template-literal scanner keeps open at
// once; deeper nesting is refused as unparsed source instead of being scanned.
const jsTemplateMaxDepth = 64

// templateChunkEnd scans template content from start, just after a backtick or the `}` that
// closes a substitution. It returns the offset past the closing backtick, or past `${` with
// opened set; an unterminated template returns len(body)+1.
func templateChunkEnd[T string | []byte](body T, start int) (end int, opened bool) {
	for index := start; index < len(body); index++ {
		switch {
		case body[index] == '\\':
			index++
		case body[index] == '`':
			return index + 1, false
		case body[index] == '$' && index+1 < len(body) && body[index+1] == '{':
			return index + 2, true
		}
	}
	return len(body) + 1, false
}

// templateEnd returns the offset past the template literal whose backtick is body[start],
// following nested substitutions and templates with a stack of substitution brace depths. Code in
// a substitution skips strings, comments and regular expressions. An unterminated template, or
// one nesting more than jsTemplateMaxDepth substitutions, returns len(body)+1.
func templateEnd(body string, start int) int {
	substitutions := []int{}
	index, opened := templateChunkEnd(body, start+1)
	for index <= len(body) {
		if opened {
			if len(substitutions) == jsTemplateMaxDepth {
				break
			}
			substitutions = append(substitutions, 0)
		} else if len(substitutions) == 0 {
			return index
		}
		next, closing := substitutionCodeEnd(body, index, &substitutions[len(substitutions)-1])
		if next >= len(body) {
			break
		}
		if closing {
			substitutions = substitutions[:len(substitutions)-1]
		}
		index, opened = templateChunkEnd(body, next+1)
	}
	return len(body) + 1
}

// substitutionCodeEnd scans substitution code from index to the backtick that opens a nested
// template or, with closing set, the `}` that closes the substitution; depth counts its braces.
// It returns len(body) when neither occurs.
func substitutionCodeEnd(body string, index int, depth *int) (int, bool) {
	for index < len(body) {
		switch character := body[index]; {
		case character == '\'' || character == '"':
			index = quotedEnd(body, index)
		case character == '`':
			return index, false
		case character == '/' && index+1 < len(body) && (body[index+1] == '/' || body[index+1] == '*'):
			end, closed := jsCommentEnd(body, index)
			if !closed {
				return len(body), false
			}
			index = end
		case character == '{':
			*depth++
			index++
		case character == '}':
			if *depth == 0 {
				return index, true
			}
			*depth--
			index++
		default:
			if end, regex := regexEnd(body, index); regex {
				index = end
			} else {
				index++
			}
		}
	}
	return len(body), false
}

// stripComments blanks comments to spaces, keeping offsets and line structure. U+2028 and U+2029
// outside a string, template or regular expression literal are ECMAScript line terminators, so
// they are rewritten to LF plus two spaces (the same three bytes) for the line-based scanners.
// Template content is never rewritten: substitutions holds the brace depth of each open `${`, so a
// backtick inside one opens a nested template and its closing `}` resumes the enclosing template.
func stripComments(body string, rejectAmbiguousJSXQuotes bool) (string, error) {
	clean := []byte(body)
	substitutions := []int{}
	openedAt := []int{}
	for index := 0; index < len(clean); {
		if unicodeLineTerminatorAt(clean, index) {
			copy(clean[index:], "\n  ")
			index += 3
			continue
		}
		top := len(substitutions) - 1
		if clean[index] == '`' || clean[index] == '}' && top >= 0 && substitutions[top] == 0 {
			if clean[index] == '}' {
				substitutions, openedAt = substitutions[:top], openedAt[:top]
			}
			end, opened := templateChunkEnd(clean, index+1)
			if end > len(clean) {
				return string(clean), sourceSyntaxError{index}
			}
			if opened {
				if len(substitutions) == jsTemplateMaxDepth {
					return string(clean), sourceSyntaxError{end - 2}
				}
				substitutions, openedAt = append(substitutions, 0), append(openedAt, end-2)
			}
			index = end
			continue
		}
		if top >= 0 && clean[index] == '{' {
			substitutions[top]++
		} else if top >= 0 && clean[index] == '}' {
			substitutions[top]--
		}
		if clean[index] == '\'' || clean[index] == '"' {
			end := quotedEnd(clean, index)
			if end > len(clean) {
				return string(clean), sourceSyntaxError{index}
			}
			if rejectAmbiguousJSXQuotes && (!jsxQuoteStartsLiteral(clean, index) || jsxQuotedTokenCouldHideRequire(clean[index:end]) || strings.ContainsAny(string(clean[index:end]), "\r\n\u2028\u2029")) {
				return string(clean), sourceSyntaxError{index}
			}
			index = end
			continue
		}
		if index+1 == len(clean) || clean[index] != '/' || clean[index+1] != '/' && clean[index+1] != '*' {
			if end, regex := regexEnd(clean, index); regex {
				index = end
				continue
			}
			index++
			continue
		}
		line, opener := clean[index+1] == '/', index
		clean[index], clean[index+1] = ' ', ' '
		index += 2
		for index < len(clean) {
			if unicodeLineTerminatorAt(clean, index) {
				copy(clean[index:], "\n  ")
				if line {
					break
				}
				index += 3
				continue
			}
			if line && (clean[index] == '\n' || clean[index] == '\r') {
				break
			}
			if !line && index+1 < len(clean) && clean[index] == '*' && clean[index+1] == '/' {
				clean[index], clean[index+1] = ' ', ' '
				index += 2
				break
			}
			if clean[index] != '\n' && clean[index] != '\r' {
				clean[index] = ' '
			}
			index++
		}
		if !line && index == len(clean) {
			return string(clean), sourceSyntaxError{opener}
		}
	}
	if len(substitutions) != 0 {
		return string(clean), sourceSyntaxError{openedAt[len(openedAt)-1]}
	}
	return string(clean), nil
}

func jsxSource(relative string) bool {
	extension := strings.ToLower(path.Ext(relative))
	return extension == ".jsx" || extension == ".tsx"
}

func jsxQuotedTokenCouldHideRequire(token []byte) bool {
	text := string(token)
	for offset := 0; offset < len(text); {
		found := strings.Index(text[offset:], "require")
		if found < 0 {
			return false
		}
		found += offset
		beforeIdentifier := found > 0 && isIdentifier(text[found-1])
		after := found + len("require")
		afterIdentifier := after < len(text) && isIdentifier(text[after])
		if !beforeIdentifier && !afterIdentifier {
			return true
		}
		offset = after
	}
	return false
}

func jsxQuoteStartsLiteral(body []byte, index int) bool {
	prefix := bytes.TrimRight(body[:index], " \t")
	if len(prefix) == 0 || bytes.HasSuffix(prefix, []byte("\n")) || bytes.HasSuffix(prefix, []byte("\r")) {
		return true
	}
	if bytes.HasSuffix(prefix, []byte("=>")) || strings.ContainsRune("([={,:;!?&|+-*%^~", rune(prefix[len(prefix)-1])) {
		return true
	}
	wordStart := len(prefix)
	for wordStart > 0 && isIdentifier(prefix[wordStart-1]) {
		wordStart--
	}
	switch string(prefix[wordStart:]) {
	case "as", "await", "case", "default", "delete", "do", "else", "export", "from", "import", "in", "instanceof", "new", "of", "return", "throw", "typeof", "void", "yield":
		return true
	default:
		return false
	}
}

// regexOpeners are the bytes after which an expression must begin, so a `/`
// there opens a regular expression and cannot be division; the start of text
// counts as one. It mirrors the documented jsresolve rule (FPK-V0-018), and
// regexStart adds `=>` and the expression-leading keywords (TJAA-V0-034).
const regexOpeners = "(,=:[&|?{;~^%*"

// regexKeywords are the keywords after which an expression must begin. They count
// only as a whole word that is not a property name (`x.return / 2` is division).
var regexKeywords = map[string]bool{"return": true, "typeof": true, "instanceof": true, "in": true, "new": true, "delete": true, "void": true, "throw": true, "case": true, "do": true, "else": true}

// operandWords are the reserved words that are complete operands, so a `/` after them is division
// like after an identifier. Every other reserved or contextual keyword (`debugger`, `break`,
// `continue`, `yield`, `await`, `of`, `let`, ...) may end a statement that automatic semicolon
// insertion closes, or start one, so the slash after it is ambiguous (TJAA-V0-045).
var operandWords = map[string]bool{"this": true, "super": true, "null": true, "true": true, "false": true}

// slashKeywords are the reserved and contextual keywords that are not operands.
var slashKeywords = map[string]bool{
	"async": true, "await": true, "break": true, "catch": true, "class": true, "const": true, "continue": true,
	"debugger": true, "default": true, "enum": true, "export": true, "extends": true, "finally": true, "for": true,
	"function": true, "if": true, "implements": true, "import": true, "interface": true, "let": true, "of": true,
	"package": true, "private": true, "protected": true, "public": true, "static": true, "switch": true, "try": true,
	"var": true, "while": true, "with": true, "yield": true,
}

// regexStart reports whether the `/` at index opens a regular expression. It is
// ambiguous, and kept as division, when the predecessor needs grammar state: `)`
// and `}` may close a condition or block, `+` and `-` may end `++` or `--`, `<`
// and `>` may be JSX or type arguments, and `of`, `yield` and `await` may be
// identifiers. A `!` takes the verdict of the position it stands in. A reserved word
// that is not an operand, a non-ASCII byte and `\` are ambiguous. An identifier,
// an operand keyword, a literal and `]` are division.
func regexStart[T string | []byte](body T, index int) (regex, ambiguous bool) {
	previous := index
	for {
		for previous > 0 && strings.IndexByte(" \t\r\n", body[previous-1]) >= 0 {
			previous--
		}
		// A `!` is the prefix negation exactly when an operand may begin where it stands, and then
		// the slash after it opens a regular expression; otherwise it is a non-null assertion that
		// ends an operand (TJAA-V0-038). The same rule applies at the `!` itself.
		if previous == 0 || body[previous-1] != '!' {
			break
		}
		previous--
		// No line terminator may precede a non-null assertion, and JavaScript has none, so a `!`
		// that starts a line is the prefix negation.
		bang := previous
		for previous > 0 && strings.IndexByte(" \t", body[previous-1]) >= 0 {
			previous--
		}
		if previous == 0 || body[previous-1] == '\n' || body[previous-1] == '\r' || unicodeLineTerminatorAt(body, max(previous-3, 0)) && previous >= 3 {
			return true, false
		}
		previous = bang
	}
	if previous == 0 || strings.IndexByte(regexOpeners, body[previous-1]) >= 0 {
		return true, false
	}
	last := body[previous-1]
	if last == '>' && previous > 1 && body[previous-2] == '=' {
		return true, false
	}
	if isIdentifier(last) {
		start := previous
		for start > 0 && isIdentifier(body[start-1]) {
			start--
		}
		if start > 0 && body[start-1] == '.' {
			return false, false
		}
		word := string(body[start:previous])
		if regexKeywords[word] {
			return true, false
		}
		// An identifier escape or a non-ASCII identifier part before the word is not read here.
		if start > 0 && (body[start-1] == '\\' || body[start-1] >= utf8.RuneSelf) {
			return false, true
		}
		return false, slashKeywords[word] && !operandWords[word]
	}
	// A line break after the `)` may end a bodiless signature (`declare function f(a)`), so only a
	// same-line slash takes the call verdict.
	if last == ')' && !lineTerminatorBetween(body, previous, index) && callParenthesis(body, previous-1) {
		return false, false
	}
	// A non-ASCII byte may be an identifier part, a space or a line terminator, and `\` ends an
	// identifier escape: this ASCII classifier cannot tell, so the slash is ambiguous
	// (TJAA-V0-045).
	return false, last >= utf8.RuneSelf || last == '\\' || strings.IndexByte(")}+-<>", last) >= 0
}

// lineTerminatorBetween reports whether body[start:end] holds LF, CR, U+2028 or U+2029.
func lineTerminatorBetween[T string | []byte](body T, start, end int) bool {
	for index := start; index < end; index++ {
		if body[index] == '\n' || body[index] == '\r' || unicodeLineTerminatorAt(body, index) {
			return true
		}
	}
	return false
}

// callParenthesisWindow bounds how far callParenthesis looks back for the opening parenthesis, so
// classifying one slash stays constant work.
const callParenthesisWindow = 256

// callParenthesis reports whether the `)` at closing ends the argument list of a call: its
// matching `(` lies within callParenthesisWindow bytes, the text between holds no quote, backtick,
// slash, brace or non-ASCII byte that could hide a boundary, and the `(` directly follows a member
// name (`x.now(`), an identifier that is not a keyword (`now(`), or `)` or `]` (`f()(`, `a[0](`).
// A slash after such a `)` is division (TJAA-V0-046). A `)` that closes an `if`, `while`, `for` or
// `with` head, or follows any other keyword, stays ambiguous.
func callParenthesis[T string | []byte](body T, closing int) bool {
	depth := 0
	open := -1
	for index := closing; index >= 0 && closing-index <= callParenthesisWindow; index-- {
		character := body[index]
		if character >= utf8.RuneSelf || strings.IndexByte("'\"`/{}", character) >= 0 {
			return false
		}
		if character == ')' {
			depth++
		} else if character == '(' {
			depth--
			if depth == 0 {
				open = index
				break
			}
		}
	}
	if open < 0 {
		return false
	}
	previous := open
	for previous > 0 && strings.IndexByte(" \t", body[previous-1]) >= 0 {
		previous--
	}
	if previous == 0 {
		return false
	}
	last := body[previous-1]
	if last == ')' || last == ']' {
		return true
	}
	if !isIdentifier(last) {
		return false
	}
	start := previous
	for start > 0 && isIdentifier(body[start-1]) {
		start--
	}
	if start > 0 && (body[start-1] == '\\' || body[start-1] >= utf8.RuneSelf) {
		return false
	}
	if body[start] >= '0' && body[start] <= '9' {
		return false
	}
	if start > 0 && body[start-1] == '.' {
		return true
	}
	word := string(body[start:previous])
	return !regexKeywords[word] && !slashKeywords[word] && !operandWords[word]
}

// regexEnd returns the offset past a regular-expression literal and its flags
// when regexStart admits the `/` at index and the literal closes before any line
// terminator. Otherwise it reports false and the `/` stays division. Callers
// check for a comment start first.
func regexEnd[T string | []byte](body T, index int) (int, bool) {
	if body[index] != '/' {
		return 0, false
	}
	if regex, _ := regexStart(body, index); !regex {
		return 0, false
	}
	return regexLiteralEnd(body, index)
}

// regexLiteralEnd returns the offset past the regular-expression-shaped text that
// starts with the `/` at index, when it closes before any line terminator.
func regexLiteralEnd[T string | []byte](body T, index int) (int, bool) {
	escaped, inClass := false, false
	for position := index + 1; position < len(body); {
		character, width := utf8.DecodeRuneInString(string(body[position:min(position+utf8.UTFMax, len(body))]))
		position += width
		switch {
		case character == '\n' || character == '\r' || character == '\u2028' || character == '\u2029':
			return 0, false
		case escaped:
			escaped = false
		case character == '\\':
			escaped = true
		case inClass:
			inClass = character != ']'
		case character == '[':
			inClass = true
		case character == '/':
			for position < len(body) && isIdentifier(body[position]) {
				position++
			}
			return position, true
		}
	}
	return 0, false
}

func quotedEnd[T string | []byte](body T, start int) int {
	delimiter := body[start]
	for index := start + 1; index < len(body); index++ {
		if body[index] == '\\' {
			index++
			continue
		}
		if body[index] == delimiter {
			return index + 1
		}
	}
	return len(body) + 1
}

func skipSpace(body string, index int) int {
	for index < len(body) && (body[index] == ' ' || body[index] == '\t' || body[index] == '\r' || body[index] == '\n') {
		index++
	}
	return index
}

func isIdentifier(value byte) bool {
	return value == '_' || value == '$' || value >= '0' && value <= '9' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

// resolveImport resolves a local reference to the first candidate file the
// graph holds. A candidate that excluded covers (TJAA-V0-100) stops the
// search as unresolved: the runtime may load that excluded file, so a later
// candidate is no proof of the edge.
func resolveImport(owner, reference string, paths map[string]string, excluded func(string) bool) (string, bool, bool) {
	clean := strings.SplitN(strings.SplitN(reference, "?", 2)[0], "#", 2)[0]
	if !strings.HasPrefix(clean, ".") {
		return "", false, looksLikeAlias(clean)
	}
	base := path.Clean(path.Join(path.Dir(owner), clean))
	if base == "." || base == ".." || strings.HasPrefix(base, "../") {
		return "", true, true
	}
	for _, candidate := range moduleCandidates(base) {
		if id := paths[candidate]; id != "" {
			return id, true, false
		}
		if excluded != nil && excluded(candidate) {
			return "", true, true
		}
	}
	return "", true, true
}

func moduleCandidates(base string) []string {
	candidates := []string{base}
	extension := path.Ext(base)
	if hasSourceExtension(base) {
		stem := strings.TrimSuffix(base, extension)
		for _, alternate := range sourceExtensions {
			candidates = append(candidates, stem+alternate)
		}
	} else {
		for _, extension := range sourceExtensions {
			candidates = append(candidates, base+extension)
		}
		for _, extension := range sourceExtensions {
			candidates = append(candidates, path.Join(base, "index"+extension))
		}
	}
	return candidates
}

func unitID(runner, relative string) string {
	if runner == "" {
		return "typescript:" + relative
	}
	return "typescript:" + runner + ":" + relative
}

func storybookUnitID(relative string) string {
	directory := "."
	parts := strings.Split(relative, "/")
	for index, part := range parts {
		if part == "src" || part == "stories" {
			if index > 0 {
				directory = strings.Join(parts[:index], "/")
			}
			break
		}
	}
	return "typescript:" + runnerStorybook + ":" + directory
}

func observedName(name string) bool {
	_, frameworkConfig := configRunner(name, "")
	return hasSourceExtension(name) || unreadModuleSource(name) || frameworkConfig || prefixedConfig(name, "vite.config") ||
		name == "package.json" || name == "deno.json" || name == "deno.jsonc" ||
		name == "bunfig.toml" || typeScriptConfigName(name) || javaScriptConfigName(name) || name == "nightwatch.json" ||
		name == ".detoxrc" || name == ".detoxrc.json" || name == ".testcaferc.json" ||
		strings.HasSuffix(name, ".feature") || strings.HasSuffix(name, ".yaml") ||
		strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".mdx")
}

func configRunner(relative, body string) (string, bool) {
	base := path.Base(relative)
	switch {
	case base == ".mocharc.js" || base == ".mocharc.cjs" || base == ".mocharc.mjs" || base == ".mocharc.json" || base == ".mocharc.jsonc" || base == ".mocharc.yaml" || base == ".mocharc.yml":
		return runnerMocha, true
	case prefixedConfig(base, "vitest.config"):
		return runnerVitest, true
	case prefixedConfig(base, "vite.config") && (strings.Contains(body, "vitest/config") || strings.Contains(body, "test:")):
		return runnerVitest, true
	case prefixedConfig(base, "jest.config") || base == "jest.config.json":
		if strings.Contains(body, "detox/runners/jest") {
			return runnerDetox, true
		}
		return runnerJest, true
	case prefixedConfig(base, "playwright.config") || strings.HasPrefix(base, "playwright.") && strings.Contains(base, ".config."):
		return runnerPlaywright, true
	case prefixedConfig(base, "cypress.config"):
		return runnerCypress, true
	case strings.HasPrefix(base, "wdio") && strings.Contains(base, ".conf."):
		return runnerWebdriverIO, true
	case base == ".testcaferc.js" || base == ".testcaferc.cjs" || base == ".testcaferc.json":
		return runnerTestCafe, true
	case prefixedConfig(base, "nightwatch.conf") || prefixedConfig(base, "nightwatch.config") || base == "nightwatch.json":
		return runnerNightwatch, true
	case base == ".detoxrc" || strings.HasPrefix(base, ".detoxrc.") || prefixedConfig(base, "detox.config"):
		return runnerDetox, true
	case strings.HasPrefix(base, "test-runner.") && path.Base(path.Dir(relative)) == ".storybook":
		return runnerStorybook, true
	}
	return "", false
}

func runtimeManifestRunner(relative string) string {
	switch path.Base(relative) {
	case "deno.json", "deno.jsonc":
		return runnerDeno
	case "bunfig.toml":
		return runnerBun
	}
	return ""
}

func readPackageScope(relative string, body []byte) (packageScope, error) {
	var manifest struct {
		Name                 string                     `json:"name"`
		Dependencies         map[string]json.RawMessage `json:"dependencies"`
		DevDependencies      map[string]json.RawMessage `json:"devDependencies"`
		PeerDependencies     map[string]json.RawMessage `json:"peerDependencies"`
		OptionalDependencies map[string]json.RawMessage `json:"optionalDependencies"`
		Mocha                json.RawMessage            `json:"mocha"`
		Jest                 json.RawMessage            `json:"jest"`
		Detox                json.RawMessage            `json:"detox"`
	}
	if err := json.Unmarshal(body, &manifest); err != nil {
		return packageScope{}, err
	}
	dependencies := make(map[string]bool)
	local := make(map[string]bool)
	for _, group := range []map[string]json.RawMessage{manifest.Dependencies, manifest.DevDependencies, manifest.PeerDependencies, manifest.OptionalDependencies} {
		for dependency, raw := range group {
			var specifier string
			if json.Unmarshal(raw, &specifier) == nil && repositorySpecifier(path.Dir(relative), specifier) {
				local[dependency] = true
				continue
			}
			dependencies[dependency] = true
		}
	}
	runners := make(map[string]bool)
	configured := make(map[string]bool)
	dependencyRunners := map[string]string{
		"mocha": runnerMocha, "vitest": runnerVitest, "jest": runnerJest, "@jest/core": runnerJest, "ava": runnerAVA,
		"@playwright/test": runnerPlaywright, "cypress": runnerCypress,
		"@wdio/cli": runnerWebdriverIO, "webdriverio": runnerWebdriverIO,
		"testcafe": runnerTestCafe, "nightwatch": runnerNightwatch, "detox": runnerDetox,
		"@storybook/test-runner": runnerStorybook, "@storybook/addon-vitest": runnerStorybookVitest,
	}
	for dependency, runner := range dependencyRunners {
		if dependencies[dependency] {
			runners[runner] = true
		}
	}
	if len(manifest.Mocha) != 0 && string(manifest.Mocha) != "null" {
		configured[runnerMocha] = true
	}
	if len(manifest.Jest) != 0 && string(manifest.Jest) != "null" {
		configured[runnerJest] = true
	}
	if len(manifest.Detox) != 0 && string(manifest.Detox) != "null" {
		configured[runnerDetox] = true
	}
	return packageScope{
		directory:       path.Dir(relative),
		manifest:        relative,
		name:            manifest.Name,
		runners:         runners,
		configured:      configured,
		packages:        dependencies,
		localSpecifiers: local,
	}, nil
}

// repositorySpecifier reports a package.json dependency specifier that names repository source
// rather than an external package (TJAA-V0-710): any workspace: specifier, and a file:, link: or
// portal: path that, joined to the manifest's directory, stays inside the root and outside every
// node_modules directory.
func repositorySpecifier(directory, specifier string) bool {
	if strings.HasPrefix(specifier, "workspace:") {
		return true
	}
	for _, scheme := range []string{"file:", "link:", "portal:"} {
		target, ok := strings.CutPrefix(specifier, scheme)
		if !ok {
			continue
		}
		target = strings.TrimPrefix(target, "//")
		if target == "" || path.IsAbs(target) || strings.Contains(target, "\\") {
			return false
		}
		joined := path.Join(directory, target)
		if joined == ".." || strings.HasPrefix(joined, "../") {
			return false
		}
		return !slices.Contains(strings.Split(joined, "/"), "node_modules")
	}
	return false
}

// dynamicConfigMarkers are the membership and project markers dynamicConfigMarker searches for.
var dynamicConfigMarkers = []string{"testMatch", "testRegex", "testDir", "specPattern", "specs:", "src:", "projects:", "include:", "exclude:", "setupFiles", "globalSetup", "capabilities:", "browsers:"}

// dynamicConfigMarker returns the offset of the first membership or project marker the adapter does
// not model, or -1.
func dynamicConfigMarker(body string) int {
	offset := -1
	for _, marker := range dynamicConfigMarkers {
		if found := strings.Index(body, marker); found >= 0 && (offset < 0 || found < offset) {
			offset = found
		}
	}
	return offset
}

// typeScriptConfigName reports whether base is tsconfig.json or a sibling spelling, such as
// tsconfig.base.json, that an `extends` chain may name, so the chain resolves against observed
// and source-bound bytes (TJAA-V0-114).
func typeScriptConfigName(base string) bool {
	return strings.HasPrefix(base, "tsconfig") && strings.HasSuffix(base, ".json")
}

func ambiguousTestAsset(relative string) bool {
	base := path.Base(relative)
	if strings.HasSuffix(base, ".feature") || strings.Contains(base, ".stories.mdx") {
		return true
	}
	// A package-manager lockfile records resolved dependencies; no test runner reads it as a
	// test asset, whatever directory holds it (TJAA-V0-111).
	if packageManagerLockfile(base) || (!strings.HasSuffix(base, ".yaml") && !strings.HasSuffix(base, ".yml")) {
		return false
	}
	for _, component := range strings.Split(path.Dir(relative), "/") {
		if component == "maestro" || component == "e2e" || component == "features" {
			return true
		}
	}
	return false
}

// packageManagerLockfile reports whether base is the file name of a YAML package-manager lockfile.
func packageManagerLockfile(base string) bool {
	return base == "pnpm-lock.yaml" || base == "shrinkwrap.yaml"
}

func isTestCandidate(relative string) bool {
	components := strings.Split(relative, "/")
	if len(components) > 1 && (components[0] == "test" || components[0] == "tests") {
		return true
	}
	base := path.Base(relative)
	for _, marker := range []string{".test.", ".spec.", "_test.", "_spec.", ".cy.", ".stories."} {
		if strings.Contains(base, marker) {
			return true
		}
	}
	return false
}

// conventionRunners widens convention candidacy with the nearest manifest's dependency runners. A
// dependency alone still assigns no runner (TJAA-V0-003); it only keeps a conventional test
// directory from being silently classified as source.
func conventionRunners(relative string, possible map[string]bool, scopes []packageScope) map[string]bool {
	scope, known := nearestScope(relative, scopes)
	if !known {
		return possible
	}
	runners := make(map[string]bool, len(possible)+len(scope.runners))
	for runner := range possible {
		runners[runner] = true
	}
	for runner := range scope.runners {
		runners[runner] = true
	}
	return runners
}

func conventionCandidate(relative string, runners map[string]bool) bool {
	for _, component := range strings.Split(path.Dir(relative), "/") {
		if component == "__tests__" && runners[runnerJest] {
			return true
		}
		if component == "cypress" && runners[runnerCypress] || component == "playwright" && runners[runnerPlaywright] {
			return true
		}
		if component != "e2e" && component != "spec" && component != "specs" {
			continue
		}
		for _, runner := range []string{runnerPlaywright, runnerCypress, runnerWebdriverIO, runnerTestCafe, runnerNightwatch, runnerDetox} {
			if runners[runner] {
				return true
			}
		}
	}
	return false
}

func isConfigPath(relative, body string) bool {
	_, known := configRunner(relative, body)
	return known
}

func requiresRuntimeFlags(relative, runner string) bool {
	return (runner == runnerNode || runner == runnerMocha) && (strings.HasSuffix(relative, ".ts") || strings.HasSuffix(relative, ".tsx"))
}

func isE2ERunner(runner string) bool {
	switch runner {
	case runnerPlaywright, runnerCypress, runnerWebdriverIO, runnerTestCafe, runnerNightwatch, runnerDetox:
		return true
	}
	return false
}

// unreadModuleSource reports a `.mts` or `.cts` module. TJAA-V0-001 leaves it
// unowned, so its imports are never read; it may be a test of an owned module,
// and a missing edge must widen rather than exclude.
func unreadModuleSource(relative string) bool {
	extension := strings.ToLower(path.Ext(relative))
	return extension == ".mts" || extension == ".cts"
}

func hasSourceExtension(relative string) bool {
	extension := strings.ToLower(path.Ext(relative))
	for _, candidate := range sourceExtensions {
		if extension == candidate {
			return true
		}
	}
	return false
}

func importsPackage(refs []string, packageName string) bool {
	for _, ref := range refs {
		if ref == packageName || strings.HasPrefix(ref, packageName+"/") {
			return true
		}
	}
	return false
}

func looksLikeAlias(reference string) bool {
	return strings.HasPrefix(reference, "@/") || strings.HasPrefix(reference, "~/") || strings.HasPrefix(reference, "#")
}

// unresolvedBareImport returns the first bare import specifier that names neither a workspace
// package nor a dependency declared for relative.
func unresolvedBareImport(root *affected.Source, relative string, refs []string, scopes []packageScope) (string, bool) {
	for _, ref := range refs {
		if strings.HasPrefix(ref, ".") || externalScheme(ref) {
			continue
		}
		packageName := importedPackage(ref)
		if packageName == "" {
			continue
		}
		if localPackage(packageName, scopes) {
			continue
		}
		// A dependency declared by a repository-source specifier is no external package, whatever
		// node_modules holds for it (TJAA-V0-710).
		if localSpecifier(relative, packageName, scopes) {
			return ref, true
		}
		if declaredPackage(relative, packageName, scopes) || installedPackage(root, relative, packageName) {
			continue
		}
		return ref, true
	}
	return "", false
}

// bareImportReason names an unresolved bare specifier's frontier: a tsconfig or jsconfig alias
// when the nearest alias config's paths claim it, so the alias was declared but not applied,
// else an undeclared bare import (TJAA-V0-771).
func bareImportReason(owner, ref string, configs []typeScriptAliases, bodies map[string]string) string {
	if nearest := nearestTypeScriptAliases(owner, configs); nearest != nil {
		if _, claimed := nearest.resolve(ref, bodies); claimed {
			return FrontierPathAlias
		}
	}
	return FrontierBareImportUndeclared
}

// installedPackage reports whether packageName is installed as a dependency where Node module
// resolution looks for it: a regular node_modules/<packageName>/package.json in the directory of
// relative or of one of its ancestors inside the root (TJAA-V0-122). node_modules may be a real
// directory or a symlink, such as one into a content-addressed store or a CI cache outside the
// root. Only that one manifest's existence and where the package directory resolves are
// observed; nothing under it is read or joins the graph. The package is an external dependency
// exactly as a package.json-declared one is only when its directory resolves outside the root or
// inside a node_modules directory (Source.ResolvesToDependency); a link into repository source
// outside node_modules, such as node_modules/alias -> ../src/lib, would hide a graph edge, so the
// specifier stays unresolved, as does a package installed nowhere or any immutable tree source.
func installedPackage(root *affected.Source, relative, packageName string) bool {
	for _, segment := range strings.Split(packageName, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	for directory := path.Dir(relative); ; directory = path.Dir(directory) {
		if info, err := root.Stat(path.Join(directory, "node_modules", packageName, "package.json")); err == nil && info.Mode().IsRegular() {
			// Node loads the nearest one, so a link into repository source ends the search.
			return root.ResolvesToDependency(path.Join(directory, "node_modules", packageName))
		}
		if directory == "." || directory == "/" {
			return false
		}
	}
}

func importedPackage(reference string) string {
	parts := strings.Split(reference, "/")
	if len(parts) == 0 {
		return ""
	}
	if strings.HasPrefix(reference, "@") && len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

func declaredPackage(relative, packageName string, scopes []packageScope) bool {
	for _, scope := range scopes {
		if inside(relative, scope.directory) && scope.packages[packageName] {
			return true
		}
	}
	return false
}

// localSpecifier reports whether a scope enclosing relative declares packageName by a
// repository-source specifier (TJAA-V0-710).
func localSpecifier(relative, packageName string, scopes []packageScope) bool {
	for _, scope := range scopes {
		if inside(relative, scope.directory) && scope.localSpecifiers[packageName] {
			return true
		}
	}
	return false
}

func localPackage(packageName string, scopes []packageScope) bool {
	for _, scope := range scopes {
		if scope.name == packageName {
			return true
		}
	}
	return false
}

func externalScheme(reference string) bool {
	for _, prefix := range []string{"node:", "bun:", "deno:", "jsr:", "npm:", "http:", "https:", "data:"} {
		if strings.HasPrefix(reference, prefix) {
			return true
		}
	}
	return false
}

func prefixedConfig(base, prefix string) bool {
	return strings.HasPrefix(base, prefix+".")
}

func nearestScope(relative string, scopes []packageScope) (packageScope, bool) {
	best := -1
	var selected packageScope
	for _, scope := range scopes {
		if !inside(relative, scope.directory) {
			continue
		}
		depth := len(strings.Split(scope.directory, "/"))
		if depth > best {
			best = depth
			selected = scope
		}
	}
	return selected, best >= 0
}

func inside(relative, directory string) bool {
	return directory == "." || relative != directory && strings.HasPrefix(relative, directory+"/")
}

func keepRunner(runners map[string]bool, runner string) map[string]bool {
	if runners[runner] {
		return map[string]bool{runner: true}
	}
	return map[string]bool{}
}

func firstPath(unit affected.Unit) string {
	if len(unit.Sources) != 0 {
		return unit.Sources[0]
	}
	return unit.Tests[0]
}

func stringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

func sortedKeys[Value any](values map[string]Value) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
