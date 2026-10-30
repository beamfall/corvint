// corvint-test-runner is an experimental optional companion. Trusted test code
// runs locally; its receipts attest observed bytes, not complete dependencies.
// It is a shim over the `corvint-test runner` group's entry (CTF-V0-004): both
// call internal/testrunner/cli, so its output stays byte-identical.
package main

import (
	"context"
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
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(command(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func command(ctx context.Context, args []string, out, errout io.Writer) int {
	return cli.Command(ctx, "corvint-test-runner", args, out, errout)
}
