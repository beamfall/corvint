// SPDX-License-Identifier: Apache-2.0
package toolgroup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/mcp/rootbind"
	"github.com/Beamfall/corvint/internal/selectionswitch"
)

// AffectedTool answers which tests a change requires: the affected-summary/1
// verdict `corvint affected --summary` writes, for a base or for the dirty
// worktree (MMR-V0-1118, V1-5254).
const AffectedTool = "corvint.affected"

// AffectedErrorProfile is the group's own tool-error object.
const AffectedErrorProfile = "corvint-affected-mcp-tool-error/0"

// AffectedSelectAllProfile is the successful answer that selects every test
// with a typed reason, where the CLI would exit 2 instead (MTT-V1-008,
// MTT-V1-011): an MCP call cannot hand the user an exit status, and "run
// everything, here is why" never narrows.
const AffectedSelectAllProfile = "affected-select-all/1"

// AffectedStaleCode is the planner's refusal for a corvint binary older than
// the repository's Go source (AFP-V0-9730); the tool answers it as a
// select-all with this reason (MTT-V1-011).
const AffectedStaleCode = "self-binary-stale"

// affectedStaleHint is the one-line recovery a stale-binary answer carries:
// the server keeps the binary it started with, so a rebuild alone changes
// nothing.
const affectedStaleHint = "rebuild corvint from this checkout (go install ./cmd/corvint) and restart corvint mcp; the running server keeps the binary it started with"

// AffectedOversizedCode is the tool error for a verdict whose result cannot
// fit one MCP message; it names the CLI as the way to read it (MMR-V0-1118).
const AffectedOversizedCode = "verdict-oversized"

// affectedBaseLimit bounds the base argument; the planner's own revision
// limit is smaller, so a longer value is refused here before it runs.
const affectedBaseLimit = 4096

// affectedFailureLimit bounds how much of a failed run's stderr the tool
// error carries, parsed or raw.
const affectedFailureLimit = 4096

// affectedFrameHeadroom is what the server adds around a tools/call result:
// the jsonrpc and id members and the frame's newline. A result is refused as
// oversized before the server's generic bound would, so the refusal is this
// group's own (MMR-V0-1118, MCPV0-042).
const affectedFrameHeadroom = 4096

// AffectedRunner runs `corvint affected --summary` for root in-process, with
// `--base base` when base is not empty, writing the planner's stdout and
// stderr and returning its exit code. Only the corvint binary holds the
// planner, so only `corvint mcp` supplies one; corvint-mcp refuses the group.
type AffectedRunner func(ctx context.Context, root, base string, stdout, stderr io.Writer) int

// Affected is the opt-in `affected` group for one repository root. It is
// read-only: the planner it runs writes nothing under the root, and the
// answer is the planner's own stdout bytes, so missing evidence stays the
// planner's UNKNOWN or widened scope and is never narrowed here. Build it with
// NewAffected: the gate it holds admits one planner run at a time for the
// root, because the server dispatches up to 64 requests at once and each run
// is a whole planner (MMR-V0-1118). The planner itself is safe to run
// concurrently (its only writes are O_EXCL-and-rename cache entries); the
// gate bounds CPU and memory per root.
type Affected struct {
	Root    string
	Binding *rootbind.Binding
	Run     AffectedRunner
	// Lookup reads the server's environment for CORVINT_SELECTION; nil is
	// os.LookupEnv.
	Lookup func(string) (string, bool)
	gate   chan struct{}
}

// NewAffected is the group for root with its own one-run gate.
func NewAffected(root string, binding *rootbind.Binding, run AffectedRunner) Affected {
	return Affected{Root: root, Binding: binding, Run: run, gate: make(chan struct{}, 1)}
}

func (group Affected) Tools() []bridge.ToolDescriptor {
	return []bridge.ToolDescriptor{{
		Name: AffectedTool,
		Description: "Which tests does a change require? The same affected-summary/1 verdict as corvint affected --summary: " +
			"with base (a full commit id, an unambiguous hex prefix of at least 7 digits or a revision such as HEAD~1) it plans base..HEAD together with the dirty worktree; " +
			"without base it plans the dirty worktree alone. The verdict carries the plan scope, dirty/selected/excluded/unknown counts, unknown reasons and the sorted tier-0 and tier-1 unit names; " +
			"missing evidence widens the scope or stays UNKNOWN, never selects fewer tests. Read-only; served by corvint mcp only. " +
			"One planner runs at a time per root. A verdict too large for one MCP message is the tool error verdict-oversized: read it with the CLI instead. " +
			"CORVINT_SELECTION in the server's environment (full, full:<runner>[,<runner>], or any unknown value) makes the answer an affected-select-all/1 result that selects every test with its reason and a hint, never a narrowed plan. " +
			"On a corvint binary older than the repository's Go source the answer is affected-select-all/1 with reason self-binary-stale and a rebuild hint, never a narrowed plan; " +
			"rebuilding changes nothing until the server restarts, so rebuild corvint and restart corvint mcp.",
		Annotations: bridge.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"base": map[string]any{"type": "string", "minLength": 1, "maxLength": affectedBaseLimit},
			},
			"additionalProperties": false,
		},
	}}
}

func (group Affected) Call(ctx context.Context, name string, arguments map[string]any) (map[string]any, *protocol.RPCError) {
	if name != AffectedTool {
		return nil, protocol.InvalidParams("Invalid params")
	}
	base := ""
	for key, value := range arguments {
		text, isString := value.(string)
		// An option-like base is refused as a parameter, so no value can reach
		// the planner's argument parser as a flag.
		if key != "base" || !isString || text == "" || len(text) > affectedBaseLimit || strings.HasPrefix(text, "-") {
			return nil, protocol.InvalidParams("Invalid params")
		}
		base = text
	}
	if group.Run == nil {
		return Failure(AffectedErrorProfile, name, "not-served", "corvint.affected is served only by corvint mcp")
	}
	if group.Binding == nil || !group.Binding.Same(group.Root) {
		return Failure(AffectedErrorProfile, name, "invalid-root", "repository unavailable")
	}
	if ctx.Err() != nil {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	// MTT-V1-008: the kill switch is read with the CLI grammar; any value,
	// known or not, selects every test with its reason and never runs a
	// planner that could narrow.
	lookup := group.Lookup
	if lookup == nil {
		lookup = os.LookupEnv
	}
	value, _ := lookup(selectionswitch.Variable)
	if kill := selectionswitch.Parse(value); kill.On() {
		return group.selectAll(ctx, name, base, kill.Reason(), killSwitchDetail(kill), killSwitchHint(kill), kill)
	}
	if group.gate != nil {
		// A call waiting behind another run honours its own cancellation and
		// never starts the planner once its context has ended.
		select {
		case group.gate <- struct{}{}:
			defer func() { <-group.gate }()
		case <-ctx.Done():
			return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
		}
	}
	var stdout, stderr bytes.Buffer
	if exit := group.Run(ctx, group.Root, base, &stdout, &stderr); exit != 0 {
		if ctx.Err() != nil {
			return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
		}
		code, message := affectedFailure(stderr.Bytes())
		if code == AffectedStaleCode {
			// MTT-V1-011: a stale binary's plan may be narrower than the
			// checkout's, and a tool error makes an agent retry or skip
			// tests; select every test and say why instead. The CLI keeps
			// its exit 2.
			return group.selectAll(ctx, name, base, AffectedStaleCode, message, affectedStaleHint, selectionswitch.Switch{})
		}
		return Failure(AffectedErrorProfile, name, code, message)
	}
	var structured map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &structured); err != nil {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	// Unit names and unknown reasons are repository-derived text that reaches
	// a model through this block, so it alone is framed by the AHI-004
	// envelope; structuredContent carries the identical parsed object
	// unwrapped for programmatic callers (MCPV0-008), and the tool-error
	// object stays the closed unframed object every group answers.
	result, failure := Answer(ctx, AffectedErrorProfile, name, structured, stdout.String(), nil, nil)
	if failure != nil {
		return nil, failure
	}
	if size, over := affectedOversized(result); over {
		return Failure(AffectedErrorProfile, name, AffectedOversizedCode, fmt.Sprintf(
			"the affected-summary/1 verdict encodes to a %d-byte tool result, over the %d-byte MCP message bound; run corvint affected --summary in the repository to read it",
			size, protocol.MaxMessageBytes-affectedFrameHeadroom))
	}
	return result, nil
}

// selectAll is the successful select-all answer: scope FULL, every test
// selected, the typed reason, a one-line detail and hint, and, for the kill
// switch, the value and runners it named.
func (group Affected) selectAll(ctx context.Context, name, base, reason, detail, hint string, kill selectionswitch.Switch) (map[string]any, *protocol.RPCError) {
	structured := map[string]any{
		"base": base, "detail": detail, "hint": hint, "mutates": false, "ok": true,
		"profile": AffectedSelectAllProfile, "reason": reason, "scope": "FULL", "selectAll": true, "tool": name,
	}
	if kill.On() {
		runners := []any{}
		for _, runner := range kill.Runners {
			runners = append(runners, runner)
		}
		structured["selection"] = map[string]any{"variable": selectionswitch.Variable, "value": kill.Value, "runners": runners}
	}
	text, err := json.Marshal(structured)
	if err != nil {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	return Answer(ctx, AffectedErrorProfile, name, structured, string(text), nil, nil)
}

// killSwitchDetail says why every test is selected.
func killSwitchDetail(kill selectionswitch.Switch) string {
	switch {
	case kill.Unknown:
		return fmt.Sprintf("%s=%q is neither full nor full:<runner>[,<runner>]; every test is selected instead of a narrowed plan", selectionswitch.Variable, kill.Value)
	case kill.All:
		return selectionswitch.Variable + "=full forces every runner to a full run; every test is selected"
	}
	return fmt.Sprintf("%s=%q forces runners %s to a full run; this tool widens that to every test until the planner serves a per-runner full plan (NMS-V1-008)", selectionswitch.Variable, kill.Value, strings.Join(kill.Runners, ", "))
}

// killSwitchHint is the one-line recovery for a kill-switch answer.
func killSwitchHint(kill selectionswitch.Switch) string {
	if kill.Unknown {
		return "set " + selectionswitch.Variable + " to full or full:<runner>[,<runner>], or unset it, in the corvint mcp server's environment and restart the server"
	}
	return "unset " + selectionswitch.Variable + " in the corvint mcp server's environment and restart the server to narrow again"
}

// affectedOversized measures result as the server encodes a frame and reports
// whether it leaves no room for the frame around it (MMR-V0-1118).
func affectedOversized(result map[string]any) (int, bool) {
	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(result); err != nil {
		return 0, true
	}
	size := body.Len() - 1
	return size, size > protocol.MaxMessageBytes-affectedFrameHeadroom
}

// affectedFailure reads the planner's error object from the last non-empty
// stderr line for its code and message, so a DEGRADED notice printed before
// the refusal (AFP-V0-846) never hides the code; an unparsable last line is
// affected-failed with that line as the message. Either message is bounded.
func affectedFailure(stderr []byte) (code, message string) {
	var emitted struct {
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	line := affectedLastLine(stderr)
	if json.Unmarshal([]byte(line), &emitted) == nil && emitted.Error != "" {
		code = emitted.Code
		if code == "" {
			code = "affected-failed"
		}
		return code, affectedBound(emitted.Error)
	}
	if line == "" {
		line = "corvint affected --summary failed"
	}
	return "affected-failed", affectedBound(line)
}

// affectedLastLine is the last non-empty line of stderr, trimmed.
func affectedLastLine(stderr []byte) string {
	text := strings.TrimRight(string(stderr), " \t\r\n")
	if index := strings.LastIndexByte(text, '\n'); index >= 0 {
		text = text[index+1:]
	}
	return strings.TrimSpace(text)
}

// affectedBound cuts message to affectedFailureLimit bytes on a rune boundary.
func affectedBound(message string) string {
	if len(message) <= affectedFailureLimit {
		return message
	}
	cut := affectedFailureLimit
	for cut > 0 && !isRuneStart(message[cut]) {
		cut--
	}
	return message[:cut]
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }
