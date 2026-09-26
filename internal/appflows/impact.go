package appflows

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
)

// ImpactSchema names the `flows impact` document (AFU-V1-017).
const ImpactSchema = "application-flow-impact/1"

// ImpactReport names what the base..HEAD changed paths reach through links and the impact graph.
type ImpactReport struct {
	Schema       string       `json:"schema"`
	Base         string       `json:"base"`
	Revision     string       `json:"revision"`
	ChangedPaths []string     `json:"changed_paths"`
	Graph        ImpactGraph  `json:"graph"`
	Flows        []ImpactFlow `json:"flows"`
}

// ImpactGraph identifies the impact graph walked; an UNKNOWN scope means a hit list may be short.
type ImpactGraph struct {
	Digest  string   `json:"digest"`
	Scope   string   `json:"scope"`
	Unknown []string `json:"unknown"`
}

// ImpactFlow is one reached flow with the variations and test keys its hit links reach.
type ImpactFlow struct {
	FlowID     string      `json:"flow_id"`
	Variations []string    `json:"variations"`
	TestKeys   []string    `json:"test_keys"`
	Hits       []ImpactHit `json:"hits"`
}

// ImpactHit is one link whose target a changed path reaches. Via is the path: the changed path, then
// each impact-graph unit from the changed one to the one owning the target, then the target path; a
// target path that itself changed has Via of that path alone.
type ImpactHit struct {
	From        string     `json:"from"`
	Basis       string     `json:"basis"`
	ReviewState string     `json:"review_state"`
	Target      LinkTarget `json:"target"`
	Via         []string   `json:"via"`
}

// ImpactDirtyWorktree is the graph unknown reason of an impact report built from a worktree that
// differs from HEAD.
const ImpactDirtyWorktree = "DIRTY_WORKTREE"

// FlowImpactAt is the `flows impact` pipeline shared by the CLI verb and the MCP tool: it diffs the
// resolved base commit against HEAD, builds the impact graph over every supported language, marks the
// graph scope UNKNOWN when the worktree differs from HEAD, and reports through FlowImpact.
func FlowImpactAt(ctx context.Context, root string, set IntentSet, base string) ([]byte, error) {
	gitPath := gitstatus.Executable()
	if !filepath.IsAbs(gitPath) {
		return nil, errors.New("git executable unavailable")
	}
	changed, err := affected.RangePaths(ctx, gitPath, root, base)
	if err != nil {
		return nil, &gokernel.Error{Code: "unsupported-affected-status", Message: err.Error()}
	}
	dirty, err := affected.DirtyPaths(ctx, gitPath, root)
	if err != nil {
		return nil, &gokernel.Error{Code: "unsupported-affected-status", Message: err.Error()}
	}
	graph, err := affected.Build(root, languages.All()...)
	if err != nil {
		return nil, err
	}
	recheck, err := affected.DirtyPaths(ctx, gitPath, root)
	if err != nil {
		return nil, &gokernel.Error{Code: "unsupported-affected-status", Message: err.Error()}
	}
	plan := affected.Select(graph, changed)
	// The graph is walked from the working tree while intents and changed paths come from HEAD; a
	// worktree that differs from HEAD may have built a different graph, so the hit list may be short.
	if differs := slices.Compact(slices.Sorted(slices.Values(append(dirty, recheck...)))); len(differs) != 0 {
		plan.Scope = affected.ScopeUnknown
		plan.Unknown = append(plan.Unknown, affected.Unknown{Reason: ImpactDirtyWorktree,
			Detail: fmt.Sprintf("the impact graph was built from a worktree that differs from HEAD at %d paths, first %s", len(differs), differs[0])})
	}
	return FlowImpact(ctx, root, set, base, graph, plan)
}

// FlowImpact reports the flows reached from plan's changed paths (AFU-V1-017). The caller builds the
// graph and selects plan over the base..HEAD tree diff.
func FlowImpact(ctx context.Context, root string, set IntentSet, base string, graph *affected.Graph, plan affected.Plan) ([]byte, error) {
	links, at, err := evaluateSet(ctx, root, set)
	if err != nil {
		return nil, err
	}
	report := ImpactReport{Schema: ImpactSchema, Base: base, Revision: at.commit, ChangedPaths: plan.Dirty,
		Graph: ImpactGraph{Digest: plan.GraphDigest, Scope: plan.Scope, Unknown: []string{}}, Flows: []ImpactFlow{}}
	for _, u := range plan.Unknown {
		report.Graph.Unknown = append(report.Graph.Unknown, u.Reason+": "+u.Detail)
	}
	changedBy := changedUnits(graph, plan.Dirty)
	for _, intent := range set.Flows {
		hits := []ImpactHit{}
		for _, l := range flowLinks(links, intent.FlowID) {
			if via := reach(graph, changedBy, plan.Dirty, l.Target.Path); via != nil {
				hits = append(hits, ImpactHit{From: l.From, Basis: l.Basis, ReviewState: l.ReviewState, Target: l.Target, Via: via})
			}
		}
		if len(hits) != 0 {
			report.Flows = append(report.Flows, impactFlow(intent, hits))
		}
	}
	return doccorpus.Encode(report)
}

// changedUnits maps each unit owning a changed path to the first such path in sorted order.
func changedUnits(graph *affected.Graph, changed []string) map[string]string {
	units := map[string]string{}
	for _, p := range changed {
		if id, ok := graph.OwnerOf(p); ok && units[id] == "" {
			units[id] = p
		}
	}
	return units
}

// reach returns the shortest path from a changed path to target, or nil. It walks imports forward
// from the target's owning unit in sorted breadth-first order, so the first unit owning a changed
// path is the nearest one.
func reach(graph *affected.Graph, changedBy map[string]string, changed []string, target string) []string {
	if target == "" {
		return nil
	}
	if slices.Contains(changed, target) {
		return []string{target}
	}
	owner, ok := graph.OwnerOf(target)
	if !ok {
		return nil
	}
	parent := map[string]string{owner: ""}
	for queue := []string{owner}; len(queue) != 0; queue = queue[1:] {
		id := queue[0]
		if changedBy[id] != "" {
			return append(append([]string{changedBy[id]}, unitChain(parent, id)...), target)
		}
		unit, _ := graph.Unit(id)
		for _, next := range forward(unit, id == owner && slices.Contains(unit.Tests, target)) {
			if _, seen := parent[next]; !seen {
				parent[next] = id
				queue = append(queue, next)
			}
		}
	}
	return nil
}

// forward lists a unit's import edges, with its test-only imports when the walk
// starts at one of its tests.
func forward(unit affected.Unit, fromTest bool) []string {
	if !fromTest {
		return unit.Imports
	}
	return append(append([]string(nil), unit.Imports...), unit.TestImports...)
}

// unitChain lists units from the changed unit back along parent links to the target's owner.
func unitChain(parent map[string]string, id string) []string {
	chain := []string{}
	for ; id != ""; id = parent[id] {
		chain = append(chain, id)
	}
	return chain
}

// impactFlow names the variations each hit reaches (its own when from is a variation, otherwise
// every variation listing the step or outcome) and those variations' test keys, any basis.
func impactFlow(intent FlowIntent, hits []ImpactHit) ImpactFlow {
	variations, keys := []string{}, []string{}
	for _, h := range hits {
		for _, v := range intent.Variations {
			if h.From == v.VariationID || slices.Contains(v.Steps, h.From) || slices.Contains(v.Outcomes, h.From) {
				variations = append(variations, v.VariationID)
				keys = append(keys, variationTests(intent, v.VariationID)...)
			}
		}
		if h.Target.Type == "test" {
			keys = append(keys, h.Target.TestKey)
		}
	}
	return ImpactFlow{FlowID: intent.FlowID, Variations: sortedSet(variations), TestKeys: sortedSet(keys), Hits: hits}
}
