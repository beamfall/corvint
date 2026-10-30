package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

// legacyCasesTool serves `docs corpus search-cases` (DCP-V1-075..077). It is
// listed only when the server was started with --legacy-cases, so the default
// tool list is unchanged.
const legacyCasesTool = "corvint.docs_search_cases"

// extractLegacyCasesFlag removes one --legacy-cases after the required
// --root R --artifact A prefix; a repeated or misplaced flag is invalid.
func extractLegacyCasesFlag(arguments []string) ([]string, bool, bool) {
	remaining := make([]string, 0, len(arguments))
	enabled := false
	for i, argument := range arguments {
		if argument != "--legacy-cases" {
			remaining = append(remaining, argument)
			continue
		}
		if enabled || i < 4 {
			return nil, false, false
		}
		enabled = true
	}
	return remaining, enabled, true
}

type legacyCaseSearcher struct {
	root string
}

func (l *legacyCaseSearcher) descriptor() bridge.ToolDescriptor {
	return bridge.ToolDescriptor{
		Name: legacyCasesTool,
		Description: "Search a caller-exported TestRail JSON or CSV test-case export (a repository-relative input) for " +
			"cases whose title, suite/section path, preconditions or steps contain every query word as a prefix. " +
			"match \"any\" instead ranks cases matching any non-stop-word query word by weighted score " +
			"(title > path > preconditions/steps) and reports a per-hit score. id (C12345 or 12345), given instead " +
			"of query, returns exactly that case. Returns case ID, path and title only, with the query, fields and " +
			"searched export so a none-found result can be audited. Manual cases are never runtime parity. Reads " +
			"one file; writes nothing.",
		Annotations: bridge.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"format": map[string]any{"type": "string", "enum": []any{doccorpus.LegacyFormatTestRailJSON, doccorpus.LegacyFormatCSV}},
				"input":  map[string]any{"type": "string", "minLength": 1, "maxLength": 4096},
				"query":  map[string]any{"type": "string", "minLength": 1, "maxLength": 1024},
				"id":     map[string]any{"type": "string", "minLength": 1, "maxLength": 11, "pattern": "^C?[1-9][0-9]{0,9}$"},
				"match":  map[string]any{"type": "string", "enum": []any{doccorpus.LegacyMatchAll, doccorpus.LegacyMatchAny}},
				"limit":  map[string]any{"type": "integer", "minimum": 1, "maximum": doccorpus.MaxResults},
			},
			"required":             []any{"format", "input"},
			"additionalProperties": false,
		},
	}
}

// decodeLegacyCaseArguments enforces the advertised schema: closed keys, no
// null, required strings, exactly one of query and id, match only with query,
// and an integer limit in bounds (default 20, as the CLI) (DCP-V1-168).
func decodeLegacyCaseArguments(raw []byte) (format, input string, q doccorpus.LegacyCaseQuery, ok bool) {
	q.Limit = 20
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return "", "", q, false
	}
	for _, key := range []string{"format", "input"} {
		if _, present := fields[key]; !present {
			return "", "", q, false
		}
	}
	_, hasQuery := fields["query"]
	_, hasID := fields["id"]
	_, hasMatch := fields["match"]
	if hasQuery == hasID || hasID && hasMatch {
		return "", "", q, false
	}
	str := func(v json.RawMessage, limit int) (string, bool) {
		var s string
		return s, json.Unmarshal(v, &s) == nil && s != "" && len(s) <= limit
	}
	for key, v := range fields {
		valid := false
		switch key {
		case "format":
			format, valid = str(v, 16)
		case "input":
			input, valid = str(v, 4096)
		case "query":
			q.Query, valid = str(v, 1024)
		case "id":
			q.ID, valid = str(v, 11)
		case "match":
			q.Match, valid = str(v, 16)
			valid = valid && (q.Match == doccorpus.LegacyMatchAll || q.Match == doccorpus.LegacyMatchAny)
		case "limit":
			var n *float64
			if json.Unmarshal(v, &n) == nil && n != nil {
				valid = *n == math.Trunc(*n) && *n >= 1 && *n <= doccorpus.MaxResults
				q.Limit = int(*n)
			}
		}
		if !valid || string(bytes.TrimSpace(v)) == "null" {
			return "", "", q, false
		}
	}
	return format, input, q, true
}

// callLegacyCases returns the CLI's exact search-cases bytes framed as
// untrusted data, with the receipt as structured content.
func (handler *toolHandler) callLegacyCases(ctx context.Context, raw []byte) (map[string]any, *protocol.RPCError) {
	format, input, q, valid := decodeLegacyCaseArguments(raw)
	if !valid {
		return nil, protocol.InvalidParams("Invalid params")
	}
	data, err := doccorpus.ReadLegacyCasesFile(handler.legacyCases.root, input)
	if err == nil {
		data, err = doccorpus.SearchLegacyCasesQuery(format, input, data, q)
	}
	if ctx.Err() != nil {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	if err != nil {
		var coded *doccorpus.Error
		if errors.As(err, &coded) {
			return toolFailureResult(legacyCasesTool, coded.Code, coded.Message)
		}
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	var structured map[string]any
	if json.Unmarshal(data, &structured) != nil {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	framed, frameErr := repoenvelope.Frame(string(data))
	if frameErr != nil {
		return toolFailureResult(legacyCasesTool, repoenvelope.CollisionCode, repoenvelope.CollisionCode)
	}
	result := map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": framed}},
		"isError":           false,
		"structuredContent": structured,
	}
	if encoded, err := json.Marshal(result); err != nil || len(encoded) > consolidateResponseBytes {
		return toolFailureResult(legacyCasesTool, "corpus-refused", "the search receipt exceeds one MCP message; use the CLI")
	}
	return result, nil
}
