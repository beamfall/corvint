package typescript

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"unicode/utf8"
)

// PlaywrightListedTest is one test/project execution of a validated Playwright listing.
type PlaywrightListedTest struct {
	// ID is Playwright's own test identity for this test and project.
	ID string
	// Path is the repository-relative spec file.
	Path    string
	Line    int
	Column  int
	Project string
	// Title is the spec title; Describe holds the enclosing describe titles, outermost first.
	Title    string
	Describe []string
}

// PlaywrightListedTests returns every test/project execution of an unfiltered
// `playwright test --list --reporter=json` report for configPath, under the same refusals as
// PlaywrightDiscoveryFromList: errors, filtering, sharding, a foreign config, paths outside
// root, and a listing whose project/file pairs differ from the pairs the config selects in the
// current sources, including the skipped-directory and static-membership refusals. A selected
// pair the listing omits is accepted under the same grep/grepInvert rules (TJAA-V0-021), whether
// explained or unverified; it has no listed execution, so it adds none. When
// revisionPaths is not nil it holds the repository-relative regular-file paths of the pinned
// revision the caller binds, and a pair the config selects among them must be listed too. Each
// execution keeps Playwright's test ID, repository-relative file, line, column, project and
// title path. A duplicate or missing ID, location or project refuses the listing.
func PlaywrightListedTests(root, configPath string, listing []byte, revisionPaths []string) ([]PlaywrightListedTest, error) {
	report, realRoot, err := validatePlaywrightListing(root, configPath, listing)
	if err != nil {
		return nil, err
	}
	tests := []PlaywrightListedTest{}
	cases := map[string][]playwrightListCase{}
	if err := walkPlaywrightListing(realRoot, report.Config.RootDir, report.Suites, cases, func(listed playwrightListedSpec) error {
		spec := listed.spec
		if !playwrightListText(spec.ID) || !playwrightListText(spec.Title) || spec.Line < 1 || spec.Column < 1 {
			return fmt.Errorf("playwright listing spec in %q lacks its id, title or location", listed.file)
		}
		projects, err := playwrightSpecProjects(listed)
		for _, project := range projects {
			tests = append(tests, PlaywrightListedTest{ID: spec.ID, Path: listed.path, Line: spec.Line, Column: spec.Column, Project: project, Title: spec.Title, Describe: listed.describe})
		}
		return err
	}); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, test := range tests {
		if seen[test.ID] {
			return nil, fmt.Errorf("playwright listing repeats test id %q", test.ID)
		}
		seen[test.ID] = true
	}
	units := map[PlaywrightDiscoveryUnit]bool{}
	for _, test := range tests {
		units[PlaywrightDiscoveryUnit{Project: test.Project, Test: test.Path}] = true
	}
	if _, _, _, err := observePlaywrightListMembership(root, configPath, units, revisionPaths, cases, nil); err != nil {
		return nil, err
	}
	sort.Slice(tests, func(i, j int) bool {
		left, right := tests[i], tests[j]
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		if left.Column != right.Column {
			return left.Column < right.Column
		}
		if left.Project != right.Project {
			return left.Project < right.Project
		}
		return left.ID < right.ID
	})
	return tests, nil
}

// playwrightListedSpec is one spec of a listing as the shared walker attributes it: its listing
// file, its repository-relative path, its enclosing describe titles, outermost first, and its
// grep case (every enclosing suite title, the spec title and its reported tags).
type playwrightListedSpec struct {
	spec     *playwrightListSpec
	file     string
	path     string
	describe []string
	listCase playwrightListCase
}

// walkPlaywrightListing is the one listing walker behind affected discovery and the corpus
// entry point (V1-1234). A spec's file is its own `file`, else its top-level suite's: a nested
// suite's `file` is not the spec's. Its describe titles are the nested suites' titles. It calls
// visit for each spec and records, for the grep omission rules (TJAA-V0-021), each spec that has
// an execution under its file in cases, with every enclosing suite's title (the title path the
// grep rules match, independent of file inheritance), the spec title and its tags with their `@`
// restored; a spec with no execution explains no omission.
func walkPlaywrightListing(realRoot, rootDir string, suites []playwrightListSuite, cases map[string][]playwrightListCase, visit func(playwrightListedSpec) error) error {
	return walkPlaywrightSuites(realRoot, rootDir, "", nil, nil, suites, 0, cases, visit)
}

func walkPlaywrightSuites(realRoot, rootDir, file string, caseTitles, describe []string, suites []playwrightListSuite, depth int, cases map[string][]playwrightListCase, visit func(playwrightListedSpec) error) error {
	if depth > playwrightListMaxDepth {
		return errors.New("playwright listing suites nest too deeply")
	}
	for _, suite := range suites {
		suiteFile, titles := file, describe
		suiteCaseTitles := append(slices.Clip(caseTitles), suite.Title)
		if suite.File != "" && depth == 0 {
			suiteFile = suite.File
		} else if suite.Title != "" {
			titles = append(append([]string{}, describe...), suite.Title)
		}
		for index := range suite.Specs {
			spec := &suite.Specs[index]
			specFile := suiteFile
			if spec.File != "" {
				specFile = spec.File
			}
			if playwrightPathOutsideMatcherModel(rootDir) || playwrightPathOutsideMatcherModel(specFile) {
				return fmt.Errorf("playwright listing file %q contains a line terminator or a character outside the Basic Multilingual Plane, which Go and JavaScript regular expressions treat differently; the listing's membership cannot be checked", specFile)
			}
			path, ok := playwrightListPath(realRoot, filepath.Join(rootDir, filepath.FromSlash(specFile)))
			if !ok || !hasSourceExtension(path) || playwrightPathOutsideMatcherModel(path) {
				return fmt.Errorf("playwright listing file %q under rootDir is not a repository-relative source path", specFile)
			}
			listCase := playwrightListCase{titles: append(slices.Clone(suiteCaseTitles), spec.Title)}
			for _, tag := range spec.Tags {
				listCase.tags = append(listCase.tags, "@"+tag)
			}
			if err := visit(playwrightListedSpec{spec: spec, file: specFile, path: path, describe: titles, listCase: listCase}); err != nil {
				return err
			}
			if len(spec.Tests) != 0 {
				cases[path] = append(cases[path], listCase)
			}
		}
		if err := walkPlaywrightSuites(realRoot, rootDir, suiteFile, suiteCaseTitles, titles, suite.Suites, depth+1, cases, visit); err != nil {
			return err
		}
	}
	return nil
}

// playwrightSpecProjects returns the project of each listed execution of a spec, refusing an
// execution with no valid projectName.
func playwrightSpecProjects(listed playwrightListedSpec) ([]string, error) {
	projects := make([]string, 0, len(listed.spec.Tests))
	for _, entry := range listed.spec.Tests {
		if entry.ProjectName == nil || !utf8.ValidString(*entry.ProjectName) {
			return nil, fmt.Errorf("playwright listing test in %q has no projectName", listed.file)
		}
		projects = append(projects, *entry.ProjectName)
	}
	return projects, nil
}

func playwrightListText(value string) bool {
	return value != "" && len(value) <= 4096 && utf8.ValidString(value)
}
