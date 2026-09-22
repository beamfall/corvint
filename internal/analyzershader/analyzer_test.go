package analyzershader

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

// These literal fixtures are pinned excerpts, not runtime repository reads.
// Their source receipts identify the full Beamfall surfaces used for dogfood.
const pinnedGLSL = `#version 300 es
#define P 1
precision highp float;
layout(location=0) in vec2 aPos;
uniform sampler2D uImage;
in vec2 vUV;
out vec4 fragColor;
void main() { fragColor = texture(uImage, vUV); }
`
const pinnedMetal = `#include "VisualShared.h"
using namespace metal;
fragment float4 frag_probe(texture2d<float> image [[texture(0)]], sampler imageSampler [[sampler(0)]]) { return float4(0.0); }
`

const (
	visualSharedBlob = "bad225aa3abc4dfc576305ef219152278c040d11"
	pulseGLSLBlob    = "e5e1de9b4e478d8833818eae6a76622c89d602a6"
	commonMetalBlob  = "7e35740db98db6b053a68b7a8269ce0f6cb758a3"
	pulseMetalBlob   = "2c983e6280248d3b054db3d0f714fa8e7d436d54"
	androidGLESBlob  = "4e4299813ab46d32bc9851ed5c9e243af40f39a9"
)

func TestPinnedBeamfallReceiptConstants(t *testing.T) {
	for _, blob := range []string{visualSharedBlob, pulseGLSLBlob, commonMetalBlob, pulseMetalBlob, androidGLESBlob} {
		if len(blob) != 40 {
			t.Fatalf("nonliteral Git blob receipt %q", blob)
		}
	}
}

func TestPinnedBeamfallSurfacesAreAdmissionCandidatesOnly(t *testing.T) {
	for _, vector := range []struct{ name, profile, family, source string }{
		{"visual-shaders-a2c968b0", "beamfall.glsl-es-3.00.fragment.visual-shaders", "shader.glsl", pinnedGLSL},
		{"apple-ui-830a154d", "beamfall.metal-3.1.fragment.apple-ui", "shader.metal", pinnedMetal},
	} {
		t.Run(vector.name, func(t *testing.T) {
			out := Analyze(frame(vector.profile, vector.family, vector.source))
			if !strings.Contains(string(out), `"status":"CANDIDATE"`) || strings.Contains(string(out), `"PASS"`) {
				t.Fatalf("output=%s", out)
			}
			var decoded output
			if err := json.Unmarshal(out, &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.ShaderProfile != vector.profile || len(decoded.Facts) == 0 {
				t.Fatalf("decoded=%+v", decoded)
			}
			for _, fact := range decoded.Facts {
				if fact.Span.Start.Line == 0 || fact.WitnessBase64 == "" || !digest(fact.WitnessSHA256) || !digest(fact.EvidenceSHA256) {
					t.Fatalf("unbound fact=%+v", fact)
				}
			}
		})
	}
}

func TestFailClosedPreprocessorAndReplayBindings(t *testing.T) {
	if got := string(Analyze(frame("beamfall.glsl-es-3.00.fragment.visual-shaders", "shader.glsl", "#version 300 es\n#if 1\nvoid main(){}\n"))); !strings.Contains(got, `"reason":"UNSUPPORTED_SCHEMA"`) {
		t.Fatalf("conditional=%s", got)
	}
	if got := string(Analyze(frame("beamfall.glsl-es-3.00.fragment.visual-shaders", "shader.glsl", "#version 310 es\nvoid main(){}\n"))); !strings.Contains(got, `"reason":"EXACT_BINDING_UNAVAILABLE"`) {
		t.Fatalf("version=%s", got)
	}
	valid := frame("beamfall.glsl-es-3.00.fragment.visual-shaders", "shader.glsl", pinnedGLSL)
	mutated := strings.Replace(string(valid), "request-1", "request-2", 1)
	if got := string(Analyze([]byte(mutated))); got == string(Analyze(valid)) || !strings.Contains(got, `"request_id":"request-2"`) {
		t.Fatalf("replay mutation=%s", got)
	}
	empty := Request{Profile: Profile, Family: Family, ShaderProfile: "unknown", ShaderLanguage: "glsl", ShaderVersion: "3.00", ShaderToolchain: "glslang-16.4.0.spirv-cross-2026-07-06T12-43-32", RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []Input{}}
	emptyFrame, err := json.Marshal(empty)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(Analyze(append(emptyFrame, '\n'))); !strings.Contains(got, `"reason":"LIMIT_EXCEEDED"`) {
		t.Fatalf("full envelope=%s", got)
	}
}

func TestMetalComputeAndFunctionDeclarationGrammar(t *testing.T) {
	compute := frame("beamfall.metal-3.1.compute.apple-ui", "shader.metal", "kernel void compute_probe(device float* output [[buffer(0)]], uint index [[thread_position_in_grid]]) {}\n")
	var result output
	if err := json.Unmarshal(Analyze(compute), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "CANDIDATE" || len(result.Facts) != 2 || result.Facts[0].Value != "buffer" || result.Facts[1].Value != "compute" || result.Facts[1].Subject != "compute_probe" {
		t.Fatalf("compute=%+v", result)
	}
	for _, source := range []string{
		"fragment float4 notAFunction;\n",
		"fragment float4 prototype_probe();\n",
		"kernel void compute_prototype();\n",
		"forged kernel void compute_probe() {}\n",
	} {
		if got := rejectionReason(t, Analyze(frame("beamfall.metal-3.1.fragment.apple-ui", "shader.metal", source))); got != "UNSUPPORTED_SCHEMA" {
			t.Fatalf("source=%q reason=%s", source, got)
		}
	}
	if got := rejectionReason(t, Analyze(frame("beamfall.glsl-es-3.00.fragment.visual-shaders", "shader.glsl", "#version 300 es\nforged void main() {}\nvoid main() {}\n"))); got != "UNSUPPORTED_SCHEMA" {
		t.Fatalf("forged GLSL declaration reason=%s", got)
	}
}

func TestCandidateIsByteStableAcrossThousandFreshPermutations(t *testing.T) {
	frame := frame("beamfall.glsl-es-3.00.fragment.visual-shaders", "shader.glsl", pinnedGLSL)
	want := string(Analyze(frame))
	for i := 0; i < 1024; i++ {
		if got := string(Analyze(append([]byte(nil), frame...))); got != want {
			t.Fatalf("iteration=%d\ngot=%s\nwant=%s", i, got, want)
		}
	}
}

func BenchmarkAnalyzeCandidateSinglePass4Inputs36Facts(b *testing.B) {
	frame := fourInputFrame()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Analyze(frame)
	}
}
func TestFourInputBenchmarkFixtureHasThirtySixFacts(t *testing.T) {
	var result output
	if err := json.Unmarshal(Analyze(fourInputFrame()), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Facts) != 36 {
		t.Fatalf("facts=%d", len(result.Facts))
	}
}

// TestDuplicateVoidMainIsRejected pins that a second well-formed `void main(){`
// in one compilation unit is a duplicate-definition compile error, not a
// second candidate entry point.
func TestDuplicateVoidMainIsRejected(t *testing.T) {
	source := "#version 300 es\nvoid main() {}\nvoid main() {}\n"
	if got := rejectionReason(t, Analyze(frame("beamfall.glsl-es-3.00.fragment.visual-shaders", "shader.glsl", source))); got != "DUPLICATE_VALUE" {
		t.Fatalf("reason=%s", got)
	}
}
func BenchmarkAnalyzeCandidateTokenizedWorstCase(b *testing.B) {
	source := strings.Repeat("// comment\n", 4096) + pinnedGLSL
	frame := frame("beamfall.glsl-es-3.00.fragment.visual-shaders", "shader.glsl", source)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Analyze(frame)
	}
}
func TestAllocationRatchet(t *testing.T) {
	if raceBuild() {
		t.Skip("allocation ratchets are measured on the non-race candidate binary")
	}
	result := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = Analyze(frame("beamfall.glsl-es-3.00.fragment.visual-shaders", "shader.glsl", pinnedGLSL))
		}
	})
	if result.AllocsPerOp() > 500 || result.AllocedBytesPerOp() > 100000 {
		t.Fatalf("allocs=%d bytes=%d", result.AllocsPerOp(), result.AllocedBytesPerOp())
	}
}

func fourInputFrame() []byte {
	inputs := make([]Input, 4)
	for i := range inputs {
		source := pinnedGLSL
		handle := "input-" + string(rune('1'+i))
		sum := sha256.Sum256([]byte(source))
		inputs[i] = Input{handle, "shader.glsl", "visuals/probe" + string(rune('1'+i)) + ".glsl", "sha256:" + hex.EncodeToString(sum[:]), base64.StdEncoding.EncodeToString([]byte(source))}
	}
	tuple := profiles["beamfall.glsl-es-3.00.fragment.visual-shaders"]
	request := Request{Profile: Profile, Family: Family, ShaderProfile: "beamfall.glsl-es-3.00.fragment.visual-shaders", ShaderLanguage: tuple.language, ShaderVersion: tuple.version, ShaderToolchain: tuple.toolchain, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: inputs}
	bytes, _ := json.Marshal(request)
	return append(bytes, '\n')
}
func frame(profile, family, source string) []byte {
	sum := sha256.Sum256([]byte(source))
	input := Input{"input-1", family, "visuals/probe.glsl", "sha256:" + hex.EncodeToString(sum[:]), base64.StdEncoding.EncodeToString([]byte(source))}
	tuple := profiles[profile]
	request := Request{Profile: Profile, Family: Family, ShaderProfile: profile, ShaderLanguage: tuple.language, ShaderVersion: tuple.version, ShaderToolchain: tuple.toolchain, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []Input{input}}
	bytes, _ := json.Marshal(request)
	return append(bytes, '\n')
}
