package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/trace"
	"github.com/Beamfall/corvint/internal/tracerecordrepo"
)

type dogfoodRecordOptions struct {
	base, target, task, outcome string
	verification                []string
	verificationArgv            [][]string
	baseSet, targetSet          bool
	taskSet, verifySet          bool
	outcomeSet                  bool
	// aggregateOverLimit opts into the nonlearning aggregate receipt when the
	// legacy trace admission refuses more than 200 changed paths (DCW-V0-374).
	aggregateOverLimit bool
}

func parseDogfoodRecordInvocation(arguments []string) (string, []string, bool, error) {
	index := 0
	root := ""
	for index < len(arguments) && (arguments[index] == "--root" || strings.HasPrefix(arguments[index], "--root=")) {
		if arguments[index] == "--root" {
			if !rootPreambleValue(arguments, index+1) {
				return "", nil, false, nil
			}
			root = arguments[index+1]
			index += 2
		} else {
			root = strings.TrimPrefix(arguments[index], "--root=")
			index++
		}
	}
	if index >= len(arguments) || arguments[index] != "dogfood-record" {
		return "", nil, false, nil
	}
	if root == "" {
		workingDirectory, err := os.Getwd()
		if err != nil {
			return "", nil, true, argumentError("cannot resolve current directory")
		}
		root = workingDirectory
	} else {
		resolved, err := resolveExplicitRoot(root)
		if err != nil {
			return "", nil, true, argumentError("argument --root: " + err.Error())
		}
		root = resolved
	}
	return root, arguments[index+1:], true, nil
}

func parseDogfoodRecordFlags(arguments []string) (dogfoodRecordOptions, error) {
	var options dogfoodRecordOptions
	for index := 0; index < len(arguments); {
		argument := arguments[index]
		if argument == "--aggregate-over-limit" {
			options.aggregateOverLimit = true
			index++
			continue
		}
		name, value, inline := strings.Cut(argument, "=")
		if name != "--base" && name != "--target" && name != "--task" && name != "--verify" && name != "--verify-argv-json" && name != "--outcome" {
			return options, argumentError("unrecognized arguments: " + argument)
		}
		if !inline {
			if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
				return options, argumentError("argument " + name + ": expected one argument")
			}
			value = arguments[index+1]
			index += 2
		} else {
			index++
		}
		switch name {
		case "--base":
			options.base, options.baseSet = value, true
		case "--target":
			options.target, options.targetSet = value, true
		case "--task":
			options.task, options.taskSet = value, true
		case "--verify":
			options.verification = append(options.verification, value)
			options.verifySet = true
		case "--verify-argv-json":
			// DCW-V0-384: a typed argv entry (LTPM-V0-013), such as a verify
			// line's quoted -run alternation, kept as exact arguments.
			argv, err := trace.ParseVerificationArgv([]byte(value))
			if err != nil {
				return options, err
			}
			options.verificationArgv = append(options.verificationArgv, argv)
			options.verifySet = true
		case "--outcome":
			if value != "passed" && value != "failed" && value != "blocked" {
				return options, argumentError("argument --outcome: invalid choice: " + pythonRepr(value) + " (choose from 'passed', 'failed', 'blocked')")
			}
			options.outcome, options.outcomeSet = value, true
		}
	}
	missing := make([]string, 0, 5)
	for _, required := range []struct {
		set  bool
		name string
	}{{options.baseSet, "--base"}, {options.targetSet, "--target"}, {options.taskSet, "--task"}, {options.verifySet, "--verify"}, {options.outcomeSet, "--outcome"}} {
		if !required.set {
			missing = append(missing, required.name)
		}
	}
	if len(missing) != 0 {
		return options, argumentError("the following arguments are required: " + strings.Join(missing, ", "))
	}
	return options, nil
}

func runDogfoodRecord(ctx context.Context, root string, arguments []string, stdout, stderr io.Writer) int {
	options, err := parseDogfoodRecordFlags(arguments)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	result, err := tracerecordrepo.RecordDogfood(ctx, root, options.base, options.target, tracerecordrepo.Input{
		Producer: trace.ProducerDogfood, Task: options.task, Verification: options.verification, VerificationArgv: options.verificationArgv, Outcome: options.outcome,
	})
	if err != nil && options.aggregateOverLimit && tracerecordrepo.DogfoodFailureReason(err) == "admitted-path-limit" {
		return runDogfoodAggregateRecord(ctx, root, options, stdout, stderr)
	}
	if err != nil {
		emitDogfoodRecordError(stderr, tracerecordrepo.DogfoodFailureReason(err), err)
		return 2
	}
	if err := emit(stdout, dogfoodRecordPayload(result)); err != nil {
		emitDogfoodRecordError(stderr, "output-failed", fmt.Errorf("cannot write dogfood record output"))
		return 2
	}
	return 0
}

// runDogfoodAggregateRecord writes the complete nonlearning aggregate receipt
// for a change whose admitted paths exceed the trace learning bound. It opens no
// trace store: learning input stays bounded at trace.MaxTracePaths (DCW-V0-374).
func runDogfoodAggregateRecord(ctx context.Context, root string, options dogfoodRecordOptions, stdout, stderr io.Writer) int {
	if len(options.verificationArgv) != 0 {
		// DCW-V0-384: the aggregate receipt binds v1 commands only.
		emitDogfoodRecordError(stderr, "aggregate-outcome-failed", fmt.Errorf("a typed argv verification, such as a quoted -run alternation, has no aggregate receipt form; list one command per test name or a committed script"))
		return 2
	}
	expected, err := tracerecordrepo.AggregateExpectedFor(ctx, root, options.base, options.target, options.task, options.verification, options.outcome)
	if err == nil {
		var receipt []byte
		receipt, err = tracerecordrepo.ProduceAggregateOutcome(ctx, root, expected)
		if err == nil {
			if _, err = stdout.Write(receipt); err != nil {
				emitDogfoodRecordError(stderr, "output-failed", fmt.Errorf("cannot write dogfood record output"))
				return 2
			}
			return 0
		}
	}
	reason := err.Error()
	if !strings.HasPrefix(reason, "aggregate-") {
		reason = "aggregate-outcome-failed"
	}
	if fix := tracerecordrepo.AggregateSchemaFix(err); fix != "" {
		// ASD-V0-089: the code stays aggregate-schema-invalid; the message names the field and fix.
		err = fmt.Errorf("%s %s", reason, fix)
	}
	emitDogfoodRecordError(stderr, reason, err)
	return 2
}

// dogfoodRecordPayload builds the dogfood record receipt. A recorded state
// also discloses truncated_ancestry: the number of ancestry rows the bounded
// replay probe omitted while binding revisions, so a consumer reads the count
// as a receipt member instead of parsing a refusal message.
func dogfoodRecordPayload(result tracerecordrepo.DogfoodResult) map[string]any {
	payload := map[string]any{
		"ok": true, "tool": "dogfood-record", "state": result.State,
		"mutates": result.Recorded != nil, "base": result.Base, "target": result.Target, "tree": result.Tree,
		"candidate_sha256": result.CandidateSHA256, "admitted_sha256": result.AdmittedSHA256,
		"candidates": result.Candidates, "admitted": result.Admitted,
	}
	if result.Recorded != nil {
		payload["trace_id"] = result.Recorded.Record.TraceID
		payload["store"] = result.Recorded.Store
		payload["trace"] = recordMap(result.Recorded.Record)
		payload["truncated_ancestry"] = result.Recorded.TruncatedAncestry
	}
	return payload
}

func emitDogfoodRecordError(stderr io.Writer, reason string, err error) {
	if public := trace.PathLimitFailureReason(err); public != "" {
		reason = public
	}
	if reason == "" {
		reason = "dogfood-record-failed"
	}
	_, _ = fmt.Fprintf(
		stderr, "{\"code\": %s, \"error\": %s, \"ok\": false}\n",
		pythonJSONString(reason), pythonJSONString(err.Error()),
	)
}
