package doccorpus

import (
	"slices"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

const (
	StabilitySchema                 = "corvint-playwright-stability/1"
	BehaviorStabilityProviderSchema = "corvint-corpus-behavior-stability-provider/1"
)

type StabilityRegistry struct {
	Schema     string               `json:"schema"`
	Policy     StabilityPolicy      `json:"policy"`
	Aggregates []StabilityAggregate `json:"aggregates"`
}

type StabilityPolicy struct {
	ID               string               `json:"id"`
	SHA256           string               `json:"sha256"`
	MatrixDimensions []string             `json:"matrix_dimensions"`
	Thresholds       []StabilityThreshold `json:"thresholds"`
}

type StabilityThreshold struct {
	Scope                       string `json:"scope"`
	RequiredRepetitions         int    `json:"required_repetitions"`
	MinimumPassed               int    `json:"minimum_passed"`
	MaximumFailed               int    `json:"maximum_failed"`
	MaximumTimedOut             int    `json:"maximum_timed_out"`
	MaximumInterrupted          int    `json:"maximum_interrupted"`
	MaximumInfrastructureFailed int    `json:"maximum_infrastructure_failed"`
	MaximumSkipped              int    `json:"maximum_skipped"`
	MaximumFlaky                int    `json:"maximum_flaky"`
	MaximumRetryConsumed        int    `json:"maximum_retry_consumed"`
}

type StabilityAggregate struct {
	ID             string                  `json:"id"`
	Scope          string                  `json:"scope"`
	PolicyID       string                  `json:"policy_id"`
	Planned        int                     `json:"planned"`
	TestID         string                  `json:"test_id"`
	Project        string                  `json:"project"`
	ContractID     string                  `json:"contract_id"`
	ContractSHA256 string                  `json:"contract_sha256"`
	Contributions  []StabilityContribution `json:"contributions"`
}

type StabilityReceiptInput struct {
	Path     string `json:"path"`
	Revision string `json:"revision"`
	SHA256   string `json:"sha256"`
}

type StabilityIdentity struct {
	ApplicationRevision string `json:"application_revision"`
	TestRevision        string `json:"test_revision"`
	ConfigSHA256        string `json:"config_sha256"`
	ContractSHA256      string `json:"contract_sha256"`
	Runner              string `json:"runner"`
	RunnerVersion       string `json:"runner_version"`
	Browser             string `json:"browser"`
	Project             string `json:"project"`
	WorkerPolicy        string `json:"worker_policy"`
	RetryPolicy         string `json:"retry_policy"`
	EnvironmentClass    string `json:"environment_class"`
	EnvironmentSHA256   string `json:"environment_sha256"`
	FixtureSchema       string `json:"fixture_schema"`
	FixtureSHA256       string `json:"fixture_sha256"`
}

type StabilityContribution struct {
	RunKind     string                `json:"run_kind"`
	Repetition  int                   `json:"repetition"`
	ManualRunID string                `json:"manual_run_id,omitempty"`
	Receipt     StabilityReceiptInput `json:"receipt"`
	SourcePaths map[string]string     `json:"source_paths"`
	Identity    StabilityIdentity     `json:"identity"`
	Cleanup     string                `json:"cleanup"`
	Attempts    []StabilityAttempt    `json:"attempts"`
}

type StabilityAttempt struct {
	Retry        int                              `json:"retry"`
	State        string                           `json:"state"`
	FailureClass string                           `json:"failure_class"`
	Artifacts    []jstestprovider.FailureArtifact `json:"artifacts"`
	Cleanup      string                           `json:"cleanup"`
}

type StabilityCounts struct {
	Planned              int `json:"planned"`
	Started              int `json:"started"`
	Completed            int `json:"completed"`
	Passed               int `json:"passed"`
	Failed               int `json:"failed"`
	TimedOut             int `json:"timed_out"`
	Interrupted          int `json:"interrupted"`
	InfrastructureFailed int `json:"infrastructure_failed"`
	Skipped              int `json:"skipped"`
	Flaky                int `json:"flaky"`
	RetryConsumed        int `json:"retry_consumed"`
	CleanupFailed        int `json:"cleanup_failed"`
	ManualReruns         int `json:"manual_reruns"`
}

type StabilityReport struct {
	ID                   string                  `json:"id"`
	Provider             string                  `json:"provider"`
	Scope                string                  `json:"scope"`
	PolicyID             string                  `json:"policy_id"`
	PolicySHA256         string                  `json:"policy_sha256"`
	MatrixDimensions     []string                `json:"matrix_dimensions"`
	TestID               string                  `json:"test_id"`
	Project              string                  `json:"project"`
	ContractID           string                  `json:"contract_id"`
	ContractSHA256       string                  `json:"contract_sha256"`
	Counts               StabilityCounts         `json:"counts"`
	Verdict              string                  `json:"verdict"`
	ContributingReceipts []StabilityContribution `json:"contributing_receipts"`
	Limitations          []string                `json:"limitations"`
}

func (c *compiler) compileStability(provider string, behavior BehaviorRegistry, registry StabilityRegistry) error {
	if registry.Schema != StabilitySchema || !textOK(registry.Policy.ID) || len(registry.Aggregates) == 0 || len(registry.Aggregates) > MaxRecords {
		return fail("invalid stability registry")
	}
	policy := registry.Policy
	policy.SHA256 = ""
	if !wireDigest(registry.Policy.SHA256) || hashValue(policy) != registry.Policy.SHA256 {
		return fail("stability policy digest mismatch")
	}
	allowedDimensions := words("application_revision test_revision config contract runner runner_version browser project worker_policy retry_policy environment_class environment fixture_schema fixture")
	if !uniqueIdentities(registry.Policy.MatrixDimensions) {
		return fail("invalid stability matrix dimensions")
	}
	for _, dimension := range registry.Policy.MatrixDimensions {
		if !allowedDimensions[dimension] {
			return fail("invalid stability matrix dimension")
		}
	}
	thresholds := map[string]StabilityThreshold{}
	for _, threshold := range registry.Policy.Thresholds {
		if !words("one-spec feature-batch suite")[threshold.Scope] || threshold.RequiredRepetitions < 1 || threshold.MinimumPassed < 0 || threshold.MinimumPassed > threshold.RequiredRepetitions || negativeThreshold(threshold) {
			return fail("invalid stability threshold")
		}
		if _, exists := thresholds[threshold.Scope]; exists {
			return fail("duplicate stability threshold")
		}
		thresholds[threshold.Scope] = threshold
	}
	for _, scope := range []string{"one-spec", "feature-batch", "suite"} {
		if _, exists := thresholds[scope]; !exists {
			return fail("stability policy scope missing")
		}
	}
	seenAggregates := map[string]bool{}
	for _, aggregate := range registry.Aggregates {
		if seenAggregates[aggregate.ID] || !textOK(aggregate.ID) || aggregate.PolicyID != registry.Policy.ID || aggregate.ContractID != behavior.ContractID || aggregate.ContractSHA256 != behavior.ContractSHA256 || !textOK(aggregate.TestID) || !textOK(aggregate.Project) {
			return fail("invalid stability aggregate identity")
		}
		seenAggregates[aggregate.ID] = true
		threshold, exists := thresholds[aggregate.Scope]
		if !exists || aggregate.Planned != threshold.RequiredRepetitions || aggregate.Planned > MaxRecords || len(aggregate.Contributions) > MaxRecords {
			return fail("stability aggregate policy mismatch")
		}
		report, err := c.aggregateStability(provider, behavior, registry.Policy, threshold, aggregate)
		if err != nil {
			return err
		}
		c.artifact.StabilityEvidence = append(c.artifact.StabilityEvidence, report)
	}
	return nil
}

func wireDigest(value string) bool {
	return wire.IsSha256(value)
}

func negativeThreshold(t StabilityThreshold) bool {
	return t.MaximumFailed < 0 || t.MaximumTimedOut < 0 || t.MaximumInterrupted < 0 || t.MaximumInfrastructureFailed < 0 || t.MaximumSkipped < 0 || t.MaximumFlaky < 0 || t.MaximumRetryConsumed < 0
}

func (c *compiler) aggregateStability(provider string, behavior BehaviorRegistry, policy StabilityPolicy, threshold StabilityThreshold, aggregate StabilityAggregate) (StabilityReport, error) {
	counts := StabilityCounts{Planned: aggregate.Planned}
	planned := map[int]bool{}
	manual := map[string]bool{}
	receipts := map[string]bool{}
	var baseline *StabilityIdentity
	for i := range aggregate.Contributions {
		contribution := &aggregate.Contributions[i]
		if contribution.RunKind == "planned-repetition" && contribution.Repetition == 1 {
			copy := contribution.Identity
			baseline = &copy
			break
		}
	}
	if baseline == nil {
		return StabilityReport{}, fail("stability planned repetition set incomplete")
	}
	for i := range aggregate.Contributions {
		contribution := &aggregate.Contributions[i]
		if receipts[contribution.Receipt.SHA256] {
			return StabilityReport{}, fail("duplicate stability receipt")
		}
		receipts[contribution.Receipt.SHA256] = true
		plannedRun := contribution.RunKind == "planned-repetition"
		if plannedRun {
			if contribution.Repetition < 1 || contribution.Repetition > aggregate.Planned || contribution.ManualRunID != "" || planned[contribution.Repetition] {
				return StabilityReport{}, fail("invalid stability repetition")
			}
			planned[contribution.Repetition] = true
			counts.Started++
		} else if contribution.RunKind == "manual-rerun" {
			if contribution.Repetition != 0 || !textOK(contribution.ManualRunID) || manual[contribution.ManualRunID] {
				return StabilityReport{}, fail("invalid stability manual rerun")
			}
			manual[contribution.ManualRunID] = true
			counts.ManualReruns++
		} else {
			return StabilityReport{}, fail("invalid stability run kind")
		}
		if contribution.Identity.Project != aggregate.Project && !slices.Contains(policy.MatrixDimensions, "project") {
			return StabilityReport{}, fail("stability aggregate project mismatch")
		}
		outcome, receipt, err := c.stabilityOutcome(behavior, aggregate, *contribution)
		if err != nil {
			return StabilityReport{}, err
		}
		if !stabilityIdentityMatches(*baseline, contribution.Identity, policy.MatrixDimensions) {
			return StabilityReport{}, fail("cross-identity stability receipt")
		}
		if plannedRun {
			countStabilityOutcome(&counts, outcome, receipt, *contribution)
		} else if stabilityCleanupFailed(*contribution) {
			counts.CleanupFailed++
		}
	}
	if len(planned) != aggregate.Planned {
		return StabilityReport{}, fail("stability planned repetition set incomplete")
	}
	verdict := "not-stable"
	if stabilityThresholdPassed(counts, threshold) {
		verdict = "clean"
	}
	return StabilityReport{ID: aggregate.ID, Provider: provider, Scope: aggregate.Scope, PolicyID: policy.ID, PolicySHA256: policy.SHA256, MatrixDimensions: slices.Clone(policy.MatrixDimensions), TestID: aggregate.TestID, Project: aggregate.Project, ContractID: aggregate.ContractID, ContractSHA256: aggregate.ContractSHA256, Counts: counts, Verdict: verdict, ContributingReceipts: aggregate.Contributions, Limitations: []string{"stability is repeated-run evidence, not test adequacy or behavior parity", "worker, retry, environment-class and fixture-schema labels are repository-owned declarations bound by the policy and receipt digest", "manual reruns are retained but never satisfy planned repetition thresholds"}}, nil
}

func (c *compiler) stabilityOutcome(behavior BehaviorRegistry, aggregate StabilityAggregate, contribution StabilityContribution) (testvaliditydoc.Test, *jstestprovider.Receipt, error) {
	input := contribution.Receipt
	if !validPath(input.Path) || !wire.IsGitOid(input.Revision) || !wireDigest(input.SHA256) {
		return testvaliditydoc.Test{}, nil, fail("invalid stability receipt identity")
	}
	source, exists := c.sources[inputKey(input.Revision, input.Path)]
	if !exists || Digest(source.Data) != input.SHA256 {
		return testvaliditydoc.Test{}, nil, fail("stale stability receipt")
	}
	decoded, err := testvaliditydoc.Decode(source.Data)
	if err != nil {
		return testvaliditydoc.Test{}, nil, fail("invalid stability receipt")
	}
	document := testvaliditydoc.Project(decoded)
	if document.Playwright == nil || document.TestsOmitted != 0 || document.Run.Execution.State == testvalidity.ExecutionInfrastructure || document.Run.Execution.State == testvalidity.ExecutionCancelled || document.Run.Freshness.State == testvalidity.FreshnessStale || document.Playwright.Cancelled || document.Playwright.StaleAppBuild || document.Playwright.External == nil || !document.Playwright.External.InputsUnchanged {
		return testvaliditydoc.Test{}, nil, fail("partial or stale stability receipt")
	}
	matches := []testvaliditydoc.Test{}
	for _, test := range document.Tests {
		if test.ID == aggregate.TestID && test.Project != nil && test.Project.Name == contribution.Identity.Project {
			matches = append(matches, test)
		}
	}
	if len(matches) != 1 {
		return testvaliditydoc.Test{}, nil, fail("stability test/project join is not exact")
	}
	test := matches[0]
	receipt := document.Playwright
	native, exact := stabilityNativeOutcome(*receipt, aggregate.TestID, contribution.Identity.Project)
	if !exact || !jstestprovider.QualifiedReceiptBindingReady(*receipt, *native) || !stabilityProjectionMatches(*native) {
		return testvaliditydoc.Test{}, nil, fail("unqualified stability test outcome")
	}
	identity := contribution.Identity
	_, applicationRevisionDeclared := c.indexes[identity.ApplicationRevision]
	if !applicationRevisionDeclared || !wire.IsGitOid(identity.TestRevision) || !wire.IsSha256(identity.ConfigSHA256) || !wire.IsSha256(identity.ContractSHA256) || identity.ApplicationRevision != receipt.External.DeclaredAppIdentity || identity.TestRevision != behavior.SourceRevision || identity.ConfigSHA256 != receipt.Identity.ConfigDigest || identity.ContractSHA256 != behavior.ContractSHA256 || identity.Runner != receipt.Identity.RunnerName || identity.RunnerVersion != receipt.Identity.RunnerVersion || identity.Browser != test.Project.Browser || identity.Project != test.Project.Name || identity.EnvironmentSHA256 != hashValue(receipt.Identity.Environment) || identity.FixtureSHA256 != Digest(test.Project.Use) {
		return testvaliditydoc.Test{}, nil, fail("contradictory stability receipt identity")
	}
	for _, value := range []string{identity.WorkerPolicy, identity.RetryPolicy, identity.EnvironmentClass, identity.FixtureSchema} {
		if !textOK(value) {
			return testvaliditydoc.Test{}, nil, fail("incomplete stability receipt identity")
		}
	}
	if !c.stabilityInputsBound(identity.TestRevision, contribution.SourcePaths, receipt.Identity) || !stabilityBehaviorTestBound(behavior, contribution, test, *receipt, *native) {
		return testvaliditydoc.Test{}, nil, fail("stability test/config source binding mismatch")
	}
	if !words("passed failed unknown")[contribution.Cleanup] || len(contribution.Attempts) != len(test.Attempts) || len(contribution.Attempts) == 0 {
		return testvaliditydoc.Test{}, nil, fail("stability cleanup or attempt evidence incomplete")
	}
	for i, attempt := range test.Attempts {
		if attempt.Retry != i {
			return testvaliditydoc.Test{}, nil, fail("noncontiguous stability retry ordinal")
		}
		if err := validateStabilityAttempt(contribution.Attempts[i], attempt, receipt.Tests, aggregate.TestID); err != nil {
			return testvaliditydoc.Test{}, nil, err
		}
	}
	return test, receipt, nil
}

func stabilityNativeOutcome(receipt jstestprovider.Receipt, testID, project string) (*jstestprovider.TestOutcome, bool) {
	var matched *jstestprovider.TestOutcome
	for i := range receipt.Tests {
		candidate := &receipt.Tests[i]
		if candidate.ID == testID && candidate.Project != nil && candidate.Project.Name == project {
			if matched != nil {
				return nil, false
			}
			matched = candidate
		}
	}
	return matched, matched != nil
}

func stabilityProjectionMatches(outcome jstestprovider.TestOutcome) bool {
	wantState, wantReason := "", ""
	switch outcome.State {
	case jstestprovider.StatePassed, jstestprovider.StateFlaky:
		wantState = testvalidity.ExecutionPassed
	case jstestprovider.StateFailed:
		wantState = testvalidity.ExecutionFailed
	case jstestprovider.StateSkipped:
		wantState = testvalidity.ExecutionSkipped
	case jstestprovider.StateTimedOut:
		wantState, wantReason = testvalidity.ExecutionInfrastructure, "TIMEOUT"
	case jstestprovider.StateInterrupted:
		wantState, wantReason = testvalidity.ExecutionCancelled, "CANCELLATION"
	case jstestprovider.StateInfrastructure:
		wantState, wantReason = testvalidity.ExecutionInfrastructure, "INFRASTRUCTURE"
	default:
		return false
	}
	projection := jstestprovider.ToTestProjection(outcome)
	return projection.Execution.State == wantState && (wantReason == "" || projection.Execution.Reason == wantReason)
}

func (c *compiler) stabilityInputsBound(revision string, sourcePaths map[string]string, identity jstestprovider.Identity) bool {
	if len(sourcePaths) == 0 || len(sourcePaths) > MaxRecords {
		return false
	}
	for original, mapped := range sourcePaths {
		if !textOK(original) || !validPath(mapped) {
			return false
		}
	}
	inputs := map[string]string{}
	for path, digest := range identity.TestFileDigests {
		inputs[path] = digest
	}
	for path, digest := range identity.ConfigInputDigests {
		inputs[path] = digest
	}
	inputs[identity.ConfigFile] = identity.ConfigDigest
	if len(inputs) == 0 {
		return false
	}
	for original, digest := range inputs {
		mapped, ok := sourcePaths[original]
		if !ok {
			return false
		}
		source, ok := c.sources[inputKey(revision, mapped)]
		if !ok || Digest(source.Data) != digest {
			return false
		}
	}
	return true
}

func stabilityBehaviorTestBound(behavior BehaviorRegistry, contribution StabilityContribution, observed testvaliditydoc.Test, receipt jstestprovider.Receipt, native jstestprovider.TestOutcome) bool {
	var declared *BehaviorTest
	for i := range behavior.Tests {
		if behavior.Tests[i].ID == native.ID {
			if declared != nil {
				return false
			}
			declared = &behavior.Tests[i]
		}
	}
	if declared == nil || observed.Project == nil || declared.Project != observed.Project.Name || declared.Title != observed.Name || declared.Evidence.Revision != contribution.Identity.TestRevision {
		return false
	}
	if native.Anchor == nil {
		return false
	}
	mapped, ok := contribution.SourcePaths[native.Anchor.File]
	return ok && mapped == declared.Evidence.Path && receipt.Identity.TestFileDigests[native.Anchor.File] == declared.Evidence.SHA256 && native.Anchor.Line >= declared.Evidence.Start && native.Anchor.Line <= declared.Evidence.End
}

func validateStabilityAttempt(evidence StabilityAttempt, native jstestprovider.Attempt, outcomes []jstestprovider.TestOutcome, testID string) error {
	if evidence.Retry != native.Retry || evidence.State != string(native.State) || !words("passed failed unknown")[evidence.Cleanup] {
		return fail("contradictory stability attempt evidence")
	}
	classes := map[string]bool{}
	switch native.State {
	case jstestprovider.StatePassed, jstestprovider.StateSkipped:
		classes["none"] = native.FailureKind == "" || native.FailureKind == "none"
	case jstestprovider.StateFailed:
		for _, class := range []string{"assertion", "synchronization", "product"} {
			classes[class] = native.FailureKind == "assertion-or-test"
		}
	case jstestprovider.StateTimedOut:
		classes["timeout"] = native.FailureKind == "test-timeout"
	case jstestprovider.StateInterrupted:
		classes["interruption"] = native.FailureKind == "" || native.FailureKind == "none"
	case jstestprovider.StateInfrastructure:
		classes["fixture"] = native.FailureKind == "browser-or-fixture"
		classes["infrastructure"] = native.FailureKind == "browser-or-fixture"
	}
	if !classes[evidence.FailureClass] {
		return fail("contradictory stability failure classification")
	}
	if evidence.FailureClass == "none" {
		if len(evidence.Artifacts) != 0 {
			return fail("passing stability attempt carries failure artifacts")
		}
		return nil
	}
	if len(evidence.Artifacts) == 0 {
		return fail("stability failure lacks supporting artifact")
	}
	var nativeArtifacts []jstestprovider.FailureArtifact
	for _, outcome := range outcomes {
		if outcome.ID == testID {
			nativeArtifacts = outcome.Artifacts
		}
	}
	for _, artifact := range evidence.Artifacts {
		if !slices.Contains(nativeArtifacts, artifact) {
			return fail("stability failure artifact is not receipt-bound")
		}
	}
	return nil
}

func stabilityIdentityMatches(left, right StabilityIdentity, matrix []string) bool {
	dimensions := map[string]bool{}
	for _, dimension := range matrix {
		dimensions[dimension] = true
	}
	checks := []struct {
		dimension string
		equal     bool
	}{
		{"application_revision", left.ApplicationRevision == right.ApplicationRevision},
		{"test_revision", left.TestRevision == right.TestRevision},
		{"config", left.ConfigSHA256 == right.ConfigSHA256},
		{"contract", left.ContractSHA256 == right.ContractSHA256},
		{"runner", left.Runner == right.Runner},
		{"runner_version", left.RunnerVersion == right.RunnerVersion},
		{"browser", left.Browser == right.Browser},
		{"project", left.Project == right.Project},
		{"worker_policy", left.WorkerPolicy == right.WorkerPolicy},
		{"retry_policy", left.RetryPolicy == right.RetryPolicy},
		{"environment_class", left.EnvironmentClass == right.EnvironmentClass},
		{"environment", left.EnvironmentSHA256 == right.EnvironmentSHA256},
		{"fixture_schema", left.FixtureSchema == right.FixtureSchema},
		{"fixture", left.FixtureSHA256 == right.FixtureSHA256},
	}
	for _, check := range checks {
		if !check.equal && !dimensions[check.dimension] {
			return false
		}
	}
	return true
}

func countStabilityOutcome(counts *StabilityCounts, outcome testvaliditydoc.Test, receipt *jstestprovider.Receipt, contribution StabilityContribution) {
	failed := outcome.State == string(jstestprovider.StateFailed)
	timedOut := outcome.State == string(jstestprovider.StateTimedOut)
	interrupted := outcome.State == string(jstestprovider.StateInterrupted)
	skipped := outcome.State == string(jstestprovider.StateSkipped)
	infrastructureFailed := outcome.State == string(jstestprovider.StateInfrastructure) || receipt.Infrastructure != nil
	for _, attempt := range contribution.Attempts {
		failed = failed || attempt.State == string(jstestprovider.StateFailed)
		timedOut = timedOut || attempt.State == string(jstestprovider.StateTimedOut)
		interrupted = interrupted || attempt.State == string(jstestprovider.StateInterrupted)
		infrastructureFailed = infrastructureFailed || attempt.State == string(jstestprovider.StateInfrastructure) || attempt.FailureClass == "infrastructure"
		skipped = skipped || attempt.State == string(jstestprovider.StateSkipped)
	}
	if failed {
		counts.Failed++
	}
	if stabilityCleanupFailed(contribution) {
		counts.CleanupFailed++
	}
	if timedOut {
		counts.TimedOut++
	}
	if interrupted {
		counts.Interrupted++
	}
	if skipped {
		counts.Skipped++
	}
	if len(outcome.Attempts) > 1 {
		counts.RetryConsumed += len(outcome.Attempts) - 1
	}
	switch jstestprovider.ExecutionState(outcome.State) {
	case jstestprovider.StatePassed:
		counts.Completed++
		counts.Passed++
	case jstestprovider.StateFailed:
		counts.Completed++
	case jstestprovider.StateTimedOut:
	case jstestprovider.StateInterrupted:
	case jstestprovider.StateInfrastructure:
		counts.Completed++
	case jstestprovider.StateSkipped:
		counts.Completed++
	case jstestprovider.StateFlaky:
		counts.Completed++
		counts.Flaky++
	}
	if infrastructureFailed {
		counts.InfrastructureFailed++
	}
}

func stabilityCleanupFailed(contribution StabilityContribution) bool {
	if contribution.Cleanup != "passed" {
		return true
	}
	for _, attempt := range contribution.Attempts {
		if attempt.Cleanup != "passed" {
			return true
		}
	}
	return false
}

func stabilityThresholdPassed(counts StabilityCounts, threshold StabilityThreshold) bool {
	return counts.Planned == threshold.RequiredRepetitions && counts.Started == counts.Planned && counts.Completed >= counts.Planned && counts.Passed >= threshold.MinimumPassed && counts.Failed <= threshold.MaximumFailed && counts.TimedOut <= threshold.MaximumTimedOut && counts.Interrupted <= threshold.MaximumInterrupted && counts.InfrastructureFailed <= threshold.MaximumInfrastructureFailed && counts.Skipped <= threshold.MaximumSkipped && counts.Flaky <= threshold.MaximumFlaky && counts.RetryConsumed <= threshold.MaximumRetryConsumed && counts.CleanupFailed == 0
}
