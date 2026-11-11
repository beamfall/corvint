// SPDX-License-Identifier: Apache-2.0

package appflows

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

// One affected plan for every test kind (APK-V0-001 to APK-V0-007) with a
// derived E2E inventory (AFU-V1-140 to AFU-V1-145): `.corvint/flows.json`
// declares the intent directory once, optional kind rules and an optional
// Playwright config with its discovery receipt. `corvint affected` then joins
// the flows itself: every selection carries its declared test kind, and the
// plan's `e2e` member lists the E2E test files the change requires, each
// naming the flow, variation and changed path that selected it. Missing or
// stale evidence widens the answer or marks it UNKNOWN; it never drops a test.
const (
	FlowsDeclarationPath    = ".corvint/flows.json"
	FlowsDeclarationProfile = "corvint-flows/0"

	maxFlowsDeclarationBytes = 64 << 10
	maxFlowsKindRules        = 256
	// maxFlowPlanDirty and maxFlowPlanInventory bound the join (APK-V0-004,
	// APK-V0-006): above either, every inventory file is selected.
	maxFlowPlanDirty     = 4096
	maxFlowPlanInventory = 16384
)

type flowsDeclarationFile struct {
	Profile    string                 `json:"profile"`
	Flows      string                 `json:"flows"`
	Kinds      []flowsKindRule        `json:"kinds,omitempty"`
	Playwright *flowsPlaywrightMember `json:"playwright,omitempty"`
}

type flowsKindRule struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

type flowsPlaywrightMember struct {
	Config    string `json:"config"`
	Discovery string `json:"discovery"`
}

// FlowsDeclaration is the validated `.corvint/flows.json`.
type FlowsDeclaration struct {
	// Dir is the intent directory.
	Dir string
	// Kinds are the path-prefix rules, in file order; the longest matching
	// prefix wins (APK-V0 Definitions).
	Kinds []KindRule
	// PlaywrightConfig and PlaywrightDiscovery are empty without the
	// `playwright` member (AFU-V1-140).
	PlaywrightConfig    string
	PlaywrightDiscovery string
}

// KindRule assigns Kind to every path at or under Path; "." is the whole
// repository.
type KindRule struct {
	Path string
	Kind string
}

// ReadFlowsDeclaration reads the declaration under root: nil with none, an
// error for one that is unreadable, not a regular file, oversized or invalid
// in any member.
func ReadFlowsDeclaration(root string) (*FlowsDeclaration, error) {
	name := filepath.Join(root, filepath.FromSlash(FlowsDeclarationPath))
	info, err := os.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", FlowsDeclarationPath, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: not a regular file", FlowsDeclarationPath)
	}
	if info.Size() > maxFlowsDeclarationBytes {
		return nil, fmt.Errorf("%s: larger than %d bytes", FlowsDeclarationPath, maxFlowsDeclarationBytes)
	}
	body, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", FlowsDeclarationPath, err)
	}
	var declared flowsDeclarationFile
	if err := json.UnmarshalRead(bytes.NewReader(body), &declared, json.RejectUnknownMembers(true)); err != nil {
		return nil, fmt.Errorf("%s: %w", FlowsDeclarationPath, err)
	}
	if declared.Profile != FlowsDeclarationProfile {
		return nil, fmt.Errorf("%s: profile must be %s", FlowsDeclarationPath, FlowsDeclarationProfile)
	}
	if !safePath(declared.Flows) || declared.Flows == "." {
		return nil, fmt.Errorf("%s: flows must name a directory inside the repository root", FlowsDeclarationPath)
	}
	if len(declared.Kinds) > maxFlowsKindRules {
		return nil, fmt.Errorf("%s: at most %d kinds rules", FlowsDeclarationPath, maxFlowsKindRules)
	}
	out := &FlowsDeclaration{Dir: declared.Flows}
	seen := map[string]bool{}
	for _, rule := range declared.Kinds {
		if (rule.Path != "." && !affected.ValidRelativePath(rule.Path)) || seen[rule.Path] {
			return nil, fmt.Errorf("%s: kinds rule path %q", FlowsDeclarationPath, rule.Path)
		}
		if rule.Kind != affected.TestKindUnit && rule.Kind != affected.TestKindIntegration {
			return nil, fmt.Errorf("%s: kinds rule %q must be unit or integration", FlowsDeclarationPath, rule.Path)
		}
		seen[rule.Path] = true
		out.Kinds = append(out.Kinds, KindRule{Path: rule.Path, Kind: rule.Kind})
	}
	if declared.Playwright != nil {
		config, discovery := declared.Playwright.Config, declared.Playwright.Discovery
		if !affected.ValidRelativePath(config) || !playwrightConfigName(config) {
			return nil, fmt.Errorf("%s: playwright.config must be a repository-relative JavaScript or TypeScript source", FlowsDeclarationPath)
		}
		if !affected.ValidRelativePath(discovery) {
			return nil, fmt.Errorf("%s: playwright.discovery must be a repository-relative path", FlowsDeclarationPath)
		}
		out.PlaywrightConfig, out.PlaywrightDiscovery = config, discovery
	}
	return out, nil
}

func playwrightConfigName(name string) bool {
	switch path.Ext(name) {
	case ".js", ".cjs", ".mjs", ".ts", ".cts", ".mts":
		return true
	}
	return false
}

// KindOf is the declared kind of a selection whose test files are paths:
// the kind of the longest matching rule when every path agrees, else
// UNKNOWN (APK-V0-002).
func (d *FlowsDeclaration) KindOf(paths []string) string {
	kind := ""
	for _, p := range paths {
		k := d.kindOfPath(p)
		if k == "" || kind != "" && kind != k {
			return affected.TestKindUnknown
		}
		kind = k
	}
	if kind == "" {
		return affected.TestKindUnknown
	}
	return kind
}

func (d *FlowsDeclaration) kindOfPath(p string) string {
	best, kind := -1, ""
	for _, rule := range d.Kinds {
		if rule.Path == "." {
			if best < 0 {
				best, kind = 0, rule.Kind
			}
			continue
		}
		if under(p, rule.Path) && len(rule.Path)+1 > best {
			best, kind = len(rule.Path)+1, rule.Kind
		}
	}
	return kind
}

// JoinFlowPlan applies the flows declaration to a plan (APK-V0-001 to
// APK-V0-007): every selection gains its test kind and the plan gains the
// `e2e` member. The intents are read at HEAD and their links evaluated at
// measured, the commit the change is measured from (the resolved base, else
// "" for HEAD), so a change to a reviewed target is a reach, not a stale
// link (APK-V0-004). A plan without the declaration is returned unchanged.
func JoinFlowPlan(ctx context.Context, root string, graph *affected.Graph, plan affected.Plan, measured string) affected.Plan {
	declared, err := ReadFlowsDeclaration(root)
	if declared == nil && err == nil {
		return plan
	}
	plan.Selected = slices.Clone(plan.Selected)
	if err != nil {
		return flowsDeclarationInvalid(plan, "", err.Error())
	}
	for i, s := range plan.Selected {
		paths := s.Tests
		if len(paths) == 0 && s.Kind == "DECLARED_CONSUMER" {
			paths = []string{strings.TrimPrefix(s.UnitID, "declared-consumer:")}
		}
		plan.Selected[i].TestKind = declared.KindOf(paths)
	}
	if graph == nil {
		return flowsDeclarationInvalid(plan, declared.Dir, "no graph to join the flows over")
	}
	set, err := LoadIntentsAt(ctx, root, declared.Dir, "HEAD")
	if err != nil {
		return flowsDeclarationInvalid(plan, declared.Dir, declared.Dir+": "+err.Error())
	}
	if measured == "" {
		measured = set.Revision
	}
	links, err := EvaluateLinks(ctx, root, set, measured)
	if err != nil {
		return flowsDeclarationInvalid(plan, declared.Dir, declared.Dir+": "+err.Error())
	}
	j := &flowJoiner{ctx: ctx, root: root, declared: declared, graph: graph, plan: plan, set: set, links: links,
		changedBy: changedUnits(graph, plan.Dirty), selected: map[e2eFile]*affected.E2ESelection{}}
	plan.E2E = j.join()
	return plan
}

func flowsDeclarationInvalid(plan affected.Plan, dir, detail string) affected.Plan {
	plan.E2E = &affected.E2EPlan{Flows: dir, Scope: affected.E2EScopeUnknown, Inventory: affected.E2EInventory{Source: affected.E2EInventoryFlows, State: "n/a"},
		Selected: []affected.E2ESelection{},
		Unknown: []affected.Unknown{{Reason: affected.UnknownFlowsDeclarationInvalid, Detail: FlowsDeclarationPath,
			Hint: detail + "; no E2E test is narrowed until the declaration is repaired (APK-V0-004)"}}}
	return plan
}

// e2eFile is one inventory entry: a Playwright project and a test file.
type e2eFile struct{ project, path string }

type flowJoiner struct {
	ctx       context.Context
	root      string
	declared  *FlowsDeclaration
	graph     *affected.Graph
	plan      affected.Plan
	set       IntentSet
	links     []EvaluatedLink
	changedBy map[string]string
	inventory []e2eFile
	selected  map[e2eFile]*affected.E2ESelection
	unknown   []affected.Unknown
	// full is set when every inventory file must be selected.
	full bool
}

func (j *flowJoiner) join() *affected.E2EPlan {
	out := &affected.E2EPlan{Flows: j.declared.Dir, Selected: []affected.E2ESelection{}}
	out.Inventory = j.deriveInventory()
	j.checkBounds()
	j.checkGlobal()
	j.checkPlanUnbounded()
	// Flow hits (APK-V0-003): a link whose target the change reaches
	// selects the test files of its selecting variations.
	for _, intent := range j.set.Flows {
		for _, l := range flowLinks(j.links, intent.FlowID) {
			via := reach(j.graph, j.changedBy, j.plan.Dirty, l.Target.Path)
			if via == nil {
				continue
			}
			for _, variation := range selectingVariations(intent, l.From) {
				for _, test := range variationTestLinks(intent, variation) {
					for _, file := range j.filesOf(test.Target.Path) {
						sel := j.pick(file, affected.E2EReasonFlowReach)
						sel.TestKeys = appendUnique(sel.TestKeys, test.Target.TestKey)
						hit := affected.E2EFlowHit{Flow: intent.FlowID, Variation: variation, ChangedPath: via[0], Target: l.Target.Path}
						if !slices.Contains(sel.Flows, hit) {
							sel.Flows = append(sel.Flows, hit)
						}
					}
				}
			}
			if l.Target.Type == "test" {
				for _, file := range j.filesOf(l.Target.Path) {
					sel := j.pick(file, affected.E2EReasonFlowReach)
					sel.TestKeys = appendUnique(sel.TestKeys, l.Target.TestKey)
					hit := affected.E2EFlowHit{Flow: intent.FlowID, Variation: variationOf(intent, l.From), ChangedPath: via[0], Target: l.Target.Path}
					if !slices.Contains(sel.Flows, hit) {
						sel.Flows = append(sel.Flows, hit)
					}
				}
			}
		}
	}
	// Missing evidence (APK-V0-004): a flow with a stale link, or with any
	// link that is not reviewed, selects every file its links attach.
	for _, intent := range j.set.Flows {
		reason, unknown := "", ""
		for _, l := range flowLinks(j.links, intent.FlowID) {
			switch l.ReviewState {
			case ReviewReviewed:
			case ReviewStale, ReviewLinkNotAtAnchor:
				reason, unknown = affected.E2EReasonStaleLink, affected.UnknownE2EFlowStale
			default:
				if reason == "" {
					reason, unknown = affected.E2EReasonUnreviewedLink, affected.UnknownE2EFlowUnreviewed
				}
			}
		}
		if reason == "" {
			continue
		}
		hint := "a link of this flow is not reviewed, so no exclusion rests on it and every test it attaches is selected; review the flow's links to narrow (APK-V0-004)"
		if reason == affected.E2EReasonStaleLink {
			hint = "a link of this flow is stale or not at its review anchor, so every test it attaches is selected; re-review the flow's links to narrow (APK-V0-004)"
		}
		j.unknown = append(j.unknown, affected.Unknown{Reason: unknown, Detail: intent.FlowID, Hint: hint})
		for _, l := range intent.Links {
			if l.Target.Type == "test" {
				for _, file := range j.filesOf(l.Target.Path) {
					j.pick(file, reason)
				}
			}
		}
	}
	// The inventory itself: a changed or reached test file, an unlinked
	// file (AFU-V1-141), or every file under a FULL answer.
	linked := j.linkedPaths()
	for _, file := range j.inventory {
		if slices.Contains(j.plan.Dirty, file.path) {
			j.pick(file, affected.E2EReasonTestFileChanged)
		} else if reach(j.graph, j.changedBy, nil, file.path) != nil {
			j.pick(file, affected.E2EReasonStaticReach)
		}
		if !linked[file.path] {
			j.pick(file, affected.E2EReasonUnlinked)
			out.Inventory.Unlinked = appendUnique(out.Inventory.Unlinked, file.path)
		}
		if j.full {
			j.pick(file, j.fullReason())
		}
	}
	for _, sel := range j.selected {
		slices.Sort(sel.Reasons)
		slices.Sort(sel.TestKeys)
		slices.SortFunc(sel.Flows, compareHits)
		out.Selected = append(out.Selected, *sel)
	}
	slices.SortFunc(out.Selected, func(a, b affected.E2ESelection) int {
		return strings.Compare(a.Project+"\x00"+a.Path, b.Project+"\x00"+b.Path)
	})
	slices.Sort(out.Inventory.Unlinked)
	out.Unknown = j.unknown
	out.Scope = affected.E2EScopeBounded
	if len(j.unknown) != 0 {
		out.Scope = affected.E2EScopeUnknown
		if j.everySelected() {
			out.Scope = affected.E2EScopeFull
		}
	}
	return out
}

// deriveInventory is the E2E inventory: the bound discovery receipt's units
// (AFU-V1-140, AFU-V1-143) or, without a playwright member, every test link
// target of the flows. An underivable receipt falls back to the flows'
// files under a FULL answer (AFU-V1-142).
func (j *flowJoiner) deriveInventory() affected.E2EInventory {
	inventory := affected.E2EInventory{Source: affected.E2EInventoryFlows, State: "n/a"}
	if j.declared.PlaywrightConfig == "" {
		j.inventory = j.flowFiles()
		inventory.Count = len(j.inventory)
		return inventory
	}
	inventory.Source = affected.E2EInventoryDiscovery
	state := "config:untracked"
	var units []typescript.PlaywrightDiscoveryUnit
	if _, err := git(j.ctx, j.root, "cat-file", "-e", j.set.Revision+":"+j.declared.PlaywrightConfig); err == nil {
		var raw []byte
		if safePath(j.declared.PlaywrightDiscovery) {
			raw, _ = ReadFile(filepath.Join(j.root, filepath.FromSlash(j.declared.PlaywrightDiscovery)))
		}
		// The declared generated paths are pruned from the source walk, as
		// `affected discovery` prunes them (AFP-V0-444).
		if _, generated, err := affected.ReadGeneratedDeclaration(j.root); err == nil && len(generated) != 0 {
			if release, err := affected.HoldIgnoredExclusion(j.root, generated); err == nil {
				defer release()
			}
		}
		units, state = typescript.VerifyPlaywrightDiscovery(j.root, j.declared.PlaywrightConfig, j.set.Revision, raw)
		state = "discovery:" + state
	}
	inventory.State = strings.TrimPrefix(state, "discovery:")
	if state != "discovery:MATCHED" {
		j.inventory = j.flowFiles()
		inventory.Count = len(j.inventory)
		j.full = true
		j.unknown = append(j.unknown, affected.Unknown{Reason: affected.UnknownE2EInventoryUnderivable, Detail: state,
			Hint: fmt.Sprintf("the E2E inventory cannot be derived from %s and %s, so every E2E test must run; refresh the receipt with corvint affected discovery --playwright-config %s --playwright-list FILE at HEAD (AFU-V1-142)", j.declared.PlaywrightConfig, j.declared.PlaywrightDiscovery, j.declared.PlaywrightConfig)})
		return inventory
	}
	for _, u := range units {
		j.inventory = append(j.inventory, e2eFile{project: u.Project, path: u.Test})
	}
	slices.SortFunc(j.inventory, func(a, b e2eFile) int { return strings.Compare(a.project+"\x00"+a.path, b.project+"\x00"+b.path) })
	j.inventory = slices.Compact(j.inventory)
	inventory.Count = len(j.inventory)
	return inventory
}

// flowFiles lists, sorted, every test link target path of every flow.
func (j *flowJoiner) flowFiles() []e2eFile {
	var files []e2eFile
	for _, intent := range j.set.Flows {
		for _, l := range intent.Links {
			if l.Target.Type == "test" && l.Target.Path != "" {
				files = append(files, e2eFile{path: l.Target.Path})
			}
		}
	}
	slices.SortFunc(files, func(a, b e2eFile) int { return strings.Compare(a.path, b.path) })
	return slices.Compact(files)
}

// filesOf lists the inventory entries at a test link target path, or the
// path alone when the inventory does not hold it (a linked test the receipt
// does not list still runs).
func (j *flowJoiner) filesOf(p string) []e2eFile {
	if p == "" {
		return nil
	}
	var files []e2eFile
	for _, file := range j.inventory {
		if file.path == p {
			files = append(files, file)
		}
	}
	if len(files) == 0 {
		files = []e2eFile{{path: p}}
	}
	return files
}

func (j *flowJoiner) linkedPaths() map[string]bool {
	linked := map[string]bool{}
	for _, intent := range j.set.Flows {
		for _, l := range intent.Links {
			if l.Target.Type == "test" {
				linked[l.Target.Path] = true
			}
		}
	}
	return linked
}

func (j *flowJoiner) pick(file e2eFile, reason string) *affected.E2ESelection {
	sel, ok := j.selected[file]
	if !ok {
		sel = &affected.E2ESelection{Path: file.path, Project: file.project, TestKind: affected.TestKindE2E, Reasons: []string{}}
		j.selected[file] = sel
	}
	sel.Reasons = appendUnique(sel.Reasons, reason)
	return sel
}

func (j *flowJoiner) everySelected() bool {
	for _, file := range j.inventory {
		if _, ok := j.selected[file]; !ok {
			return false
		}
	}
	return true
}

func (j *flowJoiner) fullReason() string {
	for _, u := range j.unknown {
		switch u.Reason {
		case affected.UnknownE2EBoundExceeded:
			return affected.E2EReasonBoundExceeded
		case affected.UnknownE2EGlobalPath:
			return affected.E2EReasonGlobalPathChanged
		case affected.UnknownE2EPlanUnbounded:
			return affected.E2EReasonPlanUnbounded
		case affected.UnknownE2EInventoryUnderivable:
			return affected.E2EReasonInventoryUnderivable
		}
	}
	return affected.E2EReasonBoundExceeded
}

func (j *flowJoiner) checkBounds() {
	if len(j.plan.Dirty) > maxFlowPlanDirty || len(j.inventory) > maxFlowPlanInventory {
		j.full = true
		j.unknown = append(j.unknown, affected.Unknown{Reason: affected.UnknownE2EBoundExceeded,
			Detail: fmt.Sprintf("changed-paths=%d inventory=%d", len(j.plan.Dirty), len(j.inventory)),
			Hint:   fmt.Sprintf("the join is bounded at %d changed paths and %d inventory files; every E2E test is selected (APK-V0-004)", maxFlowPlanDirty, maxFlowPlanInventory)})
	}
}

// checkGlobal selects every file when a changed path is global: a lockfile,
// manifest or build file, the declaration, the Playwright config or a flow
// intent (APK-V0 Definitions).
func (j *flowJoiner) checkGlobal() {
	for _, p := range j.plan.Dirty {
		if builtinGlobalNames[path.Base(p)] || p == FlowsDeclarationPath || p == j.declared.PlaywrightConfig && p != "" || under(p, j.declared.Dir) {
			j.full = true
			j.unknown = append(j.unknown, affected.Unknown{Reason: affected.UnknownE2EGlobalPath, Detail: p,
				Hint: "a global path changed, which no flow link can bound; every E2E test is selected (APK-V0-004)"})
			return
		}
	}
}

// checkPlanUnbounded selects every file when a plan unknown leaves the
// obligation closure unbounded, as the e2e-safe profile reads it.
func (j *flowJoiner) checkPlanUnbounded() {
	for _, u := range j.plan.Unknown {
		if unbounded(u) {
			j.full = true
			j.unknown = append(j.unknown, affected.Unknown{Reason: affected.UnknownE2EPlanUnbounded, Detail: u.Reason + ": " + u.Detail,
				Hint: "the plan cannot bound what the change reaches, so no flow link can exclude a test; every E2E test is selected (APK-V0-004)"})
			return
		}
	}
}

// selectingVariations are the variations a hit from `from` selects: the
// variation itself, or every variation listing the step or outcome.
func selectingVariations(intent FlowIntent, from string) []string {
	var out []string
	for _, v := range intent.Variations {
		if from == v.VariationID || slices.Contains(v.Steps, from) || slices.Contains(v.Outcomes, from) {
			out = append(out, v.VariationID)
		}
	}
	return out
}

// variationOf names from when it is a variation of intent, else "".
func variationOf(intent FlowIntent, from string) string {
	for _, v := range intent.Variations {
		if v.VariationID == from {
			return from
		}
	}
	return ""
}

// variationTestLinks lists the test links a variation attaches.
func variationTestLinks(intent FlowIntent, variation string) []FlowLink {
	var out []FlowLink
	for _, l := range intent.Links {
		if l.From == variation && l.Target.Type == "test" {
			out = append(out, l)
		}
	}
	return out
}

func compareHits(a, b affected.E2EFlowHit) int {
	return strings.Compare(a.Flow+"\x00"+a.Variation+"\x00"+a.ChangedPath+"\x00"+a.Target, b.Flow+"\x00"+b.Variation+"\x00"+b.ChangedPath+"\x00"+b.Target)
}

func appendUnique(list []string, value string) []string {
	if value == "" || slices.Contains(list, value) {
		return list
	}
	return append(list, value)
}

// ReconcileProvider records, when an e2e-safe provider was given beside the
// derived inventory, that the provider's test_selection is the selection to
// run and which derived files the provider's inventory does not name
// (AFU-V1-145). An unreadable provider leaves the member unchanged: the
// e2e-safe selection has already refused it.
func ReconcileProvider(root, provider string, e2e *affected.E2EPlan) {
	if e2e == nil || provider == "" {
		return
	}
	declared, _, err := readSelectionProvider(root, provider)
	if err != nil {
		return
	}
	named := map[string]bool{}
	for _, t := range declared.Inventory {
		named[t.Path] = true
	}
	e2e.Inventory.Source = affected.E2EInventoryProvider
	omits := []string{}
	for _, sel := range e2e.Selected {
		if !named[sel.Path] {
			omits = appendUnique(omits, sel.Path)
		}
	}
	slices.Sort(omits)
	e2e.Inventory.ProviderOmits = omits
}
