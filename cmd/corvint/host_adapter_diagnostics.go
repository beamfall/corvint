// SPDX-License-Identifier: Apache-2.0
package main

import "strings"

// DTL-V0-002 keeps diagnostics in every emitted text channel, including bounded fallback output.
func withFallbackDiagnostics(output map[string]any) map[string]any {
	out := make(map[string]any, len(output))
	for key, value := range output {
		if text, ok := value.(string); ok {
			if (strings.HasPrefix(text, "Corvint FALLBACK ") || strings.HasPrefix(text, "Corvint fallback: ")) && !strings.Contains(text, "\ncause:") {
				text += "\ncause: " + fallbackCause(text) + "\nfix: run corvint adapter --help to check the hook inputs, then run corvint query --task \"Corvint hook check\" from the repository outside the hook; additional underlying causes are NOT_OBSERVED"
			}
			out[key] = text
		} else if nested, ok := value.(map[string]any); ok {
			out[key] = withFallbackDiagnostics(nested)
		} else {
			out[key] = value
		}
	}
	return out
}

func hookDegradationDiagnostics(codes []any) []any {
	rows := make([]any, 0, len(codes))
	for _, raw := range codes {
		code, _ := raw.(string)
		cause, fix := code, "inspect corvint harness event --help and retry the event outside the hook"
		if code == "frontier-authority-unavailable" {
			cause = "no-accepted-closing-authority"
			fix = "none exists automatically; use the project's explicit review and completion workflow"
		}
		rows = append(rows, map[string]any{"code": code, "cause": cause, "fix": fix})
	}
	return rows
}

// Keep only a validated code from the trusted frame; never echo arbitrary error details.
func fallbackCause(text string) string {
	reason := adapterDegradationReason(map[string]any{"systemMessage": text})
	if reason == claudeUnavailableReason {
		return "event-read-unavailable"
	}
	if i := strings.LastIndexByte(reason, ':'); i >= 0 {
		reason = reason[i+1:]
	}
	if claudeSafeCode.MatchString(reason) {
		return reason
	}
	return "adapter-fallback"
}
