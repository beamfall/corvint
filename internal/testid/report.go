// SPDX-License-Identifier: Apache-2.0
package testid

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
)

// The per-runner name path (CTI-V1-003 to CTI-V1-007): each reader maps one
// runner's recorded machine report to the canonical IDs of the tests it
// names, in report order. A report that names a test only by its file gives
// that file-level ID with ReasonUnmapped, never a guessed name path
// (CTI-V1-011).

// MaxReportBytes bounds one report a reader consumes; a longer one is refused.
const MaxReportBytes = 64 << 20

// ReasonUnmapped is the reason printed for a test, record or selection that
// cannot be expressed as a test-level ID; it widens to the file when the
// file is known, otherwise to every test of the runner (CTI-V1-011).
const ReasonUnmapped = "test-id-unmapped"

// Named is one test a report names. Reason is empty when ID names the test
// itself and ReasonUnmapped when the report gives only its file.
type Named struct {
	ID     ID
	Reason string
}

// ErrReportTooLarge is returned for a report over MaxReportBytes.
var ErrReportTooLarge = fmt.Errorf("report exceeds %d bytes", MaxReportBytes)

func readReport(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxReportBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxReportBytes {
		return nil, ErrReportTooLarge
	}
	return data, nil
}

// GoModuleDir resolves an import path of the module modulePath, whose go.mod
// lies in the repository-relative directory moduleDir, to its package
// directory: the module's own path is moduleDir and modulePath/rel is
// moduleDir/rel. Any other import path is not resolved.
func GoModuleDir(modulePath, moduleDir string) func(importPath string) (string, bool) {
	return func(importPath string) (string, bool) {
		if modulePath == "" || !CleanFile(moduleDir) {
			return "", false
		}
		rel, ok := "", importPath == modulePath
		if !ok {
			rel, ok = strings.CutPrefix(importPath, modulePath+"/")
		}
		if !ok {
			return "", false
		}
		if rel != "" && !CleanFile(rel) {
			return "", false
		}
		return path.Join(moduleDir, rel), true
	}
}

// GoTest is the ID of test as `go test -json` reports it in the package
// directory dir: the top-level Test, Fuzz or Example function, then each
// subtest name split on `/` (CTI-V1-003).
func GoTest(dir, test string) (ID, error) {
	if test == "" {
		return ID{}, fmt.Errorf("%w: empty Go test name", ErrMalformed)
	}
	return New(Go, dir, strings.Split(test, "/")...)
}

// GoTestJSON maps a `go test -json` stream (test2json events, one per line)
// to the ID of every test it ran, one per "run" event in event order, so a
// name run twice in one package (an external _test package beside an
// internal one) appears twice for List to mark duplicate (CTI-V1-010). The
// report is expected from -count=1, as `corvint test` runs it. dirOf
// resolves each event's import path to its package directory; a package it
// cannot resolve refuses the report rather than guessing a directory. A line
// that is not a JSON event (build output) is skipped.
func GoTestJSON(r io.Reader, dirOf func(importPath string) (string, bool)) ([]Named, error) {
	data, err := readReport(r)
	if err != nil {
		return nil, err
	}
	var named []Named
	for _, line := range bytes.Split(data, []byte("\n")) {
		var event struct {
			Action  string
			Package string
			Test    string
		}
		if len(bytes.TrimSpace(line)) == 0 || json.Unmarshal(line, &event) != nil || event.Action != "run" || event.Test == "" {
			continue
		}
		dir, ok := dirOf(event.Package)
		if !ok {
			return nil, fmt.Errorf("go test -json: package %q has no known directory", event.Package)
		}
		id, err := GoTest(dir, event.Test)
		if err != nil {
			return nil, err
		}
		named = append(named, Named{ID: id})
	}
	return named, nil
}

// relativeFile is reported, an absolute or relative path, as a clean
// repository-relative slash path under root, the absolute repository root.
// Backslashes are read as separators, as a Windows report writes them.
func relativeFile(root, reported string) (string, error) {
	root = strings.TrimSuffix(strings.ReplaceAll(root, `\`, "/"), "/")
	reported = strings.ReplaceAll(reported, `\`, "/")
	if rel, ok := strings.CutPrefix(reported, root+"/"); ok && root != "" {
		reported = rel
	}
	if !CleanFile(reported) || reported == "." {
		return "", fmt.Errorf("test file %q is not under the repository root %q", reported, root)
	}
	return reported, nil
}

// JestTest is the ID of a Vitest or Jest test in file: the report's
// ancestorTitles in order, then its title, as the runner interpolated them
// (CTI-V1-004).
func JestTest(runner, file string, ancestorTitles []string, title string) (ID, error) {
	if runner != Vitest && runner != Jest {
		return ID{}, fmt.Errorf("%w: %q is not vitest or jest", ErrMalformed, runner)
	}
	return New(runner, file, append(append([]string(nil), ancestorTitles...), title)...)
}

// JestJSON maps a Jest `--json` report, or the Vitest JSON reporter's report
// in the same shape, to the ID of every assertion result (CTI-V1-004).
// root is the absolute repository root each testResults name is under.
func JestJSON(runner string, r io.Reader, root string) ([]Named, error) {
	data, err := readReport(r)
	if err != nil {
		return nil, err
	}
	var report struct {
		TestResults []struct {
			Name             string `json:"name"`
			AssertionResults []struct {
				AncestorTitles []string `json:"ancestorTitles"`
				Title          string   `json:"title"`
			} `json:"assertionResults"`
		} `json:"testResults"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("%s report: %w", runner, err)
	}
	var named []Named
	for _, result := range report.TestResults {
		file, err := relativeFile(root, result.Name)
		if err != nil {
			return nil, err
		}
		for _, assertion := range result.AssertionResults {
			id, err := JestTest(runner, file, assertion.AncestorTitles, assertion.Title)
			if err != nil {
				return nil, err
			}
			named = append(named, Named{ID: id})
		}
	}
	return named, nil
}

// PlaywrightTest is the ID of a Playwright test in file: the project name
// (empty when the configuration defines no projects), each describe title,
// then the test title (CTI-V1-005).
func PlaywrightTest(file, project string, describes []string, title string) (ID, error) {
	segments := append(append([]string{project}, describes...), title)
	return New(Playwright, file, segments...)
}

type playwrightSuite struct {
	Title string `json:"title"`
	File  string `json:"file"`
	Specs []struct {
		Title string `json:"title"`
		Tests []struct {
			ProjectName string `json:"projectName"`
		} `json:"tests"`
	} `json:"specs"`
	Suites []playwrightSuite `json:"suites"`
}

// PlaywrightJSON maps a Playwright JSON reporter report to the ID of every
// test, one per spec and project (CTI-V1-005). Each top-level suite is a
// spec file, named relative to config.rootDir; each nested suite is a
// describe block. root is the absolute repository root.
func PlaywrightJSON(r io.Reader, root string) ([]Named, error) {
	data, err := readReport(r)
	if err != nil {
		return nil, err
	}
	var report struct {
		Config struct {
			RootDir string `json:"rootDir"`
		} `json:"config"`
		Suites []playwrightSuite `json:"suites"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("playwright report: %w", err)
	}
	var named []Named
	var walk func(file string, describes []string, suite playwrightSuite) error
	walk = func(file string, describes []string, suite playwrightSuite) error {
		for _, spec := range suite.Specs {
			for _, test := range spec.Tests {
				id, err := PlaywrightTest(file, test.ProjectName, describes, spec.Title)
				if err != nil {
					return err
				}
				named = append(named, Named{ID: id})
			}
		}
		for _, child := range suite.Suites {
			if err := walk(file, append(append([]string(nil), describes...), child.Title), child); err != nil {
				return err
			}
		}
		return nil
	}
	for _, suite := range report.Suites {
		reported := suite.File
		if !strings.HasPrefix(strings.ReplaceAll(reported, `\`, "/"), "/") && report.Config.RootDir != "" {
			reported = strings.TrimSuffix(report.Config.RootDir, "/") + "/" + reported
		}
		file, err := relativeFile(root, reported)
		if err != nil {
			return nil, err
		}
		if err := walk(file, nil, suite); err != nil {
			return nil, err
		}
	}
	return named, nil
}

// PytestNodeID is the ID of a pytest node ID whose file part is relative to
// the repository-relative pytest rootdir: the file part, then each `::` part
// after it, where a `::` inside a `[...]` parameter part does not split and
// the parameter part is kept verbatim (CTI-V1-006).
func PytestNodeID(rootDir, nodeID string) (ID, error) {
	var parts []string
	depth, start := 0, 0
	for index := 0; index < len(nodeID); index++ {
		switch {
		case nodeID[index] == '[':
			depth++
		case nodeID[index] == ']' && depth > 0:
			depth--
		case depth == 0 && strings.HasPrefix(nodeID[index:], separator):
			parts = append(parts, nodeID[start:index])
			start = index + len(separator)
			index++
		}
	}
	parts = append(parts, nodeID[start:])
	if len(parts) < 2 {
		return ID{}, fmt.Errorf("%w: pytest node ID %q names no test", ErrMalformed, nodeID)
	}
	if !CleanFile(rootDir) || !CleanFile(parts[0]) {
		return ID{}, fmt.Errorf("%w: pytest node ID %q under rootdir %q is not a clean relative file", ErrMalformed, nodeID, rootDir)
	}
	return New(Pytest, path.Join(rootDir, parts[0]), parts[1:]...)
}

// PytestJSON maps a pytest-json-report document (`tests[].nodeid`) to the ID
// of every test (CTI-V1-006). rootDir is the pytest rootdir relative to the
// repository root.
func PytestJSON(r io.Reader, rootDir string) ([]Named, error) {
	data, err := readReport(r)
	if err != nil {
		return nil, err
	}
	var report struct {
		Tests []struct {
			NodeID string `json:"nodeid"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("pytest report: %w", err)
	}
	var named []Named
	for _, test := range report.Tests {
		id, err := PytestNodeID(rootDir, test.NodeID)
		if err != nil {
			return nil, err
		}
		named = append(named, Named{ID: id})
	}
	return named, nil
}

// RSpecExample is the ID of an RSpec example: its rerun file path, each
// example group description, then the example description (CTI-V1-007).
func RSpecExample(rerunFile string, groups []string, description string) (ID, error) {
	return New(RSpec, strings.TrimPrefix(rerunFile, "./"), append(append([]string(nil), groups...), description)...)
}

// RSpecJSON maps an RSpec `--format json` report, the stock shape Corvint's
// own formatter extends, to each example's ID. The rerun file path is the
// file part of the example id (`./spec/a_spec.rb[1:2]`), which names the
// including spec for a shared example. The report carries no example group
// chain, only the joined full_description, so an example is named by its
// file-level ID with ReasonUnmapped instead of a guessed split (CTI-V1-007,
// CTI-V1-011).
func RSpecJSON(r io.Reader) ([]Named, error) {
	data, err := readReport(r)
	if err != nil {
		return nil, err
	}
	var report struct {
		Examples []struct {
			ID       string `json:"id"`
			FilePath string `json:"file_path"`
		} `json:"examples"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("rspec report: %w", err)
	}
	var named []Named
	for _, example := range report.Examples {
		rerun := example.FilePath
		if file, _, ok := strings.Cut(example.ID, "["); ok && file != "" {
			rerun = file
		}
		id, err := New(RSpec, strings.TrimPrefix(rerun, "./"))
		if err != nil {
			return nil, err
		}
		named = append(named, Named{ID: id, Reason: ReasonUnmapped})
	}
	return named, nil
}

// MinitestMethod is the ID of the Minitest method name of class, defined in
// the repository-relative source file (CTI-V1-007).
func MinitestMethod(file, class, method string) (ID, error) {
	if class == "" || method == "" {
		return ID{}, fmt.Errorf("%w: Minitest test %q#%q", ErrMalformed, class, method)
	}
	return New(Minitest, file, class, method)
}

// MinitestJSONL maps the Corvint Ruby line-coverage hook's JSON-lines dump
// (internal/rubylinecov/linecov.rb) of a Minitest run to each test's ID: one
// per `{"kind":"test"}` record, whose package is the method's source file
// and whose name is `Class#method` (CTI-V1-007). A test record whose name is
// not `Class#method` refuses the report: it is not a Minitest run.
func MinitestJSONL(r io.Reader) ([]Named, error) {
	data, err := readReport(r)
	if err != nil {
		return nil, err
	}
	var named []Named
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64<<10), MaxReportBytes)
	for scanner.Scan() {
		var record struct {
			Kind    string `json:"kind"`
			Package string `json:"package"`
			Name    string `json:"name"`
		}
		if len(bytes.TrimSpace(scanner.Bytes())) == 0 {
			continue
		}
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return nil, fmt.Errorf("minitest hook record: %w", err)
		}
		if record.Kind != "test" {
			continue
		}
		class, method, ok := strings.Cut(record.Name, "#")
		if !ok {
			return nil, fmt.Errorf("%w: Minitest record name %q is not Class#method", ErrMalformed, record.Name)
		}
		id, err := MinitestMethod(record.Package, class, method)
		if err != nil {
			return nil, err
		}
		named = append(named, Named{ID: id})
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.Join(errors.New("minitest hook dump"), err)
	}
	return named, nil
}
