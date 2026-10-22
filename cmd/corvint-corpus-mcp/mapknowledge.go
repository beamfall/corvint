package main

import (
	"context"
	"encoding/json"
	"errors"
	"math"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
)

// mapKnowledgeTool serves the site-knowledge projections (SKH-V0-013) over the maps named by --map:
// the same byte-capped page-object and step-IR records `corvint flows appmap pageobjects|steps`
// prints, each pinned to the map revision and source spans. It is listed only with the planner.
const (
	mapKnowledgeTool   = "corvint.map_knowledge"
	maxKnowledgeSource = 512
	maxKnowledgeApp    = 256
)

var mapKnowledgeViews = map[string]func(context.Context, *appmap.Map, []string, appmap.Options) ([]byte, error){
	"page_objects": appmap.PageObjects,
	"steps":        appmap.Steps,
	"setup":        appmap.Setup,
}

func (p *mapPlanner) knowledgeDescriptor() bridge.ToolDescriptor {
	return bridge.ToolDescriptor{
		Name: mapKnowledgeTool,
		Description: "Read the site knowledge mined from committed E2E sources of one configured application map: " +
			"page objects (methods, locators, actions, effects, screen joins), test bodies in the closed step IR, " +
			"or setup records (fixtures, hooks, storageState, mocks, seeding) linked to the tests they arrange, " +
			"pinned to the map revision and source spans. Static, learned authority; opaque steps and unknowns are named.",
		Annotations: bridge.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"view":    map[string]any{"type": "string", "enum": []any{"page_objects", "steps", "setup"}},
				"sources": map[string]any{"type": "array", "minItems": 1, "maxItems": 16, "items": map[string]any{"type": "string", "minLength": 1, "maxLength": maxKnowledgeSource}},
				"app":     map[string]any{"type": "string", "minLength": 1, "maxLength": maxKnowledgeApp},
				"budget":  map[string]any{"type": "integer", "minimum": appmap.MinBudget, "maximum": appmap.MaxBudget},
			},
			"required":             []any{"view", "sources"},
			"additionalProperties": false,
		},
	}
}

type mapKnowledgeArguments struct {
	view    string
	sources []string
	app     string
	budget  int
}

// decodeMapKnowledgeArguments enforces the advertised input schema: a closed key set, no null,
// the required view and sources, and every type and bound.
func decodeMapKnowledgeArguments(raw []byte) (mapKnowledgeArguments, bool) {
	var a mapKnowledgeArguments
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return a, false
	}
	str := func(v json.RawMessage, limit int) (string, bool) {
		var s string
		return s, json.Unmarshal(v, &s) == nil && s != "" && len(s) <= limit
	}
	if fields["view"] == nil || fields["sources"] == nil {
		return a, false
	}
	for key, v := range fields {
		ok := false
		switch key {
		case "view":
			a.view, ok = str(v, 32)
			ok = ok && mapKnowledgeViews[a.view] != nil
		case "sources":
			var items []json.RawMessage
			ok = json.Unmarshal(v, &items) == nil && len(items) >= 1 && len(items) <= 16
			for _, item := range items {
				s, good := str(item, maxKnowledgeSource)
				ok = ok && good
				a.sources = append(a.sources, s)
			}
		case "app":
			a.app, ok = str(v, maxKnowledgeApp)
		case "budget":
			var n *float64 // a JSON number only: a string or null does not decode to it
			if json.Unmarshal(v, &n) == nil && n != nil {
				ok = *n == math.Trunc(*n) && *n >= appmap.MinBudget && *n <= appmap.MaxBudget
				a.budget = int(*n)
			}
		}
		if !ok {
			return a, false
		}
	}
	return a, true
}

// knowledge returns the projection text or a coded failure; ok is false for arguments outside the
// schema. One configured map is used by default; with several, app names exactly one.
func (p *mapPlanner) knowledge(ctx context.Context, raw []byte) (text string, code, message string, ok bool) {
	a, valid := decodeMapKnowledgeArguments(raw)
	if !valid {
		return "", "", "", false
	}
	var picked *appmap.Map
	for _, m := range p.maps {
		if a.app != "" && m.App != a.app {
			continue
		}
		if picked != nil {
			return "", "appmap-invalid-query", "more than one configured map matches; name its app", true
		}
		picked = m
	}
	if picked == nil {
		return "", "appmap-invalid-query", "no configured map has that app", true
	}
	data, err := mapKnowledgeViews[a.view](ctx, picked, a.sources, appmap.Options{Root: p.root, Budget: a.budget})
	if err != nil {
		var coded *gokernel.Error
		if errors.As(err, &coded) {
			return "", coded.Code, coded.Message, true
		}
		return "", "", "", true
	}
	return string(data), "", "", true
}
