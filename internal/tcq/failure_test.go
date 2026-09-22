package tcq

import (
	"errors"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// These expectations are authored from the specification text — the TCQ-V0-042
// error-assignment table — rather than captured from the oracle, because they
// assert refusals, and a refusal has no bytes to compare.

func evaluateWith(t *testing.T, mutate func(*Request)) error {
	t.Helper()
	documents := loadDocuments(t)
	request := newRequest(documents)
	mutate(&request)
	_, err := Evaluate(newFixtureRepository(documents), fixtureVerifier{base: documents.Base, target: documents.Target}, request)
	return err
}

// TestDynamicTupleIsAllOrNone covers the exact TCQ-V0-033 combination table.
func TestDynamicTupleIsAllOrNone(t *testing.T) {
	documents := loadDocuments(t)
	partials := map[string]func(*Request){
		"command-only":     func(request *Request) { request.Command = []byte(documents.Command) },
		"observation-only": func(request *Request) { request.Observation = []byte(documents.Observation) },
		"report-only":      func(request *Request) { request.Report = []byte(documents.Report) },
		"missing-report": func(request *Request) {
			request.Command = []byte(documents.Command)
			request.Observation = []byte(documents.Observation)
		},
	}
	for name, mutate := range partials {
		t.Run(name, func(t *testing.T) {
			requireCode(t, evaluateWith(t, mutate), CodeInvalidInput)
		})
	}
	t.Run("missing-ocm", func(t *testing.T) {
		requireCode(t, evaluateWith(t, func(request *Request) { request.OCM = nil }), CodeInvalidInput)
	})
}

// TestPreflightPrecedesStrictParse covers TCQ-V0-042 stages 2 and 3: every
// artifact's depth/member preflight runs before any artifact is strictly parsed.
func TestPreflightPrecedesStrictParse(t *testing.T) {
	documents := loadDocuments(t)
	deep := []byte(strings.Repeat("[", 9) + strings.Repeat("]", 9))
	requireCode(t, evaluateWith(t, func(request *Request) {
		request.OCM = append([]byte(" "), request.OCM...)
		request.Command, request.Observation, request.Report = deep, []byte(documents.Observation), []byte(documents.Report)
	}), CodeResourceExhausted)
	requireCode(t, evaluateWith(t, func(request *Request) {
		request.Command, request.Observation, request.Report = []byte(`{}`), deep, []byte(documents.Report)
	}), CodeResourceExhausted)
}

// TestUpstreamProfileRefusals covers TCQ-V0-002 and TCQ-V0-004: legacy CEM and
// an unknown extractor fail operationally and can produce no abstention.
func TestUpstreamProfileRefusals(t *testing.T) {
	documents := loadDocuments(t)
	cases := map[string]struct {
		mutate func(*Request)
		code   string
	}{
		"cem-0.1": {func(request *Request) { request.CEM = []byte(documents.CEM01) }, CodeUnsupportedCEMProfile},
		"ocm-profile": {func(request *Request) {
			request.OCM = []byte(strings.Replace(documents.OCM, `"spec":"ocm/0.1-experimental"`, `"spec":"ocm/0.2-imaginary"`, 1))
		}, CodeUnsupportedOCMProfile},
		"claim-extractor": {func(request *Request) {
			request.OCM = []byte(strings.Replace(documents.OCM, `"extractor":"corvint-test-claim/1"`, `"extractor":"corvint-test-claim/9"`, 1))
		}, CodeUnsupportedClaimExtractor},
		"noncanonical-ocm": {func(request *Request) {
			request.OCM = []byte(strings.Replace(documents.OCM, `{"cemSha256"`, `{ "cemSha256"`, 1))
		}, CodeNoncanonicalMap},
		"ocm-too-large": {func(request *Request) {
			request.OCM = make([]byte, maxOCMBytes+1)
		}, CodeResourceExhausted},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			requireCode(t, evaluateWith(t, testCase.mutate), testCase.code)
		})
	}
}

// TestRevisionBinding covers TCQ-V0-042 step 4 and TCQ-V0-036: the invocation
// OIDs are independent inputs, not values learned from an artifact.
func TestRevisionBinding(t *testing.T) {
	documents := loadDocuments(t)
	requireCode(t, evaluateWith(t, func(request *Request) { request.ExpectedBase = "" }), CodeExpectedBaseRequired)
	requireCode(t, evaluateWith(t, func(request *Request) { request.Target = "" }), CodeTargetRequired)

	request := newRequest(documents)
	other := strings.Repeat("3", 40)
	_, err := Evaluate(newFixtureRepository(documents), fixtureVerifier{base: documents.Base, target: other}, request)
	requireCode(t, err, CodeTargetMismatch)
}

// TestDynamicArtifactRefusals covers TCQ-V0-042 stages 6 to 8 in order.
func TestDynamicArtifactRefusals(t *testing.T) {
	documents := loadDocuments(t)
	dynamic := func(request *Request) {
		request.Command = []byte(documents.Command)
		request.Observation = []byte(documents.Observation)
		request.Report = []byte(documents.Report)
	}
	other := strings.Repeat("3", 40)
	otherCommand, err := MakeTestCommand([]string{"go", "test"}, ".", "go-test", "1.24.0", other, CleanTargetAttested)
	if err != nil {
		t.Fatalf("MakeTestCommand: %v", err)
	}
	missingCwd, err := MakeTestCommand([]string{"go", "test"}, "pkg/absent", "go-test", "1.24.0", documents.Target, CleanTargetAttested)
	if err != nil {
		t.Fatalf("MakeTestCommand: %v", err)
	}
	blobCwd, err := MakeTestCommand([]string{"go", "test"}, "pkg/sample_test.go", "go-test", "1.24.0", documents.Target, CleanTargetAttested)
	if err != nil {
		t.Fatalf("MakeTestCommand: %v", err)
	}
	targetMismatchAndMissingCwd, err := MakeTestCommand([]string{"go", "test"}, "pkg/absent", "go-test", "1.24.0", other, CleanTargetAttested)
	if err != nil {
		t.Fatalf("MakeTestCommand: %v", err)
	}
	cases := map[string]struct {
		mutate func(*Request)
		code   string
	}{
		"command-target": {func(request *Request) { dynamic(request); request.Command = otherCommand }, CodeCommandTargetMismatch},
		"cwd-missing":    {func(request *Request) { dynamic(request); request.Command = missingCwd }, CodeCommandCwdUnavailable},
		"cwd-is-blob":    {func(request *Request) { dynamic(request); request.Command = blobCwd }, CodeCommandCwdUnavailable},
		// TCQ-V0-042: command target mismatch must precede cwd failure when a
		// single command carries both defects.
		"command-target-precedes-cwd-missing": {func(request *Request) {
			dynamic(request)
			request.Command = targetMismatchAndMissingCwd
		}, CodeCommandTargetMismatch},
		"observation-command": {func(request *Request) {
			dynamic(request)
			request.Observation = []byte(documents.DirtyObservation)
		}, CodeObservationCommandMismatch},
		"report-digest": {func(request *Request) {
			dynamic(request)
			request.Report = append([]byte(documents.Report), ' ')
		}, CodeReportDigestMismatch},
		"noncanonical-command": {func(request *Request) {
			dynamic(request)
			request.Command = []byte(strings.Replace(documents.Command, `{"argv"`, `{ "argv"`, 1))
		}, CodeNoncanonicalCommand},
		"tampered-command-id": {func(request *Request) {
			dynamic(request)
			request.Command = []byte(strings.Replace(documents.Command, `"cleanTarget":"CALLER_ATTESTED_CLEAN"`, `"cleanTarget":"NOT_ATTESTED"`, 1))
		}, CodeInvalidCommand},
		"tampered-observation-rows": {func(request *Request) {
			dynamic(request)
			request.Observation = []byte(strings.Replace(documents.Observation, `"status":"SKIPPED"`, `"status":"PASSED"`, 1))
		}, CodeInvalidObservation},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			requireCode(t, evaluateWith(t, testCase.mutate), testCase.code)
		})
	}
}

// failingTreeEntry is the fixture repository with one target-tree lookup
// failing the way a Git timeout or corrupt object would.
type failingTreeEntry struct {
	*fixtureRepository
	path string
	err  error
}

func (repository failingTreeEntry) TreeEntry(revision, path string) (TreeEntry, error) {
	if path == repository.path {
		return TreeEntry{}, repository.err
	}
	return repository.fixtureRepository.TreeEntry(revision, path)
}

// TestTreeLookupFailureIsNotAnAbsentPath covers TCQ-V0-028 and TCQ-V0-042: a
// failed lookup of the command cwd or a report row's file is returned as that
// failure, never read as an unavailable cwd or an unkeyed row.
func TestTreeLookupFailureIsNotAnAbsentPath(t *testing.T) {
	documents := loadDocuments(t)
	lookupFailure := errors.New("git lookup failed")
	for _, path := range []string{"pkg", "pkg/sample_test.go"} {
		t.Run(path, func(t *testing.T) {
			request := newRequest(documents)
			request.Command, request.Observation, request.Report = []byte(documents.Command), []byte(documents.Observation), []byte(documents.Report)
			repository := failingTreeEntry{newFixtureRepository(documents), path, lookupFailure}
			_, err := Evaluate(repository, fixtureVerifier{base: documents.Base, target: documents.Target}, request)
			if !errors.Is(err, lookupFailure) {
				t.Fatalf("Evaluate error = %v, want the lookup failure", err)
			}
		})
	}
}

// TestReportCommandInconsistent covers TCQ-V0-029: exit zero with any keyed or
// unkeyed FAILED/ERROR testcase invalidates the dynamic invocation.
func TestReportCommandInconsistent(t *testing.T) {
	documents := loadDocuments(t)
	repository := newFixtureRepository(documents)
	failing := `<testsuite name="s"><testcase file="pkg/sample_test.go" name="TestBeta"><failure/></testcase></testsuite>`
	command := []byte(documents.Command)
	if _, err := MakeTestObservation(repository, command, []byte(failing), documents.Target, 0); err == nil {
		t.Fatal("observation accepted exit zero alongside a failed row")
	} else {
		requireCode(t, err, CodeReportCommandInconsistent)
	}
	observation, err := MakeTestObservation(repository, command, []byte(failing), documents.Target, 1)
	if err != nil {
		t.Fatalf("MakeTestObservation: %v", err)
	}
	request := newRequest(documents)
	request.Command = command
	request.Observation = observation
	request.Report = []byte(failing)
	result, err := Evaluate(repository, fixtureVerifier{base: documents.Base, target: documents.Target}, request)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !hasClaimWithReason(result, reasonTestFailed) {
		t.Error("a failed row did not produce test-failed")
	}
	if hasRelation(result) {
		t.Error("a failed row produced a relation")
	}
}

func hasClaimWithReason(result Result, reason string) bool {
	for _, claim := range result.Claims() {
		for _, candidate := range claim.Reasons {
			if candidate == reason {
				return true
			}
		}
	}
	return false
}

func hasRelation(result Result) bool {
	for _, claim := range result.Claims() {
		if claim.Relation != "" {
			return true
		}
	}
	return false
}

// TestCommandValidation covers the TCQ-V0-023/024 closed shape directly, since
// each refusal needs a document the constructor would never build.
func TestCommandValidation(t *testing.T) {
	valid := func() wire.Value {
		return jsonObject(
			member{"argv", jsonStrings([]string{"go", "test"})},
			member{"cleanTarget", jsonString(CleanTargetAttested)},
			member{"cwd", jsonString(".")},
			member{"environment", jsonObject(
				member{"entries", jsonArray(nil)},
				member{"policy", jsonString("OMITTED")},
			)},
			member{"runner", jsonObject(
				member{"identity", jsonString("go-test")},
				member{"version", jsonString("1.24.0")},
			)},
			member{"spec", jsonString(CommandSpec)},
			member{"targetRevision", jsonString(strings.Repeat("a", 40))},
		)
	}
	cases := map[string]func(wire.Value){
		"shell-string-argv":  func(value wire.Value) { value.Obj.Values["argv"] = jsonString("go test") },
		"empty-argv":         func(value wire.Value) { value.Obj.Values["argv"] = jsonArray(nil) },
		"empty-argv-element": func(value wire.Value) { value.Obj.Values["argv"] = jsonStrings([]string{""}) },
		"control-in-argv":    func(value wire.Value) { value.Obj.Values["argv"] = jsonStrings([]string{"a\x00b"}) },
		"environment-entry": func(value wire.Value) {
			value.Obj.Values["environment"] = jsonObject(
				member{"entries", jsonStrings([]string{"PATH"})},
				member{"policy", jsonString("OMITTED")},
			)
		},
		"environment-policy": func(value wire.Value) {
			value.Obj.Values["environment"] = jsonObject(
				member{"entries", jsonArray(nil)},
				member{"policy", jsonString("CAPTURED")},
			)
		},
		"clean-target":   func(value wire.Value) { value.Obj.Values["cleanTarget"] = jsonString("TRUSTED") },
		"absolute-cwd":   func(value wire.Value) { value.Obj.Values["cwd"] = jsonString("/etc") },
		"dotdot-cwd":     func(value wire.Value) { value.Obj.Values["cwd"] = jsonString("../x") },
		"runner-grammar": func(value wire.Value) { value.Obj.Values["runner"].Obj.Values["identity"] = jsonString("-bad") },
		"bad-revision":   func(value wire.Value) { value.Obj.Values["targetRevision"] = jsonString("HEAD") },
		"bad-spec":       func(value wire.Value) { value.Obj.Values["spec"] = jsonString("test-command/9") },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			document := valid()
			mutate(document)
			identity := commandPrefix + domainHash(domainCommand, canonicalValue(document))
			document.Obj.Keys = append(document.Obj.Keys, "id")
			document.Obj.Values["id"] = jsonString(identity)
			_, err := parseCommand(canonicalJSON(document))
			requireCode(t, err, CodeInvalidCommand)
		})
	}
	t.Run("extra-field", func(t *testing.T) {
		document := valid()
		document.Obj.Keys = append(document.Obj.Keys, "extra")
		document.Obj.Values["extra"] = jsonString("x")
		identity := commandPrefix + domainHash(domainCommand, canonicalValue(document))
		document.Obj.Keys = append(document.Obj.Keys, "id")
		document.Obj.Values["id"] = jsonString(identity)
		requireCode(t, mustFailCommand(parseCommand(canonicalJSON(document))), CodeInvalidCommand)
	})
	t.Run("duplicate-key", func(t *testing.T) {
		raw := []byte(`{"argv":["go"],"argv":["go"],"cleanTarget":"NOT_ATTESTED"}` + "\n")
		requireCode(t, mustFailCommand(parseCommand(raw)), CodeInvalidCommand)
	})
}

func mustFailCommand(_ command, err error) error { return err }
