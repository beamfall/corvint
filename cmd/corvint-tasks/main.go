// Command corvint-tasks is the Corvint task control plane executor and ticket store.
// At TCP-01 it exposes only the read, export and verify surfaces that are
// implemented; every mutation verb is absent on purpose and answers
// NOT_RUN (see internal/cli).
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
