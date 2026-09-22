package analyzershader

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"runtime/debug"
	"strconv"
	"strings"
	"testing"
)

const pinnedPulseMetalPath = "testdata/metal/Sources/BeamfallAppleLab/Shaders/Pulse.metal"

func TestRepresentativePinnedSourceAndAllocationRatchets(t *testing.T) {
	if raceBuild() {
		t.Skip("allocation ratchets are measured on the non-race candidate binary")
	}
	frame := representativePulseMetalFrame(t)
	var result output
	if err := json.Unmarshal(Analyze(frame), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "CANDIDATE" || len(result.Facts) != 7 {
		t.Fatalf("representative result=%+v", result)
	}
	allocations := testing.AllocsPerRun(25, func() { _ = Analyze(frame) })
	const allocationCeiling = 1400
	if allocations > allocationCeiling {
		t.Fatalf("representative allocations=%f ceiling=%d", allocations, allocationCeiling)
	}
}

// TestGLSLDirectiveDenseProductionRatchets uses the public production entry
// point on a valid 45 KiB grammar fixture. The restored comparator takes the
// same production frame through the removed offset walk before the production
// entry point. This is an allocation control, not a latency claim.
func TestGLSLDirectiveDenseProductionRatchets(t *testing.T) {
	if raceBuild() {
		t.Skip("allocation ratchets are measured on the non-race candidate binary")
	}
	short := directiveDenseFrame(512)
	long := directiveDenseFrame(1024)
	if bytes := len(directiveDenseSource(1024)); bytes < 45_000 || bytes > 48<<10 {
		t.Fatalf("fixture source bytes=%d", bytes)
	}
	for _, frame := range [][]byte{short, long} {
		var result struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
		}
		if err := json.Unmarshal(Analyze(frame), &result); err != nil || result.Status != "CANDIDATE" {
			t.Fatalf("valid production fixture=%+v err=%v", result, err)
		}
	}
	measure := func(frame []byte) testing.BenchmarkResult {
		return testing.Benchmark(func(b *testing.B) {
			for index := 0; index < b.N; index++ {
				_ = Analyze(frame)
			}
		})
	}
	shortResult := measure(short)
	longResult := measure(long)
	if longResult.AllocedBytesPerOp() > 8_500_000 || longResult.AllocsPerOp() > 30_000 {
		t.Fatalf("production long bytes/op=%d allocs/op=%d", longResult.AllocedBytesPerOp(), longResult.AllocsPerOp())
	}
	if longResult.AllocedBytesPerOp() > shortResult.AllocedBytesPerOp()*3 || longResult.AllocsPerOp() > shortResult.AllocsPerOp()*3 {
		t.Fatalf("nonlinear production allocation short=(%d B/op,%d allocs/op) long=(%d B/op,%d allocs/op)", shortResult.AllocedBytesPerOp(), shortResult.AllocsPerOp(), longResult.AllocedBytesPerOp(), longResult.AllocsPerOp())
	}
	legacy := testing.Benchmark(func(b *testing.B) {
		for index := 0; index < b.N; index++ {
			_ = restoredProductionAnalyze(long)
		}
	})
	if got := restoredProductionAnalyze(long); !bytes.Equal(got, Analyze(long)) {
		t.Fatal("restored comparator changed the production receipt")
	}
	if legacy.AllocedBytesPerOp() <= longResult.AllocedBytesPerOp()*8 {
		t.Fatalf("restored production comparator escaped allocation control legacy=%d candidate=%d", legacy.AllocedBytesPerOp(), longResult.AllocedBytesPerOp())
	}
	t.Logf("directive production allocation: short=%d B/op %d allocs/op long=%d B/op %d allocs/op restored=%d B/op %d allocs/op", shortResult.AllocedBytesPerOp(), shortResult.AllocsPerOp(), longResult.AllocedBytesPerOp(), longResult.AllocsPerOp(), legacy.AllocedBytesPerOp(), legacy.AllocsPerOp())
}

func directiveDenseFrame(defines int) []byte {
	source := directiveDenseSource(defines)
	return frame("beamfall.glsl-es-3.00.fragment.visual-shaders", "shader.glsl", source)
}
func directiveDenseSource(defines int) string {
	var source strings.Builder
	source.Grow(64 + defines*45)
	source.WriteString("#version 300 es\n")
	for index := 0; index < defines; index++ {
		source.WriteString("#define MACRO")
		source.WriteString(strconv.Itoa(index))
		source.WriteString(" STRUCTURAL_VALUE_123456789\n")
	}
	source.WriteString("void main(){}\n")
	return source.String()
}

// restoredQuadraticDirectiveOffsets is the exact removed lineStart strategy,
// kept only as a negative control. It intentionally converts each remaining
// suffix while walking from byte zero for every directive.
func restoredProductionAnalyze(frame []byte) []byte {
	var request Request
	if decode(frame, &request) != "" {
		return Analyze(frame)
	}
	decoded, reason := contents(request)
	if reason != "" {
		return Analyze(frame)
	}
	for _, input := range request.Inputs {
		restoredQuadraticDirectiveOffsets(string(decoded[input.Handle]))
	}
	return Analyze(frame)
}

func restoredQuadraticDirectiveOffsets(source string) {
	bytes := []byte(source)
	lines := strings.Split(source, "\n")
	for line := range lines {
		start := 0
		for count := 0; count < line; count++ {
			next := strings.IndexByte(string(bytes[start:]), '\n')
			if next < 0 {
				break
			}
			start += next + 1
		}
	}
}

func BenchmarkAnalyzeRepresentativePinnedPulseMetal(b *testing.B) {
	frame := representativePulseMetalFrame(b)
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_ = Analyze(frame)
	}
}

func representativePulseMetalFrame(t testing.TB) []byte {
	t.Helper()
	source, err := fs.ReadFile(corpusFixtures, pinnedPulseMetalPath)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(source)
	tuple := profiles["beamfall.metal-3.1.fragment.apple-ui"]
	request := Request{Profile: Profile, Family: Family, ShaderProfile: "beamfall.metal-3.1.fragment.apple-ui", ShaderLanguage: tuple.language, ShaderVersion: tuple.version, ShaderToolchain: tuple.toolchain, RequestID: "representative-pulse", ScopeID: "beamfall", CompilationUnitID: "pulse-fragment", Target: Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []Input{{Handle: "pulse", Family: "shader.metal", Path: "Sources/BeamfallAppleLab/Shaders/Pulse.metal", SHA256: "sha256:" + hex.EncodeToString(sum[:]), ContentBase64: base64.StdEncoding.EncodeToString(source)}}}
	frame, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return append(frame, '\n')
}

func raceBuild() bool {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return false
	}
	for _, setting := range info.Settings {
		if setting.Key == "-race" && setting.Value == "true" {
			return true
		}
	}
	return false
}
