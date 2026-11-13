package sessiontest

import (
	"encoding/json"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/testduration"
)

// The provider adapter (STR-V0-8701 to STR-V0-8707) runs the Vitest and
// Playwright test files the plan selected through corvint-js-test-provider,
// instead of the language's declared full command, and reads each unit's
// outcome from the receipt document the provider prints.

// providerLanguage is the affected plan's namespace of the unit ids a
// provider can run: "typescript:<runner>:<path>", JavaScript files included.
const providerLanguage = "typescript"

// providerCaptureLimit bounds the receipt document read from one provider
// run; a larger one is not read, and its units are NOT_RUN.
const providerCaptureLimit = MaxDocumentBytes * 8

// providerFailureLines bounds the failing tests one provider run names.
const providerFailureLines = 50

// splitProviderUnit returns the runner and the repository-relative test path
// of a "typescript:<runner>:<path>" unit id.
func splitProviderUnit(id string) (runner, file string, ok bool) {
	rest, ok := strings.CutPrefix(id, providerLanguage+":")
	if !ok {
		return "", "", false
	}
	runner, file, ok = strings.Cut(rest, ":")
	return runner, file, ok && runner != "" && file != ""
}

// providerUnit reports whether a declared provider runs the selected unit id
// (STR-V0-8702): its runner is declared, its path is a clean
// repository-relative path that cannot be read as an option, and a Vitest
// path lies under the directory whose suite the declared argv runs. Anything
// else stays with the language's declared full command.
func providerUnit(declared Declared, id string) bool {
	runner, file, ok := splitProviderUnit(id)
	if !ok {
		return false
	}
	argv, ok := declared.Provider(runner)
	if !ok || !providerPath(file) {
		return false
	}
	if runner == ProviderRunnerVitest {
		dir, ok := providerDir(argv)
		return ok && (dir == "." || strings.HasPrefix(file, dir+"/"))
	}
	return true
}

// providerPath reports whether file is a clean, local, slash-separated path
// that is safe as one argv word after --test-file and --test-arg.
func providerPath(file string) bool {
	if file == "" || strings.HasPrefix(file, "-") || strings.HasPrefix(file, "/") || strings.Contains(file, ":") || strings.Contains(file, `\`) {
		return false
	}
	if path.Clean(file) != file || file == ".." || strings.HasPrefix(file, "../") {
		return false
	}
	for _, r := range file {
		if unicode.IsControl(r) || r == utf8.RuneError {
			return false
		}
	}
	return true
}

// providerSelector is file as a Playwright command-line file filter.
// Playwright reads a filter as a regular expression over the absolute file
// path, so the metacharacters of a literal path are escaped.
func providerSelector(file string) string {
	var escaped strings.Builder
	for _, r := range file {
		if strings.ContainsRune(`\^$.|?*+()[]{}`, r) {
			escaped.WriteByte('\\')
		}
		escaped.WriteRune(r)
	}
	return escaped.String()
}

// providerInvocations derives one provider invocation per runner from the
// shard's provider units (STR-V0-8703): the declared argv, then --test-file
// for each selected file, and for Playwright --test-arg with the file's
// filter, which appends to the declared test command and selects what runs.
// The Vitest subcommand runs the whole suite of its directory, a superset of
// the selected files. A unit whose runner the declaration no longer names
// has no argv; Execute refuses it before discovery. The invocations are in
// runner-name order.
func providerInvocations(m Manifest, declared Declared, shard int) []Planned {
	var planned []Planned
	index := map[string]int{}
	for _, unit := range m.Units {
		if unit.Shard != shard || unit.Adapter != AdapterProvider {
			continue
		}
		runner, _, _ := splitProviderUnit(unit.Unit)
		if at, seen := index[runner]; seen {
			planned[at].Units = append(planned[at].Units, unit)
			continue
		}
		index[runner] = len(planned)
		planned = append(planned, Planned{Adapter: AdapterProvider, Argv: []string{}, Env: []string{}, Units: []Unit{unit}})
	}
	runnerOf := func(invocation Planned) string {
		runner, _, _ := splitProviderUnit(invocation.Units[0].Unit)
		return runner
	}
	sort.SliceStable(planned, func(i, j int) bool { return runnerOf(planned[i]) < runnerOf(planned[j]) })
	for at := range planned {
		runner := runnerOf(planned[at])
		prefix, ok := declared.Provider(runner)
		if !ok {
			continue
		}
		argv := append([]string{}, prefix...)
		for _, unit := range planned[at].Units {
			_, file, _ := splitProviderUnit(unit.Unit)
			argv = append(argv, "--test-file", file)
		}
		if runner == ProviderRunnerPlaywright {
			for _, unit := range planned[at].Units {
				_, file, _ := splitProviderUnit(unit.Unit)
				argv = append(argv, "--test-arg", providerSelector(file))
			}
		}
		planned[at].Argv = argv
	}
	return planned
}

// providerUnavailable records, for each provider invocation of planned that
// needs the runner named key, an invocation that never launched: every unit
// NOT_RUN (STR-V0-8705). The caller states the reason in the receipt's
// incomplete list, so the run is INCOMPLETE and never a pass.
func providerUnavailable(planned []Planned, key string, manifests map[string]AdapterManifest) []Invocation {
	var recorded []Invocation
	for _, invocation := range planned {
		if invocation.Adapter != AdapterProvider || runnerKey(invocation) != key {
			continue
		}
		recorded = append(recorded, Invocation{
			Adapter: AdapterProvider, AdapterManifest: manifests[AdapterProvider].Profile, Argv: nonNil(invocation.Argv),
			Env: []string{}, Exit: -1, Outcomes: notRun(invocation.Units),
		})
	}
	return recorded
}

// providerDocument is the part of the provider's stdout document the adapter
// reads: the receipt's kind, run-level failure and each test's state and
// file.
type providerDocument struct {
	Receipt *struct {
		Cancelled      bool `json:"cancelled"`
		Infrastructure *struct {
			Detail string `json:"detail"`
			Reason string `json:"reason"`
		} `json:"infrastructure"`
		Kind          string                 `json:"kind"`
		StaleAppBuild bool                   `json:"staleAppBuild"`
		Tests         []providerReportedTest `json:"tests"`
	} `json:"receipt"`
}

// providerReportedTest is the part of one provider receipt test the adapter reads. DurationMs is
// the runner's own per-test time (Vitest's assertion duration, Playwright's last attempt's result
// duration) and AttemptDetails keeps each Playwright attempt's: the internal/jstestprovider
// TestOutcome and AttemptDetail members of the same names.
type providerReportedTest struct {
	Anchor *struct {
		File string `json:"file"`
	} `json:"anchor"`
	AttemptDetails []struct {
		DurationMs *float64 `json:"durationMs"`
	} `json:"attemptDetails"`
	DurationMs *float64 `json:"durationMs"`
	// DurationMissing is the receipt's mark that the runner gave no time, so DurationMs is
	// the wire's zero (PTD-V1-003).
	DurationMissing bool   `json:"durationMissing"`
	FullName        string `json:"fullName"`
	Retries         int    `json:"retries"`
	State           string `json:"state"`
}

// providerSources names the duration source of each provider runner (PTD-V1-002).
var providerSources = map[string]string{ProviderRunnerPlaywright: testduration.SourcePlaywright, ProviderRunnerVitest: testduration.SourceVitest}

// providerTestMeasure is one provider test's reported time (PTD-V1-002, PTD-V1-006): the sum of
// its attempts when the receipt keeps each attempt, else its durationMs when it ran once. A test
// retried with only its last attempt's time, one with no time or marked durationMissing, and an
// infrastructure entry, which is no test run, are NOT_MEASURED (PTD-V1-003); nothing is estimated.
func providerTestMeasure(runner string, test providerReportedTest) testduration.Measure {
	source := providerSources[runner]
	switch {
	case test.State == "infrastructure":
		return testduration.Missing()
	case len(test.AttemptDetails) != 0:
		attempts := make([]testduration.Measure, 0, len(test.AttemptDetails))
		for _, attempt := range test.AttemptDetails {
			if attempt.DurationMs == nil {
				attempts = append(attempts, testduration.Missing())
				continue
			}
			attempts = append(attempts, testduration.FromMilliseconds(*attempt.DurationMs, source))
		}
		measure, _ := testduration.Attempts(attempts, source)
		return measure
	case test.DurationMs == nil || test.DurationMissing || test.Retries != 0:
		return testduration.Missing()
	}
	return testduration.FromMilliseconds(*test.DurationMs, source)
}

// providerCapture keeps at most providerCaptureLimit bytes of a provider's
// stdout and never fails the write, so the provider is drained either way.
type providerCapture struct {
	data     []byte
	overflow bool
}

func (c *providerCapture) Write(p []byte) (int, error) {
	if c.overflow || len(c.data)+len(p) > providerCaptureLimit {
		c.overflow = true
		return len(p), nil
	}
	c.data = append(c.data, p...)
	return len(p), nil
}

// providerOutcomes reads one finished provider run (STR-V0-8704,
// STR-V0-8705). The run counts only when the provider exited 0 with a
// receipt of the runner's kind that observed tests and reports no run-level
// failure; otherwise every unit is NOT_RUN and the returned reason makes the
// shard INCOMPLETE. In a counted run a unit is FAIL when a failing test is in
// its file or in no selected file. A Playwright unit otherwise needs a test
// of its own file: PASS with one that passed, NO_TESTS when all were
// skipped, NOT_RUN with a reason when the receipt names none. A Vitest
// receipt names no file for a passing test, so a Vitest unit without a
// failure is PASS: the outcome of the suite the provider ran. A summary and
// each failing test are written to out.
func providerOutcomes(planned Planned, exit int, capture *providerCapture, out io.Writer) ([]Outcome, []string) {
	runner, _, _ := splitProviderUnit(planned.Units[0].Unit)
	name := "provider " + runner
	refuse := func(reason string) ([]Outcome, []string) {
		return notRun(planned.Units), []string{name + ": " + reason + "; its selected tests are NOT_RUN"}
	}
	var document providerDocument
	if capture.overflow {
		return refuse(fmt.Sprintf("its receipt document exceeds %d bytes", providerCaptureLimit))
	}
	if json.Unmarshal(capture.data, &document) != nil || document.Receipt == nil {
		return refuse(fmt.Sprintf("it exited %d without a receipt document", exit))
	}
	receipt := document.Receipt
	switch {
	case receipt.Infrastructure != nil:
		return refuse("infrastructure failure " + providerText(receipt.Infrastructure.Reason) + ": " + providerText(receipt.Infrastructure.Detail))
	case exit != 0:
		return refuse(fmt.Sprintf("it exited %d", exit))
	case receipt.Kind != providerSubcommands[runner]:
		return refuse(fmt.Sprintf("its receipt kind is %q, want %q", providerText(receipt.Kind), providerSubcommands[runner]))
	case receipt.Cancelled:
		return refuse("the run was cancelled")
	case receipt.StaleAppBuild:
		return refuse("the app build changed while the tests ran")
	case len(receipt.Tests) == 0:
		return refuse("its receipt reports no test")
	}
	files := make([]string, len(planned.Units))
	for index, unit := range planned.Units {
		_, files[index], _ = splitProviderUnit(unit.Unit)
	}
	failing, passed, skipped := make([]bool, len(files)), make([]int, len(files)), make([]int, len(files))
	stray, counts, lines := false, map[string]int{}, 0
	// timed holds each file's test times. A provider runs only the selected files, so with one
	// file every test is its own; with more, a test no single file owns leaves every file's sum
	// unknown, since adding it to none or to a guessed one would be an estimate (PTD-V1-003).
	timed, unattributed := make([][]testduration.Measure, len(files)), false
	for _, test := range receipt.Tests {
		reported := test.FullName
		if test.Anchor != nil && test.Anchor.File != "" {
			reported = test.Anchor.File
		}
		owners := providerOwners(files, reported)
		switch {
		case len(files) == 1:
			timed[0] = append(timed[0], providerTestMeasure(runner, test))
		case len(owners) == 1:
			timed[owners[0]] = append(timed[owners[0]], providerTestMeasure(runner, test))
		default:
			unattributed = true
		}
		switch test.State {
		case "passed", "flaky":
			counts["passed"]++
			if len(owners) == 1 {
				passed[owners[0]]++
			}
		case "skipped":
			counts["skipped"]++
			if len(owners) == 1 {
				skipped[owners[0]]++
			}
		default:
			// failed, timedOut, interrupted, infrastructure, and any state
			// this adapter does not know: none is a pass.
			counts["failed"]++
			stray = stray || len(owners) == 0
			for _, owner := range owners {
				failing[owner] = true
			}
			if lines++; lines <= providerFailureLines {
				_, _ = fmt.Fprintf(out, "%s: %s %s\n", name, providerText(test.State), providerText(test.FullName))
			}
		}
	}
	if lines > providerFailureLines {
		_, _ = fmt.Fprintf(out, "%s: +%d more failing tests\n", name, lines-providerFailureLines)
	}
	_, _ = fmt.Fprintf(out, "%s: %d tests: %d passed, %d failed, %d skipped\n", name, len(receipt.Tests), counts["passed"], counts["failed"], counts["skipped"])
	outcomes := make([]Outcome, 0, len(planned.Units))
	var reasons []string
	for index, unit := range planned.Units {
		outcome := OutcomePass
		switch {
		case failing[index] || stray:
			outcome = OutcomeFail
		case runner != ProviderRunnerPlaywright || passed[index] != 0:
		case skipped[index] != 0:
			outcome = OutcomeNoTests
		default:
			outcome = OutcomeNotRun
			reasons = append(reasons, name+": its receipt names no test of "+files[index]+" alone; the unit is NOT_RUN")
		}
		// A provider file unit that ran records the sum of its tests' reported times as a
		// file-level duration (PTD-V1-002, PTD-V1-004), NOT_MEASURED when any of them has none or
		// a test cannot be placed in one file (PTD-V1-003); one invalid time makes the unit
		// invalid (PTD-V1-007). A NOT_RUN unit ran nothing and Finish marks it NOT_MEASURED alone.
		recorded := Outcome{Outcome: outcome, Slice: unit.Slice, Unit: unit.Unit}
		if outcome != OutcomeNotRun {
			measure := testduration.Missing()
			if !unattributed {
				measure, _ = testduration.Attempts(timed[index], providerSources[runner])
			}
			recorded = timedOutcome(recorded, &measure)
		}
		outcomes = append(outcomes, recorded)
	}
	return outcomes, reasons
}

// providerOwners returns the indexes of the selected files a reported file
// names: an equal path, or one that is the other's trailing path segments,
// since Playwright reports a file relative to its test directory and a
// failure stack names it absolutely.
func providerOwners(files []string, reported string) []int {
	reported = strings.ReplaceAll(reported, `\`, "/")
	var owners []int
	for index, file := range files {
		if reported == file || strings.HasSuffix(reported, "/"+file) || strings.HasSuffix(file, "/"+reported) {
			owners = append(owners, index)
		}
	}
	return owners
}

// providerText is provider-authored text as one bounded line without control
// characters.
func providerText(text string) string {
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == utf8.RuneError {
			return ' '
		}
		return r
	}, text)
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	if len(cleaned) > 300 {
		cleaned = strings.ToValidUTF8(cleaned[:300], "") + "..."
	}
	return cleaned
}
