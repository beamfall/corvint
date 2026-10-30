// Command corvint-tasks is the Corvint task control plane executor and ticket store.
// It exposes native queue mutations and generation-fenced external-agent leases.
// The help envelope lists implemented and omitted operations.
package main

import (
	"context"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"os"

	"github.com/Beamfall/corvint/internal/tasks/cli"
)

// build is the first-parent commit count of the built Corvint commit, stamped
// with -ldflags "-X main.build=N" (decision 0397). An unstamped build reports 0.
var build = "0"

func main() {
	if len(os.Args) == 6 && os.Args[1] == "lane-leader" && os.Args[2] == "--directory" && os.Args[4] == "--capsule" {
		if e := supervisor.Leader(context.Background(), os.Args[3], os.Args[5]); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		return
	}
	cli.Build = build
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	os.Exit(cli.Run(cli.Env{Cwd: cwd, Args: os.Args[1:], Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}))
}
