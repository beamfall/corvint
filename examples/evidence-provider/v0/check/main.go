// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/Beamfall/corvint/internal/extevidence"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], os.Stdout)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("provider-check", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var pin extevidence.Pin
	flags.StringVar(&pin.Schema, "profile", "", "exact record schema")
	flags.StringVar(&pin.ProviderID, "provider-id", "", "exact provider id")
	flags.StringVar(&pin.ProviderRevision, "provider-version", "", "exact provider revision")
	flags.StringVar(&pin.RepositoryRevision, "revision", "", "exact repository commit")
	flags.StringVar(&pin.RepositoryID, "repository-id", "", "repository id in /1 and /2")
	flags.StringVar(&pin.Origin, "origin", "", "root commit id in /1 and /2")
	flags.StringVar(&pin.ExecutableSHA256, "executable-sha256", "", "exact command executable SHA256")
	root := flags.String("root", ".", "repository directory")
	file := flags.String("file", "", "record file")
	command := flags.String("command", "", "contained provider argv JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || (*file == "") == (*command == "") {
		return fmt.Errorf("select exactly one --file or --command and no positional arguments")
	}
	source := *file
	if *command != "" {
		var err error
		source, err = extevidence.ParseCommand(*command)
		if err != nil {
			return err
		}
	}
	data, err := extevidence.ReadPinned(ctx, *root, source, pin)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}
