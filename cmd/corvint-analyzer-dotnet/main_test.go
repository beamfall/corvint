package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/analyzerdotnet"
)

const canonicalRequest = `{"profile":"corvint-analyzer-candidate/experimental","family":"dotnet",` +
	`"request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1",` +
	`"target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},` +
	`"inputs":[{"handle":"input-1","family":"dotnet.global-json","path":"global.json",` +
	`"sha256":"sha256:d3d8ea7b0a2d3c53e0ba3f4d9c9d10a4bfa06ff4a1cd68ff6e59e1a0e1cf8f37",` +
	`"content_base64":"eyJzZGsiOnsidmVyc2lvbiI6IjguMC40MjMifX0="}]}` + "\n"

func TestRunEmitsOneLFFramedResponse(t *testing.T) {
	var out bytes.Buffer
	if code := run(nil, strings.NewReader(canonicalRequest), &out); code != 0 {
		t.Fatalf("exit code=%d", code)
	}
	response := out.Bytes()
	if len(response) == 0 || response[len(response)-1] != '\n' {
		t.Fatalf("response is not LF framed: %q", response)
	}
	if bytes.Count(response, []byte("\n")) != 1 {
		t.Fatalf("response carries more than one LF: %q", response)
	}
	var decoded map[string]any
	if err := json.Unmarshal(response[:len(response)-1], &decoded); err != nil {
		t.Fatalf("response is not one JSON object: %v", err)
	}
	// The pinned digest above is deliberately wrong, so this proves the command
	// carries a bound rejection out rather than swallowing it.
	if decoded["reason"] != "DIGEST_MISMATCH" {
		t.Fatalf("unexpected response: %s", response)
	}
}

func TestRunRejectsOverlongInputWithoutDecoding(t *testing.T) {
	var out bytes.Buffer
	oversize := strings.Repeat("x", analyzerdotnet.MaxRequestBytes+1)
	if code := run(nil, strings.NewReader(oversize), &out); code != 0 {
		t.Fatalf("exit code=%d", code)
	}
	if !strings.Contains(out.String(), `"reason":"LIMIT_EXCEEDED"`) {
		t.Fatalf("expected LIMIT_EXCEEDED, got %s", out.String())
	}
}

func TestRunRejectsUnexpectedArgv(t *testing.T) {
	var out bytes.Buffer
	if code := run([]string{"--input-file", "request.json"}, strings.NewReader(canonicalRequest), &out); code != 0 || !strings.Contains(out.String(), `"reason":"NONCANONICAL_REQUEST"`) {
		t.Fatalf("code=%d out=%q", code, out.String())
	}
}

func TestRunReportsAReadFailureAsTheFixedSentinel(t *testing.T) {
	var out bytes.Buffer
	if code := run(nil, failingReader{}, &out); code != 0 {
		t.Fatalf("exit code=%d", code)
	}
	const want = `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown",` +
		`"request_id":"unknown","status":"REJECTED","reason":"NONCANONICAL_REQUEST"}` + "\n"
	if out.String() != want {
		t.Fatalf("sentinel drifted:\n got=%s\nwant=%s", out.String(), want)
	}
}

func TestRunReportsAWriteFailure(t *testing.T) {
	if code := run(nil, strings.NewReader(canonicalRequest), failingWriter{}); code != 2 {
		t.Fatalf("exit code=%d want 2", code)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
