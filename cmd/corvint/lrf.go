package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/lrf"
	"github.com/Beamfall/corvint/internal/lrfrepo"
)

func parseLRFInvocation(arguments []string) (string, []string, bool, error) {
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
	if index >= len(arguments) || arguments[index] != "lrf" {
		return "", nil, false, nil
	}
	if root == "" {
		workingDirectory, err := os.Getwd()
		if err != nil {
			return "", nil, true, argumentError("cannot resolve current directory")
		}
		root = workingDirectory
	} else {
		resolved, err := normalizeRoot(root)
		if err != nil {
			return "", nil, true, err
		}
		root = resolved
	}
	return root, arguments[index+1:], true, nil
}

func runLRF(ctx context.Context, root string, arguments []string, stdout, stderr io.Writer) int {
	options, err := parseLRFFlags(arguments)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	result, err := lrfrepo.Evaluate(ctx, root, options)
	if err != nil {
		emitLRFError(stderr, err)
		return 2
	}
	if ctx.Err() != nil {
		emitLRFError(stderr, &lrfrepo.Error{Code: "lrf-operational-failure", Message: "LRF evaluation was interrupted"})
		return 2
	}
	encoded := lrf.CanonicalBytes(result)
	written, err := stdout.Write(encoded)
	if err != nil || written != len(encoded) {
		emitLRFError(stderr, &lrfrepo.Error{Code: "output-failed", Message: "cannot write LRF output"})
		return 2
	}
	return result.ExitCode()
}

func parseLRFFlags(arguments []string) (lrfrepo.Options, error) {
	var options lrfrepo.Options
	for index := 0; index < len(arguments); {
		name, value, inline := strings.Cut(arguments[index], "=")
		switch name {
		case "--cem", "--ocm", "--patch", "--expected-base", "--target":
		default:
			return options, argumentError("unrecognized arguments: " + arguments[index])
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
		case "--cem":
			options.CEMPath = value
		case "--ocm":
			options.OCMPath = value
		case "--patch":
			options.PatchPath, options.PatchGiven = value, true
		case "--expected-base":
			options.ExpectedBase = value
		case "--target":
			options.Target = value
		}
	}
	if options.CEMPath == "" {
		return options, argumentError("the following arguments are required: --cem")
	}
	return options, nil
}

func emitLRFError(stderr io.Writer, err error) {
	code := lrfrepo.CodeOf(err)
	if code == "" {
		code = "lrf-operational-failure"
	}
	_, _ = fmt.Fprintf(stderr, "{\"code\": %s, \"error\": %s, \"ok\": false}\n",
		wire.CanonicalString(code), wire.CanonicalString(err.Error()))
}
