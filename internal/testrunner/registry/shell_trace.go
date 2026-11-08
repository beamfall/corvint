// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/Beamfall/corvint/internal/proctrace"
	"github.com/Beamfall/corvint/internal/testrunner"
)

// TraceShellCommand captures bounded native stdout/stderr and actual process
// status through the existing core tracer. The caller admits the pinned runner
// and shell tools, environment, repository roots and caps before calling this.
// It executes only the bound whole-file command; it performs no PATH lookup.
func TraceShellCommand(ctx context.Context, key testrunner.InventoryKey, source, shell string, cmd CommandResponse, q proctrace.Request, backend proctrace.Backend) (proctrace.Result, string, error) {
	plugin, err := ShellSelectionPlugin(key, source, shell)
	if err != nil {
		return proctrace.Result{}, "", err
	}
	want, err := plugin.Command(ctx, CommandRequest{Key: key, Purpose: "collect", RunDir: cmd.Cwd})
	if err != nil {
		return proctrace.Result{}, "", err
	}
	if !slices.Equal(cmd.Argv, want.Argv) || len(cmd.Env) != 0 || len(cmd.Artifacts) != 0 {
		return proctrace.Result{}, "", fmt.Errorf("shell-trace-command-mismatch")
	}
	stdout, stderr := &shellOutput{}, &shellOutput{}
	q.Stdout, q.Stderr = stdout, stderr
	trace, _, executeErr := TraceScriptCommand(ctx, cmd, q, backend)
	in := testrunner.Input{Runner: key.Runner, SourceFile: source, Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitCode: trace.Outcome.ExitCode, Overflow: stdout.overflow || stderr.overflow}
	if executeErr != nil {
		in.ExecutionProblems = []testrunner.Problem{{Code: "shell-trace-execution-failed", Detail: executeErr.Error()}}
	}
	if ctx.Err() != nil {
		in.Interrupted = true
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return trace, "", err
	}
	if in.Overflow {
		return trace, string(raw), fmt.Errorf("shell-report-cap-exceeded")
	}
	return trace, string(raw), executeErr
}

// The Input envelope base64-encodes both streams. This cap keeps their combined
// serialization within the existing MaxReportBytes bound, including metadata.
// Continue consuming after overflow so neither child pipe can deadlock. The buffer
// is a named field so WriteString and ReadFrom (used by io.Copy) cannot bypass Write.
type shellOutput struct {
	buf      bytes.Buffer
	overflow bool
}

func (b *shellOutput) Bytes() []byte { return b.buf.Bytes() }

func (b *shellOutput) Write(p []byte) (int, error) {
	n := len(p)
	remaining := testrunner.MaxReportBytes/4 - b.buf.Len()
	if len(p) > remaining {
		b.overflow = true
		p = p[:remaining]
	}
	_, _ = b.buf.Write(p)
	return n, nil
}
