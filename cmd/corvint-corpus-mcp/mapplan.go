package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
			"exploration they need. Candidate authority only; draft adds a Playwright skeleton.",
		Annotations: bridge.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"steps":    map[string]any{"type": "array", "minItems": 1, "maxItems": 16, "items": map[string]any{"type": "string", "minLength": 1, "maxLength": 512}},
				"request":  map[string]any{"type": "string", "minLength": 1, "maxLength": maxRequestBytes},
				"revision": map[string]any{"type": "string", "minLength": 1, "maxLength": maxRevisionBytes},
				"budget":   map[string]any{"type": "integer", "minimum": appmap.MinBudget, "maximum": appmap.MaxBudget},
				"full":     map[string]any{"type": "boolean"},
				"draft":    map[string]any{"type": "boolean"},
			},
			"oneOf":                []any{map[string]any{"required": []any{"steps"}}, map[string]any{"required": []any{"request"}}},
			"additionalProperties": false,
		},
	}
}

type mapPlanArguments struct {
	Steps    []string `json:"steps"`
	Request  *string  `json:"request"`
	Revision *string  `json:"revision"`
	Budget   int      `json:"budget"`
	Full     bool     `json:"full"`
	Draft    bool     `json:"draft"`
}

// call returns the plan text or a coded failure; ok is false for arguments outside the schema.
// Empty text with no code is an internal failure.
func (p *mapPlanner) call(ctx context.Context, raw []byte) (text string, code, message string, ok bool) {
	var a mapPlanArguments
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&a) != nil || (len(a.Steps) == 0) == (a.Request == nil) ||
		(a.Request != nil && (*a.Request == "" || len(*a.Request) > maxRequestBytes)) ||
		(a.Revision != nil && (*a.Revision == "" || len(*a.Revision) > maxRevisionBytes)) {
		return "", "", "", false
	}
	steps := a.Steps
	if a.Request != nil {
		steps = appmap.SplitRequest(*a.Request)
	}
	revision := ""
	if a.Revision != nil {
		revision = *a.Revision
	}
	data, err := appmap.Plan(ctx, p.maps, steps, appmap.PlanOptions{
		Options: appmap.Options{Root: p.root, Revision: revision, Budget: a.Budget, Full: a.Full}, Draft: a.Draft})
	if err != nil {
		var coded *gokernel.Error
		if errors.As(err, &coded) {
			return "", coded.Code, coded.Message, true
		}
		return "", "", "", true // an uncoded failure is internal; the caller reports no plan
	}
	return string(data), "", "", true
}
