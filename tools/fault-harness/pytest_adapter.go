// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/testid"
	"github.com/Beamfall/corvint/internal/testrunner/contract"
)

// pytestAdapter drives pytest through the compiled-in runner contract
// (FIH-V1-008, PYR-V1-010): the contract's node-ID selection and its JUnit
// XML report parser, run from the tree root, the pytest rootdir. A test ID
// is `pytest:<file>#<node parts>`: the unit is the file's canonical ID and
// the name is the node ID after the file (`TestK::test_x[1]`).
type pytestAdapter struct {
	pytest  []string
	timeout time.Duration
}

func (pytestAdapter) Runner() string { return testid.Pytest }

// pytestInstance is the one instance the harness drives: the tree root.
var pytestInstance = contract.Instance{Runner: testid.Pytest, Root: "."}

// pytestSelection is the canonical IDs entries name: a whole file for an
// entry without names, else each named node of it.
func pytestSelection(entries []planEntry) ([]string, error) {
	var ids []string
	for _, e := range entries {
		file, err := testid.Parse(e.Unit)
		if err != nil || file.Runner != testid.Pytest || !file.IsFileLevel() {
			return nil, fmt.Errorf("pytest adapter cannot run unit %q", e.Unit)
		}
		if len(e.Run) == 0 {
			ids = append(ids, file.String())
			continue
		}
		for _, name := range e.Run {
			// The node ID's own parser keeps a `::` inside a parameter id
			// (`test_x[a::b]`) in one segment, as pytest's report does.
			id, err := testid.PytestNodeID(pytestInstance.Root, file.File+"::"+name)
			if err != nil {
				return nil, fmt.Errorf("unit %s: unsupported test name %q: %w", e.Unit, name, err)
			}
			ids = append(ids, id.String())
		}
	}
	if len(ids) == 0 {
		return nil, errors.New("pytest adapter: an empty selection runs nothing")
	}
	return ids, nil
}

// Run runs the whole suite when entries is nil, else exactly the selected
// nodes, and returns one outcome per reported test. A run that is not a
// clean set of outcomes is an error: the harness stopped it, pytest exited
// with a status that is not 0 or 1 (or 1 with nothing failing), the report
// is missing or unreadable, a file did not collect, a test was reported
// twice with different outcomes, or a selected test has no result.
func (a pytestAdapter) Run(ctx context.Context, root string, entries []planEntry) (RunResult, error) {
	dir, err := os.MkdirTemp("", "fault-harness-pytest-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	report := filepath.Join(dir, "report.xml")
	argv := append(slices.Clone(a.pytest), contract.PytestReportArgs(report)...)
	var selected []string
	if entries != nil {
		if selected, err = pytestSelection(entries); err != nil {
			return nil, err
		}
		nodes, err := contract.PytestNodes(pytestInstance, selected)
		if err != nil {
			return nil, err
		}
		argv = append(argv, nodes...)
	}
	out, runErr := command(ctx, root, a.timeout+time.Minute, argv...)
	if errors.Is(runErr, errWallTimeout) || ctx.Err() != nil {
		return nil, fmt.Errorf("pytest stopped by the harness: %w", runErr)
	}
	exit := 0
	var exitErr *exec.ExitError
	switch {
	case runErr == nil:
	case errors.As(runErr, &exitErr):
		exit = exitErr.ExitCode()
	default:
		return nil, fmt.Errorf("pytest did not run: %v: %s", runErr, tail(out))
	}
	if reason := contract.PytestExitReason(exit); reason != "" {
		return nil, fmt.Errorf("pytest did not finish with test outcomes: %s", reason)
	}
	data, err := boundedRead(report, 64<<20)
	if err != nil {
		return nil, fmt.Errorf("pytest wrote no report: %v", err)
	}
	runner, _ := contract.For(testid.Pytest)
	results, err := runner.Parse(contract.ParseRequest{Instance: pytestInstance, Report: data, Root: root})
	if err != nil {
		return nil, err
	}
	run := contract.Reconcile(selected, results)
	if len(run.NotRun) != 0 {
		return nil, fmt.Errorf("pytest reported no result for %d selected test(s), first %s", len(run.NotRun), run.NotRun[0])
	}
	result := RunResult{}
	failing := false
	for _, r := range results {
		id, err := testid.Parse(r.ID)
		if err != nil {
			return nil, err
		}
		if r.Granularity != testid.GranularityTest {
			return nil, fmt.Errorf("pytest file %s did not collect", id.File)
		}
		var o Outcome
		switch r.Outcome {
		case contract.OutcomePass:
			o = Pass
		case contract.OutcomeSkip:
			o = Skip
		default:
			o, failing = Fail, true
		}
		key := id.FileLevel().String() + "#" + strings.Join(id.Segments, "::")
		if prev, ok := result[key]; ok && prev != o {
			return nil, fmt.Errorf("test %s reported twice with different outcomes", key)
		}
		result[key] = o
	}
	if exit == 1 && !failing {
		return nil, errors.New("pytest exited 1 with no failing test")
	}
	return result, nil
}

// tail is the last line of out, for an error message.
func tail(out []byte) string {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	return lines[len(lines)-1]
}

// DependentUnits returns every collected pytest test file of the tree: no
// Python dependency graph is available to the adapter, so the answer is
// the conservative superset, which can only overcount omitted quarantined
// tests (FIH-V1-004), never hide one.
func (a pytestAdapter) DependentUnits(ctx context.Context, root string, _ []string) ([]string, error) {
	argv := append(slices.Clone(a.pytest), "--collect-only", "-q", "-p", "no:cacheprovider", "--rootdir=.")
	out, err := command(ctx, root, a.timeout, argv...)
	var exitErr *exec.ExitError
	if err != nil && !(errors.As(err, &exitErr) && exitErr.ExitCode() == 5) {
		return nil, fmt.Errorf("pytest --collect-only: %w: %s", err, tail(out))
	}
	var units []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if !strings.Contains(line, "::") {
			continue
		}
		id, err := testid.PytestNodeID(".", line)
		if err != nil {
			return nil, err
		}
		if unit := id.FileLevel().String(); !slices.Contains(units, unit) {
			units = append(units, unit)
		}
	}
	slices.Sort(units)
	return units, nil
}
