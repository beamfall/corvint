package doccorpus

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

var subjectKinds = words("repository package module symbol endpoint ui_surface capability use_case interaction test journey document external_system data_entity change finding business_term")
var relationKinds = words("implements calls imports documents tested_by journey_for depends_on affects related_to conflicts_with supersedes")
var derivations = words("generated source-derived observed declared imported")
var claimStates = words("supported conflicted unknown")
var capabilityNames = []string{"info", "subjects", "claims", "relations", "coverage", "gaps", "journeys", "observations", "stability"}

func words(s string) map[string]bool {
	m := map[string]bool{}
	for _, s := range strings.Fields(s) {
		m[s] = true
	}
	return m
}

func (c *compiler) importRecords(p Provider) error {
	source, ok := c.sources[inputKey(p.Revision, p.Record)]
	if !ok {
		return &Error{Code: "corpus-provider-unavailable", Message: "declared provider record missing"}
	}
	declared := false
	for _, in := range c.manifest.Inputs {
		if in.Provider == p.ID && in.Purpose == "provider" && in.Path == p.Record && in.Revision == p.Revision {
			declared = true
		}
	}
	if !declared {
		return fail("provider input purpose mismatch")
	}
	var record ProviderRecord
	if err := decode(source.Data, &record); err != nil {
		return err
	}
	validSchema := record.Schema == ProviderSchema && record.BehaviorContracts == nil || record.Schema == BehaviorProviderSchema && record.BehaviorContracts != nil && record.BehaviorContracts.Stability == nil || record.Schema == BehaviorStabilityProviderSchema && record.BehaviorContracts != nil && record.BehaviorContracts.Stability != nil
	if !validSchema || record.ID != p.ID || record.Version != p.Version || record.Source != c.manifest.Repository {
		return fail("provider revision, version or repository mismatch")
	}
	if len(record.Subjects) > MaxRecords || len(record.Claims) > MaxRecords || len(record.Relations) > MaxRecords || len(record.Observations) > MaxRecords || len(record.Journeys) > 128 {
		return fail("provider record bound exceeded")
	}
	declaredCaps := map[string]bool{}
	for _, cap := range record.Capabilities {
		if !words(strings.Join(capabilityNames, " "))[cap.Name] || declaredCaps[cap.Name] || !words("present absent unsupported not-collected")[cap.State] || !textOK(cap.Reason) {
			return fail("invalid or duplicate capability declaration")
		}
		declaredCaps[cap.Name] = true
	}
	counts := map[string]int{"subjects": len(record.Subjects), "claims": len(record.Claims), "relations": len(record.Relations), "journeys": len(record.Journeys), "observations": len(record.Observations)}
	for name, count := range counts {
		if count == 0 {
			continue
		}
		present := false
		for _, cap := range record.Capabilities {
			if cap.Name == name && cap.State == "present" {
				present = true
			}
		}
		if !present {
			return fail("provider records lack a present capability declaration")
		}
	}
	for i := range record.Subjects {
		if err := c.importEvidence(p.ID, record.Subjects[i].ID, record.Subjects[i].Provider, &record.Subjects[i].Evidence); err != nil {
			return err
		}
	}
	for i := range record.Claims {
		if err := c.importEvidence(p.ID, record.Claims[i].ID, record.Claims[i].Provider, &record.Claims[i].Evidence); err != nil {
			return err
		}
	}
	for i := range record.Relations {
		if err := c.importEvidence(p.ID, record.Relations[i].ID, record.Relations[i].Provider, &record.Relations[i].Evidence); err != nil {
			return err
		}
	}
	for i := range record.Journeys {
		j := &record.Journeys[i]
		if err := c.importEvidence(p.ID, j.ID, j.Provider, &j.Evidence); err != nil {
			return err
		}
		if len(j.Steps) > 128 {
			return fail("journey step bound exceeded")
		}
		if j.Status == "verified" {
			return fail("provider cannot self-declare a verified journey")
		}
		for n := range j.Steps {
			if err := c.checkEvidence(&j.Steps[n].Evidence, true); err != nil {
				return err
			}
		}
	}
	c.artifact.Subjects = append(c.artifact.Subjects, record.Subjects...)
	c.artifact.Claims = append(c.artifact.Claims, record.Claims...)
	c.artifact.Relations = append(c.artifact.Relations, record.Relations...)
	c.artifact.Journeys = append(c.artifact.Journeys, record.Journeys...)
	c.declarations = append(c.declarations, record.Capabilities...)
	for _, link := range record.Observations {
		if !strings.HasPrefix(link.ID, p.ID+":") {
			return fail("observation id must be provider-qualified")
		}
		observation, err := c.observation(link)
		if err != nil {
			return err
		}
		c.artifact.Observations = append(c.artifact.Observations, observation)
	}
	if record.BehaviorContracts != nil {
		return c.importBehavior(p, record.BehaviorContracts)
	}
	return nil
}
func (c *compiler) importEvidence(provider, id, owner string, e *Evidence) error {
	if owner != provider || !strings.HasPrefix(id, provider+":") {
		return fail("record identity is not provider-qualified")
	}
	return c.checkEvidence(e, true)
}
func (c *compiler) checkEvidence(e *Evidence, external bool) error {
	if !derivations[e.Derivation] || !claimStates[e.State] || !words("fresh stale unknown")[e.Freshness] {
		return fail("invalid evidence axes")
	}
	if len(e.Anchors) > 64 || len(e.Limitations) > 64 {
		return fail("evidence bound exceeded")
	}
	if e.Trust != "generated" && e.Trust != "reviewed" {
		return fail("provider cannot grant verified trust")
	}
	if e.ReportedTrust != "" {
		return fail("reported trust is compiler-owned")
	}
	if e.Trust == "reviewed" {
		if e.Derivation == "generated" || e.Derivation == "source-derived" {
			return fail("generated evidence cannot self-declare review")
		}
		e.ReportedTrust = "reviewed"
		e.Trust = "generated"
		e.Limitations = append(e.Limitations, "provider-reported review is unauthenticated and grants no Core authority")
	}
	if len(e.Anchors) == 0 && (e.State != "unknown" || !textOK(e.Unknown)) {
		return fail("evidence requires anchors or explicit unknown")
	}
	if e.State == "unknown" && !textOK(e.Unknown) {
		return fail("unknown evidence requires reason")
	}
	for _, a := range e.Anchors {
		if err := c.checkAnchor(a, external); err != nil {
			return err
		}
	}
	return nil
}
func (c *compiler) checkAnchor(a Anchor, external bool) error {
	if a.Repository != c.manifest.Repository.ID || !textOK(a.Reason) {
		return fail("invalid anchor identity or reason")
	}
	if !words("source declared observed review imported")[a.Kind] {
		return fail("invalid evidence kind")
	}
	if external && a.Authority != "external-provider" {
		return fail("imported evidence cannot promote source authority")
	}
	if !external && !words("syntax source-document external-provider")[a.Authority] {
		return fail("unknown authority")
	}
	if !validPath(a.Path) {
		return fail("invalid anchor path")
	}
	expected, err := c.anchor(a.Revision, a.Path, a.Start, a.End, a.Authority, a.Kind, a.Reason)
	if err != nil {
		return err
	}
	if a.Blob != expected.Blob || a.SHA256 != expected.SHA256 || a.SpanSHA256 != expected.SpanSHA256 {
		return fail("anchor binding mismatch")
	}
	if a.Symbol != "" {
		found := false
		index := c.indexes[a.Revision]
		for _, s := range index.Symbols {
			if s.Path == a.Path && s.Name == a.Symbol && s.Line >= a.Start && s.Line <= a.End {
				found = true
			}
		}
		if !found {
			return fail("symbol anchor does not resolve")
		}
	}
	return nil
}
func (c *compiler) observation(link ObservationLink) (Observation, error) {
	source, ok := c.sources[inputKey(link.InputRevision, link.Input)]
	if !ok {
		return Observation{}, fail("observation input not declared")
	}
	declared := false
	for _, in := range c.manifest.Inputs {
		if in.Path == link.Input && in.Revision == link.InputRevision && in.Purpose == "observation" {
			declared = true
		}
	}
	if !declared || link.SourceRevision != c.manifest.Repository.Revision || link.RunID != Digest(source.Data) {
		return Observation{}, fail("observation source/run identity mismatch")
	}
	input, err := testvaliditydoc.Decode(source.Data)
	if err != nil {
		return Observation{}, fail("unsupported retained observation")
	}
	if len(link.SourcePaths) > MaxRecords {
		return Observation{}, fail("source path mapping bound exceeded")
	}
	for original, mapped := range link.SourcePaths {
		if !textOK(original) || !validPath(mapped) {
			return Observation{}, fail("invalid observation source path mapping")
		}
	}
	if link.StepInput != "" || len(link.StepEvidence) > 0 {
		stepDeclared := false
		for _, in := range c.manifest.Inputs {
			if in.Path == link.StepInput && in.Revision == link.StepRevision && in.Purpose == "evidence" {
				stepDeclared = true
			}
		}
		if !stepDeclared {
			return Observation{}, fail("independent step evidence input not declared")
		}
	}
	document := testvaliditydoc.ProjectPinned(input, func(original string) ([]byte, bool) {
		p := original
		if mapped, ok := link.SourcePaths[original]; ok {
			p = mapped
		}
		source, ok := c.sources[inputKey(link.SourceRevision, p)]
		return source.Data, ok
	})
	matches := 0
	for _, test := range document.Tests {
		if observationMatches(test, link) {
			matches++
		}
	}
	if matches != 1 {
		return Observation{}, fail("observation test identity missing or ambiguous")
	}
	for _, a := range link.StepEvidence {
		if a.Revision != link.StepRevision || a.Path != link.StepInput || a.Kind != "observed" {
			return Observation{}, fail("step witness must name the independent retained step artifact")
		}
		if err := c.checkAnchor(a, true); err != nil {
			return Observation{}, err
		}
	}
	trust := "generated"
	for _, test := range document.Tests {
		if observationMatches(test, link) && test.Projection.Execution.State == "PASSED" && test.Projection.Freshness.State == "CURRENT" {
			trust = "verified"
		}
	}
	if document.Run.Execution.State == "INCOMPLETE" || document.TestsOmitted > 0 || (document.Promotable != nil && !*document.Promotable) {
		trust = "generated"
	}
	return Observation{Link: link, InputSHA256: Digest(source.Data), Document: document, Trust: trust, Limitations: []string{"native projection and source digest binding reproduced; provider honesty, assertion adequacy and completeness are not authenticated"}}, nil
}

func (c *compiler) validateRecords() error {
	a := c.artifact
	if len(a.Subjects) > MaxRecords || len(a.Claims) > MaxRecords || len(a.Relations) > MaxRecords || len(a.Observations) > MaxRecords || len(a.Journeys) > 128 {
		return fail("corpus record bound exceeded")
	}
	ids := map[string]bool{}
	subjects := map[string]Subject{}
	admit := func(id string) error {
		if !identifier(id) || ids[id] {
			return fail("invalid or duplicate record id")
		}
		ids[id] = true
		return nil
	}
	for _, s := range a.Subjects {
		if err := admit(s.ID); err != nil {
			return err
		}
		if !subjectKinds[s.Kind] || !textOK(s.Name) {
			return fail("invalid subject")
		}
		subjects[s.ID] = s
	}
	for _, claim := range a.Claims {
		if err := admit(claim.ID); err != nil {
			return err
		}
		if _, ok := subjects[claim.Subject]; !ok {
			return fail("claim subject missing")
		}
		if !textOK(claim.Text) {
			return fail("claim text missing or over bound")
		}
	}
	for _, r := range a.Relations {
		if err := admit(r.ID); err != nil {
			return err
		}
		if !relationKinds[r.Type] {
			return fail("unsupported relationship")
		}
		if _, ok := subjects[r.From]; !ok {
			return fail("relationship source missing")
		}
		if _, ok := subjects[r.To]; !ok {
			return fail("relationship target missing")
		}
	}
	observations := map[string]Observation{}
	for _, o := range a.Observations {
		if err := admit(o.Link.ID); err != nil {
			return err
		}
		s, ok := subjects[o.Link.Subject]
		if !ok || s.Kind != "test" {
			return fail("observation must join a test subject")
		}
		observations[o.Link.ID] = o
	}
	for i := range a.Journeys {
		j := &a.Journeys[i]
		if err := admit(j.ID); err != nil {
			return err
		}
		if _, ok := subjects[j.Subject]; !ok {
			return fail("journey subject missing")
		}
		if !words("generated_not_verified missing_journey non_ui blocked")[j.Status] {
			return fail("invalid journey state")
		}
		if (j.Status == "missing_journey" || j.Status == "non_ui") && len(j.Steps) != 0 {
			return fail("absent journey cannot carry steps")
		}
		verified := len(j.Steps) > 0 && j.Status == "generated_not_verified" && j.Cleanup == "passed"
		stepIDs := map[string]bool{}
		runIdentity := ""
		for position, s := range j.Steps {
			if !identifier(s.ID) || stepIDs[s.ID] || !textOK(s.Action) || !textOK(s.Operation) || !textOK(s.Expected) {
				return fail("invalid journey step")
			}
			stepIDs[s.ID] = true
			observation, ok := observations[s.Observation]
			if !ok {
				verified = false
				continue
			}
			identity := observation.InputSHA256 + ":" + observation.Link.StepRevision + ":" + observation.Link.StepInput
			if position == 0 {
				runIdentity = identity
			}
			if identity != runIdentity {
				verified = false
			}
			if !c.stepVerified(s, observation, j.Cleanup, position, len(j.Steps)) {
				verified = false
			}
		}
		if verified {
			j.Status = "verified"
			j.Evidence.Limitations = append(j.Evidence.Limitations, "only exact retained test/step observations passed; assertion adequacy and completeness unknown")
		}
	}
	for _, s := range a.Subjects {
		if s.Kind == "test" {
			found := false
			for _, o := range a.Observations {
				if o.Link.Subject == s.ID {
					found = true
				}
			}
			if !found {
				a.Gaps = append(a.Gaps, Gap{s.ID, "not-collected", "test declaration has no retained observation"})
			}
		}
	}
	for _, claim := range a.Claims {
		if claim.Evidence.State != "supported" {
			a.Gaps = append(a.Gaps, Gap{claim.Subject, claim.Evidence.State, claim.Evidence.Unknown})
		}
	}
	return nil
}
func observationMatches(test testvaliditydoc.Test, link ObservationLink) bool {
	if test.Name != link.Test || test.Package != link.Package {
		return false
	}
	if link.TestID == "" && link.Project == "" {
		return true
	}
	return link.TestID != "" && link.Project != "" && test.ID == link.TestID && test.Project != nil && test.Project.Name == link.Project
}

func (c *compiler) stepVerified(step Step, o Observation, cleanup string, position, count int) bool {
	if o.Document.Run.Execution.State == "INCOMPLETE" || o.Document.TestsOmitted > 0 || o.Document.Run.Freshness.State != "CURRENT" {
		return false
	}
	if o.Document.Promotable != nil && !*o.Document.Promotable {
		return false
	}
	passed := false
	for _, test := range o.Document.Tests {
		if observationMatches(test, o.Link) && test.State == "passed" {
			passed = true
		}
	}
	if !passed || o.Link.StepInput == "" || len(o.Link.StepEvidence) == 0 {
		return false
	}
	source, ok := c.sources[inputKey(o.Link.StepRevision, o.Link.StepInput)]
	if !ok {
		return false
	}
	if inputKey(o.Link.StepRevision, o.Link.StepInput) == inputKey(o.Link.InputRevision, o.Link.Input) {
		return false
	}
	fullWitness := false
	for _, anchor := range o.Link.StepEvidence {
		if anchor.Start == 1 && anchor.SpanSHA256 == Digest(source.Data) {
			fullWitness = true
		}
	}
	if !fullWitness {
		return false
	}
	var run JourneyRun
	if decode(source.Data, &run) != nil {
		return false
	}
	if run.Schema != "corvint-corpus-journey-observations/1" || run.RunSHA256 != o.InputSHA256 || run.SourceRevision != o.Link.SourceRevision || run.Test != o.Link.Test || run.Package != o.Link.Package || run.Cleanup != "passed" || cleanup != run.Cleanup || len(run.Steps) > 128 || len(run.Steps) != count || position < 0 || position >= len(run.Steps) {
		return false
	}
	matches := 0
	for index, observed := range run.Steps {
		if index == position && observed.ID == step.ID && observed.Action == step.Action && observed.Operation == step.Operation && observed.Expected == step.Expected && observed.Observed == step.Expected && observed.Passed {
			matches++
		}
	}
	return matches == 1
}

func (c *compiler) capabilities() {
	a := c.artifact
	providers := []string{}
	native := false
	for _, p := range c.manifest.Providers {
		providers = append(providers, p.ID)
		native = native || p.Kind == "native"
	}
	sort.Strings(providers)
	for _, name := range capabilityNames {
		cap := Capability{Name: name, State: "absent", Reason: "not collected by any declared provider", Records: []string{}, Providers: providers, Tools: capabilityTools(name), Denominator: len(c.manifest.Inputs), Rule: "count records explicitly emitted by the declared providers"}
		if name == "info" || name == "coverage" || name == "gaps" || name == "stability" && len(a.StabilityEvidence) > 0 || (native && (name == "subjects" || name == "claims" || name == "relations")) {
			cap.State = "present"
			cap.Reason = "explicit native compiler capability"
		}
		for _, decl := range c.declarations {
			if decl.Name == name {
				if cap.State != "present" {
					cap.State = decl.State
					cap.Reason = decl.Reason
				}
				if decl.State == "present" {
					cap.State = "present"
					cap.Reason = decl.Reason
				}
			}
		}
		switch name {
		case "subjects":
			for _, r := range a.Subjects {
				cap.Records = append(cap.Records, r.ID)
			}
		case "claims":
			for _, r := range a.Claims {
				cap.Records = append(cap.Records, r.ID)
			}
		case "relations":
			for _, r := range a.Relations {
				cap.Records = append(cap.Records, r.ID)
			}
		case "journeys":
			for _, r := range a.Journeys {
				cap.Records = append(cap.Records, r.ID)
			}
		case "observations":
			for _, r := range a.Observations {
				cap.Records = append(cap.Records, r.Link.ID)
			}
		case "stability":
			for _, r := range a.StabilityEvidence {
				cap.Records = append(cap.Records, r.ID)
			}
		case "gaps":
			for i := range a.Gaps {
				cap.Records = append(cap.Records, fmt.Sprint(i))
			}
		}
		sort.Strings(cap.Records)
		cap.Count = len(cap.Records)
		a.Capabilities = append(a.Capabilities, cap)
	}
}
func capabilityTools(name string) []string {
	switch name {
	case "info":
		return []string{"info", "validate"}
	case "subjects":
		return []string{"search", "get", "locate", "trace"}
	case "relations":
		return []string{"related"}
	case "journeys":
		return []string{"journey"}
	case "coverage":
		return []string{"coverage"}
	case "gaps":
		return []string{"gaps"}
	case "stability":
		return []string{"stability"}
	}
	return []string{}
}
