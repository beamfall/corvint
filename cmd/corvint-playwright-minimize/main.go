// Command corvint-playwright-minimize is an explicitly authorized experimental
// companion. Planning reads pinned evidence; execution launches bounded trials.
// It is a shim over the `corvint-test minimize` group's entry (CTF-V0-004): both
// call internal/playwrightminimize/cli, so its output stays byte-identical.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/Beamfall/corvint/internal/playwrightminimize/cli"
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
	return cli.Run(ctx, "corvint-playwright-minimize", args, input, output)
}
