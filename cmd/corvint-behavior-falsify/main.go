// Command corvint-behavior-falsify is an explicitly approved experimental
// companion for browser-criterion falsification controls. It is a shim over the
// `corvint-test falsify` group's entry (CTF-V0-004): both call
// internal/behaviorfalsify/cli, so its output stays byte-identical.
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

	"github.com/Beamfall/corvint/internal/behaviorfalsify/cli"
)

func main() {
	streams := &clierror.Streams{Binary: "corvint-behavior-falsify", Out: os.Stdout, Err: os.Stderr}
	defer streams.Finish()

	if code, handled := cliapp.Answer("corvint-behavior-falsify", os.Args[1:], streams.Stdout(), streams.Stderr()); handled {
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
	return cli.Run(ctx, "corvint-behavior-falsify", args, input, output)
}
