// SPDX-License-Identifier: Apache-2.0

package appflows

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// The flow binding of declared binary consumers (AFU-V1-1651 to AFU-V1-1654):
// `.corvint/e2e-flows.json` names the intent directory and, for each consumer
// file of `.corvint/test-binary-consumers.json` it binds, the flows that
// consumer exercises. `corvint affected` then keeps a bound consumer when the
// change reaches a link target of one of its flows, leaves it out only when a
// reviewed link of some flow explains every changed path that reaches the
// commands it runs, and otherwise names the unexplained path as
// E2E_UNMAPPED_CHANGE. A consumer the declaration does not bind keeps the
// AFP-V0-762 and AZU-V0-007 behaviour unchanged.
const (
	E2EFlowsPath    = ".corvint/e2e-flows.json"
	E2EFlowsProfile = "corvint-e2e-flows/0"
	// E2EBasisReviewedLinks is the one omission basis (AFU-V1-1653).
	E2EBasisReviewedLinks = "reviewed-links"

	maxE2EFlowsBytes     = 1 << 20
	maxE2EFlowsConsumers = 4096
	maxE2EConsumerFlows  = 256
	// maxE2EFlowsDirty bounds the reachability walks: above it the plan is
	// left as it was, which keeps every consumer (AFU-V1-1654).
	maxE2EFlowsDirty = 4096
)

type e2eFlowsFile struct {
	Profile   string                      `json:"profile"`
	Flows     string                      `json:"flows"`
	Consumers map[string]e2eFlowsConsumer `json:"consumers"`
}

type e2eFlowsConsumer struct {
	Flows []string `json:"flows"`
}

// E2EFlowsDeclaration is the validated binding: the intent directory and the
// bound flows by consumer file, each list strictly ascending.
type E2EFlowsDeclaration struct {
	Dir       string
	Consumers map[string][]string
}

// ReadE2EFlows reads the declaration under root: nil with none, an error for
// one that is unreadable, not a regular file, oversized or invalid in any
// member (AFU-V1-1651).
func ReadE2EFlows(root string) (*E2EFlowsDeclaration, error) {
	name := filepath.Join(root, filepath.FromSlash(E2EFlowsPath))
	info, err := os.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", E2EFlowsPath, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: not a regular file", E2EFlowsPath)
	}
	if info.Size() > maxE2EFlowsBytes {
		return nil, fmt.Errorf("%s: larger than %d bytes", E2EFlowsPath, maxE2EFlowsBytes)
	}
	body, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", E2EFlowsPath, err)
	}
	var declared e2eFlowsFile
	if err := json.UnmarshalRead(bytes.NewReader(body), &declared, json.RejectUnknownMembers(true)); err != nil {
		return nil, fmt.Errorf("%s: %w", E2EFlowsPath, err)
	}
	if declared.Profile != E2EFlowsProfile {
		return nil, fmt.Errorf("%s: profile must be %s", E2EFlowsPath, E2EFlowsProfile)
	}
	if !safePath(declared.Flows) || declared.Flows == "." {
		return nil, fmt.Errorf("%s: flows must name a directory inside the repository root", E2EFlowsPath)
	}
	if len(declared.Consumers) == 0 || len(declared.Consumers) > maxE2EFlowsConsumers {
		return nil, fmt.Errorf("%s: 1 to %d consumers", E2EFlowsPath, maxE2EFlowsConsumers)
	}
	out := &E2EFlowsDeclaration{Dir: declared.Flows, Consumers: make(map[string][]string, len(declared.Consumers))}
	for file, consumer := range declared.Consumers {
		if !affected.ValidRelativePath(file) {
			return nil, fmt.Errorf("%s: consumer %q", E2EFlowsPath, file)
		}
		if len(consumer.Flows) == 0 || len(consumer.Flows) > maxE2EConsumerFlows {
			return nil, fmt.Errorf("%s: consumer %q must name 1 to %d flows", E2EFlowsPath, file, maxE2EConsumerFlows)
		}
		previous := ""
		for _, flow := range consumer.Flows {
			if !validFlowID(flow) || flow <= previous {
				return nil, fmt.Errorf("%s: consumer %q flow %q", E2EFlowsPath, file, flow)
			}
			previous = flow
		}
		out.Consumers[file] = slices.Clone(consumer.Flows)
	}
	return out, nil
}

// BindE2EFlows applies the flow binding to a plan (AFU-V1-1651 to
// AFU-V1-1654). It reads the declaration under root, the intents it names at
// HEAD and their evaluated links, and for each bound consumer the plan
// reached (a DECLARED_CONSUMER selection or an UNSELECTED_BINARY_CONSUMER
// unknown) keeps, annotates, omits or widens as the package comment states.
// A plan without the declaration is returned unchanged; one whose declaration
// is invalid gains an E2E_FLOWS_INVALID unknown and is otherwise unchanged.
func BindE2EFlows(ctx context.Context, root string, graph *affected.Graph, plan affected.Plan) affected.Plan {
	declared, err := ReadE2EFlows(root)
	if declared == nil && err == nil {
		return plan
	}
	if err != nil {
		return e2eFlowsInvalid(plan, err.Error())
	}
	if len(plan.Dirty) > maxE2EFlowsDirty || graph == nil {
		return plan
	}
	commands := consumerCommands(graph)
	for file := range declared.Consumers {
		if len(commands[file]) == 0 {
			return e2eFlowsInvalid(plan, fmt.Sprintf("consumer %q is not declared in %s", file, affected.BinaryConsumersPath))
		}
	}
	set, err := LoadIntentsAt(ctx, root, declared.Dir, "HEAD")
	if err != nil {
		return e2eFlowsInvalid(plan, declared.Dir+": "+err.Error())
	}
	known := map[string]bool{}
	for _, intent := range set.Flows {
		known[intent.FlowID] = true
	}
	for _, file := range slices.Sorted(maps.Keys(declared.Consumers)) {
		for _, flow := range declared.Consumers[file] {
			if !known[flow] {
				return e2eFlowsInvalid(plan, fmt.Sprintf("consumer %q binds flow %q, which %s does not hold", file, flow, declared.Dir))
			}
		}
	}
	links, err := EvaluateLinks(ctx, root, set, set.Revision)
	if err != nil {
		return e2eFlowsInvalid(plan, declared.Dir+": "+err.Error())
	}
	binder := &e2eBinder{graph: graph, plan: plan, links: links, commands: commands, changedBy: changedUnits(graph, plan.Dirty)}
	binder.global = slices.ContainsFunc(plan.Dirty, func(p string) bool { return builtinGlobalNames[path.Base(p)] })
	for _, file := range slices.Sorted(maps.Keys(declared.Consumers)) {
		binder.bind(file, declared.Consumers[file])
	}
	plan = binder.plan
	if len(plan.Unknown) != 0 {
		plan.Scope = affected.ScopeUnknown
	} else if plan.Scope == affected.ScopeUnknown {
		plan.Scope = affected.ScopeBounded
	}
	return plan
}

func e2eFlowsInvalid(plan affected.Plan, detail string) affected.Plan {
	plan.Unknown = append(plan.Unknown, affected.Unknown{Reason: affected.UnknownE2EFlowsInvalid, Detail: E2EFlowsPath,
		Hint: detail + "; no declared consumer is narrowed by flows until the declaration is repaired"})
	plan.Scope = affected.ScopeUnknown
	return plan
}

// consumerCommands maps each declared consumer file to the sorted command
// units whose built binary it runs (Unit.ExecConsumers, AFP-V0-762).
func consumerCommands(graph *affected.Graph) map[string][]string {
	commands := map[string][]string{}
	for _, id := range graph.UnitIDs() {
		unit, _ := graph.Unit(id)
		for _, file := range unit.ExecConsumers {
			commands[file] = append(commands[file], id)
		}
	}
	for file := range commands {
		slices.Sort(commands[file])
	}
	return commands
}

type e2eBinder struct {
	graph     *affected.Graph
	plan      affected.Plan
	links     []EvaluatedLink
	commands  map[string][]string
	changedBy map[string]string
	global    bool
}

// bind applies the binding to one consumer the declaration binds to flows.
func (b *e2eBinder) bind(file string, flows []string) {
	selected := slices.IndexFunc(b.plan.Selected, func(s affected.Selection) bool {
		return s.Kind == "DECLARED_CONSUMER" && s.UnitID == "declared-consumer:"+file
	})
	unknown := slices.IndexFunc(b.plan.Unknown, func(u affected.Unknown) bool {
		return u.Reason == affected.UnknownUnselectedBinaryConsumer && u.Detail == file
	})
	if selected < 0 && unknown < 0 {
		return // the change rebuilt none of the commands it runs
	}
	hits := b.reachedFlows(flows)
	if b.global || len(hits) != 0 {
		// Global paths run every bound consumer; a reached flow keeps it.
		if b.global && len(hits) == 0 {
			hits = slices.Clone(flows)
		}
		if selected >= 0 {
			b.plan.Selected[selected].Flows = hits
		} else {
			b.plan.Unknown[unknown].Hint = strings.TrimPrefix(b.plan.Unknown[unknown].Hint+"; bound flows reached: "+strings.Join(hits, ", "), "; ")
		}
		return
	}
	reaching := b.reachingPaths(file)
	if len(reaching) == 0 {
		// The command was rebuilt by an edge the import walk does not state
		// (an embed, an enclosed path, an exec edge): fail closed.
		first := ""
		if selected >= 0 {
			first = b.plan.Selected[selected].Witness.DirtyPath
		}
		if first == "" && len(b.plan.Dirty) != 0 {
			first = b.plan.Dirty[0]
		}
		reaching = []string{first}
	}
	var mapped []affected.E2EMapping
	var unmapped []string
	for _, p := range reaching {
		if flow, target, ok := b.explained(p); ok {
			mapped = append(mapped, affected.E2EMapping{Path: p, Flow: flow, Target: target})
		} else {
			unmapped = append(unmapped, p)
		}
	}
	if len(unmapped) != 0 {
		for _, p := range unmapped {
			b.plan.Unknown = append(b.plan.Unknown, affected.Unknown{Reason: affected.UnknownE2EUnmappedChange, Detail: p,
				Hint: fmt.Sprintf("reaches %s, which %s runs; no reviewed flow link explains it, so that consumer runs; link the path from a flow in %s with a reviewed anchor to narrow (AFU-V1-1652)", strings.Join(b.commands[file], ", "), file, E2EFlowsPath)})
		}
		return
	}
	omission := affected.E2EOmission{File: file, Flows: slices.Clone(flows), Commands: slices.Clone(b.commands[file]), Basis: E2EBasisReviewedLinks, Mapped: mapped}
	if selected >= 0 {
		b.plan.Selected = slices.Delete(slices.Clone(b.plan.Selected), selected, selected+1)
	} else {
		b.plan.Unknown = slices.Delete(slices.Clone(b.plan.Unknown), unknown, unknown+1)
	}
	b.plan.E2EOmitted = append(b.plan.E2EOmitted, omission)
}

// reachedFlows lists, sorted, the bound flows with a link (any basis) whose
// target the change reaches.
func (b *e2eBinder) reachedFlows(flows []string) []string {
	var hits []string
	for _, l := range b.links {
		if !slices.Contains(flows, l.Flow) || l.Target.Path == "" || slices.Contains(hits, l.Flow) {
			continue
		}
		if reach(b.graph, b.changedBy, b.plan.Dirty, l.Target.Path) != nil {
			hits = append(hits, l.Flow)
		}
	}
	slices.Sort(hits)
	return hits
}

// reachingPaths lists the changed paths that reach a command the consumer
// runs, in plan order: a path the command unit owns, or one whose owning
// unit the command's import walk reaches.
func (b *e2eBinder) reachingPaths(file string) []string {
	var reaching []string
	for _, p := range b.plan.Dirty {
		owner, ok := b.graph.OwnerOf(p)
		if !ok {
			continue
		}
		for _, command := range b.commands[file] {
			unit, _ := b.graph.Unit(command)
			if owner == command || len(unit.Sources) != 0 && reach(b.graph, map[string]string{owner: p}, []string{p}, unit.Sources[0]) != nil {
				reaching = append(reaching, p)
				break
			}
		}
	}
	return reaching
}

// explained returns the flow and target of the first reviewed link, in link
// order, whose target the changed path reaches.
func (b *e2eBinder) explained(p string) (string, string, bool) {
	owner, ok := b.graph.OwnerOf(p)
	if !ok {
		return "", "", false
	}
	for _, l := range b.links {
		if l.ReviewState != ReviewReviewed || l.Target.Path == "" {
			continue
		}
		if reach(b.graph, map[string]string{owner: p}, []string{p}, l.Target.Path) != nil {
			return l.Flow, l.Target.Path, true
		}
	}
	return "", "", false
}
