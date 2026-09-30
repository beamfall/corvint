// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/lspstdio"
)

func main() { os.Exit(run(os.Args[1:])) }
func run(args []string) int {
	if len(args) == 2 && args[0] == lspstdio.ContextWorkerMode {
		return lspstdio.ServeContextWorker(args[1], os.Stdin, os.Stdout)
	}
	flags := flag.NewFlagSet("corvint-lsp", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	experimental := flags.Bool("experimental", false, "")
	executable := flags.String("gopls", "", "")
	root := flags.String("root", "", "")
	guard := flags.Bool("workspace-drift-guard", false, "")
	if flags.Parse(args) != nil || !*experimental || flags.NArg() != 0 || ((*executable == "") != (*root == "")) || (*guard && *executable == "") {
		fmt.Fprintln(os.Stderr, "usage: corvint-lsp --experimental [--gopls ABSOLUTE_BINARY --root ABSOLUTE_ROOT [--workspace-drift-guard]]")
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	// Rewrap inherited descriptors as pollable files. Closing Go's startup
	// os.Stdin alone does not reliably interrupt a blocked inherited-pipe read.
	in, err := pollable(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "corvint-lsp: input unavailable")
		return 1
	}
	out, err := pollable(os.Stdout)
	if err != nil {
		in.Close()
		fmt.Fprintln(os.Stderr, "corvint-lsp: output unavailable")
		return 1
	}
	if *executable != "" {
		err = lspstdio.ServeSemantic(ctx, in, out, lspstdio.SemanticConfig{Executable: *executable, Root: *root, WorkspaceDriftGuard: *guard})
	} else {
		err = lspstdio.Serve(ctx, in, out)
	}
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return 1
		}
		fmt.Fprintln(os.Stderr, "corvint-lsp: session ended with error")
		return 1
	}
	return 0
}
