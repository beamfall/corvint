// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"io"

	"github.com/Beamfall/corvint/internal/docsenvelope"
	"github.com/Beamfall/corvint/internal/docstools"
	"github.com/Beamfall/corvint/internal/gokernel"
)

const toolVerb = docsenvelope.ToolVerb

// parseDocsToolInvocation recognises `docs-tool --root ABSOLUTE_ROOT TOOL`, the protocol verb
// corvint-mcp's docs bridge runs; any other first word is not a docs-tool invocation.
func parseDocsToolInvocation(arguments []string) (root, tool string, ok bool, err error) {
	if len(arguments) == 0 || arguments[0] != toolVerb {
		return "", "", false, nil
	}
	if len(arguments) != 4 || arguments[1] != "--root" || arguments[2] == "" {
		return "", "", true, argumentError(toolVerb + " takes --root ABSOLUTE_ROOT and one tool name")
	}
	if arguments[3] != docsenvelope.ToolDraft && arguments[3] != docsenvelope.ToolConsume {
		return "", "", true, argumentError(toolVerb + " serves " + docsenvelope.ToolDraft + " and " + docsenvelope.ToolConsume)
	}
	return arguments[2], arguments[3], true, nil
}

// runDocsTool answers one tool call: stdin holds the tool's JSON arguments, stdout one
// docsenvelope.ToolResult. Every refusal the native registry makes travels inside the
// envelope with exit 0, so the bridge can tell a refused call from a broken process.
func runDocsTool(ctx context.Context, root, tool string, stdin io.Reader, stdout, stderr io.Writer) int {
	result := docsenvelope.ToolResult{Schema: docsenvelope.ToolSchema, Tool: tool}
	raw, err := readInputBounded(ctx, stdin, docsenvelope.MaxArgumentBytes)
	switch {
	case ctx.Err() != nil:
		result.Error = &docsenvelope.Coded{Code: "cancelled"}
	case err != nil, len(raw) > docsenvelope.MaxArgumentBytes:
		result.Error = &docsenvelope.Coded{Code: "invalid-arguments"}
	default:
		registry, registryErr := docstools.New(root)
		if registryErr != nil {
			result.Error = &docsenvelope.Coded{Code: registryErr.Code}
			break
		}
		structured, text, failure, transport := registry.Call(ctx, tool, raw)
		switch {
		case transport != nil:
			result.Error = &docsenvelope.Coded{Code: transport.Code}
		case failure != nil:
			result.Failure = &docsenvelope.Coded{Code: failure.Code, Message: failure.Message}
		default:
			result.Structured, result.Text = structured, text
		}
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: "internal-error", Message: "cannot encode the docs-tool result"})
		return 2
	}
	if _, err := stdout.Write(append(encoded, '\n')); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write the docs-tool result"})
		return 2
	}
	return 0
}
