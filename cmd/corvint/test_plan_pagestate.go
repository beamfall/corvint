package main

import (
	"errors"
	"flag"
	"io"

	"github.com/Beamfall/corvint/internal/testplan"
)

// runTestPlanPageState implements `test-plan page-state` (ANL-V0-014): read-only grouping of one input.
func runTestPlanPageState(arguments []string, stdout, stderr io.Writer) int {
	f := flag.NewFlagSet("test-plan page-state", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	input := f.String("input", "", "test-consolidation-input/0 file")
	if f.Parse(arguments) != nil || f.NArg() != 0 || *input == "" {
		emitError(stderr, errors.New("test-plan page-state requires --input FILE"))
		return 2
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
	return 0
}
