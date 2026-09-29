// Command corvint-tasks is the Corvint task control plane executor and ticket store.
// It exposes native queue mutations and generation-fenced external-agent leases.
// The help envelope lists implemented and omitted operations.
package main

import (
	"os"

	"github.com/Beamfall/corvint/internal/tasks/cli"
)

// build is the first-parent commit count of the built Corvint commit, stamped
// with -ldflags "-X main.build=N" (decision 0397). An unstamped build reports 0.
var build = "0"

func main() {
	cli.Build = build
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	os.Exit(cli.Run(cli.Env{Cwd: cwd, Args: os.Args[1:], Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}))
}
