package main

import (
	"context"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/observations"
)

func parseObservationsInvocation(arguments []string) (string, int, bool, error) {
	root := ""
	index := 0
	for index < len(arguments) && (arguments[index] == "--root" || strings.HasPrefix(arguments[index], "--root=")) {
		value := ""
		if arguments[index] == "--root" {
			if !rootPreambleValue(arguments, index+1) {
				return "", 0, false, nil
			}
			value, index = arguments[index+1], index+2
		} else {
			value, index = strings.TrimPrefix(arguments[index], "--root="), index+1
		}
		root = value
	}
	if index >= len(arguments) || arguments[index] != "observations" {
		return "", 0, false, nil
	}
	if root == "" {
		var err error
		root, err = normalizeRoot(".")
		if err != nil {
			return "", 0, true, err
		}
	} else {
		resolved, err := resolveExplicitRoot(root)
		if err != nil {
			return "", 0, true, err
		}
		root = resolved
	}
	limit := 120
	for index++; index < len(arguments); index++ {
		name, value, inline := strings.Cut(arguments[index], "=")
		if name != "--limit" {
			return "", 0, true, argumentError("unrecognized arguments: " + arguments[index])
		}
		if !inline {
			if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
				return "", 0, true, argumentError("argument --limit: expected one argument")
			}
			value, index = arguments[index+1], index+1
		}
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 120 {
			return "", 0, true, argumentError("invalid --limit")
		}
		limit = parsed
	}
	return root, limit, true, nil
}

func runObservations(_ context.Context, root string, limit int, stdout, stderr interface{ Write([]byte) (int, error) }) int {
	if err := observations.Render(root, limit, stdout); err != nil {
		emitError(stderr, err)
		return 2
	}
	return 0
}
