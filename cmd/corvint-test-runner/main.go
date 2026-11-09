// corvint-test-runner is an experimental optional companion. Trusted test code
// runs locally; its receipts attest observed bytes, not complete dependencies.
// It is a shim over internal/testrunner/cli (CTF-V0-004); the `corvint-test runner`
// group that shared that entry left with the E2E proof lab (decision 0593).
package main

import (
	"context"
	"github.com/Beamfall/corvint/internal/cliapp"
	"github.com/Beamfall/corvint/internal/clierror"
	"io"
	"os"
	"os/signal"
	"syscall"

	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/cli"
)

const profile = cli.PlanProfile

type plan = tr.PlanDocument
type receipt = tr.ReceiptDocument

var admittedInvocation = cli.AdmittedInvocation

func main() {
	streams := &clierror.Streams{Binary: "corvint-test-runner", Out: os.Stdout, Err: os.Stderr}
	defer streams.Finish()

	if code, handled := cliapp.Answer("corvint-test-runner", os.Args[1:], streams.Stdout(), streams.Stderr()); handled {
		os.Exit(streams.Status(code))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(streams.Status(command(ctx, os.Args[1:], streams.Stdout(), streams.Stderr())))
}

func command(ctx context.Context, args []string, out, errout io.Writer) int {
	return cli.Command(ctx, "corvint-test-runner", args, out, errout)
}
