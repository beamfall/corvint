package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/Beamfall/corvint/internal/cliapp"
	"github.com/Beamfall/corvint/internal/clierror"
	"io"
	"os"
	"os/signal"
	"path/filepath"
)

type options struct{ root string }

func parseArguments(arguments []string) (options, error) {
	if len(arguments) != 4 || arguments[0] != "--root" || arguments[1] == "" ||
		arguments[2] != "serve" || arguments[3] != "--stdio" {
		return options{}, fmt.Errorf("usage: corvint-pulse --root ROOT serve --stdio")
	}
	root, err := filepath.Abs(arguments[1])
	if err != nil {
		return options{}, errors.New("invalid root")
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return options{}, errors.New("invalid root")
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return options{}, errors.New("invalid root")
	}
	return options{root: resolved}, nil
}

func emitCLIError(output io.Writer, code, message string) {
	raw, err := canonicalJSON(map[string]any{"code": code, "message": message, "ok": false})
	if err == nil {
		_, _ = output.Write(append(raw, '\n'))
	}
}

func runContext(ctx context.Context, arguments []string, input io.Reader, output, errorOutput io.Writer, factory actorFactory) int {
	if code, handled := serveHelp(arguments, output, errorOutput); handled {
		return code
	}
	options, err := parseArguments(arguments)
	if err != nil {
		emitCLIError(errorOutput, "invalid-arguments", err.Error())
		return 2
	}
	if err := serve(ctx, options.root, input, output, factory, freshSessionID); err != nil {
		emitCLIError(errorOutput, "pulse-server-failed", "Pulse stdio server failed")
		return 2
	}
	return 0
}

func run(arguments []string, input io.Reader, output, errorOutput io.Writer) int {
	return runContext(context.Background(), arguments, input, output, errorOutput, newWorkspaceActor)
}

func main() {
	streams := &clierror.Streams{Binary: "corvint-pulse", Out: os.Stdout, Err: os.Stderr}
	defer streams.Finish()

	if code, handled := cliapp.Answer("corvint-pulse", os.Args[1:], streams.Stdout(), streams.Stderr()); handled {
		os.Exit(streams.Status(code))
	}

	ctx, stop := signal.NotifyContext(context.Background(), terminationSignals()...)
	defer stop()
	go func() {
		<-ctx.Done()
		_ = os.Stdin.Close()
	}()
	os.Exit(streams.Status(runContext(ctx, os.Args[1:], os.Stdin, streams.Stdout(), streams.Stderr(), newWorkspaceActor)))
}
