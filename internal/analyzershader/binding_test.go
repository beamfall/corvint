package analyzershader

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

type responseBinding struct {
	Profile           string `json:"profile"`
	Family            string `json:"family"`
	ShaderProfile     string `json:"shader_profile"`
	ShaderLanguage    string `json:"shader_language"`
	ShaderVersion     string `json:"shader_version"`
	ShaderToolchain   string `json:"shader_toolchain"`
	RequestID         string `json:"request_id"`
	RequestSHA256     string `json:"request_sha256"`
	ScopeID           string `json:"scope_id"`
	CompilationUnitID string `json:"compilation_unit_id"`
	Target            Target `json:"target"`
	InputEchoes       []Echo `json:"input_echoes"`
	Status            string `json:"status"`
	Reason            string `json:"reason"`
	Facts             []Fact `json:"facts"`
}

func TestExactTupleAndEveryPostEnvelopeRejectionBindWholeRequest(t *testing.T) {
	base := shaderRequest("#version 300 es\nvoid main(){}\n")
	baseFrame := canonicalFrame(t, base)
	baseResponse := boundResponse(t, baseFrame, base)
	if baseResponse.Status != "CANDIDATE" {
		t.Fatalf("base=%+v", baseResponse)
	}
	mutations := []struct {
		name  string
		apply func(*Request)
		want  string
	}{
		{"profile", func(request *Request) { request.ShaderProfile = "unknown" }, "REJECTED"},
		{"language", func(request *Request) { request.ShaderLanguage = "gles" }, "REJECTED"},
		{"version", func(request *Request) { request.ShaderVersion = "3.10" }, "REJECTED"},
		{"toolchain", func(request *Request) { request.ShaderToolchain = "glslang-16.4.1" }, "REJECTED"},
		{"request-id", func(request *Request) { request.RequestID = "request-2" }, "CANDIDATE"},
		{"scope", func(request *Request) { request.ScopeID = "scope-2" }, "CANDIDATE"},
		{"unit", func(request *Request) { request.CompilationUnitID = "unit-2" }, "CANDIDATE"},
		{"target", func(request *Request) { request.Target.OS = "linux" }, "CANDIDATE"},
		{"features", func(request *Request) { request.Target.Features = []string{"feature-1"} }, "REJECTED"},
		{"path", func(request *Request) { request.Inputs[0].Path = "visuals/replayed.glsl" }, "CANDIDATE"},
		{"bytes", replaceInputBytes("#version 300 es\n// unique replay binding\nvoid main(){}\n"), "CANDIDATE"},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			request := base
			request.Target.Features = append([]string{}, base.Target.Features...)
			request.Inputs = append([]Input(nil), base.Inputs...)
			mutation.apply(&request)
			frame := canonicalFrame(t, request)
			response := boundResponse(t, frame, request)
			if response.Status != mutation.want {
				t.Fatalf("response=%+v want=%s", response, mutation.want)
			}
			if response.RequestSHA256 == baseResponse.RequestSHA256 {
				t.Fatalf("replay accepted without a new request identity: %+v", response)
			}
		})
	}
}

func TestClosedRejectionReasonsAndFailClosedGrammar(t *testing.T) {
	literalTaxonomy := []string{
		"AMBIGUOUS_BINDING", "ANALYZER_FAILURE", "CREDENTIAL_INPUT", "CONFLICTING_VALUE",
		"DIGEST_MISMATCH", "DUPLICATE_VALUE", "DYNAMIC_INPUT", "EXACT_BINDING_UNAVAILABLE",
		"INVALID_IDENTIFIER", "INVALID_PATH", "LIMIT_EXCEEDED", "MALFORMED_INPUT",
		"NONCANONICAL_REQUEST", "OUTPUT_LIMIT", "UNKNOWN_FAMILY", "UNKNOWN_FIELD", "UNSUPPORTED_SCHEMA",
	}
	if len(closedReasons) != len(literalTaxonomy) {
		t.Fatalf("taxonomy=%v", closedReasons)
	}
	for _, reason := range literalTaxonomy {
		if !closedReason(reason) {
			t.Fatalf("missing closed reason %s", reason)
		}
	}
	for _, reason := range []string{"NONCANONICAL_REQUEST", "INVALID_IDENTIFIER", "INVALID_PATH", "DIGEST_MISMATCH", "UNKNOWN_FAMILY", "DUPLICATE_VALUE", "LIMIT_EXCEEDED"} {
		want := `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"` + reason + `"}` + "\n"
		if got := string(sentinel(reason)); got != want {
			t.Fatalf("sentinel %s=%q want=%q", reason, got, want)
		}
	}
	if got := rejectionReason(t, sentinel("UNKNOWN_FIELD")); got != "ANALYZER_FAILURE" {
		t.Fatalf("post-envelope sentinel=%s", got)
	}
	if got := rejectionReason(t, sentinel("NOT_A_REASON")); got != "ANALYZER_FAILURE" {
		t.Fatalf("unknown sentinel=%s", got)
	}
	for _, vector := range []struct {
		name   string
		source string
		want   string
	}{
		{"directive", "#version 300 es\n#pragma once\nvoid main(){}\n", "UNSUPPORTED_SCHEMA"},
		{"future-version", "#version 310 es\nvoid main(){}\n", "EXACT_BINDING_UNAVAILABLE"},
		{"late-version", "void main(){}\n#version 300 es\n", "UNSUPPORTED_SCHEMA"},
		{"numeric-macro", "#version 300 es\n#define 1MACRO value\nvoid main(){}\n", "DYNAMIC_INPUT"},
		{"macro-replacement-operator", "#version 300 es\n#define MACRO 1 + 2\nvoid main(){}\n", "DYNAMIC_INPUT"},
		{"macro-continuation-entry-forgery", "#version 300 es\n#define MACRO \\\nvoid main(){}\n", "DYNAMIC_INPUT"},
		{"macro-continuation-token-forgery", "#version 300 es\n#define MACRO 1\\\nuniform float forged;\nvoid main(){}\n", "DYNAMIC_INPUT"},
		{"entry-prototype", "#version 300 es\nvoid main();\n", "UNSUPPORTED_SCHEMA"},
		{"nonliteral-include", "#version 300 es\n#include <shared.glsl>\nvoid main(){}\n", "DYNAMIC_INPUT"},
		{"drive-include", "#version 300 es\n#include \"C:/shared.glsl\"\nvoid main(){}\n", "DYNAMIC_INPUT"},
		{"unbalanced-grammar", "#version 300 es\nvoid main(){\n", "MALFORMED_INPUT"},
		{"unsupported-byte", "#version 300 es\n$\nvoid main(){}\n", "UNSUPPORTED_SCHEMA"},
	} {
		t.Run(vector.name, func(t *testing.T) {
			if got := rejectionReason(t, Analyze(canonicalFrame(t, shaderRequest(vector.source)))); got != vector.want {
				t.Fatalf("reason=%s want=%s", got, vector.want)
			}
		})
	}
}

func TestDuplicateHandleRejectionStaysUnbound(t *testing.T) {
	request := shaderRequest("#version 300 es\nvoid main(){}\n")
	duplicate := request.Inputs[0]
	duplicate.Path = "visuals/other.glsl"
	request.Inputs = append(request.Inputs, duplicate)
	if got := Analyze(canonicalFrame(t, request)); !bytes.Equal(got, sentinel("DUPLICATE_VALUE")) {
		t.Fatalf("duplicate handle=%s", got)
	}
}

func TestUnboundedEchoedFieldsUseListedSentinelReasons(t *testing.T) {
	for _, vector := range []struct {
		mutate func(*Request)
		want   string
	}{
		{func(request *Request) { request.Target.Features = nil }, "NONCANONICAL_REQUEST"},
		{func(request *Request) { request.Target.Features = make([]string, 65) }, "LIMIT_EXCEEDED"},
		{func(request *Request) { request.Target.OS = "bad os" }, "INVALID_IDENTIFIER"},
		{func(request *Request) { request.Target.Features = []string{"bad feature"} }, "INVALID_IDENTIFIER"},
		{func(request *Request) { request.Target.Features = []string{"b", "a"} }, "DUPLICATE_VALUE"},
		{func(request *Request) { request.Inputs[0].SHA256 = "sha256:bad" }, "DIGEST_MISMATCH"},
	} {
		request := shaderRequest("#version 300 es\nvoid main(){}\n")
		vector.mutate(&request)
		if got := Analyze(canonicalFrame(t, request)); !bytes.Equal(got, sentinel(vector.want)) {
			t.Fatalf("want=%s got=%s", vector.want, got)
		}
	}
}

func TestSafeCanonicalExtensionBindsUnknownField(t *testing.T) {
	request := shaderRequest("#version 300 es\nvoid main(){}\n")
	body := strings.TrimSuffix(string(canonicalFrame(t, request)), "\n")
	base := strings.TrimSuffix(body, "}")
	for _, edited := range []string{
		base + `,"x":[9007199254740993,{"a":"b","c":null}]}`,
		base + `,"target inputs":0}`,
		strings.Replace(body, `"features":[]`, `"features":[],"x":true`, 1),
		strings.Replace(body, `"}]}`, `","x":0,"y":"z"}]}`, 1),
	} {
		frame := []byte(edited + "\n")
		want := reject(request, echoes(request.Inputs), requestDigest(frame), "UNKNOWN_FIELD")
		if got := Analyze(frame); !bytes.Equal(got, want) {
			t.Fatalf("frame=%s\ngot=%s\nwant=%s", edited, got, want)
		}
	}
	for edited, reason := range map[string]string{
		base + `,"x":1.0}`: "NONCANONICAL_REQUEST", base + `,"y":0,"x":0}`: "NONCANONICAL_REQUEST", base + `,"x":{"b":0,"a":0}}`: "NONCANONICAL_REQUEST",
		strings.Replace(body, `{"profile":`, `{"x":0,"profile":`, 1):  "NONCANONICAL_REQUEST",
		strings.Replace(base, `"sha256:`, `"sha256:X`, 1) + `,"x":0}`: "DIGEST_MISMATCH",
	} {
		if got := Analyze([]byte(edited + "\n")); !bytes.Equal(got, sentinel(reason)) {
			t.Fatalf("frame=%s\ngot=%s", edited, got)
		}
	}
}

func TestPostEnvelopeRoutingAndSpanEvidenceBinding(t *testing.T) {
	base := shaderRequest("#version 300 es\nvoid main(){}\n")
	for _, vector := range []struct {
		name   string
		mutate func(*Request)
		want   string
	}{
		{"invalid-path-colon", func(request *Request) { request.Inputs[0].Path = "C:/probe.glsl" }, "INVALID_PATH"},
		{"invalid-path-drive-component", func(request *Request) { request.Inputs[0].Path = "visuals/C:probe.glsl" }, "INVALID_PATH"},
		{"malformed-base64", func(request *Request) { request.Inputs[0].ContentBase64 = "####" }, "MALFORMED_INPUT"},
		{"digest-mismatch", func(request *Request) {
			request.Inputs[0].SHA256 = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}, "DIGEST_MISMATCH"},
	} {
		t.Run(vector.name, func(t *testing.T) {
			request := base
			request.Inputs = append([]Input(nil), base.Inputs...)
			vector.mutate(&request)
			response := boundResponse(t, canonicalFrame(t, request), request)
			if response.Status != "REJECTED" || response.Reason != vector.want || len(response.Facts) != 0 {
				t.Fatalf("response=%+v", response)
			}
		})
	}
	request := shaderRequest("#version 300 es\nvoid main(){}\n")
	frame := canonicalFrame(t, request)
	var candidate output
	if err := json.Unmarshal(Analyze(frame), &candidate); err != nil || len(candidate.Facts) == 0 {
		t.Fatalf("candidate=%+v err=%v", candidate, err)
	}
	changed := candidate.Facts[0]
	changed.Span.End.Byte++
	if evidence(request, changed) == candidate.Facts[0].EvidenceSHA256 {
		t.Fatal("span mutation preserved evidence digest")
	}
}

func boundResponse(t *testing.T, frame []byte, request Request) responseBinding {
	t.Helper()
	var result responseBinding
	if err := json.Unmarshal(Analyze(frame), &result); err != nil {
		t.Fatal(err)
	}
	if result.Profile != Profile || result.Family != Family || result.ShaderProfile != request.ShaderProfile || result.ShaderLanguage != request.ShaderLanguage || result.ShaderVersion != request.ShaderVersion || result.ShaderToolchain != request.ShaderToolchain || result.RequestID != request.RequestID || result.RequestSHA256 != requestDigest(frame) || result.ScopeID != request.ScopeID || result.CompilationUnitID != request.CompilationUnitID || !reflect.DeepEqual(result.Target, request.Target) {
		t.Fatalf("incomplete request binding: %+v", result)
	}
	if len(result.InputEchoes) != len(request.Inputs) {
		t.Fatalf("input echoes=%+v", result.InputEchoes)
	}
	for index := range request.Inputs {
		if result.InputEchoes[index] != (Echo{Handle: request.Inputs[index].Handle, SHA256: request.Inputs[index].SHA256}) {
			t.Fatalf("input echo[%d]=%+v", index, result.InputEchoes[index])
		}
	}
	return result
}

func replaceInputBytes(source string) func(*Request) {
	return func(request *Request) {
		bytes := []byte(source)
		sum := sha256.Sum256(bytes)
		request.Inputs[0].ContentBase64 = base64.StdEncoding.EncodeToString(bytes)
		request.Inputs[0].SHA256 = "sha256:" + hex.EncodeToString(sum[:])
	}
}
