package main

import (
	"errors"
	"flag"
	"io"

	"github.com/Beamfall/corvint/internal/testplan"
)

// runTestPlanPageState implements `test-plan page-state` (ANL-V0-014): read-only grouping of one
// input, with an optional drift check against an earlier report (ANL-V0-033).
func runTestPlanPageState(arguments []string, stdout, stderr io.Writer) int {
	f := flag.NewFlagSet("test-plan page-state", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	input := f.String("input", "", "test-consolidation-input/0 file")
	baseline := f.String("baseline", "", "earlier test-page-state-groups/0 report to check drift against")
	check := f.Bool("check", false, "exit 1 when the groups drift from --baseline")
	if f.Parse(arguments) != nil || f.NArg() != 0 || *input == "" || *check != (*baseline != "") {
		emitError(stderr, errors.New("test-plan page-state requires --input FILE; --check and --baseline FILE go together"))
		return 2
	}
	var old *testplan.PageStateReport
	if *baseline != "" {
		raw, err := readTestPlanFile(*baseline)
		if err == nil {
			old, err = testplan.DecodePageStateReport(raw)
		}
		if err != nil {
			emitError(stderr, err)
			return 2
		}
	}
	raw, err := readTestPlanFile(*input)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	in, err := testplan.DecodeInput(raw)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	data, err := testplan.GroupByPageState(in)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if _, err = stdout.Write(data); err != nil {
		emitError(stderr, err)
		return 2
	}
	if old != nil {
		current, err := testplan.DecodePageStateReport(data)
		if err != nil {
			emitError(stderr, err)
			return 2
		}
		if err = testplan.PageStateDrift(old, current); err != nil {
			emitError(stderr, err)
			return 1
		}
	}
	return 0
}
