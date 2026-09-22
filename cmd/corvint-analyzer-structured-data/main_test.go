package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/analyzerstructured"
)

func cliFrame(t *testing.T, requestID string) []byte {
	t.Helper()
	body := []byte(`{"name":"Beamfall","display":"standalone"}`)
	sum := sha256.Sum256(body)
	request := analyzerstructured.Request{Profile: analyzerstructured.Profile, Family: analyzerstructured.Family, RequestID: requestID, ScopeID: "root", CompilationUnitID: "unit-1", Target: analyzerstructured.Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []analyzerstructured.Input{{Handle: "input-1", Family: analyzerstructured.WebManifestProfile, Path: "web/manifest.webmanifest", SHA256: "sha256:" + hex.EncodeToString(sum[:]), ContentBase64: base64.StdEncoding.EncodeToString(body)}}}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}

func buildCandidate(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "corvint-analyzer-structured-data")
	command := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-o", binary, "./cmd/corvint-analyzer-structured-data")
	command.Dir = root
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build candidate: %v: %s", err, output)
	}
	return binary
}

// TestFreshBuiltCLIPermutations launches one freshly built candidate process
// for each distinct canonical request. It proves no package-global replay state
// leaks across process boundaries; every request/output pair is unique.
func TestFreshBuiltCLIPermutations(t *testing.T) {
	binary := buildCandidate(t)
	for index := 0; index < 1001; index++ {
		frame := cliFrame(t, fmt.Sprintf("request-%04d", index))
		command := exec.Command(binary)
		command.Stdin = bytes.NewReader(frame)
		output, err := command.Output()
		if err != nil || !bytes.Contains(output, []byte(`"status":"CANDIDATE"`)) || !bytes.Contains(output, []byte(fmt.Sprintf(`"request_id":"request-%04d"`, index))) {
			t.Fatalf("fresh run %d: err=%v output=%s", index, err, output)
		}
	}
}

func TestCLIFramingReplayAndWriteAll(t *testing.T) {
	frame := cliFrame(t, "request-0000")
	if !bytes.Equal(analyzerstructured.Analyze(frame), analyzerstructured.Analyze(append([]byte(nil), frame...))) {
		t.Fatal("replay drift")
	}
	if output := analyzerstructured.Analyze(append([]byte(" "), frame...)); !bytes.Contains(output, []byte(`"reason":"NONCANONICAL_REQUEST"`)) {
		t.Fatalf("noncanonical=%s", output)
	}
	writer := &partialWriter{limit: 1}
	if err := writeAll(writer, []byte("exact")); err != nil || string(writer.bytes) != "exact" {
		t.Fatalf("full write err=%v bytes=%q", err, writer.bytes)
	}
	if err := writeAll(&partialWriter{}, []byte("x")); err != io.ErrShortWrite {
		t.Fatalf("zero write=%v", err)
	}
}

// run exits nonzero only when the frame is not written in full.
func TestRunReturnsNonzeroWhenFrameIsNotWritten(t *testing.T) {
	frame := cliFrame(t, "request-0000")
	stdin := func() *os.File {
		path := filepath.Join(t.TempDir(), "stdin")
		if err := os.WriteFile(path, frame, 0o600); err != nil {
			t.Fatal(err)
		}
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { file.Close() })
		return file
	}
	for _, writer := range []io.Writer{failingWriter{}, &partialWriter{}} {
		if got := run(nil, stdin(), writer); got != 1 {
			t.Fatalf("writer=%T status=%d", writer, got)
		}
	}
	full := &partialWriter{limit: 1}
	if got := run(nil, stdin(), full); got != 0 || !bytes.Equal(full.bytes, analyzerstructured.Analyze(frame)) {
		t.Fatalf("full write status=%d bytes=%q", got, full.bytes)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestDescriptorIsStableAndNamesEveryTuple(t *testing.T) {
	first, err := analyzerstructured.CandidateDescriptor()
	if err != nil {
		t.Fatal(err)
	}
	second, err := analyzerstructured.CandidateDescriptor()
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("descriptor drift: %v", err)
	}
	const frozen = `{"protocol":"corvint-structured-data-protocol/1","profile":"corvint-structured-data/experimental-v1","family":"structured-data","format_profiles":["structured.json/rfc8259-v1","structured.jsonl/rfc8259-v1","structured.yaml/1.2-core-v1","structured.toml/1.0.0-v1","structured.xml/1.0-v1","structured.plist/xml-v1","structured.properties/java-17-v1","structured.hcl/2.0-static-v1","structured.webmanifest/whatwg-v1","structured.svg/1.1-static-v1"],"fact_schema":"structured-coordinate/v1","limits":{"frame_bytes":1500000,"base64_bytes":87384,"input_bytes":65536,"inputs":64,"facts":64,"output_bytes":65536,"depth":32,"tokens":4096,"string_bytes":8192},"identity_sha256":"sha256:d2691f3669ebbcf95240939ae2931e25df374e3a05dea9e9610ca3f50bc47be9"}` + "\n"
	if string(first) != frozen {
		t.Fatalf("descriptor drift:\n got %s\nwant %s", first, frozen)
	}
}

// TestBuiltCLIReportsReadFailureAsNoncanonical pins ACP-012: a stdin that
// cannot be read (a directory) is the NONCANONICAL_REQUEST sentinel, exit 0.
func TestBuiltCLIReportsReadFailureAsNoncanonical(t *testing.T) {
	directory, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	command := exec.Command(buildCandidate(t))
	command.Stdin = directory
	output, err := command.Output()
	if err != nil || string(output) != string(noncanonical) {
		t.Fatalf("err=%v output=%q", err, output)
	}
}

type partialWriter struct {
	limit int
	bytes []byte
}

func (w *partialWriter) Write(data []byte) (int, error) {
	if w.limit == 0 {
		return 0, nil
	}
	size := w.limit
	if size > len(data) {
		size = len(data)
	}
	w.bytes = append(w.bytes, data[:size]...)
	return size, nil
}
