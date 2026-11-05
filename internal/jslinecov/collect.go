// SPDX-License-Identifier: Apache-2.0

package jslinecov

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/linecov/v8"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/dynamic"
)

// Runners are the node runners the collector drives (LPCV-V0-967).
var Runners = []string{"mocha", "jasmine", "node-test", "jest", "vitest", "ava", "bun-test", "deno-test"}

// Options configures one collection run (LPCV-V0-967).
type Options struct {
	Module     string   // the index module label; records name module/path
	Runner     string   // one of Runners
	Executable string   // absolute runner executable (node for node-test)
	Files      []string // repository-relative test files
	Work       string   // scratch directory for reports and coverage
	Parallel   int
	Timeout    time.Duration // per isolated test process
	// SuiteTimeout bounds each file's in-order run.
	SuiteTimeout time.Duration
	// Keep admits a repository-relative path into the records, normally the
	// tracked files at the index commit; nil keeps every file outside
	// node_modules.
	Keep func(rel string) bool
	Env  []string // extra environment, KEY=VALUE
	// Config is the runner configuration file, passed as the runner reads it.
	Config string
}

// Identity is the index Runner string for a collection with version, the
// caller-observed runner version: indexes collected differently decide
// nothing for one another (LPCV-V0-951).
func Identity(runner, version string) string {
	return "v8-lines/1 " + runner + " " + version
}

// Collect runs each test file once in ordinary order, recorded as its InOrder
// record, then every passing test of it alone in its own runner process under
// the runner's own name filter, with V8 coverage mapped through source maps to
// the original sources. A test whose isolated run fails, times out, or runs
// any other test yields no record; a file whose in-order run does not pass
// yields Divergent outcomes, so the caller lists it as Unordered
// (LPCV-V0-955, LPCV-V0-962, LPCV-V0-967).
func Collect(ctx context.Context, root string, o Options) ([]linecov.Outcome, error) {
	if !known(o.Runner) {
		return nil, fmt.Errorf("jslinecov: unsupported runner %q", o.Runner)
	}
	if !filepath.IsAbs(o.Executable) || !filepath.IsAbs(o.Work) {
		return nil, errors.New("jslinecov: absolute executable and work directory required")
	}
	if o.Parallel < 1 {
		o.Parallel = 1
	}
	// V8 reports real paths, so a symlinked root must be resolved to match.
	root, err := filepath.Abs(root)
	if err == nil {
		root, err = filepath.EvalSymlinks(root)
	}
	if err != nil {
		return nil, err
	}
	scope := v8.RepoScope(root, o.Keep)
	var outcomes []linecov.Outcome
	type job struct {
		file string
		t    tr.Test
	}
	var jobs []job
	for i, file := range o.Files {
		dir := filepath.Join(o.Work, fmt.Sprintf("f%d", i))
		start := time.Now()
		res, err := o.run(ctx, root, scope, dir, file, nil, o.SuiteTimeout)
		if err != nil {
			return nil, err
		}
		full := res.observation.Tests
		if why := res.problem(); why != "" {
			outcomes = append(outcomes, linecov.Outcome{Package: file, Name: linecov.InOrder, Divergent: why})
			continue
		}
		names := map[string]int{}
		for _, t := range full {
			names[fullName(o.Runner, t)]++
		}
		outcomes = append(outcomes, linecov.Outcome{Package: file, Name: linecov.InOrder,
			Record: &linecov.Record{Package: file, Name: linecov.InOrder, Profile: res.lines.Profile(o.Module), Elapsed: time.Since(start)}})
		for _, t := range full {
			name := fullName(o.Runner, t)
			switch {
			case t.State == tr.Skipped:
				continue
			case name == "" || t.Name == "":
				outcomes = append(outcomes, linecov.Outcome{Package: file, Name: t.ID, Divergent: "unnamed test"})
			case names[name] > 1:
				outcomes = append(outcomes, linecov.Outcome{Package: file, Name: name, Divergent: "name shared by another test of the file"})
			default:
				jobs = append(jobs, job{file, t})
			}
		}
	}
	isolated := make([]linecov.Outcome, len(jobs))
	var wg sync.WaitGroup
	next := make(chan int)
	var mu sync.Mutex
	var firstErr error
	for w := 0; w < o.Parallel; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				j := jobs[i]
				out, err := o.isolate(ctx, root, scope, filepath.Join(o.Work, fmt.Sprintf("t%d", i)), j.file, j.t)
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
				}
				isolated[i] = out
			}
		}()
	}
	for i := range jobs {
		next <- i
	}
	close(next)
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	outcomes = append(outcomes, isolated...)
	sort.Slice(outcomes, func(i, j int) bool {
		a, b := outcomes[i], outcomes[j]
		return a.Package < b.Package || a.Package == b.Package && a.Name < b.Name
	})
	return outcomes, nil
}

func known(r string) bool {
	for _, x := range Runners {
		if x == r {
			return true
		}
	}
	return false
}

// fullName is a test's name within its file, stable across runs: the full
// title the runner reports after the file in its identity, or the test's own
// title for node:test, whose identity carries line numbers, AVA, and the JUnit
// reports of bun and deno, whose suite and class repeat the file. Titles the
// file shares are refused, not merged.
func fullName(runner string, t tr.Test) string {
	switch runner {
	case "node-test", "ava", "bun-test", "deno-test":
		return t.Name
	}
	if _, after, ok := strings.Cut(t.ID, "::"); ok {
		return after
	}
	return ""
}

// isolate runs one test alone and checks that only it ran (LPCV-V0-955).
func (o Options) isolate(ctx context.Context, root string, scope v8.Scope, dir, file string, t tr.Test) (linecov.Outcome, error) {
	name := fullName(o.Runner, t)
	out := linecov.Outcome{Package: file, Name: name}
	filter, ok := o.filter(name, t.Name)
	if !ok {
		out.Divergent = "title the runner's filter cannot select exactly"
		return out, nil
	}
	start := time.Now()
	res, err := o.run(ctx, root, scope, dir, file, filter, o.Timeout)
	if err != nil {
		return out, err
	}
	elapsed := time.Since(start)
	if why := res.problem(); why != "" {
		out.Divergent = why
		return out, nil
	}
	ran := 0
	for _, x := range res.observation.Tests {
		switch {
		case x.State == tr.Skipped:
		case fullName(o.Runner, x) == name && x.State == tr.Passed:
			ran++
		default:
			out.Divergent = "filter ran another test: " + x.ID
			return out, nil
		}
	}
	if ran != 1 {
		out.Divergent = fmt.Sprintf("filter ran the test %d times", ran)
		return out, nil
	}
	out.Record = &linecov.Record{Package: file, Name: name, Profile: res.lines.Profile(o.Module), Elapsed: elapsed}
	return out, nil
}

// filter is the runner's own flags selecting the test named name, as
// fullName gives it, whose own title is leaf: the anchored full title, so
// suites sharing a leaf title stay apart. Vitest joins the suites and title
// with " > " where its report has spaces, so a space within the suites may be
// either; bun's name is only the leaf, which ends its full title. A pattern
// that matches another test too diverges rather than narrowing.
func (o Options) filter(name, leaf string) ([]string, bool) {
	exact := `^` + regexp.QuoteMeta(name) + `$`
	switch o.Runner {
	case "mocha":
		return []string{"--grep", exact}, true
	case "jasmine":
		return []string{"--filter=" + exact}, true
	case "node-test":
		return []string{"--test-name-pattern=" + exact}, true
	case "jest":
		return []string{"-t", exact}, true
	case "vitest":
		suites, ok := strings.CutSuffix(name, leaf)
		if !ok || suites != "" && !strings.HasSuffix(suites, " ") {
			return nil, false
		}
		pat := regexp.QuoteMeta(leaf)
		if suites != "" {
			pat = strings.ReplaceAll(regexp.QuoteMeta(suites[:len(suites)-1]), " ", "(?: | > )") + " > " + pat
		}
		return []string{"-t", "^" + pat + "$"}, true
	case "bun-test":
		return []string{"-t", `(^| )` + regexp.QuoteMeta(name) + `$`}, true
	case "deno-test":
		return []string{"--filter", "/" + exact + "/"}, true
	case "ava":
		// --match is a glob whose only wildcard is *; a leading ! negates.
		if strings.ContainsAny(name, "*") || strings.HasPrefix(name, "!") {
			return nil, false
		}
		return []string{"--match=" + name}, true
	}
	return nil, false
}

// result is one runner process: its parsed observation, the lines its
// coverage executed, and why it is not a passing run.
type result struct {
	observation tr.Observation
	lines       v8.Lines
	why         string
}

func (r result) problem() string {
	if r.why != "" {
		return r.why
	}
	for _, t := range r.observation.Tests {
		switch t.State {
		case tr.Passed, tr.Skipped:
		default:
			return "test " + t.ID + " " + t.State
		}
	}
	for _, p := range r.observation.Problems {
		return p.Code + ": " + p.Detail
	}
	return ""
}

// run executes the runner once over file with the extra flags, under
// coverage, and reads its report and coverage. An error is a collector fault;
// a failing run is a result whose why is set.
func (o Options) run(ctx context.Context, root string, scope v8.Scope, dir, file string, extra []string, timeout time.Duration) (result, error) {
	// A run reads only what it wrote: the directory starts empty and goes
	// once the run is read.
	if err := os.RemoveAll(dir); err != nil {
		return result{}, err
	}
	defer os.RemoveAll(dir)
	reports := filepath.Join(dir, "reports")
	cov := filepath.Join(dir, "coverage")
	for _, d := range []string{reports, cov} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return result{}, err
		}
	}
	inv, err := dynamic.Build(tr.Request{Runner: o.Runner, Root: root, Executable: o.Executable, ReportDir: reports, Config: o.Config})
	if err != nil {
		return result{}, fmt.Errorf("jslinecov: %w", err)
	}
	for name, b := range inv.Files {
		if err := os.WriteFile(filepath.Join(reports, name), b, 0o644); err != nil {
			return result{}, err
		}
	}
	args := append([]string{}, inv.Argv...)
	env := append(os.Environ(), o.Env...)
	for k, v := range inv.Environment {
		env = append(env, k+"="+v)
	}
	switch o.Runner {
	case "jest":
		args = append(args, "--coverage", "--coverageProvider=v8", "--coverageReporters=json", "--coverageDirectory="+cov)
	case "vitest":
		args = append(args, "--coverage.enabled", "--coverage.provider=v8", "--coverage.reporter=json", "--coverage.reportsDirectory="+cov)
	case "bun-test":
		args = append(args, "--coverage", "--coverage-reporter=lcov", "--coverage-dir="+cov)
	case "deno-test":
		args = append(args, "--coverage="+cov)
	default:
		env = append(env, "NODE_V8_COVERAGE="+cov)
	}
	args = append(args, extra...)
	switch o.Runner {
	case "jest":
		args = append(args, "--runTestsByPath", file)
	case "bun-test":
		args = append(args, "./"+file)
	default:
		args = append(args, file)
	}
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	rctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(rctx, o.Executable, args...)
	cmd.Dir, cmd.Env = root, env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	if ctx.Err() != nil {
		return result{}, ctx.Err()
	}
	if rctx.Err() != nil {
		return result{why: "timed out after " + timeout.String()}, nil
	}
	code := 0
	var exit *exec.ExitError
	switch {
	case errors.As(runErr, &exit):
		code = exit.ExitCode()
	case runErr != nil:
		return result{}, fmt.Errorf("jslinecov: run %s: %w", o.Runner, runErr)
	}
	res := result{lines: v8.Lines{}}
	if code != 0 {
		res.why = fmt.Sprintf("exit %d: %s", code, tail(stderr.Bytes()))
	}
	in := tr.Input{Runner: o.Runner, SourceRoot: root, Selectors: []string{file}, Reports: map[string][]byte{},
		Stdout: stdout.Bytes(), ExitCode: code, SuccessExitCodes: inv.SuccessExitCodes, FailureExitCodes: inv.FailureExitCodes}
	if o.Runner != "ava" {
		in.Stdout = nil
		names := append([]string{}, inv.ReportPaths...)
		for _, pat := range inv.ReportPatterns {
			m, _ := filepath.Glob(filepath.Join(reports, pat))
			for _, p := range m {
				names = append(names, filepath.Base(p))
			}
		}
		for _, n := range names {
			b, err := os.ReadFile(filepath.Join(reports, n))
			if err != nil {
				return result{why: "missing report " + n}, nil
			}
			in.Reports[n] = b
		}
	}
	obs, err := dynamic.Parse(in)
	if err != nil {
		res.why = "unreadable report: " + err.Error()
		return res, nil
	}
	if o.Runner == "jest" && extra != nil {
		// Jest marks a file whose tests a -t filter narrowed "focused"; the
		// per-test states still decide the run.
		kept := obs.Problems[:0]
		for _, p := range obs.Problems {
			if p.Code != "unknown-suite-state" || p.Detail != "focused" {
				kept = append(kept, p)
			}
		}
		obs.Problems = kept
	}
	res.observation = obs
	if code != 0 {
		return res, nil
	} // retain native failures for qualification, never coverage records
	if err := o.coverage(ctx, root, scope, cov, res.lines); err != nil {
		res.why = "unreadable coverage: " + err.Error()
	}
	return res, nil
}

// coverage reads the run's coverage in the runner's native form
// (LPCV-V0-965).
func (o Options) coverage(ctx context.Context, root string, scope v8.Scope, cov string, l v8.Lines) error {
	switch o.Runner {
	case "jest", "vitest":
		b, err := os.ReadFile(filepath.Join(cov, "coverage-final.json"))
		if err != nil {
			return err
		}
		return l.AddIstanbul(b, scope)
	case "bun-test":
		b, err := os.ReadFile(filepath.Join(cov, "lcov.info"))
		if err != nil {
			return err
		}
		return l.AddLCOV(b, root, scope)
	case "deno-test":
		cmd := exec.CommandContext(ctx, o.Executable, "coverage", "--lcov", cov)
		cmd.Dir = root
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		b, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("deno coverage: %v: %s", err, tail(stderr.Bytes()))
		}
		return l.AddLCOV(b, root, scope)
	}
	files, err := filepath.Glob(filepath.Join(cov, "coverage-*.json"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("no V8 coverage written")
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if err := l.AddV8(b, v8.Options{Scope: scope}); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(f), err)
		}
	}
	return nil
}

func tail(b []byte) string {
	b = bytes.TrimSpace(b)
	if len(b) > 400 {
		b = b[len(b)-400:]
	}
	return string(b)
}
