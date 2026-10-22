// Command corvint-behavior-falsify is an explicitly approved experimental
// companion for browser-criterion falsification controls. It is a shim over the
// `corvint-test falsify` group's entry (CTF-V0-004): both call
// internal/behaviorfalsify/cli, so its output stays byte-identical.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/Beamfall/corvint/internal/behaviorfalsify/cli"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, input io.Reader, output io.Writer) error {
	return cli.Run(ctx, "corvint-behavior-falsify", args, input, output)
}
