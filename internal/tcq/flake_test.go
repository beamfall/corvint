package tcq

import (
	"bytes"
	"strings"
	"testing"
)

const failingBetaReport = `<testsuite name="s"><testcase file="pkg/sample_test.go" name="TestBeta"><failure/></testcase></testsuite>`

// TestFlakyRuleNeedsDivergentTerminalOutcomes pins TCQ-V0-049: only two distinct
// terminal statuses diverge; skips and repeats of one status never do.
func TestFlakyRuleNeedsDivergentTerminalOutcomes(t *testing.T) {
	cases := []struct {
		name     string
		statuses []string
		want     bool
	}{
		{"failed-then-passed", []string{ReportFailed, ReportPassed}, true},
		{"error-then-passed", []string{ReportError, ReportPassed}, true},
		{"passed-twice", []string{ReportPassed, ReportPassed}, false},
		{"failed-twice", []string{ReportFailed, ReportFailed}, false},
		{"skipped-then-passed", []string{ReportSkipped, ReportPassed}, false},
		{"single", []string{ReportFailed}, false},
		{"none", nil, false},
	}
	for _, testCase := range cases {
		if got := Flaky(testCase.statuses); got != testCase.want {
			t.Errorf("%s: Flaky = %v, want %v", testCase.name, got, testCase.want)
		}
	}
}

// TestObservationEnvironmentIsAdditive pins TCQ-V0-048: an undeclared variant
// leaves the observation and the result summary unchanged and reads as unknown;
// a declared one enters the observation identity and the summary; a malformed
// one is an invalid observation.
func TestObservationEnvironmentIsAdditive(t *testing.T) {
	documents := loadDocuments(t)
	repository := newFixtureRepository(documents)
	verifier := fixtureVerifier{base: documents.Base, target: documents.Target}
	command, report := []byte(documents.Command), []byte(documents.Report)
	plain, err := MakeTestObservation(repository, command, report, documents.Target, 0)
	if err != nil {
		t.Fatalf("MakeTestObservation: %v", err)
	}
	if !bytes.Equal(plain, []byte(documents.Observation)) {
		t.Fatal("an undeclared environment changed the frozen observation bytes")
	}
	declared, err := MakeTestObservationInEnvironment(repository, command, report, documents.Target, 0, map[string]string{"os": "linux", "go": "1.27.1"})
	if err != nil {
		t.Fatalf("MakeTestObservationInEnvironment: %v", err)
	}
	if bytes.Equal(declared, plain) {
		t.Fatal("a declared environment did not change the observation identity")
	}
	evaluate := func(observation []byte) Result {
		request := newRequest(documents)
		request.Command, request.Observation, request.Report = command, observation, report
		result, err := Evaluate(repository, verifier, request)
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		return result
	}
	if _, known := evaluate(plain).ObservationEnvironment(); known {
		t.Error("an undeclared environment was reported as known")
	}
	if strings.Contains(string(evaluate(plain).Raw()), `"environment"`) {
		t.Error("an undeclared environment emitted a summary member")
	}
	result := evaluate(declared)
	environment, known := result.ObservationEnvironment()
	if !known || environment["os"] != "linux" || environment["go"] != "1.27.1" {
		t.Errorf("declared environment = %v, %v", environment, known)
	}
	if !strings.Contains(string(result.Raw()), `"environment":{"go":"1.27.1","os":"linux"}`) {
		t.Error("the result summary did not copy the declared environment")
	}
	malformed := map[string]string{
		"bad-key-grammar": strings.Replace(string(declared), `"os":"linux"`, `"zz-bad":"linux"`, 1),
		"non-string":      strings.Replace(string(declared), `"os":"linux"`, `"os":1`, 1),
		"non-object":      strings.Replace(string(declared), `{"go":"1.27.1","os":"linux"}`, `["linux"]`, 1),
	}
	for name, observation := range malformed {
		request := newRequest(documents)
		request.Command, request.Observation, request.Report = command, []byte(observation), report
		_, err := Evaluate(repository, verifier, request)
		if err == nil {
			t.Errorf("%s: malformed environment was accepted", name)
			continue
		}
		requireCode(t, err, CodeInvalidObservation)
	}
}

// TestSameRevisionDivergentOutcomesAreFlaky pins TCQ-V0-050: a passing current
// observation plus a failing prior of the same test at the same target and
// variant yields `test-flaky` and no relation, while the same request without
// the prior keeps its relation.
func TestSameRevisionDivergentOutcomesAreFlaky(t *testing.T) {
	documents := loadDocuments(t)
	repository := newFixtureRepository(documents)
	verifier := fixtureVerifier{base: documents.Base, target: documents.Target}
	command := []byte(documents.Command)
	prior, err := MakeTestObservation(repository, command, []byte(failingBetaReport), documents.Target, 1)
	if err != nil {
		t.Fatalf("MakeTestObservation: %v", err)
	}
	request := newRequest(documents)
	request.Command, request.Observation, request.Report = command, []byte(documents.Observation), []byte(documents.Report)
	baseline, err := Evaluate(repository, verifier, request)
	if err != nil {
		t.Fatalf("Evaluate baseline: %v", err)
	}
	if !hasRelation(baseline) {
		t.Fatal("baseline without priors produced no relation")
	}
	request.PriorObservations = [][]byte{prior}
	result, err := Evaluate(repository, verifier, request)
	if err != nil {
		t.Fatalf("Evaluate with prior: %v", err)
	}
	if !hasClaimWithReason(result, ReasonTestFlaky) {
		t.Error("divergent outcomes at one revision did not produce test-flaky")
	}
	const beta = "claim:sha256:18fda1aa9f1a49890e1c7675b83b11924f451ed773ecb24e0c4482ac5b00391e"
	for _, claim := range result.Claims() {
		flaky := false
		for _, reason := range claim.Reasons {
			flaky = flaky || reason == ReasonTestFlaky
		}
		if flaky != (claim.ClaimID == beta) {
			t.Errorf("%s test-flaky = %v; only the divergent TestBeta claim may carry it", claim.ClaimID, flaky)
		}
		if flaky && claim.ReportState != ReportPassed {
			t.Errorf("flaky claim report state = %q, want the current row's %q", claim.ReportState, ReportPassed)
		}
		if flaky && claim.Relation != "" {
			t.Errorf("flaky claim kept relation %q", claim.Relation)
		}
	}
	if !bytes.Equal(result.Raw(), mustEvaluate(t, repository, verifier, request).Raw()) {
		t.Error("evaluation with priors is not deterministic")
	}
}

// TestPriorObservationVariantMismatchIsNotFlaky pins the TCQ-V0-050 comparison
// rule: a prior under another declared variant, or with a variant when the
// current observation has none, never contributes to a divergence.
func TestPriorObservationVariantMismatchIsNotFlaky(t *testing.T) {
	documents := loadDocuments(t)
	repository := newFixtureRepository(documents)
	verifier := fixtureVerifier{base: documents.Base, target: documents.Target}
	command := []byte(documents.Command)
	linux := map[string]string{"os": "linux"}
	darwin := map[string]string{"os": "darwin"}
	observe := func(report string, exitCode int64, environment map[string]string) []byte {
		observation, err := MakeTestObservationInEnvironment(repository, command, []byte(report), documents.Target, exitCode, environment)
		if err != nil {
			t.Fatalf("MakeTestObservationInEnvironment: %v", err)
		}
		return observation
	}
	cases := []struct {
		name    string
		current map[string]string
		prior   map[string]string
		flaky   bool
	}{
		{"same-variant", linux, linux, true},
		{"other-variant", linux, darwin, false},
		{"declared-vs-undeclared", nil, linux, false},
		{"undeclared-vs-declared", linux, nil, false},
		{"both-undeclared", nil, nil, true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := newRequest(documents)
			request.Command, request.Report = command, []byte(documents.Report)
			request.Observation = observe(documents.Report, 0, testCase.current)
			request.PriorObservations = [][]byte{observe(failingBetaReport, 1, testCase.prior)}
			result, err := Evaluate(repository, verifier, request)
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if got := hasClaimWithReason(result, ReasonTestFlaky); got != testCase.flaky {
				t.Errorf("test-flaky = %v, want %v", got, testCase.flaky)
			}
		})
	}
}

// TestPriorObservationsRequireDynamicTupleAndTarget pins the TCQ-V0-050 input
// bounds: priors without the dynamic tuple are invalid input, a prior at another
// target is a target mismatch, and more than maxPriorObservations exhausts.
func TestPriorObservationsRequireDynamicTupleAndTarget(t *testing.T) {
	documents := loadDocuments(t)
	repository := newFixtureRepository(documents)
	verifier := fixtureVerifier{base: documents.Base, target: documents.Target}
	command := []byte(documents.Command)
	prior, err := MakeTestObservation(repository, command, []byte(documents.Report), documents.Target, 0)
	if err != nil {
		t.Fatalf("MakeTestObservation: %v", err)
	}
	static := newRequest(documents)
	static.PriorObservations = [][]byte{prior}
	_, err = Evaluate(repository, verifier, static)
	requireCode(t, err, CodeInvalidInput)

	other := strings.Repeat("3", 40)
	otherCommand, err := MakeTestCommand([]string{"go", "test"}, ".", "go-test", "1.24.0", other, CleanTargetAttested)
	if err != nil {
		t.Fatalf("MakeTestCommand: %v", err)
	}
	otherPrior, err := MakeTestObservation(repository, otherCommand, []byte(documents.Report), other, 0)
	if err != nil {
		t.Fatalf("MakeTestObservation at other target: %v", err)
	}
	dynamic := newRequest(documents)
	dynamic.Command, dynamic.Observation, dynamic.Report = command, []byte(documents.Observation), []byte(documents.Report)
	dynamic.PriorObservations = [][]byte{otherPrior}
	_, err = Evaluate(repository, verifier, dynamic)
	requireCode(t, err, CodeObservationTargetMismatch)

	dynamic.PriorObservations = nil
	for range maxPriorObservations + 1 {
		dynamic.PriorObservations = append(dynamic.PriorObservations, prior)
	}
	_, err = Evaluate(repository, verifier, dynamic)
	requireCode(t, err, CodeResourceExhausted)
}

func mustEvaluate(t *testing.T, repository Repository, verifier UpstreamVerifier, request Request) Result {
	t.Helper()
	result, err := Evaluate(repository, verifier, request)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	return result
}
