package analyzercap

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

const candidateGoRequestVector = `{"profile":"corvint-analyzer-candidate/experimental","family":"go","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input-1","family":"go.mod","path":"go.mod","sha256":"sha256:8d27e4f4ff6e3f8c1a8ced8f550a286065c62bff008c069e5f4c12061a00fa77","content_base64":"bW9kdWxlIGV4YW1wbGUuY29tL21vZHVsZQpnbyAxLjI3LjAK"}]}
`

const candidateGoSuccessVector = `{"profile":"corvint-analyzer-candidate/experimental","family":"go","request_id":"request-1","status":"CANDIDATE","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"input-1","sha256":"sha256:8d27e4f4ff6e3f8c1a8ced8f550a286065c62bff008c069e5f4c12061a00fa77"}],"facts":[{"kind":"go.language.declaration","input_handle":"input-1","related_handle":"-","subject":"go","predicate":"declares-language","value":"1.27.0","instance_id":"root","evidence_sha256":"sha256:5b07b4320d23c64b15e5bbf8553f5fd91c95b27900b513bf1d10b1051f754ffe"},{"kind":"go.module","input_handle":"input-1","related_handle":"-","subject":"root","predicate":"declares-module","value":"example.com/module","instance_id":"root","evidence_sha256":"sha256:486362f6f76464651f690778f7901c5a7aa1dd42da15b094b014685fc97e0520"}]}
`

const candidateGoRejectedVector = `{"profile":"corvint-analyzer-candidate/experimental","family":"go","request_id":"request-1","status":"REJECTED","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"input-1","sha256":"sha256:8d27e4f4ff6e3f8c1a8ced8f550a286065c62bff008c069e5f4c12061a00fa77"}],"reason":"DIGEST_MISMATCH"}
`

type candidateVectorTarget struct {
	OS           string   `json:"os"`
	Architecture string   `json:"architecture"`
	ABI          string   `json:"abi"`
	Features     []string `json:"features"`
}

type candidateVectorInput struct {
	Handle        string `json:"handle"`
	Family        string `json:"family"`
	Path          string `json:"path"`
	SHA256        string `json:"sha256"`
	ContentBase64 string `json:"content_base64"`
}

type candidateVectorRequest struct {
	Profile           string                 `json:"profile"`
	Family            string                 `json:"family"`
	RequestID         string                 `json:"request_id"`
	ScopeID           string                 `json:"scope_id"`
	CompilationUnitID string                 `json:"compilation_unit_id"`
	Target            candidateVectorTarget  `json:"target"`
	Inputs            []candidateVectorInput `json:"inputs"`
}

type candidateVectorEcho struct {
	Handle string `json:"handle"`
	SHA256 string `json:"sha256"`
}

type candidateVectorFact struct {
	Kind           string `json:"kind"`
	InputHandle    string `json:"input_handle"`
	RelatedHandle  string `json:"related_handle"`
	Subject        string `json:"subject"`
	Predicate      string `json:"predicate"`
	Value          string `json:"value"`
	InstanceID     string `json:"instance_id"`
	EvidenceSHA256 string `json:"evidence_sha256"`
}

type candidateVectorSuccess struct {
	Profile           string                `json:"profile"`
	Family            string                `json:"family"`
	RequestID         string                `json:"request_id"`
	Status            string                `json:"status"`
	ScopeID           string                `json:"scope_id"`
	CompilationUnitID string                `json:"compilation_unit_id"`
	Target            candidateVectorTarget `json:"target"`
	InputEchoes       []candidateVectorEcho `json:"input_echoes"`
	Facts             []candidateVectorFact `json:"facts"`
}

type candidateVectorRejected struct {
	Profile           string                `json:"profile"`
	Family            string                `json:"family"`
	RequestID         string                `json:"request_id"`
	Status            string                `json:"status"`
	ScopeID           string                `json:"scope_id"`
	CompilationUnitID string                `json:"compilation_unit_id"`
	Target            candidateVectorTarget `json:"target"`
	InputEchoes       []candidateVectorEcho `json:"input_echoes"`
	Reason            string                `json:"reason"`
}

func TestCandidateGoDocumentationVectorIsByteExact(t *testing.T) {
	var request candidateVectorRequest
	decodeCandidateVector(t, candidateGoRequestVector, &request)
	wantRequest := candidateVectorRequest{
		Profile:           "corvint-analyzer-candidate/experimental",
		Family:            "go",
		RequestID:         "request-1",
		ScopeID:           "root",
		CompilationUnitID: "unit-1",
		Target:            candidateVectorTarget{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}},
		Inputs: []candidateVectorInput{{
			Handle:        "input-1",
			Family:        "go.mod",
			Path:          "go.mod",
			SHA256:        "sha256:8d27e4f4ff6e3f8c1a8ced8f550a286065c62bff008c069e5f4c12061a00fa77",
			ContentBase64: "bW9kdWxlIGV4YW1wbGUuY29tL21vZHVsZQpnbyAxLjI3LjAK",
		}},
	}
	if !reflect.DeepEqual(request, wantRequest) {
		t.Fatalf("request=%#v want=%#v", request, wantRequest)
	}
	content, err := base64.StdEncoding.DecodeString(request.Inputs[0].ContentBase64)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(content), "module example.com/module\ngo 1.27.0\n"; got != want {
		t.Fatalf("content=%q want=%q", got, want)
	}
	contentDigest := sha256.Sum256(content)
	if got, want := request.Inputs[0].SHA256, "sha256:"+hex.EncodeToString(contentDigest[:]); got != want {
		t.Fatalf("input digest=%s want=%s", got, want)
	}

	var success candidateVectorSuccess
	decodeCandidateVector(t, candidateGoSuccessVector, &success)
	if success.Profile != request.Profile || success.Family != request.Family || success.RequestID != request.RequestID ||
		success.Status != "CANDIDATE" || success.ScopeID != request.ScopeID ||
		success.CompilationUnitID != request.CompilationUnitID || !reflect.DeepEqual(success.Target, request.Target) {
		t.Fatalf("success fixed fields do not echo request: request=%#v success=%#v", request, success)
	}
	wantEchoes := []candidateVectorEcho{{Handle: request.Inputs[0].Handle, SHA256: request.Inputs[0].SHA256}}
	if !reflect.DeepEqual(success.InputEchoes, wantEchoes) {
		t.Fatalf("echoes=%#v want=%#v", success.InputEchoes, wantEchoes)
	}
	wantFacts := []candidateVectorFact{
		{Kind: "go.language.declaration", InputHandle: "input-1", RelatedHandle: "-", Subject: "go", Predicate: "declares-language", Value: "1.27.0", InstanceID: "root", EvidenceSHA256: "sha256:5b07b4320d23c64b15e5bbf8553f5fd91c95b27900b513bf1d10b1051f754ffe"},
		{Kind: "go.module", InputHandle: "input-1", RelatedHandle: "-", Subject: "root", Predicate: "declares-module", Value: "example.com/module", InstanceID: "root", EvidenceSHA256: "sha256:486362f6f76464651f690778f7901c5a7aa1dd42da15b094b014685fc97e0520"},
	}
	if !reflect.DeepEqual(success.Facts, wantFacts) || compareCandidateVectorFacts(success.Facts[0], success.Facts[1]) >= 0 {
		t.Fatalf("facts=%#v want=%#v", success.Facts, wantFacts)
	}

	target := `{"os":"darwin","architecture":"arm64","abi":"none","features":[]}`
	for _, fact := range success.Facts {
		fields := []string{
			request.Family, request.RequestID, request.ScopeID, request.CompilationUnitID, target,
			fact.InputHandle, request.Inputs[0].SHA256, fact.RelatedHandle, "-", fact.Kind,
			fact.Subject, fact.Predicate, fact.Value, fact.InstanceID,
		}
		if got, want := fact.EvidenceSHA256, candidateEvidenceDigest(fields); got != want {
			t.Fatalf("%s evidence=%s want=%s", fact.Kind, got, want)
		}
	}

	var rejected candidateVectorRejected
	decodeCandidateVector(t, candidateGoRejectedVector, &rejected)
	if rejected.Profile != request.Profile || rejected.Family != request.Family || rejected.RequestID != request.RequestID ||
		rejected.Status != "REJECTED" || rejected.ScopeID != request.ScopeID || rejected.CompilationUnitID != request.CompilationUnitID ||
		!reflect.DeepEqual(rejected.Target, request.Target) || !reflect.DeepEqual(rejected.InputEchoes, wantEchoes) || rejected.Reason != "DIGEST_MISMATCH" {
		t.Fatalf("rejection does not bind the full request: request=%#v rejection=%#v", request, rejected)
	}
	replayed := request
	replayed.Inputs = append([]candidateVectorInput(nil), request.Inputs...)
	replayed.Inputs[0].SHA256 = "sha256:" + strings.Repeat("0", 64)
	if reflect.DeepEqual(rejected.InputEchoes, []candidateVectorEcho{{Handle: replayed.Inputs[0].Handle, SHA256: replayed.Inputs[0].SHA256}}) {
		t.Fatal("same-request-id rejection can replay across a changed input digest")
	}
}

func decodeCandidateVector(t *testing.T, frame string, destination any) {
	t.Helper()
	if len(frame) < 2 || frame[len(frame)-1] != '\n' || frame[len(frame)-2] == '\n' {
		t.Fatal("vector must have exactly one terminal LF")
	}
	payload := []byte(frame[:len(frame)-1])
	if !json.Valid(payload) {
		t.Fatal("vector payload is not JSON")
	}
	if err := json.Unmarshal(payload, destination); err != nil {
		t.Fatal(err)
	}
	canonical, err := json.Marshal(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(canonical, payload) {
		t.Fatalf("noncanonical vector\ngot:  %s\nwant: %s", payload, canonical)
	}
}

func compareCandidateVectorFacts(left, right candidateVectorFact) int {
	leftFields := [...]string{left.Kind, left.InputHandle, left.RelatedHandle, left.Subject, left.Predicate, left.Value, left.InstanceID, left.EvidenceSHA256}
	rightFields := [...]string{right.Kind, right.InputHandle, right.RelatedHandle, right.Subject, right.Predicate, right.Value, right.InstanceID, right.EvidenceSHA256}
	for index := range leftFields {
		if leftFields[index] < rightFields[index] {
			return -1
		}
		if leftFields[index] > rightFields[index] {
			return 1
		}
	}
	return 0
}

func candidateEvidenceDigest(fields []string) string {
	hasher := sha256.New()
	_, _ = hasher.Write([]byte("corvint-analyzer-candidate-evidence/experimental"))
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(fields)))
	_, _ = hasher.Write(size[:])
	for _, field := range fields {
		binary.BigEndian.PutUint32(size[:], uint32(len(field)))
		_, _ = hasher.Write(size[:])
		_, _ = hasher.Write([]byte(field))
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}
