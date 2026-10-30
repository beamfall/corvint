//go:build !race

package analyzerkotlinandroid

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"runtime"
	"testing"
	"time"
	"unicode/utf8"
)

// The normal-build ratchet is intentionally excluded from -race: race
// instrumentation changes every allocation and is not performance evidence.
// Allocation caps are causal against the restored decoder. The 2x latency
// guard pairs and alternates calls so host load cannot bias disjoint windows;
// it remains deliberately non-promotional because host scheduling is noisy.
func TestRepresentativeAllocationRatchetAndRestoredDecoder(t *testing.T) {
	previous := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(previous)
	raw := canonical(t, pinned(t))
	request := pinned(t)
	candidate := testing.Benchmark(func(b *testing.B) {
		for index := 0; index < b.N; index++ {
			if _, reason := decode(request); reason != "" {
				b.Fatal(reason)
			}
		}
	})
	restored := testing.Benchmark(func(b *testing.B) {
		for index := 0; index < b.N; index++ {
			if _, reason := restoredDecode(request); reason != "" {
				b.Fatal(reason)
			}
		}
	})
	t.Logf("candidate=%d ns/op %d B/op %d allocs/op restored=%d ns/op %d B/op %d allocs/op", candidate.NsPerOp(), candidate.AllocedBytesPerOp(), candidate.AllocsPerOp(), restored.NsPerOp(), restored.AllocedBytesPerOp(), restored.AllocsPerOp())
	if candidate.AllocedBytesPerOp() >= restored.AllocedBytesPerOp() || candidate.AllocsPerOp() >= restored.AllocsPerOp() {
		t.Fatalf("candidate decoder did not beat restored decoder: candidate=%d B/op %d allocs/op restored=%d B/op %d allocs/op", candidate.AllocedBytesPerOp(), candidate.AllocsPerOp(), restored.AllocedBytesPerOp(), restored.AllocsPerOp())
	}
	candidateLatency, restoredLatency := pairedDecodeLatency(t, request, 512)
	t.Logf("paired candidate=%s restored=%s", candidateLatency, restoredLatency)
	if candidateLatency > restoredLatency*2 {
		t.Fatalf("candidate latency regression: candidate=%s restored=%s", candidateLatency, restoredLatency)
	}
	if got, reason := processWithRestoredDecoder(raw); reason != "" || string(got) != string(mustProcess(t, raw)) {
		t.Fatalf("restored decoder parity reason=%q", reason)
	}
	if restored.AllocsPerOp() == 0 || restored.AllocedBytesPerOp() == 0 || restored.NsPerOp() == 0 {
		t.Fatalf("restored decoder was not measured: %+v", restored)
	}
}

func pairedDecodeLatency(t *testing.T, request Request, pairs int) (time.Duration, time.Duration) {
	t.Helper()
	var candidate, restored time.Duration
	for index := 0; index < pairs; index++ {
		if index%2 == 0 {
			candidate += measuredDecode(t, request, decode)
			restored += measuredDecode(t, request, restoredDecode)
			continue
		}
		restored += measuredDecode(t, request, restoredDecode)
		candidate += measuredDecode(t, request, decode)
	}
	return candidate, restored
}

func measuredDecode(t *testing.T, request Request, decoder func(Request) ([][]byte, string)) time.Duration {
	t.Helper()
	started := time.Now()
	_, reason := decoder(request)
	elapsed := time.Since(started)
	if reason != "" {
		t.Fatal(reason)
	}
	return elapsed
}

// processWithRestoredDecoder is the exact pre-change DecodeString decoder and
// its post-allocation aggregate limit, retained only as a causal control.
func processWithRestoredDecoder(raw []byte) ([]byte, string) {
	if len(raw) > MaxRequestBytes {
		return rejectSentinel("LIMIT_EXCEEDED"), ""
	}
	if len(raw) < 2 || raw[len(raw)-1] != '\n' || !utf8.Valid(raw) || scanCanonicalJSON(raw[:len(raw)-1]) != "" {
		return rejectSentinel("NONCANONICAL_REQUEST"), ""
	}
	payload := raw[:len(raw)-1]
	if hasUnknownField(payload) {
		return extension(payload), ""
	}
	var request Request
	if err := json.Unmarshal(payload, &request); err != nil {
		return rejectSentinel("NONCANONICAL_REQUEST"), ""
	}
	canonical, err := json.Marshal(request)
	if err != nil || string(canonical) != string(payload) {
		return reject(request, "NONCANONICAL_REQUEST"), ""
	}
	if reason := validate(request); reason != "" {
		return reject(request, reason), ""
	}
	contents, reason := restoredDecode(request)
	if reason != "" {
		return reject(request, reason), ""
	}
	facts, reason := factsFor(request, contents)
	if reason != "" {
		return reject(request, reason), ""
	}
	if reason := identityRecheck(request, contents); reason != "" {
		return reject(request, reason), ""
	}
	output, ok := encodeSuccess(request, facts)
	if !ok {
		return reject(request, "OUTPUT_LIMIT"), ""
	}
	return output, ""
}

func restoredDecode(request Request) ([][]byte, string) {
	contents, total := make([][]byte, 0, len(request.Inputs)), 0
	for _, input := range request.Inputs {
		if len(input.ContentBase64) > 1_398_104 {
			return nil, "LIMIT_EXCEEDED"
		}
		content, err := base64.StdEncoding.DecodeString(input.ContentBase64)
		if err != nil || base64.StdEncoding.EncodeToString(content) != input.ContentBase64 {
			return nil, "MALFORMED_INPUT"
		}
		if total > MaxInputBytes-len(content) {
			return nil, "LIMIT_EXCEEDED"
		}
		sum := sha256.Sum256(content)
		if input.SHA256 != "sha256:"+hex.EncodeToString(sum[:]) {
			return nil, "DIGEST_MISMATCH"
		}
		total += len(content)
		contents = append(contents, content)
	}
	return contents, ""
}
