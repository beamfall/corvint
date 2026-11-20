// SPDX-License-Identifier: Apache-2.0

package appflows

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
	"github.com/Beamfall/corvint/internal/testid"
	"github.com/Beamfall/corvint/internal/testinventory/reader"
)

const (
	linkStale     = "link-stale"
	linkUnknown   = "link-reachability-unknown"
	linkDisagrees = "link-reachability-disagrees"
	linkDangling  = "link-dangling"
	browserGroup  = "browser-group-full"
)

type linkCandidate struct {
	inv         testinventory.Inventory
	verdict     testinventory.Verdict
	files       map[string]string
	variants    map[string]linkVariation
	committed   bool
	invalidPins map[string]bool
}

// bindingReason checks the acceptance itself, independent of a particular change.
func (c linkCandidate) bindingReason(l FlowTestLink, file string) string {
	v, ok := c.variants[l.VariationID]
	switch {
	case !ok:
		return linkDangling
	case !l.HasAcceptance():
		return "unaccepted"
	case c.invalidPins[l.TestID+"\x00"+l.VariationID] || c.files[file] == "" || l.TestContentHash != c.files[file] || l.VariationDigest != v.digest:
		return linkStale
	}
	return ""
}

func linksByTest(links []FlowTestLink) map[string][]FlowTestLink {
	byTest := map[string][]FlowTestLink{}
	for _, l := range links {
		byTest[l.TestID] = append(byTest[l.TestID], l)
	}
	return byTest
}

// A version 0 declaration with a canonical inventory participates before its first
// acceptance. Without that inventory its existing discovery/provider contract applies.
func usesCanonicalLinks(root string, d *FlowsDeclaration) bool {
	if d.Version == 1 {
		return true
	}
	_, err := os.Lstat(filepath.Join(root, testinventory.Path))
	return !os.IsNotExist(err)
}

// Judge committed inventory against the candidate, including new or modified test files.
// Only committed declarations and inventories can authorize an exclusion.
func readLinkCandidate(ctx context.Context, root string, d *FlowsDeclaration) (linkCandidate, error) {
	var c linkCandidate
	g := testinventory.Git{Root: root, Executable: gitstatus.Executable()}
	head, err := g.Head(ctx)
	if err != nil {
		return c, err
	}
	tree, err := testinventory.NewCommitTree(ctx, g, head)
	if err != nil {
		return c, err
	}
	data, err := tree.Read(testinventory.Path, testinventory.MaxBytes)
	if err != nil {
		return c, err
	}
	c.inv, err = testinventory.Decode(data)
	if err != nil {
		return c, err
	}
	work, err := testinventory.NewWorkTree(ctx, g)
	if err != nil {
		return c, err
	}
	c.files, err = work.Files()
	if err != nil {
		return c, err
	}
	pinned, _ := tree.Files()
	c.committed = pinned[FlowsDeclarationPath] != "" && pinned[FlowsDeclarationPath] == c.files[FlowsDeclarationPath] && pinned[testinventory.Path] == c.files[testinventory.Path]
	instances := testinventory.Detect(c.files, work.Read)
	c.verdict = testinventory.JudgeData(data, true, nil, c.files, instances)
	set, err := LoadIntents(root, d.Dir)
	if err != nil {
		return c, err
	}
	c.variants = linkVariations(set)
	for _, f := range set.Flows {
		name := set.IntentPath(f.FlowID)
		if c.files[name] != pinned[name] {
			c.committed = false
		}
	}
	format := "sha1"
	if len(head) == 64 {
		format = "sha256"
	}
	checked := map[string]bool{}
	for _, l := range d.Links {
		id, _ := testid.Parse(l.TestID)
		if checked[id.File] {
			continue
		}
		checked[id.File] = true
		if raw, e := readSource(root, id.File); e == nil {
			c.files[id.File] = testinventory.BlobID(format, raw)
		} else {
			delete(c.files, id.File)
		}
	}
	c.invalidPins = invalidLinkPins(ctx, g, head, d)
	return c, nil
}

// Check the recorded digests at the acceptance commit as well as at the candidate.
func invalidLinkPins(ctx context.Context, g testinventory.Git, head string, d *FlowsDeclaration) map[string]bool {
	invalid := map[string]bool{}
	commits := map[string][]FlowTestLink{}
	for _, l := range d.Links {
		if l.HasAcceptance() {
			commits[l.PinnedCommit] = append(commits[l.PinnedCommit], l)
		}
	}
	for commit, links := range commits {
		files, variations := bindingAt(ctx, g, head, commit, d.Dir)
		for _, l := range links {
			id, _ := testid.Parse(l.TestID)
			if files[id.File] == "" || files[id.File] != l.TestContentHash || variations[l.VariationID].digest == "" || variations[l.VariationID].digest != l.VariationDigest {
				invalid[l.TestID+"\x00"+l.VariationID] = true
			}
		}
	}
	return invalid
}

func bindingAt(ctx context.Context, g testinventory.Git, head, commit, dir string) (map[string]string, map[string]linkVariation) {
	if _, err := g.Output(ctx, "merge-base", "--is-ancestor", commit, head); err != nil {
		return nil, nil
	}
	tree, err := testinventory.NewCommitTree(ctx, g, commit)
	if err != nil {
		return nil, nil
	}
	raw, err := tree.Read(FlowsDeclarationPath, maxFlowsDeclarationBytes)
	if err != nil {
		return nil, nil
	}
	d, err := decodeFlowsDeclaration(raw)
	if err != nil || d.Dir != dir {
		return nil, nil
	}
	set, err := LoadIntentsAt(ctx, g.Root, dir, commit)
	if err != nil {
		return nil, nil
	}
	files, _ := tree.Files()
	return files, linkVariations(set)
}

func joinAcceptedLinks(ctx context.Context, root string, graph *affected.Graph, plan affected.Plan, d *FlowsDeclaration) affected.Plan {
	out := &affected.E2EPlan{Flows: d.Dir, Scope: affected.E2EScopeBounded, Inventory: affected.E2EInventory{Source: affected.E2EInventoryFlows, State: "n/a"}, Selected: []affected.E2ESelection{}}
	plan.E2E = out
	c, err := readLinkCandidate(ctx, root, d)
	if err == nil && len(c.verdict.Instances) == 0 {
		err = errors.New("no declared runner instances")
	}
	j := flowJoiner{ctx: ctx, root: root, declared: d, graph: graph, plan: plan, selected: map[e2eFile]*affected.E2ESelection{}}
	// A runner name or sibling config does not bind the declared discovery suite.
	// Keep its file-level obligations alongside any other canonical instances.
	coversConfig := c.committed && slices.ContainsFunc(c.verdict.Instances, func(v testinventory.InstanceVerdict) bool {
		return v.Runner == testid.Playwright && !v.Full && v.Config == d.PlaywrightConfig
	})
	if d.PlaywrightConfig != "" && !coversConfig {
		set, loadErr := LoadIntentsAt(ctx, root, d.Dir, "HEAD")
		if loadErr != nil {
			j.full = true
			j.unknown = append(j.unknown, affected.Unknown{Reason: affected.UnknownE2EInventoryUnderivable, Detail: d.Dir, Hint: loadErr.Error()})
		} else {
			j.set = set
			out.Inventory = j.deriveInventory()
		}
	}
	if err != nil {
		j.full = true
		j.unknown = append(j.unknown, affected.Unknown{Reason: affected.UnknownE2EInventoryUnderivable, Detail: FlowsDeclarationPath, Hint: err.Error()})
		out.Unknown = j.unknown
		j.finishCanonicalPlan(out)
		return plan
	}
	j.checkGlobal()
	j.checkPlanUnbounded()
	inventoryFiles := 0
	for _, in := range c.inv.Instances {
		if in.Runner == testid.Playwright || in.Runner == testid.Pytest || in.Runner == testid.RSpec {
			inventoryFiles += len(in.Files)
		}
	}
	j.checkBounds(inventoryFiles + len(j.inventory))
	out.Unknown = j.unknown
	// An unresolved browser runtime edge still permits positive static hits.
	// Two static misses only authorize exclusion when that edge is also bounded.
	available := graph != nil && c.committed && !j.full && (plan.Scope == affected.ScopeBounded || plan.Scope == affected.ScopeUnknown && len(plan.Unknown) > 0)
	for _, u := range plan.Unknown {
		if u.Reason != affected.UnknownNoSelectableTest && !(u.Reason == affected.UnknownLanguageFrontier && frontierReason(u.Detail) == typescript.FrontierE2ERuntimeDependency) {
			available = false
		}
	}
	owners := map[string]string{}
	if graph != nil {
		owners = changedUnits(graph, plan.Dirty)
		for _, p := range plan.Dirty {
			if _, ok := graph.OwnerOf(p); !ok {
				available = false
			}
		}
	}
	byTest := linksByTest(d.Links)
	verdicts := map[string]testinventory.InstanceVerdict{}
	plan.FullInstances = maps.Clone(plan.FullInstances)
	if plan.FullInstances == nil {
		plan.FullInstances = map[string]string{}
	}
	for _, v := range c.verdict.Instances {
		if v.Runner != testid.Playwright && v.Runner != testid.Pytest && v.Runner != testid.RSpec {
			continue
		}
		verdicts[v.Key] = v
		if v.Full {
			plan.FullInstances[v.Key] = v.Reason
			out.Unknown = append(out.Unknown, affected.Unknown{Reason: affected.UnknownE2EInventoryUnderivable, Detail: v.Key, Hint: v.Reason})
			out.Scope = affected.E2EScopeFull
		}
		for _, p := range v.Unassigned {
			if v.Runner == testid.Playwright {
				sel := j.pick(e2eFile{path: p}, testinventory.ReasonUnassigned)
				sel.Configs = appendUnique(sel.Configs, v.Config)
			} else {
				plan.FullInstances[v.Key] = testinventory.ReasonUnassigned
			}
			out.Inventory.Unlinked = appendUnique(out.Inventory.Unlinked, p)
		}
	}
	known := map[string]bool{}
	for _, instance := range c.inv.Instances {
		iv, declared := verdicts[instance.Key]
		if !declared {
			continue
		}
		var runtimeExclusions map[e2eFile]bool
		for _, t := range instance.Tests {
			if !browserTest(instance.Runner, t) || slices.Contains(iv.Removed, t.File) {
				continue
			}
			known[t.ID] = true
			out.Inventory.Count++
			id, _ := testid.Parse(t.ID)
			project := ""
			if len(id.Segments) > 0 {
				project = id.Segments[0]
			}
			file := e2eFile{project, t.File}
			state, hit := assessFlowTest(byTest[t.ID], c, t, id, graph, owners, plan.Dirty, available && !iv.Full)
			if state == "" && instance.Runner == testid.Playwright {
				if runtimeExclusions == nil {
					runtimeExclusions = provenRuntimeExclusions(root, iv.Config, plan.Dirty)
				}
				if !runtimeExclusions[file] {
					state = linkUnknown
				}
			}
			if state != "" && state != affected.E2EReasonFlowReach {
				out.Inventory.Unlinked = appendUnique(out.Inventory.Unlinked, t.File)
			}
			if instance.Runner != testid.Playwright {
				// The existing executor consumes FullInstances irrespective of any graph selection.
				if plan.FullInstances[instance.Key] == "" {
					plan.FullInstances[instance.Key] = browserGroup
				}
				out.Unknown = append(out.Unknown, affected.Unknown{Reason: browserGroup, Detail: t.ID, Hint: "run the whole " + instance.Key + " group"})
				if state != "" && state != affected.E2EReasonFlowReach {
					out.Unknown = append(out.Unknown, affected.Unknown{Reason: state, Detail: t.ID})
				}
				continue
			}
			if state == "" {
				switch {
				case plan.FullInstances[instance.Key] != "":
					state = plan.FullInstances[instance.Key]
				case j.full:
					state = j.fullReason()
				case slices.Contains(iv.Unassigned, t.File):
					state = testinventory.ReasonUnassigned
				case slices.ContainsFunc(plan.Selected, func(s affected.Selection) bool { return slices.Contains(s.Tests, t.File) }):
					state = affected.E2EReasonStaticReach
				default:
					continue
				}
			}
			sel := j.pick(file, state)
			sel.Configs = appendUnique(sel.Configs, instance.Config)
			if state == linkStale || state == linkUnknown || state == linkDisagrees || state == linkDangling || state == "unaccepted" {
				sel.Reasons = appendUnique(sel.Reasons, testinventory.ReasonUnassigned)
			}
			sel.TestKeys = appendUnique(sel.TestKeys, t.ID)
			if hit != nil {
				if !slices.Contains(sel.Flows, *hit) {
					sel.Flows = append(sel.Flows, *hit)
				}
				for i, s := range plan.Selected {
					if s.UnitID == "typescript:playwright:"+t.File && !j.full && plan.FullInstances[instance.Key] == "" {
						plan.Selected[i].Tier = affected.TierFlow
						plan.Selected[i].Reason = affected.E2EReasonFlowReach
					}
				}
			}
		}
	}
	for _, l := range d.Links {
		if !known[l.TestID] {
			out.Unknown = append(out.Unknown, affected.Unknown{Reason: linkDangling, Detail: l.TestID + " " + l.VariationID})
		}
	}
	j.finishCanonicalPlan(out)
	return plan
}

func (j *flowJoiner) finishCanonicalPlan(out *affected.E2EPlan) {
	for _, file := range j.inventory {
		sel := j.pick(file, testinventory.ReasonUnassigned)
		sel.Configs = appendUnique(sel.Configs, j.declared.PlaywrightConfig)
		sel.TestKeys = nil // A different config's ID cannot restrict this file.
		out.Inventory.Unlinked = appendUnique(out.Inventory.Unlinked, file.path)
	}
	for _, sel := range j.selected {
		slices.Sort(sel.TestKeys)
		slices.Sort(sel.Reasons)
		slices.SortFunc(sel.Flows, compareHits)
		out.Selected = append(out.Selected, *sel)
	}
	slices.SortFunc(out.Selected, func(a, b affected.E2ESelection) int {
		return strings.Compare(a.Project+"\x00"+a.Path, b.Project+"\x00"+b.Path)
	})
	slices.Sort(out.Inventory.Unlinked)
	if j.full {
		out.Scope = affected.E2EScopeFull
	}
}

// Only a file explicitly proved excludable by the per-test Playwright closure
// can discharge a runtime dependency. A missing frontier entry or a file absent
// from the import walk is not such a proof (TJAA-V0-140/141, FLP-V1-010).
func provenRuntimeExclusions(root, config string, dirty []string) map[e2eFile]bool {
	proven := map[e2eFile]bool{}
	closures, err := typescript.PlaywrightTestClosures(root, config)
	if err != nil {
		return proven
	}
	for _, closure := range closures {
		if closure.Unknown != "" || closure.Widened != "" {
			continue
		}
		// The import walk cannot see a browser fetching application files by URL.
		for project, dir := range closure.TestDirs {
			proven[e2eFile{project, closure.Test}] = dir != "" && !slices.ContainsFunc(dirty, func(path string) bool {
				_, found := slices.BinarySearch(closure.Excludable, path)
				return !found || (dir != "." && !strings.HasPrefix(path, dir+"/"))
			})
		}
	}
	return proven
}

// assignmentReason is shared by default proposals and selection. All links
// of the test must bind and their source paths must resolve at the candidate.
func (c linkCandidate) assignmentReason(links []FlowTestLink, t testinventory.Test, id testid.ID, graph *affected.Graph) string {
	if len(links) == 0 {
		return testinventory.ReasonUnassigned
	}
	for _, l := range links {
		if reason := c.bindingReason(l, id.File); reason != "" {
			return reason
		}
	}
	if graph == nil || t.Duplicate || t.Reason != "" || id.IsFileLevel() {
		return linkUnknown
	}
	if _, ok := graph.OwnerOf(id.File); !ok {
		return linkUnknown
	}
	for _, l := range links {
		sources := c.variants[l.VariationID].sources
		if len(sources) == 0 || slices.ContainsFunc(sources, func(p string) bool {
			_, ok := graph.OwnerOf(p)
			return !ok
		}) {
			return linkUnknown
		}
	}
	return ""
}

func assessFlowTest(links []FlowTestLink, c linkCandidate, t testinventory.Test, id testid.ID, graph *affected.Graph, owners map[string]string, dirty []string, available bool) (string, *affected.E2EFlowHit) {
	if reason := c.assignmentReason(links, t, id, graph); reason != "" {
		return reason, nil
	}
	if !available {
		return linkUnknown, nil
	}
	reached := reach(graph, owners, dirty, id.File) != nil
	var hit *affected.E2EFlowHit
	for _, l := range links {
		v := c.variants[l.VariationID]
		var flowHit *affected.E2EFlowHit
		for _, p := range v.sources {
			if via := reach(graph, owners, dirty, p); len(via) > 0 && flowHit == nil {
				flowHit = &affected.E2EFlowHit{Flow: v.flow, Variation: v.id, ChangedPath: via[0], Target: p}
			}
		}
		if reached != (flowHit != nil) {
			return linkDisagrees, nil
		}
		if flowHit != nil {
			hit = flowHit
		}
	}
	if hit != nil {
		return affected.E2EReasonFlowReach, hit
	}
	return "", nil
}
