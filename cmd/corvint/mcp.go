package main

import (
	"context"
	"fmt"
	"io"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/mcp/cli"
	"github.com/Beamfall/corvint/internal/mcp/toolgroup"
)

const mcpHelp = `Serve the local read-only stdio MCP server (AHI-373, experimental).

Usage:
  corvint mcp --root ABSOLUTE_ROOT [--protocol-version 2026-07-28|2025-11-25]
  corvint mcp --root ALIAS=ABSOLUTE_ROOT [--root ALIAS=ABSOLUTE_ROOT ...]
  corvint mcp --root ABSOLUTE_ROOT --tool-profile LIST [--corpus-artifact [ALIAS=]PATH]
  corvint mcp --root ABSOLUTE_ROOT --tool-profile affected
  corvint mcp --root ABSOLUTE_ROOT --follow-ref REF
  corvint mcp --workspace FILE [--follow-ref ALIAS=REF ...]

Required: at least one --root (an absolute path, optionally ALIAS=) or one --workspace.
Runs the same server as the corvint-mcp binary, byte for byte on the wire, so a
host that can launch corvint can reach the corvint.* tools without a second
binary. Every argument after mcp goes to that server; the leading global --root
is not read. It speaks JSON-RPC over stdin and stdout until stdin closes or
SIGINT/SIGTERM arrives, and exits 2 with "corvint-mcp: invalid arguments" on
stderr for an argument it does not accept. corvint-mcp --help documents each
option. The affected group (--tool-profile affected) adds corvint.affected, which
answers which tests a change requires with the affected-summary/1 verdict that the
summary form of the affected verb prints, for a base or for the dirty worktree;
only this verb serves it, because the planner lives in this binary (MMR-V0-1118).
One planner runs at a time per root; a verdict too large for one MCP message is
the tool error verdict-oversized, read with the CLI instead. The server keeps the
binary it started with: when a call answers the tool error self-binary-stale,
rebuild this binary and restart the server, since a rebuild alone changes nothing.

Example:
  corvint mcp --root /absolute/repo --protocol-version 2025-11-25
  corvint mcp --root /absolute/repo --tool-profile affected
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
	return cli.ServeWithHooks(ctx, mcpServerVersion(), arguments, stdin, stdout, stderr, cli.Hooks{Affected: mcpAffectedRunner})
}

// mcpAffectedRunner answers corvint.affected (MMR-V0-1118, V1-5254) by running
// this binary's own `affected --summary` planner for root, so the tool's bytes
// are the CLI's bytes: the same parser, planner, self-binary freshness check
// and canonical writer. base is already bounded and never option-like.
func mcpAffectedRunner(ctx context.Context, root, base string, stdout, stderr io.Writer) int {
	arguments := []string{"--root", root, "affected", "--summary"}
	if base != "" {
		arguments = append(arguments, "--base", base)
	}
	return runContext(ctx, arguments, strings.NewReader(""), stdout, stderr)
}

var _ toolgroup.AffectedRunner = mcpAffectedRunner

// mcpServerVersion is this binary's --version line without its name, the
// serverInfo.version `corvint mcp` reports (MCPV0-951), so a plugin and the
// binary it launched can be told apart: `1.0.0-rc.3 (build N, commit SHA)`.
func mcpServerVersion() string {
	info, _ := debug.ReadBuildInfo()
	return strings.TrimPrefix(versionText(versionIdentityFrom(version, build, sourceCommit, info)), "Corvint ")
}

// mcpAfterGlobalRoot reports `corvint --root PATH mcp ...`, which the server cannot read:
// its roots are its own arguments.
func mcpAfterGlobalRoot(arguments []string) bool {
	if len(arguments) >= 3 && arguments[0] == "--root" {
		// An option-like value is a missing --root value, which the root preamble reports (GPK-V0-064).
		return !argparseOptionLike(arguments[1]) && arguments[2] == "mcp"
	}
	return len(arguments) >= 2 && strings.HasPrefix(arguments[0], "--root=") && arguments[1] == "mcp"
}
