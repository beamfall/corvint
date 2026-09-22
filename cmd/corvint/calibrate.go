package main

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/outcomecal"
	"github.com/Beamfall/corvint/internal/trace"
	"github.com/Beamfall/corvint/internal/tracerecordrepo"
)

type calibrateInvocation struct {
	root   string
	since  string
	window int
	format string
}

// calibrateInvoked reports whether the argument vector selects the calibrate
// verb, mirroring the leading `--root` handling of the observations verb.
func calibrateInvoked(arguments []string) bool {
	_, index := calibrateRoot(arguments)
	return index < len(arguments) && arguments[index] == "calibrate"
}

func calibrateRoot(arguments []string) (string, int) {
	root, index := "", 0
	for index < len(arguments) && (arguments[index] == "--root" || strings.HasPrefix(arguments[index], "--root=")) {
		if arguments[index] == "--root" {
			if !rootPreambleValue(arguments, index+1) {
				return root, index
			}
			root, index = arguments[index+1], index+2
			continue
		}
		root, index = strings.TrimPrefix(arguments[index], "--root="), index+1
	}
	return root, index
}

func parseCalibrateInvocation(arguments []string) (calibrateInvocation, error) {
	root, index := calibrateRoot(arguments)
	invocation := calibrateInvocation{format: "json"}
	if root == "" {
		resolved, err := normalizeRoot(".")
		if err != nil {
			return invocation, argumentError("cannot resolve current directory")
		}
		invocation.root = resolved
	} else {
		resolved, err := resolveExplicitRoot(root)
		if err != nil {
			return invocation, err
		}
		invocation.root = resolved
	}
	for index++; index < len(arguments); index++ {
		name, value, inline := strings.Cut(arguments[index], "=")
		if name != "--since" && name != "--window" && name != "--format" {
			return invocation, argumentError("unrecognized arguments: " + arguments[index])
		}
		if !inline {
			if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
				return invocation, argumentError("argument " + name + ": expected one argument")
			}
			value, index = arguments[index+1], index+1
		}
		if err := invocation.apply(name, value); err != nil {
			return invocation, err
		}
	}
	if invocation.since != "" && invocation.window != 0 {
		return invocation, argumentError("arguments --since and --window are mutually exclusive")
	}
	return invocation, nil
}

func (invocation *calibrateInvocation) apply(name, value string) error {
	switch name {
	case "--since":
		if strings.TrimSpace(value) == "" {
			return argumentError("argument --since: expected a revision")
		}
		invocation.since = value
	case "--window":
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > trace.MaxTraces {
			return argumentError("invalid --window")
		}
		invocation.window = parsed
	default:
		if value != "json" && value != "table" {
			return argumentError("argument --format: invalid choice: " + pythonRepr(value) + " (choose from 'json', 'table')")
		}
		invocation.format = value
	}
	return nil
}

// runCalibrate reports how recorded outcomes line up with the packet stance
// that preceded them. It reads the pinned local trace store and the committed
// index only; it writes nothing and never applies a proposed threshold. ctx
// is main's signal context, so SIGINT and SIGTERM cancel the load like every
// other packet verb.
func runCalibrate(ctx context.Context, arguments []string, stdout, stderr io.Writer) int {
	invocation, err := parseCalibrateInvocation(arguments)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	stderr = recordingStderr(invocation.root, stderr)
	type recorded struct {
		records []trace.Record
		state   string
	}
	read, err := overSnapshot(
		func() *contextindex.Index { return deferredSnapshotIndex(ctx, invocation.root) },
		func() *contextindex.Index { return snapshotIndex(ctx, invocation.root) },
		func() (*contextindex.Index, error) { return contextindex.BuildEval(ctx, invocation.root) },
		func(index *contextindex.Index) (recorded, error) {
			records, state, err := tracerecordrepo.Read(ctx, invocation.root, index)
			if err != nil {
				return recorded{}, mapRepositoryQueryTraceError(err)
			}
			return recorded{records, state}, nil
		},
	)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	report := calibrateReport(read.records, read.state, invocation)
	return writeCalibrate(report, invocation.format, stdout, stderr)
}

func calibrateReport(records []trace.Record, state string, invocation calibrateInvocation) outcomecal.Report {
	selected := selectCalibrateRecords(records, invocation)
	observations := make([]outcomecal.Observation, len(selected))
	for offset, record := range selected {
		observations[offset] = outcomecal.Observe(record)
	}
	report := outcomecal.Build(observations, state)
	report.Since, report.Window = invocation.since, invocation.window
	return report
}

// selectCalibrateRecords bounds the sample. The local store pins one revision,
// so `--since` selects only the records already bound to that revision; a
// different revision selects nothing rather than inventing history.
func selectCalibrateRecords(records []trace.Record, invocation calibrateInvocation) []trace.Record {
	if invocation.since != "" {
		matched := make([]trace.Record, 0, len(records))
		for _, record := range records {
			if record.Revision == invocation.since {
				matched = append(matched, record)
			}
		}
		return matched
	}
	if invocation.window != 0 && invocation.window < len(records) {
		return records[len(records)-invocation.window:]
	}
	return records
}

func writeCalibrate(report outcomecal.Report, format string, stdout, stderr io.Writer) int {
	rendered := []byte(report.Table())
	if format == "json" {
		encoded, err := contextindex.CanonicalJSON(report.Payload())
		if err != nil {
			emitError(stderr, fmt.Errorf("cannot encode calibration output"))
			return 2
		}
		rendered = append(encoded, '\n')
	}
	written, err := stdout.Write(rendered)
	if err != nil || written != len(rendered) {
		emitError(stderr, fmt.Errorf("cannot write calibration output"))
		return 2
	}
	return 0
}

const calibrateHelp = `
Outcome calibration (experimental, OCL-V0 proposed), read-only, JSON/text:

  corvint [--root PATH] calibrate [--since REV | --window N] [--format json|table]

Compares the packet stance recorded before each local outcome with the outcome
that followed, and reports where the stance and the result disagree.

  --since REV      Select only records bound to that revision.
  --window N       Select the newest N records.
  --format FORMAT  json (default) or table.

It reads the pinned local trace store and the committed index only. A proposed
threshold is reported and never applied; applying one stays behind the
learned-trace admission gate.
`
