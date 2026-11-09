// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/docsenvelope"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// docsHelp is the `docs` topic of a binary that holds no documentation code: the group is an
// exec-only shim onto corvint-docs (decision 0594 item 3, CMS-V0-003).
const docsHelp = `Experimental source documentation orientation, served by corvint-docs.

Usage:
  corvint [--root PATH] docs SUBCOMMAND [OPTIONS]

The corvint binary holds no documentation code (decision 0594): this group and a --corpus=FILE
[--corpus-depth=N] option before impact, affected, context or test-validity run the
corvint-docs executable beside corvint or on PATH with the same arguments, pass it no
authority, and print its bytes. When it is absent the shim refuses with code not-installed
and names the NOT_INSTALLED gap. The subcommands (nav, draft, consume, citations, corpus,
maintain, flows), their options and their help are corvint-docs's own: see
corvint-docs help docs and corvint-docs help --json.
Maturity: experimental.

Example:
  corvint --root . docs corpus search --artifact corpus.json --query TERM
`

// docsForwardWaitDelay bounds how long an interrupted corvint-docs may hold the pipes.
const docsForwardWaitDelay = 5 * time.Second

// runCorpusIntegration forwards a `--corpus=FILE` or `--corpus-depth=N` invocation to
// corvint-docs, which runs the native read back in this binary and joins the corpus onto it.
// Without either option the invocation is not a corpus join and is not handled here. An
// unvalued `--corpus` stays a native flag, as before (DCP-V1-097).
func runCorpusIntegration(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, bool) {
	for _, arg := range args {
		if arg == "--" {
			break
		}
		name, _, inline := strings.Cut(arg, "=")
		if (name == "--corpus" && inline) || name == "--corpus-depth" {
			return forwardToDocs(ctx, args, stdin, stdout, stderr), true
		}
	}
	return 0, false
}

// runDocsShim forwards `[--root PATH] docs ...` to corvint-docs unchanged.
func runDocsShim(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, bool) {
	position := commandPositionAfterRoots(args)
	if position < 0 || args[position] != "docs" {
		return 0, false
	}
	return forwardToDocs(ctx, args, stdin, stdout, stderr), true
}

// forwardToDocs runs corvint-docs with this invocation's argument vector and streams, and
// returns its exit status. It adds no flags, environment or credentials (CMS-V0-003). A missing
// executable is the named NOT_INSTALLED gap, exit 2.
func forwardToDocs(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	executable, err := docsenvelope.Locate(docsenvelope.Binary)
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: docsenvelope.NotInstalledCode, Message: docsenvelope.NotInstalledMessage(docsenvelope.Binary, "corvint docs")})
		return 2
	}
	command := exec.CommandContext(ctx, executable, args...)
	command.Stdin, command.Stdout, command.Stderr = stdin, stdout, stderr
	command.WaitDelay = docsForwardWaitDelay
	err = command.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exit):
		return exit.ExitCode()
	case ctx.Err() != nil:
		emitError(stderr, &gokernel.Error{Code: "cancelled", Message: docsenvelope.Binary + " was interrupted"})
	default:
		emitError(stderr, &gokernel.Error{Code: "internal-error", Message: "cannot run " + docsenvelope.Binary + ": " + err.Error()})
	}
	return 2
}
