// Command corvint-mcp is the standalone stdio MCP server. `corvint mcp` serves the same
// server from the core binary (AHI-198); the logic lives in internal/mcp/cli.
package main

import (
	"context"
	"os"

	"github.com/Beamfall/corvint/internal/companioncli"
	"github.com/Beamfall/corvint/internal/mcp/cli"
)

func main() {
	if code, handled := companioncli.Answer(companioncli.Spec{Binary: "corvint-mcp", Version: companioncli.ExperimentalVersion, Usage: cli.Usage()}, os.Args[1:], os.Stdout); handled { // CAP-V0-700
		os.Exit(code)
	}
	os.Exit(cli.ServeProfiled(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
