package doccorpus

import "slices"

// Legacy criteria describe observables, not test-name similarity or inferred intent.
type BehaviorLegacyCase struct {
	ID             string                    `json:"id"`
	Suite          string                    `json:"suite"`
	Case           string                    `json:"case"`
	Evidence       Anchor                    `json:"evidence"`
	State          string                    `json:"state"`
	Fixtures       []string                  `json:"fixtures"`
	Roles          []string                  `json:"roles"`
	Criteria       []BehaviorLegacyCriterion `json:"criteria"`
	Runtime        *BehaviorRuntime          `json:"runtime,omitempty"`
	RuntimeTestID  string                    `json:"runtime_test_id,omitempty"`
	RuntimeProject string                    `json:"runtime_project,omitempty"`
}
type BehaviorLegacyCriterion struct {
	ID       string `json:"id"`
	Evidence Anchor `json:"evidence"`
	Matcher  string `json:"matcher"`
	Locator  string `json:"locator"`
	Value    string `json:"value"`
}
type BehaviorLegacyMapping struct {
	Criterion       string `json:"criterion"`
	LegacyCase      string `json:"legacy_case"`
	LegacyCriterion string `json:"legacy_criterion"`
	Relation        string `json:"relation"`
	Review          Anchor `json:"review"`
}
type BehaviorLegacyRun struct {
	Schema         string                    `json:"schema"`
	ContractSHA256 string                    `json:"contract_sha256"`
	Revisions      BehaviorRevisions         `json:"revisions"`
	CaseID         string                    `json:"case_id"`
	SourceSHA256   string                    `json:"source_sha256"`
	RunSHA256      string                    `json:"run_sha256"`
	TestID         string                    `json:"test_id"`
	Project        string                    `json:"project"`
	Retry          int                       `json:"retry"`
	Cleanup        string                    `json:"cleanup"`
	Fixtures       []string                  `json:"fixtures"`
	Roles          []string                  `json:"roles"`
	Criteria       []BehaviorLegacyCriterion `json:"criteria"`
}

func (c *compiler) importBehaviorLegacy(r *BehaviorRegistry) error {
	if len(r.Legacy) > MaxRecords {
		return fail("legacy case bound exceeded")
	}
	ids := []string{}
	locations := []string{}
	for _, legacy := range r.Legacy {
		ids = append(ids, legacy.ID)
		locations = append(locations, hashValue([]string{legacy.Suite, legacy.Evidence.Repository, legacy.Evidence.Revision, legacy.Evidence.Path, legacy.Case}))
		if !textOK(legacy.Suite) || !textOK(legacy.Case) || !words("executable disabled")[legacy.State] || !uniqueIdentities(legacy.Fixtures) || !uniqueIdentities(legacy.Roles) || len(legacy.Criteria) > MaxRecords {
			return fail("invalid legacy case identity")
		}
		if err := c.checkAnchor(legacy.Evidence, true); err != nil {
			return err
		}
		criteria := []string{}
		for _, criterion := range legacy.Criteria {
			criteria = append(criteria, criterion.ID)
			if !textOK(criterion.Matcher) || !textOK(criterion.Locator) || !textOK(criterion.Value) {
				return fail("invalid extracted legacy criterion")
			}
			if err := c.checkAnchor(criterion.Evidence, true); err != nil {
				return err
			}
		}
		if !uniqueIdentities(criteria) {
			return fail("duplicate legacy criterion")
		}
		if legacy.Runtime != nil {
			if err := c.checkAnchor(legacy.Runtime.Evidence, true); err != nil {
				return err
			}
		}
	}
	if !uniqueIdentities(ids) || !uniqueIdentities(locations) {
		return fail("duplicate legacy case")
	}
	for _, test := range r.Tests {
		if !uniqueIdentities(test.Fixtures) || !uniqueIdentities(test.Roles) {
			return fail("invalid target preconditions")
		}
		if len(test.Legacy) > MaxRecords {
			return fail("legacy mapping bound exceeded")
		}
		keys := []string{}
		for _, mapping := range test.Legacy {
			if !textOK(mapping.Criterion) || !textOK(mapping.LegacyCase) || !textOK(mapping.LegacyCriterion) || !words("same stronger new obsolete blocked")[mapping.Relation] {
				return fail("invalid legacy criterion mapping")
			}
			keys = append(keys, hashValue([]string{mapping.Criterion, mapping.LegacyCase, mapping.LegacyCriterion}))
			if err := c.checkAnchor(mapping.Review, true); err != nil {
				return err
			}
		}
		if !uniqueIdentities(keys) {
			return fail("duplicate legacy mapping")
		}
	}
	return nil
}

func (c *compiler) compileBehaviorLegacy(report *BehaviorReport) {
	r := report.Registry
	report.LegacyRuntimeParity = []string{}
	covered := map[string]bool{}
	eligible := map[string]bool{}
	runtime := map[string]bool{}
	legacyCases := map[string]BehaviorLegacyCase{}
	for _, legacy := range r.Legacy {
		legacyCases[legacy.ID] = legacy
		runtime[legacy.ID] = c.behaviorLegacyRuntime(r, legacy)
		if !runtime[legacy.ID] {
			c.behaviorGap(legacy.ID, "legacy-runtime-unknown", "no qualified executable legacy baseline; source/docs/product review is not runtime parity")
		}
	}
	for _, test := range r.Tests {
		valid := len(test.Criteria) > 0 && len(r.Legacy) > 0 && slices.Contains(report.VerifiedTests, test.ID)
		mapped := map[string]bool{}
		for _, mapping := range test.Legacy {
			legacy, exists := legacyCases[mapping.LegacyCase]
			var criterion *BehaviorLegacyCriterion
			for i := range legacy.Criteria {
				if legacy.Criteria[i].ID == mapping.LegacyCriterion {
					criterion = &legacy.Criteria[i]
				}
			}
			joined := exists && criterion != nil && slices.Contains(test.Criteria, mapping.Criterion) && mapping.Review.Kind == "review" && mapping.Review.Revision == r.SourceRevision
			if joined {
				joined = criterion.Evidence.Revision == legacy.Evidence.Revision && criterion.Evidence.Path == legacy.Evidence.Path && criterion.Evidence.SHA256 == legacy.Evidence.SHA256 && criterion.Evidence.Start >= legacy.Evidence.Start && criterion.Evidence.End <= legacy.Evidence.End
			}
			if !joined {
				valid = false
				c.behaviorGap(test.ID, "legacy-criterion-contradiction", "missing, stale or unreviewed exact legacy criterion mapping")
				continue
			}
			mapped[mapping.Criterion] = true
			covered[hashValue([]string{legacy.ID, criterion.ID})] = true
			retained := false
			for _, assertion := range test.Assertions {
				if assertion.Criterion == mapping.Criterion && assertion.Matcher == criterion.Matcher && assertion.Locator == criterion.Locator && assertion.Value == criterion.Value && behaviorAssertionValid(r, test, assertion) {
					retained = true
				}
			}
			if mapping.Relation != "same" && mapping.Relation != "stronger" {
				valid = false
				c.behaviorGap(test.ID, "legacy-nonparity", mapping.Relation+" is a reviewed classification, not runtime parity")
			} else if !retained {
				valid = false
				c.behaviorGap(test.ID, "legacy-criterion-contradiction", "same/stronger target drops original observable result")
			}
			if !runtime[legacy.ID] {
				valid = false
			}
			if !slices.Equal(test.Fixtures, legacy.Fixtures) || !slices.Equal(test.Roles, legacy.Roles) {
				valid = false
				c.behaviorGap(test.ID, "legacy-precondition-mismatch", "fixture/role preconditions differ from legacy baseline")
			}
		}
		for _, criterion := range test.Criteria {
			if !mapped[criterion] {
				valid = false
				c.behaviorGap(test.ID, "unreviewed-legacy-join", "target criterion lacks specific legacy criterion relation: "+criterion)
			}
		}
		eligible[test.ID] = valid
	}
	complete := len(r.Legacy) > 0
	for _, legacy := range r.Legacy {
		if len(legacy.Criteria) == 0 {
			complete = false
			c.behaviorGap(legacy.ID, "unreviewed-legacy-join", "legacy case has no extracted assertion criteria")
		}
		for _, criterion := range legacy.Criteria {
			if !covered[hashValue([]string{legacy.ID, criterion.ID})] {
				complete = false
				c.behaviorGap(legacy.ID, "missing-legacy-criterion", "legacy branch lacks a target mapping: "+criterion.ID)
			}
		}
	}
	// A consolidated test cannot claim migration parity while another legacy branch disappears.
	if complete {
		for _, test := range r.Tests {
			if eligible[test.ID] {
				report.LegacyRuntimeParity = append(report.LegacyRuntimeParity, test.ID)
			}
		}
	}
}

func (c *compiler) behaviorLegacyRuntime(r BehaviorRegistry, legacy BehaviorLegacyCase) bool {
	if legacy.State != "executable" || legacy.Runtime == nil || len(legacy.Criteria) == 0 || !textOK(legacy.RuntimeTestID) || !textOK(legacy.RuntimeProject) {
		return false
	}
	a := legacy.Runtime.Evidence
	source, ok := c.sources[inputKey(a.Revision, a.Path)]
	var run BehaviorLegacyRun
	if !ok || a.Kind != "observed" || a.Start != 1 || a.SpanSHA256 != Digest(source.Data) || decode(source.Data, &run) != nil || run.TestID != legacy.RuntimeTestID || run.Project != legacy.RuntimeProject || run.Schema != "corvint-legacy-behavior-run/1" || run.ContractSHA256 != r.ContractSHA256 || run.Revisions != r.Revisions || run.CaseID != legacy.ID || run.SourceSHA256 != legacy.Evidence.SHA256 || run.Retry < 0 || run.Cleanup != "passed" || !slices.Equal(run.Fixtures, legacy.Fixtures) || !slices.Equal(run.Roles, legacy.Roles) || !slices.Equal(run.Criteria, legacy.Criteria) {
		return false
	}
	for _, o := range c.artifact.Observations {
		if o.Link.ID != legacy.Runtime.Observation || o.Link.SourceRevision != legacy.Evidence.Revision || o.Link.TestID != run.TestID || o.Link.Project != run.Project || o.InputSHA256 != run.RunSHA256 || !c.behaviorNativeReady(o) {
			continue
		}
		bound := false
		for _, native := range o.Document.Playwright.Tests {
			if native.ID != run.TestID || native.Anchor == nil {
				continue
			}
			path := native.Anchor.File
			if mapped, ok := o.Link.SourcePaths[path]; ok {
				path = mapped
			}
			if path == legacy.Evidence.Path && o.Document.Playwright.Identity.TestFileDigests[native.Anchor.File] == legacy.Evidence.SHA256 && native.Anchor.Line >= legacy.Evidence.Start && native.Anchor.Line <= legacy.Evidence.End {
				bound = true
			}
		}
		if !bound {
			continue
		}
		for _, observed := range o.Document.Tests {
			if observed.ID != run.TestID || observed.Project == nil || observed.Project.Name != run.Project || observed.Name != legacy.Case || observed.State != "passed" || observed.Projection.Execution.State != "PASSED" {
				continue
			}
			for _, attempt := range observed.Attempts {
				if attempt.Retry == run.Retry && attempt.State == "passed" {
					return true
				}
			}
		}
	}
	return false
}
