// Copyright 2026 Corvint contributors.
// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/Beamfall/corvint/internal/postmergehost"
)

type launcherOptions struct{ profile, profileSHA, request, requestSHA, out string }

func parseOptions(args []string) (launcherOptions, error) {
	var out launcherOptions
	if len(args) != 10 {
		return out, postmergehost.ErrHostInput
	}
	values := map[string]*string{"--profile": &out.profile, "--profile-sha256": &out.profileSHA, "--request": &out.request, "--request-sha256": &out.requestSHA, "--out": &out.out}
	for i := 0; i < len(args); i += 2 {
		target, ok := values[args[i]]
		if !ok || *target != "" || args[i+1] == "" {
			return out, postmergehost.ErrHostInput
		}
		*target = args[i+1]
		delete(values, args[i])
	}
	if len(values) != 0 {
		return out, postmergehost.ErrHostInput
	}
	return out, nil
}
func run(ctx context.Context, args []string, in io.ReadCloser, out, stderr io.Writer) int {
	if len(args) == 2 && args[0] == "--internal-envelope" {
		if postmergehost.RunGuestEnvelope(args[1], in, out) != nil {
			fmt.Fprintln(stderr, "guest envelope blocked")
			return 2
		}
		return 0
	}
	options, err := parseOptions(args)
	if err != nil {
		fmt.Fprintln(stderr, "invalid paired host invocation")
		return 2
	}
	inputs, err := postmergehost.LoadHostInputs(options.profile, options.profileSHA, options.request, options.requestSHA, options.out)
	if err != nil {
		fmt.Fprintln(stderr, "paired host input blocked")
		return 2
	}
	if postmergehost.RunHostLauncher(ctx, inputs, in, out) != nil {
		fmt.Fprintln(stderr, "paired host held; inspect retained ownership and facts")
		return 2
	}
	return 0
}
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
