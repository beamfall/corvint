package bridge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"

	"github.com/Beamfall/corvint/internal/capcatalog"
)

const (
	// ToolHelp returns `corvint help --json` or `corvint help guide` from the
	// binary's embedded catalog and guide on every profile (MCPV0-100).
	ToolHelp = "corvint.help"

	helpSchema       = "corvint-mcp-help/0"
	helpTopicCatalog = "catalog"
	helpTopicGuide   = "guide"
)

// callHelp answers corvint.help (MCPV0-100): the receipt's text is the exact
// stdout of `corvint help --json` (the embedded snapshot that CAP-V0-019 holds
// equal to it) or of `corvint help guide`, with its byte count and SHA-256.
func callHelp(ctx context.Context, arguments []byte) (Result, *Error) {
	input := struct {
		Topic *string `json:"topic"`
	}{}
	var members map[string]any
	if err := decodeClosed(arguments, &input); err != nil || json.Unmarshal(arguments, &members) != nil ||
		(input.Topic == nil && members["topic"] == nil && len(members) != 0) {
		return Result{}, failure("invalid-arguments")
	}
	topic, command, text := helpTopicCatalog, "corvint help --json", string(capcatalog.Snapshot())
	if input.Topic != nil {
		switch *input.Topic {
		case helpTopicCatalog:
		case helpTopicGuide:
			topic, command, text = helpTopicGuide, "corvint help guide", capcatalog.Guide()
		default:
			return Result{}, failure("invalid-arguments")
		}
	}
	digest := sha256.Sum256([]byte(text))
	result := Result{
		Schema: resultSchema, Tool: ToolHelp, Mutates: false, State: "READY",
		EpistemicClass: "OBSERVED", AuthorityClass: "CORVINT_BINARY", Repository: nil,
		Receipt: map[string]any{
			"schema": helpSchema, "topic": topic, "command": command,
			"byteCount": len(text), "sha256": hex.EncodeToString(digest[:]), "text": text,
		},
		Abstention: Abstention{Active: false, Reason: "NONE"},
	}
	encoded, err := result.CanonicalJSON()
	if err != nil {
		return Result{}, err
	}
	if len(encoded) > maxResultBytes || !fitsMCPFrame(encoded, result) {
		return abstained(ToolHelp, "OUTPUT_BUDGET_EXCEEDED", nil), nil
	}
	if ctx.Err() != nil {
		return Result{}, failure("cancelled")
	}
	return result, nil
}

func validHelpReceipt(receipt map[string]any) bool {
	text, textOK := receipt["text"].(string)
	digest := sha256.Sum256([]byte(text))
	return receipt["schema"] == helpSchema && textOK && receipt["sha256"] == hex.EncodeToString(digest[:]) &&
		(receipt["topic"] == helpTopicCatalog || receipt["topic"] == helpTopicGuide)
}

// helpDescriptor is corvint.help's closed tools/list entry, advertised on
// every profile (MCPV0-100).
func helpDescriptor() ToolDescriptor {
	return ToolDescriptor{
		Name:        ToolHelp,
		Description: "Return the Corvint command catalog (corvint-capability-catalog/0) exactly as `corvint help --json` prints it, or with topic guide the task-indexed agent guide exactly as `corvint help guide` prints it. Reads only the binary, never the repository.",
		Annotations: readAnnotations(),
		InputSchema: objectSchema(map[string]any{
			"topic": map[string]any{"type": "string", "enum": []any{helpTopicCatalog, helpTopicGuide}, "default": helpTopicCatalog},
		}, []any{}),
	}
}

// validHelpResult is validResult's rule for a READY corvint.help result and
// for any result claiming CORVINT_BINARY authority, which only that result may
// carry: no repository binding and a receipt whose digest matches its text.
func validHelpResult(result Result) bool {
	return result.Tool == ToolHelp && result.State == "READY" && result.Repository == nil && result.AuthorityClass == "CORVINT_BINARY" && result.EpistemicClass == "OBSERVED" &&
		!result.Abstention.Active && result.Abstention.Reason == "NONE" && validHelpReceipt(result.Receipt)
}
