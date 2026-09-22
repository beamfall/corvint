package tcq

import (
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// Resolved carries the two OIDs the shared CEM/OCM verifier resolved from the
// independent invocation inputs. TCQ-V0-036 binds these, never values learned
// from an artifact.
type Resolved struct {
	BaseRevision   string
	TargetRevision string
}

// UpstreamVerifier is the current OCM verifier, which in turn invokes the shared
// CEM 0.2 canonical verifier. TCQ-V0-001 requires the same bounded raw byte
// copies and both revision inputs to pass through it, and TCQ-V0-042 forbids
// reordering any validation internal to it — hence a seam rather than a reimplementation.
type UpstreamVerifier interface {
	VerifyOCM(cemRaw, ocmRaw []byte, expectedBase, target string) (Resolved, error)
}

// Request is one TCQ invocation. Every artifact is caller-supplied immutable
// bytes (TCQ-V0-044); TCQ opens no raw-artifact filesystem path.
type Request struct {
	CEM          []byte
	OCM          []byte
	ExpectedBase string
	Target       string

	// The dynamic tuple is all-or-none (TCQ-V0-033). Any other combination is
	// `invalid-tcq-input`.
	Command     []byte
	Observation []byte
	Report      []byte

	// Expected optionally verifies a cached result byte-for-byte (TCQ-V0-043).
	Expected []byte

	// PriorObservations are earlier observations of the same target for the
	// TCQ-V0-050 flake rule. They require the dynamic tuple, bind the resolved
	// target, and can only remove a relation, never add one.
	PriorObservations [][]byte
}

// Evaluate computes one canonical TCQ result. Operational validation stops at
// the first failing stage of TCQ-V0-042 and emits no partial artifact.
func Evaluate(repository Repository, verifier UpstreamVerifier, request Request) (Result, error) {
	inputs, err := copyInputs(request)
	if err != nil {
		return Result{}, err
	}
	document, err := parseArtifacts(inputs)
	if err != nil {
		return Result{}, err
	}
	if request.ExpectedBase == "" {
		return Result{}, fail(CodeExpectedBaseRequired)
	}
	if request.Target == "" {
		return Result{}, fail(CodeTargetRequired)
	}
	resolved, err := verifier.VerifyOCM(inputs.cem, inputs.ocm, request.ExpectedBase, request.Target)
	if err != nil {
		return Result{}, err
	}
	if err := bindRevisions(document, resolved); err != nil {
		return Result{}, err
	}
	dynamic, err := verifyDynamic(repository, inputs, document, resolved)
	if err != nil {
		return Result{}, err
	}
	return assemble(repository, document, dynamic, resolved, inputs.expected)
}

// rawInputs holds the one private copy of each artifact TCQ-V0-044 requires;
// every hash, parse, comparison, and traversal reads only these bytes.
type rawInputs struct {
	cem         []byte
	ocm         []byte
	command     []byte
	observation []byte
	report      []byte
	expected    []byte
	priors      [][]byte
}

func copyInputs(request Request) (rawInputs, error) {
	if err := checkDynamicShape(request); err != nil {
		return rawInputs{}, err
	}
	inputs := rawInputs{}
	var err error
	if inputs.cem, err = copyBounded(request.CEM, cemBounds.bytes, true); err != nil {
		return rawInputs{}, err
	}
	if inputs.ocm, err = copyBounded(request.OCM, maxOCMBytes, true); err != nil {
		return rawInputs{}, err
	}
	if inputs.command, err = copyBounded(request.Command, maxCommandBytes, false); err != nil {
		return rawInputs{}, err
	}
	if inputs.observation, err = copyBounded(request.Observation, maxObservationBytes, false); err != nil {
		return rawInputs{}, err
	}
	if inputs.report, err = copyBounded(request.Report, maxReportBytes, false); err != nil {
		return rawInputs{}, err
	}
	if inputs.expected, err = copyBounded(request.Expected, maxResultBytes, false); err != nil {
		return rawInputs{}, err
	}
	inputs.priors, err = copyPriors(request.PriorObservations)
	return inputs, err
}

// copyPriors bounds the TCQ-V0-050 prior set: at most maxPriorObservations,
// each a present artifact under the observation byte ceiling.
func copyPriors(priors [][]byte) ([][]byte, error) {
	if len(priors) > maxPriorObservations {
		return nil, fail(CodeResourceExhausted)
	}
	copies := make([][]byte, 0, len(priors))
	for _, prior := range priors {
		copied, err := copyBounded(prior, maxObservationBytes, true)
		if err != nil {
			return nil, err
		}
		copies = append(copies, copied)
	}
	return copies, nil
}

// checkDynamicShape enforces the exact TCQ-V0-033 combination table: all three
// dynamic artifacts, or none.
func checkDynamicShape(request Request) error {
	present := 0
	for _, artifact := range [][]byte{request.Command, request.Observation, request.Report} {
		if artifact != nil {
			present++
		}
	}
	if present != 0 && present != 3 {
		return fail(CodeInvalidInput)
	}
	if present == 0 && len(request.PriorObservations) > 0 {
		return fail(CodeInvalidInput)
	}
	if request.CEM == nil || request.OCM == nil {
		return fail(CodeInvalidInput)
	}
	return nil
}

func copyBounded(value []byte, limit int, required bool) ([]byte, error) {
	if value == nil {
		if required {
			return nil, fail(CodeInvalidInput)
		}
		return nil, nil
	}
	if len(value) > limit {
		return nil, fail(CodeResourceExhausted)
	}
	return append([]byte(nil), value...), nil
}

// parsedDocuments is the TCQ-V0-042 step-3 outcome, in the current
// OCM-consuming precedence: OCM before CEM, then command, observation, and the
// optional cached result.
type parsedDocuments struct {
	ocm         ocmDocument
	command     *command
	observation *observation
	priors      []observation
}

func parseArtifacts(inputs rawInputs) (parsedDocuments, error) {
	if err := preflightArtifacts(inputs); err != nil {
		return parsedDocuments{}, err
	}
	document := parsedDocuments{}
	ocm, err := readOCM(inputs.ocm)
	if err != nil {
		return parsedDocuments{}, err
	}
	document.ocm = ocm
	if err := checkCEMProfile(inputs.cem); err != nil {
		return parsedDocuments{}, err
	}
	if inputs.command != nil {
		parsed, err := parseCommand(inputs.command)
		if err != nil {
			return parsedDocuments{}, err
		}
		document.command = &parsed
		observed, err := parseObservation(inputs.observation)
		if err != nil {
			return parsedDocuments{}, err
		}
		document.observation = &observed
	}
	for _, prior := range inputs.priors {
		observed, err := parseObservation(prior)
		if err != nil {
			return parsedDocuments{}, err
		}
		document.priors = append(document.priors, observed)
	}
	if inputs.expected != nil {
		if _, err := parseCanonical(inputs.expected, resultBounds, CodeInvalidTCQ, CodeNoncanonicalTCQ); err != nil {
			return parsedDocuments{}, err
		}
	}
	return document, nil
}

// preflightArtifacts completes TCQ-V0-042 stage 2 for every present artifact
// before stage 3 strictly parses any of them, so a depth or member ceiling on a
// later artifact outranks a strict-parse refusal of an earlier one.
func preflightArtifacts(inputs rawInputs) error {
	checks := []struct {
		raw    []byte
		bounds jsonBounds
	}{
		{inputs.ocm, ocmBounds},
		{inputs.cem, cemBounds},
		{inputs.command, commandBounds},
		{inputs.observation, observationBounds},
		{inputs.expected, resultBounds},
	}
	for _, prior := range inputs.priors {
		checks = append(checks, struct {
			raw    []byte
			bounds jsonBounds
		}{prior, observationBounds})
	}
	for _, check := range checks {
		if err := preflightJSON(check.raw, check.bounds); err != nil {
			return err
		}
	}
	return nil
}

// checkCEMProfile implements TCQ-V0-002: legacy `cem/0.1` is not a TCQ input. It
// fails operationally and can produce neither an abstention nor a relation.
func checkCEMProfile(raw []byte) error {
	if err := preflightJSON(raw, cemBounds); err != nil {
		return err
	}
	value, err := wire.Parse(raw)
	if err != nil || value.Kind != wire.KindObject {
		return fail(CodeInvalidTCQ)
	}
	spec, ok := value.Obj.Get("spec")
	if !ok || spec.Kind != wire.KindString || spec.Str != CEMCanonicalSpec {
		return fail(CodeUnsupportedCEMProfile)
	}
	return nil
}

func bindRevisions(document parsedDocuments, resolved Resolved) error {
	if !oidPattern.MatchString(resolved.BaseRevision) || !oidPattern.MatchString(resolved.TargetRevision) {
		return fail(CodeInvalidTCQ)
	}
	if document.ocm.targetRevision != resolved.TargetRevision {
		return fail(CodeTargetMismatch)
	}
	return nil
}

// dynamicContext is the verified report projection, present only in dynamic
// mode, plus the TCQ-V0-050 flaky execution keys.
type dynamicContext struct {
	report *junitReport
	flaky  map[string]bool
}

// verifyDynamic runs TCQ-V0-042 stages 6 to 8: command target equality and
// target-tree cwd resolution, observation target and command binding, declared
// report byte-count and digest equality, then the strict JUnit traversal and
// observation-row equality.
func verifyDynamic(repository Repository, inputs rawInputs, document parsedDocuments, resolved Resolved) (dynamicContext, error) {
	if document.command == nil {
		return dynamicContext{}, nil
	}
	if document.command.targetRevision != resolved.TargetRevision {
		return dynamicContext{}, fail(CodeCommandTargetMismatch)
	}
	if err := resolveCwd(repository, *document.command, resolved.TargetRevision); err != nil {
		return dynamicContext{}, err
	}
	if document.observation.targetRevision != resolved.TargetRevision {
		return dynamicContext{}, fail(CodeObservationTargetMismatch)
	}
	if document.observation.commandID != document.command.id {
		return dynamicContext{}, fail(CodeObservationCommandMismatch)
	}
	for _, prior := range document.priors {
		if prior.targetRevision != resolved.TargetRevision {
			return dynamicContext{}, fail(CodeObservationTargetMismatch)
		}
	}
	if document.observation.reportBytes != int64(len(inputs.report)) ||
		document.observation.reportSHA256 != sha256Hex(inputs.report) {
		return dynamicContext{}, fail(CodeReportDigestMismatch)
	}
	report, err := parseJUnit(repository, resolved.TargetRevision, inputs.report)
	if err != nil {
		return dynamicContext{}, err
	}
	if document.observation.exitCode == 0 && report.hasNonPassing {
		return dynamicContext{}, fail(CodeReportCommandInconsistent)
	}
	if err := checkObservationRows(*document.observation, report); err != nil {
		return dynamicContext{}, err
	}
	return dynamicContext{report: &report, flaky: flakyKeys(*document.observation, document.priors)}, nil
}

// resolveCwd implements TCQ-V0-023: `.` denotes the target root; every other
// cwd must resolve inside the target Git tree and the final object must be a
// tree. Worktree filesystem directories are never cwd authority.
func resolveCwd(repository Repository, current command, target string) error {
	if current.cwd == "." {
		return nil
	}
	entry, err := repository.TreeEntry(target, current.cwd)
	if err != nil {
		return err // a Git lookup failure keeps its own meaning (TCQ-V0-042)
	}
	if !entry.Found || entry.Mode != treeMode || entry.Type != "tree" {
		return fail(CodeCommandCwdUnavailable)
	}
	return nil
}

// checkObservationRows re-derives every declared row from the raw report
// (TCQ-V0-032). An altered or reordered row set is an invalid observation.
func checkObservationRows(observed observation, report junitReport) error {
	derived := observationRows(report)
	if len(derived) != len(observed.rows) || observed.unkeyedRows != int64(report.unkeyedCount) {
		return fail(CodeInvalidObservation)
	}
	for index := range derived {
		if string(canonicalValue(derived[index])) != string(canonicalValue(observed.rows[index])) {
			return fail(CodeInvalidObservation)
		}
	}
	return nil
}
