package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestRunRejectsUnexpectedArgv(t *testing.T) {
	var out bytes.Buffer
	if code := run([]string{"--input-file", "request.json"}, strings.NewReader(""), &out); code != 0 || !strings.Contains(out.String(), `"reason":"NONCANONICAL_REQUEST"`) {
		t.Fatalf("code=%d out=%q", code, out.String())
	}
}

func TestRunAnalyzesStdinWithNoArgv(t *testing.T) {
	var out bytes.Buffer
	if code := run(nil, strings.NewReader(""), &out); code != 0 || !strings.Contains(out.String(), `"reason":"NONCANONICAL_REQUEST"`) {
		t.Fatalf("code=%d out=%q", code, out.String())
	}
}

// TestRunReportsReadFailureAsNoncanonical pins ACP-012: a stdin read failure
// is the NONCANONICAL_REQUEST sentinel with exit 0.
func TestRunReportsReadFailureAsNoncanonical(t *testing.T) {
	var out bytes.Buffer
	if code := run(nil, failingReader{}, &out); code != 0 || !strings.Contains(out.String(), `"reason":"NONCANONICAL_REQUEST"`) || !strings.HasSuffix(out.String(), "\n") {
		t.Fatalf("code=%d out=%q", code, out.String())
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

type boundedWriter struct {
	limit  int
	output []byte
	fail   error
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	if w.fail != nil {
		return 0, w.fail
	}
	if w.limit == 0 {
		return 0, nil
	}
	n := w.limit
	if n > len(p) {
		n = len(p)
	}
	w.output = append(w.output, p[:n]...)
	return n, nil
}

// ACP-011: run exits nonzero only when the frame is not written in full.
func TestRunReturnsNonzeroWhenFrameIsNotWritten(t *testing.T) {
	for _, writer := range []*boundedWriter{{fail: errors.New("write failed")}, {}} {
		if code := run(nil, strings.NewReader("{}\n"), writer); code != 2 {
			t.Fatalf("writer=%+v code=%d", writer, code)
		}
	}
}

func TestWriteAllHandlesShortAndErrorWrites(t *testing.T) {
	for _, tc := range []struct {
		name   string
		writer boundedWriter
		want   error
	}{
		{"short", boundedWriter{limit: 2}, nil},
		{"zero", boundedWriter{}, io.ErrShortWrite},
		{"error", boundedWriter{limit: 2, fail: errors.New("write failed")}, errors.New("write failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writer := tc.writer
			err := writeAll(&writer, []byte("canonical\n"))
			if tc.want == nil && err != nil || tc.want != nil && (err == nil || err.Error() != tc.want.Error()) {
				t.Fatalf("err=%v", err)
			}
			if tc.want == nil && string(writer.output) != "canonical\n" {
				t.Fatalf("short write lost bytes %q", writer.output)
			}
		})
	}
}
