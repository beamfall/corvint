package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Beamfall/corvint/internal/stepverify"
)

type stepOptions struct{ action, declaration, host, before string }

// Registration with Core help and maturity is required before dispatching this
// handler. The declaration contains absolute roots; --root only selects the
// ordinary CLI preamble and cannot override a host-owned observation scope.
func parseStepInvocation(arguments []string) (stepOptions, bool, error) {
	var options stepOptions
	index := 0
	for index < len(arguments) && (arguments[index] == "--root" || strings.HasPrefix(arguments[index], "--root=")) {
		root := ""
		if arguments[index] == "--root" {
			if !rootPreambleValue(arguments, index+1) {
				return options, false, nil
			}
			root = arguments[index+1]
			index += 2
		} else {
			root = strings.TrimPrefix(arguments[index], "--root=")
			index++
		}
		if _, err := normalizeRoot(root); err != nil {
			return options, false, nil
		}
	}
	if index >= len(arguments) || arguments[index] != "step" {
		return options, false, nil
	}
	index++
	if index >= len(arguments) {
		return options, true, stepverify.ErrInput
	}
	options.action = arguments[index]
	index++
	if options.action != "snapshot" && options.action != "verify" && options.action != "env-check" {
		return options, true, stepverify.ErrInput
	}
	seen := map[string]bool{}
	for index < len(arguments) {
		key, value, inline := strings.Cut(arguments[index], "=")
		index++
		if !inline {
			if index >= len(arguments) || strings.HasPrefix(arguments[index], "-") {
				return options, true, stepverify.ErrInput
			}
			value = arguments[index]
			index++
		}
		if seen[key] || value == "" {
			return options, true, stepverify.ErrInput
		}
		seen[key] = true
		switch key {
		case "--declaration":
			options.declaration = value
		case "--host":
			options.host = value
		case "--before":
			options.before = value
		default:
			return options, true, stepverify.ErrInput
		}
	}
	if options.declaration == "" || options.host == "" || options.action == "verify" && options.before == "" || options.action != "verify" && options.before != "" {
		return options, true, stepverify.ErrInput
	}
	return options, true, nil
}
func stepError(stderr io.Writer, err error) int {
	code := "STEP_INPUT"
	if errors.Is(err, stepverify.ErrUnsupported) || errors.Is(err, stepverify.ErrDrift) {
		code = "STEP_UNSUPPORTED"
	}
	fmt.Fprintf(stderr, "{\"ok\":false,\"code\":\"%s\"}\n", code)
	return 2
}
func runStep(ctx context.Context, options stepOptions, stdout, stderr io.Writer) int {
	data, err := stepverify.ReadInput(ctx, options.declaration, nil)
	if err != nil {
		return stepError(stderr, err)
	}
	declaration, err := stepverify.DecodeDeclaration(data)
	if err != nil {
		return stepError(stderr, err)
	}
	var before stepverify.State
	var host stepverify.Host
	if options.action == "verify" {
		// Host-protected inputs are bootstrapped through bounded no-follow reads;
		// the complete pre-state then binds all three external authority boundaries.
		hostData, readErr := stepverify.ReadInput(ctx, options.host, nil)
		if readErr != nil {
			return stepError(stderr, readErr)
		}
		host, err = stepverify.DecodeHost(hostData, declaration)
		if err != nil {
			return stepError(stderr, err)
		}
		beforeData, readErr := stepverify.ReadInput(ctx, options.before, nil)
		if readErr != nil {
			return stepError(stderr, readErr)
		}
		before, err = stepverify.DecodeState(beforeData, declaration, host)
		if err != nil {
			return stepError(stderr, err)
		}
		for _, input := range []struct {
			path string
			data []byte
		}{{options.declaration, data}, {options.host, hostData}, {options.before, beforeData}} {
			confirmed, readErr := stepverify.ReadBoundInput(ctx, input.path, declaration, host, before)
			if readErr != nil {
				return stepError(stderr, readErr)
			}
			if !bytes.Equal(input.data, confirmed) {
				return stepError(stderr, stepverify.ErrDrift)
			}
		}
	} else {
		// Declaration bootstrap is re-read against admitted current authority before use.
		confirmed, readErr := stepverify.ReadInput(ctx, options.declaration, &declaration)
		if readErr != nil {
			return stepError(stderr, readErr)
		}
		if !bytes.Equal(data, confirmed) {
			return stepError(stderr, stepverify.ErrDrift)
		}
		hostData, readErr := stepverify.ReadInput(ctx, options.host, &declaration)
		if readErr != nil {
			return stepError(stderr, readErr)
		}
		host, err = stepverify.DecodeHost(hostData, declaration)
		if err != nil {
			return stepError(stderr, err)
		}
	}
	var value any
	exit := 0
	switch options.action {
	case "snapshot":
		state, captureErr := stepverify.Snapshot(ctx, declaration, host)
		if captureErr != nil && state.Profile == "" {
			return stepError(stderr, captureErr)
		}
		value = state
		if captureErr != nil {
			exit = 2
		}
	case "verify":
		receipt, code, verifyErr := stepverify.Verify(ctx, declaration, host, before)
		if verifyErr != nil {
			return stepError(stderr, verifyErr)
		}
		value = receipt
		exit = code
	case "env-check":
		receipt, code, checkErr := stepverify.Preflight(declaration, host)
		if checkErr != nil {
			return stepError(stderr, checkErr)
		}
		value = receipt
		exit = code
	default:
		return stepError(stderr, stepverify.ErrInput)
	}
	encoded, err := stepverify.BoundedEncode(value)
	if err != nil {
		return stepError(stderr, err)
	}
	if _, err := stdout.Write(encoded); err != nil {
		return stepError(stderr, stepverify.ErrUnsupported)
	}
	return exit
}
