package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/Beamfall/corvint/internal/remoteprovider"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
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
