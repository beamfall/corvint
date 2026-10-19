package doccorpus

import (
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

// TestKnowledgeSchema is the closed provider record for reviewed knowledge about
// test examples from any framework (DCP-V1-086..092).
const TestKnowledgeSchema = "corvint-corpus-test-knowledge/1"

// MaxTestKnowledge bounds test-knowledge records per corpus (DCP-V1-092).
const MaxTestKnowledge = 4096

const testKnowledgeLimitation = "provider-reported trust level; test knowledge never marks a claim or flow verified"

type TestKnowledgeSet struct {
	Revision string                `json:"revision"`
	Records  []TestKnowledgeRecord `json:"records"`
}
type TestKnowledgeStub struct {
	Name     string `json:"name"`
	Boundary string `json:"boundary"`
}
type TestKnowledgeDocs struct {
	State string `json:"state"`
	Claim string `json:"claim,omitempty"`
}
type TestKnowledgeRecord struct {
	Receipt   *Anchor             `json:"receipt,omitempty"`
	ID        string              `json:"id"`
	Kind      string              `json:"kind"`
	Framework string              `json:"framework"`
	Title     string              `json:"title"`
	Test      Anchor              `json:"test"`
	Sources   []Anchor            `json:"sources"`
	Entry     string              `json:"entry"`
	Actor     string              `json:"actor"`
	Setup     []string            `json:"setup"`
	Stubs     []TestKnowledgeStub `json:"stubs"`
	Behavior  string              `json:"behavior"`
	Asserted  []string            `json:"asserted"`
	Flows     []string            `json:"flows"`
	Verdict   string              `json:"verdict"`
	Gap       string              `json:"gap,omitempty"`
	Docs      TestKnowledgeDocs   `json:"docs"`
	Trust     string              `json:"trust"`
	Reviews   []string            `json:"reviews"`
}

// TestKnowledgeJoin is the build-time join of one record. It is a separate
// artifact member: claim, subject and journey records are never rewritten.
type TestKnowledgeJoin struct {
	ID                   string              `json:"id"`
	Provider             string              `json:"provider"`
	TrustLevel           string              `json:"trust_level"`
	Verdict              string              `json:"verdict"`
	Status               string              `json:"status"`
	CountsTowardCoverage bool                `json:"counts_toward_coverage"`
	Selection            string              `json:"selection"`
	Claims               []string            `json:"claims"`
	Flows                []string            `json:"flows"`
	UnresolvedFlows      []string            `json:"unresolved_flows"`
	Paths                []string            `json:"paths"`
	Reasons              []string            `json:"reasons"`
	Limitations          []string            `json:"limitations"`
	Record               TestKnowledgeRecord `json:"record"`
}

type providerKnowledge struct {
	provider string
	set      *TestKnowledgeSet
}

var (
	testKnowledgeKinds    = words("unit request model component integration end-to-end legacy-suite test-case-export")
	testKnowledgeVerdicts = words("bound partial unrelated defect")
	testKnowledgeTrust    = words("run+reviewed run-unreviewed read-only legacy-reference")
	testKnowledgeDocs     = words("consistent contradicts undocumented")
	testKnowledgeStubs    = words("external in-app")
)

// safeText applies the bounded-text rule and the shared secret-shaped text scan.
func safeText(value string, limit int) bool {
	return textOK(value) && len(value) <= limit && !secretscreen.MatchString(value)
}
func safeList(values []string, limit int, allowEmpty bool) bool {
	if len(values) > limit || !allowEmpty && len(values) == 0 {
		return false
	}
	for _, v := range values {
		if !safeText(v, 4096) {
			return false
		}
	}
	return true
}

// admitTestKnowledge applies the schema-only admission rules (DCP-V1-086..088).
func (c *compiler) admitTestKnowledge(p Provider, record ProviderRecord) error {
	if c.manifest.Schema != ManifestSchemaV2 {
		return fail("test-knowledge provider requires manifest /2")
	}
	set := record.TestKnowledge
	if set == nil || record.BehaviorContracts != nil || record.Details != nil || record.RestrictedFindings != nil || len(record.Subjects)+len(record.Claims)+len(record.Relations)+len(record.Journeys)+len(record.Observations)+len(record.Capabilities) != 0 {
		return fail("test-knowledge provider carries only its test_knowledge member")
	}
	if !wire.IsGitOid(set.Revision) || len(set.Records) == 0 || len(set.Records) > MaxTestKnowledge {
		return fail("invalid test-knowledge revision or record bound")
	}
	seen := map[string]bool{}
	for _, r := range set.Records {
		if !strings.HasPrefix(r.ID, p.ID+":") || !safeText(r.ID, 1024) || seen[r.ID] {
			return fail("test-knowledge id must be unique and provider-qualified")
		}
		seen[r.ID] = true
		if !testKnowledgeKinds[r.Kind] || !testKnowledgeVerdicts[r.Verdict] || !testKnowledgeTrust[r.Trust] || !testKnowledgeDocs[r.Docs.State] {
			return fail("unknown test-knowledge kind, verdict, docs state or trust level")
		}
		if !safeText(r.Framework, 128) || !safeText(r.Title, 4096) || !safeText(r.Entry, 4096) || !safeText(r.Actor, 4096) || !safeText(r.Behavior, 16<<10) {
			return fail("invalid or secret-shaped test-knowledge text")
		}
		if !safeList(r.Setup, 64, true) || !safeList(r.Asserted, 64, true) || len(r.Stubs) > 64 || len(r.Sources) == 0 || len(r.Sources) > 64 || len(r.Flows) > 64 {
			return fail("invalid or secret-shaped test-knowledge list")
		}
		for _, stub := range r.Stubs {
			if !testKnowledgeStubs[stub.Boundary] || !safeText(stub.Name, 4096) {
				return fail("invalid test-knowledge stub")
			}
		}
		flows := map[string]bool{}
		for _, f := range r.Flows {
			if !safeText(f, 1024) || flows[f] {
				return fail("invalid or duplicate test-knowledge flow")
			}
			flows[f] = true
		}
		if (r.Verdict == "partial") != (r.Gap != "") || r.Gap != "" && !safeText(r.Gap, 16<<10) {
			return fail("test-knowledge gap is required exactly for a partial verdict")
		}
		if (r.Docs.State == "contradicts") != (r.Docs.Claim != "") || r.Docs.Claim != "" && !safeText(r.Docs.Claim, 1024) {
			return fail("test-knowledge docs claim is required exactly for contradicts")
		}
		run := r.Trust == "run+reviewed" || r.Trust == "run-unreviewed"
		if run != (r.Receipt != nil) {
			return fail("test-knowledge receipt is required exactly for a run trust level")
		}
		if (r.Trust == "run+reviewed") != (len(r.Reviews) > 0) || !safeList(r.Reviews, 16, true) {
			return fail("test-knowledge reviews are required exactly for run+reviewed")
		}
		anchors := append([]Anchor{r.Test}, r.Sources...)
		for _, a := range anchors {
			if a.Revision != set.Revision {
				return fail("test-knowledge anchor is not pinned to the set revision")
			}
		}
		if r.Receipt != nil {
			anchors = append(anchors, *r.Receipt)
		}
		for _, a := range anchors {
			if err := c.checkTestKnowledgeAnchor(a); err != nil {
				return err
			}
		}
	}
	return nil
}
func (c *compiler) checkTestKnowledgeAnchor(a Anchor) error {
	if a.Repository != c.manifest.Repository.ID || a.Authority != "external-provider" || !words("source declared observed review imported")[a.Kind] || a.Symbol != "" || a.Region != "" {
		return fail("invalid test-knowledge anchor identity")
	}
	if !wire.IsGitOid(a.Revision) || !validPath(a.Path) || !wire.IsGitOid(a.Blob) || !wire.IsSha256(a.SHA256) || !wire.IsSha256(a.SpanSHA256) || a.Start < 1 || a.End < a.Start || !safeText(a.Reason, 4096) {
		return fail("invalid test-knowledge anchor pin")
	}
	return nil
}

// resolveTestKnowledgeAnchor compares a pinned anchor with the declared input
// at revision. It reports an undeclared path as unanchored and refuses forged bindings.
func (c *compiler) resolveTestKnowledgeAnchor(a Anchor, revision string) (string, error) {
	source, ok := c.sources[inputKey(revision, a.Path)]
	if !ok {
		return "unanchored", nil
	}
	data, err := span(source.Data, a.Start, a.End)
	if source.BlobHash != a.Blob || err != nil || Digest(source.Data) != a.SHA256 || Digest(data) != a.SpanSHA256 {
		return "", fail("test-knowledge anchor binding mismatch")
	}
	return "joined", nil
}

// joinTestKnowledge runs after every provider is imported (DCP-V1-089, DCP-V1-090).
func (c *compiler) joinTestKnowledge(sets []providerKnowledge) error {
	revision := c.manifest.Repository.Revision
	flowKinds := map[string]bool{}
	for _, s := range c.artifact.Subjects {
		if s.Kind == "flow" || s.Kind == "variation" {
			flowKinds[s.ID] = true
		}
	}
	seen := map[string]bool{}
	total := 0
	for _, pk := range sets {
		total += len(pk.set.Records)
		if total > MaxTestKnowledge {
			return fail("aggregate test-knowledge bound exceeded")
		}
		for _, r := range pk.set.Records {
			if seen[r.ID] {
				return fail("duplicate test-knowledge id")
			}
			seen[r.ID] = true
			j := TestKnowledgeJoin{ID: r.ID, Provider: pk.provider, TrustLevel: r.Trust, Verdict: r.Verdict, Status: "joined", Selection: "excluded", Claims: []string{}, Flows: []string{}, UnresolvedFlows: []string{}, Paths: []string{}, Reasons: []string{}, Limitations: []string{testKnowledgeLimitation}, Record: r}
			anchors := append([]Anchor{r.Test}, r.Sources...)
			if pk.set.Revision != revision {
				// A claimed revision is never trusted on its format alone: only the corpus
				// revision's declared inputs are verified, so any other revision is stale (DCP-V1-089).
				j.Status, anchors = "stale", nil
				j.Reasons = append(j.Reasons, "stale: set revision is not the corpus revision")
			}
			for _, a := range anchors {
				status, err := c.resolveTestKnowledgeAnchor(a, revision)
				if err != nil {
					return err
				}
				if status != "joined" {
					j.Reasons = append(j.Reasons, status+": "+a.Path)
					if j.Status != "unanchored" {
						j.Status = status
					}
				}
			}
			if r.Receipt != nil {
				if status, err := c.resolveTestKnowledgeAnchor(*r.Receipt, r.Receipt.Revision); err != nil {
					return err
				} else if status != "joined" {
					j.Reasons = append(j.Reasons, "unanchored: run receipt "+r.Receipt.Path+" is not a declared input")
					j.Status = "unanchored"
				}
			}
			if j.Status == "joined" {
				c.joinTestKnowledgeTargets(&j, anchors, flowKinds)
			}
			j.CountsTowardCoverage = j.Status == "joined" && r.Trust == "run+reviewed" && (r.Verdict == "bound" || r.Verdict == "partial")
			switch {
			case j.Status == "joined" && r.Verdict == "defect":
				j.Selection = "blocks"
			case j.CountsTowardCoverage:
				j.Selection = "candidate"
			}
			sort.Strings(j.Reasons)
			c.artifact.TestKnowledge = append(c.artifact.TestKnowledge, j)
		}
	}
	sort.Slice(c.artifact.TestKnowledge, func(i, k int) bool { return c.artifact.TestKnowledge[i].ID < c.artifact.TestKnowledge[k].ID })
	return nil
}
func (c *compiler) joinTestKnowledgeTargets(j *TestKnowledgeJoin, anchors []Anchor, flowKinds map[string]bool) {
	revision := c.manifest.Repository.Revision
	paths := map[string]bool{}
	for _, a := range anchors {
		paths[a.Path] = true
	}
	for p := range paths {
		j.Paths = append(j.Paths, p)
	}
	for _, claim := range c.artifact.Claims {
		joined := false
		for _, ca := range claim.Evidence.Anchors {
			for _, a := range anchors {
				joined = joined || ca.Revision == revision && ca.Path == a.Path && ca.Start <= a.End && a.Start <= ca.End
			}
		}
		if joined {
			j.Claims = append(j.Claims, claim.ID)
		}
	}
	for _, f := range j.Record.Flows {
		if flowKinds[f] {
			j.Flows = append(j.Flows, f)
		} else {
			j.UnresolvedFlows = append(j.UnresolvedFlows, f)
		}
	}
	sort.Strings(j.Paths)
	sort.Strings(j.Claims)
	sort.Strings(j.Flows)
	sort.Strings(j.UnresolvedFlows)
}

// testKnowledgeServes reports whether op is answered from test-knowledge joins, so a
// corpus whose only rows are test knowledge still serves it without the generic capability
// (DCP-V1-091).
func testKnowledgeServes(a *Artifact, op string) bool {
	switch op {
	case "get", "locate", "claims", "recommend-tests":
		return len(a.TestKnowledge) > 0
	}
	return false
}

// testKnowledgeOnPath reports joined records whose test or source path is p.
func testKnowledgeOnPath(a *Artifact, p string) []TestKnowledgeJoin {
	out := []TestKnowledgeJoin{}
	for _, j := range a.TestKnowledge {
		if j.Status != "joined" {
			continue
		}
		for _, q := range j.Paths {
			if q == p {
				out = append(out, j)
				break
			}
		}
	}
	return out
}
func testKnowledgeAnchors(j TestKnowledgeJoin) []Anchor {
	out := append([]Anchor{j.Record.Test}, j.Record.Sources...)
	if j.Record.Receipt != nil {
		out = append(out, *j.Record.Receipt)
	}
	return out
}

// testKnowledgeCoverage reports labelled rows; only counted records enter a numerator.
func testKnowledgeCoverage(a *Artifact) []any {
	if len(a.TestKnowledge) == 0 {
		return nil
	}
	flows, covered := 0, map[string]bool{}
	for _, j := range a.TestKnowledge {
		if j.CountsTowardCoverage {
			for _, f := range j.Flows {
				covered[f] = true
			}
		}
	}
	reviewed := 0
	for _, s := range a.Subjects {
		if s.Kind == "flow" {
			flows++
			if covered[s.ID] {
				reviewed++
			}
		}
	}
	limitations := []string{testKnowledgeLimitation, "only joined run+reviewed records with a bound or partial verdict count"}
	rows := []any{map[string]any{"metric": "flows_with_reviewed_test_knowledge", "value": reviewed, "denominator": flows, "defined": flows > 0, "definition": "flow subjects named by a counted test-knowledge record / all flow subjects", "revision": a.Manifest.Repository.Revision, "limitations": limitations}}
	for _, level := range []string{"run+reviewed", "run-unreviewed", "read-only", "legacy-reference"} {
		// Rows carry counts only; record IDs page through inventory and get (DCP-V1-091).
		joined, counted := 0, 0
		for _, j := range a.TestKnowledge {
			if j.TrustLevel != level || j.Status != "joined" {
				continue
			}
			joined++
			if j.CountsTowardCoverage {
				counted++
			}
		}
		rows = append(rows, map[string]any{"metric": "test_knowledge_trust_level", "trust_level": level, "joined": joined, "counts_toward_coverage": counted, "revision": a.Manifest.Repository.Revision, "limitations": limitations})
	}
	return rows
}

// testKnowledgeGaps reports records that were not joined or name unknown flows.
func testKnowledgeGaps(a *Artifact) []Gap {
	out := []Gap{}
	for _, j := range a.TestKnowledge {
		switch j.Status {
		case "stale":
			out = append(out, Gap{j.ID, "stale-test-knowledge", strings.Join(j.Reasons, "; ")})
		case "unanchored":
			out = append(out, Gap{j.ID, "unanchored-test-knowledge", strings.Join(j.Reasons, "; ")})
		}
		for _, f := range j.UnresolvedFlows {
			out = append(out, Gap{j.ID, "unresolved-test-knowledge-flow", "declared flow is not a flow or variation subject: " + f})
		}
	}
	return out
}
