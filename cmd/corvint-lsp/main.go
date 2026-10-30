// SPDX-License-Identifier: Apache-2.0
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
	experimental := flags.Bool("experimental", false, "required opt-in: this is an experimental stdio language server")
	executable := flags.String("gopls", "", "absolute gopls binary that serves .go documents (wrapped server observations)")
	root := flags.String("root", "", "absolute workspace root; required with any language server")
	guard := flags.Bool("workspace-drift-guard", false, "refuse wrapped results once the workspace drifts from the bound revision; requires a language server")
	tsserver := flags.String("tsserver", "", "absolute typescript-language-server binary, run as BINARY --stdio, for .ts .tsx .mts .cts .js .jsx .mjs .cjs")
	rubyLSP := flags.String("ruby-lsp", "", "absolute ruby-lsp binary for .rb .rake")
	var backends []lspstdio.BackendConfig
	var parseErr error
	flags.Func("server", "repeatable SUFFIXES=ABSOLUTE_ARGV: comma-separated TS/JS or Ruby suffixes served by one wrapped server whose whitespace-separated argv starts with an absolute binary", func(spec string) error {
		b, err := lspstdio.ParseServer(spec)
		if err != nil {
			parseErr = err
			return err
		}
		backends = append(backends, b)
		return nil
	})
	const usage = "usage: corvint-lsp --experimental [--root ABSOLUTE_ROOT [--gopls ABSOLUTE_BINARY] [--tsserver ABSOLUTE_BINARY] [--ruby-lsp ABSOLUTE_BINARY] [--server SUFFIXES=ABSOLUTE_ARGV]... [--workspace-drift-guard]]"
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			// LWB-V0-010: --help lists every flag with its argument form.
			fmt.Fprintln(os.Stdout, usage)
			flags.SetOutput(os.Stdout)
			flags.PrintDefaults()
			return 0
		}
		if errors.Is(parseErr, lspstdio.ErrUnsupportedSuffix) {
			fmt.Fprintln(os.Stderr, "corvint-lsp: refused: "+lspstdio.ErrUnsupportedSuffix.Error())
		} else {
			fmt.Fprintln(os.Stderr, usage)
		}
		return 2
	}
	if *tsserver != "" {
		backends = append(backends, lspstdio.TypeScriptBackend(*tsserver))
	}
	if *rubyLSP != "" {
		backends = append(backends, lspstdio.RubyBackend(*rubyLSP))
	}
	served := *executable != "" || len(backends) > 0
	if !*experimental || flags.NArg() != 0 || served != (*root != "") || (*guard && !served) {
		fmt.Fprintln(os.Stderr, usage)
		return 2
	}
	// LWB-V0-006/007: refuse a configuration by its fixed reason before serving.
	if err := lspstdio.CheckBackends(*executable, backends); err != nil {
		if errors.Is(err, lspstdio.ErrUnsupportedSuffix) || errors.Is(err, lspstdio.ErrServerBinaryMissing) {
			fmt.Fprintln(os.Stderr, "corvint-lsp: refused: "+err.Error())
		} else {
			fmt.Fprintln(os.Stderr, usage)
		}
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
	if served {
		err = lspstdio.ServeSemantic(ctx, in, out, lspstdio.SemanticConfig{Executable: *executable, Root: *root, WorkspaceDriftGuard: *guard, Backends: backends})
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
