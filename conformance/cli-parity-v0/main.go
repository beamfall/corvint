package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	setProcessUmask()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := runCLI(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runCLI(ctx context.Context, arguments []string) error {
	if len(arguments) == 0 {
		return fmt.Errorf("usage: cli-parity-v0 capture|replay [options]")
	}
	switch arguments[0] {
	case "capture":
		flags := flag.NewFlagSet("capture", flag.ContinueOnError)
		manifest := flags.String("manifest", "conformance/cli-parity-v0/manifest.json", "manifest path")
		oracle := flags.String("oracle", "", "Python oracle command override")
		only := flags.String("only", "", "capture only cases whose id has this prefix; refuses --write")
		write := flags.Bool("write", false, "atomically replace the manifest")
		if err := flags.Parse(arguments[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return fmt.Errorf("capture accepts no positional arguments")
		}
		return capture(ctx, captureOptions{ManifestPath: *manifest, Oracle: *oracle, Only: *only, Write: *write, Output: os.Stdout})
	case "replay":
		flags := flag.NewFlagSet("replay", flag.ContinueOnError)
		manifest := flags.String("manifest", "conformance/cli-parity-v0/manifest.json", "manifest path")
		oracle := flags.String("oracle", "", "Python oracle command override")
		candidate := flags.String("candidate", "", "Go candidate command override")
		only := flags.String("only", "", "replay only cases whose id has this prefix; reports SUMMARY-FILTERED, never corpus evidence")
		if err := flags.Parse(arguments[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return fmt.Errorf("replay accepts no positional arguments")
		}
		return replay(ctx, replayOptions{ManifestPath: *manifest, Oracle: *oracle, Candidate: *candidate, Only: *only, Output: os.Stdout})
	default:
		return fmt.Errorf("unknown subcommand %q", arguments[0])
	}
}
