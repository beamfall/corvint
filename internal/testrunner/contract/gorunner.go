// SPDX-License-Identifier: Apache-2.0
package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/testid"
	"github.com/Beamfall/corvint/internal/testinventory"
)

// goRunner selects by package and `-run`, and reads `go test -json`.
type goRunner struct{}

// goEveryPackage is the package pattern of the Go full command.
const goEveryPackage = "./..."

func (goRunner) Name() string                         { return testid.Go }
func (goRunner) Lister() testinventory.ListerIdentity { return lister(testid.Go) }
func (goRunner) Default(Instance, func(string) bool) []string {
	return []string{"go", "test", goEveryPackage}
}

// goValueFlags are the go test build and test flags that take a value,
// written `-flag value` or `-flag=value` (`go help build`, `go help
// testflag`).
var goValueFlags = map[string]bool{
	"C": true, "p": true, "o": true, "exec": true, "asmflags": true, "buildmode": true, "compiler": true,
	"gccgoflags": true, "gcflags": true, "installsuffix": true, "ldflags": true, "mod": true, "modfile": true,
	"overlay": true, "pgo": true, "pkgdir": true, "tags": true, "toolexec": true, "coverpkg": true,
	"covermode": true, "vet": true, "run": true, "skip": true, "bench": true, "benchtime": true, "count": true,
	"cpu": true, "parallel": true, "timeout": true, "coverprofile": true, "cpuprofile": true, "memprofile": true,
	"memprofilerate": true, "blockprofile": true, "blockprofilerate": true, "mutexprofile": true,
	"mutexprofilefraction": true, "outputdir": true, "trace": true, "fuzz": true, "fuzztime": true,
	"fuzzminimizetime": true, "shuffle": true, "list": true,
}

// goBoolFlags are the go test flags written alone or as `-flag=value`.
var goBoolFlags = map[string]bool{
	"a": true, "n": true, "x": true, "v": true, "work": true, "race": true, "msan": true, "asan": true,
	"trimpath": true, "linkshared": true, "cover": true, "benchmem": true, "failfast": true, "fullpath": true,
	"short": true, "json": true, "buildvcs": true, "modcacherw": true,
}

// splitGoTest splits the words after `go test` into flags (each value kept
// with its flag) and package patterns, dropping -json. A word it cannot
// model (`-args`, which hands the rest to the test binary, an unknown flag,
// or a value flag with no value) is returned as the problem, and the
// selection then runs the full command with exact false.
func splitGoTest(words []string) (flags, patterns []string, problem string) {
	for index := 0; index < len(words); index++ {
		word := words[index]
		if word == "--" || !strings.HasPrefix(word, "-") || word == "-" {
			if word == "--" {
				return nil, nil, "`--` in the go full command is not modelled"
			}
			patterns = append(patterns, word)
			continue
		}
		name, _, valued := strings.Cut(strings.TrimLeft(word, "-"), "=")
		switch {
		case name == "args":
			return nil, nil, "-args in the go full command passes the rest to the test binary"
		case name == "json":
			continue
		case goBoolFlags[name]:
			flags = append(flags, word)
		case goValueFlags[name] && valued:
			flags = append(flags, word)
		case goValueFlags[name] && index+1 < len(words):
			flags = append(flags, word, words[index+1])
			index++
		case goValueFlags[name]:
			return nil, nil, "the go flag " + word + " has no value"
		default:
			return nil, nil, "the go flag " + word + " is not one Corvint models"
		}
	}
	return flags, patterns, ""
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
	// The full command runs as declared, with -json for its report.
	whole := Command{Argv: []string{"go", "test", "-json"}, Dir: request.Instance.Root, Report: request.Report, Tier: TierFull, Exact: true}
	for _, word := range request.Full[2:] {
		if word != "-json" && word != "--json" {
			whole.Argv = append(whole.Argv, word)
		}
	}
	if request.Tests == nil {
		return whole, nil
	}
	parsed, _, problem := splitGoTest(request.Full[2:])
	if problem != "" {
		whole.Exact, whole.Reason = false, problem+", so the full command runs"
		return whole, nil
	}
	flags := append([]string{"go", "test", "-json"}, parsed...)
	if len(request.Tests) == 0 {
		return Command{}, fmt.Errorf("%s: an empty selection runs nothing", request.Instance.Key())
	}
	wholeDirs := map[string]bool{}
	names := map[string][]string{}
	reasons := []string{}
	for _, text := range request.Tests {
		id, err := testid.Parse(text)
		if err != nil {
			return Command{}, err
		}
		if id.IsFileLevel() {
			wholeDirs[id.File] = true
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
	for dir := range wholeDirs {
		dirs = append(dirs, dir)
	}
	for dir := range names {
		if !wholeDirs[dir] {
			dirs = append(dirs, dir)
		}
	}
	slices.Sort(dirs)
	run := ""
	switch {
	case len(names) == 0:
	case len(wholeDirs) != 0:
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
// its elapsed time, and a package-level result for every package that ends
// with fail (a build failure, a failing test, or a failure after its tests
// passed). A package that ends abnormally is incomplete: a package-level
// error result, and an error result for each test it started and never
// ended. That is a package with no terminal event (the stream stopped), a
// package that ends with a started test never ended (a timeout, or a lost
// line), or a failed package whose output reports a panic, after which its
// remaining tests never ran.
// A line that starts with `{` but is no event (a truncated line) is counted
// and returns a *ReportError, with the results read, so the run is
// incomplete; other lines (build text) are skipped.
func (goRunner) Parse(request ParseRequest) ([]Result, error) {
	dirOf := testid.GoModuleDir(request.Module, request.Instance.Root)
	type packageState struct {
		running  map[string]bool
		ended    bool
		panicked bool
	}
	states := map[string]*packageState{}
	var order []string
	var results []Result
	malformed := 0
	packageResult := func(dir, outcome string, elapsed float64) error {
		id, err := testid.New(testid.Go, dir)
		if err != nil {
			return err
		}
		results = append(results, Result{ID: id.String(), Outcome: outcome, DurationMillis: millis(elapsed), Granularity: testid.GranularityPackage})
		return nil
	}
	unended := func(dir string, state *packageState) error {
		for _, name := range slices.Sorted(maps.Keys(state.running)) {
			id, err := testid.GoTest(dir, name)
			if err != nil {
				return err
			}
			results = append(results, Result{ID: id.String(), Outcome: OutcomeError, Granularity: testid.GranularityTest})
		}
		return nil
	}
	for _, line := range bytes.Split(request.Report, []byte("\n")) {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}
		var event struct {
			Action  string
			Package string
			Test    string
			Output  string
			Elapsed float64
		}
		if json.Unmarshal(trimmed, &event) != nil {
			if trimmed[0] == '{' {
				malformed++
			}
			continue
		}
		if event.Package == "" || !slices.Contains([]string{"start", "run", "output", "pass", "fail", "skip"}, event.Action) {
			continue
		}
		dir, ok := dirOf(event.Package)
		if !ok {
			return nil, fmt.Errorf("go test -json: package %q is not in module %q at %s", event.Package, request.Module, request.Instance.Root)
		}
		state := states[dir]
		if state == nil {
			state = &packageState{running: map[string]bool{}}
			states[dir] = state
			order = append(order, dir)
		}
		switch event.Action {
		case "start":
			continue
		case "run":
			if event.Test != "" {
				state.running[event.Test] = true
			}
			continue
		case "output":
			if strings.HasPrefix(event.Output, "panic: ") {
				state.panicked = true
			}
			continue
		}
		outcome := map[string]string{"pass": OutcomePass, "fail": OutcomeFail, "skip": OutcomeSkip}[event.Action]
		if event.Test == "" {
			state.ended = true
			if outcome != OutcomeFail && len(state.running) == 0 {
				continue
			}
			if len(state.running) != 0 || state.panicked {
				if err := unended(dir, state); err != nil {
					return nil, err
				}
				outcome = OutcomeError
			}
			if err := packageResult(dir, outcome, event.Elapsed); err != nil {
				return nil, err
			}
			continue
		}
		delete(state.running, event.Test)
		id, err := testid.GoTest(dir, event.Test)
		if err != nil {
			return nil, err
		}
		results = append(results, Result{ID: id.String(), Outcome: outcome, DurationMillis: millis(event.Elapsed), Granularity: testid.GranularityTest})
	}
	for _, dir := range order {
		if state := states[dir]; !state.ended {
			if err := unended(dir, state); err != nil {
				return nil, err
			}
			if err := packageResult(dir, OutcomeError, 0); err != nil {
				return nil, err
			}
		}
	}
	if malformed != 0 {
		return results, &ReportError{Malformed: malformed}
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
