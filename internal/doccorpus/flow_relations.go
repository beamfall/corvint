package doccorpus

import (
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// FlowRelationSchema is the closed provider record for project-declared flow-level
// relations that no source index can derive (DCP-V1-093..099).
const FlowRelationSchema = "corvint-corpus-flow-relations/1"

const (
	// MaxFlowRelations bounds flow-relation records per corpus (DCP-V1-099).
	MaxFlowRelations = 4096
	// DefaultFlowDepth and MaxFlowDepth bound downstream traversal in flow hops.
	DefaultFlowDepth = 3
	MaxFlowDepth     = 16
)

const flowRelationLimitation = "declared flow relations are project-authored corpus data ranked below project authority; they explain reach and never narrow required checks"

type FlowRelationSet struct {
	Revision string               `json:"revision"`
	Records  []FlowRelationRecord `json:"records"`
}

// FlowRelationRecord is one typed declaration. Flow is the declaring flow; Target is
// the flow it depends on (depends-on), Name the event or job (produces/consumes) and
// System the external system (external).
type FlowRelationRecord struct {
	Source *Anchor `json:"source,omitempty"`
	ID     string  `json:"id"`
	Kind   string  `json:"kind"`
	Flow   string  `json:"flow"`
	Target string  `json:"target,omitempty"`
	Name   string  `json:"name,omitempty"`
	System string  `json:"system,omitempty"`
	Review string  `json:"review"`
}

// FlowRelationJoin is the build-time join of one record, kept as a separate artifact
// member. Only a joined reviewed record is traversable.
type FlowRelationJoin struct {
	ID          string   `json:"id"`
	Provider    string   `json:"provider"`
	Status      string   `json:"status"`
	Traversable bool     `json:"traversable"`
	Reasons     []string `json:"reasons"`
	// NameSubjects are the job or event subjects a produces or consumes record's name matches,
	// set only when the corpus declares job or event subjects (DCX-V1-397).
	NameSubjects []string           `json:"name_subjects,omitempty"`
	Record       FlowRelationRecord `json:"record"`
}

// FlowReach attributes one reached flow or external system to its relation path.
type FlowReach struct {
	Flow      string   `json:"flow,omitempty"`
	System    string   `json:"system,omitempty"`
	Depth     int      `json:"depth"`
	Path      []string `json:"path"`
	Relations []string `json:"relations"`
}

// FlowWithheld is a relation touching a reached flow that was reported, not traversed.
type FlowWithheld struct {
	ID     string `json:"id"`
	At     string `json:"at"`
	Kind   string `json:"kind"`
	Review string `json:"review"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// FlowTrace is the bounded downstream walk shared by impact, affected and trace.
type FlowTrace struct {
	Schema      string         `json:"schema"`
	Authority   string         `json:"authority"`
	Depth       int            `json:"depth"`
	Seeds       []string       `json:"seeds"`
	Reached     []FlowReach    `json:"reached"`
	Withheld    []FlowWithheld `json:"withheld"`
	Truncated   bool           `json:"depth_truncated"`
	Limitations []string       `json:"limitations"`
}

type providerFlowRelations struct {
	provider string
	set      *FlowRelationSet
}

var (
	flowRelationKinds   = words("depends-on produces consumes external")
	flowRelationReviews = words("reviewed unreviewed")
)

// admitFlowRelations applies the schema-only admission rules (DCP-V1-093, DCP-V1-094).
func (c *compiler) admitFlowRelations(p Provider, record ProviderRecord) error {
	if c.manifest.Schema != ManifestSchemaV2 {
		return fail("flow-relation provider requires manifest /2")
	}
	set := record.FlowRelations
	if set == nil || record.TestKnowledge != nil || record.BehaviorContracts != nil || record.Details != nil || record.RestrictedFindings != nil || len(record.Subjects)+len(record.Claims)+len(record.Relations)+len(record.Journeys)+len(record.Observations)+len(record.Capabilities) != 0 {
		return fail("flow-relation provider carries only its flow_relations member")
	}
	if !wire.IsGitOid(set.Revision) || len(set.Records) == 0 || len(set.Records) > MaxFlowRelations {
		return fail("invalid flow-relation revision or record bound")
	}
	seen := map[string]bool{}
	for _, r := range set.Records {
		if !strings.HasPrefix(r.ID, p.ID+":") || !safeText(r.ID, 1024) || seen[r.ID] {
			return fail("flow-relation id must be unique and provider-qualified")
		}
		seen[r.ID] = true
		if !flowRelationKinds[r.Kind] || !flowRelationReviews[r.Review] {
			return fail("unknown flow-relation kind or review state")
		}
		if !safeText(r.Flow, 1024) {
			return fail("invalid or secret-shaped flow-relation flow")
		}
		target, name, system := r.Target != "", r.Name != "", r.System != ""
		valid := false
		switch r.Kind {
		case "depends-on":
			valid = target && !name && !system && safeText(r.Target, 1024) && r.Target != r.Flow
		case "produces", "consumes":
			valid = name && !target && !system && safeText(r.Name, 1024)
		case "external":
			valid = system && !target && !name && safeText(r.System, 1024)
		}
		if !valid {
			return fail("flow-relation kind requires exactly its own target, name or system")
		}
		if r.Source != nil {
			if r.Source.Revision != set.Revision {
				return fail("flow-relation source anchor is not pinned to the set revision")
			}
			if err := c.checkTestKnowledgeAnchor(*r.Source); err != nil {
				return fail("invalid flow-relation source anchor")
			}
		}
	}
	return nil
}

// joinFlowRelations runs after every provider is imported (DCP-V1-095).
func (c *compiler) joinFlowRelations(sets []providerFlowRelations) error {
	revision := c.manifest.Repository.Revision
	flows := map[string]bool{}
	// names holds declared job and event subjects by name; when the corpus declares any, a
	// produces or consumes record must name one of them (DCX-V1-397).
	names := map[string][]string{}
	for _, s := range c.artifact.Subjects {
		if s.Kind == "flow" || s.Kind == "variation" {
			flows[s.ID] = true
		}
		if s.Kind == "job" || s.Kind == "event" {
			names[s.Name] = append(names[s.Name], s.ID)
		}
	}
	seen := map[string]bool{}
	total := 0
	for _, pf := range sets {
		total += len(pf.set.Records)
		if total > MaxFlowRelations {
			return fail("aggregate flow-relation bound exceeded")
		}
		for _, r := range pf.set.Records {
			if seen[r.ID] {
				return fail("duplicate flow-relation id")
			}
			seen[r.ID] = true
			j := FlowRelationJoin{ID: r.ID, Provider: pf.provider, Status: "joined", Reasons: []string{}, Record: r}
			switch {
			case pf.set.Revision != revision:
				j.Status = "stale"
				j.Reasons = append(j.Reasons, "stale: set revision is not the corpus revision")
			case !flows[r.Flow] || r.Kind == "depends-on" && !flows[r.Target]:
				j.Status = "unresolved"
				j.Reasons = append(j.Reasons, "unresolved: flow or target is not a flow or variation subject")
			case r.Source != nil:
				status, err := c.resolveTestKnowledgeAnchor(*r.Source, revision)
				if err != nil {
					return fail("flow-relation source anchor binding mismatch")
				}
				if status != "joined" {
					j.Status = status
					j.Reasons = append(j.Reasons, status+": "+r.Source.Path)
				}
			}
			if j.Status == "joined" && len(names) > 0 && (r.Kind == "produces" || r.Kind == "consumes") {
				if matched := names[r.Name]; len(matched) > 0 {
					j.NameSubjects = slices.Sorted(slices.Values(matched))
				} else {
					j.Status = "unresolved"
					j.Reasons = append(j.Reasons, "unresolved: no declared job or event subject is named "+r.Name)
				}
			}
			if r.Review != "reviewed" {
				j.Reasons = append(j.Reasons, "unreviewed: declared relation awaits review")
			}
			j.Traversable = j.Status == "joined" && r.Review == "reviewed"
			c.artifact.FlowRelations = append(c.artifact.FlowRelations, j)
		}
	}
	sort.Slice(c.artifact.FlowRelations, func(i, k int) bool { return c.artifact.FlowRelations[i].ID < c.artifact.FlowRelations[k].ID })
	return nil
}

// FlowSeeds returns the flow subjects a changed path reaches: a flow whose subject
// evidence anchors the path, or the declaring flow of a joined relation whose source
// anchor is the path (DCP-V1-096).
func FlowSeeds(a *Artifact, paths []string) []string {
	out := map[string]bool{}
	for _, p := range paths {
		for _, s := range a.Subjects {
			if (s.Kind == "flow" || s.Kind == "variation") && subjectPath(s, p) {
				out[s.ID] = true
			}
		}
		for _, j := range a.FlowRelations {
			if j.Status == "joined" && j.Record.Source != nil && j.Record.Source.Path == p {
				out[j.Record.Flow] = true
			}
		}
	}
	seeds := []string{}
	for id := range out {
		seeds = append(seeds, id)
	}
	sort.Strings(seeds)
	return seeds
}

type flowEdge struct {
	to, via, system string
	relations       []string
}

// TraceFlows walks reviewed joined relations downstream from seeds for at most depth
// flow hops (DCP-V1-096, DCP-V1-097). B depends-on A gives A→B; A produces J and B
// consumes J give A→J→B; A external S reaches S as a terminal. Each reached node keeps
// its first breadth-first path in sorted order. A relation that would extend the walk
// but is unreviewed or not joined is withheld and reported, never traversed.
func TraceFlows(a *Artifact, seeds []string, depth int) FlowTrace {
	if depth < 1 || depth > MaxFlowDepth {
		depth = DefaultFlowDepth
	}
	t := FlowTrace{Schema: "corvint-corpus-flow-trace/1", Authority: "project-declared-corpus", Depth: depth, Seeds: append([]string{}, seeds...), Reached: []FlowReach{}, Withheld: []FlowWithheld{}, Limitations: []string{flowRelationLimitation}}
	producers, consumers := map[string][]FlowRelationJoin{}, map[string][]FlowRelationJoin{}
	dependents, externals := map[string][]FlowRelationJoin{}, map[string][]FlowRelationJoin{}
	for _, j := range a.FlowRelations {
		r := j.Record
		switch r.Kind {
		case "depends-on":
			dependents[r.Target] = append(dependents[r.Target], j)
		case "produces":
			producers[r.Flow] = append(producers[r.Flow], j)
		case "consumes":
			consumers[r.Name] = append(consumers[r.Name], j)
		case "external":
			externals[r.Flow] = append(externals[r.Flow], j)
		}
	}
	withheld := map[string]bool{}
	withhold := func(at string, j FlowRelationJoin) {
		key := j.ID + "\x00" + at
		if withheld[key] {
			return
		}
		withheld[key] = true
		reason := "relation is " + j.Status
		if j.Status == "joined" {
			reason = "relation is unreviewed"
		}
		t.Withheld = append(t.Withheld, FlowWithheld{ID: j.ID, At: at, Kind: j.Record.Kind, Review: j.Record.Review, Status: j.Status, Reason: reason})
	}
	edges := func(from string) []flowEdge {
		out := []flowEdge{}
		for _, j := range dependents[from] {
			if !j.Traversable {
				withhold(from, j)
				continue
			}
			out = append(out, flowEdge{to: j.Record.Flow, relations: []string{j.ID}})
		}
		for _, p := range producers[from] {
			if !p.Traversable {
				withhold(from, p)
				continue
			}
			for _, cj := range consumers[p.Record.Name] {
				if !cj.Traversable {
					withhold(from, cj)
					continue
				}
				out = append(out, flowEdge{to: cj.Record.Flow, via: p.Record.Name, relations: []string{p.ID, cj.ID}})
			}
		}
		for _, j := range externals[from] {
			if !j.Traversable {
				withhold(from, j)
				continue
			}
			out = append(out, flowEdge{system: j.Record.System, relations: []string{j.ID}})
		}
		sort.Slice(out, func(i, k int) bool { return edgeKey(out[i]) < edgeKey(out[k]) })
		return out
	}
	type node struct {
		id        string
		path      []string
		relations []string
	}
	visited, systems := map[string]bool{}, map[string]bool{}
	frontier := []node{}
	for _, s := range t.Seeds {
		if !visited[s] {
			visited[s] = true
			frontier = append(frontier, node{s, []string{s}, []string{}})
		}
	}
	for level := 1; len(frontier) > 0; level++ {
		next := []node{}
		for _, n := range frontier {
			for _, e := range edges(n.id) {
				if level > depth {
					t.Truncated = true
					break
				}
				relations := append(append([]string{}, n.relations...), e.relations...)
				if e.system != "" {
					if !systems[e.system] {
						systems[e.system] = true
						t.Reached = append(t.Reached, FlowReach{System: e.system, Depth: level, Path: append(append([]string{}, n.path...), e.system), Relations: relations})
					}
					continue
				}
				if visited[e.to] {
					continue
				}
				visited[e.to] = true
				path := append([]string{}, n.path...)
				if e.via != "" {
					path = append(path, e.via)
				}
				path = append(path, e.to)
				t.Reached = append(t.Reached, FlowReach{Flow: e.to, Depth: level, Path: path, Relations: relations})
				next = append(next, node{e.to, path, relations})
			}
		}
		if level > depth {
			break
		}
		frontier = next
	}
	if t.Truncated {
		t.Limitations = append(t.Limitations, "flow relation traversal stopped at depth "+strconv.Itoa(depth)+"; deeper reach is unknown")
	}
	if len(t.Withheld) > 0 {
		t.Limitations = append(t.Limitations, "unreviewed or unjoined flow relations were reported, not traversed; downstream reach through them is unknown")
	}
	if len(t.Reached) > MaxResults {
		t.Limitations = append(t.Limitations, "flow reach page withheld "+strconv.Itoa(len(t.Reached)-MaxResults)+" reached nodes")
		t.Reached = t.Reached[:MaxResults]
	}
	if len(t.Withheld) > MaxResults {
		t.Limitations = append(t.Limitations, "withheld relation page withheld "+strconv.Itoa(len(t.Withheld)-MaxResults)+" records")
		t.Withheld = t.Withheld[:MaxResults]
	}
	// Long declared names stay inside the same 512 KiB page bound as other impact members.
	for {
		raw, err := Encode(t)
		if err == nil && len(raw) <= 512<<10 || len(t.Reached)+len(t.Withheld) == 0 {
			break
		}
		t.Reached, t.Withheld = t.Reached[:len(t.Reached)/2], t.Withheld[:len(t.Withheld)/2]
		t.Limitations = append(t.Limitations, "flow trace page halved to its byte bound")
	}
	return t
}
func edgeKey(e flowEdge) string {
	return e.to + "\x00" + e.system + "\x00" + e.via + "\x00" + e.relations[len(e.relations)-1]
}

// flowRelationGaps reports records that cannot be traversed (DCP-V1-098).
func flowRelationGaps(a *Artifact) []Gap {
	out := []Gap{}
	for _, j := range a.FlowRelations {
		if j.Status != "joined" {
			out = append(out, Gap{j.ID, j.Status + "-flow-relation", strings.Join(j.Reasons, "; ")})
		} else if !j.Traversable {
			out = append(out, Gap{j.ID, "unreviewed-flow-relation", strings.Join(j.Reasons, "; ")})
		}
	}
	return out
}
func flowRelationAnchors(j FlowRelationJoin) []Anchor {
	if j.Record.Source == nil {
		return nil
	}
	return []Anchor{*j.Record.Source}
}
func isFlowSubject(a *Artifact, id string) bool {
	for _, s := range a.Subjects {
		if s.ID == id && (s.Kind == "flow" || s.Kind == "variation") {
			return true
		}
	}
	return false
}
