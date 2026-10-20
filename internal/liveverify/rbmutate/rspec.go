package rbmutate

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"examples"`
}

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

// findRunner resolves the RSpec command and proves, inside the sandbox, that
// it starts. Either lack is Unsupported with a fixed detail.
func findRunner(ctx context.Context, copy Copy, configuration settings) (rspecRunner, string) {
	runner := rspecRunner{copy: copy, configuration: configuration}
	switch {
	case len(configuration.rspec) != 0:
		runner.argv = configuration.rspec
	case regularFile(filepath.Join(copy.Dir(), "Gemfile")):
		bundle, err := exec.LookPath("bundle")
		if err != nil {
			return rspecRunner{argv: []string{"bundle", "exec", "rspec"}}, "bundle is not available"
		}
		runner.argv = []string{bundle, "exec", "rspec"}
	default:
		rspec, err := exec.LookPath("rspec")
		if err != nil {
			return rspecRunner{argv: []string{"rspec"}}, "rspec is not available"
		}
		runner.argv = []string{rspec}
	}
	output, timedOut, err := runner.run(ctx, "--version")
	switch {
	case err == nil:
		return runner, ""
	case timedOut:
		return runner, "rspec does not start within the per-run timeout"
	case containsAny(strings.ToLower(output), offlineHints):
		return runner, "gem dependencies unavailable offline"
	}
	return runner, "rspec cannot run inside the sandbox"
}

func (runner rspecRunner) run(ctx context.Context, arguments ...string) (string, bool, error) {
	argv := append(append([]string(nil), runner.argv...), arguments...)
	return runner.copy.Run(ctx, runner.configuration.perRun, runner.copy.Dir(), runner.environment(), argv...)
}

// runSpec runs the cited spec file against whatever is in the export now;
// source is the changed file's content the run sees, for the receipt.
func (runner rspecRunner) runSpec(ctx context.Context, source []byte) specRun {
	reportPath := filepath.Join(runner.copy.Scratch(), reportName)
	output, timedOut, err := runner.run(ctx, "--format", "json", "--out", reportPath, "--no-color", runner.configuration.testPath)
	content, readErr := os.ReadFile(reportPath)
	return classifyRun(err, timedOut, output, content, readErr == nil, digest(source))
}

// classifyRun separates a red example from a run whose examples never all
// ran, and both from a run that produced no exit status. RSpec's own report
// decides; the exit status only confirms it.
func classifyRun(err error, timedOut bool, output string, content []byte, haveReport bool, sourceDigest string) specRun {
	run := specRun{output: output, receipt: Receipt{SourceSHA256: sourceDigest}}
	if timedOut {
		run.outcome, run.receipt.ExitStatus, run.receipt.TimedOut = runTimedOut, -1, true
		return run
	}
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		run.receipt.ExitStatus = exit.ExitCode()
	default:
		run.outcome, run.receipt.ExitStatus = runBroken, -1
		return run
	}
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
	case run.receipt.ErrorsOutsideExamples > 0 || run.receipt.Examples == 0:
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
	case run.outcome == runTimedOut:
		return "baseline examples exceed the per-run timeout: " + configuration.testPath
	case run.outcome == runBroken:
		return "rspec cannot run inside the sandbox"
	case containsAny(strings.ToLower(run.output), offlineHints):
		return "gem dependencies unavailable offline"
	case !run.reported:
		return "rspec wrote no report: " + configuration.testPath
	case run.receipt.ErrorsOutsideExamples > 0:
		return "baseline examples do not load: " + configuration.testPath
	case run.receipt.Examples == 0:
		return "spec file runs no examples: " + configuration.testPath
	case run.receipt.Failures > 0:
		return "baseline examples fail before mutation: " + configuration.testPath
	}
	return "baseline exits non-zero with every example passing: " + configuration.testPath
}

func containsAny(text string, hints []string) bool {
	for _, hint := range hints {
		if strings.Contains(text, hint) {
			return true
		}
	}
	return false
}
