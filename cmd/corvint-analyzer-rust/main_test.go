package main

import (
	"bytes"
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// TestVersionReceiptNamesUnsupportedCEMOCM pins RAC-010: the receipt must state
// both states as UNSUPPORTED and must not claim selectability.
func TestVersionReceiptNamesUnsupportedCEMOCM(t *testing.T) {
	var out bytes.Buffer
	if code := run([]string{"--version"}, strings.NewReader(""), &out); code != 0 {
		t.Fatalf("exit=%d", code)
	}
	var receipt struct {
		Profile string `json:"profile"`
		Family  string `json:"family"`
		Status  string `json:"status"`
		CEM     string `json:"cem"`
		OCM     string `json:"ocm"`
	}
	body := out.Bytes()
	if body[len(body)-1] != '\n' {
		t.Fatalf("receipt is not LF framed: %q", body)
	}
	if err := json.Unmarshal(body[:len(body)-1], &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Family != "rust" || receipt.Status != "EXPERIMENTAL_UNSELECTABLE" ||
		receipt.CEM != "UNSUPPORTED" || receipt.OCM != "UNSUPPORTED" {
		t.Fatalf("receipt=%s", body)
	}
}

func TestUnexpectedArgumentsReject(t *testing.T) {
	for _, args := range [][]string{{"analyze"}, {"--version", "extra"}, {"-x"}} {
		var out bytes.Buffer
		if code := run(args, strings.NewReader(""), &out); code != 0 {
			t.Fatalf("args=%v exit=%d", args, code)
		}
		if !strings.Contains(out.String(), `"reason":"NONCANONICAL_REQUEST"`) {
			t.Fatalf("args=%v output=%s", args, out.String())
		}
	}
}

func TestStdinAnalysisEmitsOneFrame(t *testing.T) {
	request := `{"profile":"corvint-analyzer-candidate/experimental","family":"rust","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input-1","family":"rust.source","path":"src/lib.rs","sha256":"sha256:1d229271928d3f9e2bb0375bd6ce5db6c6d348d9e79db7f0e4b1e0c1b8d0e6f6","content_base64":"Zm4gZigpIHt9Cg=="}]}` + "\n"
	var out bytes.Buffer
	run(nil, strings.NewReader(request), &out)
	body := out.Bytes()
	if len(body) == 0 || body[len(body)-1] != '\n' || bytes.Count(body, []byte("\n")) != 1 {
		t.Fatalf("output is not exactly one LF frame: %q", body)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body[:len(body)-1], &decoded); err != nil {
		t.Fatalf("output is not canonical JSON: %q", body)
	}
	if decoded["family"] != "rust" {
		t.Fatalf("family=%v", decoded["family"])
	}
}

// TestOversizeStdinIsBounded proves the command stops reading past its wire
// ceiling rather than buffering an unbounded stream.
func TestOversizeStdinIsBounded(t *testing.T) {
	var out bytes.Buffer
	if code := run(nil, strings.NewReader(strings.Repeat("x", 2_000_000)), &out); code != 0 || out.String() != string(rejectFrame("LIMIT_EXCEEDED")) {
		t.Fatalf("oversize stdin exit=%d output=%s", code, out.String())
	}
}

// TestReadFailureIsNoncanonicalSentinel pins ACP-012: a stdin read failure is
// the NONCANONICAL_REQUEST sentinel with exit 0.
func TestReadFailureIsNoncanonicalSentinel(t *testing.T) {
	var out bytes.Buffer
	if code := run(nil, failingReader{}, &out); code != 0 || out.String() != string(rejectFrame("NONCANONICAL_REQUEST")) {
		t.Fatalf("read failure exit=%d output=%s", code, out.String())
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, os.ErrClosed }

// TestWriteErrorIsReported pins the full-write error path.
func TestWriteErrorIsReported(t *testing.T) {
	if code := run([]string{"--version"}, strings.NewReader(""), failingWriter{}); code != 1 {
		t.Fatalf("exit=%d want 1", code)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, os.ErrClosed }

// TestNoAmbientCapabilityImports is the containment guard. It asserts the exact
// import set of every production file rather than scanning for substrings: the
// analyzer package may reach only pure-computation stdlib, and the command adds
// only the three packages it needs to move bytes between stdin and stdout.
func TestNoAmbientCapabilityImports(t *testing.T) {
	pure := map[string]bool{
		"bytes": true, "crypto/sha256": true, "encoding/base64": true,
		"encoding/binary": true, "encoding/hex": true, "encoding/json": true,
		"regexp": true, "sort": true, "strings": true, "unicode/utf8": true,
	}
	command := map[string]bool{
		"fmt": true, "io": true, "os": true,
		"github.com/Beamfall/corvint/internal/analyzerrust": true,
	}
	root := moduleRoot(t)
	check := func(dir string, allowed map[string]bool) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, spec := range parsed.Imports {
				path, err := strconv.Unquote(spec.Path.Value)
				if err != nil {
					t.Fatal(err)
				}
				if !allowed[path] {
					t.Errorf("%s imports %q, which is outside the candidate capability boundary", name, path)
				}
			}
		}
	}
	check(filepath.Join(root, "internal", "analyzerrust"), pure)
	check(filepath.Join(root, "cmd", "corvint-analyzer-rust"), command)
}

// TestCommandTouchesNoAmbientState pins the command's use of os: it may read
// argv and the standard streams and set an exit code, and nothing else.
func TestCommandTouchesNoAmbientState(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(moduleRoot(t), "cmd", "corvint-analyzer-rust", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{
		"os.Getenv", "os.Environ", "os.Open", "os.ReadFile", "os.WriteFile",
		"os.Chdir", "os.UserHomeDir", "os.LookupEnv", "os.Create", "os.Remove",
	} {
		if bytes.Contains(body, []byte(banned)) {
			t.Errorf("main.go references %q", banned)
		}
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	root, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}
