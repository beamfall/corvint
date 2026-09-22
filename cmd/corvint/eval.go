package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/evalrepo"
	"github.com/Beamfall/corvint/internal/gokernel"
)

func runEvalInvocation(ctx context.Context, arguments []string, stdout, stderr io.Writer) (int, bool) {
	root, rest, handled, err := parseEvalInvocation(arguments)
	if !handled {
		return 0, false
	}
	if err != nil {
		emitError(stderr, err)
		return 2, true
	}
	options, err := parseEvalFlags(root, rest)
	if err != nil {
		var argument *gokernel.Error
		if errors.As(err, &argument) {
			emitError(stderr, err)
		} else {
			emitEvalError(stderr, err)
		}
		return 2, true
	}
	var report map[string]any
	if options.fixture == "" {
		report, err = evalrepo.Evaluate(ctx, root, options.golden)
	} else {
		report, err = evalrepo.Evaluate(ctx, root, options.golden, options.fixture)
	}
	if err != nil {
		emitEvalError(stderr, err)
		return 2, true
	}
	payload := map[string]any{"ok": true, "mutates": false, "tool": "eval", "evaluation": report}
	encoded, err := evalrepo.Encode(payload)
	if err != nil {
		emitEvalError(stderr, fmt.Errorf("cannot write eval output"))
		return 2, true
	}
	encoded = append(encoded, '\n')
	written, err := stdout.Write(encoded)
	if err != nil || written != len(encoded) {
		emitEvalError(stderr, fmt.Errorf("cannot write eval output"))
		return 2, true
	}
	return 0, true
}

func parseEvalInvocation(arguments []string) (string, []string, bool, error) {
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
	if index >= len(arguments) || arguments[index] != "eval" {
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

type evalFlags struct{ golden, fixture string }

func parseEvalFlags(root string, arguments []string) (evalFlags, error) {
	requestedGolden, requestedFixture := "", ""
	goldenSet, fixtureSet := false, false
	for index := 0; index < len(arguments); {
		argument := arguments[index]
		name, value, inline := strings.Cut(argument, "=")
		if name != "--goldens" && name != "--trace-fixture" {
			return evalFlags{}, argumentError("unrecognized arguments: " + argument)
		}
		if !inline {
			if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
				return evalFlags{}, argumentError("argument " + name + ": expected one argument")
			}
			value = arguments[index+1]
			index += 2
		} else {
			index++
		}
		if name == "--goldens" {
			requestedGolden, goldenSet = value, true
		} else {
			requestedFixture, fixtureSet = value, true
		}
	}
	options := evalFlags{}
	if goldenSet {
		path := resolveGoldenPath(root, requestedGolden)
		if regularFile(path) {
			options.golden = path
		} else {
			return evalFlags{}, fmt.Errorf("evaluation corpus does not exist: %s", requestedGolden)
		}
	} else {
		for _, candidate := range []string{filepath.Join(root, ".corvint", "eval.json"), filepath.Join(root, "testing", "context-retrieval-goldens.json")} {
			path := resolveGoldenPath(root, candidate)
			if regularFile(path) {
				options.golden = path
				break
			}
		}
		if options.golden == "" {
			return evalFlags{}, fmt.Errorf("evaluation corpus does not exist: .corvint/eval.json")
		}
	}
	if fixtureSet {
		path := resolveGoldenPath(root, requestedFixture)
		if !regularFile(path) {
			return evalFlags{}, fmt.Errorf("learned trace fixture does not exist: %s", requestedFixture)
		}
		options.fixture = path
	}
	return options, nil
}

func resolveGoldenPath(root, value string) string {
	if value == "~" || strings.HasPrefix(value, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			value = filepath.Join(home, strings.TrimPrefix(value, "~/"))
		}
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(root, value)
	}
	value, _ = filepath.Abs(filepath.Clean(value))
	if resolved, err := filepath.EvalSymlinks(value); err == nil {
		return resolved
	}
	return value
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func emitEvalError(stderr io.Writer, err error) {
	_, _ = fmt.Fprintf(stderr, "{\"error\": %s, \"ok\": false}\n", pythonJSONString(err.Error()))
}
