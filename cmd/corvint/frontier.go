// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/frontier"
)

// maxFrontierInputBytes is the largest inherited raw ceiling any Frontier
// input carries (CEM 4 MiB; OCM 1 MiB; TCQ command 64 KiB, observation and
// report 4 MiB each). The wrapper reads one byte PAST it rather than enforcing
// it: every ceiling belongs to the producer that declared it, so an oversized
// artifact must still fail with that producer's own code under the CF-V0-022
// translation table, never with a code this wrapper invented.
const maxFrontierInputBytes = 4 << 20

// frontierAdapters injects the repository-backed shared CEM/OCM verifier and
// the TCQ recomputer that `frontier.Compute` requires.
//
// It is a seam rather than a direct import because CF-V0-026 keeps repository
// access and path acquisition outside the Frontier wire and verifier, and
// because the Git-backed adapters land independently of this command surface.
// While it is nil the command REFUSES: CF-V0-001 requires one validated
// repository boundary, and a command that cannot establish one must report an
// unsupported context rather than emit a frontier nothing verified.
//
// The adapters land as ONE new file in this package that assigns this variable
// from an init function. Nothing else here changes.
var frontierAdapters func(ctx context.Context, root string) (frontier.Verifier, frontier.TCQRecomputer)

// frontierSeam is the type of frontierAdapters.
type frontierSeam = func(ctx context.Context, root string) (frontier.Verifier, frontier.TCQRecomputer)

// frontierAdaptersKey carries a replacement seam for one invocation on its
// context. Nothing in the command sets it; a test does, so tests that swap or
// clear the seam never write frontierAdapters and can run in parallel. A nil
// replacement is the unregistered state.
type frontierAdaptersKey struct{}

// frontierOptions is the wrapper's own argument surface. It holds paths and a
// rendering choice only: every value that reaches the wire is read, verified,
// and bound by internal/frontier.
type frontierOptions struct {
	cem          string
	ocm          string
	expectedBase string
	target       string
	// The dynamic test bundle is the all-or-none CF-V0-003 tuple. The wrapper
	// deliberately does not check completeness itself; it passes exactly what
	// the caller supplied so the library owns the `invalid-frontier-input`
	// verdict for every partial combination.
	command     string
	observation string
	report      string
	json        bool
}

// parseFrontierInvocation intercepts `frontier` ahead of the shared argument
// parser, the way parseWitnessInvocation does, so the command adds no verb to
// the parity-compared top-level vocabulary.
//
// Unlike the witness parser it returns no error, and returns the RAW `--root` value.
// The shared emitError path renders err.Error() into its envelope, and
// CF-V0-024 forbids echoing an unverified path or argv element; routing every
// frontier failure through the CF-V0-022 envelope instead keeps that rule
// structural rather than a review obligation.
func parseFrontierInvocation(arguments []string) (string, []string, bool) {
	index, root := 0, ""
	for index < len(arguments) && (arguments[index] == "--root" || strings.HasPrefix(arguments[index], "--root=")) {
		if arguments[index] == "--root" {
			if !rootPreambleValue(arguments, index+1) {
				return "", nil, false
			}
			root, index = arguments[index+1], index+2
			continue
		}
		root, index = strings.TrimPrefix(arguments[index], "--root="), index+1
	}
	if index >= len(arguments) || arguments[index] != "frontier" {
		return "", nil, false
	}
	return root, arguments[index+1:], true
}

// runFrontier is the whole command: build one request, run one computation,
// render it once. CF-V0-026 makes JSON and human two renderings of ONE
// computation, so the mode is chosen only at the final write and nothing is
// recomputed for it.
func runFrontier(ctx context.Context, root string, arguments []string, stdout, stderr io.Writer) int {
	jsonMode := frontierJSONRequested(arguments)
	request, buildErr := buildFrontierRequest(ctx, root, arguments)
	if buildErr != nil {
		return emitFrontierFailure(stderr, buildErr, jsonMode)
	}
	document, encoded, computeErr := frontier.Compute(ctx, request)
	if computeErr != nil {
		return emitFrontierFailure(stderr, computeErr, jsonMode)
	}
	if writeErr := writeFrontierResult(stdout, document, encoded, jsonMode); writeErr != nil {
		return emitFrontierFailure(stderr, writeErr, jsonMode)
	}
	// CF-V0-004: 0 is valid EMPTY and 1 is valid OPEN. Exit 1 is a SUCCESSFUL
	// result carrying items, so it is returned from the document rather than
	// from any error path — 2 is reserved for operational failure alone.
	return document.ExitCode()
}

// frontierJSONRequested pre-scans argv for the rendering mode. Argument
// parsing can itself fail, and a caller that asked for JSON must still receive
// the machine-readable CF-V0-022 envelope when the failure is in its own
// argument vector; deriving the mode from the parse result would silently
// hand it a human sentence instead.
func frontierJSONRequested(arguments []string) bool {
	for _, argument := range arguments {
		if argument == "--json" || strings.HasPrefix(argument, "--json=") {
			return true
		}
	}
	return false
}

// buildFrontierRequest turns argv into the one CF-V0-001 invocation. Required
// inputs are deliberately NOT checked here: an absent artifact arrives as
// absent bytes and the library issues the verdict, so the wrapper cannot drift
// from the cascade's own admitted-input rules.
func buildFrontierRequest(ctx context.Context, root string, arguments []string) (frontier.Request, error) {
	options, parseErr := parseFrontierOptions(arguments)
	if parseErr != nil {
		return frontier.Request{}, parseErr
	}
	resolvedRoot, rootErr := resolveFrontierRoot(root)
	if rootErr != nil {
		return frontier.Request{}, rootErr
	}
	verifier, recomputer, adapterErr := resolveFrontierAdapters(ctx, resolvedRoot)
	if adapterErr != nil {
		return frontier.Request{}, adapterErr
	}
	artifacts, readErr := readFrontierArtifacts(resolvedRoot, options)
	if readErr != nil {
		return frontier.Request{}, readErr
	}
	return frontier.Request{
		CEMBytes:     artifacts.cem,
		OCMBytes:     artifacts.ocm,
		ExpectedBase: options.expectedBase,
		Target:       options.target,
		Command:      artifacts.command,
		Observation:  artifacts.observation,
		JUnitReport:  artifacts.report,
		Verifier:     verifier,
		TCQ:          recomputer,
	}, nil
}

// frontierValueFlags is the closed set of flags that take a value, mapped to
// the field each one fills. A lookup table rather than a switch keeps the
// parser one loop with one responsibility, and makes the valueless flag the
// single named exception it actually is.
func frontierValueFlags(options *frontierOptions) map[string]*string {
	return map[string]*string{
		"--cem":           &options.cem,
		"--ocm":           &options.ocm,
		"--expected-base": &options.expectedBase,
		"--target":        &options.target,
		"--command":       &options.command,
		"--observation":   &options.observation,
		"--report":        &options.report,
	}
}

// parseFrontierOptions reads argv. Every rejection is the bare code
// `invalid-frontier-input`: CF-V0-024 forbids echoing an argv element, so an
// unrecognized flag can never be named back to the caller.
func parseFrontierOptions(arguments []string) (frontierOptions, error) {
	options := frontierOptions{}
	fields := frontierValueFlags(&options)
	for index := 0; index < len(arguments); index++ {
		name, value, inline := strings.Cut(arguments[index], "=")
		if name == "--json" {
			if inline {
				return options, frontierFailure(frontier.CodeInvalidInput)
			}
			options.json = true
			continue
		}
		field, known := fields[name]
		if !known {
			return options, frontierFailure(frontier.CodeInvalidInput)
		}
		if !inline {
			if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
				return options, frontierFailure(frontier.CodeInvalidInput)
			}
			index++
			value = arguments[index]
		}
		*field = value
	}
	return options, nil
}

func resolveFrontierRoot(root string) (string, error) {
	if root == "" {
		workingDirectory, err := os.Getwd()
		if err != nil {
			return "", frontierFailure(frontier.CodeInvalidInput)
		}
		return workingDirectory, nil
	}
	// normalizeRoot's own error carries a message; only its code reaches the
	// caller, because CF-V0-024 admits no message field at all.
	resolved, err := normalizeRoot(root)
	if err != nil {
		return "", frontierFailure(frontier.CodeInvalidInput)
	}
	return resolved, nil
}

// resolveFrontierAdapters refuses when no repository-backed adapter has been
// registered. CF-V0-001 makes one validated repository boundary a required
// input, so an unavailable boundary is an unsupported context (exit 2), never
// a silently empty frontier and never a pretended verification.
func resolveFrontierAdapters(ctx context.Context, root string) (frontier.Verifier, frontier.TCQRecomputer, error) {
	adapters := frontierAdapters
	if replacement, replaced := ctx.Value(frontierAdaptersKey{}).(frontierSeam); replaced {
		adapters = replacement
	}
	if adapters == nil {
		return nil, nil, frontierFailure(frontier.CodeUnsupportedContext)
	}
	verifier, recomputer := adapters(ctx, root)
	if verifier == nil || recomputer == nil {
		return nil, nil, frontierFailure(frontier.CodeUnsupportedContext)
	}
	return verifier, recomputer, nil
}

// frontierArtifacts is the raw byte copy of each supplied input. Absent inputs
// stay nil so the library sees exactly which of the CF-V0-003 tuple members
// the caller actually supplied.
type frontierArtifacts struct {
	cem         []byte
	ocm         []byte
	command     []byte
	observation []byte
	report      []byte
}

func readFrontierArtifacts(root string, options frontierOptions) (frontierArtifacts, error) {
	artifacts := frontierArtifacts{}
	reads := []struct {
		path string
		into *[]byte
	}{
		{options.cem, &artifacts.cem},
		{options.ocm, &artifacts.ocm},
		{options.command, &artifacts.command},
		{options.observation, &artifacts.observation},
		{options.report, &artifacts.report},
	}
	for _, read := range reads {
		data, err := readFrontierInput(root, read.path)
		if err != nil {
			return frontierArtifacts{}, err
		}
		*read.into = data
	}
	return artifacts, nil
}

// readFrontierInput reads one bounded artifact. CF-V0-026 permits a command
// wrapper to read files through its own reader while keeping path acquisition
// out of the wire and the verifier, so the bytes — never the path — are what
// crosses into the library. An unreadable path is `invalid-frontier-input`
// with the path itself withheld (CF-V0-024).
func readFrontierInput(root, path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}
	file, err := os.Open(resolveFrontierPath(root, path))
	if err != nil {
		return nil, frontierFailure(frontier.CodeInvalidInput)
	}
	defer func() { _ = file.Close() }()
	data, readErr := io.ReadAll(io.LimitReader(file, maxFrontierInputBytes+1))
	if readErr != nil {
		return nil, frontierFailure(frontier.CodeInvalidInput)
	}
	return data, nil
}

func resolveFrontierPath(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, filepath.FromSlash(path))
}

// writeFrontierResult renders the one sealed document. JSON writes the exact
// canonical bytes Compute already sealed, and human delegates to
// internal/frontier's renderer, which carries the CF-V0-025 assertion boundary
// in full. Neither branch recomputes or reformats anything, which is what
// CF-V0-026 means by two renderings of one computation.
func writeFrontierResult(stdout io.Writer, document frontier.Document, encoded []byte, jsonMode bool) error {
	rendered := encoded
	if !jsonMode {
		rendered = []byte(frontier.RenderHuman(document))
	}
	written, err := stdout.Write(rendered)
	if err != nil || written != len(rendered) {
		return frontierFailure(frontier.CodeInternalError)
	}
	return nil
}

// emitFrontierFailure is the single operational-failure exit (CF-V0-004,
// CF-V0-022): stdout has not been written, stderr carries exactly one bounded
// envelope — or, in human mode, one static message per code — and the status
// is 2. It renders nothing but the error's code, so no path, OID, digest, ID,
// count, XML value, argv element, or exception text can leak (CF-V0-024). The
// No repository path is consulted here: CF-V0-033 requires even refused and
// failing invocations to leave repository and .corvint state byte-for-byte unchanged.
func emitFrontierFailure(stderr io.Writer, err error, jsonMode bool) int {
	if jsonMode {
		_, _ = stderr.Write(frontier.RenderError(err))
		return frontier.ErrorExitCode
	}
	_, _ = io.WriteString(stderr, frontier.RenderErrorHuman(err))
	return frontier.ErrorExitCode
}

// frontierFailure builds a wrapper-owned operational failure. It carries a
// code and nothing else, which is how the no-echo rule stays structural: there
// is no message field for an unverified value to reach.
func frontierFailure(code string) error {
	return &frontier.Error{Code: code}
}
