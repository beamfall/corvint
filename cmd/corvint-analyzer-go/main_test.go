package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"github.com/Beamfall/corvint/internal/analyzergo"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

type chunkReader struct{ reader *bytes.Reader }

func (r chunkReader) Read(p []byte) (int, error) {
	if len(p) > 7 {
		p = p[:7]
	}
	return r.reader.Read(p)
}

func TestRunRejectsMalformedInput(t *testing.T) {
	var out bytes.Buffer
	if got := run(nil, bytes.NewBufferString("{}\n"), &out); got != 0 || out.String() != "{\"profile\":\"corvint-analyzer-candidate/experimental\",\"family\":\"unknown\",\"request_id\":\"unknown\",\"status\":\"REJECTED\",\"reason\":\"NONCANONICAL_REQUEST\"}\n" {
		t.Fatalf("status=%d output=%q", got, out.String())
	}
}
func TestRunAlwaysFramesReadAndSizeFailures(t *testing.T) {
	for _, tc := range []struct {
		reader io.Reader
		reason string
	}{{brokenReader{}, "NONCANONICAL_REQUEST"}, {bytes.NewReader(append(make([]byte, 1_500_000), 'x')), "LIMIT_EXCEEDED"}} {
		var out bytes.Buffer
		if got := run(nil, tc.reader, &out); got != 0 || out.String() != "{\"profile\":\"corvint-analyzer-candidate/experimental\",\"family\":\"unknown\",\"request_id\":\"unknown\",\"status\":\"REJECTED\",\"reason\":\""+tc.reason+"\"}\n" {
			t.Fatalf("status=%d output=%q", got, out.String())
		}
	}
}

type unreadReader struct{ t *testing.T }

func (r unreadReader) Read([]byte) (int, error) { r.t.Fatal("stdin read"); return 0, io.EOF }

// ACP-011: unexpected argv, including `--version`, is the NONCANONICAL_REQUEST
// sentinel with exit 0 and stdin is never read (decision 0236).
func TestRunRejectsUnexpectedArgv(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"--input-file", "request.json"}} {
		var out bytes.Buffer
		if got := run(args, unreadReader{t}, &out); got != 0 || out.String() != "{\"profile\":\"corvint-analyzer-candidate/experimental\",\"family\":\"unknown\",\"request_id\":\"unknown\",\"status\":\"REJECTED\",\"reason\":\"NONCANONICAL_REQUEST\"}\n" {
			t.Fatalf("args=%v status=%d output=%q", args, got, out.String())
		}
	}
}

type zeroWriter struct{}

func (zeroWriter) Write([]byte) (int, error) { return 0, nil }

// Decision 0228: a nonzero exit means only that the frame was not written in full.
func TestRunReturnsNonzeroWhenFrameIsNotWritten(t *testing.T) {
	if got := run(nil, bytes.NewBufferString("{}\n"), zeroWriter{}); got != 2 {
		t.Fatalf("status=%d", got)
	}
}
func TestRunCanonicalCandidate(t *testing.T) {
	raw := `{"profile":"corvint-analyzer-candidate/experimental","family":"go","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input-1","family":"go.mod","path":"go.mod","sha256":"sha256:8d27e4f4ff6e3f8c1a8ced8f550a286065c62bff008c069e5f4c12061a00fa77","content_base64":"bW9kdWxlIGV4YW1wbGUuY29tL21vZHVsZQpnbyAxLjI3LjAK"}]}` + "\n"
	var out bytes.Buffer
	want, err := analyzergo.Process([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if got := run(nil, bytes.NewBufferString(raw), &out); got != 0 || out.String() != string(want) {
		t.Fatalf("status=%d output=%q", got, out.String())
	}
}
func TestRunExactFramingBoundaryMatrix(t *testing.T) {
	valid := []byte(`{"profile":"corvint-analyzer-candidate/experimental","family":"go","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input-1","family":"go.mod","path":"go.mod","sha256":"sha256:8d27e4f4ff6e3f8c1a8ced8f550a286065c62bff008c069e5f4c12061a00fa77","content_base64":"bW9kdWxlIGV4YW1wbGUuY29tL21vZHVsZQpnbyAxLjI3LjAK"}]}` + "\n")
	frames := [][]byte{valid, valid[:len(valid)-1], append(append([]byte(nil), valid...), '\n')}
	for _, n := range []int{analyzergo.MaxRequestBytes - 1, analyzergo.MaxRequestBytes, analyzergo.MaxRequestBytes + 1} {
		raw := make([]byte, n)
		if n <= analyzergo.MaxRequestBytes {
			raw[0], raw[n-2], raw[n-1] = '{', '}', '\n'
		}
		frames = append(frames, raw)
	}
	for _, raw := range frames {
		want, err := analyzergo.Process(raw)
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if got := run(nil, chunkReader{bytes.NewReader(raw)}, &out); got != 0 || out.String() != string(want) {
			t.Fatalf("status=%d want=%q got=%q", got, want, out.String())
		}
	}
}
func TestRunGoBuildTagSemantics(t *testing.T) {
	for _, body := range []string{"//go:build go1.10\n\npackage p\n", "//go:build !enterprise\n\npackage p\n"} {
		sum := sha256.Sum256([]byte(body))
		raw, err := json.Marshal(analyzergo.Request{Profile: analyzergo.Profile, Family: analyzergo.Family, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: analyzergo.Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []analyzergo.Input{{Handle: "source-1", Family: "go.source", Path: "cmd/main.go", SHA256: "sha256:" + hex.EncodeToString(sum[:]), ContentBase64: base64.StdEncoding.EncodeToString([]byte(body))}}})
		if err != nil {
			t.Fatal(err)
		}
		raw = append(raw, '\n')
		want, err := analyzergo.Process(raw)
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if got := run(nil, bytes.NewReader(raw), &out); got != 0 || out.String() != string(want) || !bytes.Contains(out.Bytes(), []byte(`"status":"CANDIDATE"`)) {
			t.Fatalf("status=%d want=%q got=%q", got, want, out.String())
		}
	}
}
func TestProductionRunHelper(t *testing.T) {
	if os.Getenv("CORVINT_ANALYZER_PRODUCTION_SPY") != "1" {
		return
	}
	os.Exit(run(nil, os.Stdin, os.Stdout))
}
func TestProductionProcessNetworkRepositoryAndAmbientSpy(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "unexpected-capability-use")
	shim := filepath.Join(root, "git")
	if err := os.WriteFile(shim, []byte("#!/bin/sh\nprintf x > \"$CORVINT_ANALYZER_SPY_MARKER\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("repository-decoy\n"), 0600); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"profile":"corvint-analyzer-candidate/experimental","family":"go","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input-1","family":"go.mod","path":"go.mod","sha256":"sha256:8d27e4f4ff6e3f8c1a8ced8f550a286065c62bff008c069e5f4c12061a00fa77","content_base64":"bW9kdWxlIGV4YW1wbGUuY29tL21vZHVsZQpnbyAxLjI3LjAK"}]}` + "\n")
	want, err := analyzergo.Process(raw)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestProductionRunHelper$")
	cmd.Dir = root
	cmd.Stdin = bytes.NewReader(raw)
	cmd.Env = []string{
		"CORVINT_ANALYZER_PRODUCTION_SPY=1",
		"CORVINT_ANALYZER_SPY_MARKER=" + marker,
		"PATH=" + root,
		"HOME=" + root,
		"GIT_DIR=" + filepath.Join(root, ".git"),
		"ALL_PROXY=http://127.0.0.1:1",
		"HTTPS_PROXY=http://127.0.0.1:1",
	}
	got, err := cmd.Output()
	if err != nil || string(got) != string(want) {
		t.Fatalf("production spy err=%v want=%q got=%q", err, want, got)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("production process invoked a PATH shim: %v", err)
	}
}
