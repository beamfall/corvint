package analyzerruby

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// benchRequest is the four-input shape the allocation ceiling is measured
// against: every manifest record plus one source file.
func benchRequest() []byte {
	return makeRequest(
		makeInput("v", familyVersion, ".ruby-version", "3.2.1\n"),
		gemfileInput(),
		lockInput(),
		sourceInput("spec/thing_spec.rb", "RSpec.describe Thing do\n  it \"works\" do\n  end\nend\n"),
	)
}

func BenchmarkAnalyzeCandidate(b *testing.B) {
	raw := benchRequest()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if out := AnalyzeCanonical(raw); len(out) == 0 {
			b.Fatal("empty response")
		}
	}
}

// TestHardAllocationRatchets enforces the ACP-009 per-family allocation budget
// recorded in docs/specs/analyzer-candidate-profiles.md for `analyzerruby`.
func TestHardAllocationRatchets(t *testing.T) {
	if raceBuild {
		t.Skip("race instrumentation is outside the absolute allocation profile")
	}
	raw := benchRequest()
	result := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			AnalyzeCanonical(raw)
		}
	})
	if got := result.AllocedBytesPerOp(); got > MaxBytesPerOp {
		t.Errorf("production bytes/op=%d max=%d", got, MaxBytesPerOp)
	}
	if got := result.AllocsPerOp(); got > MaxAllocsPerOp {
		t.Errorf("production allocs/op=%d max=%d", got, MaxAllocsPerOp)
	}
	t.Logf("ACP-009 allocation sample: %d B/op (max %d), %d allocs/op (max %d)",
		result.AllocedBytesPerOp(), MaxBytesPerOp, result.AllocsPerOp(), MaxAllocsPerOp)
}

// buildCLI builds the candidate command with the ACP-009 recipe.
func buildCLI(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "corvint-analyzer-ruby")
	build := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"),
		"build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w -buildid=", "-o", binary, "./cmd/corvint-analyzer-ruby")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return binary
}

func runCLI(t *testing.T, binary string, args []string, stdin []byte) (string, int) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Stdin = strings.NewReader(string(stdin))
	cmd.Env = []string{}
	out, err := cmd.Output()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	return string(out), code
}

// TestBuiltCLIEmitsExactLFAndRejectsFileArguments pins the ACP-001/007
// contract: the command speaks only stdin/stdout, answers with exactly one
// LF-terminated object, and takes no path argument.
func TestBuiltCLIEmitsExactLFAndRejectsFileArguments(t *testing.T) {
	binary := buildCLI(t)

	out, code := runCLI(t, binary, nil, makeRequest(gemfileInput()))
	if code != 0 {
		t.Errorf("exit=%d want 0", code)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Errorf("response is not exactly one LF-terminated line: %q", out)
	}
	if !strings.Contains(out, `"status":"CANDIDATE"`) {
		t.Errorf("want CANDIDATE, got %q", out)
	}

	// A path argument is refused rather than opened; the rejection frame exits
	// 0 (ACP-011, decision 0233).
	fixture := filepath.Join(t.TempDir(), "Gemfile")
	if err := os.WriteFile(fixture, []byte(fixtureGemfile), 0o600); err != nil {
		t.Fatal(err)
	}
	argOut, argCode := runCLI(t, binary, []string{fixture}, nil)
	if argCode != 0 {
		t.Errorf("exit=%d want 0 for a file argument", argCode)
	}
	if !strings.Contains(argOut, `"status":"REJECTED"`) {
		t.Errorf("want REJECTED for a file argument, got %q", argOut)
	}
}

// TestBuiltCLIMatchesInProcessBytes proves the command adds no framing of its
// own: the process output is byte-identical to the library result.
func TestBuiltCLIMatchesInProcessBytes(t *testing.T) {
	binary := buildCLI(t)
	raw := benchRequest()
	out, _ := runCLI(t, binary, nil, raw)
	if want := string(AnalyzeCanonical(raw)); out != want {
		t.Fatalf("CLI bytes differ from library bytes:\n cli=%q\n lib=%q", out, want)
	}
}

// TestOversizedRequestRejectsWithoutDecoding keeps the bounded reader honest.
func TestOversizedRequestRejectsWithoutDecoding(t *testing.T) {
	oversized := make([]byte, MaxRequestBytes+1)
	for i := range oversized {
		oversized[i] = ' '
	}
	oversized[len(oversized)-1] = '\n'
	mustReject(t, oversized, "NONCANONICAL_REQUEST")
}

// TestAcceptedPlatformAtomPositiveControl is the positive control paired with
// the punctuated-platform rejection: `x86_64-linux` is inside RUBY_PLATFORM.
func TestAcceptedPlatformAtomPositiveControl(t *testing.T) {
	body := strings.Replace(fixtureLock, "PLATFORMS\n  ruby\n", "PLATFORMS\n  ruby\n  x86_64-linux\n", 1)
	facts := mustAccept(t, makeRequest(gemfileInput(), makeInput("lock", familyLock, "Gemfile.lock", body)))
	if !hasTuple(facts, "ruby.platform.locked|lock|-|ruby|locks-platform|x86_64-linux|root") {
		t.Fatalf("x86_64-linux platform fact missing; got %v", tuples(facts))
	}
}
