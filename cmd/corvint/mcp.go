package main

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/mcp/cli"
)

const mcpHelp = `Serve the local read-only stdio MCP server (AHI-373, experimental).

Usage:
  corvint mcp --root ABSOLUTE_ROOT [--protocol-version 2026-07-28|2025-11-25]
  corvint mcp --root ALIAS=ABSOLUTE_ROOT [--root ALIAS=ABSOLUTE_ROOT ...]
  corvint mcp --root ABSOLUTE_ROOT --tool-profile LIST [--corpus-artifact [ALIAS=]PATH]
  corvint mcp --root ABSOLUTE_ROOT --follow-ref REF
  corvint mcp --workspace FILE [--follow-ref ALIAS=REF ...]

Required: at least one --root (an absolute path, optionally ALIAS=) or one --workspace.
Runs the same server as the corvint-mcp binary, byte for byte on the wire, so a
host that can launch corvint can reach the corvint.* tools without a second
binary. Every argument after mcp goes to that server; the leading global --root
is not read. It speaks JSON-RPC over stdin and stdout until stdin closes or
SIGINT/SIGTERM arrives, and exits 2 with "corvint-mcp: invalid arguments" on
stderr for an argument it does not accept. corvint-mcp --help documents each
option.

Example:
  corvint mcp --root /absolute/repo --protocol-version 2025-11-25
`

// runMCP serves internal/mcp/cli for `corvint mcp` (AHI-373). Only `corvint mcp --help`
// and -h print this verb's help; every other argument reaches the server unchanged.
func runMCP(ctx context.Context, arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(arguments) == 1 && (arguments[0] == "--help" || arguments[0] == "-h") {
		_, _ = fmt.Fprint(stdout, helpText("mcp"))
		return 0
	}
	if slices.Contains(arguments, "--require-clean") {
		emitError(stderr, argumentError("argument --require-clean: corvint mcp serves read-only tools and takes no --require-clean"))
		return 2
	}
	return cli.Serve(ctx, arguments, stdin, stdout, stderr)
}

// mcpAfterGlobalRoot reports `corvint --root PATH mcp ...`, which the server cannot read:
// its roots are its own arguments.
func mcpAfterGlobalRoot(arguments []string) bool {
	if len(arguments) >= 3 && arguments[0] == "--root" {
		return arguments[2] == "mcp"
	}
	return len(arguments) >= 2 && strings.HasPrefix(arguments[0], "--root=") && arguments[1] == "mcp"
}
