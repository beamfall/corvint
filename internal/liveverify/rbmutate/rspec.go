package rbmutate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/mutate"
)

// reportName is RSpec's JSON report inside the scratch directory, which the
// export empties before every run.
const reportName = "rspec.json"

// offlineHints are Bundler's and RubyGems' ways of saying a gem is missing.
// The falsifier never installs anything, so these are NOT_RUN, not a red
// test.
var offlineHints = []string{
	"bundler::gemnotfound",
	"could not find gem",
	"could not find compatible versions",
	"gem::missingspecerror",
	"gem::loaderror",
	"bundler::gemfilenotfound",
}

// runOutcome classifies one RSpec run. Only a run that loaded the spec file,
// ran examples, and reported a failure is a behavioural kill.
type runOutcome int

const (
	runPassed runOutcome = iota
	runFailed
	// runUnloadable is a run whose examples never all ran: the mutant or
	// the spec file failed to load, no example ran, RSpec wrote no report,
	// or RSpec exited non-zero with every example passing.
	runUnloadable
	// runTimedOut is a run the per-run timeout ended.
	runTimedOut
	// runBroken is a run with no exit status at all: a sandbox that did
	// not launch.
	runBroken
	// runUnadmitted is a containerised run that would have passed or
	// failed, but whose report the runner's own parser rejected or
	// observed as incomplete (FPK-V0-080); it earns no verdict.
	runUnadmitted
)

// summary is the part of RSpec's JSON formatter output the receipt keeps.
type summary struct {
	Summary struct {
		ExampleCount                 int `json:"example_count"`
		FailureCount                 int `json:"failure_count"`
		PendingCount                 int `json:"pending_count"`
		ErrorsOutsideOfExamplesCount int `json:"errors_outside_of_examples_count"`
	} `json:"summary"`
	Examples []struct {
		ID        string `json:"id"`
		Status    string `json:"status"`
		Exception *struct {
			Class string `json:"class"`
		} `json:"exception"`
	} `json:"examples"`
}

// scriptErrors are Ruby's ScriptError family. An example that fails with one
// failed to load code (a mutant that no longer parses, required lazily), not
// on behaviour.
var scriptErrors = map[string]bool{"ScriptError": true, "SyntaxError": true, "LoadError": true, "NotImplementedError": true, "Gem::LoadError": true, "Gem::MissingSpecError": true}

// optionsName is the options file, in the export's root, that replaces
// RSpec's own options-file lookup: the revision's `.rspec` and `.rspec-local`
// only, never the host's `~/.rspec` or `$XDG_CONFIG_HOME/rspec/options`.
const optionsName = ".corvint-rspec-options"

type specRun struct {
	outcome  runOutcome
	receipt  Receipt
	output   string
	reported bool
}

// rspecRunner is one resolved RSpec command on one export.
type rspecRunner struct {
	argv          []string
	copy          Copy
	configuration settings
	options       string
}

// name is the runner's argv prefix with absolute paths cut to base names,
// free of host paths.
func (runner rspecRunner) name() string {
	if len(runner.argv) == 0 {
		return ""
	}
	words := make([]string, 0, len(runner.argv))
	for _, word := range runner.argv {
		if filepath.IsAbs(word) {
			word = filepath.Base(word)
		}
		words = append(words, word)
	}
	return strings.Join(words, " ")
}

// The runner-stage causes a host RSpec probe names in its detail and in
// Report.Cause, so a reader can tell a missing toolchain from a sandbox that
// refused a present one (TCQ-V0-094).
const (
	// CauseRunnerNotFound: rspec, or the interpreter it needs, is absent:
	// not on PATH, not startable, or exit status 126 or 127 inside the sandbox.
	CauseRunnerNotFound = "runner-not-found"
	// CauseBundleUnavailable: the Gemfile route cannot run: bundle is absent,
	// `bundle exec rspec --version` fails, or its gems are not installed.
	CauseBundleUnavailable = "bundle-unavailable"
	// CauseRunnerUnavailable: a present rspec exits non-zero for --version.
	CauseRunnerUnavailable = "runner-unavailable"
)

// probeDetail is a runner-stage detail: the fixed FPK-V0-069 text, then the
// cause, the probed command by base names and its exit status.
func probeDetail(text, cause, probed, status string) string {
	return fmt.Sprintf("%s (%s: probed `%s --version`, exit status %s)", text, cause, probed, status)
}

// runnerCause reads the cause probeDetail wrote, or "" for any other detail.
func runnerCause(detail string) string {
	for _, cause := range []string{CauseRunnerNotFound, CauseBundleUnavailable, CauseRunnerUnavailable} {
		if strings.Contains(detail, " ("+cause+": probed `") {
			return cause
		}
	}
	return ""
}

// resolveRunner chooses the RSpec command on the host PATH without running
// anything: the declared argv, else `bundle exec rspec` beside a Gemfile,
// else `rspec`. A command the PATH lacks is the fixed probe detail naming its
// cause, which needs no sandbox to establish (TCQ-V0-094).
func resolveRunner(copy Copy, configuration settings) (rspecRunner, bool, string) {
	runner := rspecRunner{copy: copy, configuration: configuration}
	switch {
	case len(configuration.rspec) != 0:
		runner.argv = configuration.rspec
		return runner, false, ""
	case regularFile(filepath.Join(copy.Dir(), "Gemfile")):
		bundle, err := exec.LookPath("bundle")
		if err != nil {
			return rspecRunner{argv: []string{"bundle", "exec", "rspec"}}, false,
				probeDetail("bundle is not available", CauseBundleUnavailable, "bundle exec rspec", "none: bundle is not on PATH")
		}
		runner.argv = []string{bundle, "exec", "rspec"}
		return runner, true, ""
	default:
		rspec, err := exec.LookPath("rspec")
		if err != nil {
			return rspecRunner{argv: []string{"rspec"}}, false,
				probeDetail("rspec is not available", CauseRunnerNotFound, "rspec", "none: rspec is not on PATH")
		}
		runner.argv = []string{rspec}
		return runner, false, ""
	}
}

// findRunner resolves the RSpec command and proves, inside the sandbox, that
// it starts. Either lack is Unsupported with a fixed detail that names the
// cause, the probed command and its exit status.
func findRunner(ctx context.Context, copy Copy, configuration settings) (rspecRunner, string) {
	runner, bundled, unresolved := resolveRunner(copy, configuration)
	if unresolved != "" {
		return runner, unresolved
	}
	output, timedOut, err := runner.run(ctx, "--version")
	switch {
	case err == nil:
		options, err := writeOptions(copy.Dir())
		if err != nil {
			return runner, "cannot write the rspec options file"
		}
		runner.options = options
		return runner, ""
	case timedOut:
		return runner, "rspec does not start within the per-run timeout"
	}
	var exit *exec.ExitError
	status, cause := "none: the command did not start", CauseRunnerNotFound
	if errors.As(err, &exit) {
		status = strconv.Itoa(exit.ExitCode())
		switch {
		case exit.ExitCode() == 126 || exit.ExitCode() == 127:
		case bundled:
			cause = CauseBundleUnavailable
		default:
			cause = CauseRunnerUnavailable
		}
	}
	if containsAny(strings.ToLower(output), offlineHints) {
		return runner, probeDetail("gem dependencies unavailable offline", CauseBundleUnavailable, runner.name(), status)
	}
	return runner, probeDetail("rspec cannot run inside the sandbox", cause, runner.name(), status)
}

// writeOptions writes the options file every spec run names with
// `--options`, which makes RSpec skip the global, project and local files it
// would otherwise read: the revision's own `.rspec` and `.rspec-local` are
// copied in, the host's files never are.
func writeOptions(dir string) (string, error) {
	var content []byte
	for _, name := range []string{".rspec", ".rspec-local"} {
		path := filepath.Join(dir, name)
		if !regularFile(path) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		content = append(append(content, data...), '\n')
	}
	target := filepath.Join(dir, optionsName)
	return target, os.WriteFile(target, content, 0o644)
}

func (runner rspecRunner) run(ctx context.Context, arguments ...string) (string, bool, error) {
	argv := append(append([]string(nil), runner.argv...), arguments...)
	return runner.copy.Run(ctx, runner.configuration.perRun, runner.copy.Dir(), runner.environment(), argv...)
}

// runSpec runs the cited spec file against whatever is in the export now;
// source is the changed file's content the run sees, for the receipt.
func (runner rspecRunner) runSpec(ctx context.Context, source []byte) specRun {
	reportPath := filepath.Join(runner.copy.Scratch(), reportName)
	output, timedOut, err := runner.run(ctx, "--options", runner.options, "--format", "json", "--out", reportPath, "--no-color", runner.configuration.target)
	content, readErr := os.ReadFile(reportPath)
	return classifyRun(err, timedOut, output, content, readErr == nil, digest(source))
}

// classifyRun separates a red example from a run whose examples never all
// ran, and both from a run that produced no exit status. RSpec's own report
// decides; the exit status only confirms it.
func classifyRun(err error, timedOut bool, output string, content []byte, haveReport bool, sourceDigest string) specRun {
	status, started := 0, true
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		status = exit.ExitCode()
	default:
		started = false
	}
	return classifyReport(status, started, timedOut, output, content, haveReport, sourceDigest)
}

// classifyReport is classifyRun after the exit status is known; the
// container runner (FPK-V0-079) reaches it from the runner receipt's input.
func classifyReport(status int, started, timedOut bool, output string, content []byte, haveReport bool, sourceDigest string) specRun {
	run := specRun{output: output, receipt: Receipt{SourceSHA256: sourceDigest}}
	if timedOut {
		run.outcome, run.receipt.ExitStatus, run.receipt.TimedOut = runTimedOut, -1, true
		return run
	}
	if !started {
		run.outcome, run.receipt.ExitStatus = runBroken, -1
		return run
	}
	run.receipt.ExitStatus = status
	var parsed summary
	if !haveReport || json.Unmarshal(content, &parsed) != nil {
		run.outcome = runUnloadable
		return run
	}
	run.reported = true
	run.receipt.Examples = parsed.Summary.ExampleCount
	run.receipt.Failures = parsed.Summary.FailureCount
	run.receipt.Pending = parsed.Summary.PendingCount
	run.receipt.ErrorsOutsideExamples = parsed.Summary.ErrorsOutsideOfExamplesCount
	run.receipt.FailedExamples = failedExamples(parsed)
	switch {
	case run.receipt.ErrorsOutsideExamples > 0 || run.receipt.Examples == 0 || loadFailure(parsed):
		run.outcome = runUnloadable
	case run.receipt.Failures > 0 && run.receipt.ExitStatus != 0:
		run.outcome = runFailed
	case run.receipt.Failures == 0 && run.receipt.ExitStatus == 0:
		run.outcome = runPassed
	default:
		run.outcome = runUnloadable
	}
	return run
}

// loadFailure reports whether a failed example failed with a ScriptError:
// code that did not load, which is never a behavioural kill.
func loadFailure(parsed summary) bool {
	for _, example := range parsed.Examples {
		if example.Status == "failed" && example.Exception != nil && scriptErrors[example.Exception.Class] {
			return true
		}
	}
	return false
}

// failedExamples is the sorted, bounded list of failed example ids.
func failedExamples(parsed summary) []string {
	var failed []string
	for _, example := range parsed.Examples {
		if example.Status == "failed" && example.ID != "" {
			failed = append(failed, example.ID)
		}
	}
	sort.Strings(failed)
	if len(failed) > failedExampleBound {
		failed = failed[:failedExampleBound]
	}
	return failed
}

// environment keeps the run hermetic: temporary files and Bundler's home in
// the scratch directory, no host RSpec options, UTF-8 sources, and no Rails
// preloader.
func (runner rspecRunner) environment() []string {
	scratch := runner.copy.Scratch()
	environment := mutate.PassthroughEnvironment([]string{"PATH", "HOME", "GEM_HOME", "GEM_PATH", "BUNDLE_PATH", "SystemRoot", "USERPROFILE"})
	return append(environment,
		"TMPDIR="+scratch, "TEMP="+scratch, "TMP="+scratch,
		"BUNDLE_USER_HOME="+filepath.Join(scratch, "bundle"),
		"SPEC_OPTS=", "RUBYOPT=-EUTF-8", "DISABLE_SPRING=1",
		"LANG=C", "LC_ALL=C",
	)
}

// baselineDetail names why the unmodified export could not establish a
// baseline.
func baselineDetail(run specRun, configuration settings) string {
	switch {
	case run.outcome == runUnadmitted:
		return "the runner did not admit the baseline RSpec report: " + configuration.target
	case run.outcome == runTimedOut:
		return "baseline examples exceed the per-run timeout: " + configuration.target
	case run.outcome == runBroken:
		return "rspec cannot run inside the sandbox"
	case containsAny(strings.ToLower(run.output), offlineHints):
		return "gem dependencies unavailable offline"
	case !run.reported:
		return "rspec wrote no report: " + configuration.target
	case run.receipt.ErrorsOutsideExamples > 0:
		return "baseline examples do not load: " + configuration.target
	case run.receipt.Examples == 0:
		return "spec file runs no examples: " + configuration.target
	case run.receipt.Failures > 0:
		return "baseline examples fail before mutation: " + configuration.target
	}
	return "baseline exits non-zero with every example passing: " + configuration.target
}

func containsAny(text string, hints []string) bool {
	for _, hint := range hints {
		if strings.Contains(text, hint) {
			return true
		}
	}
	return false
}
