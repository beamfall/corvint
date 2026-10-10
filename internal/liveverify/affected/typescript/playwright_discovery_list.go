package typescript

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
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
	File  string `json:"file"`
	Specs []struct {
		File  string `json:"file"`
		Tests []struct {
			ProjectName *string `json:"projectName"`
		} `json:"tests"`
	} `json:"specs"`
	Suites []playwrightListSuite `json:"suites"`
}

// PlaywrightDiscoveryFromList converts the JSON report of an unfiltered
// `playwright test --list --reporter=json` run into the canonical playwright-discovery/0
// receipt for configPath, bound to revision, the current config bytes and the current source
// digest (TJAA-V0-019). Playwright reports each spec file relative to its rootDir; the producer
// rewrites it to the repository-relative path the static profile uses. It refuses a listing
// that reports errors, was filtered or sharded, names another config, or names a test outside
// root. The listing stays caller-declared evidence: the producer cannot prove it was taken from
// the bytes it binds.
func PlaywrightDiscoveryFromList(root, configPath, revision string, listing []byte) ([]byte, error) {
	if !affected.ValidRelativePath(configPath) || !hasSourceExtension(configPath) {
		return nil, errors.New("playwright config path is not canonical TypeScript/JavaScript source")
	}
	if !playwrightHex(revision, 40) && !playwrightHex(revision, 64) {
		return nil, errors.New("revision is not a full lowercase object id")
	}
	if len(listing) > PlaywrightListMaxBytes {
		return nil, fmt.Errorf("playwright listing is over the %d-byte bound", PlaywrightListMaxBytes)
	}
	var report playwrightListing
	if err := json.Unmarshal(listing, &report); err != nil {
		return nil, fmt.Errorf("playwright listing is not a JSON report: %v", err)
	}
	if report.Config == nil || report.Suites == nil || report.Errors == nil {
		return nil, errors.New("playwright listing lacks config, suites or errors; produce it with `playwright test --list --reporter=json`")
	}
	if len(report.Errors) != 0 {
		return nil, fmt.Errorf("playwright listing reports %d error(s); the universe is incomplete", len(report.Errors))
	}
	if err := unfilteredPlaywrightArgv(report.Config.Argv); err != nil {
		return nil, err
	}
	if shard := strings.TrimSpace(string(report.Config.Shard)); shard != "" && shard != "null" {
		return nil, errors.New("playwright listing is sharded; list the whole suite")
	}
	realRoot, err := filepath.Abs(root)
	if err == nil {
		realRoot, err = filepath.EvalSymlinks(realRoot)
	}
	if err != nil {
		return nil, fmt.Errorf("repository root is unreadable: %v", err)
	}
	if playwrightPathHasLineTerminator(root) || playwrightPathHasLineTerminator(realRoot) {
		return nil, errors.New("repository root path contains a line terminator, which Go and JavaScript regular expressions treat differently; the listing's membership cannot be checked")
	}
	if listed, ok := playwrightListPath(realRoot, report.Config.ConfigFile); !ok || listed != configPath {
		return nil, fmt.Errorf("playwright listing config %q is not %s under the repository root", report.Config.ConfigFile, configPath)
	}
	if _, ok := playwrightListPath(realRoot, report.Config.RootDir); !ok && !samePlaywrightDir(realRoot, report.Config.RootDir) {
		return nil, fmt.Errorf("playwright listing rootDir %q is outside the repository root", report.Config.RootDir)
	}
	seen := map[PlaywrightDiscoveryUnit]bool{}
	if err := collectPlaywrightListUnits(realRoot, report.Config.RootDir, "", report.Suites, 0, seen); err != nil {
		return nil, err
	}
	units := make([]PlaywrightDiscoveryUnit, 0, len(seen))
	for unit := range seen {
		units = append(units, unit)
	}
	sort.Slice(units, func(i, j int) bool { return discoveryUnitLess(units[i], units[j]) })
	sourceDigest, err := ObservePlaywrightSources(root, configPath)
	if err != nil {
		return nil, err
	}
	configBytes, err := affected.ReadSource(root, configPath)
	if err != nil || !utf8.Valid(configBytes) {
		return nil, errors.New("playwright config is unreadable")
	}
	if err := checkPlaywrightListMembership(root, configPath, configBytes, seen); err != nil {
		return nil, err
	}
	if after, err := ObservePlaywrightSources(root, configPath); err != nil || after != sourceDigest {
		return nil, errors.New("playwright sources changed while the listing was checked")
	}
	sum := sha256.Sum256(configBytes)
	receipt := PlaywrightDiscovery{
		Config: PlaywrightConfigIdentity{Path: configPath, SHA256: hex.EncodeToString(sum[:])}, Profile: "playwright-discovery/0",
		Revision: revision, SourceDigest: sourceDigest, Units: units,
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

// checkPlaywrightListMembership compares the listed project/file pairs with the pairs the config
// selects among the current sources through the static profile's testDir/testMatch/testIgnore
// subset, so a stale or partial listing is refused instead of stamped with the current bindings.
// Membership that is not static is refused as well; only an unresolved browser identity, which
// does not decide file membership, is tolerated, and a config without projects is Playwright's
// one unnamed default project. A selected file the static profile cannot parse or read as UTF-8 is
// refused too.
func checkPlaywrightListMembership(root, configPath string, configBytes []byte, listed map[PlaywrightDiscoveryUnit]bool) error {
	projects, globalTestDir, unknown := parsePlaywrightConfig(configPath, string(configBytes))
	implicit := false
	for _, entry := range unknown {
		switch {
		case entry.Reason == PlaywrightUnknownBrowserIdentity:
		case entry.Reason == PlaywrightUnknownProjectSet && entry.Detail == "projects is absent" && len(projects) == 0:
			implicit = true
		default:
			return fmt.Errorf("the config's test membership is not static (%s: %s); the listing cannot be checked against the bound sources", entry.Reason, entry.Detail)
		}
	}
	if implicit {
		projects = []PlaywrightProject{{TestDir: globalTestDir}}
	}
	if err := checkPlaywrightSkippedDirectories(root, configPath, projects, globalTestDir); err != nil {
		return err
	}
	// Candidates are found by path alone, so a test file the static profile cannot parse or read
	// (.mts, .cts, non-UTF-8) still counts toward membership instead of disappearing from it.
	candidates, err := affected.SourceFiles(root, playwrightLoadableName)
	if err != nil {
		return err
	}
	if index := slices.IndexFunc(candidates, playwrightPathHasLineTerminator); index >= 0 {
		return fmt.Errorf("candidate test path %q contains a line terminator, which Go and JavaScript regular expressions treat differently; the listing's membership cannot be checked", candidates[index])
	}
	selected := map[PlaywrightDiscoveryUnit]bool{}
	readable := map[string]bool{}
	for _, unit := range playwrightUnits(root, configPath, projects, globalTestDir, candidates) {
		if !hasSourceExtension(unit.Test) {
			return fmt.Errorf("the config selects project %q test %s, which the static profile does not parse; the listing's membership cannot be checked", unit.Project, unit.Test)
		}
		if _, checked := readable[unit.Test]; !checked {
			body, err := affected.ReadSource(root, unit.Test)
			readable[unit.Test] = err == nil && utf8.Valid(body)
		}
		if !readable[unit.Test] {
			return fmt.Errorf("the config selects project %q test %s, which the static profile cannot read as bounded UTF-8 source; the listing's membership cannot be checked", unit.Project, unit.Test)
		}
		selected[PlaywrightDiscoveryUnit{Project: unit.Project, Test: unit.Test}] = true
	}
	var extra, missing []PlaywrightDiscoveryUnit
	for unit := range listed {
		if !selected[unit] {
			extra = append(extra, unit)
		}
	}
	for unit := range selected {
		if !listed[unit] {
			missing = append(missing, unit)
		}
	}
	less := func(values []PlaywrightDiscoveryUnit) func(i, j int) bool {
		return func(i, j int) bool { return discoveryUnitLess(values[i], values[j]) }
	}
	sort.Slice(extra, less(extra))
	sort.Slice(missing, less(missing))
	if len(missing) != 0 {
		return fmt.Errorf("playwright listing omits project %q test %s (%d pair(s) in all), which the config selects in the current sources; the listing is stale or filtered", missing[0].Project, missing[0].Test, len(missing))
	}
	if len(extra) != 0 {
		return fmt.Errorf("playwright listing names project %q test %s (%d pair(s) in all), which the config does not select in the current sources", extra[0].Project, extra[0].Test, len(extra))
	}
	return nil
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
			if playwrightPathHasLineTerminator(relative) {
				return fmt.Errorf("candidate test path %q contains a line terminator, which Go and JavaScript regular expressions treat differently; the listing's membership cannot be checked", relative)
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

func collectPlaywrightListUnits(realRoot, rootDir, file string, suites []playwrightListSuite, depth int, seen map[PlaywrightDiscoveryUnit]bool) error {
	if depth > playwrightListMaxDepth {
		return errors.New("playwright listing suites nest too deeply")
	}
	for _, suite := range suites {
		suiteFile := file
		if suite.File != "" {
			suiteFile = suite.File
		}
		for _, spec := range suite.Specs {
			specFile := suiteFile
			if spec.File != "" {
				specFile = spec.File
			}
			if playwrightPathHasLineTerminator(rootDir) || playwrightPathHasLineTerminator(specFile) {
				return fmt.Errorf("playwright listing file %q contains a line terminator, which Go and JavaScript regular expressions treat differently; the listing's membership cannot be checked", specFile)
			}
			test, ok := playwrightListPath(realRoot, filepath.Join(rootDir, filepath.FromSlash(specFile)))
			if !ok || !hasSourceExtension(test) || playwrightPathHasLineTerminator(test) {
				return fmt.Errorf("playwright listing file %q under rootDir is not a repository-relative source path", specFile)
			}
			for _, entry := range spec.Tests {
				if entry.ProjectName == nil {
					return fmt.Errorf("playwright listing test in %q has no projectName", specFile)
				}
				seen[PlaywrightDiscoveryUnit{Project: *entry.ProjectName, Test: test}] = true
			}
		}
		if err := collectPlaywrightListUnits(realRoot, rootDir, suiteFile, suite.Suites, depth+1, seen); err != nil {
			return err
		}
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
