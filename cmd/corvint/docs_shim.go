// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/docsenvelope"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/procprofile"
)

// docsHelp is the `docs` topic of a binary that holds none of the docs commands: the group is an
// exec-only shim onto corvint-docs (decision 0594 item 3, CMS-V0-003).
const docsHelp = `Experimental source documentation orientation, served by corvint-docs.

Usage:
  corvint [--root PATH] docs SUBCOMMAND [OPTIONS]

The docs commands are not in the corvint binary (decision 0594); it keeps only the corpus
tools its MCP server serves in process. This group and a --corpus=FILE [--corpus-depth=N]
option before impact, affected, context or test-validity run the corvint-docs executable
beside corvint or on PATH with the same arguments, pass it no authority, and print its
bytes. When it is absent the shim refuses with code not-installed and names the
NOT_INSTALLED gap. The subcommands (nav, draft, consume, citations, corpus, maintain,
flows), their options and their help are corvint-docs's own: see corvint-docs help docs
and corvint-docs help --json.
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
			return forwardToDocs(ctx, args, "corvint "+name, stdin, stdout, stderr), true
		}
	}
	return 0, false
}

// runDocsShim forwards `[--root PATH] docs ...` to corvint-docs unchanged. The docs group reads
// no worktree freshness, so the shim itself refuses --require-clean (DIRTY-CACHE-016) whether or
// not corvint-docs is installed.
func runDocsShim(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, bool) {
	position := commandPositionAfterRoots(args)
	if position < 0 || args[position] != "docs" {
		return 0, false
	}
	for _, arg := range args[position+1:] {
		if arg == "--" {
			break
		}
		if name, _, _ := strings.Cut(arg, "="); name == "--require-clean" {
			emitError(stderr, argumentError("argument --require-clean: corvint docs reads no worktree freshness and takes no --require-clean"))
			return 2, true
		}
	}
	return forwardToDocs(ctx, args, "corvint docs", stdin, stdout, stderr), true
}

// forwardToDocs runs corvint-docs with this invocation's argument vector and streams, and
// returns its exit status. It adds no flags, environment or credentials (CMS-V0-003), and drops
// only the profile knobs procprofile.ChildEnvironment names. A missing
// executable is the named NOT_INSTALLED gap for forwarded, the form the user ran, exit 2. A
// corvint-docs killed by signal N exits 128+N, as a shell reports it.
func forwardToDocs(ctx context.Context, args []string, forwarded string, stdin io.Reader, stdout, stderr io.Writer) int {
	executable, err := docsenvelope.Locate(docsenvelope.Binary)
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: docsenvelope.NotInstalledCode, Message: docsenvelope.NotInstalledMessage(docsenvelope.Binary, forwarded)})
		return 2
	}
	command := exec.CommandContext(ctx, executable, args...)
	command.Stdin, command.Stdout, command.Stderr = stdin, stdout, stderr
	command.Env = procprofile.ChildEnvironment(os.Environ())
	docsenvelope.StopOnCancel(command, docsForwardWaitDelay)
	err = command.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exit):
		status, ok := exit.Sys().(syscall.WaitStatus)
		if !ok || !status.Signaled() {
			return exit.ExitCode()
		}
		if ctx.Err() != nil {
			// This invocation's own cancellation killed the app.
			emitError(stderr, &gokernel.Error{Code: "cancelled", Message: docsenvelope.Binary + " was interrupted"})
			return 2
		}
		emitError(stderr, &gokernel.Error{Code: "internal-error", Message: docsenvelope.Binary + " was killed by signal " + status.Signal().String()})
		return 128 + int(status.Signal())
	case ctx.Err() != nil:
		emitError(stderr, &gokernel.Error{Code: "cancelled", Message: docsenvelope.Binary + " was interrupted"})
	default:
		emitError(stderr, &gokernel.Error{Code: "internal-error", Message: "cannot run " + docsenvelope.Binary + ": " + err.Error()})
	}
	return 2
}
