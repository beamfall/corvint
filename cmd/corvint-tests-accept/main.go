// corvint-tests-accept is an optional experimental local companion. It is a shim
// over the `corvint-test accept` group's entry (CTF-V0-004): both call
// internal/testacceptance/cli, so its output stays byte-identical apart from
// the build identity.
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

	"github.com/Beamfall/corvint/internal/testacceptance/cli"
)

func run(ctx context.Context, args []string, in io.Reader, out io.Writer) error {
	return cli.Run(ctx, "corvint-tests-accept", []string{"worker"}, args, in, out)
}

func main() {
	streams := &clierror.Streams{Binary: "corvint-tests-accept", Out: os.Stdout, Err: os.Stderr}
	defer streams.Finish()

	if code, handled := cliapp.Answer("corvint-tests-accept", os.Args[1:], streams.Stdout(), streams.Stderr()); handled {
		os.Exit(streams.Status(code))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if e := run(ctx, os.Args[1:], os.Stdin, streams.Stdout()); e != nil {
		fmt.Fprintln(streams.Stderr(), e)
		os.Exit(streams.Status(2))
	}
}
