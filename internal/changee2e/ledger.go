package changee2e

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/Beamfall/corvint/internal/appmap"
)

// LedgerBasis names the ANL edge-ledger relation in an obligations report (CCE-V0-103).
const LedgerBasis = "anl-edge-ledger"

// loadLedger reads the optional --ledger document and binds it to the head catalog's map: a
// ledger of another application, map digest or revision, or a trimmed one, refuses
// `e2e-ledger-invalid` rather than reaching the impact relation (CCE-V0-103). An empty name
// returns nil, and the report lists the relation as not consulted.
func loadLedger(name string, cat *Catalog) (*appmap.EdgeLedger, error) {
	if name == "" {
		return nil, nil
	}
	l, err := appmap.LoadEdgeLedger(name)
	if err != nil {
		return nil, refuse("e2e-ledger-invalid", "%v", err)
	}
	if l.App != cat.m.App || l.MapDigest != cat.m.Digest || l.Revision != cat.Revision {
		return nil, refuse("e2e-ledger-invalid", "the edge ledger was not built from the head map at %s", cat.Revision)
	}
	for k, n := range l.Omitted {
		if n > 0 {
			return nil, refuse("e2e-ledger-invalid", "the edge ledger is trimmed (%s=%d); rebuild it with --full", k, n)
		}
	}
	return l, nil
}

// provenRoute is the AMAP entry projection's shortest PROVEN, receipt-backed path to one screen
// (AMAP-V0-080): the screens from a deep-linkable entry and the test key proving each hop.
type provenRoute struct {
	path, by []string
}

// provenRoutes reads the ledger-bound entry projection of the head map. Only rows whose path
// status is PROVEN over at least one hop are kept; every other screen keeps its catalog reason.
func provenRoutes(ctx context.Context, root string, cat *Catalog, l *appmap.EdgeLedger) (map[string]provenRoute, error) {
	raw, err := appmap.ProjectEntries(ctx, cat.m, l, appmap.Options{Root: root, Revision: cat.Revision, Full: true})
	if err != nil {
		return nil, refuse("e2e-ledger-invalid", "entries: %v", err)
	}
	var doc struct {
		Entries []struct {
			ID         string   `json:"id"`
			ProvenPath []string `json:"proven_path"`
			ProvenBy   []string `json:"proven_by"`
			PathStatus string   `json:"path_status"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, refuse("e2e-ledger-invalid", "entries: %v", err)
	}
	out := map[string]provenRoute{}
	for _, e := range doc.Entries {
		if e.PathStatus == appmap.EdgeProven && len(e.ProvenPath) >= 2 && len(e.ProvenBy) == len(e.ProvenPath)-1 {
			out[e.ID] = provenRoute{e.ProvenPath, e.ProvenBy}
		}
	}
	return out, nil
}

// provenEntry synthesises the path to screen over the ledger's PROVEN edges only (CCE-V0-104).
// The route must start at an admitted deep-link entry block; each hop is the ledger edge between
// its screens proven by the route's test key (the lowest edge ID when several are), and it emits
// the admitted page-object method of that edge's crossing step: the mined, anchored statement a
// receipt saw cross the edge. A hop whose crossing names no map flow or step, whose steps are not
// adjacent on the hop's screens, whose flow declares a precondition the compiled flow does not, or
// whose crossing step is a fill or has no admitted method abstains with a named reason; nothing
// is ever generated for a hop. An empty reason means the route compiled.
func (c *Catalog) provenEntry(obs *Obligations, screen string, flow *appmap.Flow) (*Block, []planStep, []string, []string, string) {
	route, ok := obs.routes[screen]
	if !ok {
		return nil, nil, nil, nil, ""
	}
	entry := c.Block("path:entry:" + route.path[0])
	if entry == nil {
		return nil, nil, nil, nil, "proven-path-entry-not-admitted"
	}
	have := map[string]bool{}
	for _, p := range flow.Preconditions {
		have[p] = true
	}
	var steps []planStep
	for i, test := range route.by {
		e := hopEdge(obs.ledger, route.path[i], route.path[i+1], test)
		if e == nil {
			return nil, nil, nil, nil, "proven-hop-edge-missing"
		}
		from, ef := c.crossingStep(e)
		if from == nil {
			return nil, nil, nil, nil, "proven-hop-evidence-inconsistent"
		}
		// Only the crossing step is emitted, so an earlier step of the crossing flow on the same
		// screen (a fill before its submit) would be dropped: abstain rather than omit it.
		if i := slices.IndexFunc(ef.Steps, func(s appmap.Step) bool { return s.ID == from.ID }); i > 0 && ef.Steps[i-1].Screen == from.Screen {
			return nil, nil, nil, nil, "proven-hop-prefix-not-emitted"
		}
		for _, p := range ef.Preconditions {
			if !have[p] {
				return nil, nil, nil, nil, "proven-hop-precondition-not-set"
			}
		}
		if from.Action == "fill" {
			return nil, nil, nil, nil, "proven-hop-step-is-fill"
		}
		ps, _, reason := c.stepBlock(*from)
		if reason != "" {
			return nil, nil, nil, nil, "proven-hop-" + reason
		}
		steps = append(steps, ps)
	}
	return entry, steps, route.path, route.by, ""
}

// hopEdge is the lowest-ID PROVEN, receipt-backed ledger edge from→to that test proves.
func hopEdge(l *appmap.EdgeLedger, from, to, test string) *appmap.LedgerEdge {
	var best *appmap.LedgerEdge
	for i := range l.Edges {
		e := &l.Edges[i]
		if e.From != from || e.To != to || e.Status != appmap.EdgeProven || e.Evidence == nil ||
			e.Evidence.Verification.Status != appmap.Verified || e.Evidence.Verification.Receipt == nil ||
			e.Evidence.Verification.Receipt.TestKey != test {
			continue
		}
		if best == nil || e.ID < best.ID {
			best = e
		}
	}
	return best
}

// crossingStep returns the crossing's from-step and flow when the map has both, the step is
// immediately followed by the crossing's step, and the two lie on the edge's screens.
func (c *Catalog) crossingStep(e *appmap.LedgerEdge) (*appmap.Step, *appmap.Flow) {
	x := e.Evidence
	for i := range c.m.Flows {
		f := &c.m.Flows[i]
		if f.ID != x.Flow {
			continue
		}
		for j := 0; j+1 < len(f.Steps); j++ {
			a, b := f.Steps[j], f.Steps[j+1]
			if a.ID == x.FromStep && b.ID == x.Step && a.Screen == e.From && b.Screen == e.To {
				return &f.Steps[j], f
			}
		}
	}
	return nil, nil
}
