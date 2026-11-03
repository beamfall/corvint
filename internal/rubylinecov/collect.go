// SPDX-License-Identifier: Apache-2.0

// Package rubylinecov is the Ruby coverage sub-plugin of the ruby language
// plugin: the per-test line-coverage collector of
// docs/specs/live-proof-carrying-verification-v0.md for RSpec, Minitest,
// test-unit and Rails (LPCV-V0-950, LPCV-V0-954, LPCV-V0-955), kept in the
// internal/linecov index format, and the analysis that maps a Ruby change to
// the tests it can reach (LPCV-V0-952, LPCV-V0-953).
//
// One suite run collects every test: linecov.rb, loaded through RUBYOPT, takes
// Coverage.result(stop: false, clear: true) around each test, so the counters
// each test sees are its own, and every Ruby subprocess the test starts dumps
// its coverage under that test.
package rubylinecov

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/linecov"
)

//go:embed linecov.rb
var script []byte

// Load names the pseudo-test holding the coverage no test window owns:
// loading files, before(:all) hooks, and the windows of tests that required a
// file. A change reaching it keeps the static plan (LPCV-V0-953).
const Load = "(load)"

// Options configures one suite run.
type Options struct {
	// Root is the clean checkout the suite runs in; the index binds to its HEAD.
	Root string
	// Work is a host directory for the collector script and its dumps. A run
	// through a container needs it inside a mounted directory.
	Work string
	// Argv runs the whole suite in Root: {"bundle", "exec", "rspec"},
	// {"ruby", "-Itest", "-e", "Dir['test/**/*_test.rb'].each { |f| require File.expand_path(f) }"},
	// or a Rails app's {"bin/rails", "test"}. It must run tests serially in
	// one process (PARALLEL_WORKERS=1 for Rails).
	Argv []string
	// Prefix, when set, returns the argv prefix that runs Argv in a process
	// that does not inherit this environment, such as `docker run -e K=V ...
	// image`; env is the collector environment it must pass through. PathIn
	// maps a host path to the path that process sees.
	Prefix func(env []string) []string
	PathIn func(host string) string
	// Timeout bounds the suite run; zero means no bound beyond ctx.
	Timeout time.Duration
}

// Outcome is one test the run reported, with the reason it has no record
// when it has none (LPCV-V0-962).
type Outcome struct {
	Package, Name string
	Status        string // passed, failed or pending
	Recorded      bool
	Reason        string
}

// Run is one completed suite run.
type Run struct {
	Runner   string
	Outcomes []Outcome
	records  []record // passing tests with complete coverage, and Load
	files    map[string]fileShape
	exit     int
}

// ExitCode is the suite process's exit status; failing tests make it nonzero
// without voiding the passing tests' records.
func (r *Run) ExitCode() int { return r.exit }

type span [4]int32 // start line, start column, end line, end column; columns 0-based

type fileShape struct {
	Lines   []int32 `json:"lines"`
	Methods []span  `json:"methods"`
}

type record struct {
	pkg, name string
	elapsed   time.Duration
	lines     map[string][]int32
	methods   map[string][]span
}

type event struct {
	Kind      string               `json:"kind"`
	Ruby      string               `json:"ruby"`
	Test      string               `json:"test"`
	Seq       int                  `json:"seq"`
	Package   string               `json:"package"`
	Name      string               `json:"name"`
	Version   string               `json:"version"`
	Status    string               `json:"status"`
	Loaded    bool                 `json:"loaded"`
	ElapsedNS int64                `json:"elapsed_ns"`
	Lines     map[string][]int32   `json:"lines"`
	Methods   map[string][]span    `json:"methods"`
	Files     map[string]fileShape `json:"files"`
}

// Collect runs the suite once with the collector loaded. A run whose suite
// process wrote no completion marker (a crash, a timeout, a kill) publishes
// nothing, and a test whose subprocess dump is incomplete has no record
// (LPCV-V0-954, LPCV-V0-962).
func Collect(ctx context.Context, o Options) (*Run, error) {
	if len(o.Argv) == 0 {
		return nil, errors.New("rubylinecov: no suite command")
	}
	// The index binds to blobs at HEAD, so a modified tracked file would bind
	// coverage of other content (LPCV-V0-954).
	status, err := exec.CommandContext(ctx, "git", "-C", o.Root, "status", "--porcelain", "--untracked-files=no").Output()
	if err != nil {
		return nil, fmt.Errorf("rubylinecov: git status: %w", err)
	}
	if len(bytes.TrimSpace(status)) != 0 {
		return nil, errors.New("rubylinecov: the checkout has modified tracked files")
	}
	// The suite runs in Root, so a relative Work must not resolve against it.
	work, err := filepath.Abs(o.Work)
	if err != nil {
		return nil, err
	}
	o.Work = work
	pathIn := o.PathIn
	if pathIn == nil {
		pathIn = func(p string) string { return p }
	}
	dumps := filepath.Join(o.Work, "dumps")
	if err := os.RemoveAll(dumps); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dumps, 0o755); err != nil {
		return nil, err
	}
	rb := filepath.Join(o.Work, "corvint_linecov.rb")
	if err := os.WriteFile(rb, script, 0o644); err != nil {
		return nil, err
	}
	rubyopt := "-r" + pathIn(rb)
	if o.Prefix == nil { // keep the caller's own options for a process that inherits them
		rubyopt = strings.TrimSpace(rubyopt + " " + os.Getenv("RUBYOPT"))
	}
	env := []string{
		"RUBYOPT=" + rubyopt,
		"CORVINT_LINECOV_DIR=" + pathIn(dumps),
		"CORVINT_LINECOV_ROOT=" + pathIn(o.Root),
	}
	if o.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.Timeout)
		defer cancel()
	}
	argv := o.Argv
	if o.Prefix != nil {
		argv = append(o.Prefix(env), argv...)
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = o.Root
	if o.Prefix == nil {
		cmd.Env = append(withoutCollectorEnv(os.Environ()), env...)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("rubylinecov: suite run: %w", ctx.Err())
	}
	exit := 0
	var exitErr *exec.ExitError
	switch {
	case errors.As(err, &exitErr):
		exit = exitErr.ExitCode()
	case err != nil:
		return nil, fmt.Errorf("rubylinecov: suite run: %w", err)
	}
	run, err := read(dumps)
	if err != nil {
		return nil, fmt.Errorf("%w (exit %d): %s", err, exit, tail(stderr.Bytes()))
	}
	run.exit = exit
	return run, nil
}

// withoutCollectorEnv drops inherited collector settings, so a collection
// started inside another never joins it.
func withoutCollectorEnv(env []string) []string {
	out := env[:0:0]
	for _, kv := range env {
		if !strings.HasPrefix(kv, "CORVINT_LINECOV_") && !strings.HasPrefix(kv, "RUBYOPT=") {
			out = append(out, kv)
		}
	}
	return out
}

func tail(b []byte) string {
	if len(b) > 2000 {
		b = b[len(b)-2000:]
	}
	return strings.TrimSpace(string(b))
}

// dump is one process's events.
type dump struct {
	start, framework *event
	tests            []event
	child, load      []event
	files            map[string]fileShape
	done             bool
}

func readDump(path string) (*dump, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	d := &dump{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<30)
	for sc.Scan() {
		var e event
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			return nil, fmt.Errorf("rubylinecov: %s: %w", filepath.Base(path), err)
		}
		switch e.Kind {
		case "start":
			d.start = &e
		case "framework":
			d.framework = &e
		case "test":
			d.tests = append(d.tests, e)
		case "load":
			d.load = append(d.load, e)
		case "child":
			d.child = append(d.child, e)
		case "files":
			d.files = e.Files
		case "done":
			d.done = true
		default:
			return nil, fmt.Errorf("rubylinecov: %s: unknown event %q", filepath.Base(path), e.Kind)
		}
	}
	return d, sc.Err()
}

// read joins the suite dump and every subprocess dump of one run.
func read(dir string) (*Run, error) {
	suite, err := readDump(filepath.Join(dir, "suite.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("rubylinecov: no suite dump: %w", err)
	}
	if !suite.done || suite.start == nil {
		return nil, errors.New("rubylinecov: the suite process wrote no completion marker")
	}
	if suite.framework == nil || suite.framework.Name == "" {
		return nil, errors.New("rubylinecov: no RSpec, Minitest or test-unit test ran")
	}
	run := &Run{
		Runner: fmt.Sprintf("ruby %s %s %s coverage=lines,methods", suite.start.Ruby, suite.framework.Name, suite.framework.Version),
		files:  map[string]fileShape{},
	}
	mergeShapes(run.files, suite.files)
	load := record{pkg: Load, name: Load, lines: map[string][]int32{}, methods: map[string][]span{}}
	for _, e := range suite.load {
		load.add(e)
	}
	bySeq := map[string]*record{}
	incomplete := map[string]string{}
	var seqs []string
	var order []event
	for _, e := range suite.tests {
		order = append(order, e)
		if e.Status != "passed" {
			continue
		}
		r := &record{pkg: e.Package, name: e.Name, elapsed: time.Duration(e.ElapsedNS), lines: map[string][]int32{}, methods: map[string][]span{}}
		r.add(e)
		if e.Loaded {
			load.add(e)
		}
		bySeq[strconv.Itoa(e.Seq)] = r
		seqs = append(seqs, strconv.Itoa(e.Seq))
	}
	children, err := filepath.Glob(filepath.Join(dir, "child-*.jsonl"))
	if err != nil {
		return nil, err
	}
	sort.Strings(children)
	for _, path := range children {
		d, err := readDump(path)
		if err != nil {
			return nil, err
		}
		test := ""
		if d.start != nil {
			test = d.start.Test
		}
		if !d.done || d.start == nil {
			incomplete[test] = "a subprocess wrote no completion marker"
			continue
		}
		if d.start.Ruby != suite.start.Ruby {
			incomplete[test] = "a subprocess ran Ruby " + d.start.Ruby
		}
		mergeShapes(run.files, d.files)
		for _, e := range d.child {
			target := bySeq[e.Test]
			if e.Test == "load" || e.Test == "" {
				target = &load
			}
			if target != nil {
				target.add(e)
			}
		}
	}
	for _, e := range order {
		o := Outcome{Package: e.Package, Name: e.Name, Status: e.Status}
		seq := strconv.Itoa(e.Seq)
		switch {
		case e.Status != "passed":
			o.Reason = "the test did not pass"
		case incomplete[seq] != "":
			o.Reason = incomplete[seq]
			delete(bySeq, seq)
		default:
			o.Recorded = true
		}
		run.Outcomes = append(run.Outcomes, o)
	}
	if reason := incomplete["load"] + incomplete[""]; reason != "" {
		return nil, errors.New("rubylinecov: coverage outside every test is incomplete: " + reason)
	}
	for _, seq := range seqs {
		if r := bySeq[seq]; r != nil {
			run.records = append(run.records, *r)
		}
	}
	run.records = append(run.records, load)
	return run, nil
}

func (r *record) add(e event) {
	for p, lines := range e.Lines {
		r.lines[p] = append(r.lines[p], lines...)
	}
	for p, ms := range e.Methods {
		r.methods[p] = append(r.methods[p], ms...)
	}
}

func mergeShapes(into, from map[string]fileShape) {
	for p, s := range from {
		cur := into[p]
		cur.Lines = append(cur.Lines, s.Lines...)
		cur.Methods = append(cur.Methods, s.Methods...)
		into[p] = cur
	}
}

// Index builds the linecov index of a run collected at commit. oids maps every
// path at commit to its blob OID; a covered file absent from it (untracked,
// vendored outside Git) is not instrumented content and is left out. Each
// executable line is one block spanning its line with zero columns; each
// method is one block from its def to its end with 1-based columns, executed
// by the tests that called it.
func Index(module, commit string, run *Run, oids map[string]string) (*linecov.Index, error) {
	if module == "" {
		module = "."
	}
	ix := &linecov.Index{Format: linecov.Format, Module: module, Commit: commit, Runner: run.Runner}
	for _, r := range run.records {
		ix.Tests = append(ix.Tests, linecov.Test{Package: r.pkg, Name: r.name, Elapsed: r.elapsed})
	}
	// Every test file is unordered, so it runs whole when it is planned at
	// all: the collector has only the in-order run, and a test can depend on
	// state an earlier test built (a memoized value, a reused subprocess)
	// without running the code that built it, so no record yet supports
	// narrowing or excluding a file. The isolated pass of LPCV-V0-962 is what
	// lifts this (V1-4415).
	unordered := map[string]bool{}
	for _, o := range run.Outcomes {
		if !unordered[o.Package] {
			unordered[o.Package] = true
			ix.Unordered = append(ix.Unordered, o.Package)
		}
	}
	sort.Strings(ix.Unordered)
	paths := make([]string, 0, len(run.files))
	for p := range run.files {
		if oids[p] != "" {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	for _, p := range paths {
		shape := run.files[p]
		lines := map[int32]map[int32]bool{}
		for _, l := range shape.Lines {
			lines[l] = map[int32]bool{}
		}
		methods := map[span]map[int32]bool{}
		for _, m := range shape.Methods {
			methods[m] = map[int32]bool{}
		}
		for id, r := range run.records {
			for _, l := range r.lines[p] {
				if lines[l] == nil {
					lines[l] = map[int32]bool{}
				}
				lines[l][int32(id)] = true
			}
			for _, m := range r.methods[p] {
				if methods[m] == nil {
					methods[m] = map[int32]bool{}
				}
				methods[m][int32(id)] = true
			}
		}
		f := linecov.File{Path: p, OID: oids[p]}
		for l, set := range lines {
			f.Blocks = append(f.Blocks, linecov.Block{Start: l, End: l, Tests: ids(set)})
		}
		for m, set := range methods {
			f.Blocks = append(f.Blocks, linecov.Block{Start: m[0], StartCol: m[1] + 1, End: m[2], EndCol: m[3] + 1, Tests: ids(set)})
		}
		ix.Files = append(ix.Files, f)
	}
	// Merge of one part sorts the blocks and builds the lookups.
	return linecov.Merge(ix)
}

func ids(set map[int32]bool) []int32 {
	out := make([]int32, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
