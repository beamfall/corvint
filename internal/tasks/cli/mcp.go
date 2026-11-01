package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/mcp/server"
	"github.com/Beamfall/corvint/internal/mcp/tasksread"
)

// mcpCommand serves `corvint-tasks mcp [--protocol-version V]` over stdio
// until stdin closes or SIGINT/SIGTERM arrives (TMR-V0-950). It exits 2 on any
// other argument.
func mcpCommand(env Env, args []string) int {
	args, version, ok := protocol.ExtractVersionArgument(args)
	if !ok || len(args) != 0 {
		_, _ = fmt.Fprintln(env.Stderr, "corvint-tasks mcp: invalid arguments")
		return 2
	}
	// Each tool call runs its read verb through Run in this directory.
	handler := tasksread.New(func(argv []string, stdout io.Writer) int {
		call := env
		call.Args, call.Stdin, call.Stdout, call.Stderr = argv, bytes.NewReader(nil), stdout, io.Discard
		return Run(call)
	})
	instance, err := server.New(server.Config{
		ProtocolVersion: version, Name: tasksread.Name, Version: Version,
		Description:    "Local read-only Corvint Tasks queue, roadmap, ticket and receipt-audit server.",
		Capabilities:   map[string]any{"tools": map[string]any{"listChanged": false}},
		Handler:        handler,
		NameLegacyFlag: true,
	})
	if err != nil {
		_, _ = fmt.Fprintln(env.Stderr, "corvint-tasks mcp: startup failed")
		return 2
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := instance.Serve(ctx, env.Stdin, env.Stdout); err != nil && ctx.Err() == nil {
		_, _ = fmt.Fprintln(env.Stderr, "corvint-tasks mcp: transport failed")
		return 2
	}
	return 0
}
