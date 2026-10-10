package typescript

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

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
	Title string `json:"title"`
	Specs []struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		File   string `json:"file"`
		Line   int    `json:"line"`
		Column int    `json:"column"`
		Tests  []struct {
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
	if !playwrightHex(revision, 40) && !playwrightHex(revision, 64) {
		return nil, errors.New("revision is not a full lowercase object id")
	}
	report, realRoot, err := validatePlaywrightListing(root, configPath, listing)
	if err != nil {
		return nil, err
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
	configBytes, err := affected.ReadSource(root, configPath)
	if err != nil {
		return nil, errors.New("playwright config is unreadable")
	}
	sourceDigest, err := ObservePlaywrightSources(root, configPath)
	if err != nil {
		return nil, err
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
	if listed, ok := playwrightListPath(realRoot, report.Config.ConfigFile); !ok || listed != configPath {
		return report, "", fmt.Errorf("playwright listing config %q is not %s under the repository root", report.Config.ConfigFile, configPath)
	}
	if _, ok := playwrightListPath(realRoot, report.Config.RootDir); !ok && !samePlaywrightDir(realRoot, report.Config.RootDir) {
		return report, "", fmt.Errorf("playwright listing rootDir %q is outside the repository root", report.Config.RootDir)
	}
	return report, realRoot, nil
}

// unfilteredPlaywrightArgv accepts only `... test` followed by the listing, JSON reporter and
// config options, so a recorded file, project, grep, shard or changed-only filter is refused.
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
	for index := start + 1; index < len(argv); index++ {
		switch value := argv[index]; {
		case value == "--list", value == "--reporter=json", strings.HasPrefix(value, "--config="):
		case (value == "--reporter" || value == "--config" || value == "-c") && index+1 < len(argv):
			if value == "--reporter" && argv[index+1] != "json" {
				return fmt.Errorf("playwright listing argv uses reporter %q; use --reporter=json", argv[index+1])
			}
			index++
		default:
			return fmt.Errorf("playwright listing argv has %q; only --list, --reporter=json and --config are allowed so the universe is unfiltered", value)
		}
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
			test, ok := playwrightListPath(realRoot, filepath.Join(rootDir, filepath.FromSlash(specFile)))
			if !ok || !hasSourceExtension(test) {
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
