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

	"github.com/Beamfall/corvint/internal/remoteprovider"
)

func main() {
	streams := &clierror.Streams{Binary: "corvint-remote-provider", Out: os.Stdout, Err: os.Stderr}
	defer streams.Finish()

	if code, handled := cliapp.Answer("corvint-remote-provider", os.Args[1:], streams.Stdout(), streams.Stderr()); handled {
		os.Exit(streams.Status(code))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(streams.Status(run(ctx, os.Args[1:], streams.Stdout(), streams.Stderr())))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) != 3 || args[0] != "--allow-network" || args[1] != "--config" || args[2] == "" {
		fmt.Fprintln(stderr, "usage: corvint-remote-provider --allow-network --config FILE")
		return 2
	}
	config, err := remoteprovider.LoadConfig(args[2])
	if err != nil {
		fmt.Fprintln(stderr, remoteprovider.ErrRefused)
		return 1
	}
	data, err := remoteprovider.Fetch(ctx, config)
	if err != nil {
		fmt.Fprintln(stderr, remoteprovider.ErrRefused)
		return 1
	}
	if _, err := stdout.Write(data); err != nil {
		return 1
	}
	return 0
}
