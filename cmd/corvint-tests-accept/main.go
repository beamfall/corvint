// corvint-tests-accept is an optional experimental local companion. It is a shim
// over the `corvint-test accept` group's entry (CTF-V0-004): both call
// internal/testacceptance/cli, so its output stays byte-identical apart from
// the build identity.
package main

import (
	"context"
	"fmt"
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
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if e := run(ctx, os.Args[1:], os.Stdin, os.Stdout); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(2)
	}
}
