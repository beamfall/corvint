package typescript

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// PlaywrightListMaxBytes bounds the Playwright JSON listing the discovery producer reads.
const PlaywrightListMaxBytes = 64 << 20

// playwrightListMaxDepth bounds suite nesting in a Playwright JSON listing.
const playwrightListMaxDepth = 64

// playwrightListing is the subset of a `playwright test --list --reporter=json` report the
// producer reads. Other members are ignored.
type playwrightListing struct {
	Config *struct {
		ConfigFile string         `json:"configFile"`
		RootDir    string         `json:"rootDir"`
		Argv       []string       `json:"argv"`
		Shard      jsontext.Value `json:"shard"`
	} `json:"config"`
	Suites []playwrightListSuite `json:"suites"`
	Errors []jsontext.Value      `json:"errors"`
}

type playwrightListSuite struct {
	File   string                `json:"file"`
	Title  string                `json:"title"`
	Specs  []playwrightListSpec  `json:"specs"`
	Suites []playwrightListSuite `json:"suites"`
}

type playwrightListSpec struct {
	ID     string   `json:"id"`
	Title  string   `json:"title"`
	Tags   []string `json:"tags"`
	File   string   `json:"file"`
	Line   int      `json:"line"`
	Column int      `json:"column"`
	Tests  []struct {
		ProjectName *string `json:"projectName"`
	} `json:"tests"`
}

// PlaywrightDiscoveryFromList converts the JSON report of an unfiltered
// `playwright test --list --reporter=json` run into the canonical playwright-discovery/0
// receipt for configPath, bound to revision, the current config bytes and the current source
// digest (TJAA-V0-019). Playwright reports each spec file relative to its rootDir; the producer
// rewrites it to the repository-relative path the static profile uses. It refuses a listing
// that reports errors, was filtered or sharded, names another config, or names a test outside
// root. A (project, file) pair the config selects but the listing omits is accepted only when the
// project declares grep or grepInvert: it becomes a unit when a static grepInvert matches a title
// part of every listed case of that file, or a unit also recorded in unverifiedUnits when the
// exclusion cannot be proven from the listing (TJAA-V0-021). The listing stays caller-declared evidence: the producer
// cannot prove it was taken from the bytes it binds.
func PlaywrightDiscoveryFromList(root, configPath, revision string, listing []byte) ([]byte, error) {
	return PlaywrightDiscoveryFromLists(root, configPath, revision, listing, nil)
}

// PlaywrightDiscoveryFromLists is PlaywrightDiscoveryFromList with an optional ungrepped listing
// (TJAA-V0-118): the JSON report of the same unfiltered `playwright test --list --reporter=json`
// run for configPath taken with every project grep and grepInvert removed. A nil ungrepped is
// PlaywrightDiscoveryFromList. A non-nil one is refused under the same listing refusals and when
// it names a project/file pair the config does not select; otherwise it verifies an omitted pair
// only as verifiedProjects and verifies decide (TJAA-V0-119), and every other omission keeps the
// TJAA-V0-021 rules (TJAA-V0-120). Both listings stay caller-declared evidence.
func PlaywrightDiscoveryFromLists(root, configPath, revision string, listing, ungrepped []byte) ([]byte, error) {
	return PlaywrightDiscoveryFromWrappedLists(root, configPath, revision, listing, ungrepped, "")
}

// PlaywrightDiscoveryFromWrappedLists is PlaywrightDiscoveryFromLists whose ungrepped listing,
// when wrapper is not empty, was taken through that wrapper config rather than through configPath
// itself, so the tracked config is never rewritten to produce it (TJAA-V0-124). The wrapper must
// pass CheckPlaywrightUngreppedWrapper and the ungrepped listing must name it as its config; the
// grepped listing must still name configPath, and the receipt still binds configPath's bytes.
// A wrapper without an ungrepped listing is refused (TJAA-V0-125).
func PlaywrightDiscoveryFromWrappedLists(root, configPath, revision string, listing, ungrepped []byte, wrapper string) ([]byte, error) {
	ungreppedConfig := configPath
	if wrapper != "" {
		if ungrepped == nil {
			return nil, errors.New("an ungrepped wrapper config needs an ungrepped listing")
		}
		if !affected.ValidRelativePath(configPath) || !hasSourceExtension(configPath) {
			return nil, errors.New("playwright config path is not canonical TypeScript/JavaScript source")
		}
		if err := CheckPlaywrightUngreppedWrapper(root, configPath, wrapper); err != nil {
			return nil, err
		}
		ungreppedConfig = wrapper
	}
	if !playwrightHex(revision, 40) && !playwrightHex(revision, 64) {
		return nil, errors.New("revision is not a full lowercase object id")
	}
	report, realRoot, err := validatePlaywrightListing(root, configPath, listing)
	if err != nil {
		return nil, err
	}
	grepped := newPlaywrightCaseListing()
	cases := map[string][]playwrightListCase{}
	if err := walkPlaywrightListing(realRoot, report.Config.RootDir, report.Suites, cases, grepped.add); err != nil {
		return nil, err
	}
	var check *playwrightUngreppedCheck
	if ungrepped != nil {
		other, _, err := validatePlaywrightListing(root, ungreppedConfig, ungrepped)
		if err != nil {
			return nil, fmt.Errorf("ungrepped listing: %w", err)
		}
		check = &playwrightUngreppedCheck{grepped: grepped, ungrepped: newPlaywrightCaseListing()}
		if err := walkPlaywrightListing(realRoot, other.Config.RootDir, other.Suites, map[string][]playwrightListCase{}, check.ungrepped.add); err != nil {
			return nil, fmt.Errorf("ungrepped listing: %w", err)
		}
	}
	seen := grepped.pairs
	configBytes, sourceDigest, omissions, err := observePlaywrightListMembership(root, configPath, seen, nil, cases, check)
	if err != nil {
		return nil, err
	}
	units := make([]PlaywrightDiscoveryUnit, 0, len(seen)+len(omissions.explained)+len(omissions.unverified))
	for unit := range seen {
		units = append(units, unit)
	}
	units = append(append(units, omissions.explained...), omissions.unverified...)
	sort.Slice(units, func(i, j int) bool { return discoveryUnitLess(units[i], units[j]) })
	sum := sha256.Sum256(configBytes)
	receipt := PlaywrightDiscovery{
		Config: PlaywrightConfigIdentity{Path: configPath, SHA256: hex.EncodeToString(sum[:])}, Profile: "playwright-discovery/0",
		Revision: revision, SourceDigest: sourceDigest, Units: units,
	}
	if len(omissions.unverified) != 0 {
		receipt.UnverifiedUnits = omissions.unverified
	}
	raw, err := json.Marshal(receipt, json.Deterministic(true))
	if err != nil {
		return nil, err
	}
	if _, malformed := decodePlaywrightDiscovery(raw); malformed != nil {
		return nil, fmt.Errorf("produced receipt is not valid: %s: %s", malformed.reason, malformed.detail)
	}
	return raw, nil
}

// validatePlaywrightListing decodes a `playwright test --list --reporter=json` report for
// configPath and refuses one that reports errors, was filtered or sharded, names another config
// or has a rootDir outside root. It returns the report and the resolved repository root.
func validatePlaywrightListing(root, configPath string, listing []byte) (playwrightListing, string, error) {
	var report playwrightListing
	if !affected.ValidRelativePath(configPath) || !hasSourceExtension(configPath) {
		return report, "", errors.New("playwright config path is not canonical TypeScript/JavaScript source")
	}
	if len(listing) > PlaywrightListMaxBytes {
		return report, "", fmt.Errorf("playwright listing is over the %d-byte bound", PlaywrightListMaxBytes)
	}
	if err := json.Unmarshal(listing, &report); err != nil {
		return report, "", fmt.Errorf("playwright listing is not a JSON report: %v", err)
	}
	if report.Config == nil || report.Suites == nil || report.Errors == nil {
		return report, "", errors.New("playwright listing lacks config, suites or errors; produce it with `playwright test --list --reporter=json`")
	}
	if len(report.Errors) != 0 {
		return report, "", fmt.Errorf("playwright listing reports %d error(s); the universe is incomplete", len(report.Errors))
	}
	if err := unfilteredPlaywrightArgv(report.Config.Argv); err != nil {
		return report, "", err
	}
	if shard := strings.TrimSpace(string(report.Config.Shard)); shard != "" && shard != "null" {
		return report, "", errors.New("playwright listing is sharded; list the whole suite")
	}
	realRoot, err := filepath.Abs(root)
	if err == nil {
		realRoot, err = filepath.EvalSymlinks(realRoot)
	}
	if err != nil {
		return report, "", fmt.Errorf("repository root is unreadable: %v", err)
	}
	if playwrightPathOutsideMatcherModel(root) || playwrightPathOutsideMatcherModel(realRoot) {
		return report, "", errors.New("repository root path contains a line terminator or a character outside the Basic Multilingual Plane, which Go and JavaScript regular expressions treat differently; the listing's membership cannot be checked")
	}
	if listed, ok := playwrightListPath(realRoot, report.Config.ConfigFile); !ok || listed != configPath {
		return report, "", fmt.Errorf("playwright listing config %q is not %s under the repository root", report.Config.ConfigFile, configPath)
	}
	if _, ok := playwrightListPath(realRoot, report.Config.RootDir); !ok && !samePlaywrightDir(realRoot, report.Config.RootDir) {
		return report, "", fmt.Errorf("playwright listing rootDir %q is outside the repository root", report.Config.RootDir)
	}
	return report, realRoot, nil
}

// observePlaywrightListMembership runs the listing membership checks every Playwright listing
// producer shares: it observes the Playwright sources, reads the config, checks the listed
// project/file pairs against the pairs the config selects in the current sources (and, when
// revisionPaths is not nil, among those repository-relative paths of a pinned revision), and
// refuses when the sources changed during the check. It returns the config bytes and the
// source digest it observed. When cases is not nil, it holds each listed file's cases and an
// omitted pair is classified against the project's grep/grepInvert instead of refused
// outright (TJAA-V0-021); when cases is nil every omission is refused. A non-nil ungrepped check
// is applied as checkPlaywrightListMembership describes (TJAA-V0-118..120).
func observePlaywrightListMembership(root, configPath string, listed map[PlaywrightDiscoveryUnit]bool, revisionPaths []string, cases map[string][]playwrightListCase, ungrepped *playwrightUngreppedCheck) ([]byte, string, playwrightGrepOmissions, error) {
	none := playwrightGrepOmissions{}
	sourceDigest, err := ObservePlaywrightSources(root, configPath)
	if err != nil {
		return nil, "", none, err
	}
	configBytes, err := affected.ReadSource(root, configPath)
	if err != nil || !utf8.Valid(configBytes) {
		return nil, "", none, errors.New("playwright config is unreadable")
	}
	omissions, err := checkPlaywrightListMembership(root, configPath, configBytes, listed, cases, ungrepped)
	if err != nil {
		return nil, "", none, err
	}
	if revisionPaths != nil {
		if err := checkPlaywrightRevisionMembership(root, configPath, configBytes, listed, revisionPaths, cases != nil, omissions); err != nil {
			return nil, "", none, err
		}
	}
	if after, err := ObservePlaywrightSources(root, configPath); err != nil || after != sourceDigest {
		return nil, "", none, errors.New("playwright sources changed while the listing was checked")
	}
	return configBytes, sourceDigest, omissions, nil
}

// staticPlaywrightProjects returns the projects whose test membership the static profile decides.
// Only an unresolved browser identity or an unverified non-identity use value (TJAA-V0-054), which
// do not decide file membership, are tolerated, and a
// config without projects is Playwright's one unnamed default project. When dynamicGrep is true a
// dynamic project grep or grepInvert, which is case identity rather than file membership, is
// tolerated as well (TJAA-V0-021).
func staticPlaywrightProjects(configPath string, configBytes []byte, dynamicGrep bool) ([]PlaywrightProject, string, error) {
	projects, globalTestDir, unknown := parsePlaywrightConfig(configPath, string(configBytes))
	grepDetails := map[string]bool{}
	if dynamicGrep {
		for _, project := range projects {
			grepDetails[project.Name+" grep is dynamic"] = project.hasGrep && project.Grep == ""
			grepDetails[project.Name+" grepInvert is dynamic"] = project.hasGrepInvert && project.GrepInvert == ""
		}
	}
	implicit := false
	for _, entry := range unknown {
		switch {
		case entry.Reason == PlaywrightUnknownBrowserIdentity, entry.Reason == PlaywrightUnknownUseValueUnverified:
		case entry.Reason == PlaywrightUnknownConfigSyntax && grepDetails[entry.Detail]:
		case entry.Reason == PlaywrightUnknownProjectSet && entry.Detail == "projects is absent" && len(projects) == 0:
			implicit = true
		default:
			return nil, "", fmt.Errorf("the config's test membership is not static (%s: %s); the listing cannot be checked against the bound sources", entry.Reason, entry.Detail)
		}
	}
	if implicit {
		projects = []PlaywrightProject{{TestDir: globalTestDir}}
	}
	return projects, globalTestDir, nil
}

// checkPlaywrightRevisionMembership refuses a listing that omits a project/file pair the config
// selects among the regular files of a pinned revision, so a test file committed at that revision
// but absent from the working tree the listing observed cannot shrink the denominator. Files below
// a node_modules directory are not candidates, because Playwright never loads them. A pair the
// working-tree check accepted as a grep/grepInvert omission (TJAA-V0-021) is not refused again;
// dynamicGrep tolerates a dynamic grep as that check does.
func checkPlaywrightRevisionMembership(root, configPath string, configBytes []byte, listed map[PlaywrightDiscoveryUnit]bool, revisionPaths []string, dynamicGrep bool, accepted playwrightGrepOmissions) error {
	projects, globalTestDir, err := staticPlaywrightProjects(configPath, configBytes, dynamicGrep)
	if err != nil {
		return err
	}
	candidates := []string{}
	for _, candidate := range revisionPaths {
		if !playwrightLoadableName(path.Base(candidate)) || slices.Contains(strings.Split(candidate, "/"), "node_modules") {
			continue
		}
		if playwrightPathOutsideMatcherModel(candidate) {
			return fmt.Errorf("candidate test path %q contains a line terminator or a character outside the Basic Multilingual Plane, which Go and JavaScript regular expressions treat differently; the listing's membership cannot be checked", candidate)
		}
		candidates = append(candidates, candidate)
	}
	sort.Strings(candidates)
	for _, unit := range playwrightUnits(root, configPath, projects, globalTestDir, candidates) {
		pair := PlaywrightDiscoveryUnit{Project: unit.Project, Test: unit.Test}
		if !listed[pair] && !slices.Contains(accepted.explained, pair) && !slices.Contains(accepted.unverified, pair) {
			return fmt.Errorf("playwright listing omits project %q test %s, which the config selects at the source revision; the listing or working tree is stale", unit.Project, unit.Test)
		}
	}
	return nil
}

// checkPlaywrightListMembership compares the listed project/file pairs with the pairs the config
// selects among the current sources through the static profile's testDir/testMatch/testIgnore
// subset, so a stale or partial listing is refused instead of stamped with the current bindings.
// Membership that is not static is refused as well; only an unresolved browser identity, which
// does not decide file membership, is tolerated, and a config without projects is Playwright's
// one unnamed default project. A selected file the static profile cannot parse or read as UTF-8 is
// refused too. When cases is not nil an omitted pair whose project declares grep or grepInvert is
// classified by classifyPlaywrightGrepOmission (TJAA-V0-021) instead of refused. When ungrepped is
// not nil it must name only selected pairs, and an omitted pair it verifies is explained before
// that classification (TJAA-V0-118..120).
func checkPlaywrightListMembership(root, configPath string, configBytes []byte, listed map[PlaywrightDiscoveryUnit]bool, cases map[string][]playwrightListCase, ungrepped *playwrightUngreppedCheck) (playwrightGrepOmissions, error) {
	omissions := playwrightGrepOmissions{}
	projects, globalTestDir, err := staticPlaywrightProjects(configPath, configBytes, cases != nil)
	if err != nil {
		return omissions, err
	}
	if err := checkPlaywrightSkippedDirectories(root, configPath, projects, globalTestDir); err != nil {
		return omissions, err
	}
	// Candidates are found by path alone, so a test file the static profile cannot parse or read
	// (.mts, .cts, non-UTF-8) still counts toward membership instead of disappearing from it.
	candidates, err := affected.SourceFiles(root, playwrightLoadableName)
	if err != nil {
		return omissions, err
	}
	if index := slices.IndexFunc(candidates, playwrightPathOutsideMatcherModel); index >= 0 {
		return omissions, fmt.Errorf("candidate test path %q contains a line terminator or a character outside the Basic Multilingual Plane, which Go and JavaScript regular expressions treat differently; the listing's membership cannot be checked", candidates[index])
	}
	selected := map[PlaywrightDiscoveryUnit]bool{}
	readable := map[string]bool{}
	for _, unit := range playwrightUnits(root, configPath, projects, globalTestDir, candidates) {
		if !hasSourceExtension(unit.Test) {
			return omissions, fmt.Errorf("the config selects project %q test %s, which the static profile does not parse; the listing's membership cannot be checked", unit.Project, unit.Test)
		}
		if _, checked := readable[unit.Test]; !checked {
			body, err := affected.ReadSource(root, unit.Test)
			readable[unit.Test] = err == nil && utf8.Valid(body)
		}
		if !readable[unit.Test] {
			return omissions, fmt.Errorf("the config selects project %q test %s, which the static profile cannot read as bounded UTF-8 source; the listing's membership cannot be checked", unit.Project, unit.Test)
		}
		selected[PlaywrightDiscoveryUnit{Project: unit.Project, Test: unit.Test}] = true
	}
	var extra, missing []PlaywrightDiscoveryUnit
	for unit := range listed {
		if !selected[unit] {
			extra = append(extra, unit)
		}
	}
	byName := map[string]PlaywrightProject{}
	for _, project := range projects {
		byName[project.Name] = project
	}
	var verified map[string]bool
	if ungrepped != nil {
		var foreign []PlaywrightDiscoveryUnit
		for unit := range ungrepped.ungrepped.pairs {
			if !selected[unit] {
				foreign = append(foreign, unit)
			}
		}
		if len(foreign) != 0 {
			sort.Slice(foreign, func(i, j int) bool { return discoveryUnitLess(foreign[i], foreign[j]) })
			return omissions, fmt.Errorf("ungrepped playwright listing names project %q test %s (%d pair(s) in all), which the config does not select in the current sources", foreign[0].Project, foreign[0].Test, len(foreign))
		}
		verified = ungrepped.verifiedProjects(byName)
	}
	tagSources := newPlaywrightTagSources(root, configPath)
	for unit := range selected {
		if listed[unit] {
			continue
		}
		if project := byName[unit.Project]; cases != nil && (project.hasGrep || project.hasGrepInvert) {
			if ungrepped != nil && ungrepped.verifies(verified, unit) {
				omissions.explained = append(omissions.explained, unit)
				continue
			}
			switch classifyPlaywrightGrepOmission(project, cases[unit.Test], tagSources.exact(configPath, unit.Test)) {
			case playwrightOmissionExplained:
				omissions.explained = append(omissions.explained, unit)
				continue
			case playwrightOmissionUnverified:
				omissions.unverified = append(omissions.unverified, unit)
				continue
			}
		}
		missing = append(missing, unit)
	}
	less := func(values []PlaywrightDiscoveryUnit) func(i, j int) bool {
		return func(i, j int) bool { return discoveryUnitLess(values[i], values[j]) }
	}
	sort.Slice(extra, less(extra))
	sort.Slice(missing, less(missing))
	sort.Slice(omissions.explained, less(omissions.explained))
	sort.Slice(omissions.unverified, less(omissions.unverified))
	if len(missing) != 0 {
		grep := ""
		if project := byName[missing[0].Project]; cases != nil && (project.hasGrep || project.hasGrepInvert) {
			grep = ", and the project's grep/grepInvert does not exclude every listed case of that file"
		}
		return omissions, fmt.Errorf("playwright listing omits project %q test %s (%d pair(s) in all), which the config selects in the current sources%s; the listing is stale or filtered", missing[0].Project, missing[0].Test, len(missing), grep)
	}
	if len(extra) != 0 {
		return omissions, fmt.Errorf("playwright listing names project %q test %s (%d pair(s) in all), which the config does not select in the current sources", extra[0].Project, extra[0].Test, len(extra))
	}
	return omissions, nil
}

// checkPlaywrightSkippedDirectories refuses when the config selects a test file inside a directory
// the shared source walker skips (a hidden directory or one of affected.SkippedDirectories, such as
// build, dist, vendor or target): Playwright still runs it, but it is outside the bound source
// digest and the path-based enumeration. node_modules below a testDir is not searched, because
// Playwright never descends it, and symbolic links are not followed, because Playwright skips them;
// a testDir that is itself reached through a symbolic link is refused.
func checkPlaywrightSkippedDirectories(root, configPath string, projects []PlaywrightProject, globalTestDir string) error {
	directories := map[string]bool{}
	for _, project := range projects {
		directory := project.TestDir
		if directory == "" {
			directory = globalTestDir
		}
		directories[directory] = true
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	inSkipped := map[string]string{}
	entries := 0
	for directory := range directories {
		start := filepath.Join(root, filepath.FromSlash(directory))
		resolved, err := filepath.EvalSymlinks(start)
		if errors.Is(err, fs.ErrNotExist) {
			continue // Playwright finds no tests in a missing testDir
		}
		if err != nil {
			return fmt.Errorf("cannot resolve testDir %s: %w", directory, err)
		}
		if resolved != filepath.Join(resolvedRoot, filepath.FromSlash(directory)) {
			return fmt.Errorf("testDir %s is reached through a symbolic link, which the bound source observation does not follow; the listing's membership cannot be checked", directory)
		}
		err = filepath.WalkDir(start, func(current string, entry fs.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("%w: %s", affected.ErrWalkUnreadable, current)
			}
			if entries++; entries > affected.MaxWalkEntries {
				return affected.ErrWalkLimit
			}
			if entry.IsDir() {
				if current != start && entry.Name() == "node_modules" {
					return fs.SkipDir
				}
				return nil
			}
			if !entry.Type().IsRegular() || !playwrightLoadableName(entry.Name()) {
				return nil
			}
			relative, err := filepath.Rel(root, current)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			if playwrightPathOutsideMatcherModel(relative) {
				return fmt.Errorf("candidate test path %q contains a line terminator or a character outside the Basic Multilingual Plane, which Go and JavaScript regular expressions treat differently; the listing's membership cannot be checked", relative)
			}
			if skipped := playwrightSkippedAncestor(relative); skipped != "" {
				inSkipped[relative] = skipped
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("cannot enumerate testDir %s: %w", directory, err)
		}
	}
	tests := make([]string, 0, len(inSkipped))
	for test := range inSkipped {
		tests = append(tests, test)
	}
	sort.Strings(tests)
	if units := playwrightUnits(root, configPath, projects, globalTestDir, tests); len(units) != 0 {
		return fmt.Errorf("the config selects project %q test %s inside directory %s, which the bound source observation skips; the listing's membership cannot be checked", units[0].Project, units[0].Test, inSkipped[units[0].Test])
	}
	return nil
}

// playwrightSkippedAncestor returns the first ancestor directory of a repository-relative file that
// the shared source walker does not descend, or "".
func playwrightSkippedAncestor(relative string) string {
	components := strings.Split(relative, "/")
	for index, name := range components[:len(components)-1] {
		if strings.HasPrefix(name, ".") || affected.SkippedDirectories[name] {
			return strings.Join(components[:index+1], "/")
		}
	}
	return ""
}

// unfilteredPlaywrightArgv accepts only `... test` followed by the listing, JSON reporter and
// config options, so a recorded file, project, grep, shard or changed-only filter is refused.
// `--list` is required: an execution report applies test.only, which a listing disables.
func unfilteredPlaywrightArgv(argv []string) error {
	start := -1
	for index, value := range argv {
		if value == "test" {
			start = index
			break
		}
	}
	if start < 0 {
		return errors.New("playwright listing does not record a `playwright test` argv; cannot prove it is unfiltered")
	}
	listed := false
	for index := start + 1; index < len(argv); index++ {
		switch value := argv[index]; {
		case value == "--list":
			listed = true
		case value == "--reporter=json", strings.HasPrefix(value, "--config="):
		case (value == "--reporter" || value == "--config" || value == "-c") && index+1 < len(argv):
			if value == "--reporter" && argv[index+1] != "json" {
				return fmt.Errorf("playwright listing argv uses reporter %q; use --reporter=json", argv[index+1])
			}
			index++
		default:
			return fmt.Errorf("playwright listing argv has %q; only --list, --reporter=json and --config are allowed so the universe is unfiltered", value)
		}
	}
	if !listed {
		return errors.New("playwright listing argv lacks --list; an execution report is not a discovery listing")
	}
	return nil
}

// playwrightListPath returns the canonical repository-relative path of an absolute listing path
// strictly inside realRoot, resolving symlinks in its existing prefix.
func playwrightListPath(realRoot, absolute string) (string, bool) {
	if !filepath.IsAbs(absolute) {
		return "", false
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", false
	}
	relative, err := filepath.Rel(realRoot, resolved)
	if err != nil {
		return "", false
	}
	relative = filepath.ToSlash(relative)
	return relative, affected.ValidRelativePath(relative)
}

func samePlaywrightDir(realRoot, absolute string) bool {
	if !filepath.IsAbs(absolute) {
		return false
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	return err == nil && resolved == realRoot
}
