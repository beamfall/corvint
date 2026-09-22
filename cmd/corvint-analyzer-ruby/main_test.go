package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/analyzerruby"
)

const rubyRequest = `{"profile":"corvint-analyzer-candidate/experimental","family":"ruby","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[]}` + "\n"
const rubyCandidate = `{"profile":"corvint-analyzer-candidate/experimental","family":"ruby","request_id":"request-1","status":"CANDIDATE","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[],"facts":[]}` + "\n"
const rubyRefusal = `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"NONCANONICAL_REQUEST"}` + "\n"

func TestRunPinsInvocationFramingAndRefusals(t *testing.T) {
	var output bytes.Buffer
	if code := run(nil, strings.NewReader(rubyRequest), &output); code != 0 || output.String() != rubyCandidate || strings.Count(output.String(), "\n") != 1 {
		t.Fatalf("candidate exit=%d output=%q", code, output.String())
	}

	for _, test := range []struct {
		name  string
		args  []string
		input io.Reader
		code  int
		want  string
	}{
		// ACP-011: unexpected argv is the NONCANONICAL_REQUEST sentinel with exit 0.
		{name: "argument", args: []string{"request.json"}, input: strings.NewReader(rubyRequest), want: rubyRefusal},
		// ACP-012: oversize and read failure are fixed sentinels with exit 0.
		{name: "oversize", input: strings.NewReader(strings.Repeat("x", analyzerruby.MaxRequestBytes+1)), want: strings.Replace(rubyRefusal, "NONCANONICAL_REQUEST", "LIMIT_EXCEEDED", 1)},
		{name: "read failure", input: failingReader{}, want: rubyRefusal},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			if code := run(test.args, test.input, &output); code != test.code || output.String() != test.want {
				t.Fatalf("exit=%d output=%q", code, output.String())
			}
		})
	}

	if code := run(nil, strings.NewReader(rubyRequest), failingWriter{}); code != 1 {
		t.Fatalf("write failure exit=%d", code)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
