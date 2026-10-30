package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
)

// mapPlanTool serves the application-map scenario planner (AMSP-V0-010) over the maps named by
// --map at startup. It is listed only when at least one map was configured.
const (
	mapPlanTool       = "corvint.map_plan"
	maxConfiguredMaps = 8
	maxRequestBytes   = 8192
	maxRevisionBytes  = 128
)

type mapPlanner struct {
	root string
	maps []*appmap.Map
}

// newMapPlanner loads every configured map once: a root-relative path that resolves, after
// symbolic links, to a regular file inside the root. A map that changes after startup is not
// reread; restart the server to plan against it.
func newMapPlanner(root string, names []string) (*mapPlanner, error) {
	if len(names) == 0 {
		return nil, nil
	}
	if len(names) > maxConfiguredMaps {
		return nil, errors.New("too many maps")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	resolvedRoot, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, err
	}
	p := &mapPlanner{root: absolute}
	for _, name := range names {
		if !filepath.IsLocal(name) {
			return nil, errors.New("map path must be root-relative")
		}
		resolved, err := filepath.EvalSymlinks(filepath.Join(absolute, name))
		if err != nil {
			return nil, err
		}
		if rel, err := filepath.Rel(resolvedRoot, resolved); err != nil || !filepath.IsLocal(rel) {
			return nil, errors.New("map path leaves the root")
		}
		m, err := appmap.LoadMap(resolved)
		if err != nil {
			return nil, err
		}
		p.maps = append(p.maps, m)
	}
	return p, nil
}

func (p *mapPlanner) descriptor() bridge.ToolDescriptor {
	return bridge.ToolDescriptor{
		Name: mapPlanTool,
		Description: "Plan a plain-language multi-step request over the configured application maps: one session per app, " +
			"minimal navigation, route-parameter handoff and named outcome assertions, with UNMAPPED/STALE steps and the " +
			"exploration they need. Candidate authority only; draft adds a Playwright skeleton. With sources (and request, draft) " +
			"the request is one goal and the result is the site-knowledge-how/0 answer with a guarded draft copied from the " +
			"primary mined test, every line mapped to its source line or a marked TODO; a proposal, never accepted intent.",
		Annotations: bridge.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"steps":    map[string]any{"type": "array", "minItems": 1, "maxItems": 16, "items": map[string]any{"type": "string", "minLength": 1, "maxLength": 512}},
				"request":  map[string]any{"type": "string", "minLength": 1, "maxLength": maxRequestBytes},
				"revision": map[string]any{"type": "string", "minLength": 1, "maxLength": maxRevisionBytes},
				"budget":   map[string]any{"type": "integer", "minimum": appmap.MinBudget, "maximum": appmap.MaxBudget},
				"draft":    map[string]any{"type": "boolean"},
				"sources": map[string]any{"type": "array", "minItems": 1, "maxItems": 16, "items": map[string]any{"type": "string", "minLength": 1, "maxLength": maxKnowledgeSource},
					"description": "with request and draft: E2E source directories for the how draft of one goal"},
				"conditions": map[string]any{"type": "array", "maxItems": appmap.MaxSetupConditions,
					"items": map[string]any{"type": "string", "minLength": 1, "maxLength": maxKnowledgeGoal}, "description": "preconditions; with sources only"},
				"app": map[string]any{"type": "string", "minLength": 1, "maxLength": maxKnowledgeApp, "description": "the configured map's app; with sources only"},
			},
			"oneOf":                []any{map[string]any{"required": []any{"steps"}}, map[string]any{"required": []any{"request"}}},
			"additionalProperties": false,
		},
	}
}

type mapPlanArguments struct {
	steps    []string
	request  string
	revision string
	budget   int
	draft    bool
	// sources, conditions and app select the stage 7 how draft (SKH-V0-162).
	sources    []string
	conditions []string
	app        string
}

// decodeMapPlanArguments enforces the advertised input schema: a closed key set, no null, exactly
// one of steps or request, and every type and bound. There is no full escape over MCP: a plan is
// at most MaxBudget bytes, so the framed text plus its structured copy stay within one message.
func decodeMapPlanArguments(raw []byte) (mapPlanArguments, bool) {
	var a mapPlanArguments
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return a, false
	}
	str := func(v json.RawMessage, limit int) (string, bool) {
		var s string
		return s, json.Unmarshal(v, &s) == nil && s != "" && len(s) <= limit
	}
	_, hasSteps := fields["steps"]
	_, hasRequest := fields["request"]
	if hasSteps == hasRequest {
		return a, false
	}
	for key, v := range fields {
		ok := false
		switch key {
		case "steps":
			var items []json.RawMessage
			ok = json.Unmarshal(v, &items) == nil && len(items) >= 1 && len(items) <= 16
			for _, item := range items {
				s, good := str(item, 512)
				ok = ok && good
				a.steps = append(a.steps, s)
			}
		case "request":
			a.request, ok = str(v, maxRequestBytes)
		case "revision":
			a.revision, ok = str(v, maxRevisionBytes)
		case "budget":
			var n *float64 // a JSON number only: a string or null does not decode to it
			if json.Unmarshal(v, &n) == nil && n != nil {
				ok = *n == math.Trunc(*n) && *n >= appmap.MinBudget && *n <= appmap.MaxBudget
				a.budget = int(*n)
			}
		case "draft":
			ok = json.Unmarshal(v, &a.draft) == nil && string(bytes.TrimSpace(v)) != "null"
		case "sources", "conditions":
			var items []json.RawMessage
			limit, most := maxKnowledgeSource, 16
			if key == "conditions" {
				limit, most = maxKnowledgeGoal, appmap.MaxSetupConditions
			}
			ok = json.Unmarshal(v, &items) == nil && items != nil && len(items) <= most && (key == "conditions" || len(items) >= 1)
			for _, item := range items {
				s, good := str(item, limit)
				ok = ok && good
				if key == "sources" {
					a.sources = append(a.sources, s)
				} else {
					a.conditions = append(a.conditions, s)
				}
			}
		case "app":
			a.app, ok = str(v, maxKnowledgeApp)
		}
		if !ok {
			return a, false
		}
	}
	// The how draft takes one goal as request, needs draft, and has no steps or revision; its
	// conditions and app apply to it only.
	if _, how := fields["sources"]; how {
		if !hasRequest || !a.draft || fields["revision"] != nil || len(a.request) > maxKnowledgeGoal {
			return a, false
		}
	} else if fields["conditions"] != nil || fields["app"] != nil {
		return a, false
	}
	return a, true
}

// call returns the plan text or a coded failure; ok is false for arguments outside the schema.
// Empty text with no code is an internal failure.
func (p *mapPlanner) call(ctx context.Context, raw []byte) (text string, code, message string, ok bool) {
	a, valid := decodeMapPlanArguments(raw)
	if !valid {
		return "", "", "", false
	}
	if a.sources != nil {
		picked, message := p.pick(a.app)
		if picked == nil {
			return "", "appmap-invalid-query", message, true
		}
		data, err := appmap.HowDraft(ctx, picked, a.sources, a.request, a.conditions, nil, nil, appmap.Options{Root: p.root, Budget: a.budget})
		return codedText(data, err)
	}
	steps := a.steps
	if a.request != "" {
		steps = appmap.SplitRequest(a.request)
	}
	data, err := appmap.Plan(ctx, p.maps, steps, appmap.PlanOptions{
		Options: appmap.Options{Root: p.root, Revision: a.revision, Budget: a.budget}, Draft: a.draft})
	return codedText(data, err)
}

// codedText maps a tool result to its text or its coded failure; an uncoded failure is internal.
func codedText(data []byte, err error) (text string, code, message string, ok bool) {
	if err != nil {
		var coded *gokernel.Error
		if errors.As(err, &coded) {
			return "", coded.Code, coded.Message, true
		}
		return "", "", "", true // an uncoded failure is internal; the caller reports no plan
	}
	return string(data), "", "", true
}
