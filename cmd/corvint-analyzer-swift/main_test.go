package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/analyzerswift"
)

const swiftRefusal = `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"NONCANONICAL_REQUEST"}` + "\n"

func TestCLIInvocationFramingAndRefusal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows deliberately rejects descriptor acquisition")
	}
	binary := buildCLI(t)

	wire := swiftRequest(t)
	descriptor := filepath.Join(t.TempDir(), "request.json")
	if err := os.WriteFile(descriptor, wire, 0o600); err != nil {
		t.Fatal(err)
	}
	output, code := invokeCLI(t, binary, "--request-file", descriptor)
	want := analyzerswift.Analyze(bytes.NewReader(wire))
	if code != 0 || !bytes.Equal(output, want) || bytes.Count(output, []byte{'\n'}) != 1 {
		t.Fatalf("candidate exit=%d\nwant=%s\ngot=%s", code, want, output)
	}
	if digest := sha256.Sum256(output); hex.EncodeToString(digest[:]) != "f63f9eaae4481d69f1978083693e49bdde91c76ed68f1b45d50b95a756457d6c" {
		t.Fatalf("candidate digest=%x", digest)
	}

	output, code = invokeCLI(t, binary)
	if code != 1 || string(output) != swiftRefusal {
		t.Fatalf("missing descriptor exit=%d output=%q", code, output)
	}

	oversize := filepath.Join(t.TempDir(), "oversize.json")
	if err := os.WriteFile(oversize, bytes.Repeat([]byte{'x'}, 1_500_001), 0o600); err != nil {
		t.Fatal(err)
	}
	output, code = invokeCLI(t, binary, "--request-file", oversize)
	if code != 1 || string(output) != swiftRefusal {
		t.Fatalf("oversize exit=%d output=%q", code, output)
	}
}

func buildCLI(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "corvint-analyzer-swift")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "build", "-trimpath", "-buildvcs=false", "-o", binary, ".")
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v (context=%v)\n%s", err, ctx.Err(), output)
	}
	return binary
}

func invokeCLI(t *testing.T, binary string, args ...string) ([]byte, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, args...)
	output, err := command.Output()
	if ctx.Err() != nil {
		t.Fatal(ctx.Err())
	}
	if err == nil {
		return output, 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return output, exit.ExitCode()
	}
	t.Fatal(err)
	return nil, -1
}

func swiftRequest(t *testing.T) []byte {
	t.Helper()
	testdata := filepath.Join("..", "..", "internal", "analyzerswift", "testdata")
	coordinate := readSwiftFixture(t, filepath.Join(testdata, "beamfall-apple-8588", "coordinates.txt"))
	source := readSwiftFixture(t, filepath.Join(testdata, "beamfall-apple-8588", "Sources", "BeamfallA11y", "A11yID.swift"))
	uiPackage := readSwiftFixture(t, filepath.Join(testdata, "beamfall-apple-ui-830a", "Package.swift"))
	request := analyzerswift.Request{
		Profile: analyzerswift.Profile, Family: analyzerswift.Family, RequestID: "swift-apple-8588", ScopeID: "beamfall-apple", CompilationUnitID: "a11y",
		Target: analyzerswift.Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}},
		Inputs: []analyzerswift.Input{
			swiftInput("apple-coordinates", "swift.apple.coordinates", "fixtures/beamfall-apple-8588cec3/coordinates.txt", coordinate),
			swiftInput("apple-source", "swift.source", "Sources/BeamfallA11y/A11yID.swift", source),
			swiftInput("apple-ui", "swift.apple-ui.package", "Package.swift", uiPackage),
		},
	}
	value, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return append(value, '\n')
}

func swiftInput(handle, family, path string, content []byte) analyzerswift.Input {
	digest := sha256.Sum256(content)
	return analyzerswift.Input{Handle: handle, Family: family, Path: path, SHA256: "sha256:" + hex.EncodeToString(digest[:]), ContentBase64: base64.StdEncoding.EncodeToString(content)}
}

func readSwiftFixture(t *testing.T, path string) []byte {
	t.Helper()
	value, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
