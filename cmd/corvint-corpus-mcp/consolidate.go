package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/repoenvelope"
	"github.com/Beamfall/corvint/internal/testplan"
)

// consolidateTool serves the test consolidation planner (TCN-V0-012). It is listed only when the
// server was started with --consolidation.
const (
	consolidateTool = "corvint.consolidate_tests"
	// maxInlineBytes bounds the inline input document and the plan table to check.
	maxInlineBytes = 1 << 20
	// consolidateResponseBytes leaves room inside one MCP message for the JSON-RPC envelope
	// around the tool result.
	consolidateResponseBytes = protocol.MaxMessageBytes - 4096
)

type consolidator struct {
	root string
}

func (c *consolidator) descriptor() bridge.ToolDescriptor {
	paths := map[string]any{"type": "array", "maxItems": testplan.MaxEvidenceFiles, "items": map[string]any{"type": "string", "minLength": 1, "maxLength": 4096}}
	return bridge.ToolDescriptor{
		Name: consolidateTool,
		Description: "Propose a candidate consolidation of declared test variations (a test-consolidation-input/0 document " +
			"given inline): reused, duplicate, isolated and grouped rows with their reasons, or check a pasted plan table " +
			"against the recomputed one. Optional tests and maps are repository-relative provider documents and " +
			"application maps. Candidate authority only; it runs nothing and writes nothing.",
		Annotations: bridge.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"input":     map[string]any{"type": "string", "minLength": 1, "maxLength": maxInlineBytes},
				"tests":     paths,
				"maps":      paths,
				"max_steps": map[string]any{"type": "integer", "minimum": testplan.MinMaxSteps, "maximum": testplan.MaxMaxSteps},
				"format":    map[string]any{"type": "string", "enum": []any{"table", "json"}},
				"plan":      map[string]any{"type": "string", "minLength": 1, "maxLength": maxInlineBytes},
			},
			"required":             []any{"input"},
			"additionalProperties": false,
		},
	}
}

type consolidateArguments struct {
	input    string
	tests    []string
	maps     []string
	maxSteps int
	format   string
	plan     string
	check    bool
}

// decodeConsolidateArguments enforces the advertised schema: a closed key set, no null, input
// required, every type and bound, and format and plan never together (the CLI's check takes no
// --format).
func decodeConsolidateArguments(raw []byte) (consolidateArguments, bool) {
	a := consolidateArguments{format: "table"}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return a, false
	}
	str := func(v json.RawMessage, limit int) (string, bool) {
		var s string
		return s, json.Unmarshal(v, &s) == nil && s != "" && len(s) <= limit
	}
	list := func(v json.RawMessage) ([]string, bool) {
		var items []json.RawMessage
		if json.Unmarshal(v, &items) != nil || items == nil || len(items) > testplan.MaxEvidenceFiles {
			return nil, false
		}
		out := []string{}
		for _, item := range items {
			s, ok := str(item, 4096)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	}
	_, hasFormat := fields["format"]
	_, a.check = fields["plan"]
	if _, hasInput := fields["input"]; !hasInput || hasFormat && a.check {
		return a, false
	}
	for key, v := range fields {
		ok := false
		switch key {
		case "input":
			a.input, ok = str(v, maxInlineBytes)
		case "plan":
			a.plan, ok = str(v, maxInlineBytes)
		case "tests":
			a.tests, ok = list(v)
		case "maps":
			a.maps, ok = list(v)
		case "format":
			a.format, ok = str(v, 8)
			ok = ok && (a.format == "table" || a.format == "json")
		case "max_steps":
			var n *float64 // a JSON number only: a string or null does not decode to it
			if json.Unmarshal(v, &n) == nil && n != nil {
				ok = *n == math.Trunc(*n) && *n >= testplan.MinMaxSteps && *n <= testplan.MaxMaxSteps
				a.maxSteps = int(*n)
			}
		}
		if !ok || string(bytes.TrimSpace(v)) == "null" {
			return a, false
		}
	}
	return a, true
}

// confine resolves a repository-relative path, after symbolic links, to a file inside the root.
func (c *consolidator) confine(name string) (string, error) {
	refuse := &gokernel.Error{Code: "test-plan-invalid-arguments", Message: "tests and maps must be repository-relative paths inside the root"}
	if !filepath.IsLocal(name) {
		return "", refuse
	}
	resolvedRoot, err := filepath.EvalSymlinks(c.root)
	if err != nil {
		return "", refuse
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(c.root, name))
	if err != nil {
		return "", refuse
	}
	if rel, err := filepath.Rel(resolvedRoot, resolved); err != nil || !filepath.IsLocal(rel) {
		return "", refuse
	}
	return resolved, nil
}

// call returns the CLI's exact output bytes and the plan object, or a coded failure; ok is false
// for arguments outside the schema.
func (c *consolidator) call(ctx context.Context, raw []byte) (text string, plan map[string]any, code, message string, ok bool) {
	a, valid := decodeConsolidateArguments(raw)
	if !valid {
		return "", nil, "", "", false
	}
	request := testplan.Request{Root: c.root, Input: []byte(a.input), MaxSteps: a.maxSteps}
	for _, group := range []struct {
		names []string
		into  *[]string
	}{{a.tests, &request.Tests}, {a.maps, &request.Maps}} {
		for _, name := range group.names {
			resolved, err := c.confine(name)
			if err != nil {
				return failure(err)
			}
			*group.into = append(*group.into, resolved)
		}
	}
	p, err := testplan.Run(ctx, request)
	if err != nil {
		return failure(err)
	}
	output := p.Table()
	if a.check {
		if err := testplan.Check(p, []byte(a.plan)); err != nil {
			return failure(err)
		}
	} else if a.format == "json" {
		output = p.JSON()
	}
	if json.Unmarshal(p.JSON(), &plan) != nil {
		return "", nil, "", "", true
	}
	return string(output), plan, "", "", true
}

func failure(err error) (string, map[string]any, string, string, bool) {
	var coded *gokernel.Error
	if errors.As(err, &coded) {
		return "", nil, coded.Code, coded.Message, true
	}
	return "", nil, "", "", true // an uncoded failure is internal; the caller reports no plan
}

// callConsolidate frames the CLI bytes in the untrusted-data envelope with the plan as structured
// content, and refuses with test-plan-bound-exceeded rather than exceed one MCP message.
func (handler *toolHandler) callConsolidate(ctx context.Context, raw []byte) (map[string]any, *protocol.RPCError) {
	text, plan, code, message, valid := handler.consolidator.call(ctx, raw)
	if !valid {
		return nil, protocol.InvalidParams("Invalid params")
	}
	if ctx.Err() != nil {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	if code != "" {
		return toolFailureResult(consolidateTool, code, message)
	}
	if text == "" {
		return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
	}
	framed, frameErr := repoenvelope.Frame(text)
	if frameErr != nil {
		return toolFailureResult(consolidateTool, repoenvelope.CollisionCode, repoenvelope.CollisionCode)
	}
	result := map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": framed}},
		"isError":           false,
		"structuredContent": plan,
	}
	if encoded, err := json.Marshal(result); err != nil || len(encoded) > consolidateResponseBytes {
		return toolFailureResult(consolidateTool, "test-plan-bound-exceeded", "the plan and its structured copy exceed one MCP message; use the CLI")
	}
	return result, nil
}
