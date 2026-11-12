// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// refusal is a named, fail-closed stop. Its code is the stable token the spec
// scenarios name (for example `preregistration-changed`).
type refusal struct{ Code, Detail string }

func (r *refusal) Error() string {
	if r.Detail == "" {
		return r.Code
	}
	return r.Code + ": " + r.Detail
}

func refuse(code, format string, args ...any) error {
	return &refusal{Code: code, Detail: fmt.Sprintf(format, args...)}
}

// refusalCode returns the code of a refusal in err's chain, or "".
func refusalCode(err error) string {
	var r *refusal
	if errors.As(err, &r) {
		return r.Code
	}
	return ""
}

// Outcome is one test's result in one run.
type Outcome string

const (
	Pass Outcome = "pass"
	Fail Outcome = "fail"
	Skip Outcome = "skip"
)

// RunResult maps a test ID, `UNIT#NAME`, to its outcome in one run.
type RunResult map[string]Outcome

// planEntry is one selected unit with its optional test-name filter.
type planEntry struct {
	Unit string   `json:"unit"`
	Run  []string `json:"run,omitempty"`
}

// Adapter is the single compiled-in runner contract (FIH-V1-008, G23): run a
// named set of tests in a checked-out tree and return per-test outcomes. Any
// run that is not a clean set of outcomes (timeout, crash, build failure,
// unparseable output) is an error, which the harness treats as a failed
// measurement, never as a pass or a kill.
type Adapter interface {
	Runner() string
	// Run executes the whole suite when entries is nil, else exactly the
	// selected units, each limited to its Run names when it has any.
	Run(ctx context.Context, root string, entries []planEntry) (RunResult, error)
	// DependentUnits names the units whose tests depend on any of paths.
	DependentUnits(ctx context.Context, root string, paths []string) ([]string, error)
}

// runners lists the seven blocking runners of FIH-V1-008; only Go has an
// adapter in this tree (tasks 6 builds the other six).
var runners = []string{"go", "vitest", "jest", "playwright", "pytest", "rspec", "minitest"}

type adapterOptions struct {
	GoTest  []string      // argv prefix running `go test`; default {"go", "test"}
	Scope   []string      // packages of a whole-suite run; default {"./..."}
	Timeout time.Duration // wall bound per adapter command
}

func adapterFor(runner string, opt adapterOptions) (Adapter, error) {
	switch runner {
	case "go":
		a := goAdapter{goTest: opt.GoTest, scope: opt.Scope, timeout: opt.Timeout}
		if len(a.goTest) == 0 {
			a.goTest = []string{"go", "test"}
		}
		if a.timeout <= 0 {
			a.timeout = 30 * time.Minute
		}
		return a, nil
	}
	if slices.Contains(runners, runner) {
		return nil, refuse("runner-adapter-missing", "runner %q has no adapter in this build", runner)
	}
	return nil, refuse("runner-adapter-missing", "unknown runner %q", runner)
}

// selects reports whether a plan executes test id (`UNIT#NAME`).
func selects(entries []planEntry, id string) bool {
	unit, name, ok := strings.Cut(id, "#")
	if !ok {
		return false
	}
	for _, e := range entries {
		if e.Unit == unit && (len(e.Run) == 0 || slices.Contains(e.Run, name)) {
			return true
		}
	}
	return false
}

func testUnit(id string) string { u, _, _ := strings.Cut(id, "#"); return u }

type goAdapter struct {
	goTest  []string
	scope   []string
	timeout time.Duration
}

func (goAdapter) Runner() string { return "go" }

var goTestName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func (a goAdapter) Run(ctx context.Context, root string, entries []planEntry) (RunResult, error) {
	type invocation struct{ pkgs, names []string }
	var calls []invocation
	if entries == nil {
		scope := a.scope
		if len(scope) == 0 {
			scope = []string{"./..."}
		}
		calls = append(calls, invocation{pkgs: slices.Clone(scope)})
	} else {
		var whole []string
		for _, e := range entries {
			pkg, ok := strings.CutPrefix(e.Unit, "go:")
			if !ok || pkg == "" {
				return nil, fmt.Errorf("Go adapter cannot run unit %q", e.Unit)
			}
			if len(e.Run) == 0 {
				whole = append(whole, pkg)
				continue
			}
			for _, n := range e.Run {
				if !goTestName.MatchString(n) {
					return nil, fmt.Errorf("unit %s: unsupported test filter %q", e.Unit, n)
				}
			}
			calls = append(calls, invocation{pkgs: []string{pkg}, names: e.Run})
		}
		slices.Sort(whole)
		whole = slices.Compact(whole)
		if len(whole) > 0 {
			calls = append([]invocation{{pkgs: whole}}, calls...)
		}
	}
	result := RunResult{}
	for _, c := range calls {
		argv := append(slices.Clone(a.goTest), "-json", "-count=1", "-timeout", a.timeout.String())
		if len(c.names) > 0 {
			argv = append(argv, "-run", "^("+strings.Join(c.names, "|")+")$")
		}
		argv = append(argv, c.pkgs...)
		out, err := command(ctx, root, a.timeout+time.Minute, argv...)
		if stopped(ctx, err) {
			// The harness stopped the run, could not prove its cleanup or
			// truncated its output, so the output says nothing about the tree.
			return nil, fmt.Errorf("go test stopped by the harness: %w", err)
		}
		got, parseErr := goOutcomes(out)
		if parseErr != nil {
			return nil, parseErr
		}
		if err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 || !slices.Contains(mapValues(got), Fail) {
				return nil, fmt.Errorf("go test did not finish with test outcomes: %w", err)
			}
		}
		for id, o := range got {
			if prev, ok := result[id]; ok && prev != o {
				return nil, fmt.Errorf("test %s reported twice with different outcomes", id)
			}
			result[id] = o
		}
	}
	return result, nil
}

func mapValues(r RunResult) []Outcome {
	out := make([]Outcome, 0, len(r))
	for _, o := range r {
		out = append(out, o)
	}
	return out
}

type goEvent struct{ Action, Package, Test, Output string }

// goOutcomes parses `go test -json` output into top-level test outcomes. It
// keeps AEO-V0-004's fail-closed rules: a timeout, a panic or a package that
// fails with no failing test (a build failure) is an error, never a kill.
func goOutcomes(out []byte) (RunResult, error) {
	result := RunResult{}
	failedPackages := map[string]bool{}
	failingUnits := map[string]bool{}
	for _, line := range bytes.Split(out, []byte("\n")) {
		if len(line) == 0 || line[0] != '{' {
			continue // admission diagnostics of a wrapper
		}
		var e goEvent
		if json.Unmarshal(line, &e) != nil {
			return nil, errors.New("malformed Go test event")
		}
		if e.Action == "output" && strings.Contains(e.Output, "panic: test timed out after ") {
			return nil, errors.New("Go test timed out")
		}
		if e.Action == "output" && strings.HasPrefix(e.Output, "panic: ") {
			return nil, errors.New("Go test panicked")
		}
		if e.Test == "" {
			if e.Action == "fail" && e.Package != "" {
				failedPackages[e.Package] = true
			}
			continue
		}
		if strings.Contains(e.Test, "/") {
			continue // a subtest's outcome is carried by its top-level test
		}
		var o Outcome
		switch e.Action {
		case "pass":
			o = Pass
		case "fail":
			o = Fail
		case "skip":
			o = Skip
		default:
			continue
		}
		if e.Package == "" {
			return nil, errors.New("Go test event missing package")
		}
		id := "go:" + e.Package + "#" + e.Test
		result[id] = o
		if o == Fail {
			failingUnits["go:"+e.Package] = true
		}
	}
	for pkg := range failedPackages {
		if !failingUnits["go:"+pkg] {
			return nil, fmt.Errorf("Go package %s failed without a failing test", pkg)
		}
	}
	return result, nil
}

// DependentUnits returns the Go test units whose test binary depends on a
// package containing one of paths, from `go list -test -deps`.
func (a goAdapter) DependentUnits(ctx context.Context, root string, paths []string) ([]string, error) {
	goBin := "go"
	if len(a.goTest) > 0 && path.Base(a.goTest[0]) == "go" {
		goBin = a.goTest[0]
	}
	out, err := command(ctx, root, a.timeout, goBin, "list", "-e", "-test", "-f", "{{.ImportPath}}\t{{.Dir}}\t{{join .Deps \" \"}}", "./...")
	if err != nil {
		return nil, fmt.Errorf("go list: %w: %s", err, out)
	}
	type pkg struct {
		dir  string
		deps []string
	}
	pkgs := map[string]pkg{}
	byDir := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		pkgs[parts[0]] = pkg{dir: parts[1], deps: strings.Fields(parts[2])}
		if !strings.Contains(parts[0], " ") && !strings.HasSuffix(parts[0], ".test") {
			byDir[parts[1]] = parts[0]
		}
	}
	changed := map[string]bool{}
	for _, p := range paths {
		dir := filepath.Join(root, filepath.FromSlash(path.Dir(p)))
		if ip, ok := byDir[dir]; ok {
			changed[ip] = true
		}
	}
	var units []string
	for ip, p := range pkgs {
		base, isTest := strings.CutSuffix(ip, ".test")
		if !isTest {
			continue
		}
		hit := changed[base]
		for _, d := range p.deps {
			d, _, _ = strings.Cut(d, " ")
			hit = hit || changed[d]
		}
		if hit {
			units = append(units, "go:"+base)
		}
	}
	slices.Sort(units)
	return units, nil
}
