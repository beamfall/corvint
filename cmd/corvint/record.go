package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/trace"
	"github.com/Beamfall/corvint/internal/tracerecordrepo"
)

type recordOptions struct {
	task         string
	opened       []string
	changed      []string
	verification []string
	outcome      string
	taskSet      bool
	changedSet   bool
	verifySet    bool
	outcomeSet   bool
}

func parseRecordInvocation(arguments []string) (string, []string, bool, error) {
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
	if index >= len(arguments) || arguments[index] != "record" {
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

func parseRecordFlags(arguments []string) (recordOptions, error) {
	var options recordOptions
	for index := 0; index < len(arguments); {
		argument := arguments[index]
		name, value, inline := strings.Cut(argument, "=")
		if name != "--task" && name != "--opened" && name != "--changed" && name != "--verify" && name != "--outcome" {
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
		case "--task":
			options.task, options.taskSet = value, true
		case "--opened":
			path, err := normalizeRecordPath(value)
			if err != nil {
				return options, argumentError("argument --opened: " + err.Error())
			}
			options.opened = append(options.opened, path)
		case "--changed":
			path, err := normalizeRecordPath(value)
			if err != nil {
				return options, argumentError("argument --changed: " + err.Error())
			}
			options.changed = append(options.changed, path)
			options.changedSet = true
		case "--verify":
			options.verification = append(options.verification, value)
			options.verifySet = true
		case "--outcome":
			if value != "passed" && value != "failed" && value != "blocked" {
				return options, argumentError("argument --outcome: invalid choice: " + pythonRepr(value) + " (choose from 'passed', 'failed', 'blocked')")
			}
			options.outcome, options.outcomeSet = value, true
		}
	}
	missing := make([]string, 0, 4)
	if !options.taskSet {
		missing = append(missing, "--task")
	}
	if !options.changedSet {
		missing = append(missing, "--changed")
	}
	if !options.verifySet {
		missing = append(missing, "--verify")
	}
	if !options.outcomeSet {
		missing = append(missing, "--outcome")
	}
	if len(missing) != 0 {
		return options, argumentError("the following arguments are required: " + strings.Join(missing, ", "))
	}
	return options, nil
}

func normalizeRecordPath(value string) (string, error) {
	if strings.TrimSpace(value) == "" || filepath.IsAbs(value) {
		return "", fmt.Errorf("path must be repository-relative")
	}
	for _, part := range strings.Split(filepath.ToSlash(value), "/") {
		if part == ".." {
			return "", fmt.Errorf("path must be repository-relative")
		}
	}
	normalized := filepath.ToSlash(filepath.Clean(value))
	if normalized == "." {
		return "", fmt.Errorf("path must be repository-relative")
	}
	return normalized, nil
}

func runRecord(ctx context.Context, root string, arguments []string, stdout, stderr io.Writer) int {
	options, err := parseRecordFlags(arguments)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	result, err := tracerecordrepo.Record(ctx, root, tracerecordrepo.Input{
		Task: options.task, OpenedPaths: options.opened, ChangedPaths: options.changed,
		Verification: options.verification, Outcome: options.outcome,
	})
	if err != nil {
		emitRecordError(stderr, err)
		return 2
	}
	encoded, err := contextindex.CanonicalJSON(recordPayload(result))
	if err != nil {
		emitRecordError(stderr, fmt.Errorf("cannot encode trace record output"))
		return 2
	}
	written, err := fmt.Fprintf(stdout, "%s\n", encoded)
	if err != nil || written != len(encoded)+1 {
		emitRecordError(stderr, fmt.Errorf("cannot write trace record output"))
		return 2
	}
	return 0
}

func recordPayload(result tracerecordrepo.Result) map[string]any {
	return map[string]any{
		"ok": true, "mutates": true, "tool": "record", "trace_id": result.Record.TraceID,
		"store": result.Store, "trace": recordMap(result.Record),
	}
}

func recordMap(record trace.Record) map[string]any {
	return map[string]any{
		"schema_version": record.SchemaVersion, "revision": record.Revision, "trace_id": record.TraceID,
		"task": record.Task, "opened_paths": record.OpenedPaths, "changed_paths": record.ChangedPaths,
		"verification": record.Verification, "outcome": record.Outcome,
	}
}

// emitRecordError renders a trace-record refusal. A verification-command
// refusal carries its bounded reason as code (LTPM-V0-011), so the SOL-V0-007
// boundary can record it; every other refusal keeps the code-free envelope.
func emitRecordError(stderr io.Writer, err error) {
	if reason := trace.VerificationFailureReason(err); reason != "" {
		_, _ = fmt.Fprintf(stderr, "{\"code\": %s, \"error\": %s, \"ok\": false}\n", pythonJSONString(reason), pythonJSONString(err.Error()))
		return
	}
	_, _ = fmt.Fprintf(stderr, "{\"error\": %s, \"ok\": false}\n", pythonJSONString(err.Error()))
}
