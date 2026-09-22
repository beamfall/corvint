package main

import (
	"context"
	"io"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/necessity"
)

// necessityMaxLimit is the packet compiler's own bound. Accepting more here
// would refuse only after a full repository read, which NEC-V0-001 forbids.
const necessityMaxLimit = 50

// parseNecessityInvocation recognizes `[--root PATH] necessity --task TEXT
// [--subject PATH] [--limit N]` and mirrors observations.go's argument
// conventions.
func parseNecessityInvocation(arguments []string) (necessity.Options, bool, error) {
	options := necessity.Options{Limit: necessity.DefaultLimit}
	root := ""
	index := 0
	for index < len(arguments) && (arguments[index] == "--root" || strings.HasPrefix(arguments[index], "--root=")) {
		if arguments[index] == "--root" {
			if !rootPreambleValue(arguments, index+1) {
				return options, false, nil
			}
			root, index = arguments[index+1], index+2
			continue
		}
		root, index = strings.TrimPrefix(arguments[index], "--root="), index+1
	}
	if index >= len(arguments) || arguments[index] != "necessity" {
		return options, false, nil
	}
	resolved, err := resolveNecessityRoot(root)
	if err != nil {
		return options, true, err
	}
	options.Root = resolved
	taskSet := false
	for index++; index < len(arguments); index++ {
		name, value, inline := strings.Cut(arguments[index], "=")
		if name != "--task" && name != "--subject" && name != "--limit" {
			return options, true, argumentError("unrecognized arguments: " + arguments[index])
		}
		if !inline {
			if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
				return options, true, argumentError("argument " + name + ": expected one argument")
			}
			value, index = arguments[index+1], index+1
		}
		switch name {
		case "--task":
			options.Task, taskSet = value, true
		case "--subject":
			options.Subject = value
		default:
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 1 || parsed > necessityMaxLimit {
				return options, true, argumentError("invalid --limit")
			}
			options.Limit = parsed
		}
	}
	if !taskSet {
		return options, true, argumentError("the following arguments are required: --task")
	}
	return options, true, nil
}

func resolveNecessityRoot(root string) (string, error) {
	if root == "" {
		return normalizeRoot(".")
	}
	return resolveExplicitRoot(root)
}

// runNecessity emits the task-context packet with a counterfactual necessity
// label on every included path. It is read-only: an abstaining packet is
// emitted unchanged with no labels and exit status 0, and only an argument or
// repository failure is exit status 2. ctx is main's signal context, so SIGINT
// and SIGTERM cancel the load like every other packet verb.
func runNecessity(ctx context.Context, arguments []string, stdout, stderr io.Writer) int {
	options, isNecessity, err := parseNecessityInvocation(arguments)
	if !isNecessity {
		return 2
	}
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if err := necessity.Render(ctx, options, stdout); err != nil {
		emitError(stderr, err)
		return 2
	}
	return 0
}

const necessityHelp = `Label every file the task-context packet included by counterfactual removal.

Usage:
  corvint [--root PATH] necessity --task TEXT [--subject PATH] [--limit N]

Read-only (necessity-labels-v0, experimental). The verb compiles the ordinary
context packet, then recompiles it once per included path over a copy of the
index with that path removed:

  load-bearing  the packet lost a critical anchor, gained a missing critical
                selector, or left its resolved state or verdict
  supporting    the packet kept its anchors, criticals, state and verdict
  unlabelled    the counterfactual budget was spent before this path

--limit bounds both the packet and the number of recompiles (default 12, hard
ceiling 12 recompiles). A packet that abstains is emitted unchanged with no
labels. Labels are counterfactual under the current ranker, never proof that a
file is or is not relevant to a reader.
`
