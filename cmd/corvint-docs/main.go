// SPDX-License-Identifier: Apache-2.0

// Command corvint-docs is Corvint's documentation app (decision 0594 item 3):
// the `docs` command group (draft, consume, nav, citations, corpus, maintain
// and flows), the `--corpus=FILE` join over a core read, and the `docs-tool`
// verb that answers the MCP documentation tools for corvint-mcp. It takes the
// same argument vectors `corvint docs ...` and `corvint --corpus=FILE ...`
// take; the core forwards those to this executable unchanged and holds no
// documentation code (CMS-V0-003). It reads the repository and writes only
// what the documentation verbs always wrote: `docs maintain --apply`,
// `docs flows finalize` and the bounded self-observation ledger.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/Beamfall/corvint/internal/cliapp"
	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/groupreap"
	"github.com/Beamfall/corvint/internal/procprofile"
	"github.com/Beamfall/corvint/internal/runtimeenv"
)

const binaryName = "corvint-docs"

func main() {
	if code, handled := cliapp.Answer(binaryName, os.Args[1:], os.Stdout, os.Stderr); handled {
		os.Exit(code)
	}
	signalled, cancel := signal.NotifyContext(context.Background(), terminationSignals()...)
	defer cancel()
	status := runProfiled(signalled, signalled, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.LookupEnv)
	groupreap.KillLive()
	gitstatus.CloseScratch()
	os.Exit(status)
}

// runProfiled is run under the CORVINT_CPUPROFILE and CORVINT_MEMPROFILE bracket at the entry
// point, as in corvint (PRB-V0-001), so every verb inherits both knobs; the profiles are written
// on return and when signalled ends, while ctx alone drives the command.
func runProfiled(signalled, ctx context.Context, arguments []string, stdin io.Reader, stdout, stderr io.Writer, lookup runtimeenv.Lookup) int {
	return procprofile.Run(signalled, lookup, func(err error) { emitError(stderr, profileKnobError(err)) }, func() int {
		return runContext(ctx, arguments, stdin, stdout, stderr)
	})
}

func profileKnobError(err error) error {
	var knob *procprofile.Error
	if errors.As(err, &knob) {
		return &gokernel.Error{Code: knob.Code, Message: err.Error()}
	}
	return err
}

func run(arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return runContext(context.Background(), arguments, stdin, stdout, stderr)
}

// runContext dispatches one invocation. The corpus join runs first because its
// `--corpus=FILE` precedes the core read's own verb; help next, so a `--help`
// anywhere after the verb is never parsed as an option; then each docs parser
// claims its verb exactly as the core's dispatcher did.
func runContext(ctx context.Context, arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if exit, handled := runCorpusIntegration(ctx, arguments, stdin, stdout, stderr); handled {
		return exit
	}
	if topic, requested, err := parseHelpInvocation(arguments); requested || err != nil {
		if err != nil {
			emitError(stderr, err)
			return 2
		}
		if _, err := io.WriteString(stdout, helpText(topic)); err != nil {
			emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write help output"})
			return 2
		}
		return 0
	}
	if options, isCorpus, corpusErr := parseCorpusInvocation(arguments); isCorpus {
		if corpusErr != nil {
			emitError(stderr, corpusErr)
			return 2
		}
		return runCorpus(ctx, options, stdout, stderr)
	}
	if options, isDocsMaintain, maintainErr := parseDocsMaintainInvocation(arguments); isDocsMaintain {
		if maintainErr != nil {
			emitError(stderr, maintainErr)
			return 2
		}
		return runDocsMaintain(ctx, options, stdout, stderr)
	}
	if options, handled, err := parseDocsFlowsInvocation(arguments); handled {
		if err != nil {
			emitError(stderr, err)
			return 2
		}
		return runDocsFlows(ctx, options, stdout, stderr)
	}
	if options, isDocs, docsErr := parseDocsInvocation(arguments); isDocs {
		if docsErr != nil {
			emitError(stderr, docsErr)
			return 2
		}
		return runDocs(ctx, options, stdin, stdout, stderr)
	}
	if root, tool, isTool, toolErr := parseDocsToolInvocation(arguments); isTool {
		if toolErr != nil {
			emitError(stderr, toolErr)
			return 2
		}
		return runDocsTool(ctx, root, tool, stdin, stdout, stderr)
	}
	emitError(stderr, argumentError(fmt.Sprintf("%s takes [--root PATH] docs SUBCOMMAND ..., --corpus=FILE before a core read, or %s; see %s --help", binaryName, toolVerb, binaryName)))
	return 2
}
