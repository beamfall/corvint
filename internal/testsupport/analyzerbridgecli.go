package testsupport

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// CheckAnalyzerBridgeCLI exercises a native-bridge command from its own cmd
// package directory. Each caller retains its own executable, version and goldens.
func CheckAnalyzerBridgeCLI(t *testing.T, family string) {
	t.Helper()
	binary := buildAnalyzerBridgeCLI(t, family)

	output, code := invokeAnalyzerBridgeCLI(t, binary, []string{"--version"}, nil)
	if code != 0 || string(output) != "corvint-analyzer-"+family+"/v0\n" {
		t.Fatalf("version exit=%d output=%q", code, output)
	}

	request := readAnalyzerBridgeFixture(t, family+".request.json")
	want := readAnalyzerBridgeFixture(t, family+".response.json")
	output, code = invokeAnalyzerBridgeCLI(t, binary, nil, request)
	if code != 0 || !bytes.Equal(output, want) || bytes.Count(output, []byte{'\n'}) != 1 {
		t.Fatalf("candidate exit=%d\nwant=%s\ngot=%s", code, want, output)
	}

	const limit = `{"profile":"corvint-analyzer-native-bridge/v0","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"LIMIT_EXCEEDED"}` + "\n"
	output, code = invokeAnalyzerBridgeCLI(t, binary, nil, []byte(strings.Repeat("x", 1_500_000)+"\n"))
	if code != 0 || string(output) != limit {
		t.Fatalf("oversize exit=%d output=%q", code, output)
	}

	// NJB-008: argv other than a sole --version refuses with the sentinel, exit 0, and never
	// analyzes the valid request on stdin.
	const noncanonical = `{"profile":"corvint-analyzer-native-bridge/v0","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"NONCANONICAL_REQUEST"}` + "\n"
	for _, args := range [][]string{{"--version", "extra"}, {"--input-file", "request.json"}} {
		output, code = invokeAnalyzerBridgeCLI(t, binary, args, request)
		if code != 0 || string(output) != noncanonical {
			t.Fatalf("args=%q exit=%d output=%q", args, code, output)
		}
	}
}

func buildAnalyzerBridgeCLI(t *testing.T, family string) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "corvint-analyzer-"+family)
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "build", "-trimpath", "-buildvcs=false", "-o", binary, ".")
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v (context=%v)\n%s", err, ctx.Err(), output)
	}
	return binary
}

func invokeAnalyzerBridgeCLI(t *testing.T, binary string, args []string, input []byte) ([]byte, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, args...)
	command.Stdin = bytes.NewReader(input)
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

func readAnalyzerBridgeFixture(t *testing.T, name string) []byte {
	t.Helper()
	value, err := os.ReadFile(filepath.Join("..", "..", "internal", "analyzernativebridge", "testdata", "bridge-goldens", name))
	if err != nil {
		t.Fatal(err)
	}
	return value
}
