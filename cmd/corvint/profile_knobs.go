package main

import (
	"context"
	"errors"
	"io"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/procprofile"
	"github.com/Beamfall/corvint/internal/runtimeenv"
)

// runProfiledContext is main's dispatch under the CORVINT_CPUPROFILE and CORVINT_MEMPROFILE
// bracket (PRB-V0-001..008): one bracket at the entry point, so every command runContext
// dispatches, including ones added later, inherits both knobs. The profiles are written on
// return and as soon as signalled ends on SIGINT or SIGTERM; ctx, which may also end at an
// adapter's host-kill deadline, only drives the command.
func runProfiledContext(signalled, ctx context.Context, arguments []string, stdin io.Reader, stdout, stderr io.Writer, lookup runtimeenv.Lookup) int {
	return procprofile.Run(signalled, lookup, func(err error) { emitError(stderr, profileKnobError(err)) }, func() int {
		return runContext(ctx, arguments, stdin, stdout, stderr)
	})
}

func profileKnobError(err error) error {
	var knob *procprofile.Error
	if errors.As(err, &knob) {
		return &gokernel.Error{Code: knob.Code, Message: err.Error()}
	}
	return err
}

// profileKnobWorkers are the internal child entry points main runs before the bracket: their
// startup-only owned-group verification must precede every goroutine, the profiler's included.
var profileKnobWorkers = []string{"dogfood-outcome-worker", "dogfood-verifier-worker", "index-refresh-worker (CORVINT_INTERNAL_INDEX_REFRESH)"}

// profileKnobPlumbing are the hidden plumbing verbs runContext dispatches inside the bracket.
var profileKnobPlumbing = []string{"native-hook", "authority-event", "qualified-event"}

// profileKnobServers are the MCP server binaries whose entry points apply the same bracket.
var profileKnobServers = []string{"corvint-corpus-mcp", "corvint-docs-mcp", "corvint-mcp", "corvint-test-validity-mcp"}

// profileKnobCatalog is the `profiling` member of `corvint help --json` (PRB-V0-009): which
// commands and servers honour the knobs, derived from the same command list the dispatcher
// refuses unknown verbs against.
func profileKnobCatalog() map[string]any {
	knobs := []any{
		map[string]any{"name": procprofile.Knobs[0], "profile": "cpu", "format": "pprof"},
		map[string]any{"name": procprofile.Knobs[1], "profile": "heap", "format": "pprof"},
	}
	commands := make([]any, 0, len(topLevelCommands))
	for _, command := range topLevelCommands {
		commands = append(commands, command)
	}
	plumbing := make([]any, 0, len(profileKnobPlumbing))
	for _, verb := range profileKnobPlumbing {
		plumbing = append(plumbing, verb)
	}
	servers := make([]any, 0, len(profileKnobServers))
	for _, server := range profileKnobServers {
		servers = append(servers, server)
	}
	workers := make([]any, 0, len(profileKnobWorkers))
	for _, worker := range profileKnobWorkers {
		workers = append(workers, worker)
	}
	return map[string]any{
		"schema":          "corvint-profile-knobs/0",
		"knobs":           knobs,
		"default":         "off",
		"pid_placeholder": procprofile.PIDPlaceholder,
		"written":         "on process exit, and on SIGINT or SIGTERM",
		"scope":           "every command dispatched by the corvint binary, and every listed MCP server, one profile per process",
		"commands":        commands,
		"plumbing":        plumbing,
		"servers":         servers,
		"not_honoured": map[string]any{
			"entries": workers,
			"reason":  "internal child workers whose startup verification precedes every goroutine",
		},
		"authority": "diagnostic only; never changes what a command reads, returns, or mutates",
	}
}
