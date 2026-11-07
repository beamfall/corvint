// Command corvint-playwright-minimize is an explicitly authorized experimental
// companion. Planning reads pinned evidence; execution launches bounded trials.
// It is a shim over the `corvint-test minimize` group's entry (CTF-V0-004): both
// call internal/playwrightminimize/cli, so its output stays byte-identical.
package main

import (
	"context"
	"fmt"
	"github.com/Beamfall/corvint/internal/cliapp"
	"github.com/Beamfall/corvint/internal/clierror"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/Beamfall/corvint/internal/playwrightminimize/cli"
)

func main() {
	streams := &clierror.Streams{Binary: "corvint-playwright-minimize", Out: os.Stdout, Err: os.Stderr}
	defer streams.Finish()

	if code, handled := cliapp.Answer("corvint-playwright-minimize", os.Args[1:], streams.Stdout(), streams.Stderr()); handled {
		os.Exit(streams.Status(code))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdin, streams.Stdout()); err != nil {
		fmt.Fprintln(streams.Stderr(), err)
		os.Exit(streams.Status(1))
	}
}

func run(ctx context.Context, args []string, input io.Reader, output io.Writer) error {
	return cli.Run(ctx, "corvint-playwright-minimize", args, input, output)
}
