package typescript

import (
	"errors"
	"fmt"
	"path/filepath"
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
// current sources, including the skipped-directory and static-membership refusals. When
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
	if err := collectPlaywrightListedTests(realRoot, report.Config.RootDir, "", nil, report.Suites, 0, &tests); err != nil {
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
	if _, _, err := observePlaywrightListMembership(root, configPath, units, revisionPaths); err != nil {
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

func collectPlaywrightListedTests(realRoot, rootDir, file string, describe []string, suites []playwrightListSuite, depth int, tests *[]PlaywrightListedTest) error {
	if depth > playwrightListMaxDepth {
		return errors.New("playwright listing suites nest too deeply")
	}
	for _, suite := range suites {
		suiteFile, titles := file, describe
		if suite.File != "" && depth == 0 {
			suiteFile = suite.File
		} else if suite.Title != "" {
			titles = append(append([]string{}, describe...), suite.Title)
		}
		for _, spec := range suite.Specs {
			specFile := suiteFile
			if spec.File != "" {
				specFile = spec.File
			}
			path, ok := playwrightListPath(realRoot, filepath.Join(rootDir, filepath.FromSlash(specFile)))
			if !ok || !hasSourceExtension(path) {
				return fmt.Errorf("playwright listing file %q under rootDir is not a repository-relative source path", specFile)
			}
			if !playwrightListText(spec.ID) || !playwrightListText(spec.Title) || spec.Line < 1 || spec.Column < 1 {
				return fmt.Errorf("playwright listing spec in %q lacks its id, title or location", specFile)
			}
			for _, entry := range spec.Tests {
				if entry.ProjectName == nil || !utf8.ValidString(*entry.ProjectName) {
					return fmt.Errorf("playwright listing test in %q has no projectName", specFile)
				}
				*tests = append(*tests, PlaywrightListedTest{ID: spec.ID, Path: path, Line: spec.Line, Column: spec.Column, Project: *entry.ProjectName, Title: spec.Title, Describe: titles})
			}
		}
		if err := collectPlaywrightListedTests(realRoot, rootDir, suiteFile, titles, suite.Suites, depth+1, tests); err != nil {
			return err
		}
	}
	return nil
}

func playwrightListText(value string) bool {
	return value != "" && len(value) <= 4096 && utf8.ValidString(value)
}
