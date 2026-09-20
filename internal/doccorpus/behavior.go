package doccorpus

import (
	"fmt"
	"slices"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// BehaviorProviderSchema is experimental: declarations never authorize narrowing.
const BehaviorProviderSchema = "corvint-corpus-behavior-provider/1"

type BehaviorRegistry struct {
	Revisions             BehaviorRevisions  `json:"revisions"`
	Discovery             Anchor             `json:"discovery"`
	Schema                int                `json:"schema"`
	ContractID            string             `json:"contract_id"`
	ContractSHA256        string             `json:"contract_sha256"`
	SourceRevision        string             `json:"source_revision"`
	DocumentationRevision string             `json:"documentation_revision"`
	Manifest              Anchor             `json:"migration_manifest"`
	Flows                 []BehaviorFlow     `json:"flows"`
	Behaviors             []BehaviorSource   `json:"source_behaviors"`
	Tests                 []BehaviorTest     `json:"tests"`
	Stability             *StabilityRegistry `json:"stability,omitempty"`
}
type BehaviorMigration struct {
	Revisions             BehaviorRevisions `json:"revisions"`
	Schema                int               `json:"schema"`
	ContractID            string            `json:"contract_id"`
	SourceRevision        string            `json:"source_revision"`
	DocumentationRevision string            `json:"documentation_revision"`
}
type BehaviorRevisions struct {
	App  Repository `json:"app"`
	E2E  Repository `json:"golf_e2e"`
	Docs Repository `json:"docs_corpus"`
}
type BehaviorDiscovery struct {
	Schema     string              `json:"schema"`
	Mode       string              `json:"mode"`
	Revisions  BehaviorRevisions   `json:"revisions"`
	Config     Anchor              `json:"config"`
	Executions []BehaviorExecution `json:"executions"`
}
type BehaviorExecution struct {
	ID       string `json:"id"`
	Project  string `json:"project"`
	Evidence Anchor `json:"evidence"`
}
type BehaviorFlow struct {
	Derivation       string          `json:"derivation"`
	ID               string          `json:"id"`
	Evidence         Anchor          `json:"evidence"`
	Criteria         []string        `json:"criteria"`
	Tests            []string        `json:"tests"`
	RequiredPages    []string        `json:"required_pages"`
	NegativeControls []string        `json:"negative_controls"`
	OrderedEvents    []BehaviorEvent `json:"ordered_events"`
	MissingReview    *Anchor         `json:"missing_e2e_review,omitempty"`
}
type BehaviorSource struct {
	ID       string   `json:"id"`
	Evidence Anchor   `json:"evidence"`
	Flows    []string `json:"flows"`
}
type BehaviorTest struct {
	ID         string              `json:"id"`
	Project    string              `json:"project"`
	Title      string              `json:"title"`
	Evidence   Anchor              `json:"evidence"`
	Flows      []string            `json:"flows"`
	Criteria   []string            `json:"criteria"`
	Assertions []BehaviorAssertion `json:"assertions"`
	Runtime    *BehaviorRuntime    `json:"runtime,omitempty"`
}
type BehaviorAssertion struct {
	ID         string `json:"id"`
	Behavior   string `json:"behavior"`
	Criterion  string `json:"criterion"`
	Annotation Anchor `json:"reviewed_annotation"`
	Matcher    string `json:"matcher"`
	Locator    string `json:"locator"`
	Value      string `json:"value"`
}
type BehaviorRuntime struct {
	Evidence    Anchor `json:"evidence"`
	Observation string `json:"observation"`
}

// BehaviorRun is a separate, digest-pinned ordered witness, not a passing-test claim.
type BehaviorRun struct {
	Revisions             BehaviorRevisions `json:"revisions"`
	Schema                string            `json:"schema"`
	ContractID            string            `json:"contract_id"`
	ContractSHA256        string            `json:"contract_sha256"`
	SourceRevision        string            `json:"source_revision"`
	DocumentationRevision string            `json:"documentation_revision"`
	RunSHA256             string            `json:"run_sha256"`
	TestID                string            `json:"test_id"`
	Project               string            `json:"project"`
	Retry                 int               `json:"retry"`
	Cleanup               string            `json:"cleanup"`
	Events                []BehaviorEvent   `json:"events"`
}
type BehaviorEvent struct {
	Context    string `json:"browser_context"`
	Page       string `json:"page"`
	Frame      string `json:"frame"`
	Navigation string `json:"navigation"`
	ParentPage string `json:"parent_page"`
	Behavior   string `json:"behavior"`
	Criterion  string `json:"criterion"`
	Matcher    string `json:"matcher"`
	Locator    string `json:"locator"`
	Value      string `json:"value"`
	Sequence   int    `json:"sequence"`
	Kind       string `json:"kind"`
	ID         string `json:"id"`
	Passed     bool   `json:"passed"`
}
type BehaviorReport struct {
	Discovery       BehaviorDiscovery `json:"discovery"`
	Provider        string            `json:"provider"`
	Registry        BehaviorRegistry  `json:"registry"`
	VerifiedTests   []string          `json:"verified_tests"`
	LinkedFlows     []string          `json:"linked_flows"`
	LinkedBehaviors []string          `json:"linked_behaviors"`
	VerifiedFlows   []string          `json:"verified_flows"`
	Fallback        string            `json:"fallback"`
	Limitations     []string          `json:"limitations"`
}

func uniqueIdentities(values []string) bool {
	seen := map[string]bool{}
	for _, value := range values {
		if !textOK(value) || seen[value] {
			return false
		}
		seen[value] = true
	}
	return len(values) <= MaxRecords
}

func (c *compiler) importBehavior(p Provider, r *BehaviorRegistry) error {
	if r.Schema != 2 || !textOK(r.ContractID) || !wire.IsSha256(r.ContractSHA256) || !wire.IsGitOid(r.SourceRevision) || !wire.IsGitOid(r.DocumentationRevision) {
		return fail("invalid behavior registry identity")
	}
	if len(r.Flows) > MaxRecords || len(r.Tests) > MaxRecords || len(r.Behaviors) > MaxRecords {
		return fail("behavior registry bound exceeded")
	}
	if c.manifest.BehaviorRevisions == nil || !validBehaviorRevisions(r.Revisions) || !validBehaviorRevisions(*c.manifest.BehaviorRevisions) {
		return fail("behavior revision set missing or invalid")
	}
	discovery, err := c.behaviorDiscovery(r)
	if err != nil {
		return err
	}
	if err := c.checkAnchor(r.Manifest, true); err != nil {
		return err
	}
	manifestSource := c.sources[inputKey(r.Manifest.Revision, r.Manifest.Path)]
	var migration BehaviorMigration
	if r.Manifest.Start != 1 || r.Manifest.SpanSHA256 != Digest(manifestSource.Data) || decode(manifestSource.Data, &migration) != nil || migration.Schema != 2 || migration.ContractID != r.ContractID || migration.SourceRevision != r.SourceRevision || migration.DocumentationRevision != r.DocumentationRevision || migration.Revisions != r.Revisions {
		return fail("behavior migration schema-2 identity mismatch")
	}
	// The registry digest covers its contract declarations, excluding itself and runtime.
	declarations := *r
	declarations.ContractSHA256 = ""
	declarations.Stability = nil
	declarations.Tests = slices.Clone(r.Tests)
	for i := range declarations.Tests {
		declarations.Tests[i].Runtime = nil
	}
	if hashValue(declarations) != r.ContractSHA256 {
		return fail("behavior contract digest mismatch")
	}
	ids := []string{}
	for _, f := range r.Flows {
		ids = append(ids, f.ID)
		if !words("generated source-derived declared imported")[f.Derivation] {
			return fail("invalid flow derivation")
		}
		if !uniqueIdentities(f.Criteria) || !uniqueIdentities(f.Tests) || !uniqueIdentities(f.RequiredPages) || !uniqueIdentities(f.NegativeControls) || len(f.OrderedEvents) > MaxRecords {
			return fail("invalid behavior flow identities")
		}
		if err := c.checkAnchor(f.Evidence, true); err != nil {
			return err
		}
		if f.MissingReview != nil {
			if err := c.checkAnchor(*f.MissingReview, true); err != nil {
				return err
			}
		}
	}
	for _, b := range r.Behaviors {
		ids = append(ids, b.ID)
		if !uniqueIdentities(b.Flows) {
			return fail("invalid source behavior identities")
		}
		if err := c.checkAnchor(b.Evidence, true); err != nil {
			return err
		}
	}
	for _, test := range r.Tests {
		ids = append(ids, test.ID)
		if !textOK(test.Project) || !textOK(test.Title) || !uniqueIdentities(test.Flows) || !uniqueIdentities(test.Criteria) || len(test.Assertions) > MaxRecords {
			return fail("invalid behavior test identity")
		}
		if err := c.checkAnchor(test.Evidence, true); err != nil {
			return err
		}
		assertionIDs := []string{}
		for _, assertion := range test.Assertions {
			assertionIDs = append(assertionIDs, assertion.ID)
			if !textOK(assertion.Behavior) || !textOK(assertion.Criterion) || !textOK(assertion.Matcher) || !textOK(assertion.Locator) || !textOK(assertion.Value) {
				return fail("incomplete assertion identity")
			}
			if err := c.checkAnchor(assertion.Annotation, true); err != nil {
				return err
			}
		}
		if !uniqueIdentities(assertionIDs) {
			return fail("duplicate assertion identity")
		}
		if test.Runtime != nil {
			if err := c.checkAnchor(test.Runtime.Evidence, true); err != nil {
				return err
			}
		}
	}
	if !uniqueIdentities(ids) {
		return fail("duplicate behavior identity")
	}
	published := *r
	published.Stability = nil
	c.artifact.BehaviorContracts = append(c.artifact.BehaviorContracts, BehaviorReport{Discovery: discovery, Provider: p.ID, Registry: published, VerifiedTests: []string{}, LinkedFlows: []string{}, LinkedBehaviors: []string{}, VerifiedFlows: []string{}, Fallback: "full-relevant-suite", Limitations: []string{"experimental provider declarations; exact consumer fixtures not qualified", "recorded verification is not semantic adequacy or authenticated runtime provenance", "retained application freshness remains unknown; recorded verification never asserts current served content", "external repository expectations and generated prose carry no Core authority", "no narrowing authority"}})
	if r.Stability != nil {
		return c.compileStability(p.ID, *r, *r.Stability)
	}
	return nil
}

func (c *compiler) behaviorGap(subject, kind, reason string) {
	c.artifact.Gaps = append(c.artifact.Gaps, Gap{subject, kind, reason})
}

func validBehaviorRevisions(r BehaviorRevisions) bool {
	for _, repo := range []Repository{r.App, r.E2E, r.Docs} {
		if !wire.IsGitOid(repo.ID) || !wire.IsGitOid(repo.Revision) {
			return false
		}
	}
	return true
}
func (c *compiler) behaviorDiscovery(r *BehaviorRegistry) (BehaviorDiscovery, error) {
	var d BehaviorDiscovery
	if err := c.checkAnchor(r.Discovery, true); err != nil {
		return d, err
	}
	source := c.sources[inputKey(r.Discovery.Revision, r.Discovery.Path)]
	if r.Discovery.Kind != "observed" || r.Discovery.Start != 1 || r.Discovery.SpanSHA256 != Digest(source.Data) || decode(source.Data, &d) != nil || d.Schema != "corvint-playwright-discovery/1" || d.Mode != "live-playwright-list" || len(d.Executions) > MaxRecords {
		return d, fail("invalid bound live discovery")
	}
	if err := c.checkAnchor(d.Config, true); err != nil {
		return d, err
	}
	ids := []string{}
	for _, execution := range d.Executions {
		ids = append(ids, execution.ID)
		if !textOK(execution.Project) {
			return d, fail("discovery project missing")
		}
		if err := c.checkAnchor(execution.Evidence, true); err != nil {
			return d, err
		}
	}
	if !uniqueIdentities(ids) {
		return d, fail("duplicate discovery execution identity")
	}
	return d, nil
}

func (c *compiler) compileBehaviors() {
	for i := range c.artifact.BehaviorContracts {
		c.compileBehavior(&c.artifact.BehaviorContracts[i])
	}
}
func (c *compiler) compileBehavior(report *BehaviorReport) {
	r := report.Registry
	flows := map[string]BehaviorFlow{}
	tests := map[string]BehaviorTest{}
	for _, f := range r.Flows {
		flows[f.ID] = f
	}
	for _, t := range r.Tests {
		tests[t.ID] = t
	}
	fresh := r.SourceRevision == c.manifest.Repository.Revision && c.manifest.BehaviorRevisions != nil && r.Revisions == *c.manifest.BehaviorRevisions && r.Revisions.E2E == c.manifest.Repository && report.Discovery.Revisions == r.Revisions && report.Discovery.Config.Revision == r.SourceRevision
	if !fresh {
		c.behaviorGap(report.Provider, "stale-revision", "behavior source revision differs from corpus")
	}
	if len(r.Flows) == 0 || len(r.Tests) == 0 {
		c.behaviorGap(report.Provider, "unreviewed-join", "empty inventory cannot establish absent tests")
	}
	for _, b := range r.Behaviors {
		linked := len(b.Flows) > 0 && b.Evidence.Revision == r.SourceRevision
		for _, id := range b.Flows {
			if _, ok := flows[id]; !ok {
				linked = false
			}
		}
		if linked {
			report.LinkedBehaviors = append(report.LinkedBehaviors, b.ID)
		} else {
			c.behaviorGap(b.ID, "unreviewed-join", "source-discovered behavior has no exact current documented flow join")
		}
	}
	for _, t := range r.Tests {
		valid := fresh && len(t.Flows) > 0 && t.Evidence.Revision == r.SourceRevision
		discovered := false
		for _, execution := range report.Discovery.Executions {
			if execution.ID == t.ID && execution.Project == t.Project && execution.Evidence.Revision == t.Evidence.Revision && execution.Evidence.Path == t.Evidence.Path && execution.Evidence.SHA256 == t.Evidence.SHA256 && execution.Evidence.Start == t.Evidence.Start {
				discovered = true
			}
		}
		if !discovered {
			valid = false
			c.behaviorGap(t.ID, "unreviewed-join", "test/project/source missing from bound live discovery")
		}
		if t.Evidence.Revision != r.SourceRevision {
			c.behaviorGap(t.ID, "stale-revision", "test anchor differs from contract source revision")
		}
		for _, criterion := range t.Criteria {
			if !behaviorAssertionDeclared(r, t, criterion) {
				valid = false
				c.behaviorGap(t.ID, "contradiction", "criterion lacks its exact runtime assertion identity")
			}
		}
		for _, assertion := range t.Assertions {
			if !behaviorAssertionValid(r, t, assertion) {
				valid = false
				c.behaviorGap(t.ID, "contradiction", "assertion has undeclared, unreviewed or stale behavior/criterion/annotation identity")
			}
		}
		if len(t.Assertions) == 0 {
			valid = false
			c.behaviorGap(t.ID, "assertion-free-ui-test", "no runtime assertion identities declared")
		}
		if len(t.Flows) == 0 {
			c.behaviorGap(t.ID, "missing-reverse-link", "test does not identify a documented flow")
		}
		for _, id := range t.Flows {
			f, ok := flows[id]
			if !ok || !slices.Contains(f.Tests, t.ID) {
				valid = false
				c.behaviorGap(t.ID, "missing-reverse-link", "test to flow join lacks exact reverse test/project identity")
			}
			for _, criterion := range t.Criteria {
				if !slices.Contains(f.Criteria, criterion) {
					valid = false
					c.behaviorGap(t.ID, "contradiction", "test criterion absent from linked flow")
				}
			}
		}
		if valid && c.behaviorRunVerified(r, t) {
			report.VerifiedTests = append(report.VerifiedTests, t.ID)
		} else {
			c.behaviorGap(t.ID, "unverified-contract", "missing, stale, contradictory or incomplete ordered runtime evidence")
		}
	}
	for _, f := range r.Flows {
		linked := fresh && f.Evidence.Revision == r.DocumentationRevision && len(f.Tests) > 0 && len(f.Criteria) > 0
		if f.Evidence.Revision != r.DocumentationRevision {
			c.behaviorGap(f.ID, "stale-revision", "flow anchor differs from contract documentation revision")
		}
		verified := linked
		for _, id := range f.Tests {
			t, ok := tests[id]
			if !ok || !slices.Contains(t.Flows, f.ID) {
				linked = false
				c.behaviorGap(f.ID, "missing-reverse-link", "flow lacks an exact test/project reverse join")
			}
			if !slices.Contains(report.VerifiedTests, id) {
				verified = false
			}
		}
		for _, criterion := range f.Criteria {
			covered := false
			for _, id := range f.Tests {
				if slices.Contains(tests[id].Criteria, criterion) {
					covered = true
				}
			}
			if !covered {
				linked = false
				c.behaviorGap(f.ID, "unreviewed-join", "documented criterion has no exact test join: "+criterion)
			}
		}
		if len(f.Tests) == 0 {
			kind := "unreviewed-join"
			// An explicit retained human review is a reported finding, not closed-world proof.
			if fresh && len(report.Discovery.Executions) > 0 && f.MissingReview != nil && f.MissingReview.Kind == "review" && f.MissingReview.Revision == r.DocumentationRevision {
				kind = "confirmed-missing_e2e"
			}
			c.behaviorGap(f.ID, kind, "no joined tests; confirmation, if present, is provider-reported review only")
		}
		if linked {
			report.LinkedFlows = append(report.LinkedFlows, f.ID)
		}
		if verified && linked {
			report.VerifiedFlows = append(report.VerifiedFlows, f.ID)
		}
	}
	for _, execution := range report.Discovery.Executions {
		if _, ok := tests[execution.ID]; !ok {
			c.behaviorGap(execution.ID, "unreviewed-join", "live-discovered execution has no behavior contract")
		}
	}
}

func behaviorAssertionDeclared(r BehaviorRegistry, t BehaviorTest, criterion string) bool {
	for _, assertion := range t.Assertions {
		if assertion.Criterion == criterion && behaviorAssertionValid(r, t, assertion) {
			return true
		}
	}
	return false
}

func behaviorAssertionValid(r BehaviorRegistry, t BehaviorTest, assertion BehaviorAssertion) bool {
	if !slices.Contains(t.Criteria, assertion.Criterion) {
		return false
	}
	if assertion.Annotation.Kind != "review" || assertion.Annotation.Revision != r.SourceRevision || assertion.Annotation.Path != t.Evidence.Path || assertion.Annotation.SHA256 != t.Evidence.SHA256 {
		return false
	}
	for _, behavior := range r.Behaviors {
		if behavior.ID != assertion.Behavior || behavior.Evidence.Revision != r.SourceRevision {
			continue
		}
		for _, flow := range r.Flows {
			if slices.Contains(t.Flows, flow.ID) && slices.Contains(behavior.Flows, flow.ID) && slices.Contains(flow.Criteria, assertion.Criterion) {
				return true
			}
		}
	}
	return false
}

func (c *compiler) behaviorRunVerified(r BehaviorRegistry, test BehaviorTest) bool {
	if test.Runtime == nil {
		for _, f := range r.Flows {
			if !slices.Contains(test.Flows, f.ID) {
				continue
			}
			for _, page := range f.RequiredPages {
				c.behaviorGap(test.ID, "missing-required-page", page)
			}
			for _, control := range f.NegativeControls {
				c.behaviorGap(test.ID, "missing-negative-control", control)
			}
		}
		return false
	}
	a := test.Runtime.Evidence
	source, ok := c.sources[inputKey(a.Revision, a.Path)]
	if !ok || a.Start != 1 || a.SpanSHA256 != Digest(source.Data) || a.Kind != "observed" {
		return false
	}
	var run BehaviorRun
	if decode(source.Data, &run) != nil {
		return false
	}
	if run.Schema != "corvint-behavior-run/1" || run.ContractID != r.ContractID || run.ContractSHA256 != r.ContractSHA256 || run.SourceRevision != r.SourceRevision || run.DocumentationRevision != r.DocumentationRevision || run.Revisions != r.Revisions || run.TestID != test.ID || run.Project != test.Project || run.Retry < 0 || run.Cleanup != "passed" || len(run.Events) == 0 || len(run.Events) > MaxRecords {
		return false
	}
	matched := false
	var discovery BehaviorDiscovery
	if decode(c.sources[inputKey(r.Discovery.Revision, r.Discovery.Path)].Data, &discovery) != nil {
		return false
	}
	for _, o := range c.artifact.Observations {
		if o.Link.ID != test.Runtime.Observation || o.Link.TestID != test.ID || o.Link.Project != test.Project || o.InputSHA256 != run.RunSHA256 || o.Link.SourceRevision != r.SourceRevision || !c.behaviorNativeReady(o) {
			continue
		}
		if o.Document.Playwright.Identity.ConfigDigest != discovery.Config.SHA256 {
			continue
		}
		configPath := o.Document.Playwright.Identity.ConfigFile
		if mapped, ok := o.Link.SourcePaths[configPath]; ok {
			configPath = mapped
		}
		if configPath != discovery.Config.Path {
			continue
		}
		bound := false
		for _, native := range o.Document.Playwright.Tests {
			if native.ID != test.ID || native.Anchor == nil {
				continue
			}
			path := native.Anchor.File
			if mapped, ok := o.Link.SourcePaths[path]; ok {
				path = mapped
			}
			if path == test.Evidence.Path && o.Document.Playwright.Identity.TestFileDigests[native.Anchor.File] == test.Evidence.SHA256 && native.Anchor.Line >= test.Evidence.Start && native.Anchor.Line <= test.Evidence.End {
				bound = true
			}
		}
		if !bound {
			continue
		}
		for _, observed := range o.Document.Tests {
			if observed.ID != test.ID || observed.Project == nil || observed.Project.Name != test.Project || observed.Name != test.Title || observed.State != "passed" || observed.Projection.Execution.State != "PASSED" {
				continue
			}
			for _, attempt := range observed.Attempts {
				if attempt.Retry == run.Retry && attempt.State == "passed" {
					matched = true
				}
			}
		}
	}
	if !matched {
		return false
	}
	seen := map[string]bool{}
	for i, event := range run.Events {
		if event.Sequence != i+1 || !event.Passed || !textOK(event.ID) || !textOK(event.Context) || !textOK(event.Page) || !textOK(event.Frame) || !words("page assertion negative-control")[event.Kind] || seen[event.Kind+":"+event.ID] {
			return false
		}
		if event.Kind == "page" && !words("main-frame frame redirect popup setup")[event.Navigation] {
			return false
		}
		if event.Navigation == "popup" && !textOK(event.ParentPage) {
			return false
		}
		if event.Kind == "assertion" {
			declared := false
			for _, assertion := range test.Assertions {
				if behaviorAssertionEventMatches(event, assertion) {
					declared = true
				}
			}
			if !declared {
				c.behaviorGap(test.ID, "contradiction", "runtime assertion has no validated declaration")
				return false
			}
		}
		seen[event.Kind+":"+event.ID] = true
	}
	valid := true
	for _, assertion := range test.Assertions {
		matchedAssertion := false
		for _, event := range run.Events {
			if behaviorAssertionEventMatches(event, assertion) {
				matchedAssertion = true
			}
		}
		if !matchedAssertion {
			valid = false
		}
	}
	for _, f := range r.Flows {
		if !slices.Contains(test.Flows, f.ID) {
			continue
		}
		if len(f.OrderedEvents) == 0 || !slices.Equal(f.OrderedEvents, run.Events) {
			valid = false
			c.behaviorGap(test.ID, "contradiction", "runtime order differs from declared contract")
		}
		for _, page := range f.RequiredPages {
			if !seen["page:"+page] {
				valid = false
				c.behaviorGap(test.ID, "missing-required-page", page)
			}
		}
		for _, control := range f.NegativeControls {
			if !seen["negative-control:"+control] {
				valid = false
				c.behaviorGap(test.ID, "missing-negative-control", control)
			}
		}
	}
	return valid
}

func behaviorAssertionEventMatches(event BehaviorEvent, assertion BehaviorAssertion) bool {
	return event.Kind == "assertion" && event.ID == assertion.ID && event.Behavior == assertion.Behavior && event.Criterion == assertion.Criterion && event.Matcher == assertion.Matcher && event.Locator == assertion.Locator && event.Value == assertion.Value
}

func (c *compiler) behaviorNativeReady(o Observation) bool {
	d := o.Document
	if d.Playwright == nil || d.TestsOmitted != 0 || d.Run.Execution.State == "INCOMPLETE" || d.Run.Freshness.State == "STALE" {
		return false
	}
	// ProjectPinned leaves E2E app identity unknown even when all source bytes
	// match. Preserve that axis; this profile verifies only the retained run.
	if d.Run.Freshness.State != "CURRENT" && d.Run.Freshness.Reason != "retained-app-build-identity-unverifiable" {
		return false
	}
	identity := d.Playwright.Identity
	inputs := map[string]string{}
	for path, digest := range identity.TestFileDigests {
		inputs[path] = digest
	}
	for path, digest := range identity.ConfigInputDigests {
		inputs[path] = digest
	}
	inputs[identity.ConfigFile] = identity.ConfigDigest
	for original, digest := range inputs {
		path := original
		if mapped, ok := o.Link.SourcePaths[original]; ok {
			path = mapped
		}
		source, ok := c.sources[inputKey(o.Link.SourceRevision, path)]
		if !ok || Digest(source.Data) != digest {
			return false
		}
	}
	return len(inputs) > 0
}

func behaviorCoverage(a *Artifact) []any {
	rows := []any{}
	for _, report := range a.BehaviorContracts {
		r := report.Registry
		for _, metric := range []struct {
			name         string
			count, total int
		}{{"documented_flows", len(report.LinkedFlows), len(r.Flows)}, {"source_discovered_behaviors", len(report.LinkedBehaviors), len(r.Behaviors)}, {"discovered_playwright_project_executions", len(report.VerifiedTests), len(report.Discovery.Executions)}, {"verified_contracts", len(report.VerifiedFlows), len(r.Flows)}} {
			rows = append(rows, map[string]any{"metric": metric.name, "provider": report.Provider, "value": metric.count, "denominator": metric.total, "defined": metric.total > 0, "revision": r.SourceRevision, "definition": fmt.Sprintf("scoped %s inventory; separate denominator", metric.name), "limitations": report.Limitations})
		}
	}
	return rows
}
