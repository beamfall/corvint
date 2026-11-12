// SPDX-License-Identifier: Apache-2.0

package toolgroup

import (
	"strings"

	"github.com/Beamfall/corvint/internal/docsenvelope"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

// Codes every server answers in the tool domain instead of -32603 (DMF-V1-010).
const (
	CancelledCode   = "cancelled"
	ChildFailedCode = "child-process-failed"
)

// errorDetail is what a code means to a caller: a message that says what failed, whether the same
// call may succeed later, and the argument change or command that may fix it (DMF-V1-010).
type errorDetail struct {
	message   string
	retryable bool
	next      string
}

var errorDetails = map[string]errorDetail{
	CancelledCode:                 {"the call was cancelled before it finished", true, ""},
	protocol.ServerBusyCode:       {"the server is at its concurrency limit", true, "retry the same call when an earlier one finishes"},
	"deadline-exceeded":           {"the call did not finish within its deadline", true, "retry with fewer paths"},
	"internal-error":              {"the tool failed without a typed reason", true, ""},
	ChildFailedCode:               {"the tool's helper process failed", true, ""},
	"invalid-root":                {"the repository root is unavailable", false, "restart the server with --root naming a Git repository"},
	"repository-unavailable":      {"the repository root changed or is unavailable", false, "restart the server with --root naming the repository"},
	"corpus-refused":              {"the configured corpus artifact was refused", false, "rebuild the artifact with corvint-docs docs corpus build --manifest FILE"},
	"corpus-unavailable":          {"the configured corpus artifact cannot be read", false, "build it with corvint-docs docs corpus build --manifest FILE"},
	"corpus-input-unavailable":    {"a corpus input the call needs is unavailable", false, "restart the server on the repository root the artifact was built for"},
	"not-served":                  {"this tool is not served by this server", false, "use the server the tool's description names"},
	"output-bound-exceeded":       {"the tool result exceeds the MCP message bound", false, "narrow the request with limit, offset or fewer paths"},
	"unsupported-tool":            {"the server does not answer this tool", false, "call tools/list and use a listed tool"},
	"invalid-arguments":           {"the arguments do not satisfy the tool's input schema", false, "check the arguments against the inputSchema in tools/list"},
	repoenvelope.CollisionCode:    {"the answer contains the untrusted-data envelope terminator and was withheld", false, ""},
	docsenvelope.NotInstalledCode: {"the corvint-docs executable this tool runs is not installed", false, "install the core set beside the server with: " + docsenvelope.InstallCommand("") + " --bin-dir DIR"},
}

// Detail returns the message, retryability and next step of a tool error: message when it says
// more than the code, else the code's own message; an unknown code is not retryable and has no
// next step (DMF-V1-010).
func Detail(code, message string) (string, bool, string) {
	d, known := errorDetails[code]
	text := strings.TrimSpace(message)
	if text == "" || text == code {
		text = d.message
	}
	if text == "" {
		text = "the tool failed with code " + code
	}
	if !known {
		return text, false, ""
	}
	return text, d.retryable, d.next
}

// Describe adds message, retryable and next to a tool-error object that has a code, keeping every
// member it already has (an old client reads them unchanged); a next already set is kept.
func Describe(value map[string]any) map[string]any {
	code, _ := value["code"].(string)
	message, _ := value["error"].(string)
	if existing, ok := value["message"].(string); ok && existing != "" {
		message = existing
	}
	text, retryable, next := Detail(code, message)
	value["message"], value["retryable"] = text, retryable
	if existing, ok := value["next"].(string); !ok || existing == "" {
		value["next"] = next
	}
	return value
}

// Cancelled is the tool error of a call whose context ended before it finished; it is never the
// -32603 internal error (DMF-V1-010).
func Cancelled(profile, name string) (map[string]any, *protocol.RPCError) {
	return Failure(profile, name, CancelledCode, errorDetails[CancelledCode].message)
}
