package main

import (
	"context"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/testplan"
)

// runTestCommand dispatches `test-validity` and `test-plan` after any --root prefix; it reports
// false for every other command.
func runTestCommand(ctx context.Context, arguments []string, stdout, stderr io.Writer) (int, bool) {
	if root, rest, isTestValidity := parseTestValidityInvocation(arguments); isTestValidity {
		return runTestValidity(root, rest, stdout, stderr), true
	}
	if root, rest, isTestPlan := parseTestPlanInvocation(arguments); isTestPlan {
		return runTestPlan(ctx, root, rest, stdout, stderr), true
	}
	return 0, false
}

// parseTestPlanInvocation intercepts `test-plan` after any --root prefix (TCN-V0-001).
func parseTestPlanInvocation(arguments []string) (string, []string, bool) {
	index, root := 0, ""
	for index < len(arguments) && (arguments[index] == "--root" && rootPreambleValue(arguments, index+1) || strings.HasPrefix(arguments[index], "--root=")) {
		if arguments[index] == "--root" {
			root, index = arguments[index+1], index+2
			continue
		}
		root, index = strings.TrimPrefix(arguments[index], "--root="), index+1
	}
	if index >= len(arguments) || arguments[index] != "test-plan" {
		return "", nil, false
	}
	return root, arguments[index+1:], true
}

type testPlanOptions struct {
	check    bool
	input    string
	plan     string
	format   string
	revision string
	maxSteps int
	tests    []string
	maps     []string
}

func testPlanArgumentError(message string) error {
	return &gokernel.Error{Code: "test-plan-invalid-arguments", Message: message}
}

// parseTestPlanOptions decodes `consolidate` or `check`; single-valued options may be given once.
func parseTestPlanOptions(arguments []string) (testPlanOptions, error) {
	options := testPlanOptions{format: "table"}
	if len(arguments) == 0 || arguments[0] != "consolidate" && arguments[0] != "check" {
		return options, testPlanArgumentError("test-plan needs the subcommand consolidate or check")
	}
	options.check = arguments[0] == "check"
	seen := map[string]bool{}
	for index := 1; index < len(arguments); index++ {
		name, value, inline := splitWitnessFlag(arguments[index])
		switch name {
		case "--input", "--tests", "--map", "--revision", "--max-steps":
		case "--format":
			if options.check {
				return options, testPlanArgumentError("check takes no --format")
			}
		case "--plan":
			if !options.check {
				return options, testPlanArgumentError("consolidate takes no --plan")
			}
		default:
			return options, testPlanArgumentError("unrecognized test-plan option: " + name)
		}
		if !inline {
			if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
				return options, testPlanArgumentError("missing value for " + name)
			}
			index++
			value = arguments[index]
		}
		if value == "" {
			return options, testPlanArgumentError(name + " requires a non-empty value")
		}
		if name != "--tests" && name != "--map" {
			if seen[name] {
				return options, testPlanArgumentError(name + " may be given once")
			}
			seen[name] = true
		}
		switch name {
		case "--input":
			options.input = value
		case "--plan":
			options.plan = value
		case "--tests":
			options.tests = append(options.tests, value)
		case "--map":
			options.maps = append(options.maps, value)
		case "--revision":
			options.revision = value
		case "--format":
			if value != "table" && value != "json" {
				return options, testPlanArgumentError("--format must be table or json")
			}
			options.format = value
		case "--max-steps":
			n, err := strconv.Atoi(value)
			if err != nil || n < testplan.MinMaxSteps || n > testplan.MaxMaxSteps {
				return options, testPlanArgumentError("--max-steps must be an integer 2..32")
			}
			options.maxSteps = n
		}
	}
	if options.input == "" {
		return options, testPlanArgumentError("--input is required")
	}
	if options.check && options.plan == "" {
		return options, testPlanArgumentError("check requires --plan")
	}
	return options, nil
}

// readTestPlanFile reads one regular file of at most 8 MiB.
func readTestPlanFile(path string) ([]byte, error) {
	refuse := func(message string) error {
		return &gokernel.Error{Code: "test-plan-invalid-input", Message: message}
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, refuse("cannot open " + path)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, refuse(path + " is not a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, testplan.MaxInputBytes+1))
	if err != nil {
		return nil, refuse("cannot read " + path)
	}
	if len(data) > testplan.MaxInputBytes {
		return nil, refuse(path + " exceeds 8 MiB")
	}
	return data, nil
}

// runTestPlan prints a candidate consolidation plan or checks a pasted one (TCN-V0-001,
// TCN-V0-011). It writes stdout only on success; a failed check exits 1, invalid use exits 2.
func runTestPlan(ctx context.Context, root string, arguments []string, stdout, stderr io.Writer) int {
	options, err := parseTestPlanOptions(arguments)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	input, err := readTestPlanFile(options.input)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	var planFile []byte
	if options.check {
		if planFile, err = readTestPlanFile(options.plan); err != nil {
			emitError(stderr, err)
			return 2
		}
	}
	if root == "" {
		root = "."
	}
	plan, err := testplan.Run(ctx, testplan.Request{Root: root, Input: input, Tests: options.tests, Maps: options.maps,
		Revision: options.revision, MaxSteps: options.maxSteps})
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	output := plan.Table()
	if options.check {
		if err := testplan.Check(plan, planFile); err != nil {
			emitError(stderr, err)
			var coded *gokernel.Error
			if errors.As(err, &coded) && coded.Code == "test-plan-invalid-input" {
				return 2
			}
			return 1
		}
	} else if options.format == "json" {
		output = plan.JSON()
	}
	if _, err := stdout.Write(output); err != nil {
		emitError(stderr, err)
		return 2
	}
	return 0
}
