// SPDX-License-Identifier: Apache-2.0
package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/testid"
	"github.com/Beamfall/corvint/internal/testinventory"
)

// goRunner selects by package and `-run`, and reads `go test -json`.
type goRunner struct{}

func (goRunner) Name() string                         { return testid.Go }
func (goRunner) Lister() testinventory.ListerIdentity { return lister(testid.Go) }
func (goRunner) Default(Instance, func(string) bool) []string {
	return []string{"go", "test", "./..."}
}

// goPattern reports whether word is a package pattern of a go test argv,
// the words the selection replaces: `./...`, `.`, or a `./` path.
func goPattern(word string) bool {
	return word == "." || word == "./..." || strings.HasPrefix(word, "./")
}

// Command keeps the full command's `go test` flags, adds -json, and names
// the selected package directories relative to the instance root. A Go
// argv carries one -run pattern for every package it names, so the
// selection is exact only when every package runs whole, or every package
// selects the same test names; otherwise the union runs with exact false.
// A subtest runs with its top-level test.
func (goRunner) Command(request Request) (Command, error) {
	if err := checkRequest(request, testid.Go); err != nil {
		return Command{}, err
	}
	if len(request.Full) < 2 || request.Full[0] != "go" || request.Full[1] != "test" {
		command := full(request)
		if request.Tests != nil {
			command.Exact, command.Reason = false, "the go full command is not `go test`, so it runs whole"
		}
		return command, nil
	}
	flags := []string{"go", "test", "-json"}
	for _, word := range request.Full[2:] {
		if !goPattern(word) && word != "-json" {
			flags = append(flags, word)
		}
	}
	if request.Tests == nil {
		return Command{Argv: append(flags, "./..."), Dir: request.Instance.Root, Report: request.Report, Tier: TierFull, Exact: true}, nil
	}
	if len(request.Tests) == 0 {
		return Command{}, fmt.Errorf("%s: an empty selection runs nothing", request.Instance.Key())
	}
	whole := map[string]bool{}
	names := map[string][]string{}
	reasons := []string{}
	for _, text := range request.Tests {
		id, err := testid.Parse(text)
		if err != nil {
			return Command{}, err
		}
		if id.IsFileLevel() {
			whole[id.File] = true
			continue
		}
		if len(id.Segments) > 1 {
			reasons = append(reasons, "a subtest runs with its top-level test")
		}
		if !slices.Contains(names[id.File], id.Segments[0]) {
			names[id.File] = append(names[id.File], id.Segments[0])
		}
	}
	dirs := []string{}
	for dir := range whole {
		dirs = append(dirs, dir)
	}
	for dir := range names {
		if !whole[dir] {
			dirs = append(dirs, dir)
		}
	}
	slices.Sort(dirs)
	run := ""
	switch {
	case len(names) == 0:
	case len(whole) != 0:
		// One -run would filter the whole packages too, dropping tests.
		reasons = append(reasons, "one -run pattern applies to every package, so the packages with named tests run whole")
	default:
		union := []string{}
		same := true
		var first []string
		for _, dir := range dirs {
			set := slices.Sorted(slices.Values(names[dir]))
			if first == nil {
				first = set
			} else if !slices.Equal(first, set) {
				same = false
			}
			for _, name := range set {
				if !slices.Contains(union, name) {
					union = append(union, name)
				}
			}
		}
		slices.Sort(union)
		if !same {
			reasons = append(reasons, "one -run pattern applies to every package, so each runs the union of the named tests")
		}
		quoted := make([]string, len(union))
		for index, name := range union {
			quoted[index] = regexp.QuoteMeta(name)
		}
		run = "^(" + strings.Join(quoted, "|") + ")$"
	}
	argv := flags
	if run != "" {
		argv = append(argv, "-run", run)
	}
	for _, dir := range dirs {
		argv = append(argv, goPackageArg(request.Instance.Root, dir))
	}
	command := Command{Argv: argv, Dir: request.Instance.Root, Report: request.Report, Tier: tierOf(request), Exact: len(reasons) == 0}
	if !command.Exact {
		command.Reason = strings.Join(slices.Compact(reasons), "; ")
	}
	return command, nil
}

// goPackageArg is the package directory dir as a pattern relative to root.
func goPackageArg(root, dir string) string {
	if root == "." {
		if dir == "." {
			return "."
		}
		return "./" + dir
	}
	if dir == root {
		return "."
	}
	return "./" + strings.TrimPrefix(dir, root+"/")
}

// Parse reads the test2json stream: one result per test and subtest, with
// its elapsed time, and a package-level result for a package that failed
// without naming a test (a build failure). Lines that are not events
// (build output) are skipped.
func (goRunner) Parse(request ParseRequest) ([]Result, error) {
	dirOf := testid.GoModuleDir(request.Module, request.Instance.Root)
	var results []Result
	named := map[string]bool{}
	for _, line := range bytes.Split(request.Report, []byte("\n")) {
		var event struct {
			Action  string
			Package string
			Test    string
			Elapsed float64
		}
		if len(bytes.TrimSpace(line)) == 0 || json.Unmarshal(line, &event) != nil {
			continue
		}
		outcome := map[string]string{"pass": OutcomePass, "fail": OutcomeFail, "skip": OutcomeSkip}[event.Action]
		if outcome == "" || event.Package == "" {
			continue
		}
		dir, ok := dirOf(event.Package)
		if !ok {
			return nil, fmt.Errorf("go test -json: package %q is not in module %q at %s", event.Package, request.Module, request.Instance.Root)
		}
		if event.Test == "" {
			if outcome == OutcomeFail && !named[dir] {
				id, err := testid.New(testid.Go, dir)
				if err != nil {
					return nil, err
				}
				results = append(results, Result{ID: id.String(), Outcome: OutcomeFail, DurationMillis: millis(event.Elapsed), Granularity: testid.GranularityPackage})
			}
			continue
		}
		id, err := testid.GoTest(dir, event.Test)
		if err != nil {
			return nil, err
		}
		named[dir] = true
		results = append(results, Result{ID: id.String(), Outcome: outcome, DurationMillis: millis(event.Elapsed), Granularity: testid.GranularityTest})
	}
	return results, nil
}

func millis(seconds float64) int64 { return int64(seconds*1000 + 0.5) }

// relativeTo is the repository-relative file as a path under root.
func relativeTo(root, file string) string {
	if root == "." {
		return file
	}
	return strings.TrimPrefix(file, root+"/")
}
