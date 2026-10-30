package bridge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/capcatalog"
	"github.com/Beamfall/corvint/internal/repoenvelope"
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
	topic := helpTopicCatalog
	if input.Topic != nil {
		topic = *input.Topic
	}
	command, text, known := helpTopicText(topic)
	if !known {
		return Result{}, failure("invalid-arguments")
	}
	return helpResult(ctx, topic, command, text)
}

// HelpDescriptor is corvint.help's tools/list entry for the companion MCP
// servers, which have no bridge Registry (MCPV0-101..103).
func HelpDescriptor() ToolDescriptor { return helpDescriptor() }

// HelpToolResult answers corvint.help for a companion MCP server
// (MCPV0-101..104) with Registry.Call's argument bounds and callHelp, and
// returns the CallToolResult that cmd/corvint-mcp emits for the same call: the
// bridge object as structuredContent and its canonical JSON inside the AHI-004
// envelope. It reads only the binary, so it takes no root.
func HelpToolResult(ctx context.Context, arguments []byte) (map[string]any, *Error) {
	if ctx.Err() != nil {
		return nil, failure("cancelled")
	}
	if len(arguments) == 0 || len(arguments) > maxArgumentSize || !utf8.Valid(arguments) {
		return nil, failure("invalid-arguments")
	}
	result, err := callHelp(ctx, arguments)
	if err != nil {
		return nil, err
	}
	structured, err := result.Object()
	if err != nil {
		return nil, err
	}
	text, err := result.TextJSON()
	if err != nil {
		return nil, err
	}
	framed, frameErr := repoenvelope.Frame(string(text))
	if frameErr != nil {
		return nil, failure(repoenvelope.CollisionCode)
	}
	return map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": framed}},
		"isError":           false,
		"structuredContent": structured,
	}, nil
}

// helpTopicText is the CLI form and embedded text of one corvint.help topic.
func helpTopicText(topic string) (command, text string, known bool) {
	switch topic {
	case helpTopicCatalog:
		return "corvint help --json", string(capcatalog.Snapshot()), true
	case helpTopicGuide:
		return "corvint help guide", capcatalog.Guide(), true
	}
	return "", "", false
}

// helpResult binds text into the READY result. Cancellation is checked before
// the budget, so a cancelled call fails cancelled whatever the text's size; a
// text that alone exceeds the budget abstains before the result is encoded.
func helpResult(ctx context.Context, topic, command, text string) (Result, *Error) {
	if ctx.Err() != nil {
		return Result{}, failure("cancelled")
	}
	if len(text) > maxResultBytes {
		return abstained(ToolHelp, "OUTPUT_BUDGET_EXCEEDED", nil), nil
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
	return result, nil
}

// validHelpReceipt holds a receipt to exactly the six closed members, the CLI
// form of its topic, and the embedded text of that topic: byteCount and
// sha256 must describe that text, so neither a relabelled topic nor replaced
// text with a recomputed digest keeps CORVINT_BINARY authority.
func validHelpReceipt(receipt map[string]any) bool {
	if len(receipt) != 6 {
		return false
	}
	topic, _ := receipt["topic"].(string)
	command, embedded, known := helpTopicText(topic)
	text, textOK := receipt["text"].(string)
	byteCount, countOK := receipt["byteCount"].(int)
	digest := sha256.Sum256([]byte(embedded))
	return known && textOK && countOK && receipt["schema"] == helpSchema && receipt["command"] == command &&
		text == embedded && byteCount == len(embedded) && receipt["sha256"] == hex.EncodeToString(digest[:])
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
