// Command corvint-mcp is the standalone stdio MCP server. `corvint mcp` serves the same
// server from the core binary (AHI-198); the logic lives in internal/mcp/cli.
package main

import (
	"context"
	"github.com/Beamfall/corvint/internal/cliapp"
	"github.com/Beamfall/corvint/internal/clierror"
	"os"

	"github.com/Beamfall/corvint/internal/companioncli"
	"github.com/Beamfall/corvint/internal/mcp/cli"
)

func main() {
	streams := &clierror.Streams{Binary: "corvint-mcp", Protocol: true, Out: os.Stdout, Err: os.Stderr}
	defer streams.Finish()

	if code, handled := cliapp.Answer("corvint-mcp", os.Args[1:], streams.Stdout(), streams.Stderr()); handled {
		os.Exit(streams.Status(code))
	}

	if code, handled := companioncli.Answer(companioncli.Spec{Binary: "corvint-mcp", Version: companioncli.ExperimentalVersion, Usage: cli.Usage()}, os.Args[1:], streams.Stdout()); handled { // CAP-V0-700
		os.Exit(streams.Status(code))
	}
	os.Exit(streams.Status(cli.ServeProfiled(context.Background(), os.Args[1:], os.Stdin, streams.Stdout(), streams.Stderr())))
}
