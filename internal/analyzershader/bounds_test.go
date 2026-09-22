package analyzershader

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func TestProspectiveOutputUnderAtOver(t *testing.T) {
	request := shaderRequest("#version 300 es\nvoid main(){}\n")
	frame := canonicalFrame(t, request)
	echoes := echoes(request.Inputs)
	empty, reason := encodeCandidate(request, echoes, requestDigest(frame), nil)
	if reason != "" {
		t.Fatal(reason)
	}
	fact := Fact{Kind: "shader.glsl.bound", InputHandle: "input-1", RelatedHandle: "-", Subject: "bound", Predicate: "has-size", InstanceID: "unit-1"}
	encoded, err := json.Marshal(fact)
	if err != nil {
		t.Fatal(err)
	}
	fill := maxOutput - len(empty) - len(encoded)
	if fill < 2 {
		t.Fatalf("invalid fixture fill=%d", fill)
	}
	for _, vector := range []struct {
		name     string
		fill     int
		wantSize int
		want     string
	}{
		{"under", fill - 1, maxOutput - 1, ""},
		{"at", fill, maxOutput, ""},
		{"over", fill + 1, 0, "OUTPUT_LIMIT"},
	} {
		t.Run(vector.name, func(t *testing.T) {
			candidate := fact
			candidate.Value = strings.Repeat("x", vector.fill)
			result, got := encodeCandidate(request, echoes, requestDigest(frame), []Fact{candidate})
			if got != vector.want {
				t.Fatalf("reason=%q want=%q", got, vector.want)
			}
			if vector.wantSize != 0 && len(result) != vector.wantSize {
				t.Fatalf("size=%d want=%d", len(result), vector.wantSize)
			}
		})
	}
}

func TestSourceAndLexicalBoundsUnderAtOver(t *testing.T) {
	request := shaderRequest("#version 300 es\nvoid main(){}\n")
	input := request.Inputs[0]
	for _, vector := range []struct {
		name string
		end  int
		want string
	}{
		{"under", maxWitness - 1, ""},
		{"at", maxWitness, ""},
		{"over", maxWitness + 1, "LIMIT_EXCEEDED"},
	} {
		t.Run("witness-"+vector.name, func(t *testing.T) {
			_, got := sourceFact(request, input, make([]byte, maxWitness+1), 0, vector.end, "shader.glsl.bound", "bound", "has-size", "", "unit-1")
			if got != vector.want {
				t.Fatalf("reason=%q want=%q", got, vector.want)
			}
		})
	}

	const fixedTokens = 6 // validated #version is blanked before void main(){}.
	for _, vector := range []struct {
		name string
		reps int
		want string
	}{
		{"under", (maxTokens-fixedTokens)/2 - 1, "CANDIDATE"},
		{"at", (maxTokens - fixedTokens) / 2, "CANDIDATE"},
		{"over", (maxTokens-fixedTokens)/2 + 1, "REJECTED"},
	} {
		t.Run("tokens-"+vector.name, func(t *testing.T) {
			source := "#version 300 es\n" + strings.Repeat("x;", vector.reps) + "void main(){}\n"
			var result struct {
				Status string `json:"status"`
				Reason string `json:"reason"`
			}
			if err := json.Unmarshal(Analyze(canonicalFrame(t, shaderRequest(source))), &result); err != nil {
				t.Fatal(err)
			}
			if result.Status != vector.want || vector.want == "REJECTED" && result.Reason != "LIMIT_EXCEEDED" {
				t.Fatalf("result=%+v", result)
			}
		})
	}
	for _, vector := range []struct {
		name  string
		depth int
		want  string
	}{
		{"under", maxNesting - 1, "CANDIDATE"},
		{"at", maxNesting, "CANDIDATE"},
		{"over", maxNesting + 1, "REJECTED"},
	} {
		t.Run("nesting-"+vector.name, func(t *testing.T) {
			source := "#version 300 es\n" + strings.Repeat("(", vector.depth) + "x" + strings.Repeat(")", vector.depth) + ";\nvoid main(){}\n"
			var result struct {
				Status string `json:"status"`
				Reason string `json:"reason"`
			}
			if err := json.Unmarshal(Analyze(canonicalFrame(t, shaderRequest(source))), &result); err != nil {
				t.Fatal(err)
			}
			if result.Status != vector.want || vector.want == "REJECTED" && result.Reason != "LIMIT_EXCEEDED" {
				t.Fatalf("result=%+v", result)
			}
		})
	}
}

func TestWireAndInputCountBoundsUnderAtOver(t *testing.T) {
	for _, vector := range []struct {
		name string
		size int
		want string
	}{
		{"under", maxWire - 1, "NONCANONICAL_REQUEST"},
		{"at", maxWire, "NONCANONICAL_REQUEST"},
		{"over", maxWire + 1, "LIMIT_EXCEEDED"},
	} {
		t.Run("wire-"+vector.name, func(t *testing.T) {
			if got := rejectionReason(t, Analyze(make([]byte, vector.size))); got != vector.want {
				t.Fatalf("reason=%q want=%q", got, vector.want)
			}
		})
	}
	for _, vector := range []struct {
		name  string
		count int
		want  string
	}{
		{"under", 127, "CANDIDATE"},
		{"at", 128, "CANDIDATE"},
		{"over", 129, "REJECTED"},
	} {
		t.Run("inputs-"+vector.name, func(t *testing.T) {
			request := shaderRequest("#version 300 es\nvoid main(){}\n")
			request.Inputs = request.Inputs[:0]
			for index := 0; index < vector.count; index++ {
				source := []byte("#version 300 es\nvoid main(){}\n")
				sum := sha256.Sum256(source)
				request.Inputs = append(request.Inputs, Input{Handle: "input-" + padded(index), Family: "shader.glsl", Path: "visuals/probe-" + padded(index) + ".glsl", SHA256: "sha256:" + hex.EncodeToString(sum[:]), ContentBase64: base64.StdEncoding.EncodeToString(source)})
			}
			var result struct {
				Status string `json:"status"`
				Reason string `json:"reason"`
			}
			if err := json.Unmarshal(Analyze(canonicalFrame(t, request)), &result); err != nil {
				t.Fatal(err)
			}
			if result.Status != vector.want || vector.want == "REJECTED" && result.Reason != "LIMIT_EXCEEDED" {
				t.Fatalf("result=%+v", result)
			}
		})
	}
}

func TestEchoEnvelopeAndRejectedFrameBoundsUnderAtOver(t *testing.T) {
	base := shaderRequest("#version 300 es\nvoid main(){}\n")
	for _, vector := range []struct {
		name   string
		length int
		bound  bool
	}{
		{"target-under", 127, true},
		{"target-at", 128, true},
		{"target-over", 129, false},
	} {
		t.Run(vector.name, func(t *testing.T) {
			request := base
			request.Target.OS = strings.Repeat("a", vector.length)
			result := Analyze(canonicalFrame(t, request))
			if vector.bound {
				if got := rejectionReason(t, result); got != "" {
					t.Fatalf("reason=%q", got)
				}
				return
			}
			if want := sentinel("INVALID_IDENTIFIER"); !bytes.Equal(result, want) {
				t.Fatalf("unsafe target=%s", result)
			}
		})
	}
	for _, vector := range []struct {
		name   string
		length int
		bound  bool
	}{
		{"feature-under", 127, true},
		{"feature-at", 128, true},
		{"feature-over", 129, false},
	} {
		t.Run(vector.name, func(t *testing.T) {
			request := base
			request.Target.Features = []string{strings.Repeat("a", vector.length)}
			frame := canonicalFrame(t, request)
			result := Analyze(frame)
			if vector.bound {
				response := boundResponse(t, frame, request)
				if response.Reason != "UNSUPPORTED_SCHEMA" {
					t.Fatalf("response=%+v", response)
				}
				return
			}
			if want := sentinel("INVALID_IDENTIFIER"); !bytes.Equal(result, want) {
				t.Fatalf("unsafe feature=%s", result)
			}
		})
	}
	for _, vector := range []struct {
		name   string
		length int
		bound  bool
	}{
		{"digest-under", 63, false},
		{"digest-at", 64, true},
		{"digest-over", 65, false},
	} {
		t.Run(vector.name, func(t *testing.T) {
			request := base
			request.Inputs = append([]Input(nil), base.Inputs...)
			request.Inputs[0].SHA256 = "sha256:" + strings.Repeat("a", vector.length)
			frame := canonicalFrame(t, request)
			result := Analyze(frame)
			if vector.bound {
				response := boundResponse(t, frame, request)
				if response.Reason != "DIGEST_MISMATCH" {
					t.Fatalf("response=%+v", response)
				}
				return
			}
			if want := sentinel("DIGEST_MISMATCH"); !bytes.Equal(result, want) {
				t.Fatalf("unsafe digest=%s", result)
			}
		})
	}
	request := base
	frame := canonicalFrame(t, request)
	echoes := echoes(request.Inputs)
	value, ok := rejectedFrame(request, echoes, requestDigest(frame), "EXACT_BINDING_UNAVAILABLE", maxOutput)
	if !ok {
		t.Fatal("baseline rejected frame exceeds maxOutput")
	}
	for _, vector := range []struct {
		name    string
		maximum int
		want    bool
	}{
		{"rejection-under", len(value) - 1, false},
		{"rejection-at", len(value), true},
		{"rejection-over", len(value) + 1, true},
	} {
		t.Run(vector.name, func(t *testing.T) {
			got, ok := rejectedFrame(request, echoes, requestDigest(frame), "EXACT_BINDING_UNAVAILABLE", vector.maximum)
			if ok != vector.want || ok && !bytes.Equal(got, value) {
				t.Fatalf("ok=%t bytes=%d", ok, len(got))
			}
		})
	}
	for want, mutate := range map[string]func(*Request){
		"INVALID_IDENTIFIER": func(request *Request) { request.Target.OS = strings.Repeat("a", maxWire-4096) },
		"DIGEST_MISMATCH":    func(request *Request) { request.Inputs[0].SHA256 = strings.Repeat("a", maxWire-4096) },
	} {
		request := base
		request.Inputs = append([]Input(nil), base.Inputs...)
		mutate(&request)
		unsafe := canonicalFrame(t, request)
		if len(unsafe) > maxWire {
			t.Fatalf("hostile frame=%d maxWire=%d", len(unsafe), maxWire)
		}
		result := Analyze(unsafe)
		if len(result) > maxOutput || !bytes.Equal(result, sentinel(want)) {
			t.Fatalf("hostile result bytes=%d value=%s", len(result), result)
		}
	}
}

func TestStrictBase64RejectsNoncanonicalPadBitsAndEmptyContent(t *testing.T) {
	base := shaderRequest("#version 300 es\nvoid main(){}\n")
	for _, vector := range []struct {
		name, content, digest string
	}{
		{"nonzero-pad-bits", "AB==", "sha256:6e340b9cffb37a989ca544e6bb780a2c78901d3fb33738768511a30617afa01d"},
		{"empty-content", "", "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
	} {
		t.Run(vector.name, func(t *testing.T) {
			request := base
			request.Inputs = append([]Input(nil), base.Inputs...)
			request.Inputs[0].ContentBase64 = vector.content
			request.Inputs[0].SHA256 = vector.digest
			response := boundResponse(t, canonicalFrame(t, request), request)
			if response.Status != "REJECTED" || response.Reason != "MALFORMED_INPUT" {
				t.Fatalf("response=%+v", response)
			}
		})
	}
}

func shaderRequest(source string) Request {
	tuple := profiles["beamfall.glsl-es-3.00.fragment.visual-shaders"]
	bytes := []byte(source)
	sum := sha256.Sum256(bytes)
	return Request{Profile: Profile, Family: Family, ShaderProfile: "beamfall.glsl-es-3.00.fragment.visual-shaders", ShaderLanguage: tuple.language, ShaderVersion: tuple.version, ShaderToolchain: tuple.toolchain, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []Input{{Handle: "input-1", Family: "shader.glsl", Path: "visuals/probe.glsl", SHA256: "sha256:" + hex.EncodeToString(sum[:]), ContentBase64: base64.StdEncoding.EncodeToString(bytes)}}}
}

func canonicalFrame(t *testing.T, request Request) []byte {
	t.Helper()
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}

func rejectionReason(t *testing.T, result []byte) string {
	t.Helper()
	var decoded struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(result, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded.Reason
}

func padded(value int) string {
	return string([]byte{'0' + byte(value/100), '0' + byte(value/10%10), '0' + byte(value%10)})
}
