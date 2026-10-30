package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Beamfall/corvint/internal/dogfoodocm"
	"github.com/Beamfall/corvint/internal/lrfrepo"
)

func parseDogfoodOCMInvocation(arguments []string) (string, []string, bool, error) {
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
	if index >= len(arguments) || arguments[index] != "dogfood-ocm" {
		return "", nil, false, nil
	}
	resolved, err := normalizeRoot(root)
	return resolved, arguments[index+1:], true, err
}

func runDogfoodOCM(ctx context.Context, root string, arguments []string, stdout, stderr io.Writer) int {
	options, err := parseDogfoodOCMFlags(root, arguments)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	result, err := dogfoodocm.Status(ctx, options)
	if err != nil {
		emitOCMError(stderr, err)
		return 2
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		emitOCMError(stderr, argumentError("cannot encode dogfood OCM status"))
		return 2
	}
	if _, err := fmt.Fprintf(stdout, "%s\n", encoded); err != nil {
		emitOCMError(stderr, &lrfrepo.Error{Code: "output-failed", Message: "cannot write dogfood OCM output"})
		return 2
	}
	return 0
}

func parseDogfoodOCMFlags(root string, arguments []string) (dogfoodocm.Options, error) {
	result := dogfoodocm.Options{Root: root, CEMPath: ".corvint/change.cem.json"}
	if len(arguments) == 0 || arguments[0] != "status" {
		return result, argumentError("dogfood-ocm requires status")
	}
	for index := 1; index < len(arguments); {
		name := arguments[index]
		if name != "--expected-base" && name != "--target" {
			return result, argumentError("unrecognized arguments: " + strings.Join(arguments[index:], " "))
		}
		if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
			return result, argumentError("argument " + name + ": expected one argument")
		}
		value := arguments[index+1]
		if name == "--expected-base" {
			result.ExpectedBase = value
		} else {
			result.Target = value
		}
		index += 2
	}
	if result.ExpectedBase == "" {
		return result, argumentError("--expected-base is required")
	}
	if result.Target == "" {
		return result, argumentError("--target is required")
	}
	return result, nil
}
