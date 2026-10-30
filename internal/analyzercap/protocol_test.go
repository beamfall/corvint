package analyzercap

import (
	"context"
	json "encoding/json/v2"
	"strings"
	"testing"
)

func testAnalyzerRequest(t *testing.T) AnalyzerRequest {
	t.Helper()
	profile := testProfile(t, "scopeA", []Feature{{Name: "featureA", State: FeatureEnabled}}, nil)
	return AnalyzerRequest{
		Protocol: AnalyzerProtocolV0, RequestID: "request1", ProfileIdentity: profile.Identity(), LockDigest: "lock1",
		RegistrySnapshotDigest: "snapshot1", TrustEpoch: "epoch1", ResolverDigest: "resolver1", ClauseID: "clause1",
		CompilationUnitID: "unit1", Target: testTarget, ProjectionDigest: "projection1", ProjectionScheme: "scheme1",
		Inputs: []InputHandle{{Handle: "input1", Digest: "digest1"}}, InputBinding: "input-binding1", Features: profile.Features,
		Limits: ProtocolLimits{WallMilliseconds: 100, MaxChildren: 1, MaxRequestBytes: MaxProtocolBytes, MaxResponseBytes: MaxProtocolBytes, MaxStderrBytes: MaxProtocolBytes},
	}
}
func echoedResponse(request AnalyzerRequest) AnalyzerResponse {
	return AnalyzerResponse{
		Protocol: request.Protocol, RequestID: request.RequestID, ProfileIdentity: request.ProfileIdentity, LockDigest: request.LockDigest,
		RegistrySnapshotDigest: request.RegistrySnapshotDigest, TrustEpoch: request.TrustEpoch, ResolverDigest: request.ResolverDigest,
		ClauseID: request.ClauseID, CompilationUnitID: request.CompilationUnitID, Target: request.Target,
		ProjectionDigest: request.ProjectionDigest, ProjectionScheme: request.ProjectionScheme, Inputs: request.Inputs, InputBinding: request.InputBinding,
		Facts: []ProposedFact{{Kind: "fact", Value: "value"}}, ProfileEvidence: clone(request.Inputs),
		Diagnostics: []AnalyzerDiagnostic{{Code: "diagnostic1"}},
	}
}
func TestEcho(t *testing.T) {
	request := testAnalyzerRequest(t)
	encoded, err := MarshalAnalyzerRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "executable") || strings.Contains(string(encoded), "environment") {
		t.Fatal("a")
	}
	raw, err := json.Marshal(echoedResponse(request), json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	response, err := DecodeAnalyzerResponse(raw, request)
	if err != nil || len(response.Facts) != 1 || response.Facts[0].Value != "value" {
		t.Fatal("r")
	}
}
func TestProtocolHostile(t *testing.T) {
	request := testAnalyzerRequest(t)
	raw, err := json.Marshal(echoedResponse(request), json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func([]byte) []byte{
		"trailing": func(value []byte) []byte { return append(value, '\n') },
		"unknown": func(value []byte) []byte {
			return append(append(value[:len(value)-1], []byte(`,"authority":"PASS"`)...), '}')
		},
		"duplicate": func(value []byte) []byte {
			return append(append(value[:len(value)-1], []byte(`,"requestId":"forged"`)...), '}')
		},
		"malformed": func([]byte) []byte { return []byte(`{"requestId":`) },
		"forged":    func(value []byte) []byte { return []byte(strings.Replace(string(value), `"request1"`, `"forged"`, 1)) },
	} {
		t.Run(name, func(t *testing.T) {
			_, decodeErr := DecodeAnalyzerResponse(mutate(raw), request)
			reason(t, decodeErr, ProtocolEchoMismatch)
		})
	}
}

func TestProfileEvidenceMustExactlyEchoInputs(t *testing.T) {
	request := testAnalyzerRequest(t)
	for name, mutate := range map[string]func(*AnalyzerResponse){
		"missing": func(response *AnalyzerResponse) { response.ProfileEvidence = nil },
		"extra": func(response *AnalyzerResponse) {
			response.ProfileEvidence = append(response.ProfileEvidence, InputHandle{Handle: "input2", Digest: "digest2"})
		},
		"different": func(response *AnalyzerResponse) { response.ProfileEvidence[0].Digest = "forged" },
	} {
		t.Run(name, func(t *testing.T) {
			response := echoedResponse(request)
			mutate(&response)
			raw, err := json.Marshal(response, json.Deterministic(true))
			if err != nil {
				t.Fatal(err)
			}
			_, err = DecodeAnalyzerResponse(raw, request)
			reason(t, err, ProtocolEchoMismatch)
		})
	}
}

func TestProtocolRejectsForgedInputBinding(t *testing.T) {
	request := testAnalyzerRequest(t)
	response := echoedResponse(request)
	response.InputBinding = "forged-binding"
	raw, err := json.Marshal(response, json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	_, err = DecodeAnalyzerResponse(raw, request)
	reason(t, err, ProtocolEchoMismatch)
}
func TestLimits(t *testing.T) {
	request := testAnalyzerRequest(t)
	request.Inputs = append(request.Inputs, request.Inputs[0])
	_, err := MarshalAnalyzerRequest(request)
	reason(t, err, InputEvidenceMismatch)
	request = testAnalyzerRequest(t)
	request.Limits.MaxChildren = 2
	_, err = MarshalAnalyzerRequest(request)
	reason(t, err, LimitExceeded)
	request = testAnalyzerRequest(t)
	response := echoedResponse(request)
	response.Facts = make([]ProposedFact, MaxFacts+1)
	raw, marshalErr := json.Marshal(response, json.Deterministic(true))
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	_, err = DecodeAnalyzerResponse(raw, request)
	reason(t, err, LimitExceeded)
}

type rejectingAdmission struct{}

func (rejectingAdmission) Identity() Identity                        { return "admission-verifier" }
func (rejectingAdmission) CloneAdmissionVerifier() AdmissionVerifier { return rejectingAdmission{} }
func (rejectingAdmission) coreBoundedAdmissionVerifier()             {}
func (rejectingAdmission) VerifyAdmission(context.Context, AdmissionContext, AnalyzerResponse) (Admission, error) {
	return AdmissionDenied, nil
}
func TestAdmissionReject(t *testing.T) {
	source := coreInput(t, rejectingAdmission{})
	core, err := NewCore(source)
	if err != nil {
		t.Fatal(err)
	}
	authority, err := core.IssueAdmissionAuthority()
	if err != nil {
		t.Fatal(err)
	}
	admission, err := admit(context.Background(), authority, echoedResponse(req(t, authority)))
	if admission != AdmissionDenied {
		t.Fatal("a")
	}
	reason(t, err, AdmissionRejected)
}
