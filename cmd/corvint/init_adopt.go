package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/genesis"
	"github.com/Beamfall/corvint/internal/gokernel"
)

type activationOptions struct {
	authorityID      *string
	revision         string
	excludedPrefixes []string
	fullReceipt      bool
}

func parseActivationInvocation(arguments []string) (string, string, []string, bool, error) {
	index := 0
	root := ""
	for index < len(arguments) && (arguments[index] == "--root" || strings.HasPrefix(arguments[index], "--root=")) {
		if arguments[index] == "--root" {
			if !rootPreambleValue(arguments, index+1) {
				return "", "", nil, false, nil
			}
			root = arguments[index+1]
			index += 2
		} else {
			root = strings.TrimPrefix(arguments[index], "--root=")
			index++
		}
	}
	if index >= len(arguments) || arguments[index] != "init" && arguments[index] != "adopt" {
		return "", "", nil, false, nil
	}
	activation := arguments[index]
	if root == "" {
		workingDirectory, err := os.Getwd()
		if err != nil {
			return "", "", nil, true, argumentError("cannot resolve current directory")
		}
		root = workingDirectory
	} else {
		resolve := resolveExplicitRoot
		if !nativePlatformQualified(runtime.GOOS) {
			resolve = normalizeRoot
		}
		resolved, err := resolve(root)
		if err != nil {
			return "", "", nil, true, err
		}
		root = resolved
	}
	return root, activation, arguments[index+1:], true, nil
}

func parseActivationFlags(arguments []string) (activationOptions, error) {
	options := activationOptions{revision: "HEAD"}
	for index := 0; index < len(arguments); {
		argument := arguments[index]
		name, value, inline := strings.Cut(argument, "=")
		if name == "--full-receipt" {
			if inline {
				return options, argumentError("argument --full-receipt: ignored explicit argument " + pythonRepr(value))
			}
			options.fullReceipt = true
			index++
			continue
		}
		if name != "--authority-id" && name != "--revision" && name != "--exclude-prefix" {
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
		case "--authority-id":
			options.authorityID = &value
		case "--revision":
			options.revision = value
		case "--exclude-prefix":
			options.excludedPrefixes = append(options.excludedPrefixes, value)
		}
	}
	return options, nil
}

func runActivation(ctx context.Context, root, activation string, arguments []string, stdout, stderr io.Writer) int {
	return runActivationForPlatform(ctx, root, activation, arguments, runtime.GOOS, stdout, stderr)
}

func runActivationForPlatform(ctx context.Context, root, activation string, arguments []string, platform string, stdout, stderr io.Writer) int {
	options, err := parseActivationFlags(arguments)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if !nativePlatformQualified(platform) {
		emitError(stderr, genesisPlatformRefusal(platform))
		return 2
	}
	receipt := genesis.CompileRepositoryInventory(ctx, root, activation, options.authorityID, options.revision, options.excludedPrefixes)
	inventory := any(receipt)
	if !options.fullReceipt {
		inventory, err = genesis.SummarizeInventoryReceipt(receipt, 5)
		if err != nil {
			emitError(stderr, &gokernel.Error{Code: "internal-error", Message: err.Error()})
			return 2
		}
	}
	ok := receipt["operationalState"] != "INVALID"
	payload := map[string]any{"ok": ok, "mutates": false, "tool": activation, "inventory": inventory}
	encoded, err := contextindex.CanonicalJSON(payload)
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write " + activation + " output"})
		return 2
	}
	written, err := fmt.Fprintf(stdout, "%s\n", encoded)
	if err != nil || written != len(encoded)+1 {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write " + activation + " output"})
		return 2
	}
	if !ok {
		return 1
	}
	return 0
}
