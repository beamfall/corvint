// SPDX-License-Identifier: Apache-2.0
package sessiontest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/selectionswitch"
	"github.com/Beamfall/corvint/internal/testid"
	"github.com/Beamfall/corvint/internal/testinventory"
	"github.com/Beamfall/corvint/internal/testrunner/contract"
)

// The runner adapter (PYR-V1-002 to PYR-V1-006, PYR-V1-008) runs the test
// files the plan selected for a runner instance through the compiled-in
// runner contract: one invocation per instance, from its root, with the
// contract's selection argv and machine report, and records one result per
// canonical test ID beside each file unit's outcome.

// contractRunners are the runners whose narrowed `corvint test` run goes
// through the runner adapter. Every other runner still runs its language's
// full command or its provider (RNC-V1-001 task 5).
var contractRunners = map[string]bool{testid.Pytest: true}

// tierFilePrefix starts the reason of a runner unit: it runs its whole
// file (owner decision R8 (2026-11-09)).
const tierFilePrefix = "tier file: "

// tierFileReason is the reason every pytest file unit carries
// (PYR-V1-002): Python line evidence has no FIH-V1 verdict, so a selected
// test runs its whole file.
const tierFileReason = tierFilePrefix + "pytest runs each selected file whole; Python line evidence has no passed FIH-V1 verdict (owner decisions R8, R9)"

// RunnerReportPlaceholder stands for the machine report path in a planned
// runner argv; the launch replaces it with a fresh private file, so the
// manifest and the receipt never name a temporary path.
const RunnerReportPlaceholder = "<runner-report>"

// runnerReportLimit bounds the machine report read from one run; a larger
// one is not read, and every selected test is NOT_RUN.
const runnerReportLimit = MaxDocumentBytes * 8

// lockfileLimit bounds the lockfile bytes hashed for the environment record.
const lockfileLimit = 256 << 20

// owningInstance is the deepest-rooted instance of id's runner that owns
// it (RNC-V1-014): a nested rootdir owns its files, not the root instance.
func owningInstance(instances []contract.Instance, id testid.ID) (contract.Instance, bool) {
	var best contract.Instance
	found := false
	for _, instance := range instances {
		if instance.Runner != id.Runner || !instance.Owns(id.String()) {
			continue
		}
		if !found || len(instance.Root) > len(best.Root) || best.Root == "." {
			best, found = instance, true
		}
	}
	return best, found
}

// runnerFallbackPrefix starts the reason a language full command carries
// when it runs because a selection could not go through the runner adapter
// (PYR-V1-009).
const runnerFallbackPrefix = "runner fallback: "

// runnerUnits returns the runner units of one selected plan unit, or a
// fallback reason when the unit stays with its language's full command
// ("" when the plan named no runnable test file for it, or under the ci
// profile, which runs only Go, STR-V0-012): the declaration names the
// language's own full command, a file is not a contract runner's, no
// declared instance owns it, or CORVINT_SELECTION forces its runner's full
// suite (PYR-V1-009, reported by the kill-switch reason instead).
func runnerUnits(input Input, id string) ([]Unit, string, bool) {
	tests := input.ContractTests[id]
	language, _, _ := strings.Cut(id, ":")
	if len(tests) == 0 || input.RunnerProfile == ProfileCI {
		return nil, "", false
	}
	if _, declared := input.Declared.Command(language); declared {
		return nil, runnerFallbackPrefix + input.Declared.Source + " declares a " + language + " full command, which runs instead of the compiled-in runner for " + id, false
	}
	units := make([]Unit, 0, len(tests))
	for _, text := range tests {
		parsed, err := testid.Parse(text)
		switch {
		case err != nil || !parsed.IsFileLevel() || !contractRunners[parsed.Runner] || runnerLanguages[parsed.Runner] != language:
			return nil, runnerFallbackPrefix + id + " selected " + text + ", which no compiled-in runner runs by file", false
		case input.Selection.Forces(parsed.Runner):
			return nil, "", false
		}
		instance, ok := owningInstance(input.Instances, parsed)
		if !ok {
			return nil, runnerFallbackPrefix + "no declared " + parsed.Runner + " instance owns " + parsed.File + ", so the " + language + " full command runs", false
		}
		units = append(units, Unit{
			Adapter: AdapterRunner, Instance: instance.Key(), Reason: tierFileReason, ReasonCode: ReasonTierFile,
			Shard: 0, Slice: SliceWhole, Tier: contract.TierFile, Unit: parsed.String(),
		})
	}
	return units, "", true
}

// runnerInvocations derives one runner invocation per instance from the
// shard's runner units (PYR-V1-003), in instance-key order: the instance's
// full command plus the contract's selection argv and report arguments,
// from the instance root. A unit whose instance the declaration cannot
// resolve, or whose selection the contract refuses, has no argv; Execute
// refuses it before discovery.
func runnerInvocations(m Manifest, declared Declared, shard int, root string) []Planned {
	byInstance := map[string][]Unit{}
	for _, unit := range m.Units {
		if unit.Shard == shard && unit.Adapter == AdapterRunner {
			byInstance[unit.Instance] = append(byInstance[unit.Instance], unit)
		}
	}
	keys := make([]string, 0, len(byInstance))
	for key := range byInstance {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	planned := make([]Planned, 0, len(keys))
	for _, key := range keys {
		units := byInstance[key]
		invocation := Planned{Adapter: AdapterRunner, Argv: []string{}, Env: []string{}, Instance: key, Tier: contract.TierFile, Units: units}
		full, dir, _ := declared.InstanceCommand(key, root)
		runner, _, _ := strings.Cut(key, "@")
		implementation, ok := contract.For(runner)
		if len(full) != 0 && ok {
			tests := make([]string, len(units))
			for index, unit := range units {
				tests[index] = unit.Unit
			}
			command, err := implementation.Command(contract.Request{
				Instance: contract.Instance{Runner: runner, Root: dir}, Full: full, Tests: tests, Tier: contract.TierFile, Report: RunnerReportPlaceholder,
			})
			if err == nil {
				invocation.Argv, invocation.Env, invocation.Full = command.Argv, nonNil(command.Env), full
				if dir != "." {
					invocation.Dir = dir
				}
			}
		}
		planned = append(planned, invocation)
	}
	return planned
}

// RunnerEnvironment is the environment record of one runner invocation
// (PYR-V1-008): the interpreter and runner versions, and the lockfile in
// use with its SHA-256, when one is present. A version the run did not
// print is NOT_OBSERVED.
type RunnerEnvironment struct {
	Python         string `json:"python"`
	Pytest         string `json:"pytest"`
	Lockfile       string `json:"lockfile,omitempty"`
	LockfileSHA256 string `json:"lockfileSha256,omitempty"`
}

// notObserved marks an environment value no probe or output named.
const notObserved = "NOT_OBSERVED"

// probeRunner checks that the instance's full command can start pytest by
// running it with --version from the instance root (PYR-V1-006), and
// returns the pytest version it prints (PYR-V1-008).
func (r *Runner) probeRunner(ctx context.Context, invocation Planned, deadline time.Time, runner string) (string, error) {
	output, err := r.probe(ctx, deadline, runner, invocation.Dir, GoCacheOverride(invocation.Env, r.Environ), append(slices.Clone(invocation.Full[1:]), "--version")...)
	if errors.Is(err, errDiscoveryBudget) {
		return "", err
	}
	if err != nil {
		return "", fmt.Errorf("%s: %s --version failed, so pytest cannot be started or imported", contract.ReasonRunnerUnavailable, strings.Join(invocation.Full, " "))
	}
	if version := contract.PytestVersion(output); version != "" {
		return version, nil
	}
	return notObserved, nil
}

// runnerOutput passes a runner's stdout through to out and reads two kinds
// of line as they pass: pytest's session header (the Python and pytest
// versions) and a deselected count. A line longer than its buffer is passed
// through unread.
type runnerOutput struct {
	out        io.Writer
	line       []byte
	python     string
	pytest     string
	deselected int
}

func (w *runnerOutput) Write(p []byte) (int, error) {
	_, _ = w.out.Write(p)
	for _, b := range p {
		if b != '\n' {
			if len(w.line) < 64<<10 {
				w.line = append(w.line, b)
			}
			continue
		}
		w.read(string(w.line))
		w.line = w.line[:0]
	}
	return len(p), nil
}

func (w *runnerOutput) read(line string) {
	if python, pytest, ok := contract.PytestHeader(line); ok && w.python == "" {
		w.python, w.pytest = python, pytest
	}
	if count, ok := contract.PytestDeselected(line); ok && count > w.deselected {
		w.deselected = count
	}
}

func (w *runnerOutput) flush() {
	if len(w.line) != 0 {
		w.read(string(w.line))
		w.line = w.line[:0]
	}
}

// lockfileRecord is the nearest pytest lockfile of root (the instance root
// or the nearest directory above it, testinventory's precedence) and its
// SHA-256, or empty strings when there is none or it cannot be read.
func lockfileRecord(repository, root string) (string, string) {
	name := testinventory.NearestLockfile(func(rel string) bool {
		info, err := os.Stat(filepath.Join(repository, filepath.FromSlash(rel)))
		return err == nil && info.Mode().IsRegular()
	}, testid.Pytest, root)
	if name == "" {
		return "", ""
	}
	file, err := openRegular(filepath.Join(repository, filepath.FromSlash(name)))
	if err != nil {
		return name, ""
	}
	defer file.Close()
	hash := sha256.New()
	if n, err := io.Copy(hash, io.LimitReader(file, lockfileLimit+1)); err != nil || n > lockfileLimit {
		return name, ""
	}
	return name, hex.EncodeToString(hash.Sum(nil))
}

// runnerSelection is the selected canonical test IDs of a runner
// invocation's units.
func runnerSelection(units []Unit) []string {
	selected := make([]string, len(units))
	for index, unit := range units {
		selected[index] = unit.Unit
	}
	return selected
}

// runnerOutcomes reads one finished runner invocation (PYR-V1-004,
// PYR-V1-006): the machine report at reportPath is parsed by the contract
// and joined with the selected files (contract.ReconcileParsed). Each file
// unit is FAIL when a test of it failed or errored, PASS when one passed,
// NO_TESTS when all were skipped, and NOT_RUN when the report names none.
// The returned reasons make the shard INCOMPLETE: an exit status that is
// not a test outcome, no tests collected, a missing or unreadable report,
// deselected tests, or a selected file with no result.
func runnerOutcomes(planned Planned, exit int, report []byte, reportErr error, root string, deselected int) ([]Outcome, []contract.Result, []string) {
	runner, instanceRoot, _ := strings.Cut(planned.Instance, "@")
	name := "runner " + planned.Instance
	var reasons []string
	selected := runnerSelection(planned.Units)
	var results []contract.Result
	var err error
	switch {
	case reportErr != nil:
		err = reportErr
	default:
		implementation, _ := contract.For(runner)
		results, err = implementation.Parse(contract.ParseRequest{Instance: contract.Instance{Runner: runner, Root: instanceRoot}, Report: report, Root: root})
	}
	if reason := contract.PytestExitReason(exit); reason != "" && runner == testid.Pytest {
		reasons = append(reasons, name+": "+reason)
	}
	if err != nil {
		reasons = append(reasons, fmt.Sprintf("%s: %s: %s", name, contract.ReasonReportInvalid, providerText(err.Error())))
	}
	if deselected != 0 {
		reasons = append(reasons, fmt.Sprintf("%s: %s: %d collected tests were deselected by a plugin or option, so the selected files did not all run", name, contract.ReasonTestsDeselected, deselected))
	}
	run := contract.ReconcileParsed(selected, results, err)
	if len(run.NotRun) != 0 {
		reasons = append(reasons, fmt.Sprintf("%s: %d selected file(s) have no result and are NOT_RUN: %s", name, len(run.NotRun), strings.Join(run.NotRun, ", ")))
	}
	outcomes := make([]Outcome, 0, len(planned.Units))
	for _, unit := range planned.Units {
		failed, passed, skipped, notRun := false, false, false, false
		for _, result := range run.Results {
			if !testid.Matches(unit.Unit, result.ID) {
				continue
			}
			switch result.Outcome {
			case contract.OutcomeFail, contract.OutcomeError:
				failed = true
			case contract.OutcomePass:
				passed = true
			case contract.OutcomeSkip:
				skipped = true
			case contract.OutcomeNotRun:
				notRun = true
			}
		}
		outcome := OutcomeNotRun
		switch {
		case notRun:
		case failed:
			outcome = OutcomeFail
		case passed:
			outcome = OutcomePass
		case skipped:
			outcome = OutcomeNoTests
		}
		outcomes = append(outcomes, Outcome{Outcome: outcome, Slice: unit.Slice, TestID: unit.Unit, Unit: unit.Unit})
	}
	return outcomes, run.Results, reasons
}

// readRunnerReport reads the report a run wrote, at most runnerReportLimit
// bytes of a regular file.
func readRunnerReport(name string) ([]byte, error) {
	file, err := openRegular(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("the run wrote no report")
		}
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, runnerReportLimit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > runnerReportLimit {
		return nil, fmt.Errorf("the report exceeds %d bytes", runnerReportLimit)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.New("the report is empty")
	}
	return data, nil
}

// withReport replaces RunnerReportPlaceholder in argv with report.
func withReport(argv []string, report string) []string {
	out := make([]string, len(argv))
	for index, arg := range argv {
		out[index] = strings.ReplaceAll(arg, RunnerReportPlaceholder, report)
	}
	return out
}

// runnerUnavailable records, for each runner invocation of planned that
// needs the runner named key, an invocation that never launched: every
// unit NOT_RUN with the reason (PYR-V1-006). The caller states the reason
// in the receipt's incomplete list.
func runnerUnavailable(planned []Planned, key, reason string, manifests map[string]AdapterManifest) []Invocation {
	var recorded []Invocation
	for _, invocation := range planned {
		if invocation.Adapter != AdapterRunner || runnerKey(invocation) != key {
			continue
		}
		outcomes := notRun(invocation.Units)
		for index := range outcomes {
			outcomes[index].TestID = outcomes[index].Unit
		}
		recorded = append(recorded, Invocation{
			Adapter: AdapterRunner, AdapterManifest: manifests[AdapterRunner].Profile, Argv: nonNil(invocation.Argv), Dir: invocation.Dir,
			Env: []string{}, Exit: -1, Instance: invocation.Instance, Outcomes: outcomes, Reasons: []string{reason}, Tier: invocation.Tier,
		})
	}
	return recorded
}

// killSwitchReason is the reason a forced runner's full-suite unit carries
// (PYR-V1-009).
func killSwitchReason(selection selectionswitch.Switch, runner string) string {
	return selection.Line("every " + runner + " instance to run its full suite")
}

// openRegular opens a regular file without following a final symlink or
// blocking on a FIFO.
func openRegular(name string) (*os.File, error) {
	file, err := openInput(name)
	if err != nil {
		return nil, err
	}
	if info, err := file.Stat(); err != nil || !info.Mode().IsRegular() {
		file.Close()
		return nil, fmt.Errorf("%s is not a regular file", filepath.Base(name))
	}
	return file, nil
}

// recordRunner completes a finished runner invocation's record: its file
// outcomes, per-test results and typed reasons (runnerOutcomes) and its
// environment record (PYR-V1-008). version is the interpreter version the
// discovery probe read from the command's first word. A run the budget or
// a cancellation stopped records every unit NOT_RUN.
func (r *Runner) recordRunner(recorded Invocation, planned Planned, stop, report, version string, output *runnerOutput) (Invocation, string, []string) {
	if stop != "" {
		recorded.Outcomes = notRun(planned.Units)
		for index := range recorded.Outcomes {
			recorded.Outcomes[index].TestID = recorded.Outcomes[index].Unit
		}
		return recorded, stop, nil
	}
	data, err := readRunnerReport(report)
	deselected := 0
	if planned.Tier != contract.TierFull {
		// A full suite runs what the project configured; only a narrowed
		// selection must run every selected test (PYR-V1-006).
		deselected = output.deselected
	}
	outcomes, results, reasons := runnerOutcomes(planned, recorded.Exit, data, err, r.Root, deselected)
	recorded.Outcomes, recorded.Tests = outcomes, results
	prefix := "runner " + planned.Instance + ": "
	for _, reason := range reasons {
		recorded.Reasons = append(recorded.Reasons, strings.TrimPrefix(reason, prefix))
	}
	python, pytest := output.python, output.pytest
	if python == "" {
		if value, ok := strings.CutPrefix(version, "Python "); ok {
			python = value
		} else {
			python = notObserved
		}
	}
	if pytest == "" {
		pytest = planned.probed
	}
	if pytest == "" {
		pytest = notObserved
	}
	_, root, _ := strings.Cut(planned.Instance, "@")
	lockfile, sum := lockfileRecord(r.Root, root)
	recorded.Environment = &RunnerEnvironment{Python: python, Pytest: pytest, Lockfile: lockfile}
	if sum != "" {
		recorded.Environment.LockfileSHA256 = sum
	}
	return recorded, "", reasons
}
