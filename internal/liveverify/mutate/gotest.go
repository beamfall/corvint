package mutate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// offlineHints are the go command's ways of saying it needs the network. The
// falsifier never downloads modules, so these are NOT_RUN, not a failed claim.
var offlineHints = []string{
	"cannot find module",
	"no required module provides",
	"module lookup disabled",
	"missing go.sum entry",
	"dial tcp",
	"proxyconnect",
	"connection refused",
	"no such host",
}

// inspectExport reports whether the export can carry a mutation run at all.
func inspectExport(exportDir string, configuration settings) (bool, Report) {
	if !regularFile(filepath.Join(exportDir, filepath.FromSlash(configuration.changedPath))) {
		return true, unsupportedReport("changed file absent at revision: " + configuration.changedPath)
	}
	if !regularFile(filepath.Join(exportDir, filepath.FromSlash(configuration.testPath))) {
		return true, unsupportedReport("test file absent at revision: " + configuration.testPath)
	}
	changedModule, inModule := enclosingModule(exportDir, configuration.changedPath)
	if !inModule {
		return true, unsupportedReport("not a Go module: " + configuration.changedPath)
	}
	if testModule, _ := enclosingModule(exportDir, configuration.testPath); testModule != changedModule {
		return true, unsupportedReport("test and changed file are in different Go modules: " + configuration.testPath)
	}
	return false, Report{}
}

func regularFile(name string) bool {
	information, err := os.Stat(name)
	return err == nil && information.Mode().IsRegular()
}

// enclosingModule walks from the file's directory up to the export root
// looking for the go.mod that would make go test meaningful, and names its
// directory. The runner runs one module: a test outside the changed file's
// module is unsupported.
func enclosingModule(exportDir, path string) (string, bool) {
	directory := filepath.Dir(filepath.Join(exportDir, filepath.FromSlash(path)))
	for {
		if regularFile(filepath.Join(directory, "go.mod")) {
			return directory, true
		}
		if directory == exportDir || !strings.HasPrefix(directory, exportDir) {
			return "", false
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", false
		}
		directory = parent
	}
}

// testFunctionNames lists the Test functions declared by the claimed test file,
// in source order. Only these names are ever run: the claim is about this file.
func testFunctionNames(testFile string) ([]string, error) {
	source, err := os.ReadFile(testFile)
	if err != nil {
		return nil, err
	}
	file, err := parser.ParseFile(token.NewFileSet(), "test.go", source, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(file.Decls))
	for _, declaration := range file.Decls {
		function, isFunction := declaration.(*ast.FuncDecl)
		if !isFunction || function.Recv != nil || !isTestName(function.Name.Name) {
			continue
		}
		names = append(names, function.Name.Name)
	}
	return names, nil
}

func isTestName(name string) bool {
	if !strings.HasPrefix(name, "Test") || name == "TestMain" {
		return false
	}
	suffix := strings.TrimPrefix(name, "Test")
	return suffix == "" || !(suffix[0] >= 'a' && suffix[0] <= 'z')
}

// runPattern anchors the go test -run selector on exactly the claimed tests.
func runPattern(names []string) string {
	return "^(" + strings.Join(names, "|") + ")$"
}

// runOutcome classifies one go test run. Only a run that built and then
// failed is a behavioural kill; a run that never built says nothing about the
// test.
type runOutcome int

const (
	runPassed runOutcome = iota
	runFailed
	runUnbuildable
	// runBroken is a run that neither passed nor reported a test failure: a
	// sandbox that did not launch, a kill from outside, a toolchain fault.
	// It is never a verdict about the mutant.
	runBroken
	// runUnattributed is a package run that gave one of a claim's functions
	// no outcome (the binary died in another test, the output was cut): the
	// claim is re-run alone rather than credited from the group.
	runUnattributed
)

// runGrace is how much longer than go test's own -timeout the runner waits
// before killing the run, so a hung test is reported by go test as a failure
// rather than silenced by the runner.
const runGrace = 5 * time.Second

// observeRun, when set, sees the package of every go test the runner
// launches; the package's tests count invocations through it.
var observeRun func(packageDir string)

// runTests runs the claimed tests inside the sandbox against whatever is
// currently in the export and reports the classified outcome with the bounded
// output.
func runTests(ctx context.Context, configuration settings, space workspace, pattern string) (runOutcome, string) {
	// Baselines and grouped fallbacks must use the mutants' JSON mode too:
	// -json implies -v and can change testing.Verbose() inside the cited tests.
	outcome, output, _ := runGoTest(ctx, configuration.perRun, space, configuration.packageDir, pattern, true)
	return outcome, output
}

// runGoTest runs one go test of packageDir's tests matching pattern inside the
// sandbox. Scratch is emptied first; the run's process group is killed
// afterwards whatever happened, so nothing a cited test started outlives the
// judgment. An attributable run emits -json events, read as they stream by
// an eventSink that also returns each top-level function's own result; a
// plain run is classified from its bounded output and returns no outcomes.
func runGoTest(ctx context.Context, timeout time.Duration, space workspace, packageDir, pattern string, attributable bool) (runOutcome, string, map[string]runOutcome) {
	if err := resetScratch(space); err != nil {
		return runBroken, err.Error(), nil
	}
	if observeRun != nil {
		observeRun(packageDir)
	}
	commandCtx, cancel := context.WithTimeout(ctx, timeout+runGrace)
	defer cancel()
	argv := append(append([]string{}, space.prefix...), "go", "test", "-count=1")
	if attributable {
		argv = append(argv, "-json")
	}
	argv = append(argv, "-timeout", timeout.String(), "-run", pattern, packageDir)
	command := exec.CommandContext(commandCtx, argv[0], argv[1:]...)
	command.Dir = space.export
	command.Env = goEnvironment(space)
	output := &boundedBuffer{limit: outputLimit}
	sink := newEventSink(output)
	command.Stdout, command.Stderr = output, output
	if attributable {
		command.Stdout, command.Stderr = sink, sink
	}
	configureProcess(command)
	err := command.Run()
	if command.Process != nil {
		terminateProcessGroup(command.Process.Pid)
	}
	if attributable {
		return sink.classify(err), output.String(), sink.outcomes
	}
	return classifyRun(err, output.String()), output.String(), nil
}

// eventLineLimit bounds one -json line held for decoding; a longer line is
// no event and is dropped up to its newline.
const eventLineLimit = 1 << 20

// eventSink reads a -json run as it streams: every line is decoded once and
// only what the judgment needs is kept (each top-level function's final
// result and the package's build and failure markers), while the raw bytes
// also fill the bounded buffer for detail. -json implies -v, so a package's
// stream can outgrow that buffer; the cut never loses an outcome.
type eventSink struct {
	raw         *boundedBuffer
	pending     []byte
	skipping    bool
	outcomes    map[string]runOutcome
	unbuildable bool
	failed      bool
}

// testEvent is the part of a go test -json event the sink reads.
type testEvent struct {
	Action, Test, Output string
}

var outcomeByAction = map[string]runOutcome{"pass": runPassed, "skip": runPassed, "fail": runFailed}

func newEventSink(raw *boundedBuffer) *eventSink {
	return &eventSink{raw: raw, outcomes: map[string]runOutcome{}}
}

func (sink *eventSink) Write(chunk []byte) (int, error) {
	_, _ = sink.raw.Write(chunk)
	sink.pending = append(sink.pending, chunk...)
	for newline := bytes.IndexByte(sink.pending, '\n'); newline >= 0; newline = bytes.IndexByte(sink.pending, '\n') {
		if !sink.skipping {
			sink.readLine(sink.pending[:newline])
		}
		sink.skipping = false
		sink.pending = sink.pending[newline+1:]
	}
	if len(sink.pending) > eventLineLimit {
		sink.pending = sink.pending[:0]
		sink.skipping = true
	}
	return len(chunk), nil
}

// readLine records one event: pass or skip is a pass, fail is a failure, a
// subtest names nothing, and a line that is no event (a sandbox message, a
// cut line) is skipped. Package-level events and output carry the markers
// classifyRun reads from plain output.
func (sink *eventSink) readLine(line []byte) {
	var event testEvent
	if err := json.Unmarshal(line, &event); err != nil {
		return
	}
	sink.readMarkers(event.Action, event.Output)
	if event.Test == "" || strings.Contains(event.Test, "/") {
		return
	}
	result, known := outcomeByAction[event.Action]
	if !known {
		return
	}
	sink.outcomes[event.Test] = result
}

func (sink *eventSink) readMarkers(action, output string) {
	if action == "build-fail" || strings.Contains(output, "[build failed]") || strings.Contains(output, "[setup failed]") {
		sink.unbuildable = true
	}
	if reportsTestFailure(output) {
		sink.failed = true
	}
}

// classify is classifyRun over the streamed events: a build that never ran
// the tests, then any function or package failure, then a run without a
// verdict.
func (sink *eventSink) classify(err error) runOutcome {
	if err == nil {
		return runPassed
	}
	if sink.unbuildable {
		return runUnbuildable
	}
	if sink.failed || sink.anyFailed() {
		return runFailed
	}
	return runBroken
}

func (sink *eventSink) anyFailed() bool {
	for _, result := range sink.outcomes {
		if result == runFailed {
			return true
		}
	}
	return false
}

// classifyRun separates a red test from a build that never ran the test (go
// test marks the latter "[build failed]" or "[setup failed]" on its FAIL
// line) and both from a run that produced no test verdict at all. Only
// go test's own failure report counts as a failure.
func classifyRun(err error, output string) runOutcome {
	if err == nil {
		return runPassed
	}
	if strings.Contains(output, "[build failed]") || strings.Contains(output, "[setup failed]") {
		return runUnbuildable
	}
	if reportsTestFailure(output) {
		return runFailed
	}
	return runBroken
}

func reportsTestFailure(output string) bool {
	return strings.Contains(output, "--- FAIL") ||
		strings.HasPrefix(output, "FAIL\t") ||
		strings.Contains(output, "\nFAIL\t") ||
		strings.Contains(output, `"FAIL\t`)
}

// inconclusive turns a broken run into the infrastructure error it is without
// exposing repository-controlled test output.
func inconclusive(_ string) error {
	return fmt.Errorf("mutate: run ended without a verdict")
}

// goEnvironment pins the toolchain, forbids module downloads, and keeps the
// build hermetic: the host's go env file is ignored, temporary files and the
// build cache land in the workspace, and the host module cache is only read.
func goEnvironment(space workspace) []string {
	environment := passthroughEnvironment([]string{"PATH", "HOME", "SystemRoot", "USERPROFILE", "GOROOT"})
	return append(environment,
		"TMPDIR="+space.scratch, "TEMP="+space.scratch, "TMP="+space.scratch,
		"GOTMPDIR="+space.scratch, "GOCACHE="+space.cache, "GOMODCACHE="+space.moduleCache,
		"GOENV=off", "GOTOOLCHAIN=local", "GOWORK=off", "CGO_ENABLED=0",
		"GOFLAGS=-mod=readonly", "GOPROXY=off", "GOSUMDB=off",
		"LANG=C", "LC_ALL=C",
	)
}

// baselineDetail names why the unmodified export could not establish a baseline.
func baselineDetail(output string, configuration settings) string {
	lowered := strings.ToLower(output)
	for _, hint := range offlineHints {
		if strings.Contains(lowered, hint) {
			return "module dependencies unavailable offline"
		}
	}
	if strings.Contains(output, "[build failed]") || strings.Contains(lowered, "build constraints exclude") {
		return "baseline tests do not compile: " + configuration.testPath
	}
	return "baseline tests fail before mutation: " + configuration.testPath
}
