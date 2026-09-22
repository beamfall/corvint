package main

import (
	"context"
	"io"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/observations"
)

var dogfoodObservationCode = regexp.MustCompile(`^[A-Za-z0-9_-]{1,96}$`)

func parseDogfoodObserveInvocation(arguments []string) (string, []string, bool, error) {
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
	if index >= len(arguments) || arguments[index] != "dogfood-observe" {
		return "", nil, false, nil
	}
	resolved, err := normalizeRoot(root)
	return resolved, arguments[index+1:], true, err
}

func runDogfoodObserve(_ context.Context, root string, arguments []string, _ io.Writer, stderr io.Writer) int {
	event, err := parseDogfoodObserveFlags(arguments)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if err := observations.Append(root, event); err != nil {
		emitError(stderr, err)
		return 2
	}
	return 0
}

func parseDogfoodObserveFlags(arguments []string) (observations.Event, error) {
	event := observations.Event{Kind: "dogfood-step"}
	for index := 0; index < len(arguments); {
		name := arguments[index]
		if name != "--step" && name != "--status" && name != "--reason" {
			return event, argumentError("unrecognized arguments: " + strings.Join(arguments[index:], " "))
		}
		if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
			return event, argumentError("argument " + name + ": expected one argument")
		}
		value := arguments[index+1]
		switch name {
		case "--step":
			event.Step = value
		case "--status":
			event.Status = value
		case "--reason":
			event.Reason = value
		}
		index += 2
	}
	if !dogfoodObservationCode.MatchString(event.Step) {
		return event, argumentError("invalid --step")
	}
	if event.Status != "PRODUCED" && event.Status != "NOT_PRODUCED" {
		return event, argumentError("invalid --status")
	}
	if !dogfoodObservationCode.MatchString(event.Reason) {
		return event, argumentError("invalid --reason")
	}
	return event, nil
}
